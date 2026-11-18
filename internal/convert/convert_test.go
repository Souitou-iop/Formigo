package convert

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo/audio"
	"golang.org/x/text/encoding/simplifiedchinese"
)

func write(t *testing.T, dir, name string, data []byte) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func run(t *testing.T, src, to string, o Options) []byte {
	t.Helper()
	f, ok := Lookup(to)
	if !ok {
		t.Fatalf("no format %s", to)
	}
	var last float64
	out, err := Convert(context.Background(), src, "", f, o, func(d float64) {
		if d < last {
			t.Errorf("progress went back from %v to %v", last, d)
		}
		last = d
	})
	if err != nil {
		t.Fatalf("%s → %s: %v", filepath.Base(src), to, err)
	}
	if last != 1 {
		t.Errorf("progress ended at %v", last)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func testPicture(alpha bool) []byte {
	img := image.NewNRGBA(image.Rect(0, 0, 64, 40))
	for y := 0; y < 40; y++ {
		for x := 0; x < 64; x++ {
			a := uint8(255)
			if alpha && x < 8 {
				a = 0
			}
			img.SetNRGBA(x, y, color.NRGBA{uint8(x * 4), uint8(y * 6), 180, a})
		}
	}
	var b bytes.Buffer
	png.Encode(&b, img)
	return b.Bytes()
}

func TestImages(t *testing.T) {
	dir := t.TempDir()
	src := write(t, dir, "图.png", testPicture(true))
	var targets []Format
	for _, f := range Targets("png") {
		if f.Kind == Image || f.ID == "pdf" {
			targets = append(targets, f)
		}
	}
	for _, f := range targets {
		t.Run(f.ID, func(t *testing.T) {
			data := run(t, src, f.ID, Options{Quality: 80})
			switch f.ID {
			case "avif":
				if !bytes.Contains(data[:32], []byte("ftypavif")) {
					t.Fatalf("not an AVIF: %q", data[:16])
				}
				return
			case "jxl":
				if !bytes.HasPrefix(data, []byte{0xff, 0x0a}) && !bytes.Contains(data[:16], []byte("JXL ")) {
					t.Fatalf("not a JPEG XL: %x", data[:12])
				}
				return
			case "icns":
				if !bytes.HasPrefix(data, []byte("icns")) {
					t.Fatalf("not an ICNS: %q", data[:8])
				}
				return
			case "tga", "qoi", "ppm":
				p := filepath.Join(t.TempDir(), "x."+f.ID)
				os.WriteFile(p, data, 0o644)
				img, _, err := readImage(p)
				if err != nil {
					t.Fatal(err)
				}
				if b := img.Bounds(); b.Dx() != 64 || b.Dy() != 40 {
					t.Fatalf("size %v", b)
				}
				return
			}
			switch f.ID {
			case "ico":
				var hdr [3]uint16
				binary.Read(bytes.NewReader(data), binary.LittleEndian, &hdr)
				if hdr != [3]uint16{0, 1, 3} { // 64, 48, 32 and 16 fit; 16 always
					if hdr[1] != 1 || hdr[2] == 0 {
						t.Fatalf("icon header %v", hdr)
					}
				}
			case "pdf":
				if !bytes.HasPrefix(data, []byte("%PDF-")) {
					t.Fatalf("not a PDF: %q", data[:8])
				}
			default:
				img, _, err := image.Decode(bytes.NewReader(data))
				if err != nil {
					t.Fatal(err)
				}
				if b := img.Bounds(); b.Dx() != 64 || b.Dy() != 40 {
					t.Fatalf("size %v", b)
				}
				if f.ID == "png" || f.ID == "tiff" || f.ID == "gif" {
					if _, _, _, a := img.At(2, 2).RGBA(); a != 0 {
						t.Errorf("transparency lost: alpha %d", a)
					}
				}
			}
		})
	}
	half := run(t, src, "png", Options{Scale: 0.5})
	if img, _ := png.Decode(bytes.NewReader(half)); img.Bounds().Dx() != 32 {
		t.Errorf("scaled to %v", img.Bounds())
	}
	if _, err := os.Stat(filepath.Join(dir, "图 (2).png")); err != nil {
		t.Errorf("a second PNG should not overwrite the source: %v", err)
	}
}

func TestOrientation(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 3, 2))
	img.Set(0, 0, color.White)
	r := orient(img, 6) // turned 90° clockwise
	if b := r.Bounds(); b.Dx() != 2 || b.Dy() != 3 {
		t.Fatalf("bounds %v", b)
	}
	if _, _, _, a := r.At(1, 0).RGBA(); a == 0 {
		t.Error("top left should land top right")
	}
	// An Exif segment saying 6, big-endian.
	tiffData := []byte("MM\x00\x2a\x00\x00\x00\x08\x00\x01\x01\x12\x00\x03\x00\x00\x00\x01\x00\x06\x00\x00")
	seg := append([]byte("Exif\x00\x00"), tiffData...)
	jpg := []byte{0xFF, 0xD8, 0xFF, 0xE1, byte((len(seg) + 2) >> 8), byte(len(seg) + 2)}
	jpg = append(jpg, seg...)
	jpg = append(jpg, 0xFF, 0xDA, 0, 2)
	if o := exifOrientation(jpg); o != 6 {
		t.Errorf("orientation %d", o)
	}
}

