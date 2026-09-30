// mpspz — пояснительные записки по варианту студента.
//
//	mpspz -student students/<ник>/variant.yaml -doc pz2          → build/<группа>/<M>/pz2/«Фамилия ИО ПЗ2.docx»
//	mpspz -student … -doc pz2 -init                              → students/<ник>/pz/pz2.md (места «ДОПИШИ») + flow/*.flow
//
// Генерируется всё, что можно посчитать и нарисовать точно; места для текста студента — в students/<ник>/pz/pzN.md.
package main

import (
	"flag"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"mpskp/internal/codecheck"
	"mpskp/internal/flow"
	"mpskp/internal/pz"
	"mpskp/internal/render"
	"mpskp/internal/variant"
)

func main() {
	var (
		table   = flag.String("table", "data/table-2026.yaml", "таблица вариантов")
		student = flag.String("student", "", "students/<ник>/variant.yaml")
		year    = flag.String("year", "23", "год набора группы")
		doc     = flag.String("doc", "pz2", "pz1 | pz2")
		outroot = flag.String("outroot", "build", "корень результатов")
		initF   = flag.Bool("init", false, "создать/дополнить students/<ник>/pz/<doc>.md и заготовки схем алгоритмов")
		fontF   = flag.String("font", "fonts/GOST_A.ttf", "шрифт для рисунков")
	)
	flag.Parse()
	if *student == "" {
		die(fmt.Errorf("нужен -student students/<ник>/variant.yaml"))
	}
	tb, err := variant.LoadTable(*table)
	die(err)
	st, err := variant.LoadStudent(*student)
	die(err)
	st.Defaults(*year, time.Now())
	p, err := variant.Compute(tb, st)
	die(err)
	sdir := filepath.Dir(*student)
	pzdir := filepath.Join(sdir, "pz")
	out := filepath.Join(*outroot, st.Dir(), *doc)
	if !*initF {
		os.RemoveAll(out) // прошлая сборка (старые рисунки)
	}
	die(os.MkdirAll(out, 0o755))
	font, err := os.ReadFile(*fontF)
	die(err)

	textPath := filepath.Join(pzdir, *doc+".md")
	txt, _ := os.ReadFile(textPath)
	d := pz.NewDoc(out, pz.ParseStudent(string(txt)))
	varsInc := render.Asm(p)

	switch *doc {
	case "pz2":
		if *initF {
			initFlows(p, filepath.Join(pzdir, "flow"))
		}
		in := pz.PZ2{P: p, GroupFull: st.GroupFull, Checker: st.Checker, Year: time.Now().Year(), VarsInc: varsInc}
		for n := 1; n <= 3; n++ {
			f := filepath.Join(sdir, "code", fmt.Sprintf("prog%d.a51", n))
			src, err := os.ReadFile(f)
			if err != nil {
				fmt.Fprintf(os.Stderr, "нет %s — программа %d в ПЗ2 не попадёт\n", f, n)
				continue
			}
			r := codecheck.Check(codecheck.Input{Params: p, Prog: n, Src: string(src), Name: filepath.Base(f), VarsInc: varsInc, Seed: n})
			in.Programs = append(in.Programs, pz.Program{N: n, Src: string(src), Report: r})
		}
		flows, _ := filepath.Glob(filepath.Join(pzdir, "flow", "*.flow"))
		sort.Strings(flows)
		for _, f := range flows {
			src, err := os.ReadFile(f)
			die(err)
			c, err := flow.Parse(string(src))
			if err != nil {
				die(fmt.Errorf("%s: %w", f, err))
			}
			img, err := flow.Render(c, flow.Options{Font: font})
			die(err)
			id := strings.TrimSuffix(filepath.Base(f), ".flow")
			pngName := "flow-" + id + ".png"
			w, err := os.Create(filepath.Join(out, pngName))
			die(err)
			die(png.Encode(w, img))
			w.Close()
			title := c.Title
			if title == "" {
				title = id
			}
			in.Flows = append(in.Flows, pz.FlowFig{ID: id, Title: title, File: pngName})
		}
		pz.BuildPZ2(d, in)
	default:
		die(fmt.Errorf("документ %q пока не поддержан", *doc))
	}

	if *initF {
		die(pz.Save(textPath, []byte(pz.StudentTemplate(string(txt), d.Fills, strings.Replace(*doc, "pz", "ПЗ", 1)))))
		fmt.Printf("%s — места «ДОПИШИ»: %d\n", textPath, len(d.Fills))
		return
	}
	name := docName(p.Student, strings.ToUpper(strings.Replace(*doc, "pz", "ПЗ", 1)))
	die(pz.Build(d, filepath.Join(out, name)))
	left := 0
	var todo strings.Builder
	for _, f := range d.Fills {
		if !f.Done {
			left++
			fmt.Fprintf(&todo, "- `%s` — %s\n", f.ID, f.Title)
		}
	}
	rep := fmt.Sprintf("# %s: %s, %s, вариант %d\n\nФайл: `%s`. Мест «ДОПИШИ»: %d, осталось заполнить: %d (students/<ник>/pz/%s.md).\n\n%s",
		strings.Replace(*doc, "pz", "ПЗ", 1), p.Student, st.GroupFull, p.M, name, len(d.Fills), left, *doc, todo.String())
	die(os.WriteFile(filepath.Join(out, "report.md"), []byte(rep), 0o644))
	fmt.Print(rep)
	if s := os.Getenv("GITHUB_STEP_SUMMARY"); s != "" {
		if f, err := os.OpenFile(s, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o644); err == nil {
			f.WriteString(rep + "\n")
			f.Close()
		}
	}
}

// docName — «Рязанцев ИВ ПЗ2.docx» (как в ТЗ: «Михалин СН ПЗ2-v1.pdf»).
func docName(fio, doc string) string {
	f := strings.Fields(fio)
	name := "Фамилия ИО"
	if len(f) > 0 {
		name = f[0]
		if len(f) > 1 {
			name += " " + strings.NewReplacer(".", "", " ", "").Replace(strings.Join(f[1:], ""))
		}
	}
	return name + " " + doc + ".docx"
}

func die(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "mpspz:", err)
		os.Exit(2)
	}
}

// initFlows — заготовки схем алгоритмов, если папка пуста (существующие не трогаются).
func initFlows(p *variant.Params, dir string) {
	if old, _ := filepath.Glob(filepath.Join(dir, "*.flow")); len(old) > 0 {
		fmt.Printf("%s: схемы уже есть (%d) — не трогаю\n", dir, len(old))
		return
	}
	for name, b := range pz.DefaultFlows(p) {
		die(pz.Save(filepath.Join(dir, name), b))
		fmt.Printf("%s — заготовка схемы алгоритма\n", filepath.Join(dir, name))
	}
}
