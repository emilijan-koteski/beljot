import os
import random
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

from kit import BRASS, CREAM, cinzel, num  # noqa: E402
from options import chip_defs, chip_mark  # noqa: E402

HERE = os.path.dirname(os.path.abspath(__file__))
MASCOT = os.path.join(HERE, "..", "mascot") + "/"
OUT = os.path.join(HERE, "build")
W, H = 1200, 630
SIZES = {"dama-cards-half": (758, 768), "dama-welcome": (986, 1600)}


def defs(spot_x, spot_y):
    return (chip_defs("og")
            + '<radialGradient id="felt" cx=".5" cy=".42" r=".8"><stop offset="0" stop-color="#22573f"/>'
              '<stop offset=".55" stop-color="#123a29"/><stop offset="1" stop-color="#071510"/></radialGradient>'
            + f'<radialGradient id="spot" gradientUnits="userSpaceOnUse" cx="{spot_x}" cy="{spot_y}" r="420">'
              '<stop offset="0" stop-color="#f3d9a0" stop-opacity=".28"/><stop offset=".5" stop-color="#e0b56a" stop-opacity=".08"/>'
              '<stop offset="1" stop-color="#e0b56a" stop-opacity="0"/></radialGradient>'
            + '<filter id="bokeh" x="-50%" y="-50%" width="200%" height="200%"><feGaussianBlur stdDeviation="2.2"/></filter>'
            + '<filter id="lift" x="-30%" y="-30%" width="160%" height="160%">'
              '<feDropShadow dx="0" dy="12" stdDeviation="14" flood-color="#000" flood-opacity=".5"/></filter>')


def bokeh(seed, region, n=26):
    rnd = random.Random(seed)
    x0, y0, x1, y1 = region
    dots = []
    for _ in range(n):
        r = rnd.choice([1.5, 2, 2.5, 3, 4, 5])
        op = rnd.uniform(0.25, 0.75) if r < 3.5 else rnd.uniform(0.12, 0.3)
        dots.append(f'<circle cx="{num(rnd.uniform(x0, x1))}" cy="{num(rnd.uniform(y0, y1))}" r="{r}" fill="#f2d48f" opacity="{num(op)}"/>')
    return f'<g filter="url(#bokeh)">{"".join(dots)}</g>'


def dama(name, x, y, height):
    sw, sh = SIZES[name]
    width = sw * height / sh
    return f'<image href="file://{MASCOT}{name}.webp" x="{num(x)}" y="{num(y)}" width="{num(width)}" height="{num(height)}"/>'


def wordmark(cx, base, size, tag):
    return (f'<path d="{cinzel(700).path("BELJOT", size, cx, base, tracking=size * 0.22, anchor="middle")}" fill="{CREAM}"/>'
            f'<path d="{cinzel(600).path("PLAY BELJOT ONLINE", tag, cx, base + 40, tracking=tag * 0.38, anchor="middle")}" fill="{BRASS}"/>')


def page(body, spot):
    frame = f'<rect x="18" y="18" width="{W - 36}" height="{H - 36}" rx="22" fill="none" stroke="{BRASS}" stroke-opacity=".35" stroke-width="1.5"/>'
    return (f'<svg xmlns="http://www.w3.org/2000/svg" width="{W}" height="{H}" viewBox="0 0 {W} {H}">'
            f'<defs>{defs(*spot)}</defs><rect width="{W}" height="{H}" fill="url(#felt)"/><rect width="{W}" height="{H}" fill="url(#spot)"/>'
            f'{body}{frame}</svg>')


def trumps():
    sw, sh = SIZES["dama-cards-half"]
    height = 640
    x = W - 40 - sw * height / sh
    body = (bokeh(3, (620, 40, 1160, 560))
            + dama("dama-cards-half", x, H - height + 8, height)
            + f'<g filter="url(#lift)">{chip_mark("og", 360, 236, 140)}</g>'
            + bokeh(4, (120, 60, 600, 400), 12)
            + wordmark(360, 486, 56, 17))
    return page(body, (860, 230))


def presents():
    body = (bokeh(1, (40, 40, 640, 560))
            + dama("dama-welcome", 70, 22, 1010)
            + f'<g filter="url(#lift)">{chip_mark("og", 860, 236, 132)}</g>'
            + bokeh(2, (700, 60, 1150, 420), 14)
            + wordmark(860, 486, 54, 17))
    return page(body, (380, 210))


if __name__ == "__main__":
    os.makedirs(OUT, exist_ok=True)
    layout = sys.argv[1] if len(sys.argv) > 1 else "presents"
    with open(os.path.join(OUT, "og.svg"), "w") as fh:
        fh.write({"trumps": trumps, "presents": presents}[layout]())
    print("ok", layout)
