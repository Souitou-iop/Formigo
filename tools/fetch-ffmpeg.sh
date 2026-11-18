#!/bin/sh
# Fetches the FFmpeg builds Formigo bundles into resources/<platform>/bin:
# every system's, or only those of the systems named (darwin, windows,
# linux). Sources and licenses: resources/licenses/FFmpeg.txt.
#
#   tools/fetch-ffmpeg.sh [darwin] [windows] [linux]
set -eu
cd "$(dirname "$0")/.."
W=$(mktemp -d)
trap 'rm -rf "$W"' EXIT
ALL="$*"
has() { [ -z "$ALL" ] && return 0; case " $ALL " in *" $1 "*) return 0 ;; esac; return 1; }
if has darwin; then
  curl -fsSL -o "$W/a.zip" https://ffmpeg.martin-riedl.de/download/macos/arm64/1789931890_9.0.2/ffmpeg.zip
  curl -fsSL -o "$W/b.zip" https://ffmpeg.martin-riedl.de/download/macos/amd64/1789931006_9.0.2/ffmpeg.zip
  mkdir -p resources/darwin-arm64/bin resources/darwin-amd64/bin
  unzip -oq "$W/a.zip" ffmpeg -d resources/darwin-arm64/bin
  unzip -oq "$W/b.zip" ffmpeg -d resources/darwin-amd64/bin
fi
if has windows; then
  curl -fsSL -o "$W/w.zip" https://github.com/BtbN/FFmpeg-Builds/releases/download/latest/ffmpeg-n8.1-latest-win64-lgpl-8.1.zip
  mkdir -p resources/windows-amd64/bin
  unzip -ojq "$W/w.zip" '*/bin/ffmpeg.exe' -d resources/windows-amd64/bin
  unzip -ojq "$W/w.zip" '*/LICENSE.txt' -d "$W" && cp "$W/LICENSE.txt" resources/licenses/FFmpeg-LGPL.txt
fi
if has linux; then
  curl -fsSL -o "$W/l.tar.xz" https://github.com/BtbN/FFmpeg-Builds/releases/download/latest/ffmpeg-n8.1-latest-linux64-lgpl-8.1.tar.xz
  mkdir -p resources/linux-amd64/bin
  tar -xJf "$W/l.tar.xz" -C "$W" && cp "$W"/ffmpeg-*/bin/ffmpeg resources/linux-amd64/bin/
fi
chmod +x resources/*/bin/ffmpeg* 2>/dev/null || true
