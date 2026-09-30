package main

import (
	"strings"

	"mpskp/internal/schgen"
)

// Варианты символов «по ГОСТ 2.743» для стилей C/D: выводы (номера и точки подключения) те же,
// что у исходного символа — раскладка листа общая для всех стилей; меняется корпус:
// основное поле с функцией (RG, BUF, DC, RAM, MPU) и поля меток, метки выводов по-отечественному.
type gostIC struct {
	base, title string
	field       float64           // ширина полей меток слева/справа (0 — без полей)
	rename      map[string]string // метки выводов
}

var gostICs = []gostIC{
	{"74HC573", "RG", 5.08, map[string]string{"Load": "C", "OE": "EZ"}},
	{"74HC173", "RG", 5.08, map[string]string{"Cp": "C", "Mr": "R", "Oe1": "EZ1", "Oe2": "EZ2"}},
	{"74HC244", "BUF", 5.08, map[string]string{"1OE": "EZ1", "2OE": "EZ2"}},
	{"74HC138", "DC", 3.81, map[string]string{"A0": "1", "A1": "2", "A2": "4", "~{E0}": "&", "~{E1}": "", "E2": "",
		"~{Y0}": "0", "~{Y1}": "1", "~{Y2}": "2", "~{Y3}": "3", "~{Y4}": "4", "~{Y5}": "5", "~{Y6}": "6", "~{Y7}": "7"}},
	{"IDT7005", "RAM", 7.62, nil},
	{"AT89S53", "MPU", 10.16, map[string]string{"~{EA}/VPP": "~{EA}"}},
}

type gostGate struct {
	base, title string
	invOut      bool
}

var gostGates = []gostGate{
	{"74HC02", "1", true},
	{"74HC32", "1", false},
	{"74HC11", "&", false},
	{"74HC21", "&", false},
}

func stroke() *schgen.Node {
	return schgen.L("stroke", schgen.L("width", schgen.F(0.254)), schgen.L("type", schgen.A("default")))
}
func nofill() *schgen.Node { return schgen.L("fill", schgen.L("type", schgen.A("none"))) }

func rectN(x0, y0, x1, y1 float64) *schgen.Node {
	return schgen.L("rectangle", schgen.L("start", schgen.F(x0), schgen.F(y0)), schgen.L("end", schgen.F(x1), schgen.F(y1)), stroke(), nofill())
}
func lineN(x0, y0, x1, y1 float64) *schgen.Node {
	return schgen.L("polyline", schgen.L("pts", schgen.L("xy", schgen.F(x0), schgen.F(y0)), schgen.L("xy", schgen.F(x1), schgen.F(y1))), stroke(), nofill())
}
func textN(s string, x, y, size float64) *schgen.Node {
	return schgen.L("text", schgen.Q(s), schgen.L("at", schgen.F(x), schgen.F(y), schgen.F(0)),
		schgen.L("effects", schgen.L("font", schgen.L("size", schgen.F(size), schgen.F(size)))))
}

var graphics = map[string]bool{"rectangle": true, "polyline": true, "arc": true, "circle": true, "text": true, "bezier": true}

// stripGraphics убирает графику и альтернативный стиль De Morgan (_u_2).
func stripGraphics(sym *schgen.Node) {
	sym.Remove(func(k *schgen.Node) bool {
		return k.Head() == "symbol" && strings.HasSuffix(k.Arg(0), "_2")
	})
	for _, sub := range sym.All("symbol") {
		sub.Remove(func(k *schgen.Node) bool { return graphics[k.Head()] })
	}
}

func subOf(sym *schgen.Node, suffix string) *schgen.Node {
	name := sym.Arg(0) + "_" + suffix
	for _, s := range sym.All("symbol") {
		if s.Arg(0) == name {
			return s
		}
	}
	s := schgen.L("symbol", schgen.Q(name))
	// подсимволы должны идти до embedded_fonts
	sym.Kids = append(sym.Kids, s)
	for i, k := range sym.Kids {
		if k.Head() == "embedded_fonts" {
			sym.Kids = append(append(sym.Kids[:i:i], sym.Kids[i+1:]...), k)
			break
		}
	}
	return s
}

func gostifyIC(sym *schgen.Node, g gostIC) {
	// габарит корпуса по вертикали — по выводам питания/крайним выводам; по горизонтали — точки подключения ± 2,54
	var minX, maxX, minY, maxY float64
	first := true
	for _, p := range schgen.Pins(sym) {
		if p.Hidden {
			continue
		}
		if first {
			minX, maxX, minY, maxY = p.X, p.X, p.Y, p.Y
			first = false
		}
		minX, maxX = min(minX, p.X), max(maxX, p.X)
		minY, maxY = min(minY, p.Y), max(maxY, p.Y)
	}
	stripGraphics(sym)
	const pl = 2.54
	left, right := minX+pl, maxX-pl
	top, bot := maxY-pl, minY+pl // выводы питания сверху/снизу — тоже укорачиваем до 2,54
	for _, sub := range sym.All("symbol") {
		for _, pin := range sub.All("pin") {
			pin.Find("length").Kids[1] = schgen.F(pl)
			if nm := pin.Find("name"); nm != nil {
				if r, ok := g.rename[nm.Arg(0)]; ok {
					nm.Kids[1] = schgen.Q(r)
				}
				// выводы питания на ГОСТ-УГО без имён — только номера (иначе налезают на основное поле)
				if pin.Arg(0) == "power_in" && pin.Find("hide") == nil {
					nm.Kids[1] = schgen.Q("")
				}
			}
		}
	}
	body := subOf(sym, "0_1")
	body.Kids = append(body.Kids, rectN(left, top, right, bot))
	if g.field > 0 {
		body.Kids = append(body.Kids, lineN(left+g.field, top, left+g.field, bot), lineN(right-g.field, top, right-g.field, bot))
	}
	body.Kids = append(body.Kids, textN(g.title, (left+right)/2, top-2.54, 2.5))
}

func gostifyGate(sym *schgen.Node, g gostGate) {
	stripGraphics(sym)
	const pl = 2.54
	for _, sub := range sym.All("symbol") {
		pins := sub.All("pin")
		if len(pins) == 0 || strings.HasSuffix(sub.Arg(0), "_0_0") {
			continue
		}
		for _, pin := range pins {
			pin.Find("length").Kids[1] = schgen.F(pl)
			if g.invOut && pin.Arg(0) == "output" {
				pin.Kids[2] = schgen.A("inverted")
			}
		}
		sub.Kids = append(sub.Kids, rectN(-5.08, 5.08, 5.08, -5.08), textN(g.title, 0, 3.175, 2))
	}
}
