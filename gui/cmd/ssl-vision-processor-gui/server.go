package main

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/config"
	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/geometry"
	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/hub"
	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/v4l"
	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/video"
	"google.golang.org/protobuf/encoding/protojson"
)

type VisionServer struct {
	geometry *geometry.Geometry
	store    *config.Store
	hub      *hub.Hub
	video    *video.Manager
	imgDir   string
}

func NewVisionServer(geom *geometry.Geometry, store *config.Store, wsHub *hub.Hub, videos *video.Manager, imgDir string) http.Handler {
	s := &VisionServer{geometry: geom, store: store, hub: wsHub, video: videos, imgDir: imgDir}

	mux := http.NewServeMux()
	s.addRoutes(mux)

	return mux
}

// handleCameraDevices lists this host's capture devices for the camera path
// picker. Empty, not an error, on a machine without Video4Linux.
func (s *VisionServer) handleCameraDevices() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		devices := v4l.List(v4l.System)
		if devices == nil {
			devices = []v4l.Device{}
		}

		writeJSON(w, devices)
	}
}

func (s *VisionServer) handleHealth() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]string{"status": "ok"})
	}
}

// handleGetGeometry serves the current wrapper packet as canonical protojson,
// matching the wire encoding the rest of the league already expects.
func (s *VisionServer) handleGetGeometry() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data, err := protojson.Marshal(s.geometry.Snapshot())
		if err != nil {
			slog.Error("marshalling geometry", "err", err)
			http.Error(w, "internal error", http.StatusInternalServerError)

			return
		}

		w.Header().Set("Content-Type", "application/json")

		if _, err := w.Write(data); err != nil {
			slog.Error("writing geometry response", "err", err)
		}
	}
}

// configResponse is the working document plus the store's state.
type configResponse struct {
	Document config.Document `json:"document"`
	State    config.State    `json:"state"`
}

func (s *VisionServer) handleGetConfig() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		doc, _ := s.store.Working()
		writeJSON(w, configResponse{Document: doc, State: s.store.State()})
	}
}

type revisionResponse struct {
	Revision int64 `json:"revision"`
}

type putConfigRequest struct {
	Revision int64           `json:"revision"`
	Document config.Document `json:"document"`
}

// handlePutConfig replaces the working document, applying it live. Nothing
// is written to vision.yml until a save.
func (s *VisionServer) handlePutConfig() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req putConfigRequest
		if !decodeJSON(w, r, &req) {
			return
		}

		rev, err := s.store.Update(req.Revision, req.Document)
		if err != nil {
			respondConfigErr(w, err, "updating config")

			return
		}

		writeJSON(w, revisionResponse{Revision: rev})
	}
}

type saveRequest struct {
	Revision int64 `json:"revision"`
	// Force overwrites vision.yml even if it changed on disk since it was
	// loaded -- the "overwrite disk" answer to the external-change prompt.
	Force bool `json:"force"`
}

