package flow

import (
	"fmt"
	"image"
	"math"
	"strings"

	"github.com/fogleman/gg"
	"github.com/golang/freetype/truetype"
	"golang.org/x/image/font"
)

// Размеры в мм (ГОСТ 19.701: a — высота, b = 2a допускается). Масштаб — px на мм.
const (
	loopH    = 12.0 // высота границы цикла
	sideGap  = 8.0  // горизонтальный промежуток между ветвями
	padMM    = 2.0  // поле текста внутри символа
	marginMM = 4.0  // поле рисунка
)

// Options — отрисовка.
type Options struct {
	Style     Style   // вид (шрифт, размеры); нулевой — Default
	Font      []byte  // TTF — файл Style.Font из fonts/
	Scale     float64 // px на мм; 0 — 8 (≈ 200 dpi)
	MaxHeight float64 // мм; выше — перенос в следующую колонку через соединители; 0 — 220 (лист А4 с подписью)
}

const (
	connD  = 6.0  // диаметр соединителя
	colGap = 12.0 // промежуток между колонками
)

type box struct{ l, r, h float64 } // ширина слева и справа от оси, высота

type renderer struct {
	st     Style
	lineMM float64
	dc     *gg.Context
	s      float64
	face   font.Face
	sizes  map[*Node]box
	texts  map[*Node][]string
}

// Render рисует схему в PNG-изображение.
func Render(c *Chart, o Options) (image.Image, error) {
	if o.Scale == 0 {
		o.Scale = 8
	}
	if o.Style.Font == "" {
		o.Style = Default
	}
	f, err := truetype.Parse(o.Font)
	if err != nil {
		return nil, fmt.Errorf("шрифт: %w", err)
	}
	// размер шрифта в пунктах так, чтобы высота была r.st.FontMM при данном масштабе (72 pt = 1 дюйм = DPI px)
	face := truetype.NewFace(f, &truetype.Options{Size: o.Style.FontMM * o.Scale, DPI: 72, Hinting: font.HintingFull})
	r := &renderer{st: o.Style, lineMM: o.Style.FontMM * 1.26, dc: gg.NewContext(1, 1), s: o.Scale, face: face, sizes: map[*Node]box{}, texts: map[*Node][]string{}}
	r.dc.SetFontFace(face)
	if o.MaxHeight == 0 {
		o.MaxHeight = 220
	}
	// колонки: циклы верхнего уровня разворачиваются в «начало — тело — конец», режется между символами;
	// число колонок — минимальное, высоты выравниваются
	flat := flatten(c.Nodes)
	total := r.seq(flat).h
	k := math.Ceil(total / o.MaxHeight)
	target := total/k + r.st.Gap
	var cols [][]*Node
	var cur []*Node
	curH := 0.0
	for _, n := range flat {
		h := r.size(n).h
		// разрыв — перед символом, который выводит колонку за среднюю высоту (колонок ровно k)
		if len(cur) > 0 && curH+r.st.Gap+h/2 > target && len(cols) < int(k)-1 {
			cols = append(cols, cur)
			cur, curH = nil, connD+r.st.Gap
		}
		if len(cur) > 0 {
			curH += r.st.Gap
		}
		cur = append(cur, n)
		curH += h
	}
	cols = append(cols, cur)
	type col struct {
		b      box
		top    float64 // смещение первого символа (соединитель сверху)
		height float64
	}
	var cs []col
	W, H := marginMM, 0.0
	for i, ns := range cols {
		b := r.seq(ns)
		cl := col{b: b}
		if i > 0 {
			cl.top = connD + r.st.Gap
		}
		cl.height = cl.top + b.h
		if i < len(cols)-1 {
			cl.height += r.st.Gap + connD
		}
		b.l, b.r = math.Max(b.l, connD/2), math.Max(b.r, connD/2)
		cl.b = b
		cs = append(cs, cl)
		W += b.l + b.r
		if i > 0 {
			W += colGap
		}
		H = math.Max(H, cl.height)
	}
	W += marginMM
	r.dc = gg.NewContext(int(math.Ceil(W*o.Scale)), int(math.Ceil((H+2*marginMM)*o.Scale)))
	r.dc.SetFontFace(face)
	r.dc.SetRGB(1, 1, 1)
	r.dc.Clear()
	r.dc.SetRGB(0, 0, 0)
	r.dc.SetLineWidth(r.st.Stroke * o.Scale)
	r.dc.SetLineCap(gg.LineCapSquare)
	x := marginMM
	for i, cl := range cs {
		x += cl.b.l
		y := marginMM
		if i > 0 {
			r.connector(x, y, fmt.Sprintf("%c", 'А'+rune(i-1)))
			r.line(x, y+connD, x, y+cl.top)
		}
		r.drawSeq(cols[i], x, y+cl.top)
		if i < len(cs)-1 {
			yb := y + cl.top + cl.b.h
			r.line(x, yb, x, yb+r.st.Gap)
			r.connector(x, yb+r.st.Gap, fmt.Sprintf("%c", 'А'+rune(i)))
		}
		x += cl.b.r + colGap
	}
	return r.dc.Image(), nil
}

