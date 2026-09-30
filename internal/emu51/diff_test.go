package emu51

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"testing"

	"mpskp/internal/asm51"
	"mpskp/internal/render"
	"mpskp/internal/sim51"
	"mpskp/internal/variant"
)

// Сверка с ucsim s51 (эталон). Без s51 тест пропускается — в CI его нет, гоняется локально при правке эмулятора.

// randProg — случайная программа без переходов из всех команд 8051 (ветвления — со смещением 0,
// чтобы проверить флаги и такты без ухода в сторону), в конце sjmp $.
func randProg(r *rand.Rand, n int) []byte {
	dir := func() byte {
		if r.Intn(4) == 0 {
			return []byte{0xE0, 0xF0, 0xD0, 0x82, 0x83, 0x90}[r.Intn(6)] // ACC B PSW DPL DPH P1
		}
		return byte(r.Intn(0x80))
	}
	bit := func() byte {
		if r.Intn(4) == 0 {
			return []byte{0xE0, 0xF0, 0xD0, 0x90}[r.Intn(4)] + byte(r.Intn(8))
		}
		return byte(r.Intn(0x80))
	}
	var p []byte
	for len(p) < n {
		op := byte(r.Intn(256))
		lo, hi := op&0x0F, op>>4
		switch {
		case op&0x1F == 0x01, op&0x1F == 0x11, op == 0x02, op == 0x12, op == 0x22, op == 0x32, op == 0x73, op == 0xA5,
			op == 0xE0, op == 0xE2, op == 0xE3, op == 0xF0, op == 0xF2, op == 0xF3, op == 0x83, op == 0x93:
			continue // переходы, movx, movc — отдельно
		case op == 0x84 && r.Intn(3) > 0:
			continue
		}
		p = append(p, op)
		switch {
		case op == 0x10 || op == 0x20 || op == 0x30:
			p = append(p, bit(), 0)
		case op == 0x40 || op == 0x50 || op == 0x60 || op == 0x70 || op == 0x80:
			p = append(p, 0)
		case op == 0x72 || op == 0x82 || op == 0xA0 || op == 0xB0 || op == 0x92 || op == 0xA2 || op == 0xB2 || op == 0xC2 || op == 0xD2:
			p = append(p, bit())
		case op == 0x75 || op == 0x43 || op == 0x53 || op == 0x63:
			p = append(p, dir(), byte(r.Intn(256)))
		case op == 0x85:
			p = append(p, dir(), dir())
		case op == 0x90:
			p = append(p, byte(r.Intn(256)), byte(r.Intn(256)))
		case op == 0xB4 || op >= 0xB6 && op <= 0xBF:
			p = append(p, byte(r.Intn(256)), 0)
		case op == 0xB5 || op == 0xD5:
			p = append(p, dir(), 0)
		case op >= 0xD8 && op <= 0xDF:
			p = append(p, 0)
		case op == 0xC0 || op == 0xD0 || op == 0x05 || op == 0x15 || op == 0x42 || op == 0x52 || op == 0x62 ||
			op == 0xC5 || op == 0xE5 || op == 0xF5 || op == 0xA6 || op == 0xA7 || op == 0x86 || op == 0x87 ||
			op >= 0x88 && op <= 0x8F || op >= 0xA8 && op <= 0xAF:
			p = append(p, dir())
		case lo == 4 && (hi >= 2 && hi <= 6 || hi == 9 || hi == 7) || op == 0x76 || op == 0x77 || op >= 0x78 && op <= 0x7F:
			p = append(p, byte(r.Intn(256)))
		case lo == 5 && (hi >= 2 && hi <= 6 || hi == 9):
			p = append(p, dir())
		}
	}
	return append(p, 0x80, 0xFE)
}

// needS51 — сверка долгая (~2 мин) и требует s51: только по MPS_S51_DIFF=1 (после правки эмулятора).
func needS51(t *testing.T) {
	if os.Getenv("MPS_S51_DIFF") == "" {
		t.Skip("сверка с s51: MPS_S51_DIFF=1 go test ./internal/emu51")
	}
	if !sim51.Available() {
		t.Fatal("нет s51 (brew install sdcc)")
	}
}

func hexOf(t *testing.T, code []byte) string {
	m := map[int]byte{}
	for i, b := range code {
		m[i] = b
	}
	f := filepath.Join(t.TempDir(), "p.hex")
	os.WriteFile(f, []byte((&asm51.Result{Code: m}).HEX()), 0o644)
	return f
}

