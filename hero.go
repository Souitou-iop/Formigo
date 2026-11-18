package main

import (
	"math"
	"time"

	"formigo/internal/convert"

	"github.com/egoist/mygo/ui"
)

// satellite is a format riding the orbit around the gopher.
type satellite struct {
	label string
	kind  convert.Kind
	phase float64 // where it starts, in turns
}

var satellites = []satellite{
	{"JPG", convert.Image, 0.00},
	{"MP3", convert.Audio, 0.125},
	{"JSON", convert.Data, 0.25},
	{"MD", convert.Doc, 0.375},
	{"PNG", convert.Image, 0.50},
	{"FLAC", convert.Audio, 0.625},
	{"YAML", convert.Data, 0.75},
	{"HTML", convert.Doc, 0.875},
}

// orbitPeriod is how many seconds a turn of the orbit takes.
const orbitPeriod = 38.0

// orbit is the tilted ellipse the documents ride, as the icon's arrow
// circles the gopher: centered a little below the middle, around its belly.
type orbit struct {
	cx, cy, rx, ry, tilt float64
}

func orbitIn(r ui.Rect, pull float32) orbit {
	tight := 1 - 0.14*float64(pull)
	return orbit{
		cx:   float64(r.X + r.W/2),
		cy:   float64(r.Y + r.H*0.6),
		rx:   float64(r.W) * 0.48 * tight,
		ry:   float64(r.W) * 0.14 * tight,
		tilt: -9 * math.Pi / 180,
	}
}

// at is the point of the orbit at angle a, and whether it is on the near
// side, in front of the gopher.
func (o orbit) at(a float64) (x, y float32, front bool) {
	ex, ey := o.rx*math.Cos(a), o.ry*math.Sin(a)
	s, c := math.Sin(o.tilt), math.Cos(o.tilt)
	return float32(o.cx + ex*c - ey*s), float32(o.cy + ex*s + ey*c), math.Sin(a) > 0
}

