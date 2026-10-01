package codecheck

import (
	"mpskp/internal/asm51"
	"mpskp/internal/variant"
)

// Правила ТЗ про стек (имена ≤ 60 символов проверяет сам ассемблер) (выжимка — docs/research/code-pz2-requirements.md, разд. 2).

// initSP — значение из первой «mov SP, #x» (байты 75 81 x) в коде студента; -1 — такой команды нет.
func initSP(res *asm51.Result) int {
	for _, l := range res.Lines {
		if !l.Included && len(l.Bytes) == 3 && l.Bytes[0] == 0x75 && l.Bytes[1] == 0x81 {
			return int(l.Bytes[2])
		}
	}
	return -1
}

// lintStack — SP = 07h в инициализации (ТЗ, п. 2.2: «в т.ч. стека МК: SP=07h»);
// банки 1–3 при стеке с 07h — стек и банк делят одни ячейки.
func lintStack(r *Report, res *asm51.Result) {
	sp := initSP(res)
	if sp < 0 {
		r.warn("Указатель стека", "нет «mov SP, #07h» — ТЗ просит явно инициализировать стек (SP = 07h) вместе с остальными узлами")
		sp = 7
	}
	bank := 0 // старший банк, который включает программа
	for _, l := range res.Lines {
		b := l.Bytes
		if l.Included || len(b) == 0 {
			continue
		}
		switch {
		case len(b) == 2 && b[0] == 0xD2 && b[1] == 0xD3: // setb RS0
			bank = max(bank, 1)
		case len(b) == 2 && b[0] == 0xD2 && b[1] == 0xD4: // setb RS1
			bank = max(bank, 2)
		case len(b) == 3 && (b[0] == 0x75 || b[0] == 0x43) && b[1] == 0xD0: // mov/orl PSW, #x
			bank = max(bank, int(b[2]>>3&3))
		}
	}
	if bank > 0 && sp < bank*8+7 {
		r.warn("Банки регистров", "программа включает банк %d (%02Xh–%02Xh), а стек начинается с %02Xh — стек затрёт регистры банка; "+
			"поставь SP выше последнего используемого банка или работай в банке 0", bank, bank*8, bank*8+7, sp+1)
	}
}

// checkStack — после прогона: стек (от начального SP до наибольшего за все сценарии) не должен заходить на переменные студента,
// «голову», «хвост» и байты флагов из табл. 1 ТЗ.
func checkStack(r *Report, res *asm51.Result, p *variant.Params) {
	sp := initSP(res)
	if sp < 0 {
		sp = 7
	}
	reserved := map[int]string{p.Head: "«голова»", p.Head + 1: "«голова»", p.Tail: "«хвост»", p.Tail + 1: "«хвост»",
		p.Empty.Byte: "байт флага пустоты", p.Ovf.Byte: "байт флага переполнения"}
	for _, s := range res.Symbols {
		if s.Kind == "DATA" && s.Line >= 0 && !res.Lines[s.Line].Included && s.Value >= 8 && s.Value < 0x80 {
			if _, ok := reserved[s.Value]; !ok {
				reserved[s.Value] = "переменная " + s.Name
			}
		}
	}
	for a := sp + 1; a <= r.MaxSP; a++ {
		if what, ok := reserved[a]; ok {
			r.fail("Стек", "стек дошёл до %02Xh (SP от %02Xh до %02Xh) — там %s; подними начало стека или убери переменные из этой области",
				a, sp, r.MaxSP, what)
			return
		}
	}
}
