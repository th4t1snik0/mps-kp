// Package schgen строит схему Э3 в формате KiCad с нуля: символы берутся из
// masters/lib/mps.kicad_sym, расстановка и связи задаются кодом (mps.go).
package schgen

import (
	"fmt"
	"strconv"
	"strings"
)

// Node — S-выражение: либо атом (Kids == nil, Atom != ""), либо список.
type Node struct {
	Atom   string
	Quoted bool
	Kids   []*Node
	list   bool
}

func A(s string) *Node { return &Node{Atom: s} }
func Q(s string) *Node { return &Node{Atom: s, Quoted: true} }
func F(f float64) *Node {
	return &Node{Atom: fmtNum(f)}
}

// L — список (head ...kids).
func L(head string, kids ...*Node) *Node {
	n := &Node{list: true, Kids: []*Node{A(head)}}
	n.Kids = append(n.Kids, kids...)
	return n
}

func (n *Node) IsList() bool { return n.list }

// Head — первый атом списка.
func (n *Node) Head() string {
	if !n.list || len(n.Kids) == 0 || n.Kids[0].list {
		return ""
	}
	return n.Kids[0].Atom
}

// Arg — i-й аргумент (после головы) как строка.
func (n *Node) Arg(i int) string {
	if i+1 < len(n.Kids) && !n.Kids[i+1].list {
		return n.Kids[i+1].Atom
	}
	return ""
}

func (n *Node) Num(i int) float64 {
	f, _ := strconv.ParseFloat(n.Arg(i), 64)
	return f
}

// Find — первый дочерний список с головой head.
func (n *Node) Find(head string) *Node {
	for _, k := range n.Kids {
		if k.Head() == head {
			return k
		}
	}
	return nil
}

func (n *Node) All(head string) []*Node {
	var r []*Node
	for _, k := range n.Kids {
		if k.Head() == head {
			r = append(r, k)
		}
	}
	return r
}

// Remove — выкинуть дочерние списки, для которых f истинно.
func (n *Node) Remove(f func(*Node) bool) {
	out := n.Kids[:0]
	for _, k := range n.Kids {
		if !f(k) {
			out = append(out, k)
		}
	}
	n.Kids = out
}

func (n *Node) Clone() *Node {
	c := *n
	if n.Kids != nil {
		c.Kids = make([]*Node, len(n.Kids))
		for i, k := range n.Kids {
			c.Kids[i] = k.Clone()
		}
	}
	return &c
}

// Walk — обход в глубину.
func (n *Node) Walk(f func(*Node)) {
	f(n)
	for _, k := range n.Kids {
		k.Walk(f)
	}
}

// Parse разбирает текст S-выражения (один корень).
func Parse(src string) (*Node, error) {
	p := &parser{s: src}
	p.ws()
	n, err := p.node()
	if err != nil {
		return nil, err
	}
	return n, nil
}

type parser struct {
	s string
	i int
}

func (p *parser) ws() {
	for p.i < len(p.s) && strings.ContainsRune(" \t\r\n", rune(p.s[p.i])) {
		p.i++
	}
}

func (p *parser) node() (*Node, error) {
	if p.i >= len(p.s) {
		return nil, fmt.Errorf("неожиданный конец")
	}
	switch c := p.s[p.i]; c {
	case '(':
		p.i++
		n := &Node{list: true}
		for {
			p.ws()
			if p.i >= len(p.s) {
				return nil, fmt.Errorf("незакрытая скобка")
			}
			if p.s[p.i] == ')' {
				p.i++
				return n, nil
			}
			k, err := p.node()
			if err != nil {
				return nil, err
			}
			n.Kids = append(n.Kids, k)
		}
	case '"':
		p.i++
		var b strings.Builder
		for p.i < len(p.s) && p.s[p.i] != '"' {
			if p.s[p.i] == '\\' && p.i+1 < len(p.s) {
				p.i++
				switch p.s[p.i] {
				case 'n':
					b.WriteByte('\n')
				default:
					b.WriteByte(p.s[p.i])
				}
			} else {
				b.WriteByte(p.s[p.i])
			}
			p.i++
		}
		p.i++
		return &Node{Atom: b.String(), Quoted: true}, nil
	default:
		st := p.i
		for p.i < len(p.s) && !strings.ContainsRune(" \t\r\n()", rune(p.s[p.i])) {
			p.i++
		}
		return &Node{Atom: p.s[st:p.i]}, nil
	}
}

// String — вывод в стиле KiCad: список с вложенными списками — по строкам с табами.
func (n *Node) String() string {
	var b strings.Builder
	n.write(&b, 0)
	b.WriteByte('\n')
	return b.String()
}

func (n *Node) write(b *strings.Builder, depth int) {
	if !n.list {
		if n.Quoted {
			b.WriteString(strconv.Quote(n.Atom))
		} else {
			b.WriteString(n.Atom)
		}
		return
	}
	b.WriteByte('(')
	nested := false
	for _, k := range n.Kids[1:] {
		if k.list && k.Head() != "xy" {
			nested = true
		}
	}
	for i, k := range n.Kids {
		if i > 0 {
			if nested && k.list {
				b.WriteByte('\n')
				b.WriteString(strings.Repeat("\t", depth+1))
			} else {
				b.WriteByte(' ')
			}
		}
		k.write(b, depth+1)
	}
	if nested {
		b.WriteByte('\n')
		b.WriteString(strings.Repeat("\t", depth))
	}
	b.WriteByte(')')
}

func fmtNum(f float64) string {
	s := strconv.FormatFloat(f, 'f', 4, 64)
	s = strings.TrimRight(s, "0")
	s = strings.TrimSuffix(s, ".")
	if s == "-0" {
		s = "0"
	}
	return s
}
