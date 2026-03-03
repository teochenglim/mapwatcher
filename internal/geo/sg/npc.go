package sg

import (
	"encoding/json"
	"fmt"
	"os"
)

// NPCInfo holds the NPC name and division for a geographic point.
type NPCInfo struct {
	NPCName  string
	Division string
}

// PointToNPC opens the NPC boundary GeoJSON at geojsonPath and returns
// the NPC name and division whose polygon contains (lat, lng).
// Returns nil, nil if the point falls outside all features.
func PointToNPC(lat, lng float64, geojsonPath string) (*NPCInfo, error) {
	f, err := os.Open(geojsonPath) //nolint:gosec
	if err != nil {
		return nil, fmt.Errorf("opening NPC GeoJSON: %w", err)
	}
	defer f.Close()

	var fc struct {
		Features []struct {
			Properties struct {
				NPCName  string `json:"NPC_NAME"`
				Division string `json:"DIVISION"`
			} `json:"properties"`
			Geometry struct {
				Type        string          `json:"type"`
				Coordinates json.RawMessage `json:"coordinates"`
			} `json:"geometry"`
		} `json:"features"`
	}
	if err := json.NewDecoder(f).Decode(&fc); err != nil {
		return nil, fmt.Errorf("decoding NPC GeoJSON: %w", err)
	}

	for _, feat := range fc.Features {
		var rings [][][]float64 // exterior ring per polygon

		switch feat.Geometry.Type {
		case "Polygon":
			// coordinates: [ring][point][lng,lat]
			var poly [][][]float64
			if err := json.Unmarshal(feat.Geometry.Coordinates, &poly); err != nil || len(poly) == 0 {
				continue
			}
			rings = append(rings, poly[0]) // exterior ring only

		case "MultiPolygon":
			// coordinates: [polygon][ring][point][lng,lat]
			var multi [][][][]float64
			if err := json.Unmarshal(feat.Geometry.Coordinates, &multi); err != nil {
				continue
			}
			for _, poly := range multi {
				if len(poly) > 0 {
					rings = append(rings, poly[0]) // exterior ring of each polygon
				}
			}
		}

		for _, ring := range rings {
			if pipRayCast(lat, lng, ring) {
				return &NPCInfo{
					NPCName:  feat.Properties.NPCName,
					Division: feat.Properties.Division,
				}, nil
			}
		}
	}
	return nil, nil
}

// pipRayCast returns true if the point (lat, lng) lies inside the given polygon
// ring using the ray-casting algorithm. Ring coordinates are [lng, lat] pairs
// as per the GeoJSON specification (RFC 7946 §3.1.6).
func pipRayCast(lat, lng float64, ring [][]float64) bool {
	inside := false
	n := len(ring)
	j := n - 1
	for i := 0; i < n; i++ {
		if len(ring[i]) < 2 || len(ring[j]) < 2 {
			j = i
			continue
		}
		xi, yi := ring[i][0], ring[i][1] // lng, lat
		xj, yj := ring[j][0], ring[j][1]
		if (yi > lat) != (yj > lat) && lng < (xj-xi)*(lat-yi)/(yj-yi)+xi {
			inside = !inside
		}
		j = i
	}
	return inside
}
