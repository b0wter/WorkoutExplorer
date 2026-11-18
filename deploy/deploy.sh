#!/bin/sh
# Deploy the release this file came with, on Docker Compose, rootless Podman or bare metal (F-52
# AU-16, F-59 IP-10, IP-11, F-60 BM-4 to BM-6; ADR-050, ADR-052, ADR-054, ADR-055). Not an entry
# point: update.sh runs it from the verified, unpacked deploy archive, under its lock, as the
# path's deploy user:
#   update.sh vX.Y.Z    (by hand)    update.sh    (nightly, when automatic updates are on)
# It installs into DEPLOY_DIR (default /srv/workout-explorer). Path: Compose when the deploy
# directory holds compose.yaml, Podman when the service user's Quadlet directory holds our app
# unit, bare metal when /etc/systemd/system holds our app unit (exactly one). Containers: nothing
# changes until both images are pulled and the pulled Caddy accepts the new Caddyfile under our
# hardening (F-59 IP-7). Bare metal: nothing changes unless every root-only file equals the
# release's and the installed Caddy accepts the new Caddyfile; the program files come from the
# verified release tarball update.sh unpacked (WX_DEPLOY_APP). Then: install, start, wait for the
# health check, `regenerate` (F-30), which also removes the bundles no page needs (ADR-053).
# No rollback (ADR-019). Exit 0: healthy and regenerated (whatever it deleted or kept); 1: refused
# or not healthy; 3: regenerate did not finish (nothing removed; run update.sh with the same tag
# again); 4: the clean-up failed; 5: bare metal, root must install changed system files first
# (nothing changed). WX_HOST_ROOT: a prefix for /etc and /usr, for tests only.
set -eu
tag=${1:?usage: update.sh vX.Y.Z (deploy.sh is run by update.sh)}
[ "${WX_DEPLOY_FROM_UPDATE:-}" = 1 ] || { echo "deploy: run update.sh $tag, which verifies the release first" >&2; exit 1; }
printf '%s' "$tag" | grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+$' || { echo "deploy: '$tag' is not a vX.Y.Z tag" >&2; exit 1; }
dir=${DEPLOY_DIR:-/srv/workout-explorer}
a=$(cd "$(dirname "$0")" && pwd) # the unpacked archive
cd "$dir"
envval() { sed -n "s/^$1=//p" .env | tail -n 1; }
quadlet=$HOME/.config/containers/systemd
r=${WX_HOST_ROOT:-}
units=$r/etc/systemd/system
path=
[ ! -e compose.yaml ] || path="$path compose"
[ ! -e "$quadlet/workout-explorer-app.container" ] || path="$path podman"
[ ! -e "$units/workout-explorer.service" ] || path="$path bare"
case $path in
" compose" | " podman" | " bare") path=${path# } ;;
*)
	echo "deploy: need exactly one of $dir/compose.yaml, $quadlet/workout-explorer-app.container and /etc/systemd/system/workout-explorer.service; found:${path:- none}" >&2
	exit 1
	;;
esac
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

