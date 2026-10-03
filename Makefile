# RkTopNG - build helpers.
#
# Needs Go (https://go.dev/dl, 1.21 or newer) and Node (https://nodejs.org, 22 or newer, only to build the UI).
#
#   make test          unit tests: Go backend + frontend
#   make vet           go vet + svelte-check
#   make frontend      build the UI into exporter/web/dist (embedded in the binary)
#   make build-arm64   ONE static binary with the UI inside, for the CM3588 / 64-bit ARM Linux -> dist/rktopng-linux-arm64
#   make build-amd64   the same for x86-64 Linux                                               -> dist/rktopng-linux-amd64
#   make dev           UI with hot reload, talking to a running exporter:  make dev RKTOP_API=http://<ip-board>:9888

GO ?= go
NPM ?= npm
LDFLAGS := -s -w
RKTOP_API ?= http://127.0.0.1:9888

.PHONY: test test-go test-frontend vet frontend dev build-arm64 build-amd64 clean

test: test-go test-frontend

# go:embed needs exporter/web/dist to exist even before the UI is built (a fresh clone,
# `go vet`, `go test`): this placeholder page keeps it valid. `make frontend` replaces it
# with the real app.
exporter/web/dist/index.html:
	mkdir -p exporter/web/dist
	printf '<!doctype html><html><head><title>RkTopNG</title></head><body>The dashboard is not built yet: run <code>make frontend</code>.</body></html>\n' > $@

test-go: exporter/web/dist/index.html
	cd exporter && $(GO) test ./...

test-frontend: frontend/node_modules
	cd frontend && $(NPM) test

vet: frontend/node_modules exporter/web/dist/index.html
	cd exporter && $(GO) vet ./...
	cd frontend && $(NPM) run check

# Install the UI's dependencies only when package.json / the lockfile changed.
frontend/node_modules: frontend/package.json frontend/package-lock.json
	cd frontend && $(NPM) ci
	@touch $@

# Vite writes straight into the Go module (exporter/web/dist) so `go:embed` picks it up.
frontend: frontend/node_modules
	cd frontend && $(NPM) run build

dev: frontend/node_modules
	cd frontend && RKTOP_API=$(RKTOP_API) $(NPM) run dev

build-arm64: frontend
	mkdir -p dist
	cd exporter && GOOS=linux GOARCH=arm64 CGO_ENABLED=0 $(GO) build -trimpath -ldflags="$(LDFLAGS)" -o ../dist/rktopng-linux-arm64 .

build-amd64: frontend
	mkdir -p dist
	cd exporter && GOOS=linux GOARCH=amd64 CGO_ENABLED=0 $(GO) build -trimpath -ldflags="$(LDFLAGS)" -o ../dist/rktopng-linux-amd64 .

clean:
	rm -rf dist frontend/node_modules
