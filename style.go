package main

import (
	_ "embed"
	"runtime"
	"time"

	"formigo/internal/convert"

	"github.com/egoist/mygo/ui"
)

// palette is Formigo's look, taken from its icon: a sticker of a sky-blue
// gopher outlined in navy, an action-blue orbit arrow, and lavender and
// mint documents. Fills are flat, edges are navy outlines, and what can be
// pressed casts a hard shadow, as a sticker standing off the page.
type palette struct {
	bg, panel, surface, raised ui.Color
	line, lineStrong           ui.Color
	outline                    ui.Color // the navy edge of stickers
	ink, muted, faint          ui.Color
	accent, accentPressed      ui.Color
	accentInk                  ui.Color
	sky, skyTint               ui.Color // the gopher's blue
	glow                       ui.Color
	ok, bad                    ui.Color
}

func paletteFor(dark bool) palette {
	if dark {
		return palette{
			bg:            ui.Hex("#111214"),
			panel:         ui.Hex("#16171a"),
			surface:       ui.Hex("#1a1b1f"),
			raised:        ui.Hex("#222328"),
			line:          ui.RGBA(255, 255, 255, 0.09),
			lineStrong:    ui.RGBA(255, 255, 255, 0.18),
			outline:       ui.Hex("#050506"),
			ink:           ui.Hex("#ececee"),
			muted:         ui.Hex("#9a9ca3"),
			faint:         ui.Hex("#686a72"),
			accent:        ui.Hex("#3ccf9e"),
			accentPressed: ui.Hex("#3873e6"),
			accentInk:     ui.Hex("#ffffff"),
			sky:           ui.Hex("#5cb9f7"),
			skyTint:       ui.Hex("#1c2a33"),
			glow:          ui.Hex("#3ccf9e").Alpha(0.3),
			ok:            ui.Hex("#a3d65c"),
			bad:           ui.Hex("#ff6b74"),
		}
	}
	return palette{
		bg:            ui.Hex("#f2f6fc"),
		panel:         ui.Hex("#ffffff"),
		surface:       ui.Hex("#ffffff"),
		raised:        ui.Hex("#e9f3fe"),
		line:          ui.RGBA(20, 33, 61, 0.11),
		lineStrong:    ui.RGBA(20, 33, 61, 0.2),
		outline:       ui.Hex("#14213d"),
		ink:           ui.Hex("#14213d"),
		muted:         ui.Hex("#56627e"),
		faint:         ui.Hex("#8e99b2"),
		accent:        ui.Hex("#11996a"),
		accentPressed: ui.Hex("#1555dc"),
		accentInk:     ui.Hex("#ffffff"),
		sky:           ui.Hex("#5cb9f7"),
		skyTint:       ui.Hex("#e3f2fe"),
		glow:          ui.Hex("#11996a").Alpha(0.22),
		ok:            ui.Hex("#5f9e1f"),
		bad:           ui.Hex("#ef4f5a"),
	}
}

// oled is the dark palette on pure black, for OLED screens, whose black
// pixels are off: cards stand off it by a hair of gray and their edges.
func oled(p palette) palette {
	p.bg = ui.Hex("#000000")
	p.panel = ui.Hex("#0a0a0d")
	p.surface = ui.Hex("#111116")
	p.raised = ui.Hex("#18181f")
	p.skyTint = ui.Hex("#0c1a2c")
	p.line = ui.RGBA(232, 240, 255, 0.12)
	p.lineStrong = ui.RGBA(232, 240, 255, 0.22)
	p.outline = ui.Hex("#000000")
	return p
}

// hue is the color of a kind of file, from the icon's documents: lavender
// pictures, mint sound, the gopher's sky for data, and a warm amber for
// documents beside them.
func hue(k convert.Kind, dark bool) ui.Color {
	switch k {
	case convert.Image:
		if dark {
			return ui.Hex("#a597ff")
		}
		return ui.Hex("#7a68f0")
	case convert.Audio:
		if dark {
			return ui.Hex("#3fd39f")
		}
		return ui.Hex("#1fa874")
	case convert.Data:
		if dark {
			return ui.Hex("#5cb9f7")
		}
		return ui.Hex("#2492e0")
	case convert.Video:
		if dark {
			return ui.Hex("#ff7a9a")
		}
		return ui.Hex("#e0436b")
	case convert.Subtitle:
		if dark {
			return ui.Hex("#c2a8ff")
		}
		return ui.Hex("#9a6cf0")
	case convert.Archive:
		if dark {
			return ui.Hex("#d6b48a")
		}
		return ui.Hex("#a9773d")
	case convert.Font:
		if dark {
			return ui.Hex("#7fd7d0")
		}
		return ui.Hex("#1f9e94")
	}
	if dark {
		return ui.Hex("#ffbf55")
	}
	return ui.Hex("#e8930f")
}

