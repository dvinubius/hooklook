package main

import (
	"bytes"
	"embed"
	"io/fs"
	"mime"
	"net/http"
	"os"
	"strings"
)

const frontendDevEnvironmentVariable = "FRONTEND_DEV"

// frontendDev reports whether the page shell should load modules from a running
// Vite dev server instead of a production build. In development the browser
// stays on the Vite origin, which proxies page, API, capture and home requests
// back to this service, so the owner cookie, the same-origin check on
// mutations and EventSource all see one origin — and the page is still served
// only after this service has authorized it.
func frontendDev() bool {
	return os.Getenv(frontendDevEnvironmentVariable) != ""
}

// The production build is embedded, so the binary is the whole deployment.
// `all:` keeps the committed `.gitkeep` eligible, which lets `go build` work on
// a fresh checkout before anyone has run `make build-web`; the page route then
// reports the missing build instead of serving a blank document.
//
//go:embed all:frontend/dist
var frontendBuild embed.FS

// builtFrontend is frontend/dist rooted at itself, so "index.html" and
// "assets/…" are exactly the paths the built document already references.
var builtFrontend = mustSub(frontendBuild, "frontend/dist")

func mustSub(embedded embed.FS, dir string) fs.FS {
	sub, err := fs.Sub(embedded, dir)
	if err != nil {
		panic(err) // the embed pattern above guarantees the directory exists
	}
	return sub
}

// prodShell is the built application document, read once at startup. A build
// error is kept rather than ignored: serving nothing silently would look like
// an empty bin to the visitor and like a working server to the operator.
var prodShell, prodShellErr = fs.ReadFile(builtFrontend, "index.html")

func init() {
	// Go's built-in table has no font types, and assets we ship ourselves must
	// not depend on whatever mime database the host machine happens to have.
	mime.AddExtensionType(".woff2", "font/woff2")
	mime.AddExtensionType(".woff", "font/woff")
}

// devShell references Vite's client and the application entry by absolute path.
// Vite serves both directly on the origin the browser is already using; every
// other path on that origin is proxied here.
var devShell = []byte(`<!doctype html>
<html lang="en" data-theme="dark">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="referrer" content="no-referrer">
<link rel="icon" href="/favicon.svg" type="image/svg+xml">
<title>hooklook</title>
</head>
<body>
<div id="app"></div>
<script type="module" src="/@vite/client"></script>
<script type="module" src="/src/main.ts"></script>
</body>
</html>
`)

// capacityShell answers a browser that asked for a bin page when no bin could
// be created. It carries no script and no asset references, so it still renders
// when the service cannot serve anything else — which is why the few brand
// values it needs are inlined here instead of coming from the stylesheet.
var capacityShell = []byte(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="referrer" content="no-referrer">
<title>hooklook — no bin available</title>
<style>
body{margin:0;padding:15vh 24px;background:#141414;color:#FAFAFA;
font:400 15px/1.55 ui-sans-serif,system-ui,sans-serif}
main{max-width:34rem;margin:0 auto}
h1{font-size:30px;letter-spacing:-.022em;margin:0 0 .6em}
p{color:#9A9A9A;margin:0 0 1em}
code{font-family:ui-monospace,SFMono-Regular,monospace;color:#DE8A42}
</style>
</head>
<body>
<main>
<h1>No bin available</h1>
<p>hooklook could not create a bin for you right now. Existing bins and their
captured requests are untouched.</p>
<p>Reload <code>/</code> in a moment. If this keeps happening, the service is
out of room for new bins and its operator needs to free some.</p>
</main>
</body>
</html>
`)

var expiredBinShell = []byte(`<!doctype html>
<html lang="en">
<head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="referrer" content="no-referrer"><title>hooklook — bin expired</title></head>
<body><main><p>// this bin has expired or no longer exists</p><a href="/">Create New Bin</a></main></body>
</html>
`)

var sharedBinUnavailableShell = []byte(`<!doctype html>
<html lang="en">
<head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="referrer" content="no-referrer"><title>hooklook — shared bin unavailable</title></head>
<body><main><p>// this shared bin no longer exists</p><a href="/">Create New Bin</a></main></body>
</html>
`)

// writeShell sends a full HTML document. Callers set caching and authorize the
// visitor first; this only decides the content type.
func writeShell(w http.ResponseWriter, html []byte) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(html)
}

// writePageShell answers an already-authorized page request with the
// application. Development loads modules from Vite; production serves the
// embedded build.
func writePageShell(w http.ResponseWriter) {
	if frontendDev() {
		writeShell(w, devShell)
		return
	}
	if prodShellErr != nil {
		http.Error(w, "frontend build missing: run `make build-web`, then rebuild the binary", http.StatusInternalServerError)
		return
	}
	writeShell(w, prodShell)
}

// A full store can prevent a first-time visitor from obtaining a bin. Serve
// the same application bundle with a document marker so the browser can show
// that state without an owner cookie or a bin-metadata request. The standalone
// page remains a fallback when the production frontend was not built.
func writeStartupShell(w http.ResponseWriter, state string, status int, fallback []byte) {
	w.Header().Set("X-Hooklook-Error", state)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	shell := prodShell
	if frontendDev() {
		shell = devShell
	}
	if frontendDev() || prodShellErr == nil {
		marked := bytes.Replace(shell, []byte("<html"), []byte(`<html data-hooklook-startup="`+state+`"`), 1)
		if !bytes.Equal(marked, shell) {
			_, _ = w.Write(marked)
			return
		}
	}
	_, _ = w.Write(fallback)
}

func writeCapacityShell(w http.ResponseWriter) {
	writeStartupShell(w, "store_full", http.StatusInsufficientStorage, capacityShell)
}

// An unavailable regular bin URL needs an explicit dead end before the visitor
// chooses to create a replacement. The marked application document performs
// no private API request.
func writeBinExpiredShell(w http.ResponseWriter) {
	writeStartupShell(w, "bin_expired", http.StatusNotFound, expiredBinShell)
}

// An unavailable URL with a nonempty invitation gets the shared-link version
// of the same explicit dead end, regardless of the target's database state.
func writeSharedBinUnavailableShell(w http.ResponseWriter) {
	writeStartupShell(w, "shared_bin_unavailable", http.StatusNotFound, sharedBinUnavailableShell)
}

// Cache lifetimes for the two kinds of built asset. Vite hashes the JavaScript
// and CSS filenames by content, so a cached copy can never go stale. Fonts are
// copied through verbatim under stable names, so they get an ordinary lifetime
// instead of a permanent one.
const (
	hashedAssetCache = "public, max-age=31536000, immutable"
	staticFileCache  = "public, max-age=86400"
)

// builtAsset serves a file from the embedded build. Asset requests deliberately
// skip bin authorization: they hold no captured data, they are identical for
// every visitor, and the browser has to be able to fetch them for a page this
// service has already decided to hand over.
func builtAsset(cacheControl string) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		name := strings.TrimPrefix(req.URL.Path, "/")
		info, err := fs.Stat(builtFrontend, name)
		if err != nil || !info.Mode().IsRegular() {
			http.NotFound(w, req)
			return
		}
		w.Header().Set("Cache-Control", cacheControl)
		http.ServeFileFS(w, req, builtFrontend, name)
	}
}
