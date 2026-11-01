//go:build !js

package sdl

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"reflect"
	"strings"
	"testing"
)

const sdlFFIPath = "../cmd/internal/assets/ffi/sdl.json"

type cRecord struct {
	Tag          string   `json:"tag"`
	Name         string   `json:"name"`
	BitSize      uintptr  `json:"bit-size"`
	BitAlignment uintptr  `json:"bit-alignment"`
	Fields       []cField `json:"fields"`
}

type cField struct {
	Name      string  `json:"name"`
	BitOffset uintptr `json:"bit-offset"`
	BitSize   uintptr `json:"bit-size"`
	Type      struct {
		Tag string `json:"tag"`
	} `json:"type"`
}

func loadCRecords(t *testing.T, path string) map[string]cRecord {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var all []cRecord
	if err := json.Unmarshal(b, &all); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	records := map[string]cRecord{}
	for _, r := range all {
		if (r.Tag == "struct" || r.Tag == "union") && strings.HasPrefix(r.Name, "SDL_") && len(r.Fields) > 0 {
			records[r.Name] = r
		}
	}
	if len(records) == 0 {
		t.Fatalf("no SDL structs in %s", path)
	}
	return records
}

func layoutFieldKey(name string) string {
	return strings.ToLower(strings.ReplaceAll(name, "_", ""))
}

func checkLayout(t *testing.T, c cRecord, g reflect.Type) {
	t.Helper()
	if g.Size()*8 != c.BitSize {
		t.Errorf("size: Go %d, C %d", g.Size(), c.BitSize/8)
	}
	if uintptr(g.Align())*8 != c.BitAlignment {
		t.Errorf("alignment: Go %d, C %d", g.Align(), c.BitAlignment/8)
	}
	goFields := map[string]reflect.StructField{}
	for i := range g.NumField() {
		f := g.Field(i)
		if f.Name == "_" && f.Type.Size() == 0 {
			continue
		}
		goFields[layoutFieldKey(f.Name)] = f
	}
	byOffset := map[uintptr]reflect.StructField{}
	for _, f := range goFields {
		byOffset[f.Offset] = f
	}
	for _, cf := range c.Fields {
		gf, ok := goFields[layoutFieldKey(cf.Name)]
		if !ok && cf.Type.Tag == "union" {
			gf, ok = byOffset[cf.BitOffset/8]
		}
		if !ok {
			if c.Tag != "union" {
				t.Errorf("field %s: missing in Go", cf.Name)
			}
			continue
		}
		delete(goFields, layoutFieldKey(gf.Name))
		if gf.Offset*8 != cf.BitOffset || gf.Type.Size()*8 != cf.BitSize {
			t.Errorf("field %s: Go %s offset %d size %d, C offset %d size %d",
				cf.Name, gf.Name, gf.Offset, gf.Type.Size(), cf.BitOffset/8, cf.BitSize/8)
		}
	}
	if c.Tag == "union" {
		return
	}
	for _, gf := range goFields {
		t.Errorf("Go field %s: missing in C", gf.Name)
	}
}

func TestStructLayoutsMatchC(t *testing.T) {
	records := loadCRecords(t, sdlFFIPath)
	for name, g := range layoutTypes {
		c, ok := records[name]
		if !ok {
			t.Errorf("%s: not in %s", name, sdlFFIPath)
			continue
		}
		t.Run(name, func(t *testing.T) {
			checkLayout(t, c, g)
		})
	}
}

func TestStructLayoutTableComplete(t *testing.T) {
	records := loadCRecords(t, sdlFFIPath)
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", func(fi os.FileInfo) bool {
		n := fi.Name()
		return !strings.HasSuffix(n, "_test.go") && !strings.HasSuffix(n, "_js.go")
	}, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, pkg := range pkgs {
		for _, file := range pkg.Files {
			ast.Inspect(file, func(n ast.Node) bool {
				ts, ok := n.(*ast.TypeSpec)
				if !ok {
					return true
				}
				if _, ok := ts.Type.(*ast.StructType); !ok {
					return true
				}
				for cName := range records {
					if strings.EqualFold(strings.TrimPrefix(cName, "SDL_"), ts.Name.Name) {
						if _, ok := layoutTypes[cName]; !ok {
							t.Errorf("%s (Go %s) is not in layoutTypes", cName, ts.Name.Name)
						}
					}
				}
				return true
			})
		}
	}
}

