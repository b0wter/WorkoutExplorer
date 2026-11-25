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
        build build-server build-web attributions precompress-assets \
        size ci ui-test caddy caddy-check image deploy-archive docker-check update-check corpus corpus-update corpus-gen prototype-data prototype-serve dev-server dev-web clean

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
	$(MAKE) attributions precompress-assets

build-server:
	cd $(SERVER_DIR) && CGO_ENABLED=0 go build \
		-ldflags "-X main.version=$(VERSION)" \
		-o ../../$(BINARY) ./cmd/workout-explorer

build-web: deps-web
	cd $(WEB_DIR) && npm run build

# The attribution list into build/web/about.html (F-29 AB-5, AB-6): run after build-server and
# build-web (`make build` does; the Dockerfile's Go stage calls it alone). Reads the web bundle's
# licence file and the binary's build info; fails on a missing licence or an unclaimed CSP origin.
attributions:
	cd $(SERVER_DIR) && go run ./tools/attributions -web '$(CURDIR)/$(BUILD_DIR)/web' -binary '$(CURDIR)/$(BINARY)' \
		-compose '$(CURDIR)/deploy/compose.yaml' -caddyfile '$(CURDIR)/deploy/Caddyfile'

# .br sidecars next to the bundle's text files (F-22 H-7); Caddy serves them with `precompressed br`.
# After build-web; the Dockerfile's Go stage calls it alone.
precompress-assets:
	cd $(SERVER_DIR) && go run ./tools/brotliassets '$(CURDIR)/$(BUILD_DIR)/web/assets'

# Bundle budget check (ADR-002). Depends only on build-web, never on test,
# so the size is printed even when a test fails under `make -k ci`.
size: build-web
	cd $(WEB_DIR) && npm run size

# ---- CI -------------------------------------------------------------------

ci: lint test build size caddy-check docker-check update-check

# ---- browser UI suite (not part of ci: about 15 minutes) ---------------------

# Playwright (Chromium, WebKit, Firefox) against its own build and server in build/ui-test/; see src/web/e2e/README.md.
# PROJECT=<name> runs one project (chromium, webkit, firefox, iPhone, Android, iPad, firefox-phone); UI_ARGS are further Playwright arguments, e.g. UI_ARGS='-g F-73'.
ui-test: deps-web caddy
	sh $(WEB_DIR)/e2e/run.sh '$(abspath $(CADDY_PINNED))' $(if $(PROJECT),--project=$(PROJECT)) $(UI_ARGS)

# ---- Caddy (F-22 H-7, H-12; F-59 IP-6, IP-19) ---------------------------------

# The minor's one home: compose.yaml's tag of the official image (patches float).
CADDY_IMAGE  = $(shell sed -n 's/^ *image: *\(docker\.io\/library\/caddy:[0-9]*\.[0-9]*\) *$$/\1/p' deploy/compose.yaml)
CADDY_MINOR  = $(lastword $(subst :, ,$(CADDY_IMAGE)))
CADDY_PINNED = $(BUILD_DIR)/tools/caddy-$(CADDY_MINOR)/caddy
# CADDY=<binary> runs the check against another Caddy (for example to see 2.10.2 fail).
CADDY ?= $(CADDY_PINNED)

# The binary of the current patch of the pinned minor, from the official image (never committed).
caddy:
	@[ -n "$(CADDY_MINOR)" ] || { echo "caddy: no docker.io/library/caddy:<minor> image in deploy/compose.yaml"; exit 1; }
	docker pull -q --platform linux/amd64 $(CADDY_IMAGE) >/dev/null
	mkdir -p $(dir $(CADDY_PINNED))
	id=$$(docker create --platform linux/amd64 $(CADDY_IMAGE)) && { docker cp "$$id:/usr/bin/caddy" $(CADDY_PINNED).new; rc=$$?; docker rm "$$id" >/dev/null; [ $$rc = 0 ]; } && mv $(CADDY_PINNED).new $(CADDY_PINNED)
	@echo "caddy: $(CADDY_IMAGE) is $$($(CADDY_PINNED) version | cut -d' ' -f1)"

