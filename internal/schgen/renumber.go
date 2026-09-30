package schgen

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Renumber присваивает позиционные обозначения по ГОСТ 2.710: в пределах буквенного
// кода — по расположению на листе сверху вниз, столбцами слева направо.
// Столбец — элементы, перекрывающиеся по горизонтали (по крайним выводам).
// Части одного корпуса (DD3.1, DD3.2) получают номер по первой встреченной части.
func (s *Sheet) Renumber() {
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
			bs = append(bs, box{c, x0 - 1.27, x1 + 1.27, c.At.Y})
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
