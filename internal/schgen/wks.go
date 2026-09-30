package schgen

import "regexp"

var wksVariantRe = regexp.MustCompile(`Группа [^"]*?Э3`)

// PatchWks заменяет в рамке .kicad_wks жёстко вписанную строку «Группа …, Вариант …, Э3».
func PatchWks(src, line string) (string, bool) {
	if !wksVariantRe.MatchString(src) {
		return src, false
	}
	return wksVariantRe.ReplaceAllLiteralString(src, line), true
}
