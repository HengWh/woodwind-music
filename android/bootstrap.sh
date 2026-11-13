#!/bin/sh
# Local Android build tools for this project's macOS Apple Silicon workspace.
set -eu
cd "$(dirname "$0")/.."
case "$(uname -s)-$(uname -m)" in
  Darwin-arm64) ;;
  *) echo 'On this host, provide ANDROID_HOME, JAVA_HOME, R8_JAR and adb instead.' >&2; exit 1 ;;
esac
mkdir -p .tools/android
fetch() { [ -f "$2" ] || curl -fL --retry 2 "$1" -o "$2"; }
if [ ! -x .tools/android/platform-tools/adb ]; then
  fetch https://dl.google.com/android/repository/platform-tools_r35.0.2-darwin.zip .tools/android/platform-tools-stable.zip
  unzip -q -o .tools/android/platform-tools-stable.zip -d .tools/android
fi
if [ ! -x .tools/android/jdk/Contents/Home/bin/javac ]; then
  fetch https://api.adoptium.net/v3/binary/latest/21/ga/mac/aarch64/jdk/hotspot/normal/eclipse .tools/android/jdk.tar.gz
  mkdir -p .tools/android/jdk
  tar -xzf .tools/android/jdk.tar.gz -C .tools/android/jdk --strip-components=1
fi
if [ ! -f .tools/android/sdk/android-35/android.jar ]; then
  fetch https://dl.google.com/android/repository/platform-35_r02.zip .tools/android/platform35.zip
  unzip -q -o .tools/android/platform35.zip -d .tools/android/sdk
fi
if [ ! -x .tools/android/sdk/build-tools/android-16/aapt2 ]; then
  fetch https://dl.google.com/android/repository/build-tools_r36_macosx.zip .tools/android/build-tools.zip
  unzip -q -o .tools/android/build-tools.zip -d .tools/android/sdk/build-tools
fi
fetch https://dl.google.com/dl/android/maven2/com/android/tools/r8/8.12.22/r8-8.12.22.jar .tools/android/r8.jar
echo 'Android build tools are ready. Run: go run ./cmd/android -install -run'
