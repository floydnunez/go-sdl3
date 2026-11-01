//go:build !js

package sdl

import (
	"bytes"
	"encoding/binary"
	"runtime"
	"testing"
	"unsafe"
)

func retainedGoMemoryRefs(mem []byte) int {
	retainedGoMemoryMu.Lock()
	defer retainedGoMemoryMu.Unlock()
	r, ok := retainedGoMemoryAt[uintptr(unsafe.Pointer(unsafe.SliceData(mem)))]
	if !ok {
		return 0
	}
	return r.refs
}

func growStack(depth int) byte {
	var pad [512]byte
	pad[depth%512] = byte(depth)
	if depth == 0 {
		return pad[0]
	}
	return growStack(depth-1) + pad[(depth*7)%512]
}

func churnHeap() {
	junk := make([][]byte, 0, 4096)
	for i := range 4096 {
		b := make([]byte, 64)
		for j := range b {
			b[j] = 0xEE
		}
		junk = append(junk, b)
		if i%1024 == 0 {
			runtime.GC()
		}
	}
	runtime.KeepAlive(junk)
}

func surfaceFromDroppedSlice(t *testing.T) *Surface {
	t.Helper()
	pixels := make([]byte, 4*4*4)
	for i := range 16 {
		binary.NativeEndian.PutUint32(pixels[i*4:], uint32(0x11223344+i))
	}
	s, err := CreateSurfaceFrom(4, 4, PIXELFORMAT_RGBA8888, pixels, 16)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestSurfaceFromGoMemoryOutlivesTheStack(t *testing.T) {
	loadSDL(t)
	pixels := make([]byte, 4*4*4)
	s, err := CreateSurfaceFrom(4, 4, PIXELFORMAT_RGBA8888, pixels, 16)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Destroy()
	growStack(400)
	fill(t, s, &Rect{1, 1, 1, 1}, red)
	if got := binary.NativeEndian.Uint32(pixels[16+4:]); got != s.MapRGBA(255, 0, 0, 255) {
		t.Errorf("pixel the surface filled = %#x in the Go slice", got)
	}
}

func TestSurfaceFromGoMemoryOutlivesTheCaller(t *testing.T) {
	loadSDL(t)
	s := surfaceFromDroppedSlice(t)
	defer s.Destroy()
	for range 3 {
		runtime.GC()
		churnHeap()
	}
	px := s.Pixels()
	for i := range 16 {
		if got := binary.NativeEndian.Uint32(px[i*4:]); got != uint32(0x11223344+i) {
			t.Fatalf("pixel %d = %#x after the caller dropped its slice", i, got)
		}
	}
}

func TestSurfaceFromGoMemoryIsReleasedOnDestroy(t *testing.T) {
	loadSDL(t)
	pixels := make([]byte, 2*2*4)
	a, err := CreateSurfaceFrom(2, 2, PIXELFORMAT_RGBA8888, pixels, 8)
	if err != nil {
		t.Fatal(err)
	}
	b, err := CreateSurfaceFrom(2, 2, PIXELFORMAT_RGBA8888, pixels, 8)
	if err != nil {
		t.Fatal(err)
	}
	if got := retainedGoMemoryRefs(pixels); got != 2 {
		t.Fatalf("refs with two surfaces = %d", got)
	}
	a.Destroy()
	if got := retainedGoMemoryRefs(pixels); got != 1 {
		t.Errorf("refs after one Destroy = %d", got)
	}
	b.Destroy()
	if got := retainedGoMemoryRefs(pixels); got != 0 {
		t.Errorf("refs after both Destroy = %d", got)
	}
}

func TestIOFromConstMemOutlivesTheCaller(t *testing.T) {
	loadSDL(t)
	want := bytes.Repeat([]byte("const memory "), 50)
	stream := func() *IOStream {
		mem := bytes.Clone(want)
		s, err := IOFromConstMem(mem)
		if err != nil {
			t.Fatal(err)
		}
		return s
	}()
	growStack(400)
	for range 3 {
		runtime.GC()
		churnHeap()
	}
	got := make([]byte, len(want))
	n, err := stream.Read(got)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got[:n], want) {
		t.Errorf("stream read %q", got[:min(n, 40)])
	}
	if err := stream.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestIOFromConstMemIsReleasedWhenClosed(t *testing.T) {
	loadSDL(t)
	mem := []byte("released by Close")
	s, err := IOFromConstMem(mem)
	if err != nil {
		t.Fatal(err)
	}
	if got := retainedGoMemoryRefs(mem); got != 1 {
		t.Fatalf("refs while open = %d", got)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if got := retainedGoMemoryRefs(mem); got != 0 {
		t.Errorf("refs after Close = %d", got)
	}

	mem2 := []byte("not a BMP, closed by SDL")
	s2, err := IOFromConstMem(mem2)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := LoadBMP_IO(s2, true); err == nil {
		t.Fatal("LoadBMP_IO read a BMP from text")
	}
	if got := retainedGoMemoryRefs(mem2); got != 0 {
		t.Errorf("refs after SDL closed the stream = %d", got)
	}
}

func TestIOFromConstMemEmpty(t *testing.T) {
	loadSDL(t)
	s, err := IOFromConstMem(nil)
	if err != nil {
		t.Fatal(err)
	}
	if size, err := s.Size(); err != nil || size != 0 {
		t.Errorf("Size = %d, %v", size, err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}
