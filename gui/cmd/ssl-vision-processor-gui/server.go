package main

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/geometry"
	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/hub"
	"google.golang.org/protobuf/encoding/protojson"
)

type VisionServer struct {
	geometry   *geometry.Geometry
	hub        *hub.Hub
	imgDir     string
	configFile string
}

func NewVisionServer(geom *geometry.Geometry, wsHub *hub.Hub, imgDir, configFile string) http.Handler {
	s := &VisionServer{geometry: geom, hub: wsHub, imgDir: imgDir, configFile: configFile}

	mux := http.NewServeMux()
	s.addRoutes(mux)

	return mux
}

func (s *VisionServer) handleHealth() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]string{"status": "ok"}); err != nil {
			slog.Error("writing health response", "err", err)
		}
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

// fieldConfigResponse is the "virtual field" shape: just the editable
// dimensions and optional-line toggles, not the full wrapper packet (calib,
// source, generated field_lines/arcs) that handleGetGeometry serves. Path is
// which file this came from / would be saved to -- omitted on requests where
// it doesn't apply (PUT), populated on every response.
type fieldConfigResponse struct {
	Path               string                       `json:"path,omitempty"`
	Field              geometry.FieldConfig         `json:"field"`
	OptionalFieldLines geometry.OptionalLinesConfig `json:"optionalFieldLines"`
}

func (s *VisionServer) handleGetFieldConfig() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		field, optional := s.geometry.FieldConfig()

		w.Header().Set("Content-Type", "application/json")

		resp := fieldConfigResponse{Path: s.geometry.Path(), Field: field, OptionalFieldLines: optional}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			slog.Error("writing field config response", "err", err)
		}
	}
}

// respondGeometryErr classifies an error from a geometry package call and
// writes the matching HTTP response: a caller-fixable *ValidationError or
// *ReadOnlyError becomes 400 (the error's own message is safe to return --
// both types exist specifically to describe the caller's mistake back to
// them), anything else is logged and reported as a generic 500. Shared by
// every handler whose geometry call can fail either way.
//
// handlePostLoad deliberately does not use this: every LoadFrom failure is
// about the path the caller gave, not an internal failure on our part, so
// it's unconditionally 400 regardless of error type -- see its own comment.
func respondGeometryErr(w http.ResponseWriter, err error, logMsg string, logArgs ...any) {
	var validation *geometry.ValidationError
	var readOnly *geometry.ReadOnlyError

	switch {
	case errors.As(err, &validation), errors.As(err, &readOnly):
		http.Error(w, err.Error(), http.StatusBadRequest)
	default:
		slog.Error(logMsg, append(logArgs, "err", err)...)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

// handlePutFieldConfig edits the virtual field: dimensions and which optional
// markings exist. Regenerates the derived field lines/arcs and persists to
// geometry.yml -- see geometry.Geometry.UpdateField.
func (s *VisionServer) handlePutFieldConfig() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req fieldConfigResponse
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "malformed request body: "+err.Error(), http.StatusBadRequest)

			return
		}

		if err := s.geometry.UpdateField(req.Field, req.OptionalFieldLines); err != nil {
			respondGeometryErr(w, err, "updating field config")

			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

// saveAsRequest is fieldConfigResponse's shape with Path meaning "save here"
// instead of "loaded from here" -- same fields, different direction, kept as
// a distinct type so the two aren't confused at the call site.
type saveAsRequest = fieldConfigResponse

// handlePostSaveAs writes the given field config to Path and switches
// Geometry to editing that file from now on -- see geometry.Geometry.SaveAs.
func (s *VisionServer) handlePostSaveAs() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req saveAsRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "malformed request body: "+err.Error(), http.StatusBadRequest)

			return
		}

		if req.Path == "" {
			http.Error(w, "path: required", http.StatusBadRequest)

			return
		}

		if err := s.geometry.SaveAs(req.Path, req.Field, req.OptionalFieldLines); err != nil {
			respondGeometryErr(w, err, "saving field config as", "path", req.Path)

			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

type loadRequest struct {
	Path string `json:"path"`
}

// handlePostLoad replaces the live geometry with whatever's in the given
// path and returns it, so the frontend doesn't need a second round trip --
// see geometry.Geometry.LoadFrom. Any failure to load is reported as 400:
// it's about the path the caller gave, not an internal failure on our part.
func (s *VisionServer) handlePostLoad() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req loadRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "malformed request body: "+err.Error(), http.StatusBadRequest)

			return
		}

		if req.Path == "" {
			http.Error(w, "path: required", http.StatusBadRequest)

			return
		}

		if err := s.geometry.LoadFrom(req.Path); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)

			return
		}

		field, optional := s.geometry.FieldConfig()

		w.Header().Set("Content-Type", "application/json")

		resp := fieldConfigResponse{Path: s.geometry.Path(), Field: field, OptionalFieldLines: optional}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			slog.Error("writing load response", "err", err)
		}
	}
}

