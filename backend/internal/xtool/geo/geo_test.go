package geo

import (
	"math"
	"testing"
)

func TestEncode_Stable(t *testing.T) {
	p := Point{Lat: 57.64911, Lng: 10.40744}
	h1 := Encode(p, 8)
	h2 := Encode(p, 8)
	if h1 != h2 {
		t.Fatal("should be stable")
	}
	if len(h1) != 8 {
		t.Fatal("length")
	}
}

func TestEncode_Clamp(t *testing.T) {
	p := Point{Lat: 12, Lng: 34}
	if Encode(p, 0) != Encode(p, 1) {
		t.Fatal("precision 0 should clamp")
	}
	if Encode(p, 99) != Encode(p, 12) {
		t.Fatal("precision 99 should clamp")
	}
}

func TestDecode_Bounds(t *testing.T) {
	p := Point{Lat: 12, Lng: 34}
	h := Encode(p, 6)
	center, latSpan, lngSpan := Decode(h)
	if latSpan <= 0 || lngSpan <= 0 {
		t.Fatal("spans")
	}
	if math.Abs(center.Lat-p.Lat) > latSpan {
		t.Fatal("lat center")
	}
	if math.Abs(center.Lng-p.Lng) > lngSpan {
		t.Fatal("lng center")
	}
}

func TestAddRemove(t *testing.T) {
	i := New(6)
	i.Add(Feature{ID: "a", Loc: Point{1, 2}})
	if i.Len() != 1 {
		t.Fatal("len")
	}
	i.Remove("a")
	if i.Len() != 0 {
		t.Fatal("after remove")
	}
}

func TestNearby(t *testing.T) {
	i := New(6)
	q := Point{Lat: 0, Lng: 0}
	i.Add(Feature{ID: "near", Loc: Point{Lat: 0.001, Lng: 0.001}})
	i.Add(Feature{ID: "far", Loc: Point{Lat: 10, Lng: 10}})
	near := i.Nearby(q, 5)
	if len(near) != 1 || near[0].ID != "near" {
		t.Fatalf("nearby: %v", near)
	}
}

func TestDist(t *testing.T) {
	d := dist(Point{0, 0}, Point{0, 0})
	if d != 0 {
		t.Fatal("distance to self")
	}
	d2 := dist(Point{0, 0}, Point{0, 1})
	if d2 <= 0 {
		t.Fatal("distance")
	}
}

func TestNeighbors(t *testing.T) {
	n := Neighbors("abc")
	if len(n) < 1 || n[0] != "abc" {
		t.Fatal("neighbors")
	}
}

func TestNew_Defaults(t *testing.T) {
	i := New(0)
	if i.prec == 0 {
		t.Fatal("precision default")
	}
}
