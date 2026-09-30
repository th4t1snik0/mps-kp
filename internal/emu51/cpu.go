// Package emu51 — эмулятор 8051/8052 для проверки программ курсовой прямо в процессе Go (без внешнего симулятора).
//
// Что есть: все команды 8051 с тактами по машинным циклам (12 МГц → 1 МЦ = 1 мкс), банки регистров, флаги
// (CY, AC, OV, P), таймеры 0 и 1 (режимы 0–3, GATE), внешние прерывания INT0/INT1 (уровень/спад), два уровня
// приоритета, порты с внешними выводами (чтение-модификация-запись — по защёлке), 256 байт внутреннего ОЗУ (8052),
// 64 КБ внешнего ОЗУ с подключаемыми устройствами. Чего нет: UART, Timer2 (ТЗ запрещает), режим счётчика по T0/T1,
// idle/power-down.
//
// Эталон поведения — ucsim s51 0.9.9 (SDCC 4.6.0): сверка в cpu_test.go (случайные программы и эталоны чекера,
// запускается, если s51 установлен).
package emu51

import "math/rand"

// SFR-адреса.
const (
	P0   = 0x80
	SP   = 0x81
	DPL  = 0x82
	DPH  = 0x83
	PCON = 0x87
	TCON = 0x88
	TMOD = 0x89
	TL0  = 0x8A
	TL1  = 0x8B
	TH0  = 0x8C
	TH1  = 0x8D
	P1   = 0x90
	SCON = 0x98
	P2   = 0xA0
	IE   = 0xA8
	P3   = 0xB0
	IP   = 0xB8
	PSW  = 0xD0
	ACC  = 0xE0
	B    = 0xF0
)

// CPU — состояние МК.
type CPU struct {
	Code [65536]byte
	IRAM [256]byte
	SFR  [128]byte // 80h…FFh
	XRAM [65536]byte
	PC   uint16

	Cycles uint64 // машинные циклы с момента сброса

	// Pins — внешнее состояние выводов портов (1 — никто снаружи не тянет, читается защёлка).
	Pins [4]byte

	// Устройства на шине movx. Read: ok = false — читается XRAM.
	OnXWrite func(addr uint16, v byte)
	OnXRead  func(addr uint16) (v byte, ok bool)
	// OnPort — запись в защёлку порта n (0…3), old → new.
	OnPort func(n int, old, new byte)

	// VectorCycles — МЦ на аппаратный вызов вектора прерывания: 2 (LCALL, как в даташите 8051; по умолчанию).
	// ucsim s51 тратит 1 — для сверки с ним в тестах ставится 1.
	VectorCycles int

	active   [2]bool // выполняется обработчик уровня 0/1
	prevPin  [2]bool // INT0/INT1 в прошлом цикле (для спада)
	holdInt  bool    // после RETI и записи в IE/IP одна команда выполняется до прерывания
	lastPort [4]byte
}

// New — МК после сброса. seed — случайное содержимое внутреннего ОЗУ (как у настоящего и у s51).
func New(seed int64) *CPU {
	c := &CPU{VectorCycles: 2}
	r := rand.New(rand.NewSource(seed))
	for i := range c.IRAM {
		c.IRAM[i] = byte(r.Intn(256))
	}
	c.Reset()
	return c
}

// Reset — сброс: SFR по даташиту, PC = 0; ОЗУ не трогается.
func (c *CPU) Reset() {
	c.SFR = [128]byte{}
	for _, p := range []int{P0, P1, P2, P3} {
		c.SFR[p-0x80] = 0xFF
	}
	c.SFR[SP-0x80] = 0x07
	c.Pins = [4]byte{0xFF, 0xFF, 0xFF, 0xFF}
	c.lastPort = [4]byte{0xFF, 0xFF, 0xFF, 0xFF}
	c.PC, c.Cycles = 0, 0
	c.active = [2]bool{}
	c.prevPin = [2]bool{true, true}
}

// Micros — время с момента сброса при кварце 12 МГц, мкс.
func (c *CPU) Micros() float64 { return float64(c.Cycles) }

// ------------------------------------------------------------------ память

func (c *CPU) sfr(a byte) byte   { return c.SFR[a-0x80] }
func (c *CPU) setSFR(a, v byte)  { c.writeDirect(a, v) }
func (c *CPU) A() byte           { return c.SFR[ACC-0x80] }
func (c *CPU) setA(v byte)       { c.SFR[ACC-0x80] = v }
func (c *CPU) psw() byte         { return c.SFR[PSW-0x80] }
func (c *CPU) cy() byte          { return c.psw() >> 7 }
func (c *CPU) dptr() uint16      { return uint16(c.sfr(DPH))<<8 | uint16(c.sfr(DPL)) }
func (c *CPU) setDPTR(v uint16)  { c.SFR[DPH-0x80], c.SFR[DPL-0x80] = byte(v>>8), byte(v) }
func (c *CPU) rAddr(n byte) byte { return c.psw()&0x18 | n }
func (c *CPU) reg(n byte) byte   { return c.IRAM[c.rAddr(n)] }
func (c *CPU) setReg(n, v byte)  { c.IRAM[c.rAddr(n)] = v }
func (c *CPU) setFlag(m byte, on bool) {
	if on {
		c.SFR[PSW-0x80] |= m
	} else {
		c.SFR[PSW-0x80] &^= m
	}
}

