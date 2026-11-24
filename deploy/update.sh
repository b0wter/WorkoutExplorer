#!/bin/sh
# Workout Explorer updater: the one entry for every deploy on Docker Compose, rootless Podman and
# bare metal (F-52 AU-1 to AU-16, F-60 BM-1 to BM-12, INF-8; ADR-050, ADR-051, ADR-054, ADR-055).
# Lives at {deploy dir}/update.sh and runs as the path's deploy user (Compose: owner of the deploy
# directory, docker group; Podman: the service user; bare metal: the user "deploy"), never as root.
#   update.sh           scheduled run: acts only with WORKOUT_EXPLORER_AUTO_UPDATE=on, installs the
#                       latest release if it is newer than the running one
#   update.sh vX.Y.Z    manual deploy of that tag, whatever the switch, the stopped state or the
#                       running version; exit 0 clears the stopped state
# It verifies the release's SHA256SUMS with minisign against WX_UPDATE_PUBLIC_KEY (.env; bare
# metal: /etc/workout-explorer/.env) before
# anything is unpacked or run, then runs the verified deploy archive's deploy.sh under one lock.
# Last line of every run (AU-14):
#   update: running=<v|-> latest=<v|-> action=<none|applied|refused|failed> reason=<class> exit=<n>
# Exit: 0 nothing to do or applied; 1 refused, busy (manual), network (manual); 2 configuration or
# usage; 128 + signal when killed; otherwise deploy.sh's status (1 refused or not healthy,
# 3 regenerate unfinished, 4 clean-up, 5 bare metal: root must install changed files first).
# DEPLOY_DIR (default /srv/workout-explorer) is the deploy directory. WX_UPDATE_RELEASES,
# WX_UPDATE_PUSHOVER, WX_UPDATE_NOTIFY_DRYRUN (check the notification settings, send nothing) and
# WX_HOST_ROOT (a prefix for /etc, /usr) replace the release host, the Pushover endpoint, the
# alert and the host's root for tests only; none weakens the check, which trusts only
# the pinned key.
set -u
dir=${DEPLOY_DIR:-/srv/workout-explorer}
releases=${WX_UPDATE_RELEASES:-https://github.com/b0wter/WorkoutExplorer/releases}
pushover=${WX_UPDATE_PUSHOVER:-https://api.pushover.net/1/messages.json}
semver='^v[0-9]{1,9}\.[0-9]{1,9}\.[0-9]{1,9}$' # 9 digits: safe in any shell's arithmetic
manual=
running=-
latest=-
work=

say() { echo "update: $*" >&2; }
# done <action> <reason> <exit>: the run's one log line (fixed words only, AU-14), then exit.
done_() {
	[ -z "$work" ] || rm -rf "$work"
	echo "update: running=$running latest=$latest action=$1 reason=$2 exit=$3"
	exit "$3"
}
# envval <key>: the last value of <key> in .env, read as data (never sourced, AU-12).
envval() { sed -n "s/^$1=//p" "$envf" | tail -n 1; }

case $# in
0) ;;
1)
	manual=$1
	printf '%s' "$manual" | grep -Eq "$semver" || { say "'$manual' is not a vX.Y.Z tag"; done_ none config 2; }
	latest=$manual
	;;
*) say "usage: update.sh [vX.Y.Z]"; done_ none config 2 ;;
esac

# 1. Configuration, before any request (AU-12, AU-17).
[ "$(id -u)" != 0 ] || { say "never run as root; run as the deploy user"; done_ none config 2; }
for t in curl tar sha256sum flock minisign sed grep; do
	command -v "$t" >/dev/null 2>&1 || { say "$t is not installed"; done_ none config 2; }
