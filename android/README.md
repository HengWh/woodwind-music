# Android host

This host keeps `core.Video` as wood wind music’s audio player. It adds Android's platform `MediaSession` and a `mediaPlayback` foreground service, using NSGo's public `HostParams`, `PostHost`, and `OnHostEvent` APIs. NSGo is downloaded at the version pinned in `go.mod`; the host reads its Java sources from the resolved module directory without modifying them or adding a second playback engine.

Compile the pinned host with Android SDK Platform 36 (`android/bootstrap.sh` installs it locally). The manifest continues to support Android 9 / API 28 and target API 35.

## Build and run

On macOS Apple Silicon:

```sh
./android/bootstrap.sh
go run ./cmd/android -install -run
```

The bootstrap script downloads the official Android SDK tools, D8, and an Adoptium JDK into the ignored `.tools/android/` directory. Other development hosts can provide `ANDROID_HOME`, `JAVA_HOME`, `R8_JAR`, and `ADB` instead. The Go command first builds the NSGo executable, compiles the existing NSGo Java sources together with this host (replacing only the launcher activity), packages the manifest/resources, aligns the APK and signs it with the project's existing NSGo debug key. It also updates the APK checksum. Use `-device SERIAL` to select a phone and `-debug` to enable `run-as` inspection during development.

Use this command for the complete Android package. A bare `go tool nsgo build -target android_arm64` produces the framework's base package, without this playback service.

## Ownership and lifecycle

- `PlaybackRuntime` retains one `NsgoView`, Go child process and `core.Video`. A `MutableContextWrapper` points at the current activity while attached and at the application context after detaching, so a destroyed activity is not retained by the runtime's root context.
- The foreground service retains the runtime while playing. Returning to the activity reattaches the same view. Leaving the activity saves the Go session; it does not stop the music. Pausing releases the wake lock, and five minutes of pause stops the service. A detached, idle runtime is then released.
- Go owns the queue, repeat/shuffle, lyrics and desired playback state. Java owns the notification, media session, audio focus and CPU wake lock. All media-session commands pass back to Go; no independent native queue is created.
- Managed `core.Video` uses the default audio-session mode because the service requests permanent `AUDIOFOCUS_GAIN` with music attributes. Unmanaged base packages retain NSGo's normal audio-session behavior.
- Temporary focus loss pauses and records whether to resume; permanent loss and a noisy route change pause without automatically resuming. An explicit user pause cancels automatic resumption.
- The service is private, and notification actions use immutable, explicitly addressed pending intents. Media-session notifications qualify for Android's media notification exemption; no startup notification-permission prompt is required.
- A user force-stop is respected. This is not a promise to survive force-stop, reboot or an OEM's explicit process termination.

## Verification

`go test ./...` covers native queue behavior, stale callbacks, source errors, session restore, imports, the real `core.Video` protocol, media commands, focus interruption/resumption, and APK resource-table packaging. APK builds additionally run `apksigner verify`.

On a connected phone, verify directory selection/import, real audio progress, pause/resume and seek, next/previous/repeat, media buttons, the media notification, Home/background playback, screen-off playback, returning to the same runtime, and session restoration after reopening. Hardware Bluetooth routing and incoming calls need the corresponding device/setup.

For targeted service diagnostics (Android reports the foreground-service lifecycle and media state):

```sh
.tools/android/platform-tools/adb shell dumpsys activity service dev.oneo.nsgo.music/dev.oneo.nsgo.PlaybackService
```
