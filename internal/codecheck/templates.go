package codecheck

import (
	"embed"
	"fmt"
	"strings"

	"mpskp/internal/variant"
)

//go:embed templates/*.a51
var templates embed.FS

// TemplateName — какая заготовка нужна программе n варианта p.
func TemplateName(p *variant.Params, n int) string {
	switch n {
	case 2:
		if p.M%2 == 0 {
			return "prog2r"
		}
		return "prog2w"
	case 3:
		return []string{"prog3ind", "prog3y1", "prog3y2"}[p.M%3]
	}
	return "prog1"
}

// Template — заготовка программы n (рис. 7 ТЗ: заглушки, инициализация, call с %proc%, jmp $ с %stop%).
// Собирается, но логики нет — её пишет студент. groupFull — «А-12-23».
func Template(p *variant.Params, groupFull string, n int) string {
	b, err := templates.ReadFile("templates/" + TemplateName(p, n) + ".a51")
	if err != nil {
		panic(err)
	}
	fio := p.Student
	if fio == "" {
		fio = "Фамилия И.О."
	}
	header := fmt.Sprintf("; %s, %s, %d, v1", fio, groupFull, p.M)
	return strings.Replace(string(b), "{{HEADER}}", header, 1)
}