done
# The install path (AU-16, BM-1), before the first .env read: exactly one of Compose, our Podman
# app unit and our bare-metal app unit. Bare metal keeps root's .env and operator/ in
# /etc/workout-explorer, outside the deploy user's directory (BM-2, BM-10).
compose=no podman=no bare=no
[ ! -e "$dir/compose.yaml" ] || compose=yes
[ ! -e "$HOME/.config/containers/systemd/workout-explorer-app.container" ] || podman=yes
[ ! -e "${WX_HOST_ROOT:-}/etc/systemd/system/workout-explorer.service" ] || bare=yes
envf=$dir/.env
case $compose$podman$bare in
yesnono) ;;
noyesno) health=http://127.0.0.1:8080/api/health ;; # the Caddy unit's PublishPort
nonoyes)
	envf=${WX_HOST_ROOT:-}/etc/workout-explorer/.env
	for f in .env operator; do
		[ ! -e "$dir/$f" ] && [ ! -L "$dir/$f" ] || { say "$dir/$f must not be in the program directory; move it to /etc/workout-explorer (BAREMETAL.md section 7)"; done_ none config 2; }
	done
	command -v systemctl >/dev/null 2>&1 || { say "systemctl is not installed"; done_ none config 2; }
	# Caddy's address from its unit's effective environment, so a drop-in that moves it is followed (BM-1).
	cenv=$(systemctl show -p Environment workout-explorer-caddy.service 2>/dev/null)
	unitenv() { printf ' %s\n' "${cenv#Environment=}" | sed -n "s/.* \"$1=\([^\"]*\)\".*/\1/p; t; s/.* $1=\([^ ]*\).*/\1/p"; }
	b=$(unitenv WX_BIND) p=$(unitenv WX_SITE)
	b=${b%% *} p=${p##*:}
	case $b in "" | 0.0.0.0) b=127.0.0.1 ;; ::) b='[::1]' ;; *:*) b="[$b]" ;; esac
	case $p in "") p=8088 ;; *[!0-9]*) p=80 ;; esac # unset: the Caddyfile's default
	health=http://$b:$p/api/health
	;;
