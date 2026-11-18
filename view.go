package main

import (
	"time"

	"github.com/egoist/mygo/ui"
)

// compactWidth is the width below which the panel goes under the queue.
const compactWidth = 760

func (a *formigo) theme(c *ui.Context, p palette) {
	// The default theme, which follows the desktop's appearance, contrast
	// and text size, or the appearance chosen, in Formigo's colors.
	base := c.Theme()
	switch a.prefs.Appearance {
	case "light":
		base = ui.LightTheme().WithPreferences(c.Preferences())
	case "dark":
		base = ui.DarkTheme().WithPreferences(c.Preferences())
	}
	t := base.WithAccent(p.accent)
	t.Radius = 12
	t.Background, t.Surface, t.Border = p.bg, p.surface, p.lineStrong
	t.Text, t.TextMuted = p.ink, p.muted
	t.Danger, t.Success = p.bad, p.ok
	t.InactiveAccent = p.accent
	t.AccentText = p.accentInk
	t.AccentHover = p.accent.MixOklab(p.accentPressed, 0.5)
	t.AccentPressed = p.accentPressed
	t.Focus = p.accent.Alpha(0.5)
	t.FocusRingWidth, t.FocusRingOffset = 2.5, 2
	c.SetTheme(t)
}

func (a *formigo) view(c *ui.Context) {
	a.builds++
	p := a.palette(c)
	a.theme(c, p)
	if a.prefs.Motion == "off" {
		c.DisableAnimations()
	}
	defer a.persist()
	w, _ := c.Size()
	compact := w < compactWidth

	if c.Shortcut(ui.Cmd, ui.KeyO) {
		a.chooseFiles()
	}
	if c.Shortcut(ui.Cmd, ui.KeyComma) {
		a.openSettings()
	}
	if c.Shortcut(ui.Cmd, ui.KeyEnter) && !a.running && len(a.items) > 0 {
		a.start()
	}

	root := ui.Column(c).Fill().Background(p.bg)
	if files := root.DroppedFiles(); files != nil {
		if added, skipped := a.add(files); added == 0 && skipped > 0 {
			c.Toast("这些文件的格式暂不支持")
		} else if skipped > 0 {
			c.Toast("已跳过不支持或重复的文件")
		}
	}
	over := root.FileDragOver()

	root.Children(func() {
		a.titleBar(c, p, compact)
		empty := len(a.items) == 0
		ui.Box(c.Key("stage")).Grow(1).Children(func() {
			if empty {
				ui.Column(c.Key("hero")).Absolute().Fill().Transition(crossfade).Children(func() {
					a.hero(c, p, over, compact)
				})
				return
			}
			body := ui.Row(c.Key("work")).Absolute().Fill().AlignItems(ui.Stretch).Transition(crossfade)
			if compact {
				body = ui.Column(c.Key("work-compact")).Absolute().Fill().Transition(crossfade)
			}
			body.Children(func() {
				if compact {
					ui.Scroll(c.Key("scroll")).Grow(1).Children(func() {
						ui.Column(c).Padding(4, 16, 16, 16).Gap(16).Children(func() {
							a.queue(c, p, compact)
							a.panel(c, p, compact)
						})
					})
					a.actionBar(c, p, compact)
					return
				}
				ui.Column(c.Key("queue")).Grow(1).Basis(0).Transition(ui.ElementTransition{Size: true, Animation: &settle}).Children(func() {
					ui.Scroll(c.Key("scroll")).Grow(1).Children(func() {
						ui.Column(c).Padding(4, 20, 24, 24).Children(func() { a.queue(c, p, compact) })
					})
				})
				// The inspector floats beside the queue, a card of its own.
				ui.Column(c.Key("panel")).Width(356).Margin(0, 14, 14, 0).Radius(22).Clip().
					Background(p.panel).Border(1.5, p.lineStrong).
					Transition(ui.ElementTransition{Animation: &settle, Enter: &ui.Motion{X: 60, Opacity: 0}, Exit: &ui.Motion{X: 60}}).
					Children(func() {
						ui.Scroll(c.Key("panel-scroll")).Grow(1).Children(func() {
							ui.Column(c).Padding(18, 18, 18, 18).Gap(22).Children(func() { a.panel(c, p, compact) })
						})
						a.actionBar(c, p, compact)
					})
			})
		})
		if over && !empty {
			a.dropVeil(c, p)
		}
		a.encore(c, p, compact)
	})
}