var sfrs = []byte{ACC, B, PSW, SP, DPL, DPH, P1, TCON, TMOD, TL0, TH0, TL1, TH1, IE, IP}

func compare(t *testing.T, name string, c *CPU, s *sim51.Sim) {
	t.Helper()
	if pc := s.PC(); pc != int(c.PC) {
		t.Errorf("%s: PC s51 %04X, у нас %04X", name, pc, c.PC)
	}
	if cl := s.Clocks() / 12; cl != int64(c.Cycles) {
		t.Errorf("%s: МЦ s51 %d, у нас %d", name, cl, c.Cycles)
	}
	for _, a := range sfrs {
		mask := byte(0xFF)
		if a == PSW {
			mask = 0xFE // P: в железе всегда = чётность A; s51 после прямой записи в PSW хранит записанное
		}
		if v := s.SFR(int(a)); v&mask != c.SFRByte(a)&mask {
			t.Errorf("%s: SFR %02X: s51 %02X, у нас %02X", name, a, v, c.SFRByte(a))
		}
	}
	m := s.DumpIRAM()
	for i := range m {
		if m[i] != c.IRAM[i] {
			t.Errorf("%s: IRAM[%02X]: s51 %02X, у нас %02X", name, i, m[i], c.IRAM[i])
		}
	}
}

func TestRandomAgainstS51(t *testing.T) {
	needS51(t)
	r := rand.New(rand.NewSource(1))
	for k := 0; k < 60; k++ {
		code := randProg(r, 300)
		c := New(int64(k))
		copy(c.Code[:], code)
		s, err := sim51.Start(hexOf(t, code), 1)
		if err != nil {
			t.Fatal(err)
		}
		for a := 0; a < 256; a += 64 {
			s.SetIRAM(a, c.IRAM[a:a+64]...)
		}
		end := len(code) - 2
		s.BreakCode(end)
		s.Run(100000)
		for c.PC != uint16(end) && c.Cycles < 100000 {
			c.Step()
		}
		compare(t, fmt.Sprintf("программа %d", k), c, s)
		s.Close()
		if t.Failed() {
			t.Logf("код: % X", code)
			return
		}
	}
}

// Эталоны чекера (таймеры, прерывания, movx): одинаковое модельное время в обоих — одинаковое состояние.
func TestTimersAgainstS51(t *testing.T) {
	needS51(t)
	tb, err := variant.LoadTable("../../data/table-2026.yaml")
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		g    string
		m    int
		file string
		int0 bool // дёргать INT0
	}{{"А-17", 16, "prog3y1", false}, {"А-12", 15, "prog3ind", false}, {"А-12", 14, "prog3y2", false}, {"А-12", 14, "prog1", true}} {
		p, err := variant.Compute(tb, &variant.Student{Group: c.g, M: c.m})
		if err != nil {
			t.Fatal(err)
		}
		src, _ := os.ReadFile("../codecheck/testdata/ref/" + c.file + ".a51")
		vars := render.Asm(p)
		res := asm51.Assemble(c.file, string(src), func(string) (string, error) { return vars, nil })
		if len(res.Errors) > 0 {
			t.Fatal(res.Errors)
		}
		f := filepath.Join(t.TempDir(), "p.hex")
		os.WriteFile(f, []byte(res.HEX()), 0o644)
		cpu := New(3)
		cpu.VectorCycles = 1 // как в s51
		for a, b := range res.Code {
			cpu.Code[a] = b
		}
		s, err := sim51.Start(f, 1)
		if err != nil {
			t.Fatal(err)
		}
		for a := 0; a < 256; a += 64 {
			s.SetIRAM(a, cpu.IRAM[a:a+64]...)
		}
		for i, us := range []int{3000, 20000, 400000} {
			if c.int0 {
				pin := byte(0xFF)
				if i%2 == 0 {
					pin = 0xFB
				}
				s.SetPins(3, pin)
				cpu.Pins[3] = pin
			}
			s.Run(float64(us))
			target := uint64(s.Clocks() / 12)
			for cpu.Cycles < target {
				cpu.Step()
			}
			compare(t, fmt.Sprintf("%s после %d мкс", c.file, us), cpu, s)
		}
		s.Close()
	}
}
