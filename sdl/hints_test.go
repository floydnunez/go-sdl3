//go:build !js

package sdl

import (
	"errors"
	"runtime"
	"testing"
)

const testHint = "GO_SDL3_TEST_HINT"

func resetTestHintAfter(t *testing.T) {
	t.Helper()
	loadSDL(t)
	t.Cleanup(func() {
		if err := ResetHint(testHint); err != nil {
			t.Errorf("ResetHint(%s): %v", testHint, err)
		}
	})
}

func TestHintSetGetReset(t *testing.T) {
	resetTestHintAfter(t)
	if got := GetHint(testHint); got != "" {
		t.Fatalf("unset hint = %q", got)
	}
	for _, v := range []string{"1", "some value", "ünïcode", ""} {
		if err := SetHint(testHint, v); err != nil {
			t.Fatalf("SetHint(%q): %v", v, err)
		}
		if got := GetHint(testHint); got != v {
			t.Errorf("GetHint after SetHint(%q) = %q", v, got)
		}
	}
	if err := SetHint(testHint, "x"); err != nil {
		t.Fatal(err)
	}
	if err := ResetHint(testHint); err != nil {
		t.Fatal(err)
	}
	if got := GetHint(testHint); got != "" {
		t.Errorf("GetHint after ResetHint = %q", got)
	}
}

func TestHintBoolean(t *testing.T) {
	resetTestHintAfter(t)
	if err := SetHint(testHint, "1"); err != nil {
		t.Fatal(err)
	}
	if err := ResetHint(testHint); err != nil {
		t.Fatal(err)
	}
	if !GetHintBoolean(testHint, true) || GetHintBoolean(testHint, false) {
		t.Error("unset hint ignored the default")
	}
	cases := map[string]bool{"0": false, "false": false, "FALSE": false, "1": true, "true": true, "yes": true}
	for v, want := range cases {
		if err := SetHint(testHint, v); err != nil {
			t.Fatal(err)
		}
		if got := GetHintBoolean(testHint, !want); got != want {
			t.Errorf("GetHintBoolean with %q = %v, want %v", v, got, want)
		}
	}
}

func TestHintPriority(t *testing.T) {
	resetTestHintAfter(t)
	if err := SetHintWithPriority(testHint, "override", HINT_OVERRIDE); err != nil {
		t.Fatal(err)
	}
	if err := SetHintWithPriority(testHint, "normal", HINT_NORMAL); err == nil {
		t.Error("a normal-priority set over an override succeeded")
	}
	if err := SetHintWithPriority(testHint, "default", HINT_DEFAULT); err == nil {
		t.Error("a default-priority set over an override succeeded")
	}
	if err := SetHint(testHint, "plain"); err == nil {
		t.Error("SetHint over an override succeeded")
	}
	if got := GetHint(testHint); got != "override" {
		t.Errorf("lower priorities replaced an override: %q", got)
	}
	if err := SetHintWithPriority(testHint, "override 2", HINT_OVERRIDE); err != nil {
		t.Fatal(err)
	}
	if got := GetHint(testHint); got != "override 2" {
		t.Errorf("an override did not replace an override: %q", got)
	}
	ResetHints()
	if got := GetHint(testHint); got != "" {
		t.Errorf("GetHint after ResetHints = %q", got)
	}
}

func TestHintFailuresWithoutAnSDLMessageAreErrors(t *testing.T) {
	loadSDL(t)
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	iClearError()
	if err := ResetHint("GO_SDL3_TEST_HINT_NEVER_SET"); !errors.Is(err, ErrFailedWithoutMessage) {
		t.Errorf("ResetHint of a hint never set = %v, want ErrFailedWithoutMessage", err)
	}
	resetTestHintAfter(t)
	if err := SetHintWithPriority(testHint, "override", HINT_OVERRIDE); err != nil {
		t.Fatal(err)
	}
	iClearError()
	if err := SetHint(testHint, "normal"); !errors.Is(err, ErrFailedWithoutMessage) {
		t.Errorf("SetHint over an override = %v, want ErrFailedWithoutMessage", err)
	}
}

func TestHintEmptyName(t *testing.T) {
	loadSDL(t)
	if err := SetHint("", "x"); err == nil {
		t.Error("SetHint with an empty name succeeded")
	}
	if got := GetHint(""); got != "" {
		t.Errorf("GetHint(\"\") = %q", got)
	}
}
