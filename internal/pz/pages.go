package pz

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// Номера страниц в содержании. Генератор их не знает (вёрстку делает Word / LibreOffice), поэтому так:
// docx → LibreOffice → PDF → текст по страницам → страница каждого заголовка → пересборка docx с номерами.
// Без LibreOffice — содержание с точками без номеров (Word допишет при открытии: поле TOC помечено «обновить»).

// Soffice — путь к LibreOffice или "".
func Soffice() string {
	for _, n := range []string{"soffice", "libreoffice", "/Applications/LibreOffice.app/Contents/MacOS/soffice"} {
		if p, err := exec.LookPath(n); err == nil {
			return p
		}
	}
	return ""
}

// RenderPDF — docx → PDF через LibreOffice (pdf — путь результата).
func RenderPDF(docx, pdf string) error {
	so := Soffice()
	if so == "" {
		return fmt.Errorf("нет LibreOffice")
	}
	tmp, err := os.MkdirTemp("", "pzpdf")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	// свой профиль — иначе параллельный или открытый LibreOffice мешает
	cmd := exec.Command(so, "-env:UserInstallation=file://"+filepath.ToSlash(filepath.Join(tmp, "profile")), "--headless",
		"--convert-to", "pdf", "--outdir", tmp, docx)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("LibreOffice: %v: %s", err, out)
	}
	b, err := os.ReadFile(filepath.Join(tmp, strings.TrimSuffix(filepath.Base(docx), filepath.Ext(docx))+".pdf"))
	if err != nil {
		return fmt.Errorf("LibreOffice не создал PDF: %v", err)
	}
	return os.WriteFile(pdf, b, 0o644)
}

// FillTOCPages рендерит docx и проставляет номера страниц и число листов в d (для следующего Build). Ошибка — номеров нет.
func FillTOCPages(d *Doc, docx string) error {
	if _, err := exec.LookPath("pdftotext"); err != nil {
		return fmt.Errorf("нет pdftotext (poppler)")
	}
	tmp, err := os.MkdirTemp("", "pzpages")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	pdf := filepath.Join(tmp, "doc.pdf")
	if err := RenderPDF(docx, pdf); err != nil {
		return err
	}
	txt, err := exec.Command("pdftotext", "-layout", pdf, "-").Output()
	if err != nil {
		return fmt.Errorf("pdftotext: %v", err)
	}
	pages := strings.Split(string(txt), "\f")
	nums, err := tocPages(d.toc, pages)
	if err != nil {
		return err
	}
	d.tocPages = nums
	d.pages = len(pages)
	if strings.TrimSpace(pages[len(pages)-1]) == "" { // после последнего \f — пустой хвост
		d.pages--
	}
	return nil
}

var reNum = regexp.MustCompile(`^(\d+(\.\d+)*\.?)\s+`)

func normLine(s string) string {
	s = strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(s, "ё", "е"), "Ё", "Е"))
	s = strings.Join(strings.Fields(s), " ")
	return reNum.ReplaceAllString(s, "")
}

// tocPages ищет заголовки по порядку, начиная со страницы после «СОДЕРЖАНИЕ»; номер страницы — с 1.
func tocPages(toc []tocEntry, pages []string) ([]int, error) {
	start := -1
	for i, p := range pages {
		for _, l := range strings.Split(p, "\n") {
			if normLine(l) == "содержание" {
				start = i + 1
				break
			}
		}
		if start >= 0 {
			break
		}
	}
	if start < 0 {
		return nil, fmt.Errorf("в PDF не найдено «СОДЕРЖАНИЕ»")
	}
	out := make([]int, len(toc))
	pg := start
	for k, e := range toc {
		key := normLine(e.text)
		if strings.HasPrefix(key, "приложение ") { // «ПРИЛОЖЕНИЕ А. Название» — в документе заголовок «ПРИЛОЖЕНИЕ А» отдельной строкой
			key = strings.TrimSuffix(strings.SplitN(key, ". ", 2)[0], ".")
		}
		found := false
		for ; pg < len(pages) && !found; pg++ {
			for _, l := range strings.Split(pages[pg], "\n") {
				n := normLine(l)
				if n == "" {
					continue
				}
				// заголовок мог перенестись: первая строка — начало заголовка
				if n == key || (len([]rune(n)) >= 20 && strings.HasPrefix(key, n)) {
					found = true
					break
				}
			}
		}
		if !found {
			return nil, fmt.Errorf("заголовок «%s» не найден в PDF", e.text)
		}
		pg-- // следующий заголовок может быть на той же странице
		out[k] = pg + 1
	}
	return out, nil
}
