package schgen

import (
	"strings"
	"testing"
)

func TestFixes(t *testing.T) {
	lib, err := LoadLib("../../masters/lib/mps.kicad_sym")
	if err != nil {
		t.Fatal(err)
	}
	v := variants[3] // ТЗ-2026, дешифратор
	v.Fixes = &Fixes{
		Values:   map[string]string{"R10": "300"},
		BOMNotes: map[string]string{"DD8": "ОЗУ двухпортовое"},
		NoteAdd:  []string{"Резисторы R10–R17 — С2-33Н."},
		Move:     map[string][2]float64{"note": {0, 2.54}},
	}
	sh := Build(lib, v, "fix")
	if len(sh.FixErrs) > 0 {
		t.Fatalf("ошибки правок: %v", sh.FixErrs)
	}
	src := sh.String()
	if !strings.Contains(src, `"Value" "300"`) {
		t.Error("значение R10 на листе не поменялось")
	}
	if !strings.Contains(src, "5. Резисторы R10–R17 — С2-33Н.") {
		t.Error("строки в «Примечании» нет")
	}
	var r10, dd8 bool
	for _, l := range sh.BOM(true) {
		if strings.Contains(l.Refs, "R10") && strings.Contains(l.Name, "300 Ом") {
			r10 = true
		}
		if l.Refs == "DD8" && l.Note == "ОЗУ двухпортовое" {
			dd8 = true
		}
	}
	if !r10 || !dd8 {
		t.Errorf("перечень без правок: R10 %v, DD8 %v", r10, dd8)
	}

	v.Fixes = &Fixes{Values: map[string]string{"R99": "1 к"}, Move: map[string][2]float64{"kb": {1, 1}}}
	if sh := Build(lib, v, "fix"); len(sh.FixErrs) != 2 {
		t.Errorf("ждём 2 ошибки (R99, move kb), есть %v", sh.FixErrs)
	}
}