// tintOf is a kind's color as a sticker's fill: light in the light theme,
// deep in the dark.
func tintOf(k convert.Kind, dark bool) ui.Color {
	h := hue(k, dark)
	if dark {
		return h.Alpha(0.18)
	}
	return ui.RGB(255, 255, 255).MixOklab(h, 0.14)
}

// The motion vocabulary. Everything that moves uses one of these, so the
// app moves as one thing.
var (
	// settle is for layout: panels arriving, rows moving into place.
	settle = ui.Animation{Spring: ui.SpringOptions{Duration: 520 * time.Millisecond, Bounce: 0.15}}
	// snap is for direct responses to the pointer.
	snap = ui.Animation{Spring: ui.SpringOptions{Duration: 280 * time.Millisecond, Bounce: 0.25}}
	// pop is for small rewards, as a check mark landing.
	pop = ui.Animation{Spring: ui.SpringOptions{Duration: 480 * time.Millisecond, Bounce: 0.5}}
	// hop is the gopher's: a jump with a visible bounce.
	hop = ui.Animation{Spring: ui.SpringOptions{Duration: 560 * time.Millisecond, Bounce: 0.55}}
	// gather is for the orbit drawing in while files are dragged over.
	gather = ui.Animation{Spring: ui.SpringOptions{Duration: 650 * time.Millisecond, Bounce: 0.3}}
	// tint is for colors that follow state.
	tint = ui.Animation{Duration: 160 * time.Millisecond, Ease: ui.EaseOut, Oklab: true}
)

// rowMotion brings a file's row in from below, as a sticker dropped on the
// page, one after another, and folds it away when removed.
func rowMotion(order int) ui.ElementTransition {
	a := pop
	a.Delay = time.Duration(min(order, 12)) * 40 * time.Millisecond
	return ui.ElementTransition{
		Animation:    &a,
		ExitDelay:    -1,
		ExitDuration: 220 * time.Millisecond,
		ExitEase:     ui.EaseIn,
		Position:     true,
		Enter:        &ui.Motion{Y: 22, Scale: 0.94, Rotate: -1.5},
		Exit:         &ui.Motion{Collapse: true, X: -24, Rotate: -3},
	}
}

// crossfade swaps one view for another through a short blur.
var crossfade = ui.ElementTransition{
	Animation:    &settle,
	ExitDuration: 200 * time.Millisecond,
	Enter:        &ui.Motion{Scale: 0.985, Blur: 8},
	Exit:         &ui.Motion{Scale: 1.015, Blur: 8},
}

// mascotMatch is the identity the hero's gopher shares with the title bar's,
// which it flies into once there are files.
type mascotMatch struct{}

// pickMatch is the identity of a kind's chosen format's highlight, which
// slides from tile to tile.
type pickMatch struct {
	k     convert.Kind
	where string // the panel, the queue's strip or the sheet, which show at once
}

//go:embed assets/mascot.png
var mascotPNG []byte

// mascot is the gopher of the icon, holding a document being converted.
var mascot = func() *ui.Bitmap {
	b, err := ui.DecodeBitmap(mascotPNG)
	if err != nil {
		panic(err)
	}
	return b
}()

// sticker gives an element the look of the icon's stickers: a navy outline
// and a hard shadow below it, lift DIPs deep (0 for none).
func sticker(e ui.Element, p palette, width, lift float32) ui.Element {
	e.Border(width, p.outline)
	if lift > 0 {
		e.Shadow(0, lift, 0, 0, p.outline)
	}
	return e
}

// pressable makes a sticker button sink as it is pressed: its shadow goes
// as it moves down onto the page, and comes back as it springs up.
func pressable(e ui.Element, p palette, lift float32) ui.Element {
	down := float32(0)
	if e.Pressed() {
		down = 1
	}
	if hovers(e) && !e.Pressed() {
		down = -0.35
	}
	d := e.AnimateTo("press", down, snap)
	sticker(e, p, 2, max(0, lift*(1-d)))
	e.Translate(0, lift*d)
	return e
}

// Icons, drawn on a 24-unit grid with round strokes, a little heavier than
// usual to sit beside the icon's outlines.
func icon(paths string) *ui.SVG {
	return ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.1" stroke-linecap="round" stroke-linejoin="round">` + paths + `</svg>`))
}

