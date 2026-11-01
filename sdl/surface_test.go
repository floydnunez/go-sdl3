//go:build !js

package sdl

import (
	"encoding/binary"
	"math"
	"runtime"
	"testing"
)

type rgba struct{ r, g, b, a uint8 }

func near(a, b uint8, tolerance int) bool {
	d := int(a) - int(b)
	return d >= -tolerance && d <= tolerance
}

func (c rgba) near(o rgba, tolerance int) bool {
	return near(c.r, o.r, tolerance) && near(c.g, o.g, tolerance) && near(c.b, o.b, tolerance) && near(c.a, o.a, tolerance)
}

func newTestSurface(t *testing.T, w, h int, format PixelFormat) *Surface {
	t.Helper()
	loadSDL(t)
	s, err := CreateSurface(w, h, format)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Destroy)
	return s
}

func readPixel(t *testing.T, s *Surface, x, y int32) rgba {
	t.Helper()
	r, g, b, a, err := s.ReadPixel(x, y)
	if err != nil {
		t.Fatalf("ReadPixel(%d, %d): %v", x, y, err)
	}
	return rgba{r, g, b, a}
}

func fill(t *testing.T, s *Surface, rect *Rect, c rgba) {
	t.Helper()
	if err := s.FillRect(rect, s.MapRGBA(c.r, c.g, c.b, c.a)); err != nil {
		t.Fatal(err)
	}
}

func expectPixels(t *testing.T, s *Surface, want map[Point]rgba) {
	t.Helper()
	for p, c := range want {
		if got := readPixel(t, s, p.X, p.Y); got != c {
			t.Errorf("pixel (%d, %d) = %v, want %v", p.X, p.Y, got, c)
		}
	}
}

var (
	red   = rgba{255, 0, 0, 255}
	green = rgba{0, 255, 0, 255}
	blue  = rgba{0, 0, 255, 255}
)

func TestPixelFormatDetails(t *testing.T) {
	loadSDL(t)
	cases := []struct {
		format     PixelFormat
		name       string
		bpp, bytes uint8
		r, g, b, a uint32
	}{
		{PIXELFORMAT_RGBA8888, "SDL_PIXELFORMAT_RGBA8888", 32, 4, 0xFF000000, 0x00FF0000, 0x0000FF00, 0x000000FF},
		{PIXELFORMAT_ARGB8888, "SDL_PIXELFORMAT_ARGB8888", 32, 4, 0x00FF0000, 0x0000FF00, 0x000000FF, 0xFF000000},
		{PIXELFORMAT_RGB565, "SDL_PIXELFORMAT_RGB565", 16, 2, 0xF800, 0x07E0, 0x001F, 0},
	}
	for _, c := range cases {
		if got := c.format.Name(); got != c.name {
			t.Errorf("Name() = %q, want %q", got, c.name)
		}
		d, err := c.format.Details()
		if err != nil {
			t.Fatal(err)
		}
		if d.Format != c.format || d.BitsPerPixel != c.bpp || d.BytesPerPixel != c.bytes ||
			d.Rmask != c.r || d.Gmask != c.g || d.Bmask != c.b || d.Amask != c.a {
			t.Errorf("%s details = %+v", c.name, *d)
		}
		if got := GetPixelFormatForMasks(int(c.bpp), c.r, c.g, c.b, c.a); got != c.format {
			t.Errorf("GetPixelFormatForMasks for %s = %s", c.name, got.Name())
		}
		for _, col := range []rgba{red, green, blue, {255, 255, 255, 255}, {0, 0, 0, 255}} {
			p := d.MapRGBA(nil, col.r, col.g, col.b, col.a)
			r, g, b, a := GetRGBA(p, d, nil)
			if got := (rgba{r, g, b, a}); got != col {
				t.Errorf("%s: GetRGBA(MapRGBA(%v)) = %v", c.name, col, got)
			}
			if p2 := MapRGB(d, nil, col.r, col.g, col.b); p2 != p {
				t.Errorf("%s: MapRGB(%v) = %#x, MapRGBA gave %#x", c.name, col, p2, p)
			}
			r, g, b = GetRGB(p, d, nil)
			if (rgba{r, g, b, 255}) != col {
				t.Errorf("%s: GetRGB(MapRGB(%v)) = %d %d %d", c.name, col, r, g, b)
			}
		}
	}
	if got := PixelFormat(0x7fffffff).Name(); got != "SDL_PIXELFORMAT_UNKNOWN" {
		t.Errorf("Name of an invalid format = %q", got)
	}
}