func portIndex(a byte) int {
	switch a {
	case P0:
		return 0
	case P1:
		return 1
	case P2:
		return 2
	case P3:
		return 3
	}
	return -1
}

// readDirect — прямая адресация. rmw — команда «чтение-модификация-запись»: порт читается по защёлке.
func (c *CPU) readDirect(a byte, rmw bool) byte {
	if a < 0x80 {
		return c.IRAM[a]
	}
	if n := portIndex(a); n >= 0 && !rmw {
		return c.SFR[a-0x80] & c.Pins[n]
	}
	return c.SFR[a-0x80]
}

func (c *CPU) writeDirect(a, v byte) {
	if a < 0x80 {
		c.IRAM[a] = v
		return
	}
	old := c.SFR[a-0x80]
	c.SFR[a-0x80] = v
	if a == IE || a == IP {
		c.holdInt = true
	}
	if n := portIndex(a); n >= 0 && c.OnPort != nil && old != v {
		c.OnPort(n, old, v)
	}
}

// Latch — защёлка порта n (что выводит МК).
func (c *CPU) Latch(n int) byte { return c.SFR[[]int{P0, P1, P2, P3}[n]-0x80] }

// SFRByte — любой SFR по адресу 80h…FFh.
func (c *CPU) SFRByte(a byte) byte { return c.SFR[a-0x80] }

// Bit — бит по адресу бита 8051 (чтение защёлки у портов).
func (c *CPU) Bit(b byte) bool { return c.readBit(b, true) }

// SetBit — бит по адресу бита.
func (c *CPU) SetBit(b byte, v bool) { c.writeBit(b, v) }

// SetA — аккумулятор (для вызова процедуры с данными).
func (c *CPU) SetA(v byte) { c.setA(v) }

func bitByte(b byte) byte {
	if b < 0x80 {
		return 0x20 + b/8
	}
	return b & 0xF8
}

func (c *CPU) readBit(b byte, rmw bool) bool {
	return c.readDirect(bitByte(b), rmw)&(1<<(b&7)) != 0
}

func (c *CPU) writeBit(b byte, v bool) {
	a := bitByte(b)
	x := c.readDirect(a, true)
	if v {
		x |= 1 << (b & 7)
	} else {
		x &^= 1 << (b & 7)
	}
	c.writeDirect(a, x)
}

func (c *CPU) push(v byte) {
	sp := c.sfr(SP) + 1
	c.SFR[SP-0x80] = sp
	c.IRAM[sp] = v
}

func (c *CPU) pop() byte {
	sp := c.sfr(SP)
	c.SFR[SP-0x80] = sp - 1
	return c.IRAM[sp]
}

func (c *CPU) xread(a uint16) byte {
	if c.OnXRead != nil {
		if v, ok := c.OnXRead(a); ok {
			return v
		}
	}
	return c.XRAM[a]
}

func (c *CPU) xwrite(a uint16, v byte) {
	c.XRAM[a] = v
	if c.OnXWrite != nil {
		c.OnXWrite(a, v)
	}
}

func (c *CPU) fetch() byte {
	v := c.Code[c.PC]
	c.PC++
	return v
}

func (c *CPU) rel(off byte) { c.PC = uint16(int(c.PC) + int(int8(off))) }

// ------------------------------------------------------------------ арифметика

func (c *CPU) add(v byte, carry byte) {
	a := c.A()
	r := uint16(a) + uint16(v) + uint16(carry)
	c.setFlag(0x80, r > 0xFF)
	c.setFlag(0x40, (a&0x0F)+(v&0x0F)+carry > 0x0F)
	c.setFlag(0x04, (a^v)&0x80 == 0 && (a^byte(r))&0x80 != 0)
	c.setA(byte(r))
}

func (c *CPU) subb(v byte) {
	a := c.A()
	cy := c.cy()
	r := int(a) - int(v) - int(cy)
	c.setFlag(0x80, r < 0)
	c.setFlag(0x40, int(a&0x0F)-int(v&0x0F)-int(cy) < 0)
	c.setFlag(0x04, (a^v)&0x80 != 0 && (a^byte(r))&0x80 != 0)
	c.setA(byte(r))
}

func (c *CPU) parity() {
	a := c.A()
	a ^= a >> 4
	a ^= a >> 2
	a ^= a >> 1
	c.setFlag(0x01, a&1 != 0)
}
