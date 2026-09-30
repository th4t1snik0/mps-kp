package schgen

import (
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
)

// Lib — библиотека символов KiCad (.kicad_sym), символы по имени.
type Lib struct {
	Syms  map[string]*Node
	order []string
}

func LoadLib(path string) (*Lib, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	root, err := Parse(string(b))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	l := &Lib{Syms: map[string]*Node{}}
	for _, s := range root.All("symbol") {
		l.Syms[s.Arg(0)] = s
		l.order = append(l.order, s.Arg(0))
	}
	return l, nil
}

// Flat — копия символа name с раскрытым extends и новым именем newName.
func (l *Lib) Flat(name, newName string) (*Node, error) {
	s, ok := l.Syms[name]
	if !ok {
		return nil, fmt.Errorf("нет символа %q", name)
	}
	s = s.Clone()
	if ext := s.Find("extends"); ext != nil {
		base, err := l.Flat(ext.Arg(0), name)
		if err != nil {
			return nil, err
		}
		// берём графику/выводы базы, свойства — производного
		base.Remove(func(k *Node) bool { return k.Head() == "property" })
		var props []*Node
		for _, k := range s.Kids {
			if k.Head() == "property" {
				props = append(props, k)
			}
		}
		base.Kids = append(base.Kids[:2], append(props, base.Kids[2:]...)...)
		s = base
	}
	rename(s, name, newName)
	return s, nil
}

// rename меняет имя символа и имена его подсимволов NAME_u_s.
func rename(s *Node, old, nu string) {
	s.Kids[1] = Q(nu)
	for _, k := range s.All("symbol") {
		sub := k.Arg(0)
		for _, pref := range []string{old + "_", nu + "_"} {
			if strings.HasPrefix(sub, pref) {
				k.Kids[1] = Q(nu + "_" + strings.TrimPrefix(sub, pref))
				break
			}
		}
		// подсимвол мог остаться с именем базы (extends) — берём хвост _u_s
		if !strings.HasPrefix(k.Arg(0), nu+"_") {
			p := strings.Split(k.Arg(0), "_")
			if len(p) >= 3 {
				k.Kids[1] = Q(nu + "_" + p[len(p)-2] + "_" + p[len(p)-1])
			}
		}
	}
}

// Pin — вывод символа в координатах библиотеки (y вверх).
type Pin struct {
	Num, Name, Type string
	Unit            int
	X, Y            float64
	Angle           float64 // куда смотрит вывод от точки подключения
	Len             float64
	Hidden          bool
}

// Pins — все выводы символа (единица 0 = общие для всех частей), только основной стиль.
func Pins(s *Node) []Pin {
	var r []Pin
	name := s.Arg(0)
	for _, sub := range s.All("symbol") {
		p := strings.Split(strings.TrimPrefix(sub.Arg(0), name+"_"), "_")
		if len(p) != 2 {
			continue
		}
		unit, style := atoi(p[0]), atoi(p[1])
		if style > 1 {
			continue
		}
		for _, pn := range sub.All("pin") {
			at := pn.Find("at")
			pin := Pin{
				Type: pn.Arg(0), Unit: unit,
				X: at.Num(0), Y: at.Num(1), Angle: at.Num(2),
			}
			if ln := pn.Find("length"); ln != nil {
				pin.Len = ln.Num(0)
			}
			if nm := pn.Find("name"); nm != nil {
				pin.Name = nm.Arg(0)
			}
			if nb := pn.Find("number"); nb != nil {
				pin.Num = nb.Arg(0)
			}
			for _, k := range pn.Kids {
				if !k.IsList() && k.Atom == "hide" {
					pin.Hidden = true
				}
				if k.Head() == "hide" && k.Arg(0) == "yes" {
					pin.Hidden = true
				}
			}
			r = append(r, pin)
		}
	}
	sort.SliceStable(r, func(i, j int) bool { return r[i].Unit < r[j].Unit })
	return r
}

func atoi(s string) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return -1
		}
		n = n*10 + int(c-'0')
	}
	return n
}

// Pt — точка на листе, мм (y вниз).
type Pt struct{ X, Y float64 }

func (p Pt) Add(dx, dy float64) Pt { return Pt{p.X + dx, p.Y + dy} }
func (p Pt) eq(q Pt) bool          { return math.Abs(p.X-q.X) < 0.01 && math.Abs(p.Y-q.Y) < 0.01 }

// xform переводит точку библиотеки (y вверх) в лист: at + поворот rot (против часовой, градусы).
func xform(at Pt, rot int, x, y float64) Pt {
	dx, dy := x, -y
	for i := 0; i < ((rot/90)%4+4)%4; i++ {
		dx, dy = dy, -dx
	}
	return Pt{round(at.X + dx), round(at.Y + dy)}
}

func round(f float64) float64 { return math.Round(f*1e4) / 1e4 }
