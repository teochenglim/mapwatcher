package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
	kafka "github.com/segmentio/kafka-go"
	"github.com/teochenglim/mapwatch/internal/config"
	"github.com/teochenglim/mapwatch/internal/geo"
	"github.com/teochenglim/mapwatch/internal/marker"
	"github.com/teochenglim/mapwatch/internal/transformer"
)

// leaderboardClient is a small HTTP client for proxying leaderboard requests.
var leaderboardClient = &http.Client{Timeout: 5 * time.Second}

// Handlers wires the REST API and WebSocket endpoint.
type Handlers struct {
	store           *marker.Store
	hub             *Hub
	amTrans         *transformer.AlertmanagerTransformer
	promProxy       *transformer.PromProxy
	promExternalURL string
	locations       map[string]string      // name → geohash, for /api/config baseline dots
	heatmapRegions  []config.HeatmapRegion // optional region aggregation zones
	layers          config.LayersConfig    // optional overlay auto-enable settings
	modules         config.ModulesConfig   // optional frontend feature modules
	leaderboardURL  string                 // upstream leaderboard API (proxied at /api/leaderboard)
	dataDir         string                 // directory for locally-downloaded GeoJSON files
	upgrader        *websocket.Upgrader
	kafkaWriter     *kafka.Writer          // optional; nil if no broker configured
}

// validGeoJSONName matches safe file identifiers (letters, digits, hyphens, underscores).
var validGeoJSONName = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// writeJSON sends v as a JSON response with the given status code.
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("writeJSON: %v", err)
	}
}

// GetMarkers handles GET /api/markers
func (h *Handlers) GetMarkers(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.store.All())
}

// PostAlerts handles POST /api/alerts (Alertmanager webhook)
//
// Alertmanager sends one webhook per alert group, not one webhook for all alerts.
// Using Reconcile (which removes everything not in the payload) would delete markers
// from other groups on every call.  Instead we upsert firing alerts and explicitly
// remove resolved ones so markers from concurrent groups coexist correctly.
func (h *Handlers) PostAlerts(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 4<<20))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "reading body: " + err.Error()})
		return
	}

	firing, resolvedIDs, err := h.amTrans.TransformPayload(body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	var added, updated, removed int

	// Upsert firing alerts — never touches markers from other groups.
	for _, m := range firing {
		isNew, colocated := h.store.Upsert(m)
		if isNew {
			log.Printf("ws: broadcast add id=%s alertname=%s severity=%s", m.ID, m.AlertName, m.Severity)
			h.hub.BroadcastAdd(m)
			added++
		} else {
			h.hub.BroadcastUpdate(m)
			updated++
		}
		// Broadcast offset updates for co-located markers (their offsets changed).
		for _, co := range colocated {
			h.hub.BroadcastUpdate(co)
		}
	}

	// Remove resolved alerts explicitly.
	for _, id := range resolvedIDs {
		if h.store.Remove(id) {
			log.Printf("ws: broadcast remove id=%s", id)
			h.hub.BroadcastRemove(id)
			removed++
		}
	}

	log.Printf("alertmanager: processed added=%d updated=%d removed=%d", added, updated, removed)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"added":   added,
		"updated": updated,
		"removed": removed,
	})
}

// PostTap handles POST /api/tap — mobile tap events from static/mobile.html.
// Converts a tap into a marker and broadcasts it via WebSocket.
// Body: { username, color, severity, lat, lng, session_id, timestamp }
func (h *Handlers) PostTap(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 64<<10))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	var tap struct {
		Username  string  `json:"username"`
		Color     string  `json:"color"`
		Severity  string  `json:"severity"`
		Lat       float64 `json:"lat"`
		Lng       float64 `json:"lng"`
		SessionID string  `json:"session_id"`
		Timestamp string  `json:"timestamp"`
	}
	if err := json.Unmarshal(body, &tap); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid tap JSON: " + err.Error()})
		return
	}
	if tap.Lat == 0 && tap.Lng == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "lat/lng required"})
		return
	}

	// Map color to standard severity if not provided.
	if tap.Severity == "" {
		colorSev := map[string]string{
			"#ff4444": "critical", "#FF4444": "critical",
			"#ff8844": "warning",  "#FF8844": "warning",
			"#ffcc44": "warning",  "#FFCC44": "warning",
			"#44ff44": "info",     "#44FF44": "info",
			"#4444ff": "info",     "#4444FF": "info",
			"#aa44ff": "info",     "#AA44FF": "info",
		}
		tap.Severity = colorSev[tap.Color]
		if tap.Severity == "" {
			tap.Severity = "info"
		}
	}

	m := &marker.Marker{
		ID:        tap.SessionID + "-" + tap.Timestamp,
		AlertName: "UserTap",
		Severity:  tap.Severity,
		Lat:       tap.Lat,
		Lng:       tap.Lng,
		Source:    "tap",
		Labels: map[string]string{
			"username":       tap.Username,
			"dominant_color": tap.Color,
			"session_id":     tap.SessionID,
		},
		Annotations: map[string]string{
			"summary": fmt.Sprintf("%s tapped at %.4f, %.4f", tap.Username, tap.Lat, tap.Lng),
		},
	}

	isNew, colocated := h.store.Upsert(m)
	if isNew {
		h.hub.BroadcastAdd(m)
	} else {
		h.hub.BroadcastUpdate(m)
	}
	for _, co := range colocated {
		h.hub.BroadcastUpdate(co)
	}

	// Publish raw tap to Kafka so RisingWave can aggregate it for the leaderboard.
	if h.kafkaWriter != nil {
		msg, _ := json.Marshal(tap)
		if err := h.kafkaWriter.WriteMessages(context.Background(),
			kafka.Message{Value: msg},
		); err != nil {
			log.Printf("kafka publish: %v", err)
		}
	}

	writeJSON(w, http.StatusOK, m)
}

