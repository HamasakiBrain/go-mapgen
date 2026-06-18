// Package model defines the serializable project map produced by mapgen.
package model

import "time"

// ProjectMap is the root document describing an analyzed Go project.
type ProjectMap struct {
	ProjectName string              `json:"project_name"`
	RootPath    string              `json:"root_path"`
	Module      string              `json:"module"`
	GoVersion   string              `json:"go_version"`
	Engine      string              `json:"engine"` // "ssa-cha" (precise) or "ast" (fallback)
	GeneratedAt time.Time           `json:"generated_at"`
	Packages    map[string]*Package `json:"packages"`
	Functions   []*Function         `json:"functions"`
	Interfaces  []*Interface        `json:"interfaces"`
	Types       []*NamedType        `json:"types"`
	Dependencies []Dependency       `json:"dependencies"`
	Routes      []Route             `json:"routes"`
	Summary     Summary             `json:"summary"`
	Quality     Quality             `json:"quality"`
	Metrics     Metrics             `json:"metrics"`
	Business    Business            `json:"business"`
}

// Package groups symbols that share an import path.
type Package struct {
	ImportPath   string   `json:"import_path"`
	Name         string   `json:"name"`
	Dir          string   `json:"dir"`
	Description  string   `json:"description"`
	Files        []string `json:"files"`
	Functions    []string `json:"functions"`  // function IDs
	Types        []string `json:"types"`
	Interfaces   []string `json:"interfaces"`
	Imports      []string `json:"imports"`     // intra-project import paths
	LinesOfCode  int      `json:"lines_of_code"`
	TestCoverage float64  `json:"test_coverage"`
	Complexity   int      `json:"complexity"`
	BusinessArea string   `json:"business_area"`
}

// Function is a top-level func or method.
type Function struct {
	ID             string   `json:"id"` // canonical: importpath.(Recv).Name
	Name           string   `json:"name"`
	Package        string   `json:"package"`      // import path
	PackageName    string   `json:"package_name"` // short name
	File           string   `json:"file"`
	Line           int      `json:"line"`
	Signature      string   `json:"signature"`
	Doc            string   `json:"doc,omitempty"`
	Responsibility string   `json:"responsibility"`
	BusinessArea   string   `json:"business_area"`
	Exported       bool     `json:"exported"`
	IsMethod       bool     `json:"is_method"`
	Receiver       string   `json:"receiver,omitempty"`
	IsTest         bool     `json:"is_test"`
	Params         []string `json:"params"`
	Results        []string `json:"results"`
	LinesOfCode    int      `json:"lines_of_code"`
	Cyclomatic     int      `json:"cyclomatic"`     // real McCabe
	Cognitive      int      `json:"cognitive"`      // SonarSource-style approximation
	MaxNesting     int      `json:"max_nesting"`
	Calls          []string `json:"calls"`          // resolved callee IDs
	CalledBy       []string `json:"called_by"`      // resolved caller IDs
	Critical       bool     `json:"critical"`
	Smells         []string `json:"smells"`
	DesignPatterns []string `json:"design_patterns"`
}

// Interface describes an interface type and its discovered implementers.
type Interface struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Package  string   `json:"package"`
	File     string   `json:"file"`
	Line     int      `json:"line"`
	Methods  []string `json:"methods"`
	ImplBy   []string `json:"impl_by"` // type IDs implementing it (from go/types)
	Exported bool     `json:"exported"`
}

// NamedType is a non-interface named type (struct, alias, etc.).
type NamedType struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Package  string `json:"package"`
	Kind     string `json:"kind"` // struct, map, slice, basic, ...
	File     string `json:"file"`
	Line     int    `json:"line"`
	Exported bool   `json:"exported"`
}

// Dependency is an edge in the package or call graph.
type Dependency struct {
	From     string `json:"from"`
	To       string `json:"to"`
	Type     string `json:"type"` // "import" | "call"
	Strength int    `json:"strength"`
}

// Route is a discovered HTTP route.
type Route struct {
	Method  string `json:"method"`
	Path    string `json:"path"`
	Handler string `json:"handler"`
	Package string `json:"package"`
	File    string `json:"file"`
	Line    int    `json:"line"`
}

// Summary holds project-wide counters.
type Summary struct {
	TotalPackages   int      `json:"total_packages"`
	TotalFunctions  int      `json:"total_functions"`
	TotalInterfaces int      `json:"total_interfaces"`
	TotalTypes      int      `json:"total_types"`
	TotalFiles      int      `json:"total_files"`
	TotalLines      int      `json:"total_lines"`
	TotalRoutes     int      `json:"total_routes"`
	EntryPoints     []string `json:"entry_points"`
	Architecture    string   `json:"architecture"`
}

// Quality aggregates code-health signals.
type Quality struct {
	TestCoverage    float64 `json:"test_coverage"`
	AvgCyclomatic   float64 `json:"avg_cyclomatic"`
	AvgCognitive    float64 `json:"avg_cognitive"`
	Maintainability float64 `json:"maintainability"`
	Issues          []Issue `json:"issues"`
}

// Metrics holds architecture-level numbers.
type Metrics struct {
	Coupling           float64 `json:"coupling"`
	Cohesion           float64 `json:"cohesion"`
	TechnicalDebt      float64 `json:"technical_debt"`
	DocumentationScore float64 `json:"documentation_score"`
	CyclesFound        int     `json:"cycles_found"`
}

// Issue is a single detected problem.
type Issue struct {
	Type     string `json:"type"`
	File     string `json:"file"`
	Line     int    `json:"line"`
	Message  string `json:"message"`
	Severity string `json:"severity"` // info | warning | error
}

// Business captures heuristic domain analysis.
type Business struct {
	Areas         map[string]int      `json:"areas"`
	MainScenarios []string            `json:"main_scenarios"`
	CriticalPaths []string            `json:"critical_paths"`
	DomainModel   map[string][]string `json:"domain_model"`
}

// ImpactResult is the output of impact analysis.
type ImpactResult struct {
	Target          string   `json:"target"`
	Depth           int      `json:"depth"`
	AffectedNodes   []string `json:"affected_nodes"`
	AffectedFiles   []string `json:"affected_files"`
	AffectedTests   []string `json:"affected_tests"`
	ImpactScore     float64  `json:"impact_score"`
	Recommendations []string `json:"recommendations"`
}

// TraceResult is the output of a call-path trace.
type TraceResult struct {
	From    string   `json:"from"`
	To      string   `json:"to"`
	Found   bool     `json:"found"`
	Steps   int      `json:"steps"`
	Path    []string `json:"path"`
	Message string   `json:"message"`
}
