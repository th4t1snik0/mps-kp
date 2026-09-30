package codecheck

import (
	"fmt"
	"math"
	"strings"
)

const ms = 1000.0 // мкс

// ---------------------------------------------------------------- программа 1: клавиатура

// Раскладка по рис. 4 ТЗ, [строка][столбец]; коды — табл. 3 (Т = 10, С = 11).
func keyCodes(cols int) [][]int {
	if cols == 4 {
		return [][]int{{1, 2, 3, 10}, {4, 5, 6, 11}, {7, 8, 9, 0}}
	}
	return [][]int{{1, 2, 3}, {4, 5, 6}, {7, 8, 9}, {0, 10, 11}}
}

var keyName = []string{"0", "1", "2", "3", "4", "5", "6", "7", "8", "9", "Т", "С"}

// kbModel — клавиатура схемы: столбцы — 74HC173 (D0…D3, сброс по RST → 0), строки — подтяжки, 74HC244 на D0…D3
// (у 3 строк вход D3 на земле), D4…D7 при чтении не подключены; ~INT0 = И всех строк.
type kbModel struct {
	h       *harness
	cols    byte
	pressed [][2]int // (строка, столбец)
	forceLo bool     // INT0 прижат снаружи (ложное срабатывание)
}

func (k *kbModel) update() {
	p := k.h.p
	rows := byte(0)
	for r := 0; r < p.Rows; r++ {
		lo := false
		for _, rc := range k.pressed {
			if rc[0] == r && k.cols&(1<<rc[1]) == 0 {
				lo = true
			}
		}
		if !lo {
			rows |= 1 << r
		}
	}
	v := byte(k.h.rng.Intn(256))&0xF0 | rows // старшие — «висят»
	k.h.s.SetXRAM(k.h.adrKB, v)
	pins := byte(0xFF)
	if rows != byte(1<<p.Rows)-1 || k.forceLo {
		pins &^= 1 << 2 // P3.2 = ~INT0
	}
	k.h.s.SetPins(3, pins)
}

