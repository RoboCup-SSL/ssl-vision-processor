#!/usr/bin/env bash
# The -ldflags -X flags that stamp internal/buildinfo with the git state of
# this repository and of the proto submodule. Prints nothing for one that
# isn't a git checkout (a source tarball, an uninitialized submodule), which
# the binary then reports as unknown.
#
# Usage: scripts/build-ldflags.sh <buildinfo package import path>
set -u

pkg="$1"

# stamp <variable prefix> <directory>
stamp() {
	local prefix="$1" dir="$2" commit branch tag dirty=false

	# An uninitialized submodule is a plain folder inside this repository, so
	# git there would answer for this repository instead.
	[ -d "$dir" ] || return 0
	[ "$(git -C "$dir" rev-parse --show-toplevel 2>/dev/null)" = "$(cd "$dir" && pwd -P)" ] || return 0

	commit=$(git -C "$dir" rev-parse HEAD) || return 0
	branch=$(git -C "$dir" symbolic-ref --quiet --short HEAD) || branch=""
	tag=$(git -C "$dir" describe --tags --exact-match 2>/dev/null) || tag=""
	[ -n "$(git -C "$dir" status --porcelain --untracked-files=no)" ] && dirty=true

	printf -- '-X %s.%sCommit=%s -X %s.%sBranch=%s -X %s.%sTag=%s -X %s.%sDirty=%s ' \
		"$pkg" "$prefix" "$commit" "$pkg" "$prefix" "$branch" "$pkg" "$prefix" "$tag" "$pkg" "$prefix" "$dirty"
}

root=$(git rev-parse --show-toplevel 2>/dev/null) || exit 0

stamp repo "$root"
stamp proto "$root/proto"
