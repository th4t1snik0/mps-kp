package asm51

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func dirReader(dir string) Reader {
	return func(name string) (string, error) {
		b, err := os.ReadFile(filepath.Join(dir, name))
		return string(b), err
	}
}

func assembleFile(t *testing.T, path string) *Result {
	t.Helper()
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	r := Assemble(filepath.Base(path), string(src), dirReader(filepath.Dir(path)))
	for _, e := range r.Errors {
		t.Errorf("%s", e)
	}
	return r
}

func sameCode(t *testing.T, got map[int]byte, hexPath string) {
	t.Helper()
	b, err := os.ReadFile(hexPath)
	if err != nil {
		t.Fatal(err)
	}
	want, err := ParseHEX(string(b))
	if err != nil {
		t.Fatal(err)
	}
	for a, w := range want {
		if g, ok := got[a]; !ok || g != w {
			t.Errorf("%s: адрес %04Xh: у нас %02X (есть: %v), эталон %02X", hexPath, a, g, ok, w)
		}
	}
	for a := range got {
		if _, ok := want[a]; !ok {
			t.Errorf("%s: лишний байт по адресу %04Xh", hexPath, a)
		}
	}
}

// Эталоны — HEX от MCU 8051 IDE 1.4.9 (= ASEM-51 1.3), ресерч 2026-09-30.
func TestAgainstMIDE(t *testing.T) {
	for _, n := range []string{"test", "m1"} {
		r := assembleFile(t, "testdata/"+n+".a51")
		sameCode(t, r.Code, "testdata/"+n+".hex")
	}
}

func TestEncodings(t *testing.T) {
	cases := []struct{ src, want string }{
		{"mov a,#-1", "74FF"},
		{"mov 30h,31h", "853130"},
		{"mov dptr,#1234h", "901234"},
		{"mov c,P3.4", "A2B4"},
		{"mov 21h.6,c", "920E"},
		{"setb ACC.7", "D2E7"},
		{"anl c,/P1.0", "B090"},
		{"cjne @r1,#5,$", "B705FD"},
		{"djnz 30h,$", "D530FD"},
		{"movc a,@a+dptr", "93"},
		{"jmp @a+dptr", "73"},
		{"mov r7,#LOW(0A6D0h)", "7FD0"},
		{"mov a,#HIGH 0A6D0h", "74A6"},
		{"db 'Ab', 1", "416201"},
		{"orl 90h,#0FH", "43900F"},
		{"xch a,@r0", "C6"},
		{"push acc", "C0E0"},
	}
	for _, c := range cases {
		r := Assemble("t", "CSEG AT 0\n "+c.src+"\nEND\n", nil)
		if len(r.Errors) > 0 {
			t.Errorf("%s: %v", c.src, r.Errors)
			continue
		}
		got := ""
		for i := 0; i < len(r.Code); i++ {
			got += strings.ToUpper(strconv.FormatInt(int64(r.Code[i])|0x100, 16)[1:])
		}
		if got != c.want {
			t.Errorf("%s: %s, ждали %s", c.src, got, c.want)
		}
	}
}

func TestGeneric(t *testing.T) {
	src := "CSEG AT 0\nback: nop\n jmp back\n call back\n jmp fwd\n call fwd\n org 900h\nfwd: jmp back\nEND\n"
	r := Assemble("t", src, nil)
	if len(r.Errors) > 0 {
		t.Fatal(r.Errors)
	}
	want := map[int]byte{0: 0, 1: 0x80, 2: 0xFD, 3: 0x11, 4: 0x00, 5: 0x02, 6: 0x09, 7: 0x00, 8: 0x12, 9: 0x09, 10: 0x00,
		0x900: 0x02, 0x901: 0x00, 0x902: 0x00}
	for a, w := range want {
		if r.Code[a] != w {
			t.Errorf("%04X: %02X, ждали %02X", a, r.Code[a], w)
		}
	}
}

func TestErrors(t *testing.T) {
	for _, src := range []string{"mov r7,r0", "sjmp far\norg 200h\nfar: nop", "movx a,@r2", "mov a,#300", "foo a", "mov a,undefined_name"} {
		r := Assemble("t", "CSEG AT 0\n"+src+"\nEND\n", nil)
		if len(r.Errors) == 0 {
			t.Errorf("%q: ждали ошибку", src)
		}
	}
}
