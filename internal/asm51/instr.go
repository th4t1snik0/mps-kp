package asm51

import (
	"strconv"
	"strings"
)

// Виды операндов.
type opKind int

const (
	oA      opKind = iota // A
	oC                    // C (перенос)
	oAB                   // AB
	oDPTR                 // DPTR
	oAtDPTR               // @DPTR
	oAtAPC                // @A+PC
	oAtADP                // @A+DPTR
	oR                    // R0…R7
	oAtR                  // @R0, @R1
	oImm                  // #выражение
	oDir                  // выражение: прямой адрес, бит, адрес перехода
	oNBit                 // /бит
)

type operand struct {
	k opKind
	n int    // номер регистра
	e string // выражение
}

func parseOp(s string) operand {
	u := strings.ToUpper(strings.ReplaceAll(s, " ", ""))
	switch u {
	case "A":
		return operand{k: oA}
	case "C":
		return operand{k: oC}
	case "AB":
		return operand{k: oAB}
	case "DPTR":
		return operand{k: oDPTR}
	case "@DPTR":
		return operand{k: oAtDPTR}
	case "@A+PC":
		return operand{k: oAtAPC}
	case "@A+DPTR":
		return operand{k: oAtADP}
	case "@R0", "@R1":
		return operand{k: oAtR, n: int(u[2] - '0')}
	}
	if len(u) == 2 && u[0] == 'R' && u[1] >= '0' && u[1] <= '7' {
		return operand{k: oR, n: int(u[1] - '0')}
	}
	if strings.HasPrefix(s, "#") {
		return operand{k: oImm, e: strings.TrimSpace(s[1:])}
	}
	if strings.HasPrefix(s, "/") {
		return operand{k: oNBit, e: strings.TrimSpace(s[1:])}
	}
	return operand{k: oDir, e: s}
}

var mnemonics = map[string]bool{}

func init() {
	for _, m := range strings.Fields("ACALL ADD ADDC AJMP ANL CALL CJNE CLR CPL DA DEC DIV DJNZ INC JB JBC JC JMP JNB JNC JNZ JZ LCALL LJMP MOV MOVC MOVX MUL NOP ORL POP PUSH RET RETI RL RLC RR RRC SETB SJMP SUBB SWAP XCH XCHD XRL") {
		mnemonics[m] = true
	}
}

func (a *Asm) byteVal(v int) byte {
	if v < -256 || v > 255 {
		a.errf("значение %d не помещается в байт", v)
	}
	return byte(v)
}

func (a *Asm) dir(o operand) byte {
	v := a.value(o.e)
	if v < 0 || v > 255 {
		a.errf("адрес %Xh вне 0…FFh", v)
	}
	return byte(v)
}

// rel — смещение до цели от адреса следующей команды (size — длина команды).
func (a *Asm) rel(e string, size int) byte {
	v, err := a.eval(e)
	if err != nil {
		if _, ok := err.(errUndef); !ok || a.pass == 2 {
			a.errf("%s", err)
		}
		return 0
	}
	d := v - (a.pc() + size)
	if a.pass == 2 && (d < -128 || d > 127) {
		a.errf("переход на %04Xh слишком далеко (%d байт), нужен LJMP", v, d)
	}
	return byte(d)
}

func (a *Asm) addr16(e string) (byte, byte) {
	v := a.value(e)
	return byte(v >> 8), byte(v)
}

func (a *Asm) addr11(e string, op byte) {
	v := a.value(e)
	if a.pass == 2 && (v&0xF800) != ((a.pc()+2)&0xF800) {
		a.errf("цель %04Xh вне текущего блока 2 КБ", v)
	}
	a.emit(op|byte((v>>8)&7)<<5, byte(v))
}

// ariths — A,Rn / A,dir / A,@Ri / A,#imm с базовым кодом.
var ariths = map[string]byte{"ADD": 0x20, "ADDC": 0x30, "SUBB": 0x90, "ANL": 0x50, "ORL": 0x40, "XRL": 0x60}

