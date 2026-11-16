#!/bin/sh
# Deploy a GitHub Release on the Compose path (F-31 DP-10, F-59 IP-10, ADR-035, ADR-052). Run by
# the operator from a clean checkout at the release tag; CI never connects to the server.
#   deploy/deploy.sh vX.Y.Z user@host   (needs git, gh, ssh, scp, tar, sha256sum)
#   deploy/deploy.sh vX.Y.Z local        (on the server itself: same steps, no ssh/scp)
# DEPLOY_DIR on the server defaults to /srv/workout-explorer; the SSH user owns it and is in the
# docker group (DOCKER.md). It installs the release's deploy archive (compose.yaml names our image
# by digest, Caddy by minor), pulls both images, starts them, waits for the health check, then
# runs `regenerate`, which also removes the bundles no page needs (F-30, ADR-053).
# No rollback (ADR-019). Exit 0: healthy and regenerated (whatever it deleted or kept); 1: refused
# or not healthy; 3: regenerate did not finish (nothing removed; run deploy.sh again with the same
# tag); 4: the clean-up failed.
set -eu
tag=${1:?usage: deploy/deploy.sh vX.Y.Z user@host}
host=${2:?usage: deploy/deploy.sh vX.Y.Z user@host}
dir=${DEPLOY_DIR:-/srv/workout-explorer}
here=$(cd "$(dirname "$0")" && pwd)
cd "$here/.."

# 1. Tag form, clean tree, HEAD at the tag, the release's deploy archive.
printf '%s' "$tag" | grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+$' || { echo "deploy: '$tag' is not a vX.Y.Z tag" >&2; exit 1; }
[ -z "$(git status --porcelain)" ] || { echo "deploy: the working tree is not clean" >&2; exit 1; }
sha=$(git rev-parse HEAD)
[ "$(git rev-parse "$tag^{commit}" 2>/dev/null)" = "$sha" ] || { echo "deploy: HEAD is not the commit of $tag (git checkout $tag)" >&2; exit 1; }
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
archive=workout-explorer-$tag-deploy.tar.gz
gh release download "$tag" --dir "$tmp/dl" --pattern "$archive" --pattern SHA256SUMS
grep -q "  $archive\$" "$tmp/dl/SHA256SUMS" && (cd "$tmp/dl" && sha256sum -c --ignore-missing SHA256SUMS >/dev/null) ||
	{ echo "deploy: checksum of the deploy archive does not match" >&2; exit 1; }
mkdir "$tmp/a"
tar -C "$tmp/a" -xzf "$tmp/dl/$archive"
[ "$(cat "$tmp/a/COMMIT")" = "$sha" ] || { echo "deploy: the release was built from another commit than HEAD (tag moved?)" >&2; exit 1; }

if [ "$host" = local ]; then
	r() { sh -c "cd '$dir' || exit 1; $1"; }
	cp_to() { cp "$1" "$dir/$2"; }
else
	r() { ssh "$host" "cd '$dir' || exit 1; $1"; }
	cp_to() { scp -q "$1" "$host:$dir/$2"; }
fi
put() { # put <local> <remote relative path>: temporary name, then rename
	tn=$(dirname "$2")/.$(basename "$2").new
	cp_to "$1" "$tn"
	r "chmod 644 '$tn' && mv '$tn' '$2'"
}

