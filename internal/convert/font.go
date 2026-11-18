package convert

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"sort"

	"github.com/tdewolff/font"
)

// Fonts: WOFF, WOFF2 and EOT unwrap to the font inside, TrueType or
// OpenType as its outlines are, and a font wraps as WOFF for the web.

func init() {
	register(&edge{engine: "font", from: []string{"ttf", "otf", "woff", "woff2"}, to: []string{"ttf", "otf", "woff"}, cost: 1, run: convertFont})
}

func convertFont(j *job) error {
	data, err := os.ReadFile(j.src)
	if err != nil {
		return err
	}
	sfnt, err := font.ToSFNT(data)
	if err != nil {
		return fmt.Errorf("无法读取字体：%w", err)
	}
	cff := len(sfnt) >= 4 && string(sfnt[:4]) == "OTTO"
	var out []byte
	switch j.to.ID {
	case "ttf":
		if cff {
			return errors.New("这个字体是 CFF 轮廓，只能转为 OTF")
		}
		out = sfnt
	case "otf":
		if !cff {
			return errors.New("这个字体是 TrueType 轮廓，只能转为 TTF")
		}
		out = sfnt
	case "woff":
		out, err = encodeWOFF(sfnt)
		if err != nil {
			return err
		}
	default:
		return ErrUnsupported
	}
	return os.WriteFile(j.dst, out, 0o644)
}

// encodeWOFF wraps a font as WOFF 1.0: each table compressed where that
// makes it smaller.
func encodeWOFF(sfnt []byte) ([]byte, error) {
	if len(sfnt) < 12 {
		return nil, errors.New("字体不完整")
	}
	flavor := binary.BigEndian.Uint32(sfnt)
	n := int(binary.BigEndian.Uint16(sfnt[4:]))
	if len(sfnt) < 12+16*n {
		return nil, errors.New("字体不完整")
	}
	type table struct {
		tag        uint32
		checksum   uint32
		data, comp []byte
	}
	tables := make([]table, n)
	for i := range n {
		r := sfnt[12+16*i:]
		off, length := binary.BigEndian.Uint32(r[8:]), binary.BigEndian.Uint32(r[12:])
		if int(off)+int(length) > len(sfnt) {
			return nil, errors.New("字体表越界")
		}
		t := table{tag: binary.BigEndian.Uint32(r), checksum: binary.BigEndian.Uint32(r[4:]), data: sfnt[off : off+length]}
		var b bytes.Buffer
		zw, _ := zlib.NewWriterLevel(&b, zlib.BestCompression)
		zw.Write(t.data)
		zw.Close()
		t.comp = t.data
		if b.Len() < len(t.data) {
			t.comp = b.Bytes()
		}
		tables[i] = t
	}
	sort.Slice(tables, func(a, b int) bool { return tables[a].tag < tables[b].tag })
	pad := func(x int) int { return (x + 3) &^ 3 }
	offset := 44 + 20*n
	totalSFNT := 12 + 16*n
	var body bytes.Buffer
	dir := make([]byte, 20*n)
	for i, t := range tables {
		d := dir[20*i:]
		binary.BigEndian.PutUint32(d, t.tag)
		binary.BigEndian.PutUint32(d[4:], uint32(offset+body.Len()))
		binary.BigEndian.PutUint32(d[8:], uint32(len(t.comp)))
		binary.BigEndian.PutUint32(d[12:], uint32(len(t.data)))
		binary.BigEndian.PutUint32(d[16:], t.checksum)
		body.Write(t.comp)
		for body.Len()%4 != 0 {
			body.WriteByte(0)
		}
		totalSFNT += pad(len(t.data))
	}
	head := make([]byte, 44)
	copy(head, "wOFF")
	binary.BigEndian.PutUint32(head[4:], flavor)
	binary.BigEndian.PutUint32(head[8:], uint32(44+len(dir)+body.Len()))
	binary.BigEndian.PutUint16(head[12:], uint16(n))
	binary.BigEndian.PutUint32(head[16:], uint32(totalSFNT))
	binary.BigEndian.PutUint16(head[20:], 1) // version 1.0
	out := append(head, dir...)
	return append(out, body.Bytes()...), nil
}
