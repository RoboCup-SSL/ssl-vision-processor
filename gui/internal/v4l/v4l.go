// Package v4l lists this host's Video4Linux capture devices, for choosing an
// OpenCV camera path. It only sees cameras plugged into the machine running
// the GUI host.
package v4l

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// Device is one way to name a capture device.
type Device struct {
	// Path is what goes in camera.path.
	Path string `json:"path"`
	// Node is the /dev/videoN it currently points at.
	Node string `json:"node"`
	// Name is the driver's name for the device, e.g. "UC70: UC70".
	Name string `json:"name"`
	// Kind is "by-id" (follows the camera), "by-path" (follows the USB
	// port), or "node" (/dev/videoN, which can change when devices
	// re-enumerate).
	Kind string `json:"kind"`
}

// Roots are where List looks; tests point them at a fake tree.
type Roots struct {
	Dev   string // usually /dev
	Sysfs string // usually /sys/class/video4linux
}

// System is the real machine.
var System = Roots{Dev: "/dev", Sysfs: "/sys/class/video4linux"}

// List returns the capture nodes (V4L2 index 0; cameras often add a
// metadata-only node at index 1) under every stable name udev gives them,
// stable names first.
func List(r Roots) []Device {
	capture := map[string]string{} // videoN -> name

	entries, _ := os.ReadDir(r.Sysfs)
	for _, e := range entries {
		index, err := os.ReadFile(filepath.Join(r.Sysfs, e.Name(), "index"))
		if err != nil || strings.TrimSpace(string(index)) != "0" {
			continue
		}

		name, _ := os.ReadFile(filepath.Join(r.Sysfs, e.Name(), "name"))
		capture[e.Name()] = strings.TrimSpace(string(name))
	}

	var out []Device

	for _, kind := range []string{"by-id", "by-path"} {
		dir := filepath.Join(r.Dev, "v4l", kind)

		links, _ := os.ReadDir(dir)
		for _, l := range links {
			// by-path lists every USB device twice; the usbv2 form is a
			// newer alias of the same port.
			if strings.Contains(l.Name(), "-usbv2-") {
				continue
			}

			target, err := os.Readlink(filepath.Join(dir, l.Name()))
			if err != nil {
				continue
			}

			node := filepath.Base(target)
			if name, ok := capture[node]; ok {
				out = append(out, Device{
					Path: filepath.Join(dir, l.Name()),
					Node: filepath.Join(r.Dev, node),
					Name: name,
					Kind: kind,
				})
			}
		}
	}

	nodes := make([]string, 0, len(capture))
	for node := range capture {
		nodes = append(nodes, node)
	}

	slices.Sort(nodes)

	for _, node := range nodes {
		path := filepath.Join(r.Dev, node)
		out = append(out, Device{Path: path, Node: path, Name: capture[node], Kind: "node"})
	}

	return out
}
