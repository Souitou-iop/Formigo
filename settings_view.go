package main

import (
	"fmt"
	"math"
	"path/filepath"
	"time"

	"formigo/internal/convert"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// accentMatch is the identity of the ring round the chosen accent, which
// flies from swatch to swatch.
type accentMatch struct{}

var (
	appearances = []string{"system", "light", "dark"}
	motions     = []string{"full", "calm", "off"}
)

// settingsView is the settings: a window with a sidebar of pages.
func (a *formigo) settingsView(c *ui.Context) {
	p := a.palette(c)
	a.theme(c, p)
	if a.prefs.Motion == "off" {
		c.DisableAnimations()
	}
	if c.Shortcut(ui.Cmd, ui.KeyW) && a.settingsWin != nil {
		a.settingsWin.Close()
	}
	a.settingsPages(c, p)
	a.resetQuestion(c)
	a.persist()
}

// settingsPages is the settings: the pages in a sidebar, the page chosen
// beside it.
func (a *formigo) settingsPages(c *ui.Context, p palette) {
	bar := c.TitleBar()
	ui.Row(c).Fill().AlignItems(ui.Stretch).Background(p.bg).Children(func() {
		ui.Column(c.Key("side")).Width(208).Padding(max(bar.Bottom, 28)+18, 12, 16, 12).Gap(14).
			Background(p.panel).BorderWidth(0, 1.5, 0, 0).BorderColor(p.line).DragWindow().Children(func() {
			ui.Row(c).Gap(8).Padding(0, 6).Children(func() {
				if a.prefs.Mascot {
					ui.Image(c, mascot).Size(30, 30).Fit(ui.Contain)
				}
				ui.Text(c, "设置").Font(rounded()).FontSize(18).FontWeight(800).TextColor(p.ink)
			})
			ui.Sidebar(c.Key("pages"), &a.settingsPage, func() {
				ui.SidebarItem(c, "look", icPalette, "外观")
				ui.SidebarItem(c, "convert", icSliders, "转换")
				ui.SidebarItem(c, "output", icFolder, "输出")
				ui.SidebarItem(c, "about", icInfo, "关于")
			}).Grow(1).Background(ui.Transparent).Label("设置的页面")
		})
		ui.Column(c.Key("page")).Grow(1).Basis(0).Children(func() {
			ui.Row(c).Height(max(bar.Height, 28)).DragWindow()
			// Pages come in from below as the sidebar changes, the one before
			// going up and away.
			ui.Box(c.Key("stage")).Grow(1).Children(func() {
				ui.Scroll(c.Key(a.settingsPage)).Absolute().Fill().
					Transition(ui.ElementTransition{Animation: &settle, ExitDuration: 140 * time.Millisecond,
						Enter: &ui.Motion{Y: 14, Blur: 6}, Exit: &ui.Motion{Y: -10, Blur: 6}}).
					Children(func() {
						ui.Column(c).Padding(4, 32, 32, 32).Gap(24).MaxWidth(620).Children(func() {
							a.settingsContent(c, p)
						})
					})
			})
		})
	})
}

func (a *formigo) settingsContent(c *ui.Context, p palette) {
	switch a.settingsPage {
	case "convert":
		pageTitle(c, p, "转换")
		a.convertSettings(c, p)
	case "output":
		pageTitle(c, p, "输出")
		a.outputSettings(c, p)
	case "about":
		a.about(c, p)
	default:
		pageTitle(c, p, "外观")
		a.preview(c, p)
		a.lookSettings(c, p)
	}
}

func pageTitle(c *ui.Context, p palette, s string) {
	ui.Text(c, s).Font(rounded()).FontSize(26).FontWeight(800).LetterSpacing(-0.4).TextColor(p.ink).
		ContentTransition(ui.ContentTransition{Morph: true})
}

// group is a card of settings under a title, a line between its rows.
func group(c *ui.Context, p palette, title string, rows func()) {
	ui.Column(c.Key(title)).Gap(10).Children(func() {
		ui.Text(c, title).Font(rounded()).FontSize(14).FontWeight(800).TextColor(p.muted)
		ui.Column(c).Radius(16).Background(p.surface).Border(1.5, p.line).Padding(0, 16).
			Dividers(1, p.line).Gap(1).Children(rows)
	})
}

// row is a setting: its name, and the control that sets it.
func row(c *ui.Context, p palette, label string, control func()) {
	ui.Row(c.Key(label)).MinHeight(54).Padding(8, 0).Gap(16).Children(func() {
		ui.Text(c, label).FontSize(13.5).FontWeight(600).TextColor(p.ink).Grow(1)
		control()
	})
}

// toggleRow is a setting that is on or off.
func toggleRow(c *ui.Context, p palette, label string, on *bool) {
	row(c, p, label, func() { ui.Switch(c, on).Label(label) })
}

// menuLook is how Formigo's menus open: roomy rows on a card of the app's. It grows out of its button's
// corner as it opens and fades as it closes; its highlight fades from item
// to item.
func menuLook(p palette) ui.PopoverMenuButtonOptions {
	return ui.PopoverMenuButtonOptions{
		At: ui.AnchorBottomEnd, Self: ui.AnchorTopEnd,
		Panel: func(panel ui.Element) {
			panel.MinWidth(210).Padding(6).Radius(18).Background(p.surface).Border(1.5, p.lineStrong).
				TransformOrigin(1, 0).
				Transition(ui.ElementTransition{
					Animation: &snap, ExitDuration: 120 * time.Millisecond, ExitEase: ui.EaseIn,
					Enter: &ui.Motion{Scale: 0.88, Y: -8, Opacity: 0, Flip: true},
					Exit:  &ui.Motion{Scale: 0.96, Opacity: 0, Flip: true},
				})
		},
		Item: func(item ui.Element) {
			item.MinHeight(44).Padding(0, 14).Radius(12).FontSize(16).
				Transition(ui.ElementTransition{Colors: true, Duration: 110 * time.Millisecond})
		},
	}
}

// pickValue is pickRow over values stored as strings.
func pickValue(c *ui.Context, p palette, label string, value *string, values, labels []string) {
	i := 0
	for j, v := range values {
		if v == *value {
			i = j
		}
	}
	row(c, p, label, func() { ui.Segmented(c, &i, labels...).Label(label) })
	*value = values[i]
}

// choice is a segmented control over values stored as strings.
func choice(c *ui.Context, value *string, values []string, labels ...string) {
	i := 0
	for j, v := range values {
		if v == *value {
			i = j
		}
	}
	ui.Segmented(c, &i, labels...)
	*value = values[i]
}

func (a *formigo) lookSettings(c *ui.Context, p palette) {
	group(c, p, "外观", func() {
		pickValue(c, p, "外观模式", &a.prefs.Appearance, appearances, []string{"跟随系统", "浅色", "深色"})
		if a.darkNow {
			toggleRow(c, p, "OLED 纯黑", &a.prefs.OLED)
		}
		row(c, p, "主题色", func() { a.swatches(c, p) })
	})
	group(c, p, "动效", func() {
		pickValue(c, p, "动画", &a.prefs.Motion, motions, []string{"完整", "安静", "关闭"})
		toggleRow(c, p, "小地鼠", &a.prefs.Mascot)
	})
}

// swatches are the accents, as radio buttons of color; a ring flies to the
// one chosen.
func (a *formigo) swatches(c *ui.Context, p palette) {
	dark := a.darkNow
	ui.RadioGroup(c.Key("accents"), func() {
		for _, ac := range accents {
			col := ac.light
			if dark {
				col = ac.dark
			}
			on := a.prefs.Accent == ac.id
			s := ui.RadioBase(c.Key(ac.id), &a.prefs.Accent, ac.id).Size(26, 26).Radius(13).Center().
				Background(col).Label(ac.name).Tooltip(ac.name).Cursor(ui.CursorPointer)
			sticker(s, p, 2, 0)
			pose := ui.Pose{}
			switch {
			case s.Pressed():
				pose.Scale = 0.88
			case hovers(s):
				pose.Scale = 1.12
			}
			s.AnimatePose(pose, snap)
			s.Children(func() {
				if on {
					ui.Box(c.Key("ring")).Absolute().Top(-6).Left(-6).Right(-6).Bottom(-6).Radius(19).
						Border(2.5, col).PassThrough().
						Transition(ui.ElementTransition{Animation: &pop, Match: accentMatch{}})
					ui.Icon(c, icCheck).FontSize(13).TextColor(checkOn(col)).
						Transition(ui.ElementTransition{Animation: &pop, Enter: &ui.Motion{Scale: 0.3}, Exit: &ui.Motion{Scale: 0.3}})
				}
			})
		}
	}).Row().Gap(12).Padding(6).Label("主题色")
}

// preview shows the look chosen at work: a file flies into the gopher,
// the run fills in the accent, and the converted file pops out beside it,
// one pair of formats after another, at the pace the motion setting asks.
func (a *formigo) preview(c *ui.Context, p palette) {
	dark := a.darkNow
	period := demoPeriod(a.prefs.Motion, c.Preferences().ReduceMotion)
	// The gopher hops as each file lands in its arms.
	if period > 0 {
		now := c.Now()
		cyc := now.UnixNano() / int64(period)
		land := time.Unix(0, cyc*int64(period)+int64(demoLand*float64(period)))
		if !now.Before(land) {
			if a.previewAt.Before(land) {
				a.previewAt = land
			}
			land = land.Add(period)
		}
		c.After(land.Sub(now))
	}
	card := ui.Box(c.Key("preview")).Height(132).Radius(20).Background(p.surface).Border(1.5, p.line).Center()
	card.Draw(func(pt *ui.Painter, r ui.Rect) { drawDemo(pt, r, p, dark, period) })
	card.Children(func() {
		var g ui.Element
		if a.prefs.Mascot {
			g = ui.Image(c.Key("pv-gopher"), mascot).Size(96, 96).Fit(ui.Contain).
				Transition(ui.ElementTransition{Animation: &hop, Enter: &ui.Motion{Scale: 0.4, Rotate: -12}, Exit: &ui.Motion{Scale: 0.4}})
		} else {
			g = ui.Box(c.Key("pv-badge")).Size(46, 46).Radius(14).Center().Background(p.accent)
			sticker(g, p, 2, 4)
			g.Children(func() { ui.Icon(c, icConvert).FontSize(22).TextColor(p.accentInk) })
		}
		g.Label("小地鼠").Cursor(ui.CursorPointer).Margin(0, 0, 16, 0)
		pose, motion := gopherPose(c, ui.Pose{}, a.previewAt, a.ambient(c), 3, 1300*time.Millisecond)
		g.AnimatePose(pose, motion).TransformOrigin(0.5, 1)
		if g.Clicked() {
			a.previewAt = c.Now()
		}
	})
}

// demoPairs are the conversions the preview plays, in turn.
var demoPairs = [][2]satellite{
	{{"PNG", convert.Image, 0}, {"WEBP", convert.Image, 0}},
	{{"MOV", convert.Video, 0}, {"MP4", convert.Video, 0}},
	{{"MD", convert.Doc, 0}, {"HTML", convert.Doc, 0}},
	{{"WAV", convert.Audio, 0}, {"MP3", convert.Audio, 0}},
}

// demoLand is when, in a turn of the preview, the file lands on the gopher.
const demoLand = 0.42

// demoPeriod is how long a turn of the preview takes: none, standing still,
// where the motion is off.
func demoPeriod(motion string, reduce bool) time.Duration {
	switch {
	case motion == "off" || reduce:
		return 0
	case motion == "calm":
		return 5600 * time.Millisecond
	}
	return 3600 * time.Millisecond
}

// drawDemo paints a turn of the preview at the painter's time.
func drawDemo(pt *ui.Painter, r ui.Rect, p palette, dark bool, period time.Duration) {
	t, pair := 1.0, demoPairs[0]
	if period > 0 {
		n := pt.Now().UnixNano()
		t = float64(n%int64(period)) / float64(period)
		pair = demoPairs[(n/int64(period))%int64(len(demoPairs))]
		pt.AnimationFrame()
	}
	ease := func(x float64) float64 { x = min(1, max(0, x)); return 1 - (1-x)*(1-x)*(1-x) }
	pop := func(x float64) float64 { // a spring's overshoot, settling at 1
		x = min(1, max(0, x))
		return 1 + 2.4*(x-1)*(x-1)*(x-1) + 1.4*(x-1)*(x-1)
	}
	fade := float32(1)
	if period > 0 && t > 0.92 {
		fade = float32(1 - (t-0.92)/0.08)
	}
	cy := r.Y + r.H*0.42
	sx, gx, tx := r.X+r.W*0.17, r.X+r.W/2, r.X+r.W*0.83

	// The way the file goes, dashed and quiet.
	var path ui.Path
	path.MoveTo(sx+30, cy).QuadTo((sx+gx)/2, cy-34, gx-40, cy-6)
	path.MoveTo(gx+40, cy-6).QuadTo((gx+tx)/2, cy-34, tx-30, cy)
	pt.StrokePath(path.Cap(ui.RoundCap).Dash(float32(t*-40), 2, 7), 2, p.lineStrong)

	// The source, popping in, then the copy of it flying in an arc.
	if s := float32(pop(t / 0.14)); s > 0 {
		drawDoc(pt, sx, cy, 0.95*s, pair[0], p, dark, fade)
	}
	if f := (t - 0.12) / (demoLand - 0.12); f > 0 && f < 1 {
		e := ease(f)
		x := float32(float64(sx) + (float64(gx)-float64(sx))*e)
		y := float32(float64(cy) - 40*math.Sin(math.Pi*e))
		drawDoc(pt, x, y, float32(1-0.5*e), pair[0], p, dark, float32(1-0.4*e))
	}
	// The strokes of surprise as it lands.
	if f := (t - demoLand) / 0.28; f > 0 && f < 1 {
		bx, by := gx+30, r.Y+r.H*0.12
		for _, ang := range []float64{-62, -30, 2} {
			rad := ang * math.Pi / 180
			l0, l1 := float32(6), float32(6+10*(1-f))
			dx, dy := float32(math.Cos(rad)), float32(math.Sin(rad))
			var st ui.Path
			st.MoveTo(bx+dx*l0, by+dy*l0).LineTo(bx+dx*l1, by+dy*l1)
			pt.StrokePath(st.Cap(ui.RoundCap), 3.5, p.accent.Alpha(float32(1-f)))
		}
	}

	// The run, filling in the accent.
	run := ease((t - demoLand) / 0.36)
	bl, br, by := r.X+28, r.X+r.W-28, r.Y+r.H-18
	var track ui.Path
	track.MoveTo(bl, by).LineTo(br, by)
	pt.StrokePath(track.Cap(ui.RoundCap), 6, p.line)
	if run > 0 {
		var fill ui.Path
		fill.MoveTo(bl, by).LineTo(bl+max(0.5, (br-bl)*float32(run)), by)
		pt.StrokePath(fill.Cap(ui.RoundCap), 6, p.accent.Alpha(fade))
	}

	// The result, popping out, done.
	if s := float32(pop((t - 0.78) / 0.14)); t >= 0.78 && s > 0 {
		drawDoc(pt, tx, cy, 0.95*s, pair[1], p, dark, fade)
		ox, oy, rr := tx+15*s, cy-19*s, 8*s
		var dot ui.Path
		dot.Circle(ox, oy, rr)
		pt.FillPath(&dot, p.accent.Alpha(fade))
		var ck ui.Path
		ck.MoveTo(ox-3.6*s, oy).LineTo(ox-1*s, oy+2.6*s).LineTo(ox+3.8*s, oy-2.6*s)
		pt.StrokePath(ck.Cap(ui.RoundCap).Join(ui.RoundJoin), 1.8*s, p.accentInk.Alpha(fade))
	}
}

func (a *formigo) convertSettings(c *ui.Context, p palette) {
	group(c, p, "默认格式", func() {
		for _, k := range convert.Kinds {
			a.formatRow(c, p, k, k.String())
		}
	})
	group(c, p, "速度", func() {
		labels := make([]string, 8)
		for i := range labels {
			labels[i] = fmt.Sprintf("%d 个", i+1)
		}
		i := a.prefs.Workers - 1
		row(c, p, "同时转换", func() {
			n := float64(a.prefs.Workers)
			ui.Row(c).Gap(10).Children(func() {
				ui.Textf(c, "%d 个", a.prefs.Workers).FontSize(13).FontWeight(600).FontFeatures("tnum").TextColor(p.ink).
					ContentTransition(ui.ContentTransition{Numeric: true})
				ui.Stepper(c.Key("workers"), &n, 1, 8, 1).Label("同时转换的文件数")
			})
			i = int(n) - 1
		})
		a.prefs.Workers = i + 1
	})
	a.engines(c, p)
}

// engines says which engines convert, and offers LibreOffice, the user's
// to install, for old and office documents.
func (a *formigo) engines(c *ui.Context, p palette) {
	group(c, p, "转换引擎", func() {
		for _, e := range convert.Engines() {
			row(c, p, e.Name, func() {
				ui.Row(c).Gap(8).Children(func() {
					ui.Text(c, e.Handles).FontSize(12).TextColor(p.faint).MaxLines(1)
					if e.Ready {
						dot := ui.Row(c).Gap(5).Padding(3, 9).Radius(999).Background(p.ok.Alpha(0.14))
						dot.Children(func() {
							ui.Box(c).Size(6, 6).Radius(3).Background(p.ok)
							ui.Text(c, "可用").FontSize(11.5).FontWeight(650).TextColor(p.ok)
						})
						return
					}
					if e.Name != "LibreOffice" {
						ui.Text(c, "未找到").FontSize(12).TextColor(p.bad)
						return
					}
					get := ui.Button(c.Key("get-"+e.Name), "获取").Height(28).Padding(0, 12).Label("获取 " + e.Name)
					if get.Clicked() {
						go mygo.Shell.OpenExternal(convert.OfficeDownload)
					}
					again := ui.Button(c.Key("again-"+e.Name), "重新检测").Height(28).Padding(0, 12).Background(p.surface)
					if again.Clicked() {
						convert.RefreshEngines()
					}
				})
			})
		}
	})
}

// formatRow chooses the format a kind converts to.
func (a *formigo) formatRow(c *ui.Context, p palette, k convert.Kind, label string) {
	targets := convert.KindTargets(k)
	if len(targets) == 0 {
		// Nothing converts these here: video and sound without FFmpeg.
		row(c, p, label, func() { ui.Text(c, "不可用").FontSize(12.5).TextColor(p.faint) })
		return
	}
	labels := make([]string, len(targets))
	i := 0
	for j, f := range targets {
		labels[j] = f.Label + " · " + f.Note
		if f.ID == a.targets[k] {
			i = j
		}
	}
	row(c, p, label, func() {
		chosen := labels[i]
		ui.Select(c.Key(k), &chosen, labels).Width(150).Label(label + "的默认格式")
		for j, l := range labels {
			if l == chosen {
				i = j
			}
		}
	})
	a.targets[k] = targets[i].ID
}

func (a *formigo) outputSettings(c *ui.Context, p palette) {
	group(c, p, "文件", func() {
		{
			row(c, p, "存放位置", func() {
				where := a.where
				ui.Segmented(c, &where, "原文件夹", "指定文件夹").Label("存放位置")
				if where != a.where {
					a.where = where
					if where == 1 && a.outDir == "" {
						a.chooseFolder()
					}
				}
			})
			if a.where == 1 && a.outDir != "" {
				row(c, p, "文件夹", func() {
					b := ui.Button(c.Key("folder"), "").Gap(6).Padding(0, 12).Label("更改文件夹").Tooltip(a.outDir)
					b.Children(func() {
						ui.Icon(c, icFolder).FontSize(14)
						ui.Text(c, filepath.Base(a.outDir)).FontSize(12.5).MaxLines(1).MaxWidth(180)
					})
					if b.Clicked() {
						a.chooseFolder()
					}
				})
			}
		}
		row(c, p, "文件名后缀", func() {
			ui.TextInput(c.Key("suffix"), &a.prefs.Suffix).Placeholder("无").Width(150).Label("文件名后缀")
		})
		// What a name becomes, as the suffix is typed.
		example := "照片" + cleanSuffix(a.prefs.Suffix) + ".png"
		row(c, p, "示例", func() {
			ui.Row(c).Gap(6).Children(func() {
				ui.Text(c, "照片.jpg").FontSize(12.5).TextColor(p.muted)
				ui.Icon(c, icArrow).FontSize(12).TextColor(p.faint)
				ui.Text(c, example).FontSize(12.5).FontWeight(650).TextColor(p.ink).
					ContentTransition(ui.ContentTransition{Morph: true})
			})
		})
	})
	group(c, p, "完成后", func() {
		toggleRow(c, p, "发送通知", &a.prefs.Notify)
		toggleRow(c, p, revealLabel, &a.prefs.Reveal)
	})
}

// about is Formigo's name, version and gopher, and the way back to the
// defaults.
func (a *formigo) about(c *ui.Context, p palette) {
	ui.Column(c.Key("about")).Center().Gap(10).Padding(24, 0).Children(func() {
		g := ui.Image(c.Key("about-gopher"), mascot).Size(150, 150).Fit(ui.Contain).Cursor(ui.CursorPointer).Label("Formigo")
		hopY := g.Keyframes("hop", []ui.Keyframe{
			{Value: 0}, {At: 180 * time.Millisecond, Value: -20, Ease: ui.EaseOut},
			{At: 420 * time.Millisecond, Value: 0, Ease: ui.EaseIn}, {At: 520 * time.Millisecond, Value: -5, Ease: ui.EaseOut},
			{At: 620 * time.Millisecond, Value: 0, Ease: ui.EaseIn},
		}, ui.KeyframeOptions{Run: uint64(a.taps) + 100*a.intro})
		g.Translate(0, hopY)
		if g.Clicked() {
			a.tapped(c.Now())
		}
		ui.Text(c, "Formigo").Font(rounded()).FontSize(30).FontWeight(800).TextColor(p.ink)
		ui.Text(c, version()).FontSize(13).TextColor(p.muted).FontFeatures("tnum")
		ui.Row(c).Gap(8).Margin(8, 0, 0, 0).Children(func() {
			for _, k := range convert.Kinds {
				pill := ui.Row(c.Key(k)).Gap(6).Padding(4, 10).Radius(999).Background(tintOf(k, a.darkNow))
				pill.Children(func() {
					ui.Icon(c, kindIcons[k]).FontSize(12).TextColor(hue(k, a.darkNow))
					ui.Text(c, k.String()).FontSize(12.5).FontWeight(600).TextColor(p.ink)
				})
			}
		})
		reset := stickerButton(c, p, "reset", icRetry, "恢复默认设置", false)
		reset.Margin(20, 0, 0, 0)
		if reset.Clicked() {
			a.confirmReset = true
		}
	})
}

func (a *formigo) resetQuestion(c *ui.Context) {
	if ui.AlertDialog(c, &a.confirmReset, "恢复默认设置？", "外观、转换和输出的设置都会回到最初的样子。", "取消", "恢复默认") == 1 {
		a.reset()
	}
}

func version() string {
	if v := mygo.App.Version(); v != "" {
		return "版本 " + v
	}
	return "开发版"
}

// checkOn is the check's color on a swatch: white, or graphite on a light one.
func checkOn(col ui.Color) ui.Color {
	white, graphite := ui.RGB(255, 255, 255), ui.Hex("#101113")
	if contrast(col, graphite) > contrast(col, white) {
		return graphite
	}
	return white
}
