import io
import math
import os

import uharfbuzz as hb
from fontTools.pens.boundsPen import BoundsPen
from fontTools.pens.svgPathPen import SVGPathPen
from fontTools.pens.transformPen import TransformPen
from fontTools.ttLib import TTFont
from fontTools.varLib import instancer

FONTS = os.path.join(os.path.dirname(os.path.abspath(__file__)), "fonts") + "/"


def num(v):
    return f"{v:.2f}".rstrip("0").rstrip(".")


class Face:
    def __init__(self, path, axes=None):
        tt = TTFont(path)
        if axes and "fvar" in tt:
            tt = instancer.instantiateVariableFont(tt, axes)
        self.tt = tt
        self.upem = tt["head"].unitsPerEm
        self.gs = tt.getGlyphSet()
        self.order = tt.getGlyphOrder()
        buf = io.BytesIO()
        tt.save(buf)
        self.hb = hb.Font(hb.Face(buf.getvalue()))

    def shape(self, text, features=None):
        buf = hb.Buffer()
        buf.add_str(text)
        buf.guess_segment_properties()
        hb.shape(self.hb, buf, features or {"kern": True, "liga": True})
        return buf.glyph_infos, buf.glyph_positions

    def width(self, text, size, tracking=0):
        s = size / self.upem
        infos, pos = self.shape(text)
        return sum(p.x_advance for p in pos) * s + tracking * (len(infos) - 1)

    def path(self, text, size, x=0, y=0, tracking=0, anchor="start"):
        s = size / self.upem
        infos, pos = self.shape(text)
        total = self.width(text, size, tracking)
        if anchor == "middle":
            x -= total / 2
        elif anchor == "end":
            x -= total
        pen = SVGPathPen(self.gs, ntos=num)
        cx = 0
        for info, p in zip(infos, pos):
            name = self.order[info.codepoint]
            self.gs[name].draw(TransformPen(pen, (s, 0, 0, -s, x + cx + p.x_offset * s, y - p.y_offset * s)))
            cx += p.x_advance * s + tracking
        return pen.getCommands()

    def bounds(self, text, size, tracking=0):
        """Ink bounds (x0, y0, x1, y1) in SVG coords for text set at origin baseline 0."""
        s = size / self.upem
        infos, pos = self.shape(text)
        bp = BoundsPen(self.gs)
        cx = 0
        for info, p in zip(infos, pos):
            name = self.order[info.codepoint]
            self.gs[name].draw(TransformPen(bp, (s, 0, 0, -s, cx + p.x_offset * s, -p.y_offset * s)))
            cx += p.x_advance * s + tracking
        return bp.bounds

    def centered(self, text, size, cx, cy, tracking=0):
        """Path whose ink box is centred on (cx, cy)."""
        x0, y0, x1, y1 = self.bounds(text, size, tracking)
        return self.path(text, size, cx - (x0 + x1) / 2, cy - (y0 + y1) / 2, tracking)

    def arc(self, text, size, cx, cy, r, center_deg=0, tracking=0, bottom=False):
        """Text along a circle: on top reading clockwise, or on the bottom reading upright."""
        s = size / self.upem
        infos, pos = self.shape(text)
        advs = [p.x_advance * s for p in pos]
        total = sum(advs) + tracking * (len(advs) - 1)
        span = total / r
        pen = SVGPathPen(self.gs, ntos=num)
        acc = 0
        for info, adv in zip(infos, advs):
            mid = acc + adv / 2
            ox = -adv / 2
            name = self.order[info.codepoint]
            if not bottom:
                th = math.radians(center_deg) - span / 2 + mid / r
                px, py = cx + r * math.sin(th), cy - r * math.cos(th)
                m = (s * math.cos(th), s * math.sin(th), s * math.sin(th), -s * math.cos(th),
                     px + ox * math.cos(th), py + ox * math.sin(th))
            else:
                ps = math.radians(center_deg) - span / 2 + mid / r
                px, py = cx + r * math.sin(ps), cy + r * math.cos(ps)
                m = (s * math.cos(ps), -s * math.sin(ps), -s * math.sin(ps), -s * math.cos(ps),
                     px + ox * math.cos(ps), py - ox * math.sin(ps))
            self.gs[name].draw(TransformPen(pen, m))
            acc += adv + tracking
        return pen.getCommands()


_cache = {}


def face(name, **axes):
    key = (name, tuple(sorted(axes.items())))
    if key not in _cache:
        path = name if name.startswith("/") else FONTS + name + ".ttf"
        _cache[key] = Face(path, axes or None)
    return _cache[key]


def bodoni(w=700):
    return face("BodoniModa", wght=w, opsz=96)


def playfair(w=700):
    return face("PlayfairDisplay", wght=w)


def cinzel(w=600):
    return face("Cinzel", wght=w)


def cormorant(w=600):
    return face("CormorantGaramond", wght=w)


SUITS = {"S": "♠", "H": "♥", "D": "♦", "C": "♣"}


def suit_face():
    return face("/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf")


def suit(s, size, cx, cy):
    return suit_face().centered(SUITS[s], size, cx, cy)


FELT = "#0b1f15"
FELT_2 = "#13372a"
FELT_EDGE = "#1d4a36"
CREAM = "#f6f1e3"
CREAM_2 = "#efe6cf"
INK = "#0e1a14"
RED = "#b4262f"
BRASS = "#c9a876"
BRASS_HI = "#efdcaa"
BRASS_LO = "#94733f"
LIME = "#00e5a0"


def brass_grad(gid, x1="0", y1="0", x2="0", y2="1"):
    return (f'<linearGradient id="{gid}" x1="{x1}" y1="{y1}" x2="{x2}" y2="{y2}">'
            f'<stop offset="0" stop-color="{BRASS_HI}"/><stop offset=".48" stop-color="{BRASS}"/>'
            f'<stop offset="1" stop-color="{BRASS_LO}"/></linearGradient>')


def felt_grad(gid):
    return (f'<radialGradient id="{gid}" cx=".5" cy=".38" r=".75">'
            f'<stop offset="0" stop-color="{FELT_EDGE}"/><stop offset="1" stop-color="{FELT}"/></radialGradient>')
