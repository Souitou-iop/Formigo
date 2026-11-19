#!/usr/bin/env python3
"""Copy the gopher pose images into assets/poses/<name>.png for the film.

Source: presentation/mascot/poses/NN-name.png (transparent, cropped; a white
#FFFFFF background also works: it is removed by flood-filling from the border
only, so white inside the gopher stays). Each pose is placed in a square,
centred and standing on the bottom edge, so every pose shares one ground line.
A pose whose image is not there yet uses the repo's assets/mascot.png (the
icon pose) as a placeholder. Re-run whenever new pose images arrive:

    python3 tools/sync-poses.py
"""
from pathlib import Path
from PIL import Image, ImageDraw, ImageFilter

HERE = Path(__file__).resolve().parent
FILM = HERE.parent
SRC = FILM.parent / "mascot" / "poses"
PLACEHOLDER = FILM.parent.parent / "assets" / "mascot.png"
OUT = FILM / "assets" / "poses"
SIZE = 768  # the largest gopher on screen is ~600 px

POSES = ["wave", "peek", "surprise", "juggle", "point", "shield", "think", "cheer", "laptop",
         "run", "devices", "palette", "magnifier", "bow", "sleep", "dive", "surf", "throw"]


def find(name):
    hits = sorted(SRC.glob(f"[0-9][0-9]-{name}.png")) if SRC.is_dir() else []
    return hits[0] if hits else None


def cut_white(im):
    im = im.convert("RGBA")
    w, h = im.size
    corners = [im.getpixel(p) for p in [(0, 0), (w - 1, 0), (0, h - 1), (w - 1, h - 1)]]
    if all(c[3] < 16 for c in corners):
        return im  # already transparent around the subject
    step = max(1, min(w, h) // 64)
    seeds = [(x, 0) for x in range(0, w, step)] + [(x, h - 1) for x in range(0, w, step)] + \
            [(0, y) for y in range(0, h, step)] + [(w - 1, y) for y in range(0, h, step)]
    for p in seeds:
        r, g, b, a = im.getpixel(p)
        if a > 0 and min(r, g, b) >= 235:
            ImageDraw.floodfill(im, p, (255, 255, 255, 0), thresh=40)
    # pull the alpha in by a pixel so no light fringe shows on the #F5F5F7 ground
    a = im.getchannel("A").filter(ImageFilter.MinFilter(3)).filter(ImageFilter.GaussianBlur(0.6))
    im.putalpha(a)
    return im


def main():
    OUT.mkdir(parents=True, exist_ok=True)
    report = []
    for name in POSES:
        src = find(name)
        im = cut_white(Image.open(src)) if src else Image.open(PLACEHOLDER).convert("RGBA")
        bbox = im.getchannel("A").getbbox()
        if bbox:
            im = im.crop(bbox)
        s = round(max(im.size) / 0.96)  # 2 % margin each side; feet 2 % above the bottom
        sq = Image.new("RGBA", (s, s), (0, 0, 0, 0))
        sq.paste(im, ((s - im.size[0]) // 2, s - im.size[1] - round(s * 0.02)))
        im = sq
        im = im.resize((SIZE, SIZE), Image.LANCZOS)
        im.save(OUT / f"{name}.png", optimize=True)
        report.append(f"{name:10s} {'← ' + src.name if src else '(placeholder mascot.png)'}")
    print("\n".join(report))


if __name__ == "__main__":
    main()
