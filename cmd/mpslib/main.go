// mpslib собирает masters/lib/mps.kicad_sym из стандартных библиотек KiCad:
// берёт нужные символы, раскрывает extends, переименовывает под нашу элементную базу
// и правит то, чего в KiCad нет (IDT7005 из IDT7006PF).
//
//	go run ./cmd/mpslib -kicad /Applications/KiCad/KiCad.app/Contents/SharedSupport/symbols
//	go run ./cmd/mpslib -kicad … -dump        # только показать выводы
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"mpskp/internal/schgen"
)

type src struct{ lib, name, as, value, desc string }

var list = []src{
	{"MCU_Microchip_8051", "AT89x51xxP", "AT89S53", "AT89S53", "МК 8051, 12 КБ Flash, 256 Б ОЗУ, DIP-40"},
	{"74xx", "74LS573", "74HC573", "74AC573", "8 D-защёлок с третьим состоянием (КР1554ИР22)"},
	{"74xx", "74HC173", "74HC173", "74HC173", "4 D-триггера с третьим состоянием"},
	{"74xx", "74HC244", "74HC244", "74AC244", "2×4 буфера с третьим состоянием (КР1554АП4)"},
	{"74xx", "74HC02", "74HC02", "74HC02", "4 элемента 2ИЛИ-НЕ (КР1554ЛЕ1)"},
	{"74xx", "74LS32", "74HC32", "74HC32", "4 элемента 2ИЛИ (КР1554ЛЛ1)"},
	{"74xx", "74LS11", "74HC11", "74HC11", "3 элемента 3И (КР1554ЛИ3)"},
	{"74xx", "74LS21", "74HC21", "74HC21", "2 элемента 4И (КР1554ЛИ6)"},
	{"Memory_RAM", "IDT7006PF", "IDT7005", "IDT7005S55PF", "Двухпортовое статическое ОЗУ 8K×8, TQFP-64"},
	{"Display_Character", "KCSC02-105", "FYS-5612AX", "FYS-5612AX", "7-сегм. индикатор, общий катод"},
	{"Display_Character", "KCSA02-105", "FYS-5612BX", "FYS-5612BX", "7-сегм. индикатор, общий анод"},
	{"Device", "R", "R", "R", ""},
	{"Device", "C", "C", "C", ""},
	{"Device", "C_Polarized", "C_Polarized", "C_Polarized", ""},
	{"Device", "Crystal", "Crystal", "Crystal", ""},
	{"Device", "D", "D", "КД521А", ""},
	{"Switch", "SW_Push", "SW_Push", "SW_Push", ""},
	{"power", "+5V", "+5V", "+5V", ""},
	{"power", "GND", "GND", "GND", ""},
}

func main() {
	dir := flag.String("kicad", "", "каталог symbols из поставки KiCad")
	out := flag.String("out", "masters/lib/mps.kicad_sym", "куда писать")
	dump := flag.Bool("dump", false, "только вывести выводы")
	flag.Parse()
	if err := run(*dir, *out, *dump); err != nil {
		fmt.Fprintln(os.Stderr, "mpslib:", err)
		os.Exit(1)
	}
}

func run(dir, out string, dump bool) error {
	libs := map[string]*schgen.Lib{}
	root := schgen.L("kicad_symbol_lib",
		schgen.L("version", schgen.A("20251024")),
		schgen.L("generator", schgen.Q("mpslib")),
		schgen.L("generator_version", schgen.Q("10.0")))
	for _, s := range list {
		l, ok := libs[s.lib]
		if !ok {
			var err error
			l, err = schgen.LoadLib(filepath.Join(dir, s.lib+".kicad_sym"))
			if err != nil {
				return err
			}
			libs[s.lib] = l
		}
		sym, err := l.Flat(s.name, s.as)
		if err != nil {
			return fmt.Errorf("%s:%s: %w", s.lib, s.name, err)
		}
		setProp(sym, "Value", s.value)
		if s.desc != "" {
			setProp(sym, "Description", s.desc)
		}
		if s.as == "IDT7005" {
			idt7005(sym)
		}
		if s.as == "AT89S53" {
			// имена выводов без подстрочных индексов (типовое замечание)
			sym.Walk(func(n *schgen.Node) {
				if n.Head() == "name" {
					switch n.Arg(0) {
					case "V_{cc}":
						n.Kids[1] = schgen.Q("VCC")
					case "~{EA}/V_{pp}":
						n.Kids[1] = schgen.Q("~{EA}/VPP")
					}
				}
			})
		}
		switch s.as {
		case "74HC02", "74HC32", "74HC11", "74HC21":
			hidePower(sym)
		}
		if s.as == "GND" {
			gostGND(sym)
		}
		if dump {
			fmt.Printf("== %s (%s:%s)\n", s.as, s.lib, s.name)
			for _, p := range schgen.Pins(sym) {
				fmt.Printf("  u%d %-4s %-10s %-14s (%g,%g) a%g l%g\n", p.Unit, p.Num, p.Name, p.Type, p.X, p.Y, p.Angle, p.Len)
			}
			continue
		}
		root.Kids = append(root.Kids, sym)
	}
	if dump {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return err
	}
	return os.WriteFile(out, []byte(root.String()), 0o644)
}

