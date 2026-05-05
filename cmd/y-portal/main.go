package main

import (
	"bytes"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
	"y-portal/web"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
)

// ビルド時に -ldflags で上書きされるメタ情報
var (
	Version      = "dev"
	BuildTime    = "unknown"
	BuildMachine = "unknown"
	BuildGoVer   = "unknown"
)

var (
	osDetail string
	cpuModel string
	gpuModel string
	arch     string
	cachedOS string
)

var StartTime = time.Now()

var mdParser = goldmark.New(
	goldmark.WithExtensions(
		extension.GFM,
		extension.CJK,
		extension.Footnote,
		extension.Typographer,
		extension.DefinitionList,
		extension.Linkify,
	),
	goldmark.WithParserOptions(
		parser.WithAutoHeadingID(),
		parser.WithAttribute(),
		parser.WithHeadingAttribute(),
	),
	goldmark.WithRendererOptions(
		html.WithUnsafe(),
	),
)

type PageData struct {
	Title     string
	Body      template.HTML
	Username  string
	GlobalCSS string
	ManualCSS string
}

type ErrorData struct {
	Message string
}

type RootData struct {
	Hostname    string
	Users       []string
	IPs         []string
	Version     string
	BuildTime   string
	BuildInfo   string
	RuntimeInfo string
	Uptime      string
}

var tmpl *template.Template // グローバル変数

func init() {
	var err error
	// 起動時に1回だけパース
	tmpl, err = template.ParseFS(web.Content, "templates/*.html")
	if err != nil {
		panic(err)
	}
}

func main() {
	// 起動時
	fmt.Print("Gathering system info...")
	arch = runtime.GOARCH
	cachedOS = runtime.GOOS
	osDetail = getRuntimeOSDetail()
	cpuModel = getCPUModel()
	gpuModel = getGPUModel()
	fmt.Print("\033[2K\r")
	fmt.Printf(" OS : %s [%s]\n", osDetail, arch)
	fmt.Printf(" CPU: %s\n", cpuModel)
	fmt.Printf(" GPU: %s\n", gpuModel)

	// 静的ファイル配信
	http.Handle("/static/", http.FileServer(http.FS(web.Content)))

	// ルート
	http.HandleFunc("/{$}", func(w http.ResponseWriter, r *http.Request) {
		// ホスト名
		hostname, err := os.Hostname()
		if err != nil {
			hostname = "y-portal"
		}

		// IPv4アドレスの取得
		ips := []string{}
		addrs, _ := net.InterfaceAddrs()
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
				if ipnet.IP.To4() != nil {
					ips = append(ips, ipnet.IP.String())
				}
			}
		}

		// 有効なユーザの取得
		baseHome := getBaseHome()
		if baseHome == "" {
			http.Error(w, "Unsupported OS", 500)
			return
		}

		users := []string{}
		entries, _ := os.ReadDir(baseHome)
		for _, entry := range entries {
			if entry.IsDir() {
				username := entry.Name()
				// 隠しディレクトリの除外
				if strings.HasPrefix(username, ".") {
					continue
				}

				// public_htmlの確認
				pubPath := filepath.Join(baseHome, username, "public_html")
				if info, err := os.Stat(pubPath); err == nil && info.IsDir() {
					users = append(users, username)
				}
			}
		}

		// 実行環境
		pid := os.Getpid()
		cpus := runtime.NumCPU()
		runtimeInfo := fmt.Sprintf("%s [%s; %s (%d Cores); %s] [PID: %d] (%s)",
			osDetail, arch, cpuModel, cpus, gpuModel, pid, runtime.Version())

		// 起動からの経過時間
		uptimeDuration := time.Since(StartTime)

		// テンプレート
		if err := tmpl.ExecuteTemplate(w, "root.html", RootData{
			Hostname:    hostname,
			Users:       users,
			IPs:         ips,
			Version:     Version,
			BuildTime:   BuildTime,
			BuildInfo:   fmt.Sprintf("%s (%s)", BuildMachine, BuildGoVer),
			RuntimeInfo: runtimeInfo,
			Uptime:      uptimeDuration.String(),
		}); err != nil {
			http.Error(w, err.Error(), 500)
		}
	})

	// ユーザディレクトリ
	http.HandleFunc("/{path...}", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		if strings.HasPrefix(path, "/~") {
			userDirHandler(w, r)
			return
		}

		serve404(w, fmt.Sprintf("不正なURLパスです: %s", path))
	})

	port := ":4501"
	fmt.Printf("y-portal %s (Built at: %s)\n", Version, BuildTime)
	fmt.Printf("Server starting at http://localhost%s\n", port)
	if err := http.ListenAndServe(port, nil); err != nil {
		fmt.Printf("Error: %v\n", err)
	}
}

