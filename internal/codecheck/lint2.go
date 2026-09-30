package codecheck

import (
	"fmt"
	"sort"
	"strings"

	"mpskp/internal/asm51"
	"mpskp/internal/variant"
)

// Правила по лекциям, методичке и ТЗ, которые проверяются по тексту программы (выжимка — docs/research/code-pz2-requirements.md).

type instr struct {
	addr int
	mn   string
	ops  []string
	line *asm51.Line
}

func instrs(res *asm51.Result) map[int]instr {
	m := map[int]instr{}
	for i := range res.Lines {
		l := &res.Lines[i]
		if l.Addr < 0 || l.Mnemonic == "" {
			continue
		}
		code, _ := splitComment(l.Text)
		code = strings.TrimSpace(code)
		if j := strings.IndexByte(code, ':'); j >= 0 && !strings.ContainsAny(code[:j], " \t,") {
			code = strings.TrimSpace(code[j+1:])
		}
		_, rest, _ := strings.Cut(code, " ")
		var ops []string
		for _, o := range strings.Split(rest, ",") {
			if o = strings.TrimSpace(o); o != "" {
				ops = append(ops, strings.ToUpper(o))
			}
		}
		m[l.Addr] = instr{l.Addr, l.Mnemonic, ops, l}
	}
	return m
}

var vecNames = map[int]string{0x03: "INT0", 0x0B: "Timer0", 0x13: "INT1", 0x1B: "Timer1", 0x23: "UART"}

// isrWalk — обход обработчика от адреса: по порядку, через безусловные переходы, до RET/RETI (не дальше 400 команд).
func isrWalk(code map[int]instr, start int) (seen []instr, end string) {
	addr := start
	for i := 0; i < 400; i++ {
		in, ok := code[addr]
		if !ok {
			return seen, ""
		}
		seen = append(seen, in)
		switch in.mn {
		case "RET", "RETI":
			return seen, in.mn
		case "LJMP", "AJMP", "SJMP", "JMP":
			b := in.line.Bytes
			switch {
			case b[0] == 0x02:
				addr = int(b[1])<<8 | int(b[2])
			case b[0] == 0x80:
				addr = in.addr + 2 + int(int8(b[1]))
			case b[0]&0x1F == 0x01:
				addr = (in.addr+2)&0xF800 | int(b[0]>>5)<<8 | int(b[1])
			default:
				return seen, ""
			}
			if addr == in.addr {
				return seen, "" // jmp $
			}
			continue
		}
		addr = in.addr + len(in.line.Bytes)
	}
	return seen, ""
}

var writesA = map[string]bool{"ADD": true, "ADDC": true, "SUBB": true, "ANL": true, "ORL": true, "XRL": true, "INC": true, "DEC": true,
	"CLR": true, "CPL": true, "RL": true, "RLC": true, "RR": true, "RRC": true, "SWAP": true, "DA": true, "MOV": true, "MOVX": true, "MOVC": true, "XCH": true, "XCHD": true, "MUL": true, "DIV": true, "POP": true}
var writesPSW = map[string]bool{"ADD": true, "ADDC": true, "SUBB": true, "CJNE": true, "DA": true, "MUL": true, "DIV": true, "RLC": true, "RRC": true}

