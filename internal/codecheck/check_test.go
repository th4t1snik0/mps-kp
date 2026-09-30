package codecheck

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"mpskp/internal/render"
	"mpskp/internal/variant"
)

func params(t *testing.T, group string, m int) *variant.Params {
	t.Helper()
	tb, err := variant.LoadTable("../../data/table-2026.yaml")
	if err != nil {
		t.Fatal(err)
	}
	st := &variant.Student{Group: group, M: m, Name: "Иванов П.С."}
	p, err := variant.Compute(tb, st)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// refFor — эталон для программы n варианта p; первая строка подставляется под вариант.
func refFor(t *testing.T, p *variant.Params, n int) string {
	t.Helper()
	name := map[int]string{1: "prog1"}[n]
	switch n {
	case 2:
		name = map[bool]string{true: "prog2r", false: "prog2w"}[p.M%2 == 0]
	case 3:
		name = []string{"prog3ind", "prog3y1", "prog3y2"}[p.M%3]
	}
	b, err := os.ReadFile("testdata/ref/" + name + ".a51")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	i := strings.IndexByte(src, '\n')
	return fmt.Sprintf("; Иванов ПС, %s-23, %d, v1", p.Group, p.M) + src[i:]
}

func run(t *testing.T, p *variant.Params, n int, src string) *Report {
	t.Helper()
	return Check(Input{Params: p, Prog: n, Src: src, Name: fmt.Sprintf("prog%d.a51", n), VarsInc: render.Asm(p), Seed: 7})
}

func needSim(t *testing.T) {} // эмулятор встроенный — сценарии идут всегда

// Эталоны проходят на вариантах со всеми сочетаниями: 4×3/3×4, чтение/запись, ОК/ОА, все три программы 3.
func TestReferencePass(t *testing.T) {
	needSim(t)
	for _, v := range []struct {
		g string
		m int
	}{{"А-12", 14}, {"А-12", 15}, {"А-17", 16}, {"А-07", 13}, {"Аэ-21", 30}, {"А-09", 1}} {
		p := params(t, v.g, v.m)
		for n := 1; n <= 3; n++ {
			t.Run(fmt.Sprintf("%s_M%d_prog%d", v.g, v.m, n), func(t *testing.T) {
				t.Parallel()
				r := run(t, p, n, refFor(t, p, n))
				if r.Worst() == Fail || !r.SimRun {
					t.Error(r.Markdown())
				} else {
					t.Log(r.Markdown())
				}
			})
		}
	}
}

// Испорченные эталоны должны падать — иначе чекер ничего не ловит.
func TestBrokenFail(t *testing.T) {
	needSim(t)
	cases := []struct {
		name    string
		g       string
		m, n    int
		old, nu string
		item    string // пункт, который должен упасть
	}{
		{"коды клавиш перепутаны", "А-12", 14, 1, "KeyTab4: DB 1, 2, 3, 10", "KeyTab4: DB 1, 2, 3, 11", "Коды клавиш"},
		{"столбцы не вернули в 0", "А-12", 14, 1, "        clr A\n        movx @DPTR, A           ; столбцы снова", "        mov A, #0FFh\n        movx @DPTR, A           ; столбцы снова", "Столбцы"},
		{"нет маски строк", "А-12", 14, 1, "        anl A, #(1 SHL KB_ROWS) - 1\n", "", "Коды клавиш"},
		{"CS_EN = 0", "А-12", 14, 1, "        setb PIN_CSEN           ; дешифратор CS включён постоянно", "        clr PIN_CSEN", "CS_EN"},
		{"чтение без заворота", "А-12", 14, 2, "        mov DPTR, #BUF_START\nbn_ret", "        nop\nbn_ret", "Чтение"},
		{"запись без сдвига хвоста", "А-12", 15, 2, "        mov TAIL_L, DPL\n        mov TAIL_H, DPH\n        setb F_OVF", "        setb F_OVF", "Запись"},
		{"Y2 без X1=0", "А-12", 14, 3, "        jz y2_wr", "        nop", "Y2 и строб"},
		{"строб Y2 до записи", "А-12", 14, 3, "y2_wr:  mov DPTR, #ADR_Y2\n        movx @DPTR, A\n        clr TR1", "y2_wr:  clr PIN_Y2\n        mov DPTR, #ADR_Y2\n        movx @DPTR, A\n        clr TR1", "Y2 и строб"},
		{"Y1 вдвое длиннее", "А-17", 16, 3, "mov Cnt, #LOW(Y1_TICKS)\n        mov Cnt+1, #HIGH(Y1_TICKS)", "mov Cnt, #LOW(Y1_TICKS*2)\n        mov Cnt+1, #HIGH(Y1_TICKS*2)", "Строб Y1"},
		{"индикатор без инверсии для ОА", "А-12", 15, 3, "        xrl A, #IND_OFF         ; у общего анода", "        nop                     ; у общего анода", "Символы"},
		{"%proc% с пробелом", "А-12", 14, 1, "; %proc%", "; % proc%", "Пометки роботу"},
		{"нет заглушки", "А-12", 14, 1, "org 23h ; \"заглушка\" для UART\n        nop\n        reti\n", "", "Заглушки IRQ"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			p := params(t, c.g, c.m)
			src := refFor(t, p, c.n)
			if !strings.Contains(src, c.old) {
				t.Fatalf("в эталоне нет %q", c.old)
			}
			r := run(t, p, c.n, strings.Replace(src, c.old, c.nu, 1))
			for _, it := range r.Items {
				if it.Name == c.item && it.Level == Fail {
					t.Log(it.Msg)
					return
				}
			}
			t.Errorf("ждали ❌ «%s»:\n%s", c.item, r.Markdown())
		})
	}
}

func TestRobotName(t *testing.T) {
	for fio, want := range map[string]string{"Рязанцев И.В.": "Рязанцев ИВ-код-2.txt", "Михалин С. Н.": "Михалин СН-код-2.txt", "": "Фамилия ИО-код-2.txt"} {
		if got := RobotName(fio, 2); got != want {
			t.Errorf("%q → %q, ждали %q", fio, got, want)
		}
	}
}

// Заготовки собираются и проходят правила робота; логики в них нет — сценарии падают.
func TestTemplates(t *testing.T) {
	seen := map[string]bool{}
	for _, v := range []struct {
		g string
		m int
	}{{"А-12", 14}, {"А-12", 15}, {"А-17", 16}, {"А-07", 13}, {"Аэ-21", 30}, {"А-09", 1}} {
		p := params(t, v.g, v.m)
		for n := 1; n <= 3; n++ {
			name := TemplateName(p, n)
			if seen[name] {
				continue
			}
			seen[name] = true
			src := Template(p, p.Group+"-23", n)
			r := Check(Input{Params: p, Prog: n, Src: src, Name: name + ".a51", VarsInc: render.Asm(p), NoSim: true})
			if r.Worst() == Fail {
				t.Errorf("%s:\n%s", name, r.Markdown())
			}
		}
	}
	if len(seen) != 6 {
		t.Errorf("заготовок проверено %d, ждали 6", len(seen))
	}
}
