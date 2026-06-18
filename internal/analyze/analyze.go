package analyze

import (
	"math"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/HamasakiBrain/go-mapgen/internal/model"
)

// Analyze loads, type-checks and fully analyzes the project rooted at dir.
func Analyze(dir, projectName string) (*model.ProjectMap, error) {
	l, err := Load(dir)
	if err != nil {
		return nil, err
	}

	pm := &model.ProjectMap{
		ProjectName: projectName,
		RootPath:    dir,
		Module:      l.Module,
		GoVersion:   runtime.Version(),
		GeneratedAt: time.Now(),
		Business: model.Business{
			Areas:       map[string]int{},
			DomainModel: map[string][]string{},
		},
	}

	Inventory(pm, l)
	BuildCallGraph(pm, l)
	resolveInterfaceImpls(pm, l)

	aggregateFunctions(pm)
	computeTestCoverage(pm)
	computeQuality(pm)
	computeMetrics(pm)
	computeBusiness(pm)
	computeArchitecture(pm)
	computePackages(pm)
	finalizeSummary(pm)

	return pm, nil
}

// aggregateFunctions fills derived per-function fields.
func aggregateFunctions(pm *model.ProjectMap) {
	for _, fn := range pm.Functions {
		fn.Critical = fn.Cyclomatic >= 10 || fn.LinesOfCode > 100 || len(fn.CalledBy) >= 10
		fn.Responsibility = describeFunction(fn)
	}
}

func describeFunction(fn *model.Function) string {
	if fn.Doc != "" {
		return fn.Doc
	}
	prefixes := map[string]string{
		"New": "конструктор", "Get": "доступ к данным", "Set": "установка значения",
		"Create": "создание", "Delete": "удаление", "Update": "обновление",
		"Handle": "обработчик запроса", "Serve": "обслуживание", "Parse": "разбор",
		"Validate": "валидация", "Run": "запуск", "Start": "запуск", "Stop": "остановка",
	}
	for p, desc := range prefixes {
		if strings.HasPrefix(fn.Name, p) {
			if fn.IsMethod {
				return desc + " (" + fn.Receiver + ")"
			}
			return desc
		}
	}
	if fn.IsMethod {
		return "метод типа " + fn.Receiver
	}
	if fn.Exported {
		return "экспортируемая функция пакета " + fn.PackageName
	}
	return "внутренняя функция"
}

func computeTestCoverage(pm *model.ProjectMap) {
	tested := map[string]bool{}
	for _, fn := range pm.Functions {
		if fn.IsTest {
			continue
		}
		for _, caller := range fn.CalledBy {
			if cf := findByID(pm, caller); cf != nil && cf.IsTest {
				tested[fn.ID] = true
				break
			}
		}
	}
	var total, cov int
	for _, fn := range pm.Functions {
		if fn.IsTest {
			continue
		}
		total++
		if tested[fn.ID] {
			cov++
		}
	}
	if total > 0 {
		pm.Quality.TestCoverage = float64(cov) / float64(total) * 100
	}
}

func computeQuality(pm *model.ProjectMap) {
	var sumCyc, sumCog float64
	var n int
	for _, fn := range pm.Functions {
		if fn.IsTest {
			continue
		}
		n++
		sumCyc += float64(fn.Cyclomatic)
		sumCog += float64(fn.Cognitive)
		for _, s := range fn.Smells {
			sev := "warning"
			if strings.Contains(s, "цикломатическая") {
				sev = "error"
			}
			pm.Quality.Issues = append(pm.Quality.Issues, model.Issue{
				Type: smellType(s), File: fn.File, Line: fn.Line,
				Message: fn.Name + ": " + s, Severity: sev,
			})
		}
	}
	if n > 0 {
		pm.Quality.AvgCyclomatic = sumCyc / float64(n)
		pm.Quality.AvgCognitive = sumCog / float64(n)
	}
	// Simplified maintainability index in [0,100].
	mi := 100 - pm.Quality.AvgCyclomatic*3.0 - pm.Quality.AvgCognitive*1.5
	pm.Quality.Maintainability = clamp(mi, 0, 100)

	sort.Slice(pm.Quality.Issues, func(i, j int) bool {
		if pm.Quality.Issues[i].Severity != pm.Quality.Issues[j].Severity {
			return pm.Quality.Issues[i].Severity > pm.Quality.Issues[j].Severity // error before warning
		}
		return pm.Quality.Issues[i].File < pm.Quality.Issues[j].File
	})
}

