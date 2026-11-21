// Formigo converts files between formats on the device: pictures, sound,
// structured data and documents, in a window of MyGo's native UI.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"

	"formigo/internal/convert"
	"formigo/internal/heif"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/updater"
	"github.com/egoist/mygo/plugins/updater/native"
	"github.com/egoist/mygo/ui"
)

type status int

const (
	waiting status = iota
	working
	done
	failed
)

// item is a file in the queue.
type item struct {
	id    int
	path  string
	name  string
	kind  convert.Kind
	from  string // its format's ID
	size  int64
	meta  string // dimensions, duration or a count, once read
	batch int    // its place among the files added with it, to stagger them
	// target is the format this file alone goes to, chosen on its row;
	// "" follows its kind's.
	target string

	status   status
	progress float64
	out      string
	outSize  int64
	err      string
	// already is set when a document is in the format chosen, which
	// leaves nothing to convert.
	already bool
	// Runs of the row's one-shot animations: the flash as it finishes and
	// the shake as it fails.
	doneRun, failRun uint64
}

// formigo is the app's state.
type formigo struct {
	win    *mygo.Window
	items  []*item
	nextID int

	targets map[convert.Kind]string
	quality float64
	scale   string
	depth   int // 0: 16-bit, 1: 24-bit
	mono    bool
	indent  int // 0: 2 spaces, 1: 4 spaces, 2: compact
	where   int // 0: next to the file, 1: a folder chosen
	outDir  string

	running  bool
	cancel   context.CancelFunc
	finished int    // files done in the last run
	runs     uint64 // runs started, which replays the button's sheen
	cheers   uint64 // runs that converted something, which the gopher hops for

	// The easter egg: five quick clicks on the title bar's gopher play the
	// launch animation again.
	taps     int       // quick clicks so far
	lastTap  time.Time // when the last one was
	intro    uint64    // replays of the launch animation
	encoreAt time.Time // when the replay over the queue started; zero when none
	ready    uint64    // times the button became ready

	// The settings: what the user chose, where they are kept, and what
	// was written there last.
	prefs     settings
	prefsPath string
	saved     []byte
	// The settings window, the page it shows, and its
	// question before going back to the defaults.
	settingsWin  *mygo.Window
	settingsPage string
	confirmReset bool
	// darkNow is whether the frame being built is dark.
	darkNow bool

	// boops counts taps on the gopher, which make it hop; boopAt is when
	// the last one was.
	boops   uint64
	boopAt  time.Time
	cheerAt time.Time // when the last run that converted something ended
	// The other gophers hop on their own taps: the one beside the run's
	// button and the settings' preview.
	companionAt, previewAt time.Time

	// builds counts the builds of the main view: motion that only paints
	// leaves it alone.
	builds int

	// update runs f on the main thread and draws a frame: the window's
	// Update, or a plain call in tests.
	update func(f func())
}

var scales = []string{"原始尺寸", "75%", "50%", "25%"}

func newApp() *formigo {
	a := &formigo{
		targets: map[convert.Kind]string{},
		quality: 85,
		scale:   scales[0],
	}
	for _, k := range convert.Kinds {
		a.targets[k] = convert.DefaultTarget(k)
	}
	a.apply(defaults())
	a.settingsPage = "look"
	a.update = func(f func()) { f() }
	return a
}

// add queues the files at paths, the files inside directories too, skipping
// what Formigo cannot read and what is already queued.
func (a *formigo) add(paths []string) (added, skipped int) {
	var files []string
	for _, p := range paths {
		st, err := os.Stat(p)
		if err != nil {
			continue
		}
		if !st.IsDir() {
			files = append(files, p)
			continue
		}
		filepath.WalkDir(p, func(f string, d os.DirEntry, err error) error {
			if err == nil && !d.IsDir() && !strings.HasPrefix(d.Name(), ".") {
				files = append(files, f)
			}
			return nil
		})
	}
	batch := 0
	for _, p := range files {
		kind, from, ok := convert.Detect(p)
		if !ok || slices.ContainsFunc(a.items, func(it *item) bool { return it.path == p }) {
			skipped++
			continue
		}
		var size int64
		if st, err := os.Stat(p); err == nil {
			size = st.Size()
		}
		a.nextID++
		it := &item{id: a.nextID, path: p, name: filepath.Base(p), kind: kind, from: from, size: size, batch: batch}
		batch++
		a.items = append(a.items, it)
		added++
		go a.describe(it)
	}
	if added > 0 && !a.running {
		a.ready++
	}
	return added, skipped
}

