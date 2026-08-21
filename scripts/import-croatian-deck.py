"""One-time import of the Croatian (German-suited) card deck.

The source art is owner-authored raster — 32 PNGs at 450x700 (9:14), ~520 KB
each, 16.6 MB for the deck. That is unshippable on its own: `warmDeck()` in
`PlayingCard.tsx` fetches all 32 faces on match entry, so the deck's total size
is a single download the player waits on. This script is the transform that
makes it shippable, and it is kept in the repo so the import is reproducible
rather than a thing that happened once on somebody's laptop.

Two things happen per file:

1. **Resize to 400x560.** That is 5:7 — the card box every size in `CARD_SIZES`
   derives — and the source is 9:14, so the resize BAKES THE STRETCH IN. This is
   deliberate: nothing downstream then learns that a second aspect ratio exists,
   `object-fit: fill` stays an identity scale, and the French deck's 5:7
   geometry invariants keep holding untouched. Do NOT add padding and do NOT
   preserve the source aspect ratio — either one puts letterbox bars or a
   distortion inside the card's own rounded outline. At the largest render
   (90 px wide) the ~11% horizontal stretch is imperceptible.

2. **Encode WebP q80.** 0.71 MB for the whole deck versus 16.6 MB raw, at 4.4x
   the largest render's pixel width so it never upscales.

Two source files are misnamed: `AQ.png` and `TQ.png` are both *bells*, i.e. the
diamond-equivalent suit, so they import as `AD` and `TD`. The real queens
(`QC/QD/QH/QS`) are all present and correct. Suit mapping was verified visually
against the French set: leaves=S, hearts=H, bells=D, acorns=C.

This machine has no `sharp`, no PIL and no ImageMagick, and none of them should
become a project dependency for a one-time art import — so run it with an
ephemeral dependency:

    uv run --with pillow python scripts/import-croatian-deck.py

Optionally pass the source directory as the first argument; it defaults to the
location the art was authored in. Idempotent: re-running overwrites the same 32
outputs byte-for-byte.
"""

from __future__ import annotations

import sys
from pathlib import Path

from PIL import Image

# The 5:7 card box (see CARD_SIZES in client/src/features/match/lib/cardFace.ts).
# 400x560 is 4.4x the largest render (90 px) so the face never upscales.
TARGET_SIZE = (400, 560)
WEBP_QUALITY = 80

RANKS = ("7", "8", "9", "T", "J", "Q", "K", "A")
SUITS = ("S", "H", "D", "C")

# The two misnamed sources. Both are bells (the diamond-equivalent suit); they
# were saved with a `Q` where the suit letter belongs.
RENAMES = {"AQ": "AD", "TQ": "TD"}

DEFAULT_SRC = Path(r"D:\Downloads\Croatian Card Deck")
DEST = Path(__file__).resolve().parent.parent / "client" / "public" / "cards" / "croatian"


def main() -> int:
    src_dir = Path(sys.argv[1]) if len(sys.argv) > 1 else DEFAULT_SRC
    if not src_dir.is_dir():
        print(f"source directory not found: {src_dir}", file=sys.stderr)
        return 1

    DEST.mkdir(parents=True, exist_ok=True)

    # Iterate the 32 expected card IDs rather than whatever the directory holds,
    # so a MISSING source is a hard error instead of a silently absent face.
    # Extra files in the source directory are simply not read — this loop never
    # looks at them, and an unexpected 33rd file is far likelier to be a stray
    # download than a card the game needs.
    wanted = {rank + suit for rank in RANKS for suit in SUITS}
    reverse_renames = {v: k for k, v in RENAMES.items()}

    total_bytes = 0
    for card_id in sorted(wanted):
        src_name = reverse_renames.get(card_id, card_id)
        src = src_dir / f"{src_name}.png"
        if not src.is_file():
            print(f"missing source for {card_id}: {src}", file=sys.stderr)
            return 1
        dst = DEST / f"{card_id}.webp"
        with Image.open(src) as img:
            img.convert("RGB").resize(TARGET_SIZE, Image.LANCZOS).save(
                dst, "WEBP", quality=WEBP_QUALITY, method=6
            )
        total_bytes += dst.stat().st_size

    print(f"wrote {len(wanted)} faces to {DEST} ({total_bytes / 1_048_576:.2f} MB)")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
