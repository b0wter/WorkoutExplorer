#!/bin/sh
# Deploy the release this file came with, on Docker Compose or rootless Podman (F-52 AU-16,
# F-59 IP-10, IP-11; ADR-050, ADR-052, ADR-054). Not an entry point: update.sh runs it from the
# verified, unpacked deploy archive, under its lock, as the path's deploy user:
#   update.sh vX.Y.Z    (by hand)    update.sh    (nightly, when automatic updates are on)
# It installs into DEPLOY_DIR (default /srv/workout-explorer). Path: Compose when the deploy
# directory holds compose.yaml, Podman when the service user's Quadlet directory holds our app
# unit. Nothing changes until both images are pulled and the pulled Caddy accepts the new
# Caddyfile under our hardening (F-59 IP-7). Then: install, start, wait for the health check,
# `regenerate` (F-30), which also removes the bundles no page needs (ADR-053).
# No rollback (ADR-019). Exit 0: healthy and regenerated (whatever it deleted or kept); 1: refused
# or not healthy; 3: regenerate did not finish (nothing removed; run update.sh with the same tag
# again); 4: the clean-up failed.
set -eu
tag=${1:?usage: update.sh vX.Y.Z (deploy.sh is run by update.sh)}
[ "${WX_DEPLOY_FROM_UPDATE:-}" = 1 ] || { echo "deploy: run update.sh $tag, which verifies the release first" >&2; exit 1; }
printf '%s' "$tag" | grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+$' || { echo "deploy: '$tag' is not a vX.Y.Z tag" >&2; exit 1; }
dir=${DEPLOY_DIR:-/srv/workout-explorer}
a=$(cd "$(dirname "$0")" && pwd) # the unpacked archive
cd "$dir"
envval() { sed -n "s/^$1=//p" .env | tail -n 1; }
quadlet=$HOME/.config/containers/systemd
if [ -e compose.yaml ] && [ ! -e "$quadlet/workout-explorer-app.container" ]; then
	path=compose
elif [ ! -e compose.yaml ] && [ -e "$quadlet/workout-explorer-app.container" ]; then
	path=podman
else
	echo "deploy: found both or neither of $dir/compose.yaml and $quadlet/workout-explorer-app.container" >&2
	exit 1
fi
# No check of the data directory here: it is 0750 and owned by the containers' user, so the deploy
# user cannot see into it (QA-52-r1-1). A missing site/ fails visibly when Caddy starts (bind mounts
# are never created).
d=$(envval WX_DATA) # Compose only; Podman's units fix the data directory
! grep -q "^WX_WEB=" .env || { echo "deploy: .env still sets WX_WEB: the host is on the old tarball layout; migrate it first (DOCKER.md section 12)" >&2; exit 1; }

# put <file> <deploy dir relative path> <mode>: temporary name, then rename (AU-15: a running
# update.sh keeps reading its old copy).
put() {
	tn=$(dirname "$2")/.$(basename "$2").new
	cp "$1" "$tn" && chmod "$3" "$tn" && mv "$tn" "$2"
}

if [ "$path" = podman ]; then
	# IP-11 through the archive's control script: pull, guard, install units, Caddyfile and itself.
	ctl="$a/podman/workout-explorer-ctl"
	sh "$ctl" install "$a" || exit 1
	put "$a/update.sh" update.sh 755
	sh "$ctl" restart || { echo "deploy: restarting the units failed" >&2; exit 1; }
	health=127.0.0.1:8080
