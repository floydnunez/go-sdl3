//go:build !js

package sdl

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/build"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

type cType struct {
	Tag     string `json:"tag"`
	Name    string `json:"name"`
	BitSize int64  `json:"bit-size"`
	Type    *cType `json:"type"`
}

type cDecl struct {
	Tag        string `json:"tag"`
	Name       string `json:"name"`
	BitSize    int64  `json:"bit-size"`
	Variadic   bool   `json:"variadic"`
	Type       *cType `json:"type"`
	Parameters []struct {
		Name string `json:"name"`
		Type cType  `json:"type"`
	} `json:"parameters"`
	ReturnType *cType `json:"return-type"`
}

type abiClass struct {
	kind string
	size int64
}

func (c abiClass) String() string {
	if c.kind == "void" || c.kind == "ptr" {
		return c.kind
	}
	return fmt.Sprintf("%s%d", c.kind, c.size*8)
}

type cHeaders struct {
	typedefs  map[string]*cType
	records   map[string]int64
	functions map[string]cDecl
}

func loadCHeaders(t *testing.T, path string) cHeaders {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var all []cDecl
	if err := json.Unmarshal(b, &all); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	h := cHeaders{typedefs: map[string]*cType{}, records: map[string]int64{}, functions: map[string]cDecl{}}
	for _, d := range all {
		switch d.Tag {
		case "typedef":
			h.typedefs[d.Name] = d.Type
		case "struct", "union":
			if d.BitSize > 0 {
				h.records[d.Name] = d.BitSize / 8
			}
		case "function":
			h.functions[d.Name] = d
		}
	}
	return h
}

func (h cHeaders) class(t *cType) (abiClass, error) {
	switch t.Tag {
	case ":pointer", ":function-pointer", ":array", "__builtin_va_list":
		return abiClass{"ptr", 8}, nil
	case ":char", ":signed-char", ":unsigned-char", ":short", ":unsigned-short", ":int", ":unsigned-int",
		":long", ":unsigned-long", ":long-long", ":unsigned-long-long":
		if t.BitSize == 0 {
			return abiClass{}, fmt.Errorf("%s without a size", t.Tag)
		}
		return abiClass{"int", t.BitSize / 8}, nil
	case ":_Bool":
		return abiClass{"bool", 1}, nil
	case ":float", ":double":
		return abiClass{"float", t.BitSize / 8}, nil
	case ":void":
		return abiClass{"void", 0}, nil
	case ":enum":
		return abiClass{"int", 4}, nil
	case ":struct", ":union", "struct", "union":
		n, ok := h.records[t.Name]
		if !ok {
			return abiClass{}, fmt.Errorf("record %q has no size", t.Name)
		}
		return abiClass{"struct", n}, nil
	}
	if td, ok := h.typedefs[t.Tag]; ok {
		return h.class(td)
	}
	if n, ok := h.records[t.Tag]; ok {
		return abiClass{"struct", n}, nil
	}
	return abiClass{}, fmt.Errorf("unknown C type %q", t.Tag)
}

var goSizes = types.SizesFor("gc", runtime.GOARCH)

func goClass(t types.Type) (abiClass, error) {
	switch u := t.Underlying().(type) {
	case *types.Pointer, *types.Signature:
		return abiClass{"ptr", 8}, nil
	case *types.Basic:
		switch {
		case u.Kind() == types.UnsafePointer || u.Kind() == types.String:
			return abiClass{"ptr", 8}, nil
		case u.Kind() == types.Uintptr:
			return abiClass{"uintptr", 8}, nil
		case u.Info()&types.IsBoolean != 0:
			return abiClass{"bool", 1}, nil
		case u.Info()&types.IsInteger != 0:
			return abiClass{"int", goSizes.Sizeof(u)}, nil
		case u.Info()&types.IsFloat != 0:
			return abiClass{"float", goSizes.Sizeof(u)}, nil
		}
	case *types.Struct, *types.Array:
		return abiClass{"struct", goSizes.Sizeof(u)}, nil
	}
	return abiClass{}, fmt.Errorf("Go type %s cannot cross to C", t)
}

func compatible(c, g abiClass) bool {
	if g.kind == "uintptr" {
		return c.kind == "ptr" || (c.kind == "int" && c.size == 8)
	}
	return c == g
}

type goBinding struct {
	goVar string
	cName string
	via   string
	nargs int
	pos   token.Position
}

