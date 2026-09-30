package flow

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseErrors(t *testing.T) {
	for _, src := range []string{"мусор без двоеточия", "прыжок: куда-то", "цикл: пусто", "действие: а\n    действие: б"} {
		if _, err := Parse(src); err == nil {
			t.Errorf("%q: ждали ошибку", src)
		}
	}
}

// Все заготовки и схемы студентов разбираются и рисуются; высокая схема уходит в колонки, а не в длину.
func TestRenderAll(t *testing.T) {
	font, err := os.ReadFile("../../fonts/GOST_A.ttf")
	if err != nil {
		t.Fatal(err)
	}
	files, _ := filepath.Glob("../pz/flows/*.flow")
	more, _ := filepath.Glob("../../students/*/pz/flow/*.flow")
	for _, f := range append(files, more...) {
		src, _ := os.ReadFile(f)
		c, err := Parse(string(src))
		if err != nil {
			t.Errorf("%s: %v", f, err)
			continue
		}
		img, err := Render(c, Options{Font: font})
		if err != nil {
			t.Errorf("%s: %v", f, err)
			continue
		}
		if h := float64(img.Bounds().Dy()) / 8; h > 230 {
			t.Errorf("%s: высота %.0f мм — не влезет на лист", f, h)
		}
	}
}