// targetOf is the format a file goes to: its own, its kind's, or, where
// it cannot become its kind's, the first it can. Its own format is
// nothing to convert.
func (a *formigo) targetOf(it *item) string {
	if it.target != "" && convert.CanConvert(it.from, it.target) {
		return it.target
	}
	if kt := a.kindTarget(it.kind); kt == it.from || convert.CanConvert(it.from, kt) {
		return kt
	}
	if ts := convert.Targets(it.from); len(ts) > 0 {
		return ts[0].ID
	}
	return it.from
}

// kindTargets are the formats a kind's files queued can become, all of
// them, their own among them; every one any of them can, where they share
// none. Its own kind's come first.
func (a *formigo) kindTargets(k convert.Kind) []convert.Format {
	var common, union map[string]bool
	n := 0
	for _, it := range a.items {
		if it.kind != k {
			continue
		}
		mine := map[string]bool{it.from: true}
		for _, f := range convert.Targets(it.from) {
			mine[f.ID] = true
		}
		if union == nil {
			union, common = map[string]bool{}, mine
		} else {
			for id := range common {
				if !mine[id] {
					delete(common, id)
				}
			}
		}
		for id := range mine {
			union[id] = true
		}
		n++
	}
	if n == 0 {
		return convert.KindTargets(k)
	}
	pick := common
	if len(common) == 0 {
		pick = union
	}
	return orderedTargets(pick, k)
}

// orderedTargets are the formats of ids, those of kind k first, in the
// order they are offered.
func orderedTargets(ids map[string]bool, k convert.Kind) []convert.Format {
	var same, other []convert.Format
	for _, f := range convert.Formats() {
		switch {
		case !ids[f.ID]:
		case f.Kind == k:
			same = append(same, f)
		default:
			other = append(other, f)
		}
	}
	return append(same, other...)
}

// kindTarget is the format a kind's files go to: the one chosen, where
// they can become it.
func (a *formigo) kindTarget(k convert.Kind) string {
	ts := a.kindTargets(k)
	for _, f := range ts {
		if f.ID == a.targets[k] {
			return f.ID
		}
	}
	if len(ts) > 0 {
		return ts[0].ID
	}
	return a.targets[k]
}

// commonTargets are the formats every file queued can become, when files
// of more than one kind are: a batch's one choice for all of them.
func (a *formigo) commonTargets() []convert.Format {
	if len(a.kinds()) < 2 {
		return nil
	}
	var common map[string]bool
	for _, it := range a.items {
		mine := map[string]bool{it.from: true}
		for _, f := range convert.Targets(it.from) {
			mine[f.ID] = true
		}
		if common == nil {
			common = mine
			continue
		}
		for id := range common {
			if !mine[id] {
				delete(common, id)
			}
		}
	}
	return orderedTargets(common, convert.Doc)
}

// allTo has every file queued go to the format id.
func (a *formigo) allTo(id string) {
	for _, it := range a.items {
		it.target = id
	}
	a.retarget(func(*item) bool { return true })
}

// allTarget is the format every file queued goes to, "" when they differ.
func (a *formigo) allTarget() string {
	t := ""
	for i, it := range a.items {
		if i == 0 {
			t = a.targetOf(it)
		} else if a.targetOf(it) != t {
			return ""
		}
	}
	return t
}