// flatten — циклы в последовательности → символы начала и конца цикла вокруг развёрнутого тела
// (так схему можно перенести в другую колонку и внутри цикла). Решения остаются целыми.
func flatten(ns []*Node) []*Node {
	var out []*Node
	for _, n := range ns {
		if n.Kind != Loop {
			out = append(out, n)
			continue
		}
		out = append(out, &Node{Kind: loopBegin, Text: n.Text, LoopName: n.LoopName})
		out = append(out, flatten(n.Body)...)
		out = append(out, &Node{Kind: loopEnd, LoopName: n.LoopName})
	}
	return out
}

const (
	loopBegin Kind = 100 + iota
	loopEnd
)

// ------------------------------------------------------------------ размеры

func (r *renderer) wrap(text string, width float64) []string {
	var out []string
	for _, para := range strings.Split(strings.ReplaceAll(text, `\n`, "\n"), "\n") {
		words := strings.Fields(para)
		cur := ""
		for _, w := range words {
			try := strings.TrimSpace(cur + " " + w)
			if tw, _ := r.dc.MeasureString(try); tw/r.s > width && cur != "" {
				out = append(out, cur)
				cur = w
			} else {
				cur = try
			}
		}
		out = append(out, cur)
	}
	return out
}

func roundUp5(v float64) float64 { return math.Ceil(v/5) * 5 }

func (r *renderer) size(n *Node) box {
	if b, ok := r.sizes[n]; ok {
		return b
	}
	var b box
	switch n.Kind {
	case Terminator:
		lines := r.wrap(n.Text, r.st.BlockB-2*padMM-4)
		r.texts[n] = lines
		b = box{r.st.BlockB / 2, r.st.BlockB / 2, math.Max(r.lineMM*2.3, float64(len(lines))*r.lineMM+3)}
	case Process, Predefined, Data:
		w := r.st.BlockB - 2*padMM
		if n.Kind != Process {
			w -= 6 // двойные линии / скос параллелограмма
		}
		lines := r.wrap(n.Text, w)
		r.texts[n] = lines
		b = box{r.st.BlockB / 2, r.st.BlockB / 2, math.Max(r.st.BlockA, roundUp5(float64(len(lines))*r.lineMM+2*padMM))}
	case Decision:
		// текст — во вписанном прямоугольнике ромба (половина ширины и высоты)
		dw := r.st.BlockB + 10
		lines := r.wrap(n.Text, dw/2+4)
		r.texts[n] = lines
		dh := math.Max(r.st.BlockA+5, roundUp5(2*(float64(len(lines))*r.lineMM+1)))
		yes, no := r.seq(n.Yes), r.seq(n.No)
		dx := r.sideX(n)
		h := dh + r.st.Gap + math.Max(yes.h, no.h) + r.st.Gap
		b = box{math.Max(dw/2, yes.l), math.Max(dw/2+sideGap, dx+no.r), h}
		r.sizes[n] = b
		r.texts[n] = lines
		return b
	case loopBegin:
		lines := append([]string{n.LoopName}, r.wrap(n.Text, r.st.BlockB-2*padMM-4)...)
		r.texts[n] = lines
		b = box{r.st.BlockB / 2, r.st.BlockB / 2, math.Max(loopH, float64(len(lines))*r.lineMM+4)}
	case loopEnd:
		b = box{r.st.BlockB / 2, r.st.BlockB / 2, loopH}
	case Loop:
		lines := append([]string{n.LoopName}, r.wrap(n.Text, r.st.BlockB-2*padMM-4)...)
		r.texts[n] = lines
		body := r.seq(n.Body)
		lh := math.Max(loopH, float64(len(lines))*r.lineMM+4)
		b = box{math.Max(r.st.BlockB/2, body.l), math.Max(r.st.BlockB/2, body.r), lh + r.st.Gap + body.h + r.st.Gap + loopH}
	}
	r.sizes[n] = b
	return b
}

