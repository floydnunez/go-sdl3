//go:build !js

package sdl

import (
	"os"
	"testing"
	"unsafe"

	puregogen "github.com/Zyko0/purego-gen"
	"github.com/ebitengine/purego"
)

// loadSDL loads the SDL library for the tests, failing them if it is
// not found. SDL3_LIB names the library's path where the loader's search
// does not find it (Homebrew's /opt/homebrew/lib, say); unset, it is
// Path().
func loadSDL(t testing.TB) {
	t.Helper()
	if _hnd_sdl != 0 {
		return
	}
	path, ok := os.LookupEnv("SDL3_LIB")
	if !ok {
		path = Path()
	}
	if err := LoadLibrary(path); err != nil {
		t.Fatalf("load %s (set SDL3_LIB to its path): %v", path, err)
	}
}

func openSymbol(t testing.TB, name string) uintptr {
	t.Helper()
	addr, err := puregogen.OpenSymbol(_hnd_sdl, name)
	if err != nil {
		t.Fatalf("open %s: %v", name, err)
	}
	return addr
}

// TestHotcall calls SDL functions through hotcall and through purego's
// SyscallN and compares the results, and checks hotcall allocates nothing.
func TestHotcall(t *testing.T) {
	loadSDL(t)
	crc32 := openSymbol(t, "SDL_crc32")       // Uint32 SDL_crc32(Uint32 crc, const void *data, size_t len)
	murmur := openSymbol(t, "SDL_murmur3_32") // Uint32 SDL_murmur3_32(const void *data, size_t len, Uint32 seed)
	data := []byte("the quick brown fox jumps over the lazy dog")
	p := uintptr(unsafe.Pointer(&data[0]))
	n := uintptr(len(data))

	cases := []struct {
		name string
		hot  func() uintptr
		ref  func() uintptr
	}{
		{"SDL_GetVersion",
			func() uintptr { return hotcall(_addr_SDL_GetVersion, 0, 0, 0, 0, 0, 0) },
			func() uintptr { r, _, _ := purego.SyscallN(_addr_SDL_GetVersion); return r }},
		{"SDL_crc32",
			func() uintptr { return hotcall(crc32, 0x1234, p, n, 0, 0, 0) },
			func() uintptr { r, _, _ := purego.SyscallN(crc32, 0x1234, p, n); return r }},
		{"SDL_murmur3_32",
			func() uintptr { return hotcall(murmur, p, n, 77, 0, 0, 0) },
			func() uintptr { r, _, _ := purego.SyscallN(murmur, p, n, 77); return r }},
	}
	for _, c := range cases {
		got, want := uint32(c.hot()), uint32(c.ref())
		if got != want {
			t.Errorf("%s: hotcall %#x, SyscallN %#x", c.name, got, want)
		}
		if got == 0 {
			t.Errorf("%s returned 0", c.name)
		}
		if a := testing.AllocsPerRun(100, func() { c.hot() }); a != 0 {
			t.Errorf("%s: hotcall allocates %.1f times a call", c.name, a)
		}
	}
}

func BenchmarkHotcall(b *testing.B) {
	loadSDL(b)
	for b.Loop() {
		hotcall(_addr_SDL_GetVersion, 0, 0, 0, 0, 0, 0)
	}
}

func BenchmarkSyscallN(b *testing.B) {
	loadSDL(b)
	for b.Loop() {
		purego.SyscallN(_addr_SDL_GetVersion)
	}
}
