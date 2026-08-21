---
title: "Card deck style preference"
type: "feature"
created: "2026-08-21"
status: "done"
review_loop_iteration: 0
context: ["{project-root}/_bmad-output/implementation-artifacts/epic-12-context.md"]
baseline_commit: "89e7964c08b0c6032f8a4b06957c23d30b05916a"
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Beljot draws every card from one French-suited deck. Croatian-variant players grew up on a German/Hungarian-suited deck (zelje / srce / bundeva / zir), and there is no way to choose. The artwork now exists — owner-authored, no licence needed — which unblocks the story's only real dependency.

**Approach:** Add a per-user `card_deck_preference` (`french` | `croatian`, default `french`) carried by the same `PATCH /users/:id/preferences` endpoint and auth envelope as `languagePreference`. Reorganise deck assets into `client/public/cards/{deck}/` and make `cardFaceUrl` derive `{deck}/{cardId}.{ext}` so every existing card surface follows the active deck through the one seam it already uses. New accounts default to `croatian` only when the language chosen at registration is `hr`.

## Boundaries & Constraints

**Always:**
- The card box stays the French deck's 5:7 at every size. `CARD_SIZES` and its ratio/radius invariants are unchanged; the Croatian art is pre-stretched to 5:7 at encode time, so `object-fit: fill` remains an identity scale.
- Card IDs, geometry, states, glow, the face-down back, and every `data-testid` are untouched. Only the resolved asset URL changes.
- `aria-label` names the suit as the active deck depicts it, in the player's language.
- Deck choice is purely visual: no gameplay, engine, WS-payload, or bot effect. Not purchasable.
- Persistence follows the language pattern exactly — optimistic store write, revert on failure, no reload, no match interruption.
- Deck values are `french` / `croatian`. The game *variant* value is `croatia` (`game/types.go:76`). These are unrelated enums; never cross-wire them.
- Trump-indicator accent colours (orb border, radial halo, glow, glyph ink) come from a per-deck palette, never from the `H/D`-red / `S/C`-black split. The French palette keeps its exact current values so French rendering is byte-identical.
- Every trump-indicator surface resolves its mark through one shared component. Four duplicated suit-glyph maps exist today; do not add a fifth branch to each.

**Ask First:**
- Any change to `CARD_SIZES`, the 5:7 box, or the corner-radius invariant.
- Introducing a third deck value, or exposing the preference on the public profile.

**Never:**
- Do not touch `features/landing/components/PlayingCard.tsx` — the unauthenticated landing page keeps its own CSS-drawn cards.
- Do not re-suit any suit surface outside the four trump-indicator components named in Scope Amendment 1. `shared/components/ui/suit-rule.tsx` (a decorative all-four-suits divider, and unauthenticated on the auth pages) and `features/rules/components/CardLadder.tsx` (rules reference, owned by Story 12.9) both stay French-suited in every deck.
- Do not unify the three-way auth-envelope `User` construction. Add the field to all three builders; the refactor is out of scope.
- Do not add `cardDeckPreference` to `PublicProfileResponse`.
- Do not run `scripts/recolor-cards.mjs` over the Croatian art. It is owner-authored raster, not builder SVG needing token recolouring.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Deck-only PATCH | `{"cardDeckPreference":"croatian"}` | 200, deck written, `language_preference` untouched | N/A |
| Language-only PATCH | `{"languagePreference":"mk"}` | 200, language written, deck untouched | N/A |
| Both fields | both valid | 200, both written | N/A |
| Invalid deck | `{"cardDeckPreference":"german"}` | 400 | `INVALID_CARD_DECK` |
| Invalid language | `{"languagePreference":"fr"}` | 400 | `INVALID_LANGUAGE` (unchanged) |
| Empty body | `{}` | 400 — nothing to update | `ErrBadRequest` |
| Register, `hr` chosen | `languagePreference:"hr"` | row seeded `language_preference=hr`, `card_deck_preference=croatian` | N/A |
| Register, any other/absent language | absent, `""`, `"fr"`, `en`/`mk`/`sr` | language resolves as today, deck `french` | N/A |
| SSO registration | no language input | `language_preference=en`, deck `french` | N/A |
| Deck toggle, server unreachable | PATCH rejects | auth-store deck reverts to previous; rendered deck follows the store, so cards revert too | silent, matches `LanguageSelector` |
| Missing deck asset | face 200s with SPA HTML | `onError` hides the img, leaving blank parchment | existing behaviour preserved |
| Trump mark, french deck | `deck=french`, any suit | Unicode glyph as today, unchanged colours | N/A |
| Trump mark, croatian deck | `deck=croatian`, any suit | `<img>` of that suit's icon, accent from the Croatian palette | `onError` hides the img, leaving the orb |
| Croatian accent per suit | each of S/H/D/C | four visually distinct accents; leaves and acorns never resolve to the same colour | N/A |
| Non-indicator suit surface | any deck | `suit-rule` and `CardLadder` render French glyphs | N/A |

