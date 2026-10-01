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

// Титул — одна таблица без рамок с точными высотами строк (а не отступы «перед абзацем» и плавающая таблица:
// их Word, LibreOffice и просмотрщики раскладывают по-разному). Боковые графы ГОСТ 2.104 (форма 1: 5 + 7 мм) —
// ячейки той же таблицы, выдвинутой на левое поле (8 мм от края листа).

const (
	titleH    = 255.0 // высота таблицы титула, мм (поле страницы — 257: под таблицей помещается абзац разрыва)
	textW     = 175.0 // ширина поля текста: 210 − 25 − 10
	leftField = 25.0  // левое поле
)

// titleRow — строка таблицы титула: высота (мм), содержимое (абзацы), вертикальное выравнивание, боковая графа ("" — нет).
type titleRow struct {
	h      float64
	body   string
	valign string
	graph  string
}

func twip(mm float64) int { return int(mm*56.7 + 0.5) }

// titleTable собирает таблицу титула; graphs — с боковыми графами (выдвинута на левое поле).
func titleTable(rows []titleRow, graphs bool) string {
	cols := []float64{textW}
	ind := 0.0
	if graphs {
		cols = []float64{5, 7, leftField - 8 - 12, textW}
		ind = -(leftField - 8)
	}
	total := 0.0
	for _, c := range cols {
		total += c
	}
	var b strings.Builder
	fmt.Fprintf(&b, `<w:tbl><w:tblPr><w:tblW w:w="%d" w:type="dxa"/><w:tblInd w:w="%d" w:type="dxa"/>`+
		`<w:tblBorders><w:top w:val="nil"/><w:left w:val="nil"/><w:bottom w:val="nil"/><w:right w:val="nil"/><w:insideH w:val="nil"/><w:insideV w:val="nil"/></w:tblBorders><w:tblLayout w:type="fixed"/>`+
		`<w:tblCellMar><w:top w:w="0" w:type="dxa"/><w:left w:w="0" w:type="dxa"/><w:bottom w:w="0" w:type="dxa"/><w:right w:w="0" w:type="dxa"/></w:tblCellMar></w:tblPr><w:tblGrid>`,
		twip(total), twip(ind))
	for _, c := range cols {
		fmt.Fprintf(&b, `<w:gridCol w:w="%d"/>`, twip(c))
	}
	b.WriteString(`</w:tblGrid>`)
	empty := `<w:p><w:pPr><w:spacing w:before="0" w:after="0"/><w:ind w:left="0" w:firstLine="0"/></w:pPr></w:p>`
	box := `<w:tcBorders><w:top w:val="single" w:sz="8" w:color="000000"/><w:left w:val="single" w:sz="8" w:color="000000"/><w:bottom w:val="single" w:sz="8" w:color="000000"/><w:right w:val="single" w:sz="8" w:color="000000"/></w:tcBorders>`
	cell := func(w float64, pr, body string) {
		if body == "" {
			body = empty
		}
		fmt.Fprintf(&b, `<w:tc><w:tcPr><w:tcW w:w="%d" w:type="dxa"/>%s</w:tcPr>%s</w:tc>`, twip(w), pr, body)
	}
	for _, r := range rows {
		fmt.Fprintf(&b, `<w:tr><w:trPr><w:cantSplit/><w:trHeight w:val="%d" w:hRule="exact"/></w:trPr>`, twip(r.h))
		if graphs {
			if r.graph != "" {
				cell(cols[0], box+`<w:textDirection w:val="btLr"/><w:vAlign w:val="center"/>`,
					`<w:p><w:pPr><w:spacing w:before="0" w:after="0"/><w:ind w:left="0" w:firstLine="0"/><w:jc w:val="center"/></w:pPr><w:r><w:rPr><w:i/><w:sz w:val="16"/></w:rPr><w:t>`+xmlEsc(r.graph)+`</w:t></w:r></w:p>`)
				cell(cols[1], box, "")
			} else {
				cell(cols[0], "", "")
				cell(cols[1], "", "")
			}
			cell(cols[2], "", "")
		}
		va := r.valign
		if va == "" {
			va = "top"
		}
		cell(cols[len(cols)-1], `<w:vAlign w:val="`+va+`"/>`, r.body)
		b.WriteString(`</w:tr>`)
	}
	b.WriteString(`</w:tbl>`)
	// разрыв страницы — в крошечном абзаце, чтобы он поместился под таблицей на том же листе
	b.WriteString(`<w:p><w:pPr><w:spacing w:before="0" w:after="0" w:line="20" w:lineRule="exact"/><w:ind w:left="0" w:firstLine="0"/><w:rPr><w:sz w:val="2"/></w:rPr></w:pPr><w:r><w:rPr><w:sz w:val="2"/></w:rPr><w:br w:type="page"/></w:r></w:p>`)
	return b.String()
}

