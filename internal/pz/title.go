package pz

import (
	"fmt"
	"strings"
)

// Титул — сырой OOXML (pandoc не умеет точные отступы, поля Word и плавающие таблицы).
// Раскладка — по шаблону Прил. Б ТЗ и принятой ПЗ2 (Осипова 2025): название по центру на ~трети высоты,
// «Описание …» жирным, «Листов N» (поле NUMPAGES), блок «Выполнил…» справа, «Москва, год» внизу,
// слева внизу — боковые графы ГОСТ 2.104 (Инв. № подл., Подп. и дата, Взам. инв. №, Инв. № дубл., Подп. и дата).

func xmlEsc(s string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;").Replace(s)
}

// tp — абзац титула: выравнивание, отступ перед (мм), отступ слева (мм), жирный, размер (pt), текст (может содержать поле).
func tp(jc string, beforeMM, leftMM float64, bold bool, pt int, runs string) string {
	b := ""
	if bold {
		b = "<w:b/>"
	}
	return fmt.Sprintf(`<w:p><w:pPr><w:spacing w:before="%d" w:after="0"/><w:ind w:left="%d" w:firstLine="0"/><w:jc w:val="%s"/><w:rPr>%s<w:sz w:val="%d"/></w:rPr></w:pPr>%s</w:p>`,
		int(beforeMM*56.7), int(leftMM*56.7), jc, b, pt*2, strings.ReplaceAll(runs, "<w:rPr/>", "<w:rPr>"+b+fmt.Sprintf(`<w:sz w:val="%d"/>`, pt*2)+"</w:rPr>"))
}

func run(s string) string {
	return `<w:r><w:rPr/><w:t xml:space="preserve">` + xmlEsc(s) + `</w:t></w:r>`
}

func numPages() string {
	return `<w:r><w:rPr/><w:fldChar w:fldCharType="begin"/></w:r><w:r><w:rPr/><w:instrText xml:space="preserve"> NUMPAGES </w:instrText></w:r><w:r><w:rPr/><w:fldChar w:fldCharType="separate"/></w:r><w:r><w:rPr/><w:t>—</w:t></w:r><w:r><w:rPr/><w:fldChar w:fldCharType="end"/></w:r>`
}

// sideGraphs — боковые графы у левого края листа, снизу вверх 25/35/25/25/35 мм (ГОСТ 2.104, форма 1: 5 + 7 мм).
func sideGraphs() string {
	rows := []struct {
		label string
		h     float64
	}{{"Подп. и дата", 35}, {"Инв. № дубл.", 25}, {"Взам. инв. №", 25}, {"Подп. и дата", 35}, {"Инв. № подл.", 25}}
	total := 0.0
	for _, r := range rows {
		total += r.h
	}
	tw := func(mm float64) int { return int(mm * 56.7) }
	var b strings.Builder
	fmt.Fprintf(&b, `<w:tbl><w:tblPr><w:tblpPr w:leftFromText="0" w:rightFromText="0" w:vertAnchor="page" w:horzAnchor="page" w:tblpX="%d" w:tblpY="%d"/><w:tblW w:w="%d" w:type="dxa"/><w:tblLayout w:type="fixed"/>`+
		`<w:tblBorders><w:top w:val="single" w:sz="8" w:color="000000"/><w:left w:val="single" w:sz="8" w:color="000000"/><w:bottom w:val="single" w:sz="8" w:color="000000"/><w:right w:val="single" w:sz="8" w:color="000000"/><w:insideH w:val="single" w:sz="8" w:color="000000"/><w:insideV w:val="single" w:sz="8" w:color="000000"/></w:tblBorders>`+
		`<w:tblCellMar><w:left w:w="0" w:type="dxa"/><w:right w:w="0" w:type="dxa"/></w:tblCellMar></w:tblPr><w:tblGrid><w:gridCol w:w="%d"/><w:gridCol w:w="%d"/></w:tblGrid>`,
		tw(8), tw(297-10-total), tw(12), tw(5), tw(7))
	for _, r := range rows {
		fmt.Fprintf(&b, `<w:tr><w:trPr><w:trHeight w:val="%d" w:hRule="exact"/></w:trPr>`, tw(r.h))
		fmt.Fprintf(&b, `<w:tc><w:tcPr><w:tcW w:w="%d" w:type="dxa"/><w:textDirection w:val="btLr"/><w:vAlign w:val="center"/></w:tcPr><w:p><w:pPr><w:ind w:firstLine="0"/><w:jc w:val="center"/><w:spacing w:before="0" w:after="0"/></w:pPr><w:r><w:rPr><w:i/><w:sz w:val="16"/></w:rPr><w:t>%s</w:t></w:r></w:p></w:tc>`, tw(5), xmlEsc(r.label))
		fmt.Fprintf(&b, `<w:tc><w:tcPr><w:tcW w:w="%d" w:type="dxa"/></w:tcPr><w:p><w:pPr><w:ind w:firstLine="0"/></w:pPr></w:p></w:tc></w:tr>`, tw(7))
	}
	b.WriteString(`</w:tbl>`)
	return b.String()
}

