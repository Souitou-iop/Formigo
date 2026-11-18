package convert

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

// LibreOffice lays out office documents where Go's own code does not: old
// Word, Excel and PowerPoint files, OpenDocument, and any of them as PDF.
// It is the user's to install; Formigo finds it.

func init() {
	have := func() bool { return officePath() != "" }
	register(
		&edge{engine: "office", from: []string{"doc", "docx", "odt", "rtf", "html", "txt"}, to: []string{"pdf", "docx", "odt", "rtf"}, cost: 3, ok: have, run: runOffice},
		&edge{engine: "office", from: []string{"ppt", "pptx", "odp"}, to: []string{"pdf", "pptx", "odp"}, cost: 3, ok: have, run: runOffice},
		&edge{engine: "office", from: []string{"xls", "xlsx", "ods", "csv"}, to: []string{"pdf", "xlsx", "ods", "csv"}, cost: 3, ok: have, run: runOffice},
	)
}

var office struct {
	sync.Mutex
	path   string
	looked bool
	// run lets one conversion at a time use LibreOffice's profile.
	run sync.Mutex
}

// RefreshEngines looks again for the engines on the computer, once the
// user may have installed one.
func RefreshEngines() {
	office.Lock()
	office.looked = false
	office.Unlock()
	replan()
}

// OfficeDownload is where LibreOffice is downloaded from.
const OfficeDownload = "https://www.libreoffice.org/download/download-libreoffice/"

// officePath is LibreOffice's soffice, "" when it is not installed.
func officePath() string {
	office.Lock()
	defer office.Unlock()
	if office.looked {
		return office.path
	}
	office.looked = true
	office.path = ""
	candidates := []string{os.Getenv("FORMIGO_SOFFICE")}
	for _, name := range []string{"soffice", "libreoffice"} {
		if p, err := exec.LookPath(name); err == nil {
			candidates = append(candidates, p)
		}
	}
	switch runtime.GOOS {
	case "darwin":
		home, _ := os.UserHomeDir()
		candidates = append(candidates, "/Applications/LibreOffice.app/Contents/MacOS/soffice",
			filepath.Join(home, "Applications/LibreOffice.app/Contents/MacOS/soffice"))
	case "windows":
		for _, env := range []string{"ProgramFiles", "ProgramFiles(x86)"} {
			if d := os.Getenv(env); d != "" {
				candidates = append(candidates, filepath.Join(d, "LibreOffice", "program", "soffice.exe"))
			}
		}
	default:
		candidates = append(candidates, "/usr/bin/soffice", "/usr/lib/libreoffice/program/soffice",
			"/opt/libreoffice/program/soffice", "/snap/bin/libreoffice")
	}
	for _, c := range candidates {
		if st, err := os.Stat(c); c != "" && err == nil && !st.IsDir() {
			office.path = c
			break
		}
	}
	return office.path
}

// officeFilters are LibreOffice's export filters, by format.
var officeFilters = map[string]string{
	"pdf":  "pdf",
	"docx": "docx:MS Word 2007 XML",
	"odt":  "odt",
	"rtf":  "rtf",
	"xlsx": "xlsx:Calc MS Excel 2007 XML",
	"ods":  "ods",
	"csv":  "csv:Text - txt - csv (StarCalc):44,34,76,1",
	"pptx": "pptx:Impress MS PowerPoint 2007 XML",
	"odp":  "odp",
}

func runOffice(j *job) error {
	soffice := officePath()
	if soffice == "" {
		return errors.New("需要安装 LibreOffice")
	}
	filter, ok := officeFilters[j.to.ID]
	if !ok {
		return ErrUnsupported
	}
	out, err := os.MkdirTemp(filepath.Dir(j.dst), "office-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(out)
	// LibreOffice takes the name of what it writes from what it reads, and
	// its type from its extension: a copy, named plainly.
	in := filepath.Join(out, "in."+j.from.exts[0])
	data, err := os.ReadFile(j.src)
	if err != nil {
		return err
	}
	if err := os.WriteFile(in, data, 0o600); err != nil {
		return err
	}
	profile := filepath.Join(os.TempDir(), fmt.Sprintf("formigo-office-%d", os.Getuid()))
	office.run.Lock()
	defer office.run.Unlock()
	j.p(0.1)
	cmd := exec.CommandContext(j.ctx, soffice, "--headless", "--norestore", "--nologo", "--nodefault",
		"-env:UserInstallation="+(&url.URL{Scheme: "file", Path: filepath.ToSlash(profile)}).String(),
		"--convert-to", filter, "--outdir", out, in)
	hideWindow(cmd)
	msg, err := cmd.CombinedOutput()
	if err != nil {
		if j.ctx.Err() != nil {
			return j.ctx.Err()
		}
		return fmt.Errorf("LibreOffice 转换失败：%s", strings.TrimSpace(string(msg)))
	}
	written := filepath.Join(out, "in."+j.to.exts[0])
	if _, err := os.Stat(written); err != nil {
		return fmt.Errorf("LibreOffice 没有写出文件：%s", strings.TrimSpace(string(msg)))
	}
	j.p(1)
	return os.Rename(written, j.dst)
}
