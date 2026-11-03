# Workout Explorer: the single build entry point for people and CI (ADR-001).
# CI runs `make -k ci`; everything else here is for local use.
# Target contract: architect/F-01-project-skeleton-and-ci.md, "Make targets".

SHELL := /bin/sh

SERVER_DIR := src/server
WEB_DIR    := src/web
BUILD_DIR  := build
BINARY     := $(BUILD_DIR)/workout-explorer

# Version stamped into the binary; `dev` is used by `go run` (Technical Design, Conventions: Build stamping).
VERSION ?= $(shell git describe --always --dirty 2>/dev/null || echo unknown)

# Dev address for `dev-server` and the Vite proxy in `dev-web`; see F-01 design, "Local development address".
DEV_ADDR ?= 127.0.0.1:8080
ifeq ($(strip $(DEV_ADDR)),)
override DEV_ADDR := 127.0.0.1:8080
endif

# Origin of absolute links in Open Graph tags (F-23 PC-8): the local Caddy site
# (deploy/Caddyfile's WX_SITE default), which serves the pages in dev.
DEV_BASE_URL ?= http://127.0.0.1:8088

.PHONY: deps deps-server deps-web \
        lint lint-server lint-web \
        test test-server test-web \
        build build-server build-web \
        size ci caddy caddy-check docker-check corpus corpus-update corpus-gen prototype-data prototype-serve dev-server dev-web clean

# ---- dependencies ---------------------------------------------------------

deps: deps-server deps-web

deps-server:
	cd $(SERVER_DIR) && go mod download

deps-web:
	cd $(WEB_DIR) && npm ci

# ---- lint -----------------------------------------------------------------

lint: lint-server lint-web

lint-server: deps-server
	@unformatted="$$(cd $(SERVER_DIR) && gofmt -l .)"; \
	if [ -n "$$unformatted" ]; then \
		echo "gofmt: these files need formatting:"; echo "$$unformatted"; exit 1; \
	fi
	cd $(SERVER_DIR) && go vet -tags caddycheck ./...

lint-web: deps-web
	cd $(WEB_DIR) && npm run check

# ---- test -----------------------------------------------------------------

test: test-server test-web

test-server: deps-server
	cd $(SERVER_DIR) && go test -count=1 ./...

test-web: deps-web
	cd $(WEB_DIR) && npm run test

# ---- build ----------------------------------------------------------------

build: build-server build-web

build-server:
	cd $(SERVER_DIR) && CGO_ENABLED=0 go build \
		-ldflags "-X main.version=$(VERSION)" \
		-o ../../$(BINARY) ./cmd/workout-explorer

build-web: deps-web
	cd $(WEB_DIR) && npm run build

# Bundle budget check (ADR-002). Depends only on build-web, never on test,
# so the size is printed even when a test fails under `make -k ci`.
size: build-web
	cd $(WEB_DIR) && npm run size

# ---- CI -------------------------------------------------------------------

ci: lint test build size caddy-check docker-check

# ---- Caddy (F-22 H-7, H-12) -------------------------------------------------

include deploy/caddy.env
CADDY_PINNED := $(BUILD_DIR)/tools/caddy-$(CADDY_VERSION)/caddy
# CADDY=<binary> runs the check against another Caddy (for example to see 2.10.2 fail).
CADDY ?= $(CADDY_PINNED)

# Download the pinned release (checksum checked; never committed).
caddy:
	sh deploy/fetch-caddy.sh $(CADDY_PINNED)

# deploy/Caddyfile in front of a real server, synthetic files only. Fails, not
# skips, without the binary (OQ-90).
caddy-check: build caddy
	cd $(SERVER_DIR) && WX_CADDY='$(abspath $(CADDY))' WX_WEB_BUILD='$(CURDIR)/$(BUILD_DIR)/web' \
		go test -tags caddycheck -count=1 -run '^TestCaddy' ./cmd/workout-explorer

# ---- self-hoster image (F-31 DP-16, DP-17) -----------------------------------

DOCKER_IMAGE := workout-explorer:ci
COMPOSE_CI   := docker compose -f deploy/compose.yaml -f deploy/compose.build.yaml

