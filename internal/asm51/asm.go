// Package asm51 — ассемблер подмножества Intel ASM51 / Keil A51 для программ курсовой
// (шаблон робота — ТЗ-2026, рис. 7). Байты сверяются с MCU 8051 IDE (= ASEM-51) в CI.
//
// Поддержано: все команды 8051; generic JMP/CALL (ссылка назад — SJMP/AJMP/ACALL, вперёд — LJMP/LCALL,
// как в ASEM-51); DSEG/CSEG/BSEG/XSEG/ISEG [AT x], ORG, EQU, SET, BIT, DATA, IDATA, XDATA, CODE,
// DS, DB, DW, USING, END; $INCLUDE (файл), прочие $-управляющие строки игнорируются.
// Имена без учёта регистра.
package asm51

import (
	"fmt"
	"sort"
	"strings"
)

// Line — строка листинга.
type Line struct {
	File     string
	No       int    // номер строки в файле
	Text     string // исходный текст
	Comment  string // комментарий (после «;»)
	Included bool   // из $INCLUDE
	Addr     int    // адрес (для строк с кодом), -1 — нет
	Bytes    []byte
	Mnemonic string // команда (в верхнем регистре), "" — нет
}

// Symbol — имя из программы.
type Symbol struct {
	Name  string
	Kind  string // NUMB, CODE, DATA, BIT, XDATA, IDATA
	Value int
	Line  int // индекс в Lines, -1 — встроенное
}

// Error — ошибка в строке.
type Error struct {
	File string
	No   int
	Msg  string
}

func (e Error) Error() string { return fmt.Sprintf("%s:%d: %s", e.File, e.No, e.Msg) }

// Result — итог сборки.
type Result struct {
	Lines   []Line
	Symbols map[string]Symbol // по имени в верхнем регистре; только пользовательские
	Code    map[int]byte
	Errors  []Error
}

// Reader читает файл $INCLUDE.
type Reader func(name string) (string, error)

type segKind int

const (
	segCode segKind = iota
	segData
	segBit
	segXdata
	segIdata
)

// Asm — состояние сборки.
type Asm struct {
	lines  []Line
	seg    segKind
	lc     [5]int
	pass   int
	syms   map[string]Symbol
	errs   []Error
	code   map[int]byte
	short  map[int]string // решение по generic JMP/CALL на 1-м проходе: индекс строки → мнемоника
	cur    int
	ended  bool
	errSet map[int]bool
	cond   []condState // вложенные IF … ENDIF
}

// condState — ветка условной сборки: active — сейчас собираем, taken — какая-то ветка уже выбрана.
type condState struct {
	active, taken, parent bool
}

// Assemble собирает src (имя файла name), включения читает read.
func Assemble(name, src string, read Reader) *Result {
	a := &Asm{syms: map[string]Symbol{}, short: map[int]string{}, errSet: map[int]bool{}}
	a.load(name, src, read, false, 0)
	for a.pass = 1; a.pass <= 2; a.pass++ {
		a.seg, a.lc, a.ended, a.cond = segCode, [5]int{}, false, nil
		a.code = map[int]byte{}
		for i := range a.lines {
			if a.ended {
				break
			}
			a.cur = i
			a.lines[i].Addr, a.lines[i].Bytes = -1, nil
			a.line(i)
		}
	}
	if !a.ended {
		a.errs = append(a.errs, Error{File: name, No: len(strings.Split(src, "\n")), Msg: "нет END"})
	}
	user := map[string]Symbol{}
	for k, s := range a.syms {
		if s.Line >= 0 {
			user[k] = s
		}
	}
	return &Result{Lines: a.lines, Symbols: user, Code: a.code, Errors: a.errs}
}

// regDefs — стандартные файлы определений SFR: у нас эти имена встроены, файл не нужен.
var regDefs = map[string]bool{"REG51.INC": true, "REG52.INC": true, "8051.MCU": true, "8052.MCU": true, "89S53.MCU": true,
	"AT89S53.INC": true, "REG_C51.INC": true, "AT89X52.INC": true}