// TitleInfo — данные титула.
type TitleInfo struct {
	Doc      string // «Описание программной части» / «Описание аппаратной части»
	FIO      string
	Group    string
	M        int
	Checker  string
	Year     int
	Appendix string // «Приложение Б» (стиль B) или ""
}

// Title — титульный лист + разрыв страницы. Высота поля страницы — 257 мм.
func (d *Doc) Title(t TitleInfo) {
	var x strings.Builder
	x.WriteString(sideGraphs())
	top := 78.0
	if t.Appendix != "" && d.Style.AppendixLabel {
		x.WriteString(tp("right", 0, 0, false, 12, run(t.Appendix)))
		top -= 5
	}
	x.WriteString(tp("center", top, 0, false, 14, run("Проектирование микропроцессорной системы на базе МК i8051")))
	x.WriteString(tp("center", 14, 0, true, 14, run(t.Doc)))
	x.WriteString(tp("center", 14, 0, false, 12, run("Листов ")+numPages()))
	x.WriteString(tp("left", 22, 95, false, 12, run("Выполнил: "+t.FIO)))
	x.WriteString(tp("left", 3, 95, false, 12, run("Группа: "+t.Group)))
	x.WriteString(tp("left", 3, 95, false, 12, run(fmt.Sprintf("Вариант: %d", t.M))))
	x.WriteString(tp("left", 8, 95, false, 12, run("Проверил: "+t.Checker)))
	x.WriteString(tp("center", 58, 0, false, 12, run(fmt.Sprintf("Москва, %d", t.Year))))
	x.WriteString(`<w:p><w:r><w:br w:type="page"/></w:r></w:p>`)
	d.w("\n```{=openxml}\n%s\n```\n\n", x.String())
}

// TitleA — титул ПЗ1 по Прил. А ТЗ-2026 (без рамки и боковых граф): шапка вуза, «КУРСОВАЯ  РАБОТА», курс, тема,
// «Аппаратная часть», блок «Выполнил / Группа / Вариант / Дата», «Проверил / Дата», «Москва, год».
func (d *Doc) TitleA(t TitleInfo, date string) {
	var x strings.Builder
	x.WriteString(tp("center", 0, 0, false, 14, run("Федеральное государственное бюджетное образовательное учреждение высшего образования")))
	x.WriteString(tp("center", 0, 0, false, 14, run("«Национальный исследовательский университет «МЭИ»")))
	x.WriteString(tp("center", 62, 0, true, 14, run("КУРСОВАЯ  РАБОТА")))
	x.WriteString(tp("center", 4, 0, false, 14, run("по курсу \"Микропроцессорные системы\"")))
	x.WriteString(tp("center", 22, 0, false, 16, run("Проектирование микропроцессорной системы на базе МК i8051")))
	x.WriteString(tp("center", 8, 0, true, 14, run(t.Doc)))
	lab := func(before float64, k, v string) {
		if d.Style.Name == "A" && v != "" {
			// как у Осиповой: значение подчёркнуто
			x.WriteString(tp("left", before, 95, false, 14, run(k+" ")+`<w:r><w:rPr><w:u w:val="single"/><w:sz w:val="28"/></w:rPr><w:t xml:space="preserve">`+xmlEsc(v)+`</w:t></w:r>`))
			return
		}
		x.WriteString(tp("left", before, 95, false, 14, run(k+" "+v)))
	}
	lab(26, "Выполнил:", t.FIO)
	lab(2, "Группа:", t.Group)
	lab(2, "Вариант:", fmt.Sprint(t.M))
	lab(2, "Дата:", date)
	lab(8, "Проверил:", t.Checker)
	lab(2, "Дата:", "")
	x.WriteString(tp("center", 52, 0, false, 14, run(fmt.Sprintf("Москва, %d", t.Year))))
	x.WriteString(`<w:p><w:r><w:br w:type="page"/></w:r></w:p>`)
	d.w("\n```{=openxml}\n%s\n```\n\n", x.String())
}