// titleBar is drawn by the view under the window's controls, and drags the
// window. The gopher of the empty window flies into it once there are
// files, and hops when a run is done.
func (a *formigo) titleBar(c *ui.Context, p palette, compact bool) {
	bar := c.TitleBar()
	h := max(bar.Height, 52)
	ui.Row(c.Key("titlebar")).Height(h).Padding(0, bar.Right+16, 0, bar.Left+18).Gap(10).DragWindow().Children(func() {
		// The gopher and the name are one button, which only an easter egg
		// gives a purpose: each click squishes them, the fifth in a row
		// plays the launch animation again.
		brand := ui.ButtonBase(c.Key("brand")).Gap(10).Padding(2, 4).Radius(10).Label("Formigo").
			FocusRing(false).Transition(ui.ElementTransition{Animation: &settle, Position: true})
		squish := brand.Keyframes("squish", []ui.Keyframe{
			{Value: 0}, {At: 70 * time.Millisecond, Value: 1, Ease: ui.EaseOut}, {At: 260 * time.Millisecond, Value: 0, Ease: ui.EaseOut},
		}, ui.KeyframeOptions{Run: uint64(a.taps) + 10*a.intro})
		if brand.Clicked() {
			a.tapped(c.Now())
		}
		brand.Children(func() {
			if len(a.items) > 0 && a.encoreAt.IsZero() && a.prefs.Mascot {
				mark := ui.Image(c.Key("mark"), mascot).Size(34, 34).Fit(ui.Contain).
					Transition(ui.ElementTransition{Animation: &settle, Match: mascotMatch{}, Fly: true})
				hopY := mark.Keyframes("hop", []ui.Keyframe{
					{Value: 0}, {At: 180 * time.Millisecond, Value: -9, Ease: ui.EaseOut},
					{At: 420 * time.Millisecond, Value: 0, Ease: ui.EaseIn}, {At: 520 * time.Millisecond, Value: -2, Ease: ui.EaseOut},
					{At: 620 * time.Millisecond, Value: 0, Ease: ui.EaseIn},
				}, ui.KeyframeOptions{Run: a.cheers})
				mark.Translate(0, hopY).Scale(1+0.12*squish, 1-0.12*squish).TransformOrigin(0.5, 1)
			}
			ui.Text(c, "Formigo").Font(rounded()).FontSize(18).FontWeight(800).LetterSpacing(-0.3).TextColor(p.ink).
				Scale(1+0.04*squish, 1-0.06*squish).TransformOrigin(0, 1).
				Transition(ui.ElementTransition{Animation: &settle, Position: true})
		})
		ui.Spacer(c)
		if len(a.items) > 0 {
			ui.Row(c.Key("bar-actions")).Gap(6).Transition(ui.ElementTransition{Animation: &snap, Enter: &ui.Motion{Y: -6, Blur: 4}, Exit: &ui.Motion{Y: -6, Blur: 4}}).Children(func() {
				clr := ghostButton(c, p, icSweep, "清空")
				clr.Disabled(a.running).Tooltip("清空列表")
				if clr.Clicked() {
					a.clear()
				}
				add := ghostButton(c, p, icPlus, "添加")
				add.Tooltip("添加文件  ⌘O")
				if add.Clicked() {
					a.chooseFiles()
				}
			})
		}
		gear := ui.ButtonBase(c.Key("gear")).Size(32, 32).Radius(10).Center().Label("设置").Tooltip("设置  ⌘,").Cursor(ui.CursorPointer)
		gear.Background(gear.AnimateColor("bg", map[bool]ui.Color{true: p.skyTint, false: ui.Transparent}[hovers(gear)], tint))
		spin := float32(0)
		if hovers(gear) {
			spin = 60
		}
		gear.Children(func() {
			ui.Icon(c, icGear).FontSize(17).TextColor(p.ink).Rotate(gear.AnimateTo("spin", spin, settle))
		})
		softPress(gear, false)
		if gear.Clicked() {
			a.openSettings()
		}
	})
}

