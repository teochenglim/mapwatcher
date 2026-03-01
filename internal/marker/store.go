package marker

import (
	"math"
	"sync"
	"time"

	"github.com/teochenglim/mapwatch/internal/geo"
)

// Marker is the core data structure for a geo-located alert or event.
type Marker struct {
	ID          string            `json:"id"`
	Geohash     string            `json:"geohash,omitempty"`
	Lat         float64           `json:"lat"`
	Lng         float64           `json:"lng"`
	GeoBounds   *geo.GeoBounds    `json:"geoBounds,omitempty"`
	Severity    string            `json:"severity"`
	AlertName   string            `json:"alertname"`
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
	StartsAt    time.Time         `json:"startsAt"`
	UpdatedAt   time.Time         `json:"updatedAt"`
	Source      string            `json:"source"`
	Offset      *geo.LatLng       `json:"offset,omitempty"`
}

// EffectiveLat returns Lat adjusted by spread Offset, if present.
func (m *Marker) EffectiveLat() float64 {
	if m.Offset != nil {
		return m.Lat + m.Offset.Lat
	}
	return m.Lat
}

// EffectiveLng returns Lng adjusted by spread Offset, if present.
func (m *Marker) EffectiveLng() float64 {
	if m.Offset != nil {
		return m.Lng + m.Offset.Lng
	}
	return m.Lng
}

// Store is a thread-safe in-memory store for Markers.
type Store struct {
	mu      sync.RWMutex
	markers map[string]*Marker
	radius  float64 // spread radius in degrees
}

// NewStore creates a new empty Store.
func NewStore(spreadRadius float64) *Store {
	if spreadRadius <= 0 {
		spreadRadius = 0.01
	}
	return &Store{
		markers: make(map[string]*Marker),
		radius:  spreadRadius,
	}
}

// Upsert adds or updates a marker. Returns (isNew bool, colocated []*Marker)
// where colocated contains other markers sharing the same base position whose
// offsets may have changed due to recomputeOffsets.
func (s *Store) Upsert(m *Marker) (bool, []*Marker) {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, exists := s.markers[m.ID]
	m.UpdatedAt = time.Now()
	s.markers[m.ID] = m
	s.recomputeOffsets()

	// Collect co-located markers (same base position, different ID).
	precision := 1e4
	mLat := int64(math.Round(m.Lat * precision))
	mLng := int64(math.Round(m.Lng * precision))
	var colocated []*Marker
	for _, existing := range s.markers {
		if existing.ID == m.ID {
			continue
		}
		if int64(math.Round(existing.Lat*precision)) == mLat &&
			int64(math.Round(existing.Lng*precision)) == mLng {
			colocated = append(colocated, existing)
		}
	}
	return !exists, colocated
}

// Remove deletes a marker by ID. Returns true if it existed.
func (s *Store) Remove(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, exists := s.markers[id]
	if exists {
		delete(s.markers, id)
		s.recomputeOffsets()
	}
	return exists
}

// Get returns a single marker by ID, or nil.
func (s *Store) Get(id string) *Marker {
	s.mu.RLock()
	defer s.mu.RUnlock()
	m := s.markers[id]
	return m
}

// All returns a snapshot of all current markers.
func (s *Store) All() []*Marker {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*Marker, 0, len(s.markers))
	for _, m := range s.markers {
		out = append(out, m)
	}
	return out
}

// Reconcile ensures the store contains exactly the markers in the provided slice.
// Returns three slices: added IDs, updated IDs, removed IDs.
func (s *Store) Reconcile(incoming []*Marker) (added, updated, removed []string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	incomingSet := make(map[string]*Marker, len(incoming))
	for _, m := range incoming {
		incomingSet[m.ID] = m
	}

	// Add or update
	for _, m := range incoming {
		m.UpdatedAt = time.Now()
		if _, exists := s.markers[m.ID]; exists {
			updated = append(updated, m.ID)
		} else {
			added = append(added, m.ID)
		}
		s.markers[m.ID] = m
	}

	// Remove markers not in incoming
	for id := range s.markers {
		if _, ok := incomingSet[id]; !ok {
			delete(s.markers, id)
			removed = append(removed, id)
		}
	}

	s.recomputeOffsets()
	return
}

// recomputeOffsets assigns deterministic circular spread offsets to co-located markers.
// Must be called with s.mu held (write).
func (s *Store) recomputeOffsets() {
	// Group markers by snapped position (rounded to 4 decimal places to detect overlap).
	type posKey struct{ lat, lng int64 }
	groups := make(map[posKey][]*Marker)
	precision := 1e4

	for _, m := range s.markers {
		key := posKey{
			lat: int64(math.Round(m.Lat * precision)),
			lng: int64(math.Round(m.Lng * precision)),
		}
		groups[key] = append(groups[key], m)
	}

	for _, grp := range groups {
		offsets := geo.SpreadOffsets(len(grp), s.radius)
		for i, m := range grp {
			if len(grp) == 1 {
				m.Offset = nil
			} else {
				o := offsets[i]
				m.Offset = &o
			}
		}
	}
}
