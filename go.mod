module github.com/HengWh/woodwind-music

go 1.26.0

require (
	github.com/dhowden/tag v0.0.0-20240417053706-3d75831295e8
	github.com/ebitengine/purego v0.11.2
	github.com/hajimehoshi/go-mp3 v0.3.4
	github.com/jfreymuth/oggvorbis v1.0.5
	github.com/mewkiz/flac v1.0.14
	github.com/nsgo-dev/nsgo v0.0.0
	github.com/tphakala/go-aac v0.7.1
	golang.org/x/text v0.43.0
)

require (
	github.com/andybalholm/brotli v1.2.6 // indirect
	github.com/dlclark/regexp2/v2 v2.11.0 // indirect
	github.com/dop251/goja v0.0.0-20261027234916-5f913d1693d9 // indirect
	github.com/evanw/esbuild v0.28.2 // indirect
	github.com/go-sourcemap/sourcemap v2.1.3+incompatible // indirect
	github.com/google/pprof v0.0.0-20230207041349-798e818bf904 // indirect
	github.com/icza/bitio v1.1.0 // indirect
	github.com/jfreymuth/vorbis v1.0.2 // indirect
	github.com/mewkiz/pkg v0.0.0-20250417130911-3f050ff8c56d // indirect
	github.com/mewpkg/term v0.0.0-20241026122259-37a80af23985 // indirect
	github.com/tphakala/simd v1.11.0 // indirect
	github.com/yuin/goldmark v1.8.6 // indirect
	go.yaml.in/yaml/v3 v3.0.6 // indirect
	golang.org/x/image v0.45.0 // indirect
	golang.org/x/net v0.58.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
	rsc.io/qr v0.2.0 // indirect
)

tool github.com/nsgo-dev/nsgo/cmd/nsgo

// NSGo is developed beside this app: see README.md.
replace github.com/nsgo-dev/nsgo => ../nsgo
