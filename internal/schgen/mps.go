package schgen

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Variant — всё, что в схеме зависит от варианта.
type Variant struct {
	Cols, Rows int     // клавиатура: столбцы × строки (4×3 при чётном M, 3×4 при нечётном)
	Anode      bool    // индикатор с общим анодом (FYS-5612BX)
	CS         CSPins  // P2.x для каждого устройства
	Y1, Y2     string  // P1.x стробов
	KbInt      string  // INT0 | INT1 — вход МК для IRQ клавиатуры
	X2Int      string  // второй INT — строб X2
	Decoder    bool    // ТЗ-2026: CS — выходы 74HC138 (A13..A15, E2 = CS_EN); CS.* = "Y0".."Y7"
	CSEn       string  // вывод МК CS_EN (P3.4)
	Filter     string  // номинал фильтров на корпус: «68 н» (2025) / «33 н» (2026)
	Style      Style   // вид листа (A–D), см. style.go
	Jitter     *Jitter // «почерк» (jitter.go); nil — Plain
	Date       string
	Student    string
	Checker    string
}

type CSPins struct{ Buf, Y2, Kb, Ind string }

// Имена цепей CS: как в принятой схеме 2025; в режиме дешифратора — как на рис. 6 ТЗ-2026.
var (
	nCSbuf = "~{CSbuf}"
	nCSy2  = "~{CSy2}"
	nCSkb  = "~{CSkb}"
	nCSind = "~{CSind}"
)

func setCSNames(decoder bool) {
	if decoder {
		nCSbuf, nCSy2, nCSkb, nCSind = "~{CS_buf}", "~{CS_rgo}", "~{CS_kbd}", "~{CS_ind}"
	} else {
		nCSbuf, nCSy2, nCSkb, nCSind = "~{CSbuf}", "~{CSy2}", "~{CSkb}", "~{CSind}"
	}
}

const (
	nWR  = "~{WR}"
	nRD  = "~{RD}"
	nALE = "ALE"
	nY1  = "~{Y1stb}"
	nY2  = "~{Y2stb}"
)

// Координаты шин (мм, сетка 1,27).
const (
	yTop = 29.21  // верхняя горизонтальная шина
	xBL  = 48.26  // шина слева от МК (P1/P3)
	xBM  = 100.33 // шина справа от МК
	xBI  = 171.45 // шина слева от IDT7005
	xBR  = 231.14 // шина справа от IDT7005
	yH1  = 127.0  // шина под МК к клавиатуре
	xBK  = 22.86  // шина к регистру столбцов
	yH2  = 138.43 // шина к нижнему правому блоку
	xBD  = 198.12 // вертикальная шина нижнего правого блока
)

type builder struct {
	*Sheet
	v        Variant
	connBase map[string]int
	refs     map[string]int
	filterCs []*Comp
}

// gostSyms — символы, у которых есть ГОСТ-вариант «_G» (cmd/mpslib/gost.go).
var gostGateSyms = map[string]bool{"74HC02": true, "74HC32": true, "74HC11": true, "74HC21": true}
var gostICSyms = map[string]bool{"74HC573": true, "74HC173": true, "74HC244": true, "74HC138": true, "IDT7005": true, "AT89S53": true}

// Sym ставит символ с учётом стиля: ГОСТ-вариант, если стиль его требует.
func (b *builder) Sym(sym, ref, value string, at Pt, o SymOpt) *Comp {
	st := b.v.Style
	if (st.GostGates && gostGateSyms[sym]) || (st.GostICs && gostICSyms[sym]) {
		if _, ok := b.lib.Syms[sym+"_G"]; ok {
			sym += "_G"
		}
	}
	return b.Sheet.Sym(sym, ref, value, at, o)
}

// cp — контакт k (с 1) разъёма c с учётом сквозной нумерации частей.
func cp(c *Comp, k int) Pt { return c.Pin(strconv.Itoa(c.PinBase + k)) }

// role запоминает элемент под именем роли: после перенумерации по ГОСТ
// по ролям пишется примечание и проверяется netlist.
func (b *builder) role(name string, c *Comp) *Comp {
	b.Roles[name] = c
	return c
}

func (b *builder) next(prefix string) string {
	b.refs[prefix]++
	return fmt.Sprintf("%s%d", prefix, b.refs[prefix])
}

// mcuPin: "P2.5" → номер вывода AT89S53 в DIP-40.
func mcuPin(p string) string {
	if len(p) != 4 || p[0] != 'P' || p[2] != '.' {
		panic("плохой вывод порта " + p)
	}
	port, bit := int(p[1]-'0'), int(p[3]-'0')
	switch port {
	case 1:
		return strconv.Itoa(1 + bit)
	case 2:
		return strconv.Itoa(21 + bit)
	case 3:
		return strconv.Itoa(10 + bit)
	}
	panic("порт " + p)
}

// tapRight: вывод слева от шины busX → провод до шины, отвод «\» вниз к шине, метка у шины.
func (b *builder) tapRight(pin Pt, busX float64, name string) {
	e := Pt{busX - 2.54, pin.Y}
	b.Wire(pin, e)
	b.BusEntry(e, 2.54, 2.54)
	if b.Sheet.J.LabelAtPin && e.X-pin.X > 12 {
		b.Label(name, pin.Add(1.27, 0), false)
		return
	}
	b.Label(name, e, true)
}

// tapLeft: вывод справа от шины busX.
func (b *builder) tapLeft(pin Pt, busX float64, name string) {
	e := Pt{busX + 2.54, pin.Y}
	b.Wire(pin, e)
	b.BusEntry(Pt{busX, pin.Y + 2.54}, 2.54, -2.54)
	if b.Sheet.J.LabelAtPin && pin.X-e.X > 12 {
		b.Label(name, pin.Add(-1.27, 0), true)
		return
	}
	b.Label(name, e, false)
}

func (b *builder) gnd(at Pt) { b.Power("GND", at, 0) }
func (b *builder) vcc(at Pt) { b.Power("+5V", at, 0) }
func (b *builder) nc(pins ...Pt) {
	for _, p := range pins {
		b.NoConnect(p)
	}
}

