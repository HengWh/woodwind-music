#!/usr/bin/env python3
"""Normalize and inspect the four release packages after building all targets.

Run go tool nsgo build -target macos_arm64,windows_x64, go run ./cmd/android,
and go run scripts/package-windows.go first. This command never reads app data.
"""

import hashlib
import json
from pathlib import Path
import plistlib
import re
import zipfile


ROOT = Path(__file__).resolve().parents[1]
SLUG = "wood-wind-music"
ICONS = {"icon.png", "icon-dev.png", "icon.icns"}


def checksum(path):
    digest = hashlib.sha256(path.read_bytes()).hexdigest()
    path.with_name(path.name + ".sha256").write_text(f"{digest}  {path.name}\n")
    return digest


def require(condition, message):
    if not condition:
        raise SystemExit(message)


def main():
    config = (ROOT / "nsgo.yaml").read_text()
    name = re.search(r"^name: (.+)$", config, re.M)[1]
    version = re.search(r"^version: (.+)$", config, re.M)[1]
    build_number = int(re.search(r"^build_number: (\d+)$", config, re.M)[1])
    output = ROOT / re.search(r"^  output: (.+)$", config, re.M)[1]
    release = output / "releases"
    require((release / ".version").read_text().strip() == version, "Release directory has a different version")

    # Only approved icons may enter NSGo's resource copier. Plugin code and
    # account stores belong exclusively to each installation's data directory.
    resources = {p.relative_to(ROOT / "resources").as_posix() for p in (ROOT / "resources").rglob("*") if p.is_file()}
    require(resources == ICONS, f"Unexpected application resources: {sorted(resources - ICONS)}")

    apk = release / f"{SLUG}-{version}-android-arm64.apk"
    require(apk.is_file(), "Build the complete Android playback APK first")
    with zipfile.ZipFile(apk) as archive:
        entries = set(archive.namelist())
        allowed = {"AndroidManifest.xml", "resources.arsc", "classes.dex", "lib/arm64-v8a/libnsgo.so", "res/drawable/icon.png", "res/xml/network_security_config.xml"}
        signatures = {"META-INF/KEY.RSA", "META-INF/KEY.SF", "META-INF/MANIFEST.MF"}
        allowed |= {f"assets/{icon}" for icon in ICONS}
        require(len(entries) == len(archive.namelist()), "Duplicate APK entries")
        require(allowed <= entries and entries <= allowed | signatures, f"Unexpected or missing APK entries: {sorted((entries - allowed - signatures) | (allowed - entries))}")
        for icon in ICONS:
            require(archive.read(f"assets/{icon}") == (ROOT / "resources" / icon).read_bytes(), f"APK resource differs: {icon}")
        require(archive.getinfo("resources.arsc").compress_type == zipfile.ZIP_STORED, "Compressed Android resource table")

    app = output / f"{name}-{version}-macos_arm64" / f"{name}.app"
    allowed_app = {"Contents/Info.plist", f"Contents/MacOS/{name}", "Contents/_CodeSignature/CodeResources"}
    allowed_app |= {f"Contents/Resources/{icon}" for icon in ICONS | {"AppIcon.icns"}}
    app_entries = {p.relative_to(app).as_posix() for p in app.rglob("*") if p.is_file()}
    require(app_entries == allowed_app, f"Unexpected or missing macOS app entries: {sorted(app_entries ^ allowed_app)}")
    info = plistlib.loads((app / "Contents/Info.plist").read_bytes())
    require(info["CFBundleShortVersionString"] == version, "macOS version mismatch")
    require(str(info["CFBundleVersion"]) == str(build_number), "macOS build number mismatch")

    portable = release / f"{SLUG}-{version}-windows-x64-portable.zip"
    with zipfile.ZipFile(portable) as archive:
        allowed = ICONS | {f"{name}.exe", "LICENSE", "NOTICE", "docs/WINDOWS.md"}
        allowed |= {p.relative_to(ROOT).as_posix() for p in (ROOT / "LICENSES").rglob("*") if p.is_file()}
        require(set(archive.namelist()) == allowed, "Unexpected Windows portable contents")
        for icon in ICONS:
            require(archive.read(icon) == (ROOT / "resources" / icon).read_bytes(), f"Windows resource differs: {icon}")

    # NSGo falls back to the final component of the app ID for a Chinese name.
    # Canonical download names stay independent of the display name.
    for suffix in ("macos-arm64.dmg", "windows-x64-setup.exe"):
        source = release / f"music-{version}-{suffix}"
        target = release / f"{SLUG}-{version}-{suffix}"
        if source.exists():
            source.replace(target)
            source.with_name(source.name + ".sha256").unlink(missing_ok=True)
        require(target.is_file(), f"Missing release package: {target.name}")

    # The framework's base APK has no background playback host and must never
    # be uploaded alongside the complete application.
    base = release / f"music-{version}-android-arm64.apk"
    base.unlink(missing_ok=True)
    base.with_name(base.name + ".sha256").unlink(missing_ok=True)
    for script in ("install.sh", "install.ps1"):
        path = release / script
        content = path.read_bytes().replace(b"slug='music'", b"slug='wood-wind-music'")
        content = content.replace(b"$slug = 'music'", b"$slug = 'wood-wind-music'")
        path.write_bytes(content)

    packages = [release / f"{SLUG}-{version}-{suffix}" for suffix in (
        "macos-arm64.dmg", "windows-x64-setup.exe", "windows-x64-portable.zip", "android-arm64.apk")]
    report = {
        "version": version, "build_number": build_number,
        "bundled_plugins": [], "bundled_accounts": [],
        "packages": [{"file": p.name, "bytes": p.stat().st_size, "sha256": checksum(p)} for p in packages],
        "checks": ["approved resources only", "APK content allowlist", "macOS app content allowlist", "Windows portable content allowlist", "macOS version and build number"],
    }
    allowed_files = {p.name for p in packages} | {p.name + ".sha256" for p in packages} | {".version", "install.sh", "install.ps1"}
    require({p.name for p in release.iterdir()} == allowed_files, "Unexpected files in release directory")
    report_path = ROOT / "build" / f"release-{version}" / "package-audit.json"
    report_path.parent.mkdir(parents=True, exist_ok=True)
    report_path.write_text(json.dumps(report, ensure_ascii=False, indent=2) + "\n")
    print(json.dumps(report, ensure_ascii=False, indent=2))


if __name__ == "__main__":
    main()