func TestSurfaceCreateAndPixels(t *testing.T) {
	s := newTestSurface(t, 7, 5, PIXELFORMAT_RGBA8888)
	if s.W != 7 || s.H != 5 || s.Format != PIXELFORMAT_RGBA8888 || s.Pitch < 28 {
		t.Fatalf("surface %+v", *s)
	}
	px := s.Pixels()
	if len(px) != int(s.H*s.Pitch) {
		t.Fatalf("len(Pixels()) = %d, want %d", len(px), s.H*s.Pitch)
	}
	c := rgba{10, 20, 30, 40}
	fill(t, s, nil, c)
	want := s.MapRGBA(c.r, c.g, c.b, c.a)
	for y := range s.H {
		for x := range s.W {
			off := y*s.Pitch + x*4
			if got := binary.NativeEndian.Uint32(px[off : off+4]); got != want {
				t.Fatalf("pixel bytes at (%d, %d) = %#x, want %#x", x, y, got, want)
			}
		}
	}
	expectPixels(t, s, map[Point]rgba{{0, 0}: c, {6, 4}: c})
}

func TestSurfaceFillWriteRead(t *testing.T) {
	s := newTestSurface(t, 8, 8, PIXELFORMAT_RGBA8888)
	fill(t, s, nil, blue)
	fill(t, s, &Rect{2, 2, 3, 3}, red)
	if err := s.FillRects([]Rect{{0, 0, 1, 1}, {7, 7, 1, 1}}, s.MapRGBA(0, 255, 0, 255)); err != nil {
		t.Fatal(err)
	}
	if err := s.WritePixel(6, 1, 1, 2, 3, 4); err != nil {
		t.Fatal(err)
	}
	expectPixels(t, s, map[Point]rgba{
		{0, 0}: green, {7, 7}: green, {1, 1}: blue, {2, 2}: red, {4, 4}: red, {5, 5}: blue, {6, 1}: {1, 2, 3, 4},
	})
	if _, _, _, _, err := s.ReadPixel(8, 0); err == nil {
		t.Error("ReadPixel outside the surface succeeded")
	}
	if err := s.WritePixel(-1, 0, 0, 0, 0, 0); err == nil {
		t.Error("WritePixel outside the surface succeeded")
	}
}

func TestSurfaceFloatArguments(t *testing.T) {
	s := newTestSurface(t, 4, 4, PIXELFORMAT_RGBA8888)
	if err := s.Clear(1, 0.5, 0.25, 1); err != nil {
		t.Fatal(err)
	}
	if got := readPixel(t, s, 3, 3); !got.near(rgba{255, 128, 64, 255}, 1) {
		t.Errorf("after Clear(1, 0.5, 0.25, 1) pixel = %v", got)
	}
	if err := s.WritePixelFloat(1, 2, 0, 0.75, 1, 0.5); err != nil {
		t.Fatal(err)
	}
	if got := readPixel(t, s, 1, 2); !got.near(rgba{0, 191, 255, 128}, 1) {
		t.Errorf("after WritePixelFloat(0, 0.75, 1, 0.5) pixel = %v", got)
	}
	r, g, b, a, err := s.ReadPixelFloat(1, 2)
	if err != nil {
		t.Fatal(err)
	}
	for i, pair := range [][2]float32{{r, 0}, {g, 0.75}, {b, 1}, {a, 0.5}} {
		if math.Abs(float64(pair[0]-pair[1])) > 1.0/255 {
			t.Errorf("ReadPixelFloat channel %d = %g, want %g", i, pair[0], pair[1])
		}
	}
}

