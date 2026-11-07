#!/usr/bin/env bash
#
# What a new user does: run the release binary with no arguments on a machine
# that has nothing else of ours, and check it works. A first run publishes
# nothing on the network, so this is safe to run on any network.
#
# Usage: scripts/release-check.sh <binary> <expected version>

set -euo pipefail

binary=$(realpath "$1")
version="$2"

fail() {
	echo "release-check: $1" >&2
	exit 1
}

work=$(mktemp -d)
host_pid=""

cleanup() {
	if [ -n "$host_pid" ]; then
		kill "$host_pid" 2>/dev/null || true
	fi
	rm -rf "$work"
}
trap cleanup EXIT

# An empty home, an empty working directory, and nothing from this environment.
# Replaces the calling shell, so call it in a subshell; one started with & then
# has the host's own PID.
run() {
	local home="$1"
	shift
	mkdir -p "$home"
	cd "$home"
	exec env -i HOME="$home" "$binary" "$@"
}

if ldd "$binary" >/dev/null 2>&1; then
	fail "$binary is dynamically linked, so it won't run everywhere: $(ldd "$binary" | tr '\n' ' ')"
fi

got=$(run "$work/version" -version)
[ "$got" = "ssl-vision-processor-gui $version" ] || fail "-version printed: $got"
[ -z "$(ls -A "$work/version")" ] || fail "-version created files"

(run "$work/readme" -readme) | grep -q '^# vision_processor GUI' || fail "-readme didn't print the user guide"
[ -z "$(ls -A "$work/readme")" ] || fail "-readme created files"

port=$((20000 + RANDOM % 20000))
home="$work/first"
run "$home" -address "127.0.0.1:$port" >"$work/first.log" 2>&1 &
host_pid=$!

health=""
for _ in $(seq 1 100); do
	if health=$(curl -sf "http://127.0.0.1:$port/api/health"); then
		break
	fi
	kill -0 "$host_pid" 2>/dev/null || {
		cat "$work/first.log" >&2
		fail "the host exited at startup"
	}
	sleep 0.1
done

[ -n "$health" ] || fail "no answer from /api/health"
case "$health" in
*"\"version\":\"$version\""*) ;;
*) fail "/api/health said: $health" ;;
esac

config="$home/.config/rcssl/vision-processor-gui"
state="$home/.local/state/rcssl/vision-processor-gui"

for path in "$config/vision.yml" "$config/robot-heights.yml" "$state/logs/vision-processor-gui.log" "$state/img"; do
	[ -e "$path" ] || fail "$path wasn't created"
done

grep -q '^first_run: true$' "$config/vision.yml" || fail "the new config isn't marked as a first run"

for shown in "$config" "$state/logs/vision-processor-gui.log" "$state/img"; do
	grep -qF "$shown" "$work/first.log" || fail "startup didn't print $shown"
done

curl -sf "http://127.0.0.1:$port/" | grep -qi '<!doctype html' || fail "the GUI isn't served"
[ "$(curl -sf "http://127.0.0.1:$port/api/geometry/presets" | grep -o '"name"' | wc -l)" = 2 ] ||
	fail "the field presets aren't served"

# A port already in use has to read as a failure to a service manager.
if (run "$work/second" -address "127.0.0.1:$port") >"$work/second.log" 2>&1; then
	fail "a second host on the same port exited 0"
fi

kill "$host_pid"
wait "$host_pid" || fail "the host didn't exit cleanly on SIGTERM"
host_pid=""

echo "release-check: ok ($version)"
