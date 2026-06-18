package analyze

import (
	"fmt"
	"go/ast"
	"go/token"
	"strings"

	"github.com/HamasakiBrain/go-mapgen/internal/model"
)

// cyclomatic computes the real McCabe cyclomatic complexity of a body:
// 1 + number of decision points.
func cyclomatic(body *ast.BlockStmt) int {
	c := 1
	ast.Inspect(body, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt:
			c++
		case *ast.CaseClause:
			if len(x.List) > 0 { // non-default case
				c++
			}
		case *ast.CommClause:
			if x.Comm != nil { // non-default select case
				c++
			}
		case *ast.BinaryExpr:
			if x.Op == token.LAND || x.Op == token.LOR {
				c++
			}
		}
		return true
	})
	return c
}

// cognitive computes a SonarSource-style cognitive complexity: control-flow
// structures cost 1 plus the current nesting level; boolean operator sequences
// and jumps add a flat increment.
func cognitive(body *ast.BlockStmt) int {
	score := 0
	var walk func(n ast.Node, depth int)

	walkList := func(stmts []ast.Stmt, depth int) {
		for _, s := range stmts {
			walk(s, depth)
		}
	}

	walk = func(n ast.Node, depth int) {
		switch s := n.(type) {
		case *ast.BlockStmt:
			walkList(s.List, depth)
		case *ast.IfStmt:
			score += 1 + depth
			score += boolOps(s.Cond)
			if s.Init != nil {
				walk(s.Init, depth)
			}
			walk(s.Body, depth+1)
			switch e := s.Else.(type) {
			case *ast.IfStmt: // else-if: +1 flat, same nesting
				score++
				walk(e, depth)
			case *ast.BlockStmt: // else: +1 flat
				score++
				walk(e, depth+1)
			}
		case *ast.ForStmt:
			score += 1 + depth
			if s.Cond != nil {
				score += boolOps(s.Cond)
			}
			walk(s.Body, depth+1)
		case *ast.RangeStmt:
			score += 1 + depth
			walk(s.Body, depth+1)
		case *ast.SwitchStmt:
			score += 1 + depth
			for _, cc := range s.Body.List {
				if c, ok := cc.(*ast.CaseClause); ok {
					walkList(c.Body, depth+1)
				}
			}
		case *ast.TypeSwitchStmt:
			score += 1 + depth
			for _, cc := range s.Body.List {
				if c, ok := cc.(*ast.CaseClause); ok {
					walkList(c.Body, depth+1)
				}
			}
		case *ast.SelectStmt:
			score += 1 + depth
			for _, cc := range s.Body.List {
				if c, ok := cc.(*ast.CommClause); ok {
					walkList(c.Body, depth+1)
				}
			}
		case *ast.FuncLit:
			walk(s.Body, depth+1) // nested function increases nesting
		case *ast.BranchStmt:
			if s.Label != nil { // labeled break/continue/goto
				score++
			}
		case *ast.LabeledStmt:
			walk(s.Stmt, depth)
		case *ast.ExprStmt:
			// no structural cost
		case *ast.DeferStmt, *ast.GoStmt:
			// ignored
		default:
			// Descend into remaining composite statements generically.
			ast.Inspect(n, func(c ast.Node) bool {
				if c == n {
					return true
				}
				switch c.(type) {
				case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt,
					*ast.SwitchStmt, *ast.TypeSwitchStmt, *ast.SelectStmt, *ast.FuncLit:
					walk(c, depth)
					return false
				}
				return true
			})
		}
	}

	walk(body, 0)
	return score
}

// boolOps counts logical && / || operators in an expression.
func boolOps(e ast.Expr) int {
	n := 0
	ast.Inspect(e, func(node ast.Node) bool {
		if b, ok := node.(*ast.BinaryExpr); ok {
			if b.Op == token.LAND || b.Op == token.LOR {
				n++
			}
		}
		return true
	})
	return n
}