var (
	icPlus    = icon(`<path d="M12 5v14M5 12h14"/>`)
	icClose   = icon(`<path d="M6 6l12 12M18 6L6 18"/>`)
	icCheck   = icon(`<path d="M5 12.5l4.5 4.5L19 7.5"/>`)
	icAlert   = icon(`<path d="M12 8v5"/><path d="M12 16.5v.01"/><circle cx="12" cy="12" r="9"/>`)
	icArrow   = icon(`<path d="M5 12h13"/><path d="M13 6l6 6-6 6"/>`)
	icFolder  = icon(`<path d="M3.5 7.5a2 2 0 0 1 2-2h4l2 2.5h7a2 2 0 0 1 2 2v7.5a2 2 0 0 1-2 2h-13a2 2 0 0 1-2-2z"/>`)
	icSweep   = icon(`<path d="M4 7h16"/><path d="M9 7V4.5h6V7"/><path d="M6.5 7l1 12.5h9l1-12.5"/>`)
	icConvert = icon(`<path d="M4 9a8 8 0 0 1 14.3-3.3L20 7.5"/><path d="M20 3.5v4h-4"/><path d="M20 15a8 8 0 0 1-14.3 3.3L4 16.5"/><path d="M4 20.5v-4h4"/>`)
	icReveal  = icon(`<path d="M14 4h6v6"/><path d="M20 4l-8.5 8.5"/><path d="M18 14v4.5a1.5 1.5 0 0 1-1.5 1.5h-11A1.5 1.5 0 0 1 4 18.5v-11A1.5 1.5 0 0 1 5.5 6H10"/>`)
	icGear    = icon(`<circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.7 1.7 0 0 0 .3 1.8l.1.1a2 2 0 1 1-2.8 2.8l-.1-.1a1.7 1.7 0 0 0-1.8-.3 1.7 1.7 0 0 0-1 1.5V21a2 2 0 1 1-4 0v-.1a1.7 1.7 0 0 0-1.1-1.5 1.7 1.7 0 0 0-1.8.3l-.1.1a2 2 0 1 1-2.8-2.8l.1-.1a1.7 1.7 0 0 0 .3-1.8 1.7 1.7 0 0 0-1.5-1H3a2 2 0 1 1 0-4h.1a1.7 1.7 0 0 0 1.5-1.1 1.7 1.7 0 0 0-.3-1.8l-.1-.1a2 2 0 1 1 2.8-2.8l.1.1a1.7 1.7 0 0 0 1.8.3H9a1.7 1.7 0 0 0 1-1.5V3a2 2 0 1 1 4 0v.1a1.7 1.7 0 0 0 1 1.5 1.7 1.7 0 0 0 1.8-.3l.1-.1a2 2 0 1 1 2.8 2.8l-.1.1a1.7 1.7 0 0 0-.3 1.8V9a1.7 1.7 0 0 0 1.5 1H21a2 2 0 1 1 0 4h-.1a1.7 1.7 0 0 0-1.5 1z"/>`)
	icPalette = icon(`<path d="M12 3.5a8.5 8.5 0 1 0 0 17c1.1 0 1.7-.9 1.7-1.8 0-.5-.2-.9-.5-1.2-.3-.3-.5-.7-.5-1.2 0-1 .8-1.8 1.8-1.8h2.1a4 4 0 0 0 3.9-4c0-3.9-3.8-7-8.5-7z"/><circle cx="7.5" cy="11" r="1.1"/><circle cx="10.5" cy="7.5" r="1.1"/><circle cx="15" cy="8" r="1.1"/>`)
	icSliders = icon(`<path d="M5 6h9M18 6h1M5 12h3M12 12h7M5 18h11M20 18h-1"/><circle cx="16" cy="6" r="2"/><circle cx="10" cy="12" r="2"/><circle cx="18" cy="18" r="2"/>`)
	icInfo    = icon(`<circle cx="12" cy="12" r="9"/><path d="M12 11v5.5"/><path d="M12 7.5v.01"/>`)
	icUpDown  = icon(`<path d="M8 9.5l4-4 4 4"/><path d="M8 14.5l4 4 4-4"/>`)
	icRetry   = icon(`<path d="M4.5 12a7.5 7.5 0 1 0 2.2-5.3L4.5 9"/><path d="M4.5 4.5V9H9"/>`)
)

