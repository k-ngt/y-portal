# build.ps1
$ErrorActionPreference = "Stop"

# メタ情報の生成
$Version = "v1.0.0"

$SwiftCode = @'
import Foundation
let f = DateFormatter()
f.calendar = Calendar(identifier: .japanese)
f.locale = Locale(identifier: "en_US")
// EEEE: 曜日, MMMM: 月, dd: 日, GGGG: 元号, y: 年
// D: 年間通算日, w: 年間週番号
// HH:mm:ss.SSSS: 時分秒ミリ秒
// zzzz: タイムゾーン名, OOOO: GMTオフセット, VV: タイムゾーンID
f.dateFormat = "EEEE, MMMM d, GGGG y ('Day of Year:' D, 'Week of Year:' w) 'at' HH:mm:ss.SSSS (zzzz / OOOO / VV)"
print(f.string(from: Date()))
'@
$BuildTime = swift -e $SwiftCode

# OS情報の取得
$BuildArch = if ($IsWindows) { $env:PROCESSOR_ARCHITECTURE } else { uname -m }
$BuildMachine = ""
if ($IsWindows) {
    # Microsoft Windows 11 Pro 25H2 (Build 26200)
    $os = Get-CimInstance Win32_OperatingSystem
    $reg = Get-ItemProperty "HKLM:\SOFTWARE\Microsoft\Windows NT\CurrentVersion"
    $BuildMachine = "$($os.Caption) $($reg.DisplayVersion) (Build $($os.BuildNumber)) [$BuildArch]"
} 
elseif ($IsMacOS) {
    # macOS Sequoia 15.7.5 (Build 24G624)
    $ver = sw_vers -productVersion
    $build = sw_vers -buildVersion
    $kernel = (uname -v).Trim()
    $marketing = switch -Regex ($ver) {
        "^26\." { "Tahoe" }
        "^15\." { "Sequoia" }
        "^14\." { "Sonoma" }
        Default { "X" }
    }
    $BuildMachine = "macOS $marketing $ver (Build $build) [$BuildArch] [$kernel]"
} 
elseif ($IsLinux) {
    # Ubuntu 26.04 LTS (Linux 6.xx.x-generic)
    $pretty = (Get-Content /etc/os-release | Select-String "PRETTY_NAME").ToString().Split('=')[1].Trim('"')
    $kernel = uname -sr
    $BuildMachine = "$pretty ($kernel) [$BuildArch]"
} 
else {
    $BuildMachine = "Unknown OS"
}

$GoVersion = (go version).Split(" ")[2]

# フラグ
# -s: シンボルテーブルを削除
# -w: DWARFデバッグ情報を削除
# -X: main.goの変数に値を入れる
$LdFlags = "-s -w -X `"main.Version=$Version`" " +
"-X `"main.BuildTime=$BuildTime`" " +
"-X `"main.BuildMachine=$BuildMachine`" " +
"-X `"main.BuildGoVer=$GoVersion`""

# ビルド対象のOS/Archリスト
$Targets = @(
    # Windows
    @{ OS = "windows"; Arch = "amd64"; Ext = ".exe" },
    @{ OS = "windows"; Arch = "arm64"; Ext = ".exe" },
    # Darwin
    @{ OS = "darwin"; Arch = "amd64"; Ext = "" },
    @{ OS = "darwin"; Arch = "arm64"; Ext = "" },

    # Linux
    @{ OS = "linux"; Arch = "amd64"; Ext = "" },
    @{ OS = "linux"; Arch = "arm64"; Ext = "" },
    @{ OS = "linux"; Arch = "ppc64"; Ext = "" }, # PowerPC 64-bit Big Endian
    @{ OS = "linux"; Arch = "ppc64le"; Ext = "" }, # PowerPC 64-bit Little Endian
    @{ OS = "linux"; Arch = "mips64"; Ext = "" }, # MIPS 64-bit
    @{ OS = "linux"; Arch = "mips64le"; Ext = "" }, # MIPS 64-bit Little Endian
    @{ OS = "linux"; Arch = "s390x"; Ext = "" }, # IBM System z
    @{ OS = "linux"; Arch = "riscv64"; Ext = "" }, # RISC-V 64-bit
    @{ OS = "linux"; Arch = "loong64"; Ext = "" }, # LoongArch 64-bit

    # BSD
    @{ OS = "freebsd"; Arch = "amd64"; Ext = "" },
    @{ OS = "freebsd"; Arch = "arm64"; Ext = "" },
    @{ OS = "freebsd"; Arch = "riscv64"; Ext = "" },
    @{ OS = "openbsd"; Arch = "amd64"; Ext = "" },
    @{ OS = "openbsd"; Arch = "arm64"; Ext = "" },
    # @{ OS = "openbsd"; Arch = "mips64";  Ext = "" },
    @{ OS = "netbsd"; Arch = "amd64"; Ext = "" },
    @{ OS = "netbsd"; Arch = "arm64"; Ext = "" },

    # Minor
    @{ OS = "illumos"; Arch = "amd64"; Ext = "" },
    @{ OS = "solaris"; Arch = "amd64"; Ext = "" },
    @{ OS = "plan9"; Arch = "amd64"; Ext = "" },
    @{ OS = "aix"; Arch = "ppc64"; Ext = "" }  # IBM AIX
)

Write-Host "==> Building y-portal ($Version)..." -ForegroundColor Cyan

# binディレクトリがなければ作成
if (-not (Test-Path -Path "bin")) {
    New-Item -ItemType Directory -Path "bin" | Out-Null
}

# 各環境向けにループしてビルド
foreach ($target in $Targets) {
    $env:GOOS = $target.OS
    $env:GOARCH = $target.Arch
    $OutputFile = "bin/y-portal-$($target.OS)-$($target.Arch)$($target.Ext)"

    Write-Host "  -> Building for $($target.OS)/$($target.Arch)..."
    
    # -trimpath: バイナリから開発マシンの絶対パスを消す
    go build -trimpath -ldflags $LdFlags -o $OutputFile ./cmd/y-portal

    # Goのコンパイルが失敗した場合はスクリプトを止める
    if ($LASTEXITCODE -ne 0) {
        Write-Error "Build failed for $($target.OS)/$($target.Arch)!"
        exit 1
    }
}

# 環境変数を元に戻す
$env:GOOS = ""
$env:GOARCH = ""

Write-Host "==> Build Complete!" -ForegroundColor Green
