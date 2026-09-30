package codecheck

import (
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"mpskp/internal/asm51"
	"mpskp/internal/variant"
)

var (
	reMarkLoose = regexp.MustCompile(`(?i)%\s*(proc|stop)\s*%`)
	reInclude   = regexp.MustCompile(`(?i)^\s*\$INCLUDE\s*\(?\s*["']?([^"')\s]+)`)
	reForbidden = regexp.MustCompile(`(?i)\b(T2CON|T2MOD|RCAP2L|RCAP2H|TL2|TH2|TR2|TF2|ET2|WDTRST|WDTCON)\b`)
)

// lintText — проверки исходного текста до сборки.
func lintText(r *Report, src string, p *variant.Params) {
	if strings.HasPrefix(src, "\uFEFF") {
		r.warn("Кодировка", "UTF-8 с BOM: в файле для робота BOM снят (робот ждёт UTF-8, BOM ломает ASEM/MIDE)")
	}
	if !utf8.ValidString(src) {
		r.fail("Кодировка", "файл не в UTF-8 (сохрани в UTF-8 — так требует ТЗ для .txt)")
	}
	lines := strings.Split(strings.ReplaceAll(strings.TrimPrefix(src, "\uFEFF"), "\r\n", "\n"), "\n")
	first := ""
	for _, l := range lines {
		if strings.TrimSpace(l) != "" {
			first = strings.TrimSpace(l)
			break
		}
	}
	switch {
	case !strings.HasPrefix(first, ";"):
		r.fail("Первая строка", "должна быть комментарием «; ФИО, группа, вариант, версия» (рис. 7 ТЗ)")
	case !strings.Contains(first, strconv.Itoa(p.M)) || !strings.Contains(first, p.Group):
		r.warn("Первая строка", "нет группы %s или варианта %d: %q", p.Group, p.M, first)
	default:
		r.pass("Первая строка", "%s", first)
	}
	long := 0
	for i, l := range lines {
		_, c := splitComment(l)
		if n := utf8.RuneCountInString(c); n > 250 {
			r.fail("Комментарии", "строка %d: комментарий %d символов (> 250, ТЗ п. 6)", i+1, n)
			long++
		}
		code, _ := splitComment(l)
		if m := reForbidden.FindString(code); m != "" {
			r.fail("Timer2/Watchdog", "строка %d: %s — ТЗ запрещает Timer2 и Watchdog (совместимость с i8051)", i+1, m)
		}
	}
	if long == 0 {
		r.pass("Комментарии", "не длиннее 250 символов")
	}
}

// splitComment — как в asm51: «;» вне кавычек.
func splitComment(s string) (code, comment string) {
	q := byte(0)
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case q != 0:
			if c == q {
				q = 0
			}
		case c == '\'' || c == '"':
			q = c
		case c == ';':
			return s[:i], s[i+1:]
		}
	}
	return s, ""
}

// marks — строки с пометками роботу в основном файле.
type marks struct {
	proc, stop *asm51.Line
}

