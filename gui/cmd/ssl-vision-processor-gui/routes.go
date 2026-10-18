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

// addRoutes registers every route. Those through api.Route are in the OpenAPI
// spec at /api/openapi.json; the rest are being converted.
func (s *VisionServer) addRoutes(mux *http.ServeMux, a *api.API) {
	mux.Handle("GET /api/health", s.handleHealth())
	mux.Handle("GET /api/topics", s.handleGetTopics())
	mux.Handle("GET /api/network", s.handleGetNetwork())
	api.Route(a, "GET /api/alerts", api.Doc{Tags: []string{"Status"}}, s.getAlerts)
	mux.Handle("GET /api/geometry", s.handleGetGeometry())
	mux.Handle("GET /api/geometry/presets", s.handleGetFieldPresets())
	mux.Handle("GET /api/schema", s.handleGetSchema())
	api.Route(a, "GET /api/config", api.Doc{Tags: []string{"Configuration"}}, s.getConfig)
	mux.Handle("PUT /api/config", s.handlePutConfig())
	api.Route(a, "PATCH /api/config", api.Doc{
		Tags:   []string{"Configuration"},
		Errors: []string{httpjson.CodeBadRequest, httpjson.CodeInvalid, httpjson.CodePrecondition, httpjson.CodeStale, httpjson.CodeUnsupportedMedia},
	}, s.patchConfig)
	mux.Handle("GET /api/config/files", s.handleConfigFiles())
	mux.Handle("POST /api/config/save", s.handleSave())
	mux.Handle("POST /api/config/save-as", s.handleSaveAs())
	mux.Handle("POST /api/config/load", s.handleLoad())
	mux.Handle("POST /api/config/reload", s.handleReload())
	mux.Handle("GET /api/config/cameras/{id}/effective", s.handleEffectiveConfig())
	mux.Handle("POST /api/config/cameras/{id}/calibration", s.handleLockCalibration())
	mux.Handle("DELETE /api/config/cameras/{id}/calibration", s.handleUnlockCalibration())
	mux.Handle("GET /api/camera/devices", s.handleCameraDevices())
	mux.Handle("GET /api/snapshots", snapshot.HandleList(s.imgDir))
	mux.Handle("GET /api/snapshot/{camID}/{view}", snapshot.HandleGet(s.imgDir))
	mux.Handle("/api/", httpjson.NotFound())

	mux.Handle("/ws", hub.HandleWebSocket(s.hub, s.origins.Allowed))
	mux.Handle("GET /ws/video/{id}", video.HandleWebSocket(s.video, s.origins.Allowed))

	mux.Handle("/", frontend.HandleFrontend())
}
