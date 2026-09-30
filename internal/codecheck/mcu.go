package codecheck

import "mpskp/internal/emu51"

// mcu — эмулятор 8051 с интерфейсом, удобным сценариям (адреса и значения — int/byte).
type mcu struct{ c *emu51.CPU }

func (m mcu) A() byte                    { return m.c.A() }
func (m mcu) SetA(v byte)                { m.c.SetA(v) }
func (m mcu) PC() int                    { return int(m.c.PC) }
func (m mcu) SetPC(a int)                { m.c.PC = uint16(a) }
func (m mcu) Micros() float64            { return m.c.Micros() }
func (m mcu) IRAM(a int) byte            { return m.c.IRAM[a] }
func (m mcu) SetIRAM(a int, v ...byte)   { copy(m.c.IRAM[a:], v) }
func (m mcu) XRAM(a int) byte            { return m.c.XRAM[uint16(a)] }
func (m mcu) SetXRAM(a int, v ...byte)   { copy(m.c.XRAM[a:], v) }
func (m mcu) SFR(a int) byte             { return m.c.SFRByte(byte(a)) }
func (m mcu) Latch(n int) byte           { return m.c.Latch(n) }
func (m mcu) SetPins(n int, v byte)      { m.c.Pins[n] = v }
func (m mcu) Bit(b int) bool             { return m.c.Bit(byte(b)) }
func (m mcu) SetBit(b int, v bool)       { m.c.SetBit(byte(b), v) }
func (m mcu) BreakMem(string, byte, int) {} // события приходят из эмулятора сами (OnXWrite, OnPort)
