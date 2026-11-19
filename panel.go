package main

import (
	"fmt"
	"path/filepath"
	"time"

	"formigo/internal/convert"

	"github.com/egoist/mygo/ui"
)

func (a *formigo) panel(c *ui.Context, p palette, compact bool) {
	kinds := a.kinds()
	a.allPicker(c, p, "panel")
	for _, k := range kinds {
		ui.Column(c.Key(k)).Gap(10).Transition(ui.ElementTransition{
			Animation: &settle, Enter: &ui.Motion{Y: 10, Blur: 6}, Exit: &ui.Motion{Collapse: true, Blur: 6},
		}).Children(func() {
			a.formats(c, p, k)
		})
	}
	a.settings(c, p, kinds)
	a.destination(c, p)
}

// formats is a kind's formats as tiles under its name, one chosen.
func (a *formigo) formats(c *ui.Context, p palette, k convert.Kind) {
	ui.Row(c).Gap(8).Children(func() {
		a.kindBadge(c, p, k, 22)
		ui.Text(c, k.String()+"转换为").Font(rounded()).FontSize(14).FontWeight(800).TextColor(p.ink)
		ui.Spacer(c)
		ui.Text(c, fmt.Sprintf("%d 个", a.count(k))).FontSize(11.5).TextColor(p.faint).FontFeatures("tnum").
			ContentTransition(ui.ContentTransition{Numeric: true})
	})
	a.formatPicker(c, p, k, "panel")
}

// kindBadge is a kind's icon on a tile of its hue.
func (a *formigo) kindBadge(c *ui.Context, p palette, k convert.Kind, size float32) {
	dark := a.darkNow
	badge := ui.Box(c).Size(size, size).Radius(size * 0.32).Center().Background(tintOf(k, dark))
	sticker(badge, p, 1.5, 0)
	badge.Children(func() {
		ui.Icon(c, kindIcons[k]).FontSize(size * 0.55).TextColor(hue(k, dark).MixOklab(p.outline, 0.25))
	})
}

// formatPicker is a kind's formats as tiles in a grid, the chosen one's
// highlight sliding from tile to tile.
func (a *formigo) formatPicker(c *ui.Context, p palette, k convert.Kind, where string) {
	chosen := a.kindTarget(k)
	before := chosen
	targets := a.kindTargets(k)
	if len(targets) == 0 {
		return
	}
	cols := 4
	if len(targets) < 4 {
		cols = len(targets)
	}
	tiles := func() {
		for _, f := range targets {
			a.formatTile(c, p, k, f, &chosen, where)
		}
	}
	ui.RadioGroup(c.Key("group"), tiles).Grid().Columns(cols).Gap(8).Label(k.String() + "的目标格式")
	if chosen != before {
		a.targets[k] = chosen
		// Files converted already convert again to the new choice; those
		// given a format of their own keep it.
		a.retarget(func(it *item) bool { return it.kind == k && it.target == "" })
	}
}

// choices says which options the formats the files go to have.
type choices struct{ lossy, webp, resizable, sound, indent bool }

func (o choices) any() bool { return o.lossy || o.resizable || o.sound || o.indent }

func (a *formigo) choices(kinds ...convert.Kind) choices {
	var o choices
	lossyN := 0
	for _, k := range kinds {
		for _, id := range a.targetsOf(k) {
			t, _ := convert.Lookup(id)
			switch {
			case t.Kind == convert.Video:
				o.lossy, o.resizable = true, true
				lossyN++
			case t.Kind == convert.Image || id == "pdf" && k == convert.Image:
				switch id {
				case "jpg", "webp", "pdf", "avif", "jxl":
					o.lossy = true
					lossyN++
					o.webp = id == "webp"
				}
				if id != "ico" && id != "icns" {
					o.resizable = true
				}
			case t.Kind == convert.Audio:
				o.sound = true
			case id == "json" || id == "yaml" || id == "xml":
				o.indent = true
			}
		}
	}
	// Quality's 100 reads lossless only where WebP is all it shapes.
	o.webp = o.webp && lossyN == 1
	return o
}

