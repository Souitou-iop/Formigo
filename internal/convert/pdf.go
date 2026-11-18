package convert

import (
	"fmt"
	"image"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/klippa-app/go-pdfium"
	"github.com/klippa-app/go-pdfium/requests"
	"github.com/klippa-app/go-pdfium/webassembly"
)

// PDFium, compiled to WebAssembly and run in Go, draws a PDF's pages and
// reads their text, on every system, with nothing to install.

func init() {
	register(
		&edge{engine: "pdfium", from: []string{"pdf"}, to: []string{"png", "jpg"}, cost: 2, final: true, run: pdfPages},
		&edge{engine: "pdfium", from: []string{"pdf"}, to: []string{"txt"}, cost: 2, first: true, run: pdfText},
	)
}

var pdfiumPool struct {
	once sync.Once
	pool pdfium.Pool
	err  error
}

// withPDF opens the PDF at path in an instance of PDFium.
func withPDF(path string, fn func(p pdfium.Pdfium, doc *requests.Page, pages int) error) error {
	pdfiumPool.once.Do(func() {
		pdfiumPool.pool, pdfiumPool.err = webassembly.Init(webassembly.Config{MinIdle: 0, MaxIdle: 1, MaxTotal: 2})
	})
	if pdfiumPool.err != nil {
		return fmt.Errorf("PDF 引擎无法启动：%w", pdfiumPool.err)
	}
	inst, err := pdfiumPool.pool.GetInstance(30 * time.Second)
	if err != nil {
		return err
	}
	defer inst.Close()
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	doc, err := inst.OpenDocument(&requests.OpenDocument{File: &data})
	if err != nil {
		return fmt.Errorf("无法打开 PDF：%w", err)
	}
	defer inst.FPDF_CloseDocument(&requests.FPDF_CloseDocument{Document: doc.Document})
	count, err := inst.FPDF_GetPageCount(&requests.FPDF_GetPageCount{Document: doc.Document})
	if err != nil {
		return err
	}
	if count.PageCount == 0 {
		return fmt.Errorf("PDF 没有页面")
	}
	page := &requests.Page{ByIndex: &requests.PageByIndex{Document: doc.Document}}
	return fn(inst, page, count.PageCount)
}

// pdfPages draws each page as a picture: one file for a page, a folder of
// them for more.
func pdfPages(j *job) error {
	dpi := 150
	if s := j.o.Scale; s > 0 && s != 1 {
		dpi = max(36, int(150*s))
	}
	return withPDF(j.src, func(p pdfium.Pdfium, page *requests.Page, n int) error {
		dir := ""
		if n > 1 {
			dir = strings.TrimSuffix(j.dst, filepath.Ext(j.dst))
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return err
			}
		}
		for i := range n {
			if err := j.ctx.Err(); err != nil {
				return err
			}
			page.ByIndex.Index = i
			r, err := p.RenderPageInDPI(&requests.RenderPageInDPI{Page: *page, DPI: dpi})
			if err != nil {
				return fmt.Errorf("无法绘制第 %d 页：%w", i+1, err)
			}
			var img image.Image = r.Result.Image
			out := j.dst
			if dir != "" {
				out = filepath.Join(dir, fmt.Sprintf("%s %0*d.%s", j.name, len(fmt.Sprint(n)), i+1, j.to.exts[0]))
			}
			f, err := os.Create(out)
			if err == nil {
				err = encodeImage(j.ctx, f, img, j.to.ID, j.o)
				if cerr := f.Close(); err == nil {
					err = cerr
				}
			}
			if r.CleanupFunc != nil {
				r.CleanupFunc()
			}
			if err != nil {
				return err
			}
			j.p(float64(i+1) / float64(n))
		}
		if dir != "" {
			j.dst = dir
		}
		return nil
	})
}

// pdfText is a PDF's text, its pages apart.
func pdfText(j *job) error {
	var b strings.Builder
	err := withPDF(j.src, func(p pdfium.Pdfium, page *requests.Page, n int) error {
		for i := range n {
			page.ByIndex.Index = i
			t, err := p.GetPageText(&requests.GetPageText{Page: *page})
			if err != nil {
				return err
			}
			if i > 0 {
				b.WriteString("\n\n")
			}
			b.WriteString(strings.TrimSpace(strings.ReplaceAll(t.Text, "\r\n", "\n")))
			j.p(float64(i+1) / float64(n))
		}
		return nil
	})
	if err != nil {
		return err
	}
	return os.WriteFile(j.dst, []byte(b.String()+"\n"), 0o644)
}
