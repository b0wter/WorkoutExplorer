#!/bin/sh
# Deploy a GitHub Release (F-31 DP-10, ADR-035 rev. 2). Run by the operator from
# a clean checkout at the release tag; CI never connects to the server.
#   deploy/deploy.sh vX.Y.Z user@host   (needs git, gh, ssh, scp, tar, curl, sha256sum, sha512sum; linux/amd64 only)
#   deploy/deploy.sh vX.Y.Z local        (on the server itself: same steps, no ssh/scp)
# DEPLOY_DIR on the server defaults to /srv/workout-explorer; the SSH user owns it
# and is in the docker group (RUNBOOK.md). After the health wait it runs
# `regenerate` in the app container, then removes old bundles (F-30 RG-10, RG-11).
# No rollback (ADR-019). Exit 0: healthy and regenerate finished (whatever it
# deleted or kept); 1: refused or not healthy; 3: regenerate did not finish
# (nothing removed; run deploy.sh again with the same tag); 4: the clean-up failed.
set -eu
tag=${1:?usage: deploy/deploy.sh vX.Y.Z user@host}
host=${2:?usage: deploy/deploy.sh vX.Y.Z user@host}
dir=${DEPLOY_DIR:-/srv/workout-explorer}
here=$(cd "$(dirname "$0")" && pwd)
cd "$here/.."

# 1. Tag form, clean tree, HEAD at the tag, the release's assets.
printf '%s' "$tag" | grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+$' || { echo "deploy: '$tag' is not a vX.Y.Z tag" >&2; exit 1; }
[ -z "$(git status --porcelain)" ] || { echo "deploy: the working tree is not clean" >&2; exit 1; }
sha=$(git rev-parse HEAD)
[ "$(git rev-parse "$tag^{commit}" 2>/dev/null)" = "$sha" ] || { echo "deploy: HEAD is not the commit of $tag (git checkout $tag)" >&2; exit 1; }
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
gh release download "$tag" --dir "$tmp/dl" --pattern "workout-explorer-$tag-linux-amd64.tar.gz" --pattern SHA256SUMS
(cd "$tmp/dl" && sha256sum -c SHA256SUMS >/dev/null) || { echo "deploy: checksum of the release assets does not match" >&2; exit 1; }
mkdir "$tmp/a"
tar -C "$tmp/a" -xzf "$tmp/dl/workout-explorer-$tag-linux-amd64.tar.gz"
[ "$(cat "$tmp/a/COMMIT")" = "$sha" ] || { echo "deploy: the release was built from another commit than HEAD (tag moved?)" >&2; exit 1; }
sh deploy/fetch-caddy.sh "$tmp/caddy" # the pinned release, checksums from deploy/caddy.env
"$tmp/caddy" validate --config deploy/Caddyfile --adapter caddyfile >/dev/null 2>&1 || { echo "deploy: deploy/Caddyfile is invalid" >&2; exit 1; }

# 2. Copy. Every file goes to a temporary name and is renamed; web/assets only adds.
if [ "$host" = local ]; then
	r() { sh -c "cd '$dir' || exit 1; $1"; }
	cp_to() { cp "$1" "$dir/$2"; }
	tar_to() { tar -C "$dir/web/assets" --no-same-owner -xf -; }
else
	r() { ssh "$host" "cd '$dir' || exit 1; $1"; }
	cp_to() { scp -q "$1" "$host:$dir/$2"; }
	tar_to() { ssh "$host" "tar -C '$dir/web/assets' --no-same-owner -xf -"; }
