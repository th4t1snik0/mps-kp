package schgen

import (
	"fmt"
	"strings"
)

// Рамка листа перечня (А4 книжный) по ГОСТ 2.104: первый лист — основная надпись формы 2 (185×40),
// последующие — форма 2а (185×15). Как в принятом ПЭ3 (Осипова 2025) и ГОСТ-шаблоне (Женюх 2022).
// Координаты .kicad_wks — от правого нижнего угла поля (отступ 10 мм), рамка — на 5 мм дальше.

type wksBuilder struct{ b strings.Builder }

func (w *wksBuilder) rect(x0, y0, x1, y1 float64) {
	fmt.Fprintf(&w.b, "\t(rect (name \"\") (start %s %s) (end %s %s))\n", fmtNum(x0), fmtNum(y0), fmtNum(x1), fmtNum(y1))
}

func (w *wksBuilder) line(x0, y0, x1, y1 float64) {
	fmt.Fprintf(&w.b, "\t(line (name \"\") (start %s %s) (end %s %s))\n", fmtNum(x0), fmtNum(y0), fmtNum(x1), fmtNum(y1))
}

// text — (x, y) от правого нижнего угла; just: "" (центр), "left".
func (w *wksBuilder) text(s string, x, y, size float64, just string, rot bool) {
	if just == "" {
		just = "center" // в .kicad_wks по умолчанию — влево
	}
	j := " (justify " + just + ")"
	r := ""
	if rot {
		r = " (rotate 90)"
	}
	fmt.Fprintf(&w.b, "\t(tbtext %q (name \"\") (pos %s %s)%s (font (face \"GOST type A\") (size %s %s) italic)%s)\n",
		s, fmtNum(x), fmtNum(y), r, fmtNum(size), fmtNum(size), j)
}

