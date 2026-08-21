---
title: "In-match card faces render from the French SVG deck"
type: "feature"
created: "2026-08-18"
status: "done"
review_loop_iteration: 0
context: []
baseline_commit: "ef5af3a84ebceca6309d33aceae0c57be37741ba"
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** In-match card faces are drawn by `PlayingCard` out of text — a rank string, a suit glyph, and an italic letter standing in for J/Q/K. Court cards have no artwork, so the table never reads like a real deck.

**Approach:** Vendor the 32-card French SVG deck into `client/public/cards/` and have `PlayingCard` render `/cards/{RANK}{SUIT}.svg` instead of drawing the face. States, glow, a11y contract, and the brass monogram back stay as they are. The deck is retargeted onto the app's tokens by an idempotent script (renegotiated — see change log 3): suit ink becomes `--suit-red` / `--suit-black`, and the face rect becomes transparent so the card's own parchment background *is* the face — no blend layer.

## Boundaries & Constraints

**Always:**

- The filename **is** the card ID (`KS.svg`, `TH.svg`, `7D.svg`) — same two-char format as `CardId` in `matchTypes.ts`. Derive `src` from the card; add no lookup table.
- **Card geometry is derived from the artwork, not chosen** (renegotiated — see change log 2): every box is an exact 5:7 so the faces (`preserveAspectRatio="none"`) are never stretched, and the corner radius equals the artwork's own `rx` (5% of width) so the two edges coincide. The card draws **no CSS border** — the deck SVG strokes its own outline, and a second border leaves a visible seam at the corners.
- One module owns card geometry and face treatment; `HandCards`, `TrickArea`, and `CardFlight` read from it and must never restate the numbers.
- The wrapper keeps its `data-testid`s (`playing-card-{cardId}`, `playing-card-facedown`), `aria-label`, `role`, `tabIndex`, `aria-disabled`, handlers, and state classes.
- The face image is decorative (`alt=""`, `aria-hidden="true"`) so the wrapper's `aria-label` stays the single announcement.

**Ask First:** replacing the face-down back with deck artwork (this deck ships no back).

**Never:** touch `features/landing/components/PlayingCard.tsx` (marketing card); restyle `PlayerSeat` or `TrumpIndicator` (parchment surfaces, not cards); **hand-edit the deck files** — every change to them goes through `scripts/recolor-cards.mjs` so a regeneration can be replayed (renegotiated — see change log 3); add a sprite/bundler step or inline the SVGs into the JS bundle.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
| --- | --- | --- | --- |
| Spot card | `{rank:"7",suit:"D"}`, `default` | Wrapper `playing-card-7D` holds `img src="/cards/7D.svg"` | N/A |
| Ten mapping | `{rank:"T",suit:"H"}` | `src="/cards/TH.svg"` — rank char, not the `"10"` label | N/A |
| Court card | `{rank:"K",suit:"S"}` | `src="/cards/KS.svg"`, full court artwork | N/A |
| Face down | `card === null` or `face-down` | Monogram back unchanged, **no** `img` rendered | N/A |
| Playable click | `playable` + `onClick` | Click and Enter/Space fire `onClick` once | Image layers never intercept |
| Asset pending | `img` loading, or unresolvable | Parchment background shows a blank card face, no layout shift | `onError` hides the image — production's SPA fallback answers 200-with-HTML, not 404, so an unhidden `img` would paint a broken-image glyph |

</frozen-after-approval>

## Code Map

