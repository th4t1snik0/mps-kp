package codecheck

import (
	"fmt"
	"math/rand"
	"strings"

	"mpskp/internal/asm51"
	"mpskp/internal/emu51"
	"mpskp/internal/variant"
)

// Input — что проверяем.
type Input struct {
	Params  *variant.Params
	Prog    int    // 1, 2, 3 (табл. 1 ТЗ)
	Src     string // текст программы
	Name    string // имя файла (для сообщений)
	VarsInc string // vars.inc варианта
	Seed    int    // зерно случайного ОЗУ и старших битов шины
	NoSim   bool   // не запускать s51
}

// Title — что должна делать программа n для варианта (табл. 1 ТЗ-2026).
func Title(p *variant.Params, n int) string {
	switch n {
	case 1:
		return "обработчик INT0: сканирование клавиатуры, код клавиши в A или 0FFh"
	case 2:
		if p.M%2 == 0 {
			return "чтение из кольцевого буфера (данные в A), флаги пустоты и переполнения"
		}
		return "запись в кольцевой буфер (данные в A), флаги пустоты и переполнения"
	case 3:
		switch p.M % 3 {
		case 0:
			return "вывод символа по коду из A на индикатор, гашение через T3 (Timer0)"
		case 1:
			return "строб Y1 длительностью T1 на " + p.Y1Pin + " (Timer0)"
		default:
			return "Y2 = (G+M+X1+X2) mod 256 при X1≠0, иначе 0; запись в регистр, строб T2 на " + p.Y2Pin + " (Timer1)"
		}
	}
	return "?"
}

// Check — полная проверка программы.
func Check(in Input) *Report {
	r := &Report{Prog: in.Prog, Title: Title(in.Params, in.Prog), RobotName: RobotName(in.Params.Student, in.Prog)}
	lintText(r, in.Src, in.Params)
	read := func(name string) (string, error) {
		if strings.EqualFold(pathBase(name), "vars.inc") {
			return in.VarsInc, nil
		}
		return "", fmt.Errorf("включать можно только vars.inc (робот принимает один файл)")
	}
	res := asm51.Assemble(in.Name, in.Src, read)
	r.Listing, r.HEX = res.Listing(), res.HEX()
	if len(res.Errors) > 0 {
		for i, e := range res.Errors {
			if i == 15 {
				r.fail("Сборка", "…и ещё %d ошибок", len(res.Errors)-i)
				break
			}
			r.fail("Сборка", "%s", e)
		}
		return r
	}
	r.pass("Сборка", "%d байт кода", len(res.Code))

	// текст для робота должен собираться в те же байты
	r.Robot = RobotText(in.Src, in.VarsInc)
	res2 := asm51.Assemble(r.RobotName, r.Robot, nil)
	if len(res2.Errors) > 0 || !sameBytes(res.Code, res2.Code) {
		r.fail("Файл для робота", "после вклейки vars.inc код отличается или не собирается: %v", res2.Errors)
	} else {
		r.pass("Файл для робота", "vars.inc вклеен, код тот же")
	}

	m := lintAsm(r, res)
	lintRules(r, res, in.Params)
	if m == nil || in.NoSim {
		return r
	}
	r.SimRun = true
	r.MaxSP = 7
	simulate(r, in, res, m)
	return r
}

func sameBytes(a, b map[int]byte) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

// harness — запущенная программа и модели устройств.
type harness struct {
	r   *Report
	p   *variant.Params
	s   mcu
	res *asm51.Result
	rng *rand.Rand

	procAt, procRet, stopAt int

	adrKB, adrY2, adrInd int
	csenBit              int // бит P3 для CS_EN
	csenBad              []string

	onXW func(addr int, v byte) // запись movx (адрес, значение)
	onXR func(addr int)         // чтение movx
	onP1 func(old, cur byte)    // запись в P1
	p1   byte
}