*) say "found $(printf '%s' "$compose$podman$bare" | grep -o yes | wc -l | tr -d ' ') of $dir/compose.yaml, our Podman app unit and /etc/systemd/system/workout-explorer.service; need exactly one"; done_ none config 2 ;;
esac
[ -r "$envf" ] || { say "$envf is missing or unreadable"; done_ none config 2; }
switch=$(envval WORKOUT_EXPLORER_AUTO_UPDATE)
case $switch in "" | on) ;; *) say "WORKOUT_EXPLORER_AUTO_UPDATE must be unset or on"; done_ none config 2 ;; esac
key=$(envval WX_UPDATE_PUBLIC_KEY)
printf '%s' "$key" | grep -Eqx 'RW[A-Za-z0-9+/]{54}' || { say "WX_UPDATE_PUBLIC_KEY is missing or not a minisign public key"; done_ none config 2; }
[ "$compose" = no ] || { l=$(envval WX_LISTEN); health=http://${l:-127.0.0.1:8088}/api/health; }
[ -n "$manual" ] || [ "$switch" = on ] || done_ none off 0

# 2. One deploy at a time (AU-10): the kernel drops the lock when this process ends.
{ : >>"$dir/deploy.lock"; } 2>/dev/null || { say "cannot write $dir/deploy.lock"; done_ none config 2; }
exec 9>>"$dir/deploy.lock"
if ! flock -n 9; then
	[ -z "$manual" ] || { say "another deploy is running; try again later"; done_ none busy 1; }
	done_ none busy 0
fi
state=$dir/update.stopped
if [ -z "$manual" ] && [ -e "$state" ]; then done_ none stopped 0; fi

# fetch <url> <file> <max bytes>: HTTPS only (redirects too), no .curlrc, no credentials,
# time-limited, size-capped. Succeeds on a complete 200 within the cap; sets $code to the HTTP status.
fetch() {
	code=$(curl -q --proto =https --proto-redir =https -sSL --max-redirs 5 --connect-timeout 15 --max-time 120 \
		--max-filesize "$3" -o "$2" -w '%{http_code}' "$1" 2>/dev/null) && [ "$code" = 200 ] && [ "$(wc -c <"$2")" -le "$3" ]
}
# stop_ <action> <reason> <exit>: set the stopped state, alert once, log (AU-8, AU-9, AU-11).
stop_() {
	now=$(date -u +%Y-%m-%dT%H:%M:%SZ)
	printf 'time=%s tag=%s action=%s exit=%s reason=%s\n' "$now" "$latest" "$1" "$3" "$2" >"$state.new" && mv "$state.new" "$state" ||
		say "cannot write $state"
	[ -z "$manual" ] || [ "$switch" = on ] || say "set $state: if you turn automatic updates on, they stay stopped until a manual deploy succeeds"
	if [ -z "$manual" ] || [ "$switch" = on ]; then
		[ -z "$manual" ] || say "automatic updates are now stopped until a manual deploy succeeds"
		case $1:$2 in refused:*) what="refused ($2)" ;; *:root) what="stopped before any change (exit 5): root must install changed system files" ;; *) what="failed (exit $3)" ;; esac
		alert "$latest $what. Automatic updates are stopped until a manual deploy succeeds."
	fi
	done_ "$@"
}
# notify_url_ok <kind> <url>: F-70 NS-3, the set internal/notify accepts (deploy/test/notify-url-cases.tsv
# is run by both test suites): only RFC 3986 characters, at most 2048, no fragment or userinfo, https,
# or http only to a literal loopback or private IPv4 address, [::1] or [fc../[fd.. address; ntfy needs a topic path.
notify_url_ok() {
	u=$2
	[ "${#u}" -le 2048 ] || return 1
	printf '%s' "$u" | LC_ALL=C grep -Eqx "[]A-Za-z0-9._~:/?@!\$&'()*+,;=%[-]+" || return 1
	case $u in https://*) plain= ;; http://*) plain=1 ;; *) return 1 ;; esac
	rest=${u#*://}
	auth=${rest%%[/?]*}
	path=${rest#"$auth"}
	path=${path%%\?*}
	printf '%s' "$auth" | grep -Eqx '([A-Za-z0-9.-]+|\[[0-9A-Fa-f:.]+\])(:[0-9]{1,5})?' || return 1
	port=$(printf '%s' "$auth" | sed -n 's/.*:\([0-9]\{1,5\}\)$/\1/p')
	[ -z "$port" ] || { [ "$port" -ge 1 ] && [ "$port" -le 65535 ]; } || return 1
	[ "$1" != ntfy ] || { [ -n "$path" ] && [ "$path" != / ]; } || return 1
	[ -n "$plain" ] || return 0
	host=$(printf '%s' "$auth" | sed 's/:[0-9]\{1,5\}$//')
	o='(0|[1-9][0-9]?|1[0-9][0-9]|2[0-4][0-9]|25[0-5])'
	printf '%s' "$host" | grep -Eqx "(127\.$o\.$o\.$o|10\.$o\.$o\.$o|192\.168\.$o\.$o|172\.(1[6-9]|2[0-9]|3[01])\.$o\.$o|\[::1\]|\[[Ff][CcDd][0-9A-Fa-f]{2}:[0-9A-Fa-f:.]*\])"
}
# alert <text>: one message through the one service set in .env (F-70 NS-11, AU-11), else the log line
# is the alert. The settings are read as data. The secrets (Pushover keys, the ntfy or callback URL and
# token) go to curl on stdin, never on a command line. <text> holds fixed words and a validated tag only.
alert() {
	pt=$(envval WORKOUT_EXPLORER_PUSHOVER_TOKEN) pu=$(envval WORKOUT_EXPLORER_PUSHOVER_USER)
	nk=$(envval WORKOUT_EXPLORER_NOTIFY_KIND) nu=$(envval WORKOUT_EXPLORER_NOTIFY_URL) nt=$(envval WORKOUT_EXPLORER_NOTIFY_TOKEN)
	if [ -z "$nk$nu$nt" ]; then
		printf '%s' "$pt$pu" | grep -Eqx '[A-Za-z0-9]{60}' || return 0
		m=$(printf '%s' "$1" | sed 's/%/%25/g; s/ /%20/g; s/(/%28/g; s/)/%29/g')
		printf 'token=%s&user=%s&title=Workout%%20Explorer&message=%s' "$pt" "$pu" "$m" |
			curl -q --proto =https -fsS --max-time 10 -o /dev/null --data-binary @- "$pushover" 2>/dev/null ||
			say "the Pushover alert could not be sent"
		return 0
	fi
	if [ -n "$pt$pu" ] || { [ "$nk" != ntfy ] && [ "$nk" != callback ]; } || [ -z "$nu" ] || ! notify_url_ok "$nk" "$nu" ||
		{ [ -n "$nt" ] && { [ "${#nt}" -gt 256 ] || ! printf '%s' "$nt" | grep -Eqx '[A-Za-z0-9._~+/-]+=*'; }; }; then
		say "notification settings invalid; no alert"
		return 0
	fi
	[ -z "${WX_UPDATE_NOTIFY_DRYRUN:-}" ] || { say "notification settings valid"; return 0; } # tests: validate only
	case $nu in http://*) proto=http ;; *) proto=https ;; esac
	if [ "$nk" = ntfy ]; then
		set -- -H 'Title: Workout Explorer' --data-raw "$1"
	else
		set -- -H 'Content-Type: application/json' --data-raw "{\"alert\":\"$1\"}"
	fi
	# Any 2xx is success; a redirect is not followed and counts as a failure.
	code=$({
		printf 'url = "%s"\n' "$nu"
		[ -z "$nt" ] || printf 'header = "Authorization: Bearer %s"\n' "$nt"
	} | curl -q -sS --max-time 10 -o /dev/null -w '%{http_code}' --noproxy '*' --proto "=$proto" -K - "$@" 2>/dev/null)
	case $code in 2??) ;; *) say "the $nk alert could not be sent" ;; esac
}

