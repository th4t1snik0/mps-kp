package pz

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"mpskp/internal/codecheck"
	"mpskp/internal/render"
	"mpskp/internal/variant"
)

func TestStudentMerge(t *testing.T) {
	src := "<!-- шапка -->\n## p2.ide — Среда\n> ✍ подсказка\nMCU 8051 IDE 1.4.9.\n\n## p2.alg — Алгоритм\n> ✍ подсказка\n"
	m := ParseStudent(src)
	if m["p2.ide"] != "MCU 8051 IDE 1.4.9." || m["p2.alg"] != "" {
		t.Fatalf("%q", m)
	}
	fills := []Fill{{ID: "p2.alg", Title: "Алгоритм", Hint: "х"}, {ID: "p2.ide", Title: "Среда", Hint: "у"}}
	out := StudentTemplate(src+"## old.one — было\nтекст\n", fills, "ПЗ2")
	if strings.Index(out, "## p2.alg") > strings.Index(out, "## p2.ide") || !strings.Contains(out, "MCU 8051 IDE 1.4.9.") ||
		!strings.Contains(out, "## old.one — НЕ ИСПОЛЬЗУЕТСЯ") {
		t.Fatal(out)
	}
}

func TestProcs(t *testing.T) {
	src := "org 0bh\n        ljmp T0Isr\norg 1bh ; конец строба\n        clr TR1\n call Foo ; %proc%\n" +
		"; Foo — делает дело.\n; Вход: A — код. Выход: C — флаг.\n; Портит: R7.\nFoo: ret\n; T0Isr — тик\nT0Isr: reti\n"
	ps := Procs(1, src)
	if len(ps) != 3 || ps[0].Name != "Foo" || ps[0].Purpose != "делает дело" || ps[0].In != "A — код" || ps[0].Out != "C — флаг" || ps[0].Clobbers != "R7" {
		t.Fatalf("%+v", ps)
	}
}

// Полная сборка ПЗ2 на эталонах чекера (нужен pandoc).
func TestBuildPZ2(t *testing.T) {
	if _, err := exec.LookPath(Pandoc()); err != nil {
		t.Skip("нет pandoc")
	}
	tb, _ := variant.LoadTable("../../data/table-2026.yaml")
	p, err := variant.Compute(tb, &variant.Student{Group: "А-12", M: 14, Name: "Иванов П.С."})
	if err != nil {
		t.Fatal(err)
	}
	vars := render.Asm(p)
	in := PZ2{P: p, GroupFull: "А-12-23", Checker: "Михалин С.Н.", Year: 2026, VarsInc: vars}
	for n, f := range map[int]string{1: "prog1", 2: "prog2r", 3: "prog3y2"} {
		b, _ := os.ReadFile("../codecheck/testdata/ref/" + f + ".a51")
		r := codecheck.Check(codecheck.Input{Params: p, Prog: n, Src: string(b), VarsInc: vars, Seed: n})
		in.Programs = append(in.Programs, Program{N: n, Src: string(b), Report: r})
	}
	dir := t.TempDir()
	d := NewDoc(dir, map[string]string{"p2.ide": "MCU 8051 IDE 1.4.9."})
	BuildPZ2(d, in)
	out := filepath.Join(dir, "pz2.docx")
	if err := Build(d, out); err != nil {
		t.Fatal(err)
	}
	txt, err := exec.Command(Pandoc(), out, "-t", "plain").Output()
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"MCU 8051 IDE 1.4.9.", "ДОПИШИ (p2.alg", "KbScan", "BufRead", "Y2Out", "1744 / 4 = 436,0 с", "глубина стека для программы 1"} {
		if !strings.Contains(string(txt), want) {
			t.Errorf("нет %q", want)
		}
	}
}

// Заголовок в стиле принятых работ («Назначение:», «Входные параметры:», «Используемые регистры:», разделители ====).
func TestProcsOsipovaStyle(t *testing.T) {
	src := " call Scan\n; ===========================\n; Назначение: опрос клавиатуры\n; Входные параметры: нет\n; Выходные параметры: A — код\n; Используемые регистры: R0, R1\n; ===========================\nScan: ret\n"
	ps := Procs(1, src)
	if len(ps) != 1 || ps[0].Purpose != "опрос клавиатуры" || ps[0].In != "нет" || ps[0].Out != "A — код" || ps[0].Clobbers != "R0, R1" {
		t.Fatalf("%+v", ps)
	}
}
