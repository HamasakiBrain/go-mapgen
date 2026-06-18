// mapgen — type-checked Go project mapper.
//
// It loads a project with go/packages, builds an SSA + CHA call graph (so
// method/interface/cross-package dispatch resolves precisely), computes real
// complexity metrics, detects HTTP routes and import cycles, and renders the
// result to JSON, Markdown, DOT and an interactive HTML dashboard. It also
// answers impact ("what breaks if I change X") and trace ("path from A to B")
// queries over the resolved call graph.
package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/HamasakiBrain/go-mapgen/internal/analyze"
	"github.com/HamasakiBrain/go-mapgen/internal/model"
	"github.com/HamasakiBrain/go-mapgen/internal/render"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "❌", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	fs := newFlags()
	if err := fs.parse(args); err != nil {
		return err
	}

	abs, err := filepath.Abs(fs.path)
	if err != nil {
		return err
	}
	name := filepath.Base(abs)

	fmt.Printf("🔍 Анализ %s …\n", abs)
	start := time.Now()
	pm, err := analyze.Analyze(abs, name)
	if err != nil {
		return err
	}
	fmt.Printf("✅ Готово за %s · движок: %s · функций: %d · пакетов: %d\n",
		time.Since(start).Round(time.Millisecond), pm.Engine,
		pm.Summary.TotalFunctions, pm.Summary.TotalPackages)

	switch {
	case fs.impact != "":
		return runImpact(pm, fs.impact, fs.depth)
	case fs.trace != "":
		return runTrace(pm, fs.trace)
	case fs.web:
		return serve(pm, fs.port)
	default:
		return writeArtifacts(pm, fs)
	}
}

func runImpact(pm *model.ProjectMap, symbol string, depth int) error {
	res, err := analyze.Impact(pm, symbol, depth)
	if err != nil {
		return err
	}
	return printJSON(res)
}

func runTrace(pm *model.ProjectMap, spec string) error {
	from, to, ok := strings.Cut(spec, "..")
	if !ok {
		return fmt.Errorf("формат -trace: <from>..<to> (например: main..Save)")
	}
	res, err := analyze.Trace(pm, strings.TrimSpace(from), strings.TrimSpace(to))
	if err != nil {
		return err
	}
	return printJSON(res)
}

func serve(pm *model.ProjectMap, port int) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		b, err := render.HTMLBytes(pm)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(b)
	})
	mux.HandleFunc("/map.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(pm)
	})
	addr := fmt.Sprintf(":%d", port)
	fmt.Printf("🌐 Дашборд: http://localhost%s\n", addr)
	return http.ListenAndServe(addr, mux)
}

func writeArtifacts(pm *model.ProjectMap, fs *flags) error {
	type fmtSpec struct {
		ext string
		fn  func(*model.ProjectMap, string) error
	}
	all := map[string]fmtSpec{
		"json":     {".json", render.JSON},
		"markdown": {".md", render.Markdown},
		"md":       {".md", render.Markdown},
		"html":     {".html", render.HTML},
		"dot":      {".dot", render.DOT},
	}
	want := fs.formats()
	wrote := map[string]bool{}
	for _, f := range want {
		spec, ok := all[f]
		if !ok {
			return fmt.Errorf("неизвестный формат %q (json, markdown, html, dot, all)", f)
		}
		out := fs.out + spec.ext
		if wrote[out] {
			continue
		}
		if err := spec.fn(pm, out); err != nil {
			fmt.Printf("   ⚠️ %s: %v\n", f, err)
			continue
		}
		wrote[out] = true
		fmt.Printf("   📄 %s → %s\n", f, out)
	}
	printSummary(pm)
	return nil
}

func printSummary(pm *model.ProjectMap) {
	fmt.Printf("\n📊 Архитектура: %s · строк: %d · маршрутов: %d\n",
		pm.Summary.Architecture, pm.Summary.TotalLines, pm.Summary.TotalRoutes)
	fmt.Printf("📈 Покрытие: %.1f%% · ср.цикло: %.2f · поддерживаемость: %.0f/100 · долг: %.0f\n",
		pm.Quality.TestCoverage, pm.Quality.AvgCyclomatic, pm.Quality.Maintainability, pm.Metrics.TechnicalDebt)
	if n := len(pm.Quality.Issues); n > 0 {
		fmt.Printf("⚠️  Проблем: %d · критических функций: %d · циклов импортов: %d\n",
			n, len(pm.Business.CriticalPaths), pm.Metrics.CyclesFound)
	}
}

func printJSON(v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}
