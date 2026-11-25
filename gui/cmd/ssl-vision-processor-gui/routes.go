package main

import (
	"net/http"

	"github.com/RoboCup-SSL/ssl-vision-processor/gui/frontend"
	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/api"
	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/httpjson"
	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/hub"
	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/snapshot"
	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/video"
)

// Tags group the routes in the API reference, in specTags' order.
var (
	tagStatus  = []string{"Status"}
	tagConfig  = []string{"Configuration"}
	tagField   = []string{"Field"}
	tagUpdates = []string{"Live updates"}
	tagHost    = []string{"Host-local"}
)

var specTags = []api.Tag{
	{Name: tagStatus[0], Description: "The host's health, version, sockets, alerts, logs, and the vision_processors it hears."},
	{Name: tagConfig[0], Description: "The shared vision.yml document: read, edit, save, load, and lock calibrations."},
	{Name: tagField[0], Description: "The field geometry the host publishes, and the rulebook presets it starts from."},
	{Name: tagUpdates[0], Description: "What the host is hearing right now, and the WebSocket topics that push it."},
	{Name: tagHost[0], Description: "This machine only: its capture devices, its supervised vision_processors, and debug images on its disk."},
}

// Error codes every route with a JSON body can answer, from decoding it.
var bodyErrors = []string{httpjson.CodeBadRequest, httpjson.CodeUnsupportedMedia}

