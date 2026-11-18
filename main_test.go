package main

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"formigo/internal/convert"

	"github.com/egoist/mygo/ui"
)

// pumped runs the app's updates from the workers on the test's goroutine,
// as the window's Update runs them on the main thread.
type pumped struct {
	a  *formigo
	tt *ui.Tester
	ch chan func()
}

func newPumped(t *testing.T, w, h int) *pumped {
	a := newApp()
	ch := make(chan func(), 1024)
	a.update = func(f func()) { ch <- f }
	tt := ui.NewTester(a.view, w, h)
	return &pumped{a, tt, ch}
}

// settle runs updates until nothing is converting or describing, then lets
// the animations come to rest.
func (p *pumped) settle(t *testing.T) {
	t.Helper()
	deadline := time.After(time.Minute)
	for {
		select {
		case f := <-p.ch:
			f()
			p.tt.Frame()
			continue
		case <-deadline:
			t.Fatal("conversions never finished")
		default:
		}
		if !p.a.running && len(p.ch) == 0 {
			break
		}
		select {
		case f := <-p.ch:
			f()
			p.tt.Frame()
		case <-deadline:
			t.Fatal("conversions never finished")
		}
	}
	for range 40 {
		p.tt.Advance(50 * time.Millisecond)
	}
}

func shot(t *testing.T, tt *ui.Tester, name string) {
	dir := os.Getenv("FORMIGO_SHOTS")
	if dir == "" {
		return
	}
	var b bytes.Buffer
	png.Encode(&b, tt.Image())
	os.WriteFile(filepath.Join(dir, name+".png"), b.Bytes(), 0o644)
}

func samples(t *testing.T) (dir string, files []string) {
	dir = t.TempDir()
	img := image.NewNRGBA(image.Rect(0, 0, 120, 80))
	for y := range 80 {
		for x := range 120 {
			img.SetNRGBA(x, y, color.NRGBA{uint8(x * 2), uint8(y * 3), 200, 255})
		}
	}
	var b bytes.Buffer
	png.Encode(&b, img)
	files = append(files, filepath.Join(dir, "海报.png"))
	os.WriteFile(files[0], b.Bytes(), 0o644)
	files = append(files, filepath.Join(dir, "config.json"))
	os.WriteFile(files[1], []byte(`{"name": "Formigo", "tags": ["go", "ui"], "port": 8080}`), 0o644)
	files = append(files, filepath.Join(dir, "README.md"))
	os.WriteFile(files[2], []byte("# Formigo\n\n本地转换。\n"), 0o644)
	files = append(files, filepath.Join(dir, "notes.xyz"))
	os.WriteFile(files[3], []byte("?"), 0o644)
	return dir, files
}

func TestEmptyWindow(t *testing.T) {
	p := newPumped(t, 1040, 680)
	p.tt.Advance(time.Second)
	for _, s := range []string{"把文件丢给 Formigo", "选择文件", "图片", "音频", "数据", "文档"} {
		if !p.tt.HasText(s) {
			t.Errorf("no %q in %q", s, p.tt.Texts())
		}
	}
	shot(t, p.tt, "01-empty")
	p.tt.SetDark(true)
	p.tt.Advance(time.Second)
	shot(t, p.tt, "02-empty-dark")
}

