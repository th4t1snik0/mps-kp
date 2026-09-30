// montage — обзорный лист из PNG-страниц: montage <колонок> <out.png> '<glob>'. Для превью ПЗ в CI.
package main

import (
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"sort"
	"strconv"
)

func main() {
	if len(os.Args) < 4 {
		fmt.Println("montage <колонок> <out.png> '<glob>'")
		return
	}
	cols, _ := strconv.Atoi(os.Args[1])
	fs, _ := filepath.Glob(os.Args[3])
	sort.Strings(fs)
	var ims []image.Image
	for _, f := range fs {
		r, err := os.Open(f)
		if err != nil {
			continue
		}
		if im, err := png.Decode(r); err == nil {
			ims = append(ims, im)
		}
		r.Close()
	}
	if len(ims) == 0 {
		return
	}
	w, h := ims[0].Bounds().Dx(), ims[0].Bounds().Dy()
	rows := (len(ims) + cols - 1) / cols
	dst := image.NewRGBA(image.Rect(0, 0, cols*(w+6), rows*(h+6)))
	draw.Draw(dst, dst.Bounds(), image.NewUniform(image.Black), image.Point{}, draw.Src)
	for i, im := range ims {
		x, y := (i%cols)*(w+6), (i/cols)*(h+6)
		draw.Draw(dst, image.Rect(x, y, x+w, y+h), im, im.Bounds().Min, draw.Src)
	}
	o, _ := os.Create(os.Args[2])
	png.Encode(o, dst)
	o.Close()
}