// maxNesting returns the deepest nesting of control-flow blocks.
func maxNesting(body *ast.BlockStmt) int {
	max := 0
	var walk func(n ast.Node, depth int)
	walk = func(n ast.Node, depth int) {
		if depth > max {
			max = depth
		}
		switch s := n.(type) {
		case *ast.BlockStmt:
			for _, st := range s.List {
				walk(st, depth)
			}
		case *ast.IfStmt:
			walk(s.Body, depth+1)
			if s.Else != nil {
				walk(s.Else, depth+1)
			}
		case *ast.ForStmt:
			walk(s.Body, depth+1)
		case *ast.RangeStmt:
			walk(s.Body, depth+1)
		case *ast.SwitchStmt:
			walk(s.Body, depth+1)
		case *ast.TypeSwitchStmt:
			walk(s.Body, depth+1)
		case *ast.SelectStmt:
			walk(s.Body, depth+1)
		case *ast.CaseClause:
			for _, st := range s.Body {
				walk(st, depth)
			}
		case *ast.CommClause:
			for _, st := range s.Body {
				walk(st, depth)
			}
		}
	}
	walk(body, 0)
	return max
}

const (
	maxLOC        = 80  // function-length threshold
	maxParams     = 5   // parameter-count threshold
	maxCyclomatic = 15  // cyclomatic threshold
	maxNestDepth  = 4   // nesting threshold
)

func detectSmells(fn *model.Function) []string {
	var s []string
	if fn.LinesOfCode > maxLOC {
		s = append(s, fmt.Sprintf("длинная функция (%d строк)", fn.LinesOfCode))
	}
	if len(fn.Params) > maxParams {
		s = append(s, fmt.Sprintf("много параметров (%d)", len(fn.Params)))
	}
	if fn.Cyclomatic > maxCyclomatic {
		s = append(s, fmt.Sprintf("высокая цикломатическая сложность (%d)", fn.Cyclomatic))
	}
	if fn.MaxNesting > maxNestDepth {
		s = append(s, fmt.Sprintf("глубокая вложенность (%d)", fn.MaxNesting))
	}
	return s
}

func detectBusinessArea(name string) string {
	patterns := []struct {
		area string
		keys []string
	}{
		{"Auth", []string{"Login", "Logout", "Register", "Authenticate", "Authorize", "Token", "Verify", "Hash"}},
		{"Payment", []string{"Pay", "Refund", "Invoice", "Charge", "Checkout", "Transaction"}},
		{"CRUD", []string{"Create", "Read", "Update", "Delete", "Get", "List", "Find", "Save"}},
		{"Storage", []string{"Load", "Store", "Cache", "Query", "Persist", "Fetch"}},
		{"API", []string{"Handle", "Route", "Middleware", "Serve", "Request", "Response"}},
		{"Data", []string{"Parse", "Convert", "Transform", "Validate", "Format", "Marshal", "Encode", "Decode"}},
		{"System", []string{"Init", "Start", "Stop", "Shutdown", "Run", "Config"}},
		{"Network", []string{"Send", "Receive", "Connect", "Dial", "Listen", "Accept"}},
	}
	for _, p := range patterns {
		for _, k := range p.keys {
			if strings.Contains(name, k) {
				return p.area
			}
		}
	}
	return "Utility"
}

func detectPatterns(name string) []string {
	defs := map[string][]string{
		"Factory":    {"New", "Create", "Make", "Build"},
		"Singleton":  {"Instance", "GetInstance", "Default"},
		"Builder":    {"Builder", "With", "Set"},
		"Observer":   {"Notify", "Subscribe", "Publish", "Emit"},
		"Repository": {"Repository", "Repo", "Query", "FindBy"},
	}
	var out []string
	for pat, keys := range defs {
		for _, k := range keys {
			if strings.Contains(name, k) {
				out = append(out, pat)
				break
			}
		}
	}
	return out
}
