package geo

import (
	"encoding/json"
	"fmt"
	"os"
)

// RegionBounds maps ISO country codes / region names to bounding boxes.
// Extend this map or load from config as needed.
// RegionBounds maps region codes to precise bounding boxes [minLng, minLat, maxLng, maxLat].
// Used by both the `slice` CLI command and the frontend default view.
var RegionBounds = map[string][4]float64{
	"SG": {103.605, 1.159, 104.088, 1.482}, // Singapore main island + southern islands
	"MY": {99.6, 0.85, 119.3, 7.4},
	"ID": {94.9, -11.1, 141.0, 6.1},
	"TH": {97.3, 5.6, 105.7, 20.5},
	"US": {-125.0, 24.0, -66.0, 50.0},
	"EU": {-25.0, 34.0, 45.0, 72.0},
	"JP": {122.0, 24.0, 154.0, 46.0},
	"AU": {113.0, -44.0, 154.0, -10.0},
	"CN": {73.5, 18.2, 135.1, 53.6},
	"IN": {68.1, 6.7, 97.4, 35.7},
}

type geoJSONFeatureCollection struct {
	Type     string        `json:"type"`
	Features []interface{} `json:"features"`
}

type geoJSONFeature struct {
	Type       string                 `json:"type"`
	Properties map[string]interface{} `json:"properties"`
	Geometry   geoJSONGeometry        `json:"geometry"`
}

type geoJSONGeometry struct {
	Type        string      `json:"type"`
	Coordinates interface{} `json:"coordinates"`
}

// SliceGeoJSON reads src GeoJSON, filters features that intersect the bounding
// box for the given region code, and writes the result to dst.
func SliceGeoJSON(src, dst, region string) error {
	bounds, ok := RegionBounds[region]
	if !ok {
		return fmt.Errorf("unknown region %q — supported: SG, MY, ID, TH, US, EU, JP, AU, CN, IN", region)
	}
	minLng, minLat, maxLng, maxLat := bounds[0], bounds[1], bounds[2], bounds[3]

	data, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("reading GeoJSON: %w", err)
	}

	var fc geoJSONFeatureCollection
	if err := json.Unmarshal(data, &fc); err != nil {
		return fmt.Errorf("parsing GeoJSON: %w", err)
	}

	var filtered []interface{}
	for _, raw := range fc.Features {
		b, _ := json.Marshal(raw)
		var f geoJSONFeature
		if err := json.Unmarshal(b, &f); err != nil {
			continue
		}
		if featureIntersectsBounds(f.Geometry, minLng, minLat, maxLng, maxLat) {
			filtered = append(filtered, raw)
		}
	}

	out := geoJSONFeatureCollection{Type: "FeatureCollection", Features: filtered}
	outData, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling output: %w", err)
	}

	if err := os.WriteFile(dst, outData, 0o644); err != nil {
		return fmt.Errorf("writing output: %w", err)
	}

	return nil
}

// featureIntersectsBounds checks whether a geometry has any coordinate within bounds.
// This is a simple point-in-box check on coordinates — sufficient for country clipping.
func featureIntersectsBounds(g geoJSONGeometry, minLng, minLat, maxLng, maxLat float64) bool {
	coords := flatCoords(g.Coordinates)
	for i := 0; i+1 < len(coords); i += 2 {
		lng, lat := coords[i], coords[i+1]
		if lng >= minLng && lng <= maxLng && lat >= minLat && lat <= maxLat {
			return true
		}
	}
	return false
}

// flatCoords recursively flattens a GeoJSON coordinates structure into a flat
// slice of float64 pairs [lng, lat, lng, lat, ...].
func flatCoords(v interface{}) []float64 {
	switch c := v.(type) {
	case []interface{}:
		if len(c) == 2 {
			if a, ok := c[0].(float64); ok {
				if b, ok := c[1].(float64); ok {
					return []float64{a, b}
				}
			}
		}
		var result []float64
		for _, item := range c {
			result = append(result, flatCoords(item)...)
		}
		return result
	}
	return nil
}