var layoutTypes = map[string]reflect.Type{
	"SDL_AsyncIOOutcome":                    reflect.TypeFor[AsyncIOOutcome](),
	"SDL_AtomicInt":                         reflect.TypeFor[AtomicInt](),
	"SDL_AtomicU32":                         reflect.TypeFor[AtomicU32](),
	"SDL_AudioDeviceEvent":                  reflect.TypeFor[AudioDeviceEvent](),
	"SDL_AudioSpec":                         reflect.TypeFor[AudioSpec](),
	"SDL_CameraDeviceEvent":                 reflect.TypeFor[CameraDeviceEvent](),
	"SDL_CameraSpec":                        reflect.TypeFor[CameraSpec](),
	"SDL_ClipboardEvent":                    reflect.TypeFor[clipboardEvent](),
	"SDL_Color":                             reflect.TypeFor[Color](),
	"SDL_CommonEvent":                       reflect.TypeFor[CommonEvent](),
	"SDL_CursorFrameInfo":                   reflect.TypeFor[CursorFrameInfo](),
	"SDL_DateTime":                          reflect.TypeFor[DateTime](),
	"SDL_DialogFileFilter":                  reflect.TypeFor[dialogFileFilter](),
	"SDL_DisplayEvent":                      reflect.TypeFor[DisplayEvent](),
	"SDL_DisplayMode":                       reflect.TypeFor[DisplayMode](),
	"SDL_DropEvent":                         reflect.TypeFor[dropEvent](),
	"SDL_Event":                             reflect.TypeFor[Event](),
	"SDL_FColor":                            reflect.TypeFor[FColor](),
	"SDL_FPoint":                            reflect.TypeFor[FPoint](),
	"SDL_FRect":                             reflect.TypeFor[FRect](),
	"SDL_Finger":                            reflect.TypeFor[Finger](),
	"SDL_GPUBlitInfo":                       reflect.TypeFor[GPUBlitInfo](),
	"SDL_GPUBlitRegion":                     reflect.TypeFor[GPUBlitRegion](),
	"SDL_GPUBufferBinding":                  reflect.TypeFor[GPUBufferBinding](),
	"SDL_GPUBufferCreateInfo":               reflect.TypeFor[GPUBufferCreateInfo](),
	"SDL_GPUBufferLocation":                 reflect.TypeFor[GPUBufferLocation](),
	"SDL_GPUBufferRegion":                   reflect.TypeFor[GPUBufferRegion](),
	"SDL_GPUColorTargetBlendState":          reflect.TypeFor[GPUColorTargetBlendState](),
	"SDL_GPUColorTargetDescription":         reflect.TypeFor[GPUColorTargetDescription](),
	"SDL_GPUColorTargetInfo":                reflect.TypeFor[GPUColorTargetInfo](),
	"SDL_GPUComputePipelineCreateInfo":      reflect.TypeFor[gpuComputePipelineCreateInfo](),
	"SDL_GPUDepthStencilState":              reflect.TypeFor[GPUDepthStencilState](),
	"SDL_GPUDepthStencilTargetInfo":         reflect.TypeFor[GPUDepthStencilTargetInfo](),
	"SDL_GPUGraphicsPipelineCreateInfo":     reflect.TypeFor[gpuGraphicsPipelineCreateInfo](),
	"SDL_GPUGraphicsPipelineTargetInfo":     reflect.TypeFor[gpuGraphicsPipelineTargetInfo](),
	"SDL_GPUIndexedIndirectDrawCommand":     reflect.TypeFor[GPUIndexedIndirectDrawCommand](),
	"SDL_GPUIndirectDispatchCommand":        reflect.TypeFor[GPUIndirectDispatchCommand](),
	"SDL_GPUIndirectDrawCommand":            reflect.TypeFor[GPUIndirectDrawCommand](),
	"SDL_GPUMultisampleState":               reflect.TypeFor[GPUMultisampleState](),
	"SDL_GPURasterizerState":                reflect.TypeFor[GPURasterizerState](),
	"SDL_GPURenderStateCreateInfo":          reflect.TypeFor[gpuRenderStateCreateInfo](),
	"SDL_GPUSamplerCreateInfo":              reflect.TypeFor[GPUSamplerCreateInfo](),
	"SDL_GPUShaderCreateInfo":               reflect.TypeFor[gpuShaderCreateInfo](),
	"SDL_GPUStencilOpState":                 reflect.TypeFor[GPUStencilOpState](),
	"SDL_GPUStorageBufferReadWriteBinding":  reflect.TypeFor[GPUStorageBufferReadWriteBinding](),
	"SDL_GPUStorageTextureReadWriteBinding": reflect.TypeFor[GPUStorageTextureReadWriteBinding](),
	"SDL_GPUTextureCreateInfo":              reflect.TypeFor[GPUTextureCreateInfo](),
	"SDL_GPUTextureLocation":                reflect.TypeFor[GPUTextureLocation](),
	"SDL_GPUTextureRegion":                  reflect.TypeFor[GPUTextureRegion](),
	"SDL_GPUTextureSamplerBinding":          reflect.TypeFor[GPUTextureSamplerBinding](),
	"SDL_GPUTextureTransferInfo":            reflect.TypeFor[GPUTextureTransferInfo](),
	"SDL_GPUTransferBufferCreateInfo":       reflect.TypeFor[GPUTransferBufferCreateInfo](),
	"SDL_GPUTransferBufferLocation":         reflect.TypeFor[GPUTransferBufferLocation](),
	"SDL_GPUVertexAttribute":                reflect.TypeFor[GPUVertexAttribute](),
	"SDL_GPUVertexBufferDescription":        reflect.TypeFor[GPUVertexBufferDescription](),
	"SDL_GPUVertexInputState":               reflect.TypeFor[gpuVertexInputState](),
	"SDL_GPUViewport":                       reflect.TypeFor[GPUViewport](),
	"SDL_GamepadAxisEvent":                  reflect.TypeFor[GamepadAxisEvent](),
	"SDL_GamepadBinding":                    reflect.TypeFor[GamepadBinding](),
	"SDL_GamepadButtonEvent":                reflect.TypeFor[GamepadButtonEvent](),
	"SDL_GamepadDeviceEvent":                reflect.TypeFor[GamepadDeviceEvent](),
	"SDL_GamepadSensorEvent":                reflect.TypeFor[GamepadSensorEvent](),
	"SDL_GamepadTouchpadEvent":              reflect.TypeFor[GamepadTouchpadEvent](),
	"SDL_HapticCondition":                   reflect.TypeFor[HapticCondition](),
	"SDL_HapticConstant":                    reflect.TypeFor[HapticConstant](),
	"SDL_HapticCustom":                      reflect.TypeFor[HapticCustom](),
	"SDL_HapticDirection":                   reflect.TypeFor[HapticDirection](),
	"SDL_HapticEffect":                      reflect.TypeFor[HapticEffect](),
	"SDL_HapticLeftRight":                   reflect.TypeFor[HapticLeftRight](),
	"SDL_HapticPeriodic":                    reflect.TypeFor[HapticPeriodic](),
	"SDL_HapticRamp":                        reflect.TypeFor[HapticRamp](),
	"SDL_IOStreamInterface":                 reflect.TypeFor[IOStreamInterface](),
	"SDL_InitState":                         reflect.TypeFor[InitState](),
	"SDL_JoyAxisEvent":                      reflect.TypeFor[JoyAxisEvent](),
	"SDL_JoyBallEvent":                      reflect.TypeFor[JoyBallEvent](),
	"SDL_JoyBatteryEvent":                   reflect.TypeFor[JoyBatteryEvent](),
	"SDL_JoyButtonEvent":                    reflect.TypeFor[JoyButtonEvent](),
	"SDL_JoyDeviceEvent":                    reflect.TypeFor[JoyDeviceEvent](),
	"SDL_JoyHatEvent":                       reflect.TypeFor[JoyHatEvent](),
	"SDL_KeyboardDeviceEvent":               reflect.TypeFor[KeyboardDeviceEvent](),
	"SDL_KeyboardEvent":                     reflect.TypeFor[KeyboardEvent](),
	"SDL_Locale":                            reflect.TypeFor[locale](),
	"SDL_MessageBoxButtonData":              reflect.TypeFor[messageBoxButtonData](),
	"SDL_MessageBoxColor":                   reflect.TypeFor[MessageBoxColor](),
	"SDL_MessageBoxColorScheme":             reflect.TypeFor[MessageBoxColorScheme](),
	"SDL_MessageBoxData":                    reflect.TypeFor[messageBoxData](),
	"SDL_MouseButtonEvent":                  reflect.TypeFor[MouseButtonEvent](),
	"SDL_MouseDeviceEvent":                  reflect.TypeFor[MouseDeviceEvent](),
	"SDL_MouseMotionEvent":                  reflect.TypeFor[MouseMotionEvent](),
	"SDL_MouseWheelEvent":                   reflect.TypeFor[MouseWheelEvent](),
	"SDL_Palette":                           reflect.TypeFor[Palette](),
	"SDL_PathInfo":                          reflect.TypeFor[PathInfo](),
	"SDL_PenAxisEvent":                      reflect.TypeFor[PenAxisEvent](),
	"SDL_PenButtonEvent":                    reflect.TypeFor[PenButtonEvent](),
	"SDL_PenMotionEvent":                    reflect.TypeFor[PenMotionEvent](),
	"SDL_PenProximityEvent":                 reflect.TypeFor[PenProximityEvent](),
	"SDL_PenTouchEvent":                     reflect.TypeFor[PenTouchEvent](),
	"SDL_PinchFingerEvent":                  reflect.TypeFor[PinchFingerEvent](),
	"SDL_PixelFormatDetails":                reflect.TypeFor[PixelFormatDetails](),
	"SDL_Point":                             reflect.TypeFor[Point](),
	"SDL_QuitEvent":                         reflect.TypeFor[QuitEvent](),
	"SDL_Rect":                              reflect.TypeFor[Rect](),
	"SDL_RenderEvent":                       reflect.TypeFor[RenderEvent](),
	"SDL_SensorEvent":                       reflect.TypeFor[SensorEvent](),
	"SDL_StorageInterface":                  reflect.TypeFor[StorageInterface](),
	"SDL_Surface":                           reflect.TypeFor[Surface](),
	"SDL_TextEditingCandidatesEvent":        reflect.TypeFor[textEditingCandidatesEvent](),
	"SDL_TextEditingEvent":                  reflect.TypeFor[textEditingEvent](),
	"SDL_TextInputEvent":                    reflect.TypeFor[textInputEvent](),
	"SDL_Texture":                           reflect.TypeFor[Texture](),
	"SDL_TouchFingerEvent":                  reflect.TypeFor[TouchFingerEvent](),
	"SDL_UserEvent":                         reflect.TypeFor[UserEvent](),
	"SDL_Vertex":                            reflect.TypeFor[Vertex](),
	"SDL_VirtualJoystickDesc":               reflect.TypeFor[virtualJoystickDesc](),
	"SDL_VirtualJoystickSensorDesc":         reflect.TypeFor[VirtualJoystickSensorDesc](),
	"SDL_VirtualJoystickTouchpadDesc":       reflect.TypeFor[VirtualJoystickTouchpadDesc](),
	"SDL_WindowEvent":                       reflect.TypeFor[WindowEvent](),
}
