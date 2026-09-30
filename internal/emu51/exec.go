package emu51

// Step выполняет одну команду (и, если пора, вход в прерывание). Возвращает машинные циклы.
func (c *CPU) Step() int {
	c.holdInt = false
	n := c.exec()
	c.tick(n)
	if !c.holdInt { // после RETI и записи в IE/IP — сначала ещё одна команда
		if m := c.interrupt(); m > 0 {
			if c.VectorCycles == 1 {
				c.Cycles++ // как s51: вход в вектор — 1 МЦ, таймеры в нём не считают
			} else {
				c.tick(m)
			}
			n += m
		}
	}
	c.parity()
	return n
}

func (c *CPU) exec() int {
	op := c.fetch()
	lo := op & 0x0F
	// операнд младших кодов: 4 — #/A, 5 — dir, 6–7 — @Ri, 8–F — Rn
	src := func() byte {
		switch {
		case lo >= 8:
			return c.reg(lo - 8)
		case lo >= 6:
			return c.IRAM[c.reg(lo-6)]
		case lo == 5:
			return c.readDirect(c.fetch(), false)
		}
		return c.fetch() // #imm
	}
	switch {
	case op&0x1F == 0x01: // AJMP
		a := c.fetch()
		c.PC = c.PC&0xF800 | uint16(op>>5)<<8 | uint16(a)
		return 2
	case op&0x1F == 0x11: // ACALL
		a := c.fetch()
		c.push(byte(c.PC))
		c.push(byte(c.PC >> 8))
		c.PC = c.PC&0xF800 | uint16(op>>5)<<8 | uint16(a)
		return 2
	}
	hi := op >> 4
	if lo >= 4 && hi >= 2 && hi <= 9 && hi != 7 && hi != 8 { // ADD ADDC ORL ANL XRL SUBB — A, операнд
		v := src()
		switch hi {
		case 2:
			c.add(v, 0)
		case 3:
			c.add(v, c.cy())
		case 4:
			c.setA(c.A() | v)
		case 5:
			c.setA(c.A() & v)
		case 6:
			c.setA(c.A() ^ v)
		case 9:
			c.subb(v)
		}
		return 1
	}
	switch op {
	case 0x00:
		return 1
	case 0x02:
		h, l := c.fetch(), c.fetch()
		c.PC = uint16(h)<<8 | uint16(l)
		return 2
	case 0x12:
		h, l := c.fetch(), c.fetch()
		c.push(byte(c.PC))
		c.push(byte(c.PC >> 8))
		c.PC = uint16(h)<<8 | uint16(l)
		return 2
	case 0x22, 0x32: // RET, RETI
		h := c.pop()
		l := c.pop()
		c.PC = uint16(h)<<8 | uint16(l)
		if op == 0x32 {
			if c.active[1] {
				c.active[1] = false
			} else {
				c.active[0] = false
			}
			c.holdInt = true
		}
		return 2
	case 0x03: // RR A
		a := c.A()
		c.setA(a>>1 | a<<7)
		return 1
	case 0x13: // RRC A
		a := c.A()
		c.setA(a>>1 | c.cy()<<7)
		c.setFlag(0x80, a&1 != 0)
		return 1
	case 0x23: // RL A
		a := c.A()
		c.setA(a<<1 | a>>7)
		return 1
	case 0x33: // RLC A
		a := c.A()
		c.setA(a<<1 | c.cy())
		c.setFlag(0x80, a&0x80 != 0)
		return 1
	case 0x04:
		c.setA(c.A() + 1)
		return 1
	case 0x14:
		c.setA(c.A() - 1)
		return 1
	case 0x05, 0x15: // INC/DEC dir
		a := c.fetch()
		d := byte(1)
		if op == 0x15 {
			d = 0xFF
		}
		c.writeDirect(a, c.readDirect(a, true)+d)
		return 1
	case 0x06, 0x07, 0x16, 0x17:
		r := c.reg(op & 1)
		d := byte(1)
		if op >= 0x16 {
			d = 0xFF
		}
		c.IRAM[r] += d
		return 1
	case 0x10, 0x20, 0x30: // JBC, JB, JNB
		b, off := c.fetch(), c.fetch()
		set := c.readBit(b, op == 0x10)
		if op == 0x10 && set {
			c.writeBit(b, false)
		}
		if set == (op != 0x30) {
			c.rel(off)
		}
		return 2
	case 0x40, 0x50, 0x60, 0x70, 0x80: // JC JNC JZ JNZ SJMP
		off := c.fetch()
		var j bool
		switch op {
		case 0x40:
			j = c.cy() == 1
		case 0x50:
			j = c.cy() == 0
		case 0x60:
			j = c.A() == 0
		case 0x70:
			j = c.A() != 0
		default:
			j = true
		}
		if j {
			c.rel(off)
		}
		return 2
	case 0x42, 0x52, 0x62: // ORL/ANL/XRL dir,A
		a := c.fetch()
		c.writeDirect(a, logic(op>>4, c.readDirect(a, true), c.A()))
		return 1
	case 0x43, 0x53, 0x63: // dir,#
		a, v := c.fetch(), c.fetch()
		c.writeDirect(a, logic(op>>4, c.readDirect(a, true), v))
		return 2
	case 0x72, 0x82, 0xA0, 0xB0: // ORL C,bit; ANL C,bit; ORL C,/bit; ANL C,/bit
		b := c.readBit(c.fetch(), false)
		if op == 0xA0 || op == 0xB0 {
			b = !b
		}
		cy := c.cy() == 1
		if op == 0x72 || op == 0xA0 {
			cy = cy || b
		} else {
			cy = cy && b
		}
		c.setFlag(0x80, cy)
		return 2
	case 0x73:
		c.PC = uint16(c.A()) + c.dptr()
		return 2
	case 0x74:
		c.setA(c.fetch())
		return 1
	case 0x75:
		a, v := c.fetch(), c.fetch()
		c.writeDirect(a, v)
		return 2
	case 0x76, 0x77:
		c.IRAM[c.reg(op&1)] = c.fetch()
		return 1
	case 0x83:
		c.setA(c.Code[uint16(c.A())+c.PC])
		return 2
	case 0x93:
		c.setA(c.Code[uint16(c.A())+c.dptr()])
		return 2
	case 0x84: // DIV AB
		a, b := c.A(), c.sfr(B)
		c.setFlag(0x80, false)
		if b == 0 {
			c.setFlag(0x04, true)
			return 4
		}
		c.setA(a / b)
		c.SFR[B-0x80] = a % b
		c.setFlag(0x04, false)
		return 4
	case 0xA4: // MUL AB
		r := uint16(c.A()) * uint16(c.sfr(B))
		c.setA(byte(r))
		c.SFR[B-0x80] = byte(r >> 8)
		c.setFlag(0x80, false)
		c.setFlag(0x04, r > 0xFF)
		return 4
	case 0x85: // MOV dir,dir (источник первым)
		s, d := c.fetch(), c.fetch()
		c.writeDirect(d, c.readDirect(s, false))
		return 2
	case 0x86, 0x87:
		d := c.fetch()
		c.writeDirect(d, c.IRAM[c.reg(op&1)])
		return 2
	case 0x90:
		h, l := c.fetch(), c.fetch()
		c.setDPTR(uint16(h)<<8 | uint16(l))
		return 2
	case 0x92:
		c.writeBit(c.fetch(), c.cy() == 1)
		return 2
	case 0xA2:
		c.setFlag(0x80, c.readBit(c.fetch(), false))
		return 1
	case 0xA3:
		c.setDPTR(c.dptr() + 1)
		return 2
	case 0xA5:
		return 1 // не определена
	case 0xA6, 0xA7:
		c.IRAM[c.reg(op&1)] = c.readDirect(c.fetch(), false)
		return 2
	case 0xB2:
		b := c.fetch()
		c.writeBit(b, !c.readBit(b, true))
		return 1
	case 0xB3:
		c.setFlag(0x80, c.cy() == 0)
		return 1
	case 0xC2:
		c.writeBit(c.fetch(), false)
		return 1
	case 0xC3:
		c.setFlag(0x80, false)
		return 1
	case 0xD2:
		c.writeBit(c.fetch(), true)
		return 1
	case 0xD3:
		c.setFlag(0x80, true)
		return 1
	case 0xB4, 0xB5, 0xB6, 0xB7: // CJNE A,#/A,dir/@Ri,#
		var x, y byte
		switch op {
		case 0xB4:
			x, y = c.A(), c.fetch()
		case 0xB5:
			x, y = c.A(), c.readDirect(c.fetch(), false)
		default:
			x, y = c.IRAM[c.reg(op&1)], c.fetch()
		}
		off := c.fetch()
		c.setFlag(0x80, x < y)
		if x != y {
			c.rel(off)
		}
		return 2
	case 0xC0:
		c.push(c.readDirect(c.fetch(), false))
		return 2
	case 0xD0:
		a := c.fetch()
		c.writeDirect(a, c.pop())
		return 2
	case 0xC4:
		a := c.A()
		c.setA(a<<4 | a>>4)
		return 1
	case 0xC5:
		a := c.fetch()
		v := c.readDirect(a, false)
		c.writeDirect(a, c.A())
		c.setA(v)
		return 1
	case 0xC6, 0xC7:
		r := c.reg(op & 1)
		c.IRAM[r], c.SFR[ACC-0x80] = c.A(), c.IRAM[r]
		return 1
	case 0xD6, 0xD7: // XCHD
		r := c.reg(op & 1)
		a, m := c.A(), c.IRAM[r]
		c.setA(a&0xF0 | m&0x0F)
		c.IRAM[r] = m&0xF0 | a&0x0F
		return 1
	case 0xD4: // DA A
		v := uint16(c.A())
		if v&0x0F > 9 || c.psw()&0x40 != 0 {
			v += 6
			if v > 0xFF {
				c.setFlag(0x80, true)
			}
		}
		if (v>>4)&0x1F > 9 || c.cy() == 1 {
			v += 0x60
			c.setFlag(0x80, true)
		}
		c.setA(byte(v))
		return 1
	case 0xD5:
		a, off := c.fetch(), c.fetch()
		v := c.readDirect(a, true) - 1
		c.writeDirect(a, v)
		if v != 0 {
			c.rel(off)
		}
		return 2
	case 0xE0:
		c.setA(c.xread(c.dptr()))
		return 2
	case 0xE2, 0xE3:
		c.setA(c.xread(uint16(c.sfr(P2))<<8 | uint16(c.reg(op&1))))
		return 2
	case 0xF0:
		c.xwrite(c.dptr(), c.A())
		return 2
	case 0xF2, 0xF3:
		c.xwrite(uint16(c.sfr(P2))<<8|uint16(c.reg(op&1)), c.A())
		return 2
	case 0xE4:
		c.setA(0)
		return 1
	case 0xF4:
		c.setA(^c.A())
		return 1
	case 0xE5:
		c.setA(c.readDirect(c.fetch(), false))
		return 1
	case 0xE6, 0xE7:
		c.setA(c.IRAM[c.reg(op&1)])
		return 1
	case 0xF5:
		c.writeDirect(c.fetch(), c.A())
		return 1
	case 0xF6, 0xF7:
		c.IRAM[c.reg(op&1)] = c.A()
		return 1
	}
	// Rn-формы
	n := op & 7
	switch op & 0xF8 {
	case 0x08:
		c.setReg(n, c.reg(n)+1)
		return 1
	case 0x18:
		c.setReg(n, c.reg(n)-1)
		return 1
	case 0x78:
		c.setReg(n, c.fetch())
		return 1
	case 0x88:
		c.writeDirect(c.fetch(), c.reg(n))
		return 2
	case 0xA8:
		c.setReg(n, c.readDirect(c.fetch(), false))
		return 2
	case 0xB8:
		y, off := c.fetch(), c.fetch()
		x := c.reg(n)
		c.setFlag(0x80, x < y)
		if x != y {
			c.rel(off)
		}
		return 2
	case 0xC8:
		v := c.reg(n)
		c.setReg(n, c.A())
		c.setA(v)
		return 1
	case 0xD8:
		off := c.fetch()
		v := c.reg(n) - 1
		c.setReg(n, v)
		if v != 0 {
			c.rel(off)
		}
		return 2
	case 0xE8:
		c.setA(c.reg(n))
		return 1
	case 0xF8:
		c.setReg(n, c.A())
		return 1
	}
	return 1
}

