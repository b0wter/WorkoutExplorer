#!/bin/sh
# Remove old bundles from web/assets (F-30 RG-10). Run by deploy.sh in the
# deploy directory, only after `regenerate` exited 0:
#   sh clean-assets.sh <tag> < names     (names: the assets the served pages use, one per line)
# A bundle is the list bundles/<tag> of one release's web/assets. Kept: the
# current release's list and every list that names an asset in use that is not
# on the current list. Every other
# list and every file in web/assets on no kept list is removed.
# Prints "files_removed=N" and "bundles_removed=N".
set -eu
tag=${1:?usage: clean-assets.sh <tag> < names}
[ -f "bundles/$tag" ] || { echo "clean-assets: no list bundles/$tag" >&2; exit 1; }
used=$(mktemp)
keep=$(mktemp)
trap 'rm -f "$used" "$keep"' EXIT
# Names on the current list keep no old list: they are content hashes, so a page
# naming only those loads only current chunks (QA-30-r1-1).
grep -v '^$' | grep -Fvx -f "bundles/$tag" > "$used" || true
cat "bundles/$tag" > "$keep"
lists=0
for l in bundles/*; do
	[ -f "$l" ] && [ "$l" != "bundles/$tag" ] || continue
	# grep -f with an empty pattern file matches nothing
	if grep -Fxq -f "$l" "$used"; then
		cat "$l" >> "$keep"
	else
		# not kept: any file of it still needed is on a kept list
		rm -f -- "$l"
		lists=$((lists + 1))
	fi
done
files=0
for f in web/assets/* web/assets/.[!.]*; do
	[ -f "$f" ] || continue
	grep -Fxq -- "${f#web/assets/}" "$keep" && continue
	rm -f -- "$f"
	files=$((files + 1))
done
echo "files_removed=$files"
echo "bundles_removed=$lists"
