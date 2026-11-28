import os
import random

from kit import BRASS, CREAM, RED, cinzel, cormorant, num
from options import chip_defs, chip_mark

HERE = os.path.dirname(os.path.abspath(__file__))
REPO = os.path.abspath(os.path.join(HERE, "..", "..", ".."))
MASCOT = os.path.join(HERE, "..", "mascot")
CARDS = os.path.join(REPO, "client", "public", "cards", "french")
OUT = os.path.join(HERE, "build", "social")

MASCOT_SIZES = {"dama-welcome": (986, 1600), "dama-dealing": (1053, 1600), "dama-signature": (993, 1600)}
SHOTS = os.path.join(HERE, "build", "social")
SHOT_SIZE = (780, 1688)
CARD_RATIO = 336 / 240

TAGLINE_MK = "Седни на масата."
TAGLINE_HR = "Sjedni za stol."
URL = "BELJOT.ONLINE"


def defs(w, h, spot):
    sx, sy, sr = spot
    return (chip_defs("s")
            + '<radialGradient id="felt" cx=".5" cy=".45" r=".8"><stop offset="0" stop-color="#22573f"/>'
              '<stop offset=".55" stop-color="#123a29"/><stop offset="1" stop-color="#071510"/></radialGradient>'
            + f'<radialGradient id="spot" gradientUnits="userSpaceOnUse" cx="{sx}" cy="{sy}" r="{sr}">'
              '<stop offset="0" stop-color="#f3d9a0" stop-opacity=".26"/><stop offset=".5" stop-color="#e0b56a" stop-opacity=".07"/>'
              '<stop offset="1" stop-color="#e0b56a" stop-opacity="0"/></radialGradient>'
            + '<filter id="bokeh" x="-50%" y="-50%" width="200%" height="200%"><feGaussianBlur stdDeviation="2.2"/></filter>'
            + '<filter id="lift" x="-30%" y="-30%" width="160%" height="160%">'
              '<feDropShadow dx="0" dy="12" stdDeviation="14" flood-color="#000" flood-opacity=".5"/></filter>'
            + '<filter id="soft" x="-30%" y="-30%" width="160%" height="160%">'
              '<feDropShadow dx="0" dy="6" stdDeviation="8" flood-color="#000" flood-opacity=".45"/></filter>')


def bokeh(seed, region, n=26, scale=1.0):
    rnd = random.Random(seed)
    x0, y0, x1, y1 = region
    dots = []
    for _ in range(n):
        r = rnd.choice([1.5, 2, 2.5, 3, 4, 5]) * scale
        op = rnd.uniform(0.25, 0.75) if r < 3.5 * scale else rnd.uniform(0.12, 0.3)
        dots.append(f'<circle cx="{num(rnd.uniform(x0, x1))}" cy="{num(rnd.uniform(y0, y1))}" r="{num(r)}" fill="#f2d48f" opacity="{num(op)}"/>')
    return f'<g filter="url(#bokeh)">{"".join(dots)}</g>'


def canvas(w, h, body, spot, background=True):
    ground = f'<rect width="{w}" height="{h}" fill="url(#felt)"/><rect width="{w}" height="{h}" fill="url(#spot)"/>' if background else ""
    return (f'<svg xmlns="http://www.w3.org/2000/svg" width="{w}" height="{h}" viewBox="0 0 {w} {h}">'
            f'<defs>{defs(w, h, spot)}</defs>{ground}{body}</svg>')


def image(path, x, y, w, h, extra=""):
    return f'<image href="file://{path}" x="{num(x)}" y="{num(y)}" width="{num(w)}" height="{num(h)}" {extra}/>'


def dama(name, x, y, height):
    sw, sh = MASCOT_SIZES[name]
    return image(os.path.join(MASCOT, f"{name}.webp"), x, y, sw * height / sh, height)


def card(card_id, cx, cy, width, rot):
    h = width * CARD_RATIO
    return (f'<g transform="rotate({rot} {num(cx)} {num(cy)})" filter="url(#soft)">'
            f'<rect x="{num(cx - width / 2)}" y="{num(cy - h / 2)}" width="{num(width)}" height="{num(h)}" rx="{num(width * 0.06)}" fill="#fff"/>'
            + image(os.path.join(CARDS, f"{card_id}.svg"), cx - width / 2, cy - h / 2, width, h, 'preserveAspectRatio="none"')
            + "</g>")


def fan_with_chip(cx, cy, width):
    """The trump fan with the brand chip resting on its lower centre."""
    chip_cy = cy + width * CARD_RATIO * 0.48
    return trump_fan(cx, cy, width) + f'<g filter="url(#lift)">{chip_mark("s", cx, chip_cy, width * 0.7)}</g>'


def trump_fan(cx, cy, width):
    """Jack and nine of the trump suit, the two strongest cards in Belot, fanned with the ace."""
    spread = width * 0.42
    return (card("AH", cx - spread, cy + width * 0.08, width, -14)
            + card("9H", cx, cy - width * 0.04, width, 0)
            + card("JH", cx + spread, cy + width * 0.08, width, 14))


