package multicast

import (
	"net"
	"os"
	"slices"
	"strconv"
	"strings"
)

// Interface is one local network interface as the Network page shows it.
type Interface struct {
	Name string `json:"name"`
	// Address is its first IPv4 address, if any; Broadcast that subnet's
	// broadcast address.
	Address   string `json:"address,omitempty"`
	Broadcast string `json:"broadcast,omitempty"`
	// Usable means up, multicast capable, and with an IPv4 address: the
	// least a socket needs to use it at all.
	Usable bool `json:"usable"`
	// AutoUse is what automatic selection decides; AutoReason why it
	// doesn't use the interface.
	AutoUse    bool   `json:"autoUse"`
	AutoReason string `json:"autoReason,omitempty"`
	// Used is whether the host's sockets use it under the current selection.
	Used bool `json:"used"`
}

// virtualPrefixes are interface names automatic selection leaves out:
// container and VM bridges and VPN tunnels, which never carry field traffic.
var virtualPrefixes = []string{
	"docker", "br-", "veth", "virbr", "vmnet", "vboxnet", "lxc", "lxd", "cni",
	"flannel", "cali", "tun", "tap", "wg", "tailscale", "zt", "utun",
}

// ifaceFacts is what ListInterfaces reads from the OS, split out so the
// decisions made from it can be tested.
type ifaceFacts struct {
	name  string
	flags net.Flags
	ip    net.IP
	mask  net.IPMask
}

// ListInterfaces describes every local interface, with Used left false.
// Select fills it in.
func ListInterfaces() []Interface {
	ifis, err := net.Interfaces()
	if err != nil {
		return nil
	}

	out := make([]Interface, 0, len(ifis))

	for _, ifi := range ifis {
		f := ifaceFacts{name: ifi.Name, flags: ifi.Flags}

		if addrs, err := ifi.Addrs(); err == nil {
			for _, addr := range addrs {
				if ipNet, ok := addr.(*net.IPNet); ok && ipNet.IP.To4() != nil && len(ipNet.Mask) == net.IPv4len {
					f.ip, f.mask = ipNet.IP.To4(), ipNet.Mask

					break
				}
			}
		}

		out = append(out, describe(f))
	}

	return out
}

func describe(f ifaceFacts) Interface {
	i := Interface{Name: f.name}

	if f.ip != nil {
		i.Address = f.ip.String()

		if f.flags&net.FlagBroadcast != 0 {
			broadcast := make(net.IP, net.IPv4len)
			for b := range broadcast {
				broadcast[b] = f.ip[b] | ^f.mask[b]
			}

			i.Broadcast = broadcast.String()
		}
	}

	up := f.flags&net.FlagUp != 0
	multicast := f.flags&net.FlagMulticast != 0
	i.Usable = up && multicast && f.ip != nil

	switch {
	case f.flags&net.FlagLoopback != 0:
		i.AutoReason = "loopback"
	case !up:
		i.AutoReason = "down"
	case f.flags&net.FlagRunning == 0:
		i.AutoReason = "no carrier"
	case !multicast:
		i.AutoReason = "no multicast"
	case f.ip == nil:
		i.AutoReason = "no IPv4 address"
	case isVirtual(f.name):
		i.AutoReason = "virtual"
	default:
		i.AutoUse = true
	}

	return i
}

func isVirtual(name string) bool {
	return slices.ContainsFunc(virtualPrefixes, func(p string) bool {
		return strings.HasPrefix(name, p)
	})
}

// Select marks which interfaces the host uses, automatically or by skipping
// the named ones, and returns the names of all the others: the skip list for
// the sockets.
func Select(ifaces []Interface, auto bool, skip []string) ([]Interface, []string) {
	out := slices.Clone(ifaces)

	var skipped []string

	for i := range out {
		if auto {
			out[i].Used = out[i].AutoUse
		} else {
			out[i].Used = out[i].Usable && !slices.Contains(skip, out[i].Name)
		}

		if !out[i].Used {
			skipped = append(skipped, out[i].Name)
		}
	}

	return out, skipped
}

// LoopbackMulticast reports the loopback interface's name and whether it has
// multicast enabled. Ubuntu leaves it off, so programs on this machine that
// talk multicast over loopback (typical when no network is connected) don't
// hear each other. ok is false if there's no loopback interface.
func LoopbackMulticast() (name string, enabled, ok bool) {
	ifis, err := net.Interfaces()
	if err != nil {
		return "", false, false
	}

	for _, ifi := range ifis {
		if ifi.Flags&net.FlagLoopback != 0 {
			return ifi.Name, ifi.Flags&net.FlagMulticast != 0, true
		}
	}

	return "", false, false
}

// UnprivilegedPortStart is the lowest port an unprivileged process may bind
// on this host (Linux's net.ipv4.ip_unprivileged_port_start), or 1024, the
// traditional limit, where that can't be read.
func UnprivilegedPortStart() int {
	data, err := os.ReadFile("/proc/sys/net/ipv4/ip_unprivileged_port_start")
	if err != nil {
		return 1024
	}

	n, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 1024
	}

	return n
}
