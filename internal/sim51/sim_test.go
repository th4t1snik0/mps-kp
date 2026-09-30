package sim51

import (
	"os"
	"path/filepath"
	"testing"

	"mpskp/internal/asm51"
)

func load(t *testing.T) *Sim {
	t.Helper()
	if !Available() {
		if os.Getenv("MPS_REQUIRE_S51") != "" {
			t.Fatal("нет s51, а MPS_REQUIRE_S51 задан")
		}
		t.Skip("нет s51 (brew install sdcc / S51=путь)")
	}
	dir := "../asm51/testdata"
	src, _ := os.ReadFile(filepath.Join(dir, "test.a51"))
	r := asm51.Assemble("test.a51", string(src), func(n string) (string, error) {
		b, err := os.ReadFile(filepath.Join(dir, n))
		return string(b), err
	})
	if len(r.Errors) > 0 {
		t.Fatal(r.Errors)
	}
	hex := filepath.Join(t.TempDir(), "t.hex")
	os.WriteFile(hex, []byte(r.HEX()), 0o644)
	s, err := Start(hex, 1)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	return s
}

// test.a51: YourFunc пишет 0FEh в клавиатуру (2000h), читает строки, копирует XRAM A000.. в IRAM, пишет Y2 (C000h); jmp $ по 0037h.
func TestProtocol(t *testing.T) {
	s := load(t)
	s.SetXRAM(0xA000, 0x11, 0x22, 0x33)
	s.BreakMem("xram", 'w', 0x2000)
	stop := s.BreakCode(0x37)
	st := s.Run(10000)
	if !st.Event || st.PC != 0x45 {
		t.Fatalf("ждали событие записи в xram[2000]: %+v", st)
	}
	if v := s.XRAM(0x2000); v != 0xFE {
		t.Fatalf("xram[2000] = %02X", v)
	}
	s.SetXRAM(0x2000, 0xF7)
	st = s.Run(10000)
	if st.Event || st.Timeout || st.PC != 0x37 {
		t.Fatalf("ждали останов на 0037: %+v", st)
	}
	if v := s.IRAM(0x31); v != 0xF7 {
		t.Fatalf("строки прочитаны как %02X", v)
	}
	if !s.Bit(0x1A) || s.Bit(0x0E) {
		t.Fatal("флаги F_OVF/F_EMPTY")
	}
	// с брейкпоинта на jmp $ — снова он же через 2 МЦ
	t0 := s.Micros()
	st = s.Run(1000)
	if st.PC != 0x37 || st.Timeout || st.Micros-t0 > 3 {
		t.Fatalf("повторный заход на jmp $: %+v (dt %.1f)", st, st.Micros-t0)
	}
	s.Delete(stop)
	st = s.Run(1000)
	if !st.Timeout {
		t.Fatalf("без брейкпоинтов ждали таймаут: %+v", st)
	}
	if s.Latch(1)&1 != 1 {
		t.Fatal("P1.0 после setb")
	}
	// PC на брейкпоинт и прогон — не залипает на месте
	s.BreakCode(0x39)
	s.SetPC(0x34)
	st = s.Run(1000)
	if st.PC != 0x39 {
		t.Fatalf("после pc 0034 ждали 0039: %+v", st)
	}
	s.SetPC(0x39)
	st = s.Run(1000)
	t.Logf("старт с PC на брейкпоинте: %+v", st)
}