// TitleInfo — данные титула.
type TitleInfo struct {
	Doc     string // «Описание программной части» / «Описание аппаратной части»
	FIO     string
	Group   string
	M       int
	Checker string
	Year    int
}

// Title — титул ПЗ2 по шаблону Прил. Б ТЗ (docs/source/шаблон-ПЗ2-ПрилБ.docx) и принятой ПЗ2 (Осипова 2025):
// название на ~трети высоты, «Описание …» жирным, «Листов N» (поле NUMPAGES), блок «Выполнил…» справа,
// «Москва, год» внизу, слева внизу — боковые графы. Строки боковых граф сверху вниз: 35/25/25/35/25 мм.
func (d *Doc) Title(t TitleInfo) {
	rows := []titleRow{
		{h: 72},
		{h: 14, body: tp("center", 0, 0, false, 14, run("Проектирование микропроцессорной системы на базе МК i8051"))},
		{h: 14, body: tp("center", 0, 0, true, 14, run(t.Doc))},
		{h: 10, body: tp("center", 0, 0, false, 12, run("Листов ")+numPages())},
		{h: 35, graph: "Подп. и дата", valign: "bottom", body: tp("left", 0, 95, false, 12, run("Выполнил: "+t.FIO)) +
			tp("left", 1.5, 95, false, 12, run("Группа: "+t.Group)) + tp("left", 1.5, 95, false, 12, run(fmt.Sprintf("Вариант: %d", t.M)))},
		{h: 25, graph: "Инв. № дубл.", body: tp("left", 5, 95, false, 12, run("Проверил: "+t.Checker))},
		{h: 25, graph: "Взам. инв. №"},
		{h: 35, graph: "Подп. и дата"},
		{h: 25, graph: "Инв. № подл.", valign: "center", body: tp("center", 0, 0, false, 12, run(fmt.Sprintf("Москва, %d", t.Year)))},
	}
	d.w("\n```{=openxml}\n%s\n```\n\n", titleTable(rows, true))
}

// TitleA — титул ПЗ1 по Прил. А ТЗ-2026 (без рамки и боковых граф): шапка вуза, «КУРСОВАЯ  РАБОТА», курс, тема,
// «Аппаратная часть», блок «Выполнил / Группа / Вариант / Дата», «Проверил / Дата», «Москва, год».
func (d *Doc) TitleA(t TitleInfo, date string) {
	lab := func(before float64, k, v string) string {
		if d.Style.Name == "A" && v != "" {
			// как у Осиповой: значение подчёркнуто
			return tp("left", before, 95, false, 14, run(k+" ")+`<w:r><w:rPr><w:u w:val="single"/><w:sz w:val="28"/></w:rPr><w:t xml:space="preserve">`+xmlEsc(v)+`</w:t></w:r>`)
		}
		return tp("left", before, 95, false, 14, run(k+" "+v))
	}
	rows := []titleRow{
		{h: 22, body: tp("center", 0, 0, false, 14, run("Федеральное государственное бюджетное образовательное учреждение высшего образования")) +
			tp("center", 0, 0, false, 14, run("«Национальный исследовательский университет «МЭИ»"))},
		{h: 42},
		{h: 18, body: tp("center", 0, 0, true, 14, run("КУРСОВАЯ  РАБОТА")) + tp("center", 2, 0, false, 14, run("по курсу \"Микропроцессорные системы\""))},
		{h: 18},
		{h: 24, body: tp("center", 0, 0, false, 16, run("Проектирование микропроцессорной системы на базе МК i8051")) + tp("center", 6, 0, true, 14, run(t.Doc))},
		{h: 20},
		{h: 32, body: lab(0, "Выполнил:", t.FIO) + lab(1, "Группа:", t.Group) + lab(1, "Вариант:", fmt.Sprint(t.M)) + lab(1, "Дата:", date)},
		{h: 16, body: lab(0, "Проверил:", t.Checker) + lab(1, "Дата:", "")},
		{h: 63, valign: "bottom", body: tp("center", 0, 0, false, 14, run(fmt.Sprintf("Москва, %d", t.Year)))},
	}
	d.w("\n```{=openxml}\n%s\n```\n\n", titleTable(rows, false))
}