func TestSurfaceBlits(t *testing.T) {
	src := newTestSurface(t, 2, 2, PIXELFORMAT_RGBA8888)
	fill(t, src, nil, green)
	if err := src.WritePixel(0, 0, red.r, red.g, red.b, red.a); err != nil {
		t.Fatal(err)
	}
	dst := newTestSurface(t, 8, 8, PIXELFORMAT_RGBA8888)

	fill(t, dst, nil, blue)
	if err := src.Blit(nil, dst, &Rect{X: 3, Y: 3}); err != nil {
		t.Fatal(err)
	}
	expectPixels(t, dst, map[Point]rgba{{3, 3}: red, {4, 4}: green, {2, 2}: blue, {5, 5}: blue})

	fill(t, dst, nil, blue)
	if err := src.BlitScaled(nil, dst, &Rect{0, 0, 4, 4}, SCALEMODE_NEAREST); err != nil {
		t.Fatal(err)
	}
	expectPixels(t, dst, map[Point]rgba{{0, 0}: red, {1, 1}: red, {2, 2}: green, {3, 3}: green, {4, 4}: blue})

	fill(t, dst, nil, blue)
	if err := src.BlitTiled(nil, dst, nil); err != nil {
		t.Fatal(err)
	}
	expectPixels(t, dst, map[Point]rgba{{0, 0}: red, {1, 1}: green, {2, 2}: red, {6, 6}: red, {7, 7}: green})

	fill(t, dst, nil, blue)
	if err := src.BlitTiledWithScale(nil, 2, SCALEMODE_NEAREST, dst, nil); err != nil {
		t.Fatal(err)
	}
	expectPixels(t, dst, map[Point]rgba{{0, 0}: red, {1, 1}: red, {2, 2}: green, {4, 4}: red, {5, 5}: red, {6, 6}: green})

	grid := newTestSurface(t, 3, 3, PIXELFORMAT_RGBA8888)
	fill(t, grid, nil, green)
	fill(t, grid, &Rect{1, 1, 1, 1}, red)
	fill(t, dst, nil, blue)
	if err := grid.Blit9Grid(nil, 1, 1, 1, 1, 2, SCALEMODE_NEAREST, dst, nil); err != nil {
		t.Fatal(err)
	}
	expectPixels(t, dst, map[Point]rgba{{0, 0}: green, {1, 1}: green, {2, 2}: red, {5, 5}: red, {6, 6}: green, {7, 7}: green})

	fill(t, dst, nil, blue)
	if err := src.BlitUnchecked(&Rect{0, 0, 2, 2}, dst, &Rect{1, 1, 2, 2}); err != nil {
		t.Fatal(err)
	}
	expectPixels(t, dst, map[Point]rgba{{1, 1}: red, {2, 2}: green, {0, 0}: blue})
}