func setProp(sym *schgen.Node, key, val string) {
	for _, p := range sym.All("property") {
		if p.Arg(0) == key {
			p.Kids[2] = schgen.Q(val)
		}
	}
}

// idt7005: у IDT7006 (16K) в TQFP-64 выводы 23 и 58 — A13R/A13L, у IDT7005 (8K) они N/C.
// Остальные номера совпадают (сверено по даташиту IDT7005S, стр. 2).
func idt7005(sym *schgen.Node) {
	sym.Walk(func(n *schgen.Node) {
		if n.Head() != "symbol" {
			return
		}
		n.Remove(func(k *schgen.Node) bool {
			if k.Head() != "pin" {
				return false
			}
			nb := k.Find("number")
			return nb != nil && (nb.Arg(0) == "23" || nb.Arg(0) == "58")
		})
		// GND 5/9/24/41 в символе KiCad стоят в одной точке — разносим, чтобы были видны
		// все номера (как у VCC 8/13/57); 9/24/41 там passive — делаем power_in;
		// BUSY: в нашем режиме slave (M/S=0) — вход запрета записи
		for _, k := range n.All("pin") {
			nm := k.Find("name")
			if nm != nil && nm.Arg(0) == "GND" {
				k.Kids[1] = schgen.A("power_in")
				x := map[string]float64{"5": -3.81, "9": -1.27, "24": 1.27, "41": 3.81}[k.Find("number").Arg(0)]
				k.Find("at").Kids[1] = schgen.F(x)
			}
			if nm != nil && strings.Contains(nm.Arg(0), "BUSY") {
				k.Kids[1] = schgen.A("input")
			}
		}
	})
	for _, p := range sym.All("property") {
		if p.Arg(0) == "Datasheet" {
			p.Kids[2] = schgen.Q("https://www.renesas.com/us/en/document/dst/7005-datasheet")
		}
		if p.Arg(0) == "ki_keywords" {
			p.Kids[2] = schgen.Q(strings.ReplaceAll(p.Arg(1), "16K", "8K"))
		}
	}
}

// gostGND — «земля» как в принятой схеме: короткая ножка и толстая черта (ГОСТ 2.721).
func gostGND(sym *schgen.Node) {
	for _, sub := range sym.All("symbol") {
		if sub.Arg(0) != "GND_0_1" {
			continue
		}
		sub.Kids = sub.Kids[:2]
		stroke := func(w float64) *schgen.Node {
			return schgen.L("stroke", schgen.L("width", schgen.F(w)), schgen.L("type", schgen.A("default")))
		}
		sub.Kids = append(sub.Kids,
			schgen.L("polyline",
				schgen.L("pts", schgen.L("xy", schgen.F(0), schgen.F(0)), schgen.L("xy", schgen.F(0), schgen.F(-1.905))),
				stroke(0), schgen.L("fill", schgen.L("type", schgen.A("none")))),
			schgen.L("rectangle",
				schgen.L("start", schgen.F(-1.905), schgen.F(-1.905)),
				schgen.L("end", schgen.F(1.905), schgen.F(-2.54)),
				stroke(0), schgen.L("fill", schgen.L("type", schgen.A("outline")))))
	}
}

// hidePower: у многосекционных вентилей часть «питание» (VCC 14, GND 7) убираем,
// а выводы питания делаем общими скрытыми с именами +5V/GND — KiCad сам соединит их
// с цепями +5V и GND (как «вывод 14 к +5 В, 7 на GND» в примечании схемы).
func hidePower(sym *schgen.Node) {
	name := sym.Arg(0)
	var pwr []*schgen.Node
	var keep []*schgen.Node
	for _, k := range sym.Kids {
		if k.Head() == "symbol" {
			isPwr := false
			for _, p := range k.All("pin") {
				if p.Arg(0) == "power_in" {
					isPwr = true
				}
			}
			if isPwr {
				for _, p := range k.All("pin") {
					nm := p.Find("name")
					if nm.Arg(0) == "VCC" {
						nm.Kids[1] = schgen.Q("+5V")
					}
					p.Kids = append(p.Kids, schgen.L("hide", schgen.A("yes")))
					pwr = append(pwr, p)
				}
				continue
			}
		}
		keep = append(keep, k)
	}
	sym.Kids = keep
	sym.Kids = append(sym.Kids, append([]*schgen.Node{schgen.L("symbol", schgen.Q(name+"_0_0"))}, nil...)[0])
	common := sym.Kids[len(sym.Kids)-1]
	common.Kids = append(common.Kids, pwr...)
	// embedded_fonts должен идти последним
	for i, k := range sym.Kids {
		if k.Head() == "embedded_fonts" {
			sym.Kids = append(append(sym.Kids[:i:i], sym.Kids[i+1:]...), k)
			break
		}
	}
}
