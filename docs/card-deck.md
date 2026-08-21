# Card deck assets

The playing-card faces in `client/public/cards/`, rendered by
`features/match/components/PlayingCard.tsx`.

## Two decks, one folder per deck

```text
client/public/cards/
  french/    32 SVG   — the original French-suited deck (default)
  croatian/  32 WebP  — German/Hungarian-suited: leaves, hearts, bells, acorns
client/public/suits/
  croatian/   4 WebP  — the suit SYMBOLS for the UI chrome (no french/ folder:
                        that deck keeps its Unicode glyphs)
```

Each filename is the canonical card ID from `shared/types/matchTypes.ts`
(`CardId` = `${Rank}${Suit}`, ranks `7 8 9 T J Q K A` x suits `S H D C`), so
`cardFaceUrl(cardId, deck)` derives `cards/{deck}/{cardId}.{ext}` and neither deck
needs a mapping table. Adding or renaming a file here changes what the game
renders, and `cardFace.test.ts` fails if any of either deck's 32 goes missing.

Which deck a player sees is the per-user `cardDeckPreference` (`french` |
`croatian`, migration `000020`), carried on the auth envelope and settable from
the in-match Settings dialog or the profile sidebar. It is **purely visual**: card
IDs, card values, geometry, states, glow, the face-down back and every engine or
WS payload are identical either way.

> **The deck is not the game variant.** The deck values are `french` /
> `croatian`; the game variant values are `bitola` / `croatia`. Two unrelated
> enums that happen to share a country — a Croatian-variant room does not imply
> the Croatian deck, and vice versa. Never cross-wire them.

The unauthenticated landing page is out of scope for all of this: it keeps its own
CSS-drawn cards in `features/landing/components/PlayingCard.tsx`.

The UI chrome is a **separate, narrower** story — see *Suit symbols* below. Four
trump-indicator surfaces follow the active deck; every other suit surface stays
French-suited under every deck.

## Format — both decks are 5:7

`CARD_SIZES` in `client/src/features/match/lib/cardFace.ts` derives every card box
from its width at exactly 5:7, and the corner radius from the artwork's `rx` (5%
of width). Any other ratio distorts the artwork silently, so both decks are
authored or encoded to that ratio and `object-fit: fill` stays an identity scale.

- **French:** every face is `viewBox="-120 -168 240 336"` — 5:7, the 2.5in x
  3.5in poker card — with `preserveAspectRatio="none"` and `rx="12"` corners.
  Because aspect ratio is *not* preserved, a face stretches to fill whatever box
  it is given.
- **Croatian:** 400x560 px WebP, i.e. the same 5:7, at 4.4x the largest render
  (90 px wide) so it never upscales. The source art is 9:14; the import
  **bakes the stretch in** (see the recipe below) rather than letterboxing it, so
  nothing downstream learns that a second aspect ratio ever existed.

## French deck — provenance

Generated with the deck builder at <https://www.me.uk/cards/makeadeck.cgi> by
Adrian Kennard (RevK). Source for the underlying card set:
<https://codeberg.org/RevK/SVG-playing-cards>.

**Licence: CC0 public domain.** No attribution is required — recorded here only so
the deck can be regenerated consistently and so its licence status never has to be
re-established. The author asks (optionally, not as a condition) that the link on the
Ace of Spades be left intact; the builder output we use embeds no links, so there is
nothing to preserve.

### These files are generated, then transformed

They are **not** raw builder output, and they are not hand-edited either. After
downloading, `scripts/recolor-cards.mjs` retargets them onto the app's design
tokens:

