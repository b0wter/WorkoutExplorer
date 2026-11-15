# The app image (F-59 IP-1, ADR-052): the Go binary and the web build on the
# pinned distroless base, nothing else (no Caddy, Caddyfile, settings or data).
# Built by `make image` (CI: once per run, pushed unchanged on a vX.Y.Z tag) and
# by compose.build.yaml for self-builders. Build context: the repository root,
# filtered by Dockerfile.dockerignore. Platform: linux/amd64 only. Builder
# versions equal go.mod's toolchain and src/web/.nvmrc; `make docker-check` enforces both.

FROM node:24-bookworm@sha256:e7add2a490bb17d97b485b8641bd88455fff7839ade0f5eba493c3508124d413 AS web
WORKDIR /src
COPY Makefile ./
COPY src/web src/web
RUN make build-web

FROM golang:1.26.8-bookworm@sha256:dc9ad6c05acc7a88e5b71bde60a5fe3bd4b9f0db209011711b464107438a8107 AS go
ARG VERSION=unknown
WORKDIR /src
COPY Makefile ./
COPY src/server src/server
RUN make VERSION="$VERSION" build-server
COPY --from=web /src/build/web build/web
# The attribution list into about.html (F-29 AB-5): needs the web build, the binary, the module
# cache, the Caddyfile (CSP origins) and compose.yaml (the Caddy minor).
COPY deploy/Caddyfile deploy/compose.yaml deploy/
RUN make attributions precompress-assets && chmod -R a+rX build && chmod 755 build/workout-explorer

# gcr.io/distroless/static-debian13:nonroot (multi-arch index digest); a bump is a reviewed change
# and must keep tools/attributions/sources.json's package list (`make docker-check`).
FROM gcr.io/distroless/static-debian13:nonroot@sha256:25ac8f0c3b2334a9eb05452266bd4697864a8c8ef0ef71e5e6480d7f92868329
ARG VERSION=unknown
ARG REVISION=unknown
ARG SOURCE=https://github.com/b0wter/WorkoutExplorer
LABEL org.opencontainers.image.source=$SOURCE \
      org.opencontainers.image.version=$VERSION \
      org.opencontainers.image.revision=$REVISION \
      org.opencontainers.image.licenses=AGPL-3.0-only \
      org.opencontainers.image.title=workout-explorer
COPY --from=go /src/build/workout-explorer /srv/workout-explorer/bin/
COPY --from=go /src/build/web /srv/workout-explorer/web
COPY LICENSE /srv/workout-explorer/LICENSE
USER 10001:10001
ENTRYPOINT ["/srv/workout-explorer/bin/workout-explorer"]
