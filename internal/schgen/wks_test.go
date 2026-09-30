package schgen

import (
	"os"
	"strings"
	"testing"
)

func TestPatchWks(t *testing.T) {
	b, err := os.ReadFile("../../masters/gost_ramka.kicad_wks")
	if err != nil {
		t.Fatal(err)
	}
	out, ok := PatchWks(string(b), "Группа А-17-22, Вариант 21, Э3")
	if !ok || !strings.Contains(out, "Группа А-17-22, Вариант 21, Э3") {
		t.Fatal("вариант не вписан в рамку")
	}
}
