package transformer

import (
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/teochenglim/mapwatch/internal/geo"
	"github.com/teochenglim/mapwatch/internal/marker"
)

// amPayload is the Alertmanager webhook v4 payload.
type amPayload struct {
	Version string    `json:"version"`
	Status  string    `json:"status"`
	Alerts  []amAlert `json:"alerts"`
}

type amAlert struct {
	Status      string            `json:"status"`
	Fingerprint string            `json:"fingerprint"`
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
	StartsAt    time.Time         `json:"startsAt"`
	EndsAt      time.Time         `json:"endsAt"`
}

// AlertmanagerTransformer converts Alertmanager webhook payloads into Markers.
type AlertmanagerTransformer struct {
	// Locations maps datacenter/region label values to geohash strings.
	Locations map[string]string
}

// NewAlertmanagerTransformer returns a transformer backed by the given location table.
func NewAlertmanagerTransformer(locations map[string]string) *AlertmanagerTransformer {
	if locations == nil {
		locations = make(map[string]string)
	}
	return &AlertmanagerTransformer{Locations: locations}
}

func (t *AlertmanagerTransformer) Name() string { return "alertmanager" }

// Transform parses an Alertmanager webhook payload and returns one Marker per alert.
// Alerts with unresolvable geo are skipped with a warning log.
func (t *AlertmanagerTransformer) Transform(payload []byte) ([]*marker.Marker, error) {
	var p amPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return nil, fmt.Errorf("alertmanager: unmarshal payload: %w", err)
	}

	markers := make([]*marker.Marker, 0, len(p.Alerts))
	for _, a := range p.Alerts {
		m, err := t.alertToMarker(a)
		if err != nil {
			log.Printf("alertmanager: skipping alert %q: %v", a.Fingerprint, err)
			continue
		}
		markers = append(markers, m)
	}
	return markers, nil
}

func (t *AlertmanagerTransformer) alertToMarker(a amAlert) (*marker.Marker, error) {
	lat, lng, geohashStr, bounds, err := t.resolveGeo(a.Labels)
	if err != nil {
		return nil, err
	}

	m := &marker.Marker{
		ID:          a.Fingerprint,
		Geohash:     geohashStr,
		Lat:         lat,
		Lng:         lng,
		GeoBounds:   bounds,
		Severity:    a.Labels["severity"],
		AlertName:   a.Labels["alertname"],
		Labels:      a.Labels,
		Annotations: a.Annotations,
		StartsAt:    a.StartsAt,
		Source:      "alertmanager",
	}
	return m, nil
}

// resolveGeo applies the priority chain: geohash → lat/lng → datacenter lookup.
func (t *AlertmanagerTransformer) resolveGeo(labels map[string]string) (lat, lng float64, geohashStr string, bounds *geo.GeoBounds, err error) {
	// Priority 1: geohash label
	if h, ok := labels["geohash"]; ok && h != "" {
		center, b, decErr := geo.DecodeGeohash(h)
		if decErr == nil {
			return center.Lat, center.Lng, h, &b, nil
		}
		log.Printf("resolveGeo: invalid geohash %q: %v", h, decErr)
	}

	// Priority 2: lat + lng labels
	if latStr, ok := labels["lat"]; ok {
		if lngStr, ok2 := labels["lng"]; ok2 {
			parsedLat, errLat := strconv.ParseFloat(latStr, 64)
			parsedLng, errLng := strconv.ParseFloat(lngStr, 64)
			if errLat == nil && errLng == nil {
				return parsedLat, parsedLng, "", nil, nil
			}
		}
	}

	// Priority 3: datacenter / region lookup
	for _, key := range []string{"datacenter", "region"} {
		if val, ok := labels[key]; ok && val != "" {
			if h, ok2 := t.Locations[val]; ok2 {
				center, b, decErr := geo.DecodeGeohash(h)
				if decErr == nil {
					return center.Lat, center.Lng, h, &b, nil
				}
			}
		}
	}

	return 0, 0, "", nil, fmt.Errorf("no resolvable geo info in labels %v", labels)
}
