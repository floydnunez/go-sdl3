//go:build !js

package sdl

import (
	"testing"
)

func TestRectIntersection(t *testing.T) {
	loadSDL(t)
	cases := []struct {
		a, b Rect
		want *Rect
	}{
		{Rect{0, 0, 10, 10}, Rect{5, 5, 10, 10}, &Rect{5, 5, 5, 5}},
		{Rect{0, 0, 10, 10}, Rect{2, 3, 4, 5}, &Rect{2, 3, 4, 5}},
		{Rect{-10, -10, 15, 15}, Rect{0, 0, 10, 10}, &Rect{0, 0, 5, 5}},
		{Rect{0, 0, 10, 10}, Rect{10, 0, 10, 10}, nil},
		{Rect{0, 0, 10, 10}, Rect{20, 20, 1, 1}, nil},
		{Rect{0, 0, 0, 10}, Rect{0, 0, 10, 10}, nil},
	}
	for _, c := range cases {
		got := c.a.Intersection(&c.b)
		if (got == nil) != (c.want == nil) || (got != nil && *got != *c.want) {
			t.Errorf("%v ∩ %v = %v, want %v", c.a, c.b, got, c.want)
		}
		if has := c.a.HasIntersection(&c.b); has != (c.want != nil) {
			t.Errorf("HasIntersection(%v, %v) = %v", c.a, c.b, has)
		}
	}
}

func TestFRectIntersectionIncludesEdges(t *testing.T) {
	loadSDL(t)
	cases := []struct {
		a, b FRect
		want bool
	}{
		{FRect{0, 0, 10, 10}, FRect{5, 5, 10, 10}, true},
		{FRect{0, 0, 10, 10}, FRect{10, 0, 10, 10}, true},
		{FRect{0, 0, 0, 10}, FRect{0, 0, 10, 10}, true},
		{FRect{0, 0, 10, 10}, FRect{10.5, 0, 10, 10}, false},
		{FRect{0, 0, -1, 10}, FRect{0, 0, 10, 10}, false},
		{FRect{0.25, 0.25, 0.5, 0.5}, FRect{0.5, 0.5, 0.5, 0.5}, true},
	}
	for _, c := range cases {
		if got := c.a.HasRectIntersectionFloat(&c.b); got != c.want {
			t.Errorf("HasRectIntersectionFloat(%v, %v) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}

func TestRectUnion(t *testing.T) {
	loadSDL(t)
	cases := []struct{ a, b, want Rect }{
		{Rect{0, 0, 10, 10}, Rect{5, 5, 10, 10}, Rect{0, 0, 15, 15}},
		{Rect{-5, 2, 1, 1}, Rect{3, -4, 2, 2}, Rect{-5, -4, 10, 7}},
		{Rect{0, 0, 0, 0}, Rect{3, 4, 5, 6}, Rect{3, 4, 5, 6}},
	}
	for _, c := range cases {
		got, err := c.a.Union(&c.b)
		if err != nil {
			t.Fatalf("Union(%v, %v): %v", c.a, c.b, err)
		}
		if *got != c.want {
			t.Errorf("%v ∪ %v = %v, want %v", c.a, c.b, *got, c.want)
		}
	}
}

func TestRectEnclosingPoints(t *testing.T) {
	loadSDL(t)
	points := []Point{{3, 4}, {-2, 7}, {5, -1}, {0, 0}}
	got, ok := GetRectEnclosingPoints(points, nil)
	if !ok {
		t.Fatal("GetRectEnclosingPoints found no points")
	}
	if want := (Rect{-2, -1, 8, 9}); *got != want {
		t.Errorf("enclosing rect = %v, want %v", *got, want)
	}
	clip := Rect{0, 0, 4, 5}
	got, ok = GetRectEnclosingPoints(points, &clip)
	if !ok {
		t.Fatal("GetRectEnclosingPoints with a clip found no points")
	}
	if want := (Rect{0, 0, 4, 5}); *got != want {
		t.Errorf("clipped enclosing rect = %v, want %v", *got, want)
	}
	if _, ok := GetRectEnclosingPoints([]Point{{100, 100}}, &clip); ok {
		t.Error("a point outside the clip was enclosed")
	}
	if _, ok := GetRectEnclosingPoints(nil, nil); ok {
		t.Error("no points were enclosed")
	}
}

func TestRectNilArguments(t *testing.T) {
	loadSDL(t)
	r := Rect{0, 0, 10, 10}
	if r.HasIntersection(nil) {
		t.Error("HasIntersection(nil) = true")
	}
	if got := r.Intersection(nil); got != nil {
		t.Errorf("Intersection(nil) = %v", got)
	}
	if _, err := r.Union(nil); err == nil {
		t.Error("Union(nil) succeeded")
	}
	var nilRect *Rect
	if nilRect.HasIntersection(&r) {
		t.Error("nil.HasIntersection = true")
	}
	var nilF *FRect
	if nilF.HasRectIntersectionFloat(&FRect{0, 0, 1, 1}) {
		t.Error("nil.HasRectIntersectionFloat = true")
	}
}