func wav16(rate, ch int, samples []int16) []byte {
	var b bytes.Buffer
	b.WriteString("RIFF")
	binary.Write(&b, binary.LittleEndian, uint32(36+2*len(samples)))
	b.WriteString("WAVEfmt ")
	binary.Write(&b, binary.LittleEndian, uint32(16))
	binary.Write(&b, binary.LittleEndian, uint16(1))
	binary.Write(&b, binary.LittleEndian, uint16(ch))
	binary.Write(&b, binary.LittleEndian, uint32(rate))
	binary.Write(&b, binary.LittleEndian, uint32(rate*ch*2))
	binary.Write(&b, binary.LittleEndian, uint16(ch*2))
	binary.Write(&b, binary.LittleEndian, uint16(16))
	b.WriteString("data")
	binary.Write(&b, binary.LittleEndian, uint32(2*len(samples)))
	binary.Write(&b, binary.LittleEndian, samples)
	return b.Bytes()
}

func decodeAll(t *testing.T, data []byte) (rate, ch int, s []float32) {
	t.Helper()
	dec, _, err := audio.NewDecoder(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	buf := make([]float32, 4096)
	for {
		n, err := dec.Read(buf)
		s = append(s, buf[:n]...)
		if err == io.EOF || n == 0 {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	return dec.SampleRate(), dec.Channels(), s
}

func TestAudio(t *testing.T) {
	dir := t.TempDir()
	const n = 10000 // more than two FLAC blocks, and a short last one
	samples := make([]int16, 2*n)
	for i := 0; i < n; i++ {
		v := int16(12000 * math.Sin(float64(i)/7))
		samples[2*i], samples[2*i+1] = v, -v/2
	}
	src := write(t, dir, "tone.wav", wav16(44100, 2, samples))
	_, _, want := decodeAll(t, wav16(44100, 2, samples))

	rate, ch, got := decodeAll(t, run(t, src, "flac", Options{}))
	if rate != 44100 || ch != 2 || len(got) != len(want) {
		t.Fatalf("FLAC: %d Hz, %d channels, %d samples; want %d", rate, ch, len(got), len(want))
	}
	for i := range want {
		if math.Abs(float64(got[i]-want[i])) > 1.0/32767 {
			t.Fatalf("FLAC sample %d: %v, want %v", i, got[i], want[i])
		}
	}

	flacPath := filepath.Join(dir, "tone.flac")
	rate, ch, got = decodeAll(t, run(t, flacPath, "wav", Options{BitDepth: 24, Mono: true}))
	if rate != 44100 || ch != 1 || len(got) != n {
		t.Fatalf("WAV: %d Hz, %d channels, %d samples", rate, ch, len(got))
	}
	mid := (want[2000] + want[2001]) / 2
	if math.Abs(float64(got[1000]-mid)) > 1e-3 {
		t.Errorf("mono sample %v, want %v", got[1000], mid)
	}
	d, c, r, err := AudioInfo(src)
	if err != nil || c != 2 || r != 44100 || d.Milliseconds() != 226 {
		t.Errorf("info %v %d %d %v", d, c, r, err)
	}
}

func TestDataRoundTrip(t *testing.T) {
	dir := t.TempDir()
	src := write(t, dir, "配置.json", []byte(`{"zeta": 1, "名字": "你好, \"世界\"", "alpha": [1, 2.5, true, null],
		"nested": {"b": "两\n行", "a": {"deep": "x"}}, "list": [{"id": 1, "tag": "<a>"}, {"id": 2}]}`))
	yml := run(t, src, "yaml", Options{})
	if i, j := bytes.Index(yml, []byte("zeta")), bytes.Index(yml, []byte("alpha")); i < 0 || j < i {
		t.Errorf("YAML lost the order:\n%s", yml)
	}
	ymlPath := filepath.Join(dir, "配置.yaml")
	tml := run(t, ymlPath, "toml", Options{})
	if !bytes.Contains(tml, []byte("[[list]]")) || !bytes.Contains(tml, []byte("[nested.a]")) {
		t.Errorf("TOML:\n%s", tml)
	}
	back := run(t, filepath.Join(dir, "配置.toml"), "json", Options{Indent: -1})
	want := `{"zeta":1,"名字":"你好, \"世界\"","alpha":[1,2.5,true],"nested":{"b":"两\n行","a":{"deep":"x"}},"list":[{"id":1,"tag":"<a>"},{"id":2}]}` + "\n"
	if string(back) != want {
		t.Errorf("JSON after YAML and TOML:\n%s\nwant\n%s", back, want)
	}
}

func TestTables(t *testing.T) {
	dir := t.TempDir()
	rows := [][]string{{"名字", "备注"}, {"001", "你好, \"世界\""}, {"002", "两\n行"}, {"3", "1.50"}}
	js, _ := json.Marshal(rows)
	src := write(t, dir, "输入.json", js)
	csvData := run(t, src, "csv", Options{})
	csvPath := filepath.Join(dir, "输入.csv")
	got := run(t, csvPath, "json", Options{Indent: -1})
	want := `[{"名字":"001","备注":"你好, \"世界\""},{"名字":"002","备注":"两\n行"},{"名字":3,"备注":"1.50"}]` + "\n"
	if string(got) != want {
		t.Errorf("CSV %q became\n%s\nwant\n%s", csvData, got, want)
	}
	tsv := run(t, filepath.Join(dir, "输入 (2).json"), "tsv", Options{})
	if !strings.HasPrefix(string(tsv), "名字\t备注\n001\t") {
		t.Errorf("TSV %q", tsv)
	}
	bad := write(t, dir, "坏.json", []byte(`{"a": }`))
	f, _ := Lookup("yaml")
	if _, err := Convert(context.Background(), bad, "", f, Options{}, nil); err == nil || !strings.Contains(err.Error(), "JSON") {
		t.Errorf("bad JSON: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "坏.yaml")); err == nil {
		t.Error("a failed conversion left a file")
	}
}

func TestDocs(t *testing.T) {
	dir := t.TempDir()
	md := write(t, dir, "笔记.md", []byte("# 标题\n\n一段 **粗体** 和 [链接](https://example.com)。\n\n- 甲\n- 乙\n\n| a | b |\n|---|---|\n| 1 | 2 |\n"))
	h := string(run(t, md, "html", Options{}))
	for _, s := range []string{"<title>标题</title>", "<strong>粗体</strong>", "<table>", `<h1 id=`} {
		if !strings.Contains(h, s) {
			t.Errorf("HTML lacks %q", s)
		}
	}
	txt := string(run(t, md, "txt", Options{}))
	if !strings.Contains(txt, "标题\n\n一段 粗体 和 链接 (https://example.com)。") || !strings.Contains(txt, "• 甲\n• 乙") {
		t.Errorf("text:\n%s", txt)
	}
	back := string(run(t, filepath.Join(dir, "笔记.html"), "md", Options{}))
	if !strings.Contains(back, "# 标题") || !strings.Contains(back, "**粗体**") {
		t.Errorf("Markdown:\n%s", back)
	}
	gbk, _ := simplifiedchinese.GBK.NewEncoder().Bytes([]byte("旧文件\n\n第二段 <b>"))
	g := string(run(t, write(t, dir, "旧.txt", gbk), "html", Options{}))
	if !strings.Contains(g, "<p>旧文件</p>") || !strings.Contains(g, "第二段 &lt;b&gt;") {
		t.Errorf("GBK text as HTML:\n%s", g)
	}
	if s := TextSummary(md); s != "56 字" && !strings.HasSuffix(s, "字") {
		t.Errorf("summary %q", s)
	}
}

func TestUnsupported(t *testing.T) {
	src := write(t, t.TempDir(), "a.xyz", []byte("x"))
	if _, err := Convert(context.Background(), src, "", Format{ID: "png", Kind: Image}, Options{}, nil); err != ErrUnsupported {
		t.Errorf("err %v", err)
	}
	if _, _, ok := Detect("A.JPEG"); !ok {
		t.Error("upper-case extensions")
	}
}

func TestRegisteredDecoder(t *testing.T) {
	dir := t.TempDir()
	src := write(t, dir, "IMG_0001.heic", []byte("not really HEIC"))
	defer SetFFmpeg(ffmpegPath())
	SetFFmpeg("")
	if _, _, ok := Detect(src); ok {
		t.Fatal("HEIC known before its decoder is registered, without FFmpeg")
	}
	RegisterImageDecoder([]string{"heic"}, func(path, out string) error {
		return os.WriteFile(out, testPicture(false), 0o644)
	})
	defer func() { delete(toPNG, "heic"); replan() }()
	if k, _, ok := Detect(src); !ok || k != Image {
		t.Fatalf("kind %v, %v", k, ok)
	}
	if w, h, err := ImageInfo(src); err != nil || w != 64 || h != 40 {
		t.Errorf("info %d×%d %v", w, h, err)
	}
	img, _, err := image.Decode(bytes.NewReader(run(t, src, "jpg", Options{})))
	if err != nil || img.Bounds().Dx() != 64 {
		t.Fatalf("decoded %v, %v", img, err)
	}
}
