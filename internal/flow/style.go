package flow

import (
	"hash/fnv"
	"math/rand"
)

// Style — вид схем алгоритмов студента: в пределах ГОСТ 19.701 (символы, пропорции, подписи «да/нет»), но у всей группы
// рисунки не одинаковые — как у реальных работ (Visio, draw.io, Word). Выбирается детерминированно от ФИО.
type Style struct {
	Font       string  // файл шрифта в fonts/
	FontMM     float64 // высота шрифта, мм
	BlockB     float64 // ширина символа, мм
	BlockA     float64 // минимальная высота символа, мм
	Gap        float64 // вертикальный промежуток, мм
	Stroke     float64 // толщина линий, мм
	TermRadius float64 // скругление терминатора: 0.5 — «таблетка», меньше — прямоугольник со скруглёнными углами
	YesNo      [2]string
	JoinArrows bool // стрелка на каждом слиянии (а не только справа налево)
}

// Default — вид по умолчанию (GOST type A, как рамка схемы Э3).
var Default = Style{Font: "GOST_A.ttf", FontMM: 3.5, BlockB: 40, BlockA: 15, Gap: 5, Stroke: 0.35, TermRadius: 0.5, YesNo: [2]string{"да", "нет"}}

// PickStyle — вид от строки-зерна (обычно «ФИО|группа|M»).
// Fonts — шрифты схем алгоритмов (fonts/): файл и высота строчной буквы, мм. Каждый обязан читаться truetype
// (тест TestFontsParse): PT Sans убран — его таблицу кернинга библиотека не читает, ПЗ1/ПЗ2 падали у каждого пятого варианта.
var Fonts = []struct {
	File string
	MM   float64
}{{"GOST_A.ttf", 3.5}, {"LiberationSans-Regular.ttf", 2.9}, {"LiberationSerif-Regular.ttf", 3.1}, {"Carlito-Regular.ttf", 3.1}}

func PickStyle(seed string) Style {
	h := fnv.New64a()
	h.Write([]byte(seed + "|flow"))
	r := rand.New(rand.NewSource(int64(h.Sum64())))
	f := Fonts[r.Intn(len(Fonts))]
	s := Style{
		Font:       f.File,
		FontMM:     f.MM + []float64{-0.2, 0, 0.2}[r.Intn(3)],
		BlockB:     []float64{35, 40, 45}[r.Intn(3)],
		BlockA:     []float64{15, 15, 20}[r.Intn(3)],
		Gap:        []float64{4, 5, 6}[r.Intn(3)],
		Stroke:     []float64{0.25, 0.3, 0.35, 0.4}[r.Intn(4)],
		TermRadius: []float64{0.5, 0.5, 0.25}[r.Intn(3)],
		YesNo:      [][2]string{{"да", "нет"}, {"Да", "Нет"}, {"да", "нет"}}[r.Intn(3)],
		JoinArrows: r.Intn(2) == 0,
	}
	return s
}
