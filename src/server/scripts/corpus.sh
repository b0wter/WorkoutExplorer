#!/bin/sh
# make corpus driver (ADR-004; F-02 design, "Build and CI changes"; F-03
# design, "CLI and make corpus").
#
# usage: corpus.sh <binary> <corpus dir> <output dir>
#
# Runs `<binary> parse -datafile` on every *.fit under <corpus dir>/real and
# <corpus dir>/synthetic (sorted), writes <output dir>/<set>/<name>.json,
# <name>.bin and <name>.bin.br, and compares the report with
# <corpus dir>/expected/<set>/<name>.json. Lines ok, DIFF and NEW end with
# "  bin <bytes> br <bytes>", the sizes of the data file and its brotli
# encoding (real-file sizes appear only here, never in committed files,
# ADR-004 item 7 rev. 3). `parse` exits 0 for an accepted file and 3 for a
# rejected one (F-08): the rejection report on stdout is compared with the
# golden file like a report, and the line ends in "rejected" instead of the
# sizes. Any other non-zero exit is FAIL. Prints per file:
#   ok     report equals the golden report
#   DIFF   report differs; followed by `diff -u expected actual`
#   FAIL   parse exited non-zero; followed by its stderr
#   NEW    no golden report yet (run make corpus-update and review it)
#   STALE  golden report without a FIT file
#   UNKNOWN an entry in real/ or synthetic/ that is not a *.fit file (lower
#          case, e.g. GARMIN01.FIT or a subdirectory); rename it to
#          <brand>-<model>-<what>.fit (ADR-004 item 4). Hidden files are skipped.
# then one line for the M and G invariants (ADR-029 rev. 1, ok or FAIL),
# then "real: N files (local only)" (real files are git-ignored, ADR-004
# rev. 2; an empty real/ passes) and a final count line. Exits 1 if any line
# is not ok.

set -u
LC_ALL=C
export LC_ALL

if [ "$#" -ne 3 ]; then
	echo "usage: $0 <binary> <corpus dir> <output dir>" >&2
	exit 2
fi
bin=$1
corpus=$2
out=$3

ok=0 diffs=0 fails=0 new=0 stale=0 unknown=0 realfiles=0

for set in real synthetic; do
	mkdir -p "$out/$set"
	# "*" does not match hidden files, so .gitkeep is skipped.
	for fit in "$corpus/$set"/*; do
		[ -e "$fit" ] || continue
		case "$fit" in
		*.fit) [ -f "$fit" ] || { echo "UNKNOWN $fit"; unknown=$((unknown + 1)); continue; } ;;
		*) echo "UNKNOWN $fit"; unknown=$((unknown + 1)); continue ;;
		esac
		name=$(basename "$fit" .fit)
		[ "$set" = real ] && realfiles=$((realfiles + 1))
		actual="$out/$set/$name.json"
		expected="$corpus/expected/$set/$name.json"
		errfile="$out/$set/$name.stderr"
		data="$out/$set/$name.bin"
		rm -f "$data" "$data.br"
		"$bin" parse -datafile "$data" "$fit" >"$actual" 2>"$errfile"
		status=$?
		if [ "$status" -ne 0 ] && [ "$status" -ne 3 ]; then
			echo "FAIL   $set/$name.fit"
			sed 's/^/       /' "$errfile"
			fails=$((fails + 1))
			rm -f "$errfile"
			continue
		fi
		if [ "$status" -eq 3 ]; then
			sizes="rejected"
		else
			sizes="bin $(wc -c <"$data" | tr -d ' ') br $(wc -c <"$data.br" | tr -d ' ')"
		fi
		if [ ! -f "$expected" ]; then
			echo "NEW    $set/$name.fit  $sizes"
			new=$((new + 1))
		elif cmp -s "$expected" "$actual"; then
			echo "ok     $set/$name.fit  $sizes"
			ok=$((ok + 1))
		else
			echo "DIFF   $set/$name.fit  $sizes"
			diff -u "$expected" "$actual"
			diffs=$((diffs + 1))
		fi
		rm -f "$errfile"
	done
	for json in "$corpus/expected/$set"/*.json; do
		[ -e "$json" ] || continue
		name=$(basename "$json" .json)
		if [ ! -f "$corpus/$set/$name.fit" ]; then
			echo "STALE  expected/$set/$name.json"
			stale=$((stale + 1))
		fi
	done
done

# ADR-029 rev. 1 (QA-17-r1-1): the moving-time and gain invariants on every
# accepted corpus file, synthetic and local real, normalised and decoded
# (TestRoundTripCorpus). Failures name the file, never a value from it; the
# full output stays in <output dir>/invariants.log.
inv=0
log=$(cd "$out" && pwd)/invariants.log
if (cd "$(dirname "$0")/.." && go test -count=1 -run '^TestRoundTripCorpus$' ./internal/datafile >"$log" 2>&1); then
	echo "ok     invariants (M, G) on every accepted file"
else
	echo "FAIL   invariants (M, G)"
	grep -E -- '--- FAIL' "$log" | sed 's/^/       /'
	inv=1
fi

echo "real: $realfiles files (local only)"
echo "corpus: $ok ok, $diffs diff, $fails fail, $new new, $stale stale, $unknown unknown"
[ $((diffs + fails + new + stale + unknown + inv)) -eq 0 ]
