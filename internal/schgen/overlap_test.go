package schgen

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// Наложений на листе нет ни в одном стиле и «почерке» (замечание руководителя 06.10.2026: «взаимные пересечения УГО»).
func TestNoOverlaps(t *testing.T) {
	lib, err := LoadLib("../../masters/lib/mps.kicad_sym")
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[string][]string{}
	n := 0
	for i, v := range variants {
		for _, st := range []string{"A", "B", "C", "D", "mix1", "mix2", "mix3", "mix4", "mix5", "mix6"} {
			for k := 0; k < 6; k++ {
				seed := fmt.Sprintf("ол|%d|%s|%d", i, st, k)
				v.Style = PickStyle(st, seed)
				j := MakeJitter(seed, v.Rows)
				v.Jitter = &j
				sh := Build(lib, v, "test")
				o, err := Overlaps(sh.String())
				if err != nil {
					t.Fatal(err)
				}
				n++
				for _, x := range o {
					key := regexp.MustCompile(`[-\d.,()–]+`).ReplaceAllString(x, "#")
					kinds[key] = append(kinds[key], fmt.Sprintf("%d/%s/%d", i, st, k))
				}
			}
		}
	}
	var keys []string
	for k := range kinds {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		ex := kinds[k]
		t.Errorf("%d раз из %d листов: %s (напр. %s)", len(ex), n, k, strings.Join(ex[:min(3, len(ex))], ", "))
	}
}

// Нумерация по столбцам читается глазом однозначно (замечание руководителя 06.10.2026: «нумерация элементов»).
func TestNumberingClear(t *testing.T) {
	lib, err := LoadLib("../../masters/lib/mps.kicad_sym")
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[string][]string{}
	for i, v := range variants {
		for _, st := range []string{"A", "B", "C", "D", "mix1", "mix2", "mix3"} {
			for k := 0; k < 4; k++ {
				seed := fmt.Sprintf("ном|%d|%s|%d", i, st, k)
				v.Style = PickStyle(st, seed)
				j := MakeJitter(seed, v.Rows)
				v.Jitter = &j
				for _, x := range Build(lib, v, "test").NumberingDoubts() {
					kinds[x] = append(kinds[x], fmt.Sprintf("%d/%s/%d", i, st, k))
				}
			}
		}
	}
	for k, ex := range kinds {
		t.Errorf("%d раз: %s (напр. %s)", len(ex), k, ex[0])
	}
}
