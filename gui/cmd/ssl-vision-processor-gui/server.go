package main

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/alerts"
	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/config"
	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/geometry"
	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/httpjson"
	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/hub"
	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/origins"
	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/v4l"
	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/video"
	"google.golang.org/protobuf/encoding/protojson"
)

type VisionServer struct {
	geometry *geometry.Geometry
	store    *config.Store
	status   StatusSource
	origins  *origins.Policy
	hub      *hub.Hub
	video    *video.Manager
	imgDir   string
}

func NewVisionServer(geom *geometry.Geometry, store *config.Store, status StatusSource, policy *origins.Policy, wsHub *hub.Hub, videos *video.Manager, imgDir string) http.Handler {
	s := &VisionServer{geometry: geom, store: store, status: status, origins: policy, hub: wsHub, video: videos, imgDir: imgDir}

	mux := http.NewServeMux()
	a := newAPI(mux)
	s.addRoutes(mux, a)
	a.ServeDocs()

	// A browser page from an untrusted origin can't change state; a trusted
	// one also gets CORS headers. A request must name this host by one of its
	// own names, against DNS rebinding.
	return policy.Handler(mux,
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			httpjson.Error(w, http.StatusForbidden, httpjson.CodeCrossOrigin, "cross-origin request refused; see -allowOrigin")
		}),
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			httpjson.Error(w, http.StatusMisdirectedRequest, httpjson.CodeUnknownHost, "this host doesn't answer to "+r.Host+"; add it with -allowHost")
		}))
}

// handleCameraDevices lists this host's capture devices, candidates for a
// camera's camera.path. Empty, not an error, on a machine without
// Video4Linux.
func (s *VisionServer) handleCameraDevices() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		devices := v4l.List(v4l.System)
		if devices == nil {
			devices = []v4l.Device{}
		}

		httpjson.Write(w, http.StatusOK, devices)
	}
}

// handleGetTopics lists the WebSocket topics /ws accepts.
func (s *VisionServer) handleGetTopics() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		httpjson.Write(w, http.StatusOK, s.hub.Topics())
	}
}

// handleGetNetwork is network.state on demand: what each socket hears.
func (s *VisionServer) handleGetNetwork() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		httpjson.Write(w, http.StatusOK, s.status.Network())
	}
}

// getAlerts lists every problem the host sees right now, most serious first:
// alerts.state on demand.
func (s *VisionServer) getAlerts(_ *http.Request, _ struct{}) ([]alerts.Alert, error) {
	return s.status.Alerts(), nil
}

// handleHealth reports that the host is up and serving.
func (s *VisionServer) handleHealth() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		httpjson.Write(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}

// handleGetGeometry serves the current wrapper packet as canonical protojson,
// matching the wire encoding the rest of the league already expects.
func (s *VisionServer) handleGetGeometry() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data, err := protojson.Marshal(s.geometry.Snapshot())
		if err != nil {
			slog.Error("marshalling geometry", "err", err)
			httpjson.Error(w, http.StatusInternalServerError, httpjson.CodeInternal, "internal error")

			return
		}

		w.Header().Set("Content-Type", "application/json")

		if _, err := w.Write(data); err != nil {
			slog.Error("writing geometry response", "err", err)
		}
	}
}

// handleGetSchema describes every vision_processor setting: type, fallback,
// bounds, and when a change takes effect.
func (s *VisionServer) handleGetSchema() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		httpjson.Write(w, http.StatusOK, config.CurrentSchema())
	}
}

// configResponse is the working document plus the store's state.
type configResponse struct {
	// ETag is the document's revision, for PATCH's If-Match.
	ETag     revision        `header:"ETag" json:"-"`
	Document config.Document `json:"document"`
	State    config.State    `json:"state"`
}

// getConfig serves the working document and the store's state. Its revision
// is also the ETag, for PATCH's If-Match.
func (s *VisionServer) getConfig(_ *http.Request, _ struct{}) (configResponse, error) {
	doc, _ := s.store.Working()
	state := s.store.State()

	return configResponse{ETag: revision(state.Revision), Document: doc, State: state}, nil
}

// revision is a document revision as an ETag or If-Match value: "12".
type revision int64

