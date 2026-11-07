from fontTools.pens.boundsPen import BoundsPen
from fontTools.pens.recordingPen import RecordingPen

from kit import (BRASS, BRASS_HI, BRASS_LO, CREAM, CREAM_2, FELT, FELT_2, INK, LIME, RED, bodoni,
                 brass_grad, cinzel, felt_grad, num, playfair, suit)

INK_ON_LIGHT = "#0e1a14"
BRASS_DEEP = "#7c633e"
GREEN_DEEP = "#196536"


def svg(view, body, w=None, h=None, defs=""):
    vb = " ".join(num(v) for v in view)
    size = (f' width="{num(w)}"' if w else "") + (f' height="{num(h)}"' if h else "")
    return (f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="{vb}"{size}>'
            f"<defs>{defs}</defs>{body}</svg>")


def shadow(fid, dy=6, blur=8, opacity=0.35):
    return (f'<filter id="{fid}" x="-30%" y="-30%" width="160%" height="160%">'
            f'<feDropShadow dx="0" dy="{dy}" stdDeviation="{blur}" flood-color="#000" flood-opacity="{opacity}"/></filter>')


def icon_bg(gid):
    return felt_grad(gid), f'<rect width="512" height="512" rx="112" fill="url(#{gid})"/>'


def curved_diamond(cx, cy, w, h, k=0.2):
    t, r, b, l = (cx, cy - h / 2), (cx + w / 2, cy), (cx, cy + h / 2), (cx - w / 2, cy)
    q = lambda sx, sy: f"{num(cx + sx * w * k)},{num(cy + sy * h * k)}"
    return (f"M{num(t[0])},{num(t[1])} Q{q(1, -1)} {num(r[0])},{num(r[1])} Q{q(1, 1)} {num(b[0])},{num(b[1])} "
            f"Q{q(-1, 1)} {num(l[0])},{num(l[1])} Q{q(-1, -1)} {num(t[0])},{num(t[1])}Z")



def card_face(uid, x, y, w, h, theme, letter_size, corners=True, rot=0, filt=None, suit_key="D", edge=None):
    s = w / 100
    edge_attr = f' stroke="{edge}" stroke-width="{num(max(1.2 * s, 0.6))}"' if edge else ""
    cx, cy = x + w / 2, y + h / 2
    tr = f' transform="rotate({rot} {num(cx)} {num(cy)})"' if rot else ""
    fl = f' filter="url(#{filt})"' if filt else ""
    parts = [f'<g{tr}{fl}>',
             f'<rect x="{num(x)}" y="{num(y)}" width="{num(w)}" height="{num(h)}" rx="{num(9 * s)}" fill="{CREAM}"{edge_attr}/>',
             f'<rect x="{num(x + 3.5 * s)}" y="{num(y + 3.5 * s)}" width="{num(w - 7 * s)}" height="{num(h - 7 * s)}" '
             f'rx="{num(6 * s)}" fill="none" stroke="url(#{uid}b)" stroke-width="{num(1.3 * s)}"/>',
             f'<path d="{bodoni().centered("B", letter_size, cx, cy + 1 * s)}" fill="{INK}"/>']
    if corners:
        idx = [f'<path d="{bodoni().centered("B", 17 * s, x + 14 * s, y + 19 * s)}" fill="{INK}"/>',
               f'<path d="{suit(suit_key, 13 * s, x + 14 * s, y + 35 * s)}" fill="{RED if suit_key in "HD" else INK}"/>']
        parts += idx
        parts.append(f'<g transform="rotate(180 {num(cx)} {num(cy)})">{"".join(idx)}</g>')
    parts.append("</g>")
    return "".join(parts)


def card_back(uid, x, y, w, h, rot=0, filt=None):
    s = w / 100
    cx, cy = x + w / 2, y + h / 2
    tr = f' transform="rotate({rot} {num(cx)} {num(cy)})"' if rot else ""
    fl = f' filter="url(#{filt})"' if filt else ""
    return (f'<g{tr}{fl}><rect x="{num(x)}" y="{num(y)}" width="{num(w)}" height="{num(h)}" rx="{num(9 * s)}" fill="{FELT_2}" '
            f'stroke="url(#{uid}b)" stroke-width="{num(3 * s)}"/>'
            f'<rect x="{num(x + 9 * s)}" y="{num(y + 9 * s)}" width="{num(w - 18 * s)}" height="{num(h - 18 * s)}" rx="{num(4 * s)}" '
            f'fill="none" stroke="url(#{uid}b)" stroke-width="{num(1 * s)}" opacity=".7"/>'
            f'<path d="{curved_diamond(cx, cy, 34 * s, 46 * s)}" fill="url(#{uid}b)" opacity=".9"/></g>')