## Scope Amendment 1 — trump-indicator artwork

Authorised by the project owner on 2026-08-21, after this spec was approved, superseding the original **Never** clause that held UI chrome French-suited. Four surfaces follow the active deck: `TrumpIndicator`, `TrumpPrompt`, `TrumpReveal`, and the `PlayerSeat` trump chip. The owner's wording bounds it — *"symbols used on the UI and dialogs as trump indicators"* — so decorative and rules-reference suit surfaces are explicitly excluded.

</frozen-after-approval>

## Code Map

**The one seam.** `client/src/features/match/lib/cardFace.ts:72` `cardFaceUrl(cardId)` returns `${BASE_URL}cards/${cardId}.svg`. Its only callers are `PlayingCard.tsx:76` (inside `warmDeck`) and `PlayingCard.tsx:264` (the `img src`). Every other card surface — `HandCards`, `TrickArea`, `CardFlight`, `DealAnimation`, `TrumpPrompt`, `TrumpReveal`, `DeclarationPrompt`, `DeclarationReveal`, `BelotPrompt`, `BelotReveal` — renders `<PlayingCard>` and inherits the deck for free.

- `PlayingCard.tsx:69-79` — `warmDeck()` is latched by a module-level `deckWarmed` boolean. It must become deck-keyed, or the first deck mounted wins for the page's lifetime.
- `PlayingCard.tsx:28-42` — `RANK_FULL_NAME` / `SUIT_FULL_NAME` hardcode English; `:159` builds `"${rank} of ${suit}"`. This is the `aria-label` work.
- `cardFace.ts:19-46` — `ART_ASPECT`/`ART_RADIUS_RATIO`/`CARD_SIZES`. Read-only: the Croatian art is encoded to 5:7 so these hold.
- `cardFace.test.ts:17,21` assert `/cards/KS.svg`; `:33` asserts `existsSync(public/cards/{ID}.svg)`; `PlayingCard.test.tsx:25,32,39` assert the `src`. These pin the old flat path and must be updated for both decks. `:42,46` (ratio/radius) must keep passing unchanged.
- `scripts/recolor-cards.mjs:29,65` — `CARDS_DIR` is the flat `public/cards`; after the move it finds zero SVGs. Repoint to `cards/french`.
- `_bmad-output/implementation-artifacts/spec-svg-card-deck.md` — `done` and frozen; its `/cards/{ID}.svg` path contract is what this story renegotiates. Append a Spec Change Log entry there rather than silently overriding it.

**Trump-indicator surfaces (Scope Amendment 1).** Four components each declare their own suit-glyph map and their own red/black colour split. Render sizes are the ceiling that sets the icon resolution — the largest is 30 px.