func TestSurfaceTransforms(t *testing.T) {
	s := newTestSurface(t, 4, 2, PIXELFORMAT_RGBA8888)
	fill(t, s, nil, blue)
	if err := s.WritePixel(0, 0, red.r, red.g, red.b, red.a); err != nil {
		t.Fatal(err)
	}

	dup, err := s.Duplicate()
	if err != nil {
		t.Fatal(err)
	}
	defer dup.Destroy()
	expectPixels(t, dup, map[Point]rgba{{0, 0}: red, {3, 1}: blue})

	conv, err := s.Convert(PIXELFORMAT_ARGB8888)
	if err != nil {
		t.Fatal(err)
	}
	defer conv.Destroy()
	if conv.Format != PIXELFORMAT_ARGB8888 {
		t.Errorf("converted format = %s", conv.Format.Name())
	}
	expectPixels(t, conv, map[Point]rgba{{0, 0}: red, {3, 1}: blue})

	conv565, err := s.ConvertWithColorspace(PIXELFORMAT_RGB565, nil, COLORSPACE_SRGB, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer conv565.Destroy()
	expectPixels(t, conv565, map[Point]rgba{{0, 0}: red, {3, 1}: blue})

	scaled, err := s.Scale(8, 4, SCALEMODE_NEAREST)
	if err != nil {
		t.Fatal(err)
	}
	defer scaled.Destroy()
	if scaled.W != 8 || scaled.H != 4 {
		t.Errorf("scaled to %dx%d", scaled.W, scaled.H)
	}
	expectPixels(t, scaled, map[Point]rgba{{0, 0}: red, {1, 1}: red, {2, 2}: blue})

	if err := s.Flip(FLIP_HORIZONTAL); err != nil {
		t.Fatal(err)
	}
	expectPixels(t, s, map[Point]rgba{{3, 0}: red, {0, 0}: blue})
	if err := s.Flip(FLIP_VERTICAL); err != nil {
		t.Fatal(err)
	}
	expectPixels(t, s, map[Point]rgba{{3, 1}: red, {3, 0}: blue})

	if err := s.WritePixel(1, 0, 200, 100, 50, 128); err != nil {
		t.Fatal(err)
	}
	if err := s.PremultiplyAlpha(false); err != nil {
		t.Fatal(err)
	}
	if got := readPixel(t, s, 1, 0); !got.near(rgba{100, 50, 25, 128}, 1) {
		t.Errorf("premultiplied pixel = %v", got)
	}
}

func TestSurfacePalette(t *testing.T) {
	s := newTestSurface(t, 4, 4, PIXELFORMAT_INDEX8)
	palette, err := CreatePalette(4)
	if err != nil {
		t.Fatal(err)
	}
	defer palette.Destroy()
	colors := []Color{{0, 0, 0, 255}, {255, 0, 0, 255}, {0, 255, 0, 255}, {0, 0, 255, 255}}
	if err := palette.SetColors(colors, 0); err != nil {
		t.Fatal(err)
	}
	if err := s.SetPalette(palette); err != nil {
		t.Fatal(err)
	}
	if s.Palette() != palette {
		t.Error("Palette() is not the palette set")
	}
	if err := s.FillRect(nil, 2); err != nil {
		t.Fatal(err)
	}
	if err := s.FillRect(&Rect{0, 0, 1, 1}, 3); err != nil {
		t.Fatal(err)
	}
	expectPixels(t, s, map[Point]rgba{{0, 0}: blue, {3, 3}: green})
	if got := s.Pixels()[s.Pitch+1]; got != 2 {
		t.Errorf("index byte = %d, want 2", got)
	}

	s2 := newTestSurface(t, 2, 2, PIXELFORMAT_INDEX8)
	p2, err := s2.CreatePalette()
	if err != nil {
		t.Fatal(err)
	}
	if s2.Palette() != p2 {
		t.Error("CreatePalette did not attach the palette")
	}
}

func TestSurfaceSettings(t *testing.T) {
	s := newTestSurface(t, 4, 4, PIXELFORMAT_RGBA8888)
	key := s.MapRGBA(1, 2, 3, 255)
	if err := s.SetColorKey(true, key); err != nil {
		t.Fatal(err)
	}
	if !s.HasColorKey() {
		t.Error("HasColorKey = false")
	}
	if got, err := s.ColorKey(); err != nil || got != key {
		t.Errorf("ColorKey = %#x, %v", got, err)
	}
	if err := s.SetColorKey(false, 0); err != nil {
		t.Fatal(err)
	}
	if s.HasColorKey() {
		t.Error("HasColorKey after disabling = true")
	}
	if err := s.SetColorMod(10, 20, 30); err != nil {
		t.Fatal(err)
	}
	if r, g, b, err := s.ColorMod(); err != nil || r != 10 || g != 20 || b != 30 {
		t.Errorf("ColorMod = %d %d %d, %v", r, g, b, err)
	}
	if err := s.SetAlphaMod(77); err != nil {
		t.Fatal(err)
	}
	if a, err := s.AlphaMod(); err != nil || a != 77 {
		t.Errorf("AlphaMod = %d, %v", a, err)
	}
	if err := s.SetBlendMode(BLENDMODE_NONE); err != nil {
		t.Fatal(err)
	}
	if m, err := s.BlendMode(); err != nil || m != BLENDMODE_NONE {
		t.Errorf("BlendMode = %d, %v", m, err)
	}
	if !s.SetClipRect(&Rect{1, 1, 2, 2}) {
		t.Error("SetClipRect = false")
	}
	if r, err := s.ClipRect(); err != nil || *r != (Rect{1, 1, 2, 2}) {
		t.Errorf("ClipRect = %v, %v", r, err)
	}
	fill(t, s, nil, blue)
	fill(t, s, &Rect{0, 0, 4, 4}, red)
	s.SetClipRect(nil)
	if r, err := s.ClipRect(); err != nil || *r != (Rect{0, 0, 4, 4}) {
		t.Errorf("ClipRect after reset = %v, %v", r, err)
	}
	if err := s.SetRLE(true); err != nil {
		t.Fatal(err)
	}
	if !s.HasRLE() {
		t.Error("HasRLE = false")
	}
	if err := s.Lock(); err != nil {
		t.Fatal(err)
	}
	s.Unlock()
	if err := s.SetColorspace(COLORSPACE_SRGB); err != nil {
		t.Fatal(err)
	}
	if got := s.Colorspace(); got != COLORSPACE_SRGB {
		t.Errorf("Colorspace = %d", got)
	}
	props, err := s.Properties()
	if err != nil || props == 0 {
		t.Errorf("Properties = %d, %v", props, err)
	}
}

func TestSurfaceClipRectLimitsFill(t *testing.T) {
	s := newTestSurface(t, 4, 4, PIXELFORMAT_RGBA8888)
	fill(t, s, nil, blue)
	s.SetClipRect(&Rect{1, 1, 2, 2})
	fill(t, s, nil, red)
	expectPixels(t, s, map[Point]rgba{{0, 0}: blue, {1, 1}: red, {2, 2}: red, {3, 3}: blue})
}

func TestSurfaceAlternateImages(t *testing.T) {
	s := newTestSurface(t, 4, 4, PIXELFORMAT_RGBA8888)
	if s.HasAlternateImages() {
		t.Fatal("new surface has alternate images")
	}
	big := newTestSurface(t, 8, 8, PIXELFORMAT_RGBA8888)
	if err := s.AddAlternateImage(big); err != nil {
		t.Fatal(err)
	}
	if !s.HasAlternateImages() {
		t.Error("HasAlternateImages = false after adding one")
	}
	images, err := s.Images()
	if err != nil {
		t.Fatal(err)
	}
	if len(images) != 2 || images[0] != s || images[1] != big {
		t.Errorf("Images() = %v, want [%p %p]", images, s, big)
	}
	s.RemoveAlternateImages()
	if s.HasAlternateImages() {
		t.Error("HasAlternateImages after RemoveAlternateImages = true")
	}
}

func TestSurfaceFromGoMemory(t *testing.T) {
	loadSDL(t)
	pixels := make([]byte, 4*4*4)
	s, err := CreateSurfaceFrom(4, 4, PIXELFORMAT_RGBA8888, pixels, 16)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Destroy()
	fill(t, s, &Rect{1, 1, 1, 1}, red)
	if got := binary.NativeEndian.Uint32(pixels[16+4:]); got != s.MapRGBA(255, 0, 0, 255) {
		t.Errorf("Go slice does not hold the filled pixel: %#x", got)
	}
	binary.NativeEndian.PutUint32(pixels[2*16+3*4:], s.MapRGBA(0, 0, 255, 255))
	expectPixels(t, s, map[Point]rgba{{3, 2}: blue, {1, 1}: red})
	runtime.KeepAlive(pixels)
}

func TestSurfaceInvalidArguments(t *testing.T) {
	loadSDL(t)
	if _, err := CreateSurface(-1, 4, PIXELFORMAT_RGBA8888); err == nil {
		t.Error("CreateSurface with a negative width succeeded")
	}
	if _, err := CreateSurface(4, 4, PixelFormat(0x7fffffff)); err == nil {
		t.Error("CreateSurface with an invalid format succeeded")
	}
	var none *Surface
	none.Destroy()
	if err := none.FillRect(nil, 0); err == nil {
		t.Error("FillRect on a nil surface succeeded")
	}
	if _, _, _, _, err := none.ReadPixel(0, 0); err == nil {
		t.Error("ReadPixel on a nil surface succeeded")
	}
	s := newTestSurface(t, 2, 2, PIXELFORMAT_RGBA8888)
	if err := s.Blit(nil, nil, nil); err == nil {
		t.Error("Blit to a nil surface succeeded")
	}
}

func TestSurfaceManyCreateDestroy(t *testing.T) {
	loadSDL(t)
	for i := range 2000 {
		s, err := CreateSurface(16+i%17, 16+i%13, PIXELFORMAT_RGBA8888)
		if err != nil {
			t.Fatal(err)
		}
		fill(t, s, nil, rgba{uint8(i), uint8(i >> 8), 0, 255})
		d, err := s.Duplicate()
		if err != nil {
			t.Fatal(err)
		}
		if got := readPixel(t, d, 3, 3); got != (rgba{uint8(i), uint8(i >> 8), 0, 255}) {
			t.Fatalf("iteration %d: duplicated pixel = %v", i, got)
		}
		d.Destroy()
		s.Destroy()
		if i%500 == 0 {
			runtime.GC()
		}
	}
}
