package convert

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/go-pdf/fpdf"
	"golang.org/x/image/font/gofont/goregular"
)

// TestMatrix converts a file of every format Formigo can make to every
// format it offers for it, and checks that what it wrote reads back: as
// media FFmpeg decodes, or as a file Formigo itself converts on. It is
// slow, so it runs only when asked:
//
//	FORMIGO_MATRIX=1 go test -run TestMatrix ./internal/convert
func TestMatrix(t *testing.T) {
	if os.Getenv("FORMIGO_MATRIX") == "" {
		t.Skip("set FORMIGO_MATRIX=1 to run")
	}
	ff := needFFmpeg(t)
	dir := t.TempDir()
	seeds := map[string]string{"jxl": filepath.Join("testdata", "seed.jxl")}

	// The sources made directly; every other format comes from converting
	// these, so that each one converted to is also tried as a source.
	put := func(id, name string, data []byte) { seeds[id] = write(t, dir, name, data) }
	put("png", "seed.png", testPicture(true))
	put("svg", "seed.svg", []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="64" height="40"><rect width="64" height="40" fill="#3ccf9e"/><circle cx="32" cy="20" r="12" fill="#ff9447"/></svg>`))
	samples := make([]int16, 22050)
	for i := range samples {
		samples[i] = int16(8000 * ((i/50)%2*2 - 1))
	}
	put("wav", "seed.wav", wav16(22050, 1, samples))
	put("json", "seed.json", []byte(`[{"name":"小地鼠","size":3,"ok":true},{"name":"Formigo","size":7,"ok":false}]`))
	put("md", "seed.md", []byte("# 标题\n\n一段 **加粗** 的文字。\n\n| a | b |\n| - | - |\n| 1 | 2 |\n"))
	put("srt", "seed.srt", []byte("1\n00:00:01,000 --> 00:00:02,500\n你好\n\n2\n00:00:03,000 --> 00:00:04,000\nHello\n"))
	put("ttf", "seed.ttf", goregular.TTF)
	mov := filepath.Join(dir, "seed.mov")
	if out, err := exec.Command(ff, "-hide_banner", "-loglevel", "error", "-f", "lavfi", "-i", "testsrc=duration=1:size=160x120:rate=12",
		"-f", "lavfi", "-i", "sine=frequency=440:duration=1", "-shortest", "-c:v", "mpeg4", "-c:a", "aac", mov).CombinedOutput(); err != nil {
		t.Fatalf("making a clip: %v %s", err, out)
	}
	seeds["mov"] = mov
	pdf := fpdf.New("P", "pt", "A4", "")
	pdf.SetFont("Helvetica", "", 24)
	pdf.AddPage()
	pdf.Text(72, 100, "Hello Formigo")
	seeds["pdf"] = filepath.Join(dir, "seed.pdf")
	if err := pdf.OutputFileAndClose(seeds["pdf"]); err != nil {
		t.Fatal(err)
	}
	// A folder is only a step between archives: the app takes the files
	// inside a folder dropped on it, never the folder itself.
	folder := filepath.Join(dir, "seed-folder")
	os.MkdirAll(filepath.Join(folder, "子目录"), 0o755)
	os.WriteFile(filepath.Join(folder, "a.txt"), []byte("a"), 0o644)
	os.WriteFile(filepath.Join(folder, "子目录", "b.json"), []byte(`{"b":1}`), 0o644)
	// Formats Formigo reads but does not write, from the system's tools
	// where it has them.
	if _, err := exec.LookPath("sips"); err == nil {
		heic := filepath.Join(dir, "seed.heic")
		if exec.Command("sips", "-s", "format", "heic", seeds["png"], "--out", heic).Run() == nil {
			seeds["heic"] = heic
		}
	}
	for id, args := range map[string][]string{
		"tbz": {"-cjf", "seed.tbz", "-C", folder, "."},
		"txz": {"-cJf", "seed.txz", "-C", folder, "."},
	} {
		cmd := exec.Command("tar", args...)
		cmd.Dir = dir
		if cmd.Run() == nil {
			seeds[id] = filepath.Join(dir, args[1])
		}
	}
	if exec.Command("sh", "-c", "gzip -c "+seeds["json"]+" > "+filepath.Join(dir, "seed.json.gz")).Run() == nil {
		seeds["gz"] = filepath.Join(dir, "seed.json.gz")
	}
	if p, err := exec.LookPath("7z"); err == nil {
		z := filepath.Join(dir, "seed.7z")
		if exec.Command(p, "a", "-bd", z, folder).Run() == nil {
			seeds["7z"] = z
		}
	}

	type result struct {
		from, to string
		err      error
	}
	var (
		mu      sync.Mutex
		results []result
	)
	record := func(from, to string, err error) {
		mu.Lock()
		results = append(results, result{from, to, err})
		mu.Unlock()
	}

	// readBack is whether what was written as format to reads.
	readBack := func(out string, to Format) error {
		st, err := os.Stat(out)
		if err != nil {
			return err
		}
		if st.IsDir() {
			entries, _ := os.ReadDir(out)
			if len(entries) == 0 {
				return fmt.Errorf("empty folder")
			}
			return nil
		}
		if st.Size() == 0 {
			return fmt.Errorf("empty file")
		}
		switch to.Kind {
		case Video, Audio:
			if b, err := exec.Command(ff, "-v", "error", "-i", out, "-f", "null", "-").CombinedOutput(); err != nil || len(strings.TrimSpace(string(b))) > 0 {
				return fmt.Errorf("FFmpeg cannot decode it: %v %s", err, firstLine(string(b)))
			}
			return nil
		}
		if to.ID == "gif" || to.Kind == Image && slices.Contains(pictureIn, to.ID) {
			if _, _, err := readImage(out); err != nil {
				return fmt.Errorf("does not read back: %w", err)
			}
			return nil
		}
		// Anything else Formigo reads is converted on, to its first target.
		if ts := Targets(to.ID); len(ts) > 0 {
			back := filepath.Join(filepath.Dir(out), "back")
			os.MkdirAll(back, 0o755)
			next, err := Convert(context.Background(), out, back, ts[0], Options{}, nil)
			if err != nil {
				return fmt.Errorf("does not read back as %s: %w", ts[0].ID, err)
			}
			os.RemoveAll(next)
		}
		return nil
	}

	// Seeds first: every format a seed converts to becomes a seed itself.
	tried := map[string]bool{}
	for {
		var todo []string
		for id := range seeds {
			if !tried[id] {
				todo = append(todo, id)
			}
		}
		if len(todo) == 0 {
			break
		}
		sort.Strings(todo)
		var wg sync.WaitGroup
		sem := make(chan struct{}, 6)
		var smu sync.Mutex
		for _, from := range todo {
			tried[from] = true
			smu.Lock()
			src := seeds[from]
			smu.Unlock()
			for _, to := range Targets(from) {
				wg.Add(1)
				go func() {
					defer wg.Done()
					sem <- struct{}{}
					defer func() { <-sem }()
					outDir := filepath.Join(dir, "out", from+"-to-"+to.ID)
					os.MkdirAll(outDir, 0o755)
					out, err := Convert(context.Background(), src, outDir, to, Options{Quality: 70}, nil)
					if err == nil {
						err = readBack(out, to)
					}
					record(from, to.ID, err)
					if err == nil && to.ID != "folder" {
						smu.Lock()
						if _, ok := seeds[to.ID]; !ok {
							if st, serr := os.Stat(out); serr == nil && (!st.IsDir() || to.ID == "folder") {
								seeds[to.ID] = out
							}
						}
						smu.Unlock()
					}
				}()
			}
		}
		wg.Wait()
	}

	sort.Slice(results, func(i, j int) bool {
		if results[i].from != results[j].from {
			return results[i].from < results[j].from
		}
		return results[i].to < results[j].to
	})
	failed := 0
	for _, r := range results {
		// A web font unwraps only to the format its outlines are.
		if r.err != nil && strings.Contains(r.err.Error(), "轮廓") && (r.from == "woff" || r.from == "woff2") {
			continue
		}
		if r.err != nil {
			failed++
			t.Errorf("%s → %s: %v", r.from, r.to, r.err)
		}
	}
	var never []string
	for _, f := range Formats() {
		if _, ok := seeds[f.ID]; !ok && len(Targets(f.ID)) > 0 {
			never = append(never, f.ID)
		}
	}
	t.Logf("%d conversions from %d source formats, %d failed; sources not made here: %s",
		len(results), len(tried), failed, strings.Join(never, " "))
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