// targetsOf are the formats the files of a kind go to, each once.
func (a *formigo) targetsOf(k convert.Kind) []string {
	var out []string
	for _, it := range a.items {
		if t := a.targetOf(it); it.kind == k && !slices.Contains(out, t) {
			out = append(out, t)
		}
	}
	return out
}

// retarget has the files converted already convert again, to what they
// go to now.
func (a *formigo) retarget(match func(*item) bool) {
	if a.running {
		return
	}
	for _, it := range a.items {
		if match(it) && it.status == done {
			it.status = waiting
		}
	}
	a.ready++
}

// describe reads what a file's row says of it, off the main thread.
func (a *formigo) describe(it *item) {
	var meta string
	switch it.kind {
	case convert.Image:
		if w, h, err := convert.ImageInfo(it.path); err == nil {
			meta = fmt.Sprintf("%d×%d", w, h)
		}
	case convert.Video:
		if d, w, h, err := convert.MediaInfo(it.path); err == nil {
			meta = clock(d)
			if w > 0 {
				meta += fmt.Sprintf(" · %d×%d", w, h)
			}
		}
	case convert.Audio:
		if d, _, _, err := convert.MediaInfo(it.path); err == nil && it.from != "wav" && it.from != "flac" && it.from != "mp3" && it.from != "ogg" {
			meta = clock(d)
		} else if d, ch, rate, err := convert.AudioInfo(it.path); err == nil {
			parts := []string{}
			if d > 0 {
				parts = append(parts, clock(d))
			}
			parts = append(parts, map[bool]string{true: "单声道", false: "立体声"}[ch == 1])
			parts = append(parts, fmt.Sprintf("%.1f kHz", float64(rate)/1000))
			meta = strings.Join(parts, " · ")
		}
	case convert.Data:
		meta = convert.DataSummary(it.path)
	case convert.Doc:
		if it.from == "md" || it.from == "txt" || it.from == "html" {
			meta = convert.TextSummary(it.path)
		}
	}
	a.update(func() { it.meta = meta })
}

// tapped counts a click on the title bar's gopher, and replays the launch
// animation on the fifth in quick succession.
func (a *formigo) tapped(now time.Time) {
	if now.Sub(a.lastTap) > 650*time.Millisecond {
		a.taps = 0
	}
	a.taps++
	a.lastTap = now
	if a.taps < 5 {
		return
	}
	a.taps = 0
	a.intro++
	if len(a.items) > 0 {
		a.encoreAt = now
	}
}

// boop makes the hero's gopher hop and kick the documents round, and runs
// then once the hop has peaked, so a tap that opens a picker shows the hop
// before the picker covers it.
func (a *formigo) boop(now time.Time, then func()) {
	a.boops++
	a.boopAt = now
	if then == nil {
		return
	}
	delay := 380 * time.Millisecond
	if a.prefs.Motion == "off" {
		delay = 0
	}
	time.AfterFunc(delay, func() { a.update(then) })
}

func (a *formigo) remove(id int) {
	a.items = slices.DeleteFunc(a.items, func(it *item) bool { return it.id == id && it.status != working })
}

func (a *formigo) clear() {
	if a.running {
		return
	}
	a.items = nil
}

// kinds returns the kinds of the files queued, in the interface's order.
func (a *formigo) kinds() []convert.Kind {
	var ks []convert.Kind
	for _, k := range convert.Kinds {
		if slices.ContainsFunc(a.items, func(it *item) bool { return it.kind == k }) {
			ks = append(ks, k)
		}
	}
	return ks
}

func (a *formigo) count(k convert.Kind) int {
	n := 0
	for _, it := range a.items {
		if it.kind == k {
			n++
		}
	}
	return n
}

// pending are the files the next run converts.
func (a *formigo) pending() []*item {
	var out []*item
	for _, it := range a.items {
		if it.status == waiting || it.status == failed {
			out = append(out, it)
		}
	}
	return out
}

func (a *formigo) options() convert.Options {
	o := convert.Options{Quality: int(a.quality), Indent: []int{2, 4, -1}[a.indent], Mono: a.mono}
	if a.depth == 1 {
		o.BitDepth = 24
	}
	switch a.scale {
	case "75%":
		o.Scale = 0.75
	case "50%":
		o.Scale = 0.5
	case "25%":
		o.Scale = 0.25
	}
	return o
}