// ghostButton is a quiet button of the title bar: an icon, and a label
// unless space is short.
func ghostButton(c *ui.Context, p palette, ic *ui.SVG, label string) ui.Element {
	b := ui.ButtonBase(c.Key(label)).Padding(6, 11).Radius(10).Gap(6).Label(label).Cursor(ui.CursorPointer)
	bg := ui.Transparent
	switch {
	case b.Pressed():
		bg = p.skyTint.MixOklab(p.sky, 0.2)
	case hovers(b):
		bg = p.skyTint
	}
	b.Background(b.AnimateColor("bg", bg, tint))
	b.AnimatePose(ui.Pose{Scale: map[bool]float32{true: 0.96, false: 1}[b.Pressed()]}, snap)
	b.Children(func() {
		ui.Icon(c, ic).FontSize(15).TextColor(p.ink)
		ui.Text(c, label).FontSize(12.5).FontWeight(600).TextColor(p.ink).SingleLine()
	})
	return b
}

// dropVeil covers the window while files are dragged over it.
func (a *formigo) dropVeil(c *ui.Context, p palette) {
	veil := ui.Box(c.Key("veil")).Absolute().Fill().Padding(10).PassThrough().ZIndex(10).
		Transition(ui.ElementTransition{Animation: &snap, ExitDuration: 160 * time.Millisecond, Enter: &ui.Motion{}, Exit: &ui.Motion{}})
	veil.Children(func() {
		ui.Column(c).Grow(1).Radius(24).Center().Gap(10).
			Background(p.bg.Alpha(0.86)).Border(3, p.accent).BorderStyle(ui.BorderDashed).Children(func() {
			// The gopher jumps up to take them.
			if a.prefs.Mascot {
				g := ui.Image(c.Key("veil-gopher"), mascot).Size(150, 150).Fit(ui.Contain)
				if a.ambient(c) {
					jump := g.Loop("jump", 900*time.Millisecond, ui.Bounce(ui.EaseOut))
					g.Translate(0, -14*jump).Rotate(-3 + 6*jump)
				}
			} else {
				ui.Icon(c, icPlus).FontSize(56).TextColor(p.accent)
			}
			ui.Text(c, "松手，交给我！").Font(rounded()).FontSize(20).FontWeight(800).TextColor(p.ink)
		})
	})
}

// encoreLength is how long the launch animation's replay over the queue
// stays before the gopher flies home.
const encoreLength = 3200 * time.Millisecond

// encore plays the launch animation again over the queue, the easter egg
// of the title bar's gopher: the gopher flies out of the title bar into the
// middle, the orbit opens round it again, and then it flies back. A click
// ends it early.
func (a *formigo) encore(c *ui.Context, p palette, compact bool) {
	if a.encoreAt.IsZero() {
		return
	}
	left := encoreLength - c.Now().Sub(a.encoreAt)
	if left <= 0 {
		a.encoreAt = time.Time{}
		return
	}
	c.After(left)
	cover := ui.Column(c.Key("encore")).Absolute().Fill().ZIndex(20).Background(p.bg.Alpha(0.96)).
		Transition(ui.ElementTransition{Animation: &settle, ExitDuration: 260 * time.Millisecond, Enter: &ui.Motion{}, Exit: &ui.Motion{}})
	if cover.Clicked() {
		a.encoreAt = time.Time{}
	}
	cover.Children(func() { a.hero(c, p, false, compact) })
}
