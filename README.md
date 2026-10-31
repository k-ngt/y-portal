# y-portal

A small single-binary web portal for a shared machine. It serves each user's
`~/public_html` at `/~username/`, and turns Markdown (`.md`) and Typst (`.typ`)
files into HTML pages on the fly.

- The top page lists every user who has a `public_html` directory, along with
  system and build information.
- Markdown is rendered with [goldmark](https://github.com/yuin/goldmark)
  (GFM, footnotes, definition lists and more). Code blocks get Prism.js syntax
  highlighting.
- Typst is compiled with the `typst` CLI using its experimental HTML export.
- Rendered pages are cached on disk and rebuilt only when the source changes.
- Stylesheets stack in layers: site-wide, per user, per file type and per file.
- Images named like `photo@2x.png` get a matching `srcset` automatically.

## Contents

- [Requirements](#requirements)
- [Building](#building)
- [Running](#running)
- [URL layout](#url-layout)
- [How files are handled](#how-files-are-handled)
- [Caching](#caching)
- [Styling](#styling)
- [Retina images](#retina-images)
- [Permissions helper (`fix-pub.sh`)](#permissions-helper-fix-pubsh)
- [Project layout](#project-layout)
- [License](#license)

## Requirements

| What | Needed for | Notes |
| --- | --- | --- |
| Go 1.25+ | Building | See `go.mod`. |
| [`typst`](https://github.com/typst/typst) on `PATH` | Serving `.typ` files | Must support `typst compile --features html` (experimental HTML export). Without it, only `.typ` pages fail; everything else works. |
| PowerShell 7+ (`pwsh`) | `build.ps1` only | Uses `$IsWindows` / `$IsMacOS` / `$IsLinux`. |
| Swift (`swift`) | `build.ps1` only | Generates the build timestamp string. |
| `setfacl` (Linux `acl` package) | `fix-pub.sh` only | |

**Supported runtime OSes:** Linux, macOS and Windows. Home directories are
looked up at a fixed location for each:

| OS | Home base |
| --- | --- |
| Linux | `/home` |
| macOS | `/Users` |
| Windows | `C:\Users` |

`build.ps1` also builds BSD, illumos, Solaris, Plan 9 and AIX binaries. On
those systems the server starts, but `/` and `/~user/` respond with
`500 Unsupported OS` because no home base is defined for them.

## Building

Quick local build (version info stays at `dev` / `unknown`):

```bash
go build -o bin/y-portal ./cmd/y-portal
```

Release build for every target, with version and build metadata embedded:

```bash
pwsh ./build.ps1
```

Binaries are written to `bin/y-portal-<os>-<arch>[.exe]`. The script sets
`main.Version`, `main.BuildTime`, `main.BuildMachine` and `main.BuildGoVer`
through `-ldflags -X`, and shows them on the top page. The version string is
hard-coded near the top of `build.ps1`.

Templates and built-in stylesheets under `web/` are embedded into the binary
with `go:embed`. **If you change anything in `web/`, rebuild for it to take
effect.**

## Running

```bash
./bin/y-portal
```

- It listens on port **4501** on all interfaces. The port is fixed in
  `cmd/y-portal/main.go`.
- At startup it prints the OS, CPU and GPU it detected. Requests for user files
  are logged to stdout (`Accessing: ...`, `Rendering cache for: ...`).
- The account that runs the server needs:
  - **read** access to every user's `public_html`, and
  - **write** access to the directories that hold `.md` / `.typ` files, so it
    can create the `.y-cache/` cache directories (see [Caching](#caching)).

  `fix-pub.sh` grants this access to an account named `y-p-u`.

## URL layout

| URL | Serves |
| --- | --- |
| `/` | Portal top page: hostname, users with a `public_html`, IPv4 addresses, version, build and runtime info, uptime. |
| `/static/...` | Built-in assets embedded in the binary (`web/static/`). |
| `/~user` | Redirects to `/~user/`. |
| `/~user/path/to/file` | `<home base>/user/public_html/path/to/file`, handled by file type (below). |
| Anything else | The 404 page. |

The 404 page probes each parent directory of the requested URL and links to
the ones that exist, using each page's `<title>` as the link text.

## How files are handled

The server decides what to do from the file extension. Matching is
**case-sensitive**: `README.MD` is not treated as Markdown.

| Request | Processing | Cached | Styles added |
| --- | --- | --- | --- |
| `*.md` | Markdown → HTML through goldmark, wrapped in `web/templates/md.html` | Yes | Yes (`md` layer) |
| `*.typ` | `typst compile <src> --features html <out>`, then stylesheet links inserted before `</head>` | Yes | Yes (`typ` layer) |
| `*.html` | Stylesheet links inserted before the first `</head>` | No (processed on every request) | Yes (`html` layer) |
| Directory | Redirects to add a trailing `/`, then serves `index.html` in it as above | — | — |
| Anything else (images, PDFs, CSS, JS, `.txt`, …) | Served as is | No | No |
| Any path with `?raw=1` | Served as is, with no conversion | No | No |

Things to be aware of:

- **Only `index.html` counts as a directory index.** A directory that has
  `index.md` or `index.typ` but no `index.html` returns 404. Link to
  `index.md` directly, or add an `index.html`.
- **`?raw=1` on a directory returns a file listing** (Go's standard listing),
  even when there is no `index.html`. Do not count on hidden URLs to keep files
  private.
- The `.y-cache/` directories sit inside `public_html`, so they can be reached
  over HTTP like any other file.
- For `.html` and `.typ`, stylesheets are injected only if the document has a
  literal lowercase `</head>`. A file with no `</head>` (or `</HEAD>`) is sent
  without the extra stylesheets.

### Markdown

- **Extensions:** GFM (tables, strikethrough, task lists, autolinks), CJK line
  breaking, footnotes, typographic punctuation, definition lists and linkify.
- **Headings** get automatic `id`s, so you can link to `page.md#heading`.
- **Attributes** are supported, e.g. `## Title {#custom-id .class}`.
- **Raw HTML passes through unescaped** (goldmark's `WithUnsafe`). Treat
  Markdown authors as you would treat authors of HTML pages.
- **Code blocks** are highlighted in the browser by Prism.js
  (`prism-tomorrow` theme, with the autoloader). Prism loads from
  cdnjs.cloudflare.com, so the viewer needs internet access to get
  highlighting. Tag the fence with a language (e.g. `` ```go ``).
- The page `<title>` is the file name (e.g. `notes.md`).

### Typst

- The `typst` binary must be on the server's `PATH`. y-portal runs
  `typst c <file>.typ --features html <tmp>.html`.
- The page structure (`<head>`, `<title>` and so on) comes from Typst's own
  HTML output. The `md.html` template and Prism.js are not used.
- If compilation fails, the response is `500` with Typst's error output.

## Caching

Rendered `.md` and `.typ` pages are written to a `.y-cache/` directory next
to the source file:

```text
public_html/notes/
├── report.typ
├── todo.md
└── .y-cache/
    ├── report.typ.cache.html
    └── todo.md.cache.html
```

- A page is re-rendered when it has no cache yet, or when the **source file's
  modification time** is newer than the cache. Otherwise the cached file is
  served as is.
- Cache files are first written to a temporary file and then renamed into
  place, so a reader never sees a half-written page.
- **Nothing else invalidates the cache**, including:
  - files a Typst document `#import`s or `#include`s,
  - images or other assets the page refers to,
  - a new y-portal binary with changed templates or rendering.

  In those cases, `touch` the source file or delete the `.y-cache/` directory.
- Stylesheets are *linked*, not inlined. Editing a CSS file never requires a
  re-render: reloading the page is enough (subject to browser caching).

## Styling

Every converted page (`.md`, `.typ`, `.html`) gets up to five stylesheet links
in this order. **Later layers override earlier ones** at equal selector
specificity.

| # | URL | File on disk | Applies to |
| --- | --- | --- | --- |
| 1 | `/static/common.css` | `web/static/common.css` (built in) | All pages of all users, plus the top page and the 404 page |
| 2 | `/static/<type>.css` | `web/static/md.css`, `typ.css`, `html.css` (built in) | All pages of that type, for all users |
| 3 | `/~user/style/common.css` | `~user/public_html/style/common.css` | All of *that user's* converted pages, in every subdirectory |
| 4 | `/~user/style/<type>.css` | `~user/public_html/style/md.css`, `typ.css`, `html.css` | That user's pages of that type |
| 5 | value of the `@css` directive | wherever it points | That one file only |

`<type>` is `md`, `typ` or `html`, matching the source file's extension.
Markdown pages also load the Prism theme *before* layer 1, so every layer can
override code-block colors.

### Scope

- **Layers 1–2 affect every user.** They are compiled into the binary, so
  changing them means editing `web/static/` and rebuilding.
- **Layers 3–4 affect one user.** The URL is always `/~user/style/...` whatever
  the page's directory, so a single `style/` folder at the root of
  `public_html` styles every page in it, however deeply nested.
- **Layer 5 affects one file.**
- The top page uses only `common.css` and `root.css`, and the 404 page uses
  only `common.css`. User stylesheets never apply to either.
- Plain files (images, `.txt`, `?raw=1`, …) are never styled.
- User stylesheets are optional. A missing one just produces a 404 request in
  the browser, which does no harm.
- Your own `.html` pages also receive layers 1–4. To opt out of a rule, override
  it in your own CSS. Use `?raw=1` to view a page exactly as written.

### What the built-in styles do

- `common.css` turns on light and dark mode (`color-scheme: light dark`),
  defines `--accent-color` (maroon in light mode, orange in dark), applies
  `box-sizing: border-box`, uses `system-ui` fonts, caps media at
  `max-width: 100%`, adds themed scrollbars and enables cross-page view
  transitions.
- `typ.css` puts `code` in rounded boxes and inverts it in dark mode, because
  Typst emits light-mode colors for code.
- `root.css` contains small layout tweaks for the top page.
- `md.css` and `html.css` are currently empty placeholders.

### Per-file stylesheet (`@css` directive)

Add one line near the top of the file:

```markdown
<!-- @css: /~alice/style/slides.css -->
# My page
```

```typst
// @css: /~alice/style/report.css
= My report
```

```html
<!-- @css: /~alice/style/landing.css -->
<!DOCTYPE html>
...
```

- Either syntax works in any file type. Use the one that is a comment in that
  language: `<!-- ... -->` for Markdown and HTML, `// ...` for Typst.
- The directive must be on a line of its own (leading and trailing whitespace is
  fine). For the HTML-comment form, `-->` must be on the same line.
- Only the **first** directive in the **first 128 lines** is used. You get one
  extra stylesheet per file.
- The value becomes the `href` exactly as written. A relative path resolves
  against the page's URL, so a root-relative path (`/~user/...`) is the most
  predictable choice.

## Retina images

In rendered `.md`, `.typ` and `.html` output, any `<img>` whose `src` ends in
`@<scale>x.<ext>` gets a matching `srcset`:

```html
<img src="photo@2x.png">
<!-- becomes -->
<img src="photo@2x.png" srcset="photo@2x.png 2x">
```

The browser then shows the image at its intended display size rather than at
its pixel size. Fractional scales like `@1.5x` work too.

## Permissions helper (`fix-pub.sh`)

`fix-pub.sh` is meant to be installed as `/usr/local/bin/fix-pub` and run by
each user. It recursively grants the account `y-p-u` (the account the server
is expected to run as) `rwx` on the caller's `~/public_html` via an ACL:

```bash
fix-pub
```

The ACL covers only files and directories that already exist. Run it again
after adding new files or directories, or if pages start returning permission
errors.

## Project layout

```text
.
├── cmd/y-portal/main.go   # HTTP server, conversion, caching, CSS/srcset injection
├── web/
│   ├── embed.go           # go:embed for static/ and templates/
│   ├── static/            # built-in CSS (common, md, typ, html, root)
│   └── templates/         # root.html, md.html, 404.html
├── build.ps1              # multi-platform release build with version metadata
├── fix-pub.sh             # ACL helper for ~/public_html
└── go.mod
```

## License

[MIT](LICENSE)