else
	# 1. Nothing changes until both images are pulled with the new compose.yaml and the pulled
	# Caddy accepts the new Caddyfile (F-59 IP-10).
	rm -rf .deploy-new && mkdir -p .deploy-new/conf
	cp "$a/compose.yaml" "$a/compose.notify.yaml" .deploy-new/
	cp "$a/Caddyfile" .deploy-new/conf/Caddyfile
	# The new file set, with .env's COMPOSE_FILE overrides (compose.notify.yaml) taken from .deploy-new.
	f=
	for c in $(envval COMPOSE_FILE | tr : ' '); do f="$f -f .deploy-new/$c"; done
	caddy_image=$(sed -n 's/^ *image: *\(docker\.io\/library\/caddy:[0-9.]*\) *$/\1/p' "$a/compose.yaml")
	caddy_prev=$(docker image inspect -f '{{.Id}}' "$caddy_image" 2>/dev/null || true)
	# shellcheck disable=SC2086 # $f is a list of options
	docker compose --project-directory . ${f:--f .deploy-new/compose.yaml} pull -q || { echo "deploy: pulling the images failed; nothing changed" >&2; exit 1; }
	# The Caddy guard (F-59 IP-7): a refused patch gives the tag back to the previous image, so no later `up -d` starts it.
	docker run --rm --network none --read-only --user 10002:10002 --cap-drop ALL --security-opt no-new-privileges:true --tmpfs /config --tmpfs /data \
		-v "$PWD/.deploy-new/conf:/c:ro" -e WX_DATA=/srv/workouts "$caddy_image" caddy validate --config /c/Caddyfile --adapter caddyfile >/dev/null 2>&1 ||
		{ [ -z "$caddy_prev" ] || docker tag "$caddy_prev" "$caddy_image"; echo "deploy: $caddy_image does not run hardened or rejects the release's Caddyfile; nothing changed" >&2; exit 1; }

	# 2. Install, then start what changed: app on its new image; caddy only when a new patch arrived
	# or its definition changed (brief page outage, accepted). A Caddyfile-only change is a reload.
	conf_new=$(sha256sum "$a/Caddyfile" | cut -d' ' -f1)
	conf_old=$(sha256sum conf/Caddyfile 2>/dev/null | cut -d' ' -f1 || true)
	caddy_old=$(docker compose ps -q caddy 2>/dev/null || true)
	mkdir -p operator # the empty mount source (compose never creates it, DP-3); nothing is ever written inside
	put "$a/compose.yaml" compose.yaml 644
	put "$a/compose.notify.yaml" compose.notify.yaml 644 # used only when .env's COMPOSE_FILE names it (F-27)
	[ "$conf_new" = "$conf_old" ] || put "$a/Caddyfile" conf/Caddyfile 644
	put "$a/update.sh" update.sh 755
	rm -rf .deploy-new
	docker compose up -d
	caddy_now=$(docker compose ps -q caddy || true)
	[ "$conf_new" = "$conf_old" ] || [ "$caddy_now" != "$caddy_old" ] || docker compose exec -T caddy caddy reload --config /srv/workout-explorer/conf/Caddyfile --adapter caddyfile
	l=$(envval WX_LISTEN)
	health=${l:-127.0.0.1:8088}
fi

# The updater units of both paths, as the source for installing the timer later (DOCKER.md section
# 8, PODMAN.md section 9); the installed units are never changed by a release (AU-13).
mkdir -p updater/system updater/user
for f in "$a"/updater/system/* "$a"/updater/user/*; do put "$f" "updater/${f#"$a/updater/"}" 644; done

# 3. Health: the new tag answers through Caddy.
up=no
for i in $(seq 1 30); do
	got=$(curl -q --noproxy '*' -fsS --max-time 3 "http://$health/api/health" 2>/dev/null || true)
	case $got in *'"version":"'"$tag"'"'*) echo "deploy: $tag is up"; up=yes; break ;; esac
	sleep 2
done
if [ "$up" != yes ]; then
	echo "deploy: no healthy $tag after 60 s; service state and last log lines (app, caddy):" >&2
	if [ "$path" = podman ]; then
		sh "$a/podman/workout-explorer-ctl" status >&2 || true
	else
		{ docker compose ps; docker compose logs --tail 30 app; docker compose logs --tail 15 caddy; } >&2 || true
	fi
	exit 1
fi

# 4. Regenerate every live workout with the new version (F-30) and remove the bundles no served
# page needs (ADR-053): its log lines go to stderr, the report (key=value lines) to stdout.
echo "deploy: regenerating workouts"
if [ "$path" = podman ]; then
	if report=$(sh "$a/podman/workout-explorer-ctl" regenerate); then rc=0; else rc=$?; fi
else
	if report=$(docker compose exec -T app /srv/workout-explorer/bin/workout-explorer regenerate -data "${d:-/srv/workouts}" -web /srv/workout-explorer/web); then rc=0; else rc=$?; fi
fi
count() { printf '%s\n' "$report" | sed -n "s/^$1=//p"; }
summary="rebuilt=$(count rebuilt) deleted=$(count deleted) kept=$(count kept) skipped=$(count skipped)"
case $rc in
0) echo "deploy: $tag done: $summary files_removed=$(count files_removed) bundles_removed=$(count bundles_removed)" ;;
4)
	echo "deploy: $tag is live and regenerated ($summary), but removing old bundles failed" >&2
	exit 4
	;;
*)
	echo "deploy: $tag is live, but regenerate did not finish (exit $rc; $summary); nothing removed. Run update.sh $tag again." >&2
	exit 3
	;;
esac
