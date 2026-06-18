package analyze

import (
	"fmt"
	"go/token"

	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

// Loaded bundles everything downstream analysis needs.
type Loaded struct {
	Pkgs    []*packages.Package
	Prog    *ssa.Program
	SSAPkgs []*ssa.Package
	Fset    *token.FileSet
	Module  string
}

// Load type-checks the project at dir and builds its SSA form.
// It tolerates partial type errors so analysis still runs on imperfect trees.
func Load(dir string) (*Loaded, error) {
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles |
			packages.NeedImports | packages.NeedDeps | packages.NeedTypes |
			packages.NeedSyntax | packages.NeedTypesInfo | packages.NeedModule,
		Dir:   dir,
		Tests: true, // load _test.go so test→code edges feed coverage/impact
	}

	pkgs, err := packages.Load(cfg, "./...")
	if err != nil {
		return nil, fmt.Errorf("packages.Load: %w", err)
	}
	if len(pkgs) == 0 {
		return nil, fmt.Errorf("no Go packages found under %s", dir)
	}

	l := &Loaded{Fset: pkgs[0].Fset}

	// Keep only project packages (those with syntax) and record the module path.
	for _, p := range pkgs {
		if p.Module != nil && l.Module == "" {
			l.Module = p.Module.Path
		}
		if len(p.Syntax) > 0 {
			l.Pkgs = append(l.Pkgs, p)
		}
	}
	if len(l.Pkgs) == 0 {
		return nil, fmt.Errorf("packages found but none had parseable syntax under %s", dir)
	}

	// Build SSA for the precise call graph. ssautil tolerates type errors.
	prog, ssaPkgs := ssautil.AllPackages(l.Pkgs, ssa.InstantiateGenerics)
	prog.Build()
	l.Prog = prog
	for _, sp := range ssaPkgs {
		if sp != nil {
			l.SSAPkgs = append(l.SSAPkgs, sp)
		}
	}
	return l, nil
}