// sideX — ось ветви «нет» справа от оси решения.
func (r *renderer) sideX(n *Node) float64 {
	dw := r.st.BlockB + 10
	yes, no := r.seq(n.Yes), r.seq(n.No)
	return math.Max(dw/2+sideGap, yes.r+sideGap+no.l)
}

func (r *renderer) decisionH(n *Node) float64 {
	return math.Max(r.st.BlockA+5, roundUp5(2*(float64(len(r.texts[n]))*r.lineMM+1)))
}

func (r *renderer) loopTopH(n *Node) float64 {
	return math.Max(loopH, float64(len(r.texts[n]))*r.lineMM+4)
}

func (r *renderer) seq(ns []*Node) box {
	var b box
	for i, n := range ns {
		s := r.size(n)
		b.l, b.r = math.Max(b.l, s.l), math.Max(b.r, s.r)
		b.h += s.h
		if i > 0 {
			b.h += r.st.Gap
		}
	}
	return b
}

// ------------------------------------------------------------------ рисование (x — ось, y — верх; мм)

func (r *renderer) line(x1, y1, x2, y2 float64) {
	r.dc.DrawLine(x1*r.s, y1*r.s, x2*r.s, y2*r.s)
	r.dc.Stroke()
}

// arrow — наконечник в точке (x, y), направление (dx, dy).
func (r *renderer) arrow(x, y, dx, dy float64) {
	const L, W = 2.5, 0.9
	px, py := -dy, dx
	r.dc.MoveTo(x*r.s, y*r.s)
	r.dc.LineTo((x-dx*L+px*W)*r.s, (y-dy*L+py*W)*r.s)
	r.dc.LineTo((x-dx*L-px*W)*r.s, (y-dy*L-py*W)*r.s)
	r.dc.ClosePath()
	r.dc.Fill()
}

func (r *renderer) text(lines []string, x, cy float64) {
	top := cy - float64(len(lines))*r.lineMM/2
	for i, l := range lines {
		r.dc.DrawStringAnchored(l, x*r.s, (top+(float64(i)+0.5)*r.lineMM)*r.s, 0.5, 0.35)
	}
}

func (r *renderer) label(s string, x, y float64, ax float64) {
	r.dc.DrawStringAnchored(s, x*r.s, y*r.s, ax, 0.35)
}

// connector — соединитель (окружность с буквой), верх в (x, y).
func (r *renderer) connector(x, y float64, name string) {
	r.dc.DrawCircle(x*r.s, (y+connD/2)*r.s, connD/2*r.s)
	r.dc.Stroke()
	r.label(name, x, y+connD/2, 0.5)
}

func (r *renderer) poly(pts ...float64) {
	r.dc.MoveTo(pts[0]*r.s, pts[1]*r.s)
	for i := 2; i < len(pts); i += 2 {
		r.dc.LineTo(pts[i]*r.s, pts[i+1]*r.s)
	}
	r.dc.ClosePath()
	r.dc.Stroke()
}

func (r *renderer) drawSeq(ns []*Node, x, y float64) {
	for i, n := range ns {
		if i > 0 {
			r.line(x, y, x, y+r.st.Gap)
			if r.st.JoinArrows {
				r.arrow(x, y+r.st.Gap, 0, 1)
			}
			y += r.st.Gap
		}
		r.draw(n, x, y)
		y += r.size(n).h
	}
}

