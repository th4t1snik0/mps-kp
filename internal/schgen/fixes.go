package schgen

import (
	"fmt"
	"sort"
	"strings"
)

// Fixes — точечные правки листа студента по замечаниям руководителя: students/<ник>/schema/fixes.yaml.
// Только оформление и номиналы/типы — связи (электрику) так не меняют: их правят в построителе для всех.
// Обозначения — как на листе (после нумерации по ГОСТ): посмотреть их можно в СХЕМА-N/sheet.json (refs, bom).
type Fixes struct {
	Values   map[string]string     `yaml:"values"`    // значение на листе: R10: "300", C16: "330 н", DD4: "74HC573"
	BOMNames map[string]string     `yaml:"bom_names"` // наименование в перечне целиком: R10: "С2-33Н-0,125 300 Ом ±5 %"
	BOMNotes map[string]string     `yaml:"bom_notes"` // «Примечание» в перечне: DD8: "Двухпортовое ОЗУ 8К×8"
	NoteAdd  []string              `yaml:"note_add"`  // строки в конец «Примечания» на листе (номер ставится сам)
	Move     map[string][2]float64 `yaml:"move"`      // сдвиг, мм [вправо, вниз]: note, dec, power, sheet
}

var moveKeys = map[string]bool{"note": true, "dec": true, "power": true, "sheet": true}

// applyMoves — сдвиги из fixes поверх «почерка» (до построения листа).
func (f *Fixes) applyMoves(j *Jitter) []string {
	var errs []string
	for k, d := range f.Move {
		switch k {
		case "note":
			j.NoteDX, j.NoteDY = j.NoteDX+d[0], j.NoteDY+d[1]
		case "dec":
			j.DecDX, j.DecDY = j.DecDX+d[0], j.DecDY+d[1]
		case "power":
			j.PwrDY += d[1]
		case "sheet":
			j.ShiftX, j.ShiftY = j.ShiftX+d[0], j.ShiftY+d[1]
		default:
			errs = append(errs, fmt.Sprintf("move: «%s» — нельзя, можно: note, dec, power, sheet", k))
		}
	}
	return errs
}

// applyValues — значения на листе (после нумерации).
func (s *Sheet) applyValues(f *Fixes) []string {
	byRef := map[string][]*Comp{}
	var refs []string
	for _, c := range s.syms {
		if strings.HasPrefix(c.Ref, "#") {
			continue
		}
		if _, ok := byRef[c.Ref]; !ok {
			refs = append(refs, c.Ref)
		}
		byRef[c.Ref] = append(byRef[c.Ref], c)
	}
	sort.Strings(refs)
	var errs []string
	check := func(what string, m map[string]string) {
		for r := range m {
			if _, ok := byRef[r]; !ok {
				errs = append(errs, fmt.Sprintf("%s: на листе нет %s (есть: %s)", what, r, strings.Join(refs, ", ")))
			}
		}
	}
	check("values", f.Values)
	check("bom_names", f.BOMNames)
	check("bom_notes", f.BOMNotes)
	for r, v := range f.Values {
		for _, c := range byRef[r] {
			c.setValue(v)
		}
	}
	return errs
}

func (c *Comp) setValue(v string) {
	c.Value = v
	c.node.Walk(func(n *Node) {
		if n.Head() == "property" && n.Arg(0) == "Value" {
			n.Kids[2] = Q(v)
		}
	})
}