// lintAsm — проверки собранной программы. Возвращает найденные пометки (nil — сценарии не запускать).
func lintAsm(r *Report, res *asm51.Result) *marks {
	m := &marks{}
	ok := true
	for i := range res.Lines {
		l := &res.Lines[i]
		if l.Included {
			continue
		}
		for _, loose := range reMarkLoose.FindAllStringSubmatch(l.Comment, -1) {
			exact := "%" + strings.ToLower(loose[1]) + "%"
			if loose[0] != exact {
				r.fail("Пометки роботу", "строка %d: %q — пиши ровно %s, без пробелов и заглавных", l.No, loose[0], exact)
				ok = false
				continue
			}
			if l.Addr < 0 || l.Mnemonic == "" {
				r.fail("Пометки роботу", "строка %d: %s должна стоять в строке с командой (рис. 7 ТЗ)", l.No, exact)
				ok = false
				continue
			}
			ptr := &m.proc
			if exact == "%stop%" {
				ptr = &m.stop
			}
			if *ptr != nil {
				r.fail("Пометки роботу", "строка %d: %s второй раз (первый — строка %d)", l.No, exact, (*ptr).No)
				ok = false
				continue
			}
			*ptr = l
		}
	}
	if m.proc == nil {
		r.fail("%proc%", "нет строки «call Процедура ; %%proc%%»")
		ok = false
	} else if !strings.HasSuffix(m.proc.Mnemonic, "CALL") {
		r.fail("%proc%", "строка %d: %%proc%% должна стоять на команде call (ТЗ п. 3), а тут %s", m.proc.No, m.proc.Mnemonic)
		ok = false
	} else {
		r.pass("%proc%", "строка %d, адрес %04Xh", m.proc.No, m.proc.Addr)
	}
	if m.stop == nil {
		r.fail("%stop%", "нет строки «jmp $ ; %%stop%%»")
		ok = false
	} else if !jumpsToSelf(m.stop) {
		r.fail("%stop%", "строка %d: %%stop%% должна стоять на «jmp $» (ТЗ п. 4)", m.stop.No)
		ok = false
	} else {
		r.pass("%stop%", "строка %d, адрес %04Xh", m.stop.No, m.stop.Addr)
	}

	// заглушки векторов
	var bad []string
	for _, v := range []int{0x03, 0x0B, 0x13, 0x1B, 0x23} {
		if _, has := res.Code[v]; !has {
			bad = append(bad, strconv.FormatInt(int64(v), 16)+"h")
		}
	}
	if _, has := res.Code[0]; !has {
		r.fail("Вектор сброса", "по адресу 0 нет команды (sjmp START)")
	}
	if len(bad) > 0 {
		r.fail("Заглушки IRQ", "на векторах %s пусто — нужны «nop / reti» (ТЗ п. 1)", strings.Join(bad, ", "))
	} else {
		r.pass("Заглушки IRQ", "на всех векторах есть код")
	}

	// процедура: многострочный комментарий перед меткой (ТЗ п. 7)
	if m.proc != nil {
		target := callTarget(m.proc)
		for _, s := range res.Symbols {
			if s.Kind != "CODE" || s.Value != target || s.Line < 0 {
				continue
			}
			n := 0
			for j := s.Line - 1; j >= 0 && n < 10; j-- {
				t := strings.TrimSpace(res.Lines[j].Text)
				if !strings.HasPrefix(t, ";") {
					break
				}
				n++
			}
			if n < 2 {
				r.warn("Описание процедуры", "перед %s меньше двух строк комментария — нужно назначение, вход, выход (ТЗ п. 7)", s.Name)
			} else {
				r.pass("Описание процедуры", "перед %s — %d строк комментария", s.Name, n)
			}
			break
		}
	}
	// доля строк с комментарием (ТЗ п. 5: по смыслу — проверить может только человек)
	code, commented := 0, 0
	for _, l := range res.Lines {
		if l.Included || l.Mnemonic == "" {
			continue
		}
		code++
		if strings.TrimSpace(l.Comment) != "" {
			commented++
		}
	}
	if code > 0 && commented*3 < code {
		r.warn("Комментарии в коде", "прокомментировано %d из %d команд — ТЗ просит комментировать по смыслу", commented, code)
	}
	if !ok {
		return nil
	}
	return m
}

func jumpsToSelf(l *asm51.Line) bool {
	b := l.Bytes
	switch {
	case len(b) == 2 && b[0] == 0x80 && b[1] == 0xFE:
		return true
	case len(b) == 3 && b[0] == 0x02 && int(b[1])<<8|int(b[2]) == l.Addr:
		return true
	case len(b) == 2 && b[0]&0x1F == 0x01 && (l.Addr+2)&0xF800|int(b[0]>>5)<<8|int(b[1]) == l.Addr:
		return true
	}
	return false
}

func callTarget(l *asm51.Line) int {
	b := l.Bytes
	switch {
	case len(b) == 3 && b[0] == 0x12:
		return int(b[1])<<8 | int(b[2])
	case len(b) == 2 && b[0]&0x1F == 0x11:
		return (l.Addr+2)&0xF800 | int(b[0]>>5)<<8 | int(b[1])
	}
	return -1
}
