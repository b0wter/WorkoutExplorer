#!/bin/sh
# Sign and publish a draft release (F-52 AU-5, OQ-212, P-2; ADR-050 item 4). Run by the operator on
# their own machine, from a checkout that has the tag, after CI created the draft:
#   deploy/sign-release.sh vX.Y.Z [secret key]   (default key: ~/.minisign/workout-explorer.key)
# Needs gh (logged in), git, make, minisign, sha256sum, GNU tar. The secret key stays outside the
# repository and is never uploaded; minisign asks for its password. Every check is against the
# local tag, never CI's word (AU-5): (a) the release is a draft named for the tag, no higher release
# is published (P-4), every asset is listed in SHA256SUMS and matches, the names carry the tag
# (AU-4); (b) compose.yaml names exactly ghcr.io/b0wter/workout-explorer:<tag>@sha256:<digest>;
# (c) the deploy archive equals `make deploy-archive` with that reference, run in a clean worktree
# of the tag: same members, types, modes and bytes (this covers COMMIT and the Podman Image=);
# (d) the tarball's COMMIT is the tag's commit. Any mismatch signs nothing. Still trusted: the image
# bytes behind the digest and the tarball's binary, both built by CI (P-9). Then it signs SHA256SUMS,
# verifies the signature with deploy/minisign.pub, uploads SHA256SUMS.minisig, and publishes the
# release as latest (which moves the image's `latest` through CI's latest workflow).
set -eu
tag=${1:?usage: deploy/sign-release.sh vX.Y.Z [secret key]}
key=${2:-$HOME/.minisign/workout-explorer.key}
printf '%s' "$tag" | grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+$' || { echo "sign: '$tag' is not a vX.Y.Z tag" >&2; exit 1; }
top=$(git rev-parse --show-toplevel)
pub=$top/deploy/minisign.pub
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
# No failing command may end the clean-up (set -e); signals exit so that dash runs the EXIT trap too.
trap '[ ! -d "$tmp/wt" ] || git -C "$top" worktree remove --force "$tmp/wt" >/dev/null 2>&1 || true; rm -rf "$tmp"' EXIT
trap 'exit 129' HUP; trap 'exit 130' INT; trap 'exit 143' TERM
gh release download "$tag" --dir "$tmp/r" --pattern '*'
rm -f "$tmp/r/SHA256SUMS.minisig" # a draft from an interrupted run; signed again below
cd "$tmp/r"
deploy=workout-explorer-$tag-deploy.tar.gz
tarball=workout-explorer-$tag-linux-amd64.tar.gz
# (a) Every asset listed, every listed file present and matching, nothing else.
[ "$(ls | grep -vx SHA256SUMS | sort)" = "$(sed 's/^[0-9a-f]\{64\}  //' SHA256SUMS | sort)" ] || { echo "sign: the assets differ from the files SHA256SUMS lists" >&2; exit 1; }
sha256sum -c --quiet SHA256SUMS || { echo "sign: a checksum does not match" >&2; exit 1; }
esc=$(printf '%s' "$tag" | sed 's/\./\\./g')
! grep -Evq "^[0-9a-f]{64}  workout-explorer-$esc-[A-Za-z0-9._-]+\$" SHA256SUMS && [ -f "$deploy" ] && [ -f "$tarball" ] ||
	{ echo "sign: SHA256SUMS names a file without the tag, or an archive is missing" >&2; exit 1; }
# (b) Our repository, this tag, a digest: fixed here, not a pattern for the owner.
ref=$(tar -xOzf "$deploy" ./compose.yaml | sed -n "s|^    image: \(ghcr\.io/b0wter/workout-explorer:$esc@sha256:[0-9a-f]\{64\}\)\$|\1|p")
[ -n "$ref" ] && [ "$(printf '%s\n' "$ref" | wc -l)" = 1 ] ||
	{ echo "sign: the deploy archive's compose.yaml does not name ghcr.io/b0wter/workout-explorer:$tag by digest exactly once" >&2; exit 1; }
# (c) Rebuilt from the tag with that reference; listings compared without dates, then the bytes.
git -C "$top" worktree add -q --detach "$tmp/wt" "$sha" && make -s -C "$tmp/wt" deploy-archive APP_IMAGE="$ref" >/dev/null ||
	{ echo "sign: cannot rebuild the deploy archive from $tag" >&2; exit 1; }
listing() { tar --numeric-owner -tvzf "$1" | awk '{ $4 = ""; $5 = ""; print }' | sort; }
mkdir "$tmp/x" "$tmp/y"
# The draft is unpacked only after its listing equals ours: regular files and directories, our names.
[ "$(listing "$deploy")" = "$(listing "$tmp/wt/build/deploy.tar.gz")" ] &&
	tar -xzf "$deploy" -C "$tmp/x" && tar -xzf "$tmp/wt/build/deploy.tar.gz" -C "$tmp/y" && diff -r "$tmp/x" "$tmp/y" >/dev/null ||
	{ echo "sign: the deploy archive differs from the one $tag builds (members, types, modes or bytes)" >&2; exit 1; }
# (d)
[ "$(tar -xOzf "$tarball" COMMIT)" = "$sha" ] || { echo "sign: the tarball was built from another commit than $tag ($sha)" >&2; exit 1; }
echo "sign: $tag, image $ref: compare the digest with the ci job's make deploy-archive line before you enter the password"

minisign -S -s "$key" -m SHA256SUMS
minisign -V -q -p "$pub" -m SHA256SUMS || { echo "sign: the signature does not verify with deploy/minisign.pub (wrong key?)" >&2; exit 1; }
gh release upload "$tag" SHA256SUMS.minisig --clobber
gh release edit "$tag" --draft=false --latest
echo "sign: $tag signed and published"
