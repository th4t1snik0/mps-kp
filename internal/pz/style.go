package pz

import (
	"hash/fnv"
)

// DocStyle — вид документа по образцам принятых работ (у группы ПЗ не одинаковые):
//
//	A — как Осипова 2025 (принята): номер страницы сверху по центру + «Курсовая работа, МПС ч2» справа,
//	    заголовки разделов прописными по центру, «Рис. 3.1. …», «Таблица 3.1» справа над таблицей;
//	B — как Базарнов 2023 / Женюх 2022: номер снизу по центру, заголовки по центру,
//	    «Рис. N. …»;
//	C — ГОСТ 7.32: номер снизу по центру, заголовки с абзацного отступа, «Рисунок N — …», «Таблица N — …».
type DocStyle struct {
	Name         string
	HeaderText   string // верхний колонтитул справа ("" — нет)
	NumTop       bool   // номер страницы сверху (иначе снизу), по центру
	H1Center     bool   // заголовки разделов по центру (иначе с абзацного отступа)
	H1Caps       bool   // прописными
	FigShort     bool   // «Рис. N. …» (иначе «Рисунок N — …»)
	FigBySection bool   // номер рисунка/таблицы в пределах раздела: 3.1, 3.2
	TabRight     bool   // «Таблица N» отдельной строкой справа, название — по центру следующей строкой
	CodeSz       int    // кегль листингов в полупунктах (0 — 9 pt); подбирает FitCode под самую длинную строку
}

var docStyles = map[string]DocStyle{
	"A": {Name: "A", HeaderText: "Курсовая работа, МПС ч2", NumTop: true, H1Center: true, H1Caps: true, FigShort: true, FigBySection: true, TabRight: true},
	"B": {Name: "B", H1Center: true, H1Caps: true, FigShort: true},
	"C": {Name: "C"},
}

// PickDocStyle — стиль по имени (A, B, C) или, если пусто, от строки-зерна (ФИО|группа|M).
func PickDocStyle(name, seed string) DocStyle {
	if s, ok := docStyles[name]; ok {
		return s
	}
	h := fnv.New32a()
	h.Write([]byte(seed + "|doc"))
	return docStyles[string(rune('A'+h.Sum32()%3))]
}

func (ds DocStyle) codeSz() int {
	if ds.CodeSz > 0 {
		return ds.CodeSz
	}
	return 18
}

// FitCode — кегль листинга (полупункты, 14…18 = 7…9 pt), при котором строка из maxCols символов Courier New
// (ширина знака 0,6 кегля) помещается в поле текста 175 мм; длиннее 7 pt — переносится.
func FitCode(maxCols int) int {
	if maxCols <= 0 {
		return 18
	}
	sz := int(2 * 175 / 25.4 * 72 / (0.6 * float64(maxCols)))
	return max(14, min(18, sz))
}