func userDirHandler(w http.ResponseWriter, r *http.Request) {
	// URLパス: /~username/path/to/file
	trimmed := strings.TrimPrefix(r.URL.Path, "/~")
	parts := strings.SplitN(trimmed, "/", 2)
	username := parts[0]

	subPath := ""
	if len(parts) > 1 {
		subPath = parts[1]
	}

	baseHome := getBaseHome()
	if baseHome == "" {
		http.Error(w, "Unsupported OS", 500)
		return
	}

	// 実際のパスを組み立てる
	fullPath := filepath.Join(baseHome, username, "public_html", subPath)

	info, err := os.Stat(fullPath)
	if err != nil {
		serve404(w, fmt.Sprintf("ファイルが存在しません: %s", fullPath))
		return
	}

	// ?raw=1 が付いてる場合は, 変換せずに生のファイルを返す
	if r.URL.Query().Get("raw") == "1" {
		http.ServeFile(w, r, fullPath)
		return
	}

	if info.IsDir() {
		// ディレクトリやのにURLの末尾が "/" で終わってない場合
		if !strings.HasSuffix(r.URL.Path, "/") {
			// 末尾に "/" をつけたURLにリダイレクトさせる
			http.Redirect(w, r, r.URL.Path+"/", http.StatusMovedPermanently)
			return
		}

		fullPath = filepath.Join(fullPath, "index.html")
	}

	fmt.Printf("Accessing: %s\n", fullPath)

	// 拡張子を見て処理を分岐
	ext := filepath.Ext(fullPath)
	switch ext {
	case ".md":
		serveWithCache(w, r, fullPath, "md", username, info)
	case ".typ":
		serveWithCache(w, r, fullPath, "typ", username, info)
	case ".html":
		htmlData, err := os.ReadFile(fullPath)
		if err != nil {
			serve404(w, fmt.Sprintf("HTMLファイルの読み込みに失敗しました: %v", err))
			return
		}
		manualCSS := extractManualCSS(fullPath)
		cssTags := buildCSSTags(username, "html.css", manualCSS)
		injectedHTML := strings.Replace(string(htmlData), "</head>", cssTags+"</head>", 1)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(injectedHTML))
	default:
		http.ServeFile(w, r, fullPath)
	}
}

