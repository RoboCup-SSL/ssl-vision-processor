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

// Tags group the routes in the API reference.
var (
	tagStatus  = []string{"Status"}
	tagConfig  = []string{"Configuration"}
	tagField   = []string{"Field"}
	tagHost    = []string{"Host-local"}
	tagUpdates = []string{"Live updates"}
)

// Error codes every route with a JSON body can answer, from decoding it.
var bodyErrors = []string{httpjson.CodeBadRequest, httpjson.CodeUnsupportedMedia}

// addRoutes registers every route. Each is in the OpenAPI spec at
// /api/openapi.json, documented from its handler's types and doc comment.
func (s *VisionServer) addRoutes(mux *http.ServeMux, a *api.API) {
	api.Route(a, "GET /api/health", api.Doc{Tags: tagStatus}, s.health)
	api.Route(a, "GET /api/alerts", api.Doc{Tags: tagStatus}, s.getAlerts)
	api.Route(a, "GET /api/network", api.Doc{Tags: tagStatus}, s.network)

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
	}, s.effectiveConfig)
	api.Route(a, "POST /api/config/cameras/{id}/calibration", api.Doc{
		Tags:   tagConfig,
		Errors: []string{httpjson.CodeBadRequest, httpjson.CodeNotFound, httpjson.CodeNoLiveCalibration},
	}, s.lockCalibration)
	api.Route(a, "DELETE /api/config/cameras/{id}/calibration", api.Doc{
		Tags:   tagConfig,
		Errors: []string{httpjson.CodeBadRequest, httpjson.CodeNotFound},
	}, s.unlockCalibration)
	api.Route(a, "GET /api/config/files", api.Doc{Tags: tagConfig}, s.configFiles)
	api.Route(a, "POST /api/config/save", api.Doc{
		Tags:   tagConfig,
		Errors: append([]string{httpjson.CodeInvalid, httpjson.CodeStale, httpjson.CodeDiskChanged}, bodyErrors...),
	}, s.save)
	api.Route(a, "POST /api/config/save-as", api.Doc{
		Tags:   tagConfig,
		Errors: append([]string{httpjson.CodeInvalid, httpjson.CodeStale}, bodyErrors...),
	}, s.saveAs)
	api.Route(a, "POST /api/config/load", api.Doc{
		Tags:   tagConfig,
		Errors: append([]string{httpjson.CodeInvalid}, bodyErrors...),
	}, s.load)
	api.Route(a, "POST /api/config/reload", api.Doc{
		Tags:   tagConfig,
		Errors: []string{httpjson.CodeInvalid},
	}, s.reload)

	api.Route(a, "GET /api/geometry", api.Doc{Tags: tagField}, s.getGeometry)
	api.Route(a, "GET /api/geometry/presets", api.Doc{Tags: tagField}, s.fieldPresets)

	api.Route(a, "GET /api/camera/devices", api.Doc{Tags: tagHost}, s.cameraDevices)
	api.Route(a, "GET /api/snapshots", api.Doc{Tags: tagHost}, s.snapshots)
	api.Raw[snapshotRequest](a, "GET /api/snapshot/{camID}/{view}", api.Doc{
		Tags:        tagHost,
		Summary:     "Serves a camera's latest debug image for one view",
		Description: "The most recently written of the views GET /api/snapshots lists, as vision_processor wrote it.",
		Errors:      []string{httpjson.CodeNotFound},
	}, snapshot.HandleGet(s.imgDir), api.RawResponse{Status: http.StatusOK, ContentType: "image/*", Description: "The image, PNG or JPEG."})

	api.Route(a, "GET /api/topics", api.Doc{Tags: tagUpdates}, s.topics)
	api.Raw[none](a, "GET /ws", api.Doc{
		Tags:        tagUpdates,
		Summary:     "Opens the live-updates WebSocket",
		Description: "Subscribe to topics as described in Live updates above.",
	}, hub.HandleWebSocket(s.hub, s.origins.Allowed), api.RawResponse{Status: http.StatusSwitchingProtocols, Description: "Upgraded to a WebSocket."})
	api.Raw[videoRequest](a, "GET /ws/video/{id}", api.Doc{
		Tags:    tagUpdates,
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
	// CamID is the camera's camera_id.
	CamID int `path:"camID"`
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