// icLabels — обозначение и тип правее вывода питания сверху, как в принятой схеме.
func (b *builder) icLabels(vccTip Pt) SymOpt {
	if b.Sheet.J.RefLeft {
		r := vccTip.Add(-3.81, -2.54)
		v := vccTip.Add(-3.81, 0)
		return SymOpt{RefAt: &r, ValAt: &v, RefJust: "right", ValJust: "right"}
	}
	r := vccTip.Add(3.81, -2.54)
	v := vccTip.Add(3.81, 0)
	return SymOpt{RefAt: &r, ValAt: &v, RefJust: "left", ValJust: "left"}
}

// Build собирает лист Э3 под вариант.
func Build(lib *Lib, v Variant, seed string) *Sheet {
	b := &builder{Sheet: NewSheet(lib, seed), v: v, refs: map[string]int{}}
	b.Roles = map[string]*Comp{}
	b.Sheet.J = Plain
	if v.Jitter != nil {
		b.Sheet.J = *v.Jitter
	}
	if b.v.Style.Name == "" {
		b.v.Style = Styles["A"]
	}
	b.SetPerpEntries(b.v.Style.Perp)
	setCSNames(v.Decoder)
	if v.Filter == "" {
		v.Filter = "68 н"
		b.v.Filter = v.Filter
	}
	b.Title = TitleBlock{Date: v.Date, Comments: map[int]string{2: v.Student, 3: v.Checker}}
	b.connectors()
	b.mcu()
	b.addrLatch()
	b.y2Reg()
	b.idt()
	b.power()
	b.keyboard()
	b.rowsAndInt()
	b.indicator()
	if v.Decoder {
		mark := b.mark()
		b.decoder()
		b.moveSince(mark, b.Sheet.J.DecDX, b.Sheet.J.DecDY)
	}
	b.buses()
	b.Renumber()
	b.notes()
	return b.Sheet
}

// ---------------------------------------------------------------- МК DD3

func (b *builder) mcu() {
	at := Pt{73.66, 78.74}
	opt := b.icLabels(Pt{73.66, 38.1})
	u := b.role("mcu", b.Sym("AT89S53", "DD3", "AT89S53", at, opt))
	b.vcc(u.Pin("40"))
	b.gnd(u.Pin("20"))

	// левая сторона: стробы, INT, WR/RD — метки к шине BL
	left := map[string]string{
		mcuPin(b.v.Y1): nY1, mcuPin(b.v.Y2): nY2,
		"12": "~{INT0}", "13": "~{INT1}", "16": nWR, "17": nRD,
	}
	if b.v.Decoder {
		left[mcuPin(b.v.CSEn)] = "CS_EN"
	}
	for _, n := range []string{"1", "2", "3", "4", "5", "6", "7", "8", "10", "11", "12", "13", "14", "15", "16", "17"} {
		if name, ok := left[n]; ok {
			b.tapLeft(u.Pin(n), xBL, name)
		} else {
			b.nc(u.Pin(n))
		}
	}
	// правая сторона: AD0..7, A8..10, CS, ALE
	for i := 0; i < 8; i++ {
		b.tapRight(u.Pin(strconv.Itoa(39-i)), xBM, fmt.Sprintf("AD%d", i))
	}
	if b.v.Decoder {
		// ТЗ-2026: весь P2 — старший байт адреса A8..A15 (A13..A15 — на дешифратор CS)
		for i := 0; i < 8; i++ {
			b.tapRight(u.Pin(strconv.Itoa(21+i)), xBM, fmt.Sprintf("A%d", 8+i))
		}
		b.nc(u.Pin("29")) // PSEN: ПЗУ нет
		b.tapRight(u.Pin("30"), xBM, nALE)
		b.reset(u)
		return
	}
	for i := 0; i < 3; i++ {
		b.tapRight(u.Pin(strconv.Itoa(21+i)), xBM, fmt.Sprintf("A%d", 8+i))
	}
	cs := map[string]string{
		mcuPin(b.v.CS.Buf): nCSbuf, mcuPin(b.v.CS.Y2): nCSy2,
		mcuPin(b.v.CS.Kb): nCSkb, mcuPin(b.v.CS.Ind): nCSind,
	}
	for n := 24; n <= 28; n++ {
		if name, ok := cs[strconv.Itoa(n)]; ok {
			b.tapRight(u.Pin(strconv.Itoa(n)), xBM, name)
		} else {
			b.nc(u.Pin(strconv.Itoa(n)))
		}
	}
	b.nc(u.Pin("29")) // PSEN: ПЗУ нет
	b.tapRight(u.Pin("30"), xBM, nALE)
	b.reset(u)
}

// reset — сброс, кварц, EA.
func (b *builder) reset(u *Comp) {
	// сброс: C от +5 В на RST, R с RST на GND (τ = 100 мс)
	rst := u.Pin("9")
	node := Pt{40.64, rst.Y}
	b.Wire(rst, node)
	c := b.Sym("C_Polarized", b.next("C"), "10 мк", Pt{40.64, rst.Y - 3.81}, SymOpt{
		RefAt: ptr(Pt{35.56, rst.Y - 5.08}), ValAt: ptr(Pt{35.56, rst.Y - 2.54}), RefJust: "right", ValJust: "right"})
	b.role("cRst", c)
	b.Wire(c.Pin("2"), node)
	b.vcc(c.Pin("1"))
	r := b.Sym("R", b.next("R"), "10 к", Pt{40.64, rst.Y + 3.81}, SymOpt{
		RefAt: ptr(Pt{35.56, rst.Y + 2.54}), ValAt: ptr(Pt{35.56, rst.Y + 5.08}), RefJust: "right", ValJust: "right"})
	b.role("rRst", r)
	b.Wire(node, r.Pin("1"))
	b.gnd(r.Pin("2"))
	// RST заводим на MR регистра столбцов: сброс устройств по Reset (лекции ч1, с. 76, п. 5)
	b.Label("RST", Pt{50.8, rst.Y}, false)

	// кварц: ZQ1 вертикально, C слева
	x1, x2 := u.Pin("19"), u.Pin("18")
	zq := b.Sym("Crystal", "ZQ1", "12 МГц", Pt{40.64, 107.95}, SymOpt{Rot: 90,
		RefAt: ptr(Pt{44.45, 106.68}), ValAt: ptr(Pt{44.45, 109.22}), RefJust: "left", ValJust: "left"})
	top, bot := zq.Pin("2"), zq.Pin("1")
	b.Wire(top, x1)
	b.Wire(bot, Pt{52.07, bot.Y}, Pt{52.07, x2.Y}, x2)
	ca := b.Sym("C", b.next("C"), "30 п", Pt{31.75, top.Y}, SymOpt{Rot: 90,
		RefAt: ptr(Pt{30.48, top.Y - 5.08}), ValAt: ptr(Pt{33.655, top.Y - 2.54}), RefJust: "left", ValJust: "left"})
	cb := b.Sym("C", b.next("C"), "30 п", Pt{31.75, bot.Y}, SymOpt{Rot: 90,
		RefAt: ptr(Pt{30.48, bot.Y - 5.08}), ValAt: ptr(Pt{33.655, bot.Y - 2.54}), RefJust: "left", ValJust: "left"})
	b.role("zq", zq)
	b.role("cX1", ca)
	b.role("cX2", cb)
	b.Wire(ca.Pin("2"), top)
	b.Wire(cb.Pin("2"), bot)
	g := Pt{27.94, bot.Y + 2.54}
	b.Wire(ca.Pin("1"), Pt{27.94, top.Y}, Pt{27.94, bot.Y}, g)
	b.Wire(cb.Pin("1"), Pt{27.94, bot.Y})
	b.gnd(g)

	// EA на +5 В: работа из внутренней Flash
	ea := u.Pin("31")
	b.Wire(ea, Pt{55.88, ea.Y}, Pt{55.88, ea.Y + 2.54})
	b.Power("+5V", Pt{55.88, ea.Y + 2.54}, 180)
}