// PostMarkers handles POST /api/markers — generic marker upsert.
func (h *Handlers) PostMarkers(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	var m marker.Marker
	if err := json.Unmarshal(body, &m); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid marker JSON: " + err.Error()})
		return
	}
	if m.ID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "marker.id is required"})
		return
	}
	m.Source = "api"

	// If geohash is provided but lat/lng are zero, decode the geohash.
	if m.Geohash != "" && m.Lat == 0 && m.Lng == 0 {
		if center, bounds, err := geo.DecodeGeohash(m.Geohash); err == nil {
			m.Lat = center.Lat
			m.Lng = center.Lng
			m.GeoBounds = &bounds
		}
	}

	isNew, colocated := h.store.Upsert(&m)
	if isNew {
		h.hub.BroadcastAdd(&m)
	} else {
		h.hub.BroadcastUpdate(&m)
	}
	for _, co := range colocated {
		h.hub.BroadcastUpdate(co)
	}
	writeJSON(w, http.StatusOK, &m)
}

// DeleteMarker handles DELETE /api/markers/:id
func (h *Handlers) DeleteMarker(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if h.store.Remove(id) {
		h.hub.BroadcastRemove(id)
		w.WriteHeader(http.StatusNoContent)
	} else {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": fmt.Sprintf("marker %q not found", id)})
	}
}

// GetMarkerDetails handles GET /api/markers/:id/details
func (h *Handlers) GetMarkerDetails(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	m := h.store.Get(id)
	if m == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "marker not found"})
		return
	}

	if h.promProxy == nil {
		writeJSON(w, http.StatusOK, []interface{}{})
		return
	}

	end := time.Now()
	start := end.Add(-1 * time.Hour)

	if s := r.URL.Query().Get("start"); s != "" {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			start = t
		}
	}
	if e := r.URL.Query().Get("end"); e != "" {
		if t, err := time.Parse(time.RFC3339, e); err == nil {
			end = t
		}
	}

	results, err := h.promProxy.QueryMarkerDetails(r.Context(), m, start, end)
	if err != nil {
		var unavailErr *transformer.PrometheusUnavailableError
		if errors.As(err, &unavailErr) {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "Prometheus unavailable"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, results)
}

// GetMarkerLinks handles GET /api/markers/:id/links — returns rendered PromQL links.
func (h *Handlers) GetMarkerLinks(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	m := h.store.Get(id)
	if m == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "marker not found"})
		return
	}
	if h.promProxy == nil {
		writeJSON(w, http.StatusOK, []interface{}{})
		return
	}
	links, err := h.promProxy.RenderLinks(m, h.promExternalURL)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, links)
}

