package schgen

import (
	"hash/fnv"
	"math/rand"
)

// Jitter — «почерк» листа: мелкие отличия в пределах ГОСТ, чтобы у студентов с одним стилем листы не совпадали.
// Однозначно выводится из ФИО, группы и варианта (один и тот же студент всегда получает один и тот же лист).
// Электрику не меняет: только положение чертежа, размеры шрифтов, место меток и подписей, шаг строк клавиатуры.
type Jitter struct {
	ShiftX, ShiftY float64 // сдвиг всего чертежа в рамке, мм
	LabelFont      float64 // метки цепей: 1,0 / 1,1 (больше — черта над ~{CS} сливается с проводом строкой выше)
	RefFont        float64 // обозначения и типы микросхем, разъёмов, индикатора: 1,27 / 1,5 (мелкие элементы — всегда 1,27)
	NoteFont       float64 // «Примечание»: 1,5 / 1,7
	LabelAtPin     bool    // метки у выводов микросхем, а не у шины
	RefLeft        bool    // обозначение микросхемы слева от вывода питания
	KbDY           float64 // шаг строк клавиатуры: 12,7 / 13,97 (13,97 — только при 3 строках)
}

// Plain — без вариаций (как принятая схема; для тестов и эталонов).
var Plain = Jitter{LabelFont: labelFont, RefFont: 1.27, NoteFont: 1.5, KbDY: 12.7}

// MakeJitter — вариации по строке-зерну (ФИО|группа|вариант).
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
	return j
}