# deploy/Caddyfile in front of a real server, synthetic files only. Fails, not
# skips, without the binary (OQ-90).
caddy-check: build caddy
	cd $(SERVER_DIR) && WX_CADDY='$(abspath $(CADDY))' WX_WEB_BUILD='$(CURDIR)/$(BUILD_DIR)/web' \
		go test -tags caddycheck -count=1 -run '^TestCaddy' ./cmd/workout-explorer

# ---- app image and deploy files (F-59 IP-1 to IP-5, IP-12; ADR-052) -----------

IMAGE_TAG      := workout-explorer:ci
IMAGE_ARCHIVE  := $(BUILD_DIR)/image.oci.tar
DEPLOY_ARCHIVE := $(BUILD_DIR)/deploy.tar.gz
REVISION ?= $(shell git rev-parse HEAD 2>/dev/null || echo unknown)
# The release's value for our image in compose.yaml and the Podman app unit; make ci uses a test digest.
APP_IMAGE ?= ghcr.io/test/workout-explorer:v0.0.0@sha256:0000000000000000000000000000000000000000000000000000000000000000
# Every file the deploy archive may hold (IP-5, F-52 AU-3): never .env, the Dockerfile, compose.build.yaml,
# minisign.pub, sign-release.sh or the updater's tests.
DEPLOY_ALLOW := ^\./$$|^\./(compose\.yaml|compose\.notify\.yaml|Caddyfile|deploy\.sh|update\.sh|updater/|updater/(system|user)/|updater/(system|user)/workout-explorer-update\.(service|timer)|example\.env|operator\.example\.txt|impressum\.example\.txt|COMMIT|podman/|podman/workout-explorer-(app|caddy)\.container|podman/workout-explorer-(edge|backend|egress)\.network|podman/workout-explorer-ctl|systemd/|systemd/workout-explorer(|-caddy|-caddy-upgrade|-regenerate)\.service|systemd/50-workout-explorer\.rules|systemd/workout-explorer\.slice|systemd/workout-explorer-caddy-upgrade\.path|systemd/caddy\.pref|systemd/notify\.conf|systemd/workout-explorer-run)$$

# The image, built with no cache, saved as one OCI archive: the bytes every check below and the
# release job see (IP-3). One plain linux/amd64 manifest: no provenance or SBOM attestations.
image:
	docker build --no-cache --platform linux/amd64 --provenance=false --sbom=false \
		--build-arg VERSION='$(VERSION)' --build-arg REVISION='$(REVISION)' -f deploy/Dockerfile -t $(IMAGE_TAG) .
	mkdir -p $(BUILD_DIR) && docker save -o $(IMAGE_ARCHIVE) $(IMAGE_TAG) && docker rmi $(IMAGE_TAG) >/dev/null

# The deploy files of a release (IP-5) with APP_IMAGE written in; CI calls it with the tested digest.
# Modes are normalised so that CI's archive and sign-release.sh's rebuild match under any umask.
deploy-archive:
	rm -rf $(BUILD_DIR)/deploy && mkdir -p $(BUILD_DIR)/deploy
	cp deploy/compose.yaml deploy/compose.notify.yaml deploy/Caddyfile deploy/deploy.sh deploy/update.sh deploy/example.env \
		deploy/operator.example.txt deploy/impressum.example.txt $(BUILD_DIR)/deploy/
	cp -r deploy/podman deploy/systemd deploy/updater $(BUILD_DIR)/deploy/
	git rev-parse HEAD > $(BUILD_DIR)/deploy/COMMIT
	cd $(BUILD_DIR)/deploy && sed -i 's|ghcr\.io/[a-z0-9._-]*/workout-explorer:unreleased|$(APP_IMAGE)|' compose.yaml podman/workout-explorer-app.container && \
		[ "$$(grep -c 'image: $(APP_IMAGE)$$' compose.yaml)" = 1 ] && [ "$$(grep -c '^Image=$(APP_IMAGE)$$' podman/workout-explorer-app.container)" = 1 ] && \
		! grep -q ':unreleased' compose.yaml podman/workout-explorer-app.container || { echo "deploy-archive: our image is not written exactly once into compose.yaml and the Podman app unit"; exit 1; }
	tar -C $(BUILD_DIR)/deploy --sort=name --owner=0 --group=0 --numeric-owner --mode='u=rwX,go=rX' -czf $(DEPLOY_ARCHIVE) .