func TestConvertFlow(t *testing.T) {
	dir, files := samples(t)
	p := newPumped(t, 1040, 680)
	p.tt.Frame()
	if !p.tt.DropFiles(500, 400, files...) {
		t.Fatal("the window took no drop")
	}
	p.settle(t)
	if len(p.a.items) != 3 {
		t.Fatalf("%d files queued, want 3 (the .xyz skipped)", len(p.a.items))
	}
	for _, s := range []string{"海报.png", "120×80", "3 个字段", "图片转换为", "数据转换为", "文档转换为", "转换 3 个文件"} {
		if !p.tt.HasText(s) {
			t.Errorf("no %q in %q", s, p.tt.Texts())
		}
	}
	shot(t, p.tt, "03-queue")

	// Choose WebP: its quality shows.
	if err := p.tt.Click("WEBP 网页"); err != nil {
		t.Fatal(err)
	}
	p.tt.Advance(time.Second)
	if p.a.targets[convert.Image] != "webp" || !p.tt.HasText("画质") {
		t.Fatalf("target %q, texts %q", p.a.targets[convert.Image], p.tt.Texts())
	}
	if err := p.tt.Click("YAML 配置"); err != nil {
		t.Fatal(err)
	}
	if err := p.tt.Click("TXT 纯文本"); err != nil {
		t.Fatal(err)
	}
	p.tt.Advance(time.Second)
	shot(t, p.tt, "04-options")

	if err := p.tt.Click("转换 3 个文件"); err != nil {
		t.Fatal(err)
	}
	p.settle(t)
	for _, name := range []string{"海报.webp", "config.yaml", "README.txt"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
	for _, it := range p.a.items {
		if it.status != done {
			t.Errorf("%s: status %d, %s", it.name, it.status, it.err)
		}
	}
	if !p.tt.HasText("全部完成 · 再转一次") {
		t.Errorf("texts %q", p.tt.Texts())
	}
	shot(t, p.tt, "06-done")

	// A right click on a file converted offers its folder and the file.
	// (Not chosen: it would open the system's file manager.)
	if err := p.tt.RightClick("海报.png"); err != nil {
		t.Fatal(err)
	}
	p.tt.Advance(300 * time.Millisecond)
	menu := strings.Join(p.tt.Menu(), "|")
	for _, s := range []string{revealLabel, "打开转换后的文件", "显示原文件", "移除"} {
		if !strings.Contains(menu, s) {
			t.Errorf("no %q in the menu %q", s, menu)
		}
	}
	shot(t, p.tt, "06b-context-menu")
	p.tt.Key(0, ui.KeyEscape)
	p.tt.Advance(300 * time.Millisecond)

	// The look of a run in progress, set by hand: the conversions above
	// are too quick to catch.
	p.a.running = true
	p.a.items[0].status, p.a.items[0].progress = working, 0.62
	p.a.items[1].status = done
	p.a.items[2].status = waiting
	p.tt.Frame()
	p.tt.Advance(1500 * time.Millisecond)
	shot(t, p.tt, "05-running")
	p.a.running = false
	p.a.items[0].status = done

	// Removing a row: hover it, then its button.
	r, ok := p.tt.Find("README.md")
	if !ok {
		t.Fatal("no README row")
	}
	p.a.items[2].status = waiting
	p.tt.Move(r.X+10, r.Y+5)
	p.tt.Advance(300 * time.Millisecond)
	if err := p.tt.Click("移除"); err != nil {
		t.Fatal(err)
	}
	p.tt.Advance(time.Second)
	if len(p.a.items) != 2 || p.tt.HasText("README.md") {
		t.Errorf("%d items after removing, texts %q", len(p.a.items), p.tt.Texts())
	}
}

func TestFailureAndRetry(t *testing.T) {
	dir := t.TempDir()
	bad := filepath.Join(dir, "broken.json")
	os.WriteFile(bad, []byte(`{"a": `), 0o644)
	p := newPumped(t, 1040, 680)
	p.a.add([]string{bad})
	p.tt.Frame()
	p.settle(t)
	if err := p.tt.Click("转换 1 个文件"); err != nil {
		t.Fatal(err)
	}
	p.settle(t)
	if p.a.items[0].status != failed || !p.tt.HasText("重试 1 个文件") {
		t.Fatalf("status %d, texts %q", p.a.items[0].status, p.tt.Texts())
	}
	shot(t, p.tt, "07-failed")
}

func TestSameDocument(t *testing.T) {
	dir := t.TempDir()
	md := filepath.Join(dir, "a.md")
	os.WriteFile(md, []byte("# a\n"), 0o644)
	p := newPumped(t, 1040, 680)
	p.a.add([]string{md})
	p.a.targets[convert.Doc] = "md"
	p.tt.Frame()
	p.settle(t)
	if err := p.tt.Click("转换 1 个文件"); err != nil {
		t.Fatal(err)
	}
	p.settle(t)
	if entries, _ := os.ReadDir(dir); len(entries) != 1 || !p.tt.HasText("已是 MD 格式") || p.a.running {
		t.Errorf("%d files, running %v, texts %q", len(entries), p.a.running, p.tt.Texts())
	}
}

func TestCompact(t *testing.T) {
	_, files := samples(t)
	p := newPumped(t, 390, 844)
	p.tt.Advance(time.Second)
	shot(t, p.tt, "08-compact-empty")
	p.a.add(files[:3])
	p.tt.Frame()
	p.settle(t)
	if !p.tt.HasText("转换 3 个文件") || !p.tt.HasText("图片转换为") {
		t.Errorf("texts %q", p.tt.Texts())
	}
	shot(t, p.tt, "09-compact-queue")
}

func TestEasterEgg(t *testing.T) {
	p := newPumped(t, 1040, 680)
	p.tt.Advance(time.Second)
	// Four quick clicks do nothing; the fifth plays the launch again.
	for i := range 5 {
		if i == 4 && p.a.intro != 0 {
			t.Fatal("replayed before the fifth click")
		}
		if err := p.tt.Click("Formigo"); err != nil {
			t.Fatal(err)
		}
		p.tt.Advance(120 * time.Millisecond)
	}
	if p.a.intro != 1 || !p.a.encoreAt.IsZero() {
		t.Fatalf("intro %d, encore %v on the empty window", p.a.intro, p.a.encoreAt)
	}

	// Over the queue, the replay covers it for a while, then goes.
	_, files := samples(t)
	p.a.add(files[:1])
	p.tt.Frame()
	p.settle(t)
	for range 5 {
		if err := p.tt.Click("Formigo"); err != nil {
			t.Fatal(err)
		}
		p.tt.Advance(120 * time.Millisecond)
	}
	if p.a.intro != 2 || p.a.encoreAt.IsZero() || !p.tt.HasText("把文件丢给 Formigo") {
		t.Fatalf("intro %d, encore %v, texts %q", p.a.intro, p.a.encoreAt, p.tt.Texts())
	}
	shot(t, p.tt, "13-encore")
	for range 12 {
		p.tt.Advance(100 * time.Millisecond)
	}
	shot(t, p.tt, "14-encore-mid")
	for range 28 {
		p.tt.Advance(100 * time.Millisecond)
	}
	if !p.a.encoreAt.IsZero() || p.tt.HasText("把文件丢给 Formigo") {
		t.Errorf("the replay stayed: encore %v, texts %q", p.a.encoreAt, p.tt.Texts())
	}

	// Slow clicks are not the egg.
	for range 5 {
		p.tt.Click("Formigo")
		p.tt.Advance(time.Second)
	}
	if p.a.intro != 2 {
		t.Errorf("slow clicks replayed it: intro %d", p.a.intro)
	}
}

func TestBoop(t *testing.T) {
	p := newPumped(t, 1040, 680)
	p.tt.Advance(time.Second)
	g, ok := p.tt.Access("小地鼠")
	if !ok {
		t.Fatal("no gopher")
	}
	p.tt.ClickAt(g.Bounds.X+g.Bounds.W/2, g.Bounds.Y+g.Bounds.H/2)
	if p.a.boops != 1 || p.a.boopAt.IsZero() {
		t.Fatalf("boops %d after a tap", p.a.boops)
	}
	p.tt.Advance(240 * time.Millisecond)
	shot(t, p.tt, "27-boop")
	// A boop is only fun: nothing else happens.
	p.tt.Advance(time.Second)
	if len(p.ch) != 0 {
		t.Errorf("a boop did more than hop: %d updates queued", len(p.ch))
	}
}

// Booping the gopher moves it in frames that only paint: the view builds
// where the motion turns, not in every frame.
func TestMotionOnlyPaints(t *testing.T) {
	p := newPumped(t, 1040, 680)
	p.tt.Advance(2 * time.Second)
	g, ok := p.tt.Access("小地鼠")
	if !ok {
		t.Fatal("no gopher")
	}
	p.tt.ClickAt(g.Bounds.X+g.Bounds.W/2, g.Bounds.Y+g.Bounds.H/2)
	before := p.a.builds
	for range 50 { // 800 ms of frames
		p.tt.Advance(16 * time.Millisecond)
	}
	if n := p.a.builds - before; n > 6 {
		t.Errorf("a boop built the view %d times in 50 frames", n)
	}
}
