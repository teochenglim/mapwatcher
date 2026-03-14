package server

import (
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/gorilla/websocket"
	kafka "github.com/segmentio/kafka-go"
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

	var kw *kafka.Writer
	if brokers := os.Getenv("REDPANDA_BROKERS"); brokers != "" {
		kw = &kafka.Writer{
			Addr:         kafka.TCP(strings.Split(brokers, ",")...),
			Topic:        "user_taps",
			Balancer:     &kafka.LeastBytes{},
			Async:        true, // fire-and-forget; don't block HTTP handler
		}
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
		modules:         cfg.Modules,
		leaderboardURL:  cfg.LeaderboardURL,
		dataDir:         dataDir,
		upgrader:        upgrader,
		kafkaWriter:     kw,
	}

	// Expire tap markers after 30 s so recomputeOffsets stays fast.
	go func() {
		tick := time.NewTicker(5 * time.Second)
		defer tick.Stop()
		for range tick.C {
			cutoff := time.Now().Add(-30 * time.Second)
			for _, m := range store.All() {
				if m.Source == "tap" && m.UpdatedAt.Before(cutoff) {
					if store.Remove(m.ID) {
						hub.BroadcastRemove(m.ID)
					}
				}
			}
		}
	}()

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// API routes
	r.Get("/api/config", h.GetConfig)
	r.Post("/api/alerts", h.PostAlerts)
	r.Get("/api/markers", h.GetMarkers)
	r.Post("/api/tap", h.PostTap)
	r.Post("/api/markers", h.PostMarkers)
	r.Delete("/api/markers/{id}", h.DeleteMarker)
	r.Get("/api/markers/{id}/details", h.GetMarkerDetails)
	r.Get("/api/markers/{id}/links", h.GetMarkerLinks)
	r.Get("/api/geojson/{name}", h.ServeGeoJSON)
	r.Get("/api/leaderboard", h.GetLeaderboard)
	r.Post("/api/leaderboard/clear", h.PostLeaderboardClear)

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
