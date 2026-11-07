import os
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

from kit import BRASS, CREAM, cinzel, num  # noqa: E402
from options import chip_defs, chip_mark  # noqa: E402

HERE = os.path.dirname(os.path.abspath(__file__))
REPO = os.path.abspath(os.path.join(HERE, "..", "..", ".."))
CARDS = os.path.join(REPO, "client", "public", "cards", "french") + "/"
OUT = os.path.join(HERE, "build")


def card(face, cx, cy, w, rot):
    h = w * 1.4
    return (f'<g transform="translate({num(cx)} {num(cy)}) rotate({rot})">'
            f'<rect x="{num(-w / 2 + 2)}" y="{num(-h / 2 + 6)}" width="{num(w)}" height="{num(h)}" rx="{num(w * 0.05)}" fill="#000" opacity=".35" filter="url(#cs)"/>'
            f'<rect x="{num(-w / 2)}" y="{num(-h / 2)}" width="{num(w)}" height="{num(h)}" rx="{num(w * 0.05)}" fill="#fbf8ef" stroke="#00000033" stroke-width="1"/>'
            f'<image href="file://{CARDS}{face}.svg" x="{num(-w / 2)}" y="{num(-h / 2)}" width="{num(w)}" height="{num(h)}"/></g>')


def og_svg():
    W, H = 1200, 630
    defs = (chip_defs("og")
            + '<radialGradient id="felt" cx=".5" cy=".42" r=".75"><stop offset="0" stop-color="#22573f"/>'
              '<stop offset=".55" stop-color="#123a29"/><stop offset="1" stop-color="#081710"/></radialGradient>'
            + '<filter id="cs" x="-30%" y="-30%" width="160%" height="160%"><feGaussianBlur stdDeviation="9"/></filter>'
            + '<filter id="chipShadow" x="-30%" y="-30%" width="160%" height="160%">'
              '<feDropShadow dx="0" dy="14" stdDeviation="16" flood-color="#000" flood-opacity=".55"/></filter>'
            + '<radialGradient id="spot" cx=".5" cy=".38" r=".5"><stop offset="0" stop-color="#fff" stop-opacity=".09"/>'
              '<stop offset="1" stop-color="#fff" stop-opacity="0"/></radialGradient>')
    parts = [f'<rect width="{W}" height="{H}" fill="url(#felt)"/>',
             f'<rect width="{W}" height="{H}" fill="url(#spot)"/>',
             f'<rect x="18" y="18" width="{W - 36}" height="{H - 36}" rx="22" fill="none" stroke="{BRASS}" stroke-opacity=".35" stroke-width="1.5"/>']
    fan = [("AH", -36), ("JH", -18), ("TH", 0), ("9H", 18), ("KH", 36)]
    px, py = 600, 590
    for face, rot in fan:
        parts.append(f'<g transform="rotate({rot} {px} {py})">{card(face, px, py - 330, 150, 0)}</g>')
    parts.append(f'<g filter="url(#chipShadow)">{chip_mark("og", 600, 250, 146)}</g>')
    name = cinzel(700)
    parts.append(f'<path d="{name.path("BELJOT", 62, 600, 532, tracking=14, anchor="middle")}" fill="{CREAM}"/>')
    sub = cinzel(600)
    parts.append(f'<path d="{sub.path("PLAY BELJOT ONLINE", 18, 600, 570, tracking=7, anchor="middle")}" fill="{BRASS}"/>')
    rule_w = 130
    parts.append(f'<rect x="{600 - 300 - rule_w / 2}" y="563" width="{rule_w}" height="1.5" fill="{BRASS}" opacity=".7"/>')
    parts.append(f'<rect x="{600 + 300 - rule_w / 2}" y="563" width="{rule_w}" height="1.5" fill="{BRASS}" opacity=".7"/>')
    return (f'<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" width="{W}" height="{H}" '
            f'viewBox="0 0 {W} {H}"><defs>{defs}</defs>{"".join(parts)}</svg>')


if __name__ == "__main__":
    os.makedirs(OUT, exist_ok=True)
    open(f"{OUT}/og.svg", "w").write(og_svg())
    print("ok")
