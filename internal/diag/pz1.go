package diag

import (
	"fmt"
	"image"
)

// Structural — структурная схема МПС по рис. 2 ТЗ: MPM, RAM, ROM сверху, IOU, CPAN, IC снизу, общая шина, X и Y.
func Structural(fontTTF []byte, fontMM, stroke float64) (image.Image, error) {
	c, err := New(160, 72, fontTTF, fontMM, stroke)
	if err != nil {
		return nil, err
	}
	const bw, bh = 30, 14
	top, bot := []string{"MPM", "RAM", "ROM"}, []string{"IOU", "CPAN", "IC"}
	xs := []float64{22, 65, 108}
	busY := 36.0
	c.Thick(1.6, 12, busY, 150, busY)
	c.Text("Общая шина", 150, busY-3, 1, 0)
	for i, x := range xs {
		c.Box(x, 6, bw, bh, top[i])
		c.Line(false, x+bw/2, 6+bh, x+bw/2, busY)
		c.Box(x, 52, bw, bh, bot[i])
		c.Line(false, x+bw/2, busY, x+bw/2, 52)
	}
	// X и Y — у IOU
	c.Line(true, 2, 55, 22, 55)
	c.Text("X", 4, 52, 0, 0)
	c.Line(true, 22, 63, 2, 63)
	c.Text("Y", 4, 66.5, 0, 0)
	return c.Image(), nil
}

// FuncInfo — подписи функциональной схемы (обозначения по схеме Э3 и вариант).
type FuncInfo struct {
	MCU, Latch, Dec, Buf, KbCol, KbRow, IndReg, Y2Reg string // «DD1 AT89S53» и т.п.
	CSBuf, CSKb, CSInd, CSY2                          string // Yn
	Keyboard, Y1Pin, Y2Pin, CSEn                      string
}

// Functional — функциональная схема: МК, защёлка адреса, дешифратор, буфер, пульт (клавиатура, индикатор), регистр Y2.
func Functional(fi FuncInfo, fontTTF []byte, fontMM, stroke float64) (image.Image, error) {
	c, err := New(170, 130, fontTTF, fontMM*0.9, stroke)
	if err != nil {
		return nil, err
	}
	// МК слева
	c.Box(4, 18, 34, 92, "Микроконтроллер\n"+fi.MCU+"\n\nINT0 ← клавиатура\nINT1 ← строб X2")
	// шины
	busD, busA := 30.0, 44.0
	c.Thick(1.2, 38, busD, 166, busD)
	c.Text("AD0–AD7 (данные / младший адрес)", 100, busD-3, 0.5, 0)
	c.Box(46, 38, 26, 14, "Защёлка\nадреса\n"+fi.Latch)
	c.Line(true, 59, busD, 59, 38)
	c.Line(true, 38, 48, 46, 48) // ALE
	c.Text("ALE", 40, 46, 0, fontMM*0.75)
	c.Thick(1.2, 72, busA, 166, busA)
	c.Text("A0–A12", 150, busA-3, 0.5, 0)
	// дешифратор
	c.Box(46, 60, 26, 20, "Дешифратор\n"+fi.Dec)
	c.Line(true, 38, 66, 46, 66)
	c.Text("A13–A15", 40, 64, 0, fontMM*0.7)
	c.Line(true, 38, 76, 46, 76)
	c.Text(fi.CSEn, 40, 74, 0, fontMM*0.7)
	// устройства
	type dev struct {
		x                  float64
		title, cs, special string
	}
	devs := []dev{{80, "Кольцевой буфер\n" + fi.Buf, fi.CSBuf, ""}, {108, "Клавиатура\n" + fi.Keyboard + "\n" + fi.KbCol + ", " + fi.KbRow, fi.CSKb, "INT0"},
		{136, "Индикатор\n" + fi.IndReg, fi.CSInd, ""}}
	csY := 92.0
	for _, d := range devs {
		c.Box(d.x, 56, 26, 26, d.title)
		c.Line(true, d.x+8, busD, d.x+8, 56)
		c.Line(true, d.x+18, busA, d.x+18, 56)
		c.Line(true, d.x+13, csY, d.x+13, 82)
		c.Text(d.cs, d.x+14, 86, 0, fontMM*0.7)
	}
	c.Line(false, 72, 70, 76, 70, 76, csY, 150, csY)
	c.Text("CS (Y0–Y7)", 77, csY+3, 0, fontMM*0.7)
	// регистр Y2 — ниже шины CS
	c.Box(80, 100, 30, 14, "Регистр Y2\n"+fi.Y2Reg)
	c.Line(true, 76, csY, 76, 107, 80, 107)
	c.Text(fi.CSY2, 70, 104, 0, fontMM*0.7)
	c.Line(true, 110, 107, 124, 107)
	c.Text("Y2", 126, 107, 0, 0)
	c.Line(true, 38, 104, 62, 104, 62, 119, 124, 119)
	c.Text("строб Y2 ("+fi.Y2Pin+")", 126, 119, 0, fontMM*0.75)
	c.Line(true, 38, 108, 56, 108, 56, 125, 124, 125)
	c.Text("строб Y1 ("+fi.Y1Pin+")", 126, 125, 0, fontMM*0.75)
	// X2 от внешнего устройства — во второй порт буфера
	c.Line(true, 101, 98, 101, 82)
	c.Text("X2", 102.5, 88, 0, fontMM*0.75)
	return c.Image(), nil
}