// GetConfig handles GET /api/config — exposes non-sensitive runtime config to the frontend.
func (h *Handlers) GetConfig(w http.ResponseWriter, r *http.Request) {
	type locItem struct {
		Name string  `json:"name"`
		Lat  float64 `json:"lat"`
		Lng  float64 `json:"lng"`
	}
	locs := make([]locItem, 0, len(h.locations))
	for name, hash := range h.locations {
		if center, _, err := geo.DecodeGeohash(hash); err == nil {
			locs = append(locs, locItem{Name: name, Lat: center.Lat, Lng: center.Lng})
		}
	}

	type regionItem struct {
		Name   string        `json:"name"`
		Bounds [2][2]float64 `json:"bounds"`
		Color  string        `json:"color,omitempty"`
	}
	regions := make([]regionItem, 0, len(h.heatmapRegions))
	for _, hr := range h.heatmapRegions {
		regions = append(regions, regionItem{
			Name:   hr.Name,
			Bounds: hr.Bounds,
			Color:  hr.Color,
		})
	}

	// Check which GeoJSON data files are present on disk so the frontend can
	// show or hide layer buttons without doing N parallel HEAD probes.
	layerFiles := map[string]string{
		"division":  "sg-npc-boundary",
		"roads":     "sg-roads",
		"cycling":   "sg-cycling",
		"mrt":       "sg-mrt",
		"busStops":  "sg-bus-stops",
		"busRoutes": "sg-bus-routes",
	}
	availableLayers := make(map[string]bool, len(layerFiles))
	for key, file := range layerFiles {
		_, err := os.Stat(filepath.Join(h.dataDir, file+".geojson"))
		availableLayers[key] = err == nil
	}

	log.Printf("[config] GET /api/config: locations=%d heatmapRegions=%d prometheusUrl=%s",
		len(locs), len(regions), h.promExternalURL)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"prometheusUrl":  h.promExternalURL,
		"locations":      locs,
		"heatmapRegions": regions,
		"layers": map[string]bool{
			"division":  h.layers.Division,
			"roads":     h.layers.Roads,
			"cycling":   h.layers.Cycling,
			"mrt":       h.layers.MRT,
			"busStops":  h.layers.BusStops,
			"busRoutes": h.layers.BusRoutes,
		},
		"availableLayers": availableLayers,
		"modules": map[string]bool{
			"sound":       h.modules.Sound,
			"leaderboard": h.modules.Leaderboard,
			"stats":       h.modules.Stats,
		},
		"leaderboardUrl": h.leaderboardURL,
	})
}

// GetLeaderboard handles GET /api/leaderboard — proxies to the configured upstream.
// Returns 503 when leaderboard_url is not configured.
func (h *Handlers) GetLeaderboard(w http.ResponseWriter, r *http.Request) {
	if h.leaderboardURL == "" {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "leaderboard_url not configured",
		})
		return
	}

	resp, err := leaderboardClient.Get(h.leaderboardURL) //nolint:noctx
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	defer resp.Body.Close()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	if _, err := io.Copy(w, resp.Body); err != nil {
		log.Printf("GetLeaderboard: copy error: %v", err)
	}
}

// PostLeaderboardClear handles POST /api/leaderboard/clear — proxies to upstream clear endpoint.
func (h *Handlers) PostLeaderboardClear(w http.ResponseWriter, r *http.Request) {
	if h.leaderboardURL == "" {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "leaderboard_url not configured",
		})
		return
	}
	clearURL := strings.TrimSuffix(h.leaderboardURL, "/leaderboard") + "/leaderboard/clear"
	resp, err := leaderboardClient.Post(clearURL, "application/json", http.NoBody) //nolint:noctx
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	defer resp.Body.Close()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	if _, err := io.Copy(w, resp.Body); err != nil {
		log.Printf("PostLeaderboardClear: copy error: %v", err)
	}
}

// ServeGeoJSON handles GET /api/geojson/{name} — serves a locally-downloaded GeoJSON file.
// {name} must be alphanumeric (hyphens/underscores allowed); the file is read from
// dataDir/{name}.geojson.  Run `mapwatch download-sg` first to populate the data dir.
func (h *Handlers) ServeGeoJSON(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if !validGeoJSONName.MatchString(name) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid name"})
		return
	}

	path := filepath.Join(h.dataDir, name+".geojson")
	f, err := os.Open(path) //nolint:gosec
	if err != nil {
		if os.IsNotExist(err) {
			writeJSON(w, http.StatusNotFound, map[string]string{
				"error": fmt.Sprintf("GeoJSON %q not found — run: mapwatch download-sg", name),
			})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	defer f.Close()

	w.Header().Set("Content-Type", "application/geo+json")
	if _, err := io.Copy(w, f); err != nil {
		log.Printf("ServeGeoJSON: copy error: %v", err)
	}
}

// ServeWS handles GET /ws — WebSocket upgrade.
func (h *Handlers) ServeWS(w http.ResponseWriter, r *http.Request) {
	log.Printf("ws: upgrade request from %s", r.RemoteAddr)
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("ws: upgrade failed from %s: %v", r.RemoteAddr, err)
		return
	}
	h.hub.Register(conn)
}
