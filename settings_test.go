package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"formigo/internal/convert"

	"github.com/egoist/mygo/ui"
)

func TestSettingsPersist(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Formigo", "settings.json")
	a := newApp()
	a.prefsPath = path
	a.load()
	a.prefs.Accent, a.prefs.Suffix, a.prefs.Motion, a.prefs.Mascot = "orange", "-f", "calm", false
	a.targets[convert.Image], a.quality, a.depth = "webp", 70, 1
	a.persist()

	b := newApp()
	b.prefsPath = path
	b.load()
	if b.prefs.Accent != "orange" || b.prefs.Suffix != "-f" || b.prefs.Motion != "calm" || b.prefs.Mascot {
		t.Errorf("prefs came back as %+v", b.prefs)
	}
	if b.targets[convert.Image] != "webp" || b.quality != 70 || b.depth != 1 {
		t.Errorf("choices came back as %v %v %v", b.targets, b.quality, b.depth)
	}

	// What a file says that makes no sense is left at the defaults.
	os.WriteFile(path, []byte(`{"accent":"plaid","motion":"wild","workers":99,"targets":{"image":"mp3"},"quality":3,"where":1,"outDir":"/nope"}`), 0o644)
	c := newApp()
	c.prefsPath = path
	c.load()
	if c.prefs.Accent != "neutral" || c.prefs.Motion != "full" || c.prefs.Workers != defaultWorkers() ||
		c.targets[convert.Image] != "png" || c.quality != 85 || c.where != 0 {
		t.Errorf("bad values kept: %+v targets %v quality %v where %d", c.prefs, c.targets, c.quality, c.where)
	}
	os.WriteFile(path, []byte(`not json`), 0o644)
	d := newApp()
	d.prefsPath = path
	d.load()
	if d.prefs.Accent != "neutral" || !d.prefs.Mascot {
		t.Errorf("a broken file should leave the defaults: %+v", d.prefs)
	}
}