func loadSDLPackage(t *testing.T) (*token.FileSet, []*ast.File, *types.Package) {
	t.Helper()
	fset := token.NewFileSet()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	var files []*ast.File
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		ok, err := build.Default.MatchFile(".", e.Name())
		if err != nil {
			t.Fatal(err)
		}
		if !ok {
			continue
		}
		f, err := parser.ParseFile(fset, e.Name(), nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, f)
	}
	conf := types.Config{Importer: importer.ForCompiler(fset, "source", nil)}
	pkg, err := conf.Check("github.com/Zyko0/go-sdl3/sdl", fset, files, nil)
	if err != nil {
		t.Fatal(err)
	}
	return fset, files, pkg
}

func isBindingVarName(name string) bool {
	return len(name) > 1 && name[0] == 'i' && name[1] >= 'A' && name[1] <= 'Z'
}

func calleeName(call *ast.CallExpr) string {
	switch f := call.Fun.(type) {
	case *ast.SelectorExpr:
		if x, ok := f.X.(*ast.Ident); ok {
			return x.Name + "." + f.Sel.Name
		}
	case *ast.Ident:
		return f.Name
	}
	return ""
}

func addrName(e ast.Expr) string {
	if id, ok := e.(*ast.Ident); ok && strings.HasPrefix(id.Name, "_addr_") {
		return strings.TrimPrefix(id.Name, "_addr_")
	}
	return ""
}

func collectBindings(t *testing.T, fset *token.FileSet, files []*ast.File) []goBinding {
	t.Helper()
	var out []goBinding
	funcDecls := map[string]*ast.FuncDecl{}
	for _, f := range files {
		for _, d := range f.Decls {
			if fd, ok := d.(*ast.FuncDecl); ok && fd.Recv == nil {
				funcDecls[fd.Name.Name] = fd
			}
		}
	}
	callsIn := func(body ast.Node, goVar string) {
		ast.Inspect(body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			switch calleeName(call) {
			case "purego.SyscallN", "hotcall":
				if len(call.Args) == 0 {
					return true
				}
				c := addrName(call.Args[0])
				if c == "" {
					t.Errorf("%s: %s with a computed address", fset.Position(call.Pos()), calleeName(call))
					return true
				}
				if call.Ellipsis.IsValid() {
					t.Errorf("%s: %s with spread arguments", fset.Position(call.Pos()), calleeName(call))
				}
				nargs := len(call.Args) - 1
				if calleeName(call) == "hotcall" {
					for nargs > 0 {
						lit, ok := call.Args[nargs].(*ast.BasicLit)
						if !ok || lit.Value != "0" {
							break
						}
						nargs--
					}
				}
				out = append(out, goBinding{goVar, c, calleeName(call), nargs, fset.Position(call.Pos())})
			}
			return true
		})
	}
	for _, f := range files {
		ast.Inspect(f, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.CallExpr:
				if calleeName(n) != "purego.RegisterLibFunc" || len(n.Args) != 3 {
					return true
				}
				u, ok := n.Args[0].(*ast.UnaryExpr)
				lit, ok2 := n.Args[2].(*ast.BasicLit)
				if !ok || !ok2 {
					t.Errorf("%s: RegisterLibFunc call not understood", fset.Position(n.Pos()))
					return true
				}
				name, err := strconv.Unquote(lit.Value)
				if err != nil {
					t.Fatal(err)
				}
				out = append(out, goBinding{u.X.(*ast.Ident).Name, name, "purego.RegisterLibFunc", -1, fset.Position(n.Pos())})
			case *ast.AssignStmt:
				if len(n.Lhs) != 1 || len(n.Rhs) != 1 {
					return true
				}
				lhs, ok := n.Lhs[0].(*ast.Ident)
				if !ok || !isBindingVarName(lhs.Name) {
					return true
				}
				switch r := n.Rhs[0].(type) {
				case *ast.FuncLit:
					callsIn(r.Body, lhs.Name)
				case *ast.Ident:
					if fd, ok := funcDecls[r.Name]; ok {
						callsIn(fd.Body, lhs.Name)
					}
				}
			}
			return true
		})
	}
	return out
}

var bindingsWithKnownMismatches = map[string]string{
	"SDL_GUIDToString":             "takes SDL_GUID by value, Go passes the GUID pointer; no Go wrapper calls it",
	"SDL_StringToGUID":             "returns SDL_GUID by value, Go reads a GUID pointer; no Go wrapper calls it",
	"SDL_GetJoystickGUIDInfo":      "takes SDL_GUID by value, Go passes the GUID pointer; no Go wrapper calls it",
	"SDL_GetGamepadMappingForGUID": "takes SDL_GUID by value, Go passes the GUID pointer; no Go wrapper calls it",
	"SDL_GetJoystickGUID":          "returns SDL_GUID by value, Go reads a GUID pointer; Joystick.GUID panics before the call",
	"SDL_GetJoystickGUIDForID":     "returns SDL_GUID by value, Go reads a GUID pointer; JoystickID.JoystickGUIDForID panics before the call",
	"SDL_GetGamepadGUIDForID":      "returns SDL_GUID by value, Go reads a GUID pointer; JoystickID.GamepadGUIDForID panics before the call",
}

