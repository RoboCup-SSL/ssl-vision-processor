package multicast

import (
	"net"
	"slices"
	"testing"
)

func facts(name string, flags net.Flags, ip string) ifaceFacts {
	f := ifaceFacts{name: name, flags: flags}
	if ip != "" {
		f.ip = net.ParseIP(ip).To4()
		f.mask = net.CIDRMask(24, 32)
	}

	return f
}

const physical = net.FlagUp | net.FlagRunning | net.FlagBroadcast | net.FlagMulticast

func TestDescribeAutoSelection(t *testing.T) {
	cases := []struct {
		f      ifaceFacts
		use    bool
		reason string
	}{
		{facts("enp10s0", physical, "192.168.30.231"), true, ""},
		{facts("lo", net.FlagUp|net.FlagRunning|net.FlagLoopback, "127.0.0.1"), false, "loopback"},
		{facts("wlp9s0", net.FlagBroadcast|net.FlagMulticast, ""), false, "down"},
		{facts("eth1", physical&^net.FlagRunning, "10.0.0.2"), false, "no carrier"},
		{facts("eth2", physical&^net.FlagMulticast, "10.0.1.2"), false, "no multicast"},
		{facts("eth3", physical, ""), false, "no IPv4 address"},
		{facts("docker0", physical, "172.17.0.1"), false, "virtual"},
		{facts("tailscale0", physical, "100.64.0.1"), false, "virtual"},
	}

	for _, tc := range cases {
		got := describe(tc.f)
		if got.AutoUse != tc.use || got.AutoReason != tc.reason {
			t.Errorf("%s: autoUse=%v reason=%q, want %v %q", tc.f.name, got.AutoUse, got.AutoReason, tc.use, tc.reason)
		}
	}

	if got := describe(facts("enp10s0", physical, "192.168.30.231")); got.Broadcast != "192.168.30.255" {
		t.Errorf("broadcast = %s, want 192.168.30.255", got.Broadcast)
	}
}

func TestSelectAutoAndManual(t *testing.T) {
	ifaces := []Interface{
		describe(facts("enp10s0", physical, "192.168.30.231")),
		describe(facts("docker0", physical, "172.17.0.1")),
		describe(facts("eth1", physical&^net.FlagRunning, "10.0.0.2")),
	}

	_, skipped := Select(ifaces, true, []string{"enp10s0"})
	if !slices.Equal(skipped, []string{"docker0", "eth1"}) {
		t.Errorf("auto skipped %v, want docker0 and eth1 (the skip list is ignored)", skipped)
	}

	// Manual: everything usable except what's skipped, so a virtual
	// interface can be opted back in.
	marked, skipped := Select(ifaces, false, []string{"enp10s0"})
	if !slices.Equal(skipped, []string{"enp10s0"}) || !marked[1].Used || !marked[2].Used {
		t.Errorf("manual skipped %v, marked %+v, want only enp10s0 skipped", skipped, marked)
	}
}
