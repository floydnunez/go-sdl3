package sdl

import (
	"runtime"
	"unsafe"
)

// InitGPUHotpath replaces the auto-generated closures of the calls a GPU
// frame makes with versions that allocate nothing (hotcall): purego's
// SyscallN takes its arguments as a variadic slice and builds its call
// block on the heap, a few hundred bytes a call. Call this once after
// sdl.Init(), and make these calls from one goroutine (the one SDL's GPU
// runs on); on System V platforms hotcall panics if two overlap.
func InitGPUHotpath() {
	iDrawGPUIndexedPrimitives = drawGPUIndexedPrimitivesNoAlloc
	iDrawGPUPrimitives = drawGPUPrimitivesNoAlloc
	iBindGPUFragmentSamplers = bindGPUFragmentSamplersNoAlloc
	iBindGPUGraphicsPipeline = bindGPUGraphicsPipelineNoAlloc
	iBindGPUVertexBuffers = bindGPUVertexBuffersNoAlloc
	iBindGPUIndexBuffer = bindGPUIndexBufferNoAlloc
	iPushGPUFragmentUniformData = pushGPUFragmentUniformDataNoAlloc
	iPushGPUVertexUniformData = pushGPUVertexUniformDataNoAlloc
	iBeginGPURenderPass = beginGPURenderPassNoAlloc
	iEndGPURenderPass = endGPURenderPassNoAlloc
	iMapGPUTransferBuffer = mapGPUTransferBufferNoAlloc
	iUnmapGPUTransferBuffer = unmapGPUTransferBufferNoAlloc
	iWaitAndAcquireGPUSwapchainTexture = waitAndAcquireGPUSwapchainTextureNoAlloc
	iPollEvent = pollEventNoAlloc
}

func boolArg(b bool) uintptr {
	if b {
		return 1
	}
	return 0
}

func drawGPUIndexedPrimitivesNoAlloc(render_pass *GPURenderPass, num_indices uint32, num_instances uint32, first_index uint32, vertex_offset int32, first_instance uint32) {
	hotcall(_addr_SDL_DrawGPUIndexedPrimitives, uintptr(unsafe.Pointer(render_pass)), uintptr(num_indices), uintptr(num_instances), uintptr(first_index), uintptr(vertex_offset), uintptr(first_instance))
	runtime.KeepAlive(render_pass)
}

func drawGPUPrimitivesNoAlloc(render_pass *GPURenderPass, num_vertices uint32, num_instances uint32, first_vertex uint32, first_instance uint32) {
	hotcall(_addr_SDL_DrawGPUPrimitives, uintptr(unsafe.Pointer(render_pass)), uintptr(num_vertices), uintptr(num_instances), uintptr(first_vertex), uintptr(first_instance), 0)
	runtime.KeepAlive(render_pass)
}

func bindGPUFragmentSamplersNoAlloc(render_pass *GPURenderPass, first_slot uint32, texture_sampler_bindings *GPUTextureSamplerBinding, num_bindings uint32) {
	hotcall(_addr_SDL_BindGPUFragmentSamplers, uintptr(unsafe.Pointer(render_pass)), uintptr(first_slot), uintptr(unsafe.Pointer(texture_sampler_bindings)), uintptr(num_bindings), 0, 0)
	runtime.KeepAlive(render_pass)
	runtime.KeepAlive(texture_sampler_bindings)
}

func bindGPUGraphicsPipelineNoAlloc(render_pass *GPURenderPass, graphics_pipeline *GPUGraphicsPipeline) {
	hotcall(_addr_SDL_BindGPUGraphicsPipeline, uintptr(unsafe.Pointer(render_pass)), uintptr(unsafe.Pointer(graphics_pipeline)), 0, 0, 0, 0)
	runtime.KeepAlive(render_pass)
	runtime.KeepAlive(graphics_pipeline)
}

func bindGPUVertexBuffersNoAlloc(render_pass *GPURenderPass, first_slot uint32, bindings *GPUBufferBinding, num_bindings uint32) {
	hotcall(_addr_SDL_BindGPUVertexBuffers, uintptr(unsafe.Pointer(render_pass)), uintptr(first_slot), uintptr(unsafe.Pointer(bindings)), uintptr(num_bindings), 0, 0)
	runtime.KeepAlive(render_pass)
	runtime.KeepAlive(bindings)
}