func (r *revision) UnmarshalText(text []byte) error {
	n, err := strconv.ParseInt(strings.Trim(string(text), `"`), 10, 64)
	if err != nil {
		return errors.New(`want a revision such as "12"`)
	}

	*r = revision(n)

	return nil
}

func (r revision) String() string { return revisionETag(int64(r)) }

func revisionETag(rev int64) string {
	return `"` + strconv.FormatInt(rev, 10) + `"`
}

// writeRevision answers a config write with the revision it left the working
// document at, in the body and as the ETag.
func writeRevision(w http.ResponseWriter, rev int64) {
	w.Header().Set("ETag", revisionETag(rev))
	httpjson.Write(w, http.StatusOK, revisionResponse{Revision: rev})
}

// revisionResponse answers a config write with the revision it left the
// working document at.
type revisionResponse struct {
	ETag     revision `header:"ETag" json:"-"`
	Revision int64    `json:"revision"`
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
		if !httpjson.Decode(w, r, &req) {
			return
		}

		rev, err := s.store.Update(req.Revision, req.Document)
		if err != nil {
			respondConfigErr(w, err, "updating config")

			return
		}

		writeRevision(w, rev)
	}
}

// patchConfigRequest is a merge patch and the revision it's based on.
type patchConfigRequest struct {
	// IfMatch is the revision the patch is based on, as GET /api/config's ETag
	// gives it.
	IfMatch revision `header:"If-Match" required:"true"`
	// Patch is merged into the working document's JSON: objects merge, null
	// removes a key, arrays (cameras) are replaced whole.
	Patch map[string]any `body:"application/merge-patch+json,application/json"`
}

// patchConfig applies a JSON Merge Patch (RFC 7396) to the working document
// and applies it live, for clients that change a few keys rather than
// resending the whole document. A stale If-Match is a 409, like PUT's stale
// revision.
func (s *VisionServer) patchConfig(_ *http.Request, in patchConfigRequest) (revisionResponse, error) {
	doc, current := s.store.Working()
	if int64(in.IfMatch) != current {
		return revisionResponse{}, config.ErrStale
	}

	doc, err := config.MergePatch(doc, in.Patch)
	if err != nil {
		return revisionResponse{}, err
	}

	next, err := s.store.Update(current, doc)
	if err != nil {
		return revisionResponse{}, err
	}

	return revisionResponse{ETag: revision(next), Revision: next}, nil
}

// handleConfigFiles lists the YAML files beside the current vision.yml, for
// load and save-as.
func (s *VisionServer) handleConfigFiles() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		files, err := s.store.Files()
		if err != nil {
			respondConfigErr(w, err, "listing config files")

			return
		}

		httpjson.Write(w, http.StatusOK, files)
	}
}

type saveRequest struct {
	Revision int64 `json:"revision"`
	// Force overwrites vision.yml even if it changed on disk since it was
	// loaded -- the "overwrite disk" answer to the external-change prompt.
	Force bool `json:"force"`
}

// handleSave writes the working document to vision.yml. A 409 disk_changed
// means the file was edited elsewhere since it was loaded (config.state's
// external describes how); force overwrites it anyway.
func (s *VisionServer) handleSave() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req saveRequest
		if !httpjson.Decode(w, r, &req) {
			return
		}

		if err := s.store.Save(req.Revision, req.Force); err != nil {
			respondConfigErr(w, err, "saving config")

			return
		}

		writeRevision(w, s.store.Revision())
	}
}

type saveAsRequest struct {
	Revision int64  `json:"revision"`
	Path     string `json:"path"`
}

// handleSaveAs writes the working document to another path, and loads from,
// saves to, and watches that file from then on.
func (s *VisionServer) handleSaveAs() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req saveAsRequest
		if !httpjson.Decode(w, r, &req) {
			return
		}

		if req.Path == "" {
			httpjson.Error(w, http.StatusBadRequest, httpjson.CodeInvalid, "path: required", "path: required")

			return
		}

		if err := s.store.SaveAs(req.Revision, req.Path); err != nil {
			respondConfigErr(w, err, "saving config as", "path", req.Path)

			return
		}

		writeRevision(w, s.store.Revision())
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
		if !httpjson.Decode(w, r, &req) {
			return
		}

		if req.Path == "" {
			httpjson.Error(w, http.StatusBadRequest, httpjson.CodeInvalid, "path: required", "path: required")

			return
		}

		if err := s.store.Load(req.Path); err != nil {
			respondConfigErr(w, err, "loading config", "path", req.Path)

			return
		}

		writeRevision(w, s.store.Revision())
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

		writeRevision(w, s.store.Revision())
	}
}

