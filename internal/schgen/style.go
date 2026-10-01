package schgen

import (
	"hash/fnv"
	"math/rand"
	"strings"
)

// Style — вид листа. Электрика одинакова во всех стилях; каждый стиль повторяет реальную работу
// (docs/research/styles.md, variants-works*.md):
//
//	A — как принятая схема Рязанцева (2025): американские вентили, отводы 45°, «Сигнал | Вывод», рамка с боковыми графами;
//	B — как принятая схема Осиповой (2025): ГОСТ-вентили, отводы 45°, шина BUS1, «№ | Цепь», только штамп;
//	C — как Сорокина (2023): ГОСТ-вентили и микросхемы с полями, отводы 90°, один XS1 из частей XS1.1–1.3 «Конт. | Цепь», только штамп;
//	D — как Базарнов (2023): ГОСТ-вентили и микросхемы, отводы 90°, шины B1…B5, отдельные XS «Конт. | Цепь», рамка с боковыми графами.
type Style struct {
	Name      string
	GostGates bool      // вентили прямоугольником «1»/«&» (ГОСТ 2.743)
	GostICs   bool      // микросхемы с основным полем (RG, BUF, DC, RAM, MPU)
	Perp      bool      // отводы к шине под 90°
	BusNames  bool      // имена шин B1…Bn
	MainBus   string    // имя одной главной шины (BUS1), если BusNames = false
	Conn      ConnStyle // вид таблицы разъёмов
	Sections  bool      // один разъём XS1 из частей (XS1.1, XS1.2, XS1.3)
	FullFrame bool      // рамка с боковыми графами (masters/gost_ramka_full.kicad_wks)
}

var Styles = map[string]Style{
	"A": {Name: "A", Conn: ConnSignalPin, FullFrame: true},
	"B": {Name: "B", GostGates: true, MainBus: "BUS1", Conn: ConnNumNet},
	"C": {Name: "C", GostGates: true, GostICs: true, Perp: true, Conn: ConnContNet, Sections: true},
	"D": {Name: "D", GostGates: true, GostICs: true, Perp: true, BusNames: true, Conn: ConnContNet, FullFrame: true},
}

// PickStyle — стиль по имени A|B|C|D; пусто или «auto» — собранный из признаков по зерну (MixStyle).
func PickStyle(name, seed string) Style {
	name = strings.ToUpper(strings.TrimSpace(name))
	if s, ok := Styles[name]; ok {
		return s
	}
	return MixStyle(seed)
}

// MixStyle — «авто»-стиль: каждый признак (вентили, микросхемы, отводы, шины, таблица разъёмов, XS1 из частей, рамка)
// выбирается отдельно по зерну «группа|вариант» — 288 сочетаний, а один вариант всегда получает один и тот же лист.
// Все признаки встречаются в принятых или реальных работах (см. описание Styles), электрика от них не зависит.
func MixStyle(seed string) Style {
	h := fnv.New64a()
	h.Write([]byte("style|" + seed))
	r := rand.New(rand.NewSource(int64(h.Sum64())))
	s := Style{Name: "авто", GostGates: r.Intn(2) == 1, GostICs: r.Intn(2) == 1, Perp: r.Intn(2) == 1,
		Conn: []ConnStyle{ConnSignalPin, ConnNumNet, ConnContNet}[r.Intn(3)], Sections: r.Intn(2) == 1, FullFrame: r.Intn(2) == 1}
	switch r.Intn(3) {
	case 1:
		s.MainBus = "BUS1"
	case 2:
		s.BusNames = true
	}
	return s
}
