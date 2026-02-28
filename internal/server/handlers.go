package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
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
	upgrader        *websocket.Upgrader
}

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
func (h *Handlers) PostAlerts(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 4<<20))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "reading body: " + err.Error()})
		return
	}

	incoming, err := h.amTrans.Transform(body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	// Reconcile firing alerts: add/update present, remove absent.
	added, updated, removed := h.store.Reconcile(incoming)

	for _, id := range added {
		if m := h.store.Get(id); m != nil {
			h.hub.BroadcastAdd(m)
		}
	}
	for _, id := range updated {
		if m := h.store.Get(id); m != nil {
			h.hub.BroadcastUpdate(m)
		}
	}
	for _, id := range removed {
		h.hub.BroadcastRemove(id)
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"added":   len(added),
		"updated": len(updated),
		"removed": len(removed),
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

	added := h.store.Upsert(&m)
	if added {
		h.hub.BroadcastAdd(&m)
	} else {
		h.hub.BroadcastUpdate(&m)
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
	writeJSON(w, http.StatusOK, map[string]string{
		"prometheusUrl": h.promExternalURL,
	})
}

// ServeWS handles GET /ws — WebSocket upgrade.
func (h *Handlers) ServeWS(w http.ResponseWriter, r *http.Request) {
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("ws upgrade: %v", err)
		return
	}
	h.hub.Register(conn)
}
