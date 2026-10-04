# RkTopNG - build helpers.
#
# Needs Go (https://go.dev/dl, 1.21 or newer) and Node (https://nodejs.org, 22 or newer, only to build the UI).
#
#   make test          unit tests: Go backend + frontend
#   make vet           go vet + svelte-check
#   make frontend      build the UI into exporter/web/dist (embedded in the binary)
#   make build-arm64   ONE static binary with the UI inside, for the CM3588 / RK3588 (arm64 only: the app reads
#                      Rockchip-specific sysfs and debugfs, so there is no x86 build)       -> dist/rktopng-linux-arm64
#   make dev           UI with hot reload, talking to a running exporter:  make dev RKTOP_API=http://<ip-board>:9888
#   make release VERSION=0.1.1           test + build both binaries + checksums in dist/release/ (see "Releases" below)
#   make release-publish VERSION=0.1.1   create the GitHub release v0.1.1 with those files

GO ?= go
NPM ?= npm
LDFLAGS := -s -w
RKTOP_API ?= http://127.0.0.1:9888

.PHONY: test test-go test-frontend vet frontend dev build-arm64 clean release release-check release-publish

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

clean:
	rm -rf dist frontend/node_modules

# --- Releases ---------------------------------------------------------------------------------
#
# A release is a git tag plus a GitHub release carrying the static arm64 binary and its checksum.
# The Docker image of the CasaOS store and native installs are both made from those same files.
#
#   1. commit, then:  git tag v0.1.1 && git push origin main v0.1.1
#   2. make release VERSION=0.1.1           (nothing leaves this machine)
#   3. make release-publish VERSION=0.1.1   (creates the public GitHub release)
#
# The checks refuse a release from a dirty tree, from a commit the tag does not point to, or with
# binaries built from another commit.
VERSION ?=
GH ?= gh
RELEASE_DIR := dist/release
RELEASE_FILES := $(RELEASE_DIR)/rktopng-linux-arm64 $(RELEASE_DIR)/SHA256SUMS

release-check:
	@echo "$(VERSION)" | grep -Eq '^[0-9]+\.[0-9]+\.[0-9]+$$' || { echo "usage: make release VERSION=0.1.1 (three numbers, no leading v)"; exit 1; }
	@[ -z "$$(git status --porcelain)" ] || { echo "the working tree has uncommitted changes: commit them first"; git status --short; exit 1; }
	@tag="$$(git rev-parse -q --verify 'v$(VERSION)^{commit}')" || { echo "tag v$(VERSION) does not exist: git tag v$(VERSION) && git push origin main v$(VERSION)"; exit 1; }; \
	  [ "$$tag" = "$$(git rev-parse HEAD)" ] || { echo "tag v$(VERSION) points to $$(echo $$tag | cut -c1-12), not to HEAD ($$(git rev-parse --short=12 HEAD)): check out the tagged commit"; exit 1; }

release: release-check test vet build-arm64
	rm -rf $(RELEASE_DIR) && mkdir -p $(RELEASE_DIR)
	cp dist/rktopng-linux-arm64 $(RELEASE_DIR)/
	cd $(RELEASE_DIR) && shasum -a 256 rktopng-linux-arm64 > SHA256SUMS
	git rev-parse HEAD > $(RELEASE_DIR)/COMMIT
	@echo; echo "Release v$(VERSION) built from $$(git rev-parse --short=12 HEAD) in $(RELEASE_DIR)/:"; cat $(RELEASE_DIR)/SHA256SUMS
	@echo "Publish it with:  make release-publish VERSION=$(VERSION)"

release-publish: release-check
	@[ -f $(RELEASE_DIR)/SHA256SUMS ] || { echo "run first:  make release VERSION=$(VERSION)"; exit 1; }
	@[ "$$(cat $(RELEASE_DIR)/COMMIT)" = "$$(git rev-parse HEAD)" ] || { echo "the files in $(RELEASE_DIR) come from another commit: run make release VERSION=$(VERSION) again"; exit 1; }
	@git ls-remote --exit-code --tags origin 'refs/tags/v$(VERSION)' >/dev/null || { echo "tag v$(VERSION) is not on origin: git push origin v$(VERSION)"; exit 1; }
	cd $(RELEASE_DIR) && shasum -a 256 -c SHA256SUMS
	$(GH) release create v$(VERSION) $(RELEASE_FILES) --verify-tag --title "RkTopNG v$(VERSION)" --generate-notes