- `TrumpIndicator.tsx:19` `SUIT_SYMBOL`, `:44-48` `suitColor()`, glyph at `:166` (fontSize 22) inside the 44 px parchment orb at `:150-165`. The orb's `border`, `radial-gradient` halo and `boxShadow` are all keyed off the same `H/D` red / `S/C` black test and are the most visible thing to get wrong.
- `TrumpPrompt.tsx:46` `SUIT_SYMBOL`, glyphs at `:111` (fontSize 28, the suit-choice tiles) and `:257` (fontSize 15). `cardFace.ts:56` `CARD_FACE_BORDER` exists specifically for these tiles.
- `TrumpReveal.tsx:41` `SUIT_GLYPH`, glyph at `:283` (fontSize 30 — the largest suit mark in the app, so it sets the asset resolution).
- `PlayerSeat.tsx:49` `SUIT_GLYPH`, glyph at `:303` (fontSize 18, the trump-caller chip).
- Read-only, must stay French: `shared/components/ui/suit-rule.tsx:15` (decorative divider, all four suits at once, fontSize 17) and `features/rules/components/CardLadder.tsx:15-31` (`SUIT_GLYPH` + `SUIT_COLOR` + `SuitGlyph`, Story 12.9's territory).
- Icons already imported: `client/public/suits/croatian/{S,H,D,C}.webp`, 64x64 lossless WebP with alpha, 9.8 KB total, via `scripts/import-croatian-suits.py`. The French deck ships no icon files — it keeps its Unicode glyphs.

**Server (mirror `language_preference` throughout).**
- `user/model.go:14` — `LanguagePreference` closes the identity block; the deck field goes at `:15`, before the wallet comment.
- `user/handler.go:104-106` `UpdatePreferencesRequest` (single required field), `:108-113` `supportedLanguages`, `:523-556` `UpdatePreferences`. Today a deck-only PATCH 400s with `INVALID_LANGUAGE` because `""` misses the allowlist — both fields must become `*string`.
- `user/repository.go:30` + `gorm_repo.go:141` — `UpdateLanguagePreference`. Add a sibling method; `gorm_repo.go:170` (`UpdateUsername`) is the multi-column `Updates(map[...])` precedent. **Seven** stubs implement the interface and each needs a one-line addition: `user/handler_test.go:252`, `auth/handler_test.go:22`, `lobby/lobby_test.go:91`, `chat/handler_test.go:20`, `friend/handler_test.go:172`, `user/xp_service_test.go:15`, `user/honor_service_test.go:17`.
- `auth/handler.go:169-193` `authResponseData` is the single envelope builder (called `:271`, `:317`, `:453`, `sso_handler.go:291`); field list `:37-65`. Only two non-test `&user.User{}` row literals exist: `auth/handler.go:247-255` (Register — language resolved at `:229-232`, derive deck there) and `auth/sso_handler.go:163-171`.
- `user/handler.go:23-70` `ProfileResponse` carries `languagePreference` at `:29`, populated `:335`. `PublicProfileResponse` `:80-102` deliberately omits it; `handler_test.go:944` and `:1742` assert the public body never contains it.
- `apperr/errors.go:73` `ErrInvalidLanguage` — pattern for the new `ErrInvalidCardDeck`.
- Migrations: next is `000020`. `000018_add_honor_gate_to_rooms.up.sql:45` and its `.down.sql:1-8` are the add-column/reverse-drop style. Constrained strings carry **no** CHECK — `000002_create_users.up.sql:6` is `language_preference VARCHAR(10) NOT NULL DEFAULT 'en'`; validation lives in Go.
- No Go golden file or testdata fixture contains `languagePreference`, so the new field breaks no strict comparison.

**Client plumbing.**
- `shared/types/apiTypes.ts:16-43` `User` (12 fields, no Zod — HTTP responses are cast, not parsed).
- Three field-by-field envelope builders all need the new field: `shared/api/axiosClient.ts`, `shared/hooks/mutations/useAuth.ts`, `shared/hooks/useAuth.ts`.
- `shared/api/profile.ts:69-71` `UpdatePreferencesRequest` and `:87-92` `updatePreferences` (return type is an inline `Promise<{ languagePreference: string }>` — widen both). Callers: `LanguageSelector.tsx:60`, `SettingsDialog.tsx:62`, `reconcileLanguage.ts:29`.
- `test-utils.tsx:18` `makeUser(overrides)` defaults every `User` field (`languagePreference` at `:23`); one line there keeps all 81 call sites compiling.
- `features/match/components/SettingsDialog.tsx:88-160` — the Language `<section>` with its `role="radiogroup"` and `settings-language-option-{code}` test ids. The deck section is a copy of this block.
- `features/profile/components/SidePanel.tsx:6-12` — `{eyebrow, title, children, testId}`; `LinkedAccounts.tsx:93-98` is the composition to copy. Mount beside `LinkedAccounts` at `ProfilePage.tsx:165`, outside the `career` gate. `PublicPlayerProfilePage.test.tsx:154` is the precedent for asserting a private panel never mounts publicly.
- i18n: `shared/i18n/{en,hr,mk,sr}.json`. `i18n.parity.test.ts:64` enforces exact dotted-key-set equality against `en.json`; `:87` rejects any empty or whitespace-only leaf in all four. Key order is free; placeholders and punctuation are not checked.

**Asset source.** 32 PNGs at `D:\Downloads\Croatian Card Deck`, 450x700 RGB, ~520 KB each (16.6 MB total — unshippable, and `warmDeck` fetches all 32 on match entry). Two are misnamed: `AQ.png` and `TQ.png` are both *bells*, i.e. `AD` and `TD`. Suit mapping verified visually against the French set: leaves=S, hearts=H, bells=D, acorns=C.

## Tasks & Acceptance

**Execution:**

- [x] `client/public/cards/french/` -- `git mv` all 32 SVGs into it -- per-deck folders are the epic's asset layout; keeps both decks symmetric.
- [x] `client/public/cards/croatian/` -- add 32 WebP faces: resize each source PNG to 400x560 (Lanczos, stretching 9:14 to the deck's 5:7) and encode WebP q80; import `AQ`->`AD` and `TQ`->`TD` -- 0.71 MB for the deck versus 16.6 MB raw, at 4.4x the largest render.
- [x] `docs/card-deck.md` -- document both decks, the new `{deck}/` layout, Croatian provenance (owner-authored, no licence required), and the exact resize/encode recipe -- provenance must never need re-establishing.
- [x] `scripts/recolor-cards.mjs` -- repoint `CARDS_DIR` at `cards/french` -- it silently processes zero files otherwise.
- [x] `server/migrations/000020_add_card_deck_to_users.{up,down}.sql` -- add `card_deck_preference VARCHAR(20) NOT NULL DEFAULT 'french'`, reversing drop -- default is the backfill.
- [x] `server/internal/user/model.go` -- add `CardDeckPreference` after `LanguagePreference` -- keeps the identity block ordered.
- [x] `server/internal/apperr/errors.go` -- add `ErrInvalidCardDeck` -- domain errors are never inline.
- [x] `server/internal/user/repository.go` + `gorm_repo.go` -- add `UpdateCardDeckPreference` -- narrower than reshaping the language method's signature.
- [x] seven repository stubs (see Code Map) -- add the method -- interface compliance.
- [x] `server/internal/user/handler.go` -- make both `UpdatePreferencesRequest` fields `*string`, validate and write each independently, echo both; add the deck to `ProfileResponse` and its self-branch -- a deck-only PATCH must not require resending language.
- [x] `server/internal/auth/handler.go` -- add the field to `RegisterResponseData` and `authResponseData`; in `Register`, seed `croatian` when the resolved language is `hr`, else `french` -- one envelope, one derivation point.
- [x] `server/internal/auth/sso_handler.go` -- seed `french` -- SSO has no language input.
- [x] `client/src/shared/types/apiTypes.ts` + `axiosClient.ts` + `hooks/mutations/useAuth.ts` + `hooks/useAuth.ts` + `test-utils.tsx` -- carry `cardDeckPreference` -- three builders is known duplication; unification is out of scope.
- [x] `client/src/shared/api/profile.ts` -- make both request fields optional and widen the response type -- deck-only and language-only PATCHes must both type-check.
- [x] `client/src/features/match/lib/cardFace.ts` -- add a `CardDeck` type and make `cardFaceUrl(cardId, deck)` derive `cards/{deck}/{cardId}.{ext}` with the extension per deck -- derived, never a per-card lookup table.
- [x] `client/src/features/match/components/PlayingCard.tsx` -- read the active deck from `authStore`, key the `warmDeck` latch by deck, and build `aria-label` from localized rank + per-deck suit names -- a Croatian card must not be announced as a French suit.
- [x] `client/src/shared/i18n/{en,hr,mk,sr}.json` -- add the card-label keys (8 ranks, 4 suits per deck, face-down, label template) and the deck-picker labels -- parity test requires identical key sets and non-empty leaves in all four.
- [x] `client/src/features/match/components/SettingsDialog.tsx` -- add a Card Deck section mirroring Language, with `settings-deck-option-{deck}` test ids -- same optimistic-with-revert persistence.
- [x] `client/src/features/profile/components/CardDeckPanel.tsx` + `ProfilePage.tsx` -- new `SidePanel` beside `LinkedAccounts` -- the deck is settable outside a match.
- [x] tests -- update `cardFace.test.ts` (both decks' paths and asset existence) and `PlayingCard.test.tsx` (src per deck, localized labels); add Go coverage for every I/O Matrix row; assert the panel never mounts on `PublicPlayerProfilePage` -- the Matrix is the contract.
- [x] `_bmad-output/implementation-artifacts/spec-svg-card-deck.md` -- append a Spec Change Log entry recording the `/cards/{ID}.svg` -> `/cards/{deck}/{ID}.{ext}` renegotiation -- a frozen contract is renegotiated in writing, never silently.

**Execution — Scope Amendment 1:**

- [x] `client/src/features/match/lib/suitArt.ts` -- new: `suitIconUrl(suit, deck)`, `SUIT_ACCENT: Record<CardDeck, Record<Suit, string>>` with the sampled Croatian values, and a `SuitMark` component that renders the Unicode glyph for french and an `<img>` for croatian -- one seam instead of a fifth copy of the glyph/colour branch in four files.
- [x] `client/src/features/match/components/TrumpIndicator.tsx` -- replace `SUIT_SYMBOL` and `suitColor()` with `SuitMark` + `SUIT_ACCENT`; drive the orb border, halo and glow from the accent -- the red-halo-around-a-gold-bell case is the whole point of the amendment.
- [x] `client/src/features/match/components/TrumpPrompt.tsx` -- `SuitMark` at both sites (28 px tiles, 15 px inline), accent-driven tile colour -- suit choice must show the deck the player will actually see.
- [x] `client/src/features/match/components/TrumpReveal.tsx` -- `SuitMark` at 30 px -- largest suit mark in the app.
- [x] `client/src/features/match/components/PlayerSeat.tsx` -- `SuitMark` at 18 px for the trump-caller chip -- last surface still on the old glyph map.
- [x] `client/src/features/match/lib/suitArt.test.ts` + component tests -- cover the four new matrix rows: french renders a glyph, croatian renders the icon `<img>`, the four Croatian accents are pairwise distinct (pinning leaves != acorns), and `suit-rule` / `CardLadder` still render French glyphs under the Croatian deck -- the last one is the only guard on the excluded surfaces.
- [x] `docs/card-deck.md` -- document the suit-icon set alongside the faces: source, 64x64 lossless rationale, the importer, and the accent palette with its sampled provenance -- provenance must never need re-establishing.

**Acceptance Criteria:**

- Given a player with `cardDeckPreference: "croatian"`, when any card renders in a match, then every surface draws the Croatian face — hand, trick area, card flight, deal animation, and every trump, declaration, and Belote prompt and reveal.
- Given the Croatian deck is active, when a trump indicator, trump prompt, trump reveal or seat trump chip renders, then it draws the Croatian suit icon with its own accent colour, and no red or black accent appears on a gold bell or a green leaf.
- Given the French deck is active, when any of those four surfaces renders, then its output is unchanged from before this story — same glyphs, same colours.
- Given the Croatian deck is active, when the decorative suit divider or the rules-page card ladder renders, then both still show French glyphs.
- Given a player toggles the deck mid-match, when the selection lands, then the cards on the table change deck with no reload and no interruption to play.
- Given a player registers with `hr` selected, when their account is created, then their deck is `croatian`; for every other language it is `french`.
- Given a player signs in on another browser, when the auth envelope resolves, then their stored deck is applied before the first card renders.
- Given every pre-existing `CARD_SIZES` ratio and radius assertion, when the suite runs, then it passes unchanged.
- Given `make lint` and `make test`, when run, then both pass.

## Spec Change Log

- **2026-08-21 — Scope Amendment 1, owner-authorised (not a review finding).** The owner supplied trump-indicator artwork after approval and directed that it ship in this story. Amended: the **Never** clause holding UI chrome French-suited was replaced with a bounded version; four matrix rows, one Code Map block, seven tasks and three ACs were added. Avoided known-bad state: dropping the icons into the existing colour logic, which paints a red halo around a gold bell and a black one around a green leaf. **KEEP on any re-derivation:** the exclusion of `suit-rule.tsx` and `CardLadder.tsx` (owner's wording was "trump indicators"; the ladder is Story 12.9's), the single shared `SuitMark` seam rather than a fifth per-file branch, and the requirement that French output stay byte-identical.
- **2026-08-21 — Scope Amendment 1 implemented; two deltas from the written tasks.** (1) The task listed `SuitMark` inside `suitArt.ts`, but a `.ts` file cannot hold JSX, and putting both in one `.tsx` made `react-refresh/only-export-components` fire five times — the linter naming the real problem, since a module exporting a component *and* constants breaks fast refresh. Split instead, mirroring `cardFace.ts` + `PlayingCard.tsx` next door: `lib/suitArt.ts` stays **pure** (palette, `suitIconUrl`, glyph map — unit-testable with no DOM, and the spec's exact filename, with `suitArt.test.ts` alongside) and `components/SuitMark.tsx` holds the component. Still one seam for the mark and one table for the palette, which is what the task was protecting. (2) `SUIT_ACCENT` alone could not keep French byte-identical, so it ships with two companion tables beside it: `SUIT_GLOW_ALPHA` (French's pre-existing halo asymmetry — `77` on the reds, `55` on the blacks; a near-black glow at the red's strength reads as a smudge under the parchment orb) and `SUIT_INK_ON_FELT` (`TrumpReveal`'s body-copy suit word, which sits on dark felt where `#c62828` is unreadable — French keeps `#ff8585`/`#f5f2e8` exactly). Both are per-deck-per-suit lookups, not red/black tests, and both live beside the accent so the palette is still one file to read and one to change. Also: `TrumpIndicator` keeps a `LEGACY_INK_CLASS` per-suit map feeding `SuitMark`'s `className`, because four pre-existing tests assert on `.text-red-500` / `.text-text-primary`; those classes have never carried colour (the inline `color` always won), and expressing them as a per-suit lookup means no `H/D` conditional survives in that file. **KEEP:** the French rows of all three tables being test-pinned to their pre-story literals — that is the only thing standing between a future palette edit and a silent restyle of the deck nobody was changing.
- **2026-08-21 — step-04 adversarial review: 16 findings, all patched, no spec amendment and no revert.** Nothing in the frozen block changed. The headline finding was that Scope Amendment 1 had re-suited the *artwork* but not the *words*: all four surfaces still resolved `match.suits.diamonds`, so the Croatian deck showed a gold bell captioned and announced "Diamonds" — a direct violation of the frozen `Always` ("`aria-label` names the suit as the active deck depicts it, in the player's language"), and `PlayerSeat`'s chip title was hardcoded English on top of that. Every suit/rank name on the four surfaces now resolves through `suitNameKey(suit, deck)` / `rankNameKey(rank)`, and the two parallel vocabularies collapsed into one: the word-keyed `match.ranks.*` / `match.suits.*` are **deleted** (they duplicated `match.card.*` verbatim in all four locales, and the parity test compares key sets, not values, so nothing could detect them drifting). `ScoreReveal` moved onto the same vocabulary but **pins `french`** — it is outside the four surfaces the amendment authorised, and the Never clause forbids re-suiting it. Also fixed: two prototype-chain holes of the same class (`resolveCardDeck` used `value in DECK_EXT`, so `"toString"` resolved as a deck; and the palette/glyph lookups returned `Object.prototype.toString` — a *function* — which `??` cannot catch and React throws on, found by a test written for the first hole); a stuck-hidden face (`onError` set `visibility` imperatively and React reuses the node across `src` changes, so one transient miss blanked a card for the page's lifetime and survived a deck switch — the img is keyed by its resolved URL now, with `onLoad` clearing as the twin); the non-atomic both-field write (two sequential repo calls became one multi-column `UpdatePreferences(id, *lang, *deck)`, replacing both per-column methods, so a 500 can no longer leave the language committed and the deck reverted); the deck-toggle race (both entry points now share one latest-wins `persistCardDeck` helper, reverting to the *resolved* previous value); `undefined` interpolated into `border`/`color` when an unvalidated suit arrives off the `z.string()` WS contract; deck labels colliding with language names in hr/mk ("Hrvatski" under both headings — now "Hrvatske karte"); a Bulgarian "коя" in mk; `SuitMark` dropping the caller's `className` and text-shadow on the icon branch (a `shadow` prop now maps to `text-shadow` or `drop-shadow` per branch); and the deck preview showing hearts, the one suit that looks the same in both decks (now bells vs diamonds). `CardDeck` moved to `shared/types/matchTypes.ts` beside `Suit`/`Variant` — `shared/` was importing it from `features/` to type its own wire field — and `cardFace.ts` re-exports it. Coverage added for every gap the review named: the deck-keyed `warmDeck` latch (four cases, and the harness has to import the component and the store from the *same* `vi.resetModules()` graph or the switch is invisible), `TrumpPrompt`'s waiting-branch "considering" chips, `TrumpReveal` actually reading `SUIT_INK_ON_FELT`, the Language rows' PATCH body and `aria-checked` through the shared `SettingRow`, a `CardDeckPanel` suite of its own, the atomic-write failure path, and face recovery. One test that **asserted the defect** (`getByLabelText("Diamonds")` with the Croatian deck active) was corrected, not preserved. **KEEP:** one vocabulary rather than two, and the deck argument threaded through the *name* lookup as well as the artwork — the words and the pictures are the same contract, and splitting them is what produced the worst bug in the change.
- **2026-08-21 — matrix audit gap closed during step-03 verification.** The "missing deck asset" row had no test; the asset-existence tests prove presence but never exercise `onError`. Added a per-deck test firing the error event and asserting the img is hidden while the card survives. **KEEP:** per-deck parameterisation — the two decks carry different extensions, so a path bug can exist in one and not the other.
- **2026-08-21 - owner follow-up after step-05: three directed items.** (1) `ScoreReveal` no longer pins the French vocabulary - the owner extended Scope Amendment 1's naming rule to it. It draws no suit artwork at all, so the name is its only representation and there was nothing for a deck-aware name to contradict; the decorative divider and the rules-page ladder stay excluded, unchanged. (2) The two palette values flagged as judgment are now derived and pinned: `SUIT_GLOW_ALPHA` from accent luminance (a rule validated by reproducing French's pre-existing 55/77/77/55 exactly), and `SUIT_INK_ON_FELT` against a measured contrast floor. **The measurement found a real defect the eye had missed** - acorn ink shipped at 4.73:1, under the 4.89:1 French bar - now `#e19967`. (3) A pre-existing flake in Story 12.6's `TestDeclarationPhase_TimerExpiryOnLastSeatOpensTrick1` is fixed at root cause. Known-bad state avoided: leaving `make test` unable to distinguish a real regression from scheduler noise. **KEEP on any re-derivation:** the contrast floor and luminance assertions (they are what caught the acorn value), the strict floor comparison rather than a tolerance, and `indexOfMatchStateWithPhase` staying free of `*testing.T` so it is legal inside a testify `Eventually` goroutine.
- **2026-08-21 - owner review of the Croatian suit names: casing corrected in mk/hr/sr.** All four Croatian-deck suit names were stored capitalised, but they are ordinary common nouns (zelje / srce / bundeva / zir, and their Macedonian equivalents) rather than the French row's borrowed, capitalised card words (Pik / Herc / Karo / Tref). Most templates place the suit mid-sentence - "zvao bundeva za aduta", "go zede zvonche za adut", "adut na ..." - and none of these languages capitalises common nouns there, so the capital was wrong at every such call site. Now stored lower case in hr/sr/mk; `en` stays capitalised because English does capitalise suit names, and the French rows are untouched (they are pre-existing values, and the story's AC requires French output to be unchanged). Safe because the only surface needing a leading capital, `TrumpIndicator`'s caption, takes it from CSS `capitalize` rather than the stored string. `i18n.parity.test.ts` now pins the casing per locale - the parity checks either side of it cover key sets and non-empty leaves, never wording, so nothing else would have caught it. **A related claim was withdrawn, not implemented:** the suit-first label template (`{{suit}} {{rank}}`) was queried as unidiomatic, but suit-first nominative is how Croatian and Serbian card players actually speak ("pik dama", "herc as", from German Pikdame/Herzas), so extending it to the Croatian deck is correct. A genitive rewrite would also have needed a second key set, since the same stored value is used standalone ("Adut: bundeva"). **KEEP on any re-derivation:** the casing test, and the suit-first template.
- **2026-08-21 - owner corrected the Macedonian and Serbian suit vocabulary.** Macedonian acorns was `zhir` (Cyrillicised from Serbo-Croatian) where the Macedonian word is `zhelad`, corrected by the project owner - a native speaker - and the rule that rule authority is the owner applies to locale wording exactly as it does to game rules. That correction revealed the pattern: native Macedonian uses the LITERAL readings throughout (leaf / bell / acorn / heart), while the hr and sr rows had been given Croatian COLLOQUIAL readings (`zelje` = greens, `bundeva` = pumpkin) and were byte-identical to each other, which the epic's own "Croatian and Serbian forms are never mixed" constraint forbids. Serbian is now `list / srce / zvono / zhir` on owner decision. Note what legitimately does NOT differ: `srce` is the same word in hr and sr, and `zhir` is correct Serbian while Macedonian is `zhelad` - so neither identical values nor differing ones are evidence of an error on their own. **KEEP on any re-derivation:** the four locales' suit rows are individually owner-reviewed wording, not derivable from each other by translation - do not regenerate them, and do not "fix" hr and sr into agreement.

