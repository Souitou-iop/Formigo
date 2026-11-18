// Package convert turns files of one format into another, on this machine:
// pictures, video, sound, documents, structured data, subtitles, archives
// and fonts.
//
// Formats are joined by converters, each an edge from some formats to
// others run by an engine: Go's own code, or FFmpeg, PDFium and LibreOffice
// when they are there. A file becomes any format a route of up to three
// edges reaches, the cheapest route chosen, as Filee plans its own.
package convert

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
)

// Kind is the family a format belongs to, which files are grouped by.
type Kind int

const (
	Image Kind = iota
	Video
	Audio
	Doc
	Data
	Subtitle
	Archive
	Font
)

// Kinds lists every kind, in the order the interface shows them.
var Kinds = []Kind{Image, Video, Audio, Doc, Data, Subtitle, Archive, Font}

func (k Kind) String() string {
	switch k {
	case Image:
		return "图片"
	case Video:
		return "视频"
	case Audio:
		return "音频"
	case Doc:
		return "文档"
	case Data:
		return "数据"
	case Subtitle:
		return "字幕"
	case Archive:
		return "压缩包"
	case Font:
		return "字体"
	}
	return "?"
}

// Format is a format files are read or written in.
type Format struct {
	// ID is the format's usual extension, lower case, without the dot.
	ID string
	// Label is its short name, as "JPG".
	Label string
	// Note says in a word or two what it is good for.
	Note string
	Kind Kind
	// exts are the extensions of its files, its ID's first; none for a
	// folder.
	exts []string
}

func f(id, label, note string, k Kind, more ...string) Format {
	return Format{ID: id, Label: label, Note: note, Kind: k, exts: append([]string{id}, more...)}
}

// formats is every format, by kind in the order they are offered.
var formats = []Format{
	f("png", "PNG", "无损", Image),
	f("jpg", "JPG", "照片", Image, "jpeg", "jpe", "jfif"),
	f("webp", "WEBP", "网页", Image),
	f("avif", "AVIF", "高压缩", Image),
	f("heic", "HEIC", "苹果照片", Image, "heif", "hif"),
	f("gif", "GIF", "动图", Image),
	f("bmp", "BMP", "位图", Image, "dib"),
	f("tiff", "TIFF", "印刷", Image, "tif"),
	f("ico", "ICO", "图标", Image),
	f("icns", "ICNS", "Mac 图标", Image),
	f("tga", "TGA", "贴图", Image, "tpic"),
	f("qoi", "QOI", "快速无损", Image),
	f("ppm", "PPM", "Netpbm", Image, "pgm", "pbm", "pnm", "pam"),
	f("svg", "SVG", "矢量", Image),
	f("jxl", "JXL", "JPEG XL", Image),

	f("mp4", "MP4", "通用", Video),
	f("mov", "MOV", "苹果", Video, "qt"),
	f("mkv", "MKV", "多轨", Video),
	f("webm", "WEBM", "网页", Video),
	f("avi", "AVI", "经典", Video),
	f("wmv", "WMV", "Windows", Video, "asf"),
	f("flv", "FLV", "Flash", Video, "f4v"),
	f("m4v", "M4V", "iTunes", Video),
	f("ts", "TS", "广播", Video, "mts", "m2ts", "m2t"),
	f("3gp", "3GP", "老手机", Video, "3g2"),
	f("mpg", "MPEG", "DVD", Video, "mpeg", "m2v", "vob", "dat"),
	f("ogv", "OGV", "Ogg", Video),

	f("mp3", "MP3", "通用", Audio),
	f("m4a", "M4A", "AAC", Audio, "m4b", "m4r"),
	f("aac", "AAC", "高效", Audio, "adts"),
	f("wav", "WAV", "无压缩", Audio, "wave"),
	f("flac", "FLAC", "无损压缩", Audio),
	f("ogg", "OGG", "Vorbis", Audio, "oga"),
	f("opus", "OPUS", "语音", Audio),
	f("aiff", "AIFF", "苹果无压缩", Audio, "aif", "aifc"),
	f("wma", "WMA", "Windows", Audio),
	f("amr", "AMR", "录音", Audio, "awb"),
	f("ape", "APE", "猴子音频", Audio),
	f("wv", "WV", "WavPack", Audio),
	f("caf", "CAF", "Core Audio", Audio),
	f("ac3", "AC3", "杜比", Audio, "eac3"),
	f("mka", "MKA", "Matroska", Audio),

	f("pdf", "PDF", "文档", Doc),
	f("docx", "DOCX", "Word", Doc, "docm", "dotx"),
	f("doc", "DOC", "旧版 Word", Doc, "dot", "wps"),
	f("odt", "ODT", "开放文档", Doc, "fodt", "ott"),
	f("rtf", "RTF", "富文本", Doc),
	f("md", "MD", "Markdown", Doc, "markdown", "mdown", "mkd"),
	f("html", "HTML", "网页", Doc, "htm", "xhtml"),
	f("txt", "TXT", "纯文本", Doc, "text", "log"),
	f("epub", "EPUB", "电子书", Doc),
	f("pptx", "PPTX", "演示", Doc, "pptm", "ppsx", "potx"),
	f("ppt", "PPT", "旧版演示", Doc, "pps", "pot"),
	f("odp", "ODP", "开放演示", Doc, "fodp"),

	f("json", "JSON", "通用", Data),
	f("jsonl", "JSONL", "逐行", Data, "ndjson"),
	f("yaml", "YAML", "配置", Data, "yml"),
	f("toml", "TOML", "配置", Data),
	f("xml", "XML", "标记", Data),
	f("csv", "CSV", "表格", Data),
	f("tsv", "TSV", "表格", Data, "tab"),
	f("xlsx", "XLSX", "Excel", Data, "xlsm", "xltx"),
	f("xls", "XLS", "旧版 Excel", Data, "xlt"),
	f("ods", "ODS", "开放表格", Data, "fods"),
	f("plist", "PLIST", "苹果配置", Data),

	f("srt", "SRT", "通用", Subtitle),
	f("vtt", "VTT", "网页", Subtitle, "webvtt"),
	f("ass", "ASS", "特效", Subtitle, "ssa"),
	f("lrc", "LRC", "歌词", Subtitle),

	f("zip", "ZIP", "通用", Archive),
	{ID: "tgz", Label: "TAR.GZ", Note: "Unix", Kind: Archive, exts: []string{"tar.gz", "tgz"}},
	f("tar", "TAR", "归档", Archive),
	f("7z", "7Z", "高压缩", Archive),
	f("rar", "RAR", "WinRAR", Archive),
	{ID: "tbz", Label: "TAR.BZ2", Note: "Unix", Kind: Archive, exts: []string{"tar.bz2", "tbz2", "tbz"}},
	{ID: "txz", Label: "TAR.XZ", Note: "Unix", Kind: Archive, exts: []string{"tar.xz", "txz"}},
	f("gz", "GZ", "单文件", Archive),
	{ID: "folder", Label: "文件夹", Note: "解压", Kind: Archive},

	f("ttf", "TTF", "TrueType", Font),
	f("otf", "OTF", "OpenType", Font),
	f("woff", "WOFF", "网页字体", Font),
	f("woff2", "WOFF2", "网页字体", Font),
}

