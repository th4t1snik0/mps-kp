package asm51

import (
	"fmt"
	"strconv"
	"strings"
)

// ParseHEX читает Intel HEX в карту адрес → байт.
func ParseHEX(s string) (map[int]byte, error) {
	m := map[int]byte{}
	for _, l := range strings.Fields(s) {
		l = strings.TrimPrefix(l, ":")
		if len(l) < 10 {
			return nil, fmt.Errorf("короткая строка HEX %q", l)
		}
		n, _ := strconv.ParseInt(l[0:2], 16, 32)
		addr, _ := strconv.ParseInt(l[2:6], 16, 32)
		if l[6:8] != "00" {
			continue
		}
		for i := 0; i < int(n); i++ {
			b, err := strconv.ParseUint(l[8+2*i:10+2*i], 16, 8)
			if err != nil {
				return nil, err
			}
			m[int(addr)+i] = byte(b)
		}
	}
	return m, nil
}

// Diff — различия двух образов кода (для сверки с другим ассемблером), не больше max строк.
func Diff(a, b map[int]byte, max int) []string {
	var out []string
	for addr := 0; addr < 0x10000 && len(out) < max; addr++ {
		x, okA := a[addr]
		y, okB := b[addr]
		switch {
		case okA && !okB:
			out = append(out, fmt.Sprintf("%04Xh: у нас %02X, у эталона пусто", addr, x))
		case !okA && okB:
			out = append(out, fmt.Sprintf("%04Xh: у нас пусто, у эталона %02X", addr, y))
		case okA && x != y:
			out = append(out, fmt.Sprintf("%04Xh: у нас %02X, у эталона %02X", addr, x, y))
		}
	}
	return out
}