# Fails, not skips, without Docker (OQ-90). Publishes nothing.
docker-check: image deploy-archive
	@gov=$$(sed -n 's/^toolchain go//p' $(SERVER_DIR)/go.mod); \
	grep -q "^FROM golang:$$gov-" deploy/Dockerfile || { echo "docker-check: Dockerfile Go builder is not $$gov (go.mod toolchain)"; exit 1; }; \
	grep -q "^FROM node:$$(cut -d. -f1 $(WEB_DIR)/.nvmrc)-" deploy/Dockerfile || { echo "docker-check: Dockerfile Node builder differs from $(WEB_DIR)/.nvmrc"; exit 1; }
	@m='$(CADDY_MINOR)'; [ -n "$$m" ] && grep -qx "Image=docker.io/library/caddy:$$m" deploy/podman/workout-explorer-caddy.container && \
		grep -qx "Pin: version $$m.\*" deploy/systemd/caddy.pref || { echo "docker-check: the Caddy minor differs between compose.yaml, the Podman unit and the apt pin (IP-6)"; exit 1; }
	@# Forbidden files never reach the build context (F-31).
	@ctx=$$(mktemp -d); mkdir -p $$ctx/src/web $$ctx/src/server/x/testdata; touch $$ctx/src/web/P.FIT $$ctx/src/server/p.Bin $$ctx/src/server/p.br $$ctx/src/web/.env.local $$ctx/src/server/x/testdata/t; \
	printf 'FROM scratch\nCOPY . /c\n' > $$ctx/Dockerfile; cp deploy/Dockerfile.dockerignore $$ctx/Dockerfile.dockerignore; \
	leak=$$(docker build -q -f $$ctx/Dockerfile -t wx-ci-ctx $$ctx >/dev/null 2>&1 && docker create --name wx-ci-ctx wx-ci-ctx x >/dev/null 2>&1 && docker export wx-ci-ctx | tar -t | grep -ciE 'p\.fit|p\.bin|p\.br|\.env|testdata'); \
	docker rm -f wx-ci-ctx >/dev/null 2>&1; docker rmi -f wx-ci-ctx >/dev/null 2>&1; rm -rf $$ctx; \
	[ "$$leak" = 0 ] || { echo "docker-check: forbidden files reach the build context"; exit 1; }
	@# The deploy archive: only allow-listed files, scripts parse, every Compose file set validates (test digest).
	@bad=$$(tar -tzf $(DEPLOY_ARCHIVE) | grep -Ev '$(DEPLOY_ALLOW)'); [ -z "$$bad" ] || { echo "docker-check: not allowed in the deploy archive: $$bad"; exit 1; }
	@for f in deploy.sh update.sh podman/workout-explorer-ctl systemd/workout-explorer-run; do sh -n $(BUILD_DIR)/deploy/$$f || { echo "docker-check: $$f does not parse"; exit 1; }; done
	@d=$(BUILD_DIR)/deploy; cp deploy/example.env $$d/.env; made=; [ -e deploy/.env ] || { cp deploy/example.env deploy/.env; made=1; }; \
	docker compose -f $$d/compose.yaml config -q && docker compose -f $$d/compose.yaml -f $$d/compose.notify.yaml config -q && \
	docker compose -f deploy/compose.yaml -f deploy/compose.build.yaml config -q && \
	docker compose -f deploy/compose.yaml -f deploy/compose.build.yaml -f deploy/compose.notify.yaml config -q; rc=$$?; \
	rm -f $$d/.env; [ -z "$$made" ] || rm -f deploy/.env; [ $$rc = 0 ] || { echo "docker-check: a Compose file set does not validate"; exit 1; }
	@# The image archive: one plain manifest, then its file system, labels, user and base packages (IP-1, IP-2).
	@# index.json is the layout's own index; it must point at exactly one image manifest, no nested index or attestation.
	@i=$$(tar -xOf $(IMAGE_ARCHIVE) index.json); [ "$$(printf '%s' "$$i" | grep -o '"mediaType":"[^"]*"' | sort | uniq -c | tr -s ' ')" = \
		"$$(printf ' 1 "mediaType":"application/vnd.oci.image.index.v1+json"\n 1 "mediaType":"application/vnd.oci.image.manifest.v1+json"')" ] || \
		{ echo "docker-check: the archive is not one plain image manifest: $$i"; exit 1; }
	@t=$$(mktemp -d); trap 'docker rm -f wx-ci wx-ci-base >/dev/null 2>&1; docker rmi -f $(IMAGE_TAG) >/dev/null 2>&1; rm -rf $$t' EXIT; \
	docker load -q -i $(IMAGE_ARCHIVE) >/dev/null && \
	base=$$(sed -n 's/^FROM \(gcr\.io\/distroless\/[^ ]*\)$$/\1/p' deploy/Dockerfile) && [ -n "$$base" ] && \
	docker create --name wx-ci $(IMAGE_TAG) >/dev/null && docker create --platform linux/amd64 --name wx-ci-base "$$base" x >/dev/null && \
	docker export wx-ci | tar -t | sed 's|/$$||' | sort > $$t/img && docker export wx-ci-base | tar -t | sed 's|/$$||' | sort > $$t/base || { echo "docker-check: cannot load or list the image"; exit 1; }; \
	extra=$$(comm -13 $$t/base $$t/img | grep -Ev '^srv$$|^srv/workout-explorer$$|^srv/workout-explorer/LICENSE$$|^srv/workout-explorer/bin$$|^srv/workout-explorer/bin/workout-explorer$$|^srv/workout-explorer/web(/.*)?$$'); \
	[ -z "$$extra" ] || { echo "docker-check: the image holds files outside IP-1:"; echo "$$extra" | head -20; exit 1; }; \
	grep -qx 'srv/workout-explorer/web/index.html' $$t/img && grep -qx 'srv/workout-explorer/LICENSE' $$t/img || { echo "docker-check: the image lacks the web build or LICENSE"; exit 1; }; \
	for l in source version revision licenses title; do \
		[ -n "$$(docker inspect -f "{{index .Config.Labels \"org.opencontainers.image.$$l\"}}" $(IMAGE_TAG))" ] || { echo "docker-check: label $$l missing"; exit 1; }; done; \
	u=$$(docker inspect -f '{{.Config.User}}' $(IMAGE_TAG)); [ "$$u" = 10001:10001 ] || { echo "docker-check: the image's user is '$$u', not 10001:10001"; exit 1; }; \
	docker run --rm --network none --read-only --cap-drop ALL $(IMAGE_TAG) 2>&1 | grep -q '^usage: workout-explorer' || { echo "docker-check: the image's binary does not run"; exit 1; }; \
	sed -n 's|^var/lib/dpkg/status.d/\([^/.]*\)$$|\1|p' $$t/img > $$t/pkgs; \
	for p in $$(cat $$t/pkgs); do grep -qx "usr/share/doc/$$p/copyright" $$t/img || { echo "docker-check: no copyright file for $$p"; exit 1; }; done; \
	(cd $(SERVER_DIR) && go run ./tools/attributions -base-packages $$t/pkgs) || exit 1; \
	echo "docker-check: ok ($$(tar -xOf $(IMAGE_ARCHIVE) index.json | grep -o 'sha256:[0-9a-f]*' | head -1))"

# ---- updater (F-52) ------------------------------------------------------------

# deploy/update.sh (and the real deploy.sh) against a local fake release host, and deploy/sign-release.sh
# against a fake gh and a scratch repository, with throwaway keys (deploy/test/). Uses the distribution's
# minisign, else a test double (Python cryptography); fails, never skips, without both.
update-check:
	sh -n deploy/update.sh && sh -n deploy/sign-release.sh
	python3 -W ignore::ResourceWarning -I deploy/test/update_test.py
	python3 -W ignore::ResourceWarning -I deploy/test/sign_test.py

# ---- parser corpus (ADR-004) ------------------------------------------------

CORPUS_DIR := $(SERVER_DIR)/testdata/corpus

# Parse every corpus file, diff the reports against expected/ and print the
# data file sizes (F-03).
corpus: build-server
	sh $(SERVER_DIR)/scripts/corpus.sh $(BINARY) $(CORPUS_DIR) $(BUILD_DIR)/corpus

# Rewrite every golden report and golden data file (and remove stale ones);
# review the diff of expected/ and testdata/datafile/ (ADR-004, F-03 design).
corpus-update:
	cd $(SERVER_DIR) && go test ./internal/ingest -run '^TestCorpus$$' -update && go test ./internal/datafile -run '^(TestDataFileCorpus|TestAddedLapsSample|TestAutoLapsSample)$$' -update

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