// byID and byExt find formats by their ID and their extensions.
var byID, byExt = func() (map[string]Format, map[string]Format) {
	ids, exts := map[string]Format{}, map[string]Format{}
	for _, f := range formats {
		ids[f.ID] = f
		for _, e := range f.exts {
			exts[e] = f
		}
	}
	return ids, exts
}()

// Lookup returns the format with the given ID.
func Lookup(id string) (Format, bool) {
	f, ok := byID[id]
	return f, ok
}

// Ext returns the format's extension of the file at path, lower case,
// without the dot: that of a name's last two parts where they name one, as
// "tar.gz".
func Ext(path string) string {
	name := strings.ToLower(filepath.Base(path))
	for _, two := range []string{"tar.gz", "tar.bz2", "tar.xz"} {
		if strings.HasSuffix(name, "."+two) {
			return two
		}
	}
	return strings.TrimPrefix(filepath.Ext(name), ".")
}

// FormatOf returns the format of the file at path, by its extension, if
// Formigo reads it.
func FormatOf(path string) (Format, bool) {
	f, ok := byExt[Ext(path)]
	if !ok || len(Targets(f.ID)) == 0 {
		return Format{}, false
	}
	return f, true
}

// Detect reports the kind of the file at path and its format's ID.
func Detect(path string) (k Kind, from string, ok bool) {
	f, ok := FormatOf(path)
	return f.Kind, f.ID, ok
}

// Supported lists the extensions of a kind's formats Formigo reads, for a
// file dialog's filter.
func Supported(k Kind) []string {
	var out []string
	for _, f := range formats {
		if f.Kind == k && len(Targets(f.ID)) > 0 {
			for _, e := range f.exts {
				if !strings.Contains(e, ".") {
					out = append(out, e)
				}
			}
		}
	}
	return out
}

// Targets returns the formats a file of the format from converts to: those
// of its kind first, in the order they are offered, then the others.
func Targets(from string) []Format {
	plans := plan(from)
	var same, other []Format
	for _, f := range formats {
		if _, ok := plans[f.ID]; !ok || f.ID == from {
			continue
		}
		if f.Kind == byID[from].Kind {
			same = append(same, f)
		} else {
			other = append(other, f)
		}
	}
	return append(same, other...)
}