def wordmark_card(x, base, theme, size=46):
    main = CREAM if theme == "dark" else INK_ON_LIGHT
    accent = BRASS if theme == "dark" else BRASS_DEEP
    f = bodoni()
    w = f.width("Beljot", size)
    d1 = f.path("Beljot", size, x, base)
    d2 = cinzel(600).path(".online", size * 0.3, x + w + size * 0.08, base, tracking=size * 0.05)
    w2 = cinzel(600).width(".online", size * 0.3, size * 0.05)
    return f'<path d="{d1}" fill="{main}"/><path d="{d2}" fill="{accent}"/>', w + size * 0.08 + w2


def card_lockup(theme, uid):
    defs = brass_grad(uid + "b")
    mark = card_face(uid, 4, 4, 50, 70, theme, 46, corners=True, edge="#0e1a1433" if theme == "light" else None)
    words, ww = wordmark_card(70, 54, theme)
    return svg((0, 0, 70 + ww + 4, 78), mark + words, defs=defs)


def card_icon(uid):
    g, bg = icon_bg(uid + "g")
    defs = g + brass_grad(uid + "b") + shadow(uid + "s", 10, 14, 0.45)
    body = bg + card_back(uid, 150, 92, 210, 294, rot=12, filt=uid + "s") + card_face(uid, 140, 112, 210, 294, "dark", 190, rot=-8, filt=uid + "s")
    return svg((0, 0, 512, 512), body, defs=defs)


def card_favicon(uid):
    defs = brass_grad(uid + "b")
    return svg((0, 0, 32, 32), card_face(uid, 5.5, 1, 21, 30, "dark", 22, corners=False, edge="#0e1a1488"), defs=defs)



def chip_mark(uid, cx, cy, r, text=True):
    s = r / 58
    parts = [f'<clipPath id="{uid}c"><circle cx="{num(cx)}" cy="{num(cy)}" r="{num(58 * s)}"/></clipPath>',
             f'<circle cx="{num(cx)}" cy="{num(cy)}" r="{num(58 * s)}" fill="url(#{uid}b)"/>',
             f'<g clip-path="url(#{uid}c)">']
    for i in range(8):
        a = i * 45 + 22.5
        parts.append(f'<rect x="{num(cx - 5 * s)}" y="{num(cy - 60 * s)}" width="{num(10 * s)}" height="{num(11 * s)}" rx="{num(1.5 * s)}" '
                     f'fill="{FELT}" transform="rotate({a} {num(cx)} {num(cy)})"/>')
    parts.append("</g>")
    parts.append(f'<circle cx="{num(cx)}" cy="{num(cy)}" r="{num(48.5 * s)}" fill="url(#{uid}g)"/>')
    parts.append(f'<circle cx="{num(cx)}" cy="{num(cy)}" r="{num(46 * s)}" fill="none" stroke="url(#{uid}b)" stroke-width="{num(0.9 * s)}"/>')
    if text:
        f = cinzel(700)
        parts.append(f'<path d="{f.arc("BELJOT", 9.6 * s, cx, cy, 36.2 * s, 0, tracking=2.6 * s)}" fill="url(#{uid}b)"/>')
        parts.append(f'<path d="{f.arc("ONLINE", 7.4 * s, cx, cy, 42.8 * s, 0, tracking=2.4 * s, bottom=True)}" fill="url(#{uid}b)"/>')
        parts.append(f'<path d="{suit("H", 7.5 * s, cx - 39.6 * s, cy)}" fill="{RED}"/>')
        parts.append(f'<path d="{suit("S", 7.5 * s, cx + 39.6 * s, cy)}" fill="{CREAM}"/>')
    inner = 27 if text else 34
    parts.append(f'<circle cx="{num(cx)}" cy="{num(cy)}" r="{num(inner * s)}" fill="{CREAM}"/>')
    parts.append(f'<circle cx="{num(cx)}" cy="{num(cy)}" r="{num((inner - 2.6) * s)}" fill="none" stroke="url(#{uid}b)" stroke-width="{num(0.9 * s)}"/>')
    parts.append(f'<path d="{bodoni().centered("B", (34 if text else 44) * s, cx, cy + 0.5 * s)}" fill="{FELT}"/>')
    return "".join(parts)


