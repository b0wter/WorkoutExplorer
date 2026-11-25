#!/bin/sh
# make ui-test driver: builds into build/ui-test/ (never build/web or build/data), starts its own Go server
# and Caddy on free ports, runs the Playwright suite in the matching Docker image, stops both.
# usage: run.sh <caddy binary> [playwright args, e.g. --project=webkit -g "F-73"]
set -eu

root=$(cd "$(dirname "$0")/../../.." && pwd)
caddy=$1
shift
out=$root/build/ui-test
image=mcr.microsoft.com/playwright:v1.49.1-noble # keep equal to @playwright/test in package.json

free_port() { node -e 'const s=require("net").createServer().listen(0,"127.0.0.1",()=>{console.log(s.address().port);s.close()})'; }

rm -rf "$out/data" "$out/results"
mkdir -p "$out/data" "$out/results" "$out/fit"

echo "ui-test: build"
(cd "$root/src/web" && npx vite build --outDir ../../build/ui-test/web --emptyOutDir >/dev/null && node scripts/bundled-licenses.mjs ../../build/ui-test/web/.vite/license.json)
(cd "$root/src/server" && CGO_ENABLED=0 go build -o "$out/workout-explorer" ./cmd/workout-explorer &&
	go run ./tools/attributions -web "$out/web" -binary "$out/workout-explorer" -compose "$root/deploy/compose.yaml" -caddyfile "$root/deploy/Caddyfile" &&
	go run ./tools/brotliassets "$out/web/assets" &&
	go run ./tools/corpusgen -e2e "$out/fit")
(cd "$root/src/web/e2e" && npm ci --no-audit --no-fund >/dev/null)

api_port=$(free_port)
site_port=$(free_port)
base=http://127.0.0.1:$site_port
pids=""
cleanup() { for p in $pids; do kill "$p" 2>/dev/null || true; done; wait 2>/dev/null || true; }
trap cleanup EXIT INT TERM

WORKOUT_EXPLORER_BASE_URL=$base WORKOUT_EXPLORER_UPLOADS_PER_HOUR_IPV4=240 \
	"$out/workout-explorer" serve -addr "127.0.0.1:$api_port" -data "$out/data" -web "$out/web" >"$out/server.log" 2>&1 &
pids="$pids $!"
WX_SITE=$base WX_BIND=127.0.0.1 WX_API=127.0.0.1:$api_port WX_DATA=$out/data \
	"$caddy" run --config "$root/deploy/Caddyfile" --adapter caddyfile >"$out/caddy.log" 2>&1 &
pids="$pids $!"

i=0
until curl -sf "$base/api/status" >/dev/null 2>&1; do
	i=$((i + 1))
	if [ "$i" -gt 100 ]; then echo "ui-test: server did not start, see $out/server.log and $out/caddy.log"; exit 1; fi
	sleep 0.2
done
echo "ui-test: server on $base"

docker run --rm --network host --ipc=host -u "$(id -u):$(id -g)" -e HOME=/tmp -e WX_BASE="$base" \
	-v "$root:/repo" -w /repo/src/web/e2e "$image" npx playwright test "$@"
