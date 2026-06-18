package analyze

import (
	"go/ast"
	"go/token"
	"strings"

	"github.com/HamasakiBrain/go-mapgen/internal/model"
)

var httpVerbs = map[string]bool{
	"GET": true, "POST": true, "PUT": true, "DELETE": true,
	"PATCH": true, "HEAD": true, "OPTIONS": true,
	"Any": true, "Handle": true, "HandleFunc": true,
}

// detectRoutes finds router registrations like r.GET("/path", handler) or
// mux.HandleFunc("/path", handler) inside a function body.
func detectRoutes(pm *model.ProjectMap, l *Loaded, d *ast.FuncDecl, fn *model.Function) {
	if d.Body == nil {
		return
	}
	ast.Inspect(d.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || !httpVerbs[sel.Sel.Name] {
			return true
		}
		if len(call.Args) == 0 {
			return true
		}
		lit, ok := call.Args[0].(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}
		path := strings.Trim(lit.Value, "\"`")
		if path == "" || !strings.HasPrefix(path, "/") {
			return true
		}
		handler := ""
		if len(call.Args) > 1 {
			handler = exprName(call.Args[1])
		}
		pm.Routes = append(pm.Routes, model.Route{
			Method:  normalizeVerb(sel.Sel.Name),
			Path:    path,
			Handler: handler,
			Package: fn.Package,
			File:    fn.File,
			Line:    l.Fset.Position(call.Pos()).Line,
		})
		return true
	})
}

func normalizeVerb(v string) string {
	switch v {
	case "HandleFunc", "Handle", "Any":
		return "ANY"
	default:
		return v
	}
}

func exprName(e ast.Expr) string {
	switch x := e.(type) {
	case *ast.Ident:
		return x.Name
	case *ast.SelectorExpr:
		return exprName(x.X) + "." + x.Sel.Name
	case *ast.CallExpr:
		return exprName(x.Fun) + "(...)"
	default:
		return ""
	}
}