func prog1(h *harness) {
	p := h.p
	k := &kbModel{h: h}
	h.s.BreakMem("xram", 'w', h.adrKB)
	h.s.BreakMem("xram", 'r', h.adrKB)
	h.onXW = func(addr int, v byte) {
		if addr == h.adrKB {
			k.cols = v & 0x0F
			k.update()
		}
	}
	k.update()
	if h.runTo(100*ms, h.stopAt) < 0 {
		h.r.fail("Инициализация", "за 100 мс не дошли до %%stop%%")
		return
	}
	h.r.pass("Инициализация", "дошли до %%stop%%")
	// %proc% в основной программе (как у принятой Осиповой 2025), а не в векторе INT0: по ТЗ процедура — обработчик IRQ;
	// проверяем прямым вызовом с нажатой клавишей и предупреждаем
	direct := h.procAt >= 0x2B
	if direct {
		h.r.warn("Вызов", "call с %%proc%% стоит в основной программе, а по ТЗ (табл. 1) программа 1 — процедура обработчика IRQ клавиатуры: "+
			"вызывай её из вектора INT0 (03h). Сценарии ниже — прямым вызовом при нажатой клавише")
	}
	ie := h.s.SFR(0xA8)
	if !direct && ie&0x81 != 0x81 {
		h.r.fail("INT0", "после инициализации прерывание INT0 не разрешено (IE = %02Xh, нужны EA и EX0)", ie)
		return
	}
	used := byte(1<<p.Cols) - 1
	if !direct && k.cols&used != 0 {
		h.r.fail("Столбцы", "после инициализации столбцы = %04bb — должны быть все 0, иначе нажатие не опустит строку и INT0 не сработает", k.cols&used)
		return
	}
	codes := keyCodes(p.Cols)
	type sc struct {
		name string
		keys [][2]int
		want int
		lo   bool
	}
	var cases []sc
	for r := 0; r < p.Rows; r++ {
		for c := 0; c < p.Cols; c++ {
			cases = append(cases, sc{name: "«" + keyName[codes[r][c]] + "»", keys: [][2]int{{r, c}}, want: codes[r][c]})
		}
	}
	cases = append(cases,
		sc{"две клавиши в строке", [][2]int{{0, 0}, {0, 1}}, 0xFF, false},
		sc{"две клавиши в столбце", [][2]int{{0, 0}, {1, 0}}, 0xFF, false},
		sc{"две клавиши вразброс", [][2]int{{0, 1}, {p.Rows - 1, p.Cols - 1}}, 0xFF, false},
		sc{"INT0 без нажатия (помеха)", nil, 0xFF, true},
	)
	var bad []string
	restored := true
	holdCalls := 0
	worst := 0.0
	for _, c := range cases {
		if direct && c.lo {
			continue // помеха на INT0 без нажатия — только для обработчика прерывания
		}
		k.pressed, k.forceLo = c.keys, c.lo
		k.update()
		if direct {
			h.toProc()
		}
		t0 := h.s.Micros()
		if h.runTo(50*ms, h.procRet) < 0 {
			bad = append(bad, c.name+": процедура не вызвана за 50 мс (call %proc% должен быть в обработчике INT0)")
			k.pressed, k.forceLo = nil, false
			k.update()
			if h.runTo(50*ms, h.stopAt) < 0 {
				break
			}
			continue
		}
		worst = math.Max(worst, h.s.Micros()-t0)
		// удержание: одно нажатие — один вызов (по спаду и со сбросом IE0), а не очередь вызовов, пока держат
		if c.name == "«1»" && len(c.keys) == 1 && !direct {
			extra := 0
			h.runUntil(30*ms, func(pc int) bool {
				if pc == h.procRet {
					extra++
				}
				return false
			})
			if extra > 0 {
				holdCalls = extra
			}
		}
		if a := int(h.s.A()); a != c.want {
			bad = append(bad, fmt.Sprintf("%s: A = %s, ждали %s", c.name, hx(a), hx(c.want)))
		}
		k.pressed, k.forceLo = nil, false
		k.update()
		if h.runTo(50*ms, h.stopAt) < 0 {
			bad = append(bad, c.name+": после обработки не вернулись на %stop% за 50 мс")
			break
		}
		if k.cols&used != 0 {
			restored = false
		}
	}
	if len(bad) > 0 {
		h.r.fail("Коды клавиш", "%s", joinCap(bad))
	} else {
		h.r.pass("Коды клавиш", "все %d клавиш, две клавиши и помеха — верно; от нажатия до возврата ≤ %.0f мкс", p.Rows*p.Cols, worst)
	}
	if holdCalls > 0 {
		h.r.warn("Удержание клавиши", "пока клавишу держат, обработчик вызвался ещё %d раз за 30 мс — нужен запуск INT0 по спаду (IT0 = 1) и сброс IE0 в конце опроса", holdCalls)
	}
	if worst > 2000 {
		h.r.warn("Время опроса", "от нажатия до результата %.0f мкс — дольше 2 мс; программный антидребезг методичка не предполагает (дребезг гасит RC-фильтр)", worst)
	}
	if !restored {
		h.r.fail("Столбцы", "после опроса столбцы не возвращены в 0 — следующее нажатие не вызовет INT0")
	}
}

// ---------------------------------------------------------------- программа 2: кольцевой буфер

type bufState struct {
	head, tail int
	empty, ovf bool
}

func (h *harness) setBuf(b bufState) {
	p := h.p
	h.s.SetIRAM(p.Head, byte(b.head), byte(b.head>>8))
	h.s.SetIRAM(p.Tail, byte(b.tail), byte(b.tail>>8))
	h.s.SetBit(p.Empty.Addr, b.empty)
	h.s.SetBit(p.Ovf.Addr, b.ovf)
}

