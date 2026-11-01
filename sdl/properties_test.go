//go:build !js

package sdl

import (
	"fmt"
	"math"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
	"unsafe"
)

func newTestProperties(t *testing.T) PropertiesID {
	t.Helper()
	loadSDL(t)
	props, err := CreateProperties()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(props.Destroy)
	return props
}

func TestPropertiesStringRoundTrip(t *testing.T) {
	props := newTestProperties(t)
	values := map[string]string{
		"ascii":   "hello",
		"utf8":    "héllo wörld ✓ 日本",
		"spaces":  "  leading and trailing  ",
		"long":    strings.Repeat("0123456789abcdef", 4096),
		"percent": "100%s %d %n",
	}
	for name, value := range values {
		if err := props.SetStringProperty(name, value); err != nil {
			t.Fatalf("SetStringProperty(%q): %v", name, err)
		}
	}
	runtime.GC()
	for name, want := range values {
		if got := props.PropertyType(name); got != PROPERTY_TYPE_STRING {
			t.Errorf("PropertyType(%q) = %d, want string", name, got)
		}
		if got := props.StringProperty(name, "default"); got != want {
			t.Errorf("StringProperty(%q) = %.40q, want %.40q", name, got, want)
		}
	}
}

func TestPropertiesEmptyStringIsStored(t *testing.T) {
	props := newTestProperties(t)
	if err := props.SetStringProperty("empty", ""); err != nil {
		t.Fatal(err)
	}
	if !props.HasProperty("empty") {
		t.Fatal(`SetStringProperty("empty", "") left no property`)
	}
	if got := props.StringProperty("empty", "default"); got != "" {
		t.Errorf("StringProperty = %q, want empty", got)
	}
}

func TestPropertiesNumberFloatBooleanPointer(t *testing.T) {
	props := newTestProperties(t)
	numbers := []int64{0, 1, -1, math.MaxInt64, math.MinInt64, 1 << 40}
	for i, n := range numbers {
		if err := props.SetNumberProperty(fmt.Sprint("n", i), n); err != nil {
			t.Fatal(err)
		}
	}
	floats := []float32{0, 1.5, -2.25, math.MaxFloat32, math.SmallestNonzeroFloat32, float32(math.Inf(-1))}
	for i, f := range floats {
		if err := props.SetFloatProperty(fmt.Sprint("f", i), f); err != nil {
			t.Fatal(err)
		}
	}
	if err := props.SetBooleanProperty("yes", true); err != nil {
		t.Fatal(err)
	}
	if err := props.SetBooleanProperty("no", false); err != nil {
		t.Fatal(err)
	}
	target := new(int64)
	if err := props.SetPointerProperty("ptr", Pointer(target)); err != nil {
		t.Fatal(err)
	}
	for i, want := range numbers {
		name := fmt.Sprint("n", i)
		if got := props.NumberProperty(name, 7); got != want {
			t.Errorf("NumberProperty(%q) = %d, want %d", name, got, want)
		}
		if got := props.PropertyType(name); got != PROPERTY_TYPE_NUMBER {
			t.Errorf("PropertyType(%q) = %d, want number", name, got)
		}
	}
	for i, want := range floats {
		name := fmt.Sprint("f", i)
		if got := props.FloatProperty(name, 7); got != want {
			t.Errorf("FloatProperty(%q) = %g, want %g", name, got, want)
		}
		if got := props.PropertyType(name); got != PROPERTY_TYPE_FLOAT {
			t.Errorf("PropertyType(%q) = %d, want float", name, got)
		}
	}
	nan := float32(math.NaN())
	if err := props.SetFloatProperty("nan", nan); err != nil {
		t.Fatal(err)
	}
	if got := props.FloatProperty("nan", 7); !math.IsNaN(float64(got)) {
		t.Errorf("FloatProperty(nan) = %g", got)
	}
	if !props.BooleanProperty("yes", false) || props.BooleanProperty("no", true) {
		t.Error("boolean properties did not round-trip")
	}
	if got := props.PropertyType("yes"); got != PROPERTY_TYPE_BOOLEAN {
		t.Errorf("PropertyType(yes) = %d, want boolean", got)
	}
	if got := props.PointerProperty("ptr"); got != Pointer(target) {
		t.Errorf("PointerProperty = %p, want %p", got, target)
	}
	runtime.KeepAlive(target)
	if got := props.PropertyType("ptr"); got != PROPERTY_TYPE_POINTER {
		t.Errorf("PropertyType(ptr) = %d, want pointer", got)
	}
}

func TestPropertiesMissingNamesReturnDefaults(t *testing.T) {
	props := newTestProperties(t)
	if props.HasProperty("missing") {
		t.Error("HasProperty(missing) = true")
	}
	if got := props.PropertyType("missing"); got != PROPERTY_TYPE_INVALID {
		t.Errorf("PropertyType(missing) = %d", got)
	}
	if got := props.StringProperty("missing", "dflt"); got != "dflt" {
		t.Errorf("StringProperty = %q", got)
	}
	if got := props.NumberProperty("missing", -42); got != -42 {
		t.Errorf("NumberProperty = %d", got)
	}
	if got := props.FloatProperty("missing", 2.5); got != 2.5 {
		t.Errorf("FloatProperty = %g", got)
	}
	if !props.BooleanProperty("missing", true) || props.BooleanProperty("missing", false) {
		t.Error("BooleanProperty ignored its default")
	}
	if got := props.PointerProperty("missing"); got != nil {
		t.Errorf("PointerProperty = %p", got)
	}
}

