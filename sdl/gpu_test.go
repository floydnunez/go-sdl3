//go:build !js

package sdl

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"runtime"
	"testing"

	"github.com/Zyko0/go-sdl3/internal"
)

const testVertexMSL = `#include <metal_stdlib>
using namespace metal;
struct VIn { float2 pos [[attribute(0)]]; float2 uv [[attribute(1)]]; };
struct VOut { float4 pos [[position]]; float2 uv; };
struct VU { float2 scale; float2 offset; };
vertex VOut main0(VIn in [[stage_in]], constant VU& u [[buffer(0)]]) {
	VOut o;
	o.pos = float4(in.pos * u.scale + u.offset, 0.0, 1.0);
	o.uv = in.uv;
	return o;
}
`

const testFragmentMSL = `#include <metal_stdlib>
using namespace metal;
struct VOut { float4 pos [[position]]; float2 uv; };
struct FU { float4 tint; };
fragment float4 main0(VOut in [[stage_in]], constant FU& u [[buffer(0)]], texture2d<float> tex [[texture(0)]], sampler smp [[sampler(0)]]) {
	return tex.sample(smp, in.uv) * u.tint;
}
`

const gpuTargetSize = 4

var (
	quadCorners   = [4][4]float32{{0, 0, 0, 1}, {1, 0, 1, 1}, {1, 1, 1, 0}, {0, 1, 0, 0}}
	quadIndices   = [6]uint16{0, 1, 2, 0, 2, 3}
	quadTexels    = [4]rgba{red, green, blue, {255, 255, 255, 255}}
	quadTint      = [4]float32{0.5, 1, 1, 1}
	quadPlacement = [4]float32{2, 2, -1, -1}
)

type gpuScene struct {
	videoStarted  bool
	device        *GPUDevice
	shaders       []*GPUShader
	pipeline      *GPUGraphicsPipeline
	buffers       []*GPUBuffer
	textures      []*GPUTexture
	sampler       *GPUSampler
	transfers     []*GPUTransferBuffer
	quad          *GPUBuffer
	quadList      *GPUBuffer
	indices       *GPUBuffer
	texture       *GPUTexture
	target        *GPUTexture
	download      *GPUTransferBuffer
	vertexUniform []byte
	fragUniform   []byte
	samplers      []GPUTextureSamplerBinding
	indexed       []GPUBufferBinding
	listed        []GPUBufferBinding
	indexBinding  *GPUBufferBinding
	colorTargets  []GPUColorTargetInfo
}

func float32Bytes(v []float32) []byte {
	b := make([]byte, 4*len(v))
	for i, f := range v {
		binary.NativeEndian.PutUint32(b[4*i:], math.Float32bits(f))
	}
	return b
}

func startGPUScene(t *testing.T) *gpuScene {
	t.Helper()
	loadSDL(t)
	if runtime.GOOS != "darwin" {
		t.Skip("the GPU tests use Metal shaders")
	}
	var s *gpuScene
	err := onMainThread(func() error {
		var err error
		s, err = newGPUScene()
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := onMainThread(s.close); err != nil {
			t.Error(err)
		}
	})
	return s
}

func newGPUScene() (*gpuScene, error) {
	s := &gpuScene{}
	if err := s.build(); err != nil {
		return nil, errors.Join(err, s.close())
	}
	return s, nil
}

func (s *gpuScene) close() error {
	var errs []error
	if s.device != nil {
		for _, sh := range s.shaders {
			s.device.ReleaseShader(sh)
		}
		if s.pipeline != nil {
			s.device.ReleaseGraphicsPipeline(s.pipeline)
		}
		for _, b := range s.buffers {
			s.device.ReleaseBuffer(b)
		}
		for _, tex := range s.textures {
			s.device.ReleaseTexture(tex)
		}
		if s.sampler != nil {
			s.device.ReleaseSampler(s.sampler)
		}
		for _, tb := range s.transfers {
			s.device.ReleaseTransferBuffer(tb)
		}
		s.device.Destroy()
	}
	if s.videoStarted {
		QuitSubSystem(INIT_VIDEO)
		if err := ResetHint(HINT_MAC_BACKGROUND_APP); err != nil {
			errs = append(errs, fmt.Errorf("resetting %s: %w", HINT_MAC_BACKGROUND_APP, err))
		}
	}
	return errors.Join(errs...)
}