// CanConvert says whether a file of the format from converts to the
// format to.
func CanConvert(from, to string) bool {
	_, ok := plan(from)[to]
	return ok && from != to
}

// KindTargets returns the formats of kind k files of that kind convert to,
// the choices of a kind's default.
func KindTargets(k Kind) []Format {
	var out []Format
	for _, t := range formats {
		if t.Kind != k {
			continue
		}
		for _, s := range formats {
			if s.Kind == k && CanConvert(s.ID, t.ID) {
				out = append(out, t)
				break
			}
		}
	}
	return out
}

// DefaultTarget is the format a kind converts to unless the user chooses.
func DefaultTarget(k Kind) string {
	return map[Kind]string{
		Image: "png", Video: "mp4", Audio: "mp3", Doc: "html", Data: "json",
		Subtitle: "srt", Archive: "folder", Font: "ttf",
	}[k]
}

// Options tune the conversion. The zero value is sensible for every kind.
type Options struct {
	// Quality is the quality of lossy pictures, video and sound, 1 to 100;
	// 0 is 85. WebP at 100 is lossless.
	Quality int
	// Scale resizes pictures and video, 0 or 1 keeping their size.
	Scale float64
	// BitDepth is the bits per sample of uncompressed and lossless sound
	// written, 16 or 24; 0 is 16.
	BitDepth int
	// Mono mixes sound down to one channel.
	Mono bool
	// Indent is the indentation of JSON and YAML: 0 is 2, -1 compact.
	Indent int
	// Name is the name of the file written, without its extension: the
	// source's when empty.
	Name string
}

func (o Options) quality() int {
	if o.Quality <= 0 {
		return 85
	}
	return min(o.Quality, 100)
}

// Progress is told how much of a conversion is done, from 0 to 1.
type Progress func(done float64)

// ErrUnsupported is returned for a file Formigo cannot read.
var ErrUnsupported = errors.New("不支持的文件格式")

// Convert writes the file at src in format to, in dir, and returns the path
// written: a file, or a folder where the result is several files (an
// archive unpacked, a document's pages). The name is src's with the new
// extension, made unique so that nothing is overwritten.
func Convert(ctx context.Context, src, dir string, to Format, o Options, p Progress) (string, error) {
	if p == nil {
		p = func(float64) {}
	}
	from, ok := FormatOf(src)
	if !ok {
		return "", ErrUnsupported
	}
	route := plan(from.ID)[to.ID]
	if from.ID == to.ID {
		// The same format again, as smaller or resized: an engine that
		// reads and writes it.
		route = sameFormat(from.ID)
	}
	if route == nil {
		return "", fmt.Errorf("%s 不能转换为 %s", from.Label, to.Label)
	}
	if dir == "" {
		dir = filepath.Dir(src)
	}
	name := o.Name
	if name == "" {
		name = strings.TrimSuffix(filepath.Base(src), "."+Ext(src))
	}
	work, err := os.MkdirTemp(dir, ".formigo-*")
	if err != nil {
		// A folder Formigo cannot write in: work in the system's.
		if work, err = os.MkdirTemp("", "formigo-*"); err != nil {
			return "", err
		}
	}
	defer os.RemoveAll(work)

	in := src
	for i, e := range route.steps {
		next := byID[route.through[i+1]]
		out := filepath.Join(work, fmt.Sprintf("step%d", i))
		if next.ID != "folder" {
			out += "." + next.exts[0]
		}
		n := float64(len(route.steps))
		step := func(d float64) { p((float64(i) + min(max(d, 0), 1)) / n * 0.98) }
		job := &job{ctx: ctx, src: in, dst: out, from: byID[route.through[i]], to: next, o: o, p: step, name: name}
		if err := e.run(job); err != nil {
			return "", err
		}
		if err := ctx.Err(); err != nil {
			return "", err
		}
		if job.dst != out { // the step wrote several files, in a folder
			out = job.dst
		}
		in = out
	}
	ext := ""
	if to.ID != "folder" {
		ext = to.exts[0]
	}
	if st, err := os.Stat(in); err == nil && st.IsDir() {
		ext = "" // several files, in a folder of the name
	}
	// The name may have been taken while converting.
	dst := UniquePath(dir, name, ext)
	if err := os.Rename(in, dst); err != nil {
		return "", err
	}
	if st, err := os.Stat(dst); err == nil && !st.IsDir() {
		_ = os.Chmod(dst, 0o644)
	}
	p(1)
	return dst, nil
}

