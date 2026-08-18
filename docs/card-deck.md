# Card deck assets

The playing-card faces in `client/public/cards/`, rendered by
`features/match/components/PlayingCard.tsx`.

## Contents

32 SVGs — the Belot deck (ranks `7 8 9 T J Q K A` x suits `S H D C`). Each filename
is the canonical card ID from `shared/types/matchTypes.ts` (`CardId` = `${Rank}${Suit}`),
so `PlayingCard` derives `src` directly from the card and needs no mapping table.
Adding or renaming a file here changes what the game renders, and
`cardFace.assets.test.ts` fails if any of the 32 goes missing.

## Format

Every face is `viewBox="-120 -168 240 336"` — 5:7, the 2.5in x 3.5in poker card —
with `preserveAspectRatio="none"` and `rx="12"` corners. Because aspect ratio is
*not* preserved, a face stretches to fill whatever box it is given, so
`CARD_SIZES` in `client/src/features/match/lib/cardFace.ts` derives every box
from its width at exactly that ratio, and the corner radius from `rx` (5% of
width). Any other ratio distorts the artwork silently.

## Provenance

Generated with the deck builder at <https://www.me.uk/cards/makeadeck.cgi> by
Adrian Kennard (RevK). Source for the underlying card set:
<https://codeberg.org/RevK/SVG-playing-cards>.

**Licence: CC0 public domain.** No attribution is required — recorded here only so
the deck can be regenerated consistently and so its licence status never has to be
re-established. The author asks (optionally, not as a condition) that the link on the
Ace of Spades be left intact; the builder output we use embeds no links, so there is
nothing to preserve.

## These files are generated, then transformed

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

**To regenerate:** pull a fresh set from the builder into `client/public/cards/`
keeping the card-ID filenames, then run

```sh
node scripts/recolor-cards.mjs
```

The script is idempotent, so re-running it on an already-processed set is a no-op.
