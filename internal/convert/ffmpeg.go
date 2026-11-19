package convert

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// FFmpeg converts video and sound, and the pictures Go cannot read or
// write: the copy the app ships with, set by SetFFmpeg, or one on the
// computer.

var (
	videoIn  = []string{"mp4", "mov", "mkv", "webm", "avi", "wmv", "flv", "m4v", "ts", "3gp", "mpg", "ogv"}
	videoOut = []string{"mp4", "mov", "mkv", "webm", "avi", "wmv", "flv", "m4v", "ts", "3gp", "mpg", "ogv"}
	soundIn  = []string{"mp3", "m4a", "aac", "wav", "flac", "ogg", "opus", "aiff", "wma", "amr", "ape", "wv", "caf", "ac3", "mka"}
	soundOut = []string{"mp3", "m4a", "aac", "wav", "flac", "ogg", "opus", "aiff"}
)

func init() {
	have := func() bool { return ffmpegPath() != "" }
	register(
		&edge{engine: "ffmpeg", from: videoIn, to: videoOut, cost: 1, ok: have, run: runFFmpeg},
		// A video's sound, an animated GIF of it, or a frame of it, which
		// go no further: a video's sound is not a picture's.
		&edge{engine: "ffmpeg", from: videoIn, to: append([]string{"gif", "png", "jpg"}, soundOut...), cost: 1, final: true, ok: have, run: runFFmpeg},
		&edge{engine: "ffmpeg", from: soundIn, to: soundOut, cost: 1, ok: have, run: runFFmpeg},
		// An animated GIF as a video.
		&edge{engine: "ffmpeg", from: []string{"gif"}, to: []string{"mp4", "webm", "mov"}, cost: 1, first: true, ok: have, run: runFFmpeg},
		// Pictures Go cannot read, where the system cannot either.
		&edge{engine: "ffmpeg", from: []string{"heic", "avif", "jxl"}, to: []string{"png"}, cost: 2, ok: have, run: runFFmpeg},
		&edge{engine: "ffmpeg", from: []string{"png", "jpg"}, to: []string{"avif"}, cost: 1,
			ok: func() bool { return have() && encoder("libsvtav1", "libaom-av1") != "" }, run: runFFmpeg},
		&edge{engine: "ffmpeg", from: []string{"png", "jpg"}, to: []string{"jxl"}, cost: 1,
			ok: func() bool { return have() && encoder("libjxl") != "" }, run: runFFmpeg},
	)
}

var ffmpeg struct {
	sync.Mutex
	path     string
	looked   bool
	encoders map[string]bool
}

// SetFFmpeg sets the FFmpeg the app ships with.
func SetFFmpeg(path string) {
	ffmpeg.Lock()
	ffmpeg.path, ffmpeg.looked, ffmpeg.encoders = path, true, nil
	ffmpeg.Unlock()
	replan()
}

// ffmpegPath is the FFmpeg conversions run, "" when there is none.
func ffmpegPath() string {
	ffmpeg.Lock()
	defer ffmpeg.Unlock()
	if ffmpeg.looked {
		return ffmpeg.path
	}
	ffmpeg.looked = true
	candidates := []string{os.Getenv("FORMIGO_FFMPEG")}
	if p, err := exec.LookPath("ffmpeg"); err == nil {
		candidates = append(candidates, p)
	}
	candidates = append(candidates, "/opt/homebrew/bin/ffmpeg", "/usr/local/bin/ffmpeg", "/usr/bin/ffmpeg")
	for _, c := range candidates {
		if st, err := os.Stat(c); c != "" && err == nil && !st.IsDir() {
			ffmpeg.path = c
			break
		}
	}
	return ffmpeg.path
}

