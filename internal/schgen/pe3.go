package schgen

import (
	"fmt"
	"strings"
)

// Перечень элементов (ПЭ3) по ГОСТ 2.701 на листах А4: графы 20 / 110 / 10 / 45 мм, шапка 15 мм, строка 8 мм.
// Рисуется тем же KiCad-листом (линии и текст), рамка — gost_ramka с внешним прямоугольником А4 (PE3Wks).
const (
	peX0, peX1, peX2, peX3, peX4 = 20.0, 40.0, 150.0, 160.0, 205.0
	peTop, peHead, peRow         = 5.0, 15.0, 8.0
	peBottom                     = 237.0 // верх основной надписи (55 мм от низа рамки)
	peFont                       = 2.5
	peNameMax, peNoteMax         = 44, 24 // символов в строке графы (GOST type A 2,5 мм); «×», «→» в шрифте нет — не использовать
)

type peItem struct {
	ref, name, qty, note string
	group                bool
}

// wrap режет текст по словам на строки не длиннее n символов.
func wrap(s string, n int) []string {
	var out []string
	cur := ""
	for _, w := range strings.Fields(s) {
		if cur != "" && len([]rune(cur+" "+w)) > n {
			out = append(out, cur)
			cur = w
			continue
		}
		if cur == "" {
			cur = w
		} else {
			cur += " " + w
		}
	}
	if cur != "" || len(out) == 0 {
		out = append(out, cur)
	}
	return out
}

func peRows(lines []BOMLine) []peItem {
	var rows []peItem
	for i, l := range lines {
		if l.Group != "" {
			if i > 0 {
				rows = append(rows, peItem{}) // пустая строка между группами
			}
			rows = append(rows, peItem{name: l.Group, group: true})
			continue
		}
		names, notes := wrap(l.Name, peNameMax), wrap(l.Note, peNoteMax)
		n := max(len(names), len(notes))
		for k := 0; k < n; k++ {
			r := peItem{}
			if k == 0 {
				r.ref, r.qty = l.Refs, fmt.Sprint(l.Qty)
			}
			if k < len(names) {
				r.name = names[k]
			}
			if k < len(notes) {
				r.note = notes[k]
			}
			rows = append(rows, r)
		}
	}
	return rows
}

// PE3 раскладывает перечень по страницам. Возвращает листы (А4) в порядке страниц.
func PE3(lines []BOMLine, seed string, tb TitleBlock) []*Sheet {
	rows := peRows(lines)
	avail := peBottom - peTop - peHead
	per := int(avail / peRow)
	var pages []*Sheet
	for p := 0; p*per < len(rows) || p == 0; p++ {
		s := NewSheet(nil, fmt.Sprintf("%s/pe3-%d", seed, p+1))
		s.A4 = true
		s.Title = tb
		s.peGrid()
		end := min(len(rows), (p+1)*per)
		for i, r := range rows[p*per : end] {
			y := peTop + peHead + float64(i)*peRow + peRow/2
			if r.group {
				s.peText(r.name, Pt{(peX1 + peX2) / 2, y}, "center", true)
				continue
			}
			s.peText(r.ref, Pt{(peX0 + peX1) / 2, y}, "center", false)
			s.peText(r.name, Pt{peX1 + 1.5, y}, "left", false)
			s.peText(r.qty, Pt{(peX2 + peX3) / 2, y}, "center", false)
			s.peText(r.note, Pt{peX3 + 1.5, y}, "left", false)
		}
		pages = append(pages, s)
	}
	return pages
}

func (s *Sheet) peLine(a, b Pt) {
	s.Graphic(L("polyline",
		L("pts", L("xy", F(a.X), F(a.Y)), L("xy", F(b.X), F(b.Y))),
		L("stroke", L("width", F(0.3)), L("type", A("default"))),
		L("fill", L("type", A("none"))),
		L("uuid", Q(s.uuid()))))
}

func (s *Sheet) peText(t string, at Pt, just string, underline bool) {
	if t == "" {
		return
	}
	eff := L("effects", L("font", L("face", Q("GOST type A")), L("size", F(peFont), F(peFont)), A("italic")))
	if just != "center" {
		eff.Kids = append(eff.Kids, L("justify", A(just)))
	}
	s.items = append(s.items, L("text", Q(t),
		L("exclude_from_sim", A("no")),
		L("at", F(at.X), F(at.Y), F(0)),
		eff,
		L("uuid", Q(s.uuid()))))
	if underline {
		w := float64(len([]rune(t))) * peFont * 0.62
		s.peLine(Pt{at.X - w/2, at.Y + 1.8}, Pt{at.X + w/2, at.Y + 1.8})
	}
}

// peGrid — шапка и вертикальные графы до основной надписи.
func (s *Sheet) peGrid() {
	top, head := peTop, peTop+peHead
	s.peLine(Pt{peX0, head}, Pt{peX4, head})
	for _, x := range []float64{peX1, peX2, peX3} {
		s.peLine(Pt{x, top}, Pt{x, peBottom})
	}
	for y := head + peRow; y < peBottom-0.1; y += peRow {
		s.peLine(Pt{peX0, y}, Pt{peX4, y})
	}
	s.peText("Поз.", Pt{(peX0 + peX1) / 2, top + 4.5}, "center", false)
	s.peText("обозна-", Pt{(peX0 + peX1) / 2, top + 8}, "center", false)
	s.peText("чение", Pt{(peX0 + peX1) / 2, top + 11.5}, "center", false)
	s.peText("Наименование", Pt{(peX1 + peX2) / 2, top + peHead/2}, "center", false)
	s.peText("Кол.", Pt{(peX2 + peX3) / 2, top + peHead/2}, "center", false)
	s.peText("Примечание", Pt{(peX3 + peX4) / 2, top + peHead/2}, "center", false)
}

// PE3Wks — рамка для страницы перечня из рамки схемы: внешний прямоугольник А4, «Перечень элементов», номер листа.
func PE3Wks(src, variantLine string, page, pages int) string {
	src = strings.Replace(src, "(start 389.989 282.0022)", "(start 179.989 282.0022)", 1)
	src = strings.Replace(src, `"Разработка микропроцессорной"`, `"Перечень"`, 1)
	src = strings.Replace(src, `"системы на базе МК i8051"`, `"элементов"`, 1)
	src = strings.Replace(src, `"Лист ${#}"`, fmt.Sprintf(`"Лист %d"`, page), 1)
	src = strings.Replace(src, `"Листов ${##}"`, fmt.Sprintf(`"Листов %d"`, pages), 1)
	out, _ := PatchWks(src, variantLine)
	return out
}
