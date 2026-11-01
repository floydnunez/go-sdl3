//go:build darwin || (linux && (amd64 || arm64))

package sdl

import (
	"sync/atomic"
	"unsafe"

	_ "github.com/ebitengine/purego" // its syscall15X trampoline
)

// hotcall calls the C function fn with up to six integer or pointer
// arguments as purego's syscall_syscall15X does, through its syscall15X
// trampoline and runtime.cgocall, but from a call block kept in the
// package (hotArgs) instead of one built on the heap each call. It
// returns the function's integer result. One call at a time: it panics
// when another goroutine is inside it.
func hotcall(fn, a1, a2, a3, a4, a5, a6 uintptr) uintptr {
	if !hotBusy.CompareAndSwap(false, true) {
		panic("sdl: hotcall from two goroutines at once (GPU calls must come from one)")
	}
	hot = hotArgs{
		fn: fn, a1: a1, a2: a2, a3: a3, a4: a4, a5: a5, a6: a6,
		// purego's syscall_syscall15X copies the integers into the float
		// registers' slots too; none of these calls takes a float.
		f1: a1, f2: a2, f3: a3, f4: a4, f5: a5, f6: a6,
	}
	runtime_cgocall(syscall15XABI0, unsafe.Pointer(&hot))
	r := hot.a1
	hotBusy.Store(false)
	return r
}

// hotArgs is purego v0.10.2's syscall15Args on 64-bit System V: the
// trampoline reads the function and its arguments from it and writes the
// results back into a1..a3. Its layout must match purego's (see
// TestHotcall, which calls SDL both ways).
type hotArgs struct {
	fn, a1, a2, a3, a4, a5, a6, a7, a8, a9, a10, a11, a12, a13, a14, a15 uintptr
	f1, f2, f3, f4, f5, f6, f7, f8                                       uintptr
	arm64_r8                                                             uintptr
}

var (
	hot     hotArgs
	hotBusy atomic.Bool
)

// runtime.cgocall runs fn on the system stack with arg; the runtime keeps
// it linkable for purego (go.dev/issue/67401).
//
//go:linkname runtime_cgocall runtime.cgocall
func runtime_cgocall(fn uintptr, arg unsafe.Pointer) int32

// syscall15XABI0 is the address of purego's syscall15X trampoline.
//
//go:linkname syscall15XABI0 github.com/ebitengine/purego.syscall15XABI0
var syscall15XABI0 uintptr