def chip_defs(uid):
    return brass_grad(uid + "b", "0", "0", "1", "1") + felt_grad(uid + "g")


def wordmark_chip(x, base, theme, size=34):
    main = CREAM if theme == "dark" else INK_ON_LIGHT
    accent = BRASS if theme == "dark" else BRASS_DEEP
    f = cinzel(700)
    w = f.width("BELJOT", size, size * 0.12)
    d1 = f.path("BELJOT", size, x, base, tracking=size * 0.12)
    d2 = cinzel(600).path("ONLINE", size * 0.3, x + w, base + size * 0.48, tracking=size * 0.16, anchor="end")
    rule = f'<rect x="{num(x)}" y="{num(base + size * 0.24)}" width="{num(w - cinzel(600).width("ONLINE", size * 0.3, size * 0.16) - size * 0.3)}" height="{num(size * 0.03)}" fill="{accent}" opacity=".7"/>'
    return f'<path d="{d1}" fill="{main}"/>{rule}<path d="{d2}" fill="{accent}"/>', w


def chip_lockup(theme, uid):
    words, ww = wordmark_chip(92, 45, theme)
    return svg((0, 0, 92 + ww + 6, 80), chip_mark(uid, 40, 40, 38) + words, defs=chip_defs(uid))


def chip_icon(uid):
    g, bg = icon_bg(uid + "bg")
    defs = g + chip_defs(uid) + shadow(uid + "s", 10, 16, 0.5)
    return svg((0, 0, 512, 512), bg + f'<g filter="url(#{uid}s)">{chip_mark(uid, 256, 256, 208)}</g>', defs=defs)


def chip_favicon(uid):
    return svg((0, 0, 32, 32), chip_mark(uid, 16, 16, 15.5, text=False), defs=chip_defs(uid))



def small_card(x, y, w, h, content, rot, px, py, theme_stroke):
    s = w / 52
    return (f'<g transform="rotate({rot} {num(px)} {num(py)})">'
            f'<rect x="{num(x)}" y="{num(y)}" width="{num(w)}" height="{num(h)}" rx="{num(6 * s)}" fill="{CREAM}" '
            f'stroke="{theme_stroke}" stroke-width="{num(1.1 * s)}"/>{content}</g>')


def hand_mark(uid, ox, oy, scale, theme, indices=True):
    s = scale
    w, h = 52 * s, 74 * s
    px, py = ox + 70 * s, oy + 118 * s
    x = px - w / 2
    y = oy + 24 * s
    stroke = "#00000022" if theme == "light" else "#00000055"
    cards = []
    for rot, key in ((-20, "H"), (20, "C")):
        pip = f'<path d="{suit(key, 26 * s, px, y + h / 2)}" fill="{RED if key == "H" else INK}"/>'
        if indices:
            pip += f'<path d="{suit(key, 9 * s, x + 9 * s, y + 11 * s)}" fill="{RED if key == "H" else INK}"/>'
        cards.append(small_card(x, y, w, h, pip, rot, px, py, stroke))
    front = f'<path d="{playfair(900).centered("B", 46 * s, px, y + h / 2 + 1 * s)}" fill="{INK}"/>'
    if indices:
        front += f'<path d="{suit("S", 9 * s, x + 9 * s, y + 11 * s)}" fill="{INK}"/>'
    cards.append(small_card(x, y - 6 * s, w, h, front.replace(f"{num(y + h / 2 + 1 * s)}", f"{num(y + h / 2 + 1 * s)}"), 0, px, py, stroke))
    return f'<g filter="url(#{uid}s)">{"".join(cards)}</g>'


def wordmark_hand(x, base, theme, size=46):
    main = CREAM if theme == "dark" else INK_ON_LIGHT
    accent = LIME if theme == "dark" else GREEN_DEEP
    f = playfair(800)
    w = f.width("Beljot", size)
    d1 = f.path("Beljot", size, x, base)
    d2 = playfair(700).path(".online", size * 0.42, x + w + size * 0.04, base)
    w2 = playfair(700).width(".online", size * 0.42)
    return f'<path d="{d1}" fill="{main}"/><path d="{d2}" fill="{accent}"/>', w + size * 0.04 + w2


