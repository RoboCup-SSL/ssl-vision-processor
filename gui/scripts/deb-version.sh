#!/usr/bin/env bash
#
# The .deb version, from git: a release tag v1.2.3 is 1.2.3, and v1.2.3-rc1 is
# 1.2.3~rc1, which sorts before 1.2.3. Commits after a tag add +git<n>.g<sha>,
# which sorts after it; with no tag at all, 0.0.0+git<commits>.g<sha>. A dirty
# tree adds .dirty.

set -euo pipefail

if desc=$(git describe --tags --match 'v[0-9]*' --long 2>/dev/null); then
	# v1.2.3-rc1-4-gabc1234: the tag, commits since, and the commit.
	tag=${desc%-*-g*}
	rest=${desc#"$tag"-}
	count=${rest%%-*}
	sha=${rest#*-}
	version=${tag#v}
	version=${version//-/\~}
	if [ "$count" != 0 ]; then
		version="$version+git$count.$sha"
	fi
else
	version="0.0.0+git$(git rev-list --count HEAD).g$(git rev-parse --short HEAD)"
fi

if ! git diff --quiet HEAD -- 2>/dev/null; then
	case "$version" in
	*+*) version="$version.dirty" ;;
	*) version="$version+dirty" ;;
	esac
fi

echo "$version"
