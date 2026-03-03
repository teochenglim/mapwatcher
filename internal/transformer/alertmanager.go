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
	// Locations maps label values (datacenter, location, region …) to geohash strings.
	Locations map[string]string
	// GeoPriority is the ordered list of label keys used to resolve an alert's position.
	// Sentinel values: "geohash" and "lat_lng". All other values are label names looked
	// up in Locations. Defaults to ["geohash","lat_lng","datacenter","location","region"].
	GeoPriority []string
}

// NewAlertmanagerTransformer returns a transformer backed by the given location table
// and geo resolution priority list. Pass nil for geoPriority to use the default order.
func NewAlertmanagerTransformer(locations map[string]string, geoPriority []string) *AlertmanagerTransformer {
	if locations == nil {
		locations = make(map[string]string)
	}
	if len(geoPriority) == 0 {
		geoPriority = []string{"geohash", "lat_lng", "datacenter", "location", "region"}
	}
	return &AlertmanagerTransformer{Locations: locations, GeoPriority: geoPriority}
}

func (t *AlertmanagerTransformer) Name() string { return "alertmanager" }

// Transform parses an Alertmanager webhook payload and returns one Marker per alert.
// Alerts with unresolvable geo are skipped with a warning log.
// Implements the Transformer interface (returns only firing markers).
func (t *AlertmanagerTransformer) Transform(payload []byte) ([]*marker.Marker, error) {
	firing, _, err := t.TransformPayload(payload)
	return firing, err
}

// TransformPayload parses an Alertmanager webhook payload and returns separate
// slices for firing markers and resolved fingerprints.
// Use this instead of Transform so that resolved alerts are properly removed.
func (t *AlertmanagerTransformer) TransformPayload(payload []byte) (firing []*marker.Marker, resolvedIDs []string, err error) {
	var p amPayload
	if err = json.Unmarshal(payload, &p); err != nil {
		return nil, nil, fmt.Errorf("alertmanager: unmarshal payload: %w", err)
	}

	log.Printf("alertmanager: webhook received status=%s alerts=%d", p.Status, len(p.Alerts))

	for _, a := range p.Alerts {
		if a.Status == "resolved" {
			log.Printf("alertmanager: resolved fingerprint=%s alertname=%s", a.Fingerprint, a.Labels["alertname"])
			resolvedIDs = append(resolvedIDs, a.Fingerprint)
			continue
		}
		m, merr := t.alertToMarker(a)
		if merr != nil {
			log.Printf("alertmanager: skipping alert fingerprint=%s alertname=%s: %v", a.Fingerprint, a.Labels["alertname"], merr)
			continue
		}
		log.Printf("alertmanager: firing fingerprint=%s alertname=%s geohash=%s severity=%s", m.ID, m.AlertName, m.Geohash, m.Severity)
		firing = append(firing, m)
	}
	return firing, resolvedIDs, nil
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

// resolveGeo resolves an alert's map position by iterating t.GeoPriority in order.
// Each step is either a sentinel ("geohash", "lat_lng") or a label name looked up
// in t.Locations (e.g. "datacenter", "location", "region").
func (t *AlertmanagerTransformer) resolveGeo(labels map[string]string) (lat, lng float64, geohashStr string, bounds *geo.GeoBounds, err error) {
	for _, step := range t.GeoPriority {
		switch step {
		case "geohash":
			if h, ok := labels["geohash"]; ok && h != "" {
				center, b, decErr := geo.DecodeGeohash(h)
				if decErr == nil {
					return center.Lat, center.Lng, h, &b, nil
				}
				log.Printf("resolveGeo: invalid geohash %q: %v", h, decErr)
			}
		case "lat_lng":
			if latStr, ok := labels["lat"]; ok {
				if lngStr, ok2 := labels["lng"]; ok2 {
					parsedLat, errLat := strconv.ParseFloat(latStr, 64)
					parsedLng, errLng := strconv.ParseFloat(lngStr, 64)
					if errLat == nil && errLng == nil {
						return parsedLat, parsedLng, "", nil, nil
					}
				}
			}
		default:
			// Named label lookup: datacenter, location, region, or any custom key.
			if val, ok := labels[step]; ok && val != "" {
				if h, ok2 := t.Locations[val]; ok2 {
					center, b, decErr := geo.DecodeGeohash(h)
					if decErr == nil {
						return center.Lat, center.Lng, h, &b, nil
					}
				}
			}
		}
	}
	return 0, 0, "", nil, fmt.Errorf("no resolvable geo info in labels %v", labels)
}