// changeSheet — лист регистрации изменений по форме ГОСТ 19.604: шапка в две строки («Номера листов (страниц)» над
// изменённых / заменённых / новых / аннулированных), кегль шапки 9 pt, rows пустых строк по 8 мм.
func changeSheet(rows int) string {
	w := []float64{9, 19, 19, 19, 20, 20, 20, 24, 12, 13} // сумма — 175 мм
	var b strings.Builder
	b.WriteString(`<w:tbl><w:tblPr><w:tblW w:w="` + fmt.Sprint(twip(textW)) + `" w:type="dxa"/>` +
		`<w:tblBorders><w:top w:val="single" w:sz="8" w:color="000000"/><w:left w:val="single" w:sz="8" w:color="000000"/><w:bottom w:val="single" w:sz="8" w:color="000000"/><w:right w:val="single" w:sz="8" w:color="000000"/><w:insideH w:val="single" w:sz="4" w:color="000000"/><w:insideV w:val="single" w:sz="4" w:color="000000"/></w:tblBorders>` +
		`<w:tblLayout w:type="fixed"/><w:tblCellMar><w:left w:w="40" w:type="dxa"/><w:right w:w="40" w:type="dxa"/></w:tblCellMar></w:tblPr><w:tblGrid>`)
	for _, c := range w {
		fmt.Fprintf(&b, `<w:gridCol w:w="%d"/>`, twip(c))
	}
	b.WriteString(`</w:tblGrid>`)
	p := func(s string) string {
		return `<w:p><w:pPr><w:spacing w:before="0" w:after="0" w:line="240" w:lineRule="auto"/><w:ind w:left="0" w:firstLine="0"/><w:jc w:val="center"/></w:pPr><w:r><w:rPr><w:sz w:val="18"/></w:rPr><w:t xml:space="preserve">` + xmlEsc(s) + `</w:t></w:r></w:p>`
	}
	cell := func(width float64, extra, text string) {
		fmt.Fprintf(&b, `<w:tc><w:tcPr><w:tcW w:w="%d" w:type="dxa"/>%s<w:vAlign w:val="center"/></w:tcPr>%s</w:tc>`, twip(width), extra, p(text))
	}
	sum := func(a []float64) (s float64) {
		for _, x := range a {
			s += x
		}
		return
	}
	merged := []string{"Всего листов (страниц) в докум.", "№ докум.", "Входящий № сопроводит. докум. и дата", "Подп.", "Дата"}
	b.WriteString(`<w:tr><w:trPr><w:tblHeader/><w:trHeight w:val="454"/></w:trPr>`)
	cell(w[0], `<w:vMerge w:val="restart"/>`, "Изм.")
	cell(sum(w[1:5]), `<w:gridSpan w:val="4"/>`, "Номера листов (страниц)")
	for i, t := range merged {
		cell(w[5+i], `<w:vMerge w:val="restart"/>`, t)
	}
	b.WriteString(`</w:tr><w:tr><w:trPr><w:tblHeader/><w:trHeight w:val="1020"/></w:trPr>`)
	cell(w[0], `<w:vMerge/>`, "")
	for i, t := range []string{"изменённых", "заменённых", "новых", "аннулиро\u00adванных"} {
		cell(w[1+i], "", t)
	}
	for i := range merged {
		cell(w[5+i], `<w:vMerge/>`, "")
	}
	b.WriteString(`</w:tr>`)
	for r := 0; r < rows; r++ {
		fmt.Fprintf(&b, `<w:tr><w:trPr><w:trHeight w:val="%d" w:hRule="exact"/></w:trPr>`, twip(8))
		for _, c := range w {
			cell(c, "", "")
		}
		b.WriteString(`</w:tr>`)
	}
	b.WriteString(`</w:tbl>`)
	return b.String()
}
