package analyze

import (
	"go/types"
	"sort"

	"github.com/HamasakiBrain/go-mapgen/internal/model"
)

// resolveInterfaceImpls fills Interface.ImplBy using go/types method-set
// satisfaction — the precise answer, not name-based guessing.
func resolveInterfaceImpls(pm *model.ProjectMap, l *Loaded) {
	ifaces := map[string]*types.Interface{}
	var named []*types.Named

	for _, p := range l.Pkgs {
		if p.Types == nil {
			continue
		}
		scope := p.Types.Scope()
		for _, name := range scope.Names() {
			tn, ok := scope.Lookup(name).(*types.TypeName)
			if !ok {
				continue
			}
			if it, ok := tn.Type().Underlying().(*types.Interface); ok {
				if it.NumMethods() > 0 {
					ifaces[objID(tn)] = it
				}
				continue
			}
			if n, ok := tn.Type().(*types.Named); ok {
				named = append(named, n)
			}
		}
	}

	implMap := map[string]map[string]bool{}
	for id, it := range ifaces {
		set := map[string]bool{}
		for _, n := range named {
			if types.Implements(n, it) || types.Implements(types.NewPointer(n), it) {
				set[objID(n.Obj())] = true
			}
		}
		implMap[id] = set
	}

	for _, mi := range pm.Interfaces {
		if set, ok := implMap[mi.ID]; ok {
			impls := make([]string, 0, len(set))
			for id := range set {
				impls = append(impls, id)
			}
			sort.Strings(impls)
			mi.ImplBy = impls
		}
	}
}
