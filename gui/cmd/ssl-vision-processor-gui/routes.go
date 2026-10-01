package main

import (
	"net/http"

	"github.com/RoboCup-SSL/ssl-vision-processor/gui/frontend"
	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/hub"
	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/snapshot"
)

func (s *VisionServer) addRoutes(mux *http.ServeMux) {
	mux.Handle("GET /api/health", s.handleHealth())
	mux.Handle("GET /api/geometry", s.handleGetGeometry())
	mux.Handle("GET /api/geometry/field", s.handleGetFieldConfig())
	mux.Handle("PUT /api/geometry/field", s.handlePutFieldConfig())
	mux.Handle("POST /api/geometry/field/save-as", s.handlePostSaveAs())
	mux.Handle("POST /api/geometry/field/load", s.handlePostLoad())
	mux.Handle("GET /api/geometry/presets", s.handleGetFieldPresets())
	mux.Handle("GET /api/config/line-corners", s.handleGetLineCorners())
	mux.Handle("PUT /api/config/line-corners", s.handlePutLineCorners())
	mux.Handle("GET /api/snapshots", snapshot.HandleList(s.imgDir))
	mux.Handle("GET /api/snapshot/{camID}/{view}", snapshot.HandleGet(s.imgDir))
	mux.Handle("/api/", http.NotFoundHandler())

	mux.Handle("/ws", hub.HandleWebSocket(s.hub))

	mux.Handle("/", frontend.HandleFrontend())
}