func logic(kind, x, y byte) byte {
	switch kind {
	case 4:
		return x | y
	case 5:
		return x & y
	}
	return x ^ y
}

// ------------------------------------------------------------------ таймеры и прерывания

func (c *CPU) pin(n int) bool { return c.Pins[3]&c.SFR[P3-0x80]&(1<<(2+n)) != 0 } // INT0 = P3.2, INT1 = P3.3

func (c *CPU) tick(n int) {
	for i := 0; i < n; i++ {
		c.Cycles++
		tcon := &c.SFR[TCON-0x80]
		tmod := c.sfr(TMOD)
		for t := 0; t < 2; t++ {
			// внешние прерывания
			p := c.pin(t)
			itBit, ieBit := byte(1)<<(2*t), byte(2)<<(2*t)
			if *tcon&itBit != 0 {
				if c.prevPin[t] && !p {
					*tcon |= ieBit
				}
			} else if p {
				*tcon &^= ieBit
			} else {
				*tcon |= ieBit
			}
			c.prevPin[t] = p
		}
		m0 := tmod & 3
		// Timer0
		if *tcon&0x10 != 0 && (tmod&0x08 == 0 || c.pin(0)) && tmod&0x04 == 0 {
			if c.count(TL0, TH0, m0) {
				*tcon |= 0x20
			}
		}
		// в режиме 3 TH0 — 8-битный счётчик под TR1 с флагом TF1, а Timer1 стоит
		if m0 == 3 {
			if *tcon&0x40 != 0 {
				c.SFR[TH0-0x80]++
				if c.SFR[TH0-0x80] == 0 {
					*tcon |= 0x80
				}
			}
			continue
		}
		m1 := tmod >> 4 & 3
		if m1 != 3 && *tcon&0x40 != 0 && (tmod&0x80 == 0 || c.pin(1)) && tmod&0x40 == 0 {
			if c.count(TL1, TH1, m1) {
				*tcon |= 0x80
			}
		}
	}
}