// outputDir is where converted files go: next to each file, or the folder
// chosen.
func (a *formigo) outputDir(it *item) string {
	if a.where == 1 && a.outDir != "" {
		return a.outDir
	}
	return filepath.Dir(it.path)
}

// start converts the files waiting, a few at a time.
func (a *formigo) start() {
	if a.running {
		return
	}
	// Converted again, with what is chosen now.
	if len(a.pending()) == 0 {
		for _, it := range a.items {
			it.status, it.progress, it.out, it.err = waiting, 0, "", ""
		}
	}
	jobs := a.pending()
	if len(jobs) == 0 {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.running, a.cancel, a.finished = true, cancel, 0
	a.runs++
	opts := a.options()
	type job struct {
		it   *item
		to   convert.Format
		dir  string
		opts convert.Options
	}
	queue := make(chan job, len(jobs))
	for _, it := range jobs {
		to, _ := convert.Lookup(a.targetOf(it))
		it.status, it.progress, it.err, it.already = waiting, 0, "", false
		if it.from == to.ID && it.kind != convert.Image && it.kind != convert.Video && it.kind != convert.Audio && it.kind != convert.Data {
			// A document in its own format would come out the same; a
			// picture, a clip or data comes out resized, recompressed or
			// tidied.
			it.status, it.progress, it.already = done, 1, true
			a.finished++
			continue
		}
		o := opts
		o.Name = strings.TrimSuffix(it.name, filepath.Ext(it.name)) + cleanSuffix(a.prefs.Suffix)
		queue <- job{it, to, a.outputDir(it), o}
	}
	jobs = slices.DeleteFunc(jobs, func(it *item) bool { return it.already })
	if len(jobs) == 0 {
		a.running, a.cancel = false, nil
		cancel()
		return
	}
	close(queue)
	var wg sync.WaitGroup
	for range min(len(jobs), max(1, a.prefs.Workers)) {
		wg.Go(func() {
			for j := range queue {
				a.update(func() { j.it.status = working })
				var last time.Time
				out, err := convert.Convert(ctx, j.it.path, j.dir, j.to, j.opts, func(d float64) {
					// A frame per tick of progress is plenty.
					if now := time.Now(); now.Sub(last) > 30*time.Millisecond || d >= 1 {
						last = now
						a.update(func() { j.it.progress = d })
					}
				})
				var size int64
				if st, serr := os.Stat(out); err == nil && serr == nil {
					size = st.Size()
				}
				a.update(func() {
					it := j.it
					switch {
					case err == nil:
						it.status, it.progress, it.out, it.outSize = done, 1, out, size
						it.doneRun++
						a.finished++
					case ctx.Err() != nil:
						it.status, it.progress = waiting, 0
					default:
						it.status, it.err = failed, err.Error()
						it.failRun++
					}
				})
			}
		})
	}
	go func() {
		wg.Wait()
		a.update(a.stopped)
	}()
}

// stopped is called on the main thread once a run ends.
func (a *formigo) stopped() {
	a.running = false
	if a.cancel != nil {
		a.cancel()
	}
	if a.finished == 0 {
		return
	}
	a.cheers++
	a.cheerAt = time.Now()
	var last *item
	for _, it := range a.items {
		if it.status == done && it.out != "" {
			last = it
		}
	}
	// Told, if the user asked, when the window is not in front. Showing
	// waits for the system (and its permission, the first time),
	// so it goes off the main thread.
	if a.prefs.Notify && a.win != nil && !a.win.IsFocused() {
		n := mygo.NewNotification(mygo.NotificationOptions{
			Title: "转换完成",
			Body:  fmt.Sprintf("%d 个文件已转换", a.finished),
		})
		go n.Show()
	}
	if a.prefs.Reveal && last != nil {
		a.reveal(last)
	}
}

// cleanSuffix keeps what a file's name may hold of the suffix the user
// typed.
func cleanSuffix(s string) string {
	return strings.Map(func(r rune) rune {
		if strings.ContainsRune(`/\:*?"<>|`, r) || r < 0x20 {
			return -1
		}
		return r
	}, strings.TrimSpace(s))
}

func (a *formigo) stop() {
	if a.cancel != nil {
		a.cancel()
	}
}

// chooseFiles shows the system's picker, off the main thread.
func (a *formigo) chooseFiles() {
	go func() {
		var exts []string
		for _, k := range convert.Kinds {
			exts = append(exts, convert.Supported(k)...)
		}
		slices.Sort(exts)
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Parent:   a.win,
			Title:    "选择要转换的文件",
			Multiple: true,
			Filters:  []mygo.FileFilter{{Name: "Formigo 支持的文件", Extensions: exts}},
		})
		if err != nil || len(paths) == 0 {
			return
		}
		a.update(func() { a.add(paths) })
	}()
}

