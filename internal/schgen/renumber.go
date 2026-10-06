package schgen

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Renumber присваивает позиционные обозначения по ГОСТ 2.710: в пределах буквенного
// кода — по расположению на листе сверху вниз, столбцами слева направо.
// Столбец — элементы, перекрывающиеся по горизонтали (по корпусам, без ножек выводов).
// Части одного корпуса (DD3.1, DD3.2) получают номер по первой встреченной части.
func (s *Sheet) Renumber() {
	s.numCols = map[string][][]colItem{}
	groups := map[string][]*Comp{}
	for _, c := range s.syms {
		if strings.HasPrefix(c.Ref, "#") {
			continue
		}
		p := refPrefix(c.Ref)
		groups[p] = append(groups[p], c)
	}
	for prefix, cs := range groups {
		type box struct {
			c         *Comp
			x0, x1, y float64
		}
		var bs []box
		for _, c := range cs {
			x0, x1 := c.At.X, c.At.X
			for _, p := range c.pins {
				x0, x1 = min(x0, p.X), max(x1, p.X)
			}
			// корпус без ножек выводов: столбцы — по телам элементов, а не по концам выводов (иначе цепочки перекрытий
			// сливают в один «столбец» соседние узлы, и порядок номеров глазу неочевиден)
			if x1-x0 > 6 {
				x0, x1 = x0+2.54, x1-2.54
			}
			bs = append(bs, box{c, x0 - 0.5, x1 + 0.5, c.At.Y})
		}
		sort.SliceStable(bs, func(i, j int) bool { return bs[i].x0 < bs[j].x0 })
		// столбцы: жадно сливаем перекрывающиеся по x
		var cols [][]box
		right := -1e9
		for _, b := range bs {
			if len(cols) == 0 || b.x0 >= right {
				cols = append(cols, nil)
				right = b.x1
			}
			right = max(right, b.x1)
			cols[len(cols)-1] = append(cols[len(cols)-1], b)
		}
		s.numCols[prefix] = nil
		for _, col := range cols {
			var refs []colItem
			for _, b := range col {
				refs = append(refs, colItem{b.c, b.x0, b.x1, b.y})
			}
			s.numCols[prefix] = append(s.numCols[prefix], refs)
		}
		newRef := map[string]string{}
		n := 0
		for _, col := range cols {
			sort.SliceStable(col, func(i, j int) bool { return col[i].y < col[j].y })
			for _, b := range col {
				if _, ok := newRef[b.c.Ref]; !ok {
					n++
					newRef[b.c.Ref] = fmt.Sprintf("%s%d", prefix, n)
				}
			}
		}
		for _, c := range cs {
			c.setRef(newRef[c.Ref])
		}
	}
}

type colItem struct {
	c         *Comp
	x0, x1, y float64
}

// NumberingDoubts — места, где порядок «по столбцам» неочевиден глазу: соседние столбцы почти касаются (зазор < 2,54 мм),
// или в столбце, слитом по цепочке перекрытий, нижний элемент правее и не перекрывается с верхним, а верхний левее.
// Такие места правят раскладкой (элементы разносят), а не правилом.
func (s *Sheet) NumberingDoubts() []string {
	var out []string
	for prefix, cols := range s.numCols {
		for i := 1; i < len(cols); i++ {
			r := -1e9
			for _, b := range cols[i-1] {
				r = max(r, b.x1)
			}
			l := 1e9
			for _, b := range cols[i] {
				l = min(l, b.x0)
			}
			if l-r < 2.54 {
				out = append(out, fmt.Sprintf("%s: столбцы %s… и %s… почти касаются (зазор %.2f мм)", prefix,
					cols[i-1][0].c.Ref, cols[i][0].c.Ref, l-r))
			}
		}
		for _, col := range cols {
			for i := range col {
				for j := range col {
					a, b := col[i], col[j]
					// a выше b, но целиком правее него — глаз прочтёт b раньше a
					if a.y < b.y && a.x0 > b.x1 {
						out = append(out, fmt.Sprintf("%s: %s выше, но правее %s — в столбце их порядок неочевиден", prefix, a.c.Ref, b.c.Ref))
					}
				}
			}
		}
	}
	sort.Strings(out)
	return dedup(out)
}

func (c *Comp) setRef(ref string) {
	c.Ref = ref
	c.node.Walk(func(n *Node) {
		switch {
		case n.Head() == "property" && n.Arg(0) == "Reference":
			n.Kids[2] = Q(ref)
		case n.Head() == "reference":
			n.Kids[1] = Q(ref)
		}
	})
}

func refPrefix(ref string) string { return strings.TrimRight(ref, "0123456789") }

func refNum(ref string) int {
	n, _ := strconv.Atoi(ref[len(refPrefix(ref)):])
	return n
}