// UniquePath returns dir/name.ext, or dir/name (2).ext and so on when taken;
// dir/name for an ext of "".
func UniquePath(dir, name, ext string) string {
	dot := ""
	if ext != "" {
		dot = "." + ext
	}
	p := filepath.Join(dir, name+dot)
	for i := 2; ; i++ {
		if _, err := os.Lstat(p); errors.Is(err, os.ErrNotExist) {
			return p
		}
		p = filepath.Join(dir, fmt.Sprintf("%s (%d)%s", name, i, dot))
	}
}

// job is a step of a conversion: the file src, of the format from, to be
// written at dst in the format to.
type job struct {
	ctx      context.Context
	src, dst string
	from, to Format
	o        Options
	p        Progress
	// name is the name the result takes, for what it holds: the pages of a
	// document are name 1.png and so on.
	name string
}

// edge is a converter: what an engine writes, of what it reads.
type edge struct {
	engine   string
	from, to []string
	cost     int
	// final edges are the last step of a route only: they write several
	// files, as a document's pages.
	final bool
	// first edges are the first step only: what they read must be the
	// file itself, as an animated GIF or a PDF with text, not a format a
	// step made of something else.
	first bool
	// ok says whether the engine is there; nil when it always is.
	ok  func() bool
	run func(*job) error
}

var (
	edgesMu sync.RWMutex
	edges   []*edge
	plans   = map[string]map[string]*route{}
)

// register adds converters, as engines are found.
func register(es ...*edge) {
	edgesMu.Lock()
	defer edgesMu.Unlock()
	edges = append(edges, es...)
	clear(plans)
}

// replan forgets the routes planned, once an engine comes or goes.
func replan() {
	edgesMu.Lock()
	clear(plans)
	edgesMu.Unlock()
}

// route is the steps from a format to another, through the formats
// through, its first the source's.
type route struct {
	steps   []*edge
	through []string
	cost    int
}

// plan returns the cheapest route from the format from to every format it
// reaches in up to three steps.
func plan(from string) map[string]*route {
	edgesMu.RLock()
	if r, ok := plans[from]; ok {
		edgesMu.RUnlock()
		return r
	}
	edgesMu.RUnlock()
	edgesMu.Lock()
	defer edgesMu.Unlock()
	if r, ok := plans[from]; ok {
		return r
	}
	var live []*edge
	for _, e := range edges {
		if e.ok == nil || e.ok() {
			live = append(live, e)
		}
	}
	best := map[string]*route{from: {through: []string{from}}}
	frontier := []string{from}
	for range 3 {
		var next []string
		for _, at := range frontier {
			r := best[at]
			if len(r.steps) > 0 && r.steps[len(r.steps)-1].final {
				continue
			}
			for _, e := range live {
				if !slices.Contains(e.from, at) || e.first && len(r.steps) > 0 {
					continue
				}
				for _, to := range e.to {
					if to == at {
						continue
					}
					cost := r.cost + e.cost
					if old, ok := best[to]; ok && old.cost <= cost {
						continue
					}
					best[to] = &route{
						steps:   append(slices.Clone(r.steps), e),
						through: append(slices.Clone(r.through), to),
						cost:    cost,
					}
					next = append(next, to)
				}
			}
		}
		frontier = next
	}
	delete(best, from)
	plans[from] = best
	return best
}

// sameFormat is a route of one step from a format to itself.
func sameFormat(id string) *route {
	edgesMu.RLock()
	defer edgesMu.RUnlock()
	for _, e := range edges {
		if (e.ok == nil || e.ok()) && slices.Contains(e.from, id) && slices.Contains(e.to, id) {
			return &route{steps: []*edge{e}, through: []string{id, id}}
		}
	}
	return nil
}

// Engines describes the engines and whether each is there, for the
// settings.
func Engines() []EngineStatus {
	return []EngineStatus{
		{Name: "FFmpeg", Ready: ffmpegPath() != "", Handles: "视频与音频"},
		{Name: "PDFium", Ready: true, Handles: "PDF 转图片与文本"},
		{Name: "LibreOffice", Ready: officePath() != "", Handles: "Word、Excel、PPT 与 PDF 排版"},
	}
}

// EngineStatus is whether an engine is there and what it converts.
type EngineStatus struct {
	Name    string
	Ready   bool
	Handles string
}

// writeVia runs a converter that writes to a stream, into job.dst.
func writeVia(j *job, write func(w *os.File) error) error {
	f, err := os.Create(j.dst)
	if err != nil {
		return err
	}
	err = write(f)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	return err
}

// Formats returns every format, in the order they are offered.
func Formats() []Format { return slices.Clone(formats) }
