// Package pz — пояснительные записки (ПЗ1 — КМ-2, ПЗ2 — КМ-3): генерируемая часть (таблицы, расчёты, рисунки,
// листинги) + текст студента из students/<ник>/pz/pzN.md по местам «ДОПИШИ». Сборка — pandoc → docx.
package pz

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Fill — место, которое пишет студент.
type Fill struct {
	ID, Title, Hint string
	Done            bool
}

// Doc — markdown для pandoc с нумерацией разделов, рисунков, таблиц.
type Doc struct {
	b          strings.Builder
	h1, h2, h3 int
	fig, tab   int
	Dir        string            // папка сборки (рисунки кладутся сюда)
	Student    map[string]string // текст студента по ид
	Fills      []Fill
	appendixNo int
	Style      DocStyle
}

// NewDoc — документ; student — разобранный students/<ник>/pz/pzN.md (может быть nil).
func NewDoc(dir string, student map[string]string) *Doc {
	return &Doc{Dir: dir, Student: student, Style: docStyles["C"]}
}

func esc(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `*`, `\*`, `_`, `\_`, "`", "\\`", `<`, `\<`, `>`, `\>`, `[`, `\[`, `]`, `\]`, `#`, `\#`, `|`, `\|`, `$`, `\$`, `~`, `\~`, `^`, `\^`)
	return r.Replace(s)
}

// Esc — экранирование текста для markdown (номера, формулы с «*» и «_»).
func Esc(s string) string { return esc(s) }

func (d *Doc) w(format string, a ...any) { fmt.Fprintf(&d.b, format, a...) }

// Styled — абзацы в пользовательском стиле (TitlePage, Center, Fill…).
func (d *Doc) Styled(style string, lines ...string) {
	d.w("\n::: {custom-style=\"%s\"}\n", style)
	for _, l := range lines {
		if l == "" {
			l = " "
		}
		d.w("%s\n\n", l)
	}
	d.w(":::\n\n")
}

// PageBreak — разрыв страницы.
func (d *Doc) PageBreak() {
	d.w("\n```{=openxml}\n<w:p><w:r><w:br w:type=\"page\"/></w:r></w:p>\n```\n\n")
}

// H1 — раздел «N Название» с новой страницы; num = false — без номера (Аннотация, Приложения).
func (d *Doc) H1(title string, num bool) {
	if num {
		d.h1++
		d.h2 = 0
		if d.Style.FigBySection {
			d.fig, d.tab = 0, 0
		}
		dot := ""
		if d.Style.H1Center {
			dot = "."
		}
		d.w("\n# %d%s %s {-}\n\n", d.h1, dot, esc(title))
		return
	}
	d.w("\n# %s {-}\n\n", esc(title))
}

// H2 — подраздел «N.M Название».
func (d *Doc) H2(title string) {
	d.h2++
	d.h3 = 0
	dot := ""
	if d.Style.H1Center {
		dot = "."
	}
	d.w("\n## %d.%d%s %s {-}\n\n", d.h1, d.h2, dot, esc(title))
}

// H3 — пункт «N.M.K Название».
func (d *Doc) H3(title string) {
	d.h3++
	d.w("\n### %d.%d.%d %s {-}\n\n", d.h1, d.h2, d.h3, esc(title))
}

// P — абзац (markdown разрешён: **жирный**, *курсив*).
func (d *Doc) P(format string, a ...any) { d.w("%s\n\n", fmt.Sprintf(format, a...)) }

// Bullets — маркированный список.
func (d *Doc) Bullets(items ...string) {
	for _, it := range items {
		d.w("- %s\n", it)
	}
	d.w("\n")
}

// NextFig / NextTab — номер, который получит следующий рисунок / таблица (для ссылок в тексте до них).
func (d *Doc) NextFig() string { return d.label(d.fig + 1) }
func (d *Doc) NextTab() string { return d.label(d.tab + 1) }

func (d *Doc) label(n int) string {
	if d.Style.FigBySection && d.h1 > 0 {
		return fmt.Sprintf("%d.%d", d.h1, n)
	}
	return fmt.Sprint(n)
}

// Table — таблица с подписью над ней («Таблица N — …» или «Таблица N» справа + название по центру). Возвращает номер.
func (d *Doc) Table(caption string, head []string, rows [][]string) string {
	d.tab++
	n := d.label(d.tab)
	if d.Style.TabRight {
		d.Styled("TableNum", "Таблица "+n)
		d.Styled("TableCaption", esc(caption))
	} else {
		d.Styled("TableCaption", "Таблица "+n+" — "+esc(caption))
	}
	d.w("\n")
	line := func(cells []string) {
		d.w("|")
		for _, c := range cells {
			d.w(" %s |", strings.ReplaceAll(c, "|", `\|`))
		}
		d.w("\n")
	}
	line(head)
	d.w("|")
	for range head {
		d.w("---|")
	}
	d.w("\n")
	for _, r := range rows {
		line(r)
	}
	d.w("\n")
	return n
}

// Figure — рисунок из файла (в Dir) по центру с подписью под ним («Рисунок N — …» или «Рис. N. …»). widthCm = 0 — 16 см.
func (d *Doc) Figure(caption, file string, widthCm float64) string {
	d.fig++
	n := d.label(d.fig)
	if widthCm == 0 {
		widthCm = 16
	}
	d.w("\n::: {custom-style=\"Figure\"}\n![](%s){width=%.1fcm}\n:::\n\n", filepath.ToSlash(file), widthCm)
	if d.Style.FigShort {
		d.Styled("ImageCaption", "Рис. "+n+". "+esc(caption))
	} else {
		d.Styled("ImageCaption", "Рисунок "+n+" — "+esc(caption))
	}
	return n
}

