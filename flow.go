package main

import (
	"slices"
	"strings"

	"formigo/internal/convert"

	"github.com/egoist/mygo/ui"
)

// exts are the extensions of a kind's files queued, the first few.
func (a *formigo) exts(k convert.Kind) string {
	var exts []string
	for _, it := range a.items {
		if e := strings.ToUpper(convert.Ext(it.path)); it.kind == k && !slices.Contains(exts, e) {
			exts = append(exts, e)
		}
	}
	if len(exts) > 3 {
		exts = append(exts[:3], "…")
	}
	return strings.Join(exts, "、")
}

// allPicker offers, when files of several kinds are queued, the formats
// every one of them can become, one choice for the whole batch: pictures
// and documents all to PDF, say. Choosing gives every file that format of
// its own; each row's chip can still change one.
func (a *formigo) allPicker(c *ui.Context, p palette, where string) {
	common := a.commonTargets()
	if len(common) == 0 {
		return
	}
	chosen := a.allTarget()
	if !slices.ContainsFunc(common, func(f convert.Format) bool { return f.ID == chosen }) {
		chosen = ""
	}
	before := chosen
	ui.Column(c.Key("all-" + where)).Gap(10).Transition(ui.ElementTransition{
		Animation: &settle, Enter: &ui.Motion{Y: 8, Blur: 6}, Exit: &ui.Motion{Collapse: true, Blur: 6},
	}).Children(func() {
		ui.Row(c).Gap(8).Children(func() {
			badge := ui.Box(c).Size(22, 22).Radius(7).Center().Background(p.skyTint)
			sticker(badge, p, 1.5, 0)
			badge.Children(func() { ui.Icon(c, icConvert).FontSize(12).TextColor(p.accent) })
			ui.Text(c, "全部转为").Font(rounded()).FontSize(14).FontWeight(800).TextColor(p.ink)
			ui.Spacer(c)
			ui.Textf(c, "%d 个文件", len(a.items)).FontSize(11.5).TextColor(p.faint).FontFeatures("tnum").
				ContentTransition(ui.ContentTransition{Numeric: true})
		})
		tiles := func() {
			for _, f := range common {
				a.formatTile(c, p, f.Kind, f, &chosen, "all-"+where)
			}
		}
		ui.RadioGroup(c.Key("group"), tiles).Grid().Columns(min(4, len(common))).Gap(8).Label("全部文件的目标格式")
	})
	if chosen != before && chosen != "" {
		a.allTo(chosen)
	}
}
