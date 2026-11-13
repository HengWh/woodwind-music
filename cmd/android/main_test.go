package main

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestMergeKeepsAndroidResourceTableStored(t *testing.T) {
	dir := t.TempDir()
	makeZip := func(name string, files map[string]string) string {
		path := filepath.Join(dir, name)
		f, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		w := zip.NewWriter(f)
		for name, data := range files {
			e, err := w.Create(name)
			if err != nil {
				t.Fatal(err)
			}
			io.WriteString(e, data)
		}
		if err := w.Close(); err != nil {
			t.Fatal(err)
		}
		f.Close()
		return path
	}
	base := makeZip("base.apk", map[string]string{"AndroidManifest.xml": "old", "resources.arsc": "old", "classes.dex": "old", "lib/arm64-v8a/libnsgo.so": "engine", "assets/icon.png": "asset", "META-INF/OLD.SF": "old"})
	shell := makeZip("shell.apk", map[string]string{"AndroidManifest.xml": "service manifest", "resources.arsc": "resources", "res/drawable/icon.png": "logo"})
	dex := filepath.Join(dir, "classes.dex")
	os.WriteFile(dex, []byte("new runtime"), 0600)
	out := filepath.Join(dir, "merged.apk")
	if err := merge(base, shell, dex, out); err != nil {
		t.Fatal(err)
	}
	z, err := zip.OpenReader(out)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	seen := map[string]bool{}
	for _, f := range z.File {
		if seen[f.Name] {
			t.Fatalf("duplicate %s", f.Name)
		}
		seen[f.Name] = true
		r, _ := f.Open()
		b, _ := io.ReadAll(r)
		r.Close()
		if f.Name == "resources.arsc" && (f.Method != zip.Store || string(b) != "resources") {
			t.Fatal("resource table must be uncompressed")
		}
		if f.Name == "classes.dex" && string(b) != "new runtime" {
			t.Fatal("base runtime leaked")
		}
	}
	if len(seen) != 6 || seen["META-INF/OLD.SF"] {
		t.Fatalf("entries: %v", seen)
	}
}
