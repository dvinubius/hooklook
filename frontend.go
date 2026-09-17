package main

import (
	"net/http"
	"os"
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

// devShell references Vite's client and the application entry by absolute path.
// Vite serves both directly on the origin the browser is already using; every
// other path on that origin is proxied here.
const devShell = `<!doctype html>
<html lang="en" data-theme="dark">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="referrer" content="no-referrer">
<title>hooklook</title>
</head>
<body>
<div id="app"></div>
<script type="module" src="/@vite/client"></script>
<script type="module" src="/src/main.ts"></script>
</body>
</html>
`

func writeShell(w http.ResponseWriter, html string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(html))
}