var kindIcons = map[convert.Kind]*ui.SVG{
	convert.Image:    icon(`<rect x="3.5" y="4.5" width="17" height="15" rx="2.5"/><circle cx="9" cy="10" r="1.6"/><path d="M20.5 15.5l-4.5-4.5-8.5 8.5"/>`),
	convert.Audio:    icon(`<path d="M8 5.5v13l10.5-6.5z"/>`),
	convert.Data:     icon(`<path d="M8.5 4.5c-2 0-3 1-3 3v2c0 1.5-.8 2.5-2 2.5 1.2 0 2 1 2 2.5v2c0 2 1 3 3 3"/><path d="M15.5 4.5c2 0 3 1 3 3v2c0 1.5.8 2.5 2 2.5-1.2 0-2 1-2 2.5v2c0 2-1 3-3 3"/>`),
	convert.Video:    icon(`<rect x="3.5" y="5.5" width="13" height="13" rx="2.5"/><path d="M16.5 10l4-2.5v9l-4-2.5"/>`),
	convert.Subtitle: icon(`<rect x="3.5" y="5" width="17" height="14" rx="2.5"/><path d="M7 12.5h4M13.5 12.5h3.5M7 15.5h7"/>`),
	convert.Archive:  icon(`<rect x="4" y="3.5" width="16" height="17" rx="2.5"/><path d="M12 3.5v2M12 7.5v2M12 11.5v2"/><rect x="10.3" y="13.5" width="3.4" height="3.6" rx="1"/>`),
	convert.Font:     icon(`<path d="M5 19.5l5.5-15h1l5.5 15"/><path d="M7.3 14h7.4"/><path d="M15.5 19.5h4"/>`),
	convert.Doc:      icon(`<path d="M14 3.5H7a2 2 0 0 0-2 2v13a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2v-10z"/><path d="M14 3.5v5h5"/><path d="M8.5 13h7M8.5 16.5h5"/>`),
}

// rounded is the face of headlines: a rounded sans, as friendly as the
// gopher, where the system has one. Chinese falls back to the system's.
func rounded() string {
	if runtime.GOOS == "darwin" {
		return "SF Pro Rounded"
	}
	return ""
}

// boopKick is how far, in radians, a tap on the gopher kicks the documents
// round the orbit.
const boopKick = 1.4

// The springs of the gopher: the jump of a boop, quick and straight up; the
// landing, bouncing, whose overshoot is the squash and the wiggle; and the
// slow sway of its bob.
var (
	hopUp   = ui.Animation{Spring: ui.SpringOptions{Duration: 200 * time.Millisecond}}
	landing = ui.Animation{Spring: ui.SpringOptions{Duration: 620 * time.Millisecond, Bounce: 0.55}}
)

// hopTime is how long a boop's jump goes up before it falls back.
const hopTime = 170 * time.Millisecond

// gopherPose is where a gopher shows: base, as its pointer state puts it,
// raised by the hop of a boop at hop, and swaying by amp DIPs every half
// period while it bobs. Its motion only paints: the view builds again only
// where the pose turns, which it asks for with After. It returns the pose
// and the spring to it.
func gopherPose(c *ui.Context, base ui.Pose, hop time.Time, bob bool, amp float32, half time.Duration) (ui.Pose, ui.Animation) {
	now := c.Now()
	if base.Scale == 0 {
		base.Scale = 1
	}
	if !hop.IsZero() {
		if dt := now.Sub(hop); dt < hopTime {
			c.After(hopTime - dt)
			base.Y -= 30
			base.Rotate -= 9
			base.Scale *= 1.08
			return base, hopUp
		} else if dt < 900*time.Millisecond {
			return base, landing // falling back, bouncing as it lands
		}
	}
	if !bob {
		return base, snap
	}
	// Up for one half period, down for the next, the spring between them
	// as slow as the half, so that it sways as a sine would.
	ms := now.UnixMilli()
	h := half.Milliseconds()
	c.After(time.Duration(h-ms%h) * time.Millisecond)
	if (ms/h)%2 == 0 {
		base.Y -= amp
	}
	return base, ui.Animation{Spring: ui.SpringOptions{Duration: half}}
}

// hovers is whether the pointer is over e.
func hovers(e ui.Element) bool { return e.Hovered() }

// softPress gives a quiet button the feel of the rest: it sinks a little as
// it is pressed, and lifts a little under the pointer if lift.
func softPress(e ui.Element, lift bool) ui.Element {
	pose := ui.Pose{}
	switch {
	case e.Pressed():
		pose.Scale = 0.94
	case lift && hovers(e):
		pose.Y = -1.5
	}
	return e.AnimatePose(pose, snap)
}

// boopPhase is how far along the last tap's kick of the orbit is, from 0 to
// 1, and its flash of the pop strokes, from 1 to 0.
func boopPhase(at, now time.Time) (kick, flash float64) {
	if at.IsZero() {
		return 1, 0
	}
	t := now.Sub(at).Seconds()
	k := min(1, t/0.9)
	kick = 1 - (1-k)*(1-k)*(1-k)
	flash = max(0, 1-t/0.7)
	return kick, flash
}
