package schgen

import (
	"hash/fnv"
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

// PickStyle — стиль по имени; пусто или «auto» — по ФИО (чтобы у соседей по группе листы различались), без ФИО — A.
func PickStyle(name, fio string) Style {
	name = strings.ToUpper(strings.TrimSpace(name))
	if s, ok := Styles[name]; ok {
		return s
	}
	if strings.TrimSpace(fio) == "" {
		return Styles["A"]
	}
	h := fnv.New32a()
	h.Write([]byte(strings.TrimSpace(fio)))
	return Styles[string(rune('A'+h.Sum32()%4))]
}
