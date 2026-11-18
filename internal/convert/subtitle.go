package convert

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Subtitles and lyrics: each read as cues, a time and its text, written in
// any of the others, or as their text alone.

func init() {
	subs := []string{"srt", "vtt", "ass", "lrc"}
	register(&edge{engine: "subtitle", from: subs, to: append(subs, "txt"), cost: 1, run: convertSubtitle})
}

type cue struct {
	start, end time.Duration
	text       string // lines apart by "\n"
}

func convertSubtitle(j *job) error {
	text, err := readText(j.src)
	if err != nil {
		return err
	}
	var cues []cue
	switch j.from.ID {
	case "srt", "vtt":
		cues = parseSRT(text)
	case "ass":
		cues = parseASS(text)
	case "lrc":
		cues = parseLRC(text)
	}
	if len(cues) == 0 {
		return fmt.Errorf("没有读到字幕")
	}
	var out string
	switch j.to.ID {
	case "srt":
		out = writeSRT(cues, false)
	case "vtt":
		out = writeSRT(cues, true)
	case "ass":
		out = writeASS(cues)
	case "lrc":
		out = writeLRC(cues)
	case "txt":
		var b strings.Builder
		for _, c := range cues {
			b.WriteString(c.text + "\n")
		}
		out = b.String()
	}
	return os.WriteFile(j.dst, []byte(out), 0o644)
}

var (
	srtTime = regexp.MustCompile(`(?:(\d+):)?(\d{1,2}):(\d{2})[.,](\d{1,3})\s*-->\s*(?:(\d+):)?(\d{1,2}):(\d{2})[.,](\d{1,3})`)
	tagRe   = regexp.MustCompile(`<[^>]+>|\{\\[^}]*\}`)
)

func clockOf(h, m, s, frac string) time.Duration {
	hh, _ := strconv.Atoi(h)
	mm, _ := strconv.Atoi(m)
	ss, _ := strconv.Atoi(s)
	for len(frac) < 3 {
		frac += "0"
	}
	ms, _ := strconv.Atoi(frac[:3])
	return time.Duration(hh)*time.Hour + time.Duration(mm)*time.Minute + time.Duration(ss)*time.Second + time.Duration(ms)*time.Millisecond
}

// parseSRT reads SubRip and WebVTT, which differ in little but their
// header and their decimal mark.
func parseSRT(text string) []cue {
	var cues []cue
	for _, block := range regexp.MustCompile(`\n\s*\n`).Split(strings.TrimSpace(text), -1) {
		lines := strings.Split(block, "\n")
		for i, l := range lines {
			m := srtTime.FindStringSubmatch(l)
			if m == nil {
				continue
			}
			body := strings.TrimSpace(strings.Join(lines[i+1:], "\n"))
			body = strings.TrimSpace(tagRe.ReplaceAllString(body, ""))
			if body != "" {
				cues = append(cues, cue{clockOf(m[1], m[2], m[3], m[4]), clockOf(m[5], m[6], m[7], m[8]), body})
			}
			break
		}
	}
	return cues
}

