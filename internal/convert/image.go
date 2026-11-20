package convert

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"slices"

	_ "github.com/gen2brain/jpegxl"
	"github.com/go-pdf/fpdf"
	webp "github.com/mayahiro/go-webp"
	"github.com/spakin/netpbm"
	"github.com/xfmoulet/qoi"
	"golang.org/x/image/bmp"
	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"
)

// pictureIn are the pictures Go reads itself; pictureOut those it writes.
var (
	pictureIn  = []string{"png", "jpg", "webp", "gif", "bmp", "tiff", "ico", "tga", "qoi", "ppm", "svg", "jxl"}
	pictureOut = []string{"png", "jpg", "webp", "gif", "bmp", "tiff", "ico", "icns", "tga", "qoi", "ppm", "pdf"}
)

func init() {
	register(&edge{engine: "image", from: pictureIn, to: pictureOut, cost: 1, run: convertImage})
}

// toPNG writes pictures of the formats Go cannot read as PNG, by
// extension, as the system's decoders do (RegisterImageDecoder).
var toPNG = map[string]func(path, out string) error{}

// RegisterImageDecoder lets Formigo read pictures of the extensions exts,
// which decode writes as PNG: the system's decoder of HEIC on
// macOS. Register before any conversion starts.
func RegisterImageDecoder(exts []string, decode func(path, out string) error) {
	for _, e := range exts {
		toPNG[e] = decode
		f, ok := byExt[e]
		if !ok {
			continue
		}
		ext := e
		register(&edge{engine: "system", from: []string{f.ID}, to: []string{"png"}, cost: 1,
			ok: func() bool { _, ok := toPNG[ext]; return ok },
			run: func(j *job) error {
				d, ok := toPNG[Ext(j.src)]
				if !ok {
					d, ok = toPNG[ext]
				}
				if !ok {
					return ErrUnsupported
				}
				return d(j.src, j.dst)
			}})
	}
	replan()
}

// ImageInfo describes a picture without decoding its pixels where it can.
func ImageInfo(path string) (w, h int, err error) {
	switch Ext(path) {
	case "svg", "ico", "tga", "qoi", "ppm", "pgm", "pbm", "pnm", "pam":
		img, _, err := readImage(path)
		if err != nil {
			return 0, 0, err
		}
		return img.Bounds().Dx(), img.Bounds().Dy(), nil
	}
	if _, ok := toPNG[Ext(path)]; ok {
		data, err := readPicture(path)
		if err != nil {
			return 0, 0, err
		}
		cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
		return cfg.Width, cfg.Height, err
	}
	f, err := os.Open(path)
	if err != nil {
		return 0, 0, err
	}
	defer f.Close()
	cfg, _, err := image.DecodeConfig(bufio.NewReader(f))
	if err != nil {
		return 0, 0, err
	}
	return cfg.Width, cfg.Height, nil
}

// readPicture reads the picture at path, through the registered decoder
// for the formats Go cannot read.
func readPicture(path string) ([]byte, error) {
	decode, ok := toPNG[Ext(path)]
	if !ok {
		return os.ReadFile(path)
	}
	tmp, err := os.CreateTemp("", "formigo-*.png")
	if err != nil {
		return nil, err
	}
	tmp.Close()
	defer os.Remove(tmp.Name())
	if err := decode(path, tmp.Name()); err != nil {
		return nil, fmt.Errorf("无法读取图片：%w", err)
	}
	return os.ReadFile(tmp.Name())
}

