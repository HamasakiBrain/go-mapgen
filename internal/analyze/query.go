package analyze

import (
	"fmt"
	"sort"
	"strings"

	"github.com/HamasakiBrain/go-mapgen/internal/model"
)

func indexByID(pm *model.ProjectMap) map[string]*model.Function {
	m := make(map[string]*model.Function, len(pm.Functions))
	for _, fn := range pm.Functions {
		m[fn.ID] = fn
	}
	return m
}

// Resolve maps a user-supplied symbol to a canonical function ID.
// Accepts full IDs, "pkg.Func", "Recv.Method", or a bare name.
func Resolve(pm *model.ProjectMap, symbol string) (string, error) {
	byID := indexByID(pm)
	if _, ok := byID[symbol]; ok {
		return symbol, nil
	}

	var matches []*model.Function
	for _, fn := range pm.Functions {
		switch {
		case fn.ID == symbol,
			fn.Name == symbol,
			strings.HasSuffix(fn.ID, "."+symbol),
			fn.Receiver != "" && fn.Receiver+"."+fn.Name == symbol,
			fn.PackageName+"."+fn.Name == symbol:
			matches = append(matches, fn)
		}
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("символ %q не найден", symbol)
	}
	// Prefer exported, non-test, then deterministic by ID.
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].Exported != matches[j].Exported {
			return matches[i].Exported
		}
		if matches[i].IsTest != matches[j].IsTest {
			return matches[j].IsTest
		}
		return matches[i].ID < matches[j].ID
	})
	return matches[0].ID, nil
}

// Impact performs a reverse BFS over the call graph to find everything that
// transitively depends on the target, up to depth levels.
func Impact(pm *model.ProjectMap, symbol string, depth int) (*model.ImpactResult, error) {
	target, err := Resolve(pm, symbol)
	if err != nil {
		return nil, err
	}
	byID := indexByID(pm)

	res := &model.ImpactResult{Target: target, Depth: depth}
	visited := map[string]bool{target: true}
	files := map[string]bool{}
	tests := map[string]bool{}
	frontier := []string{target}

	for level := 0; level < depth && len(frontier) > 0; level++ {
		var next []string
		for _, id := range frontier {
			fn := byID[id]
			if fn == nil {
				continue
			}
			for _, caller := range fn.CalledBy {
				if visited[caller] {
					continue
				}
				visited[caller] = true
				next = append(next, caller)
				res.AffectedNodes = append(res.AffectedNodes, caller)
				if cf := byID[caller]; cf != nil {
					files[cf.File] = true
					if cf.IsTest {
						tests[cf.ID] = true
					}
				}
			}
		}
		frontier = next
	}

	sort.Strings(res.AffectedNodes)
	res.AffectedFiles = setToSorted(files)
	res.AffectedTests = setToSorted(tests)

	if total := len(pm.Functions); total > 0 {
		res.ImpactScore = float64(len(res.AffectedNodes)) / float64(total) * 100
	}

	if len(res.AffectedTests) == 0 {
		res.Recommendations = append(res.Recommendations,
			"⚠️ Затронутый код не покрыт тестами по графу вызовов — добавьте тесты перед изменением.")
	} else {
		res.Recommendations = append(res.Recommendations,
			fmt.Sprintf("Перепроверьте тесты: %s", strings.Join(capList(res.AffectedTests, 5), ", ")))
	}
	if len(res.AffectedNodes) > 15 {
		res.Recommendations = append(res.Recommendations,
			fmt.Sprintf("Большой радиус влияния (%d функций) — изменение рискованное.", len(res.AffectedNodes)))
	}
	if res.ImpactScore > 25 {
		res.Recommendations = append(res.Recommendations,
			fmt.Sprintf("Затронуто %.1f%% всех функций — рассмотрите поэтапный рефакторинг.", res.ImpactScore))
	}
	return res, nil
}

// Trace finds a call path from one function to another via forward BFS.
func Trace(pm *model.ProjectMap, from, to string) (*model.TraceResult, error) {
	fromID, err := Resolve(pm, from)
	if err != nil {
		return nil, err
	}
	toID, err := Resolve(pm, to)
	if err != nil {
		return nil, err
	}
	byID := indexByID(pm)

	res := &model.TraceResult{From: fromID, To: toID}
	if fromID == toID {
		res.Found, res.Path, res.Message = true, []string{fromID}, "from == to"
		return res, nil
	}

	prev := map[string]string{fromID: ""}
	queue := []string{fromID}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		fn := byID[cur]
		if fn == nil {
			continue
		}
		for _, callee := range fn.Calls {
			if _, seen := prev[callee]; seen {
				continue
			}
			prev[callee] = cur
			if callee == toID {
				res.Found = true
				res.Path = reconstruct(prev, fromID, toID)
				res.Steps = len(res.Path) - 1
				res.Message = fmt.Sprintf("путь найден за %d шагов", res.Steps)
				return res, nil
			}
			queue = append(queue, callee)
		}
	}
	res.Message = "путь вызова не найден"
	return res, nil
}

func reconstruct(prev map[string]string, from, to string) []string {
	var path []string
	for cur := to; cur != ""; cur = prev[cur] {
		path = append(path, cur)
		if cur == from {
			break
		}
	}
	for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
		path[i], path[j] = path[j], path[i]
	}
	return path
}

func setToSorted(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func capList(in []string, n int) []string {
	if len(in) <= n {
		return in
	}
	return in[:n]
}