func ptr(p Pt) *Pt { return &p }

// ---------------------------------------------------------------- защёлка адреса DD4

func (b *builder) latch573(ref string, at Pt) *Comp {
	u := b.Sym("74HC573", ref, "74AC573", at, b.icLabels(Pt{at.X, at.Y - 20.32}))
	b.vcc(u.Pin("20"))
	b.gnd(u.Pin("10"))
	// OE → GND
	oe := u.Pin("1")
	b.Wire(oe, Pt{oe.X - 2.54, oe.Y}, Pt{oe.X - 2.54, oe.Y + 1.27})
	b.gnd(Pt{oe.X - 2.54, oe.Y + 1.27})
	return u
}

func (b *builder) addrLatch() {
	u := b.role("latchA", b.latch573("DD4", Pt{142.24, 55.88}))
	for i := 0; i < 8; i++ {
		b.tapLeft(u.Pin(strconv.Itoa(2+i)), xBM, fmt.Sprintf("AD%d", i))
		b.tapRight(u.Pin(strconv.Itoa(19-i)), xBI, fmt.Sprintf("A%d", i))
	}
	b.tapLeft(u.Pin("11"), xBM, nALE)
}

// ---------------------------------------------------------------- регистр Y2 DD5 + DD1.2

func (b *builder) y2Reg() {
	u := b.role("latchY2", b.latch573("DD5", Pt{142.24, 109.22}))
	for i := 0; i < 8; i++ {
		b.tapLeft(u.Pin(strconv.Itoa(2+i)), xBM, fmt.Sprintf("AD%d", i))
		q := u.Pin(strconv.Itoa(19 - i))
		e := Pt{160.02, q.Y}
		b.Wire(q, e)
		b.BusEntry(e, 2.54, 2.54)
		b.Label(fmt.Sprintf("Y2_%d", i), Pt{q.X + 1.27, q.Y}, false)
	}
	// Load = ИЛИ-НЕ(CSy2, WR)
	ld := u.Pin("11")
	g := b.role("norY2", b.Sym("74HC02", "DD1", "74HC02", Pt{ld.X - 11.43, ld.Y}, SymOpt{Unit: 1,
		RefAt: ptr(Pt{ld.X - 13.97, ld.Y - 5.08}), RefJust: "left", HideVal: true}))
	b.Wire(g.Pin("1"), ld)
	b.tapLeft(g.Pin("2"), xBM, nCSy2)
	b.tapLeft(g.Pin("3"), xBM, nWR)
}

// ---------------------------------------------------------------- IDT7005 DD8

