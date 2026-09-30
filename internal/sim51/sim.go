// Package sim51 — управление симулятором ucsim (s51 из SDCC) по TCP-консоли.
//
// s51 запускается с -Z порт -P: каждая команда возвращает эхо, вывод и «\0» (приглашение).
// Команды выполняются синхронно; «step N ms» — прогон с ограничением по времени, останавливается
// на брейкпоинтах (fetch и события по памяти). Версия, на которой проверено, — ucsim 0.9.9 (SDCC 4.6.0);
// у ucsim 0.6.x (apt Ubuntu) другие такты — не использовать.
package sim51

import (
	"bufio"
	"bytes"
	"fmt"
	"net"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Sim — запущенный симулятор.
type Sim struct {
	cmd  *exec.Cmd
	conn net.Conn
	r    *bufio.Reader
	// Trace — если не nil, сюда пишется весь диалог (для отладки).
	Trace func(cmd, out string)
	// XTAL — частота кварца, Гц.
	XTAL int
	seq  int
}

// Binary — путь к s51 (переменная S51, иначе из PATH).
func Binary() string {
	if p := os.Getenv("S51"); p != "" {
		return p
	}
	return "s51"
}

// Available — есть ли s51.
func Available() bool {
	_, err := exec.LookPath(Binary())
	return err == nil
}

// Start запускает s51 с программой hexPath (Intel HEX). seed — зерно для случайного содержимого ОЗУ после сброса.
func Start(hexPath string, seed int) (*Sim, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	cmd := exec.Command(Binary(), "-q", "-P", "-t", "8052", "-X", "12M", "-R", strconv.Itoa(seed), "-Z", strconv.Itoa(port), hexPath)
	var stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stderr, &stderr
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("запуск s51: %w", err)
	}
	s := &Sim{cmd: cmd, XTAL: 12_000_000}
	deadline := time.Now().Add(5 * time.Second)
	for {
		s.conn, err = net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", port))
		if err == nil {
			break
		}
		if time.Now().After(deadline) {
			cmd.Process.Kill()
			return nil, fmt.Errorf("s51 не открыл порт %d: %v; вывод: %s", port, err, stderr.String())
		}
		time.Sleep(30 * time.Millisecond)
	}
	s.r = bufio.NewReader(s.conn)
	// согласование telnet и баннер — пропускаем до ответа на маркер
	if _, err := s.sync(""); err != nil {
		s.Close()
		return nil, err
	}
	return s, nil
}

// Close завершает симулятор.
func (s *Sim) Close() {
	if s.conn != nil {
		s.conn.Write([]byte("quit\n"))
		s.conn.Close()
	}
	if s.cmd != nil && s.cmd.Process != nil {
		done := make(chan struct{})
		go func() { s.cmd.Wait(); close(done) }()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			s.cmd.Process.Kill()
		}
	}
}

func (s *Sim) readUntil(sub string) (string, error) {
	var buf []byte
	s.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	for {
		c, err := s.r.ReadByte()
		if err != nil {
			return string(buf), fmt.Errorf("s51: ждали %q: %w; получено: %q", sub, err, tail(string(buf)))
		}
		if c != 0 {
			buf = append(buf, c)
		}
		if bytes.HasSuffix(buf, []byte(sub)) {
			return string(buf), nil
		}
	}
}

func tail(s string) string {
	if len(s) > 300 {
		return "…" + s[len(s)-300:]
	}
	return s
}

func (s *Sim) read() (string, error) {
	s.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	b, err := s.r.ReadBytes(0)
	if err != nil {
		return string(b), fmt.Errorf("s51: %w", err)
	}
	return string(b[:len(b)-1]), nil
}

