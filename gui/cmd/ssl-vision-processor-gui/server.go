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
	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/snapshot"
	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/supervisor"
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
	procs    *supervisor.Supervisor // nil when not supervising
	imgDir   string
}

func NewVisionServer(geom *geometry.Geometry, store *config.Store, status StatusSource, policy *origins.Policy, wsHub *hub.Hub, videos *video.Manager, procs *supervisor.Supervisor, imgDir string) http.Handler {
	s := &VisionServer{geometry: geom, store: store, status: status, origins: policy, hub: wsHub, video: videos, procs: procs, imgDir: imgDir}

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

// none is a route's input when it takes nothing.
type none = struct{}

// healthResponse says the host is up.
type healthResponse struct {
	// Status is always "ok".
	Status string `json:"status"`
}

// health reports that the host is up and serving.
func (s *VisionServer) health(*http.Request, none) (healthResponse, error) {
	return healthResponse{Status: "ok"}, nil
}

// topics lists the WebSocket topics /ws accepts.
func (s *VisionServer) topics(*http.Request, none) ([]string, error) {
	return s.hub.Topics(), nil
}

// network reports each multicast socket's address and what it hears:
// network.state on demand.
func (s *VisionServer) network(*http.Request, none) (NetworkState, error) {
	return s.status.Network(), nil
}

// getAlerts lists every problem the host sees right now, most serious first:
// alerts.state on demand.
func (s *VisionServer) getAlerts(*http.Request, none) ([]alerts.Alert, error) {
	return s.status.Alerts(), nil
}

// wrapperPacket is an SSL_WrapperPacket as canonical protojson.
type wrapperPacket []byte

func (p wrapperPacket) MarshalJSON() ([]byte, error) { return p, nil }

// getGeometry serves the wrapper packet the host publishes: the field and
// every camera's calibration, as canonical protojson, the encoding the rest of
// the league already reads.
func (s *VisionServer) getGeometry(*http.Request, none) (wrapperPacket, error) {
	return protojson.Marshal(s.geometry.Snapshot())
}

// getSchema describes every vision_processor setting: type, fallback, bounds,
// and when a change takes effect.
func (s *VisionServer) getSchema(*http.Request, none) (config.Schema, error) {
	return config.CurrentSchema(), nil
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
func (s *VisionServer) getConfig(*http.Request, none) (configResponse, error) {
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

func (r revision) String() string { return `"` + strconv.FormatInt(int64(r), 10) + `"` }

// revisionResponse answers a config write with the revision it left the
// working document at.
type revisionResponse struct {
	ETag     revision `header:"ETag" json:"-"`
	Revision int64    `json:"revision"`
}

func revisionOf(rev int64) revisionResponse {
	return revisionResponse{ETag: revision(rev), Revision: rev}
}

// putConfigRequest is a whole working document and the revision it's based
// on.
type putConfigRequest struct {
	// Revision is the revision the document is based on; a stale one is a 409.
	Revision int64           `json:"revision"`
	Document config.Document `json:"document"`
}

// putConfig replaces the working document and applies it live. Nothing is
// written to vision.yml until a save.
func (s *VisionServer) putConfig(_ *http.Request, req putConfigRequest) (revisionResponse, error) {
	rev, err := s.store.Update(req.Revision, req.Document)

	return revisionOf(rev), err
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

// patchConfig merge-patches the working document and applies it live. It
// follows RFC 7396, for clients that change a few keys rather than resending
// the whole document. A stale If-Match is a 409, like PUT's stale revision.
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

	return revisionOf(next), err
}

// configFiles lists the YAML files beside the current vision.yml, for load
// and save-as.
func (s *VisionServer) configFiles(*http.Request, none) ([]config.File, error) {
	return s.store.Files()
}

// saveRequest saves the working document at a revision.
type saveRequest struct {
	// Revision must be the current one; a stale one is a 409.
	Revision int64 `json:"revision"`
	// Force overwrites vision.yml even if it changed on disk since it was
	// loaded: the "overwrite disk" answer to the external-change prompt.
	Force bool `json:"force"`
}

// save writes the working document to vision.yml. A 409 disk_changed means
// the file was edited elsewhere since it was loaded (config.state's external
// describes how); force overwrites it anyway.
func (s *VisionServer) save(_ *http.Request, req saveRequest) (revisionResponse, error) {
	if err := s.store.Save(req.Revision, req.Force); err != nil {
		return revisionResponse{}, err
	}

	return revisionOf(s.store.Revision()), nil
}

// saveAsRequest saves the working document to another file.
type saveAsRequest struct {
	// Revision must be the current one; a stale one is a 409.
	Revision int64 `json:"revision"`
	// Path is the file to write, on the host.
	Path string `json:"path"`
}

// saveAs writes the working document to another path. The host loads from,
// saves to, and watches that file from then on.
func (s *VisionServer) saveAs(_ *http.Request, req saveAsRequest) (revisionResponse, error) {
	if req.Path == "" {
		return revisionResponse{}, &httpjson.Problem{Code: httpjson.CodeInvalid, Message: "path: required", Issues: []string{"path: required"}}
	}

	if err := s.store.SaveAs(req.Revision, req.Path); err != nil {
		return revisionResponse{}, err
	}

	return revisionOf(s.store.Revision()), nil
}

// loadRequest names a vision.yml to load.
type loadRequest struct {
	// Path is the file to load, on the host.
	Path string `json:"path"`
}

// load switches to editing another vision.yml. It replaces the working
// document, unsaved changes included, and applies it live.
func (s *VisionServer) load(_ *http.Request, req loadRequest) (revisionResponse, error) {
	if req.Path == "" {
		return revisionResponse{}, &httpjson.Problem{Code: httpjson.CodeInvalid, Message: "path: required", Issues: []string{"path: required"}}
	}

	if err := s.store.Load(req.Path); err != nil {
		return revisionResponse{}, err
	}

	return revisionOf(s.store.Revision()), nil
}

// reload accepts vision.yml as it is on disk. It discards unsaved changes:
// the "load from disk" answer to the external-change prompt, and "revert to
// disk".
func (s *VisionServer) reload(*http.Request, none) (revisionResponse, error) {
	if err := s.store.Reload(); err != nil {
		return revisionResponse{}, err
	}

	return revisionOf(s.store.Revision()), nil
}

// cameraRequest names a camera by camera_id.
type cameraRequest struct {
	// ID is the camera's camera_id.
	ID int `path:"id"`
}

// effectiveConfig lists every setting of one camera as its vision_processor
// runs it. Each value says where it comes from: the camera, the shared
// defaults, the host, or vision_processor's fallback.
func (s *VisionServer) effectiveConfig(_ *http.Request, in cameraRequest) (config.EffectiveConfig, error) {
	doc, _ := s.store.Working()

	return doc.EffectiveConfig(in.ID)
}

// lockCalibration locks the camera's latest live calibration. It's stored in
// the working document and published in place of whatever the camera sends;
// once saved, it survives restarts. A 409 no_live_calibration means none has
// been received yet.
func (s *VisionServer) lockCalibration(_ *http.Request, in cameraRequest) (revisionResponse, error) {
	rev, err := s.store.LockCalibration(in.ID)

	return revisionOf(rev), err
}

// unlockCalibration removes the camera's locked calibration. The host stops
// publishing any calibration for it, so its vision_processor calibrates again
// the next time it starts.
func (s *VisionServer) unlockCalibration(_ *http.Request, in cameraRequest) (revisionResponse, error) {
	rev, err := s.store.UnlockCalibration(in.ID)

	return revisionOf(rev), err
}

// cameraDevices lists this host's capture devices: candidates for a camera's
// camera.path, stable by-id and by-path links first. Empty on a machine
// without Video4Linux.
func (s *VisionServer) cameraDevices(*http.Request, none) ([]v4l.Device, error) {
	devices := v4l.List(v4l.System)
	if devices == nil {
		devices = []v4l.Device{}
	}

	return devices, nil
}

// snapshots lists the debug images on this host, by camera and view.
func (s *VisionServer) snapshots(*http.Request, none) ([]snapshot.Entry, error) {
	return snapshot.List(s.imgDir)
}

// processes lists the vision_processors the host runs itself (-vp), by
// camera_id: processes.state on demand. Empty when it runs none.
func (s *VisionServer) processes(*http.Request, none) ([]supervisor.Status, error) {
	return s.procs.Statuses(), nil
}

// restartProcess restarts the camera's vision_processor now, rather than
// waiting for a startup-only setting to change. A 404 not_found means the
// host doesn't run that camera's vision_processor.
func (s *VisionServer) restartProcess(_ *http.Request, in cameraRequest) ([]supervisor.Status, error) {
	if err := s.procs.Restart(in.ID); errors.Is(err, supervisor.ErrNotSupervised) {
		return nil, httpjson.Errorf(httpjson.CodeNotFound, "the host doesn't run camera %d's vision_processor; start the host with -vp", in.ID)
	} else if err != nil {
		return nil, err
	}

	return s.procs.Statuses(), nil
}

// respondConfigErr maps a handler's error to an httpjson error: one it
// returned as a Problem as is, caller mistakes 400 with each problem listed,
// conflicts 409 with a code, anything else a logged 500.
func respondConfigErr(w http.ResponseWriter, err error, logMsg string, logArgs ...any) {
	conflict := func(code string) {
		httpjson.Error(w, http.StatusConflict, code, err.Error())
	}

	var problem *httpjson.Problem

	switch {
	case errors.As(err, &problem):
		httpjson.Error(w, httpjson.StatusOf(problem.Code), problem.Code, problem.Message, problem.Issues...)
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

// fieldPresetResponse is one rulebook field.
type fieldPresetResponse struct {
	// Name is the division, e.g. "Division B".
	Name               string                       `json:"name"`
	Field              geometry.FieldConfig         `json:"field"`
	OptionalFieldLines geometry.OptionalLinesConfig `json:"optionalFieldLines"`
}

// fieldPresets lists the rulebook field dimensions, Division A and B. A
// preset file the host can't read (from the wrong working directory, for one)
// is left out rather than failing the request.
func (s *VisionServer) fieldPresets(*http.Request, none) ([]fieldPresetResponse, error) {
	presets := make([]fieldPresetResponse, 0, len(fieldPresets))

	for _, p := range fieldPresets {
		field, optional, err := geometry.LoadPreset(p.Path)
		if err != nil {
			slog.Warn("skipping unreadable field preset", "name", p.Name, "path", p.Path, "err", err)

			continue
		}

		presets = append(presets, fieldPresetResponse{Name: p.Name, Field: field, OptionalFieldLines: optional})
	}

	return presets, nil
}
