package pz

import (
	"bytes"
	"image/png"
	"math"
	"os"

	"github.com/fogleman/gg"
	"github.com/golang/freetype/truetype"
)

// Боковые графы ГОСТ 2.104 (форма 1: 5 + 7 мм) — картинкой: повёрнутый текст в ячейке таблицы простые просмотрщики docx
// (в т. ч. встроенные в мессенджеры и облака) не умеют и рассыпают по буквам; картинку показывает любой.
var sideRows = []struct {
	label string
	h     float64 // мм, сверху вниз
}{{"Подп. и дата", 35}, {"Инв. № дубл.", 25}, {"Взам. инв. №", 25}, {"Подп. и дата", 35}, {"Инв. № подл.", 25}}

const sideW, sidePx = 12.0, 20.0 // ширина полосы, мм; пикселей на мм

func sideH() float64 {
	h := 0.0
	for _, r := range sideRows {
		h += r.h
	}
	return h
}

// sideGraphPNG — полоса граф 12 × 145 мм; font — TTF (GOST type A).
func sideGraphPNG(fontPath string) ([]byte, error) {
	b, err := os.ReadFile(fontPath)
	if err != nil {
		return nil, err
	}
	f, err := truetype.Parse(b)
	if err != nil {
		return nil, err
	}
	W, H := int(sideW*sidePx), int(sideH()*sidePx)
	dc := gg.NewContext(W, H)
	dc.SetRGB(1, 1, 1)
	dc.Clear()
	dc.SetRGB(0, 0, 0)
	lw := 0.5 * sidePx // 0,5 мм — основная линия
	dc.SetLineWidth(lw)
	dc.DrawRectangle(lw/2, lw/2, float64(W)-lw, float64(H)-lw)
	dc.Stroke()
	dc.SetLineWidth(0.3 * sidePx) // внутренние линии — тоньше
	dc.DrawLine(5*sidePx, 0, 5*sidePx, float64(H))
	dc.Stroke()
	dc.SetFontFace(truetype.NewFace(f, &truetype.Options{Size: 3.0 * sidePx, DPI: 72})) // кегль ≈ 3 мм (высота букв ≈ 2 мм)
	y := 0.0
	for i, r := range sideRows {
		if i > 0 {
			dc.DrawLine(0, y*sidePx, float64(W), y*sidePx)
			dc.Stroke()
		}
		// подпись — снизу вверх, по центру графы шириной 5 мм
		cx, cy := 2.5*sidePx, (y+r.h/2)*sidePx
		dc.Push()
		dc.RotateAbout(-math.Pi/2, cx, cy)
		dc.DrawStringAnchored(r.label, cx, cy, 0.5, 0.35)
		dc.Pop()
		y += r.h
	}
	var out bytes.Buffer
	if err := png.Encode(&out, dc.Image()); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// inlineImage — картинка в строке (w:drawing), r:embed = rid; размеры в мм.
func inlineImage(rid, name string, wmm, hmm float64) string {
	cx, cy := int(wmm*36000), int(hmm*36000)
	return `<w:r><w:drawing><wp:inline distT="0" distB="0" distL="0" distR="0" xmlns:wp="http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing">` +
		`<wp:extent cx="` + itoa(cx) + `" cy="` + itoa(cy) + `"/><wp:docPr id="9001" name="` + name + `"/>` +
		`<a:graphic xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/picture">` +
		`<pic:pic xmlns:pic="http://schemas.openxmlformats.org/drawingml/2006/picture"><pic:nvPicPr><pic:cNvPr id="9001" name="` + name + `"/><pic:cNvPicPr/></pic:nvPicPr>` +
		`<pic:blipFill><a:blip r:embed="` + rid + `" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"/><a:stretch><a:fillRect/></a:stretch></pic:blipFill>` +
		`<pic:spPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="` + itoa(cx) + `" cy="` + itoa(cy) + `"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom></pic:spPr>` +
		`</pic:pic></a:graphicData></a:graphic></wp:inline></w:drawing></w:r>`
}

func itoa(n int) string {
	return string(appendInt(nil, n))
}

func appendInt(b []byte, n int) []byte {
	if n >= 10 {
		b = appendInt(b, n/10)
	}
	return append(b, byte('0'+n%10))
}