func serveWithCache(w http.ResponseWriter, r *http.Request, srcPath string, mode string, username string, srcStat os.FileInfo) {
	// キャッシュ用ディレクトリの準備
	cacheDir := filepath.Join(filepath.Dir(srcPath), ".y-cache")
	// キャッシュファイル名の決定
	cacheFile := filepath.Join(cacheDir, filepath.Base(srcPath)+".cache.html")
	// タイムスタンプの比較
	cacheStat, err := os.Stat(cacheFile)

	// キャッシュがない, またはソースの方が新しい場合に再レンダリング
	if err != nil || srcStat.ModTime().After(cacheStat.ModTime()) {
		fmt.Printf("Rendering cache for: %s\n", srcPath)

		os.Mkdir(cacheDir, 0777)

		var htmlContent string
		switch mode {
		case "md":
			htmlContent = convMD(srcPath, username)
			// ユニークな一時ファイルを作成してアトミックにリネーム
			tmpFile := fmt.Sprintf("%s.%d.tmp", cacheFile, time.Now().UnixNano())
			// キャッシュファイルに書き出し
			os.WriteFile(tmpFile, []byte(htmlContent), 0644)
			// リネーム
			os.Rename(tmpFile, cacheFile)
		case "typ":
			// ユニークかつTypstがHTML出力と認識できる拡張子の一時ファイル名
			tmpFile := fmt.Sprintf("%s.%d.tmp.html", cacheFile, time.Now().UnixNano())

			// Typstには一時ファイルをターゲットとして出力させる
			if err := convTyp(srcPath, tmpFile); err != nil {
				http.Error(w, "Typst Compilation Error: "+err.Error(), 500)
				return
			}

			htmlData, err := os.ReadFile(tmpFile)
			if err != nil {
				http.Error(w, "Cache Read Error: "+err.Error(), 500)
				return
			}

			manualCSS := extractManualCSS(srcPath)
			cssTags := buildCSSTags(username, "typ.css", manualCSS)
			injectedHTML := strings.Replace(string(htmlData), "</head>", cssTags+"</head>", 1)

			os.WriteFile(tmpFile, []byte(injectedHTML), 0644)
			os.Rename(tmpFile, cacheFile)
		}
	}
	// キャッシュファイルを返す
	http.ServeFile(w, r, cacheFile)
}

func convTyp(src, dst string) error {
	cmd := exec.Command("typst", "c", src, "--features", "html", dst)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("%v: %s", err, stderr.String())
	}

	return nil
}

func convMD(path string, username string) string {
	data, _ := os.ReadFile(path)
	var buf strings.Builder
	if err := mdParser.Convert(data, &buf); err != nil {
		return "Error converting Markdown"
	}

	pData := PageData{
		Title:     filepath.Base(path),
		Body:      template.HTML(buf.String()),
		Username:  username,
		GlobalCSS: "md.css",
		ManualCSS: extractManualCSS(path),
	}

	var out bytes.Buffer
	if err := tmpl.ExecuteTemplate(&out, "md.html", pData); err != nil {
		return "Error executing Markdown template"
	}
	return out.String()
}

func buildCSSTags(username string, globalCSS string, manualCSS string) string {
	var sb strings.Builder

	// Global CSS
	sb.WriteString("<link rel=\"stylesheet\" href=\"/static/common.css\">\n")
	fmt.Fprintf(&sb, "<link rel=\"stylesheet\" href=\"/static/%s\">\n", globalCSS)
	// User Auto
	fmt.Fprintf(&sb, "<link rel=\"stylesheet\" href=\"/~%s/style/common.css\">\n", username)
	fmt.Fprintf(&sb, "<link rel=\"stylesheet\" href=\"/~%s/style/%s\">\n", username, globalCSS)
	// User Manual
	if manualCSS != "" {
		fmt.Fprintf(&sb, "<link rel=\"stylesheet\" href=\"%s\">\n", manualCSS)
	}

	return sb.String()
}

func extractManualCSS(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}

	lines := strings.Split(string(data), "\n")
	searchLimit := 128
	if len(lines) < searchLimit {
		searchLimit = len(lines)
	}

	for i := 0; i < searchLimit; i++ {
		line := strings.TrimSpace(lines[i])
		if strings.HasPrefix(line, "// @css:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "// @css:"))
		}
		if strings.HasPrefix(line, "<!-- @css:") {
			return strings.TrimSpace(
				strings.TrimPrefix(
					strings.TrimSuffix(line, "-->"), "<!-- @css:"))
		}
	}

	return ""
}

