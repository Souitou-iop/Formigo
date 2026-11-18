package convert

import (
	"archive/zip"
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/go-pdf/fpdf"
	"golang.org/x/image/font/gofont/goregular"
)

func ids(fs []Format) []string {
	var out []string
	for _, f := range fs {
		out = append(out, f.ID)
	}
	return out
}

// TestRoutes: what a format becomes, its own kind first, through routes of
// more than one step where there is no direct one.
func TestRoutes(t *testing.T) {
	png := ids(Targets("png"))
	if png[0] != "jpg" || !slices.Contains(png, "pdf") || slices.Contains(png, "png") {
		t.Errorf("png → %v", png)
	}
	// A RAR becomes a ZIP through a folder.
	if !CanConvert("rar", "zip") || !CanConvert("7z", "tgz") {
		t.Error("archives do not repack through a folder")
	}
	// Markdown becomes Word, Word becomes an e-book.
	for _, p := range [][2]string{{"md", "docx"}, {"docx", "epub"}, {"epub", "rtf"}, {"xlsx", "yaml"}, {"srt", "lrc"}, {"woff2", "woff"}} {
		if !CanConvert(p[0], p[1]) {
			t.Errorf("%s does not convert to %s", p[0], p[1])
		}
	}
	// A PDF's pages are pictures only at the end of a route.
	if r := plan("pdf")["webp"]; r != nil {
		t.Errorf("pdf → webp through %v", r.through)
	}
	if f, ok := FormatOf("备份.TAR.GZ"); !ok || f.ID != "tgz" {
		t.Errorf("tar.gz is %v", f)
	}
	if len(Supported(Video)) == 0 && ffmpegPath() != "" {
		t.Error("no video read with FFmpeg")
	}
	if KindTargets(Font)[0].ID != "ttf" {
		t.Errorf("font targets %v", ids(KindTargets(Font)))
	}
}

func needFFmpeg(t *testing.T) string {
	t.Helper()
	p := ffmpegPath()
	if p == "" {
		t.Skip("no FFmpeg")
	}
	return p
}

// TestVideoAndSound: a clip made by FFmpeg converts to other containers,
// to its sound, to a GIF and to a frame.
func TestVideoAndSound(t *testing.T) {
	ff := needFFmpeg(t)
	dir := t.TempDir()
	src := filepath.Join(dir, "片段.mov")
	cmd := exec.Command(ff, "-hide_banner", "-loglevel", "error", "-f", "lavfi", "-i", "testsrc=duration=1.5:size=320x240:rate=24",
		"-f", "lavfi", "-i", "sine=frequency=440:duration=1.5", "-shortest", "-c:v", "mpeg4", "-c:a", "aac", src)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("making a clip: %v %s", err, out)
	}
	for _, to := range []string{"mp4", "webm", "mkv", "avi", "gif", "png", "mp3", "m4a", "wav", "flac", "ogg", "opus"} {
		t.Run(to, func(t *testing.T) {
			data := run(t, src, to, Options{Quality: 70, Scale: 0.5})
			if len(data) < 100 {
				t.Fatalf("%d bytes", len(data))
			}
		})
	}
	// Sound to sound, through a WAV made from the clip.
	wav := filepath.Join(dir, "片段.wav")
	if _, err := os.Stat(wav); err != nil {
		t.Fatal(err)
	}
	for _, to := range []string{"mp3", "aiff", "aac"} {
		if data := run(t, wav, to, Options{Mono: true}); len(data) < 100 {
			t.Errorf("%s: %d bytes", to, len(data))
		}
	}
	if d, w, h, err := MediaInfo(src); err != nil || w != 320 || h != 240 || d.Seconds() < 1.4 {
		t.Errorf("info %v %dx%d %v", d, w, h, err)
	}
}

