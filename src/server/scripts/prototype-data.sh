#!/bin/sh
# make prototype-data driver (F-07 design, section B-1:
# architect/F-07-prototype-page-and-scrubbing-gate.md).
#
# usage: prototype-data.sh <binary> <corpus dir> <out dir> [<real.fit> ...]
#
# Writes <out dir>/<name>.bin, <name>.bin.br and <name>.json for the synthetic
# ride-12h, ride-4h and the sample pages' data files of <corpus dir>/synthetic and for every real file given
# (named real-<base>). The outputs contain coordinates: they stay under build/
# (git-ignored, ADR-004 rev. 3) and real names and sizes appear only in the
# terminal. Prints per file
#   ok     <name>  bin <bytes> br <bytes>
# then the ride-4h budget line (NFR: 500000 bytes brotli) and a count line.
# Exits 1 on the first failure, 2 on bad arguments.

set -eu
LC_ALL=C
export LC_ALL

if [ "$#" -lt 3 ]; then
	echo "usage: $0 <binary> <corpus dir> <out dir> [<real.fit> ...]" >&2
	exit 2
fi
bin=$1
corpus=$2
out=$3
shift 3

mkdir -p "$out"

count=0

# gen <name> <fit file>
gen() {
	name=$1
	fit=$2
	rm -f "$out/$name.bin" "$out/$name.bin.br" "$out/$name.json"
	if ! "$bin" parse -datafile "$out/$name.bin" "$fit" >"$out/$name.json"; then
		echo "FAIL   $name"
		exit 1
	fi
	brsize=$(wc -c <"$out/$name.bin.br" | tr -d ' ')
	echo "ok     $name  bin $(wc -c <"$out/$name.bin" | tr -d ' ') br $brsize"
	count=$((count + 1))
}

gen ride-12h "$corpus/synthetic/ride-12h.fit"
gen ride-4h "$corpus/synthetic/ride-4h.fit"
# Data files the sample pages point at (F-13 design B-1; tools/protoserve dataFor).
for n in ride-30min run-30min indoor-no-gps indoor-no-distance-no-gps pool-swim triathlon; do
	gen "$n" "$corpus/synthetic/$n.fit"
done

br4=$(wc -c <"$out/ride-4h.bin.br" | tr -d ' ')
if [ "$br4" -lt 500000 ]; then
	echo "budget ride-4h br $br4 / 500000 PASS"
else
	echo "budget ride-4h br $br4 / 500000 FAIL"
	exit 1
fi

for f in "$@"; do
	if [ ! -f "$f" ]; then
		echo "FAIL   $f: not a file"
		exit 1
	fi
	case "$f" in
	*.fit) ;;
	*)
		echo "FAIL   $f: not a .fit file"
		exit 1
		;;
	esac
	base=$(basename "$f" .fit)
	case "$base" in
	'' | -* | *[!a-z0-9-]*)
		echo "FAIL   $f: name must be lower-case letters, digits and hyphens"
		exit 1
		;;
	esac
	gen "real-$base" "$f"
done

echo "prototype data: $count files in $out"