func (a *Asm) instr(m string, ops []string) {
	o := make([]operand, len(ops))
	for i, s := range ops {
		o[i] = parseOp(s)
		if sym, ok := a.syms[strings.ToUpper(strings.TrimSpace(s))]; ok && sym.Kind == "REG" {
			o[i] = operand{k: oR, n: sym.Value}
		}
	}
	want := func(n int) bool {
		if len(o) != n {
			a.errf("%s: нужно операндов %d, а не %d", m, n, len(o))
			// чтобы длина не поехала между проходами — заполняем «пусто»
			for len(o) < n {
				o = append(o, operand{k: oDir, e: "0"})
			}
			return false
		}
		return true
	}
	bad := func() {
		a.errf("%s %s: недопустимая комбинация операндов", m, strings.Join(ops, ","))
	}
	is := func(ks ...opKind) bool {
		if len(o) != len(ks) {
			return false
		}
		for i, k := range ks {
			if o[i].k != k {
				return false
			}
		}
		return true
	}

	switch m {
	case "NOP":
		want(0)
		a.emit(0x00)
	case "RET":
		want(0)
		a.emit(0x22)
	case "RETI":
		want(0)
		a.emit(0x32)
	case "ADD", "ADDC", "SUBB", "ANL", "ORL", "XRL":
		want(2)
		base := ariths[m]
		switch {
		case is(oA, oR):
			a.emit(base | 8 | byte(o[1].n))
		case is(oA, oDir):
			a.emit(base|5, a.dir(o[1]))
		case is(oA, oAtR):
			a.emit(base | 6 | byte(o[1].n))
		case is(oA, oImm):
			a.emit(base|4, a.byteVal(a.value(o[1].e)))
		case m != "ADD" && m != "ADDC" && m != "SUBB" && is(oDir, oA):
			a.emit(base|2, a.dir(o[0]))
		case m != "ADD" && m != "ADDC" && m != "SUBB" && is(oDir, oImm):
			a.emit(base|3, a.dir(o[0]), a.byteVal(a.value(o[1].e)))
		case m == "ANL" && is(oC, oDir):
			a.emit(0x82, a.dir(o[1]))
		case m == "ANL" && is(oC, oNBit):
			a.emit(0xB0, a.dir(o[1]))
		case m == "ORL" && is(oC, oDir):
			a.emit(0x72, a.dir(o[1]))
		case m == "ORL" && is(oC, oNBit):
			a.emit(0xA0, a.dir(o[1]))
		default:
			bad()
		}
	case "INC", "DEC":
		want(1)
		base := byte(0x00)
		if m == "DEC" {
			base = 0x10
		}
		switch {
		case is(oA):
			a.emit(base | 4)
		case is(oR):
			a.emit(base | 8 | byte(o[0].n))
		case is(oDir):
			a.emit(base|5, a.dir(o[0]))
		case is(oAtR):
			a.emit(base | 6 | byte(o[0].n))
		case m == "INC" && is(oDPTR):
			a.emit(0xA3)
		default:
			bad()
		}
	case "MUL", "DIV":
		want(1)
		if !is(oAB) {
			bad()
		}
		a.emit(map[string]byte{"MUL": 0xA4, "DIV": 0x84}[m])
	case "DA", "RL", "RLC", "RR", "RRC", "SWAP":
		want(1)
		if !is(oA) {
			bad()
		}
		a.emit(map[string]byte{"DA": 0xD4, "RL": 0x23, "RLC": 0x33, "RR": 0x03, "RRC": 0x13, "SWAP": 0xC4}[m])
	case "CLR", "CPL", "SETB":
		want(1)
		codes := map[string][3]byte{"CLR": {0xE4, 0xC3, 0xC2}, "CPL": {0xF4, 0xB3, 0xB2}, "SETB": {0, 0xD3, 0xD2}}[m]
		switch {
		case is(oA) && m != "SETB":
			a.emit(codes[0])
		case is(oC):
			a.emit(codes[1])
		case is(oDir):
			a.emit(codes[2], a.dir(o[0]))
		default:
			bad()
		}
	case "MOV":
		want(2)
		switch {
		case is(oA, oR):
			a.emit(0xE8 | byte(o[1].n))
		case is(oA, oDir):
			a.emit(0xE5, a.dir(o[1]))
		case is(oA, oAtR):
			a.emit(0xE6 | byte(o[1].n))
		case is(oA, oImm):
			a.emit(0x74, a.byteVal(a.value(o[1].e)))
		case is(oR, oA):
			a.emit(0xF8 | byte(o[0].n))
		case is(oR, oDir):
			a.emit(0xA8|byte(o[0].n), a.dir(o[1]))
		case is(oR, oImm):
			a.emit(0x78|byte(o[0].n), a.byteVal(a.value(o[1].e)))
		case is(oDir, oA):
			a.emit(0xF5, a.dir(o[0]))
		case is(oDir, oR):
			a.emit(0x88|byte(o[1].n), a.dir(o[0]))
		case is(oDir, oDir):
			a.emit(0x85, a.dir(o[1]), a.dir(o[0]))
		case is(oDir, oAtR):
			a.emit(0x86|byte(o[1].n), a.dir(o[0]))
		case is(oDir, oImm):
			a.emit(0x75, a.dir(o[0]), a.byteVal(a.value(o[1].e)))
		case is(oAtR, oA):
			a.emit(0xF6 | byte(o[0].n))
		case is(oAtR, oDir):
			a.emit(0xA6|byte(o[0].n), a.dir(o[1]))
		case is(oAtR, oImm):
			a.emit(0x76|byte(o[0].n), a.byteVal(a.value(o[1].e)))
		case is(oC, oDir):
			a.emit(0xA2, a.dir(o[1]))
		case is(oDir, oC):
			a.emit(0x92, a.dir(o[0]))
		case is(oDPTR, oImm):
			h, l := a.addr16(o[1].e)
			a.emit(0x90, h, l)
		default:
			bad()
		}
	case "MOVC":
		want(2)
		switch {
		case is(oA, oAtADP):
			a.emit(0x93)
		case is(oA, oAtAPC):
			a.emit(0x83)
		default:
			bad()
		}
	case "MOVX":
		want(2)
		switch {
		case is(oA, oAtDPTR):
			a.emit(0xE0)
		case is(oA, oAtR):
			a.emit(0xE2 | byte(o[1].n))
		case is(oAtDPTR, oA):
			a.emit(0xF0)
		case is(oAtR, oA):
			a.emit(0xF2 | byte(o[0].n))
		default:
			bad()
		}
	case "PUSH", "POP":
		want(1)
		if !is(oDir) {
			bad()
		}
		a.emit(map[string]byte{"PUSH": 0xC0, "POP": 0xD0}[m], a.dir(o[0]))
	case "XCH":
		want(2)
		switch {
		case is(oA, oR):
			a.emit(0xC8 | byte(o[1].n))
		case is(oA, oDir):
			a.emit(0xC5, a.dir(o[1]))
		case is(oA, oAtR):
			a.emit(0xC6 | byte(o[1].n))
		default:
			bad()
		}
	case "XCHD":
		want(2)
		if !is(oA, oAtR) {
			bad()
		}
		a.emit(0xD6 | byte(o[1].n))
	case "JC", "JNC", "JZ", "JNZ", "SJMP":
		want(1)
		op := map[string]byte{"JC": 0x40, "JNC": 0x50, "JZ": 0x60, "JNZ": 0x70, "SJMP": 0x80}[m]
		a.emit(op, a.rel(o[0].e, 2))
	case "JB", "JNB", "JBC":
		want(2)
		op := map[string]byte{"JB": 0x20, "JNB": 0x30, "JBC": 0x10}[m]
		b := a.dir(o[0])
		a.emit(op, b, a.rel(o[1].e, 3))
	case "DJNZ":
		want(2)
		switch {
		case is(oR, oDir):
			a.emit(0xD8|byte(o[0].n), a.rel(o[1].e, 2))
		case is(oDir, oDir):
			d := a.dir(o[0])
			a.emit(0xD5, d, a.rel(o[1].e, 3))
		default:
			bad()
		}
	case "CJNE":
		want(3)
		var op, x byte
		switch {
		case is(oA, oDir, oDir):
			op, x = 0xB5, a.dir(o[1])
		case is(oA, oImm, oDir):
			op, x = 0xB4, a.byteVal(a.value(o[1].e))
		case is(oR, oImm, oDir):
			op, x = 0xB8|byte(o[0].n), a.byteVal(a.value(o[1].e))
		case is(oAtR, oImm, oDir):
			op, x = 0xB6|byte(o[0].n), a.byteVal(a.value(o[1].e))
		default:
			bad()
			a.emit(0, 0, 0)
			return
		}
		a.emit(op, x, a.rel(o[2].e, 3))
	case "AJMP":
		want(1)
		a.addr11(o[0].e, 0x01)
	case "ACALL":
		want(1)
		a.addr11(o[0].e, 0x11)
	case "LJMP", "LCALL":
		want(1)
		h, l := a.addr16(o[0].e)
		a.emit(map[string]byte{"LJMP": 0x02, "LCALL": 0x12}[m], h, l)
	case "JMP", "CALL":
		want(1)
		if m == "JMP" && is(oAtADP) {
			a.emit(0x73)
			return
		}
		a.generic(m, o[0].e)
	}
}