func (a *formigo) formatTile(c *ui.Context, p palette, k convert.Kind, f convert.Format, chosen *string, where string) {
	on := *chosen == f.ID
	dark := a.darkNow
	height := float32(56)
	tile := ui.RadioBase(c.Key(f.ID), chosen, f.ID).Height(height).Radius(13).Column().Center().Gap(2).
		Label(f.Label + " " + f.Note).Cursor(ui.CursorPointer)
	hot, down := hovers(tile), tile.Pressed()
	edge := p.line
	if hot && !on {
		edge = p.lineStrong
	}
	tile.Background(p.surface).Border(1.5, tile.AnimateColor("edge", edge, tint))
	pose := ui.Pose{}
	switch {
	case down:
		pose.Y, pose.Scale = 2, 0.97
	case hot && !on:
		pose.Y = -2
	}
	tile.AnimatePose(pose, snap)
	tile.Children(func() {
		if on {
			// The chosen tile is a sticker, standing off the page; its
			// highlight flies from the tile chosen before.
			pick := ui.Box(c.Key("pick")).Absolute().Fill().Radius(13).PassThrough().ZIndex(-1).
				Background(tintOf(k, dark)).
				Transition(ui.ElementTransition{Animation: &snap, Match: pickMatch{k, where}})
			sticker(pick, p, 2, 3)
		}
		ink := p.ink
		if on {
			ink = hue(k, dark).MixOklab(p.outline, 0.3)
		}
		ui.Text(c, f.Label).Font(rounded()).FontSize(14).FontWeight(800).LetterSpacing(0.2).
			TextColor(ink).SingleLine()
		ui.Text(c, f.Note).FontSize(10).TextColor(p.muted).SingleLine()
	})
}

// settings are the options of the formats chosen, each shown only where it
// matters, folding in and out as the choice changes.
func (a *formigo) settings(c *ui.Context, p palette, kinds []convert.Kind) {
	o := a.choices(kinds...)
	lossy, resizable, sound, indent := o.lossy, o.resizable, o.sound, o.indent
	if !o.any() {
		return
	}
	fold := ui.ElementTransition{
		Animation: &settle, ExitDuration: 180 * time.Millisecond,
		Enter: &ui.Motion{Collapse: true, Blur: 4}, Exit: &ui.Motion{Collapse: true, Blur: 4},
	}
	ui.Column(c.Key("options")).Gap(12).Transition(ui.ElementTransition{Animation: &settle, Enter: &ui.Motion{Y: 8, Blur: 6}, Exit: &ui.Motion{Collapse: true}}).Children(func() {
		sectionTitle(c, p, "选项")
		card := ui.Column(c.Key("card")).Radius(16).Background(p.surface).Border(1.5, p.line).Padding(4, 14).
			Dividers(1, p.line).Gap(1)
		card.Children(func() {
			if lossy {
				setting(c, "quality", fold, func() {
					ui.Row(c).Gap(10).Children(func() {
						ui.Text(c, "画质").FontSize(12.5).TextColor(p.ink).Width(52)
						ui.Slider(c, &a.quality, 10, 100).Label("画质").Grow(1)
						label := fmt.Sprintf("%.0f", a.quality)
						if a.quality >= 100 && o.webp {
							label = "无损"
						}
						ui.Text(c, label).FontSize(12).FontFeatures("tnum").TextColor(p.muted).Width(30).TextAlign(ui.End).
							ContentTransition(ui.ContentTransition{Numeric: true})
					})
				})
			}
			if resizable {
				setting(c, "scale", fold, func() {
					ui.Row(c).Gap(10).Children(func() {
						ui.Text(c, "尺寸").FontSize(12.5).TextColor(p.ink).Width(52)
						ui.Spacer(c)
						ui.Select(c, &a.scale, scales).Label("尺寸").Width(132)
					})
				})
			}
			if sound {
				setting(c, "depth", fold, func() {
					ui.Row(c).Gap(10).Children(func() {
						ui.Text(c, "位深").FontSize(12.5).TextColor(p.ink).Width(52)
						ui.Spacer(c)
						ui.Segmented(c, &a.depth, "16 位", "24 位").Label("位深")
					})
				})
				setting(c, "mono", fold, func() {
					ui.Row(c).Gap(10).Children(func() {
						ui.Text(c, "合并为单声道").FontSize(12.5).TextColor(p.ink).Grow(1)
						ui.Switch(c, &a.mono).Label("合并为单声道")
					})
				})
			}
			if indent {
				setting(c, "indent", fold, func() {
					ui.Row(c).Gap(10).Children(func() {
						ui.Text(c, "缩进").FontSize(12.5).TextColor(p.ink).Width(52)
						ui.Spacer(c)
						ui.Segmented(c, &a.indent, "2 格", "4 格", "紧凑").Label("缩进")
					})
				})
			}
		})
	})
}