func (s *gpuScene) build() error {
	if err := SetHint(HINT_MAC_BACKGROUND_APP, "1"); err != nil {
		return fmt.Errorf("setting %s: %w", HINT_MAC_BACKGROUND_APP, err)
	}
	if err := InitSubSystem(INIT_VIDEO); err != nil {
		return fmt.Errorf("starting video: %w", err)
	}
	s.videoStarted = true
	device, err := CreateGPUDevice(GPU_SHADERFORMAT_MSL, true, "")
	if err != nil {
		return fmt.Errorf("creating the GPU device: %w", err)
	}
	s.device = device
	if got := device.Driver(); got != "metal" {
		return fmt.Errorf("driver = %q, want metal", got)
	}
	if device.ShaderFormats()&GPU_SHADERFORMAT_MSL == 0 {
		return fmt.Errorf("device shader formats %#x lack MSL", device.ShaderFormats())
	}

	newShader := func(src string, stage GPUShaderStage, samplers uint32) (*GPUShader, error) {
		sh, err := device.CreateGPUShader(&GPUShaderCreateInfo{
			Code: []byte(src), Entrypoint: "main0", Format: GPU_SHADERFORMAT_MSL,
			Stage: stage, NumSamplers: samplers, NumUniformBuffers: 1,
		})
		if err != nil {
			return nil, fmt.Errorf("creating the stage %d shader: %w", stage, err)
		}
		s.shaders = append(s.shaders, sh)
		return sh, nil
	}
	vs, err := newShader(testVertexMSL, GPU_SHADERSTAGE_VERTEX, 0)
	if err != nil {
		return err
	}
	fs, err := newShader(testFragmentMSL, GPU_SHADERSTAGE_FRAGMENT, 1)
	if err != nil {
		return err
	}
	s.pipeline, err = device.CreateGraphicsPipeline(&GPUGraphicsPipelineCreateInfo{
		VertexShader:   vs,
		FragmentShader: fs,
		VertexInputState: GPUVertexInputState{
			VertexBufferDescriptions: []GPUVertexBufferDescription{{Slot: 0, Pitch: 16, InputRate: GPU_VERTEXINPUTRATE_VERTEX}},
			VertexAttributes: []GPUVertexAttribute{
				{Location: 0, BufferSlot: 0, Format: GPU_VERTEXELEMENTFORMAT_FLOAT2, Offset: 0},
				{Location: 1, BufferSlot: 0, Format: GPU_VERTEXELEMENTFORMAT_FLOAT2, Offset: 8},
			},
		},
		PrimitiveType: GPU_PRIMITIVETYPE_TRIANGLELIST,
		TargetInfo: GPUGraphicsPipelineTargetInfo{
			ColorTargetDescriptions: []GPUColorTargetDescription{{Format: GPU_TEXTUREFORMAT_R8G8B8A8_UNORM}},
		},
	})
	if err != nil {
		return fmt.Errorf("creating the pipeline: %w", err)
	}

	var quad, list []float32
	for _, c := range quadCorners {
		quad = append(quad, c[:]...)
	}
	for _, i := range quadIndices {
		list = append(list, quadCorners[i][:]...)
	}
	quadBytes, listBytes := float32Bytes(quad), float32Bytes(list)
	indexBytes := make([]byte, 2*len(quadIndices))
	for i, v := range quadIndices {
		binary.NativeEndian.PutUint16(indexBytes[2*i:], v)
	}
	texelBytes := make([]byte, 0, 16)
	for _, c := range quadTexels {
		texelBytes = append(texelBytes, c.r, c.g, c.b, c.a)
	}

	newBuffer := func(usage GPUBufferUsageFlags, size int) (*GPUBuffer, error) {
		b, err := device.CreateBuffer(&GPUBufferCreateInfo{Usage: usage, Size: uint32(size)})
		if err != nil {
			return nil, fmt.Errorf("creating a %d-byte buffer: %w", size, err)
		}
		s.buffers = append(s.buffers, b)
		return b, nil
	}
	if s.quad, err = newBuffer(GPU_BUFFERUSAGE_VERTEX, len(quadBytes)); err != nil {
		return err
	}
	if s.quadList, err = newBuffer(GPU_BUFFERUSAGE_VERTEX, len(listBytes)); err != nil {
		return err
	}
	if s.indices, err = newBuffer(GPU_BUFFERUSAGE_INDEX, len(indexBytes)); err != nil {
		return err
	}
	newTexture := func(usage GPUTextureUsageFlags, size uint32) (*GPUTexture, error) {
		tex, err := device.CreateTexture(&GPUTextureCreateInfo{
			Type: GPU_TEXTURETYPE_2D, Format: GPU_TEXTUREFORMAT_R8G8B8A8_UNORM, Usage: usage,
			Width: size, Height: size, LayerCountOrDepth: 1, NumLevels: 1, SampleCount: GPU_SAMPLECOUNT_1,
		})
		if err != nil {
			return nil, fmt.Errorf("creating a %dx%d texture: %w", size, size, err)
		}
		s.textures = append(s.textures, tex)
		return tex, nil
	}
	if s.texture, err = newTexture(GPU_TEXTUREUSAGE_SAMPLER, 2); err != nil {
		return err
	}
	if s.target, err = newTexture(GPU_TEXTUREUSAGE_COLOR_TARGET, gpuTargetSize); err != nil {
		return err
	}
	s.sampler, err = device.CreateSampler(&GPUSamplerCreateInfo{
		MinFilter: GPU_FILTER_NEAREST, MagFilter: GPU_FILTER_NEAREST, MipmapMode: GPU_SAMPLERMIPMAPMODE_NEAREST,
		AddressModeU: GPU_SAMPLERADDRESSMODE_CLAMP_TO_EDGE, AddressModeV: GPU_SAMPLERADDRESSMODE_CLAMP_TO_EDGE,
		AddressModeW: GPU_SAMPLERADDRESSMODE_CLAMP_TO_EDGE,
	})
	if err != nil {
		return fmt.Errorf("creating the sampler: %w", err)
	}
	newTransfer := func(usage GPUTransferBufferUsage, size int) (*GPUTransferBuffer, error) {
		tb, err := device.CreateTransferBuffer(&GPUTransferBufferCreateInfo{Usage: usage, Size: uint32(size)})
		if err != nil {
			return nil, fmt.Errorf("creating a %d-byte transfer buffer: %w", size, err)
		}
		s.transfers = append(s.transfers, tb)
		return tb, nil
	}
	if s.download, err = newTransfer(GPU_TRANSFERBUFFERUSAGE_DOWNLOAD, gpuTargetSize*gpuTargetSize*4); err != nil {
		return err
	}
	parts := [][]byte{quadBytes, listBytes, indexBytes, texelBytes}
	uploadSize := 0
	for _, p := range parts {
		uploadSize += len(p)
	}
	upload, err := newTransfer(GPU_TRANSFERBUFFERUSAGE_UPLOAD, uploadSize)
	if err != nil {
		return err
	}
	ptr, err := device.MapTransferBuffer(upload, false)
	if err != nil {
		return fmt.Errorf("mapping the upload buffer: %w", err)
	}
	staging := internal.PtrToSlice[byte](ptr, uploadSize)
	offsets := make([]uint32, len(parts))
	off := 0
	for i, p := range parts {
		offsets[i] = uint32(off)
		off += copy(staging[off:], p)
	}
	device.UnmapTransferBuffer(upload)

	cb, err := device.AcquireCommandBuffer()
	if err != nil {
		return fmt.Errorf("acquiring a command buffer: %w", err)
	}
	cp := cb.BeginCopyPass()
	for i, dst := range []*GPUBuffer{s.quad, s.quadList, s.indices} {
		cp.UploadToGPUBuffer(&GPUTransferBufferLocation{TransferBuffer: upload, Offset: offsets[i]},
			&GPUBufferRegion{Buffer: dst, Size: uint32(len(parts[i]))}, false)
	}
	cp.UploadToGPUTexture(&GPUTextureTransferInfo{TransferBuffer: upload, Offset: offsets[3], PixelsPerRow: 2, RowsPerLayer: 2},
		&GPUTextureRegion{Texture: s.texture, W: 2, H: 2, D: 1}, false)
	cp.End()
	if err := s.submitAndWait(cb); err != nil {
		return fmt.Errorf("uploading: %w", err)
	}

	s.vertexUniform = float32Bytes(quadPlacement[:])
	s.fragUniform = float32Bytes(quadTint[:])
	s.samplers = []GPUTextureSamplerBinding{{Texture: s.texture, Sampler: s.sampler}}
	s.indexed = []GPUBufferBinding{{Buffer: s.quad}}
	s.listed = []GPUBufferBinding{{Buffer: s.quadList}}
	s.indexBinding = &GPUBufferBinding{Buffer: s.indices}
	s.colorTargets = []GPUColorTargetInfo{{
		Texture: s.target, ClearColor: FColor{0, 0, 0, 1}, LoadOp: GPU_LOADOP_CLEAR, StoreOp: GPU_STOREOP_STORE,
	}}
	return nil
}

