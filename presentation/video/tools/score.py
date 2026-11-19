#!/usr/bin/env python3
"""Write the film's score (ABC, played by `om sound`) into index.html.

Light, warm and plucky: marimba + glockenspiel tune, ukulele off-beat strums,
pizzicato and vibes for colour, a warm pad, bass and drums. F major, 120 BPM,
one bar = 2 s, so every scene cut sits on a bar line. Each section names its
bars (chords, meter) and how much of the band plays; `!mark:x!` pins a section
to the timeline label `scene:x`.

    python3 tools/score.py && om sound .
"""
from pathlib import Path

FILM = Path(__file__).resolve().parent.parent / "index.html"

# chord -> (pad voicing, uke voicing, bass root, arpeggio notes)
CH = {
    "F":  ("[FAc]",  "[FAcf]",  "F,,", "F,A,cA,"),
    "Dm": ("[DFA]",  "[DFAd]",  "D,,", "D,F,AF,"),
    "Bb": ("[DFB]",  "[DFBd]",  "B,,,", "B,,D,FD,"),
    "C":  ("[EGc]",  "[EGce]",  "C,", "C,E,GE,"),
    "Am": ("[EAc]",  "[EAce]",  "A,,", "A,,C,EC,"),
    "Gm": ("[DGB]",  "[DGBd]",  "G,,", "G,,B,,DB,,"),
}

# melody bars (8 eighths), keyed by name
MEL = {
    "motif1": "F2 A2 c2 f2", "motif2": "d2 c2 A2 F2", "motif3": "G2 A2 c2 e2", "motif4": "f4 z4",
    "hookF": "F/A/c/f/ a2 g f c2", "hookDm": "D/F/A/d/ f2 e d A2", "hookBb": "B,/D/F/B/ d2 c B F2", "hookC": "C/E/G/c/ e2 g2 c2",
    "arpF": "F/A/c/f/ F/A/c/f/ F/A/c/f/ F/A/c/f/", "arpBb": "B,/D/F/B/ B,/D/F/B/ B,/D/F/B/ B,/D/F/B/",
    "arpC": "C/E/G/c/ C/E/G/c/ C/E/G/c/ C/E/G/c/", "arpDm": "D/F/A/d/ D/F/A/d/ D/F/A/d/ D/F/A/d/",
    "a1": "c2 A2 F2 A2", "a2": "d2 A2 F4", "a3": "d2 f2 d2 B2", "a4": "G2 A2 c4",
    "b1": "A2 c2 f2 e2", "b2": "e2 c2 A4", "b3": "B2 d2 f2 d2", "b4": "c2 B2 A2 G2",
    "s1": "c2 z2 A2 z2", "s2": "z8", "s3": "d2 z2 c2 z2", "s4": "z8",
    "logo": "F2 A2 c2 f2", "hold": "!fermata!f8", "rest": "z8",
}

# a section: (mark or None, [bars]); a bar: dict(ch, m=meter-eighths, mel, drum 0-3, bass 0-3, uke, pad, glk, pz, vib, crash, breath)
def bar(ch, mel="rest", drum=0, bass=0, uke=0, pad=1, glk=0, pz=0, vib=None, m=8, crash=False, breath=False, dyn=None):
    return dict(ch=ch, mel=mel, drum=drum, bass=bass, uke=uke, pad=pad, glk=glk, pz=pz, vib=vib, m=m, crash=crash, breath=breath, dyn=dyn)

S = []
S.append(("intro", [bar("F", "motif1", glk=1, dyn="mp"), bar("Bb", "motif2", glk=1), bar("C", "motif3", drum=1, glk=1)]))
S.append(("chaos", [bar("Dm", "arpDm", drum=2, bass=2, uke=2, crash=True, dyn="mf"), bar("Bb", "arpBb", drum=3, bass=2, uke=2),
                    bar("C", "arpC", drum=3, bass=3, uke=2, breath=True)]))