// encoder returns the first of names this FFmpeg has, "" if none.
func encoder(names ...string) string {
	path := ffmpegPath()
	if path == "" {
		return ""
	}
	ffmpeg.Lock()
	if ffmpeg.encoders == nil {
		ffmpeg.encoders = map[string]bool{}
		cmd := exec.Command(path, "-hide_banner", "-encoders")
		hideWindow(cmd)
		out, _ := cmd.Output()
		for _, line := range strings.Split(string(out), "\n") {
			if fs := strings.Fields(line); len(fs) >= 2 && len(fs[0]) == 6 {
				ffmpeg.encoders[fs[1]] = true
			}
		}
	}
	enc := ffmpeg.encoders
	ffmpeg.Unlock()
	for _, n := range names {
		if enc[n] {
			return n
		}
	}
	return ""
}

// soundArgs are the arguments that encode sound in the format to.
func soundArgs(to string, o Options) ([]string, error) {
	q := o.quality()
	var a []string
	switch to {
	case "mp3":
		if e := encoder("libmp3lame", "libshine"); e != "" {
			a = []string{"-c:a", e}
			if e == "libmp3lame" {
				a = append(a, "-q:a", strconv.Itoa(max(0, min(9, (100-q)/11))))
			} else {
				a = append(a, "-b:a", fmt.Sprintf("%dk", 96+q))
			}
		} else {
			return nil, errors.New("这个 FFmpeg 不能写 MP3")
		}
	case "m4a", "aac":
		a = []string{"-c:a", encoder("aac_at", "aac"), "-b:a", fmt.Sprintf("%dk", 64+q*8/5)}
		if to == "aac" {
			a = append(a, "-f", "adts")
		}
	case "wav":
		a = []string{"-c:a", map[bool]string{true: "pcm_s24le", false: "pcm_s16le"}[o.BitDepth == 24]}
	case "aiff":
		a = []string{"-c:a", map[bool]string{true: "pcm_s24be", false: "pcm_s16be"}[o.BitDepth == 24]}
	case "flac":
		a = []string{"-c:a", "flac", "-sample_fmt", map[bool]string{true: "s32", false: "s16"}[o.BitDepth == 24]}
		if o.BitDepth == 24 {
			a = append(a, "-bits_per_raw_sample", "24")
		}
	case "ogg":
		e := encoder("libvorbis", "vorbis")
		a = []string{"-c:a", e}
		if e == "libvorbis" {
			a = append(a, "-q:a", strconv.Itoa(max(0, min(10, q/10))))
		} else {
			a = append(a, "-strict", "-2")
		}
	case "opus":
		e := encoder("libopus", "opus")
		a = []string{"-c:a", e, "-b:a", fmt.Sprintf("%dk", 32+q*11/10)}
		if e == "opus" {
			a = append(a, "-strict", "-2")
		}
	default:
		return nil, ErrUnsupported
	}
	if o.Mono {
		a = append(a, "-ac", "1")
	}
	return a, nil
}

