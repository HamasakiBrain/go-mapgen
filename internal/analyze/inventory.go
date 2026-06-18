package analyze

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"github.com/HamasakiBrain/go-mapgen/internal/model"
	"golang.org/x/tools/go/packages"
)

// qualifier renders package-qualified type names using short package names.
func qualifier(p *types.Package) string {
	if p == nil {
		return ""
	}
	return p.Name()
}

// recvTypeName returns the bare named-type of a receiver (pointer stripped).
func recvTypeName(t types.Type) string {
	if ptr, ok := t.(*types.Pointer); ok {
		t = ptr.Elem()
	}
	if named, ok := t.(*types.Named); ok {
		return named.Obj().Name()
	}
	return types.TypeString(t, qualifier)
}

// funcID builds the canonical, stable identifier for a function object.
func funcID(obj *types.Func) string {
	pkgPath := ""
	if obj.Pkg() != nil {
		pkgPath = obj.Pkg().Path()
	}
	name := obj.Name()
	if sig, ok := obj.Type().(*types.Signature); ok && sig.Recv() != nil {
		return pkgPath + ".(" + recvTypeName(sig.Recv().Type()) + ")." + name
	}
	return pkgPath + "." + name
}

// objID builds a stable identifier for a named type / interface.
func objID(obj types.Object) string {
	if obj.Pkg() != nil {
		return obj.Pkg().Path() + "." + obj.Name()
	}
	return obj.Name()
}

// Inventory walks every project package and fills the model's symbol tables.
// It computes real per-function metrics from the AST + type info.
func Inventory(pm *model.ProjectMap, l *Loaded) {
	pm.Packages = map[string]*model.Package{}
	// Tests:true yields several variants per package (normal, in-package test,
	// external _test). Dedup symbols/files by identity so counts stay honest
	// while test functions still contribute call edges.
	seenFunc := map[string]bool{}
	seenFile := map[string]bool{}

	for _, p := range l.Pkgs {
		if isSyntheticPkg(p) {
			continue
		}
		// External _test packages and the synthesized test binary don't get a
		// table entry, but their (test) functions are still harvested below.
		realPkg := !strings.HasSuffix(p.PkgPath, "_test") &&
			!(p.Name == "main" && strings.HasSuffix(p.PkgPath, ".test"))

		var pkg *model.Package
		if realPkg {
			if existing, ok := pm.Packages[p.PkgPath]; ok {
				pkg = existing
			} else {
				pkg = &model.Package{ImportPath: p.PkgPath, Name: p.Name}
				pm.Packages[p.PkgPath] = pkg
				for impPath := range p.Imports {
					pkg.Imports = append(pkg.Imports, impPath)
				}
			}
		}

		for i, file := range p.Syntax {
			fname := ""
			if i < len(p.CompiledGoFiles) {
				fname = p.CompiledGoFiles[i]
			} else {
				fname = l.Fset.Position(file.Pos()).Filename
			}
			isTestFile := strings.HasSuffix(fname, "_test.go")

			if pkg != nil && !seenFile[fname] {
				if pkg.Dir == "" {
					pkg.Dir = dirOf(fname)
				}
				loc := l.Fset.Position(file.End()).Line
				pkg.LinesOfCode += loc
				pm.Summary.TotalLines += loc
				pkg.Files = append(pkg.Files, fname)
			}
			fileCounted := seenFile[fname]
			seenFile[fname] = true

			for _, decl := range file.Decls {
				switch d := decl.(type) {
				case *ast.FuncDecl:
					fn := buildFunction(p, l, d, fname, isTestFile)
					if fn == nil || seenFunc[fn.ID] {
						continue
					}
					seenFunc[fn.ID] = true
					pm.Functions = append(pm.Functions, fn)
					if pkg != nil {
						pkg.Functions = append(pkg.Functions, fn.ID)
					}
					if !fn.IsTest {
						detectRoutes(pm, l, d, fn)
					}
				case *ast.GenDecl:
					if d.Tok != token.TYPE || pkg == nil || fileCounted {
						continue
					}
					for _, spec := range d.Specs {
						ts, ok := spec.(*ast.TypeSpec)
						if !ok {
							continue
						}
						handleTypeSpec(pm, pkg, p, l, ts, fname)
					}
				}
			}
		}
	}

	pm.Summary.TotalFiles = countFiles(pm)
	pm.Summary.TotalPackages = len(pm.Packages)
	pm.Summary.TotalFunctions = len(pm.Functions)
	pm.Summary.TotalInterfaces = len(pm.Interfaces)
	pm.Summary.TotalTypes = len(pm.Types)
}

