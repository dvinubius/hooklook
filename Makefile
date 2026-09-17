# hooklook developer commands.
#
# Development runs two servers. The browser uses the Vite origin, which proxies
# the home, bin page, API, SSE and capture routes to Go; Go still serves and
# authorizes the page shell, and points it at Vite's module graph. Set
# PUBLIC_BASE_URL to the Vite origin so capture URLs and Go's same-origin check
# on owner mutations agree with the origin the browser is really on.

GO_ADDR      ?= 127.0.0.1:8080
DEV_ORIGIN   ?= http://localhost:5173
ADMIN_TOKEN  ?= local-operator-secret

.PHONY: dev-go dev-web build build-web test test-race vet clean-web

## Go server for development, serving the Vite-backed page shell.
dev-go:
	FRONTEND_DEV=1 PUBLIC_BASE_URL=$(DEV_ORIGIN) ADMIN_TOKEN=$(ADMIN_TOKEN) go run .

## Vite dev server. Open $(DEV_ORIGIN) — not the Go port.
dev-web:
	cd frontend && npm run dev

## Production frontend build. Must run before `go build`: the binary embeds
## frontend/dist, so the assets have to exist at Go build time.
build-web:
	cd frontend && npm ci && npm run build

## Full production build, in the required order.
build: build-web
	go build -o webhook-inspector .

test:
	cd frontend && npm test
	go test ./...

test-race:
	go test -race ./...

vet:
	go vet ./...

clean-web:
	rm -rf frontend/dist
