// Package snapshot serves the debug images the C++ vision processor writes to
// disk as <cam_id>.<view>.{jpg,png}, via tmp-then-rename so reads never see a
// torn frame.
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

// HandleList lists the debug images vision_processor has written to dir
// (-imgDir), by camera and view.
func HandleList(dir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		entries, err := list(dir)
		if err != nil {
			slog.Error("listing snapshots", "dir", dir, "err", err)
			httpjson.Error(w, http.StatusInternalServerError, httpjson.CodeInternal, "internal error")

			return
		}

		httpjson.Write(w, http.StatusOK, entries)
	}
}

func list(dir string) ([]Entry, error) {
	files, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return []Entry{}, nil
	}

	if err != nil {
		return nil, err
	}

	seen := make(map[Entry]struct{})

	for _, f := range files {
		m := filename.FindStringSubmatch(f.Name())
		if m == nil {
			continue
		}

		id, err := strconv.Atoi(m[1])
		if err != nil {
			continue // too many digits for an int
		}

		seen[Entry{CameraID: id, View: m[2]}] = struct{}{}
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

// latest returns the most recently modified file matching dir/camID.view.*, or
// "" if none exists.
func latest(dir, camID, view string) (string, error) {
	matches, err := filepath.Glob(filepath.Join(dir, camID+"."+view+".*"))
	if err != nil {
		return "", err
	}

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