type lineCornersRequest struct {
	Corners []geometry.Corner `json:"corners"`
	// GoalSideMarker is 1-based, matching the corner picker's on-screen
	// number for whichever marker was chosen as the goal-side corner.
	GoalSideMarker int `json:"goalSideMarker"`
}

// handleGetLineCorners reads back whatever was last written to the
// instance's config.yml (see geometry.ReadLineCorners), so the corner picker
// can restore its markers instead of always starting from the default inset
// rectangle. A file with nothing saved yet -- or that can't be read at all --
// degrades to an empty response rather than an error: not being calibrated
// yet is the normal state, not a failure.
func (s *VisionServer) handleGetLineCorners() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		corners, marker, err := geometry.ReadLineCorners(s.configFile)
		if err != nil {
			slog.Warn("no existing line corners to load", "path", s.configFile, "err", err)

			corners, marker = nil, 0
		}

		w.Header().Set("Content-Type", "application/json")

		resp := lineCornersRequest{Corners: corners, GoalSideMarker: marker}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			slog.Error("writing line corners response", "err", err)
		}
	}
}

// handlePutLineCorners writes the corner picker's calibration hint into the
// instance's own config.yml (geometry.line_corners) -- see
// geometry.WriteLineCorners. This isn't part of the shared field template
// (Geometry/geometry.yml); it's a stand-in for the not-yet-built
// internal/config (see gui/CLAUDE.md's "Not yet built"), kept in
// internal/geometry for now since it's already of immediate use.
func (s *VisionServer) handlePutLineCorners() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req lineCornersRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "malformed request body: "+err.Error(), http.StatusBadRequest)

			return
		}

		if len(req.Corners) != 4 {
			http.Error(w, "corners: exactly 4 required", http.StatusBadRequest)

			return
		}

		// goalSideMarker's own range check lives in WriteLineCorners, not
		// duplicated here -- it returns a *ValidationError for exactly this
		// case, which respondGeometryErr turns back into a 400.
		if err := geometry.WriteLineCorners(s.configFile, req.Corners, req.GoalSideMarker); err != nil {
			respondGeometryErr(w, err, "writing line corners", "path", s.configFile)

			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

// fieldPresets are the rulebook defaults, read live from the same files a
// human would open (geometry-divA.yml, geometry-divB.yml at the repo root) --
// not a copy that could drift from them. See geometry.ReadOnlyFiles: saves
// through handlePutFieldConfig refuse to touch these paths.
var fieldPresets = []struct {
	Name string
	Path string
}{
	{Name: "Division A", Path: "geometry-divA.yml"},
	{Name: "Division B", Path: "geometry-divB.yml"},
}

type fieldPresetResponse struct {
	Name string `json:"name"`
	fieldConfigResponse
}

// handleGetFieldPresets serves the rulebook presets for the "start from a
// preset" feature. A preset file that can't be read (wrong working
// directory, for one) is skipped with a warning rather than failing the
// whole request -- the other preset(s) are still useful.
func (s *VisionServer) handleGetFieldPresets() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		presets := make([]fieldPresetResponse, 0, len(fieldPresets))

		for _, p := range fieldPresets {
			field, optional, err := geometry.LoadPreset(p.Path)
			if err != nil {
				slog.Warn("skipping unreadable field preset", "name", p.Name, "path", p.Path, "err", err)

				continue
			}

			presets = append(presets, fieldPresetResponse{
				Name:                p.Name,
				fieldConfigResponse: fieldConfigResponse{Field: field, OptionalFieldLines: optional},
			})
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(presets); err != nil {
			slog.Error("writing field presets response", "err", err)
		}
	}
}