// Timing — временные диаграммы цикла movx при 12 МГц: а) чтение, б) запись. Параметры — из даташита AT89S53 (нс).
type Timing struct {
	TAVLL, TLLAX, TLLWL, TRLRH, TRLDV, TRHDX, TQVWH, TWHQX, TWLWH float64
	MemRead, MemWrite                                             string // подписи сигналов памяти в скобках, напр. «(OEL)»
}

func (t Timing) draw(c *Canvas, y0 float64, write bool) {
	// шкала времени: 1 нс = 0.12 мм
	k := 0.12
	x0 := 34.0
	ale := []float64{0, 120}                                    // ALE высокий до 120 нс
	strobe := []float64{120 + t.TLLWL, 120 + t.TLLWL + t.TRLRH} // RD/WR низкий
	end := strobe[1] + 100
	X := func(ns float64) float64 { return x0 + ns*k }
	rows := []string{"ALE", "P2", "P0", "RD"}
	if write {
		rows[3] = "WR"
	}
	h := 7.0
	for i, r := range rows {
		y := y0 + float64(i)*14
		if r == "RD" || r == "WR" {
			c.Overbar(r, 2, y+h/2, 0)
			mem := map[bool]string{true: t.MemWrite, false: t.MemRead}[write]
			if mem != "" {
				c.Text("(", 3+c.Width(r), y+h/2, 0, 0)
				c.Overbar(mem, 3+c.Width(r)+c.Width("("), y+h/2, 0)
				c.Text(")", 3+c.Width(r)+c.Width("(")+c.Width(mem), y+h/2, 0, 0)
			}
		} else {
			c.Text(r, 2, y+h/2, 0, 0)
		}
		switch r {
		case "ALE":
			c.Line(false, X(-40), y+h, X(ale[0]), y+h, X(ale[0])+1, y, X(ale[1]), y, X(ale[1])+1, y+h, X(end), y+h)
		case "P2":
			bus(c, X(-40), X(-40)+1, X(end)-1, X(end), y, h, "A8–A15")
		case "P0":
			bus(c, X(-40), X(-40)+1, X(ale[1]+t.TLLAX), X(ale[1]+t.TLLAX)+1, y, h, "A0–A7")
			if write {
				dv := strobe[0] - 20
				bus(c, X(dv), X(dv)+1, X(strobe[1]+t.TWHQX), X(strobe[1]+t.TWHQX)+1, y, h, "данные МК")
			} else {
				dv := strobe[0] + t.TRLDV
				bus(c, X(dv), X(dv)+1, X(strobe[1]+t.TRHDX+40), X(strobe[1]+t.TRHDX+40)+1, y, h, "данные ОЗУ")
			}
		default:
			c.Line(false, X(-40), y, X(strobe[0]), y, X(strobe[0])+1, y+h, X(strobe[1]), y+h, X(strobe[1])+1, y, X(end), y)
		}
	}
	// размеры
	yd := y0 + 4*14 + 3
	c.Dim(X(ale[1])-X(0)+X(0), X(ale[1]+t.TLLAX), y0+28-3, fmt.Sprintf("tLLAX ≥ %.0f", t.TLLAX))
	c.Dim(X(ale[1]), X(strobe[0]), yd, fmt.Sprintf("tLLWL %.0f…%.0f", t.TLLWL, t.TLLWL+100))
	if write {
		c.Dim(X(strobe[0]), X(strobe[1]), yd+6, fmt.Sprintf("tWLWH ≥ %.0f", t.TWLWH))
		c.Dim(X(strobe[1]-t.TQVWH), X(strobe[1]), y0+28+h+2.5, fmt.Sprintf("tQVWH ≥ %.0f", t.TQVWH))
		c.Dim(X(strobe[1]), X(strobe[1]+t.TWHQX), yd, fmt.Sprintf("tWHQX ≥ %.0f", t.TWHQX))
	} else {
		c.Dim(X(strobe[0]), X(strobe[1]), yd+6, fmt.Sprintf("tRLRH ≥ %.0f", t.TRLRH))
		c.Dim(X(strobe[0]), X(strobe[0]+t.TRLDV), y0+28+h+2.5, fmt.Sprintf("tRLDV ≤ %.0f", t.TRLDV))
	}
	c.Text("время в нс, 12 МГц", X(end), yd+12, 1, c.fontMM*0.75)
}

// bus — шина: «ромбики» перехода в xa…xb и xc…xd, текст внутри.
func bus(c *Canvas, xa, xb, xc, xd, y, h float64, label string) {
	c.Line(false, xa, y+h/2, xb, y, xc, y, xd, y+h/2, xc, y+h, xb, y+h, xa, y+h/2)
	c.Text(label, (xb+xc)/2, y+h/2, 0.5, c.fontMM*0.8)
}

// TimingDiagrams — два рисунка: чтение и запись.
func TimingDiagrams(t Timing, fontTTF []byte, fontMM, stroke float64) (read, write image.Image, err error) {
	for i, w := range []bool{false, true} {
		c, e := New(165, 84, fontTTF, fontMM, stroke)
		if e != nil {
			return nil, nil, e
		}
		t.draw(c, 4, w)
		if i == 0 {
			read = c.Image()
		} else {
			write = c.Image()
		}
	}
	return read, write, nil
}
