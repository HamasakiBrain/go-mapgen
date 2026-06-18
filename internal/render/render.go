// Package render writes a ProjectMap to JSON, Markdown, DOT and HTML.
package render

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/HamasakiBrain/go-mapgen/internal/model"
)

// JSON writes the full project map as indented JSON.
func JSON(pm *model.ProjectMap, path string) error {
	data, err := json.MarshalIndent(pm, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// Markdown writes a human-readable report.
func Markdown(pm *model.ProjectMap, path string) error {
	var b strings.Builder
	p := func(format string, a ...any) { fmt.Fprintf(&b, format, a...) }

	p("# 🗺️ Карта проекта: %s\n\n", pm.ProjectName)
	p("_Сгенерировано %s · движок: **%s** · модуль: `%s`_\n\n",
		pm.GeneratedAt.Format("2006-01-02 15:04"), pm.Engine, pm.Module)

	p("## 📊 Обзор\n\n")
	p("| | |\n|---|---|\n")
	p("| Архитектура | **%s** |\n", pm.Summary.Architecture)
	p("| Пакетов | %d |\n", pm.Summary.TotalPackages)
	p("| Функций | %d |\n", pm.Summary.TotalFunctions)
	p("| Интерфейсов | %d |\n", pm.Summary.TotalInterfaces)
	p("| Типов | %d |\n", pm.Summary.TotalTypes)
	p("| Файлов | %d |\n", pm.Summary.TotalFiles)
	p("| Строк кода | %d |\n", pm.Summary.TotalLines)
	p("| HTTP-маршрутов | %d |\n\n", pm.Summary.TotalRoutes)

	p("## 📈 Качество\n\n")
	p("- **Покрытие тестами** (по графу вызовов): %.1f%%\n", pm.Quality.TestCoverage)
	p("- **Средняя цикломатическая сложность**: %.2f\n", pm.Quality.AvgCyclomatic)
	p("- **Средняя когнитивная сложность**: %.2f\n", pm.Quality.AvgCognitive)
	p("- **Индекс поддерживаемости**: %.1f/100\n", pm.Quality.Maintainability)
	p("- **Документированность экспортируемого API**: %.1f%%\n", pm.Metrics.DocumentationScore)
	p("- **Связанность / Сцепление**: %.2f / %.2f\n", pm.Metrics.Coupling, pm.Metrics.Cohesion)
	p("- **Технический долг**: %.1f\n", pm.Metrics.TechnicalDebt)
	if pm.Metrics.CyclesFound > 0 {
		p("- ⚠️ **Циклические импорты**: %d\n", pm.Metrics.CyclesFound)
	}
	p("\n")

	if len(pm.Quality.Issues) > 0 {
		p("## ⚠️ Проблемы (%d)\n\n", len(pm.Quality.Issues))
		for _, is := range pm.Quality.Issues {
			emoji := "🟡"
			if is.Severity == "error" {
				emoji = "🔴"
			}
			p("- %s `%s:%d` — %s\n", emoji, filepath.Base(is.File), is.Line, is.Message)
		}
		p("\n")
	}

	if len(pm.Routes) > 0 {
		p("## 🔗 HTTP-маршруты\n\n")
		for _, r := range pm.Routes {
			p("- `%s %s` → %s (`%s:%d`)\n", r.Method, r.Path, r.Handler, filepath.Base(r.File), r.Line)
		}
		p("\n")
	}

	p("## 🎯 Бизнес-области\n\n")
	for _, kv := range sortedAreas(pm.Business.Areas) {
		p("- **%s**: %d\n", kv.k, kv.v)
	}
	if len(pm.Business.CriticalPaths) > 0 {
		p("\n### 🔴 Критические функции\n\n")
		for _, c := range pm.Business.CriticalPaths {
			p("- %s\n", c)
		}
	}
	p("\n")

	p("## 📦 Пакеты\n\n")
	for _, path := range sortedPkgKeys(pm.Packages) {
		pkg := pm.Packages[path]
		p("### `%s`\n", pkg.ImportPath)
		p("%s · сложность %d/10 · файлов %d · строк %d", pkg.Description, pkg.Complexity, len(pkg.Files), pkg.LinesOfCode)
		if pkg.TestCoverage > 0 {
			p(" · покрытие %.0f%%", pkg.TestCoverage)
		}
		p("\n\n")
	}

	p("## 🕸️ Граф зависимостей пакетов\n\n```mermaid\ngraph LR\n")
	for _, d := range pm.Dependencies {
		if d.Type != "import" {
			continue
		}
		p("    %s --> %s\n", mermaidID(d.From), mermaidID(d.To))
	}
	p("```\n")

	return os.WriteFile(path, []byte(b.String()), 0o644)
}

// DOT writes the package dependency graph in Graphviz format.
func DOT(pm *model.ProjectMap, path string) error {
	var b strings.Builder
	b.WriteString("digraph mapgen {\n  rankdir=LR;\n  node [shape=box style=\"rounded,filled\" fontname=\"Helvetica\"];\n\n")
	for _, key := range sortedPkgKeys(pm.Packages) {
		pkg := pm.Packages[key]
		color := "#bfdbfe"
		if pkg.Complexity > 7 {
			color = "#fca5a5"
		} else if pkg.Complexity > 4 {
			color = "#fde68a"
		}
		fmt.Fprintf(&b, "  %q [label=\"%s\\n%d funcs\" fillcolor=%q];\n",
			pkg.ImportPath, pkg.Name, len(pkg.Functions), color)
	}
	b.WriteString("\n")
	for _, d := range pm.Dependencies {
		style := "color=\"#3b82f6\""
		if d.Type == "call" {
			style = "color=\"#10b981\" style=dashed"
		}
		fmt.Fprintf(&b, "  %q -> %q [%s];\n", d.From, d.To, style)
	}
	b.WriteString("}\n")
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

type kv struct {
	k string
	v int
}

func sortedAreas(m map[string]int) []kv {
	var out []kv
	for k, v := range m {
		out = append(out, kv{k, v})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].v != out[j].v {
			return out[i].v > out[j].v
		}
		return out[i].k < out[j].k
	})
	return out
}

func sortedPkgKeys(m map[string]*model.Package) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func mermaidID(s string) string {
	r := strings.NewReplacer("/", "_", ".", "_", "-", "_")
	return r.Replace(s)
}
