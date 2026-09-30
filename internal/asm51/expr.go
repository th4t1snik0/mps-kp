package asm51

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

// Выражения ASM51: числа 12, 0Ah, 1010b, 17o/17q, 0x1F, 'c'; $ — текущий адрес;
// операторы (по возрастанию приоритета): OR XOR | AND | EQ NE LT LE GT GE = <> < <= > >= | + - | * / MOD SHL SHR | унарные - NOT HIGH LOW;
// X.n — адрес бита (байт 20h…2Fh или SFR с адресом, кратным 8).

type tok struct {
	kind byte // 'n' число, 'i' имя, 'o' оператор, '(' ')', 0 — конец
	s    string
	v    int
}

func lex(s string) ([]tok, error) {
	var out []tok
	i := 0
	for i < len(s) {
		c := s[i]
		switch {
		case c == ' ' || c == '\t':
			i++
		case c == '\'' || c == '"':
			j := strings.IndexByte(s[i+1:], c)
			if j < 0 {
				return nil, fmt.Errorf("незакрытая кавычка")
			}
			lit := s[i+1 : i+1+j]
			if len(lit) == 0 || len(lit) > 2 {
				return nil, fmt.Errorf("символьная константа %q: нужно 1–2 символа", lit)
			}
			v := 0
			for k := 0; k < len(lit); k++ {
				v = v<<8 | int(lit[k])
			}
			out = append(out, tok{kind: 'n', v: v})
			i += j + 2
		case c >= '0' && c <= '9':
			j := i
			for j < len(s) && (isAlnum(s[j])) {
				j++
			}
			v, err := parseNum(s[i:j])
			if err != nil {
				return nil, err
			}
			out = append(out, tok{kind: 'n', v: v})
			i = j
		case isIdentStart(c):
			j := i
			for j < len(s) && (isAlnum(s[j]) || s[j] == '_' || s[j] == '?' || s[j] == '@') {
				j++
			}
			w := s[i:j]
			switch strings.ToUpper(w) {
			case "OR", "XOR", "AND", "NOT", "MOD", "SHL", "SHR", "HIGH", "LOW", "EQ", "NE", "LT", "LE", "GT", "GE":
				out = append(out, tok{kind: 'o', s: strings.ToUpper(w)})
			default:
				out = append(out, tok{kind: 'i', s: w})
			}
			i = j
		case c == '$':
			out = append(out, tok{kind: 'i', s: "$"})
			i++
		case c == '(' || c == ')':
			out = append(out, tok{kind: c})
			i++
		case strings.ContainsRune("+-*/.=|&", rune(c)):
			out = append(out, tok{kind: 'o', s: string(c)})
			i++
		case c == '<' || c == '>':
			op := string(c)
			if i+1 < len(s) && (s[i+1] == '=' || (c == '<' && s[i+1] == '>')) {
				op += string(s[i+1])
			}
			out = append(out, tok{kind: 'o', s: op})
			i += len(op)
		default:
			return nil, fmt.Errorf("недопустимый символ %q в выражении", rune(c))
		}
	}
	return out, nil
}

func isIdentStart(c byte) bool { return c == '_' || c == '?' || unicode.IsLetter(rune(c)) }
func isAlnum(c byte) bool {
	return c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c == '_'
}

func parseNum(s string) (int, error) {
	u := strings.ToUpper(s)
	base := 10
	switch {
	case strings.HasPrefix(u, "0X"):
		u, base = u[2:], 16
	case strings.HasSuffix(u, "H"):
		u, base = u[:len(u)-1], 16
	case strings.HasSuffix(u, "B"):
		u, base = u[:len(u)-1], 2
	case strings.HasSuffix(u, "O"), strings.HasSuffix(u, "Q"):
		u, base = u[:len(u)-1], 8
	case strings.HasSuffix(u, "D"):
		u = u[:len(u)-1]
	}
	v, err := strconv.ParseInt(u, base, 64)
	if err != nil || v > 0xFFFF {
		return 0, fmt.Errorf("неверное число %q", s)
	}
	return int(v), nil
}

// errUndef — в выражении есть ещё не определённое имя (ссылка вперёд на 1-м проходе).
type errUndef struct{ name string }

func (e errUndef) Error() string { return "неизвестное имя " + e.name }

type parser struct {
	t   []tok
	p   int
	sym func(name string) (int, bool)
	pc  int
	und string // первое неопределённое имя
	err error
}

