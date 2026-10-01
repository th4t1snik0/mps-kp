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
	// только заготовки генератора: схемы студентов проверяет сборка ПЗ2 (длинную уменьшает и предупреждает),
	// иначе одна длинная схема одного студента роняет тесты и джобу у всех
	files, _ := filepath.Glob("../pz/flows/*.flow")
	for _, f := range files {
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
