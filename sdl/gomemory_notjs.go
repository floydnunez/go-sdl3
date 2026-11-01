//go:build !js

package sdl

import (
	"fmt"
	"sync"
	"unsafe"

	"github.com/Zyko0/go-sdl3/internal"
)

const goMemoryProperty = "go-sdl3.go-memory"

type retainedGoMemory struct {
	mem  []byte
	refs int
}

var (
	retainedGoMemoryMu sync.Mutex
	retainedGoMemoryAt = map[uintptr]*retainedGoMemory{}
	releaseGoMemoryCB  = sync.OnceValue(func() CleanupPropertyCallback {
		return NewCleanupPropertyCallback(releaseGoMemory)
	})
)

func retainGoMemory(props PropertiesID, mem []byte) error {
	if len(mem) == 0 {
		return nil
	}
	addr := uintptr(unsafe.Pointer(unsafe.SliceData(mem)))
	retainedGoMemoryMu.Lock()
	r, ok := retainedGoMemoryAt[addr]
	if !ok {
		r = &retainedGoMemory{mem: mem}
		retainedGoMemoryAt[addr] = r
	}
	r.refs++
	retainedGoMemoryMu.Unlock()
	if !iSetPointerPropertyWithCleanup(props, goMemoryProperty, addr, releaseGoMemoryCB(), 0) {
		return internal.LastErr()
	}
	return nil
}

func releaseGoMemory(addr uintptr) {
	retainedGoMemoryMu.Lock()
	defer retainedGoMemoryMu.Unlock()
	r, ok := retainedGoMemoryAt[addr]
	if !ok {
		panic(fmt.Sprintf("sdl: SDL released Go memory at %#x that was not retained", addr))
	}
	r.refs--
	if r.refs == 0 {
		delete(retainedGoMemoryAt, addr)
	}
}
