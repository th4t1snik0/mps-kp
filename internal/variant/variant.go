// Package variant считает все параметры курсовой по группе и номеру M.
// Числа из ТЗ лежат в data/table-<год>.yaml, правила (k = M mod 7 и т.п.) — здесь.
package variant

import (
	"fmt"
	"os"
	"regexp"
	"strconv"

	"gopkg.in/yaml.v3"
)

// Linear — формула вида base + per_m*M.
type Linear struct {
	Base int `yaml:"base"`
	PerM int `yaml:"per_m"`
}

func (l Linear) At(m int) int { return l.Base + l.PerM*m }

// ByParity — значение, зависящее от чётности M.
type ByParity struct {
	Odd  string `yaml:"odd"`
	Even string `yaml:"even"`
}

func (p ByParity) At(m int) string {
	if m%2 == 0 {
		return p.Even
	}
	return p.Odd
}

type GroupRow struct {
	T1ms      Linear   `yaml:"t1_ms"`
	Y1Timer   int      `yaml:"y1_timer"`
	CSBuf     string   `yaml:"cs_buf"`
	VBytes    Linear   `yaml:"v_bytes"`
	HeadAddr  int      `yaml:"head_addr"`
	TailAddr  int      `yaml:"tail_addr"`
	EmptyBit  Linear   `yaml:"empty_bit"`
	OvfBit    Linear   `yaml:"ovf_bit"`
	CSY2      string   `yaml:"cs_y2"`
	Y2Timer   int      `yaml:"y2_timer"`
	T2us      Linear   `yaml:"t2_us"`
	CSInd     ByParity `yaml:"cs_ind"`
	T3ms      Linear   `yaml:"t3_ms"`
	T3Timer   int      `yaml:"t3_timer"`
	Indicator string   `yaml:"indicator"`
	CSKb      string   `yaml:"cs_kb"`
	KbInt     string   `yaml:"kb_int"`
}

type Table struct {
	Year          int                 `yaml:"year"`
	CrystalMHz    int                 `yaml:"crystal_mhz"`
	X2MaxRatePerS int                 `yaml:"x2_max_rate_per_s"`
	Groups        map[string]GroupRow `yaml:"groups"`
}

func LoadTable(path string) (*Table, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var t Table
	if err := yaml.Unmarshal(b, &t); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if t.CrystalMHz == 0 {
		t.CrystalMHz = 12
	}
	if t.X2MaxRatePerS == 0 {
		t.X2MaxRatePerS = 5
	}
	return &t, nil
}

// Student — то, что человек пишет в students/<ник>/variant.yaml.
type Student struct {
	Group     string `yaml:"group"`      // «А-12» — ключ в таблице
	GroupFull string `yaml:"group_full"` // «А-12-22» — для рамки
	M         int    `yaml:"m"`
	Name      string `yaml:"name"`    // «Рязанцев И.В.»
	Checker   string `yaml:"checker"` // «Михалин С.Н.»
	Date      string `yaml:"date"`
}

func LoadStudent(path string) (*Student, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var s Student
	if err := yaml.Unmarshal(b, &s); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if s.GroupFull == "" {
		s.GroupFull = s.Group
	}
	return &s, nil
}

// Pin — вывод порта МК, например P2.5.
type Pin struct {
	Port, Bit int
}

var pinRe = regexp.MustCompile(`^P([0-3])\.([0-7])$`)

func ParsePin(s string) (Pin, error) {
	m := pinRe.FindStringSubmatch(s)
	if m == nil {
		return Pin{}, fmt.Errorf("не похоже на вывод порта: %q (нужно вида P2.5)", s)
	}
	p, _ := strconv.Atoi(m[1])
	b, _ := strconv.Atoi(m[2])
	return Pin{p, b}, nil
}

func (p Pin) String() string { return fmt.Sprintf("P%d.%d", p.Port, p.Bit) }

// Device — внешнее устройство на шине.
type Device struct {
	Name    string `json:"name"`
	CS      string `json:"cs"`
	Base    int    `json:"base"`     // базовый адрес окна
	WinSize int    `json:"win_size"` // размер окна, байт
}

type Bit struct {
	Addr int    `json:"addr"` // адрес бита в bit-области
	Byte int    `json:"byte"` // байт 20h..2Fh
	Pos  int    `json:"pos"`  // разряд 0..7
	Asm  string `json:"asm"`  // запись вида 23h.2
}

