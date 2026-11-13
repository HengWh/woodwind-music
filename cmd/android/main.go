// Command android builds Woodwind's NSGo app with its Android playback host.
// It uses only the framework's public host API; no NSGo checkout is modified.
package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"encoding/xml"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/nsgo-dev/nsgo/cmd/nsgo/apk"
	"go.yaml.in/yaml/v3"
)

type project struct {
	Name    string `yaml:"name"`
	ID      string `yaml:"id"`
	Version string `yaml:"version"`
	Icon    struct {
		PNG string `yaml:"png"`
	} `yaml:"icon"`
	Build struct {
		Output string `yaml:"output"`
	} `yaml:"build"`
}

func main() {
	install := flag.Bool("install", false, "install on the connected phone")
	run := flag.Bool("run", false, "open the installed app")
	device := flag.String("device", "", "adb device serial")
	debug := flag.Bool("debug", false, "allow Android developer inspection (run-as)")
	flag.Parse()
	if err := build(*install || *run, *run, *device, *debug); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func build(install, launch bool, device string, debug bool) error {
	raw, err := os.ReadFile("nsgo.yaml")
	if err != nil {
		return err
	}
	var p project
	if err = yaml.Unmarshal(raw, &p); err != nil {
		return err
	}
	if p.Build.Output == "" {
		p.Build.Output = "dist"
	}
	sdk := os.Getenv("ANDROID_HOME")
	if sdk == "" {
		sdk = ".tools/android/sdk"
	}
	javaHome := os.Getenv("JAVA_HOME")
	if javaHome == "" {
		javaHome = ".tools/android/jdk/Contents/Home"
	}
	java := filepath.Join(javaHome, "bin", "java")
	javac := filepath.Join(javaHome, "bin", "javac")
	jar := filepath.Join(sdk, "android-35", "android.jar")
	if _, err := os.Stat(jar); err != nil {
		jar = filepath.Join(sdk, "platforms", "android-35", "android.jar")
	}
	r8 := os.Getenv("R8_JAR")
	if r8 == "" {
		r8 = ".tools/android/r8.jar"
	}
	matches, _ := filepath.Glob(filepath.Join(sdk, "build-tools", "*", "aapt2"))
	sort.Strings(matches)
	if len(matches) == 0 {
		return fmt.Errorf("Android build tools missing; run android/bootstrap.sh or set ANDROID_HOME and JAVA_HOME")
	}
	tools := filepath.Dir(matches[len(matches)-1])
	for _, f := range []string{java, javac, jar, r8} {
		if _, err := os.Stat(f); err != nil {
			return fmt.Errorf("missing %s; run android/bootstrap.sh: %w", f, err)
		}
	}
	if err := command("go", "tool", "nsgo", "build", "-target", "android_arm64"); err != nil {
		return err
	}
	nsgo, err := exec.Command("go", "list", "-m", "-f", "{{.Dir}}", "github.com/nsgo-dev/nsgo").Output()
	if err != nil {
		return err
	}
	framework := strings.TrimSpace(string(nsgo))
	stage, err := os.MkdirTemp("", "woodwind-android-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	classes, dex := filepath.Join(stage, "classes"), filepath.Join(stage, "dex")
	os.MkdirAll(classes, 0700)
	os.MkdirAll(dex, 0700)
	sources, err := filepath.Glob(filepath.Join(framework, "internal/platform/android/java/dev/oneo/nsgo/*.java"))
	if err != nil {
		return err
	}
	var compile []string
	for _, source := range sources {
		if filepath.Base(source) != "NsgoActivity.java" {
			compile = append(compile, source)
		}
	}
	own, _ := filepath.Glob("android/src/dev/oneo/nsgo/*.java")
	compile = append(compile, own...)
	args := []string{"-nowarn", "-Xlint:-options", "--release", "17", "-classpath", jar, "-d", classes}
	args = append(args, compile...)
	if err := command(javac, args...); err != nil {
		return err
	}
	args = []string{"-cp", r8, "com.android.tools.r8.D8", "--lib", jar, "--min-api", "28", "--output", dex}
	err = filepath.WalkDir(classes, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(path, ".class") {
			args = append(args, path)
		}
		return nil
	})
	if err != nil {
		return err
	}
	if err := command(java, args...); err != nil {
		return err
	}
	manifest, err := os.ReadFile("android/AndroidManifest.xml")
	if err != nil {
		return err
	}
	escape := func(s string) string { var b bytes.Buffer; xml.EscapeText(&b, []byte(s)); return b.String() }
	manifest = []byte(strings.NewReplacer("$APP_ID", escape(p.ID), "$LABEL", escape(p.Name), "$VERSION_CODE", strconv.Itoa(apk.VersionCode(p.Version)), "$VERSION", escape(p.Version), "$DEBUG", strconv.FormatBool(debug)).Replace(string(manifest)))
	manifestPath := filepath.Join(stage, "AndroidManifest.xml")
	os.WriteFile(manifestPath, manifest, 0600)
	res := filepath.Join(stage, "res", "drawable")
	os.MkdirAll(res, 0700)
	if err := copyFile(p.Icon.PNG, filepath.Join(res, "icon.png")); err != nil {
		return err
	}
	compiled := filepath.Join(stage, "resources.zip")
	if err := command(filepath.Join(tools, "aapt2"), "compile", "--dir", filepath.Join(stage, "res"), "-o", compiled); err != nil {
		return err
	}
	shell := filepath.Join(stage, "shell.apk")
	if err := command(filepath.Join(tools, "aapt2"), "link", "-I", jar, "--manifest", manifestPath, "-o", shell, compiled); err != nil {
		return err
	}
	base := filepath.Join(p.Build.Output, p.Name+"-"+p.Version+"-android_arm64", p.Name+".apk")
	unsigned := filepath.Join(stage, "unsigned.apk")
	if err := merge(base, shell, filepath.Join(dex, "classes.dex"), unsigned); err != nil {
		return err
	}
	aligned := filepath.Join(stage, "aligned.apk")
	if err := command(filepath.Join(tools, "zipalign"), "-f", "-P", "16", "4", unsigned, aligned); err != nil {
		return err
	}
	if err := signingKey(stage); err != nil {
		return err
	}
	signed := filepath.Join(stage, "signed.apk")
	if err := command(java, "-jar", filepath.Join(tools, "lib", "apksigner.jar"), "sign", "--key", filepath.Join(stage, "key.pk8"), "--cert", filepath.Join(stage, "cert.der"), "--out", signed, aligned); err != nil {
		return err
	}
	if err := command(java, "-jar", filepath.Join(tools, "lib", "apksigner.jar"), "verify", "--verbose", signed); err != nil {
		return err
	}
	if err := copyFile(signed, base); err != nil {
		return err
	}
	release := filepath.Join(p.Build.Output, "releases", "woodwind-music-"+p.Version+"-android-arm64.apk")
	if err := copyFile(signed, release); err != nil {
		return err
	}
	// NSGo's base package wrote checksums before adding our service.
	// Recompute them rather than leave a checksum for a different APK.
	if err := writeChecksum(release); err != nil {
		return err
	}
	fmt.Println("Android playback APK:", release)
	if install {
		adb := os.Getenv("ADB")
		if adb == "" {
			adb = ".tools/android/platform-tools/adb"
		}
		if _, err := os.Stat(adb); err != nil {
			adb = "adb"
		}
		prefix := []string{}
		if device != "" {
			prefix = append(prefix, "-s", device)
		}
		if err := command(adb, append(prefix, "install", "--no-streaming", "-r", release)...); err != nil {
			return err
		}
		if launch {
			return command(adb, append(prefix, "shell", "am", "start", "-n", p.ID+"/dev.oneo.nsgo.NsgoActivity")...)
		}
	}
	return nil
}

func command(name string, args ...string) error {
	c := exec.Command(name, args...)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	c.Stdin = os.Stdin
	if err := c.Run(); err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	return nil
}

func copyFile(from, to string) error {
	b, err := os.ReadFile(from)
	if err != nil {
		return err
	}
	return os.WriteFile(to, b, 0644)
}

func merge(base, shell, dex, out string) error {
	f, err := os.Create(out)
	if err != nil {
		return err
	}
	defer f.Close()
	w := zip.NewWriter(f)
	for i, path := range []string{shell, base} {
		z, err := zip.OpenReader(path)
		if err != nil {
			return err
		}
		for _, entry := range z.File {
			if i == 1 && !strings.HasPrefix(entry.Name, "assets/") && !strings.HasPrefix(entry.Name, "lib/") {
				continue
			}
			r, err := entry.Open()
			if err != nil {
				z.Close()
				return err
			}
			header := &zip.FileHeader{Name: entry.Name, Method: zip.Deflate}
			// Android 11+ requires an uncompressed, aligned resource table.
			if entry.Name == "resources.arsc" {
				header.Method = zip.Store
			}
			dest, err := w.CreateHeader(header)
			if err == nil {
				_, err = io.Copy(dest, r)
			}
			r.Close()
			if err != nil {
				z.Close()
				return err
			}
		}
		z.Close()
	}
	b, err := os.ReadFile(dex)
	if err != nil {
		return err
	}
	d, err := w.Create("classes.dex")
	if err != nil {
		return err
	}
	if _, err = d.Write(b); err != nil {
		return err
	}
	return w.Close()
}

func signingKey(stage string) error {
	data, err := os.ReadFile(".nsgo/android/debug.pem")
	if err != nil {
		return err
	}
	var private, cert []byte
	for len(data) > 0 {
		block, rest := pem.Decode(data)
		data = rest
		if block == nil {
			break
		}
		switch block.Type {
		case "RSA PRIVATE KEY":
			key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
			if err != nil {
				return err
			}
			private, err = x509.MarshalPKCS8PrivateKey(key)
			if err != nil {
				return err
			}
		case "CERTIFICATE":
			cert = block.Bytes
		}
	}
	if len(private) == 0 || len(cert) == 0 {
		return fmt.Errorf("missing debug signing identity")
	}
	if err := os.WriteFile(filepath.Join(stage, "key.pk8"), private, 0600); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(stage, "cert.der"), cert, 0600)
}

func writeChecksum(path string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	text := fmt.Sprintf("%x  %s\n", sha256.Sum256(b), filepath.Base(path))
	return os.WriteFile(path+".sha256", []byte(text), 0644)
}
