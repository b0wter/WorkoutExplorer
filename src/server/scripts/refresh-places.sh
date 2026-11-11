#!/bin/sh
# Refreshes internal/places/places.tsv from GeoNames cities15000 (ADR-045, F-45 ST-1).
# By hand, rarely; make ci never runs it. Downloads into build/ (git-ignored).
#
# usage: scripts/refresh-places.sh   (from src/server)
set -eu
LC_ALL=C
url=https://download.geonames.org/export/dump/cities15000.zip
work=../../build/geonames
mkdir -p "$work"
curl -sSfL -o "$work/cities15000.zip" "$url"
unzip -oq "$work/cities15000.zip" -d "$work"
# Columns 2 (name), 5, 6 (latitude, longitude), upstream order; feature code
# PPLX (city districts, column 8) dropped so a district never names the town (OQ-162).
awk -F'\t' '$8 != "PPLX" { print $2 "\t" $5 "\t" $6 }' "$work/cities15000.txt" > internal/places/places.tsv
cat > internal/places/SOURCE.md <<NOTE
# Source of places.tsv

GeoNames \`cities15000\`, <$url>, downloaded $(date -u +%Y-%m-%d).
Licence: Creative Commons Attribution 4.0 (CC BY 4.0), <https://www.geonames.org/>.
Modified: columns reduced to name, latitude, longitude; rows with feature code PPLX (city districts) dropped.

Command (scripts/refresh-places.sh):

    awk -F'\t' '\$8 != "PPLX" { print \$2 "\t" \$5 "\t" \$6 }' cities15000.txt > places.tsv
NOTE
