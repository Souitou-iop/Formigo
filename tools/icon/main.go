// Icon makes Formigo's icon from the gopher of its icon (iCOCN): the
// gopher on transparency for Windows and Linux. macOS takes the Icon
// Composer icon itself (design/AppIcon.icon).
//
//	go run ./tools/icon iCOCN/Formigo.png
package main

import (
	"image"
	"image/draw"
	"image/png"
	"log"
	"os"
	"path/filepath"

	xdraw "golang.org/x/image/draw"
)

func main() {
	f, err := os.Open(os.Args[1])
	if err != nil {
		log.Fatal(err)
	}
	src, err := png.Decode(f)
	f.Close()
	if err != nil {
		log.Fatal(err)
	}
	// The gopher at 80% of the canvas, as the Icon Composer icon has it.
	write(filepath.Join("resources", "icon.png"), place(src, 1024, 0.8))
}

func place(src image.Image, side int, scale float64) *image.NRGBA {
	dst := image.NewNRGBA(image.Rect(0, 0, side, side))
	draw.Draw(dst, dst.Bounds(), image.Transparent, image.Point{}, draw.Src)
	s := int(float64(side) * scale)
	o := (side - s) / 2
	xdraw.CatmullRom.Scale(dst, image.Rect(o, o, o+s, o+s), src, src.Bounds(), xdraw.Over, nil)
	return dst
}

func write(path string, img image.Image) {
	f, err := os.Create(path)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		log.Fatal(err)
	}
}
