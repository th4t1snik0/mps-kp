// Package codecheck — проверка программ курсовой (КМ-3) до отправки роботу:
// правила оформления (ТЗ-2026, разд. 2.2 и рис. 7), сборка своим ассемблером (asm51), текст для робота
// с вклеенным vars.inc и сценарии в симуляторе s51 с моделями устройств схемы.
//
// Соглашения, которые проверяют сценарии (ТЗ их не фиксирует — выбраны нами, см. docs/code-guide.md):
// голова (HEAD) — адрес ячейки для следующей записи, хвост (TAIL) — для следующего чтения;
// пустой буфер — F_EMPTY = 1 и HEAD = TAIL; HEAD = TAIL при F_EMPTY = 0 — буфер полон.
package codecheck

import (
	"fmt"
	"strings"
)

// Level — итог пункта проверки.
type Level int

const (
	Pass Level = iota
	Warn
	Fail
)

func (l Level) Icon() string { return [...]string{"✅", "⚠️", "❌"}[l] }

// Item — пункт отчёта.
type Item struct {
	Level Level
	Name  string
	Msg   string
}

// Report — итог проверки одной программы.
type Report struct {
	Prog      int
	Title     string
	Items     []Item
	Listing   string
	HEX       string
	Robot     string // текст для робота (vars.inc вклеен)
	RobotName string // «Фамилия ИО-код-n.txt»
	SimRun    bool   // сценарии в симуляторе выполнены
	MaxSP     int    // наибольшее значение SP за все сценарии (глубина стека = MaxSP − 07h)
}

func (r *Report) add(l Level, name, format string, a ...any) {
	r.Items = append(r.Items, Item{Level: l, Name: name, Msg: fmt.Sprintf(format, a...)})
}

func (r *Report) pass(name, format string, a ...any) { r.add(Pass, name, format, a...) }
func (r *Report) warn(name, format string, a ...any) { r.add(Warn, name, format, a...) }
func (r *Report) fail(name, format string, a ...any) { r.add(Fail, name, format, a...) }

// Worst — худший уровень.
func (r *Report) Worst() Level {
	w := Pass
	for _, it := range r.Items {
		w = max(w, it.Level)
	}
	return w
}

// Markdown — отчёт для людей и ИИ.
func (r *Report) Markdown() string {
	var b strings.Builder
	fmt.Fprintf(&b, "## %s Программа %d — %s\n\n", r.Worst().Icon(), r.Prog, r.Title)
	if r.RobotName != "" {
		fmt.Fprintf(&b, "Файл для робота: `%s`\n\n", r.RobotName)
	}
	b.WriteString("| | Проверка | Результат |\n| --- | --- | --- |\n")
	for _, it := range r.Items {
		fmt.Fprintf(&b, "| %s | %s | %s |\n", it.Level.Icon(), it.Name, strings.ReplaceAll(it.Msg, "|", "\\|"))
	}
	if !r.SimRun {
		b.WriteString("\nСценарии в симуляторе не запускались.\n")
	}
	return b.String()
}
