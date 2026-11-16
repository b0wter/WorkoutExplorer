#!/bin/sh
# Sign and publish a draft release (F-52 AU-5, OQ-212, P-2; ADR-050 item 4). Run by the operator on
# their own machine, from a checkout that has the tag, after CI created the draft:
#   deploy/sign-release.sh vX.Y.Z [secret key]   (default key: ~/.minisign/workout-explorer.key)
# Needs gh (logged in), git, minisign, sha256sum, tar. The secret key stays outside the repository
# and is never uploaded; minisign asks for its password. Checks: the release is a draft named for
# the tag, no higher release is published (P-4), every asset is listed in SHA256SUMS and matches,
# the names carry the tag (AU-4), COMMIT in both archives is the tag's commit, and our image is
# named by digest. Then it signs SHA256SUMS, verifies the signature with deploy/minisign.pub,
# uploads SHA256SUMS.minisig, and publishes the release as latest (which moves the image's
# `latest` through CI's latest workflow).
set -eu
tag=${1:?usage: deploy/sign-release.sh vX.Y.Z [secret key]}
key=${2:-$HOME/.minisign/workout-explorer.key}
printf '%s' "$tag" | grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+$' || { echo "sign: '$tag' is not a vX.Y.Z tag" >&2; exit 1; }
top=$(git rev-parse --show-toplevel)
pub=$top/deploy/minisign.pub
! grep -q PLACEHOLDER "$pub" || { echo "sign: deploy/minisign.pub is still the placeholder (RUNBOOK.md, \"Release signing key\")" >&2; exit 1; }
[ -f "$key" ] || { echo "sign: no secret key at $key" >&2; exit 1; }
case $(cd "$(dirname "$key")" && pwd -P)/ in "$(cd "$top" && pwd -P)"/*) echo "sign: the secret key must not live inside the repository" >&2; exit 1 ;; esac
sha=$(git rev-parse "$tag^{commit}")

[ "$(gh release view "$tag" --json isDraft -q .isDraft)" = true ] || { echo "sign: $tag is not a draft release (already published?)" >&2; exit 1; }
cur=$(gh release view --json tagName -q .tagName 2>/dev/null || true) # the latest published release
if [ -n "$cur" ]; then
	hi=$(printf '%s\n%s\n' "${cur#v}" "${tag#v}" | sort -t. -k1,1n -k2,2n -k3,3n | tail -1)
	[ "$hi" = "${tag#v}" ] && [ "$cur" != "$tag" ] || { echo "sign: $cur is already published; releases go out in version order (P-4)" >&2; exit 1; }
fi

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
gh release download "$tag" --dir "$tmp/r" --pattern '*'
rm -f "$tmp/r/SHA256SUMS.minisig" # a draft from an interrupted run; signed again below
cd "$tmp/r"
deploy=workout-explorer-$tag-deploy.tar.gz
tarball=workout-explorer-$tag-linux-amd64.tar.gz
# Every asset listed, every listed file present and matching, nothing else.
[ "$(ls | grep -vx SHA256SUMS | sort)" = "$(sed 's/^[0-9a-f]\{64\}  //' SHA256SUMS | sort)" ] || { echo "sign: the assets differ from the files SHA256SUMS lists" >&2; exit 1; }
sha256sum -c --quiet SHA256SUMS || { echo "sign: a checksum does not match" >&2; exit 1; }
esc=$(printf '%s' "$tag" | sed 's/\./\\./g')
! grep -Evq "^[0-9a-f]{64}  workout-explorer-$esc-[A-Za-z0-9._-]+\$" SHA256SUMS && [ -f "$deploy" ] && [ -f "$tarball" ] ||
	{ echo "sign: SHA256SUMS names a file without the tag, or an archive is missing" >&2; exit 1; }
[ "$(tar -xOzf "$deploy" ./COMMIT)" = "$sha" ] && [ "$(tar -xOzf "$tarball" COMMIT)" = "$sha" ] ||
	{ echo "sign: the release was built from another commit than $tag ($sha)" >&2; exit 1; }
tar -xOzf "$deploy" ./compose.yaml | grep -Eq "^    image: ghcr\.io/[a-z0-9._-]+/workout-explorer:$esc@sha256:[0-9a-f]{64}\$" ||
	{ echo "sign: the deploy archive's compose.yaml does not name our image by digest" >&2; exit 1; }

minisign -S -s "$key" -m SHA256SUMS
minisign -V -q -p "$pub" -m SHA256SUMS || { echo "sign: the signature does not verify with deploy/minisign.pub (wrong key?)" >&2; exit 1; }
gh release upload "$tag" SHA256SUMS.minisig --clobber
gh release edit "$tag" --draft=false --latest
echo "sign: $tag signed and published"
