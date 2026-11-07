//go:build !linux

package multicast

import "net"

// onlyJoinedGroups is Linux only (IP_MULTICAST_ALL). Elsewhere a socket may
// hear other groups on its port.
func onlyJoinedGroups(*net.UDPConn) error { return nil }
