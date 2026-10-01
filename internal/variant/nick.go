package variant

import (
	"strings"
	"unicode"
)

// Ник студента — имя папки students/<ник>/: латиницей «группа_фамилия_ио», например А-12, «Рязанцев И.В.» →
// a12_ryazantsev_iv. Не придумывается, а считается из group и name в variant.yaml (тест сверяет все папки);
// узнать свой: make nick G=А-12 FIO="Рязанцев И.В.".

// translit — привычная транслитерация (я → ya, ю → yu, й → y, х → kh, ц → ts), строчными; прочие знаки выкидываются.
var translit = map[rune]string{
	'а': "a", 'б': "b", 'в': "v", 'г': "g", 'д': "d", 'е': "e", 'ё': "e", 'ж': "zh", 'з': "z", 'и': "i", 'й': "y",
	'к': "k", 'л': "l", 'м': "m", 'н': "n", 'о': "o", 'п': "p", 'р': "r", 'с': "s", 'т': "t", 'у': "u", 'ф': "f",
	'х': "kh", 'ц': "ts", 'ч': "ch", 'ш': "sh", 'щ': "shch", 'ъ': "", 'ы': "y", 'ь': "", 'э': "e", 'ю': "yu", 'я': "ya",
}

func lat(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		switch {
		case translit[r] != "" || r == 'ь' || r == 'ъ':
			b.WriteString(translit[r])
		case r < 128 && (unicode.IsLetter(r) || unicode.IsDigit(r)):
			b.WriteRune(r)
		}
	}
	return b.String()
}

// Nick — ник по группе («А-12», «Аэ-21») и ФИО («Рязанцев И.В.»). Пусто — ФИО не разобрать.
func Nick(group, fio string) string {
	f := strings.Fields(strings.ReplaceAll(fio, ".", ". "))
	if len(f) == 0 {
		return ""
	}
	io := ""
	for _, p := range f[1:] {
		if r := []rune(p); len(r) > 0 {
			io += lat(string(r[0]))
		}
	}
	n := lat(group) + "_" + lat(f[0])
	if io != "" {
		n += "_" + io
	}
	return n
}