func (b *builder) idt() {
	at := Pt{200.66, 81.28}
	r, v := Pt{205.74, 34.29}, Pt{205.74, 36.83}
	u := b.role("idt", b.Sym("IDT7005", "DD8", "IDT7005S55PF", at, SymOpt{RefAt: &r, ValAt: &v, RefJust: "left", ValJust: "left"}))
	// питание: три VCC сверху связаны, GND снизу
	vt := 35.56
	for _, n := range []string{"8", "13", "57"} {
		p := u.Pin(n)
		b.Wire(p, Pt{p.X, vt})
	}
	b.Wire(Pt{u.Pin("8").X, vt}, Pt{u.Pin("57").X, vt})
	b.vcc(Pt{u.Pin("13").X, vt})
	gy := u.Pin("5").Y + 2.54
	for _, n := range []string{"5", "9", "24", "41"} {
		b.Wire(u.Pin(n), Pt{u.Pin(n).X, gy})
	}
	b.Wire(Pt{u.Pin("5").X, gy}, Pt{u.Pin("41").X, gy})
	b.gnd(Pt{u.Pin("24").X, gy})

	// левый порт — МК
	b.tapLeft(u.Pin("59"), xBI, nCSbuf)
	b.tapLeft(u.Pin("62"), xBI, nRD)
	for i := 0; i <= 10; i++ {
		b.tapLeft(u.Pin(strconv.Itoa(44+i)), xBI, fmt.Sprintf("A%d", i))
	}
	dl := []string{"63", "64", "1", "2", "3", "4", "6", "7"}
	for i, n := range dl {
		b.tapLeft(u.Pin(n), xBI, fmt.Sprintf("AD%d", i))
	}
	// R/WL ← WR: МК и читает буфер, и пишет в него (процедура записи — данные из A)
	b.tapLeft(u.Pin("61"), xBI, nWR)
	// BUSYL, SEML → +5 В (BUSY-вход в slave не блокирует, семафоры не используются)
	xl := 185.42
	for _, n := range []string{"42", "60"} {
		b.Wire(u.Pin(n), Pt{xl, u.Pin(n).Y})
	}
	b.Wire(Pt{xl, u.Pin("42").Y}, Pt{xl, u.Pin("60").Y})
	b.Power("+5V", Pt{xl, u.Pin("42").Y}, 90)
	// M/S → GND: режим slave, арбитраж BUSY выключен
	ms := u.Pin("40")
	b.Wire(ms, Pt{xl, ms.Y})
	b.Power("GND", Pt{xl, ms.Y}, 270)
	b.nc(u.Pin("43"))
	if b.v.Decoder {
		// ТЗ-2026: окно 8 КБ, буфер до 2100 байт — нужны A11, A12
		b.tapLeft(u.Pin("55"), xBI, "A11")
		b.tapLeft(u.Pin("56"), xBI, "A12")
	} else {
		// A11L, A12L → GND (буфер ≤ 2 КБ)
		b.Wire(u.Pin("55"), Pt{xl, u.Pin("55").Y})
		b.Wire(u.Pin("56"), Pt{xl, u.Pin("56").Y}, Pt{xl, u.Pin("55").Y})
		b.Wire(Pt{xl, u.Pin("56").Y}, Pt{xl, u.Pin("56").Y + 1.27})
		b.gnd(Pt{xl, u.Pin("56").Y + 1.27})
	}

	// правый порт — «почтовый ящик» внешнего устройства (ТЗ: независимая 8-разрядная шина со стробом записи):
	// адрес зашит единицами (последняя ячейка окна, вне кольца), CER = 0, OER = 1 — порт только пишет,
	// запись — стробом X2stb (R/WR), он же — прерывание МК; в обработчике МК читает ящик левым портом
	// и кладёт отсчёт в кольцо процедурой записи. Указатели и флаги ведёт МК.
	xr := 215.9
	cer := u.Pin("22")
	b.Wire(cer, Pt{xr, cer.Y})
	b.Power("GND", Pt{xr, cer.Y}, 90)
	b.tapRight(u.Pin("20"), xBR, "~{"+b.v.X2Int+"}")
	oer := u.Pin("19")
	b.Wire(oer, Pt{xr - 2.54, oer.Y})
	b.Power("+5V", Pt{xr - 2.54, oer.Y}, 270)
	b.Wire(u.Pin("39"), Pt{xr, u.Pin("39").Y}, Pt{xr, u.Pin("21").Y})
	b.Wire(u.Pin("21"), Pt{xr, u.Pin("21").Y})
	b.Power("+5V", Pt{xr, u.Pin("39").Y}, 270)
	b.nc(u.Pin("38"))
	last := 12
	if !b.v.Decoder {
		last = 10 // A11R, A12R → GND (буфер ≤ 2 КБ)
		b.Wire(u.Pin("26"), Pt{xr, u.Pin("26").Y})
		b.Wire(u.Pin("25"), Pt{xr, u.Pin("25").Y}, Pt{xr, u.Pin("26").Y})
		b.Wire(Pt{xr, u.Pin("25").Y}, Pt{xr, u.Pin("25").Y + 1.27})
		b.gnd(Pt{xr, u.Pin("25").Y + 1.27})
	}
	top, bot := u.Pin("37"), u.Pin(strconv.Itoa(37-last))
	for i := 0; i <= last; i++ {
		p := u.Pin(strconv.Itoa(37 - i))
		b.Wire(p, Pt{xr, p.Y})
	}
	b.Wire(Pt{xr, top.Y}, Pt{xr, bot.Y})
	b.Power("+5V", Pt{xr, top.Y}, 270)
	dr := []string{"10", "11", "12", "14", "15", "16", "17", "18"}
	for i, n := range dr {
		b.tapRight(u.Pin(n), xBR, fmt.Sprintf("X2_%d", i))
	}
}

// ---------------------------------------------------------------- разъёмы XS2 (X2), XS3 (Y)

const (
	xConn = 256.54 // точка подключения выводов разъёмов
	yXS2  = 58.42
	yXS3  = 106.68
)

func (b *builder) connectors() {
	x2 := []ConnPin{{Signal: "~{X2stb}"}}
	for i := 0; i < 8; i++ {
		x2 = append(x2, ConnPin{Signal: fmt.Sprintf("X2_%d", i)})
	}
	x2 = append(x2, ConnPin{Signal: "GND"})
	y := []ConnPin{{Signal: "~{Y1stb}"}, {Signal: "~{Y2stb}"}}
	for i := 0; i < 8; i++ {
		y = append(y, ConnPin{Signal: fmt.Sprintf("Y2_%d", i)})
	}
	y = append(y, ConnPin{Signal: "GND"})
	pwr := []ConnPin{{Signal: "Vcc", Power: true}, {Signal: "GND", Power: true}}
	cs := b.v.Style.Conn
	if b.v.Style.Sections {
		// один XS1: XS1.1 — X2, XS1.2 — Y, XS1.3 — питание; нумерация контактов сквозная
		b.lib.AddConn("CONN_XS", [][]ConnPin{x2, y, pwr}, cs)
		b.connBase = map[string]int{"x2": 0, "y": len(x2), "pwr": len(x2) + len(y)}
		return
	}
	b.lib.AddConn("CONN_X2", [][]ConnPin{x2}, cs)
	b.lib.AddConn("CONN_Y", [][]ConnPin{y}, cs)
	b.lib.AddConn("CONN_PWR", [][]ConnPin{pwr}, cs)
}

// conn ставит разъём (или его часть при Sections).
func (b *builder) conn(role, which, sym, ref string, at Pt) *Comp {
	if b.v.Style.Sections {
		unit := map[string]int{"x2": 1, "y": 2, "pwr": 3}[which]
		c := b.role(role, b.Sym("CONN_XS", "XS1", "", at, SymOpt{HideVal: true, Unit: unit}))
		c.PinBase = b.connBase[which]
		return c
	}
	return b.role(role, b.Sym(sym, ref, "", at, SymOpt{HideVal: true}))
}