func setting(c *ui.Context, key string, tr ui.ElementTransition, fn func()) {
	ui.Column(c.Key(key)).Padding(10, 0).Transition(tr).Children(fn)
}

func sectionTitle(c *ui.Context, p palette, s string) {
	ui.Text(c, s).Font(rounded()).FontSize(14).FontWeight(800).TextColor(p.ink)
}

// destination is where converted files go.
func (a *formigo) destination(c *ui.Context, p palette) {
	ui.Column(c.Key("dest")).Gap(10).Children(func() {
		sectionTitle(c, p, "存放到")
		where := a.where
		ui.Segmented(c, &where, "原文件夹", "其他位置…").Label("存放到").AlignSelf(ui.Start)
		if where != a.where {
			a.where = where
			if where == 1 && a.outDir == "" {
				a.chooseFolder()
			}
		}
		if a.where == 1 && a.outDir != "" {
			path := ui.ButtonBase(c.Key("path")).Padding(9, 12).Radius(12).Gap(8).Background(p.surface).Border(1.5, p.line).
				Cursor(ui.CursorPointer).Label("更改文件夹").Tooltip(a.outDir).
				Transition(ui.ElementTransition{Animation: &settle, Enter: &ui.Motion{Collapse: true, Blur: 4}, Exit: &ui.Motion{Collapse: true, Blur: 4}})
			path.Children(func() {
				ui.Icon(c, icFolder).FontSize(14).TextColor(p.muted)
				ui.Text(c, filepath.Base(a.outDir)).FontSize(12).TextColor(p.ink).MaxLines(1).Grow(1).Shrink(1).Basis(0)
				ui.Text(c, "更改").FontSize(11.5).TextColor(p.accent)
			})
			if path.Clicked() {
				a.chooseFolder()
			}
		}
	})
}

