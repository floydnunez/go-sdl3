//go:build !js

package sdl

import (
	"testing"

	"github.com/Zyko0/go-sdl3/internal"
)

func TestIOStreamPrintfWritesPercentSigns(t *testing.T) {
	loadSDL(t)
	cases := []struct {
		format string
		values []any
		want   string
	}{
		{"%s", []any{"100%s done, 50%d left, %%"}, "100%s done, 50%d left, %%"},
		{"%d%%", []any{5}, "5%"},
		{"plain", nil, "plain"},
	}
	for _, c := range cases {
		stream := IOFromDynamicMem()
		if stream == nil {
			t.Fatal(internal.LastErr())
		}
		n, err := stream.Printf(c.format, c.values...)
		if err != nil {
			t.Fatalf("Printf(%q): %v", c.format, err)
		}
		if n != uint64(len(c.want)) {
			t.Errorf("Printf(%q) wrote %d bytes, want %d", c.format, n, len(c.want))
		}
		if _, err := stream.Seek(0, IO_SEEK_SET); err != nil {
			t.Fatal(err)
		}
		buf := make([]byte, len(c.want)+16)
		m, err := stream.Read(buf)
		if err != nil {
			t.Fatalf("Read after Printf(%q): %v", c.format, err)
		}
		if got := string(buf[:m]); got != c.want {
			t.Errorf("Printf(%q, %v) wrote %q, want %q", c.format, c.values, got, c.want)
		}
		if err := stream.Close(); err != nil {
			t.Fatal(err)
		}
	}
}
