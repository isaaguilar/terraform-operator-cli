#!/usr/bin/env zsh
set -o nounset
set -o errexit
set -o pipefail

# TODO
#       Get the last release, and figure out a way to compare it to the
#       new changelog. That will allow this to work on any machine since
#       state does not have to persist on a single machine.

stat .rmgmt/changelogs/next.md .rmgmt/_lasthash ".rmgmt/releases/$VERSION" >/dev/null
if diff <(md5 .rmgmt/changelogs/next.md) .rmgmt/_lasthash; then
    printf "\nChangelog '.rmgmt/changelogs/next.md' not updated. Please update to continue.\n\n"
    exit 1
fi

git tag $VERSION

changelog=".rmgmt/changelogs/${VERSION}.md"
printf "## Changes in $VERSION\n\n" > "$changelog"
cat ".rmgmt/changelogs/next.md" >> "$changelog"

# Release!
git push origin tag $VERSION
gh release create $VERSION -t "$VERSION release" -F "$changelog" .rmgmt/releases/$VERSION/*.tgz

md5 .rmgmt/changelogs/next.md > .rmgmt/_lasthash