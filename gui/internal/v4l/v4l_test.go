package v4l

import (
	"os"
	"path/filepath"
	"testing"
)

// fakeTree builds a camera with a capture node (video1) and a metadata node
// (video2), as a UVC webcam does.
func fakeTree(t *testing.T) Roots {
	t.Helper()

	root := t.TempDir()
	r := Roots{Dev: filepath.Join(root, "dev"), Sysfs: filepath.Join(root, "sys")}

	for node, index := range map[string]string{"video1": "0", "video2": "1"} {
		dir := filepath.Join(r.Sysfs, node)
		must(t, os.MkdirAll(dir, 0o755))
		must(t, os.WriteFile(filepath.Join(dir, "index"), []byte(index+"\n"), 0o644))
		must(t, os.WriteFile(filepath.Join(dir, "name"), []byte("UC70: UC70\n"), 0o644))
	}

	links := map[string]string{
		"by-id/usb-Cam_SN1-video-index0":                      "../../video1",
		"by-id/usb-Cam_SN1-video-index1":                      "../../video2",
		"by-path/pci-0000:10:00.0-usb-0:4:1.0-video-index0":   "../../video1",
		"by-path/pci-0000:10:00.0-usbv2-0:4:1.0-video-index0": "../../video1",
	}

	for name, target := range links {
		path := filepath.Join(r.Dev, "v4l", name)
		must(t, os.MkdirAll(filepath.Dir(path), 0o755))
		must(t, os.Symlink(target, path))
	}

	return r
}

func must(t *testing.T, err error) {
	t.Helper()

	if err != nil {
		t.Fatal(err)
	}
}

func TestListCaptureNodesStableNamesFirst(t *testing.T) {
	r := fakeTree(t)

	got := List(r)

	want := []Device{
		{Path: filepath.Join(r.Dev, "v4l/by-id/usb-Cam_SN1-video-index0"), Node: filepath.Join(r.Dev, "video1"), Name: "UC70: UC70", Kind: "by-id"},
		{Path: filepath.Join(r.Dev, "v4l/by-path/pci-0000:10:00.0-usb-0:4:1.0-video-index0"), Node: filepath.Join(r.Dev, "video1"), Name: "UC70: UC70", Kind: "by-path"},
		{Path: filepath.Join(r.Dev, "video1"), Node: filepath.Join(r.Dev, "video1"), Name: "UC70: UC70", Kind: "node"},
	}

	if len(got) != len(want) {
		t.Fatalf("got %d devices, want %d: %+v", len(got), len(want), got)
	}

	for i := range want {
		if got[i] != want[i] {
			t.Errorf("device %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestListNoV4L(t *testing.T) {
	if got := List(Roots{Dev: t.TempDir(), Sysfs: t.TempDir()}); len(got) != 0 {
		t.Fatalf("got %+v on a machine without cameras", got)
	}
}