func (s *gpuScene) submitAndWait(cb *GPUCommandBuffer) error {
	fence, err := cb.SubmitAndAcquireFence()
	if err != nil {
		return fmt.Errorf("submitting: %w", err)
	}
	defer s.device.ReleaseFence(fence)
	if err := s.device.WaitForFences(true, []*GPUFence{fence}); err != nil {
		return fmt.Errorf("waiting for the fence: %w", err)
	}
	v := GetVersion()
	want := !metalQueryFenceIsInverted(v)
	if got := s.device.QueryFence(fence); got != want {
		return fmt.Errorf("QueryFence after WaitForFences = %v, want %v on SDL %s", got, want, v)
	}
	return nil
}

func metalQueryFenceIsInverted(v Version) bool {
	return v.Major() == 3 && v.Minor() == 4 && v.Patch() == 14
}

func (s *gpuScene) record(rp *GPURenderPass, cb *GPUCommandBuffer, indexed bool) {
	rp.BindGraphicsPipeline(s.pipeline)
	rp.BindFragmentSamplers(s.samplers)
	cb.PushVertexUniformData(0, s.vertexUniform)
	cb.PushFragmentUniformData(0, s.fragUniform)
	if indexed {
		rp.BindVertexBuffers(s.indexed)
		rp.BindIndexBuffer(s.indexBinding, GPU_INDEXELEMENTSIZE_16BIT)
		rp.DrawIndexedPrimitives(uint32(len(quadIndices)), 1, 0, 0, 0)
	} else {
		rp.BindVertexBuffers(s.listed)
		rp.DrawPrimitives(uint32(len(quadIndices)), 1, 0, 0)
	}
}

