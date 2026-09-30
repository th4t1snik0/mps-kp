package pz

import (
	"embed"

	"mpskp/internal/variant"
)

//go:embed flows/*.flow
var flowFS embed.FS

// DefaultFlows — заготовки схем алгоритмов под вариант: инициализация и алгоритмы трёх программ.
// Студент подгоняет их под свой код (схема должна соответствовать программе).
func DefaultFlows(p *variant.Params) map[string][]byte {
	names := []string{"1-init", "2-kbscan"}
	if p.M%2 == 0 {
		names = append(names, "3-bufread")
	} else {
		names = append(names, "3-bufwrite")
	}
	names = append(names, [][]string{{"4-ind", "5-t0ind"}, {"4-y1", "5-t0y1"}, {"4-y2", "5-y2isr"}}[p.M%3]...)
	out := map[string][]byte{}
	for _, n := range names {
		b, err := flowFS.ReadFile("flows/" + n + ".flow")
		if err != nil {
			panic(err)
		}
		out[n+".flow"] = b
	}
	return out
}
