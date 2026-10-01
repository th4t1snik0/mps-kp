package pz

import (
	"strings"
	"testing"
)

func TestNormalizeOOXML(t *testing.T) {
	in := `<w:p><w:pPr><w:jc w:val="center"/><w:keepNext/><w:tabs><w:tab w:val="right" w:pos="9"/></w:tabs><w:spacing w:before="0"/><w:rPr><w:u w:val="single"/><w:sz w:val="28"/></w:rPr></w:pPr></w:p>`
	want := `<w:p><w:pPr><w:keepNext/><w:tabs><w:tab w:val="right" w:pos="9"/></w:tabs><w:spacing w:before="0"/><w:jc w:val="center"/><w:rPr><w:sz w:val="28"/><w:u w:val="single"/></w:rPr></w:pPr></w:p>`
	if got := NormalizeOOXML(in); got != want {
		t.Errorf("\nесть  %s\nждём %s", got, want)
	}
	if errs := checkOrder("t", strings.NewReader(`<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">`+want+`</w:document>`)); len(errs) > 0 {
		t.Error(errs)
	}
}
