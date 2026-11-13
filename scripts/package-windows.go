//go:build ignore

// Run after nsgo build: go run scripts/package-windows.go
package main

import (
	"archive/zip"
	"crypto/sha256"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"go.yaml.in/yaml/v3"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	var config struct {
		Name, Version string
		Build         struct{ Output string }
	}
	b, err := os.ReadFile("nsgo.yaml")
	if err != nil {
		return err
	}
	if err = yaml.Unmarshal(b, &config); err != nil {
		return err
	}
	if config.Build.Output == "" {
		config.Build.Output = "dist"
	}
	dir := filepath.Join(config.Build.Output, config.Name+"-"+config.Version+"-windows_x64")
	if _, err = os.Stat(filepath.Join(dir, config.Name+".exe")); err != nil {
		return fmt.Errorf("build windows_x64 first: %w", err)
	}
	releases := filepath.Join(config.Build.Output, "releases")
	if err = os.MkdirAll(releases, 0o755); err != nil {
		return err
	}
	name := strings.ToLower(strings.ReplaceAll(config.Name, " ", "-")) + "-" + config.Version + "-windows-x64-portable.zip"
	path := filepath.Join(releases, name)
	out, err := os.Create(path)
	if err != nil {
		return err
	}
	zw := zip.NewWriter(out)
	add := func(path, name string) error {
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		info, err := f.Stat()
		if err != nil {
			return err
		}
		h, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		h.Name, h.Method = filepath.ToSlash(name), zip.Deflate
		w, err := zw.CreateHeader(h)
		if err != nil {
			return err
		}
		_, err = io.Copy(w, f)
		return err
	}
	err = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		if strings.HasSuffix(rel, ".nsi") || strings.HasSuffix(rel, "-setup.ico") || strings.HasSuffix(rel, "-uninstall.ico") || strings.HasSuffix(rel, "-setup.exe") {
			return nil
		}
		return add(path, rel)
	})
	if err == nil {
		for _, file := range []string{"LICENSE", "NOTICE", "docs/WINDOWS.md"} {
			if err = add(file, file); err != nil {
				break
			}
		}
	}
	if err == nil {
		err = filepath.WalkDir("LICENSES", func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			return add(path, path)
		})
	}
	zipErr, closeErr := zw.Close(), out.Close()
	if err != nil {
		return err
	}
	if zipErr != nil {
		return zipErr
	}
	if closeErr != nil {
		return closeErr
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	hash := sha256.New()
	if _, err = io.Copy(hash, f); err != nil {
		return err
	}
	if err = os.WriteFile(path+".sha256", []byte(fmt.Sprintf("%x  %s\n", hash.Sum(nil), name)), 0o644); err != nil {
		return err
	}
	fmt.Println(path)
	return nil
}
