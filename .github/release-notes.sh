#!/usr/bin/env bash
# Writes the notes of the release TAG to stdout: the version's section of
# CHANGELOG.md, links to the files uploaded to the release, then the FAQ of
# release-template.md. Without the release, or before its uploads, the
# downloads are left out.
#
#   .github/release-notes.sh v0.3.0 [owner/repo]
set -euo pipefail

tag="$1"
repo="${2:-${GITHUB_REPOSITORY:-}}"
version="${tag#v}"
here="$(dirname "$0")"

section="$(awk -v v="$version" '
  /^## / { if (sec) exit; sec = ($2 == v || $2 == "[" v "]"); next }
  sec { print }
' "$here/../CHANGELOG.md")"
if ! grep -q '[^[:space:]]' <<< "$section"; then
  echo "CHANGELOG.md has no section for $version" >&2
  exit 1
fi
printf '%s\n' "$section"

# The names GitHub gave the uploads.
assets=""
if [ -n "$repo" ]; then
  assets="$(gh release view "$tag" --repo "$repo" --json assets -q '.assets[].name' 2> /dev/null || true)"
fi

# link LABEL GLOB prints "[LABEL](url)" for the first asset GLOB matches.
link() {
  local name
  while IFS= read -r name; do
    # shellcheck disable=SC2053 # the glob is meant to match
    if [[ -n "$name" && "$name" == $2 ]]; then
      printf '[%s](https://github.com/%s/releases/download/%s/%s)' "$1" "$repo" "$tag" "$name"
      return
    fi
  done <<< "$assets"
}

# row TITLE LINK... prints a list item of the links that exist, or nothing.
row() {
  local title="$1" out="" l
  shift
  for l in "$@"; do
    if [ -n "$l" ]; then out="${out:+$out | }$l"; fi
  done
  if [ -n "$out" ]; then printf -- '- %s：%s\n' "$title" "$out"; fi
}

if [ -n "$assets" ]; then
  printf '\n---\n\n## 📥 下载地址 / Downloads\n\n'

  printf '### Windows\n\n'
  row '安装版 Installer' "$(link '64 位 x64' '*-setup-*amd64.exe')"
  row '免安装版 Portable' "$(link '64 位 x64 (ZIP)' '*-windows-amd64-portable.zip')"

  printf '\n### macOS\n\n'
  row 'Apple 芯片与 Intel 芯片通用 Universal' "$(link 'DMG' '*-macos-universal.dmg')" "$(link 'ZIP' '*-macos-universal.zip')"

  printf '\n### Linux\n\n'
  row 'Debian / Ubuntu (DEB)' "$(link 'x64' '*_amd64.deb')"
  row 'Fedora / openSUSE / RHEL (RPM)' "$(link 'x64' '*.x86_64.rpm')"
  row '通用压缩包 Any distribution (tar.gz)' "$(link 'x64' '*-linux-amd64.tar.gz')"

  printf '\n### Android\n\n'
  row 'APK（Android 9 及以上 / Android 9 or later）' "$(link 'APK' '*-android.apk')"

  printf '\n### iOS\n\n'
  row '未签名 IPA，需自行签名安装 / Unsigned, sign it yourself to install' "$(link 'IPA' '*-ios-unsigned.ipa')"

  sums="$(link 'SHA256SUMS' 'SHA256SUMS')"
  if [ -n "$sums" ]; then
    printf '\n校验文件 Checksums：%s\n' "$sums"
  fi
fi

sed -e "s/{version}/$version/g" -e "s/{tag}/$tag/g" "$here/release-template.md"