func TestSettingsWindow(t *testing.T) {
	a := newApp()
	tt := ui.NewTester(a.settingsView, 720, 560)
	tt.Advance(time.Second)
	for _, s := range []string{"外观", "主题色", "动画", "小地鼠"} {
		if !tt.HasText(s) {
			t.Errorf("no %q in %q", s, tt.Texts())
		}
	}
	if tt.HasText("界面密度") {
		t.Error("the density is gone from the settings")
	}
	shot(t, tt, "20-settings-look")
	if err := tt.Click("暖橘"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("深色"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(time.Second)
	if a.prefs.Accent != "orange" || a.prefs.Appearance != "dark" {
		t.Fatalf("accent %q, appearance %q", a.prefs.Accent, a.prefs.Appearance)
	}
	shot(t, tt, "21-settings-look-dark")

	if err := tt.Click("转换"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(time.Second)
	if a.settingsPage != "convert" || !tt.HasText("默认格式") || !tt.HasText("同时转换") {
		t.Fatalf("page %q, texts %q", a.settingsPage, tt.Texts())
	}
	shot(t, tt, "22-settings-convert")
	// The stepper steps.
	st, ok := tt.Access("同时转换的文件数")
	if !ok {
		t.Fatal("no stepper")
	}
	before := a.prefs.Workers
	for i := 0; i < 20 && st.Bounds.Y+st.Bounds.H > 540; i++ {
		tt.Scroll(400, 300, 0, 120)
		tt.Advance(200 * time.Millisecond)
		st, _ = tt.Access("同时转换的文件数")
	}
	tt.ClickAt(st.Bounds.X+st.Bounds.W/2, st.Bounds.Y+st.Bounds.H*0.25)
	tt.Advance(300 * time.Millisecond)
	if a.prefs.Workers != min(before+1, 8) {
		t.Errorf("stepper: workers %d, was %d", a.prefs.Workers, before)
	}

	if err := tt.Click("输出"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(time.Second)
	field, ok := tt.AccessByRole(ui.RoleTextField, "文件名后缀")
	if !ok {
		t.Fatal("no suffix field")
	}
	tt.ClickAt(field.Bounds.X+field.Bounds.W/2, field.Bounds.Y+field.Bounds.H/2)
	tt.Type("-formigo")
	tt.Advance(time.Second)
	if a.prefs.Suffix != "-formigo" || !tt.HasText("照片-formigo.png") {
		t.Fatalf("suffix %q, texts %q", a.prefs.Suffix, tt.Texts())
	}
	shot(t, tt, "23-settings-output")

	if err := tt.Click("关于"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(time.Second)
	shot(t, tt, "24-settings-about")
	if err := tt.Click("恢复默认设置"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(500 * time.Millisecond)
	if err := tt.Click("恢复默认"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(500 * time.Millisecond)
	if a.prefs.Accent != "neutral" || a.prefs.Appearance != "system" || a.prefs.Suffix != "" {
		t.Errorf("not reset: %+v", a.prefs)
	}
}

func TestSuffixAndMascotOff(t *testing.T) {
	dir, files := samples(t)
	p := newPumped(t, 1040, 680)
	p.a.prefs.Suffix, p.a.prefs.Mascot, p.a.prefs.Workers = " -done ", false, 1
	p.tt.Advance(time.Second)
	if _, ok := p.tt.Access("选择文件"); !ok {
		t.Error("no way to choose files without the gopher")
	}
	shot(t, p.tt, "26-no-mascot")
	p.a.add(files[1:2])
	p.tt.Frame()
	p.settle(t)
	if err := p.tt.Click("转换 1 个文件"); err != nil {
		t.Fatal(err)
	}
	p.settle(t)
	if _, err := os.Stat(filepath.Join(dir, "config-done.json")); err != nil {
		t.Errorf("suffix not used: %v", err)
	}
}

func TestAccentInk(t *testing.T) {
	// Every accent in either theme carries its text at 4.5:1 or more.
	for _, ac := range accents {
		for _, dark := range []bool{false, true} {
			a := newApp()
			a.prefs.Accent = ac.id
			a.prefs.Appearance = map[bool]string{false: "light", true: "dark"}[dark]
			var p palette
			ui.NewTester(func(c *ui.Context) { p = a.palette(c) }, 10, 10).Frame()
			if r := contrast(p.accent, p.accentInk); r < 4.5 {
				t.Errorf("%s dark=%v: contrast %.2f", ac.id, dark, r)
			}
		}
	}
}

// TestOLED: in the dark, the settings offer pure black, which the windows
// then show; in the light the switch is not there.
func TestOLED(t *testing.T) {
	a := newApp()
	a.prefs.Appearance = "light"
	tt := ui.NewTester(a.settingsView, 720, 560)
	tt.Advance(time.Second)
	if tt.HasText("OLED 纯黑") {
		t.Error("pure black offered in the light")
	}
	// Set behind the view's back, as no tap would: the next frame builds.
	a.prefs.Appearance = "dark"
	tt.Frame()
	tt.Advance(time.Second)
	sw, ok := tt.AccessByRole(ui.RoleSwitch, "OLED 纯黑")
	if !ok {
		t.Fatalf("no switch in the dark: %q", tt.Texts())
	}
	tt.ClickAt(sw.Bounds.X+sw.Bounds.W/2, sw.Bounds.Y+sw.Bounds.H/2)
	tt.Advance(time.Second)
	if !a.prefs.OLED {
		t.Fatal("the switch did not turn pure black on")
	}
	shot(t, tt, "21b-settings-oled")
	var p palette
	ui.NewTester(func(c *ui.Context) { p = a.palette(c) }, 10, 10).Frame()
	if p.bg != ui.Hex("#000000") {
		t.Errorf("background %v", p.bg)
	}
	_, files := samples(t)
	m := newPumped(t, 1040, 680)
	m.a.prefs = a.prefs
	m.a.add(files[:3])
	m.tt.Frame()
	m.settle(t)
	shot(t, m.tt, "02b-queue-oled")
}

// TestThemes: each theme, chosen in the settings, colors the dark window,
// on graphite rather than navy.
func TestThemes(t *testing.T) {
	_, files := samples(t)
	for _, ac := range accents {
		a := newApp()
		a.prefs.Appearance = "dark"
		tt := ui.NewTester(a.settingsView, 720, 560)
		tt.Advance(time.Second)
		if err := tt.Click(ac.name); err != nil {
			t.Fatal(err)
		}
		tt.Advance(time.Second)
		if a.prefs.Accent != ac.id {
			t.Fatalf("chose %s, got %q", ac.name, a.prefs.Accent)
		}
		shot(t, tt, "30-theme-"+ac.id+"-settings")
		var p palette
		ui.NewTester(func(c *ui.Context) { p = a.palette(c) }, 10, 10).Frame()
		if p.accent != ac.dark || p.bg != ui.Hex("#111214") {
			t.Errorf("%s: accent %v, background %v", ac.id, p.accent, p.bg)
		}
		m := newPumped(t, 1040, 680)
		m.a.prefs = a.prefs
		m.a.add(files[:3])
		m.tt.Frame()
		m.settle(t)
		shot(t, m.tt, "30-theme-"+ac.id+"-queue")
		h := newPumped(t, 1040, 680)
		h.a.prefs = a.prefs
		h.tt.Advance(2 * time.Second)
		shot(t, h.tt, "30-theme-"+ac.id+"-hero")
	}
}

// TestWithoutFFmpeg: where there is no FFmpeg, video has no formats: the
// settings say so, and movies are not taken.
func TestWithoutFFmpeg(t *testing.T) {
	convert.SetFFmpeg("")
	t.Cleanup(func() {
		for _, p := range []string{os.Getenv("FORMIGO_FFMPEG"), "/opt/homebrew/bin/ffmpeg", "/usr/local/bin/ffmpeg", "/usr/bin/ffmpeg"} {
			if st, err := os.Stat(p); p != "" && err == nil && !st.IsDir() {
				convert.SetFFmpeg(p)
				return
			}
		}
	})
	a := newApp()
	a.settingsPage = "convert"
	tt := ui.NewTester(a.settingsView, 720, 560)
	tt.Advance(time.Second)
	if !tt.HasText("不可用") {
		t.Errorf("video shows formats without FFmpeg: %q", tt.Texts())
	}
	dir := t.TempDir()
	clip := filepath.Join(dir, "clip.mov")
	os.WriteFile(clip, []byte("not really a movie"), 0o644)
	m := newPumped(t, 1040, 680)
	if added, skipped := m.a.add([]string{clip}); added != 0 || skipped != 1 {
		t.Errorf("a movie without FFmpeg: added %d, skipped %d", added, skipped)
	}
	m.tt.Frame()
}