func smellType(s string) string {
	switch {
	case strings.Contains(s, "длинная"):
		return "long_function"
	case strings.Contains(s, "параметров"):
		return "too_many_params"
	case strings.Contains(s, "цикломатическая"):
		return "high_complexity"
	case strings.Contains(s, "вложенность"):
		return "deep_nesting"
	default:
		return "smell"
	}
}

func computeMetrics(pm *model.ProjectMap) {
	if len(pm.Packages) > 0 {
		var imports int
		for _, p := range pm.Packages {
			for _, imp := range p.Imports {
				if _, ok := pm.Packages[imp]; ok {
					imports++
				}
			}
		}
		pm.Metrics.Coupling = float64(imports) / float64(len(pm.Packages))

		var coh float64
		for _, p := range pm.Packages {
			if len(p.Files) > 0 {
				coh += float64(len(p.Functions)) / float64(len(p.Files))
			}
		}
		pm.Metrics.Cohesion = coh / float64(len(pm.Packages))
	}

	for _, is := range pm.Quality.Issues {
		switch is.Severity {
		case "error":
			pm.Metrics.TechnicalDebt += 10
		case "warning":
			pm.Metrics.TechnicalDebt += 5
		default:
			pm.Metrics.TechnicalDebt += 2
		}
	}
	pm.Metrics.TechnicalDebt += pm.Quality.AvgCyclomatic * 2

	var exported, documented int
	for _, fn := range pm.Functions {
		if fn.Exported && !fn.IsTest {
			exported++
			if fn.Doc != "" {
				documented++
			}
		}
	}
	if exported > 0 {
		pm.Metrics.DocumentationScore = float64(documented) / float64(exported) * 100
	} else {
		pm.Metrics.DocumentationScore = 100
	}
}

func computeBusiness(pm *model.ProjectMap) {
	for _, fn := range pm.Functions {
		if fn.Exported && !fn.IsTest {
			pm.Business.Areas[fn.BusinessArea]++
		}
	}
	for _, t := range pm.Types {
		for _, p := range []string{"User", "Account", "Order", "Product", "Payment", "Customer"} {
			if strings.HasPrefix(t.Name, p) {
				pm.Business.DomainModel[t.Package] = append(pm.Business.DomainModel[t.Package], t.Name)
				break
			}
		}
	}
	add := func(cond bool, s string) {
		if cond {
			pm.Business.MainScenarios = append(pm.Business.MainScenarios, s)
		}
	}
	add(hasFunc(pm, "main"), "Запуск приложения")
	add(hasArea(pm, "Auth"), "Аутентификация и авторизация")
	add(hasArea(pm, "Payment"), "Платежные операции")
	add(hasArea(pm, "CRUD"), "Операции с данными (CRUD)")
	add(len(pm.Routes) > 0, "HTTP API")

	for _, fn := range pm.Functions {
		if fn.Critical {
			pm.Business.CriticalPaths = append(pm.Business.CriticalPaths,
				fn.ID+" (cyclo="+itoa(fn.Cyclomatic)+", fan-in="+itoa(len(fn.CalledBy))+")")
		}
	}
	sort.Strings(pm.Business.CriticalPaths)
}

