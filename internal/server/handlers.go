package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
	"github.com/teochenglim/mapwatch/internal/config"
	"github.com/teochenglim/mapwatch/internal/geo"
	"github.com/teochenglim/mapwatch/internal/marker"
	"github.com/teochenglim/mapwatch/internal/transformer"
)

// Handlers wires the REST API and WebSocket endpoint.
type Handlers struct {
	store           *marker.Store
	hub             *Hub
	amTrans         *transformer.AlertmanagerTransformer
	promProxy       *transformer.PromProxy
	promExternalURL string
	locations       map[string]string      // name → geohash, for /api/config baseline dots
	heatmapRegions  []config.HeatmapRegion // optional region aggregation zones
	dataDir         string                 // directory for locally-downloaded GeoJSON files
	upgrader        *websocket.Upgrader
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

	log.Printf("[config] GET /api/config: locations=%d heatmapRegions=%d prometheusUrl=%s",
		len(locs), len(regions), h.promExternalURL)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"prometheusUrl":  h.promExternalURL,
		"locations":      locs,
		"heatmapRegions": regions,
	})
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
