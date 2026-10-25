#!/usr/bin/env bash
#
# Check, enable, or disable multicast on the loopback interface.
#
# vision_processor and the gui host talk over multicast. When both run on one
# machine, or a test sends and receives on the same machine, the packets go
# over loopback, which Ubuntu ships with multicast off. The change takes effect
# at once and lasts until reboot.
#
# Usage: scripts/lo-multicast.sh [status|on|off] [interface]
#   status  print whether multicast is on; exit 0 if on, 1 if off (default)
#   on      enable it (uses sudo when not root)
#   off     disable it again (uses sudo when not root)
#   interface defaults to lo

set -euo pipefail

action="${1:-status}"
iface="${2:-lo}"

if ! command -v ip >/dev/null; then
	echo "lo-multicast: the ip command (iproute2) is required" >&2
	exit 2
fi

if ! ip link show "$iface" >/dev/null 2>&1; then
	echo "lo-multicast: no interface named $iface" >&2
	exit 2
fi

enabled() {
	# The flags list, e.g. <LOOPBACK,MULTICAST,UP,LOWER_UP>.
	ip -o link show "$iface" | grep -q '<[^>]*\bMULTICAST\b[^>]*>'
}

as_root() {
	if [ "$(id -u)" -eq 0 ]; then
		"$@"
	else
		sudo "$@"
	fi
}

case "$action" in
status)
	if enabled; then
		echo "multicast is on for $iface"
	else
		echo "multicast is off for $iface; enable it with: $0 on $iface"
		exit 1
	fi
	;;
on | off)
	as_root ip link set "$iface" multicast "$action"
	if enabled; then state=on; else state=off; fi
	echo "multicast is now $state for $iface"
	[ "$state" = "$action" ]
	;;
*)
	echo "usage: $0 [status|on|off] [interface]" >&2
	exit 2
	;;
esac
