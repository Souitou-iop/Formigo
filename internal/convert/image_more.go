package convert

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"math"

	"github.com/srwiley/oksvg"
	"github.com/srwiley/rasterx"
)

// decodeSVG draws an SVG at its own size, or 1024 points across when it
// gives none, the longer side at most 4096 pixels.
func decodeSVG(data []byte) (image.Image, error) {
	icon, err := oksvg.ReadIconStream(bytes.NewReader(data), oksvg.IgnoreErrorMode)
	if err != nil {
		return nil, err
	}
	w, h := icon.ViewBox.W, icon.ViewBox.H
	if w <= 0 || h <= 0 {
		w, h = 1024, 1024
	}
	if s := 4096 / math.Max(w, h); s < 1 {
		w, h = w*s, h*s
	}
	if m := math.Max(w, h); m < 512 {
		// Small drawings come out sharp: twice their size at least.
		s := math.Ceil(512 / m)
		w, h = w*s, h*s
	}
	iw, ih := int(math.Ceil(w)), int(math.Ceil(h))
	img := image.NewNRGBA(image.Rect(0, 0, iw, ih))
	icon.SetTarget(0, 0, float64(iw), float64(ih))
	scanner := rasterx.NewScannerGV(iw, ih, img, img.Bounds())
	icon.Draw(rasterx.NewDasher(iw, ih, scanner), 1)
	return img, nil
}

// decodeICO reads the largest picture of an icon: a PNG, or a bitmap of 32
// or 24 bits with its mask.
func decodeICO(data []byte) (image.Image, error) {
	if len(data) < 6 || binary.LittleEndian.Uint16(data[2:]) != 1 {
		return nil, errors.New("不是 ICO 图标")
	}
	n := int(binary.LittleEndian.Uint16(data[4:]))
	bestArea, bestOff, bestLen := -1, 0, 0
	for i := range n {
		e := 6 + 16*i
		if e+16 > len(data) {
			break
		}
		w, h := int(data[e]), int(data[e+1])
		if w == 0 {
			w = 256
		}
		if h == 0 {
			h = 256
		}
		size := int(binary.LittleEndian.Uint32(data[e+8:]))
		off := int(binary.LittleEndian.Uint32(data[e+12:]))
		if w*h > bestArea && off+size <= len(data) {
			bestArea, bestOff, bestLen = w*h, off, size
		}
	}
	if bestArea < 0 {
		return nil, errors.New("图标里没有图片")
	}
	entry := data[bestOff : bestOff+bestLen]
	if bytes.HasPrefix(entry, []byte("\x89PNG")) {
		return png.Decode(bytes.NewReader(entry))
	}
	return decodeDIB(entry)
}

// decodeDIB reads an icon's bitmap: its header, its pixels bottom up, then
// a mask of a bit per pixel, of double the height in all.
func decodeDIB(b []byte) (image.Image, error) {
	if len(b) < 40 {
		return nil, errors.New("图标位图不完整")
	}
	hdr := int(binary.LittleEndian.Uint32(b))
	w := int(int32(binary.LittleEndian.Uint32(b[4:])))
	h := int(int32(binary.LittleEndian.Uint32(b[8:]))) / 2
	bits := int(binary.LittleEndian.Uint16(b[14:]))
	if bits != 32 && bits != 24 {
		return nil, fmt.Errorf("不支持 %d 位的图标", bits)
	}
	if w <= 0 || h <= 0 || w > 1024 || h > 1024 {
		return nil, errors.New("图标尺寸不对")
	}
	stride := (w*bits/8 + 3) &^ 3
	maskStride := ((w + 31) / 32) * 4
	pix := b[hdr:]
	if len(pix) < stride*h {
		return nil, errors.New("图标位图不完整")
	}
	mask := pix[stride*h:]
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	anyAlpha := false
	for y := range h {
		row := pix[(h-1-y)*stride:]
		for x := range w {
			p := row[x*bits/8:]
			a := uint8(255)
			if bits == 32 {
				a = p[3]
				anyAlpha = anyAlpha || a != 0
			}
			img.SetNRGBA(x, y, color.NRGBA{p[2], p[1], p[0], a})
		}
	}
	// Without alpha, the mask says what shows.
	if !anyAlpha || bits == 24 {
		for y := range h {
			for x := range w {
				i := (h-1-y)*maskStride + x/8
				transparent := i < len(mask) && mask[i]&(0x80>>(x%8)) != 0
				c := img.NRGBAAt(x, y)
				c.A = map[bool]uint8{true: 0, false: 255}[transparent]
				img.SetNRGBA(x, y, c)
			}
		}
	}
	return img, nil
}

// encodeICNS writes a macOS icon of PNGs of 16 to 1024 pixels, as many as
// the picture's size gives without enlarging it much.
func encodeICNS(w io.Writer, img image.Image) error {
	types := []struct {
		code string
		size int
	}{{"icp4", 16}, {"icp5", 32}, {"icp6", 64}, {"ic07", 128}, {"ic08", 256}, {"ic09", 512}, {"ic10", 1024}}
	side := max(img.Bounds().Dx(), img.Bounds().Dy())
	var body bytes.Buffer
	for i, t := range types {
		if t.size > side*2 && i > 3 {
			break
		}
		var b bytes.Buffer
		if err := png.Encode(&b, resize(square(img), t.size, t.size)); err != nil {
			return err
		}
		body.WriteString(t.code)
		binary.Write(&body, binary.BigEndian, uint32(8+b.Len()))
		body.Write(b.Bytes())
	}
	var head [8]byte
	copy(head[:], "icns")
	binary.BigEndian.PutUint32(head[4:], uint32(8+body.Len()))
	if _, err := w.Write(head[:]); err != nil {
		return err
	}
	_, err := w.Write(body.Bytes())
	return err
}

// square centers a picture on a transparent square, as icons are.
func square(img image.Image) image.Image {
	b := img.Bounds()
	side := max(b.Dx(), b.Dy())
	if b.Dx() == b.Dy() {
		return img
	}
	out := image.NewNRGBA(image.Rect(0, 0, side, side))
	off := image.Pt((side-b.Dx())/2, (side-b.Dy())/2)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			out.Set(x-b.Min.X+off.X, y-b.Min.Y+off.Y, img.At(x, y))
		}
	}
	return out
}
