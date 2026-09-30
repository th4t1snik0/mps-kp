package pz

import (
	"embed"

	"mpskp/internal/variant"
)

//go:embed flows/*.flow
var flowFS embed.FS

// DefaultFlows — заготовки схем всех алгоритмов из ТЗ-2026 (разд. 2.2: инициализация, индикация, строб Y1, Y2 со стробом,
// буфер — чтение и запись, клавиатура, обработчики прерываний) + основной цикл. Схемы процедур, которые есть в коде студента,
// он подгоняет под свой код (схема обязана соответствовать программе); остальные — алгоритмы полной системы.
func DefaultFlows(p *variant.Params) map[string][]byte {
	out := map[string][]byte{}
	ents, _ := flowFS.ReadDir("flows")
	for _, e := range ents {
		b, err := flowFS.ReadFile("flows/" + e.Name())
		if err != nil {
			panic(err)
		}
		out[e.Name()] = b
	}
	return out
}
