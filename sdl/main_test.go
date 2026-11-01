//go:build !js

package sdl

import (
	"os"
	"runtime"
	"testing"
)

func init() {
	runtime.LockOSThread()
}

var mainThreadCalls = make(chan func())

func TestMain(m *testing.M) {
	done := make(chan int)
	go func() {
		done <- m.Run()
	}()
	for {
		select {
		case f := <-mainThreadCalls:
			f()
		case code := <-done:
			os.Exit(code)
		}
	}
}

func onMainThread(f func() error) error {
	result := make(chan error)
	mainThreadCalls <- func() {
		result <- f()
	}
	return <-result
}