func (b *builder) placeConnXY() {
	const bx = 250.19 // шины к разъёмам
	// XS2: строб X2 → вход прерывания и R/WR правого порта IDT, данные X2 → правый порт IDT
	c := b.conn("xsX2", "x2", "CONN_X2", "XS2", Pt{xConn, yXS2})
	stb := cp(c, 1)
	e := Pt{xBR + 2.54, stb.Y}
	b.Wire(stb, e)
	b.BusEntry(Pt{xBR, stb.Y + 2.54}, 2.54, -2.54)
	b.Label("~{"+b.v.X2Int+"}", e, false)
	for i := 0; i < 8; i++ {
		p := cp(c, 2+i)
		b.Wire(p, Pt{bx + 2.54, p.Y})
		b.BusEntry(Pt{bx, p.Y + 2.54}, 2.54, -2.54)
		b.Label(fmt.Sprintf("X2_%d", i), Pt{bx + 2.54, p.Y}, false)
	}
	gp := cp(c, 10)
	b.Wire(gp, Pt{gp.X - 1.27, gp.Y}, Pt{gp.X - 1.27, gp.Y + 1.27})
	b.gnd(Pt{gp.X - 1.27, gp.Y + 1.27})
	top := cp(c, 2).Y + 2.54
	b.Bus(Pt{bx, top}, Pt{bx, 99.06}, Pt{xBR, 99.06})

	// XS3: стробы Y1/Y2 с P1, данные Y2 с DD5
	c = b.conn("xsY", "y", "CONN_Y", "XS3", Pt{xConn, yXS3})
	for i, n := range []string{nY1, nY2} {
		p := cp(c, 1+i)
		e := Pt{xBR + 2.54, p.Y}
		b.Wire(p, e)
		b.BusEntry(Pt{xBR, p.Y + 2.54}, 2.54, -2.54)
		b.Label(n, e, false)
	}
	for i := 0; i < 8; i++ {
		p := cp(c, 3+i)
		b.Wire(p, Pt{bx + 2.54, p.Y})
		b.BusEntry(Pt{bx, p.Y + 2.54}, 2.54, -2.54)
		b.Label(fmt.Sprintf("Y2_%d", i), Pt{bx + 2.54, p.Y}, false)
	}
	gp = cp(c, 11)
	b.Wire(gp, Pt{gp.X - 1.27, gp.Y}, Pt{gp.X - 1.27, gp.Y + 1.27})
	b.gnd(Pt{gp.X - 1.27, gp.Y + 1.27})
	// шина Y2: от DD5 вниз, вправо под IDT, вверх к разъёму
	b.Bus(Pt{162.56, 96.52 + 2.54}, Pt{162.56, 132.08}, Pt{bx, 132.08})
	b.Bus(Pt{bx, cp(c, 3).Y + 2.54}, Pt{bx, cp(c, 10).Y + 2.54})
}

// ---------------------------------------------------------------- клавиатура

// раскладка по рис. 4 ТЗ: [строка][столбец]
func keyLegend(cols, rows int) [][]string {
	if cols == 4 {
		return [][]string{{"1", "2", "3", "Т"}, {"4", "5", "6", "С"}, {"7", "8", "9", "0"}}
	}
	return [][]string{{"1", "2", "3"}, {"4", "5", "6"}, {"7", "8", "9"}, {"0", "Т", "С"}}
}

const (
	kbX0    = 83.82  // первый столбец
	kbDX    = 12.7   // шаг столбцов
	kbRow0  = 182.88 // первая строка
	kbDY    = 12.7
	kbVcc   = 73.66 // вертикаль +5 В подтяжек
	kbPull  = 77.47
	kbRser  = 137.16
	kbC     = 143.51
	kbRowX0 = 152.4 // вертикали строк к 244 и «И»
	kbColY  = 154.94
)

func (b *builder) rowY(r int) float64 { return kbRow0 + float64(r)*b.Sheet.J.KbDY }

