#!/bin/sh
# Unattended full run of the UI suite (all projects) on a clean snapshot of main, with a dated report outside the repo.
# usage: report.sh [ref]            ref defaults to main
# env:   UI_REPORT_DIR   where reports go (default ${XDG_STATE_HOME:-$HOME/.local/state}/workout-explorer/ui-reports)
#        UI_REPORT_KEEP  how many reports to keep (default 14)
#        UI_REPORT_REPO  the repository to snapshot (default: the one holding this script)
# exit:  0 all passed, 1 test failures, 2 setup error (Docker or git missing, build failed, no result file)
# The snapshot is a temporary detached worktree, removed afterwards; the repository's own build/ and the dev ports
# 18080/18081 are never touched (make ui-test builds in the snapshot and picks free ports).
set -u

ref=${1:-main}
here=$(cd "$(dirname "$0")" && pwd)
repo=${UI_REPORT_REPO:-$(cd "$here/../../.." && pwd)}
reports=${UI_REPORT_DIR:-${XDG_STATE_HOME:-$HOME/.local/state}/workout-explorer/ui-reports}
keep=${UI_REPORT_KEEP:-14}

mkdir -p "$reports" || exit 2
for tool in git docker make node; do
	command -v "$tool" >/dev/null 2>&1 || { echo "ui-test-report: $tool not found in PATH" >&2; exit 2; }
done
docker info >/dev/null 2>&1 || { echo "ui-test-report: Docker is not reachable for this user" >&2; exit 2; }

# One run at a time: mkdir is atomic; a lock whose process is gone is stale.
lock=$reports/.lock
if ! mkdir "$lock" 2>/dev/null; then
	if kill -0 "$(cat "$lock/pid" 2>/dev/null)" 2>/dev/null; then
		echo "ui-test-report: another run is active (pid $(cat "$lock/pid"))" >&2
		exit 2
	fi
	rm -rf "$lock"
	mkdir "$lock" || exit 2
fi
echo $$ >"$lock/pid"

stamp=$(date +%Y-%m-%d_%H%M)
dest=$reports/$stamp
snap=$(mktemp -d "${TMPDIR:-/tmp}/wx-ui-snapshot.XXXXXX")
cleanup() {
	git -C "$repo" worktree remove --force "$snap" >/dev/null 2>&1 || rm -rf "$snap"
	git -C "$repo" worktree prune >/dev/null 2>&1
	rm -rf "$lock"
}
trap cleanup EXIT INT TERM

mkdir -p "$dest" || exit 2
rmdir "$snap" # git worktree add wants to create the directory itself
if ! git -C "$repo" worktree add --detach "$snap" "$ref" >"$dest/setup.log" 2>&1; then
	echo "ui-test-report: cannot snapshot $ref, see $dest/setup.log" >&2
	exit 2
fi
commit=$(git -C "$snap" rev-parse --short=12 HEAD)
start=$(date '+%Y-%m-%d %H:%M:%S')
t0=$(date +%s)

# The Makefile's ui-test target does the build, the servers and the Playwright run (all projects).
(cd "$snap" && make ui-test) >"$dest/run.log" 2>&1
rc=$?
t1=$(date +%s)
end=$(date '+%Y-%m-%d %H:%M:%S')

out=$snap/build/ui-test
if [ ! -f "$out/report.json" ]; then
	echo "setup error: no result file (build or server start failed); see run.log" >"$dest/SUMMARY.md"
	echo "ui-test-report: setup error, see $dest/run.log" >&2
	exit 2
fi
[ -d "$out/html" ] && cp -R "$out/html" "$dest/html"
[ -d "$out/results" ] && cp -R "$out/results" "$dest/artifacts"
cp "$out/report.json" "$dest/report.json"
node "$here/summary.mjs" "$dest/report.json" "$commit" "$ref" "$start" "$end" "$((t1 - t0))" "$dest" >"$dest/SUMMARY.md"
summary_rc=$?

# Keep the newest $keep dated directories.
ls -1d "$reports"/[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]_[0-9][0-9][0-9][0-9] 2>/dev/null | sort -r | tail -n +"$((keep + 1))" | while read -r old; do rm -rf "$old"; done

echo "ui-test-report: $dest ($(head -n 1 "$dest/SUMMARY.md"))"
[ "$summary_rc" -eq 0 ] && [ "$rc" -eq 0 ] && exit 0
exit 1