// Formula — формула по центру с номером справа не делаем (ГОСТ допускает без номера, если нет ссылок).
func (d *Doc) Formula(s string) { d.Styled("Center", esc(s)) }

// Code — листинг моноширинным шрифтом.
func (d *Doc) Code(text string) {
	fence := "```"
	for strings.Contains(text, fence) {
		fence += "`"
	}
	d.w("\n%s\n%s\n%s\n\n", fence, strings.TrimRight(text, "\n"), fence)
}

// Fill — место для текста студента: если в students/<ник>/pz/pzN.md раздел «## id» заполнен — вставляется он,
// иначе — подсвеченная подсказка «✍ ДОПИШИ: …».
func (d *Doc) Fill(id, title, hint string) {
	txt := strings.TrimSpace(d.Student[id])
	f := Fill{ID: id, Title: title, Hint: hint, Done: txt != ""}
	d.Fills = append(d.Fills, f)
	if f.Done {
		d.w("\n%s\n\n", txt)
		return
	}
	d.Styled("Fill", "✍ ДОПИШИ ("+id+" в students/…/pz/): "+esc(hint))
}

// TOC — оглавление (поле Word, обновляется при открытии документа) с новой страницы.
func (d *Doc) TOC() {
	d.w("\n```{=openxml}\n" +
		`<w:p><w:pPr><w:pStyle w:val="TOCHeading"/></w:pPr><w:r><w:t>СОДЕРЖАНИЕ</w:t></w:r></w:p>` +
		`<w:p><w:r><w:fldChar w:fldCharType="begin" w:dirty="true"/></w:r><w:r><w:instrText xml:space="preserve"> TOC \o "1-2" \h \z \u </w:instrText></w:r>` +
		`<w:r><w:fldChar w:fldCharType="separate"/></w:r><w:r><w:t>Оглавление: правой кнопкой → «Обновить поле» (Word обновляет само при открытии).</w:t></w:r>` +
		`<w:r><w:fldChar w:fldCharType="end"/></w:r></w:p>` + "\n```\n\n")
}

// Num — число с десятичной запятой (ГОСТ): Num(3.19, 3) → «3,190».
func Num(v float64, prec int) string {
	return strings.Replace(fmt.Sprintf("%.*f", prec, v), ".", ",", 1)
}

// Appendix — «ПРИЛОЖЕНИЕ А» с новой страницы.
func (d *Doc) Appendix(title string) string {
	letter := []string{"А", "Б", "В", "Г", "Д", "Е", "Ж", "И", "К"}[d.appendixNo]
	d.appendixNo++
	d.w("\n# ПРИЛОЖЕНИЕ %s {-}\n\n", letter)
	d.Styled("Center", "**"+esc(title)+"**")
	return letter
}

// Markdown — итоговый текст.
func (d *Doc) Markdown() string { return d.b.String() }

// ------------------------------------------------------------------ текст студента

var reSection = regexp.MustCompile(`(?m)^##\s+([A-Za-z0-9_.-]+)`)

// ParseStudent разбирает pzN.md: разделы «## ид …», подсказки «> ✍ …» и комментарии <!-- --> не входят в текст.
func ParseStudent(src string) map[string]string {
	src = regexp.MustCompile(`(?s)<!--.*?-->`).ReplaceAllString(src, "")
	out := map[string]string{}
	idx := reSection.FindAllStringSubmatchIndex(src, -1)
	for i, m := range idx {
		id := src[m[2]:m[3]]
		end := len(src)
		if i+1 < len(idx) {
			end = idx[i+1][0]
		}
		body := src[m[1]:end]
		if j := strings.IndexByte(body, '\n'); j >= 0 {
			body = body[j+1:] // остаток строки заголовка — название раздела
		} else {
			body = ""
		}
		var keep []string
		for _, l := range strings.Split(body, "\n") {
			if strings.HasPrefix(strings.TrimSpace(l), "> ✍") {
				continue
			}
			keep = append(keep, l)
		}
		out[id] = strings.TrimSpace(strings.Join(keep, "\n"))
	}
	return out
}

// StudentTemplate — файл для студента: все места «ДОПИШИ» в порядке документа; уже написанный текст сохраняется,
// разделы, которых в документе больше нет, уходят в конец с пометкой (текст не теряется).
func StudentTemplate(existing string, fills []Fill, doc string) string {
	old := ParseStudent(existing)
	var b strings.Builder
	fmt.Fprintf(&b, "<!-- Текст студента для %s. Каждый раздел начинается строкой «## ид — название»: ид не менять.\n", doc)
	b.WriteString("     Строки «> ✍ …» — подсказки, в документ не попадают. Пиши под подсказкой обычными абзацами;\n")
	b.WriteString("     markdown можно: **жирный**, *курсив*, списки через «- ». Пустой раздел → в документе жёлтое «ДОПИШИ».\n")
	b.WriteString("     Файл можно пересоздать (make pz-init): написанный текст сохранится. -->\n")
	used := map[string]bool{}
	for _, f := range fills {
		used[f.ID] = true
		fmt.Fprintf(&b, "\n## %s — %s\n> ✍ %s\n\n", f.ID, f.Title, f.Hint)
		if t := old[f.ID]; t != "" {
			b.WriteString(t + "\n")
		}
	}
	var orphans []string
	for id, t := range old {
		if !used[id] && t != "" {
			orphans = append(orphans, id)
		}
	}
	sort.Strings(orphans)
	for _, id := range orphans {
		fmt.Fprintf(&b, "\n## %s — НЕ ИСПОЛЬЗУЕТСЯ (такого места в документе больше нет — перенеси текст или удали)\n\n%s\n", id, old[id])
	}
	return b.String()
}

// Save пишет файл, создавая папки.
func Save(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
