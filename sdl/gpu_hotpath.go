package sdl

import (
	"runtime"
	"unsafe"
)

// syscall_syscall15X is the fixed-arity syscall function from purego.
// Using go:linkname to access it directly avoids the heap allocation
// from SyscallN's variadic slice parameter.
//
//go:linkname syscall_syscall15X github.com/ebitengine/purego.syscall_syscall15X
func syscall_syscall15X(fn, a1, a2, a3, a4, a5, a6, a7, a8, a9, a10, a11, a12, a13, a14, a15 uintptr) (r1, r2, err uintptr)

// InitGPUHotpath replaces the auto-generated closures for the 4 hottest GPU
// functions with zero-allocation versions that bypass SyscallN's variadic args.
// Call this once after sdl.Init().
func InitGPUHotpath() {
	iDrawGPUIndexedPrimitives = drawGPUIndexedPrimitivesNoAlloc
	iBindGPUFragmentSamplers = bindGPUFragmentSamplersNoAlloc
	iPushGPUFragmentUniformData = pushGPUFragmentUniformDataNoAlloc
	iPushGPUVertexUniformData = pushGPUVertexUniformDataNoAlloc
}

func drawGPUIndexedPrimitivesNoAlloc(render_pass *GPURenderPass, num_indices uint32, num_instances uint32, first_index uint32, vertex_offset int32, first_instance uint32) {
	syscall_syscall15X(_addr_SDL_DrawGPUIndexedPrimitives,
		uintptr(unsafe.Pointer(render_pass)), uintptr(num_indices), uintptr(num_instances),
		uintptr(first_index), uintptr(vertex_offset), uintptr(first_instance),
		0, 0, 0, 0, 0, 0, 0, 0, 0)
	runtime.KeepAlive(render_pass)
}

func bindGPUFragmentSamplersNoAlloc(render_pass *GPURenderPass, first_slot uint32, texture_sampler_bindings *GPUTextureSamplerBinding, num_bindings uint32) {
	syscall_syscall15X(_addr_SDL_BindGPUFragmentSamplers,
		uintptr(unsafe.Pointer(render_pass)), uintptr(first_slot),
		uintptr(unsafe.Pointer(texture_sampler_bindings)), uintptr(num_bindings),
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0)
	runtime.KeepAlive(render_pass)
	runtime.KeepAlive(texture_sampler_bindings)
}

func pushGPUFragmentUniformDataNoAlloc(command_buffer *GPUCommandBuffer, slot_index uint32, data uintptr, length uint32) {
	syscall_syscall15X(_addr_SDL_PushGPUFragmentUniformData,
		uintptr(unsafe.Pointer(command_buffer)), uintptr(slot_index),
		data, uintptr(length),
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0)
	runtime.KeepAlive(command_buffer)
}

func pushGPUVertexUniformDataNoAlloc(command_buffer *GPUCommandBuffer, slot_index uint32, data uintptr, length uint32) {
	syscall_syscall15X(_addr_SDL_PushGPUVertexUniformData,
		uintptr(unsafe.Pointer(command_buffer)), uintptr(slot_index),
		data, uintptr(length),
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0)
	runtime.KeepAlive(command_buffer)
}
