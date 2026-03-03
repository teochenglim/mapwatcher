package server

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/gorilla/websocket"
	"github.com/teochenglim/mapwatch/internal/config"
	"github.com/teochenglim/mapwatch/internal/marker"
	"github.com/teochenglim/mapwatch/internal/transformer"
)

// New builds and returns the HTTP server, wiring all dependencies.
// dataDir is the directory from which locally-downloaded GeoJSON files are served
// (e.g. ./data); pass an empty string to disable GeoJSON serving.
func New(cfg *config.Config, staticFS http.FileSystem, dataDir string) *http.Server {
	store := marker.NewStore(cfg.Spread.Radius)
	hub := NewHub(store)
	amTrans := transformer.NewAlertmanagerTransformer(cfg.Locations, cfg.GeoPriority)

	var promProxy *transformer.PromProxy
	if cfg.Prometheus.URL != "" {
		promProxy = transformer.NewPromProxy(&cfg.Prometheus, cfg.QueryTemplates)
	}

	upgrader := &websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}

	h := &Handlers{
		store:           store,
		hub:             hub,
		amTrans:         amTrans,
		promProxy:       promProxy,
		promExternalURL: cfg.Prometheus.ExternalURL,
		locations:       cfg.Locations,
		heatmapRegions:  cfg.Heatmap.Regions,
		layers:          cfg.Layers,
		dataDir:         dataDir,
		upgrader:        upgrader,
	}

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// API routes
	r.Get("/api/config", h.GetConfig)
	r.Post("/api/alerts", h.PostAlerts)
	r.Get("/api/markers", h.GetMarkers)
	r.Post("/api/markers", h.PostMarkers)
	r.Delete("/api/markers/{id}", h.DeleteMarker)
	r.Get("/api/markers/{id}/details", h.GetMarkerDetails)
	r.Get("/api/markers/{id}/links", h.GetMarkerLinks)
	r.Get("/api/geojson/{name}", h.ServeGeoJSON)

	// WebSocket
	r.Get("/ws", h.ServeWS)

	// Static frontend
	if staticFS != nil {
		r.Handle("/*", http.FileServer(staticFS))
	}

	return &http.Server{
		Addr:    cfg.Server.Addr,
		Handler: r,
	}
}
