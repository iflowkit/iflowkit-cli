package filex

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func EnsureDir(path string) error {
	return os.MkdirAll(path, 0o755)
}

// AtomicWriteFile writes data to a temp file in the same directory and then renames it.
// On Windows, callers should remove the destination file first if it already exists.
func AtomicWriteFile(filename string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(filename)
	if err := EnsureDir(dir); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(perm); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp.Name(), filename); err != nil {
		return fmt.Errorf("atomic rename failed: %w", err)
	}
	return nil
}

// ExtractZipFile extracts a zip archive to destDir with Zip-Slip protection.
func ExtractZipFile(zipPath, destDir string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()

	if err := EnsureDir(destDir); err != nil {
		return err
	}
	root, err := filepath.Abs(destDir)
	if err != nil {
		return err
	}
	root = filepath.Clean(root) + string(os.PathSeparator)

	for _, f := range r.File {
		name := strings.ReplaceAll(f.Name, "\\", "/")
		name = strings.TrimPrefix(name, "/")
		target := filepath.Join(destDir, filepath.FromSlash(name))
		absTarget, err := filepath.Abs(target)
		if err != nil {
			return err
		}
		absTarget = filepath.Clean(absTarget)
		if !strings.HasPrefix(absTarget+string(os.PathSeparator), root) {
			return fmt.Errorf("zip entry escapes destination: %s", f.Name)
		}

		if f.FileInfo().IsDir() {
			if err := EnsureDir(absTarget); err != nil {
				return err
			}
			continue
		}
		if err := EnsureDir(filepath.Dir(absTarget)); err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			return err
		}
		out, err := os.OpenFile(absTarget, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, f.Mode())
		if err != nil {
			_ = rc.Close()
			return err
		}
		if _, err := io.Copy(out, rc); err != nil {
			_ = out.Close()
			_ = rc.Close()
			return err
		}
		_ = out.Close()
		_ = rc.Close()
	}
	return nil
}

// ZipDirToBytes zips the contents of srcDir (recursively) and returns the zip bytes.
// The zip will NOT include an outer srcDir folder; entries are relative to srcDir.
func ZipDirToBytes(srcDir string) ([]byte, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	root, err := filepath.Abs(srcDir)
	if err != nil {
		_ = zw.Close()
		return nil, err
	}

	// Ensure deterministic-ish output by using a stable mod time.
	stableTime := time.Unix(0, 0)

	walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		rel = strings.TrimPrefix(rel, "/")
		if rel == "" {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return err
		}
		h, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		h.Name = rel
		h.Method = zip.Deflate
		h.Modified = stableTime

		w, err := zw.CreateHeader(h)
		if err != nil {
			return err
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = io.Copy(w, f)
		return err
	})

	closeErr := zw.Close()
	if walkErr != nil {
		return nil, walkErr
	}
	if closeErr != nil {
		return nil, closeErr
	}
	return buf.Bytes(), nil
}
