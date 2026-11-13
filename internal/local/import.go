package local

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

var ErrNoMusic = errors.New("no music files in the selected folder")

// ImportFolder copies a picked document tree into app-owned storage. Android
// picks have an fs.FS rather than a disk path; keeping a copy lets the existing
// catalog, tags, lyrics and native player use stable files after a restart.
// A failed or cancelled import leaves no partial library behind.
func ImportFolder(ctx context.Context, source fs.FS, parent, name string) (root string, err error) {
	if source == nil {
		return "", errors.New("the selected folder cannot be read")
	}
	if err = os.MkdirAll(parent, 0o700); err != nil {
		return "", err
	}
	name = filepath.Base(strings.ReplaceAll(name, "\\", "/"))
	if name == "." || name == "/" || name == "" {
		name = "Music"
	}
	root, err = os.MkdirTemp(parent, name+"-")
	if err != nil {
		return "", err
	}
	defer func() {
		if err != nil {
			os.RemoveAll(root)
		}
	}()
	songs := 0
	// Android document reads cross a process boundary. io.Copy's default
	// 32 KiB reads reopen a provider descriptor far too often for a library
	// of gigabytes; reuse a buffer matching NSGo's 1 MiB document chunks.
	buffer := make([]byte, 1<<20)
	err = fs.WalkDir(source, ".", func(path string, entry fs.DirEntry, walkErr error) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if walkErr != nil {
			return walkErr
		}
		if path == "." {
			return nil
		}
		if strings.HasPrefix(entry.Name(), ".") {
			if entry.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if entry.IsDir() || entry.Type()&fs.ModeSymlink != 0 {
			return nil
		}
		music := false
		switch strings.ToLower(filepath.Ext(path)) {
		case ".mp3", ".flac", ".ogg", ".wav", ".m4a", ".aac":
			music = true
		case ".lrc", ".jpg", ".jpeg", ".png", ".webp":
		default:
			return nil
		}
		if !fs.ValidPath(path) || !filepath.IsLocal(filepath.FromSlash(path)) {
			return fmt.Errorf("invalid file path: %q", path)
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		dest := filepath.Join(root, filepath.FromSlash(path))
		if err := copyImported(ctx, source, path, dest, buffer); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		if music {
			songs++
		}
		return nil
	})
	if err == nil && songs == 0 {
		err = ErrNoMusic
	}
	return root, err
}

func copyImported(ctx context.Context, source fs.FS, path, dest string, buffer []byte) error {
	in, err := source.Open(path)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dest), 0o700); err != nil {
		return err
	}
	out, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	// Hide os.File.ReadFrom, which would ignore CopyBuffer's buffer and
	// fall back to small reads for a provider-backed source.
	_, copyErr := io.CopyBuffer(struct{ io.Writer }{out}, importReader{ctx, in}, buffer)
	return errors.Join(copyErr, out.Close())
}

type importReader struct {
	ctx context.Context
	io.Reader
}

func (r importReader) Read(b []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.Reader.Read(b)
}