func (r *renderer) draw(n *Node, x, y float64) {
	b := r.size(n)
	hw := r.st.BlockB / 2
	switch n.Kind {
	case Terminator:
		h := b.h
		r.dc.DrawRoundedRectangle((x-hw)*r.s, y*r.s, r.st.BlockB*r.s, h*r.s, h*r.st.TermRadius*r.s)
		r.dc.Stroke()
		r.text(r.texts[n], x, y+h/2)
	case Process:
		r.poly(x-hw, y, x+hw, y, x+hw, y+b.h, x-hw, y+b.h)
		r.text(r.texts[n], x, y+b.h/2)
	case Predefined:
		r.poly(x-hw, y, x+hw, y, x+hw, y+b.h, x-hw, y+b.h)
		r.line(x-hw+3, y, x-hw+3, y+b.h)
		r.line(x+hw-3, y, x+hw-3, y+b.h)
		r.text(r.texts[n], x, y+b.h/2)
	case Data:
		const sk = 4
		r.poly(x-hw+sk, y, x+hw, y, x+hw-sk, y+b.h, x-hw, y+b.h)
		r.text(r.texts[n], x, y+b.h/2)
	case loopBegin:
		const c = 4
		r.poly(x-hw+c, y, x+hw-c, y, x+hw, y+c, x+hw, y+b.h, x-hw, y+b.h, x-hw, y+c)
		r.text(r.texts[n], x, y+b.h/2)
	case loopEnd:
		const c = 4
		r.poly(x-hw, y, x+hw, y, x+hw, y+b.h-c, x+hw-c, y+b.h, x-hw+c, y+b.h, x-hw, y+b.h-c)
		r.text([]string{n.LoopName}, x, y+b.h/2)
	case Loop:
		const c = 4
		th := r.loopTopH(n)
		r.poly(x-hw+c, y, x+hw-c, y, x+hw, y+c, x+hw, y+th, x-hw, y+th, x-hw, y+c)
		r.text(r.texts[n], x, y+th/2)
		r.line(x, y+th, x, y+th+r.st.Gap)
		body := r.seq(n.Body)
		r.drawSeq(n.Body, x, y+th+r.st.Gap)
		yb := y + th + r.st.Gap + body.h
		r.line(x, yb, x, yb+r.st.Gap)
		ye := yb + r.st.Gap
		r.poly(x-hw, ye, x+hw, ye, x+hw, ye+loopH-c, x+hw-c, ye+loopH, x-hw+c, ye+loopH, x-hw, ye+loopH-c)
		r.text([]string{n.LoopName}, x, ye+loopH/2)
	case Decision:
		dw := r.st.BlockB + 10
		dh := r.decisionH(n)
		r.poly(x, y, x+dw/2, y+dh/2, x, y+dh, x-dw/2, y+dh/2)
		r.text(r.texts[n], x, y+dh/2)
		yes, no := r.seq(n.Yes), r.seq(n.No)
		dx := x + r.sideX(n)
		top := y + dh + r.st.Gap
		merge := top + math.Max(yes.h, no.h) + r.st.Gap
		// «да» — вниз по оси
		r.label(r.st.YesNo[0], x+1.2, y+dh+2.2, 0)
		if len(n.Yes) > 0 {
			r.line(x, y+dh, x, top)
			r.drawSeq(n.Yes, x, top)
			r.line(x, top+yes.h, x, merge)
		} else {
			r.line(x, y+dh, x, merge)
		}
		// «нет» — вправо и вниз, возврат на ось со стрелкой (поток справа налево)
		r.label(r.st.YesNo[1], x+dw/2+1.2, y+dh/2-2.2, 0)
		r.line(x+dw/2, y+dh/2, dx, y+dh/2)
		if len(n.No) > 0 {
			r.line(dx, y+dh/2, dx, top)
			r.drawSeq(n.No, dx, top)
			r.line(dx, top+no.h, dx, merge)
		} else {
			r.line(dx, y+dh/2, dx, merge)
		}
		r.line(dx, merge, x, merge)
		r.arrow(x, merge, -1, 0)
	}
}
