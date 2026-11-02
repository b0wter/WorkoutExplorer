#!/bin/sh
# Download the pinned Caddy release binary (deploy/caddy.env) to $1, checked
# against the pinned SHA-512; a no-op when $1 already has the
# pinned binary checksum. The
# binary is never committed (OQ-70).
set -eu
dest=$1
. "$(dirname "$0")/caddy.env"

if [ -x "$dest" ] && echo "$CADDY_BIN_SHA512_LINUX_AMD64  $dest" | sha512sum -c --quiet - 2>/dev/null; then
	exit 0
fi
# shortcut: linux/amd64 only (CI, the VPS); add a checksum line per platform when another is needed.
if [ "$(uname -s)/$(uname -m)" != "Linux/x86_64" ]; then
	echo "fetch-caddy: only linux/amd64 is pinned; put Caddy v$CADDY_VERSION at $dest yourself" >&2
	exit 1
fi
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
tgz=caddy_${CADDY_VERSION}_linux_amd64.tar.gz
curl -sSfL --retry 3 -o "$tmp/$tgz" "https://github.com/caddyserver/caddy/releases/download/v$CADDY_VERSION/$tgz"
echo "$CADDY_SHA512_LINUX_AMD64  $tmp/$tgz" | sha512sum -c --quiet -
tar -xzf "$tmp/$tgz" -C "$tmp" caddy
echo "$CADDY_BIN_SHA512_LINUX_AMD64  $tmp/caddy" | sha512sum -c --quiet -
mkdir -p "$(dirname "$dest")"
mv "$tmp/caddy" "$dest"
"$dest" version