func TestPropertiesClearEnumerateCopy(t *testing.T) {
	props := newTestProperties(t)
	names := []string{"a", "b", "c", "ü"}
	for i, n := range names {
		if err := props.SetNumberProperty(n, int64(i)); err != nil {
			t.Fatal(err)
		}
	}
	if err := props.ClearProperty("b"); err != nil {
		t.Fatal(err)
	}
	if props.HasProperty("b") {
		t.Error("b still present after ClearProperty")
	}
	var seen []string
	cb := NewEnumeratePropertiesCallback(func(p PropertiesID, name string) {
		if p != props {
			t.Errorf("callback got props %d, want %d", p, props)
		}
		seen = append(seen, name)
	})
	if !props.EnumerateProperties(cb) {
		t.Fatal("EnumerateProperties failed")
	}
	slices.Sort(seen)
	if want := []string{"a", "c", "ü"}; !slices.Equal(seen, want) {
		t.Errorf("enumerated %q, want %q", seen, want)
	}
	dst := newTestProperties(t)
	if err := CopyProperties(props, dst); err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{"a", "c", "ü"} {
		if got, want := dst.NumberProperty(n, -1), props.NumberProperty(n, -2); got != want {
			t.Errorf("copied %q = %d, want %d", n, got, want)
		}
	}
	if dst.HasProperty("b") {
		t.Error("cleared property was copied")
	}
}

func TestNewPropertiesFromMap(t *testing.T) {
	loadSDL(t)
	cMemory := newTestSurface(t, 1, 1, PIXELFORMAT_RGBA8888)
	cAddr := uintptr(unsafe.Pointer(cMemory))
	props, err := NewProperties(map[string]any{
		"s":   "text",
		"i":   int(-3),
		"i32": int32(-4),
		"i64": int64(-5),
		"u":   uint(6),
		"u32": uint32(7),
		"f32": float32(1.5),
		"f64": float64(2.5),
		"b":   true,
		"p":   cAddr,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer props.Destroy()
	if got := props.StringProperty("s", ""); got != "text" {
		t.Errorf("s = %q", got)
	}
	for name, want := range map[string]int64{"i": -3, "i32": -4, "i64": -5, "u": 6, "u32": 7} {
		if got := props.NumberProperty(name, 0); got != want {
			t.Errorf("%s = %d, want %d", name, got, want)
		}
	}
	if props.FloatProperty("f32", 0) != 1.5 || props.FloatProperty("f64", 0) != 2.5 {
		t.Error("float values did not round-trip")
	}
	if !props.BooleanProperty("b", false) {
		t.Error("b = false")
	}
	if uintptr(props.PointerProperty("p")) != cAddr {
		t.Error("p did not round-trip")
	}
	if _, err := NewProperties(map[string]any{"bad": []int{1}}); err == nil {
		t.Error("NewProperties accepted a []int")
	}
}

func TestPropertiesInvalidAndDestroyedIDs(t *testing.T) {
	loadSDL(t)
	if err := PropertiesID(0).SetStringProperty("x", "y"); err == nil {
		t.Error("SetStringProperty on ID 0 succeeded")
	}
	if PropertiesID(0).HasProperty("x") {
		t.Error("HasProperty on ID 0 = true")
	}
	props, err := CreateProperties()
	if err != nil {
		t.Fatal(err)
	}
	if err := props.SetStringProperty("x", "y"); err != nil {
		t.Fatal(err)
	}
	props.Destroy()
	if props.HasProperty("x") {
		t.Error("HasProperty on a destroyed group = true")
	}
	if got := props.StringProperty("x", "gone"); got != "gone" {
		t.Errorf("StringProperty on a destroyed group = %q", got)
	}
	if err := props.SetStringProperty("x", "y"); err == nil {
		t.Error("SetStringProperty on a destroyed group succeeded")
	}
}

func TestPropertiesConcurrentUse(t *testing.T) {
	props := newTestProperties(t)
	var wg sync.WaitGroup
	for g := range 16 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range 500 {
				name := fmt.Sprintf("g%d.%d", g, i%20)
				value := fmt.Sprintf("v%d", i)
				if err := props.SetStringProperty(name, value); err != nil {
					t.Error(err)
					return
				}
				if got := props.StringProperty(name, ""); got != value {
					t.Errorf("%s = %q, want %q", name, got, value)
					return
				}
			}
		}()
	}
	wg.Wait()
	if err := props.Lock(); err != nil {
		t.Fatal(err)
	}
	props.Unlock()
}

func TestGlobalProperties(t *testing.T) {
	loadSDL(t)
	props, err := GetGlobalProperties()
	if err != nil {
		t.Fatal(err)
	}
	if props == 0 {
		t.Fatal("global properties ID is 0")
	}
}