S.append(("feed", [bar("F", "hookF", 3, 3, 2, glk=1, crash=True, dyn="f"), bar("Dm", "hookDm", 3, 3, 2, glk=1),
                   bar("Bb", "hookBb", 3, 3, 2, glk=1), bar("C", "hookC", 3, 3, 2, glk=1),
                   bar("C", "motif3", 1, 1, 2, glk=1, m=4)]))   # half a bar: the last juggle lands
S.append(("demo", [bar("F", "logo", drum=1, bass=1, glk=1, dyn="mp"), bar("C", "motif3", drum=1, bass=1)]))
# under the recordings: a steady, lighter groove, the tune coming and going
prog1 = ["F", "Am", "Bb", "C"]
d12 = []
for i in range(15):
    ch = prog1[i % 4]
    grp = i // 4
    mel = (["s1", "s2", "s3", "s4"] if grp in (0, 2) else ["a1", "a2", "a3", "a4"])[i % 4] if grp < 3 else ["b1", "b2"][i % 2]
    d12.append(bar(ch, mel, drum=2, bass=2, uke=2, crash=(i == 0), dyn="mf" if i == 0 else None))
S.append(("d1", d12))
prog3 = ["Bb", "C", "Am", "Dm"]
d3 = []
vib = ["b1", "b2", "b3", "b4"]
for i in range(9):
    d3.append(bar(prog3[i % 4], "rest", drum=2, bass=2, uke=0, pz=1, vib=vib[i % 4], crash=(i == 0)))
S.append(("d3", d3))
prog4 = ["F", "Dm", "Bb", "C", "F", "Dm", "Bb", "C"]
d4 = []
for i, ch in enumerate(prog4):
    mel = ["a1", "a2", "a3", "a4", "b1", "b2", "arpBb", "arpC"][i]
    d4.append(bar(ch, mel, drum=2 if i < 6 else 3, bass=2 if i < 4 else 3, uke=2, glk=1 if i >= 6 else 0, crash=(i == 0), breath=(i == 7)))
S.append(("d4", d4))
S.append(("bento", [bar("F", "hookF", 3, 3, 2, glk=1, crash=True, dyn="f"), bar("Dm", "hookDm", 3, 3, 2, glk=1), bar("Bb", "hookBb", 3, 3, 2, glk=1),
                    bar("C", "hookC", 3, 3, 2, glk=1), bar("F", "hookF", 3, 3, 2, glk=1, breath=True)]))
S.append(("privacy", [bar("Bb", "s1", dyn="p"), bar("C", "s2"), bar("Dm", "motif2")]))
S.append(("platforms", [bar("F", "a1", 3, 3, 2, crash=True, dyn="mf"), bar("Am", "a2", 3, 3, 2), bar("Bb", "a3", 3, 3, 2), bar("C", "a4", 3, 3, 2),
                        bar("C", "rest", 1, 1, 0, m=2)]))
S.append(("end", [bar("F", "logo", 3, 3, 2, glk=1, crash=True, dyn="f"), bar("F", "hold", 0, 0, 0, glk=1), bar("F", "rest", 0, 0, 0, pad=0)]))


def scale(pattern, m):
    """Cut an 8-eighth bar down to m eighths (2/4 = 4, 1/4 = 2) by taking its start."""
    if m == 8:
        return pattern
    if pattern.startswith("z8"):
        return "z%d" % m
    out, n = [], 0
    for tok in pattern.split():
        out.append(tok)
        n += 2  # every token below is a quarter-length group
        if n >= m:
            break
    return " ".join(out)


