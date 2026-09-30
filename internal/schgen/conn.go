package schgen

import "fmt"

// Размеры разъёма-таблицы «Сигнал | Вывод» (как в принятой схеме).
const (
	connRow  = 3.81 // высота строки
	connSigW = 12.7 // ширина графы «Сигнал»
	connNumW = 7.62 // ширина графы «Вывод»
	connPin  = 2.54 // длина вывода слева
	connFont = 1.27
)

// ConnPin — строка разъёма.
type ConnPin struct {
	Signal string // текст в графе «Сигнал» (можно с ~{…})
	Power  bool   // вывод питания разъёма (power_out: цепь запитана отсюда, PWR_FLAG не нужен)
}

// AddConn регистрирует в библиотеке листа символ-таблицу name с выводами 1..len(rows).
// Точка подключения вывода i: (0, (i-1)*connRow) от точки привязки, вход слева.
func (l *Lib) AddConn(name string, rows []ConnPin) {
	n := len(rows)
	x0 := connPin
	top := connRow * 1.5 // верх шапки над центром первой строки
	bottom := -(float64(n-1)*connRow + connRow/2)
	stroke := L("stroke", L("width", F(0)), L("type", A("default")))
	nofill := L("fill", L("type", A("none")))
	gfx := L("symbol", Q(name+"_0_1"),
		L("rectangle", L("start", F(x0), F(top)), L("end", F(x0+connSigW+connNumW), F(bottom)), stroke.Clone(), nofill.Clone()))
	line := func(x1, y1, x2, y2 float64) {
		gfx.Kids = append(gfx.Kids, L("polyline",
			L("pts", L("xy", F(x1), F(y1)), L("xy", F(x2), F(y2))), stroke.Clone(), nofill.Clone()))
	}
	text := func(s string, x, y float64, just string) {
		eff := L("effects", L("font", L("size", F(connFont), F(connFont))))
		if just != "" {
			eff.Kids = append(eff.Kids, L("justify", A(just)))
		}
		gfx.Kids = append(gfx.Kids, L("text", Q(s), L("at", F(x), F(y), F(0)), eff))
	}
	line(x0+connSigW, top, x0+connSigW, bottom)
	for i := 0; i <= n-1; i++ {
		y := connRow/2 - float64(i)*connRow
		line(x0, y, x0+connSigW+connNumW, y)
	}
	text("Сигнал", x0+connSigW/2, connRow, "")
	text("Вывод", x0+connSigW+connNumW/2, connRow, "")
	pins := L("symbol", Q(name+"_1_1"))
	for i, r := range rows {
		y := -float64(i) * connRow
		text(r.Signal, x0+0.635, y, "left")
		text(fmt.Sprint(i+1), x0+connSigW+connNumW/2, y, "")
		typ := "passive"
		if r.Power {
			typ = "power_out"
		}
		pins.Kids = append(pins.Kids, L("pin", A(typ), A("line"),
			L("at", F(0), F(y), F(0)), L("length", F(connPin)),
			L("name", Q(r.Signal), L("effects", L("font", L("size", F(connFont), F(connFont))))),
			L("number", Q(fmt.Sprint(i+1)), L("effects", L("font", L("size", F(connFont), F(connFont)))))))
	}
	prop := func(k, v string, x, y float64, hide bool) *Node {
		p := L("property", Q(k), Q(v), L("at", F(x), F(y), F(0)))
		if hide {
			p.Kids = append(p.Kids, L("hide", A("yes")))
		}
		p.Kids = append(p.Kids, L("effects", L("font", L("size", F(1.27), F(1.27)))))
		return p
	}
	sym := L("symbol", Q(name),
		L("pin_numbers", L("hide", A("yes"))),
		L("pin_names", L("offset", F(0)), L("hide", A("yes"))),
		L("exclude_from_sim", A("no")), L("in_bom", A("yes")), L("on_board", A("yes")),
		prop("Reference", "XS", x0+(connSigW+connNumW)/2, top+1.27, false),
		prop("Value", name, x0, bottom-1.27, true),
		prop("Footprint", "", 0, 0, true),
		prop("Datasheet", "", 0, 0, true),
		prop("Description", "Разъём", 0, 0, true),
		gfx, pins,
	)
	l.Syms[name] = sym
}
