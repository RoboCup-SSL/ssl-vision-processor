package multicast

import (
	"net"

	"golang.org/x/sys/unix"
)

// onlyJoinedGroups limits conn to the groups it joined itself. Go binds a
// multicast listen address as 0.0.0.0, and Linux by default delivers every
// group joined by any socket on the machine to such a socket, so without
// this each camera's video socket on the shared port hears every camera.
func onlyJoinedGroups(conn *net.UDPConn) error {
	raw, err := conn.SyscallConn()
	if err != nil {
		return err
	}

	var sockErr error

	if err := raw.Control(func(fd uintptr) {
		sockErr = unix.SetsockoptInt(int(fd), unix.IPPROTO_IP, unix.IP_MULTICAST_ALL, 0)
	}); err != nil {
		return err
	}

	return sockErr
}