## Design Notes

`cardFaceUrl` gains a deck argument rather than reading a store, so it stays pure and unit-testable; `PlayingCard` is the single component that resolves the active deck and is already the only caller.

```ts
export type CardDeck = "french" | "croatian";
const DECK_EXT: Record<CardDeck, string> = { french: "svg", croatian: "webp" };

export function cardFaceUrl(cardId: CardId, deck: CardDeck): string {
  return `${import.meta.env.BASE_URL}cards/${deck}/${cardId}.${DECK_EXT[deck]}`;
}
```

The Croatian art is 9:14; the card box is 5:7. Rather than change the box or letterbox the art, the resize *bakes the stretch in* — so nothing downstream learns that a second aspect ratio exists, and `object-fit: fill` stays an identity scale. At the largest render (90 px) the ~11% horizontal stretch is imperceptible.

`aria-label` composes from a locale template plus a per-deck suit name, so each locale controls word order and case:

```
match.card.label            "{{rank}} of {{suit}}"
match.card.rank.7 … .A      eight rank names
match.card.suit.french.D    "Diamonds"
match.card.suit.croatian.D  "Bells" / hr: "bundeva"
```

Suit values are authored in whatever grammatical form the locale's own `label` template needs — the four locales are not required to share word order.

**Suit marks (Scope Amendment 1).** The French deck keeps its Unicode glyphs — they are already sized, coloured and kerned across four surfaces, and swapping them for images would change French rendering for no reason. So `SuitMark` branches on deck, not on asset availability:

```tsx
// french -> glyph (unchanged); croatian -> icon. Accent is the caller's to apply
// to borders/halos, so the orb and the tiles stay in charge of their own chrome.
export const SUIT_ACCENT: Record<CardDeck, Record<Suit, string>> = {
  french:   { S: "#1a1a1a", H: "#c62828", D: "#c62828", C: "#1a1a1a" },
  croatian: { S: "#4a7a3a", H: "#c62828", D: "#c9a23c", C: "#96501e" },
};
```

The Croatian values are sampled from the artwork by coverage, not eyeballed: hearts are 30% `#e00020`; bells read gold (`#e0c020`) over a dark outline; leaves are 25% `#204020`/`#406020`. Leaves **and** acorns are both green-dominant, so acorns take the brown of their nut (`#a04000`) — the conventional Grün/Eichel split. Collapsing them to the same green would defeat the point of a per-suit palette, which is why a test pins them pairwise distinct.

At 64x64 the icon covers the largest mark (30 px) at ~2.1x. That was the owner's call over a 96 px downscale of the 500x500 masters, which would have cost 7.8 KB more for the whole set; if 3x-DPR softness ever matters, `import-croatian-suits.py` documents the one-constant change.

**Asset encode — use this, do not improvise.** This machine has no `sharp`, no PIL, and no ImageMagick, and none of them should become a project dependency for a one-time art import. `uv` is installed, so run the transform with an ephemeral dependency and keep the script in `scripts/` so the import stays reproducible:

