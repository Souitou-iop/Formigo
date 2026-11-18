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

// accent is a color the user may choose for the app, flat as the icon's.
type accent struct {
	id, name    string
	light, dark ui.Color
}

var accents = []accent{
	{"mint", "薄荷", ui.Hex("#11996a"), ui.Hex("#3ccf9e")},
	{"orange", "暖橘", ui.Hex("#e2681c"), ui.Hex("#ff9447")},
	{"sky", "地鼠天蓝", ui.Hex("#1f7fe0"), ui.Hex("#5cb9f7")},
	{"neutral", "中性白", ui.Hex("#1a1b1f"), ui.Hex("#ececee")},
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
	p := paletteFor(dark)
	if dark && a.prefs.OLED {
		p = oled(p)
	}
	ac := accentByID(a.prefs.Accent)
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