// sync — маркер «expr N»: читаем всё до его ответа. Так ответы не съезжают, даже если s51 печатает
// приглашение («\0») не там, где ждём (на Linux «step» отвечает асинхронно: приглашение сразу, «Stop at» потом).
func (s *Sim) sync(prefix string) (string, error) {
	s.seq++
	mark := fmt.Sprint(900000000 + s.seq)
	if _, err := s.conn.Write([]byte("expr " + mark + "\n")); err != nil {
		return prefix, err
	}
	out, err := s.readUntil("\n" + mark + "\r\n")
	// эхо строк приходит раньше ответов, если команды пришли одним пакетом — вырезаем именно строки маркера
	out = strings.Replace(prefix+out, "expr "+mark+"\r\n", "", 1)
	out = strings.TrimSuffix(out, mark+"\r\n")
	return out, err
}

// Cmd выполняет команду консоли и возвращает вывод (без эха и приглашений).
func (s *Sim) Cmd(c string) (string, error) {
	if _, err := s.conn.Write([]byte(c + "\n")); err != nil {
		return "", err
	}
	pre := ""
	if strings.HasPrefix(c, "step") || strings.HasPrefix(c, "run") {
		// дождаться конца прогона: «Stop at …» печатается всегда (и по брейкпоинту, и по времени)
		var err error
		if pre, err = s.readUntil("Stop at "); err != nil {
			return pre, err
		}
	}
	out, err := s.sync(pre)
	out = strings.ReplaceAll(strings.Replace(out, c+"\r\n", "", 1), "\r\n", "\n")
	if s.Trace != nil {
		s.Trace(c, out)
	}
	return out, err
}

func (s *Sim) must(c string) string {
	out, err := s.Cmd(c)
	if err != nil {
		panic(err)
	}
	return out
}

func (s *Sim) exprInt(e string) int {
	out := strings.TrimSpace(s.must("expr " + e))
	lines := strings.Split(out, "\n")
	v, err := strconv.ParseInt(strings.TrimSpace(lines[len(lines)-1]), 10, 64)
	if err != nil || strings.Contains(out, "error") {
		panic(fmt.Errorf("s51: expr %s → %q", e, out))
	}
	return int(v)
}

// Мемори-доступ. Ошибки протокола — паника (симулятор в неизвестном состоянии), ловится в Guard.

func (s *Sim) IRAM(a int) byte { return byte(s.exprInt(fmt.Sprintf("iram[0x%x]", a))) }
func (s *Sim) XRAM(a int) byte { return byte(s.exprInt(fmt.Sprintf("xram[0x%x]", a))) }
func (s *Sim) SFR(a int) byte  { return byte(s.exprInt(fmt.Sprintf("sfr[0x%x]", a))) }
func (s *Sim) PC() int         { return s.exprInt("PC") }
func (s *Sim) A() byte         { return s.SFR(0xE0) }

// Bit — значение бита по адресу бита 8051 (00…7F — ОЗУ 20h…2Fh, 80…FF — SFR).
func (s *Sim) Bit(b int) bool {
	var v byte
	if b < 0x80 {
		v = s.IRAM(0x20 + b/8)
	} else {
		v = s.SFR(b &^ 7)
	}
	return v&(1<<(b%8)) != 0
}

func (s *Sim) SetIRAM(a int, v ...byte) {
	s.must(fmt.Sprintf("set memory iram 0x%x %s", a, hexList(v)))
}
func (s *Sim) SetXRAM(a int, v ...byte) {
	s.must(fmt.Sprintf("set memory xram 0x%x %s", a, hexList(v)))
}
func (s *Sim) SetSFR(a int, v byte) { s.must(fmt.Sprintf("set memory sfr 0x%x 0x%x", a, v)) }
func (s *Sim) SetA(v byte)          { s.SetSFR(0xE0, v) }
func (s *Sim) SetPC(a int)          { s.must(fmt.Sprintf("pc 0x%x", a)) }

// SetBit — бит 8051.
func (s *Sim) SetBit(b int, v bool) {
	x := 0
	if v {
		x = 1
	}
	s.must(fmt.Sprintf("set bit 0x%x %d", b, x))
}

// SetPins — внешнее состояние выводов порта n (0…3); 1 — «не тянем» (читается защёлка).
func (s *Sim) SetPins(port int, v byte) { s.must(fmt.Sprintf("set hardware port[%d] 0x%x", port, v)) }