```sh
uv run --with pillow python scripts/import-croatian-deck.py
```

Per file, that is `Image.open(src).convert("RGB").resize((400, 560), Image.LANCZOS).save(dst, "WEBP", quality=80, method=6)`, reading from the source directory named in the Code Map and mapping `AQ` to `AD` and `TQ` to `TD`. This exact recipe was run and verified during planning: 32 files, 0.71 MB total, ~23 KB each. The `resize` to 400x560 is what bakes in the 9:14 to 5:7 stretch — do not add padding and do not preserve the source aspect ratio.

## Verification

**Commands:**

- `make lint` -- expected: ESLint, Prettier and golangci-lint all clean.
- `make test` -- expected: `go test ./...` and `npx vitest run` both green, including the updated `cardFace` / `PlayingCard` suites and the i18n parity test.
- `make migrate` -- expected: `000020` applies; re-running the down migration drops the column cleanly.

**Manual checks:**

- With the Croatian deck active, enter a match and confirm the hand, a resolved trick, the trump prompt and a declaration reveal all draw Croatian faces at the same size and corner radius as before.
- Toggle the deck from the in-match Settings dialog mid-hand and confirm the table re-skins without interrupting play; toggle again from the profile sidebar.
- With devtools throttling the PATCH to a failure, confirm the choice reverts.

