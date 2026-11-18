package convert

import (
	"bufio"
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	"io"
)

// TGA, Truevision's: written as 32-bit pixels, run-length encoded; read in
// true color or gray, plain or run-length encoded. (A package of its own
// would register it for every picture, TGA having no signature.)

func encodeTGA(w io.Writer, img image.Image) error {
	b := img.Bounds()
	if b.Dx() > 65535 || b.Dy() > 65535 {
		return errors.New("图片太大，TGA 写不下")
	}
	var h [18]byte
	h[2] = 10 // run-length encoded true color
	binary.LittleEndian.PutUint16(h[12:], uint16(b.Dx()))
	binary.LittleEndian.PutUint16(h[14:], uint16(b.Dy()))
	h[16] = 32
	h[17] = 8 | 0x20 // 8 bits of alpha, rows top down
	bw := bufio.NewWriter(w)
	bw.Write(h[:])
	px := func(x, y int) [4]byte {
		c := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
		return [4]byte{c.B, c.G, c.R, c.A}
	}
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; {
			p := px(x, y)
			n := 1
			for x+n < b.Max.X && n < 128 && px(x+n, y) == p {
				n++
			}
			if n > 1 {
				bw.WriteByte(byte(0x80 | (n - 1)))
				bw.Write(p[:])
				x += n
				continue
			}
			// A run of different pixels.
			start := x
			for x < b.Max.X && x-start < 128 && (x+1 >= b.Max.X || px(x+1, y) != px(x, y)) {
				x++
			}
			if x == start {
				x++
			}
			bw.WriteByte(byte(x - start - 1))
			for i := start; i < x; i++ {
				q := px(i, y)
				bw.Write(q[:])
			}
		}
	}
	// The footer of TGA 2.0.
	bw.Write(make([]byte, 8))
	bw.WriteString("TRUEVISION-XFILE.\x00")
	return bw.Flush()
}

func decodeTGA(r io.Reader) (image.Image, error) {
	br := bufio.NewReader(r)
	var h [18]byte
	if _, err := io.ReadFull(br, h[:]); err != nil {
		return nil, err
	}
	kind, depth := h[2], int(h[16])
	w, ht := int(binary.LittleEndian.Uint16(h[12:])), int(binary.LittleEndian.Uint16(h[14:]))
	if h[1] != 0 || w == 0 || ht == 0 {
		return nil, errors.New("不支持的 TGA")
	}
	rle := kind >= 9
	gray := kind == 3 || kind == 11
	if !(kind == 2 || kind == 3 || kind == 10 || kind == 11) || !(depth == 8 && gray || depth == 24 || depth == 32 || depth == 16) {
		return nil, errors.New("不支持的 TGA")
	}
	io.CopyN(io.Discard, br, int64(h[0]))
	bpp := depth / 8
	img := image.NewNRGBA(image.Rect(0, 0, w, ht))
	topDown := h[17]&0x20 != 0
	read := func(p []byte) color.NRGBA {
		switch {
		case gray:
			return color.NRGBA{p[0], p[0], p[0], 255}
		case bpp == 2:
			v := binary.LittleEndian.Uint16(p)
			r, g, b := byte(v>>10&31)<<3, byte(v>>5&31)<<3, byte(v&31)<<3
			return color.NRGBA{r, g, b, 255}
		case bpp == 3:
			return color.NRGBA{p[2], p[1], p[0], 255}
		}
		return color.NRGBA{p[2], p[1], p[0], p[3]}
	}
	pix := make([]byte, bpp)
	i, total := 0, w*ht
	set := func(c color.NRGBA) {
		x, y := i%w, i/w
		if !topDown {
			y = ht - 1 - y
		}
		img.SetNRGBA(x, y, c)
		i++
	}
	for i < total {
		if !rle {
			if _, err := io.ReadFull(br, pix); err != nil {
				return nil, err
			}
			set(read(pix))
			continue
		}
		head, err := br.ReadByte()
		if err != nil {
			return nil, err
		}
		n := int(head&0x7f) + 1
		if head&0x80 != 0 {
			if _, err := io.ReadFull(br, pix); err != nil {
				return nil, err
			}
			c := read(pix)
			for k := 0; k < n && i < total; k++ {
				set(c)
			}
			continue
		}
		for k := 0; k < n && i < total; k++ {
			if _, err := io.ReadFull(br, pix); err != nil {
				return nil, err
			}
			set(read(pix))
		}
	}
	return img, nil
}