// generic — JMP/CALL как в ASEM-51: цель известна на 1-м проходе (ссылка назад) — короткая форма,
// иначе LJMP/LCALL. Решение фиксируется на 1-м проходе, чтобы длины совпали.
func (a *Asm) generic(m, e string) {
	if a.pass == 1 {
		choice := "L" + m
		if v, err := a.eval(e); err == nil {
			next := a.pc() + 2
			switch {
			case m == "JMP" && v-next >= -128 && v-next <= 127:
				choice = "SJMP"
			case (v & 0xF800) == (next & 0xF800):
				choice = "A" + m
			}
		}
		a.short[a.cur] = choice
	}
	switch c := a.short[a.cur]; c {
	case "SJMP":
		a.emit(0x80, a.rel(e, 2))
	case "AJMP":
		a.addr11(e, 0x01)
	case "ACALL":
		a.addr11(e, 0x11)
	default:
		h, l := a.addr16(e)
		a.emit(map[string]byte{"LJMP": 0x02, "LCALL": 0x12}[c], h, l)
	}
}

// Встроенные имена 8051/8052 (как в MCU 8051 IDE).
var builtin = map[string]Symbol{}

func init() {
	add := func(kind, list string) {
		for _, p := range strings.Fields(list) {
			kv := strings.SplitN(p, "=", 2)
			v, _ := strconv.ParseInt(kv[1], 16, 32)
			builtin[kv[0]] = Symbol{Name: kv[0], Kind: kind, Value: int(v), Line: -1}
		}
	}
	add("DATA", "P0=80 SP=81 DPL=82 DPH=83 PCON=87 TCON=88 TMOD=89 TL0=8A TL1=8B TH0=8C TH1=8D P1=90 SCON=98 SBUF=99 P2=A0 IE=A8 P3=B0 IP=B8 "+
		"T2CON=C8 T2MOD=C9 RCAP2L=CA RCAP2H=CB TL2=CC TH2=CD PSW=D0 ACC=E0 B=F0 WDTRST=A6 WDTCON=A7")
	add("BIT", "IT0=88 IE0=89 IT1=8A IE1=8B TR0=8C TF0=8D TR1=8E TF1=8F RI=98 TI=99 RB8=9A TB8=9B REN=9C SM2=9D SM1=9E SM0=9F "+
		"EX0=A8 ET0=A9 EX1=AA ET1=AB ES=AC ET2=AD EA=AF RXD=B0 TXD=B1 INT0=B2 INT1=B3 T0=B4 T1=B5 WR=B6 RD=B7 "+
		"PX0=B8 PT0=B9 PX1=BA PT1=BB PS=BC PT2=BD P=D0 F1=D1 OV=D2 RS0=D3 RS1=D4 F0=D5 AC=D6 CY=D7 "+
		"CP_RL2=C8 C_T2=C9 TR2=CA EXEN2=CB TCLK=CC RCLK=CD EXF2=CE TF2=CF")
	add("CODE", "RESET=0 EXTI0=3 TIMER0=B EXTI1=13 TIMER1=1B SINT=23 TIMER2=2B")
}