func (h *harness) getBuf() bufState {
	p := h.p
	return bufState{
		head:  int(h.s.IRAM(p.Head)) | int(h.s.IRAM(p.Head+1))<<8,
		tail:  int(h.s.IRAM(p.Tail)) | int(h.s.IRAM(p.Tail+1))<<8,
		empty: h.s.Bit(p.Empty.Addr), ovf: h.s.Bit(p.Ovf.Addr),
	}
}

func (b bufState) String() string {
	return fmt.Sprintf("голова %04Xh, хвост %04Xh, F_EMPTY=%d, F_OVF=%d", b.head, b.tail, b2i(b.empty), b2i(b.ovf))
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

func prog2(h *harness) {
	p := h.p
	buf := p.Dev("Буфер (IDT7005)")
	S, E := buf.Base, buf.Base+p.V
	read := p.M%2 == 0
	switch h.runTo(100*ms, h.procAt, h.stopAt) {
	case h.procAt:
	case h.stopAt:
		h.r.fail("Вызов", "до %%stop%% не выполнилась строка с %%proc%% — для программы 2 call должен быть в основной программе")
		return
	default:
		h.r.fail("Инициализация", "за 100 мс не дошли до строки с %%proc%%")
		return
	}
	if got, want := h.getBuf(), (bufState{S, S, true, false}); got != want {
		h.r.fail("Инициализация", "перед вызовом: %s; ждали пустой буфер: %s", got, want)
	} else {
		h.r.pass("Инициализация", "буфер пуст: %s", got)
	}
	type sc struct {
		name  string
		in    bufState
		data  map[int]byte // заранее в буфере
		a     byte         // A на входе (запись)
		outs  []bufState   // допустимые итоги (первый — основной)
		wantA int          // чтение: ожидаемый A (-1 — не проверять)
		cell  int          // запись: куда должен лечь отсчёт
	}
	var bad []string
	// run — один вызов процедуры; возвращает итог (или nil, если не вернулась) и номер совпавшего допустимого итога (-1 — нет).
	run := func(c sc) (*bufState, int) {
		guard := map[int]byte{}
		for _, a := range []int{S - 1, S, S + 1, E - 1, E, S + 6, S + 7, S + 8, S + 9, S + 10, S + 19, S + 20, S + 21} {
			guard[a] = byte(h.rng.Intn(256))
			h.s.SetXRAM(a, guard[a])
		}
		for a, v := range c.data {
			guard[a] = v
			h.s.SetXRAM(a, v)
		}
		h.setBuf(c.in)
		h.s.SetA(c.a)
		if !h.call(20 * ms) {
			bad = append(bad, c.name+": процедура не вернулась за 20 мс")
			return nil, -1
		}
		got := h.getBuf()
		match := -1
		for i, o := range c.outs {
			if got == o {
				match = i
				break
			}
		}
		if match < 0 {
			var want []string
			for _, o := range c.outs {
				want = append(want, o.String())
			}
			bad = append(bad, fmt.Sprintf("%s: %s, ждали %s", c.name, got, strings.Join(want, " или ")))
		}
		if read && c.wantA >= 0 {
			if a := int(h.s.A()); a != c.wantA {
				bad = append(bad, fmt.Sprintf("%s: A = %s, ждали %s", c.name, hx(a), hx(c.wantA)))
			}
		}
		if !read {
			guard[c.cell] = c.a
		}
		for a, v := range guard {
			if got := h.s.XRAM(a); got != v {
				what := "лишняя запись"
				if !read && a == c.cell {
					what = "отсчёт не записан"
				}
				bad = append(bad, fmt.Sprintf("%s: %s в %04Xh (%s вместо %s)", c.name, what, a, hx(int(got)), hx(int(v))))
			}
		}
		return &got, match
	}
	name := map[bool]string{true: "Чтение", false: "Запись"}[read]
	n := 0
	policy := ""
	if read {
		for _, c := range []sc{
			{name: "пустой буфер", in: bufState{S + 7, S + 7, true, false}, outs: []bufState{{S + 7, S + 7, true, false}}, wantA: -1},
			{name: "один отсчёт", in: bufState{S + 8, S + 7, false, false}, data: map[int]byte{S + 7: 0xA5}, outs: []bufState{{S + 8, S + 8, true, false}}, wantA: 0xA5},
			{name: "три отсчёта", in: bufState{S + 10, S + 7, false, false}, data: map[int]byte{S + 7: 0x11, S + 8: 0x22, S + 9: 0x33}, outs: []bufState{{S + 10, S + 8, false, false}}, wantA: 0x11},
			{name: "заворот хвоста", in: bufState{S + 2, E - 1, false, false}, data: map[int]byte{E - 1: 0x3C, S: 0x3D}, outs: []bufState{{S + 2, S, false, false}}, wantA: 0x3C},
			{name: "последний отсчёт на границе", in: bufState{S, E - 1, false, false}, data: map[int]byte{E - 1: 0x42}, outs: []bufState{{S, S, true, false}}, wantA: 0x42},
			// заполненный буфер, годится для обеих политик (V и V−1): чтение сбрасывает флаг переполнения
			{name: "после переполнения", in: bufState{S + 19, S + 20, false, true}, data: map[int]byte{S + 20: 0x7E}, outs: []bufState{{S + 19, S + 21, false, false}}, wantA: 0x7E},
		} {
			run(c)
			n++
		}
	} else {
		for _, c := range []sc{
			{name: "в пустой буфер", in: bufState{S + 7, S + 7, true, false}, a: 0xA5, outs: []bufState{{S + 8, S + 7, false, false}}, cell: S + 7},
			{name: "в непустой", in: bufState{S + 9, S + 7, false, false}, a: 0x5A, outs: []bufState{{S + 10, S + 7, false, false}}, cell: S + 9},
			{name: "заворот головы", in: bufState{E - 1, S + 3, false, false}, a: 0x3C, outs: []bufState{{S, S + 3, false, false}}, cell: E - 1},
		} {
			run(c)
			n++
		}
		// голова догоняет хвост: по ТЗ «полон» можно понимать как V отсчётов (голова = хвост при F_EMPTY = 0) или как в методичке
		// (с. 14) — V−1: запись, после которой голова = хвост, уже переполнение (хвост сдвигается, F_OVF = 1). Принимаются обе.
		got, _ := run(sc{name: "голова догоняет хвост", in: bufState{S + 6, S + 7, false, false}, a: 0x66, cell: S + 6,
			outs: []bufState{{S + 7, S + 7, false, false}, {S + 7, S + 7, false, true}, {S + 7, S + 8, false, true}}})
		n++
		switch {
		case got != nil && got.tail == S+8:
			policy = "V − 1 (как в методичке: голова = хвост после записи — переполнение)"
			run(sc{name: "переполнение на границе", in: bufState{E - 1, S, false, false}, a: 0x88, cell: E - 1, outs: []bufState{{S, S + 1, false, true}}})
			n++
		case got != nil:
			policy = "V (голова = хвост при F_EMPTY = 0 — буфер полон)"
			for _, c := range []sc{
				{name: "переполнение", in: bufState{S + 7, S + 7, false, true}, a: 0x77, outs: []bufState{{S + 8, S + 8, false, true}}, cell: S + 7},
				{name: "переполнение на границе", in: bufState{E - 1, E - 1, false, false}, a: 0x88, outs: []bufState{{S, S, false, true}}, cell: E - 1},
			} {
				run(c)
				n++
			}
		}
	}
	if len(bad) > 0 {
		h.r.fail(name, "%s", joinCap(bad))
	} else {
		msg := fmt.Sprintf("%d сценариев (пусто, заворот, переполнение) — верно", n)
		if policy != "" {
			msg += "; политика буфера: " + policy
		}
		h.r.pass(name, "%s", msg)
	}
}

// ---------------------------------------------------------------- программа 3

// timerCheck — таймеры по ТЗ-2026: T1 и T3 — таймер 0, T2 — таймер 1 (проверка разрешения прерывания после инициализации).
func (h *harness) timerCheck(want int, what string) {
	ie := h.s.SFR(0xA8)
	et := []byte{0x02, 0x08}
	if ie&et[want] == 0 {
		h.r.warn("Таймер", "%s по ТЗ отсчитывает таймер %d, а его прерывание после инициализации не разрешено (IE = %02Xh)", what, want, ie)
	}
}

func tolMs(us float64) float64 { return math.Max(1*ms, us*0.01) }

func pinBit(pin string) int {
	var port, bit int
	fmt.Sscanf(pin, "P%d.%d", &port, &bit)
	return bit
}

// strobe — поиск импульса 0 на выводе P1.bit: время спада и фронта (мкс), -1 — не было.
type strobe struct {
	bit        int
	fall, rise float64
	other      bool // менялись другие выводы P1
}

func (h *harness) watchP1(st *strobe) {
	h.onP1 = func(old, cur byte) {
		m := byte(1 << st.bit)
		if (old^cur)&^m != 0 {
			st.other = true
		}
		t := h.s.Micros()
		if old&m != 0 && cur&m == 0 && st.fall < 0 {
			st.fall = t
		}
		if old&m == 0 && cur&m != 0 && st.fall >= 0 && st.rise < 0 {
			st.rise = t
		}
	}
}

func prog3Y1(h *harness) {
	p := h.p
	bit := pinBit(p.Y1Pin)
	h.s.BreakMem("sfr", 'w', 0x90)
	if h.runTo(100*ms, h.procAt) < 0 {
		h.r.fail("Инициализация", "за 100 мс не дошли до строки с %%proc%%")
		return
	}
	h.timerCheck(0, "T1")
	T := float64(p.T1ms) * ms
	for i := 1; i <= 2; i++ {
		h.p1 = h.s.Latch(1)
		if h.p1&(1<<bit) == 0 {
			h.r.fail("Строб Y1", "вызов %d: перед вызовом %s = 0 — пассивный уровень строба 1", i, p.Y1Pin)
			return
		}
		st := &strobe{bit: bit, fall: -1, rise: -1}
		h.watchP1(st)
		if i == 2 {
			h.toProc()
		}
		t0 := h.s.Micros()
		if h.runTo(10*ms, h.procRet) < 0 {
			h.r.fail("Строб Y1", "вызов %d: процедура не вернулась за 10 мс — ждать конец строба надо в прерывании таймера, а не в цикле", i)
			return
		}
		h.wait(T + 200*ms)
		switch {
		case st.fall < 0:
			h.r.fail("Строб Y1", "вызов %d: на %s не было спада", i, p.Y1Pin)
		case st.rise < 0:
			h.r.fail("Строб Y1", "вызов %d: строб не закончился за T1 + 200 мс", i)
		default:
			d := st.rise - st.fall
			lvl := Pass
			if math.Abs(d-T) > tolMs(T) {
				lvl = Fail
			} else if math.Abs(d-T) > T*0.002 {
				lvl = Warn
			}
			h.r.add(lvl, fmt.Sprintf("Строб Y1, вызов %d", i), "%.3f мс при T1 = %d мс (допуск ±%.1f мс), спад через %.0f мкс после call", d/ms, p.T1ms, tolMs(T)/ms, st.fall-t0)
		}
		if st.other {
			h.r.warn("P1", "вызов %d: менялись и другие выводы P1, кроме %s", i, p.Y1Pin)
		}
		if h.runTo(10*ms, h.stopAt) < 0 {
			h.r.fail("Возврат", "после строба не вернулись на %%stop%%")
			return
		}
	}
}

// сегменты (общий катод): 0–9, E; допустимые начертания 6, 7, 9 с/без лишнего сегмента
var segs = [][]byte{{0x3F}, {0x06}, {0x5B}, {0x4F}, {0x66}, {0x6D}, {0x7D, 0x7C}, {0x07, 0x27}, {0x7F}, {0x6F, 0x67}, {0x79}}

func prog3Ind(h *harness) {
	p := h.p
	off := byte(0)
	if p.Indicator == "anode" {
		off = 0xFF
	}
	type wr struct {
		t float64
		v byte
	}
	var log []wr
	h.s.BreakMem("xram", 'w', h.adrInd)
	h.onXW = func(addr int, v byte) {
		if addr == h.adrInd {
			log = append(log, wr{h.s.Micros(), v})
		}
	}
	if h.runTo(100*ms, h.procAt) < 0 {
		h.r.fail("Инициализация", "за 100 мс не дошли до строки с %%proc%%")
		return
	}
	h.timerCheck(0, "T3")
	shown := func() (byte, bool) {
		if len(log) == 0 {
			return 0, false
		}
		return log[len(log)-1].v, true
	}
	var bad []string
	first := true
	for code := 0; code <= 12; code++ {
		a := byte(code)
		if code == 12 {
			a = 0xFF
		}
		h.s.SetA(a)
		if !first {
			h.toProc()
		}
		first = false
		if h.runTo(10*ms, h.procRet) < 0 {
			bad = append(bad, fmt.Sprintf("код %d: процедура не вернулась за 10 мс", a))
			break
		}
		v, ok := shown()
		switch {
		case code <= 10:
			good := false
			for _, s := range segs[code] {
				good = good || ok && v == s^off
			}
			if !good {
				bad = append(bad, fmt.Sprintf("код %d: на индикаторе %s, ждали %s", a, hx(int(v)), hx(int(segs[code][0]^off))))
			}
		default:
			if ok && v != off {
				bad = append(bad, fmt.Sprintf("код %s: на индикаторе %s, ждали погашенный %s", hx(int(a)), hx(int(v)), hx(int(off))))
			}
		}
		h.runTo(10*ms, h.stopAt)
	}
	if len(bad) > 0 {
		h.r.fail("Символы", "%s", joinCap(bad))
	} else {
		h.r.pass("Символы", "0–9, E и гашение (коды 11, FFh) — верно; %s", map[bool]string{true: "общий анод", false: "общий катод"}[off == 0xFF])
	}
	// длительность T3 и перезапуск повторным вызовом
	T := float64(p.T3ms) * ms
	measure := func(name string, code byte, again float64) {
		log = nil
		h.s.SetA(code)
		h.toProc()
		h.runTo(10*ms, h.procRet)
		if len(log) == 0 {
			h.r.fail(name, "вызов не записал символ в индикатор")
			return
		}
		t0 := log[len(log)-1].t
		if again > 0 {
			h.runTo(10*ms, h.stopAt)
			h.wait(again)
			log = nil
			h.s.SetA(7)
			h.toProc()
			h.runTo(10*ms, h.procRet)
			if len(log) == 0 {
				h.r.fail(name, "повторный вызов не записал символ")
				return
			}
			t0 = log[len(log)-1].t
		}
		h.runTo(10*ms, h.stopAt)
		n := len(log)
		h.wait(T + 500*ms)
		for _, w := range log[n:] {
			if w.v == off {
				d := w.t - t0
				lvl := Pass
				if math.Abs(d-T) > tolMs(T) {
					lvl = Fail
				} else if math.Abs(d-T) > T*0.002 {
					lvl = Warn
				}
				h.r.add(lvl, name, "погашен через %.3f с при T3 = %.3f с (допуск ±%.0f мс)", d/1e6, T/1e6, tolMs(T)/ms)
				return
			}
		}
		h.r.fail(name, "за T3 + 0,5 с индикатор не погас")
	}
	measure("Время индикации T3", 5, 0)
	measure("Перезапуск T3 новым символом", 3, 1000*ms)
}

func prog3Y2(h *harness) {
	p := h.p
	var x1, x2 = -1, -1
	for _, s := range h.res.Symbols {
		switch strings.ToUpper(s.Name) {
		case "X1":
			x1 = s.Value
		case "X2":
			x2 = s.Value
		}
	}
	if x1 < 0 || x2 < 0 || x1 > 0x7F || x2 > 0x7F {
		h.r.fail("X1, X2", "нужны переменные X1 и X2 во внутреннем ОЗУ (DSEG: «X1: DS 1», «X2: DS 1»)")
		return
	}
	bit := pinBit(p.Y2Pin)
	type wr struct {
		t float64
		v byte
	}
	var log []wr
	h.s.BreakMem("xram", 'w', h.adrY2)
	h.s.BreakMem("sfr", 'w', 0x90)
	h.onXW = func(addr int, v byte) {
		if addr == h.adrY2 {
			log = append(log, wr{h.s.Micros(), v})
		}
	}
	if h.runTo(100*ms, h.procAt) < 0 {
		h.r.fail("Инициализация", "за 100 мс не дошли до строки с %%proc%%")
		return
	}
	h.timerCheck(1, "T2")
	T := float64(p.T2us)
	tol := math.Max(20, T*0.01)
	var bad []string
	worst := 0.0
	cases := [][2]int{{3, 200}, {9, 255}, {0, 77}, {1, 0}, {5, 250}, {9, 9}}
	for i, c := range cases {
		want := 0
		if c[0] != 0 {
			want = (p.G + p.M + c[0] + c[1]) % 256
		}
		name := fmt.Sprintf("X1=%d, X2=%d", c[0], c[1])
		h.s.SetIRAM(x1, byte(c[0]))
		h.s.SetIRAM(x2, byte(c[1]))
		log = nil
		h.p1 = h.s.Latch(1)
		if h.p1&(1<<bit) == 0 {
			bad = append(bad, name+": перед вызовом "+p.Y2Pin+" = 0 (пассивный уровень строба 1)")
			break
		}
		st := &strobe{bit: bit, fall: -1, rise: -1}
		h.watchP1(st)
		if i > 0 {
			h.toProc()
		}
		if h.runTo(10*ms, h.procRet) < 0 {
			bad = append(bad, name+": процедура не вернулась за 10 мс")
			break
		}
		h.wait(T + 5*ms)
		switch {
		case len(log) != 1:
			bad = append(bad, fmt.Sprintf("%s: записей в регистр Y2 — %d, нужна одна", name, len(log)))
		case int(log[0].v) != want:
			bad = append(bad, fmt.Sprintf("%s: Y2 = %d, ждали %d", name, log[0].v, want))
		}
		switch {
		case st.fall < 0 || st.rise < 0:
			bad = append(bad, name+": нет строба 0 на "+p.Y2Pin)
		default:
			if len(log) > 0 && log[0].t > st.fall {
				bad = append(bad, name+": строб начался раньше записи Y2 — данные должны быть в регистре к началу строба")
			}
			d := st.rise - st.fall
			worst = math.Max(worst, math.Abs(d-T))
			if math.Abs(d-T) > tol {
				bad = append(bad, fmt.Sprintf("%s: строб %.0f мкс, ждали %d ± %.0f", name, d, p.T2us, tol))
			}
		}
		if st.other {
			bad = append(bad, name+": менялись другие выводы P1")
		}
		if h.runTo(10*ms, h.stopAt) < 0 {
			bad = append(bad, name+": не вернулись на %stop%")
			break
		}
	}
	if len(bad) > 0 {
		h.r.fail("Y2 и строб", "%s", joinCap(bad))
	} else {
		h.r.pass("Y2 и строб", "%d сценариев: значение, X1 = 0 → 0, строб T2 = %d мкс (отклонение ≤ %.0f мкс, допуск %.0f)", len(cases), p.T2us, worst, tol)
	}
}

// joinCap — первые 6 ошибок и счётчик остальных (отчёт читают люди и ИИ, простыня не помогает).
func joinCap(bad []string) string {
	if len(bad) > 6 {
		return strings.Join(bad[:6], "; ") + fmt.Sprintf("; …и ещё %d", len(bad)-6)
	}
	return strings.Join(bad, "; ")
}
