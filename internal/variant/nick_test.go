package variant

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

func TestNick(t *testing.T) {
	for _, c := range []struct{ g, fio, want string }{
		{"А-12", "Рязанцев И.В.", "a12_ryazantsev_iv"},
		{"Аэ-21", "Щукина Ю.Э.", "ae21_shchukina_yue"},
		{"А-06", "Хрусталёв Я. Ж.", "a06_khrustalev_yazh"},
		{"А-17", "Осипова Мария Андреевна", "a17_osipova_ma"},
	} {
		if got := Nick(c.g, c.fio); got != c.want {
			t.Errorf("Nick(%q, %q) = %q, ждём %q", c.g, c.fio, got, c.want)
		}
	}
}

// Папки students/ названы по правилу «группа_фамилия_ио» (кроме образцов example-*), варианты в группе не повторяются.
func TestStudentDirs(t *testing.T) {
	files, _ := filepath.Glob("../../students/*/variant.yaml")
	seen := map[string]string{}
	for _, f := range files {
		dir := filepath.Base(filepath.Dir(f))
		if strings.HasPrefix(dir, "example") {
			continue
		}
		s, err := LoadStudent(f)
		if err != nil {
			t.Error(err)
			continue
		}
		if want := Nick(s.Group, s.Name); dir != want {
			t.Errorf("students/%s: по group и name папка должна называться students/%s (правило: латиницей группа_фамилия_ио)", dir, want)
		}
		key := fmt.Sprintf("%s/%d", s.Group, s.M)
		if other, ok := seen[key]; ok {
			t.Errorf("students/%s и students/%s: одна группа и один вариант — результаты лягут в одну папку", dir, other)
		}
		seen[key] = dir
	}
}