func checkBinding(h cHeaders, pkg *types.Package, b goBinding) []string {
	fn, ok := h.functions[b.cName]
	if !ok {
		return []string{"not in the C headers"}
	}
	obj := pkg.Scope().Lookup(b.goVar)
	if obj == nil {
		return []string{"no package-level " + b.goVar}
	}
	sig, ok := obj.Type().Underlying().(*types.Signature)
	if !ok {
		return []string{b.goVar + " is not a func"}
	}
	var problems []string
	if sig.Params().Len() != len(fn.Parameters) {
		problems = append(problems, fmt.Sprintf("Go has %d params, C %d", sig.Params().Len(), len(fn.Parameters)))
	}
	if b.nargs >= 0 && b.nargs != len(fn.Parameters) {
		problems = append(problems, fmt.Sprintf("%s passes %d args, C takes %d", b.via, b.nargs, len(fn.Parameters)))
	}
	registersOnly := b.via != "purego.RegisterLibFunc"
	for i, p := range fn.Parameters {
		cc, err := h.class(&p.Type)
		if err != nil {
			problems = append(problems, fmt.Sprintf("param %s: %v", p.Name, err))
			continue
		}
		if registersOnly && (cc.kind == "float" || cc.kind == "struct") {
			problems = append(problems, fmt.Sprintf("param %s is %s, %s passes integer registers only", p.Name, cc, b.via))
		}
		if i >= sig.Params().Len() {
			continue
		}
		gt := sig.Params().At(i).Type()
		gc, err := goClass(gt)
		if err != nil {
			problems = append(problems, fmt.Sprintf("param %s: %v", p.Name, err))
		} else if !compatible(cc, gc) {
			problems = append(problems, fmt.Sprintf("param %s: C %s, Go %s (%s)", p.Name, cc, gc, gt))
		}
	}
	rc, err := h.class(fn.ReturnType)
	if err != nil {
		return append(problems, fmt.Sprintf("return: %v", err))
	}
	if registersOnly && (rc.kind == "float" || rc.kind == "struct") {
		problems = append(problems, fmt.Sprintf("returns %s, %s reads an integer register only", rc, b.via))
	}
	switch {
	case rc.kind == "void":
		if sig.Results().Len() != 0 {
			problems = append(problems, fmt.Sprintf("C returns void, Go returns %s", sig.Results()))
		}
	case sig.Results().Len() != 1:
		problems = append(problems, fmt.Sprintf("C returns %s, Go returns %d values", rc, sig.Results().Len()))
	default:
		gt := sig.Results().At(0).Type()
		gc, err := goClass(gt)
		if err != nil {
			problems = append(problems, fmt.Sprintf("return: %v", err))
		} else if !compatible(rc, gc) {
			problems = append(problems, fmt.Sprintf("return: C %s, Go %s (%s)", rc, gc, gt))
		}
	}
	return problems
}

func TestBindingSignaturesMatchC(t *testing.T) {
	h := loadCHeaders(t, sdlFFIPath)
	fset, files, pkg := loadSDLPackage(t)
	bindings := collectBindings(t, fset, files)
	mismatched := map[string]bool{}
	for _, b := range bindings {
		problems := checkBinding(h, pkg, b)
		if len(problems) == 0 {
			continue
		}
		mismatched[b.cName] = true
		if _, known := bindingsWithKnownMismatches[b.cName]; known {
			continue
		}
		for _, p := range problems {
			t.Errorf("%s: %s (%s via %s): %s", b.pos, b.cName, b.goVar, b.via, p)
		}
	}
	for name, reason := range bindingsWithKnownMismatches {
		if !mismatched[name] {
			t.Errorf("%s matches C now; remove it from bindingsWithKnownMismatches (%s)", name, reason)
		}
	}
}

func TestEveryBindingVariableIsBound(t *testing.T) {
	fset, files, pkg := loadSDLPackage(t)
	bound := map[string]bool{}
	for _, b := range collectBindings(t, fset, files) {
		bound[b.goVar] = true
	}
	declared := 0
	for _, name := range pkg.Scope().Names() {
		v, ok := pkg.Scope().Lookup(name).(*types.Var)
		if !ok || !isBindingVarName(name) {
			continue
		}
		if _, ok := v.Type().Underlying().(*types.Signature); !ok {
			continue
		}
		declared++
		if !bound[name] {
			t.Errorf("%s: %s is declared but never bound to a C function", fset.Position(v.Pos()), name)
		}
	}
	if declared == 0 {
		t.Fatal("no binding variables found")
	}
}
