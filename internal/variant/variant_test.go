package variant

import "testing"

func load(t *testing.T) *Table {
	t.Helper()
	tb, err := LoadTable("../../data/table-2025.yaml")
	if err != nil {
		t.Fatal(err)
	}
	return tb
}

// Эталон — вариант, посчитанный руками в ПЗ1: А-12, M=14.
func TestA12M14(t *testing.T) {
	p, err := Compute(load(t), &Student{Group: "А-12", M: 14})
	if err != nil {
		t.Fatal(err)
	}
	check := func(name string, got, want any) {
		t.Helper()
		if got != want {
			t.Errorf("%s: got %v, want %v", name, got, want)
		}
	}
	check("k", p.K, 0)
	check("Y1", p.Y1Pin, "P1.0")
	check("Y2", p.Y2Pin, "P1.1")
	check("T1", p.T1ms, 352)
	check("V", p.V, 1540)
	check("T2", p.T2us, 1745)
	check("T3", p.T3ms, 2830)
	check("T2 reload", Hex(p.T2Reload), "0F92Fh")
	check("empty bit", p.Empty.Asm, "23h.2")
	check("ovf bit", p.Ovf.Asm, "21h.6")
	check("keyboard", p.Keyboard, "4x3")
	check("kb int", p.KbIntPin, "P3.2")
	check("ind", Hex(p.Dev("Индикатор").Base), "7800h")
	check("kb", Hex(p.Dev("Клавиатура").Base), "0B800h")
	check("buf", Hex(p.Dev("Буфер (IDT7005)").Base), "0D800h")
	check("y2", Hex(p.Dev("Регистр Y2").Base), "0E800h")
	check("unused", p.UnusedCS, "P2.3")
	check("fill", p.FillTimeS, 308.0)
	check("tick", p.Y1.TickMs, 2)
	check("y1 ticks", p.Y1.Ticks, 176)
	check("t3 ticks", p.T3.Ticks, 1415)
}

// Все группы и все M считаются без ошибок и без конфликтов CS.
func TestAllVariants(t *testing.T) {
	tb := load(t)
	for g := range tb.Groups {
		for m := 1; m <= 30; m++ {
			if _, err := Compute(tb, &Student{Group: g, M: m}); err != nil {
				t.Errorf("%s M=%d: %v", g, m, err)
			}
		}
	}
}