// readImage decodes the picture at path, turned upright as its EXIF says.
func readImage(path string) (image.Image, []byte, error) {
	data, err := readPicture(path)
	if err != nil {
		return nil, nil, err
	}
	var img image.Image
	switch f, _ := byExt[Ext(path)]; f.ID {
	case "svg":
		img, err = decodeSVG(data)
	case "ico":
		img, err = decodeICO(data)
	case "tga":
		img, err = decodeTGA(bytes.NewReader(data))
	case "qoi":
		img, err = qoi.Decode(bytes.NewReader(data))
	case "ppm":
		img, err = netpbm.Decode(bytes.NewReader(data), nil)
	default:
		img, _, err = image.Decode(bytes.NewReader(data))
	}
	if err != nil {
		return nil, nil, fmt.Errorf("无法读取图片：%w", err)
	}
	return orient(img, exifOrientation(data)), data, nil
}

func convertImage(j *job) error {
	img, _, err := readImage(j.src)
	if err != nil {
		return err
	}
	j.p(0.4)
	if err := j.ctx.Err(); err != nil {
		return err
	}
	o := j.o
	if o.Scale > 0 && o.Scale != 1 {
		b := img.Bounds()
		img = resize(img, max(1, int(float64(b.Dx())*o.Scale+0.5)), max(1, int(float64(b.Dy())*o.Scale+0.5)))
	}
	j.p(0.55)
	return writeVia(j, func(w *os.File) error { return encodeImage(j.ctx, w, img, j.to.ID, o) })
}

// encodeImage writes img in the format to.
func encodeImage(ctx context.Context, w io.Writer, img image.Image, to string, o Options) error {
	switch to {
	case "png":
		// The smallest files, but for photos, which take seconds to
		// squeeze for a few percent.
		level := png.BestCompression
		if b := img.Bounds(); b.Dx()*b.Dy() > 4_000_000 {
			level = png.DefaultCompression
		}
		return (&png.Encoder{CompressionLevel: level}).Encode(w, img)
	case "jpg":
		return jpeg.Encode(w, flatten(img), &jpeg.Options{Quality: o.quality()})
	case "webp":
		opts := &webp.Options{Compression: webp.CompressionLossy, Quality: o.quality()}
		if o.quality() >= 100 {
			opts = nil
		}
		return webp.EncodeContext(ctx, w, img, opts)
	case "gif":
		return gif.Encode(w, quantize(img), nil)
	case "bmp":
		return bmp.Encode(w, img)
	case "tiff":
		return tiff.Encode(w, img, &tiff.Options{Compression: tiff.Deflate, Predictor: true})
	case "ico":
		return encodeICO(w, img)
	case "icns":
		return encodeICNS(w, img)
	case "tga":
		return encodeTGA(w, img)
	case "qoi":
		return qoi.Encode(w, img)
	case "ppm":
		return netpbm.Encode(w, img, &netpbm.EncodeOptions{Format: netpbm.PPM, MaxValue: 255, Plain: false})
	case "pdf":
		return encodePDF(w, img, o)
	}
	return ErrUnsupported
}

func resize(img image.Image, w, h int) image.Image {
	dst := image.NewNRGBA(image.Rect(0, 0, w, h))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), img, img.Bounds(), xdraw.Over, nil)
	return dst
}

// flatten puts the picture on white, for formats without transparency.
func flatten(img image.Image) image.Image {
	if opaque, ok := img.(interface{ Opaque() bool }); ok && opaque.Opaque() {
		return img
	}
	b := img.Bounds()
	dst := image.NewRGBA(b)
	draw.Draw(dst, b, image.White, image.Point{}, draw.Src)
	draw.Draw(dst, b, img, b.Min, draw.Over)
	return dst
}

// exifOrientation reads the orientation a JPEG's camera wrote, 1 if none.
func exifOrientation(data []byte) int {
	if len(data) < 4 || data[0] != 0xFF || data[1] != 0xD8 {
		return 1
	}
	for i := 2; i+4 <= len(data); {
		if data[i] != 0xFF {
			return 1
		}
		marker, n := data[i+1], int(binary.BigEndian.Uint16(data[i+2:]))
		if marker == 0xDA || n < 2 || i+2+n > len(data) {
			return 1
		}
		seg := data[i+4 : i+2+n]
		if marker == 0xE1 && len(seg) > 14 && string(seg[:6]) == "Exif\x00\x00" {
			return tiffOrientation(seg[6:])
		}
		i += 2 + n
	}
	return 1
}

