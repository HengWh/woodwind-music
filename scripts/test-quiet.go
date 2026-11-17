//go:build ignore

// Run from the repository: go run scripts/test-quiet.go [-run regexp] [directory].
// This entry point intentionally has no native target or device options.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	flags := flag.NewFlagSet("test-quiet", flag.ContinueOnError)
	pattern := flags.String("run", "", "run tests matching this regular expression")
	timeout := flags.Duration("timeout", 5*time.Minute, "timeout for each test package")
	if err := flags.Parse(os.Args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if flags.NArg() > 1 || *timeout <= 0 {
		return errors.New("usage: go run scripts/test-quiet.go [-run regexp] [-timeout 5m] [directory]")
	}
	if _, err := regexp.Compile(*pattern); err != nil {
		return fmt.Errorf("test filter: %w", err)
	}
	root, err := repositoryRoot()
	if err != nil {
		return err
	}
	dir := "."
	if flags.NArg() == 1 {
		dir = flags.Arg(0)
	}
	// Only local directories are accepted: never an entry point or extra flags.
	abs, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	abs, err = filepath.EvalSymlinks(abs)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(root, abs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return errors.New("test directory must be inside this repository")
	}
	info, err := os.Stat(abs)
	if err != nil || !info.IsDir() {
		return errors.New("test path must be a directory")
	}
	dir = "./" + filepath.ToSlash(rel)

	tmp, err := os.MkdirTemp("", "woodwind-quiet-test-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	data := filepath.Join(tmp, "data")
	if err := os.MkdirAll(data, 0o700); err != nil {
		return err
	}
	// Keep build artifacts local to this checkout as well; parallel worktrees
	// must not need write access to a shared user cache.
	cache := filepath.Join(root, ".nsgo", "quiet-go-cache")
	env := quietEnvironment(data, cache)
	fmt.Println("Checking code; tests use no windows, null audio, and temporary app data.")
	if err := command(root, env, "check", dir); err != nil {
		return err
	}
	args := []string{"test"}
	if *pattern != "" {
		args = append(args, "-run", *pattern)
	}
	args = append(args, dir, "--", "-tags=woodwind_quiet", "-count=1", "-timeout="+timeout.String())
	return command(root, env, args...)
}

func repositoryRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		content, err := os.ReadFile(filepath.Join(dir, "go.mod"))
		first, _, _ := strings.Cut(string(content), "\n")
		if err == nil && strings.TrimSpace(first) == "module github.com/HengWh/woodwind-music" {
			return filepath.EvalSymlinks(dir)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("run inside the woodwind-music repository")
		}
		dir = parent
	}
}

func quietEnvironment(data, cache string) []string {
	var env []string
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		key = strings.ToUpper(key)
		if strings.HasPrefix(key, "NSGO_TEST_") ||
			(strings.HasPrefix(key, "WOOD_WIND_") && strings.HasSuffix(key, "_SHOTS")) {
			continue
		}
		switch key {
		case "GOFLAGS", "GOWORK", "GOOS", "GOARCH", "GOCACHE",
			"WOOD_WIND_MUSIC_DATA_DIR", "WOODWIND_MUSIC_DATA_DIR", "NSGO_MUSIC_DATA_DIR",
			"WOOD_WIND_WINDOWS_NATIVE", "WOOD_WIND_MOBILE_ART_DIR", "MYGO_MUSIC_TEST_ECHO":
			continue
		}
		env = append(env, entry)
	}
	return append(env, "GOFLAGS=", "GOWORK=off", "GOCACHE="+cache, "WOOD_WIND_MUSIC_DATA_DIR="+data)
}

func command(root string, env []string, args ...string) error {
	cmd := exec.Command("go", append([]string{"tool", "nsgo"}, args...)...)
	cmd.Dir, cmd.Env = root, env
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("nsgo %s: %w", args[0], err)
	}
	return nil
}
