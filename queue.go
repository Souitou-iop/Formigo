package main

import (
	"fmt"
	"math"
	"slices"
	"strings"
	"time"

	"formigo/internal/convert"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func (a *formigo) queue(c *ui.Context, p palette, compact bool) {
	ui.Row(c.Key("queue-head")).Gap(8).Padding(6, 4, 10, 4).Children(func() {
		ui.Text(c, "文件").Font(rounded()).FontSize(15).FontWeight(800).TextColor(p.ink)
		ui.Text(c, fmt.Sprint(len(a.items))).FontSize(13).TextColor(p.faint).FontFeatures("tnum").
			ContentTransition(ui.ContentTransition{Numeric: true})
		ui.Spacer(c)
		total := int64(0)
		for _, it := range a.items {
			total += it.size
		}
		ui.Text(c, bytesText(total)).FontSize(12).TextColor(p.faint).FontFeatures("tnum").
			ContentTransition(ui.ContentTransition{Numeric: true})
	})
	gap := float32(8)
	// The files by kind, as they were recognized, each kind under what
	// it becomes.
	list := ui.Column(c.Key("rows")).Gap(18).Transition(ui.ElementTransition{Height: true, Animation: &settle})
	list.Children(func() {
		for _, k := range a.kinds() {
			ui.Column(c.Key(k)).Gap(gap).Transition(ui.ElementTransition{
				Animation: &settle, Enter: &ui.Motion{Y: 10, Blur: 6}, Exit: &ui.Motion{Collapse: true, Blur: 6},
			}).Children(func() {
				a.groupHead(c, p, k, compact)
				for _, it := range a.items {
					if it.kind == k {
						a.row(c, p, it, compact)
					}
				}
			})
		}
	})
	{
		// More files go anywhere on the window, or come from this button.
		more := ui.ButtonBase(c.Key("more")).Height(50).Radius(16).Center().Gap(8).Margin(12, 0, 0, 0).
			Border(2, p.lineStrong).BorderStyle(ui.BorderDashed).Cursor(ui.CursorPointer).Label("添加更多文件")
		hot := hovers(more)
		more.Background(more.AnimateColor("bg", map[bool]ui.Color{true: p.skyTint, false: ui.Transparent}[hot], tint))
		more.BorderColor(more.AnimateColor("line", map[bool]ui.Color{true: p.accent, false: p.lineStrong}[hot], tint))
		ink := map[bool]ui.Color{true: p.accent, false: p.muted}[hot]
		more.Children(func() {
			ui.Icon(c, icPlus).FontSize(15).TextColor(ink)
			ui.Text(c, "拖入或点击添加更多").FontSize(13).FontWeight(600).TextColor(ink)
		})
		softPress(more, true)
		if more.Clicked() {
			a.chooseFiles()
		}
	}
}

// groupHead is a kind's head on the queue: what was recognized, and, in
// narrow windows whose panel is far below, what it becomes, in a menu.
func (a *formigo) groupHead(c *ui.Context, p palette, k convert.Kind, compact bool) {
	ui.Row(c.Key("head")).Gap(10).Padding(2, 2, 0, 2).Children(func() {
		a.kindBadge(c, p, k, 28)
		ui.Column(c).Grow(1).Shrink(1).Gap(1).Children(func() {
			ui.Text(c, k.String()).Font(rounded()).FontSize(15).FontWeight(800).TextColor(p.ink)
			ui.Textf(c, "%d 个 · %s", a.count(k), a.exts(k)).FontSize(11.5).TextColor(p.faint).
				FontFeatures("tnum").MaxLines(1).ContentTransition(ui.ContentTransition{Numeric: true})
		})
		if compact {
			a.kindMenu(c, p, k)
		}
	})
}

// kindMenu is a kind's format, a chip whose menu chooses another.
func (a *formigo) kindMenu(c *ui.Context, p palette, k convert.Kind) {
	dark := a.darkNow
	h := hue(k, dark)
	cur, _ := convert.Lookup(a.kindTarget(k))
	m := ui.ButtonBase(c.Key("kind-target")).Height(30).Padding(0, 10, 0, 12).Radius(15).Gap(6).
		Background(tintOf(k, dark)).Border(1.5, h).Label(k.String() + "转换为").Description(cur.Label).
		Cursor(ui.CursorPointer).Disabled(a.running)
	softPress(m, false)
	ink := h.MixOklab(p.outline, 0.25)
	m.Children(func() {
		ui.Text(c, "转换为").FontSize(11.5).TextColor(ink)
		ui.Text(c, cur.Label).FontSize(12).FontWeight(800).LetterSpacing(0.3).TextColor(ink).SingleLine().
			ContentTransition(ui.ContentTransition{Morph: true})
		ui.Icon(c, icUpDown).FontSize(11).TextColor(ink)
	})
	m.PopoverMenuButton(func(menu *ui.Menu) {
		for _, f := range a.kindTargets(k) {
			if menu.Item(f.Label+" · "+f.Note).Radio(f.ID == cur.ID).Chosen() && f.ID != cur.ID {
				a.targets[k] = f.ID
				a.retarget(func(it *item) bool { return it.kind == k && it.target == "" })
			}
		}
	}, menuLook(p))
}

// row is a file's row.
func (a *formigo) row(c *ui.Context, p palette, it *item, compact bool) {
	a.card(c.Key(it.id), p, it, compact).Transition(rowMotion(it.batch))
}

// card is what a row shows: the file, where it goes, and how it is going.
func (a *formigo) card(c *ui.Context, p palette, it *item, compact bool) ui.Element {
	dark := a.darkNow
	k := hue(it.kind, dark)
	height, tileW, tileH := float32(68), float32(42), float32(44)
	row := ui.Row(c).Height(height).Padding(0, 12, 0, 12).Gap(12).Radius(16)

	// Finishing washes the row in green for a moment; failing shakes it.
	flash := row.Keyframes("flash", []ui.Keyframe{
		{Value: 0}, {At: 90 * time.Millisecond, Value: 1, Ease: ui.EaseOut}, {At: 1100 * time.Millisecond, Value: 0, Ease: ui.EaseInOut},
	}, ui.KeyframeOptions{Run: it.doneRun, NonMotion: true})
	if it.doneRun == 0 {
		flash = 0
	}
	shake := row.Keyframes("shake", []ui.Keyframe{
		{Value: 0}, {At: 60 * time.Millisecond, Value: -7}, {At: 140 * time.Millisecond, Value: 6},
		{At: 220 * time.Millisecond, Value: -4}, {At: 300 * time.Millisecond, Value: 2}, {At: 380 * time.Millisecond, Value: 0, Ease: ui.EaseOut},
	}, ui.KeyframeOptions{Run: it.failRun})
	if it.failRun > 0 {
		row.Translate(shake, 0)
	}

	// At rest a quiet card; under the pointer a sticker, its outline
	// darkening as it lifts off its shadow.
	hot := hovers(row)
	bg := row.AnimateColor("bg", p.surface, tint).MixOklab(p.ok, 0.18*flash)
	edge := p.line
	switch {
	case it.status == failed:
		edge = p.bad
	case hot:
		edge = p.outline
	}
	up := float32(0)
	if hot {
		up = 1
	}
	lift := row.AnimateTo("lift", up, snap)
	row.Background(bg).Border(1.5, row.AnimateColor("edge", edge, tint)).Translate(0, -2*lift)
	if lift > 0.01 {
		row.Shadow(0, 3*lift, 0, 0, p.outline.Alpha(lift))
	}
	if it.status == done {
		row.Cursor(ui.CursorPointer).Tooltip(revealLabel)
		if row.DoubleClicked() {
			a.reveal(it)
		}
	}
	{
		// A right click offers the file: where it was written, and the
		// file itself.
		row.ContextMenu(func(m *ui.Menu) {
			done := it.status == done && it.out != ""
			if m.Item(revealLabel).Disabled(!done).Chosen() {
				a.reveal(it)
			}
			if m.Item("打开转换后的文件").Disabled(!done).Chosen() {
				go mygo.Shell.OpenPath(it.out)
			}
			m.Separator()
			if m.Item("显示原文件").Chosen() {
				mygo.Shell.ShowItemInFolder(it.path)
			}
			if m.Item("移除").Disabled(it.status == working).Chosen() {
				id := it.id
				a.update(func() { a.remove(id) })
			}
		})
	}

	row.Children(func() {
		// The file's kind: a tile in its hue, with its extension.
		tile := ui.Column(c.Key("tile")).Size(tileW, tileH).Radius(11).Center().Gap(1).
			Background(tintOf(it.kind, dark))
		sticker(tile, p, 1.5, 2)
		ink := k.MixOklab(p.outline, 0.25)
		tile.Children(func() {
			ui.Icon(c, kindIcons[it.kind]).FontSize(15).TextColor(ink)
			ui.Text(c, strings.ToUpper(convert.Ext(it.path))).FontSize(8.5).FontWeight(800).LetterSpacing(0.3).TextColor(ink).SingleLine()
		})

		ui.Column(c.Key("words")).Grow(1).Shrink(1).Basis(0).Gap(3).Children(func() {
			ui.Text(c, it.name).FontSize(13.5).FontWeight(650).TextColor(p.ink).MaxLines(1)
			ui.Row(c).Gap(6).Children(func() {
				line := bytesText(it.size)
				// Narrow rows keep the size and what it became, once done.
				if it.meta != "" && !(compact && it.status == done) {
					line += "  ·  " + it.meta
				}
				switch {
				case it.status == failed:
					ui.Text(c, it.err).FontSize(11.5).TextColor(p.bad).MaxLines(1).Tooltip(it.err).
						Transition(ui.ElementTransition{Animation: &snap, Enter: &ui.Motion{Y: 4, Blur: 3}})
				case it.status == done && it.already:
					ui.Text(c, line).FontSize(11.5).TextColor(p.muted).MaxLines(1).FontFeatures("tnum")
					target, _ := convert.Lookup(a.targetOf(it))
					ui.Text(c.Key("already"), "已是 "+target.Label+" 格式").FontSize(11.5).TextColor(p.faint).SingleLine().
						Transition(ui.ElementTransition{Animation: &snap, Enter: &ui.Motion{X: -6, Blur: 4}})
				case it.status == done && it.outSize > 0:
					ui.Text(c, line).FontSize(11.5).TextColor(p.muted).MaxLines(1).FontFeatures("tnum")
					a.delta(c, p, it)
				default:
					ui.Text(c, line).FontSize(11.5).TextColor(p.muted).MaxLines(1).FontFeatures("tnum")
				}
			})
		})

		// Where it goes: its kind's format, or one of its own, which the
		// chip's menu chooses.
		ui.Row(c.Key("route")).Gap(6).Children(func() {
			if !compact {
				ui.Icon(c, icArrow).FontSize(13).TextColor(p.faint)
			}
			a.fileTarget(c, p, it)
		})

		// Its state: waiting, a ring filling, a check, or what went wrong.
		ui.Box(c.Key("state")).Size(32, 32).Center().Children(func() {
			a.state(c, p, it, hot)
		})
	})
	return row
}

// fileTarget is a file's format, a chip whose menu gives it one of its own
// or has it follow its kind's again. A format of its own is outlined in
// ink.
func (a *formigo) fileTarget(c *ui.Context, p palette, it *item) {
	dark := a.darkNow
	h := hue(it.kind, dark)
	target, _ := convert.Lookup(a.targetOf(it))
	kindT, _ := convert.Lookup(a.kindTarget(it.kind))
	own := it.target != "" && it.target != kindT.ID && convert.CanConvert(it.from, it.target)
	edge := h
	if own {
		edge = p.outline
	}
	chip := ui.ButtonBase(c.Key("target")).Height(26).Padding(0, 7, 0, 10).Radius(13).Gap(3).
		Background(tintOf(it.kind, dark)).Border(1.5, edge).
		Label(it.name + " 的目标格式").Description(target.Label).Cursor(ui.CursorPointer).Disabled(a.running)
	softPress(chip, false)
	ink := h.MixOklab(p.outline, 0.25)
	chip.Children(func() {
		ui.Text(c, target.Label).FontSize(11).FontWeight(800).LetterSpacing(0.3).TextColor(ink).SingleLine().
			ContentTransition(ui.ContentTransition{Morph: true})
		ui.Icon(c, icUpDown).FontSize(9.5).TextColor(ink)
	})
	chip.PopoverMenuButton(func(m *ui.Menu) {
		if m.Item("跟随"+it.kind.String()+" · "+kindT.Label).Radio(!own).Chosen() && own {
			it.target = ""
			a.retarget(func(x *item) bool { return x == it })
		}
		m.Separator()
		// Its own kind's formats, then each other kind's in a menu of its
		// own: a video's sound, a document's pictures.
		choose := func(f convert.Format) {
			it.target = f.ID
			if f.ID == kindT.ID {
				it.target = ""
			}
			a.retarget(func(x *item) bool { return x == it })
		}
		targets := convert.Targets(it.from)
		for _, f := range targets {
			if f.Kind == it.kind && m.Item(f.Label+" · "+f.Note).Radio(own && f.ID == it.target).Chosen() && f.ID != a.targetOf(it) {
				choose(f)
			}
		}
		for _, k := range convert.Kinds {
			if k == it.kind || !slices.ContainsFunc(targets, func(f convert.Format) bool { return f.Kind == k }) {
				continue
			}
			m.Submenu("转为"+k.String(), func(sub *ui.Menu) {
				for _, f := range targets {
					if f.Kind == k && sub.Item(f.Label+" · "+f.Note).Radio(own && f.ID == it.target).Chosen() && f.ID != a.targetOf(it) {
						choose(f)
					}
				}
			})
		}
	}, menuLook(p))
}

func (a *formigo) state(c *ui.Context, p palette, it *item, hot bool) {
	swap := ui.ElementTransition{
		Animation:    &pop,
		ExitDuration: 160 * time.Millisecond,
		Enter:        &ui.Motion{Scale: 0.4, Rotate: -25},
		Exit:         &ui.Motion{Scale: 1.4},
	}
	switch it.status {
	case working:
		ring := ui.Box(c.Key("ring")).Absolute().Fill().Transition(swap)
		shown := float32(ring.AnimateTo("p", float32(it.progress), settle))
		ring.Draw(func(pt *ui.Painter, r ui.Rect) {
			cx, cy, rad := r.X+r.W/2, r.Y+r.H/2, r.W/2-3
			var track ui.Path
			track.Circle(cx, cy, rad)
			pt.StrokePath(&track, 3, p.line)
			// The arc starts at the top and turns slowly as it fills, so
			// that a long wait still looks alive.
			spin := float32(0)
			if !pt.Preferences().ReduceMotion {
				spin = float32(math.Mod(float64(pt.Now().UnixMilli())/2400, 1))
				pt.AnimationFrame()
			}
			// From the top (a quarter turn before the path's start), the
			// arc may cross the start of the circle: draw it in two parts.
			lo := float32(wrap(float64(spin) + 0.75))
			hi := lo + min(1, max(0.06, shown))
			for _, seg := range [][2]float32{{lo, min(hi, 1)}, {0, hi - 1}} {
				from, to := seg[0], seg[1]
				if to <= from {
					continue
				}
				var arc ui.Path
				arc.Circle(cx, cy, rad).Trim(from, to).Cap(ui.RoundCap)
				pt.StrokePath(&arc, 3, p.accent)
			}
		})
	case done:
		ok := ui.Box(c.Key("ok")).Size(26, 26).Radius(13).Center().Background(p.ok).Transition(swap)
		sticker(ok, p, 1.5, 2)
		ok.Children(func() { ui.Icon(c, icCheck).FontSize(14).TextColor(ui.RGB(255, 255, 255)) })
	case failed:
		bad := ui.ButtonBase(c.Key("bad")).Size(28, 28).Radius(14).Center().Background(p.bad.Alpha(0.16)).Border(1.5, p.bad).
			Transition(swap).Tooltip(it.err + "\n点击重试").Label("重试")
		softPress(bad, false)
		bad.Children(func() {
			ic := icAlert
			if hovers(bad) {
				ic = icRetry
			}
			ui.Icon(c, ic).FontSize(15).TextColor(p.bad).ContentTransition(ui.ContentTransition{})
		})
		if bad.Clicked() && !a.running {
			a.start()
		}
	default:
		if a.running {
			// Waiting its turn: three dots breathing in sequence.
			dots := ui.Row(c.Key("wait")).Gap(3).Transition(swap)
			dots.Children(func() {
				for i := range 3 {
					d := ui.Box(c.Key(i)).Size(5, 5).Radius(2.5).Background(p.sky)
					d.Opacity(0.35 + 0.65*d.Keyframes("b", []ui.Keyframe{{Value: 0}, {At: 300 * time.Millisecond, Value: 1, Ease: ui.EaseInOut}, {At: 900 * time.Millisecond, Value: 0, Ease: ui.EaseInOut}},
						ui.KeyframeOptions{Repeat: ui.RepeatForever, Delay: time.Duration(i) * 150 * time.Millisecond, NonMotion: true}))
				}
			})
			return
		}
		rm := ui.ButtonBase(c.Key("rm")).Size(26, 26).Radius(8).Center().Label("移除").Cursor(ui.CursorPointer)
		show := float32(0)
		if hot || rm.Focused() {
			show = 1
		}
		rm.Opacity(rm.AnimateTo("show", show, ui.Animation{Duration: 140 * time.Millisecond, NonMotion: true}))
		if hovers(rm) {
			rm.Background(p.skyTint)
		}
		softPress(rm, false)
		rm.Children(func() { ui.Icon(c, icClose).FontSize(14).TextColor(p.ink) })
		id := it.id
		rm.OnClick(func() { a.remove(id) }) // once the rows are built

	}
}

// delta says how much smaller, or larger, the file came out.
func (a *formigo) delta(c *ui.Context, p palette, it *item) {
	if it.size == 0 {
		return
	}
	r := float64(it.outSize)/float64(it.size) - 1
	text := fmt.Sprintf("→ %s", bytesText(it.outSize))
	col := p.muted
	// Sizes of text say little: a page's HTML carries its styles.
	compare := it.kind == convert.Image || it.kind == convert.Audio
	switch {
	case !compare:
	case r <= -0.05:
		text += fmt.Sprintf("  −%.0f%%", -r*100)
		col = p.ok
	case r >= 0.05:
		text += fmt.Sprintf("  +%.0f%%", r*100)
	}
	ui.Text(c.Key("delta"), text).FontSize(11.5).FontWeight(560).TextColor(col).SingleLine().FontFeatures("tnum").
		Transition(ui.ElementTransition{Animation: &snap, Enter: &ui.Motion{X: -6, Blur: 4}})
}