func (b *builder) keyboard() {
	v := b.v
	// DD2 74HC173 — столбцы
	at := Pt{63.5, 157.48}
	u := b.role("kb173", b.Sym("74HC173", "DD2", "74HC173", at, b.icLabels(Pt{at.X, at.Y - 22.86})))
	b.vcc(u.Pin("16"))
	b.gnd(u.Pin("8"))
	for i := 0; i < 4; i++ {
		b.tapLeft(u.Pin(strconv.Itoa(14-i)), xBK, fmt.Sprintf("AD%d", i))
	}
	// Oe1, Oe2 → GND (выходы всегда открыты)
	o1, o2 := u.Pin("1"), u.Pin("2")
	xo := o1.X - 2.54
	b.Wire(o1, Pt{xo, o1.Y}, Pt{xo, o2.Y})
	b.Wire(o2, Pt{xo, o2.Y})
	b.Power("GND", Pt{xo, o1.Y}, 270)
	// E1, E2 ← CSkb
	e1, e2 := u.Pin("9"), u.Pin("10")
	b.tapLeft(e2, xBK, nCSkb)
	b.Wire(e1, Pt{e1.X - 5.08, e1.Y}, Pt{e1.X - 5.08, e2.Y})
	// Cp ← WR: фронт в конце записи, данные на P0 стоят уже сотни нс (ч1 лекций, с. 76–78);
	// E1, E2 = CSkb разрешают запись только при обращении к клавиатуре
	b.tapLeft(u.Pin("7"), xBK, nWR)
	// MR ← RST: при сбросе МК столбцы обнуляются
	b.tapLeft(u.Pin("15"), xBK, "RST")

	// выходы Q → шина столбцов
	xc := 85.09
	for i := 0; i < v.Cols; i++ {
		q := u.Pin(strconv.Itoa(3 + i))
		e := Pt{xc - 2.54, q.Y}
		b.Wire(q, e)
		b.BusEntry(e, 2.54, 2.54)
		b.Label(fmt.Sprintf("Col%d", i+1), Pt{q.X + 1.27, q.Y}, false)
	}
	for i := v.Cols; i < 4; i++ {
		b.nc(u.Pin(strconv.Itoa(3 + i)))
	}
	lastCol := kbX0 + float64(v.Cols-1)*kbDX
	b.Bus(Pt{xc, u.Pin("3").Y + 2.54}, Pt{xc, kbColY}, Pt{lastCol + 2.54, kbColY})

	legend := keyLegend(v.Cols, v.Rows)
	lastRow := b.rowY(v.Rows - 1)
	// столбцы: отвод с шины, диод катодом к столбцу, вертикаль вниз
	for c := 0; c < v.Cols; c++ {
		x := kbX0 + float64(c)*kbDX
		top := Pt{x, kbColY + 2.54}
		b.BusEntry(Pt{x + 2.54, kbColY}, -2.54, 2.54)
		b.Label(fmt.Sprintf("Col%d", c+1), top.Add(0, 1.27), false)
		d := b.role(fmt.Sprintf("vd%d", c), b.Sym("D", b.next("VD"), "КД521А", Pt{x, 163.83}, SymOpt{Rot: 270,
			RefAt: ptr(Pt{x + 2.54, 163.83}), RefJust: "left", HideVal: true}))
		b.Wire(top, d.Pin("1"))
		b.Wire(d.Pin("2"), Pt{x, lastRow - 3.81})
	}
	// кнопки: SB нумеруются по столбцам сверху вниз
	for c := 0; c < v.Cols; c++ {
		x := kbX0 + float64(c)*kbDX
		for r := 0; r < v.Rows; r++ {
			y := b.rowY(r) - 3.81
			sw := b.role(fmt.Sprintf("sb%d_%d", c, r), b.Sym("SW_Push", b.next("SB"), legend[r][c], Pt{x + 5.08, y}, SymOpt{
				RefAt: ptr(Pt{x + 5.08, y - 6.35}), ValAt: ptr(Pt{x + 5.08, y - 4.445})}))
			p2 := sw.Pin("2")
			b.Wire(p2, Pt{p2.X, b.rowY(r)})
		}
	}
	// строки: подтяжка 2k к +5 В, последовательный 10k, C на GND (3RC ≥ 5 мс)
	b.vcc(Pt{kbVcc, b.rowY(0) - 5.08})
	b.Wire(Pt{kbVcc, b.rowY(0) - 5.08}, Pt{kbVcc, lastRow})
	pull := make([]*Comp, v.Rows)
	for r := 0; r < v.Rows; r++ {
		y := b.rowY(r)
		pull[r] = b.role(fmt.Sprintf("pull%d", r), b.Sym("R", b.next("R"), "2 к", Pt{kbPull, y}, SymOpt{Rot: 90,
			RefAt: ptr(Pt{kbPull, y - 5.08}), ValAt: ptr(Pt{kbPull, y - 2.54})}))
		b.Wire(Pt{kbVcc, y}, pull[r].Pin("1"))
	}
	for r := 0; r < v.Rows; r++ {
		y := b.rowY(r)
		rs := b.role(fmt.Sprintf("ser%d", r), b.Sym("R", b.next("R"), "10 к", Pt{kbRser, y}, SymOpt{Rot: 90,
			RefAt: ptr(Pt{kbRser, y - 5.08}), ValAt: ptr(Pt{kbRser, y - 2.54})}))
		b.Wire(pull[r].Pin("2"), rs.Pin("1"))
		b.Wire(rs.Pin("2"), Pt{kbC, y})
	}
	for r := 0; r < v.Rows; r++ {
		y := b.rowY(r)
		c := b.Sym("C", b.next("C"), "220 н", Pt{kbC, y + 3.81}, SymOpt{
			RefAt: ptr(Pt{kbC + 2.54, y + 3.175}), ValAt: ptr(Pt{kbC + 2.54, y + 5.715}), RefJust: "left", ValJust: "left"})
		b.gnd(c.Pin("2"))
		b.role(fmt.Sprintf("ck%d", r), c)
	}
}

// ---------------------------------------------------------------- строки → 244, «И» → INT; OR → OE 244

func (b *builder) rowsAndInt() {
	v := b.v
	at := Pt{173.99, 195.58}
	u := b.role("buf", b.Sym("74HC244", "DD7", "74AC244", at, b.icLabels(Pt{at.X, at.Y - 20.32})))
	b.vcc(u.Pin("20"))
	b.gnd(u.Pin("10"))
	in := []string{"2", "4", "6", "8"}
	outs := []string{"18", "16", "14", "12"}

	// «И» по строкам: 3 строки — 74HC11 (3И), 4 строки — 74HC21 (4И)
	andSym, andIn, andOut := "74HC11", []string{"1", "2", "13"}, "12"
	if v.Rows == 4 {
		andSym, andIn, andOut = "74HC21", []string{"1", "2", "4", "5"}, "6"
	}
	ga := b.role("and", b.Sym(andSym, "DD6", andSym, Pt{168.91, 160.02}, SymOpt{Unit: 1,
		RefAt: ptr(Pt{166.37, 153.67}), RefJust: "left", HideVal: true}))
	for r := 0; r < v.Rows; r++ {
		y := b.rowY(r)
		x := kbRowX0 + float64(r)*1.27
		name := fmt.Sprintf("Row%d", r+1)
		b.Label(name, Pt{kbC + 0.635, y}, false)
		a := ga.Pin(andIn[r])
		pin := u.Pin(in[r])
		b.Wire(Pt{kbC, y}, Pt{x, y})
		b.Wire(Pt{x, y}, Pt{x, a.Y}, a)
		if r == 0 {
			b.Wire(Pt{x, y}, pin)
		} else {
			b.Wire(Pt{x, pin.Y}, pin)
		}
		b.tapRight(u.Pin(outs[r]), xBD, fmt.Sprintf("AD%d", r))
	}
	// выход «И» → вход прерывания клавиатуры
	b.tapRight(ga.Pin(andOut), xBD, "~{"+v.KbInt+"}")

	// неиспользуемые входы 244 → GND
	unused := []string{"8", "17", "15", "13", "11"}[v.Rows-3:]
	xg := 158.75
	var ys []float64
	for _, n := range unused {
		p := u.Pin(n)
		b.Wire(p, Pt{xg, p.Y})
		ys = append(ys, p.Y)
	}
	b.Wire(Pt{xg, ys[0]}, Pt{xg, ys[len(ys)-1]}, Pt{xg, ys[len(ys)-1] + 1.27})
	b.gnd(Pt{xg, ys[len(ys)-1] + 1.27})
	for r := v.Rows; r < 4; r++ {
		b.nc(u.Pin(outs[r]))
	}
	for _, n := range []string{"3", "5", "7", "9"} {
		b.nc(u.Pin(n))
	}

	// 1OE, 2OE ← ИЛИ(CSkb, RD): чтение строк только при обращении к клавиатуре
	g := b.role("or", b.Sym("74HC32", "DD9", "74HC32", Pt{213.36, 213.36}, SymOpt{Unit: 1,
		RefAt: ptr(Pt{210.82, 207.01}), RefJust: "left", HideVal: true}))
	b.tapLeft(g.Pin("1"), xBD, nCSkb)
	b.tapLeft(g.Pin("2"), xBD, nRD)
	o := g.Pin("3")
	oe1, oe2 := u.Pin("1"), u.Pin("19")
	loopY := max(220.98, b.rowY(v.Rows-1)+5.08) // ниже выхода DD9.1 и последней строки клавиатуры
	b.Wire(o, Pt{223.52, o.Y}, Pt{223.52, loopY}, Pt{157.48, loopY}, Pt{157.48, oe1.Y}, oe1)
	b.Wire(Pt{157.48, oe2.Y}, oe2)
}