func tiffOrientation(t []byte) int {
	var bo binary.ByteOrder
	switch string(t[:2]) {
	case "II":
		bo = binary.LittleEndian
	case "MM":
		bo = binary.BigEndian
	default:
		return 1
	}
	off := int(bo.Uint32(t[4:]))
	if off+2 > len(t) {
		return 1
	}
	n := int(bo.Uint16(t[off:]))
	for i := 0; i < n; i++ {
		e := off + 2 + i*12
		if e+12 > len(t) {
			return 1
		}
		if bo.Uint16(t[e:]) == 0x0112 {
			if v := int(bo.Uint16(t[e+8:])); v >= 1 && v <= 8 {
				return v
			}
			return 1
		}
	}
	return 1
}

// orient turns a picture upright as its EXIF orientation says.
func orient(img image.Image, o int) image.Image {
	if o <= 1 || o > 8 {
		return img
	}
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	dw, dh := w, h
	if o >= 5 {
		dw, dh = h, w
	}
	dst := image.NewNRGBA(image.Rect(0, 0, dw, dh))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var dx, dy int
			switch o {
			case 2:
				dx, dy = w-1-x, y
			case 3:
				dx, dy = w-1-x, h-1-y
			case 4:
				dx, dy = x, h-1-y
			case 5:
				dx, dy = y, x
			case 6:
				dx, dy = h-1-y, x
			case 7:
				dx, dy = h-1-y, w-1-x
			case 8:
				dx, dy = y, w-1-x
			}
			dst.Set(dx, dy, img.At(b.Min.X+x, b.Min.Y+y))
		}
	}
	return dst
}

// quantize reduces a picture to a palette of its own colors, by median
// cut, with one transparent entry when it has transparency, and dithers it.
func quantize(img image.Image) *image.Paletted {
	b := img.Bounds()
	step := max(1, (b.Dx()*b.Dy())/250000)
	var px [][3]uint8
	transparent := false
	i := 0
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			i++
			c := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
			if c.A < 128 {
				transparent = true
				continue
			}
			if i%step == 0 {
				px = append(px, [3]uint8{c.R, c.G, c.B})
			}
		}
	}
	n := 256
	if transparent {
		n = 255
	}
	pal := medianCut(px, n)
	if transparent {
		pal = append(pal, color.NRGBA{})
	}
	if len(pal) == 0 {
		pal = color.Palette{color.Black}
	}
	dst := image.NewPaletted(b, pal)
	if !transparent {
		draw.FloydSteinberg.Draw(dst, b, img, b.Min)
		return dst
	}
	// Dither the opaque part; the transparent part takes the last entry.
	draw.FloydSteinberg.Draw(dst, b, flatten(img), b.Min)
	clear := uint8(len(pal) - 1)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			_, _, _, a := img.At(x, y).RGBA()
			if a < 0x8000 {
				dst.SetColorIndex(x, y, clear)
			}
		}
	}
	return dst
}

func medianCut(px [][3]uint8, n int) color.Palette {
	if len(px) == 0 {
		return nil
	}
	boxes := [][][3]uint8{px}
	for len(boxes) < n {
		// Split the box with the widest range of a channel.
		best, bestCh, bestRange := -1, 0, 0
		for i, bx := range boxes {
			if len(bx) < 2 {
				continue
			}
			for ch := 0; ch < 3; ch++ {
				lo, hi := uint8(255), uint8(0)
				for _, p := range bx {
					lo, hi = min(lo, p[ch]), max(hi, p[ch])
				}
				if r := int(hi) - int(lo); r > bestRange {
					best, bestCh, bestRange = i, ch, r
				}
			}
		}
		if best < 0 || bestRange == 0 {
			break
		}
		bx := boxes[best]
		slices.SortFunc(bx, func(a, b [3]uint8) int { return int(a[bestCh]) - int(b[bestCh]) })
		mid := len(bx) / 2
		boxes[best] = bx[:mid]
		boxes = append(boxes, bx[mid:])
	}
	pal := make(color.Palette, 0, len(boxes))
	for _, bx := range boxes {
		var r, g, b int
		for _, p := range bx {
			r, g, b = r+int(p[0]), g+int(p[1]), b+int(p[2])
		}
		k := len(bx)
		pal = append(pal, color.NRGBA{uint8(r / k), uint8(g / k), uint8(b / k), 255})
	}
	return pal
}

