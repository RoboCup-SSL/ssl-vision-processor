#!/usr/bin/env bash
#
# Installs the GUI's .deb as an admin would and checks the result, through a
# reconfigure, an upgrade, removal and purge. With systemd running, as on a CI
# runner, it checks the service too; in a container without it, only the
# install. It changes the machine it runs on: run it only on a CI runner or in
# a throwaway container.
#
# Usage: sudo scripts/deb-check.sh <.deb> <expected version>

set -euo pipefail

deb=$(realpath "$1")
version="$2"

package=rcssl-vision-processor-gui
unit=$package.service
defaults=/etc/default/$package
state=/var/lib/rcssl/vision-processor-gui
wants=/etc/systemd/system/multi-user.target.wants/$unit

fail() {
	echo "deb-check: $1" >&2
	exit 1
}

[ "$(id -u)" = 0 ] || fail "run as root: it installs the package"

systemd=false
if [ -d /run/systemd/system ]; then
	systemd=true
else
	echo "deb-check: no systemd running, so the service itself isn't checked"
fi

export DEBIAN_FRONTEND=noninteractive

answer() {
	echo "$package $package/$1 select $2" | debconf-set-selections
}

# wait_healthy waits for the service to answer on port 8085.
wait_healthy() {
	for _ in $(seq 1 100); do
		if health=$(curl -sf http://127.0.0.1:8085/api/health); then
			case "$health" in
			*"\"version\":\"$version\""*) return ;;
			*) fail "/api/health said: $health" ;;
			esac
		fi
		sleep 0.1
	done
	journalctl -u "$unit" --no-pager -n 30 >&2 || true
	fail "the service didn't answer on port 8085"
}

# A non-interactive install takes the defaults: no service.
apt-get install -y "$deb" >/dev/null

[ "$(rcssl-vision-processor-gui -version)" = "rcssl-vision-processor-gui $version" ] || fail "wrong binary version"
id -nG rcssl-vp-gui | tr ' ' '\n' | grep -qx rcssl || fail "rcssl-vp-gui isn't in the rcssl group"
grep -qx 'ADDRESS=127.0.0.1:8085' "$defaults" || fail "$defaults doesn't limit the GUI to this machine"
grep -qx 'ARGS=' "$defaults" || fail "$defaults has no ARGS line for extra options"
[ ! -e "$wants" ] || fail "a non-interactive install enabled the service"

if $systemd; then
	! systemctl --quiet is-active "$unit" || fail "a non-interactive install started the service"
	# A container can't set it: it's the host's.
	if ! systemd-detect-virt --quiet --container; then
		[ "$(sysctl -n net.core.rmem_max)" -ge 4194304 ] || fail "net.core.rmem_max wasn't raised"
	fi
fi

# Reconfigured to start now, on every interface, with an extra option added by
# hand, which reconfiguring keeps.
sed -i 's/^ARGS=.*/ARGS="-logLevel Debug"/' "$defaults"
answer service now
answer listen network
dpkg-reconfigure "$package"

[ -e "$wants" ] || fail "reconfiguring didn't enable the service"
grep -qx 'ADDRESS=:8085' "$defaults" || fail "reconfiguring didn't open the GUI to the network"
grep -qx 'ARGS="-logLevel Debug"' "$defaults" || fail "reconfiguring dropped ARGS"

if $systemd; then
	wait_healthy
	[ "$(stat -c %U:%G "$state")" = rcssl-vp-gui:rcssl ] || fail "$state is owned by $(stat -c %U:%G "$state")"
	grep -q '^first_run: true$' "$state/vision.yml" || fail "the service's new config isn't a first run"
	ss -ltnH 'sport = :8085' | grep -Eq '(\*|0\.0\.0\.0|\[::\]):8085' || fail "the service isn't listening on every interface"
	tr '\0' ' ' </proc/"$(systemctl show -p MainPID --value "$unit")"/cmdline | grep -q -- '-logFile= -logLevel Debug' ||
		fail "ARGS didn't reach the service's command line"
fi

# An upgrade keeps the service and address as the admin left them, whatever
# the stored answers say: here, an address narrowed by hand, and a stored
# answer of no service.
answer service none
sed -i 's/^ADDRESS=.*/ADDRESS=127.0.0.1:8085/' "$defaults"
apt-get install -y --reinstall "$deb" >/dev/null

[ -e "$wants" ] || fail "an upgrade disabled the service"
grep -qx 'ADDRESS=127.0.0.1:8085' "$defaults" || fail "an upgrade changed the address"

if $systemd; then
	wait_healthy
fi

# Reconfigured to no service: the stored answer, applied this time.
answer service none
dpkg-reconfigure "$package"

[ ! -e "$wants" ] || fail "reconfiguring to no service left it enabled"
if $systemd; then
	! systemctl --quiet is-active "$unit" || fail "reconfiguring to no service left it running"
fi

# Removing keeps the settings; purging drops them but keeps the GUI's own
# configuration, which holds locked calibrations.
apt-get remove -y "$package" >/dev/null
! command -v rcssl-vision-processor-gui >/dev/null || fail "remove left the binary"
[ -f "$defaults" ] || fail "remove deleted $defaults"

apt-get purge -y "$package" >/dev/null
[ ! -e "$defaults" ] || fail "purge left $defaults"
[ -z "$(debconf-show "$package" 2>/dev/null)" ] || fail "purge left the debconf answers"
if $systemd; then
	[ -f "$state/vision.yml" ] || fail "purge deleted the GUI's configuration"
fi

echo "deb-check: ok ($deb)"
