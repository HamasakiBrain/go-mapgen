package analyze

import (
	"go/types"
	"sort"

	"github.com/HamasakiBrain/go-mapgen/internal/model"
	"golang.org/x/tools/go/callgraph/cha"
	"golang.org/x/tools/go/ssa"
)

// BuildCallGraph computes a type-checked call graph via SSA + CHA and writes
// resolved Calls/CalledBy edges onto every function, plus package-level
// dependency edges. CHA resolves interface and method dispatch — the part the
// old AST-name matcher could never do.
func BuildCallGraph(pm *model.ProjectMap, l *Loaded) {
	byID := map[string]*model.Function{}
	for _, fn := range pm.Functions {
		byID[fn.ID] = fn
	}

	calls := map[string]map[string]bool{}    // caller ID -> set of callee IDs
	calledBy := map[string]map[string]bool{} // callee ID -> set of caller IDs
	pkgCall := map[[2]string]int{}           // (fromPkg,toPkg) -> count

	add := func(from, to string, fromPkg, toPkg string) {
		if from == to {
			return
		}
		if calls[from] == nil {
			calls[from] = map[string]bool{}
		}
		calls[from][to] = true
		if calledBy[to] == nil {
			calledBy[to] = map[string]bool{}
		}
		calledBy[to][from] = true
		if fromPkg != toPkg {
			pkgCall[[2]string{fromPkg, toPkg}]++
		}
	}

	if l.Prog != nil {
		cg := cha.CallGraph(l.Prog)
		for fn, node := range cg.Nodes {
			callerID := ssaFuncID(fn)
			if callerID == "" || byID[callerID] == nil {
				continue
			}
			for _, e := range node.Out {
				calleeID := ssaFuncID(e.Callee.Func)
				if calleeID == "" || byID[calleeID] == nil {
					continue
				}
				add(callerID, calleeID,
					byID[callerID].Package, byID[calleeID].Package)
			}
		}
		pm.Engine = "ssa-cha"
	} else {
		pm.Engine = "ast"
	}

	for id, set := range calls {
		byID[id].Calls = sortedKeys(set)
	}
	for id, set := range calledBy {
		byID[id].CalledBy = sortedKeys(set)
	}

	buildDependencies(pm, pkgCall)
	pm.Metrics.CyclesFound = detectCycles(pm)
}

// ssaFuncID maps an SSA function back to our canonical function ID.
// The CHA graph includes a synthetic root with a nil Func, and SSA wrappers
// have no Object — both must be skipped.
func ssaFuncID(fn *ssa.Function) string {
	if fn == nil {
		return ""
	}
	tf, ok := fn.Object().(*types.Func)
	if !ok || tf == nil {
		return ""
	}
	return funcID(tf)
}

func buildDependencies(pm *model.ProjectMap, pkgCall map[[2]string]int) {
	// Intra-project import edges.
	for path, pkg := range pm.Packages {
		for _, imp := range pkg.Imports {
			if _, ok := pm.Packages[imp]; !ok {
				continue // external import, skip for the project graph
			}
			pm.Dependencies = append(pm.Dependencies, model.Dependency{
				From: path, To: imp, Type: "import", Strength: 1,
			})
		}
	}
	// Aggregated package-to-package call edges.
	var keys [][2]string
	for k := range pkgCall {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i][0] != keys[j][0] {
			return keys[i][0] < keys[j][0]
		}
		return keys[i][1] < keys[j][1]
	})
	for _, k := range keys {
		pm.Dependencies = append(pm.Dependencies, model.Dependency{
			From: k[0], To: k[1], Type: "call", Strength: pkgCall[k],
		})
	}
}

// detectCycles finds import cycles in the intra-project package graph.
func detectCycles(pm *model.ProjectMap) int {
	graph := map[string][]string{}
	for _, d := range pm.Dependencies {
		if d.Type == "import" {
			graph[d.From] = append(graph[d.From], d.To)
		}
	}
	const (
		white = 0
		gray  = 1
		black = 2
	)
	color := map[string]int{}
	cycles := 0

	var dfs func(string)
	dfs = func(u string) {
		color[u] = gray
		for _, v := range graph[u] {
			switch color[v] {
			case white:
				dfs(v)
			case gray:
				cycles++
			}
		}
		color[u] = black
	}
	var nodes []string
	for n := range graph {
		nodes = append(nodes, n)
	}
	sort.Strings(nodes)
	for _, n := range nodes {
		if color[n] == white {
			dfs(n)
		}
	}
	if cycles > 0 {
		pm.Quality.Issues = append(pm.Quality.Issues, model.Issue{
			Type:     "import_cycle",
			Message:  "обнаружены циклические импорты между пакетами проекта",
			Severity: "error",
		})
	}
	return cycles
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
