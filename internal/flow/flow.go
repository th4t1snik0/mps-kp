// Package flow — схемы алгоритмов по ГОСТ 19.701-90 из простого текстового описания.
//
// Описание — строки с отступом по 2 пробела (вложенность блоков «если» и «цикл»):
//
//	# KbScan — опрос клавиатуры           ← заголовок (подпись рисунка), необязателен
//	начало: KbScan                        ← терминатор
//	действие: Счётчик нажатий := 0        ← процесс (прямоугольник)
//	вызов: BufNext                        ← предопределённый процесс (двойные боковые линии)
//	данные: Прочитать строки из ADR_KB    ← данные (параллелограмм)
//	цикл: Ц1: для столбцов 1…3            ← граница цикла (начало/конец), тело — с отступом
//	  если: Контакт замкнут?              ← решение (ромб); ветви — «да:» и «нет:» с отступом
//	    да:
//	      действие: …
//	    нет:
//	      действие: …
//	конец: Возврат (A — код)              ← терминатор
//
// Текст в блоке переносится по словам; «\n» — принудительный перенос. Раскладка сверху вниз без обратных
// стрелок: повторение — только символом «граница цикла», как допускает ГОСТ 19.701 (п. 3.2.2.6).
package flow

import (
	"fmt"
	"strings"
)

// Kind — вид символа.
type Kind int

const (
	Terminator Kind = iota
	Process
	Predefined
	Data
	Decision
	Loop
)

// Node — блок схемы.
type Node struct {
	Kind     Kind
	Text     string
	LoopName string  // для цикла — имя в символе конца
	Body     []*Node // тело цикла
	Yes, No  []*Node // ветви решения
}

// Chart — схема алгоритма.
type Chart struct {
	Title string
	Nodes []*Node
}

var kinds = map[string]Kind{"начало": Terminator, "конец": Terminator, "действие": Process, "вызов": Predefined,
	"данные": Data, "если": Decision, "цикл": Loop}

type line struct {
	indent int
	key    string
	text   string
	no     int
}

// Parse разбирает описание.
func Parse(src string) (*Chart, error) {
	c := &Chart{}
	var ls []line
	for i, raw := range strings.Split(strings.ReplaceAll(src, "\r\n", "\n"), "\n") {
		t := strings.TrimRight(raw, " \t")
		s := strings.TrimLeft(t, " ")
		if s == "" || strings.HasPrefix(s, "//") {
			continue
		}
		if strings.HasPrefix(s, "#") {
			if c.Title == "" && len(ls) == 0 {
				c.Title = strings.TrimSpace(strings.TrimLeft(s, "#"))
			}
			continue
		}
		ind := len(t) - len(s)
		k, v, ok := strings.Cut(s, ":")
		if !ok {
			return nil, fmt.Errorf("строка %d: нет «вид: текст» — %q", i+1, s)
		}
		ls = append(ls, line{indent: ind, key: strings.ToLower(strings.TrimSpace(k)), text: strings.TrimSpace(v), no: i + 1})
	}
	p := &parser{ls: ls}
	nodes, err := p.block(0)
	if err != nil {
		return nil, err
	}
	if p.i < len(ls) {
		return nil, fmt.Errorf("строка %d: лишний отступ", ls[p.i].no)
	}
	if len(nodes) == 0 {
		return nil, fmt.Errorf("пустая схема")
	}
	c.Nodes = nodes
	return c, nil
}

type parser struct {
	ls    []line
	i     int
	loops int
}

func (p *parser) block(indent int) ([]*Node, error) {
	var out []*Node
	for p.i < len(p.ls) {
		l := p.ls[p.i]
		if l.indent < indent {
			return out, nil
		}
		if l.indent > indent {
			return nil, fmt.Errorf("строка %d: неожиданный отступ", l.no)
		}
		k, ok := kinds[l.key]
		if !ok {
			return nil, fmt.Errorf("строка %d: неизвестный блок %q (начало, конец, действие, вызов, данные, если, цикл)", l.no, l.key)
		}
		p.i++
		n := &Node{Kind: k, Text: l.text}
		switch k {
		case Loop:
			p.loops++
			n.LoopName = fmt.Sprintf("Ц%d", p.loops)
			if name, cond, ok := strings.Cut(l.text, ":"); ok && len([]rune(strings.TrimSpace(name))) <= 4 {
				n.LoopName, n.Text = strings.TrimSpace(name), strings.TrimSpace(cond)
			}
			body, err := p.block(indent + 2)
			if err != nil {
				return nil, err
			}
			if len(body) == 0 {
				return nil, fmt.Errorf("строка %d: у цикла нет тела", l.no)
			}
			n.Body = body
		case Decision:
			for p.i < len(p.ls) && p.ls[p.i].indent == indent+2 && (p.ls[p.i].key == "да" || p.ls[p.i].key == "нет") {
				br := p.ls[p.i]
				p.i++
				b, err := p.block(indent + 4)
				if err != nil {
					return nil, err
				}
				if br.key == "да" {
					n.Yes = b
				} else {
					n.No = b
				}
			}
		}
		out = append(out, n)
	}
	return out, nil
}