func (a *Asm) eval(s string) (int, error) {
	t, err := lex(s)
	if err != nil {
		return 0, err
	}
	if len(t) == 0 {
		return 0, fmt.Errorf("пустое выражение")
	}
	p := &parser{t: t, sym: a.lookup, pc: a.pc()}
	v := p.bin(0)
	if p.p < len(p.t) {
		return 0, fmt.Errorf("лишнее в выражении %q", s)
	}
	if p.und != "" {
		return 0, errUndef{p.und}
	}
	if p.err != nil {
		return 0, p.err
	}
	return v, nil
}

var levels = [][]string{{"OR", "XOR", "|"}, {"AND", "&"}, {"EQ", "NE", "LT", "LE", "GT", "GE", "=", "<>", "<", "<=", ">", ">="}, {"+", "-"}, {"*", "/", "MOD", "SHL", "SHR"}}

func (p *parser) peek() tok {
	if p.p < len(p.t) {
		return p.t[p.p]
	}
	return tok{}
}

func (p *parser) bin(l int) int {
	if l == len(levels) {
		return p.unary()
	}
	v := p.bin(l + 1)
	for {
		t := p.peek()
		if t.kind != 'o' || !contains(levels[l], t.s) {
			return v
		}
		p.p++
		r := p.bin(l + 1)
		v = apply(p, t.s, v, r)
	}
}

func contains(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}

func b2i(b bool) int {
	if b {
		return 0xFFFF
	}
	return 0
}

func apply(p *parser, op string, a, b int) int {
	switch op {
	case "OR", "|":
		return a | b
	case "XOR":
		return a ^ b
	case "AND", "&":
		return a & b
	case "EQ", "=":
		return b2i(a == b)
	case "NE", "<>":
		return b2i(a != b)
	case "LT", "<":
		return b2i(a < b)
	case "LE", "<=":
		return b2i(a <= b)
	case "GT", ">":
		return b2i(a > b)
	case "GE", ">=":
		return b2i(a >= b)
	case "+":
		return a + b
	case "-":
		return a - b
	case "*":
		return a * b
	case "/", "MOD":
		if b == 0 {
			p.fail(fmt.Errorf("деление на 0"))
			return 0
		}
		if op == "/" {
			return a / b
		}
		return a % b
	case "SHL":
		return a << uint(b&15)
	case "SHR":
		return (a & 0xFFFF) >> uint(b&15)
	}
	return 0
}

func (p *parser) unary() int {
	t := p.peek()
	if t.kind == 'o' {
		switch t.s {
		case "-":
			p.p++
			return -p.unary()
		case "+":
			p.p++
			return p.unary()
		case "NOT":
			p.p++
			return ^p.unary() & 0xFFFF
		case "HIGH":
			p.p++
			return (p.unary() >> 8) & 0xFF
		case "LOW":
			p.p++
			return p.unary() & 0xFF
		}
	}
	v := p.primary()
	// X.n — адрес бита
	if t := p.peek(); t.kind == 'o' && t.s == "." {
		p.p++
		n := p.primary()
		if n < 0 || n > 7 {
			p.fail(fmt.Errorf("номер бита %d вне 0…7", n))
			return 0
		}
		switch {
		case v >= 0x20 && v <= 0x2F:
			return (v-0x20)*8 + n
		case v >= 0x80 && v <= 0xFF && v%8 == 0:
			return v + n
		case p.und != "":
			return 0
		default:
			p.fail(fmt.Errorf("байт %02Xh не бит-адресуемый", v))
			return 0
		}
	}
	return v
}

func (p *parser) fail(err error) {
	if p.err == nil {
		p.err = err
	}
}

func (p *parser) primary() int {
	t := p.peek()
	switch t.kind {
	case 'n':
		p.p++
		return t.v
	case 'i':
		p.p++
		if t.s == "$" {
			return p.pc
		}
		v, ok := p.sym(t.s)
		if !ok && p.und == "" {
			p.und = t.s
		}
		return v
	case '(':
		p.p++
		v := p.bin(0)
		if p.peek().kind != ')' {
			p.fail(fmt.Errorf("нет «)»"))
			return v
		}
		p.p++
		return v
	}
	p.fail(fmt.Errorf("ожидалось число или имя"))
	p.p = len(p.t)
	return 0
}
