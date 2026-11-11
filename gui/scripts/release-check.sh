#!/usr/bin/env bash
#
# What a new user does: run the release binary with no arguments on a machine
# that has nothing else of ours, and check it works. Then the same with
# -runCameras and a stand-in vision_processor. A first run publishes nothing
# on the network, so this is safe to run on any network.
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

# An empty home, an empty working directory, and nothing from this environment
# but PATH when run_path is set. Replaces the calling shell, so call it in a
# subshell; one started with & then has the host's own PID.
run_path=""
run() {
	local home="$1"
	shift
	mkdir -p "$home"
	cd "$home"
	exec env -i HOME="$home" ${run_path:+PATH="$run_path"} "$binary" "$@"
}

# wait_healthy waits for the host on $port to answer, or fails with its log.
wait_healthy() {
	local log="$1"
	health=""
	for _ in $(seq 1 100); do
		if health=$(curl -sf "http://127.0.0.1:$port/api/health"); then
			return
		fi
		kill -0 "$host_pid" 2>/dev/null || {
			cat "$log" >&2
			fail "the host exited at startup"
		}
		sleep 0.1
	done
	fail "no answer from /api/health"
}

if ldd "$binary" >/dev/null 2>&1; then
	fail "$binary is dynamically linked, so it won't run everywhere: $(ldd "$binary" | tr '\n' ' ')"
fi

got=$(run "$work/version" -version)
[ "$got" = "rcssl-vision-processor-gui $version" ] || fail "-version printed: $got"
[ -z "$(ls -A "$work/version")" ] || fail "-version created files"

(run "$work/readme" -readme) | grep -q '^# vision_processor GUI' || fail "-readme didn't print the user guide"
[ -z "$(ls -A "$work/readme")" ] || fail "-readme created files"

port=$((20000 + RANDOM % 20000))
home="$work/first"
run "$home" -address "127.0.0.1:$port" >"$work/first.log" 2>&1 &
host_pid=$!
wait_healthy "$work/first.log"

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

# -runCameras finds vision_processor on PATH, and won't start without it.
if (run "$work/no-vp" -runCameras -address "127.0.0.1:$port") >"$work/no-vp.log" 2>&1; then
	fail "-runCameras without vision_processor on PATH exited 0"
fi
grep -q 'vision_processor' "$work/no-vp.log" || fail "-runCameras without vision_processor didn't say why: $(cat "$work/no-vp.log")"

# A stand-in vision_processor: it records its arguments, writes a snapshot and
# a debug dump as the real one does, relative to its working directory, and
# runs until SIGTERM.
fakebin="$work/bin"
mkdir -p "$fakebin"
cat >"$fakebin/vision_processor" <<'FAKE'
#!/bin/sh
echo "$@" >args
: >img/0.raw.jpg
: >.flat.1.png
trap 'exit 0' TERM
while :; do sleep 0.1; done
FAKE
chmod +x "$fakebin/vision_processor"

home="$work/local"
config="$home/.config/rcssl/vision-processor-gui"
state="$home/.local/state/rcssl/vision-processor-gui"
run_path="$fakebin:/usr/bin:/bin"
run "$home" -runCameras -address "127.0.0.1:$port" >"$work/local.log" 2>&1 &
host_pid=$!
wait_healthy "$work/local.log"

grep -q '^ *config_path: cam0.yml$' "$config/vision.yml" || fail "camera 0 has no config_path under -runCameras"
grep -qF "bot_heights_file: $config/robot-heights.yml" "$config/cam0.yml" ||
	fail "cam0.yml doesn't name the height table by its full path"
grep -qF "$fakebin/vision_processor" "$work/local.log" || fail "startup didn't print which vision_processor it runs"

for _ in $(seq 1 100); do
	[ -e "$state/run/cam0/args" ] && [ -e "$state/img/cam0/0.raw.jpg" ] && break
	sleep 0.1
done
[ "$(cat "$state/run/cam0/args" 2>/dev/null)" = "$config/cam0.yml" ] ||
	fail "vision_processor wasn't run with cam0.yml's full path: $(cat "$state/run/cam0/args" 2>/dev/null)"
[ -e "$state/img/cam0/0.raw.jpg" ] || fail "the snapshot didn't reach camera 0's image folder"
[ -e "$state/run/cam0/.flat.1.png" ] || fail "the debug dump isn't in camera 0's own directory"
[ -z "$(ls -A "$config" | grep -v '\.yml$')" ] || fail "something besides config files landed in the config folder: $(ls -A "$config")"

kill "$host_pid"
wait "$host_pid" || fail "the -runCameras host didn't exit cleanly on SIGTERM"
host_pid=""

echo "release-check: ok ($version)"