func serve404(w http.ResponseWriter, errMsg string) {
	w.WriteHeader(http.StatusNotFound)

	// エラーメッセージを構造体に詰めて渡す
	if err := tmpl.ExecuteTemplate(w, "404.html", ErrorData{Message: errMsg}); err != nil {
		fmt.Fprint(w, "404 Not Found (Template Error)")
	}
}

func getRuntimeOSDetail() string {
	var detail string
	switch runtime.GOOS {
	case "darwin":
		name, _ := exec.Command("sw_vers", "-productName").Output()   // macOS
		out, _ := exec.Command("sw_vers", "-productVersion").Output() // 15.7.5
		build, _ := exec.Command("sw_vers", "-buildVersion").Output() // 24G624
		kernel, _ := exec.Command("uname", "-v").Output()             // Darwin...

		ver := strings.TrimSpace(string(out))
		marketing := "X"
		if strings.HasPrefix(ver, "26.") {
			marketing = "Tahoe"
		}
		if strings.HasPrefix(ver, "15.") {
			marketing = "Sequoia"
		}
		if strings.HasPrefix(ver, "14.") {
			marketing = "Sonoma"
		}

		detail = fmt.Sprintf("%s %s %s (Build %s) [%s]",
			strings.TrimSpace(string(name)),      // macOS
			strings.TrimSpace(string(marketing)), // Sequoia
			strings.TrimSpace(string(out)),       // 15.7.5
			strings.TrimSpace(string(build)),     // 24G624
			strings.TrimSpace(string(kernel)))    // Darwin...
	case "linux":
		pretty, _ := exec.Command("sh", "-c", "grep PRETTY_NAME /etc/os-release | cut -d'\"' -f2").Output()
		kernel, _ := exec.Command("uname", "-sr").Output()
		detail = fmt.Sprintf("%s (%s)", strings.TrimSpace(string(pretty)), strings.TrimSpace(string(kernel)))
	case "windows":
		cmdStr := `$os = Get-CimInstance Win32_OperatingSystem; $reg = Get-ItemProperty "HKLM:\SOFTWARE\Microsoft\Windows NT\CurrentVersion"; Write-Output "$($os.Caption) $($reg.DisplayVersion) (Build $($os.BuildNumber))"`
		out, _ := exec.Command("powershell", "-Command", cmdStr).Output()
		detail = strings.TrimSpace(string(out))
	default:
		detail = runtime.GOOS
	}

	return detail
}

func getCPUModel() string {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("sysctl", "-n", "machdep.cpu.brand_string")
	case "linux":
		cmd = exec.Command("sh", "-c", "grep 'model name' /proc/cpuinfo | head -n1 | cut -d':' -f2")
	case "windows":
		cmd = exec.Command("wmic", "cpu", "get", "name")
	default:
		return "Unknown CPU"
	}
	out, _ := cmd.Output()
	return strings.TrimSpace(string(out))
}

func getGPUModel() string {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		if runtime.GOARCH == "arm64" {
			// Apple Siliconの場合は AGXAccelerator
			cmd = exec.Command("sh", "-c", "ioreg -c AGXAccelerator -r -l | grep '\"model\"' | cut -d'\"' -f4 | head -n1")
		} else {
			// Intel Macの場合は IOPCIDevice
			cmd = exec.Command("sh", "-c", "ioreg -c IOPCIDevice -r -l | grep '\"model\"' | cut -d'\"' -f4 | head -n1")
		}
	case "linux":
		cmd = exec.Command("sh", "-c", "lspci | grep -i vga | cut -d':' -f3")
	case "windows":
		cmd = exec.Command("powershell", "-Command", "Get-CimInstance Win32_VideoController | Select-Object -ExpandProperty Name")
	default:
		return "Unknown GPU"
	}
	out, _ := cmd.Output()
	return strings.TrimSpace(string(out))
}

func getBaseHome() string {
	switch runtime.GOOS {
	case "darwin":
		return "/Users"
	case "linux":
		return "/home"
	case "windows":
		return "C:\\Users"
	default:
		return ""
	}
}