// videoArgs are the arguments that encode video, and its sound, in the
// container to.
func videoArgs(to string, o Options) ([]string, error) {
	q := o.quality()
	crf := strconv.Itoa(max(14, min(40, 41-q*21/100)))
	h264 := func() []string {
		switch e := encoder("libx264", "h264_videotoolbox", "libopenh264", "h264_mf"); e {
		case "libx264":
			return []string{"-c:v", e, "-crf", crf, "-preset", "medium", "-pix_fmt", "yuv420p"}
		case "h264_videotoolbox":
			return []string{"-c:v", e, "-q:v", strconv.Itoa(max(30, q-15)), "-pix_fmt", "yuv420p"}
		case "":
			return nil
		default:
			return []string{"-c:v", e, "-b:v", fmt.Sprintf("%dk", q*80), "-pix_fmt", "yuv420p"}
		}
	}
	aac, _ := soundArgs("m4a", Options{Quality: min(q, 85), Mono: o.Mono})
	var a []string
	switch to {
	case "mp4", "mov", "m4v", "mkv", "flv", "ts", "3gp":
		v := h264()
		if v == nil {
			return nil, errors.New("这个 FFmpeg 不能写 H.264 视频")
		}
		a = append(v, aac...)
		if to == "mp4" || to == "mov" || to == "m4v" {
			a = append(a, "-movflags", "+faststart")
		}
		if to == "3gp" {
			a = append(a, "-vf", "scale='min(1280,iw)':-2", "-ar", "44100")
		}
	case "webm":
		switch e := encoder("libvpx-vp9", "libsvtav1", "libaom-av1", "libvpx"); e {
		case "libvpx-vp9", "libvpx":
			a = []string{"-c:v", e, "-crf", strconv.Itoa(max(15, min(50, 50-q*28/100))), "-b:v", "0", "-row-mt", "1", "-deadline", "good", "-cpu-used", "4", "-pix_fmt", "yuv420p"}
		case "libsvtav1", "libaom-av1":
			a = []string{"-c:v", e, "-crf", strconv.Itoa(max(20, min(55, 55-q*30/100))), "-pix_fmt", "yuv420p"}
		default:
			return nil, errors.New("这个 FFmpeg 不能写 WebM 视频")
		}
		opus, _ := soundArgs("opus", Options{Quality: q, Mono: o.Mono})
		a = append(a, opus...)
	case "avi":
		a = []string{"-c:v", "mpeg4", "-q:v", strconv.Itoa(max(2, min(20, (100-q)/5))), "-tag:v", "XVID"}
		if mp3, err := soundArgs("mp3", Options{Quality: q, Mono: o.Mono}); err == nil {
			a = append(a, mp3...)
		}
	case "wmv":
		a = []string{"-c:v", "wmv2", "-b:v", fmt.Sprintf("%dk", q*60), "-c:a", "wmav2", "-b:a", "192k"}
	case "mpg":
		a = []string{"-c:v", "mpeg2video", "-q:v", strconv.Itoa(max(2, min(20, (100-q)/5))), "-c:a", "mp2", "-b:a", "224k", "-f", "mpeg"}
	case "ogv":
		a = []string{"-c:v", encoder("libtheora", "theora"), "-q:v", strconv.Itoa(max(0, min(10, q/10)))}
		ogg, _ := soundArgs("ogg", Options{Quality: q, Mono: o.Mono})
		a = append(a, ogg...)
	default:
		return nil, ErrUnsupported
	}
	return a, nil
}

// ffmpegArgs are the arguments between the input and the output of a
// step.
func ffmpegArgs(j *job) ([]string, error) {
	scale := ""
	if s := j.o.Scale; s > 0 && s != 1 {
		scale = fmt.Sprintf("scale=trunc(iw*%g/2)*2:-2", s)
	}
	video := func(a []string) []string {
		if scale != "" {
			a = append(a, "-vf", scale)
		}
		return a
	}
	isVideo := j.from.Kind == Video || j.from.ID == "gif"
	switch {
	case j.to.Kind == Audio:
		a, err := soundArgs(j.to.ID, j.o)
		return append([]string{"-vn", "-map", "0:a:0"}, a...), err
	case isVideo && j.to.Kind == Video:
		a, err := videoArgs(j.to.ID, j.o)
		if j.from.ID == "gif" { // a GIF has no sound, and odd sizes
			a = slices_without(a, "-c:a")
			return append([]string{"-vf", "scale=trunc(iw/2)*2:trunc(ih/2)*2", "-an"}, a...), err
		}
		return video(a), err
	case isVideo && j.to.ID == "gif":
		width := "min(640\\,iw)"
		if s := j.o.Scale; s > 0 && s != 1 {
			width = fmt.Sprintf("iw*%g", s)
		}
		return []string{"-vf", "fps=12,scale=" + width + ":-2:flags=lanczos,split[a][b];[a]palettegen=stats_mode=diff[p];[b][p]paletteuse=dither=bayer", "-loop", "0"}, nil
	case isVideo && (j.to.ID == "png" || j.to.ID == "jpg"):
		// The frame that best stands for the start of the video.
		return video([]string{"-vf", "thumbnail", "-frames:v", "1", "-update", "1"}), nil
	case j.to.ID == "png":
		return []string{"-frames:v", "1", "-update", "1"}, nil
	case j.to.ID == "avif":
		e := encoder("libsvtav1", "libaom-av1")
		return []string{"-c:v", e, "-crf", strconv.Itoa(max(10, min(50, 50-j.o.quality()*35/100))), "-still-picture", "1", "-pix_fmt", "yuv420p", "-frames:v", "1"}, nil
	case j.to.ID == "jxl":
		return []string{"-c:v", "libjxl", "-distance", fmt.Sprintf("%.1f", max(0, float64(100-j.o.quality())/10)), "-frames:v", "1"}, nil
	}
	return nil, ErrUnsupported
}

