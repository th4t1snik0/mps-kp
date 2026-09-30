// mpsgen — генератор материалов курсовой «МПС ч.2» под вариант.
//
//	mpsgen -student students/ivan/variant.yaml -render
//	mpsgen -group А-17 -m 16 -name "Иванов И.И." -render   # любой вариант без файла студента
//
// Результат — в build/<группа>/<вариант>/ (например build/А-17-23/16/): params.md, params.json, vars.inc,
// schematic.kicad_sch (+ .kicad_pro, ramka.kicad_wks); с -render ещё PDF/PNG и ERC (scripts/render.sh, kicad-cli).
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"mpskp/internal/render"
	"mpskp/internal/schgen"
	"mpskp/internal/variant"
)

func main() {
	var (
		table  = flag.String("table", "data/table-2026.yaml", "таблица вариантов из ТЗ")
		stud   = flag.String("student", "", "students/<ник>/variant.yaml")
		group  = flag.String("group", "", "группа (вместо -student)")
		m      = flag.Int("m", 0, "номер варианта (вместо -student)")
		out    = flag.String("out", "", "папка для результатов (по умолчанию <outroot>/<группа>/<вариант>)")
		root   = flag.String("outroot", "build", "корень для результатов")
		name   = flag.String("name", "", "Фамилия И.О. студента в рамку (вместо name в variant.yaml)")
		chk    = flag.String("checker", "", "Фамилия И.О. преподавателя (по умолчанию Михалин С.Н.)")
		year   = flag.String("year", "23", "год набора группы: А-12 → А-12-<year>")
		doRend = flag.Bool("render", false, "после генерации запустить scripts/render.sh (PDF, PNG, ERC)")
		style  = flag.String("style", "", "стиль листа A|B|C|D (пусто — по ФИО, см. internal/schgen/style.go)")
		plain  = flag.Bool("plain", false, "без «почерка» (сдвигов и вариаций шрифтов) — как эталон")
		lib    = flag.String("lib", "masters/lib/mps.kicad_sym", "библиотека символов (пусто — без схемы)")
		wks    = flag.String("wks", "masters/gost_ramka.kicad_wks", "рамка ГОСТ")
	)
	flag.Parse()
	plainSheet = *plain
	dir, err := run(*table, *stud, *group, *m, *name, *chk, *style, *year, *root, *out, *lib, *wks)
	if err == nil && *doRend {
		cmd := exec.Command("scripts/render.sh", dir)
		cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
		err = cmd.Run()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "ошибка:", err)
		os.Exit(1)
	}
}

func run(tablePath, studPath, group string, m int, name, checker, style, year, root, out, libPath, wksPath string) (string, error) {
	tb, err := variant.LoadTable(tablePath)
	if err != nil {
		return "", err
	}
	var st *variant.Student
	if studPath != "" {
		if st, err = variant.LoadStudent(studPath); err != nil {
			return "", err
		}
	} else {
		st = &variant.Student{Group: group, M: m}
	}
	if name != "" {
		st.Name = name
	}
	if checker != "" {
		st.Checker = checker
	}
	if style != "" {
		st.Style = style
	}
	st.Defaults(year, time.Now())
	if out == "" {
		out = filepath.Join(root, st.Dir())
	}
	p, err := variant.Compute(tb, st)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		return "", err
	}
	js, _ := json.MarshalIndent(p, "", "  ")
	files := map[string]string{
		"params.md":   render.Markdown(p),
		"params.json": string(js) + "\n",
		"vars.inc":    render.Asm(p),
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(out, name), []byte(body), 0o644); err != nil {
			return "", err
		}
	}
	fmt.Printf("%s, вариант %d, %s → %s: params.md, params.json, vars.inc\n", st.GroupFull, p.M, st.Name, out)
	for _, w := range p.Warnings {
		fmt.Println("  ⚠", w)
	}
	if libPath == "" {
		return out, nil
	}
	return out, schematic(p, st, out, libPath, wksPath)
}

var plainSheet bool

// schematic рисует схему Э3 с нуля построителем (internal/schgen).
func schematic(p *variant.Params, st *variant.Student, out, libPath, wksPath string) error {
	lib, err := schgen.LoadLib(libPath)
	if err != nil {
		return err
	}
	v := schgen.Variant{
		Cols: p.Cols, Rows: p.Rows, Anode: p.Indicator == "anode",
		CS: schgen.CSPins{
			Buf: p.Dev("Буфер (IDT7005)").CS, Y2: p.Dev("Регистр Y2").CS,
			Kb: p.Dev("Клавиатура").CS, Ind: p.Dev("Индикатор").CS,
		},
		Y1: p.Y1Pin, Y2: p.Y2Pin, KbInt: p.KbInt, X2Int: p.X2Int,
		Decoder: p.CSMode == "decoder", CSEn: p.CSEnPin, Filter: p.FilterCap,
		Style: schgen.PickStyle(st.Style, st.Name),
		Date:  st.Date, Student: st.Name, Checker: st.Checker,
	}
	if !plainSheet {
		j := schgen.MakeJitter(fmt.Sprintf("%s|%s|%d", st.Name, st.GroupFull, p.M), p.Rows)
		v.Jitter = &j
	}
	sh := schgen.Build(lib, v, fmt.Sprintf("%s-%d", p.Group, p.M))
	writes := map[string]string{
		"schematic.kicad_sch": sh.String(),
		"schematic.kicad_pro": schgen.Project,
	}
	if v.Style.FullFrame {
		wksPath = strings.TrimSuffix(wksPath, ".kicad_wks") + "_full.kicad_wks"
	}
	w, err := os.ReadFile(wksPath)
	if err != nil {
		return err
	}
	wks, ok := schgen.PatchWks(string(w), render.VariantLine(st.GroupFull, p.M))
	if !ok {
		fmt.Println("  ⚠ в рамке не нашлась строка «Группа …, Э3» — вариант в рамку не вписан")
	}
	writes["ramka.kicad_wks"] = wks
	// перечень элементов (ГОСТ 2.701): полный — для ПЗ1, черновик КМ-1 — только микросхемы и разъёмы (ТЗ, разд. 3)
	peLine := strings.Replace(render.VariantLine(st.GroupFull, p.M), "Э3", "ПЭ3", 1)
	full, km1 := sh.BOM(true), sh.BOM(false)
	for _, set := range []struct {
		name  string
		lines []schgen.BOMLine
	}{{"perechen", full}, {"perechen-km1", km1}} {
		pages := schgen.PE3(set.lines, fmt.Sprintf("%s-%d-%s", p.Group, p.M, set.name), sh.Title)
		for i, pg := range pages {
			writes[fmt.Sprintf("%s-%d.kicad_sch", set.name, i+1)] = pg.String()
			writes[fmt.Sprintf("%s-%d.kicad_wks", set.name, i+1)] = schgen.PE3Frame(i == 0, i+1, len(pages), peLine, v.Style.FullFrame)
		}
	}
	writes["perechen.md"] = schgen.BOMMarkdown(full, "Перечень элементов — "+peLine) + "\n" +
		schgen.BOMMarkdown(km1, "Черновик для КМ-1 (только микросхемы и разъёмы)")
	for name, body := range writes {
		if err := os.WriteFile(filepath.Join(out, name), []byte(body), 0o644); err != nil {
			return err
		}
	}
	ind := map[bool]string{true: "общий анод", false: "общий катод"}[v.Anode]
	fmt.Printf("  схема: %s (стиль %s, клавиатура %dx%d, %s)\n", filepath.Join(out, "schematic.kicad_sch"), v.Style.Name, p.Cols, p.Rows, ind)
	return nil
}