def bot_badge(cx, cy, r):
    """The app's bot icon (Lucide, ISC) in brass on a felt disc with a brass ring."""
    s = r * 1.3 / 24
    ox, oy = cx - 12 * s, cy - 12.5 * s
    stroke = f'fill="none" stroke="url(#sb)" stroke-width="{num(2 * s)}" stroke-linecap="round" stroke-linejoin="round"'
    icon = (f'<g transform="translate({num(ox)} {num(oy)}) scale({num(s)})" fill="none" stroke="{BRASS}" '
            f'stroke-width="2" stroke-linecap="round" stroke-linejoin="round">'
            '<path d="M12 8V4H8"/><rect width="16" height="12" x="4" y="8" rx="2"/>'
            '<path d="M2 14h2"/><path d="M20 14h2"/><path d="M15 13v2"/><path d="M9 13v2"/></g>')
    return (f'<g filter="url(#lift)"><circle cx="{num(cx)}" cy="{num(cy)}" r="{num(r)}" fill="url(#sg)"/>'
            f'<circle cx="{num(cx)}" cy="{num(cy)}" r="{num(r)}" {stroke}/>'
            f'<circle cx="{num(cx)}" cy="{num(cy)}" r="{num(r * 0.88)}" fill="none" stroke="url(#sb)" stroke-opacity=".5" stroke-width="{num(s * 0.6)}"/>'
            f'{icon}</g>')


def phone(name, x, y, width):
    sw, sh = SHOT_SIZE
    h = width * sh / sw
    r = width * 0.11
    bezel = width * 0.035
    clip = f"clip-{name}"
    return (f'<clipPath id="{clip}"><rect x="{num(x)}" y="{num(y)}" width="{num(width)}" height="{num(h)}" rx="{num(r)}"/></clipPath>'
            f'<rect x="{num(x - bezel)}" y="{num(y - bezel)}" width="{num(width + 2 * bezel)}" height="{num(h + 2 * bezel)}" '
            f'rx="{num(r + bezel)}" fill="#050c08" stroke="{BRASS}" stroke-opacity=".55" stroke-width="2" filter="url(#lift)"/>'
            + f'<g clip-path="url(#{clip})">{image(os.path.join(SHOTS, f"{name}.png"), x, y, width, h)}</g>')


def line(text, cx, base, size, fill, weight=600, tracking=0.0):
    return f'<path d="{cormorant(weight).path(text, size, cx, base, tracking=size * tracking, anchor="middle")}" fill="{fill}"/>'


def brand(cx, base, size):
    """BELJOT.ONLINE sign-off in cream Cinzel caps, set apart from the brass Croatian line."""
    return caps(URL, cx, base, size, CREAM)


def caps(text, cx, base, size, fill=BRASS):
    return f'<path d="{cinzel(600).path(text, size, cx, base, tracking=size * 0.38, anchor="middle")}" fill="{fill}"/>'


def divider(cx, y, half):
    gap = half * 0.16
    return (f'<line x1="{num(cx - half)}" y1="{num(y)}" x2="{num(cx - gap)}" y2="{num(y)}" stroke="{BRASS}" stroke-opacity=".6" stroke-width="{num(half * 0.012)}"/>'
            f'<line x1="{num(cx + gap)}" y1="{num(y)}" x2="{num(cx + half)}" y2="{num(y)}" stroke="{BRASS}" stroke-opacity=".6" stroke-width="{num(half * 0.012)}"/>'
            f'<circle cx="{num(cx - gap * 0.45)}" cy="{num(y)}" r="{num(half * 0.025)}" fill="{RED}"/>'
            f'<circle cx="{num(cx + gap * 0.45)}" cy="{num(y)}" r="{num(half * 0.025)}" fill="{CREAM}"/>')


def bilingual(cx, top, mk, hr, mk_size, hr_size=None):
    """Macedonian line in cream above the Croatian line in brass, split by the suit divider."""
    hr_size = hr_size or mk_size * 0.72
    mk_base = top + mk_size * 0.78
    rule = mk_base + mk_size * 0.42
    hr_base = rule + hr_size * 1.05
    return (line(mk, cx, mk_base, mk_size, CREAM)
            + divider(cx, rule, mk_size * 2.2)
            + line(hr, cx, hr_base, hr_size, BRASS))


def profile(size):
    c = size / 2
    body = (bokeh(11, (0, 0, size, size), 18, size / 1080)
            + f'<g filter="url(#lift)">{chip_mark("s", c, c, size * 0.4)}</g>')
    return canvas(size, size, body, (c, c * 0.8, size * 0.6))


def watermark():
    return canvas(150, 150, chip_mark("s", 75, 75, 73, text=False), (75, 75, 1), background=False)


def facebook_cover():
    w, h = 1640, 624
    cx = w / 2
    body = (bokeh(21, (0, 0, w, h), 40)
            + fan_with_chip(340, 250, 130)
            + dama("dama-welcome", 1110, 30, 900)
            + bilingual(cx, 190, TAGLINE_MK, TAGLINE_HR, 80, 50)
            + brand(cx, 450, 30))
    return canvas(w, h, body, (cx, 260, 620))


