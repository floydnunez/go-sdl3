//go:build !js && !(darwin || (linux && (amd64 || arm64)))

package sdl

import (
	_ "unsafe" // go:linkname
)

// hotcall calls the C function fn with up to six integer or pointer
// arguments through purego's syscall_syscall15X, skipping SyscallN's
// variadic slice. On Windows purego hands the call to the syscall
// package, which builds no call block on the heap.
func hotcall(fn, a1, a2, a3, a4, a5, a6 uintptr) uintptr {
	r, _, _ := syscall_syscall15X(fn, a1, a2, a3, a4, a5, a6, 0, 0, 0, 0, 0, 0, 0, 0, 0)
	return r
}

//go:linkname syscall_syscall15X github.com/ebitengine/purego.syscall_syscall15X
func syscall_syscall15X(fn, a1, a2, a3, a4, a5, a6, a7, a8, a9, a10, a11, a12, a13, a14, a15 uintptr) (r1, r2, err uintptr)