func (s *VisionServer) handleSave() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req saveRequest
		if !decodeJSON(w, r, &req) {
			return
		}

		if err := s.store.Save(req.Revision, req.Force); err != nil {
			respondConfigErr(w, err, "saving config")

			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

type saveAsRequest struct {
	Revision int64  `json:"revision"`
	Path     string `json:"path"`
}

func (s *VisionServer) handleSaveAs() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req saveAsRequest
		if !decodeJSON(w, r, &req) {
			return
		}

		if req.Path == "" {
			http.Error(w, "path: required", http.StatusBadRequest)

			return
		}

		if err := s.store.SaveAs(req.Revision, req.Path); err != nil {
			respondConfigErr(w, err, "saving config as", "path", req.Path)

			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

type loadRequest struct {
	Path string `json:"path"`
}

// handleLoad switches to editing another vision.yml, replacing the working
// document (unsaved changes included) and applying it live.
func (s *VisionServer) handleLoad() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req loadRequest
		if !decodeJSON(w, r, &req) {
			return
		}

		if req.Path == "" {
			http.Error(w, "path: required", http.StatusBadRequest)

			return
		}

		if err := s.store.Load(req.Path); err != nil {
			respondConfigErr(w, err, "loading config", "path", req.Path)

			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

// handleReload accepts the current file on disk over the working document:
// both the "load from disk" answer to the external-change prompt and the
// menu's "revert to disk".
func (s *VisionServer) handleReload() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := s.store.Reload(); err != nil {
			respondConfigErr(w, err, "reloading config")

			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

func (s *VisionServer) handleLockCalibration() http.HandlerFunc {
	return s.calibrationHandler(s.store.LockCalibration, "locking calibration")
}

func (s *VisionServer) handleUnlockCalibration() http.HandlerFunc {
	return s.calibrationHandler(s.store.UnlockCalibration, "unlocking calibration")
}

func (s *VisionServer) calibrationHandler(action func(int) (int64, error), logMsg string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(r.PathValue("id"))
		if err != nil {
			http.Error(w, "id: want a camera_id", http.StatusBadRequest)

			return
		}

		rev, err := action(id)
		if err != nil {
			respondConfigErr(w, err, logMsg, "cam", id)

			return
		}

		writeJSON(w, revisionResponse{Revision: rev})
	}
}

// conflictResponse is the body of a 409, so the frontend can tell a stale
// edit (refetch) from a file changed on disk (prompt the user).
type conflictResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

// respondConfigErr maps a config.Store error to an HTTP response: caller
// mistakes are 400 with the error's own message, conflicts 409 with a code,
// anything else a logged 500.
func respondConfigErr(w http.ResponseWriter, err error, logMsg string, logArgs ...any) {
	conflict := func(code string) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)

		if err := json.NewEncoder(w).Encode(conflictResponse{Error: code, Message: err.Error()}); err != nil {
			slog.Error("writing conflict response", "err", err)
		}
	}

	switch {
	case config.IsValidation(err):
		http.Error(w, err.Error(), http.StatusBadRequest)
	case errors.Is(err, config.ErrStale):
		conflict("stale")
	case errors.Is(err, config.ErrDiskChanged):
		conflict("disk_changed")
	case errors.Is(err, config.ErrNoLiveCalibration):
		conflict("no_live_calibration")
	case errors.Is(err, config.ErrUnknownCamera):
		http.Error(w, err.Error(), http.StatusNotFound)
	default:
		slog.Error(logMsg, append(logArgs, "err", err)...)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

// fieldPresets are the rulebook defaults, read live from the same files a
// human would open (geometry-divA.yml, geometry-divB.yml at the repo root) --
// not a copy that could drift from them. Saves refuse to touch these paths.
var fieldPresets = []struct {
	Name string
	Path string
}{
	{Name: "Division A", Path: "geometry-divA.yml"},
	{Name: "Division B", Path: "geometry-divB.yml"},
}

type fieldPresetResponse struct {
	Name               string                       `json:"name"`
	Field              geometry.FieldConfig         `json:"field"`
	OptionalFieldLines geometry.OptionalLinesConfig `json:"optionalFieldLines"`
}

// handleGetFieldPresets serves the rulebook presets for the setup wizard. A
// preset file that can't be read (wrong working directory, for one) is
// skipped with a warning rather than failing the whole request.
func (s *VisionServer) handleGetFieldPresets() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		presets := make([]fieldPresetResponse, 0, len(fieldPresets))

		for _, p := range fieldPresets {
			field, optional, err := geometry.LoadPreset(p.Path)
			if err != nil {
				slog.Warn("skipping unreadable field preset", "name", p.Name, "path", p.Path, "err", err)

				continue
			}

			presets = append(presets, fieldPresetResponse{Name: p.Name, Field: field, OptionalFieldLines: optional})
		}

		writeJSON(w, presets)
	}
}

func decodeJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		http.Error(w, "malformed request body: "+err.Error(), http.StatusBadRequest)

		return false
	}

	return true
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("writing response", "err", err)
	}
}
