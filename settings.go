package main

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"runtime"

	"formigo/internal/convert"

	"github.com/egoist/mygo/ui"
)

// settings is what Formigo remembers between runs: how it looks and moves,
// and the choices of the last conversions, kept in a JSON file of the app's
// data directory.
type settings struct {
	Appearance string `json:"appearance"` // "system", "light" or "dark"
	Accent     string `json:"accent"`     // an accent's ID
	Motion     string `json:"motion"`     // "full", "calm" or "off"
	Mascot     bool   `json:"mascot"`     // the gopher shows
	OLED       bool   `json:"oled"`       // dark is pure black, for OLED screens

	Targets map[string]string `json:"targets"` // a kind's format, by kind
	Quality float64           `json:"quality"`
	Scale   string            `json:"scale"`
	Depth   int               `json:"depth"`
	Mono    bool              `json:"mono"`
	Indent  int               `json:"indent"`
	Workers int               `json:"workers"`

	Where  int    `json:"where"`
	OutDir string `json:"outDir"`
	Suffix string `json:"suffix"` // added to the names of files written
	Notify bool   `json:"notify"` // a notification once a run is done
	Reveal bool   `json:"reveal"` // the folder shows once a run is done
}

// accent is a theme the user may choose for the app: a main color, flat as
// the icon's, and the tones it gives the rest of the window.
type accent struct {
	id, name    string
	light, dark ui.Color
	day, night  tones
}

// tones are what a theme tints beyond its main color. In the light
// appearance: the page, raised and highlighted fills, and the deep ink of
// text, sticker edges and their hard shadows. In the dark one: the page and
// the surfaces on it, faintly.
type tones struct {
	bg, panel, surface, raised, tint string
	ink, muted, faint                string // light appearance only
}

// The same table is in Android's Theme.kt and iOS's Theme.swift.
var accents = []accent{
	{"mint", "薄荷", ui.Hex("#11996a"), ui.Hex("#3ccf9e"),
		tones{bg: "#f0f7f3", raised: "#e5f4ec", tint: "#dbf0e5", ink: "#0b3626", muted: "#4f6b5f", faint: "#86a094"},
		tones{bg: "#101614", panel: "#151c19", surface: "#19211e", raised: "#212b27", tint: "#1c3129"}},
	{"orange", "暖橘", ui.Hex("#e2681c"), ui.Hex("#ff9447"),
		tones{bg: "#fbf5f0", raised: "#fcebdf", tint: "#fbe2d0", ink: "#3d1d0a", muted: "#7a5c4a", faint: "#ab9283"},
		tones{bg: "#161210", panel: "#1c1714", surface: "#211b17", raised: "#2b231e", tint: "#382619"}},
	{"sky", "地鼠天蓝", ui.Hex("#1f7fe0"), ui.Hex("#5cb9f7"),
		tones{bg: "#f2f6fc", raised: "#e9f3fe", tint: "#e3f2fe", ink: "#14213d", muted: "#56627e", faint: "#8e99b2"},
		tones{bg: "#10141a", panel: "#151a21", surface: "#192027", raised: "#212933", tint: "#1c2c3b"}},
	{"neutral", "中性黑白", ui.Hex("#1a1b1f"), ui.Hex("#ececee"),
		tones{bg: "#f4f4f5", raised: "#ededee", tint: "#e7e7e9", ink: "#1a1b1f", muted: "#5f6168", faint: "#95979e"},
		tones{bg: "#111214", panel: "#16171a", surface: "#1a1b1f", raised: "#222328", tint: "#24262b"}},
}

func accentByID(id string) accent {
	for _, a := range accents {
		if a.id == id {
			return a
		}
	}
	return accents[3] // neutral, the default
}

var kindIDs = map[convert.Kind]string{
	convert.Image: "image", convert.Video: "video", convert.Audio: "audio", convert.Doc: "doc",
	convert.Data: "data", convert.Subtitle: "subtitle", convert.Archive: "archive", convert.Font: "font",
}

func defaultWorkers() int { return max(1, min(4, runtime.NumCPU()/2)) }

// defaults are a new install's settings.
func defaults() settings {
	s := settings{
		Appearance: "system", Accent: "neutral", Motion: "full", Mascot: true,
		Targets: map[string]string{}, Quality: 85, Scale: scales[0],
		Workers: defaultWorkers(), Notify: true,
	}
	for k, id := range kindIDs {
		s.Targets[id] = convert.DefaultTarget(k)
	}
	return s
}

// snapshot is the app's state as settings.
func (a *formigo) snapshot() settings {
	s := a.prefs
	s.Targets = map[string]string{}
	for k, id := range kindIDs {
		s.Targets[id] = a.targets[k]
	}
	s.Quality, s.Scale, s.Depth, s.Mono, s.Indent = a.quality, a.scale, a.depth, a.mono, a.indent
	s.Where, s.OutDir = a.where, a.outDir
	return s
}