fi
put() { # put <local> <remote relative path> <mode>
	tn=$(dirname "$2")/.$(basename "$2").new
	cp_to "$1" "$tn"
	r "chmod $3 '$tn' && mv '$tn' '$2'"
}
# Bundle lists (RG-10), before anything is copied: the files already in web/assets
# when the first list is written form one list of their own (releases before F-30).
r '[ -d bundles ] || { rm -rf bundles.new && mkdir bundles.new && ls -A web/assets > bundles.new/pre-f30 && mv bundles.new bundles; }'
(cd "$tmp/a/web/assets" && ls -A) > "$tmp/bundle"
put "$tmp/bundle" "bundles/$tag" 644
put deploy/clean-assets.sh clean-assets.sh 644
caddy_new=$(sha256sum "$tmp/caddy" | cut -d' ' -f1)
caddy_old=$(r "sha256sum bin/caddy 2>/dev/null | cut -d' ' -f1" || true)
conf_new=$(sha256sum deploy/Caddyfile | cut -d' ' -f1)
conf_old=$(r "sha256sum conf/Caddyfile 2>/dev/null | cut -d' ' -f1" || true)
tar -C "$tmp/a/web/assets" -cf - . | tar_to
put "$tmp/a/web/index.html" web/index.html 644 # the page template Go reads at publish
r "mkdir -p operator" # the empty mount source (compose never creates it, DP-3); nothing is ever written inside
put "$tmp/a/web/about.html" web/about.html 644 # the About page Go fills at start (F-29 AB-4); never touches operator/
put "$tmp/a/workout-explorer" bin/workout-explorer 755
[ "$caddy_new" = "$caddy_old" ] || put "$tmp/caddy" bin/caddy 755
put deploy/compose.yaml compose.yaml 644
put deploy/compose.notify.yaml compose.notify.yaml 644 # used only when .env's COMPOSE_FILE names it (F-27)
[ "$conf_new" = "$conf_old" ] || put deploy/Caddyfile conf/Caddyfile 644

# 3. Start what is missing or changed, restart app, wait for the new version.
[ "$caddy_new" = "$caddy_old" ] || r "docker compose up -d --force-recreate caddy" # new Caddy binary: brief page outage (accepted)
r "docker compose up -d"
r "docker compose restart app"
# 4. Caddyfile only: a graceful reload (a recreated Caddy has read it already).
[ "$conf_new" = "$conf_old" ] || [ "$caddy_new" != "$caddy_old" ] || r "docker compose exec -T caddy /srv/workout-explorer/bin/caddy reload --config /srv/workout-explorer/conf/Caddyfile --adapter caddyfile"
# The HTML entries Caddy serves, after the restart (they name the new assets, which are already there).
for f in upload.html edit.html generic.html; do put "$tmp/a/web/$f" "web/$f" 644; done

want=$tag
up=no
for i in $(seq 1 30); do
	got=$(r 'l=$(sed -n "s/^WX_LISTEN=//p" .env); curl -fsS --max-time 3 "http://${l:-127.0.0.1:8088}/api/health"' 2>/dev/null || true)
	case $got in *'"version":"'"$want"'"'*) echo "deploy: $want is up"; up=yes; break ;; esac
	sleep 2
done
if [ "$up" != yes ]; then
	echo "deploy: no healthy $want after 60 s; service state and last log lines (app, caddy):" >&2
	r "docker compose ps; docker compose logs --tail 30 app; docker compose logs --tail 15 caddy" >&2 || true
	exit 1
fi

# 5. Regenerate every live workout with the new version (F-30): its log lines go
# to stderr, the report (key=value lines) to stdout. Then the old bundles.
echo "deploy: regenerating workouts"
if report=$(r 'd=$(sed -n "s/^WX_DATA=//p" .env); w=$(sed -n "s/^WX_WEB=//p" .env); docker compose exec -T app /srv/workout-explorer/bin/workout-explorer regenerate -data "${d:-/srv/workouts}" -web "${w:-/srv/workout-explorer/web}"'); then rc=0; else rc=$?; fi
count() { printf '%s\n' "$report" | sed -n "s/^$1=//p"; }
summary="rebuilt=$(count rebuilt) deleted=$(count deleted) kept=$(count kept) skipped=$(count skipped)"
if [ "$rc" != 0 ]; then
	echo "deploy: $want is live, but regenerate did not finish (exit $rc; $summary); nothing removed. Run deploy.sh $want again." >&2
	exit 3
fi
if ! clean=$(printf '%s\n' "$report" | sed -n 's/^asset=//p' | r "sh clean-assets.sh '$tag'"); then
	echo "deploy: $want is live and regenerated ($summary), but removing old bundles failed" >&2
	exit 4
fi
echo "deploy: $want done: $summary $(printf '%s' "$clean" | tr '\n' ' ')"
