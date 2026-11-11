# Source of places.tsv

GeoNames `cities15000`, <https://download.geonames.org/export/dump/cities15000.zip>, downloaded 2026-11-11.
Licence: Creative Commons Attribution 4.0 (CC BY 4.0), <https://www.geonames.org/>.
Modified: columns reduced to name, latitude, longitude; rows with feature code PPLX (city districts) dropped.

Command (scripts/refresh-places.sh):

    awk -F'\t' '$8 != "PPLX" { print $2 "\t" $5 "\t" $6 }' cities15000.txt > places.tsv