func slices_without(a []string, flag string) []string {
	var out []string
	for i := 0; i < len(a); i++ {
		if a[i] == flag && i+1 < len(a) {
			i++
			continue
		}
		if strings.HasPrefix(a[i], "-b:a") || a[i] == "-ac" {
			i++
			continue
		}
		out = append(out, a[i])
	}
	return out
}

var (
	durationRe = regexp.MustCompile(`Duration: (\d+):(\d+):(\d+(?:\.\d+)?)`)
	outTimeRe  = regexp.MustCompile(`^out_time_us=(\d+)`)
)

func runFFmpeg(j *job) error {
	path := ffmpegPath()
	if path == "" {
		return errors.New("没有找到 FFmpeg")
	}
	mid, err := ffmpegArgs(j)
	if err != nil {
		return err
	}
	args := append([]string{"-hide_banner", "-nostdin", "-y", "-i", j.src}, mid...)
	args = append(args, "-progress", "pipe:1", "-nostats", j.dst)
	cmd := exec.CommandContext(j.ctx, path, args...)
	hideWindow(cmd)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	var total time.Duration
	sc := bufio.NewScanner(stdout)
	for sc.Scan() {
		if total == 0 {
			if m := durationRe.FindStringSubmatch(stderr.String()); m != nil {
				h, _ := strconv.Atoi(m[1])
				mi, _ := strconv.Atoi(m[2])
				s, _ := strconv.ParseFloat(m[3], 64)
				total = time.Duration((float64(h*3600+mi*60) + s) * float64(time.Second))
			}
		}
		if m := outTimeRe.FindStringSubmatch(sc.Text()); m != nil && total > 0 {
			us, _ := strconv.ParseInt(m[1], 10, 64)
			j.p(float64(time.Duration(us)*time.Microsecond) / float64(total))
		}
	}
	if err := cmd.Wait(); err != nil {
		if j.ctx.Err() != nil {
			return j.ctx.Err()
		}
		return fmt.Errorf("FFmpeg 转换失败：%s", lastLine(stderr.String()))
	}
	if st, err := os.Stat(j.dst); err != nil || st.Size() == 0 {
		return fmt.Errorf("FFmpeg 没有写出文件：%s", lastLine(stderr.String()))
	}
	return nil
}

// lastLine is the last line of FFmpeg's output that says something.
func lastLine(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		l := strings.TrimSpace(lines[i])
		if l != "" && !strings.HasPrefix(l, "[out#") && !strings.HasPrefix(l, "Conversion failed") {
			return l
		}
	}
	return "未知错误"
}

// MediaInfo is a video's or sound's length and, for video, its size, as
// FFmpeg reads them.
func MediaInfo(path string) (d time.Duration, w, h int, err error) {
	p := ffmpegPath()
	if p == "" {
		return 0, 0, 0, errors.New("没有找到 FFmpeg")
	}
	cmd := exec.Command(p, "-hide_banner", "-i", path)
	hideWindow(cmd)
	out, _ := cmd.CombinedOutput()
	if m := durationRe.FindSubmatch(out); m != nil {
		hh, _ := strconv.Atoi(string(m[1]))
		mm, _ := strconv.Atoi(string(m[2]))
		ss, _ := strconv.ParseFloat(string(m[3]), 64)
		d = time.Duration((float64(hh*3600+mm*60) + ss) * float64(time.Second))
	}
	if m := regexp.MustCompile(`Video: .*?, (\d{2,5})x(\d{2,5})`).FindSubmatch(out); m != nil {
		w, _ = strconv.Atoi(string(m[1]))
		h, _ = strconv.Atoi(string(m[2]))
	}
	if d == 0 && w == 0 {
		return 0, 0, 0, errors.New("无法读取媒体信息")
	}
	return d, w, h, nil
}
