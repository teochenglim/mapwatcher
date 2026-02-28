package transformer

import "github.com/teochenglim/mapwatch/internal/marker"

// Transformer is implemented by any data source integration that can convert
// a raw payload into a slice of Markers.
type Transformer interface {
	Name() string
	Transform(payload []byte) ([]*marker.Marker, error)
}
