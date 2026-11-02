# Self-hoster image (F-31 DP-16, ADR-034 rev. 3): both services run it, with the
# same files at the same paths as the operator's mounts. Build context: the
# repository root, filtered by Dockerfile.dockerignore. Nothing is published.
# Platform: linux/amd64 only (the Caddy pin). Builder versions equal go.mod's
# toolchain and src/web/.nvmrc, the final base equals compose.yaml's; `make docker-check` enforces both.

FROM node:24-bookworm@sha256:e7add2a490bb17d97b485b8641bd88455fff7839ade0f5eba493c3508124d413 AS web
WORKDIR /src
COPY Makefile ./
COPY deploy/caddy.env deploy/
COPY src/web src/web
RUN make build-web

FROM golang:1.26.8-bookworm@sha256:dc9ad6c05acc7a88e5b71bde60a5fe3bd4b9f0db209011711b464107438a8107 AS go
ARG VERSION=unknown
WORKDIR /src
COPY Makefile ./
COPY deploy/caddy.env deploy/fetch-caddy.sh deploy/
COPY src/server src/server
RUN make VERSION="$VERSION" build-server && sh deploy/fetch-caddy.sh build/caddy
COPY --from=web /src/build/web build/web
COPY deploy/Caddyfile conf/Caddyfile
RUN chmod -R a+rX build conf && chmod 755 build/workout-explorer build/caddy

# gcr.io/distroless/static-debian13:nonroot, the digest of compose.yaml.
FROM gcr.io/distroless/static-debian13:nonroot@sha256:25ac8f0c3b2334a9eb05452266bd4697864a8c8ef0ef71e5e6480d7f92868329
COPY --from=go /src/build/workout-explorer /src/build/caddy /srv/workout-explorer/bin/
COPY --from=go /src/build/web /srv/workout-explorer/web
COPY --from=go /src/conf /srv/workout-explorer/conf