// count — +1 к таймеру (TL, TH) в режиме m; true — переполнение.
func (c *CPU) count(tl, th, m byte) bool {
	L, H := &c.SFR[tl-0x80], &c.SFR[th-0x80]
	switch m {
	case 0: // 13 бит: TL — младшие 5
		*L = (*L + 1) & 0x1F
		if *L == 0 {
			*H++
			return *H == 0
		}
	case 1:
		*L++
		if *L == 0 {
			*H++
			return *H == 0
		}
	case 2:
		*L++
		if *L == 0 {
			*L = *H
			return true
		}
	case 3: // Timer0: TL0 8 бит
		*L++
		return *L == 0
	}
	return false
}

// interrupt — вход в обработчик, если есть разрешённый запрос выше текущего уровня. Возвращает МЦ (2) или 0.
func (c *CPU) interrupt() int {
	ie, ip, tcon := c.sfr(IE), c.sfr(IP), c.sfr(TCON)
	if ie&0x80 == 0 {
		return 0
	}
	type src struct {
		flag   bool
		en, pr byte
		vec    uint16
		clear  byte // бит TCON, сбрасываемый при входе (0 — нет)
	}
	srcs := []src{
		{tcon&0x02 != 0, 0x01, 0x01, 0x03, map[bool]byte{true: 0x02, false: 0}[tcon&0x01 != 0]},
		{tcon&0x20 != 0, 0x02, 0x02, 0x0B, 0x20},
		{tcon&0x08 != 0, 0x04, 0x04, 0x13, map[bool]byte{true: 0x08, false: 0}[tcon&0x04 != 0]},
		{tcon&0x80 != 0, 0x08, 0x08, 0x1B, 0x80},
		{c.sfr(SCON)&0x03 != 0, 0x10, 0x10, 0x23, 0},
	}
	for level := 1; level >= 0; level-- {
		if c.active[1] || (level == 0 && c.active[0]) {
			continue
		}
		for _, s := range srcs {
			if !s.flag || ie&s.en == 0 || (ip&s.pr != 0) != (level == 1) {
				continue
			}
			c.SFR[TCON-0x80] &^= s.clear
			c.push(byte(c.PC))
			c.push(byte(c.PC >> 8))
			c.PC = s.vec
			c.active[level] = true
			return c.VectorCycles
		}
	}
	return 0
}