// apply puts settings into the app's state, keeping what is valid.
func (a *formigo) apply(s settings) {
	d := defaults()
	if s.Appearance != "light" && s.Appearance != "dark" {
		s.Appearance = "system"
	}
	if s.Motion != "calm" && s.Motion != "off" {
		s.Motion = "full"
	}
	s.Accent = accentByID(s.Accent).id
	if s.Workers < 1 || s.Workers > 8 {
		s.Workers = d.Workers
	}
	for k, id := range kindIDs {
		if f, ok := convert.Lookup(s.Targets[id]); ok && f.Kind == k {
			a.targets[k] = f.ID
		}
	}
	if s.Quality >= 10 && s.Quality <= 100 {
		a.quality = s.Quality
	}
	for _, sc := range scales {
		if s.Scale == sc {
			a.scale = sc
		}
	}
	a.depth, a.mono = min(max(s.Depth, 0), 1), s.Mono
	a.indent = min(max(s.Indent, 0), 2)
	a.where, a.outDir = 0, ""
	if s.Where == 1 {
		if st, err := os.Stat(s.OutDir); err == nil && st.IsDir() {
			a.where, a.outDir = 1, s.OutDir
		}
	}
	a.prefs = s
}

// load reads the settings file, if there is one.
func (a *formigo) load() {
	a.apply(defaults())
	if a.prefsPath == "" {
		return
	}
	data, err := os.ReadFile(a.prefsPath)
	if err != nil {
		return
	}
	s := defaults()
	if json.Unmarshal(data, &s) == nil {
		a.apply(s)
	}
	a.saved, _ = json.Marshal(a.snapshot())
}

// persist writes the settings when they changed, and has every window show
// the change. Views call it as they finish building.
func (a *formigo) persist() {
	data, err := json.MarshalIndent(a.snapshot(), "", "  ")
	if err != nil || string(data) == string(a.saved) {
		return
	}
	a.saved = data
	if a.win != nil {
		a.win.Invalidate()
	}
	if a.settingsWin != nil {
		a.settingsWin.Invalidate()
	}
	if a.prefsPath == "" {
		return
	}
	os.MkdirAll(filepath.Dir(a.prefsPath), 0o755)
	tmp := a.prefsPath + ".tmp"
	if os.WriteFile(tmp, data, 0o644) == nil {
		os.Rename(tmp, a.prefsPath)
	}
}

// reset goes back to a new install's settings.
func (a *formigo) reset() {
	a.apply(defaults())
}

// dark says whether the window shows dark: as the system does, or as the
// user chose.
func (a *formigo) dark(c *ui.Context) bool {
	switch a.prefs.Appearance {
	case "light":
		return false
	case "dark":
		return true
	}
	return c.Theme().Dark
}

// palette is the window's colors: the light or dark one, as the appearance
// says, in the accent chosen.
func (a *formigo) palette(c *ui.Context) palette {
	dark := a.dark(c)
	a.darkNow = dark
	ac := accentByID(a.prefs.Accent)
	p := themed(paletteFor(dark), ac, dark)
	if dark && a.prefs.OLED {
		p = oled(p)
	}
	p.accent = ac.light
	if dark {
		p.accent = ac.dark
	}
	p.accentPressed = p.accent.MixOklab(p.outline, 0.22)
	p.glow = p.accent.Alpha(0.25)
	// White or graphite on the accent, whichever reads better.
	navy := ui.Hex("#101113")
	if contrast(p.accent, navy) > contrast(p.accent, p.accentInk) {
		p.accentInk = navy
	}
	return p
}

// themed gives a palette its theme's tones.
func themed(p palette, ac accent, dark bool) palette {
	if dark {
		t := ac.night
		p.bg, p.panel, p.surface, p.raised = ui.Hex(t.bg), ui.Hex(t.panel), ui.Hex(t.surface), ui.Hex(t.raised)
		p.skyTint = ui.Hex(t.tint)
		return p
	}
	t := ac.day
	p.bg, p.raised, p.skyTint = ui.Hex(t.bg), ui.Hex(t.raised), ui.Hex(t.tint)
	p.outline, p.ink, p.muted, p.faint = ui.Hex(t.ink), ui.Hex(t.ink), ui.Hex(t.muted), ui.Hex(t.faint)
	p.line, p.lineStrong = p.ink.Alpha(0.11), p.ink.Alpha(0.2)
	return p
}

// contrast is the WCAG contrast ratio of two colors.
func contrast(a, b ui.Color) float64 {
	l1, l2 := luminance(a), luminance(b)
	if l1 < l2 {
		l1, l2 = l2, l1
	}
	return (l1 + 0.05) / (l2 + 0.05)
}

// luminance is a color's relative luminance, as WCAG measures contrast.
func luminance(c ui.Color) float64 {
	lin := func(v uint8) float64 {
		x := float64(v) / 255
		if x <= 0.04045 {
			return x / 12.92
		}
		return math.Pow((x+0.055)/1.055, 2.4)
	}
	return 0.2126*lin(c.R) + 0.7152*lin(c.G) + 0.0722*lin(c.B)
}

// ambient says whether motion that only decorates plays: the orbit, the
// gopher's bobbing, the sheen. It stops for calm and no motion, and when
// the system asks for less.
func (a *formigo) ambient(c *ui.Context) bool {
	return a.prefs.Motion == "full" && !c.Preferences().ReduceMotion
}
