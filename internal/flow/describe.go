package flow

import (
	"strings"
)

// Describe — пояснение к схеме алгоритма словами: шаги по порядку, условия и циклы (для ПЗ2 «что делает алгоритм по шагам»).
// Это пересказ самой схемы, поэтому он всегда ей соответствует; студент может заменить его своим текстом.
func Describe(c *Chart) string {
	var b strings.Builder
	title := c.Title
	if title == "" {
		title = "алгоритм"
	}
	steps := describeSeq(c.Nodes, 0)
	b.WriteString("Схема «" + title + "» выполняется так: ")
	b.WriteString(strings.Join(steps, "; "))
	b.WriteString(".")
	return b.String()
}

func lowerFirst(s string) string {
	s = strings.TrimSpace(strings.ReplaceAll(s, `\n`, " "))
	r := []rune(s)
	if len(r) > 1 && r[0] >= 'А' && r[0] <= 'Я' && !(r[1] >= 'А' && r[1] <= 'Я') {
		r[0] = r[0] - 'А' + 'а'
	}
	return strings.TrimRight(string(r), ".;")
}

func describeSeq(ns []*Node, depth int) []string {
	var out []string
	for _, n := range ns {
		t := lowerFirst(n.Text)
		switch n.Kind {
		case Terminator:
			if t == "—" || t == "" {
				continue
			}
			if len(out) == 0 {
				out = append(out, "вход — «"+strings.TrimSpace(n.Text)+"»")
			} else {
				out = append(out, "в конце — "+t)
			}
		case Process:
			out = append(out, t)
		case Predefined:
			out = append(out, "вызывается подпрограмма: "+t)
		case Data:
			out = append(out, "обмен с устройством: "+t)
		case Decision:
			s := "проверяется условие «" + strings.TrimSuffix(strings.TrimSpace(n.Text), "?") + "»"
			if len(n.Yes) > 0 {
				s += ": если да — " + strings.Join(describeSeq(n.Yes, depth+1), ", ")
			}
			if len(n.No) > 0 {
				if len(n.Yes) == 0 {
					s += ": если нет — "
				} else {
					s += ", иначе — "
				}
				s += strings.Join(describeSeq(n.No, depth+1), ", ")
			}
			out = append(out, s)
		case Loop:
			out = append(out, "в цикле "+n.LoopName+" ("+t+") повторяется: "+strings.Join(describeSeq(n.Body, depth+1), ", "))
		}
	}
	return out
}
