package local

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
)

func TestImportDocumentTree(t *testing.T) {
	source := fstest.MapFS{
		"专辑/song.MP3":      {Data: []byte("audio")},
		"专辑/song.lrc":      {Data: []byte("lyrics")},
		"专辑/cover.jpg":     {Data: []byte("cover")},
		"notes.txt":        {Data: []byte("skip")},
		".hidden/song.mp3": {Data: []byte("skip")},
		"link.mp3":         {Mode: fs.ModeSymlink, Data: []byte("skip")},
	}
	root, err := ImportFolder(context.Background(), source, t.TempDir(), "音乐")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"专辑/song.MP3", "专辑/song.lrc", "专辑/cover.jpg"} {
		if _, err := os.Stat(filepath.Join(root, name)); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"notes.txt", ".hidden", "link.mp3"} {
		if _, err := os.Stat(filepath.Join(root, name)); !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("imported %s", name)
		}
	}
	// Removing the original tree cannot invalidate the imported music.
	delete(source, "专辑/song.MP3")
	if b, err := os.ReadFile(filepath.Join(root, "专辑/song.MP3")); err != nil || string(b) != "audio" {
		t.Fatal("import is not independent")
	}
}

type countedImportFS struct {
	fs.FS
	reads, largest int
}

func (s *countedImportFS) Open(name string) (fs.File, error) {
	f, err := s.FS.Open(name)
	if err != nil {
		return nil, err
	}
	return &countedImportFile{File: f, source: s}, nil
}

type countedImportFile struct {
	fs.File
	source *countedImportFS
}

func (f *countedImportFile) Read(b []byte) (int, error) {
	f.source.reads++
	f.source.largest = max(f.source.largest, len(b))
	return f.File.Read(b)
}

func TestImportUsesLargeProviderReads(t *testing.T) {
	source := &countedImportFS{FS: fstest.MapFS{"song.flac": {Data: make([]byte, 2<<20)}}}
	path := filepath.Join(t.TempDir(), "song.flac")
	if err := copyImported(context.Background(), source, "song.flac", path, make([]byte, 1<<20)); err != nil {
		t.Fatal(err)
	}
	if source.reads > 4 || source.largest < 1<<20 {
		t.Fatalf("too many provider round trips: %d reads, largest %d", source.reads, source.largest)
	}
	info, err := os.Stat(path)
	if err != nil || info.Size() != 2<<20 {
		t.Fatalf("incomplete copy: %v, %v", info, err)
	}
}

func TestImportCleansUpOnFailure(t *testing.T) {
	for _, cancel := range []bool{false, true} {
		parent := t.TempDir()
		ctx, stop := context.WithCancel(context.Background())
		if cancel {
			stop()
		}
		_, err := ImportFolder(ctx, fstest.MapFS{"cover.jpg": {Data: []byte("cover")}}, parent, "Music")
		stop()
		if cancel && !errors.Is(err, context.Canceled) || !cancel && !errors.Is(err, ErrNoMusic) {
			t.Fatalf("unexpected error %v", err)
		}
		entries, err := os.ReadDir(parent)
		if err != nil || len(entries) != 0 {
			t.Fatalf("partial import remains: %v, %v", entries, err)
		}
	}
}