// TestPDF: a PDF's pages drawn, its text read.
func TestPDF(t *testing.T) {
	dir := t.TempDir()
	pdf := fpdf.New("P", "pt", "A4", "")
	pdf.SetFont("Helvetica", "", 24)
	for _, s := range []string{"Hello Formigo", "Second page"} {
		pdf.AddPage()
		pdf.Text(72, 100, s)
	}
	src := filepath.Join(dir, "two.pdf")
	if err := pdf.OutputFileAndClose(src); err != nil {
		t.Fatal(err)
	}
	text := string(run(t, src, "txt", Options{}))
	if !strings.Contains(text, "Hello Formigo") || !strings.Contains(text, "Second page") {
		t.Errorf("text %q", text)
	}
	out, err := Convert(context.Background(), src, dir, byID["png"], Options{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	pages, _ := filepath.Glob(filepath.Join(out, "*.png"))
	if len(pages) != 2 || filepath.Base(pages[0]) != "two 1.png" {
		t.Fatalf("pages %v in %s", pages, out)
	}
	if _, w, h := pictureSize(t, pages[0]); w < 1000 || h < 1500 {
		t.Errorf("page %dx%d", w, h)
	}
}

func pictureSize(t *testing.T, path string) (string, int, int) {
	t.Helper()
	w, h, err := ImageInfo(path)
	if err != nil {
		t.Fatal(err)
	}
	return path, w, h
}

// TestDocuments: Markdown through Word, e-books and rich text and back,
// keeping its headings, lists, tables and Chinese.
func TestDocuments(t *testing.T) {
	dir := t.TempDir()
	md := write(t, dir, "笔记.md", []byte("# 标题\n\n一段**重要**的话。\n\n- 第一\n- 第二\n\n| 名 | 值 |\n|---|---|\n| a | 1 |\n\n## 第二章\n\n结尾 😀\n"))
	docx := filepath.Join(dir, "笔记.docx")
	run(t, md, "docx", Options{})
	zr, err := zip.OpenReader(docx)
	if err != nil {
		t.Fatal(err)
	}
	zr.Close()
	back := string(run(t, docx, "md", Options{}))
	for _, s := range []string{"# 标题", "**重要**", "- 第一", "第二章", "| a"} {
		if !strings.Contains(back, s) {
			t.Errorf("docx → md lost %q: %q", s, back)
		}
	}
	epub := filepath.Join(dir, "笔记.epub")
	run(t, docx, "epub", Options{})
	data, _ := os.ReadFile(epub)
	if !bytes.Contains(data[:60], []byte("mimetypeapplication/epub+zip")) {
		t.Errorf("epub's mimetype is not first and stored")
	}
	html := string(run(t, epub, "html", Options{}))
	if !strings.Contains(html, "标题") || !strings.Contains(html, "结尾") {
		t.Errorf("epub → html %q", html)
	}
	rtf := filepath.Join(dir, "笔记.rtf")
	run(t, md, "rtf", Options{})
	text := string(run(t, rtf, "txt", Options{}))
	if !strings.Contains(text, "一段重要的话") || !strings.Contains(text, "😀") {
		t.Errorf("rtf → txt %q", text)
	}
}

// TestMoreData: XML, JSON Lines, property lists and Excel, there and back.
func TestMoreData(t *testing.T) {
	dir := t.TempDir()
	src := write(t, dir, "people.json", []byte(`[{"name":"阿明","age":30,"tags":"a"},{"name":"Bea","age":25,"tags":"b"}]`))
	for _, to := range []string{"xml", "jsonl", "plist", "xlsx"} {
		run(t, src, to, Options{})
		back := filepath.Join(dir, "people (2).json")
		os.Remove(back)
		mid := filepath.Join(dir, "people."+byID[to].exts[0])
		data := string(run(t, mid, "json", Options{Indent: -1}))
		if !strings.Contains(data, "阿明") || !strings.Contains(data, "25") {
			t.Errorf("%s round trip: %s", to, data)
		}
		os.Remove(back)
	}
}

// TestSubtitles: SubRip through WebVTT, ASS and lyrics.
func TestSubtitles(t *testing.T) {
	dir := t.TempDir()
	src := write(t, dir, "片.srt", []byte("1\n00:00:01,500 --> 00:00:03,000\n你好\n第二行\n\n2\n00:01:02,000 --> 00:01:04,250\n<i>再见</i>\n"))
	vtt := string(run(t, src, "vtt", Options{}))
	if !strings.HasPrefix(vtt, "WEBVTT") || !strings.Contains(vtt, "00:00:01.500 --> 00:00:03.000\n你好\n第二行") {
		t.Errorf("vtt %q", vtt)
	}
	run(t, src, "ass", Options{})
	ass := filepath.Join(dir, "片.ass")
	back := string(run(t, ass, "srt", Options{}))
	if !strings.Contains(back, "00:01:02,000 --> 00:01:04,250\n再见") {
		t.Errorf("ass → srt %q", back)
	}
	lrc := string(run(t, src, "lrc", Options{}))
	if !strings.Contains(lrc, "[01:02.00]再见") {
		t.Errorf("lrc %q", lrc)
	}
}

// TestArchives: a ZIP opens into a folder and repacks as a gzipped tar.
func TestArchives(t *testing.T) {
	dir := t.TempDir()
	var b bytes.Buffer
	zw := zip.NewWriter(&b)
	for name, body := range map[string]string{"a.txt": "甲", "sub/b.txt": "乙"} {
		w, _ := zw.Create(name)
		w.Write([]byte(body))
	}
	zw.Close()
	src := write(t, dir, "包.zip", b.Bytes())
	out, err := Convert(context.Background(), src, dir, byID["folder"], Options{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(filepath.Join(out, "sub", "b.txt")); string(got) != "乙" {
		t.Errorf("unpacked %q in %s", got, out)
	}
	tgz, err := Convert(context.Background(), src, dir, byID["tgz"], Options{}, nil)
	if err != nil || filepath.Base(tgz) != "包.tar.gz" {
		t.Fatalf("%s %v", tgz, err)
	}
	again, err := Convert(context.Background(), tgz, dir, byID["folder"], Options{Name: "again"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(filepath.Join(again, "a.txt")); string(got) != "甲" {
		t.Errorf("tgz unpacked %q", got)
	}
	// An archive whose names lead out of its folder is refused.
	b.Reset()
	zw = zip.NewWriter(&b)
	w, _ := zw.Create("../evil.txt")
	w.Write([]byte("x"))
	zw.Close()
	evil := write(t, dir, "evil.zip", b.Bytes())
	if _, err := Convert(context.Background(), evil, dir, byID["folder"], Options{}, nil); err == nil {
		t.Error("a path out of the folder was written")
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(dir), "evil.txt")); err == nil {
		t.Error("evil.txt escaped")
	}
}

// TestFonts: TrueType to WOFF and back, unchanged.
func TestFonts(t *testing.T) {
	dir := t.TempDir()
	src := write(t, dir, "Go.ttf", goregular.TTF)
	woff := run(t, src, "woff", Options{})
	if !bytes.HasPrefix(woff, []byte("wOFF")) || len(woff) >= len(goregular.TTF) {
		t.Fatalf("woff %d bytes, ttf %d", len(woff), len(goregular.TTF))
	}
	back := run(t, filepath.Join(dir, "Go.woff"), "ttf", Options{})
	if !bytes.Equal(back[:4], goregular.TTF[:4]) || len(back) < len(goregular.TTF)*9/10 {
		t.Errorf("ttf back %d bytes", len(back))
	}
	if _, err := Convert(context.Background(), src, dir, byID["otf"], Options{}, nil); err == nil {
		t.Error("TrueType outlines written as OTF")
	}
}

// TestOffice: Word to PDF with LibreOffice, where it is installed.
func TestOffice(t *testing.T) {
	if officePath() == "" {
		t.Skip("LibreOffice is not installed")
	}
	dir := t.TempDir()
	md := write(t, dir, "报告.md", []byte("# 报告\n\n正文。\n"))
	run(t, md, "docx", Options{})
	pdf := run(t, filepath.Join(dir, "报告.docx"), "pdf", Options{})
	if !bytes.HasPrefix(pdf, []byte("%PDF")) {
		t.Errorf("not a PDF")
	}
}

// TestNoNonsenseRoutes: a still picture never becomes a video or sound,
// nor its PDF text.
func TestNoNonsenseRoutes(t *testing.T) {
	for _, to := range []string{"mp4", "mp3", "txt", "webm"} {
		if CanConvert("png", to) {
			t.Errorf("png → %s through %v", to, plan("png")[to].through)
		}
	}
	if !CanConvert("gif", "mp4") || !CanConvert("mov", "mp3") || !CanConvert("pdf", "txt") {
		t.Error("real routes lost")
	}
}