// Latch — содержимое защёлки порта (что МК выводит), без учёта внешних сигналов.
func (s *Sim) Latch(port int) byte {
	out := s.must(fmt.Sprintf("info hardware port[%d]", port))
	m := regexp.MustCompile(`(?m)^0x03 ([0-9a-f]+)`).FindStringSubmatch(out)
	if m == nil {
		panic(fmt.Errorf("s51: нет защёлки порта %d в %q", port, out))
	}
	v, _ := strconv.ParseInt(m[1], 16, 32)
	return byte(v)
}

func hexList(v []byte) string {
	var p []string
	for _, x := range v {
		p = append(p, fmt.Sprintf("0x%x", x))
	}
	return strings.Join(p, " ")
}

var reClks = regexp.MustCompile(`\((\d+) clks\)`)

// Clocks — такты кварца с момента сброса (машинный цикл 8051 = 12 тактов).
func (s *Sim) Clocks() int64 {
	out := s.must("state")
	m := reClks.FindStringSubmatch(out)
	if m == nil {
		panic(fmt.Errorf("s51: нет тактов в %q", out))
	}
	v, _ := strconv.ParseInt(m[1], 10, 64)
	return v
}

// Micros — время с момента сброса, мкс.
func (s *Sim) Micros() float64 { return float64(s.Clocks()) * 1e6 / float64(s.XTAL) }

// Брейкпоинты. Номер возвращается для Delete.

var reBreak = regexp.MustCompile(`Breakpoint (\d+) at`)

// BreakCode — останов перед выборкой команды по адресу.
func (s *Sim) BreakCode(a int) int {
	out := s.must(fmt.Sprintf("break 0x%x", a))
	m := reBreak.FindStringSubmatch(out)
	if m == nil {
		panic(fmt.Errorf("s51: break → %q", out))
	}
	n, _ := strconv.Atoi(m[1])
	return n
}

// BreakMem — останов после команды, записавшей ('w') или прочитавшей ('r') ячейку mem (xram, iram, sfr).
// Какое событие сработало, s51 не сообщает — определяйте по коду перед PC (см. Stop.Event).
func (s *Sim) BreakMem(mem string, rw byte, a int) int {
	s.must(fmt.Sprintf("break %s %c 0x%x", mem, rw, a))
	out := s.must("info breakpoints")
	last := 0
	for _, l := range strings.Split(out, "\n") {
		if f := strings.Fields(l); len(f) > 0 {
			if n, err := strconv.Atoi(f[0]); err == nil {
				last = max(last, n)
			}
		}
	}
	return last
}

// Delete удаляет брейкпоинт.
func (s *Sim) Delete(n int) { s.must(fmt.Sprintf("delete %d", n)) }

// Stop — почему прогон остановился.
type Stop struct {
	PC      int
	Event   bool    // событие по памяти (break xram/sfr)
	Timeout bool    // вышло время, брейкпоинт не сработал
	Micros  float64 // время останова с момента сброса
}

var (
	reStop = regexp.MustCompile(`Stop at 0x([0-9a-f]+): \((\d+)\)`)
)

// Run выполняет программу не дольше maxMicros мкс модельного времени (до брейкпоинта).
func (s *Sim) Run(maxMicros float64) Stop {
	t0 := s.Clocks()
	us := int64(maxMicros)
	if us < 1 {
		us = 1
	}
	out := s.must(fmt.Sprintf("step %d us", us))
	t1 := s.Clocks()
	st := Stop{PC: s.PC(), Micros: float64(t1) * 1e6 / float64(s.XTAL)}
	if m := reStop.FindStringSubmatch(out); m != nil && m[2] == "112" {
		st.Event = true
	}
	// «step» по времени тоже пишет «Breakpoint» — отличаем по фактически прошедшему времени
	if !st.Event && float64(t1-t0)*1e6/float64(s.XTAL) >= float64(us)-1 {
		st.Timeout = true
	}
	return st
}

// Guard превращает панику протокола в ошибку.
func Guard(err *error) {
	if r := recover(); r != nil {
		if e, ok := r.(error); ok {
			*err = e
			return
		}
		panic(r)
	}
}
