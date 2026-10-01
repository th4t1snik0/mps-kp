package schgen

import (
	"hash/fnv"
	"math/rand"
)

// Jitter — «почерк» листа: мелкие отличия в пределах ГОСТ, чтобы у студентов с одним стилем листы не совпадали.
// Однозначно выводится из группы и варианта (любой прогон на вариант даёт один и тот же лист).
// Электрику не меняет: положение чертежа, размеры шрифтов, место меток и подписей, шаг строк клавиатуры,
// размер точек соединений, толщина шин и проводов, курсив меток.
type Jitter struct {
	ShiftX, ShiftY float64 // сдвиг всего чертежа в рамке, мм
	LabelFont      float64 // метки цепей: 1,0 / 1,1 (больше — черта над ~{CS} сливается с проводом строкой выше)
	RefFont        float64 // обозначения и типы микросхем, разъёмов, индикатора: 1,27 / 1,5 (мелкие элементы — всегда 1,27)
	NoteFont       float64 // «Примечание»: 1,5 / 1,7
	LabelAtPin     bool    // метки у выводов микросхем, а не у шины
	RefLeft        bool    // обозначение микросхемы слева от вывода питания
	KbDY           float64 // шаг строк клавиатуры: 12,7 / 13,97 (13,97 — только при 3 строках)
	JunctionD      float64 // диаметр точки соединения, мм (0 — по умолчанию KiCad)
	BusW, WireW    float64 // толщина шин и проводов, мм (0 — по умолчанию KiCad)
	LabelItalic    bool    // метки цепей курсивом
}

// Plain — без вариаций (как принятая схема; для тестов и эталонов).
var Plain = Jitter{LabelFont: labelFont, RefFont: 1.27, NoteFont: 1.5, KbDY: 12.7}

// MakeJitter — вариации по строке-зерну (группа|вариант).
func MakeJitter(seed string, rows int) Jitter {
	h := fnv.New64a()
	h.Write([]byte(seed))
	r := rand.New(rand.NewSource(int64(h.Sum64())))
	j := Jitter{
		ShiftX:     float64(r.Intn(11)) * 2.54,   // 0…25,4
		ShiftY:     float64(r.Intn(13)-8) * 1.27, // −10,16…+5,08
		LabelFont:  []float64{1.0, 1.1}[r.Intn(2)],
		RefFont:    []float64{1.27, 1.5}[r.Intn(2)],
		NoteFont:   []float64{1.5, 1.7}[r.Intn(2)],
		LabelAtPin: r.Intn(2) == 1,
		RefLeft:    r.Intn(2) == 1,
		KbDY:       12.7,
	}
	if rows == 3 && r.Intn(2) == 1 {
		j.KbDY = 13.97
	}
	// новые признаки — после старых, чтобы не сдвигать прежние выборы
	j.JunctionD = []float64{0, 0.9, 1.2}[r.Intn(3)]
	j.BusW = []float64{0, 0.45, 0.6}[r.Intn(3)]
	j.WireW = []float64{0, 0.2}[r.Intn(2)]
	j.LabelItalic = r.Intn(2) == 1
	return j
}