// encodeICO writes an icon of the picture centered on a square, at the
// sizes Windows and browsers pick from, each a PNG.
func encodeICO(w io.Writer, img image.Image) error {
	b := img.Bounds()
	side := max(b.Dx(), b.Dy())
	var sizes []int
	for _, s := range []int{256, 128, 64, 48, 32, 16} {
		if s <= side || s == 16 {
			sizes = append(sizes, s)
		}
	}
	var images [][]byte
	for _, s := range sizes {
		sq := image.NewNRGBA(image.Rect(0, 0, s, s))
		scale := float64(s) / float64(side)
		dw, dh := max(1, int(float64(b.Dx())*scale+0.5)), max(1, int(float64(b.Dy())*scale+0.5))
		r := image.Rect((s-dw)/2, (s-dh)/2, (s-dw)/2+dw, (s-dh)/2+dh)
		xdraw.CatmullRom.Scale(sq, r, img, b, xdraw.Over, nil)
		var buf bytes.Buffer
		if err := png.Encode(&buf, sq); err != nil {
			return err
		}
		images = append(images, buf.Bytes())
	}
	var hdr bytes.Buffer
	binary.Write(&hdr, binary.LittleEndian, [3]uint16{0, 1, uint16(len(sizes))})
	offset := 6 + 16*len(sizes)
	for i, s := range sizes {
		dim := uint8(s)
		if s >= 256 {
			dim = 0
		}
		hdr.Write([]byte{dim, dim, 0, 0})
		binary.Write(&hdr, binary.LittleEndian, [2]uint16{1, 32})
		binary.Write(&hdr, binary.LittleEndian, [2]uint32{uint32(len(images[i])), uint32(offset)})
		offset += len(images[i])
	}
	if _, err := w.Write(hdr.Bytes()); err != nil {
		return err
	}
	for _, im := range images {
		if _, err := w.Write(im); err != nil {
			return err
		}
	}
	return nil
}

// encodePDF writes a page the size of the picture, at 144 pixels an inch,
// with the picture on it.
func encodePDF(w io.Writer, img image.Image, o Options) error {
	b := img.Bounds()
	wd, ht := float64(b.Dx())/2, float64(b.Dy())/2
	orientation := "P"
	if wd > ht {
		orientation = "L"
	}
	doc := fpdf.NewCustom(&fpdf.InitType{OrientationStr: orientation, UnitStr: "pt", Size: fpdf.SizeType{Wd: wd, Ht: ht}})
	doc.SetMargins(0, 0, 0)
	doc.SetAutoPageBreak(false, 0)
	doc.AddPage()
	var buf bytes.Buffer
	tp := "JPG"
	if op, ok := img.(interface{ Opaque() bool }); ok && op.Opaque() {
		if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: max(o.quality(), 90)}); err != nil {
			return err
		}
	} else {
		tp = "PNG"
		if err := png.Encode(&buf, img); err != nil {
			return err
		}
	}
	doc.RegisterImageOptionsReader("page", fpdf.ImageOptions{ImageType: tp}, &buf)
	doc.ImageOptions("page", 0, 0, wd, ht, false, fpdf.ImageOptions{ImageType: tp}, 0, "")
	if err := doc.Error(); err != nil {
		return err
	}
	return doc.Output(w)
}