func buildFunction(p *packages.Package, l *Loaded, d *ast.FuncDecl, file string, isTest bool) *model.Function {
	def := p.TypesInfo.Defs[d.Name]
	obj, ok := def.(*types.Func)
	if !ok || obj == nil {
		return nil
	}
	sig, _ := obj.Type().(*types.Signature)

	fn := &model.Function{
		ID:          funcID(obj),
		Name:        obj.Name(),
		Package:     p.PkgPath,
		PackageName: p.Name,
		File:        file,
		Line:        l.Fset.Position(d.Pos()).Line,
		Exported:    obj.Exported(),
		IsTest:      isTest && strings.HasPrefix(obj.Name(), "Test"),
		Signature:   signatureString(obj, sig),
		Calls:       []string{},
		CalledBy:    []string{},
		Smells:      []string{},
	}
	if d.Doc != nil {
		fn.Doc = cleanDoc(d.Doc.Text())
	}
	if sig != nil {
		if sig.Recv() != nil {
			fn.IsMethod = true
			fn.Receiver = recvTypeName(sig.Recv().Type())
		}
		fn.Params = tupleStrings(sig.Params())
		fn.Results = tupleStrings(sig.Results())
	}

	if d.Body != nil {
		start := l.Fset.Position(d.Body.Pos()).Line
		end := l.Fset.Position(d.Body.End()).Line
		fn.LinesOfCode = end - start + 1
		fn.Cyclomatic = cyclomatic(d.Body)
		fn.Cognitive = cognitive(d.Body)
		fn.MaxNesting = maxNesting(d.Body)
	} else {
		fn.Cyclomatic = 1
	}

	fn.BusinessArea = detectBusinessArea(fn.Name)
	fn.DesignPatterns = detectPatterns(fn.Name)
	fn.Smells = detectSmells(fn)
	return fn
}

func handleTypeSpec(pm *model.ProjectMap, pkg *model.Package, p *packages.Package, l *Loaded, ts *ast.TypeSpec, file string) {
	def := p.TypesInfo.Defs[ts.Name]
	tn, ok := def.(*types.TypeName)
	if !ok || tn == nil {
		return
	}
	id := objID(tn)
	line := l.Fset.Position(ts.Pos()).Line

	if iface, ok := ts.Type.(*ast.InterfaceType); ok {
		mi := &model.Interface{
			ID:       id,
			Name:     tn.Name(),
			Package:  p.PkgPath,
			File:     file,
			Line:     line,
			Exported: tn.Exported(),
			ImplBy:   []string{},
		}
		if iface.Methods != nil {
			for _, m := range iface.Methods.List {
				for _, nm := range m.Names {
					mi.Methods = append(mi.Methods, nm.Name)
				}
			}
		}
		pm.Interfaces = append(pm.Interfaces, mi)
		pkg.Interfaces = append(pkg.Interfaces, mi.Name)
		return
	}

	nt := &model.NamedType{
		ID:       id,
		Name:     tn.Name(),
		Package:  p.PkgPath,
		Kind:     typeKind(tn.Type()),
		File:     file,
		Line:     line,
		Exported: tn.Exported(),
	}
	pm.Types = append(pm.Types, nt)
	pkg.Types = append(pkg.Types, nt.Name)
}

func signatureString(obj *types.Func, sig *types.Signature) string {
	if sig == nil {
		return "func " + obj.Name() + "()"
	}
	full := types.TypeString(sig, qualifier) // "func(a int) bool"
	body := strings.TrimPrefix(full, "func")
	if sig.Recv() != nil {
		return "func (" + recvTypeName(sig.Recv().Type()) + ") " + obj.Name() + body
	}
	return "func " + obj.Name() + body
}

func tupleStrings(t *types.Tuple) []string {
	if t == nil {
		return nil
	}
	out := make([]string, 0, t.Len())
	for i := 0; i < t.Len(); i++ {
		out = append(out, types.TypeString(t.At(i).Type(), qualifier))
	}
	return out
}

func typeKind(t types.Type) string {
	switch u := t.Underlying().(type) {
	case *types.Struct:
		return "struct"
	case *types.Map:
		return "map"
	case *types.Slice:
		return "slice"
	case *types.Array:
		return "array"
	case *types.Signature:
		return "func"
	case *types.Chan:
		return "chan"
	case *types.Basic:
		return "basic"
	case *types.Pointer:
		return "pointer"
	default:
		_ = u
		return "other"
	}
}

func cleanDoc(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	var out []string
	for _, ln := range lines {
		ln = strings.TrimSpace(strings.TrimPrefix(ln, "//"))
		if ln != "" && !strings.HasPrefix(ln, "@") {
			out = append(out, ln)
		}
	}
	return strings.Join(out, " ")
}

func dirOf(path string) string {
	if i := strings.LastIndexByte(path, '/'); i >= 0 {
		return path[:i]
	}
	return path
}

func countFiles(pm *model.ProjectMap) int {
	seen := map[string]bool{}
	for _, p := range pm.Packages {
		for _, f := range p.Files {
			seen[f] = true
		}
	}
	return len(seen)
}

func isSyntheticPkg(p *packages.Package) bool {
	return p.PkgPath == "" || strings.HasSuffix(p.PkgPath, ".test") || p.Name == "main" && len(p.Syntax) == 0
}
