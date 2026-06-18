package main

import (
	"flag"
	"strings"
)

type flags struct {
	fs      *flag.FlagSet
	path    string
	out     string
	format  string
	impact  string
	depth   int
	trace   string
	web     bool
	port    int
}

func newFlags() *flags {
	f := &flags{fs: flag.NewFlagSet("mapgen", flag.ContinueOnError)}
	f.fs.StringVar(&f.path, "path", ".", "корень анализируемого проекта")
	f.fs.StringVar(&f.out, "out", "mapgen", "префикс выходных файлов")
	f.fs.StringVar(&f.format, "format", "all", "форматы через запятую: json,markdown,html,dot или all")
	f.fs.StringVar(&f.impact, "impact", "", "анализ влияния символа (что сломается при изменении)")
	f.fs.IntVar(&f.depth, "depth", 4, "глубина обхода для -impact")
	f.fs.StringVar(&f.trace, "trace", "", "трассировка пути вызова в формате <from>..<to>")
	f.fs.BoolVar(&f.web, "web", false, "поднять интерактивный дашборд")
	f.fs.IntVar(&f.port, "port", 8080, "порт веб-сервера")
	return f
}

func (f *flags) parse(args []string) error { return f.fs.Parse(args) }

func (f *flags) formats() []string {
	if strings.TrimSpace(f.format) == "" || f.format == "all" {
		return []string{"json", "markdown", "html", "dot"}
	}
	var out []string
	for _, p := range strings.Split(f.format, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