// actionBar holds the button that starts the run, and its progress.
func (a *formigo) actionBar(c *ui.Context, p palette, compact bool) {
	waitingN := len(a.pending())
	total, finished, failedN := 0, 0, 0
	var progress float64
	for _, it := range a.items {
		total++
		switch it.status {
		case done:
			finished++
			progress++
		case failed:
			failedN++
		case working:
			progress += it.progress
		}
	}
	bar := ui.Column(c.Key("action")).Padding(14, 18, 18, 18).Gap(12).BorderWidth(1.5, 0, 0, 0).BorderColor(p.line)
	if compact {
		bar.Padding(12, 16, 16, 16).Background(p.panel)
	}
	bar.Children(func() {
		if a.running {
			ui.Row(c.Key("prog")).Gap(10).Transition(ui.ElementTransition{Animation: &settle, Enter: &ui.Motion{Collapse: true, Blur: 4}, Exit: &ui.Motion{Collapse: true, Blur: 4}}).Children(func() {
				ui.Progress(c, progress/float64(max(1, total))).Grow(1).Label("进度")
				ui.Textf(c, "%d/%d", finished, total).FontSize(11.5).TextColor(p.muted).FontFeatures("tnum").
					ContentTransition(ui.ContentTransition{Numeric: true})
			})
		}
		ui.Row(c).Gap(10).Children(func() {
			if a.prefs.Mascot {
				a.companion(c, p)
			}
			label := fmt.Sprintf("转换 %d 个文件", waitingN)
			var ic *ui.SVG = icConvert
			switch {
			case a.running:
				label = "停止"
				ic = icClose
			case waitingN == 0 && finished > 0:
				label = "全部完成 · 再转一次"
				ic = icRetry
			case failedN > 0 && waitingN == failedN:
				label = fmt.Sprintf("重试 %d 个文件", failedN)
				ic = icRetry
			}
			var go_ ui.Element
			if a.running {
				go_ = ui.Button(c.Key("go"), "").Background(p.surface)
			} else {
				go_ = ui.PrimaryButton(c.Key("go"), "")
			}
			go_.Grow(1).Height(46).Radius(14).Gap(8).Label(label)
			pressable(go_, p, 4)
			ink := p.accentInk
			if a.running {
				ink = p.ink
			}
			// A sheen crosses the button as it becomes ready.
			sheen := go_.Keyframes("sheen", []ui.Keyframe{{Value: 0}, {At: 900 * time.Millisecond, Value: 1, Ease: ui.CubicBezier(0.4, 0, 0.2, 1)}},
				ui.KeyframeOptions{Run: a.ready, Delay: 250 * time.Millisecond})
			if a.ready > 0 && sheen > 0 && sheen < 1 && !a.running && a.ambient(c) {
				go_.DrawOver(func(pt *ui.Painter, r ui.Rect) {
					w := r.W * 0.35
					x := r.X - w + (r.W+w)*sheen
					pt.Clip(r, 14, func() {
						pt.FillGradient(ui.Rect{X: x, Y: r.Y, W: w, H: r.H}, ui.LinearGradient{
							Angle: 90,
							Stops: []ui.GradientStop{{Color: ui.RGBA(255, 255, 255, 0), At: 0}, {Color: ui.RGBA(255, 255, 255, 0.28), At: 0.5}, {Color: ui.RGBA(255, 255, 255, 0), At: 1}},
						}, 0)
					})
				})
			}
			go_.Children(func() {
				ui.Icon(c, ic).FontSize(17).TextColor(ink).ContentTransition(ui.ContentTransition{})
				ui.Text(c, label).Font(rounded()).FontSize(15).FontWeight(800).TextColor(ink).SingleLine().
					ContentTransition(ui.ContentTransition{Morph: true})
			})
			if !a.running {
				go_.Tooltip("⌘↩")
			}
			if go_.Clicked() {
				if a.running {
					a.stop()
				} else {
					a.start()
				}
			}
			if !a.running && finished > 0 {
				var last *item
				for _, it := range a.items {
					if it.status == done {
						last = it
					}
				}
				show := ui.Button(c.Key("reveal"), "").Size(46, 46).Radius(14).Background(p.surface).Label(revealLabel).Tooltip(revealLabel).
					Transition(ui.ElementTransition{Animation: &pop, Enter: &ui.Motion{Scale: 0.6}, Exit: &ui.Motion{Scale: 0.6}})
				pressable(show, p, 4)
				show.Children(func() { ui.Icon(c, icReveal).FontSize(17).TextColor(p.ink) })
				if show.Clicked() && last != nil {
					a.reveal(last)
				}
			}
		})
	})
}

// companion is the gopher beside the button: it sways while it waits,
// quicker while files convert, and jumps when a run converted something or
// it is tapped, in frames that only paint.
func (a *formigo) companion(c *ui.Context, p palette) {
	g := ui.Image(c.Key("companion"), mascot).Size(46, 46).Fit(ui.Contain).AccessHidden(true).Cursor(ui.CursorPointer)
	if g.Clicked() {
		a.companionAt = c.Now()
	}
	half, amp := 1300*time.Millisecond, float32(2.5)
	if a.running {
		half, amp = 260*time.Millisecond, 3
	}
	hop := a.companionAt
	if a.cheerAt.After(hop) {
		hop = a.cheerAt
	}
	pose, motion := gopherPose(c, ui.Pose{}, hop, a.ambient(c), amp, half)
	g.AnimatePose(pose, motion).TransformOrigin(0.5, 1)
}