def drums(b):
    lvl, m = b["drum"], b["m"]
    if lvl == 0:
        return "z%d" % m
    if lvl == 1:
        q = ["^F,,^F,,"] * 4
    elif lvl == 2:
        q = ["[C,,^F,,]^F,,", "[^C,,^F,,]^F,,", "[C,,^F,,]^F,,", "[^C,,^F,,]^F,,"]
    else:
        q = ["[C,,^F,,]^F,,", "[D,,^F,,][C,,^F,,]", "[C,,^F,,]^F,,", "[D,,^F,,]^A,,"]
    if b["crash"]:
        q[0] = "[C,,^C,]^F,,"
    if b["breath"]:
        q[3] = "z2"
    return " ".join(q[: m // 2])


def bass(b):
    r, m, lvl = CH[b["ch"]][2], b["m"], b["bass"]
    up = r.replace(",", "", 1) if "," in r else r.lower()
    if lvl == 0:
        return "z%d" % m
    pat = {1: [f"{r}4", f"{r}4"], 2: [f"{r}2", f"z{r}", f"{r}2", f"{r}2"], 3: [f"{r}2", f"z{r}", f"{up}{r}", f"z{r}"]}[lvl]
    if lvl == 1:
        return " ".join(pat[: max(1, m // 4)]) if m >= 4 else f"{r}{m}"
    if b["breath"]:
        pat[-1] = "z2"
    return " ".join(pat[: m // 2])


def uke(b):
    c, m = CH[b["ch"]][1], b["m"]
    if not b["uke"]:
        return "z%d" % m
    pat = [f"z{c}", f"z{c}", f"z{c}", f"z{c}"]
    if b["breath"]:
        pat[-1] = "z2"
    return " ".join(pat[: m // 2])


def pad(b):
    c, m = CH[b["ch"]][0], b["m"]
    return f"{c}{m}" if b["pad"] else f"z{m}"


def mel(b, key="mel"):
    name = b[key]
    if name is None:
        return "z%d" % b["m"]
    p = MEL[name]
    if b["breath"] and name not in ("hold",):
        # the last quarter goes silent before a drop
        toks = p.split()
        p = " ".join(toks[:-1]) + " z2" if len(toks) > 1 else p
    return scale(p, b["m"])


def pizz(b):
    a, m = CH[b["ch"]][3], b["m"]
    return f"{a} {a}" if (b["pz"] and m == 8) else ("z%d" % m)


def build():
    voices = {k: [] for k in ["MEL", "GLK", "VIB", "UKE", "PZ", "PAD", "BAS", "DR"]}
    first_meter = None
    for mark, bars in S:
        for i, b in enumerate(bars):
            m = b["m"]
            meter = {8: "4/4", 4: "2/4", 2: "1/4"}[m]
            pre = f"[M:{meter}] "
            dyn = f"!{b['dyn']}! " if b["dyn"] else ""
            mk = f"!mark:{mark}! " if (i == 0) else ""
            voices["MEL"].append(pre + dyn + mel(b))
            voices["GLK"].append(pre + dyn + (mel(b) if b["glk"] else "z%d" % m))
            voices["VIB"].append(pre + dyn + mel(b, "vib"))
            voices["UKE"].append(pre + dyn + uke(b))
            voices["PZ"].append(pre + dyn + pizz(b))
            voices["PAD"].append(pre + mk + dyn + pad(b))
            voices["BAS"].append(pre + dyn + bass(b))
            voices["DR"].append(pre + dyn + drums(b))
    head = {
        "MEL": "inst=marimba role=melody gain=0.9", "GLK": "inst=glock transpose=12 gain=0.35", "VIB": "inst=vibes gain=0.6",
        "UKE": "inst=uke gain=0.55", "PZ": "inst=pizz gain=0.5", "PAD": "inst=warmpad gain=0.4",
        "BAS": "inst=bass gain=0.8", "DR": "inst=drums kit=acoustic gain=0.75",
    }
    out = ["X:1", "T:Formigo · gopher pocket", "M:4/4", "L:1/8", "Q:1/4=120", "K:F"]
    for v, bars in voices.items():
        out.append(f"V:{v} {head[v]}")
        for k in range(0, len(bars), 4):
            out.append(" | ".join(bars[k:k + 4]) + " |")
    return "\n".join(out)


def main():
    abc = build()
    html = FILM.read_text()
    block = f'<script type="text/vnd.abc" id="om-score">\n{abc}\n</script>'
    start = html.find('<script type="text/vnd.abc" id="om-score">')
    if start >= 0:
        end = html.index("</script>", start) + len("</script>")
        html = html[:start] + block + html[end:]
    else:
        html = html.replace("<script>\nconst V = OM.vars();", block + "\n<script>\nconst V = OM.vars();", 1)
    FILM.write_text(html)
    print(abc.count("|"), "bar lines written")


if __name__ == "__main__":
    main()
