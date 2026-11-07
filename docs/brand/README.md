# Beljot brand kit

The logo is a brass casino chip: felt inserts on the rim, BELJOT around the top, ONLINE around the bottom, a red heart and a cream spade at the sides, and a cream centre with a Bodoni **B**.

## Files

| File | Use |
| --- | --- |
| `beljot-chip.svg` / `.pdf` | Full-colour chip: print, merch, large on screen (64px and up). |
| `beljot-chip-compact.svg` | Chip without the rim text: favicons and anything under 64px. The app serves it as `client/public/beljot-logo.svg`. |
| `beljot-lockup-on-dark.svg` / `.pdf` | Chip with the BELJOT · ONLINE wordmark, for dark grounds. |
| `beljot-lockup-on-light.svg` / `.pdf` | The same lockup for light grounds. |
| `beljot-chip-black.svg` / `.pdf` | One colour, black: stamps, single-colour print, engraving. |
| `beljot-chip-white.svg` | One colour, white: reversed out of dark photos or dark stock. |
| `beljot-chip-gold.svg` / `.pdf` | One colour, brass `#c9a876`: the plate for gold foil or embossing. |
| `beljot-chip-1024.png` | Raster for tools that refuse vectors. |
| `client/public/og-image.jpg` | The 1200×630 link preview shown when beljot.online is shared: Dama holding a hand of cards beside the chip. After changing it, re-scrape the link in Facebook's Sharing Debugger. |
| `mascot/dama-*.webp` | Dama, the house hostess, cut out on transparency: `welcome`, `cards-half`, `signature`, `dealing` and `win`. |

Every letter is outlined, so no file needs a font installed. The PDFs are vector; they are sized at 100mm (chip) and 180mm wide (lockups) but scale freely.

## Colours

| Name | Hex | Notes |
| --- | --- | --- |
| Felt | `#0b1f15` | Background and rim inserts. |
| Felt light | `#1d4a36` | Top of the felt gradient. |
| Brass | `#c9a876` | Ring and lettering; gradient from `#efdcaa` to `#94733f`. |
| Cream | `#f6f1e3` | Chip centre. |
| Card red | `#b4262f` | The heart. |

For gold foil, give the printer `beljot-chip-gold.pdf` as the foil plate and ask them to match a metallic gold (for example Pantone 871 C); confirm the swatch on their stock before a full run.

## Use

- **Minimum size:** the full chip from 64px (20mm in print), the compact chip down to 16px.
- **Clear space:** keep at least a quarter of the chip's diameter free on every side.
- **Don't** recolour the chip outside the one-colour versions, stretch it, rotate it, or set the wordmark in a live font.

## Mascot

Dama is the house hostess: long dark waves, a burgundy and gold gown, pearls and a ruby diamond pendant. Every pose is generated on a flat grey ground and cut out, so she sits on the felt, the cream background or the link preview. Keep effects (chips, sparkles) out of her layer, and check the hair edges on both dark and light grounds before shipping a new pose.

## Fonts

Cinzel (wordmark, rim) and Bodoni Moda (the B), both SIL Open Font License, free for commercial use.

## Regenerate

`source/` builds every file here, plus `client/public/beljot-logo.svg`, `beljot-chip.svg`, the PNG icons, the link-preview image `og-image.jpg` (`og.py`) and the header wordmark (`client/src/shared/components/brand/wordmarkPaths.ts`). Change the design there, never the paths by hand:

```bash
cd docs/brand/source
./fetch_fonts.sh && pip install -r requirements.txt
python3 final.py && python3 og.py
cd ../../../client && node ../docs/brand/source/render.mjs
```

`og.py` has two layouts: `trumps` (the default, Dama on the right with her hand of cards) and `presents` (Dama on the left presenting the chip). Run `python3 og.py presents` to switch.

`options.py` also holds the four logo directions that were not chosen (card, hand, crown, diamond dot).
