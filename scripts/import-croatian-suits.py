"""One-time import of the Croatian (German-suited) suit icons.

Companion to `import-croatian-deck.py`. That script imports the 32 card *faces*;
this one imports the four *suit symbols* used by the UI chrome — the trump
indicator, the trump prompt and reveal, and the seat trump chip. Those four are
the whole consumer list: Scope Amendment 1 deliberately excludes the rules-page
card ladder (Story 12.9's) and the decorative `suit-rule` divider, and a test
guards that both keep drawing French glyphs under the Croatian deck.

Those surfaces draw Unicode glyphs today (`♠♥♦♣`) coloured by CSS tokens. The
Croatian suits have no Unicode equivalent, so they ship as raster art instead.
The owner-authored source provides three sizes per suit — 64x64 PNG, 500x500
RGBA PNG, and a 2K JPEG. The 64x64 set is what we import, per owner direction:
9.8 KB for all four, against 17.6 KB for a 96x96 downscale of the masters. The
largest surface that draws a suit symbol renders it at 30 px (`TrumpReveal`), so
64 px is ~2.1x — crisp at 1x and 2x, marginally soft on a 3x phone. If that
softness ever matters, re-run this against `{Suit}.png` (the 500x500 masters)
with SIZE raised to 96; nothing else has to change.

Encoding is **lossless with alpha preserved**. All four sources carry real
transparency (palette + tRNS), and the icons composite over the felt, dialog
panels and the parchment orb, so the alpha edge has to stay clean — lossy WebP
saves 2.2 KB across the whole set and frays exactly the edge that matters.
`exact=True` keeps the RGB of fully transparent pixels intact so re-runs are
byte-stable.

Filenames map to the canonical card-ID suit letters, matching the deck import
and `Suit` in `client/src/shared/types/matchTypes.ts`. The source uses
French-suit names for the files, so the mapping is explicit here: leaves=S,
hearts=H, bells=D, acorns=C — verified visually against the card faces.

This machine has no `sharp`, no PIL and no ImageMagick, and none of them should
become a project dependency for a one-time art import — so run it with an
ephemeral dependency:

    uv run --with pillow python scripts/import-croatian-suits.py

Optionally pass the source directory as the first argument; it defaults to the
location the art was authored in. Idempotent: re-running overwrites the same
four outputs byte-for-byte.
"""

from __future__ import annotations

import sys
from pathlib import Path

from PIL import Image

# 64x64 is the owner-selected source size (see module docstring). The largest
# consumer renders at 30 px.
SIZE = 64

# Source filename (French suit name) -> canonical card-ID suit letter.
# Leaves are the spade-equivalent, bells the diamond-equivalent.
SUIT_FILES = {
    "Spades": "S",
    "Hearts": "H",
    "Diamonds": "D",
    "Clubs": "C",
}

DEFAULT_SRC = Path(r"D:\Downloads\Croatian Card Deck (1)")
DEST = Path(__file__).resolve().parent.parent / "client" / "public" / "suits" / "croatian"


def main() -> int:
    src_dir = Path(sys.argv[1]) if len(sys.argv) > 1 else DEFAULT_SRC
    if not src_dir.is_dir():
        print(f"source directory not found: {src_dir}", file=sys.stderr)
        return 1

    DEST.mkdir(parents=True, exist_ok=True)

    total_bytes = 0
    for src_name, letter in sorted(SUIT_FILES.items(), key=lambda kv: kv[1]):
        src = src_dir / f"{src_name}_{SIZE}x{SIZE}.png"
        if not src.is_file():
            print(f"missing source for {letter}: {src}", file=sys.stderr)
            return 1
        dst = DEST / f"{letter}.webp"
        with Image.open(src) as img:
            rgba = img.convert("RGBA")
            if rgba.size != (SIZE, SIZE):
                rgba = rgba.resize((SIZE, SIZE), Image.LANCZOS)
            # An icon with no transparency would sit on an opaque tile over the
            # felt, so a source that lost its alpha is a hard error, not a
            # cosmetic surprise found later on a dark panel.
            if rgba.getchannel("A").getextrema()[0] == 255:
                print(f"{src.name} has no transparent pixels", file=sys.stderr)
                return 1
            rgba.save(dst, "WEBP", lossless=True, method=6, exact=True)
        total_bytes += dst.stat().st_size

    print(f"wrote {len(SUIT_FILES)} suit icons to {DEST} ({total_bytes / 1024:.1f} KB)")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