if [ "$path" = bare ]; then
	app=${WX_DEPLOY_APP:-}
	[ -f "$app/workout-explorer" ] && [ -f "$app/web/index.html" ] || { echo "deploy: no verified release tarball (update.sh unpacks it)" >&2; exit 1; }
	# 1. Checks; nothing changes before they pass. Root-only files (BM-5): each must equal the
	# release's copy (a missing one differs); our units are never edited in place, only by drop-ins.
	differ=
	for f in "$a"/systemd/*.service "$a"/systemd/*.slice "$a"/systemd/*.path; do
		cmp -s "$f" "$units/${f##*/}" || differ="$differ /etc/systemd/system/${f##*/}"
	done
	n=workout-explorer.service.d/notify.conf
	[ ! -e "$units/$n" ] || cmp -s "$a/systemd/notify.conf" "$units/$n" || differ="$differ /etc/systemd/system/$n"
	cmp -s "$a/systemd/workout-explorer-run" "$r/usr/local/sbin/workout-explorer-run" || differ="$differ /usr/local/sbin/workout-explorer-run"
	cmp -s "$a/systemd/caddy.pref" "$r/etc/apt/preferences.d/workout-explorer-caddy.pref" || differ="$differ /etc/apt/preferences.d/workout-explorer-caddy.pref"
	# The installed Caddy's minor must be the pinned one (a pin installed without `apt install caddy`).
	pin=$(sed -n 's/^Pin: version \([0-9]*\.[0-9]*\)\.\*$/\1/p' "$a/systemd/caddy.pref")
	have=$("$r/usr/bin/caddy" version 2>/dev/null | sed -n 's/^v\([0-9]*\.[0-9]*\)\..*/\1/p') || true
	[ -n "$pin" ] && [ "$have" = "$pin" ] || differ="$differ /usr/bin/caddy(${have:-none},pin:$pin)"
	if [ -n "$differ" ]; then
		echo "deploy: nothing changed. As root, install these files from the release you verified by hand (BAREMETAL.md section 6):$differ" >&2
		exit 5
	fi
	# The new Caddyfile with the installed Caddy and its unit's values for every WX_ key the file
	# reads, as this user, with a throwaway home: no data directory, no admin socket (BM-6, T-3).
	cenv=$(systemctl show -p Environment workout-explorer-caddy.service 2>/dev/null) || true
	unitenv() { printf ' %s\n' "${cenv#Environment=}" | sed -n "s/.* \"$1=\([^\"]*\)\".*/\1/p; t; s/.* $1=\([^ ]*\).*/\1/p"; }
	h=$a/.caddy-home
	mkdir -p "$h"
	set -- PATH=/usr/bin:/bin HOME="$h" XDG_CONFIG_HOME="$h" XDG_DATA_HOME="$h"
	for k in $(grep -o '{\$WX_[A-Z_]*' "$a/Caddyfile" | sed 's/^{\$//' | sort -u); do set -- "$@" "$k=$(unitenv "$k")"; done
	env -i "$@" "$r/usr/bin/caddy" validate --config "$a/Caddyfile" --adapter caddyfile >/dev/null 2>&1 ||
		{ echo "deploy: the installed Caddy rejects the release's Caddyfile; nothing changed" >&2; exit 1; }

	# 2. Stage next to the targets (same file system); a killed run's leftovers go first.
	[ -d web ] || [ ! -d web.old ] || mv web.old web
	rm -rf bin/.workout-explorer.new web.new web.old conf/.Caddyfile.new
	mkdir -p bin conf
	{ cp "$app/workout-explorer" bin/.workout-explorer.new && chmod 755 bin/.workout-explorer.new &&
		cp -R "$app/web" web.new && chmod -R u=rwX,go=rX web.new && cp "$a/Caddyfile" conf/.Caddyfile.new && chmod 644 conf/.Caddyfile.new; } ||
		{ rm -rf bin/.workout-explorer.new web.new conf/.Caddyfile.new; echo "deploy: staging the new files failed; nothing changed" >&2; exit 1; }

	# 3. Swap by rename. The app reads web/ only at start, so the window before its restart is harmless.
	mv bin/.workout-explorer.new bin/workout-explorer
	[ ! -d web ] || mv web web.old
	mv web.new web
	caddy_restart=no
	if cmp -s conf/.Caddyfile.new conf/Caddyfile; then rm -f conf/.Caddyfile.new; else mv conf/.Caddyfile.new conf/Caddyfile; caddy_restart=yes; fi
	systemctl is-active -q workout-explorer-caddy.service || caddy_restart=yes
	put "$a/update.sh" update.sh 755

	# 4. Restart through the polkit rule (ADR-055); Caddy only for a new Caddyfile (a moment without pages).
	systemctl --no-ask-password restart workout-explorer.service || { echo "deploy: restarting workout-explorer.service failed" >&2; exit 1; }
	[ "$caddy_restart" = no ] || systemctl --no-ask-password restart workout-explorer-caddy.service ||
		{ echo "deploy: restarting workout-explorer-caddy.service failed" >&2; exit 1; }
	rm -rf web.old # no rollback (ADR-019); old pages keep their assets in {data}/site/assets/ (IP-8)
	b=$(unitenv WX_BIND) p=$(unitenv WX_SITE)
	b=${b%% *} p=${p##*:}
	case $b in "" | 0.0.0.0) b=127.0.0.1 ;; ::) b='[::1]' ;; *:*) b="[$b]" ;; esac
	case $p in "") p=8088 ;; *[!0-9]*) p=80 ;; esac # unset: the Caddyfile's default
	health=$b:$p
elif [ "$path" = podman ]; then
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
	elif [ "$path" = bare ]; then
		systemctl --no-pager status workout-explorer.service workout-explorer-caddy.service >&2 || true
	else
		{ docker compose ps; docker compose logs --tail 30 app; docker compose logs --tail 15 caddy; } >&2 || true
	fi
	exit 1
fi

# 4. Regenerate every live workout with the new version (F-30) and remove the bundles no served
# page needs (ADR-053): its log lines go to stderr, the report (key=value lines) to stdout.
echo "deploy: regenerating workouts"
if [ "$path" = bare ]; then
	# A fixed oneshot unit as the app user (BM-4, OQ-267); its report goes to that unit's journal,
	# which this user cannot read. A failed oneshot keeps its exit status until the next start (T-2).
	if systemctl --no-ask-password start workout-explorer-regenerate.service; then rc=0; else
		st=$(systemctl show -p ExecMainCode -p ExecMainStatus workout-explorer-regenerate.service 2>/dev/null || true)
		case $st in *ExecMainCode=1*) rc=$(printf '%s\n' "$st" | sed -n 's/^ExecMainStatus=//p') ;; *) rc=1 ;; esac
		[ "$rc" != 0 ] || rc=1 # start refused or the unit never ran
	fi
	report="see: sudo journalctl -u workout-explorer-regenerate -n 20"
elif [ "$path" = podman ]; then
	if report=$(sh "$a/podman/workout-explorer-ctl" regenerate); then rc=0; else rc=$?; fi
else
	if report=$(docker compose exec -T app /srv/workout-explorer/bin/workout-explorer regenerate -data "${d:-/srv/workouts}" -web /srv/workout-explorer/web); then rc=0; else rc=$?; fi
fi
count() { printf '%s\n' "$report" | sed -n "s/^$1=//p"; }
summary="rebuilt=$(count rebuilt) deleted=$(count deleted) kept=$(count kept) skipped=$(count skipped)"
[ "$path" != bare ] || summary=$report
case $rc in
0) [ "$path" = bare ] && echo "deploy: $tag done ($summary)" || echo "deploy: $tag done: $summary files_removed=$(count files_removed) bundles_removed=$(count bundles_removed)" ;;
4)
	echo "deploy: $tag is live and regenerated ($summary), but removing old bundles failed" >&2
	exit 4
	;;
*)
	echo "deploy: $tag is live, but regenerate did not finish (exit $rc; $summary); nothing removed. Run update.sh $tag again." >&2
	exit 3
	;;
esac
