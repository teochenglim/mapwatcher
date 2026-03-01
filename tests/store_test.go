package tests

import (
	"testing"
	"time"

	"github.com/teochenglim/mapwatch/internal/marker"
)

func newMarker(id string, lat, lng float64) *marker.Marker {
	return &marker.Marker{
		ID:       id,
		Lat:      lat,
		Lng:      lng,
		Severity: "warning",
		StartsAt: time.Now(),
	}
}

func TestStoreUpsert(t *testing.T) {
	s := marker.NewStore(0.01)

	if added, _ := s.Upsert(newMarker("a", 1.35, 103.8)); !added {
		t.Fatal("expected true (new marker)")
	}
	if s.Get("a") == nil {
		t.Fatal("marker not found after upsert")
	}

	if added, _ := s.Upsert(newMarker("a", 1.36, 103.81)); added {
		t.Fatal("expected false (update)")
	}
	if got := s.Get("a").Lat; got != 1.36 {
		t.Fatalf("expected updated lat 1.36, got %v", got)
	}
}

func TestStoreRemove(t *testing.T) {
	s := marker.NewStore(0.01)
	s.Upsert(newMarker("b", 1.35, 103.8))

	if !s.Remove("b") {
		t.Fatal("expected true for existing marker")
	}
	if s.Get("b") != nil {
		t.Fatal("marker still present after remove")
	}
	if s.Remove("b") {
		t.Fatal("expected false for already-removed marker")
	}
}

func TestStoreAll(t *testing.T) {
	s := marker.NewStore(0.01)
	s.Upsert(newMarker("x", 1.0, 103.0))
	s.Upsert(newMarker("y", 1.1, 103.1))

	if n := len(s.All()); n != 2 {
		t.Fatalf("expected 2 markers, got %d", n)
	}
}

func TestStoreReconcile(t *testing.T) {
	s := marker.NewStore(0.01)
	s.Upsert(newMarker("keep", 1.0, 103.0))
	s.Upsert(newMarker("remove", 1.1, 103.1))

	incoming := []*marker.Marker{
		newMarker("keep", 1.0, 103.0), // updated
		newMarker("new", 1.2, 103.2),  // added
		// "remove" absent → deleted
	}

	added, updated, removed := s.Reconcile(incoming)

	if len(added) != 1 || added[0] != "new" {
		t.Fatalf("expected added=[new], got %v", added)
	}
	if len(updated) != 1 || updated[0] != "keep" {
		t.Fatalf("expected updated=[keep], got %v", updated)
	}
	if len(removed) != 1 || removed[0] != "remove" {
		t.Fatalf("expected removed=[remove], got %v", removed)
	}
	if s.Get("remove") != nil {
		t.Fatal("removed marker still present")
	}
	if s.Get("new") == nil {
		t.Fatal("new marker missing after reconcile")
	}
}

func TestStoreSpreadOffsets(t *testing.T) {
	s := marker.NewStore(0.01)
	// Two co-located markers → both get non-nil, distinct offsets.
	s.Upsert(newMarker("m1", 1.35, 103.8))
	s.Upsert(newMarker("m2", 1.35, 103.8))

	m1, m2 := s.Get("m1"), s.Get("m2")
	if m1.Offset == nil {
		t.Fatal("expected non-nil offset for m1")
	}
	if m2.Offset == nil {
		t.Fatal("expected non-nil offset for m2")
	}
	if m1.Offset.Lat == m2.Offset.Lat && m1.Offset.Lng == m2.Offset.Lng {
		t.Fatal("co-located markers have identical offsets")
	}
}

func TestStoreSingleMarkerNoOffset(t *testing.T) {
	s := marker.NewStore(0.01)
	s.Upsert(newMarker("solo", 1.35, 103.8))
	if m := s.Get("solo"); m.Offset != nil {
		t.Fatalf("solo marker should have nil offset, got %+v", m.Offset)
	}
}
