import json
import os
import sys

sys.path.insert(0, os.path.dirname(__file__))

from kit import BRASS, FELT, cinzel, felt_grad, num, suit  # noqa: E402
from options import chip_defs, chip_lockup, chip_mark, shadow, svg  # noqa: E402

HERE = os.path.dirname(os.path.abspath(__file__))
REPO = os.path.abspath(os.path.join(HERE, "..", "..", ".."))
PUB = REPO + "/client/public"
BRAND = REPO + "/docs/brand"
BUILD = os.path.join(HERE, "build")
PNG_JOBS = os.path.join(BUILD, "png_jobs.json")


def labelled(s):
    return s.replace("<svg ", '<svg role="img" aria-label="Beljot" ', 1)


def compact_chip():
    return labelled(svg((0, 0, 64, 64), chip_mark("k", 32, 32, 31.5, text=False), defs=chip_defs("k")))


def full_chip():
    return labelled(svg((0, 0, 120, 120), chip_mark("f", 60, 60, 59), defs=chip_defs("f")))


def circle_path(cx, cy, r):
    return f"M{num(cx - r)},{num(cy)}a{num(r)},{num(r)} 0 1,0 {num(2 * r)},0a{num(r)},{num(r)} 0 1,0 {num(-2 * r)},0Z"


def mono_chip(color, uid):
    cx = cy = 60
    s = 59 / 58
    from kit import bodoni
    inserts = "".join(
        f'<rect x="{num(cx - 5 * s)}" y="{num(cy - 60 * s)}" width="{num(10 * s)}" height="{num(11 * s)}" rx="{num(1.5 * s)}" '
        f'transform="rotate({i * 45 + 22.5} {cx} {cy})"/>' for i in range(8))
    mask = (f'<mask id="{uid}m" maskUnits="userSpaceOnUse" x="0" y="0" width="120" height="120">'
            f'<rect width="120" height="120" fill="#fff"/><g fill="#000">{inserts}'
            f'<path d="{bodoni().centered("B", 34 * s, cx, cy + 0.5 * s)}"/></g>'
            f'<circle cx="{cx}" cy="{cy}" r="{num(24.4 * s)}" fill="none" stroke="#000" stroke-width="{num(0.9 * s)}"/></mask>')
    f = cinzel(700)
    body = (f'<g fill="{color}" mask="url(#{uid}m)">'
            f'<path fill-rule="evenodd" d="{circle_path(cx, cy, 58 * s)}{circle_path(cx, cy, 48.5 * s)}"/>'
            f'<circle cx="{cx}" cy="{cy}" r="{num(46 * s)}" fill="none" stroke="{color}" stroke-width="{num(0.9 * s)}"/>'
            f'<path d="{f.arc("BELJOT", 9.6 * s, cx, cy, 36.2 * s, 0, tracking=2.6 * s)}"/>'
            f'<path d="{f.arc("ONLINE", 7.4 * s, cx, cy, 42.8 * s, 0, tracking=2.4 * s, bottom=True)}"/>'
            f'<path d="{suit("H", 7.5 * s, cx - 39.6 * s, cy)}"/><path d="{suit("S", 7.5 * s, cx + 39.6 * s, cy)}"/>'
            f'<circle cx="{cx}" cy="{cy}" r="{num(27 * s)}"/></g>')
    return labelled(svg((0, 0, 120, 120), body, defs=mask))


def apple_touch_icon():
    defs = felt_grad("ag") + chip_defs("a") + shadow("as", 3, 5, 0.45)
    body = f'<rect width="180" height="180" fill="url(#ag)"/><g filter="url(#as)">{chip_mark("a", 90, 90, 74)}</g>'
    return svg((0, 0, 180, 180), body, defs=defs)


def wordmark_paths():
    name_f, suf_f = cinzel(700), cinzel(700)
    size, track = 100, 8
    name = name_f.path("BELJOT", size, 0, 0, tracking=track)
    nx0, ny0, nx1, ny1 = name_f.bounds("BELJOT", size, track)
    gap = 7
    sx = nx1 + gap
    ssize, strack = 50, 4.5
    suffix = suf_f.path(".ONLINE", ssize, sx, 0, tracking=strack)
    sx0, sy0, sx1, sy1 = suf_f.bounds(".ONLINE", ssize, strack)
    pad = 1
    full = (nx0 - pad, ny0 - pad, (sx + sx1) - nx0 + 2 * pad, ny1 - ny0 + 2 * pad)
    only = (nx0 - pad, ny0 - pad, nx1 - nx0 + 2 * pad, ny1 - ny0 + 2 * pad)
    vb = lambda b: " ".join(num(v) for v in b)
    return {"viewBox": vb(full), "nameViewBox": vb(only), "aspect": round(full[2] / full[3], 4),
            "nameAspect": round(only[2] / only[3], 4), "name": name, "suffix": suffix}


def write(path, text):
    os.makedirs(os.path.dirname(path), exist_ok=True)
    with open(path, "w") as fh:
        fh.write(text)


if __name__ == "__main__":
    os.makedirs(BUILD, exist_ok=True)
    write(f"{PUB}/beljot-logo.svg", compact_chip() + "\n")
    write(f"{PUB}/beljot-chip.svg", full_chip() + "\n")
    write(f"{BRAND}/beljot-chip.svg", full_chip() + "\n")
    write(f"{BRAND}/beljot-chip-compact.svg", compact_chip() + "\n")
    write(f"{BRAND}/beljot-chip-black.svg", mono_chip("#000000", "b") + "\n")
    write(f"{BRAND}/beljot-chip-white.svg", mono_chip("#ffffff", "w") + "\n")
    write(f"{BRAND}/beljot-chip-gold.svg", mono_chip(BRASS, "g") + "\n")
    write(f"{BRAND}/beljot-lockup-on-dark.svg", labelled(chip_lockup("dark", "ld")) + "\n")
    write(f"{BRAND}/beljot-lockup-on-light.svg", labelled(chip_lockup("light", "ll")) + "\n")
    write(f"{BUILD}/apple-touch-icon.svg", apple_touch_icon())
    wm = wordmark_paths()
    ts = ("export const WORDMARK = {\n"
          f'  viewBox: "{wm["viewBox"]}",\n'
          f'  nameViewBox: "{wm["nameViewBox"]}",\n'
          f'  aspect: {wm["aspect"]},\n'
          f'  nameAspect: {wm["nameAspect"]},\n'
          f'  name: "{wm["name"]}",\n'
          f'  suffix: "{wm["suffix"]}",\n'
          "} as const;\n")
    write(f"{REPO}/client/src/shared/components/brand/wordmarkPaths.ts", ts)
    json.dump([
        {"svg": f"{PUB}/beljot-logo.svg", "png": f"{PUB}/favicon-32.png", "size": 32},
        {"svg": f"{BUILD}/apple-touch-icon.svg", "png": f"{PUB}/apple-touch-icon.png", "size": 180},
        {"svg": f"{BRAND}/beljot-chip.svg", "png": f"{BRAND}/beljot-chip-1024.png", "size": 1024},
    ], open(PNG_JOBS, "w"))
    print("ok", len(wm["name"]) + len(wm["suffix"]), "bytes of wordmark path data")