// addRoutes registers every route. Each is in the OpenAPI spec at
// /api/openapi.json, documented from its handler's types and doc comment.
func (s *VisionServer) addRoutes(mux *http.ServeMux, a *api.API) {
	api.Route(a, "GET /api/health", api.Doc{Tags: tagStatus}, s.getHealth)
	api.Route(a, "GET /api/alerts", api.Doc{Tags: tagStatus}, s.listAlerts)
	api.Route(a, "GET /api/network", api.Doc{Tags: tagStatus}, s.getNetwork)
	api.Route(a, "GET /api/instances", api.Doc{Tags: tagStatus}, s.listInstances)
	api.Route(a, "GET /api/system", api.Doc{Tags: tagStatus}, s.getSystem)
	api.Route(a, "GET /api/logs", api.Doc{Tags: tagStatus, Errors: []string{httpjson.CodeBadRequest}}, s.getLogs)

	api.Route(a, "GET /api/schema", api.Doc{Tags: tagConfig}, s.getSchema)
	api.Route(a, "GET /api/config", api.Doc{Tags: tagConfig}, s.getConfig)
	api.Route(a, "PUT /api/config", api.Doc{
		Tags:   tagConfig,
		Errors: append([]string{httpjson.CodeInvalid, httpjson.CodeStale}, bodyErrors...),
	}, s.putConfig)
	api.Route(a, "PATCH /api/config", api.Doc{
		Tags:   tagConfig,
		Errors: append([]string{httpjson.CodeInvalid, httpjson.CodePrecondition, httpjson.CodeStale}, bodyErrors...),
	}, s.patchConfig)
	api.Route(a, "GET /api/config/cameras/{id}/effective", api.Doc{
		Tags:   tagConfig,
		Errors: []string{httpjson.CodeBadRequest, httpjson.CodeNotFound},
	}, s.getEffectiveConfig)
	api.Route(a, "POST /api/config/cameras/{id}/calibration", api.Doc{
		Tags:   tagConfig,
		Errors: []string{httpjson.CodeBadRequest, httpjson.CodeNotFound, httpjson.CodeNoLiveCalibration},
	}, s.lockCalibration)
	api.Route(a, "DELETE /api/config/cameras/{id}/calibration", api.Doc{
		Tags:   tagConfig,
		Errors: []string{httpjson.CodeBadRequest, httpjson.CodeNotFound},
	}, s.unlockCalibration)
	api.Route(a, "GET /api/config/files", api.Doc{Tags: tagConfig}, s.listConfigFiles)
	api.Route(a, "POST /api/config/save", api.Doc{
		Tags:   tagConfig,
		Errors: append([]string{httpjson.CodeInvalid, httpjson.CodeStale, httpjson.CodeDiskChanged}, bodyErrors...),
	}, s.saveConfig)
	api.Route(a, "POST /api/config/save-as", api.Doc{
		Tags:   tagConfig,
		Errors: append([]string{httpjson.CodeInvalid, httpjson.CodeStale}, bodyErrors...),
	}, s.saveConfigAs)
	api.Route(a, "POST /api/config/load", api.Doc{
		Tags:   tagConfig,
		Errors: append([]string{httpjson.CodeInvalid}, bodyErrors...),
	}, s.loadConfig)
	api.Route(a, "POST /api/config/reload", api.Doc{
		Tags:   tagConfig,
		Errors: []string{httpjson.CodeInvalid},
	}, s.reloadConfig)

	api.Route(a, "GET /api/geometry", api.Doc{Tags: tagField}, s.getGeometry)
	api.Route(a, "GET /api/geometry/presets", api.Doc{Tags: tagField}, s.listFieldPresets)
	api.Route(a, "GET /api/live-field", api.Doc{Tags: tagUpdates}, s.getLiveField)

	api.Route(a, "GET /api/capture-devices", api.Doc{Tags: tagHost}, s.listCaptureDevices)
	api.Route(a, "GET /api/processes", api.Doc{Tags: tagHost}, s.listProcesses)
	api.Route(a, "POST /api/processes/{id}/restart", api.Doc{
		Tags:   tagHost,
		Errors: []string{httpjson.CodeBadRequest, httpjson.CodeNotFound, httpjson.CodeDeviceConflict, httpjson.CodeNoCamera},
	}, s.restartProcess)
	api.Route(a, "GET /api/snapshots", api.Doc{Tags: tagHost}, s.listSnapshots)
	api.Raw[snapshotRequest](a, "GET /api/snapshots/{id}/{view}", api.Doc{
		Tags:        tagHost,
		ID:          "getSnapshot",
		Summary:     "Serves a camera's latest debug image for one view",
		Description: "The most recently written of the views GET /api/snapshots lists, as vision_processor wrote it.",
		Errors:      []string{httpjson.CodeNotFound},
	}, snapshot.HandleGet(s.imgDir), api.RawResponse{Status: http.StatusOK, ContentType: "image/*", Description: "The image, PNG or JPEG."})

	api.Route(a, "GET /api/topics", api.Doc{Tags: tagUpdates}, s.listTopics)
	api.Raw[none](a, "GET /ws", api.Doc{
		Tags:        tagUpdates,
		ID:          "openUpdates",
		Summary:     "Opens the live-updates WebSocket",
		Description: "Subscribe to topics as described in Live updates above.",
	}, hub.HandleWebSocket(s.hub, s.origins.Allowed), api.RawResponse{Status: http.StatusSwitchingProtocols, Description: "Upgraded to a WebSocket."})
	api.Raw[videoRequest](a, "GET /ws/video/{id}", api.Doc{
		Tags:    tagUpdates,
		ID:      "openVideo",
		Summary: "Opens a camera's live video WebSocket",
		Description: "JSON status (once a second) and format (codec and size) messages, and binary fragmented MP4 " +
			"segments ready for a MediaSource SourceBuffer in sequence mode. A new viewer starts at the last keyframe.",
		Errors: []string{httpjson.CodeBadRequest, httpjson.CodeNotFound},
	}, video.HandleWebSocket(s.video, s.origins.Allowed), api.RawResponse{Status: http.StatusSwitchingProtocols, Description: "Upgraded to a WebSocket."})

	mux.Handle("/api/", httpjson.NotFound())
	mux.Handle("/", frontend.HandleFrontend())
}

// snapshotRequest names a debug image.
type snapshotRequest struct {
	// ID is the camera's camera_id.
	ID int `path:"id"`
	// View is the image's view, as GET /api/snapshots lists it, e.g. raw or
	// geomcalib_input.
	View string `path:"view"`
}

// videoRequest picks a camera's stream and how much of it to send.
type videoRequest struct {
	// ID is the camera's camera_id.
	ID int `path:"id"`
	// Mode is full (every frame, the default) or keyframes (about one a
	// second, for grids).
	Mode string `query:"mode" enum:"full,keyframes"`
}