// handleEffectiveConfig is every setting of one camera as its
// vision_processor runs it, and where each value comes from.
func (s *VisionServer) handleEffectiveConfig() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(r.PathValue("id"))
		if err != nil {
			httpjson.Error(w, http.StatusBadRequest, httpjson.CodeBadRequest, "id: want a camera_id")

			return
		}

		doc, _ := s.store.Working()

		eff, err := doc.EffectiveConfig(id)
		if err != nil {
			respondConfigErr(w, err, "resolving effective config", "cam", id)

			return
		}

		httpjson.Write(w, http.StatusOK, eff)
	}
}

// handleLockCalibration stores the camera's latest live calibration in the
// working document, so it is published in place of whatever the camera sends
// and, once saved, survives restarts. A 409 no_live_calibration means none
// has been received yet.
func (s *VisionServer) handleLockCalibration() http.HandlerFunc {
	return s.calibrationHandler(s.store.LockCalibration, "locking calibration")
}

// handleUnlockCalibration removes the camera's locked calibration and stops
// publishing any calibration for it, so its vision_processor calibrates
// again the next time it starts.
func (s *VisionServer) handleUnlockCalibration() http.HandlerFunc {
	return s.calibrationHandler(s.store.UnlockCalibration, "unlocking calibration")
}

func (s *VisionServer) calibrationHandler(action func(int) (int64, error), logMsg string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(r.PathValue("id"))
		if err != nil {
			httpjson.Error(w, http.StatusBadRequest, httpjson.CodeBadRequest, "id: want a camera_id")

			return
		}

		rev, err := action(id)
		if err != nil {
			respondConfigErr(w, err, logMsg, "cam", id)

			return
		}

		writeRevision(w, rev)
	}
}

// respondConfigErr maps a config.Store error to an httpjson error: caller
// mistakes are 400 with each problem listed, conflicts 409 with a code,
// anything else a logged 500.
func respondConfigErr(w http.ResponseWriter, err error, logMsg string, logArgs ...any) {
	conflict := func(code string) {
		httpjson.Error(w, http.StatusConflict, code, err.Error())
	}

	switch {
	case config.IsValidation(err):
		httpjson.Error(w, http.StatusBadRequest, httpjson.CodeInvalid, err.Error(), config.Issues(err)...)
	case errors.Is(err, config.ErrStale):
		conflict(httpjson.CodeStale)
	case errors.Is(err, config.ErrDiskChanged):
		conflict(httpjson.CodeDiskChanged)
	case errors.Is(err, config.ErrNoLiveCalibration):
		conflict(httpjson.CodeNoLiveCalibration)
	case errors.Is(err, config.ErrUnknownCamera):
		httpjson.Error(w, http.StatusNotFound, httpjson.CodeNotFound, err.Error())
	default:
		slog.Error(logMsg, append(logArgs, "err", err)...)
		httpjson.Error(w, http.StatusInternalServerError, httpjson.CodeInternal, "internal error")
	}
}

// fieldPresets are the rulebook defaults, read live from the read-only
// reference files in the repo's config/ directory, not a copy that could
// drift from them. Saves refuse to touch these paths.
var fieldPresets = []struct {
	Name string
	Path string
}{
	{Name: "Division A", Path: "config/legacy/geometry-divA.yml"},
	{Name: "Division B", Path: "config/legacy/geometry-divB.yml"},
}

type fieldPresetResponse struct {
	Name               string                       `json:"name"`
	Field              geometry.FieldConfig         `json:"field"`
	OptionalFieldLines geometry.OptionalLinesConfig `json:"optionalFieldLines"`
}

// handleGetFieldPresets serves the rulebook field dimensions (Division A and
// B). A preset file that can't be read (wrong working directory, for one) is
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

		httpjson.Write(w, http.StatusOK, presets)
	}
}