def youtube_banner():
    w, h = 2560, 1440
    cx = w / 2
    body = (bokeh(31, (0, 300, w, 1140), 70, 1.4)
            + fan_with_chip(780, 650, 150)
            + dama("dama-welcome", 2040, 500, 1100)
            + bilingual(1400, 560, TAGLINE_MK, TAGLINE_HR, 100, 62)
            + brand(1400, 875, 36))
    return canvas(w, h, body, (cx, 720, 1100))


def post_welcome():
    w, h = 1080, 1350
    cx = w / 2
    body = (bokeh(41, (0, 0, w, h), 40, 1.3)
            + bilingual(cx, 150, TAGLINE_MK, TAGLINE_HR, 96, 64)
            + dama("dama-welcome", 70, 470, 940)
            + f'<g filter="url(#lift)">{chip_mark("s", 790, 760, 160)}</g>'
            + line("2 на 2 · бесплатно", 790, 1010, 44, CREAM)
            + line("2 na 2 · besplatno", 790, 1066, 38, BRASS)
            + brand(790, 1160, 26))
    return canvas(w, h, body, (cx, 520, 800))


def post_decks():
    w, h = 1080, 1350
    cx = w / 2
    width = 330
    body = (bokeh(51, (0, 0, w, h), 34, 1.3)
            + bilingual(cx, 90, "Француски или хрватски карти?", "Francuske ili hrvatske karte?", 58, 46)
            + phone("match-phone-french", cx - width - 40, 330, width)
            + phone("match-phone-croatian", cx + 40, 330, width)
            + line("Ти избираш.", cx, 1180, 54, CREAM)
            + line("Ti biraš.", cx, 1240, 42, BRASS)
            + brand(cx, 1305, 24))
    return canvas(w, h, body, (cx, 640, 760))


def post_bots():
    w, h = 1080, 1350
    cx = w / 2
    body = (bokeh(61, (0, 0, w, h), 40, 1.3)
            + bilingual(cx, 100, "Фали четврти?", "Fali četvrti?", 86, 64)
            + dama("dama-dealing", 400, 310, 820)
            + bot_badge(240, 720, 130)
            + line("Ботот седнува веднаш.", cx, 1180, 50, CREAM)
            + line("Bot sjeda odmah.", cx, 1238, 40, BRASS)
            + brand(cx, 1305, 24))
    return canvas(w, h, body, (cx, 620, 760))


GRID_TILE = (1080, 1440)


def stacked(lines, cx, first_base, size, fill, leading=1.0):
    return "".join(line(text, cx, first_base + i * size * leading, size, fill) for i, text in enumerate(lines))


def instagram_grid():
    """One 3x3 mosaic of 3:4 tiles; the right column carries the chip, taglines and sign-off one tile each."""
    tw, th = GRID_TILE
    w, h = tw * 3, th * 3
    col3 = tw * 2.5
    body = (bokeh(71, (0, 0, w, h), 120, 3)
            + dama("dama-welcome", 542, 407, 3300)
            + f'<g filter="url(#lift)">{chip_mark("s", col3, 700, 360)}</g>'
            + stacked(["Седни на", "масата."], col3, th + 560, 150, CREAM)
            + divider(col3, th + 820, 330)
            + line(TAGLINE_HR, col3, th + 960, 96, BRASS)
            + trump_fan(tw * 0.5, 760, 250)
            + line("2 на 2 · бесплатно", col3, th * 2 + 600, 92, CREAM)
            + line("2 na 2 · besplatno", col3, th * 2 + 720, 74, BRASS)
            + brand(col3, th * 2 + 940, 46))
    return canvas(w, h, body, (w * 0.45, h * 0.3, w * 0.75))


JOBS = {
    "profile-1080": (profile(1080), 1080, 1080, "png"),
    "profile-800": (profile(800), 800, 800, "png"),
    "watermark-150": (watermark(), 150, 150, "png"),
    "facebook-cover-1640x624": (facebook_cover(), 1640, 624, "png"),
    "youtube-banner-2560x1440": (youtube_banner(), 2560, 1440, "jpeg"),
    "post-01-welcome": (post_welcome(), 1080, 1350, "jpeg"),
    "post-02-decks": (post_decks(), 1080, 1350, "jpeg"),
    "post-03-bots": (post_bots(), 1080, 1350, "jpeg"),
    "instagram-grid": (instagram_grid(), 3240, 4320, "jpeg"),
}


if __name__ == "__main__":
    import json

    os.makedirs(OUT, exist_ok=True)
    manifest = []
    for name, (svg, w, h, kind) in JOBS.items():
        path = os.path.join(OUT, f"{name}.svg")
        with open(path, "w") as fh:
            fh.write(svg)
        job = {"name": name, "svg": path, "width": w, "height": h, "type": kind}
        if name == "instagram-grid":
            job["tiles"] = {"width": GRID_TILE[0], "height": GRID_TILE[1], "dir": "instagram-grid"}
        manifest.append(job)
    with open(os.path.join(OUT, "jobs.json"), "w") as fh:
        json.dump(manifest, fh, indent=1)
    print("ok", len(manifest))