# Pins agree, both Compose forms validate, the image builds with no cache and holds
# both binaries and the pinned Caddy. Fails, not skips, without Docker (OQ-90). Publishes nothing.
docker-check:
	@base=$$(grep -o 'static-debian13:nonroot@sha256:[0-9a-f]*' deploy/compose.yaml | head -1); \
	[ -n "$$base" ] && [ "$$(grep -c "^FROM gcr.io/distroless/$$base" deploy/Dockerfile)" = 1 ] || { echo "docker-check: base digest differs between deploy/compose.yaml and deploy/Dockerfile"; exit 1; }; \
	gov=$$(sed -n 's/^toolchain go//p' $(SERVER_DIR)/go.mod); \
	grep -q "^FROM golang:$$gov-" deploy/Dockerfile || { echo "docker-check: Dockerfile Go builder is not $$gov (go.mod toolchain)"; exit 1; }; \
	grep -q "^FROM node:$$(cut -d. -f1 $(WEB_DIR)/.nvmrc)-" deploy/Dockerfile || { echo "docker-check: Dockerfile Node builder differs from $(WEB_DIR)/.nvmrc"; exit 1; }; \
	made=; [ -e deploy/.env ] || { cp deploy/example.env deploy/.env; made=1; }; \
	trap '[ -z "$$made" ] || rm -f deploy/.env; docker rm -f wx-ci >/dev/null 2>&1; docker rmi -f $(DOCKER_IMAGE) >/dev/null 2>&1' EXIT; \
	docker compose -f deploy/compose.yaml config -q && $(COMPOSE_CI) config -q && $(COMPOSE_CI) -f deploy/compose.notify.yaml config -q || exit 1; \
	docker build --no-cache -f deploy/Dockerfile -t $(DOCKER_IMAGE) . || exit 1; \
	ctx=$$(mktemp -d); mkdir -p $$ctx/src/web $$ctx/src/server/x/testdata; touch $$ctx/src/web/P.FIT $$ctx/src/server/p.Bin $$ctx/src/server/p.br $$ctx/src/web/.env.local $$ctx/src/server/x/testdata/t; \
	printf 'FROM scratch\nCOPY . /c\n' > $$ctx/Dockerfile; cp deploy/Dockerfile.dockerignore $$ctx/Dockerfile.dockerignore; \
	leak=$$(docker build -q -f $$ctx/Dockerfile -t wx-ci-ctx $$ctx >/dev/null 2>&1 && docker create --name wx-ci-ctx wx-ci-ctx x >/dev/null 2>&1 && docker export wx-ci-ctx | tar -t | grep -ciE 'p\.fit|p\.bin|p\.br|\.env|testdata'); \
	docker rm -f wx-ci-ctx >/dev/null 2>&1; docker rmi -f wx-ci-ctx >/dev/null 2>&1; rm -rf $$ctx; \
	[ "$$leak" = 0 ] || { echo "docker-check: forbidden files reach the build context"; exit 1; }; \
	docker create --entrypoint /srv/workout-explorer/bin/caddy --name wx-ci $(DOCKER_IMAGE) >/dev/null && \
	docker cp wx-ci:/srv/workout-explorer/bin - | tar -t | grep -q 'bin/workout-explorer$$' && \
	docker run --rm --entrypoint /srv/workout-explorer/bin/caddy $(DOCKER_IMAGE) version | grep -q "^v$(CADDY_VERSION)" && \
	docker cp wx-ci:/srv/workout-explorer/web/index.html - >/dev/null && echo "docker-check: ok" || { echo "docker-check: image content wrong"; exit 1; }

# ---- parser corpus (ADR-004) ------------------------------------------------

CORPUS_DIR := $(SERVER_DIR)/testdata/corpus

# Parse every corpus file, diff the reports against expected/ and print the
# data file sizes (F-03).
corpus: build-server
	sh $(SERVER_DIR)/scripts/corpus.sh $(BINARY) $(CORPUS_DIR) $(BUILD_DIR)/corpus

# Rewrite every golden report and golden data file (and remove stale ones);
# review the diff of expected/ and testdata/datafile/ (ADR-004, F-03 design).
corpus-update:
	cd $(SERVER_DIR) && go test ./internal/ingest -run '^TestCorpus$$' -update && go test ./internal/datafile -run '^TestDataFileCorpus$$' -update

# Regenerate the synthetic corpus files; must give identical bytes.
corpus-gen:
	cd $(SERVER_DIR) && go run ./tools/corpusgen -out testdata/corpus/synthetic

# ---- prototype (F-07) -------------------------------------------------------

PROTO_DATA  := $(BUILD_DIR)/prototype/data
PROTO_ADDR  ?= :18080
PROTO_FLAGS ?=
REAL        ?=

# Data files for the prototype page: synthetic ride-12h and ride-4h, plus REAL=<fit files> on request.
prototype-data: build-server
	sh $(SERVER_DIR)/scripts/prototype-data.sh $(BINARY) $(CORPUS_DIR) $(PROTO_DATA) $(REAL)

# Serve the sample pages (testdata/page executed with build/web/index.html), build/web assets and the prototype data with brotli (dev only). PROTO_FLAGS=-tls for HTTPS.
prototype-serve: build-web
	cd $(SERVER_DIR) && go run ./tools/protoserve -addr '$(PROTO_ADDR)' -web '$(CURDIR)/$(BUILD_DIR)/web' -data '$(CURDIR)/$(PROTO_DATA)' -samples '$(CURDIR)/$(SERVER_DIR)/testdata/page' $(PROTO_FLAGS)

# ---- local development ----------------------------------------------------

dev-server:
	cd $(SERVER_DIR) && WORKOUT_EXPLORER_BASE_URL='$(DEV_BASE_URL)' go run ./cmd/workout-explorer serve -addr '$(DEV_ADDR)' -data '$(CURDIR)/$(BUILD_DIR)/data' -web '$(CURDIR)/$(BUILD_DIR)/web'

dev-web: deps-web
	cd $(WEB_DIR) && DEV_ADDR='$(DEV_ADDR)' npm run dev

clean:
	rm -rf $(BUILD_DIR)
