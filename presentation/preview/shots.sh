#!/bin/zsh
# Render each slide alone at 1920x1080 after 4s of (virtual) animation time.
H=/Users/ebato/Library/Caches/ms-playwright/chromium_headless_shell-1255/chrome-headless-shell-mac-arm64/chrome-headless-shell
OUT=${OUT:-/private/tmp/claude-501/-Users-ebato--claude-spares-spare-PrtzVR/3b1c897a-30e9-4b5e-a274-cefc91f12534/scratchpad/shots}
mkdir -p $OUT
for id in "$@"; do
  node build.mjs $id 1 && cp index.html one-$id.html
  $H --headless --disable-gpu --hide-scrollbars --allow-file-access-from-files --window-size=1920,1080 --virtual-time-budget=${T:-4000} --screenshot=$OUT/$id.png file://$PWD/one-$id.html >/dev/null 2>&1
  rm one-$id.html
done