# 2. Nothing on the server changes until both images are pulled with the new compose.yaml and the
# pulled Caddy accepts the new Caddyfile (F-59 IP-10).
# {data}/site must exist (ADR-043; Caddy's mount has create_host_path: false).
r 'd=$(sed -n "s/^WX_DATA=//p" .env); [ -d "${d:-/srv/workouts}/site" ]' || { echo "deploy: {data}/site is missing on the target; create it first (deploy/DOCKER.md section 3)" >&2; exit 1; }
! r 'grep -q "^WX_WEB=" .env' || { echo "deploy: .env still sets WX_WEB: the target is on the old tarball layout; migrate it first (deploy/DOCKER.md section 12)" >&2; exit 1; }
r "rm -rf .deploy-new && mkdir -p .deploy-new/conf"
for f in compose.yaml compose.notify.yaml; do cp_to "$tmp/a/$f" ".deploy-new/$f"; done
cp_to "$tmp/a/Caddyfile" .deploy-new/conf/Caddyfile
# The new file set, with .env's COMPOSE_FILE overrides (compose.notify.yaml) taken from .deploy-new.
new='f=; for c in $(sed -n "s/^COMPOSE_FILE=//p" .env | tr : " "); do f="$f -f .deploy-new/$c"; done; docker compose --project-directory . ${f:--f .deploy-new/compose.yaml}'
caddy_image=$(sed -n 's/^ *image: *\(docker\.io\/library\/caddy:[0-9.]*\) *$/\1/p' "$tmp/a/compose.yaml")
caddy_prev=$(r "docker image inspect -f '{{.Id}}' '$caddy_image' 2>/dev/null" || true)
r "$new pull -q" || { echo "deploy: pulling the images failed; nothing changed" >&2; exit 1; }
# The Caddy guard (F-59 IP-7): a refused patch gives the tag back to the previous image, so no later `up -d` starts it.
r "docker run --rm --network none --read-only --user 10002:10002 --cap-drop ALL --security-opt no-new-privileges:true --tmpfs /config --tmpfs /data -v \"\$PWD/.deploy-new/conf:/c:ro\" -e WX_DATA=/srv/workouts '$caddy_image' caddy validate --config /c/Caddyfile --adapter caddyfile >/dev/null 2>&1" ||
	{ [ -z "$caddy_prev" ] || r "docker tag '$caddy_prev' '$caddy_image'"; echo "deploy: $caddy_image does not run hardened or rejects the release's Caddyfile; nothing changed" >&2; exit 1; }

# 3. Install, then start what changed: app on its new image; caddy only when a new patch arrived
# or its definition changed (brief page outage, accepted). A Caddyfile-only change is a reload.
conf_new=$(sha256sum "$tmp/a/Caddyfile" | cut -d' ' -f1)
conf_old=$(r "sha256sum conf/Caddyfile 2>/dev/null | cut -d' ' -f1" || true)
caddy_old=$(r "docker compose ps -q caddy 2>/dev/null" || true)
r "mkdir -p operator" # the empty mount source (compose never creates it, DP-3); nothing is ever written inside
put "$tmp/a/compose.yaml" compose.yaml
put "$tmp/a/compose.notify.yaml" compose.notify.yaml # used only when .env's COMPOSE_FILE names it (F-27)
[ "$conf_new" = "$conf_old" ] || put "$tmp/a/Caddyfile" conf/Caddyfile
r "rm -rf .deploy-new"
r "docker compose up -d"
caddy_now=$(r "docker compose ps -q caddy" || true)
[ "$conf_new" = "$conf_old" ] || [ "$caddy_now" != "$caddy_old" ] || r "docker compose exec -T caddy caddy reload --config /srv/workout-explorer/conf/Caddyfile --adapter caddyfile"

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

# 4. Regenerate every live workout with the new version (F-30) and remove the bundles no served
# page needs (ADR-053): its log lines go to stderr, the report (key=value lines) to stdout.
echo "deploy: regenerating workouts"
if report=$(r 'd=$(sed -n "s/^WX_DATA=//p" .env); docker compose exec -T app /srv/workout-explorer/bin/workout-explorer regenerate -data "${d:-/srv/workouts}" -web /srv/workout-explorer/web'); then rc=0; else rc=$?; fi
count() { printf '%s\n' "$report" | sed -n "s/^$1=//p"; }
summary="rebuilt=$(count rebuilt) deleted=$(count deleted) kept=$(count kept) skipped=$(count skipped)"
case $rc in
0) echo "deploy: $want done: $summary files_removed=$(count files_removed) bundles_removed=$(count bundles_removed)" ;;
4)
	echo "deploy: $want is live and regenerated ($summary), but removing old bundles failed" >&2
	exit 4
	;;
*)
	echo "deploy: $want is live, but regenerate did not finish (exit $rc; $summary); nothing removed. Run deploy.sh $want again." >&2
	exit 3
	;;
esac