def hand_lockup(theme, uid):
    defs = shadow(uid + "s", 2, 2.5, 0.35)
    words, ww = wordmark_hand(104, 58, theme)
    body = hand_mark(uid, -2, -10, 0.74, theme, indices=False) + words
    return svg((0, 0, 104 + ww + 4, 82), body, defs=defs)


def hand_icon(uid):
    g, bg = icon_bg(uid + "g")
    defs = g + shadow(uid + "s", 8, 10, 0.45)
    return svg((0, 0, 512, 512), bg + hand_mark(uid, 256 - 70 * 3.05, 256 - 74 * 3.05 + 8, 3.05, "dark"), defs=defs)


def hand_favicon(uid):
    defs = shadow(uid + "s", 0.5, 0.5, 0.3)
    return svg((0, 0, 32, 32), hand_mark(uid, 16 - 70 * 0.215, 16 - 74 * 0.215 + 1, 0.215, "dark", indices=False), defs=defs)



def crown(cx, top, w):
    s = w / 26
    pts = [(-13, 15), (-13, 6), (-6.5, 10.5), (0, 2), (6.5, 10.5), (13, 6), (13, 15)]
    d = "M" + " L".join(f"{num(cx + px * s)},{num(top + py * s)}" for px, py in pts) + "Z"
    dots = "".join(f'<circle cx="{num(cx + px * s)}" cy="{num(top + py * s)}" r="{num(1.7 * s)}"/>' for px, py in ((-13, 5), (0, 1), (13, 5)))
    band = f'<rect x="{num(cx - 13 * s)}" y="{num(top + 16.2 * s)}" width="{num(26 * s)}" height="{num(2.4 * s)}" rx="{num(0.6 * s)}"/>'
    return f'<path d="{d}"/>{dots}{band}'


def crown_mark(uid, cx, cy, scale, with_crown=True):
    s = scale
    outer = curved_diamond(cx, cy, 96 * s, 120 * s, k=0.17)
    inner = curved_diamond(cx, cy, 84 * s, 106 * s, k=0.17)
    parts = [f'<path d="{outer}" fill="url(#{uid}g)" stroke="url(#{uid}b)" stroke-width="{num(2.4 * s)}"/>',
             f'<path d="{inner}" fill="none" stroke="url(#{uid}b)" stroke-width="{num(0.8 * s)}" opacity=".75"/>']
    if with_crown:
        parts.append(f'<g fill="url(#{uid}b)">{crown(cx, cy - 37 * s, 24 * s)}</g>')
        parts.append(f'<path d="{bodoni(700).centered("B", 44 * s, cx, cy + 8 * s)}" fill="url(#{uid}b)"/>')
    else:
        parts.append(f'<path d="{bodoni(700).centered("B", 56 * s, cx, cy + 1 * s)}" fill="url(#{uid}b)"/>')
    return "".join(parts)


def crown_defs(uid):
    return brass_grad(uid + "b") + felt_grad(uid + "g")


def wordmark_crown(x, base, theme, size=30):
    main = BRASS_HI if theme == "dark" else FELT
    accent = BRASS if theme == "dark" else BRASS_DEEP
    f = cinzel(700)
    tr = size * 0.22
    w = f.width("BELJOT", size, tr)
    d1 = f.path("BELJOT", size, x, base, tracking=tr)
    sub = cinzel(600)
    d2 = sub.path("ONLINE", size * 0.32, x + w / 2, base + size * 0.62, tracking=size * 0.32, anchor="middle")
    sw = sub.width("ONLINE", size * 0.32, size * 0.32)
    rl = (w - sw) / 2 - size * 0.35
    y = base + size * 0.5
    rules = (f'<rect x="{num(x)}" y="{num(y)}" width="{num(rl)}" height="{num(size * 0.035)}" fill="{accent}"/>'
             f'<rect x="{num(x + w - rl)}" y="{num(y)}" width="{num(rl)}" height="{num(size * 0.035)}" fill="{accent}"/>')
    return f'<path d="{d1}" fill="{main}"/>{rules}<path d="{d2}" fill="{accent}"/>', w


def crown_lockup(theme, uid):
    words, ww = wordmark_crown(84, 42, theme)
    return svg((0, 0, 84 + ww + 6, 80), crown_mark(uid, 36, 40, 0.62) + words, defs=crown_defs(uid))


def crown_icon(uid):
    g, bg = icon_bg(uid + "bg")
    return svg((0, 0, 512, 512), bg + crown_mark(uid, 256, 258, 3.3), defs=g + crown_defs(uid))