func (a *formigo) hero(c *ui.Context, p palette, over, compact bool) {
	dark := a.darkNow
	col := ui.Column(c).Fill().Center().Padding(0, 24, 36, 24).Gap(0)
	// The view arrives in order: the gopher pops in, the orbit spreads out
	// around it, then the words rise.
	tl := col.Timeline("arrive", ui.TimelineOptions{Run: 1 + a.intro})
	gopherIn := tl.Tween(ui.Tween{From: 0, To: 1, Duration: 700 * time.Millisecond, Ease: ui.CubicBezier(0.34, 1.56, 0.64, 1)})
	orbitOpen := tl.Tween(ui.Tween{From: 0, To: 1, Duration: 900 * time.Millisecond, Ease: ui.CubicBezier(0.16, 1, 0.3, 1)}, ui.AtTime(120*time.Millisecond))
	titleIn := tl.Tween(ui.Tween{From: 0, To: 1, Duration: 600 * time.Millisecond, Ease: ui.CubicBezier(0.16, 1, 0.3, 1)}, ui.AtTime(340*time.Millisecond))
	actionIn := tl.Tween(ui.Tween{From: 0, To: 1, Duration: 600 * time.Millisecond, Ease: ui.CubicBezier(0.16, 1, 0.3, 1)}, ui.AtTime(460*time.Millisecond))

	col.Children(func() {
		size := float32(420)
		if compact {
			size = 350
		}
		g := float32(0)
		if over {
			g = 1
		}
		stage := ui.Box(c.Key("orbit")).Size(size, size*0.86).Center()
		pull := stage.AnimateTo("gather", g, gather)
		open := orbitOpen
		still := !a.ambient(c)
		boops, boopAt := a.boops, a.boopAt
		if a.prefs.Motion == "off" {
			boopAt = time.Time{}
		}
		// The back of the orbit, under the gopher.
		stage.Draw(func(pt *ui.Painter, r ui.Rect) { drawOrbit(pt, r, p, dark, pull, open, false, still, boops, boopAt) })
		stage.Children(func() {
			side := size * 0.74
			var gopher ui.Element
			if a.prefs.Mascot {
				gopher = ui.Image(c.Key("gopher"), mascot).Size(side, side).Fit(ui.Contain).
					Margin(0, 0, size*0.04, 0).Transition(ui.ElementTransition{Animation: &settle, Match: mascotMatch{}, Fly: true})
			} else {
				// Without the gopher, the convert mark as a sticker of its own.
				gopher = ui.Box(c.Key("badge")).Size(side*0.46, side*0.46).Radius(side * 0.14).Center().Background(p.accent)
				sticker(gopher, p, 3, 6)
				gopher.Children(func() { ui.Icon(c, icConvert).FontSize(side * 0.22).TextColor(p.accentInk) })
			}
			gopher.Label("小地鼠").Cursor(ui.CursorPointer)
			// At rest it sways; files over the window make it jump up, a
			// pointer over it makes it lean in, and a tap boops it, for the
			// fun of it: it hops and kicks the documents round. All of it
			// moves in frames that only paint.
			base := ui.Pose{Scale: 0.4 + 0.6*gopherIn}
			switch {
			case over:
				base = ui.Pose{Y: -22, Scale: 1.08, Rotate: -5}
			case gopher.Pressed():
				base = ui.Pose{Y: 2, Scale: 0.96}
			case hovers(gopher):
				base = ui.Pose{Y: -6, Scale: 1.03, Rotate: 3}
			}
			lift, motion := gopherPose(c, base, a.boopAt, !still && !over && gopherIn >= 1, 5, 1300*time.Millisecond)
			if over || gopherIn < 1 {
				motion = hop
			}
			gopher.AnimatePose(lift, motion).TransformOrigin(0.5, 1).Opacity(min(1, gopherIn*1.6))
			if gopher.Clicked() {
				a.boop(c.Now(), nil)
			}
			// The front of the orbit, over the gopher.
			ui.Box(c.Key("front")).Absolute().Fill().PassThrough().
				Draw(func(pt *ui.Painter, r ui.Rect) { drawOrbit(pt, r, p, dark, pull, open, true, still, boops, boopAt) })
		})

		head := "把文件丢给 Formigo"
		if over {
			head = "松手，交给我！"
		}
		ui.Text(c, head).Font(rounded()).FontSize(30).FontWeight(800).LetterSpacing(-0.4).TextColor(p.ink).
			Margin(4, 0, 0, 0).Opacity(titleIn).Translate(0, 14*(1-titleIn)).
			ContentTransition(ui.ContentTransition{Morph: true})
		// The kinds, as two even rows of chips of one width, columns aligned.
		ui.Column(c).Gap(8).Margin(14, 24, 0, 24).Opacity(titleIn).Translate(0, 10*(1-titleIn)).Children(func() {
			per := (len(convert.Kinds) + 1) / 2
			for i := 0; i < len(convert.Kinds); i += per {
				ui.Row(c.Key(i)).Gap(8).Justify(ui.Center).Children(func() {
					for _, k := range convert.Kinds[i:min(i+per, len(convert.Kinds))] {
						pill := ui.Row(c.Key(k)).Width(92).Gap(6).Padding(4, 10).Radius(999).Justify(ui.Center).Background(tintOf(k, dark))
						pill.Children(func() {
							ui.Icon(c, kindIcons[k]).FontSize(12).TextColor(hue(k, dark))
							ui.Text(c, k.String()).FontSize(12.5).FontWeight(600).TextColor(p.ink).SingleLine()
						})
					}
				})
			}
		})
		ui.Row(c).Gap(12).Margin(26, 0, 0, 0).Opacity(actionIn).Translate(0, 10*(1-actionIn)).Children(func() {
			if stickerButton(c, p, "pick", icPlus, "选择文件", true).Clicked() {
				a.chooseFiles()
			}
		})
	})
}

// stickerButton is a button of the icon's kind: a navy outline, a hard
// shadow, and a press that pushes it onto the page. It is a MyGo button,
// primary or not, with the sticker's look over it.
func stickerButton(c *ui.Context, p palette, key string, ic *ui.SVG, label string, primary bool) ui.Element {
	var b ui.Element
	if primary {
		b = ui.PrimaryButton(c.Key(key), "")
	} else {
		b = ui.Button(c.Key(key), "").Background(p.surface)
	}
	b.Height(44).Padding(0, 20).Radius(14).Gap(8).Label(label)
	pressable(b, p, 4)
	ink := p.ink
	if primary {
		ink = p.accentInk
	}
	b.Children(func() {
		ui.Icon(c, ic).FontSize(17).TextColor(ink)
		ui.Text(c, label).Font(rounded()).FontSize(15).FontWeight(700).TextColor(ink).SingleLine()
	})
	return b
}