## Suggested Review Order

**The preference contract (start here)**

- The whole feature in one function: deck + card ID derive the URL, no lookup table.
  [`cardFace.ts:116`](../../client/src/features/match/lib/cardFace.ts#L116)

- Own-property check, not `in`: `"toString" in DECK_EXT` was true and passed as a deck.
  [`cardFace.ts:98`](../../client/src/features/match/lib/cardFace.ts#L98)

- Type sits beside `Suit`/`Variant` so `shared/` never imports from `features/`.
  [`matchTypes.ts:29`](../../client/src/shared/types/matchTypes.ts#L29)

**Persistence**

- Column mirrors `language_preference` exactly: no CHECK, validation lives in Go.
  [`000020_add_card_deck_to_users.up.sql:31`](../../server/migrations/000020_add_card_deck_to_users.up.sql#L31)

- One UPDATE for both columns, so a failed second write cannot half-apply a PATCH.
  [`gorm_repo.go:145`](../../server/internal/user/gorm_repo.go#L145)

- Optional pointers: validates every supplied field before writing any, echoes only what it wrote.
  [`handler.go:571`](../../server/internal/user/handler.go#L571)

- The only place the default is derived: `hr` at registration means the Croatian deck.
  [`handler.go:249`](../../server/internal/auth/handler.go#L249)

- Rides the shared envelope, so the deck is known before the first card renders.
  [`handler.go:188`](../../server/internal/auth/handler.go#L188)

**The render seam**

- Subscribing to the store here is what re-skins a live table with no reload.
  [`PlayingCard.tsx:8`](../../client/src/features/match/components/PlayingCard.tsx#L8)

- Latch is a Set, not a boolean: otherwise the first deck mounted wins for the page's life.
  [`PlayingCard.tsx:59`](../../client/src/features/match/components/PlayingCard.tsx#L59)

**Trump-indicator chrome (Scope Amendment 1)**

- Accents sampled from the artwork; leaves and acorns must not collapse to one green.
  [`suitArt.ts:74`](../../client/src/features/match/lib/suitArt.ts#L74)

- Two companion tables exist solely to keep French rendering byte-identical.
  [`suitArt.ts:110`](../../client/src/features/match/lib/suitArt.ts#L110)

- Deck-aware suit naming: the fix for a gold bell captioned "Diamonds".
  [`suitArt.ts:205`](../../client/src/features/match/lib/suitArt.ts#L205)

- One seam replacing four duplicated glyph maps: glyph for French, `img` for Croatian.
  [`SuitMark.tsx:56`](../../client/src/features/match/components/SuitMark.tsx#L56)

- Orb border, halo and glow all now follow the accent rather than a red/black test.
  [`TrumpIndicator.tsx:156`](../../client/src/features/match/components/TrumpIndicator.tsx#L156)

- The largest suit mark in the app at 30px, which set the icon resolution.
  [`TrumpReveal.tsx:221`](../../client/src/features/match/components/TrumpReveal.tsx#L221)

**The two pickers**

- Shared latest-wins token: two fast toggles cannot leave store and server disagreeing.
  [`cardDeckPreference.ts:38`](../../client/src/shared/lib/cardDeckPreference.ts#L38)

- Deck section mirrors the language section it sits beside.
  [`SettingsDialog.tsx:194`](../../client/src/features/match/components/SettingsDialog.tsx#L194)

- Previews the bells ace, a suit that actually differs between decks.
  [`CardDeckPanel.tsx:27`](../../client/src/features/profile/components/CardDeckPanel.tsx#L27)

**Assets and provenance**

- Resize bakes the 9:14 to 5:7 stretch in, so nothing downstream sees a second ratio.
  [`import-croatian-deck.py:48`](../../scripts/import-croatian-deck.py#L48)

- Lossless with alpha; hard-errors if a source ever loses transparency.
  [`import-croatian-suits.py:52`](../../scripts/import-croatian-suits.py#L52)

- Both decks' provenance and the sampled-to-shipped accent derivation.
  [`card-deck.md:1`](../../docs/card-deck.md#L1)

**Peripherals**

- The only guard on the excluded surfaces: French glyphs under the Croatian deck.
  [`suitScope.test.tsx:1`](../../client/src/features/match/components/suitScope.test.tsx#L1)

- Hostile-input table that caught the prototype-chain defect.
  [`cardFace.test.ts:1`](../../client/src/features/match/lib/cardFace.test.ts#L1)

- Deck-aware `aria-label` plus the per-deck missing-asset recovery cases.
  [`PlayingCard.test.tsx:1`](../../client/src/features/match/components/PlayingCard.test.tsx#L1)