def crown_favicon(uid):
    return svg((0, 0, 32, 32), crown_mark(uid, 16, 16, 0.255, with_crown=False), defs=crown_defs(uid))



def j_dot(face_obj, size):
    """Centre and size of the j's dot, relative to the j's own origin."""
    gname = face_obj.order[face_obj.shape("j")[0][0].codepoint]
    rec = RecordingPen()
    face_obj.gs[gname].draw(rec)
    contours, cur = [], []
    for op, args in rec.value:
        cur.append((op, args))
        if op in ("closePath", "endPath"):
            contours.append(cur)
            cur = []
    best = None
    for c in contours:
        bp = BoundsPen(face_obj.gs)
        for op, args in c:
            getattr(bp, op)(*args)
        if bp.bounds and (best is None or bp.bounds[1] > best[1]):
            best = bp.bounds
    s = size / face_obj.upem
    x0, y0, x1, y1 = best
    return (x0 + x1) / 2 * s, -(y0 + y1) / 2 * s, (x1 - x0) * s


def wordmark_dot(x, base, theme, size=50):
    main = CREAM if theme == "dark" else INK_ON_LIGHT
    f = bodoni(700)
    pre = f.width("bel", size)
    body = f.path("belȷot", size, x, base)
    dx, dy, dw = j_dot(f, size)
    jx = x + pre + dx
    jy = base + dy
    dia = f'<path d="{curved_diamond(jx, jy + size * 0.01, dw * 1.45, dw * 1.95)}" fill="{RED}"/>'
    return f'<path d="{body}" fill="{main}"/>{dia}', f.width("belȷot", size)


def dot_lockup(theme, uid):
    words, ww = wordmark_dot(6, 58, theme)
    sub = cinzel(600).path("ONLINE", 9, 6 + ww, 74, tracking=3.2, anchor="end")
    accent = BRASS if theme == "dark" else BRASS_DEEP
    return svg((0, 0, ww + 12, 82), words + f'<path d="{sub}" fill="{accent}"/>')


def dot_monogram(cx, cy, size, fill):
    f = bodoni(700)
    x0, y0, x1, y1 = f.bounds("b", size)
    bx = cx - (x0 + x1) / 2 - size * 0.08
    by = cy - (y0 + y1) / 2
    letter = f'<path d="{f.path("b", size, bx, by)}" fill="{fill}"/>'
    dia = f'<path d="{curved_diamond(bx + x1 + size * 0.13, by + y0 + size * 0.1, size * 0.17, size * 0.23)}" fill="{RED}"/>'
    return letter + dia


def dot_icon(uid):
    g, bg = icon_bg(uid + "g")
    return svg((0, 0, 512, 512), bg + dot_monogram(256, 262, 380, CREAM), defs=g)


def dot_favicon(uid):
    g = f'<rect width="32" height="32" rx="7" fill="{FELT}"/>'
    return svg((0, 0, 32, 32), g + dot_monogram(16, 16.6, 25, CREAM))


OPTIONS = [
    {"key": "card", "name": "The Card", "lockup": card_lockup, "icon": card_icon, "favicon": card_favicon,
     "fonts": "Bodoni Moda Bold · Cinzel", "pitch": "A playing card with a Didone B and corner indices: says card game at any size."},
    {"key": "chip", "name": "The Chip", "lockup": chip_lockup, "icon": chip_icon, "favicon": chip_favicon,
     "fonts": "Cinzel Bold · Bodoni Moda Bold", "pitch": "A brass casino chip with the name around the rim: the high-roller seal."},
    {"key": "hand", "name": "The Hand", "lockup": hand_lockup, "icon": hand_icon, "favicon": hand_favicon,
     "fonts": "Playfair Display Black · ExtraBold", "pitch": "A fanned hand, B in front: the most playful and eye-catching of the five."},
    {"key": "crown", "name": "The Crown", "lockup": crown_lockup, "icon": crown_icon, "favicon": crown_favicon,
     "fonts": "Cinzel Bold · Bodoni Moda Bold", "pitch": "A crowned B inside a card diamond: a luxury-house monogram, the most elite."},
    {"key": "dot", "name": "The Diamond Dot", "lockup": dot_lockup, "icon": dot_icon, "favicon": dot_favicon,
     "fonts": "Bodoni Moda Bold", "pitch": "A clean wordmark whose j is dotted with a red diamond: the quietest, most editorial."},
]