// ---------------------------------------------------------------- индикатор: DD10 + DD1.3 + R + HG1

func (b *builder) indicator() {
	at := Pt{238.76, 171.45}
	u := b.role("latchInd", b.latch573("DD10", at))
	for i := 0; i < 8; i++ {
		b.tapLeft(u.Pin(strconv.Itoa(2+i)), xBD, fmt.Sprintf("AD%d", i))
	}
	ld := u.Pin("11")
	g := b.role("norInd", b.Sym("74HC02", "DD1", "74HC02", Pt{ld.X - 10.16, ld.Y}, SymOpt{Unit: 2,
		RefAt: ptr(Pt{ld.X - 12.7, ld.Y - 6.35}), RefJust: "left", HideVal: true}))
	b.Wire(g.Pin("4"), ld)
	b.tapLeft(g.Pin("5"), xBD, nCSind)
	b.tapLeft(g.Pin("6"), xBD, nWR)

	// Q → шина → R → шина → сегменты
	const xq, xs = 259.08, 281.94
	for i := 0; i < 8; i++ {
		q := u.Pin(strconv.Itoa(19 - i))
		b.tapRight(q, xq, fmt.Sprintf("Q%d", i))
	}
	seg := []string{"a", "b", "c", "d", "e", "f", "g", "dp"}
	y0 := 153.67
	for i := 0; i < 8; i++ {
		y := y0 + float64(i)*5.08
		r := b.role(fmt.Sprintf("rseg%d", i), b.Sym("R", b.next("R"), "330", Pt{270.51, y}, SymOpt{Rot: 90,
			RefAt: ptr(Pt{270.18, y - 1.905}), ValAt: ptr(Pt{270.84, y - 1.905}), RefJust: "right", ValJust: "left"}))
		e := Pt{xq + 2.54, y}
		b.Wire(e, r.Pin("1"))
		b.BusEntry(Pt{xq, y + 2.54}, 2.54, -2.54)
		b.Label(fmt.Sprintf("Q%d", i), e, false)
		b.tapRight(r.Pin("2"), xs, seg[i])
	}
	b.Bus(Pt{xq, 158.75 + 2.54}, Pt{xq, y0 + 2.54})
	b.Bus(Pt{xq, y0 + 2.54}, Pt{xq, y0 + 7*5.08 + 2.54})
	sym, val := "FYS-5612AX", "FYS-5612AX"
	if b.v.Anode {
		sym, val = "FYS-5612BX", "FYS-5612BX"
	}
	hg := b.role("hg", b.Sym(sym, "HG1", val, Pt{303.53, 166.37}, SymOpt{
		RefAt: ptr(Pt{303.53, 149.86}), ValAt: ptr(Pt{303.53, 152.4}), RefJust: "center", ValJust: "center"}))
	pins := []string{"7", "6", "4", "2", "1", "9", "10", "5"}
	for i, n := range pins {
		b.tapLeft(hg.Pin(n), xs, seg[i])
	}
	b.Bus(Pt{xs, y0 + 2.54}, Pt{xs, y0 + 7*5.08 + 2.54})
	c1, c2 := hg.Pin("3"), hg.Pin("8")
	xc := c1.X + 2.54
	b.Wire(c1, Pt{xc, c1.Y}, Pt{xc, c2.Y})
	b.Wire(c2, Pt{xc, c2.Y})
	if b.v.Anode {
		b.Wire(Pt{xc, c1.Y}, Pt{xc, c1.Y - 2.54})
		b.vcc(Pt{xc, c1.Y - 2.54})
	} else {
		b.Wire(Pt{xc, c2.Y}, Pt{xc, c2.Y + 1.27})
		b.gnd(Pt{xc, c2.Y + 1.27})
	}
}

// ---------------------------------------------------------------- питание: XS1, 470u, 68n на каждый корпус

func (b *builder) power() {
	mark := b.mark() // полоса фильтров с XS1 сдвигается целиком (J.PwrDY)
	yv, yg := 238.76, 248.92
	x := 40.64
	b.vcc(Pt{x, yv - 2.54})
	b.Wire(Pt{x, yv - 2.54}, Pt{x, yv})
	ce := b.Sym("C_Polarized", b.next("C"), "470 мк", Pt{x, (yv + yg) / 2}, SymOpt{
		RefAt: ptr(Pt{x - 2.54, yv + 3.81}), ValAt: ptr(Pt{x - 2.54, yv + 6.35}), RefJust: "right", ValJust: "right"})
	b.Wire(Pt{x, yv}, ce.Pin("1"))
	b.Wire(ce.Pin("2"), Pt{x, yg})
	b.Wire(Pt{x, yg}, Pt{x, yg + 1.27})
	b.gnd(Pt{x, yg + 1.27})
	xs := x + 11.43 // последний фильтр — левее столбца конденсаторов клавиатуры (нумерация по столбцам)
	n := 10         // по конденсатору на корпус DD
	if b.v.Decoder {
		n = 11 // + дешифратор
	}
	for i := 0; i < n; i++ {
		cx := xs + float64(i)*8.89
		c := b.Sym("C", b.next("C"), b.v.Filter, Pt{cx, (yv + yg) / 2}, SymOpt{
			RefAt: ptr(Pt{cx + 2.54, yv + 3.81}), ValAt: ptr(Pt{cx + 2.54, yv + 6.35}), RefJust: "left", ValJust: "left"})
		b.filterCs = append(b.filterCs, c)
		b.Wire(Pt{cx, yv}, c.Pin("1"))
		b.Wire(c.Pin("2"), Pt{cx, yg})
	}
	last := xs + float64(n-1)*8.89
	b.Wire(Pt{x, yv}, Pt{last, yv})
	b.Wire(Pt{x, yg}, Pt{last, yg})
	xc := b.conn("xsPwr", "pwr", "CONN_PWR", "XS1", Pt{175.26, yv})
	b.Wire(Pt{last, yv}, cp(xc, 1))
	b.Wire(Pt{last, yg}, Pt{168.91, yg}, Pt{168.91, cp(xc, 2).Y}, cp(xc, 2))
	b.moveSince(mark, 0, b.Sheet.J.PwrDY)

	b.placeConnXY()
}