func parseASS(text string) []cue {
	var cues []cue
	format := []string{"layer", "start", "end", "style", "name", "marginl", "marginr", "marginv", "effect", "text"}
	for _, l := range strings.Split(text, "\n") {
		l = strings.TrimSpace(l)
		low := strings.ToLower(l)
		if strings.HasPrefix(low, "format:") && len(cues) == 0 {
			var f []string
			for _, p := range strings.Split(l[7:], ",") {
				f = append(f, strings.ToLower(strings.TrimSpace(p)))
			}
			if contains(f, "text") {
				format = f
			}
			continue
		}
		if !strings.HasPrefix(low, "dialogue:") {
			continue
		}
		parts := strings.SplitN(l[9:], ",", len(format))
		if len(parts) < len(format) {
			continue
		}
		field := map[string]string{}
		for i, name := range format {
			field[name] = strings.TrimSpace(parts[i])
		}
		start, ok1 := assTime(field["start"])
		end, ok2 := assTime(field["end"])
		body := strings.ReplaceAll(strings.ReplaceAll(field["text"], `\N`, "\n"), `\n`, "\n")
		body = strings.TrimSpace(tagRe.ReplaceAllString(body, ""))
		if ok1 && ok2 && body != "" {
			cues = append(cues, cue{start, end, body})
		}
	}
	sort.SliceStable(cues, func(a, b int) bool { return cues[a].start < cues[b].start })
	return cues
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

var assClock = regexp.MustCompile(`^(\d+):(\d{2}):(\d{2})[.](\d{1,3})$`)

func assTime(s string) (time.Duration, bool) {
	m := assClock.FindStringSubmatch(s)
	if m == nil {
		return 0, false
	}
	frac := m[4]
	if len(frac) == 2 { // centiseconds
		frac += "0"
	}
	return clockOf(m[1], m[2], m[3], frac), true
}

var lrcTime = regexp.MustCompile(`\[(\d{1,3}):(\d{2})(?:[.:](\d{1,3}))?\]`)

// parseLRC reads lyrics: a line's times, each a cue that lasts until the
// next.
func parseLRC(text string) []cue {
	var cues []cue
	for _, l := range strings.Split(text, "\n") {
		ms := lrcTime.FindAllStringSubmatch(l, -1)
		if ms == nil {
			continue
		}
		body := strings.TrimSpace(lrcTime.ReplaceAllString(l, ""))
		body = regexp.MustCompile(`<\d+:\d+(?:[.:]\d+)?>`).ReplaceAllString(body, "")
		for _, m := range ms {
			cues = append(cues, cue{start: clockOf("0", m[1], m[2], m[3]), text: body})
		}
	}
	sort.SliceStable(cues, func(a, b int) bool { return cues[a].start < cues[b].start })
	var out []cue
	for i, c := range cues {
		c.end = c.start + 4*time.Second
		if i+1 < len(cues) {
			c.end = cues[i+1].start
		}
		if c.text != "" {
			out = append(out, c)
		}
	}
	return out
}

func stamp(d time.Duration, sep string) string {
	ms := d.Milliseconds()
	return fmt.Sprintf("%02d:%02d:%02d%s%03d", ms/3600000, ms/60000%60, ms/1000%60, sep, ms%1000)
}

func writeSRT(cues []cue, vtt bool) string {
	var b strings.Builder
	if vtt {
		b.WriteString("WEBVTT\n\n")
	}
	for i, c := range cues {
		if vtt {
			fmt.Fprintf(&b, "%s --> %s\n%s\n\n", stamp(c.start, "."), stamp(c.end, "."), c.text)
		} else {
			fmt.Fprintf(&b, "%d\n%s --> %s\n%s\n\n", i+1, stamp(c.start, ","), stamp(c.end, ","), c.text)
		}
	}
	return b.String()
}

func writeASS(cues []cue) string {
	var b strings.Builder
	b.WriteString("[Script Info]\nScriptType: v4.00+\nPlayResX: 1920\nPlayResY: 1080\nWrapStyle: 0\n\n")
	b.WriteString("[V4+ Styles]\nFormat: Name, Fontname, Fontsize, PrimaryColour, SecondaryColour, OutlineColour, BackColour, Bold, Italic, Underline, StrikeOut, ScaleX, ScaleY, Spacing, Angle, BorderStyle, Outline, Shadow, Alignment, MarginL, MarginR, MarginV, Encoding\n")
	b.WriteString("Style: Default,PingFang SC,64,&H00FFFFFF,&H000000FF,&H00000000,&H64000000,0,0,0,0,100,100,0,0,1,3,1,2,60,60,50,1\n\n")
	b.WriteString("[Events]\nFormat: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\n")
	at := func(d time.Duration) string {
		cs := d.Milliseconds() / 10
		return fmt.Sprintf("%d:%02d:%02d.%02d", cs/360000, cs/6000%60, cs/100%60, cs%100)
	}
	for _, c := range cues {
		fmt.Fprintf(&b, "Dialogue: 0,%s,%s,Default,,0,0,0,,%s\n", at(c.start), at(c.end), strings.ReplaceAll(c.text, "\n", `\N`))
	}
	return b.String()
}

func writeLRC(cues []cue) string {
	var b strings.Builder
	for _, c := range cues {
		cs := c.start.Milliseconds() / 10
		fmt.Fprintf(&b, "[%02d:%02d.%02d]%s\n", cs/6000, cs/100%60, cs%100, strings.ReplaceAll(c.text, "\n", " "))
	}
	return b.String()
}
