package geo

import (
	"math"

	"github.com/mmcloughlin/geohash"
)

// GeoBounds represents a geohash bounding box.
type GeoBounds struct {
	MinLat, MaxLat float64
	MinLng, MaxLng float64
}

// LatLng is a simple lat/lng coordinate.
type LatLng struct {
	Lat, Lng float64
}

// DecodeGeohash returns the center lat/lng and bounding box for the given geohash string.
func DecodeGeohash(hash string) (center LatLng, bounds GeoBounds, err error) {
	box := geohash.BoundingBox(hash)
	center = LatLng{
		Lat: (box.MinLat + box.MaxLat) / 2,
		Lng: (box.MinLng + box.MaxLng) / 2,
	}
	bounds = GeoBounds{
		MinLat: box.MinLat,
		MaxLat: box.MaxLat,
		MinLng: box.MinLng,
		MaxLng: box.MaxLng,
	}
	return
}

// SpreadOffsets computes deterministic circular spread offsets for n co-located
// markers around the center. radius is in degrees (default 0.01).
// Returns a slice of LatLng offsets indexed 0..n-1.
func SpreadOffsets(n int, radius float64) []LatLng {
	offsets := make([]LatLng, n)
	if n == 0 {
		return offsets
	}
	if n == 1 {
		// Single marker — no offset needed.
		return offsets
	}
	for i := 0; i < n; i++ {
		angle := 2 * math.Pi * float64(i) / float64(n)
		offsets[i] = LatLng{
			Lat: radius * math.Sin(angle),
			Lng: radius * math.Cos(angle),
		}
	}
	return offsets
}
