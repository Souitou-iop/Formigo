package convert

import (
	"archive/tar"
	"archive/zip"
	"compress/bzip2"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bodgit/sevenzip"
	"github.com/nwaples/rardecode/v2"
	"github.com/ulikunitz/xz"
	"golang.org/x/text/encoding/simplifiedchinese"
)

// Archives open into a folder, and a folder packs into an archive; one
// archive becomes another through the folder.

func init() {
	register(
		&edge{engine: "archive", from: []string{"zip", "tgz", "tar", "7z", "rar", "tbz", "txz", "gz"}, to: []string{"folder"}, cost: 1, run: unpack},
		&edge{engine: "archive", from: []string{"folder"}, to: []string{"zip", "tgz", "tar"}, cost: 1, run: pack},
	)
}

// maxUnpacked is the most an archive may hold, against archives that
// expand without end.
const maxUnpacked = 20 << 30

// unpack opens an archive into the folder job.dst.
func unpack(j *job) error {
	if err := os.MkdirAll(j.dst, 0o755); err != nil {
		return err
	}
	var total int64
	put := func(name string, mode fs.FileMode, dir bool, r io.Reader) error {
		if err := j.ctx.Err(); err != nil {
			return err
		}
		name = filepath.FromSlash(strings.TrimLeft(strings.ReplaceAll(name, `\`, "/"), "/"))
		dst := filepath.Join(j.dst, name)
		// A name may not lead out of the folder.
		if rel, err := filepath.Rel(j.dst, dst); err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return fmt.Errorf("压缩包里的路径不安全：%s", name)
		}
		if dir {
			return os.MkdirAll(dst, 0o755)
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		f, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode.Perm()|0o600)
		if err != nil {
			return err
		}
		n, err := io.Copy(f, io.LimitReader(r, maxUnpacked-total+1))
		total += n
		if cerr := f.Close(); err == nil {
			err = cerr
		}
		if total > maxUnpacked {
			return errors.New("压缩包解开后太大")
		}
		return err
	}
	untar := func(r io.Reader) error {
		tr := tar.NewReader(r)
		for {
			h, err := tr.Next()
			if err == io.EOF {
				return nil
			}
			if err != nil {
				return fmt.Errorf("压缩包有误：%w", err)
			}
			switch h.Typeflag {
			case tar.TypeDir:
				err = put(h.Name, 0o755, true, nil)
			case tar.TypeReg:
				err = put(h.Name, fs.FileMode(h.Mode), false, tr)
			}
			if err != nil {
				return err
			}
		}
	}
	f, err := os.Open(j.src)
	if err != nil {
		return err
	}
	defer f.Close()
	switch j.from.ID {
	case "zip":
		st, _ := f.Stat()
		zr, err := zip.NewReader(f, st.Size())
		if err != nil {
			return fmt.Errorf("压缩包有误：%w", err)
		}
		for i, zf := range zr.File {
			name := zf.Name
			if zf.Flags&0x800 == 0 { // not marked UTF-8: an older archive of GBK names
				if s, err := simplifiedchinese.GBK.NewDecoder().String(name); err == nil && !isASCII(name) {
					name = s
				}
			}
			if zf.FileInfo().IsDir() {
				err = put(name, 0o755, true, nil)
			} else {
				rc, oerr := zf.Open()
				if oerr != nil {
					return fmt.Errorf("压缩包有误：%w", oerr)
				}
				err = put(name, zf.Mode(), false, rc)
				rc.Close()
			}
			if err != nil {
				return err
			}
			j.p(float64(i+1) / float64(len(zr.File)))
		}
	case "tar":
		return untar(f)
	case "tgz", "gz":
		gr, err := gzip.NewReader(f)
		if err != nil {
			return fmt.Errorf("压缩包有误：%w", err)
		}
		if j.from.ID == "gz" {
			name := gr.Name
			if name == "" {
				name = strings.TrimSuffix(filepath.Base(j.src), filepath.Ext(j.src))
			}
			return put(filepath.Base(name), 0o644, false, gr)
		}
		return untar(gr)
	case "tbz":
		return untar(bzip2.NewReader(f))
	case "txz":
		xr, err := xz.NewReader(f)
		if err != nil {
			return fmt.Errorf("压缩包有误：%w", err)
		}
		return untar(xr)
	case "7z":
		r, err := sevenzip.OpenReader(j.src)
		if err != nil {
			return fmt.Errorf("压缩包有误：%w", err)
		}
		defer r.Close()
		for i, zf := range r.File {
			if zf.FileInfo().IsDir() {
				err = put(zf.Name, 0o755, true, nil)
			} else {
				rc, oerr := zf.Open()
				if oerr != nil {
					return fmt.Errorf("压缩包有误：%w", oerr)
				}
				err = put(zf.Name, zf.Mode(), false, rc)
				rc.Close()
			}
			if err != nil {
				return err
			}
			j.p(float64(i+1) / float64(len(r.File)))
		}
	case "rar":
		r, err := rardecode.OpenReader(j.src)
		if err != nil {
			return fmt.Errorf("压缩包有误：%w", err)
		}
		defer r.Close()
		for {
			h, err := r.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				return fmt.Errorf("压缩包有误：%w", err)
			}
			if err := put(h.Name, h.Mode(), h.IsDir, r); err != nil {
				return err
			}
		}
	default:
		return ErrUnsupported
	}
	return nil
}

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			return false
		}
	}
	return true
}

// pack writes the folder job.src as an archive.
func pack(j *job) error {
	var files []string
	filepath.WalkDir(j.src, func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && !strings.HasPrefix(d.Name(), ".") {
			files = append(files, p)
		}
		return nil
	})
	out, err := os.Create(j.dst)
	if err != nil {
		return err
	}
	defer out.Close()
	rel := func(p string) string {
		r, _ := filepath.Rel(j.src, p)
		return filepath.ToSlash(r)
	}
	switch j.to.ID {
	case "zip":
		zw := zip.NewWriter(out)
		for i, p := range files {
			st, err := os.Stat(p)
			if err != nil {
				return err
			}
			h, _ := zip.FileInfoHeader(st)
			h.Name, h.Method = rel(p), zip.Deflate
			h.Flags |= 0x800 // names in UTF-8
			w, err := zw.CreateHeader(h)
			if err != nil {
				return err
			}
			if err := copyFile(w, p); err != nil {
				return err
			}
			j.p(float64(i+1) / float64(len(files)))
		}
		return zw.Close()
	case "tar", "tgz":
		var w io.Writer = out
		var gw *gzip.Writer
		if j.to.ID == "tgz" {
			gw = gzip.NewWriter(out)
			w = gw
		}
		tw := tar.NewWriter(w)
		for i, p := range files {
			st, err := os.Stat(p)
			if err != nil {
				return err
			}
			h, _ := tar.FileInfoHeader(st, "")
			h.Name, h.Format = rel(p), tar.FormatPAX
			h.ModTime = st.ModTime().Truncate(time.Second)
			if err := tw.WriteHeader(h); err != nil {
				return err
			}
			if err := copyFile(tw, p); err != nil {
				return err
			}
			j.p(float64(i+1) / float64(len(files)))
		}
		if err := tw.Close(); err != nil {
			return err
		}
		if gw != nil {
			return gw.Close()
		}
		return nil
	}
	return ErrUnsupported
}

func copyFile(w io.Writer, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(w, f)
	return err
}