func lintRules(r *Report, res *asm51.Result, p *variant.Params) {
	code := instrs(res)
	var bad, stubWarn, saveWarn []string
	for _, v := range []int{0x03, 0x0B, 0x13, 0x1B, 0x23} {
		first, ok := code[v]
		if !ok {
			continue
		}
		if first.mn == "NOP" {
			if next, ok := code[v+1]; !ok || next.mn != "RETI" {
				stubWarn = append(stubWarn, fmt.Sprintf("%s (%02Xh)", vecNames[v], v))
			}
			continue
		}
		seen, end := isrWalk(code, v)
		if end == "RET" {
			bad = append(bad, fmt.Sprintf("обработчик %s (%02Xh) выходит по RET, а не RETI — после него прерывания этого уровня больше не придут", vecNames[v], v))
		}
		// сохранение A и PSW — для обработчиков таймеров (прогр. 1 возвращает код в A по ТЗ — там не требуем)
		if v != 0x0B && v != 0x1B {
			continue
		}
		pushA, pushPSW, useA, usePSW := false, false, false, false
		for _, in := range seen {
			switch {
			case in.mn == "PUSH" && len(in.ops) == 1 && (in.ops[0] == "ACC" || in.ops[0] == "0E0H"):
				pushA = true
			case in.mn == "PUSH" && len(in.ops) == 1 && (in.ops[0] == "PSW" || in.ops[0] == "0D0H"):
				pushPSW = true
			case in.mn == "CALL" || in.mn == "LCALL" || in.mn == "ACALL":
				useA, usePSW = true, true // вызванная процедура — считаем, что портит
			}
			if len(in.ops) > 0 && in.ops[0] == "A" && writesA[in.mn] {
				useA = true
			}
			if writesPSW[in.mn] {
				usePSW = true
			}
		}
		var miss []string
		if useA && !pushA {
			miss = append(miss, "A")
		}
		if usePSW && !pushPSW {
			miss = append(miss, "PSW")
		}
		if len(miss) > 0 {
			saveWarn = append(saveWarn, fmt.Sprintf("%s: меняет %s без push/pop", vecNames[v], strings.Join(miss, " и ")))
		}
	}
	for _, b := range bad {
		r.fail("RETI", "%s", b)
	}
	if len(stubWarn) > 0 {
		r.warn("Заглушки IRQ", "на векторах %s — nop, но за ним не reti (заглушка по ТЗ — ровно «nop / reti»)", strings.Join(stubWarn, ", "))
	}
	if len(saveWarn) > 0 {
		r.warn("Контекст в прерываниях", "%s — обработчик может прийти в любой момент основной программы, сохраняй регистры, которые меняешь (лекции, ч. 2)", strings.Join(saveWarn, "; "))
	}

	// описание у каждой процедуры (ТЗ п. 7), а не только у той, что с %proc%
	var noHdr []string
	targets := map[int]bool{}
	for _, in := range code {
		b := in.line.Bytes
		switch {
		case in.mn == "LCALL" || (in.mn == "CALL" && b[0] == 0x12):
			targets[int(b[1])<<8|int(b[2])] = true
		case in.mn == "ACALL" || (in.mn == "CALL" && b[0]&0x1F == 0x11):
			targets[(in.addr+2)&0xF800|int(b[0]>>5)<<8|int(b[1])] = true
		}
	}
	for _, s := range res.Symbols {
		if s.Kind != "CODE" || s.Line < 0 || !targets[s.Value] || res.Lines[s.Line].Included {
			continue
		}
		n := 0
		for j := s.Line - 1; j >= 0 && n < 10; j-- {
			if !strings.HasPrefix(strings.TrimSpace(res.Lines[j].Text), ";") {
				break
			}
			n++
		}
		if n == 0 {
			noHdr = append(noHdr, s.Name)
		}
	}
	sort.Strings(noHdr)
	if len(noHdr) > 0 {
		r.warn("Описание процедур", "нет комментария-заголовка перед %s — у каждой процедуры: назначение, вход, выход (ТЗ п. 7)", strings.Join(noHdr, ", "))
	}

	// переменные DSEG не должны занимать адреса указателей и байты флагов из табл. 1
	reserved := map[int]string{p.Head: "«голова»", p.Head + 1: "«голова»", p.Tail: "«хвост»", p.Tail + 1: "«хвост»",
		p.Empty.Byte: "байт флага пустоты", p.Ovf.Byte: "байт флага переполнения"}
	var clash []string
	for _, s := range res.Symbols {
		if s.Kind != "DATA" || s.Line < 0 || res.Lines[s.Line].Included {
			continue
		}
		if what, ok := reserved[s.Value]; ok {
			clash = append(clash, fmt.Sprintf("%s = %s занимает %s", s.Name, hx(s.Value), what))
		}
	}
	sort.Strings(clash)
	if len(clash) > 0 {
		r.fail("Память", "%s (адреса по табл. 1 ТЗ — в vars.inc)", strings.Join(clash, "; "))
	}
}
