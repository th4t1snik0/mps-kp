// Package diag — простые чертежи для ПЗ1 (структурная и функциональная схемы, временные диаграммы) в PNG.
// Координаты — мм, начало в левом верхнем углу; шрифт — как у схем алгоритмов студента (flow.Style).
package diag

import (
	"image"
	"math"
	"strings"

	"github.com/fogleman/gg"
	"github.com/golang/freetype/truetype"
	"golang.org/x/image/font"
)

// Canvas — чертёж.
type Canvas struct {
	dc     *gg.Context
	s      float64 // px на мм
	fontMM float64
	face   font.Face
	fnt    *truetype.Font
	stroke float64
}

// New — чертёж w×h мм. fontTTF — байты TTF, fontMM — высота шрифта.
func New(w, h float64, fontTTF []byte, fontMM, stroke float64) (*Canvas, error) {
	f, err := truetype.Parse(fontTTF)
	if err != nil {
		return nil, err
	}
	c := &Canvas{s: 8, fontMM: fontMM, fnt: f, stroke: stroke}
	c.dc = gg.NewContext(int(w*c.s), int(h*c.s))
	c.dc.SetRGB(1, 1, 1)
	c.dc.Clear()
	c.dc.SetRGB(0, 0, 0)
	c.setFont(fontMM)
	c.dc.SetLineWidth(stroke * c.s)
	c.dc.SetLineCap(gg.LineCapButt)
	return c, nil
}

func (c *Canvas) setFont(mm float64) {
	c.face = truetype.NewFace(c.fnt, &truetype.Options{Size: mm * c.s, DPI: 72, Hinting: font.HintingFull})
	c.dc.SetFontFace(c.face)
}

// Image — результат.
func (c *Canvas) Image() image.Image { return c.dc.Image() }

// Width — ширина текста, мм.
func (c *Canvas) Width(s string) float64 { w, _ := c.dc.MeasureString(s); return w / c.s }

// Line — ломаная; arrow — наконечник в конце.
func (c *Canvas) Line(arrow bool, pts ...float64) {
	c.dc.SetLineWidth(c.stroke * c.s)
	c.dc.MoveTo(pts[0]*c.s, pts[1]*c.s)
	for i := 2; i < len(pts); i += 2 {
		c.dc.LineTo(pts[i]*c.s, pts[i+1]*c.s)
	}
	c.dc.Stroke()
	if arrow && len(pts) >= 4 {
		n := len(pts)
		x, y, px, py := pts[n-2], pts[n-1], pts[n-4], pts[n-3]
		dx, dy := x-px, y-py
		l := math.Hypot(dx, dy)
		c.Arrow(x, y, dx/l, dy/l)
	}
}

// Thick — толстая линия (шина).
func (c *Canvas) Thick(w float64, pts ...float64) {
	c.dc.SetLineWidth(w * c.s)
	c.dc.MoveTo(pts[0]*c.s, pts[1]*c.s)
	for i := 2; i < len(pts); i += 2 {
		c.dc.LineTo(pts[i]*c.s, pts[i+1]*c.s)
	}
	c.dc.Stroke()
	c.dc.SetLineWidth(c.stroke * c.s)
}

// Dashed — пунктир.
func (c *Canvas) Dashed(pts ...float64) {
	c.dc.SetDash(1.5*c.s, 1*c.s)
	c.Line(false, pts...)
	c.dc.SetDash()
}

// Arrow — наконечник в (x, y) по направлению (dx, dy).
func (c *Canvas) Arrow(x, y, dx, dy float64) {
	const L, W = 2.5, 0.9
	px, py := -dy, dx
	c.dc.MoveTo(x*c.s, y*c.s)
	c.dc.LineTo((x-dx*L+px*W)*c.s, (y-dy*L+py*W)*c.s)
	c.dc.LineTo((x-dx*L-px*W)*c.s, (y-dy*L-py*W)*c.s)
	c.dc.ClosePath()
	c.dc.Fill()
}

// Box — прямоугольник с текстом по центру («\n» — перенос).
func (c *Canvas) Box(x, y, w, h float64, text string) {
	c.dc.SetLineWidth(c.stroke * c.s)
	c.dc.DrawRectangle(x*c.s, y*c.s, w*c.s, h*c.s)
	c.dc.Stroke()
	lines := strings.Split(text, "\n")
	lh := c.fontMM * 1.25
	top := y + h/2 - float64(len(lines))*lh/2
	for i, l := range lines {
		c.dc.DrawStringAnchored(l, (x+w/2)*c.s, (top+(float64(i)+0.5)*lh)*c.s, 0.5, 0.35)
	}
}

// Text — надпись; ax: 0 — слева от точки, 0.5 — по центру, 1 — справа; size — мм (0 — обычный).
func (c *Canvas) Text(s string, x, y, ax, size float64) {
	if size > 0 && size != c.fontMM {
		c.setFont(size)
		defer c.setFont(c.fontMM)
	}
	c.dc.DrawStringAnchored(s, x*c.s, y*c.s, ax, 0.35)
}

// Overbar — черта над текстом (инверсный сигнал: RD, WR).
func (c *Canvas) Overbar(s string, x, y, ax float64) {
	w := c.Width(s)
	x0 := x - w*ax
	c.Text(s, x, y, ax, 0)
	c.dc.SetLineWidth(0.2 * c.s)
	c.dc.DrawLine(x0*c.s, (y-c.fontMM*0.62)*c.s, (x0+w)*c.s, (y-c.fontMM*0.62)*c.s)
	c.dc.Stroke()
	c.dc.SetLineWidth(c.stroke * c.s)
}

// Dim — размерная линия со стрелками в обе стороны между x1 и x2 на высоте y, подпись над ней.
func (c *Canvas) Dim(x1, x2, y float64, label string) {
	c.dc.SetLineWidth(0.18 * c.s)
	c.dc.DrawLine(x1*c.s, y*c.s, x2*c.s, y*c.s)
	c.dc.Stroke()
	const L, W = 1.6, 0.55
	for _, e := range []struct{ x, d float64 }{{x1, -1}, {x2, 1}} {
		c.dc.MoveTo(e.x*c.s, y*c.s)
		c.dc.LineTo((e.x-e.d*L)*c.s, (y-W)*c.s)
		c.dc.LineTo((e.x-e.d*L)*c.s, (y+W)*c.s)
		c.dc.ClosePath()
		c.dc.Fill()
	}
	c.Text(label, (x1+x2)/2, y-1.6, 0.5, c.fontMM*0.8)
	c.dc.SetLineWidth(c.stroke * c.s)
}