func pathBase(p string) string {
	if i := strings.LastIndexAny(p, `/\`); i >= 0 {
		return p[i+1:]
	}
	return p
}

func (a *Asm) load(name, src string, read Reader, inc bool, depth int) {
	src = strings.TrimPrefix(src, "\xef\xbb\xbf")
	for n, text := range strings.Split(strings.ReplaceAll(src, "\r\n", "\n"), "\n") {
		code, comment := splitComment(text)
		a.lines = append(a.lines, Line{File: name, No: n + 1, Text: text, Comment: comment, Included: inc, Addr: -1})
		t := strings.TrimSpace(code)
		if !strings.HasPrefix(t, "$") {
			continue
		}
		u := strings.ToUpper(t)
		if !strings.HasPrefix(u, "$INCLUDE") {
			continue
		}
		f := strings.Trim(strings.TrimSpace(t[len("$INCLUDE"):]), "()\"' \t")
		if regDefs[strings.ToUpper(pathBase(f))] {
			continue // определения SFR 8051/8052/AT89S53 встроены (как REG51.INC / 89S53.MCU у Keil и ASEM)
		}
		if depth > 4 || read == nil {
			a.errs = append(a.errs, Error{name, n + 1, "$INCLUDE не поддержан здесь"})
			continue
		}
		s, err := read(f)
		if err != nil {
			a.errs = append(a.errs, Error{name, n + 1, "не прочитать " + f + ": " + err.Error()})
			continue
		}
		a.load(f, s, read, true, depth+1)
	}
}

// splitComment отделяет «; комментарий» вне кавычек.
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

func (a *Asm) pc() int { return a.lc[a.seg] }

func (a *Asm) errf(format string, args ...any) {
	if a.pass != 2 || a.errSet[a.cur] {
		return
	}
	a.errSet[a.cur] = true
	l := a.lines[a.cur]
	a.errs = append(a.errs, Error{l.File, l.No, fmt.Sprintf(format, args...)})
}

func (a *Asm) lookup(name string) (int, bool) {
	u := strings.ToUpper(name)
	if s, ok := a.syms[u]; ok {
		return s.Value, true
	}
	if v, ok := builtin[u]; ok {
		return v.Value, true
	}
	return 0, false
}

func (a *Asm) define(name, kind string, v int, redefinable bool) {
	u := strings.ToUpper(name)
	if len(name) > 60 {
		a.errf("имя %s длиннее 60 символов (требование ТЗ)", name)
	}
	if _, ok := builtin[u]; ok {
		a.errf("имя %s занято (SFR/бит 8051)", name)
		return
	}
	if old, ok := a.syms[u]; ok {
		if a.pass == 1 && old.Line != a.cur && !(redefinable && old.Kind == "NUMB") {
			a.errs = append(a.errs, Error{a.lines[a.cur].File, a.lines[a.cur].No, "повторное определение " + name})
			return
		}
		if a.pass == 2 && old.Line == a.cur && old.Value != v && !redefinable {
			a.errf("фазовая ошибка: %s = %04Xh на 1-м проходе, %04Xh на 2-м", name, old.Value, v)
		}
	}
	a.syms[u] = Symbol{Name: name, Kind: kind, Value: v, Line: a.cur}
}

// value вычисляет выражение; на 1-м проходе неизвестные имена → 0 без ошибки.
func (a *Asm) value(s string) int {
	v, err := a.eval(s)
	if err != nil {
		if _, ok := err.(errUndef); ok && a.pass == 1 {
			return 0
		}
		a.errf("%s", err)
	}
	return v
}

func (a *Asm) emit(b ...byte) {
	l := &a.lines[a.cur]
	if l.Addr < 0 {
		l.Addr = a.lc[segCode]
	}
	for _, x := range b {
		addr := a.lc[segCode]
		if a.pass == 2 {
			if _, dup := a.code[addr]; dup {
				a.errf("код перекрывается по адресу %04Xh", addr)
			}
			a.code[addr] = x
		}
		a.lc[segCode]++
	}
	l.Bytes = append(l.Bytes, b...)
}

var segNames = map[string]segKind{"CSEG": segCode, "DSEG": segData, "BSEG": segBit, "XSEG": segXdata, "ISEG": segIdata}
var segSym = map[segKind]string{segCode: "CODE", segData: "DATA", segBit: "BIT", segXdata: "XDATA", segIdata: "IDATA"}

func (a *Asm) line(i int) {
	code, _ := splitComment(a.lines[i].Text)
	t := strings.TrimSpace(code)
	if t == "" || strings.HasPrefix(t, "$") {
		return
	}
	// условная сборка (Keil A51): IF выражение / ELSEIF выражение / ELSE / ENDIF
	w0, restIf := word(t)
	switch strings.ToUpper(w0) {
	case "IF":
		parent := a.enabled()
		v := a.value(restIf)
		a.cond = append(a.cond, condState{active: parent && v != 0, taken: v != 0, parent: parent})
		return
	case "ELSEIF", "ELSE":
		if len(a.cond) == 0 {
			a.errf("%s без IF", strings.ToUpper(w0))
			return
		}
		c := &a.cond[len(a.cond)-1]
		v := 1
		if strings.EqualFold(w0, "ELSEIF") {
			v = a.value(restIf)
		}
		c.active = c.parent && !c.taken && v != 0
		c.taken = c.taken || v != 0
		return
	case "ENDIF":
		if len(a.cond) == 0 {
			a.errf("ENDIF без IF")
			return
		}
		a.cond = a.cond[:len(a.cond)-1]
		return
	}
	if !a.enabled() {
		return
	}
	// метка «имя:»
	if j := labelEnd(t); j > 0 {
		name := strings.TrimSpace(t[:j])
		a.define(name, segSym[a.seg], a.pc(), false)
		t = strings.TrimSpace(t[j+1:])
		if t == "" {
			if a.seg == segCode {
				a.lines[i].Addr = a.pc()
			}
			return
		}
	}
	w1, rest := word(t)
	u1 := strings.ToUpper(w1)
	// «имя EQU выражение» и т.п.
	w2, rest2 := word(rest)
	switch strings.ToUpper(w2) {
	case "EQU", "SET", "BIT", "DATA", "IDATA", "XDATA", "CODE":
		kind := map[string]string{"EQU": "NUMB", "SET": "NUMB", "BIT": "BIT", "DATA": "DATA", "IDATA": "IDATA", "XDATA": "XDATA", "CODE": "CODE"}[strings.ToUpper(w2)]
		if r := parseOp(rest2); r.k == oR && strings.EqualFold(w2, "EQU") {
			a.define(w1, "REG", r.n, false) // псевдоним регистра (Keil A51: имя EQU R7)
			return
		}
		v, err := a.eval(rest2)
		if err != nil {
			if _, ok := err.(errUndef); ok && a.pass == 1 {
				return // определится на 2-м проходе
			}
			a.errf("%s", err)
			return
		}
		if kind == "BIT" && (v < 0 || v > 0xFF) {
			a.errf("адрес бита %Xh вне 0…FFh", v)
		}
		a.define(w1, kind, v, strings.EqualFold(w2, "SET"))
		return
	case "DS", "DB", "DW":
		// метка без двоеточия перед DS/DB/DW (допускает A51)
		_, isOp := mnemonics[u1]
		_, isSeg := segNames[u1]
		if !isOp && !isSeg {
			a.define(w1, segSym[a.seg], a.pc(), false)
			u1, rest = strings.ToUpper(w2), rest2
		}
	}
	if k, ok := segNames[u1]; ok {
		a.seg = k
		if r := strings.TrimSpace(rest); r != "" {
			w, e := word(r)
			if !strings.EqualFold(w, "AT") {
				a.errf("ожидалось %s AT адрес", u1)
				return
			}
			a.lc[k] = a.value(e)
		}
		return
	}
	switch u1 {
	case "END":
		a.ended = true
		return
	case "USING", "NAME", "PUBLIC", "EXTRN", "EXTERN":
		return
	case "RSEG", "SEGMENT", "MACRO", "REPT", "IRP":
		a.errf("%s не поддержан: робот принимает один файл — пиши абсолютными сегментами CSEG AT / DSEG AT (рис. 7 ТЗ)", u1)
		return
	case "ORG":
		a.lc[a.seg] = a.value(rest)
		if a.seg == segCode {
			a.lines[i].Addr = a.pc()
		}
		return
	case "DS":
		v, err := a.eval(rest)
		if err != nil {
			a.errf("DS: %s", err)
			return
		}
		if a.seg == segCode {
			a.lines[i].Addr = a.pc()
		}
		a.lc[a.seg] += v
		return
	case "DB", "DW":
		if a.seg != segCode {
			a.errf("%s — только в CSEG", u1)
			return
		}
		a.lines[i].Addr = a.pc()
		for _, op := range splitOps(rest) {
			if u1 == "DB" && len(op) >= 3 && (op[0] == '\'' || op[0] == '"') && op[len(op)-1] == op[0] {
				for k := 1; k < len(op)-1; k++ {
					a.emit(op[k])
				}
				continue
			}
			v := a.value(op)
			if u1 == "DB" {
				a.emit(a.byteVal(v))
			} else {
				a.emit(byte(v>>8), byte(v))
			}
		}
		return
	}
	if _, ok := mnemonics[u1]; !ok {
		a.errf("неизвестная команда %s", w1)
		return
	}
	if a.seg != segCode {
		a.errf("команда вне CSEG")
		return
	}
	a.lines[i].Addr = a.pc()
	a.lines[i].Mnemonic = u1
	a.instr(u1, splitOps(rest))
}

// labelEnd — позиция «:» метки (допускается «имя :»), -1 — метки нет.
func labelEnd(t string) int {
	j := 0
	for j < len(t) && (isAlnum(t[j]) || t[j] == '_' || t[j] == '?' || t[j] >= 0x80) {
		j++
	}
	if j == 0 {
		return -1
	}
	k := j
	for k < len(t) && (t[k] == ' ' || t[k] == '\t') {
		k++
	}
	if k < len(t) && t[k] == ':' {
		return k
	}
	return -1
}

func word(s string) (string, string) {
	s = strings.TrimSpace(s)
	j := strings.IndexAny(s, " \t")
	if j < 0 {
		return s, ""
	}
	return s[:j], strings.TrimSpace(s[j:])
}

func splitOps(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	var out []string
	depth, q, start := 0, byte(0), 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case q != 0:
			if c == q {
				q = 0
			}
		case c == '\'' || c == '"':
			q = c
		case c == '(':
			depth++
		case c == ')':
			depth--
		case c == ',' && depth == 0:
			out = append(out, strings.TrimSpace(s[start:i]))
			start = i + 1
		}
	}
	return append(out, strings.TrimSpace(s[start:]))
}

// HEX — Intel HEX по 16 байт в строке.
func (r *Result) HEX() string {
	var addrs []int
	for k := range r.Code {
		addrs = append(addrs, k)
	}
	sort.Ints(addrs)
	var b strings.Builder
	for i := 0; i < len(addrs); {
		j := i
		for j+1 < len(addrs) && addrs[j+1] == addrs[j]+1 && j+1-i < 16 {
			j++
		}
		n := j - i + 1
		sum := n + addrs[i]>>8 + addrs[i]&0xFF
		fmt.Fprintf(&b, ":%02X%04X00", n, addrs[i])
		for k := i; k <= j; k++ {
			fmt.Fprintf(&b, "%02X", r.Code[addrs[k]])
			sum += int(r.Code[addrs[k]])
		}
		fmt.Fprintf(&b, "%02X\n", (-sum)&0xFF)
		i = j + 1
	}
	b.WriteString(":00000001FF\n")
	return b.String()
}

// Listing — листинг: адрес, байты, строка.
func (r *Result) Listing() string {
	var b strings.Builder
	for _, l := range r.Lines {
		addr, hex := "    ", ""
		if l.Addr >= 0 {
			addr = fmt.Sprintf("%04X", l.Addr)
		}
		for _, x := range l.Bytes {
			hex += fmt.Sprintf("%02X", x)
		}
		inc := "  "
		if l.Included {
			inc = "=1"
		}
		fmt.Fprintf(&b, "%s %-8s %s %5d  %s\n", addr, hex, inc, l.No, l.Text)
	}
	return b.String()
}

func (a *Asm) enabled() bool { return len(a.cond) == 0 || a.cond[len(a.cond)-1].active }
