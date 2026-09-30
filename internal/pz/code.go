package pz

import (
	"regexp"
	"sort"
	"strings"
)

// Proc — подпрограмма из кода студента (по комментарию перед меткой, который требует ТЗ п. 7).
type Proc struct {
	Prog                             int
	Name, Purpose, In, Out, Clobbers string
	ISR                              string // для обработчиков: источник прерывания
}

var (
	reCall   = regexp.MustCompile(`(?i)^\s*(?:[A-Za-z_]\w*:)?\s*[la]?call\s+([A-Za-z_]\w*)`)
	reVecJmp = regexp.MustCompile(`(?i)^\s*[la]?jmp\s+([A-Za-z_]\w*)`)
	reOrg    = regexp.MustCompile(`(?i)^\s*org\s+([0-9a-f]+)h?\b`)
	reLabel  = regexp.MustCompile(`^\s*([A-Za-z_]\w*)\s*:`)
	reField  = regexp.MustCompile(`(Назначение|Входные параметры|Входные данные|Выходные параметры|Выходные данные|Используемые регистры|Вход|Выход|Портит|Сохраняет)\s*:`)
)

var vectors = map[string]string{"03": "INT0", "0B": "Timer0", "13": "INT1", "1B": "Timer1", "23": "UART"}

// Procs — подпрограммы программы n: цели call и обработчики прерываний (вектор с кодом, а не заглушкой).
func Procs(n int, src string) []Proc {
	lines := strings.Split(strings.ReplaceAll(src, "\r\n", "\n"), "\n")
	code := func(l string) string { c, _ := splitComment(l); return strings.TrimSpace(c) }
	comment := func(l string) string { _, c := splitComment(l); return strings.TrimSpace(c) }
	labelAt := map[string]int{}
	for i, l := range lines {
		if m := reLabel.FindStringSubmatch(code(l)); m != nil {
			labelAt[strings.ToUpper(m[1])] = i
		}
	}
	var out []Proc
	seen := map[string]bool{}
	add := func(name, isr string) {
		u := strings.ToUpper(name)
		i, ok := labelAt[u]
		if !ok || seen[u] {
			return
		}
		seen[u] = true
		var cm []string
		for j := i - 1; j >= 0; j-- {
			t := strings.TrimSpace(lines[j])
			if !strings.HasPrefix(t, ";") {
				break
			}
			cm = append([]string{strings.TrimSpace(strings.TrimLeft(t, ";"))}, cm...)
		}
		p := parseHeader(strings.Join(cm, " "), name)
		p.Prog, p.ISR = n, isr
		out = append(out, p)
	}
	// обработчики: после «org вектор» — переход на метку или код прямо на векторе
	for i, l := range lines {
		m := reOrg.FindStringSubmatch(code(l))
		if m == nil {
			continue
		}
		v := strings.ToUpper(strings.TrimLeft(m[1], "0"))
		if len(v) < 2 {
			v = "0" + v
		}
		src, ok := vectors[v]
		if !ok || i+1 >= len(lines) {
			continue
		}
		next := code(lines[i+1])
		switch {
		case strings.EqualFold(next, "nop"):
		case reVecJmp.MatchString(next):
			add(reVecJmp.FindStringSubmatch(next)[1], src)
		case reCall.MatchString(next):
			// вызов процедуры прямо из вектора (программа 1: call … ; %proc%) — процедура добавится ниже
		case next != "":
			out = append(out, Proc{Prog: n, Name: "(вектор " + v + "h)", ISR: src, Purpose: comment(l)})
		}
	}
	for _, l := range lines {
		if m := reCall.FindStringSubmatch(code(l)); m != nil {
			add(m[1], "")
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].ISR == "" && out[j].ISR != "" })
	return out
}

func parseHeader(txt, name string) Proc {
	p := Proc{Name: name}
	idx := reField.FindAllStringSubmatchIndex(txt, -1)
	head := txt
	if len(idx) > 0 {
		head = txt[:idx[0][0]]
	}
	head = strings.TrimSpace(head)
	for _, pre := range []string{name + " —", name + " -", name + ":"} {
		if strings.HasPrefix(head, pre) {
			head = strings.TrimSpace(head[len(pre):])
		}
	}
	p.Purpose = strings.Trim(strings.TrimRight(head, " ."), "=-; ")
	for k, m := range idx {
		end := len(txt)
		if k+1 < len(idx) {
			end = idx[k+1][0]
		}
		v := strings.Trim(strings.TrimSpace(txt[m[1]:end]), " .=-*")
		switch txt[m[2]:m[3]] {
		case "Назначение":
			if p.Purpose == "" {
				p.Purpose = v
			}
		case "Вход", "Входные параметры", "Входные данные":
			p.In = v
		case "Выход", "Выходные параметры", "Выходные данные":
			p.Out = v
		case "Портит", "Сохраняет", "Используемые регистры":
			if p.Clobbers != "" {
				p.Clobbers += "; "
			}
			if txt[m[2]:m[3]] == "Сохраняет" {
				v = "сохраняет " + v
			}
			p.Clobbers += v
		}
	}
	return p
}

func splitComment(s string) (code, comment string) {
	q := byte(0)
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case q != 0:
			if c == q {
				q = 0
			}
		case c == '\'' || c == '"':
			q = c
		case c == ';':
			return s[:i], s[i+1:]
		}
	}
	return s, ""
}
