// Package snapshot serves the debug images the C++ vision processor writes to
// disk as <cam_id>.<view>.{jpg,png}, via tmp-then-rename so reads never see a
// torn frame. They're in the image directory itself, or in its cam<id>
// folder for a vision_processor the host runs (-runCameras).
//
// This assumes the Go host and the vision processor share a filesystem, which
// only holds for a single-host setup; it does not work for a remote instance.
package snapshot

import (
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"

	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/httpjson"
)

// filename matches "<cam_id>.<view>.<ext>", e.g. "0.raw.jpg" or
// "0.pixels.corner.png" -- view itself may contain dots (GeomModel.cpp writes
// "pixels.corner"/"pixels.refined"), so it's `.+`, not a restricted character
// class. This only ever runs against real names returned by os.ReadDir, never
// untrusted input, so there's no injection risk in being permissive here; see
// validSegment below for the regex that actually guards a filepath.Glob call.
var filename = regexp.MustCompile(`^(\d+)\.(.+)\.(?:jpg|jpeg|png)$`)

// Entry identifies one available snapshot.
type Entry struct {
	CameraID int    `json:"cameraId"`
	View     string `json:"view"`
}

// camFolder matches a camera's own folder in the image directory.
var camFolder = regexp.MustCompile(`^cam\d+$`)

// List is the debug images vision_processor has written to dir (-imgDir) and
// its cam<id> folders, one entry per camera and view, sorted. A missing dir
// has none.
func List(dir string) ([]Entry, error) {
	files, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return []Entry{}, nil
	}

	if err != nil {
		return nil, err
	}

	seen := make(map[Entry]struct{})

	for _, f := range files {
		if f.IsDir() && camFolder.MatchString(f.Name()) {
			inner, err := os.ReadDir(filepath.Join(dir, f.Name()))
			if err != nil {
				return nil, err
			}

			for _, g := range inner {
				addEntry(seen, g.Name())
			}

			continue
		}

		addEntry(seen, f.Name())
	}

	entries := make([]Entry, 0, len(seen))
	for e := range seen {
		entries = append(entries, e)
	}

	// os.ReadDir order is directory-entry order, not usefully sorted for a UI,
	// and map iteration is randomised on top of that -- fix an order so the
	// grid does not reshuffle itself every 5s poll.
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].CameraID != entries[j].CameraID {
			return entries[i].CameraID < entries[j].CameraID
		}

		return entries[i].View < entries[j].View
	})

	return entries, nil
}

// addEntry adds the snapshot file name names, if it is one.
func addEntry(seen map[Entry]struct{}, name string) {
	m := filename.FindStringSubmatch(name)
	if m == nil {
		return
	}

	id, err := strconv.Atoi(m[1])
	if err != nil {
		return // too many digits for an int
	}

	seen[Entry{CameraID: id, View: m[2]}] = struct{}{}
}

// validSegment matches one path-value component of a snapshot request: digits
// for a camera id, or a view name (which may contain dots, e.g.
// "pixels.corner"). This guards untrusted URL input right before it reaches
// filepath.Glob in latest() -- it excludes '/' (no escaping the single path
// segment camID+"."+view+".*" is joined into) and Glob's own metacharacters
// (*, ?, [, ]), which is what actually matters; dots are harmless here.
var validSegment = regexp.MustCompile(`^[a-zA-Z0-9_.-]+$`)

// HandleGet serves a camera's latest debug image for one view, e.g.
// /api/snapshot/0/raw.
func HandleGet(dir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		camID := r.PathValue("camID")
		view := r.PathValue("view")

		if !validSegment.MatchString(camID) || !validSegment.MatchString(view) {
			httpjson.Error(w, http.StatusNotFound, httpjson.CodeNotFound, "no such snapshot")

			return
		}

		path, err := latest(dir, camID, view)
		if err != nil {
			slog.Error("finding snapshot", "cam", camID, "view", view, "err", err)
			httpjson.Error(w, http.StatusInternalServerError, httpjson.CodeInternal, "internal error")

			return
		}

		if path == "" {
			httpjson.Error(w, http.StatusNotFound, httpjson.CodeNotFound, "no such snapshot")

			return
		}

		http.ServeFile(w, r, path)
	}
}

// latest returns the most recently modified file matching camID.view.* in dir
// or its cam<camID> folder, or "" if none exists.
func latest(dir, camID, view string) (string, error) {
	name := camID + "." + view + ".*"

	matches, err := filepath.Glob(filepath.Join(dir, name))
	if err != nil {
		return "", err
	}

	own, err := filepath.Glob(filepath.Join(dir, "cam"+camID, name))
	if err != nil {
		return "", err
	}

	matches = append(matches, own...)

	var (
		newest     string
		newestTime int64
	)

	for _, path := range matches {
		info, err := os.Stat(path)
		if err != nil {
			continue
		}

		if t := info.ModTime().UnixNano(); t > newestTime {
			newest, newestTime = path, t
		}
	}

	return newest, nil
}