// drawOrbit paints half the orbit: the far side, under the gopher, or the
// near side, over it. It only paints, as the time goes, without building the
// view again. pull, from 0 to 1, tightens it while files are dragged over;
// open spreads it out as the view arrives.
func drawOrbit(pt *ui.Painter, r ui.Rect, p palette, dark bool, pull, open float32, front, still bool, boops uint64, boopAt time.Time) {
	o := orbitIn(r, pull)
	o.rx *= float64(0.6 + 0.4*open)
	o.ry *= float64(0.6 + 0.4*open)
	reduce := still || pt.Preferences().ReduceMotion
	secs := float64(pt.Now().UnixNano()%int64(1e15)) / 1e9
	if reduce {
		secs = 0
	} else if front {
		pt.AnimationFrame() // one layer asks for the frames of both
	}
	turn := secs / orbitPeriod * 2 * math.Pi
	// Each tap on the gopher kicks the documents on round, for good.
	kick, flash := boopPhase(boopAt, pt.Now())
	if boops > 0 {
		turn += boopKick * (float64(boops) - 1 + kick)
	}
	if flash > 0 && front {
		pt.AnimationFrame() // the kick plays, even where nothing drifts
	}

	// The track, faint all round.
	track := p.lineStrong.MixOklab(p.accent, 0.25*pull)
	var path ui.Path
	started := false
	const steps = 96
	for i := 0; i <= steps; i++ {
		a := float64(i) / steps * 2 * math.Pi
		x, y, near := o.at(a)
		if near != front {
			started = false
			continue
		}
		if !started {
			path.MoveTo(x, y)
			started = true
		} else {
			path.LineTo(x, y)
		}
	}
	// The gopher's own arrow circles it already: the track only says where
	// the documents go, dashed and quiet.
	pt.StrokePath(path.Cap(ui.RoundCap).Dash(float32(secs*-14), 2, 7), 2, track.Alpha(open))

	// The documents riding it, smaller and paler on the far side.
	for _, s := range satellites {
		a := s.phase*2*math.Pi + turn
		x, y, near := o.at(a)
		if near != front {
			continue
		}
		depth := float32((math.Sin(a) + 1) / 2) // 0 far, 1 near
		scale := (0.62 + 0.44*depth) * (0.5 + 0.5*open)
		drawDoc(pt, x, y, scale, s, p, dark, (0.7+0.3*depth)*open)
	}

	// The icon's three strokes of surprise, as files come over it.
	burst := max(pull, float32(flash))
	if front && burst > 0.02 {
		cx, cy := r.X+r.W*0.73, r.Y+r.H*0.14
		for i, ang := range []float64{-62, -30, 2} {
			rad := ang * math.Pi / 180
			l0, l1 := float32(10), float32(10+14*burst)
			dx, dy := float32(math.Cos(rad)), float32(math.Sin(rad))
			var st ui.Path
			st.MoveTo(cx+dx*l0, cy+dy*l0).LineTo(cx+dx*l1, cy+dy*l1)
			pt.StrokePath(st.Cap(ui.RoundCap), 5, p.accent.Alpha(burst))
			_ = i
		}
	}
}

// drawDoc paints a document sticker: a page with a folded corner in the
// kind's tint, outlined in navy, its format on it.
func drawDoc(pt *ui.Painter, x, y, scale float32, s satellite, p palette, dark bool, alpha float32) {
	w, h := 46*scale, 54*scale
	fold := 13 * scale
	l, t := x-w/2, y-h/2
	k := hue(s.kind, dark)
	page := func() *ui.Path {
		var pg ui.Path
		rr := 7 * scale
		pg.MoveTo(l+rr, t).LineTo(l+w-fold, t).LineTo(l+w, t+fold).LineTo(l+w, t+h-rr).
			QuadTo(l+w, t+h, l+w-rr, t+h).LineTo(l+rr, t+h).QuadTo(l, t+h, l, t+h-rr).
			LineTo(l, t+rr).QuadTo(l, t, l+rr, t).Close()
		return &pg
	}
	// The hard shadow, then the page, its outline and its fold.
	pt.Transform(ui.Translation(0, 3*scale), 0, 0, func() { pt.FillPath(page(), p.outline.Alpha(alpha)) })
	face := ui.RGB(255, 255, 255).MixOklab(k, 0.16)
	if dark {
		face = p.surface.MixOklab(k, 0.25)
	}
	pt.FillPath(page(), face.Alpha(alpha))
	pt.StrokePath(page().Join(ui.RoundJoin), 2*scale+0.4, p.outline.Alpha(alpha))
	var f ui.Path
	f.MoveTo(l+w-fold, t).LineTo(l+w-fold, t+fold-2*scale).QuadTo(l+w-fold, t+fold, l+w-fold+2*scale, t+fold).LineTo(l+w, t+fold).Close()
	pt.FillPath(&f, k.Alpha(0.55*alpha))
	pt.StrokePath(f.Join(ui.RoundJoin), 1.6*scale+0.3, p.outline.Alpha(alpha))
	size := 10.5 * scale
	tw, th := pt.MeasureText(0, ui.Span{Text: s.label, Size: size, Weight: 800})
	pt.RichText(x-tw/2, y-th/2+3*scale, 0, ui.Span{Text: s.label, Size: size, Weight: 800, Color: k.MixOklab(p.outline, 0.35).Alpha(alpha)})
}

func wrap(t float64) float64 {
	t = math.Mod(t, 1)
	if t < 0 {
		t++
	}
	return t
}