// PE3Frame — .kicad_wks для страницы перечня. first — основная надпись формы 2, иначе 2а.
// side — боковые графы (стили A, D — в пару к рамке схемы).
func PE3Frame(first bool, page, pages int, docLine string, side bool) string {
	w := &wksBuilder{}
	w.b.WriteString("(kicad_wks (version 20231118) (generator \"mpsgen\")\n")
	w.b.WriteString("\t(setup (textsize 2.5 2.5) (linewidth 0.3) (textlinewidth 0.25) (left_margin 10) (right_margin 10) (top_margin 10) (bottom_margin 10))\n")
	// внешняя рамка: А4 книжный 210×297, поле слева 20, остальные 5
	w.rect(-5, -5, 180, 282)
	R := -5.0 // правый край основной надписи
	if first {
		// форма 2: 185×40; слева 5 граф (7/10/23/15/10) × 8 строк по 5 мм; справа 120: обозначение 15, наименование 70 + Лит./Лист/Листов + организация
		L := R + 185
		T := -5.0 + 40
		w.rect(R, -5, L, T)
		// ниже шапки графы «Изм.» и «Лист» объединены (там «Разраб.», «Пров.»…) — черта между ними только сверху
		w.line(L-7, 15, L-7, T)
		for _, x := range []float64{L - 17, L - 40, L - 55, L - 65} {
			w.line(x, -5, x, T)
		}
		for i := 1; i < 8; i++ {
			y := -5 + float64(i)*5
			w.line(L, y, L-65, y)
		}
		r0 := L - 65              // правая часть: от r0 до R
		w.line(r0, T-15, R, T-15) // под обозначением документа
		w.line(R+50, -5, R+50, T-15)
		w.line(R+50, T-20, R, T-20) // под «Лит. Лист Листов»
		w.line(R+50, T-25, R, T-25)
		w.line(R+35, T-15, R+35, T-25) // Лит | Лист
		w.line(R+20, T-15, R+20, T-25) // Лист | Листов
		w.line(R+45, T-20, R+45, T-25)
		w.line(R+40, T-20, R+40, T-25)
		row := func(i int) float64 { return 37.5 - float64(i)*5 } // центр i-й строки сверху (с 1): 3 строки изменений, шапка, Разраб., Пров., Н. контр., Утв.
		labels := []string{"Изм.", "Лист", "№ докум.", "Подп.", "Дата"}
		for i, s := range labels {
			x := []float64{L - 3.5, L - 12, L - 28.5, L - 47.5, L - 60}[i]
			w.text(s, x, row(4), 2.0, "", false)
		}
		w.text("Разраб.", L-1, row(5), 2.0, "left", false)
		w.text("Пров.", L-1, row(6), 2.0, "left", false)
		w.text("Н. контр.", L-1, row(7), 2.0, "left", false)
		w.text("Утв.", L-1, row(8), 2.0, "left", false)
		w.text("${COMMENT2}", L-18, row(5), 2.0, "left", false)
		w.text("${COMMENT3}", L-18, row(6), 2.0, "left", false)
		w.text("${ISSUE_DATE}", L-60, row(5), 1.6, "", false)
		w.text(docLine, (r0+R)/2, T-7.5, 4.5, "", false)
		w.text("Перечень", (r0+R+50)/2, T-20.5, 3.5, "", false)
		w.text("элементов", (r0+R+50)/2, T-26, 3.5, "", false)
		w.text("Лит.", R+42.5, T-17.5, 2.0, "", false)
		w.text("Лист", R+27.5, T-17.5, 2.0, "", false)
		w.text("Листов", R+10, T-17.5, 2.0, "", false)
		w.text(fmt.Sprint(page), R+27.5, T-22.5, 2.0, "", false)
		w.text(fmt.Sprint(pages), R+10, T-22.5, 2.0, "", false)
		w.text("НИУ «МЭИ»", R+25, T-28.5, 2.0, "", false)
		w.text("Институт ИВТИ", R+25, T-32, 2.0, "", false)
		w.text("Кафедра ВМСС", R+25, T-35.5, 2.0, "", false)
	} else {
		// форма 2а: 185×15; слева 5 граф × 3 строки; справа обозначение 110 + «Лист» 10
		L := R + 185
		T := -5.0 + 15
		w.rect(R, -5, L, T)
		for _, x := range []float64{L - 7, L - 17, L - 40, L - 55, L - 65} {
			w.line(x, -5, x, T)
		}
		w.line(L, 0, L-65, 0)
		w.line(L, 5, L-65, 5)
		w.line(R+10, -5, R+10, T)
		w.line(R+10, 3, R, 3)
		for i, s := range []string{"Изм.", "Лист", "№ докум.", "Подп.", "Дата"} {
			x := []float64{L - 3.5, L - 12, L - 28.5, L - 47.5, L - 60}[i]
			w.text(s, x, -2.5+1.25, 2.0, "", false)
		}
		w.text(docLine, (L-65+R+10)/2, 2.5, 4.5, "", false)
		w.text("Лист", R+5, 6.5, 2.0, "", false)
		w.text(fmt.Sprint(page), R+5, -1, 2.5, "", false)
	}
	if side {
		// боковые графы (ГОСТ 2.104), как на листе схемы: x 8…20 мм от края листа → от угла поля 190…202
		X := func(xa float64) float64 { return 200 - xa }
		Y := func(ya float64) float64 { return 287 - ya }
		y := 292.0
		for _, it := range []struct {
			s string
			h float64
		}{{"Инв. № подл.", 25}, {"Подп. и дата", 35}, {"Взам. инв. №", 25}, {"Инв. № дубл.", 25}, {"Подп. и дата", 35}} {
			w.rect(X(8), Y(y-it.h), X(13), Y(y))
			w.rect(X(13), Y(y-it.h), X(20), Y(y))
			w.text(it.s, X(10.5), Y(y-it.h/2), 2.0, "", true)
			y -= it.h
		}
		if first {
			y = 5
			for _, it := range []struct {
				s string
				h float64
			}{{"Перв. примен.", 60}, {"Справ. №", 60}} {
				w.rect(X(8), Y(y), X(13), Y(y+it.h))
				w.rect(X(13), Y(y), X(20), Y(y+it.h))
				w.text(it.s, X(10.5), Y(y+it.h/2), 2.0, "", true)
				y += it.h
			}
		}
	}
	w.b.WriteString(")\n")
	return w.b.String()
}