func simulate(r *Report, in Input, res *asm51.Result, m *marks) {
	cpu := emu51.New(int64(in.Seed + 1))
	for a, b := range res.Code {
		cpu.Code[a] = b
	}
	p := in.Params
	h := &harness{r: r, p: p, s: mcu{cpu}, res: res, rng: rand.New(rand.NewSource(int64(in.Seed))),
		procAt: m.proc.Addr, procRet: m.proc.Addr + len(m.proc.Bytes), stopAt: m.stop.Addr,
		adrKB: p.Dev("Клавиатура").Base, adrY2: p.Dev("Регистр Y2").Base, adrInd: p.Dev("Индикатор").Base, csenBit: -1}
	if pin, err := variant.ParsePin(p.CSEnPin); err == nil && pin.Port == 3 {
		h.csenBit = pin.Bit
	}
	// любое обращение movx идёт через дешифратор 74HC138 — при CS_EN = 0 ни одно устройство не выбрано
	csen := func(what string, addr uint16) {
		if h.csenBit >= 0 && cpu.Latch(3)&(1<<h.csenBit) == 0 {
			h.csenBad = append(h.csenBad, fmt.Sprintf("%s %04Xh (PC %04Xh)", what, addr, cpu.PC))
		}
	}
	cpu.OnXWrite = func(addr uint16, v byte) {
		csen("запись", addr)
		if h.onXW != nil {
			h.onXW(int(addr), v)
		}
	}
	cpu.OnXRead = func(addr uint16) (byte, bool) {
		csen("чтение", addr)
		if h.onXR != nil {
			h.onXR(int(addr))
		}
		return 0, false
	}
	cpu.OnPort = func(n int, old, cur byte) {
		if n != 1 {
			return
		}
		if h.onP1 != nil {
			h.onP1(old, cur)
		}
		h.p1 = cur
	}
	h.p1 = cpu.Latch(1)
	switch in.Prog {
	case 1:
		prog1(h)
	case 2:
		prog2(h)
	case 3:
		switch p.M % 3 {
		case 0:
			prog3Ind(h)
		case 1:
			prog3Y1(h)
		default:
			prog3Y2(h)
		}
	}
	if len(h.csenBad) > 0 {
		r.fail("CS_EN", "обращение к устройству при %s = 0 (дешифратор выключен, устройство не выбрано): %s", p.CSEnPin, strings.Join(uniq(h.csenBad), "; "))
	} else if h.csenBit >= 0 {
		r.pass("CS_EN", "все обращения к устройствам — при %s = 1", p.CSEnPin)
	}
}

func uniq(xs []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, x := range xs {
		if !seen[x] {
			seen[x] = true
			out = append(out, x)
		}
	}
	if len(out) > 5 {
		out = append(out[:5], "…")
	}
	return out
}

// runUntil — прогон не дольше maxUs мкс (1 МЦ = 1 мкс), пока done(PC) не вернёт true; проверка — после каждой команды
// (как останов s51 перед выборкой). false — вышло время.
func (h *harness) runUntil(maxUs float64, done func(pc int) bool) bool {
	c := h.s.c
	end := c.Cycles + uint64(maxUs)
	for c.Cycles < end {
		c.Step()
		if sp := int(c.SFRByte(0x81)); sp > h.r.MaxSP {
			h.r.MaxSP = sp
		}
		if done(int(c.PC)) {
			return true
		}
	}
	return false
}

// runTo — прогон до выборки команды по одному из адресов. Возвращает адрес или -1.
func (h *harness) runTo(maxUs float64, addrs ...int) int {
	hit := -1
	h.runUntil(maxUs, func(pc int) bool {
		for _, a := range addrs {
			if pc == a {
				hit = a
				return true
			}
		}
		return false
	})
	return hit
}

// wait — модельное время вперёд на us мкс (события устройств обрабатываются).
func (h *harness) wait(us float64) { h.runUntil(us, func(int) bool { return false }) }

// toProc — PC на команду call с %proc%. Переставлять PC можно только из основной программы: если остановились
// внутри обработчика прерывания, без reti МК считает, что обработчик ещё идёт, и прерывания того же уровня блокируются.
func (h *harness) toProc() {
	if pc := h.s.PC(); pc != h.stopAt && pc != h.procAt {
		h.runTo(10*ms, h.stopAt)
	}
	h.s.SetPC(h.procAt)
}

// call — вызвать процедуру ещё раз: PC на команду call с %proc% и прогон до возврата.
func (h *harness) call(maxUs float64) bool {
	h.toProc()
	return h.runTo(maxUs, h.procRet) >= 0
}

func hx(v int) string {
	if v > 0xFF {
		return fmt.Sprintf("%04Xh", v)
	}
	return fmt.Sprintf("%02Xh", v)
}