func makeBit(a int) Bit {
	return Bit{Addr: a, Byte: 0x20 + a/8, Pos: a % 8, Asm: fmt.Sprintf("%s.%d", Hex(0x20+a/8), a%8)}
}

// Interval — длинный интервал, который отсчитывается тиками таймера.
type Interval struct {
	Ms      int `json:"ms"`
	TickMs  int `json:"tick_ms"`
	Ticks   int `json:"ticks"`
	Reload  int `json:"reload"` // TH:TL для одного тика
	Counter int `json:"counter_bits"`
}

type Params struct {
	Year    int    `json:"year"`
	Group   string `json:"group"`
	M       int    `json:"m"`
	K       int    `json:"k"`
	Student string `json:"student"`
	Checker string `json:"checker"`

	Keyboard  string `json:"keyboard"` // "4x3" | "3x4" (столбцы × строки)
	Cols      int    `json:"cols"`
	Rows      int    `json:"rows"`
	Indicator string `json:"indicator"` // cathode | anode | ""

	Y1Pin, Y2Pin string
	KbInt, X2Int string // INT0 / INT1
	KbIntPin     string // P3.2 / P3.3
	X2IntPin     string

	T1ms, T2us, T3ms int
	V                int
	Y1Timer, Y2Timer int
	T3Timer          int
	T2Reload         int

	Y1, T3 Interval

	Head, Tail  int
	Empty, Ovf  Bit
	Devices     []Device
	Prog2       string
	Prog3       string
	FillTimeS   float64
	CycleUs     float64
	UnusedCS    string
	Warnings    []string
	CrystalMHz  int
	MinFillNote string
}

var intPin = map[string]string{"INT0": "P3.2", "INT1": "P3.3"}