| Builder output | In repo | Why |
| --- | --- | --- |
| `red` (#ff0000) | `#c62828` | `--suit-red` in the `.game-table` scope of `index.css` |
| `black` (#000000) | `#1a1a1a` | `--suit-black` in the same scope |
| `fill="white"` | `fill="none"` | the face is transparent; `PlayingCard`'s parchment shows through |
| no `stroke-width` | `stroke-width="3"` | 1 unit of a 240-unit viewBox renders at ~0.2px and vanishes |

The colour swap matters because SVGs loaded through `<img>` are isolated from page
CSS — neither `currentColor` nor custom properties reach inside them — so the ink
cannot be themed at runtime. Without it the deck's `#ff0000` sits next to suit
glyphs drawn in `var(--suit-red)`, putting two different reds for the same suit on
screen at once. Court cards additionally carry `#44F` blue and `#FC4` gold, which
have no counterpart in the theme and are left alone.

The transparent face replaced an earlier `mix-blend-mode: multiply` layer. Blending
was lossy: it shifted every ink on the card, tinted the top and bottom indices
differently under the vertical gradient, darkened cards whose artwork had not
loaded, and inverted under forced-colors mode.

**To regenerate:** pull a fresh set from the builder into
`client/public/cards/french/` keeping the card-ID filenames, then run

```sh
node scripts/recolor-cards.mjs
```

The script is idempotent, so re-running it on an already-processed set is a no-op.
It reads `client/public/cards/french/` only, and is French-specific by
construction: its substitutions name the builder's palette (`red`, `black`,
`#44F`). **Never run it over the Croatian deck** — that art is owner-authored
raster, not builder SVG needing token recolouring.

## Croatian deck — provenance

**Owner-authored. No licence is required and none needs to be tracked** — the
project owner drew these faces, so there is no third-party grant to honour, no
attribution to carry, and nothing to re-establish later. Recorded here precisely
so that question never gets reopened.

Suit mapping, verified visually against the French set:

| Card ID suit | French | Croatian (German-suited) |
| --- | --- | --- |
| `S` | Spades | Leaves (*zelje*) |
| `H` | Hearts | Hearts (*srce*) |
| `D` | Diamonds | Bells (*bundeva*) |
| `C` | Clubs | Acorns (*žir*) |

Screen-reader labels name the suit as the **active deck** depicts it, in the
player's language — see `match.card.suit.{deck}.{S,H,D,C}` in
`client/src/shared/i18n/`.

### The import recipe

The source art is 32 PNGs at 450x700 RGB, ~520 KB each — 16.6 MB for the deck,
which is unshippable: `warmDeck()` fetches all 32 faces on match entry, so the
deck's total size is one download the player waits on. `scripts/import-croatian-deck.py`
is the transform that makes it shippable, and it lives in the repo so the import
stays reproducible rather than a thing that happened once on somebody's laptop.

Per file: resize to **400x560 with Lanczos** (this is what bakes the 9:14 to 5:7
stretch in — no padding, and the source aspect ratio is deliberately *not*
preserved), then encode **WebP quality 80, method 6**. Result: **0.71 MB for the
deck, ~23 KB per face.** At the largest render the ~11% horizontal stretch is
imperceptible.

Two source files are misnamed and are remapped on import: `AQ.png` and `TQ.png`
are both *bells*, i.e. `AD` and `TD`. The real queens (`QC/QD/QH/QS`) are all
present and correct.

**To re-import** (needs no project dependency — `pillow` is pulled ephemerally):

```sh
uv run --with pillow python scripts/import-croatian-deck.py [source-dir]
```

The script is idempotent and rewrites the same 32 outputs.

## Suit symbols — the UI chrome

Separate from the card faces, and separately scoped. The **four** surfaces that
draw a trump indicator follow the active deck (Story 12.4, Scope Amendment 1,
owner-authorised 2026-08-21 — *"symbols used on the UI and dialogs as trump
indicators"*):

| Surface | Where | Render size |
| --- | --- | --- |
| `TrumpIndicator` | top-right HUD chip, inside the 44 px parchment orb | 22 px |
| `TrumpPrompt` | the four suit-choice tiles, and the inline "considering" row | 28 px / 15 px |
| `TrumpReveal` | the wax seal — the largest suit mark in the app | 30 px |
| `PlayerSeat` | the trump-caller chip | 18 px |

All four resolve their mark through the single `SuitMark` component in
`client/src/features/match/components/SuitMark.tsx`, which reads its palette from
`client/src/features/match/lib/suitArt.ts`. Before that seam existed, each
declared its own glyph map *and* its own `suit === "H" || suit === "D"` red/black
colour test — four copies of the same two decisions, and a fifth per-file branch
is exactly what the seam prevents.

**Deliberately excluded, and guarded by a test:**
`shared/components/ui/suit-rule.tsx` (a decorative all-four-suits divider that
also renders unauthenticated, where there is no preference to read) and
`features/rules/components/CardLadder.tsx` (the rules reference — Story 12.9's
territory). Both stay French-suited under every deck.

### Provenance

**Owner-authored, same as the faces. No licence is required and none needs to be
tracked.** The source supplies three sizes per suit — 64x64 PNG, 500x500 RGBA
PNG, and a 2K JPEG.

The **64x64 set is what ships**, per owner direction: 9.8 KB for all four,
against 17.6 KB for a 96x96 downscale of the masters. The largest consumer is
`TrumpReveal` at 30 px, so 64 px is ~2.1x — crisp at 1x and 2x, marginally soft
on a 3x-DPR phone. If that softness ever matters, re-run the importer against the
500x500 masters with `SIZE` raised to 96; nothing else has to change.

Encoding is **lossless WebP with alpha preserved**. All four sources carry real
transparency, and the icons composite over felt, dialog panels and the parchment
orb, so the alpha edge has to stay clean — lossy WebP saves 2.2 KB across the
whole set and frays exactly the edge that matters. `exact=True` keeps the RGB of
fully transparent pixels intact so re-runs are byte-stable.

Filenames are the canonical card-ID suit letters. The source uses French-suit
names, so the importer maps them explicitly: leaves=S, hearts=H, bells=D,
acorns=C — the same mapping as the faces, verified visually against them.

**To re-import:**

```sh
uv run --with pillow python scripts/import-croatian-suits.py [source-dir]
```

### The accent palette

An icon is only half the change. Every one of the four surfaces also paints
chrome *around* the mark — an orb border, a radial halo, a glow, a tile ink, a
wax ring — and all of it used to key off the same red/black test. Dropping
Croatian icons into that logic paints a **red halo around a gold bell** and a
**black one around a green leaf**, which is the specific defect the amendment
exists to avoid. So the colours are a per-deck, per-suit table in `lib/suitArt.ts`:

| Suit | French accent | Croatian accent | Measured in the icon | How the accent follows |
| --- | --- | --- | --- | --- |
| `S` leaves | `#1a1a1a` | `#4a7a3a` | 25% across `#204020` + `#406020` | midpoint of the two greens, lightened for a cream ground |
| `H` hearts | `#c62828` | `#c62828` | 30% `#e00020` | **reuses the French red on purpose** — see below |
| `D` bells | `#c62828` | `#c9a23c` | `#e0c020` gold over a `#202020` outline | the gold, darkened until it holds against parchment |
| `C` acorns | `#1a1a1a` | `#96501e` | 14% `#204020` green, 6% `#a04000` brown | the *nut* brown, not the dominant cap green — see below |

### How to re-derive it

The "measured" column is reproducible — it is the dominant opaque colour by pixel
coverage, quantised to a 32-step cube, in each shipped 64x64 icon:

```sh
uv run --with pillow python - <<'PY'
from PIL import Image
from collections import Counter
for s in "SHDC":
    with Image.open(f"client/public/suits/croatian/{s}.webp") as im:
        px = im.convert("RGBA").load()
        c = Counter()
        for y in range(64):
            for x in range(64):
                r, g, b, a = px[x, y]
                if a > 200:
                    c[(r // 32 * 32, g // 32 * 32, b // 32 * 32)] += 1
        tot = sum(c.values())
        print(s, " ".join("#%02x%02x%02x:%d%%" % (r, g, b, round(100 * n / tot))
                          for (r, g, b), n in c.most_common(4)))
PY
```

A measured colour is **not** the accent, and the gap is deliberate. The icons sit
on their own artwork; the accent paints chrome on a **cream parchment orb** and a
**card-face tile**, so each raw is pulled toward mid-value until it holds contrast
against cream without going muddy. Two of the four need explaining beyond that:

- **Hearts deliberately reuses the French `#c62828`.** The measured `#e00020` is a
  brighter, bluer red, and adopting it would have been a *gratuitous* change: the
  suit is the same shape and the same colour family in both decks, so a player
  toggling decks would see the one suit that did not need to change flicker to a
  different red. Sharing the value also means the French row stays the single
  definition of "the red the app uses", which the `--suit-red` token already is.
- **Acorns takes the brown, not its dominant green.** Coverage alone would give
  acorns `#204020` — the same green as leaves — because the measured leaf-and-cap
  greenery outweighs the nut. That is the one case where the dominant colour is
  the *wrong* answer: an accent exists to tell suits apart, and two identical
  greens tell you nothing. So acorns takes the nut's `#a04000`, which is also the
  conventional Grün/Eichel split. A test pins all four pairwise distinct so this
  reasoning cannot be quietly undone by someone re-running the sampler.

Two companion tables live beside it:

- `SUIT_GLOW_ALPHA` — halo strength, as the hex-alpha suffix appended to an
  accent. French keeps its pre-existing asymmetry (`77` on the reds, `55` on the
  blacks: a near-black glow at the red's strength reads as a smudge under the
  parchment orb). Croatian's accents are all mid-value, so one strength serves
  all four.
- `SUIT_INK_ON_FELT` — suit ink for text on **dark felt** rather than parchment,
  which only `TrumpReveal`'s body copy needs. `#c62828` is close to unreadable
  there, which is what `--suit-red-up` exists for; the Croatian mid-green and
  mid-brown vanish into the table entirely, so each is lifted.

**The French rows of all three tables reproduce the pre-existing values exactly**,
so French rendering is unchanged by this story — the literals are safe
substitutes for the `var(--suit-red, …)` / `var(--suit-black, …)` forms two call
sites used, because all four consumers render inside `.game-table`, where
`index.css` defines those tokens as precisely `#c62828` and `#1a1a1a`. Tests pin
the French rows so a future palette edit cannot quietly restyle them.