// chooseFolder asks where converted files go.
func (a *formigo) chooseFolder() {
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Parent:            a.win,
			Title:             "转换后的文件存放在",
			ButtonLabel:       "选择",
			Directory:         true,
			CreateDirectories: true,
		})
		a.update(func() {
			if err == nil && len(paths) == 1 {
				a.outDir = paths[0]
			}
			if a.outDir == "" {
				a.where = 0
			}
		})
	}()
}

// outputs are the files the last runs wrote.
func (a *formigo) outputs() []string {
	var out []string
	for _, it := range a.items {
		if it.status == done && it.out != "" {
			out = append(out, it.out)
		}
	}
	return out
}

// openSettings shows the settings window, a new one or the one open.
func (a *formigo) openSettings() {
	if a.settingsWin != nil {
		a.settingsWin.Focus()
		return
	}
	w := mygo.NewWindow(mygo.WindowOptions{
		Title:         "Formigo 设置",
		Width:         720,
		Height:        560,
		MinWidth:      600,
		MinHeight:     460,
		StateKey:      "settings",
		TitleBarStyle: mygo.TitleBarHidden,
		Content:       ui.View(a.settingsView),
	})
	a.settingsWin = w
	w.OnClosed(func() { a.settingsWin = nil })
}

// refresh redraws the open windows: the About page of the settings shows
// what the updater has found, and it may have found it while either was
// closed or in the background.
func (a *formigo) refresh() {
	a.update(func() {})
	if a.settingsWin != nil {
		a.settingsWin.Update(func() {})
	}
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

// revealLabel is what the system calls showing a file in its folder.
var revealLabel = map[string]string{
	"darwin": "在访达中显示", "windows": "在资源管理器中显示",
}[runtime.GOOS]

func init() {
	if revealLabel == "" {
		revealLabel = "打开所在文件夹"
	}
}

func (a *formigo) reveal(it *item) {
	if it.out != "" {
		mygo.Shell.ShowItemInFolder(it.out)
	}
}

func clock(d time.Duration) string {
	s := int(d.Round(time.Second).Seconds())
	if s >= 3600 {
		return fmt.Sprintf("%d:%02d:%02d", s/3600, s/60%60, s%60)
	}
	return fmt.Sprintf("%d:%02d", s/60, s%60)
}

func bytesText(n int64) string {
	switch {
	case n < 1000:
		return fmt.Sprintf("%d B", n)
	case n < 1000*1000:
		return fmt.Sprintf("%.0f KB", float64(n)/1000)
	case n < 1000*1000*1000:
		return fmt.Sprintf("%.1f MB", float64(n)/1e6)
	}
	return fmt.Sprintf("%.2f GB", float64(n)/1e9)
}

func main() {
	// HEIC and HEIF pictures, read with the system's decoder where there
	// is one: ImageIO on macOS.
	if d, err := heif.NewDecoder(); err == nil {
		convert.RegisterImageDecoder([]string{"heic", "heif", "avif"}, d.ToPNG)
	}
	// The FFmpeg the app ships with, for video and sound.
	if dir, err := mygo.App.Path(mygo.PathResources); err == nil {
		name := "ffmpeg"
		if runtime.GOOS == "windows" {
			name += ".exe"
		}
		if p := filepath.Join(dir, "bin", name); fileExists(p) {
			convert.SetFFmpeg(p)
		}
	}
	// The updater draws its window in the app's own UI and checks in the
	// background once a day; the app speaks Chinese, so does it.
	mygo.Use(native.New(updater.Options{Language: "zh-Hans"}))
	a := newApp()
	if dir, err := mygo.App.Path(mygo.PathUserData); err == nil {
		a.prefsPath = filepath.Join(dir, "settings.json")
	}
	a.load()
	// What the updater finds shows on the settings' About page, in either
	// window.
	updater.OnChange(func() { a.refresh() })
	if runtime.GOOS == "darwin" {
		mygo.App.SetMenu(a.menu())
	}
	// Files opened with the app (dropped on its Dock icon, passed on the
	// command line) join the queue, those of its launch once its window is
	// there.
	var opened struct {
		sync.Mutex
		paths []string
		ready bool
	}
	flush := func() {
		opened.Lock()
		paths := opened.paths
		opened.paths = nil
		opened.Unlock()
		if len(paths) > 0 {
			a.update(func() { a.add(paths) })
		}
	}
	mygo.App.OnOpenFile(func(path string) {
		opened.Lock()
		opened.paths = append(opened.paths, path)
		ready := opened.ready
		opened.Unlock()
		if ready {
			flush()
		}
	})
	mygo.App.WhenReady(func() {
		a.win = mygo.NewWindow(mygo.WindowOptions{
			Title:         "Formigo",
			Width:         1040,
			Height:        680,
			MinWidth:      720,
			MinHeight:     520,
			StateKey:      "main",
			TitleBarStyle: mygo.TitleBarHidden,
			Content:       ui.View(a.view),
		})
		win := a.win
		a.update = func(f func()) { win.Update(f) }
		// Files given on the command line.
		if len(os.Args) > 1 {
			a.add(os.Args[1:])
		}
	})
	// After the window is made, in the order they were registered.
	mygo.App.WhenReady(func() {
		opened.Lock()
		opened.ready = true
		opened.Unlock()
		flush()
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}

// menu is the menu bar of macOS: the app's menu with the settings, a File
// menu to add files and convert, and the usual Edit and Window menus.
func (a *formigo) menu() *mygo.Menu {
	onMain := func(f func()) func(*mygo.MenuItem, *mygo.Window) {
		return func(*mygo.MenuItem, *mygo.Window) { a.update(f) }
	}
	return mygo.NewMenu([]*mygo.MenuItem{
		{Label: "Formigo", Submenu: []*mygo.MenuItem{
			{Role: mygo.RoleAbout, Label: "关于 Formigo"},
			updater.MenuItem(),
			mygo.Separator(),
			{Label: "设置…", Accelerator: "CmdOrCtrl+,", Click: onMain(a.openSettings)},
			mygo.Separator(),
			{Role: mygo.RoleServices},
			mygo.Separator(),
			{Role: mygo.RoleHide},
			{Role: mygo.RoleHideOthers},
			{Role: mygo.RoleUnhide},
			mygo.Separator(),
			{Role: mygo.RoleQuit},
		}},
		{Label: "文件", Submenu: []*mygo.MenuItem{
			{Label: "添加文件…", Accelerator: "CmdOrCtrl+O", Click: onMain(a.chooseFiles)},
			{Label: "开始转换", Accelerator: "CmdOrCtrl+Enter", Click: onMain(a.start)},
			mygo.Separator(),
			{Label: "清空列表", Click: onMain(a.clear)},
			mygo.Separator(),
			{Role: mygo.RoleClose},
		}},
		{Role: mygo.RoleEditMenu},
		{Role: mygo.RoleWindowMenu},
	})
}