// ---------------------------------------------------------------- примечание над штампом

func (b *builder) notes() {
	andUnused := "3, 4, 5, 9, 10, 11"
	if b.v.Rows == 4 {
		andUnused = "9, 10, 12, 13"
	}
	nor, and, or := b.Roles["norY2"].Ref, b.Roles["and"].Ref, b.Roles["or"].Ref
	lines := []string{
		"Примечание",
		fmt.Sprintf("1. %s подключить в непосредственной близости от выводов питания микросхем DD1–DD%d.", refRanges(b.filterCs), len(b.filterCs)),
		fmt.Sprintf("2. %s — вывод 7 на GND, вывод 14 к +5 В.", joinRefs(nor, and, or)),
		fmt.Sprintf("3. Неиспользуемые входы к GND: %s выв. 8, 9, 11, 12; %s выв. %s; %s выв. 4, 5, 9, 10, 12, 13.", nor, and, andUnused, or),
		fmt.Sprintf("4. %s, %s устанавливать рядом с ZQ1.", b.Roles["cX1"].Ref, b.Roles["cX2"].Ref),
	}
	b.Text(strings.Join(lines, "\n"), Pt{229.87 + b.Sheet.J.NoteDX, 206.375 + b.Sheet.J.NoteDY}, b.Sheet.J.NoteFont)
}

// joinRefs — «DD3, DD6, DD10» в порядке номеров.
func joinRefs(refs ...string) string {
	sort.Slice(refs, func(i, j int) bool { return refNum(refs[i]) < refNum(refs[j]) })
	return strings.Join(refs, ", ")
}

// refRanges — «C5–C10, C14–C17»: подряд идущие номера сворачиваются в диапазон.
func refRanges(cs []*Comp) string {
	var nums []int
	prefix := ""
	for _, c := range cs {
		prefix = refPrefix(c.Ref)
		nums = append(nums, refNum(c.Ref))
	}
	sort.Ints(nums)
	var parts []string
	for i := 0; i < len(nums); {
		j := i
		for j+1 < len(nums) && nums[j+1] == nums[j]+1 {
			j++
		}
		switch {
		case j == i:
			parts = append(parts, fmt.Sprintf("%s%d", prefix, nums[i]))
		case j == i+1:
			parts = append(parts, fmt.Sprintf("%s%d, %s%d", prefix, nums[i], prefix, nums[j]))
		default:
			parts = append(parts, fmt.Sprintf("%s%d–%s%d", prefix, nums[i], prefix, nums[j]))
		}
		i = j + 1
	}
	return strings.Join(parts, ", ")
}

// ---------------------------------------------------------------- дешифратор CS (ТЗ-2026, рис. 6)

func (b *builder) decoder() {
	at := Pt{330.2, 66.04}
	u := b.role("dec", b.Sym("74HC138", "DD11", "74HC138", at, b.icLabels(Pt{at.X, at.Y - 15.24})))
	b.vcc(u.Pin("16"))
	b.gnd(u.Pin("8"))
	// адрес A13..A15 → A0..A2
	for i, n := range []string{"1", "2", "3"} {
		p := u.Pin(n)
		e := Pt{p.X - 12.7, p.Y}
		b.Wire(e, p)
		b.Label(fmt.Sprintf("A%d", 13+i), e, false)
	}
	// Ē0, Ē1 → GND; E2 ← CS_EN
	e0, e1 := u.Pin("4"), u.Pin("5")
	xg := e0.X - 2.54
	b.Wire(e0, Pt{xg, e0.Y}, Pt{xg, e1.Y}, e1)
	b.Power("GND", Pt{xg, e0.Y}, 270)
	e2 := u.Pin("6")
	b.Wire(Pt{e2.X - 12.7, e2.Y}, e2)
	b.Label("CS_EN", Pt{e2.X - 12.7, e2.Y}, false)
	// выходы Yn → CS по варианту
	yPin := []string{"15", "14", "13", "12", "11", "10", "9", "7"}
	cs := map[string]string{b.v.CS.Buf: nCSbuf, b.v.CS.Y2: nCSy2, b.v.CS.Ind: nCSind, b.v.CS.Kb: nCSkb}
	for n := 0; n < 8; n++ {
		p := u.Pin(yPin[n])
		name, ok := cs[fmt.Sprintf("Y%d", n)]
		if !ok {
			b.nc(p)
			continue
		}
		e := Pt{p.X + 12.7, p.Y}
		b.Wire(p, e)
		b.Label(name, e, true)
	}
}

// ---------------------------------------------------------------- шины

func (b *builder) busNames() {
	st := b.v.Style
	t := func(s string, at Pt) { b.Text(s, at, 2.5) }
	switch {
	case st.BusNames: // ГОСТ: шины Bx
		t("B1", Pt{xBM + 1.27, yTop + 1.27})
		t("B1", Pt{xBD + 1.27, yH2 + 1.27})
		t("B2", Pt{163.83, 128.27})
		t("B3", Pt{88.9, kbColY - 3.81})
		t("B4", Pt{259.08 + 1.27, 148.59})
		t("B5", Pt{281.94 + 1.27, 148.59})
	case st.MainBus != "":
		t(st.MainBus, Pt{xBM + 1.27, yTop + 1.27})
	}
}

func (b *builder) buses() {
	b.busNames()
	b.Bus(Pt{xBL, 101.6}, Pt{xBL, yTop}, Pt{xBR, yTop}, Pt{xBR, 121.92})
	b.Bus(Pt{xBM, yTop}, Pt{xBM, yH2})
	b.Bus(Pt{xBI, yTop}, Pt{xBI, 121.92})
	b.Bus(Pt{xBM, yH1}, Pt{xBK, yH1}, Pt{xBK, 177.8})
	b.Bus(Pt{xBM, yH2}, Pt{xBD, yH2}, Pt{xBD, 218.44})
}