func bindGPUIndexBufferNoAlloc(render_pass *GPURenderPass, binding *GPUBufferBinding, index_element_size GPUIndexElementSize) {
	hotcall(_addr_SDL_BindGPUIndexBuffer, uintptr(unsafe.Pointer(render_pass)), uintptr(unsafe.Pointer(binding)), uintptr(index_element_size), 0, 0, 0)
	runtime.KeepAlive(render_pass)
	runtime.KeepAlive(binding)
}

func pushGPUFragmentUniformDataNoAlloc(command_buffer *GPUCommandBuffer, slot_index uint32, data uintptr, length uint32) {
	hotcall(_addr_SDL_PushGPUFragmentUniformData, uintptr(unsafe.Pointer(command_buffer)), uintptr(slot_index), data, uintptr(length), 0, 0)
	runtime.KeepAlive(command_buffer)
}

func pushGPUVertexUniformDataNoAlloc(command_buffer *GPUCommandBuffer, slot_index uint32, data uintptr, length uint32) {
	hotcall(_addr_SDL_PushGPUVertexUniformData, uintptr(unsafe.Pointer(command_buffer)), uintptr(slot_index), data, uintptr(length), 0, 0)
	runtime.KeepAlive(command_buffer)
}

func beginGPURenderPassNoAlloc(command_buffer *GPUCommandBuffer, color_target_infos *GPUColorTargetInfo, num_color_targets uint32, depth_stencil_target_info *GPUDepthStencilTargetInfo) *GPURenderPass {
	r := hotcall(_addr_SDL_BeginGPURenderPass, uintptr(unsafe.Pointer(command_buffer)), uintptr(unsafe.Pointer(color_target_infos)), uintptr(num_color_targets), uintptr(unsafe.Pointer(depth_stencil_target_info)), 0, 0)
	runtime.KeepAlive(command_buffer)
	runtime.KeepAlive(color_target_infos)
	runtime.KeepAlive(depth_stencil_target_info)
	return (*GPURenderPass)(*(*unsafe.Pointer)(unsafe.Pointer(&r)))
}

func endGPURenderPassNoAlloc(render_pass *GPURenderPass) {
	hotcall(_addr_SDL_EndGPURenderPass, uintptr(unsafe.Pointer(render_pass)), 0, 0, 0, 0, 0)
	runtime.KeepAlive(render_pass)
}

func mapGPUTransferBufferNoAlloc(device *GPUDevice, transfer_buffer *GPUTransferBuffer, cycle bool) uintptr {
	r := hotcall(_addr_SDL_MapGPUTransferBuffer, uintptr(unsafe.Pointer(device)), uintptr(unsafe.Pointer(transfer_buffer)), boolArg(cycle), 0, 0, 0)
	runtime.KeepAlive(device)
	runtime.KeepAlive(transfer_buffer)
	return r
}

func unmapGPUTransferBufferNoAlloc(device *GPUDevice, transfer_buffer *GPUTransferBuffer) {
	hotcall(_addr_SDL_UnmapGPUTransferBuffer, uintptr(unsafe.Pointer(device)), uintptr(unsafe.Pointer(transfer_buffer)), 0, 0, 0, 0)
	runtime.KeepAlive(device)
	runtime.KeepAlive(transfer_buffer)
}

func waitAndAcquireGPUSwapchainTextureNoAlloc(command_buffer *GPUCommandBuffer, window *Window, swapchain_texture **GPUTexture, swapchain_texture_width *uint32, swapchain_texture_height *uint32) bool {
	r := hotcall(_addr_SDL_WaitAndAcquireGPUSwapchainTexture, uintptr(unsafe.Pointer(command_buffer)), uintptr(unsafe.Pointer(window)), uintptr(unsafe.Pointer(swapchain_texture)), uintptr(unsafe.Pointer(swapchain_texture_width)), uintptr(unsafe.Pointer(swapchain_texture_height)), 0)
	runtime.KeepAlive(command_buffer)
	runtime.KeepAlive(window)
	runtime.KeepAlive(swapchain_texture)
	runtime.KeepAlive(swapchain_texture_width)
	runtime.KeepAlive(swapchain_texture_height)
	return uint8(r) != 0
}

func pollEventNoAlloc(event *Event) bool {
	r := hotcall(_addr_SDL_PollEvent, uintptr(unsafe.Pointer(event)), 0, 0, 0, 0, 0)
	runtime.KeepAlive(event)
	return uint8(r) != 0
}