func Compute(t *Table, s *Student) (*Params, error) {
	g, ok := t.Groups[s.Group]
	if !ok {
		keys := ""
		for k := range t.Groups {
			keys += " " + k
		}
		return nil, fmt.Errorf("группы %q нет в таблице %d (есть:%s)", s.Group, t.Year, keys)
	}
	if s.M < 1 || s.M > 30 {
		return nil, fmt.Errorf("M=%d вне диапазона 1..30", s.M)
	}
	m := s.M
	p := &Params{Year: t.Year, Group: s.Group, M: m, K: m % 7, Student: s.Name, Checker: s.Checker, CrystalMHz: t.CrystalMHz}

	if p.K+1 > 7 {
		return nil, fmt.Errorf("k+1 > 7")
	}
	p.Y1Pin = fmt.Sprintf("P1.%d", p.K)
	p.Y2Pin = fmt.Sprintf("P1.%d", p.K+1)

	if m%2 == 0 {
		p.Keyboard, p.Cols, p.Rows = "4x3", 4, 3
		p.Prog2 = "чтение из кольцевого буфера"
	} else {
		p.Keyboard, p.Cols, p.Rows = "3x4", 3, 4
		p.Prog2 = "запись в кольцевой буфер"
	}
	switch m % 3 {
	case 0:
		p.Prog3 = "вывод символа на индикатор + гашение по T3 (таймер)"
	case 1:
		p.Prog3 = "строб Y1 длительностью T1 (таймер)"
	case 2:
		p.Prog3 = "расчёт Y2 = (M+X1+X2) mod 256, запись в регистр, строб T2 (таймер)"
	}

	p.Indicator = g.Indicator
	if p.Indicator == "" {
		p.Warnings = append(p.Warnings, "тип индикатора (общий катод/анод) для группы не заполнен в таблице — проверь табл. 1 ТЗ")
	}

	p.KbInt = g.KbInt
	switch g.KbInt {
	case "INT0":
		p.X2Int = "INT1"
	case "INT1":
		p.X2Int = "INT0"
	default:
		return nil, fmt.Errorf("kb_int должен быть INT0 или INT1, а не %q", g.KbInt)
	}
	p.KbIntPin, p.X2IntPin = intPin[p.KbInt], intPin[p.X2Int]

	p.T1ms, p.T2us, p.T3ms, p.V = g.T1ms.At(m), g.T2us.At(m), g.T3ms.At(m), g.VBytes.At(m)
	p.Y1Timer, p.Y2Timer, p.T3Timer = g.Y1Timer, g.Y2Timer, g.T3Timer
	if p.Y1Timer == p.Y2Timer {
		p.Warnings = append(p.Warnings, "Y1 и Y2 на одном таймере — так не бывает, проверь таблицу")
	}

	p.CycleUs = 12.0 / float64(t.CrystalMHz)
	t2cycles := int(float64(p.T2us)/p.CycleUs + 0.5)
	if t2cycles > 65536 {
		return nil, fmt.Errorf("T2=%d мкс не влезает в один отсчёт 16-битного таймера", p.T2us)
	}
	p.T2Reload = 65536 - t2cycles

	tick := pickTick(p.T1ms, p.T3ms, p.CycleUs)
	p.Y1 = interval(p.T1ms, tick, p.CycleUs)
	p.T3 = interval(p.T3ms, tick, p.CycleUs)

	p.Head, p.Tail = g.HeadAddr, g.TailAddr
	p.Empty, p.Ovf = makeBit(g.EmptyBit.At(m)), makeBit(g.OvfBit.At(m))
	if p.Empty.Addr > 0x7F || p.Ovf.Addr > 0x7F {
		return nil, fmt.Errorf("адрес бита флага вне bit-области")
	}

	// Устройства: 4 CS на P2.3..P2.7, одна линия лишняя.
	cs := map[string]string{"Буфер (IDT7005)": g.CSBuf, "Регистр Y2": g.CSY2, "Индикатор": g.CSInd.At(m), "Клавиатура": g.CSKb}
	used := map[int]string{}
	for _, name := range []string{"Индикатор", "Клавиатура", "Буфер (IDT7005)", "Регистр Y2"} {
		pin, err := ParsePin(cs[name])
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		if pin.Port != 2 || pin.Bit < 3 {
			return nil, fmt.Errorf("%s: CS %s — ожидается P2.3..P2.7 (P2.0..P2.2 заняты адресом A8..A10)", name, pin)
		}
		if other, dup := used[pin.Bit]; dup {
			return nil, fmt.Errorf("CS %s занят и у «%s», и у «%s»", pin, other, name)
		}
		used[pin.Bit] = name
		// старший байт: F8h со сброшенным битом CS, младшие 3 бита — адрес A8..A10
		hi := 0xF8 &^ (1 << pin.Bit)
		p.Devices = append(p.Devices, Device{Name: name, CS: pin.String(), Base: hi << 8, WinSize: 2048})
	}
	for b := 3; b <= 7; b++ {
		if _, ok := used[b]; !ok {
			p.UnusedCS = fmt.Sprintf("P2.%d", b)
		}
	}
	if p.V > 2048 {
		return nil, fmt.Errorf("V=%d > 2048: буфер не влезает в 11 адресных линий", p.V)
	}

	p.FillTimeS = float64(p.V) / float64(t.X2MaxRatePerS)
	return p, nil
}

// pickTick выбирает самый крупный тик (мс), на который делятся T1 и T3
// и который помещается в 16-битный таймер.
func pickTick(t1, t3 int, cycleUs float64) int {
	g := gcd(t1, t3)
	maxTick := int(65536 * cycleUs / 1000) // 65 мс при 12 МГц
	best := 1
	for d := 1; d <= g && d <= maxTick; d++ {
		if g%d == 0 {
			best = d
		}
	}
	return best
}

func interval(ms, tick int, cycleUs float64) Interval {
	cycles := int(float64(tick)*1000/cycleUs + 0.5)
	iv := Interval{Ms: ms, TickMs: tick, Ticks: ms / tick, Reload: 65536 - cycles, Counter: 8}
	if iv.Ticks > 255 {
		iv.Counter = 16
	}
	return iv
}

func gcd(a, b int) int {
	for b != 0 {
		a, b = b, a%b
	}
	return a
}

// Hex — число в стиле ассемблера 8051: 0D800h, 42h.
func Hex(v int) string {
	s := fmt.Sprintf("%X", v)
	if len(s)%2 == 1 && v > 0xFF {
		s = "0" + s
	}
	if s[0] >= 'A' && s[0] <= 'F' {
		s = "0" + s
	}
	return s + "h"
}

// Device по имени.
func (p *Params) Dev(name string) Device {
	for _, d := range p.Devices {
		if d.Name == name {
			return d
		}
	}
	return Device{}
}