func computeArchitecture(pm *model.ProjectMap) {
	var mvc, layered, hex int
	for _, p := range pm.Packages {
		name := p.Name + " " + p.ImportPath
		switch {
		case containsAny(name, "controller", "view", "model"):
			mvc++
		case containsAny(name, "handler", "service", "repository"):
			layered++
		case containsAny(name, "domain", "application", "infrastructure", "adapter"):
			hex++
		}
	}
	switch {
	case hex >= 3:
		pm.Summary.Architecture = "Hexagonal"
	case layered >= 3:
		pm.Summary.Architecture = "Layered"
	case mvc >= 3:
		pm.Summary.Architecture = "MVC"
	case len(pm.Packages) <= 2:
		pm.Summary.Architecture = "Monolith"
	default:
		pm.Summary.Architecture = "Modular"
	}
}

func computePackages(pm *model.ProjectMap) {
	cov := map[string]struct{ tested, total int }{}
	for _, fn := range pm.Functions {
		if fn.IsTest {
			continue
		}
		c := cov[fn.Package]
		c.total++
		for _, caller := range fn.CalledBy {
			if cf := findByID(pm, caller); cf != nil && cf.IsTest {
				c.tested++
				break
			}
		}
		cov[fn.Package] = c
	}
	for path, p := range pm.Packages {
		p.Description = describePackage(p)
		p.Complexity = packageComplexity(p)
		if c := cov[path]; c.total > 0 {
			p.TestCoverage = float64(c.tested) / float64(c.total) * 100
		}
	}
}

func describePackage(p *model.Package) string {
	name := strings.ToLower(p.Name)
	switch {
	case name == "main":
		return "Точка входа в приложение"
	case containsAny(name, "handler", "controller", "api", "http"):
		return "Обработка HTTP-запросов и маршрутизация"
	case containsAny(name, "service", "usecase"):
		return "Бизнес-логика и сценарии использования"
	case containsAny(name, "repository", "store", "dao", "db"):
		return "Доступ к данным и их хранение"
	case containsAny(name, "model", "entity", "domain"):
		return "Доменные модели и структуры данных"
	case containsAny(name, "config"):
		return "Конфигурация приложения"
	default:
		return "Пакет " + p.Name
	}
}

func packageComplexity(p *model.Package) int {
	c := 1
	if len(p.Files) > 10 {
		c += 2
	}
	if len(p.Functions) > 20 {
		c += 2
	}
	if len(p.Imports) > 15 {
		c += 2
	}
	if len(p.Interfaces) > 5 {
		c += 2
	}
	if c > 10 {
		c = 10
	}
	return c
}

func finalizeSummary(pm *model.ProjectMap) {
	pm.Summary.TotalRoutes = len(pm.Routes)
	for _, fn := range pm.Functions {
		if fn.Name == "main" && fn.PackageName == "main" {
			pm.Summary.EntryPoints = append(pm.Summary.EntryPoints, fn.ID)
		}
	}
	sort.Strings(pm.Summary.EntryPoints)
	sort.Slice(pm.Functions, func(i, j int) bool { return pm.Functions[i].ID < pm.Functions[j].ID })
	sort.Slice(pm.Interfaces, func(i, j int) bool { return pm.Interfaces[i].ID < pm.Interfaces[j].ID })
	sort.Slice(pm.Types, func(i, j int) bool { return pm.Types[i].ID < pm.Types[j].ID })
}

// ---- small helpers ----

func findByID(pm *model.ProjectMap, id string) *model.Function {
	for _, fn := range pm.Functions {
		if fn.ID == id {
			return fn
		}
	}
	return nil
}

func hasFunc(pm *model.ProjectMap, name string) bool {
	for _, fn := range pm.Functions {
		if fn.Name == name {
			return true
		}
	}
	return false
}

func hasArea(pm *model.ProjectMap, area string) bool {
	return pm.Business.Areas[area] > 0
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

func clamp(v, lo, hi float64) float64 {
	return math.Max(lo, math.Min(hi, v))
}

func itoa(i int) string { return strconv.Itoa(i) }