func (s *gpuScene) downloadTarget(cb *GPUCommandBuffer) ([]byte, error) {
	cp := cb.BeginCopyPass()
	cp.DownloadFromGPUTexture(&GPUTextureRegion{Texture: s.target, W: gpuTargetSize, H: gpuTargetSize, D: 1},
		&GPUTextureTransferInfo{TransferBuffer: s.download, PixelsPerRow: gpuTargetSize, RowsPerLayer: gpuTargetSize})
	cp.End()
	if err := s.submitAndWait(cb); err != nil {
		return nil, err
	}
	ptr, err := s.device.MapTransferBuffer(s.download, false)
	if err != nil {
		return nil, fmt.Errorf("mapping the download buffer: %w", err)
	}
	out := bytes.Clone(internal.PtrToSlice[byte](ptr, gpuTargetSize*gpuTargetSize*4))
	s.device.UnmapTransferBuffer(s.download)
	return out, nil
}

func (s *gpuScene) render(t *testing.T, indexed bool) []byte {
	t.Helper()
	var pixels []byte
	err := onMainThread(func() error {
		cb, err := s.device.AcquireCommandBuffer()
		if err != nil {
			return fmt.Errorf("acquiring a command buffer: %w", err)
		}
		rp := cb.BeginRenderPass(s.colorTargets, nil)
		if rp == nil {
			return errors.Join(errors.New("BeginRenderPass returned nil"), cb.Cancel())
		}
		s.record(rp, cb, indexed)
		rp.End()
		pixels, err = s.downloadTarget(cb)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return pixels
}

func expectQuadPixels(t *testing.T, pixels []byte) {
	t.Helper()
	for y := range gpuTargetSize {
		for x := range gpuTargetSize {
			texel := quadTexels[(y/2)*2+x/2]
			want := rgba{
				uint8(math.Round(float64(texel.r) * float64(quadTint[0]))),
				uint8(math.Round(float64(texel.g) * float64(quadTint[1]))),
				uint8(math.Round(float64(texel.b) * float64(quadTint[2]))),
				uint8(math.Round(float64(texel.a) * float64(quadTint[3]))),
			}
			o := (y*gpuTargetSize + x) * 4
			got := rgba{pixels[o], pixels[o+1], pixels[o+2], pixels[o+3]}
			if !got.near(want, 1) {
				t.Errorf("pixel (%d, %d) = %v, want %v", x, y, got, want)
			}
		}
	}
}

func withGPUHotpath(t *testing.T) {
	t.Helper()
	drawIndexed, draw, fragSamplers, pipeline := iDrawGPUIndexedPrimitives, iDrawGPUPrimitives, iBindGPUFragmentSamplers, iBindGPUGraphicsPipeline
	vertexBuffers, indexBuffer, fragUniform, vertexUniform := iBindGPUVertexBuffers, iBindGPUIndexBuffer, iPushGPUFragmentUniformData, iPushGPUVertexUniformData
	beginPass, endPass, mapTransfer, unmapTransfer := iBeginGPURenderPass, iEndGPURenderPass, iMapGPUTransferBuffer, iUnmapGPUTransferBuffer
	acquireSwapchain, pollEvent := iWaitAndAcquireGPUSwapchainTexture, iPollEvent
	InitGPUHotpath()
	t.Cleanup(func() {
		iDrawGPUIndexedPrimitives, iDrawGPUPrimitives, iBindGPUFragmentSamplers, iBindGPUGraphicsPipeline = drawIndexed, draw, fragSamplers, pipeline
		iBindGPUVertexBuffers, iBindGPUIndexBuffer, iPushGPUFragmentUniformData, iPushGPUVertexUniformData = vertexBuffers, indexBuffer, fragUniform, vertexUniform
		iBeginGPURenderPass, iEndGPURenderPass, iMapGPUTransferBuffer, iUnmapGPUTransferBuffer = beginPass, endPass, mapTransfer, unmapTransfer
		iWaitAndAcquireGPUSwapchainTexture, iPollEvent = acquireSwapchain, pollEvent
	})
}

func TestGPUOffscreenDraw(t *testing.T) {
	s := startGPUScene(t)
	for _, indexed := range []bool{true, false} {
		expectQuadPixels(t, s.render(t, indexed))
	}
}

func TestGPUHotpathDrawsTheSamePixels(t *testing.T) {
	s := startGPUScene(t)
	plainIndexed, plainListed := s.render(t, true), s.render(t, false)
	withGPUHotpath(t)
	hotIndexed, hotListed := s.render(t, true), s.render(t, false)
	expectQuadPixels(t, hotIndexed)
	expectQuadPixels(t, hotListed)
	if !bytes.Equal(plainIndexed, hotIndexed) || !bytes.Equal(plainListed, hotListed) {
		t.Errorf("hot path pixels differ:\nplain %x %x\nhot   %x %x", plainIndexed, plainListed, hotIndexed, hotListed)
	}
}

func TestGPUHotpathRenderPassAllocatesNothing(t *testing.T) {
	s := startGPUScene(t)
	withGPUHotpath(t)
	var allocs float64
	var pixels []byte
	err := onMainThread(func() error {
		cb, err := s.device.AcquireCommandBuffer()
		if err != nil {
			return fmt.Errorf("acquiring a command buffer: %w", err)
		}
		allocs = testing.AllocsPerRun(20, func() {
			rp := cb.BeginRenderPass(s.colorTargets, nil)
			s.record(rp, cb, true)
			s.record(rp, cb, false)
			rp.End()
		})
		pixels, err = s.downloadTarget(cb)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if allocs != 0 {
		t.Errorf("a hot-path render pass allocates %.1f times", allocs)
	}
	expectQuadPixels(t, pixels)
}
