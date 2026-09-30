package codecheck

import (
	"strings"
)

// RobotText — текст для робота: $INCLUDE (vars.inc) заменён содержимым vars.inc (робот принимает один файл),
// BOM снят, переводы строк — CRLF (робот под Windows; A51 понимает оба).
func RobotText(src, varsInc string) string {
	src = strings.TrimPrefix(src, bom)
	var out []string
	for _, l := range strings.Split(strings.ReplaceAll(src, "\r\n", "\n"), "\n") {
		code, _ := splitComment(l)
		if m := reInclude.FindStringSubmatch(code); m != nil && strings.EqualFold(pathBase(m[1]), "vars.inc") {
			out = append(out, "; ---- vars.inc (вклеен mpscode) ----")
			for _, v := range strings.Split(strings.TrimRight(strings.ReplaceAll(varsInc, "\r\n", "\n"), "\n"), "\n") {
				out = append(out, v)
			}
			out = append(out, "; ---- конец vars.inc ----")
			continue
		}
		out = append(out, l)
	}
	return strings.Join(out, "\r\n")
}

const bom = "\xef\xbb\xbf"

func pathBase(p string) string {
	if i := strings.LastIndexAny(p, `/\`); i >= 0 {
		return p[i+1:]
	}
	return p
}

// RobotName — имя файла по ТЗ: «Фамилия ИО-код-n.txt» (из «Рязанцев И.В.» → «Рязанцев ИВ-код-1.txt»).
func RobotName(fio string, n int) string {
	f := strings.Fields(fio)
	name := "Фамилия ИО"
	if len(f) > 0 {
		name = f[0]
		if len(f) > 1 {
			name += " " + strings.NewReplacer(".", "", " ", "").Replace(strings.Join(f[1:], ""))
		}
	}
	return name + "-код-" + string(rune('0'+n)) + ".txt"
}