# A run killed by systemd's timeout, a hang-up or Ctrl-C still ends with its log line; once
# deploy.sh has started, as a failed deploy (stopped state, alert). The shell runs the trap when
# the foreground command (curl, deploy.sh), killed with it, has ended.
applying=
killed() {
	trap '' HUP INT TERM
	[ -z "$applying" ] || stop_ failed deploy "$1"
	done_ none killed "$1"
}
trap 'killed 129' HUP
trap 'killed 130' INT
trap 'killed 143' TERM

# 3. Which tag (AU-6): scheduled runs only, against the running version.
if [ -z "$manual" ]; then
	got=$(curl -q --noproxy '*' -fsS --max-time 5 "$health" 2>/dev/null) || done_ none health 0
	running=$(printf '%s' "$got" | sed -n 's/.*"version":"\([^"]*\)".*/\1/p')
	printf '%s' "$running" | grep -Eq "$semver" || { running=-; done_ none health 0; }
	# GitHub's "latest release" redirect names the tag (drafts and pre-releases are never latest).
	to=$(curl -q --proto =https -sS --connect-timeout 15 --max-time 30 -o /dev/null -w '%{http_code} %{redirect_url}' "$releases/latest" 2>/dev/null) || done_ none network 0
	case $to in 30[1278]\ https://*) ;; *) done_ none network 0 ;; esac
	latest=${to##*/}
	printf '%s' "$latest" | grep -Eq "$semver" || { latest=-; done_ none current 0; }
	newer() { # newer <a> <b>: a > b, numeric per component
		set -- $(printf '%s.%s' "${1#v}" "${2#v}" | tr . ' ')
		[ "$1" -gt "$4" ] || { [ "$1" -eq "$4" ] && { [ "$2" -gt "$5" ] || { [ "$2" -eq "$5" ] && [ "$3" -gt "$6" ]; }; }; }
	}
	newer "$latest" "$running" || done_ none current 0
fi
tag=$latest

# 4. Verify (AU-7). Nothing is unpacked or run before the last check, and nothing is fetched
# again or read from outside $work after the signature check.
work=$(mktemp -d) || { say "cannot create a work directory"; done_ none config 2; }
dl=$releases/download/$tag
fetch "$dl/SHA256SUMS" "$work/SHA256SUMS" 65536 || { [ -n "$manual" ] && done_ none network 1; done_ none network 0; }
fetch "$dl/SHA256SUMS.minisig" "$work/SHA256SUMS.minisig" 4096 || {
	[ "$code" != 404 ] || stop_ refused signature 1 # a published release without its signature
	[ -n "$manual" ] && done_ none network 1
	done_ none network 0
}
minisign -V -q -P "$key" -m "$work/SHA256SUMS" -x "$work/SHA256SUMS.minisig" >/dev/null 2>&1 || stop_ refused signature 1
# The tag is in the signed text through the names (AU-4): every line names this tag, and both archives are listed.
archive=workout-explorer-$tag-deploy.tar.gz
esc=$(printf '%s' "$tag" | sed 's/\./\\./g')
grep -Evq "^[0-9a-f]{64}  workout-explorer-$esc-[A-Za-z0-9._-]+\$" "$work/SHA256SUMS" && stop_ refused manifest 1
[ "$(grep -c "  workout-explorer-$esc-deploy\.tar\.gz\$" "$work/SHA256SUMS")" = 1 ] &&
	[ "$(grep -c "  workout-explorer-$esc-linux-amd64\.tar\.gz\$" "$work/SHA256SUMS")" = 1 ] || stop_ refused manifest 1
# get <name> <file> <max bytes>: fetch a file the verified manifest names and check its SHA-256.
get() {
	want=$(sed -n "s/^\([0-9a-f]\{64\}\)  $(printf '%s' "$1" | sed 's/\./\\./g')\$/\1/p" "$work/SHA256SUMS")
	fetch "$dl/$1" "$2" "$3" || { [ -n "$manual" ] && done_ none network 1; done_ none network 0; }
	[ "$(sha256sum "$2" | cut -d' ' -f1)" = "$want" ] || stop_ refused checksum 1
}
# unpack <archive> <empty dir>: members regular files and directories only, relative names of plain
# characters without ".." (-P lists names as stored, a leading / kept); unpacked without owners.
unpack() {
	list=$(tar -tPzf "$1" 2>/dev/null) && types=$(tar -tvPzf "$1" 2>/dev/null | cut -c1) || stop_ refused archive 1
	[ -n "$list" ] && [ "$(printf '%s\n' "$list" | wc -l)" = "$(printf '%s\n' "$types" | wc -l)" ] || stop_ refused archive 1
	printf '%s\n' "$types" | grep -qv '^[-d]$' && stop_ refused archive 1
	printf '%s\n' "$list" | grep -Evq '^(\./)?([A-Za-z0-9._-]+/)*[A-Za-z0-9._-]*$' && stop_ refused archive 1
	printf '%s\n' "$list" | grep -Eq '(^|/)\.\.(/|$)' && stop_ refused archive 1
	mkdir "$2" && tar -xzf "$1" -C "$2" --no-same-owner --no-same-permissions 2>/dev/null || stop_ refused archive 1
	[ -z "$(find "$2" ! -type f ! -type d)" ] || stop_ refused archive 1
}
get "$archive" "$work/deploy.tar.gz" 10485760
unpack "$work/deploy.tar.gz" "$work/a"
[ -f "$work/a/deploy.sh" ] || stop_ refused archive 1
# Bare metal: a release whose units read .env from the deploy user's directory (v0.9.1 and older) is
# never applied, so exit 5 never asks root to install them (BM-10, QA-60-r2-1).
[ "$bare" = no ] || grep -qx 'EnvironmentFile=/etc/workout-explorer/.env' "$work/a/systemd/workout-explorer.service" 2>/dev/null ||
	{ say "$tag predates the /etc/workout-explorer layout; bare metal needs a newer release"; done_ none config 2; }
# Bare metal: the program files come from the release tarball, checked the same way (BM-7).
# Container paths never fetch it (AU-3).
if [ "$bare" = yes ]; then
	get "workout-explorer-$tag-linux-amd64.tar.gz" "$work/app.tar.gz" 209715200
	unpack "$work/app.tar.gz" "$work/t"
	[ -f "$work/t/workout-explorer" ] && [ -f "$work/t/web/index.html" ] || stop_ refused archive 1
	export WX_DEPLOY_APP="$work/t"
fi

# 5. Apply: the verified archive's deploy.sh (AU-16); its lines come before ours.
applying=yes
WX_DEPLOY_FROM_UPDATE=1 DEPLOY_DIR=$dir sh "$work/a/deploy.sh" "$tag" </dev/null 9>&- # the lock stays with this process
rc=$?
[ "$rc" != 5 ] || stop_ failed root 5 # bare metal: root-only files differ, nothing changed (BM-5)
[ "$rc" = 0 ] || stop_ failed deploy "$rc"
rm -f "$state"
done_ applied - 0