- `client/src/features/match/lib/cardFace.ts` — **new**. Owns `CardSize`, `CARD_SIZES` (box + radius per size), `CARD_FACE_BACKGROUND`, `CARD_FACE_BORDER`, `cardFaceUrl`. Lives in `lib/` beside `tableTheme.ts` / `trickLayout.ts`; exporting these from the component file trips `react-refresh/only-export-components`.
- `client/src/features/match/components/PlayingCard.tsx` — the only face-up card renderer in the match; single point of change for the 10 consumers (`HandCards`, `TrickArea`, `CardFlight`, `DealAnimation`, `TrumpPrompt`, `TrumpReveal`, `DeclarationPrompt`, `DeclarationReveal`, `BelotPrompt`, `BelotReveal`).
- `client/src/features/match/components/PlayingCard.test.tsx` — the only test asserting rendered rank/suit text (lines 15-37); must be rewritten.
- Geometry restaters that must now read `CARD_SIZES`: `HandCards.tsx` (`CARD_WIDTH`/`CARD_HEIGHT`), `CardFlight.tsx` (`BASE_W`/`BASE_H`), `TrickArea.tsx` (`TRICK_SLOT_W`/`TRICK_SLOT_H`, plus the dashed slot's hardcoded `borderRadius: 6`).
- `client/src/features/match/components/TrumpPrompt.tsx` — duplicates the parchment face literal in two suit-tile style blocks (`:132` considering chips, `:246` picker buttons).
- `client/public/` — existing static-SVG convention: served from root, referenced as `/beljot-logo.svg` via `<img>`. No Vite config change needed.

## Tasks & Acceptance

**Execution:**

- [x] `client/public/cards/` — copy the 32 SVGs from `D:\Downloads\French Card Deck` verbatim — vendored assets, served like the existing `/beljot-logo.svg`.
- [x] `docs/card-deck.md` — record provenance and how to regenerate — deck built with me.uk/cards/makeadeck.cgi (Adrian Kennard / RevK), **CC0, no attribution required**, confirmed with the user and against the source site. Not named `ATTRIBUTION.md` because nothing is owed, and moved out of `public/` (change log 4) so it is not published with the site.
- [x] `client/src/features/match/components/PlayingCard.tsx` — replace the face-up text render with the layered `img` + parchment multiply overlay; export `CARD_FACE_BACKGROUND` / `CARD_FACE_BORDER`; delete the dead `DISPLAY_RANK`, `DISPLAY_SUIT`, `isFace`, `suitColor`; warm all 32 faces once on first mount; rewrite the doc comment to describe the image render.
- [x] `client/src/features/match/components/TrumpPrompt.tsx` — consume the exported face constants in both suit-tile style blocks so tiles cannot drift from the card face.
- [x] `client/src/features/match/components/PlayingCard.test.tsx` — rewrite the three text assertions as `src` assertions and cover the matrix: ten mapping, face-down has no `img`, decorative-image a11y, click still fires when playable.
- [x] `client/src/features/match/lib/cardFace.ts` — **new (change log 2)** — extract `CardSize`, `CARD_SIZES`, face constants, and `cardFaceUrl`; every box derived from its width via `cardBox()` at the artwork's own ratio and `rx`.
- [x] `HandCards.tsx`, `CardFlight.tsx`, `TrickArea.tsx` — **(change log 2)** — read geometry from `CARD_SIZES` instead of restating it; the dashed trick slot takes the card's radius so slot and card share one outline.
- [x] `scripts/recolor-cards.mjs` + `docs/card-deck.md` — **new (change log 3)** — idempotent transform retargeting deck ink onto `--suit-red` / `--suit-black`, muting the court blue, and making the face rect transparent; doc moved out of `public/` and records how to regenerate.
- [x] `client/src/features/match/lib/cardFace.test.ts` — **new (change log 4)** — assert all 32 assets exist and pin the 5:7 / `rx` ratios.
- [x] `PlayingCard.tsx`, `DealAnimation.tsx`, `DeclarationReveal.tsx`, `HandCards.tsx`, `.prettierignore` — **(change log 4)** — review patches: blend layer removed, back radius, `BASE_URL`, `onError`, warm-up refs, proxy geometry, meld overlap derived from card width.

**Acceptance Criteria:**

- Given a match in progress, when a hand is dealt, then hand cards, trick-area cards, and the flight overlay show deck artwork, with full court artwork for J/Q/K at `sm`, `md`, and `lg`.
- Given any card at any size, when it renders, then the artwork fills the box with no letterboxing, no stretch, and exactly one visible edge — no gap between a playable card's lime ring and its face.
- Given the round-2 trump dialog, when it renders, then the suit tiles and the candidate card read as the same material.
- Given `make lint`, when it runs, then no unused-symbol, type, or `react-refresh` findings remain.
- Given `make test`, when it runs, then the suite is green with no edits to consumer test files.

## Spec Change Log

1. **Licence question closed (user, mid-implementation).** The spec assumed unknown provenance and specced an `ATTRIBUTION.md` with the licence flagged for confirmation. The user identified the deck as their own output from <https://www.me.uk/cards/makeadeck.cgi>; verified against the source site as **CC0 public domain, no attribution required** (author Adrian Kennard / RevK, source `codeberg.org/RevK/SVG-playing-cards`), and the builder output embeds no links, so the optional Ace-of-Spades credit link does not apply. Amended: file renamed `README.md` and rewritten as provenance + regeneration notes. Avoids the known-bad state of shipping a standing "ACTION REQUIRED" legal flag for a question already settled. **KEEP:** recording provenance at all — it is what lets the deck be regenerated consistently and stops the licence being re-litigated.

2. **Frozen geometry boundary renegotiated (user, mid-implementation).** The frozen block froze the card box at its old dimensions and marked aspect-ratio changes "Ask First"; the user directed the change directly ("fix the border radius and size to match the new card svgs, i dont want this empty space… everywhere really", then "no border just border radius?"). Root cause confirmed by rendering the real markup in a browser: the old box was 0.688 against the artwork's 5:7, which **stretched** every face, and the old `rounded-md` (6px) exceeded the artwork's own radius (~4.4px), so the wrapper's corner cut outside the artwork's and exposed a sliver of backing — visible as a gap between a playable card's lime ring and its face, plus a doubled outline from the CSS border sitting over the SVG's own stroke. Amended: boxes are exact 5:7 (45x63 / 75x105 / 90x126), radius = artwork `rx` (5% of width), no CSS border on the card. Because four files restated the old numbers, geometry moved into `lib/cardFace.ts` as the single source. **KEEP:** deriving geometry from the asset rather than hand-picking it — any future deck swap changes one table; and the pointer-transparent layer stack, which the click/keyboard tests already pin.

3. **Deck ink retargeted onto theme tokens; blend layer removed (user, post-review).** The user asked whether the SVGs could carry the theme's red and black instead of the builder's `#ff0000`/`#000000` — independently the same defect both review agents raised, since `TrumpPrompt`'s chips draw the same suit in `var(--suit-red)` and put two reds on screen at once. Because `<img>`-loaded SVGs are isolated from page CSS, this can only happen in the files, so the frozen "never edit the vendored SVG sources" boundary was renegotiated: they are now **generated, then transformed** by `scripts/recolor-cards.mjs` (idempotent, documented in `docs/card-deck.md`). Amended: `red`→`#c62828`, `black`→`#1a1a1a`, court `#44F`→`#34497f` (chosen from three rendered variants; gold `#FC4` left alone because `--brass` sits too close to the parchment face to hold court detail), face rect→transparent, outline `stroke-width="3"`. Making the face transparent let the parchment background *be* the face and **deleted the `mix-blend-mode` layer entirely**, which retired four separate review findings: the double-multiply on unloaded cards, gradient-dependent tinting of top vs bottom indices, forced-colors blackout, and blend-inside-a-3D-layer risk during card flight. `sm` also went 45→56 wide: the artwork's rank index scales with the card and was 5.5px, and in an overlapped meld row it is the only cue to which cards scored. **KEEP:** the transform living in a re-runnable script rather than hand-edits — it is what makes a deck regeneration survivable; and geometry staying derived (`cardBox(width)`), which is what made the `sm` change a one-number edit.

4. **Review triage (both agents, no loopback).** No `intent_gap` or `bad_spec` findings — the change stands. Patched in place: double-multiply fallback, face-down back radius (it has no `rx` to match, and the inset brass frame was bulging past the outer corner — now concentric off a back-specific radius), `CARD_SIZES` invariants computed rather than typed, `cardFaceUrl` built off `BASE_URL`, `onError` hiding a face that cannot load (production's SPA fallback answers 200 with HTML, not 404, so browsers would paint a broken-image glyph), `warmDeck` retaining its `Image` refs and guarding on `document` rather than `Image` (jsdom defines `Image`), `DealAnimation`'s deal proxies taking `cardBox` instead of a 0.667-ratio box, `HandCards`' empty-hand height tracking `CARD_HEIGHT`, and `public/cards/` added to `.prettierignore`. Two tests added for gaps the reviewers named: all 32 `Rank x Suit` assets must exist (verified to fail on a renamed file), and the 5:7 / `rx` ratios are pinned. The provenance doc moved out of `public/` — it was being published at `/cards/README.md`. Four findings deferred to `deferred-work.md`.

5. **Single-deck path contract renegotiated for multi-deck support (Epic 12 Story 12.4, planning-time 2026-08-18).** The frozen block states the filename **is** the card ID at `/cards/{RANK}{SUIT}.svg`. Story 12.4 adds a Croatian/German-suited deck selectable as a player preference (FR63), which requires a deck segment: **`/cards/{deck}/{RANK}{SUIT}.svg`**. Renegotiated at planning time via bmad-correct-course (sprint-change-proposal-2026-08-18.md) rather than silently overridden, per this spec's own `<frozen-after-approval>` terms. **What survives unchanged:** the filename is still the card ID with no lookup table — `cardFaceUrl` gains a deck argument and nothing else; geometry stays derived and owned by `lib/cardFace.ts`; the deck files stay generated through `scripts/recolor-cards.mjs` and are never hand-edited; the marketing card in `features/landing/` stays out of scope; and the face-down monogram back is unchanged (the "Ask First" on replacing it with deck artwork still stands and is NOT taken here). **Not yet actioned** — this entry records the approved renegotiation; the move of the existing 32 files into `cards/french/` happens in Story 12.4, gated on the new deck's artwork being sourced under a compatible licence.

6. **Path contract ACTIONED, and the extension became per-deck (Epic 12 Story 12.4, implementation 2026-08-21).** Entry 5 above approved `/cards/{deck}/{ID}.svg`; this is the entry recording what actually shipped, and the one place it differs. The 32 SVGs moved by `git mv` into `client/public/cards/french/` and the Croatian deck landed beside them as 32 **WebP** — so the extension is **per deck, not global**: `cardFaceUrl(cardId, deck)` resolves `cards/{deck}/{cardId}.{ext}` off a `DECK_EXT: Record<CardDeck, string>` lookup (`{ french: "svg", croatian: "webp" }`). Reason: the Croatian art is owner-authored raster, not vector, and the raw PNGs are 16.6 MB for the deck — unshippable when `warmDeck()` fetches all 32 on match entry. Resized 450x700 -> 400x560 (Lanczos, which *bakes the 9:14 -> 5:7 stretch in* so no second aspect ratio ever reaches the renderer) and encoded WebP q80, the deck is 0.71 MB. The reproducible transform lives in `scripts/import-croatian-deck.py`, documented in `docs/card-deck.md` alongside the deck's provenance (owner-authored; **no licence required**, which is what closed entry 5's procurement gate). **Everything entry 5 promised would survive, did:** the filename is still the card ID with no per-card lookup table; `CARD_SIZES` and its 5:7 / `rx` invariants are untouched and their assertions pass unchanged; `object-fit: fill` is still an identity scale; geometry stays owned by `lib/cardFace.ts`; the French set is still generated-then-transformed by `scripts/recolor-cards.mjs` (repointed to `cards/french`, and now French-*only* by construction — the Croatian raster must never go through it); `features/landing/`'s CSS-drawn card is untouched; and the face-down monogram back is unchanged, with the "Ask First" on giving it deck artwork still standing and still not taken. **Also changed here, beyond the path:** `warmDeck`'s module-level latch became a `Set<CardDeck>` (one boolean meant the first deck mounted won for the page's lifetime, so a player who switched decks got no warm-up for the deck they were actually looking at), and `PlayingCard`'s hardcoded English `aria-label` became a locale template plus a **per-deck** suit name — a bells card must not be announced as diamonds. **KEEP:** the deck argument being passed in rather than read from a store, which is what keeps `cardFaceUrl` pure and made both decks' paths unit-testable; and the asset-existence test, now parameterised over decks, which is still the only thing standing between a renamed file and a silently blank card in play.

## Design Notes

Layer inside the existing wrapper, which already paints parchment, border, and glow:

```tsx
<img src={`/cards/${cardId}.svg`} alt="" aria-hidden="true"
     className="absolute inset-0 h-full w-full" draggable={false} />
<div className="absolute inset-0 pointer-events-none"
     style={{ background: CARD_FACE_BACKGROUND, mixBlendMode: "multiply" }} />
```

Multiply over the deck's white face yields parchment while black and red pips stay themselves. `isolate` on the card scopes the blend so the face tone does not depend on the surface it is dealt onto (felt, dialog, flight overlay). The wrapper's parchment background doubles as the loading and error fallback.

Geometry is derived from the asset, not chosen. The deck is `viewBox="-120 -168 240 336"` (5:7) with `preserveAspectRatio="none"` — it fills whatever box it is given, so a box of any other ratio distorts the artwork silently, and `rx="12"` of a 240-unit width means the artwork's corner radius is always 5% of the rendered width. Hence `CARD_SIZES` carries `{width, height: width * 1.4, radius: width * 0.05}` per size, and the card draws no CSS border of its own.

Warm-up derives its 32 IDs from the surviving `RANK_FULL_NAME` x `SUIT_FULL_NAME` keys, so no new rank/suit table appears.

The `TrumpPrompt` tiles already carry the identical gradient literal, so binding them to the constants is visually a no-op today; the point is that the card face becomes a single source of truth. The tiles keep `CARD_FACE_BORDER` even though real cards dropped it — a tile has no artwork stroke of its own, so it needs a drawn edge.

## Verification

**Commands:**

- `cd client && npx vitest run src/features/match/components/PlayingCard.test.tsx src/features/match/components/TrumpPrompt.test.tsx` — expected: all pass.
- `make lint` — expected: clean; catches any surviving reference to the deleted maps.
- `make test` — expected: full Go + Vitest suite green.

**Manual checks:**

- `make dev`, start a bot match: deal animation, hand, trick area, card flight, trump prompt (rounds 1 and 2), trump reveal, declaration prompt/reveal, belote prompt/reveal all show deck artwork.
- DevTools Network: `/cards/` requests return 200 and serve from cache on reload.

## Suggested Review Order

**Start here — the design in one place**

- Geometry is *computed* from the artwork, not chosen: ratio and radius both come from the viewBox.
  [`cardFace.ts:28`](../../client/src/features/match/lib/cardFace.ts#L28)

- Only the widths are a design choice; `sm` is deliberately wider than a straight scale-down.
  [`cardFace.ts:38`](../../client/src/features/match/lib/cardFace.ts#L38)

**The asset contract**

- Why the files are transformed rather than used raw: `<img>` SVGs cannot be themed by CSS.
  [`recolor-cards.mjs:33`](../../scripts/recolor-cards.mjs#L33)

- Court accents are deck-local, with no theme counterpart — this is their one definition.
  [`recolor-cards.mjs:43`](../../scripts/recolor-cards.mjs#L43)

- The four substitutions, including the transparent face that removed the blend layer.
  [`recolor-cards.mjs:51`](../../scripts/recolor-cards.mjs#L51)

- URL derived from the card ID, so a filename *is* the contract; `BASE_URL` keeps sub-path serving working.
  [`cardFace.ts:72`](../../client/src/features/match/lib/cardFace.ts#L72)

**The render**

- The parchment IS the face now — ink lands on it at its exact token colour, no blend.
  [`PlayingCard.tsx:251`](../../client/src/features/match/components/PlayingCard.tsx#L251)

- A face that cannot load is hidden, because production answers 200-with-HTML rather than 404.
  [`PlayingCard.tsx:272`](../../client/src/features/match/components/PlayingCard.tsx#L272)

- The back keeps its own radius: no artwork means no `rx` to coincide with, and the inset frame must nest.
  [`PlayingCard.tsx:83`](../../client/src/features/match/components/PlayingCard.tsx#L83)

- Warm-up states what it actually does, and retains refs so requests are not GC-cancelled.
  [`PlayingCard.tsx:69`](../../client/src/features/match/components/PlayingCard.tsx#L69)

**Consumers that used to restate the numbers**

- Fan arithmetic reads the shared box instead of a hardcoded 88/128.
  [`HandCards.tsx:34`](../../client/src/features/match/components/HandCards.tsx#L34)

- Trick slots and the dashed placeholder share the card's radius, so slot and card show one outline.
  [`TrickArea.tsx:152`](../../client/src/features/match/components/TrickArea.tsx#L152)

- Flight scale base tracks the real card box.
  [`CardFlight.tsx:53`](../../client/src/features/match/components/CardFlight.tsx#L53)

- Meld overlap is a fraction of card width, so a wider `sm` does not squeeze the label off a phone.
  [`DeclarationReveal.tsx:22`](../../client/src/features/match/components/DeclarationReveal.tsx#L22)

**Peripherals**

- The two gaps the reviewers named: every asset must exist, and the ratios are pinned.
  [`cardFace.test.ts:25`](../../client/src/features/match/lib/cardFace.test.ts#L25)

- Provenance, licence, and the regeneration procedure — moved out of `public/` so it is not published.
  [`card-deck.md`](../../docs/card-deck.md)
