---
title: "Variant rules-reference reconciliation"
type: "feature"
created: "2026-08-21"
status: "done"
review_loop_iteration: 0
context: ["{project-root}/_bmad-output/implementation-artifacts/epic-12-context.md"]
baseline_commit: "fba840984aba617a91c173022a0ccd1315f7e7cb"
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** The rules reference describes one ruleset — Bitola — and Croatian rooms are now creatable. Four statements are false for Croatian (the 3+2 deal, the face-up candidate, melds "announced on your turn in the first trick", the `facts` deal caption); three rules are documented nowhere (the round-2 reveal, the round-2 all-pass outcome, whether a card may count in more than one declaration); and three shared rules are wrong or missing for both variants — the both-teams-cross-**tied** tiebreaker (D137), Capot, which no locale mentions at all, and the all-eight instant win, which the page states as "any suit" while the engine requires the **trump** suit. The page also calls the game "Beljot" — the brand — not what players call it.

**Approach:** Keep one content spine and split only where the engine diverges. Blocks gain an optional variant scope and an optional "differs in the other variant" note; a variant tab bar picks which scope renders, and each scoped block carries a hover/tap marker saying what the other variant does. Four of the six chapters stay identical between tabs, which is the point. Then close the shared-rule gaps and rename the game per locale.

## Boundaries & Constraints

**Always:**
- Rule authority is the owner, not published guides. Every sentence must match engine behaviour verified in the Code Map.
- All four locales move together — `rulesContent.parity.test.ts` gates section ids, order and block shapes, so every block's variant scope must be identical across en/mk/hr/sr.
- Localization rules hold: mk all-Cyrillic, hr ijekavica, sr ekavica, never mixed; em-dashes en-only; `„…"` quotes in mk/hr/sr; "contract" banned as a game term; non-English must read idiomatically, not as calques.
- **The game, the +20 K+Q bonus and the all-eight-trumps hand deliberately share one word** in every locale — the game is named after the hand. Each of the three uses must be unambiguous from its own sentence; none is renamed to avoid the overlap.
- Tab labels come from `variantLabel` in `shared/lib/roomLabels.ts`. No new variant i18n key family.
- The difference marker must open on tap as well as hover — hover-only is unreachable on a phone.
- Server, engine and `features/match/lib/variantRules.ts` are untouched. This is a client content story.

**Decided (do not re-open):**
- Game name: en "Belote", mk „Бељот" (already correct), hr/sr "Bela" — inflected, so "Bela se igra s 32 karte", "Nauči Belu u jednom sjedenju", „Pravila Bele".
- The `bela` meld keeps its existing per-locale name (en "Belote", mk „Бељот", hr/sr "Bela"). Only its rule text changes, to all eight cards of the **trump** suit.

**Ask First:**
- Any *further* divergence found between a rules-page statement and engine behaviour while writing copy. Two turned up during planning that no story had flagged; report rather than silently correcting, because the owner is the rule authority.
- Any change that would touch `shared.ts` `DECLARATIONS_BASE` or the parity test's declaration-ids assertion — that is a wire-shape change to the content module, not a copy fix.

**Never:**
- Do not duplicate whole chapters per variant. Only genuinely divergent blocks are scoped.
- Do not add a per-variant tie rule. A tied hand fails for the taker in both variants (12.7 deferred); the existing tie sentence is correct and stays.
- Do not implement the any-suit instant win, and do not move the `bela` meld out of `DECLARATIONS_BASE`.
- Do not rename the app/brand. "Beljot" stays in `appName`, the copyright, the footer byline, the "Beljot Online" eyebrow and the "new to Beljot" prompt.
- Do not touch code identifiers, i18n key names, or the `belot` / `bela` content ids — prose values only.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Rules page, default | `/rules`, no prior choice | Bitola tab active; its scoped blocks render, Croatian-scoped ones absent | N/A |
| Tab switch | Croatian tab clicked | Croatian-scoped blocks replace Bitola's; the four shared chapters do not change; scroll-spy still resolves | N/A |
| In-match overlay | `RulesDialog` opened in a `croatia` match | Croatian tab pre-selected from the match's variant | Unknown variant falls back to Bitola |
| Marker, pointer | Hover the marker on a scoped block | Tooltip states what the other variant does | N/A |
| Marker, touch | Tap the marker | Same tooltip opens; dismisses on a second tap or an outside tap | N/A |
| Locale swap | Language switched while a tab is active | Content re-renders in the new locale; the active tab is preserved | Unknown locale falls back to en |
| Parity drift | A block scoped in one locale only | `rulesContent.parity.test.ts` fails | Test failure, not silent divergence |

</frozen-after-approval>

## Code Map

**Content module — the whole story lives here**

- `client/src/features/rules/content/types.ts:44-52` `RuleBlock` union — add the optional variant scope and the optional other-variant note here. `:54-60` `RuleSection` — `title`/`lede` are single strings; keep them variant-neutral and push specifics into blocks rather than widening these. `:63-87` `RulesUi` — needs a marker label; `facts` is pinned at exactly 4 by the parity test.
- `client/src/features/rules/content/{en,mk,hr,sr}.ts` (~300 lines each). Divergent points, from the locale audit:
  - `basics` `steps` items 2 and 3 — the 3+2 deal and the candidate flip. en/mk `:104-105`, `:108-109`; hr/sr `:103-104`, `:107-108`. **Scope the whole `steps` block per variant** — item counts differ and `blockShape` is `steps:${length}`, so two separately scoped blocks are the only parity-safe shape.
  - `basics` `title` (en `:89`, hr `:88`) and `lede` (en `:90`, hr/sr `:89`) — neutral enough for both. Leave.
  - `melds` `lede` (en/mk `:171`, hr/sr `:170`) and `blocks[0]` `p` (en/mk `:175`, hr/sr `:174`) — declaration timing. Genericize the lede; scope the timing sentence.
  - `melds` `blocks[2]` `rule` (en/mk `:179-181`, hr/sr `:178-180`) — the tie-break chain, which already matches the engine. **No locale mentions one-card-one-group.** Add it as a scoped block: Bitola dedup vs Croatian overlap.
  - `ui.facts[2]` caption (en/mk `:278`, hr/sr `:277`) — Bitola-specific, and mk/hr/sr say "3, then 2, then 3", matching **neither** variant. Pre-existing bug; make the caption variant-neutral.
  - `scoring` `blocks[2]` `note` (en/mk `:215`, hr/sr `:214`) — "the side with more total points takes the match", silent on equal totals. The D137 sentence lands here. Shared.
  - Zero hits for the round-2 reveal, the all-pass outcome, or **Capot** in any locale — additions, not corrections.
  - Game name, GAME sense (header-comment brand mentions stay): en `:101,223,272,274,297`; mk `:101,223,272,274,297` (already „Бељот" — no change); hr/sr `:100,222,271,273,296`. **hr/sr `:296` is genitive** and `:271` accusative — inflect, do not substitute. **Trap:** mk `:20,23,62,175,181,204` are „Бељот"/„Ромбељот" as the *bonus* or the *meld*; a blind find-and-replace corrupts meld copy.
  - `declarations.bela` — en/mk `:61-66`, hr/sr `:60-65`. Name stays; summary and detail change to the trump-suit rule.
- `client/src/features/rules/content/shared.ts:44-53` `DECLARATIONS_BASE` — read-only. `bela` keeps id, 1001 pts, tier 3, kind `run`.
- `client/src/features/rules/content/rulesContent.ts:36-52` — the assembler. Variant filtering belongs at render time, not here: `RULES_CONTENT` must stay one object per locale so `getRulesContent("sr-Latn")` identity holds (parity test `:106-110`).

**Render layer**

- `client/src/features/rules/components/RuleBlock.tsx:146-259` — the light-theme `switch` on `block.kind`. `rule` (`:155`), `steps` (`:187`) and `tiers` (`:184`) have headers to hang a marker on; `p` (`:148`) and `note` (`:181`) are bare and need it inline at the end of the text.
- `client/src/features/match/components/RulesDialog.tsx:302-439` `DarkBlock` — the parallel dark-theme switch. A marker added to only one of the two switches silently vanishes in the overlay.
- `client/src/features/rules/RulesPage.tsx:11-62` — `content`, `sections`, `firstId`, and the scroll-spy `useEffect` keyed on `[sections, firstId]`. Filtering `sections` per variant changes that identity every tab flip, so the spy re-runs and `activeId` can point at a section the new tab no longer renders.
- `client/src/features/rules/RulesContext.ts:8-15` `RulesProvider` / `useRules` — the active variant should ride this existing context, not a new prop chain through `Chapter` → `RuleBlock`.
- `client/src/features/rules/components/Chapter.tsx:325-327` renders `section.blocks`; `ChapterIndex.tsx:33` maps `sections`; `RulesHero.tsx:158` maps `ui.facts`.
- `client/src/features/match/MatchPage.tsx:2270` `<RulesDialog open onOpenChange>` — takes no variant. `matchState.variant` is in scope (used at `:1593`).

**Primitives, i18n, tests**

- `client/src/shared/components/ui/tabs.tsx` and `tooltip.tsx` exist with **zero importers** — this story is their first use. base-ui `^1.3.0`. `TooltipRoot` accepts `open` + `onOpenChange` (`node_modules/@base-ui/react/tooltip/root/TooltipRoot.d.ts:22-31`), so a controlled open state gives tap-to-reveal; the uncontrolled trigger is hover/focus only. `TooltipProvider` can wrap locally.
- `client/src/shared/lib/roomLabels.ts:31-35` `variantLabel` — tab labels. `Variant` union in `shared/types/matchTypes.ts:159`.
- `client/src/shared/i18n/{en,hr,sr}.json` — GAME-sense mentions are only `landing.subtitle:1199` and `landing.f2body:1220`; mk already reads „Бељот" in both. Every other hit (`:3`, `:18`, `:1074`, `:1196`, `:1261`, `:1265`) is the brand and stays. `lobby.createRoomModal.variantHint:657` in all four restates the two option labels above it — rewrite to state the actual difference. Gates: `i18n.test.ts:124`, `i18n.parity.test.ts`.
- `client/src/features/rules/content/rulesContent.parity.test.ts:13-18` `blockShape` — must fold the variant scope and marker presence into the shape string, or a block scoped in one locale only passes silently.
- `client/src/features/rules/RulesPage.test.tsx:36,86,99,105` and `client/src/features/match/components/RulesDialog.test.tsx:32,74` assert the old game name across en/mk/hr/sr — all six break on the rename and must be updated, not worked around.
- Testids to preserve: `rules-page`, `rules-chapter-index`, `toc-<id>`, `ladder-trump`, `ladder-plain`, `melds-grid`, `meld-<id>`, `rules-tier-<tier>`, `rules-play-cta`, `rules-dialog`, `rules-dialog-close`, `rules-toc-<id>`, `rules-meld-<id>`.

**Engine ground truth (read-only — cite when writing copy)**

- `server/internal/game/types.go:183-203` `RulesFor` — the two presets, all seven fields.
- `bidding.go:139-163` round 1 adopts the candidate's suit and ignores `action.Suit`; round 2 requires a named suit and **locks out the candidate's own suit**. `:165-181` the picker takes the candidate card into hand (candidate variants only). `:42-55` the Croatian dealer cannot pass in round 2. `:232-278` Bitola's round-2 pass-out reshuffles all 32 cards and rotates the dealer. First bidder and trick-1 leader are both `(DealerSeat+1)%4` — the seat to the dealer's right — so the existing "starting to the dealer's right" tiebreak wording is correct for both variants.
- `declarations.go:36-105` detection; `:108-140` `dedupOneCardOneGroup` keeps the higher `Value`, and equal value keeps the four-of-a-kind; `:240-275` `declarationBeats` — value → four-of-a-kind-beats-sequence → natural-order top card → trump → nearest seat to the trick leader. Only the one-card-one-group fact is missing from the page.
- `scoring.go:31-45` Capot **replaces** the +10 last trick; `:88-100` the Capot team takes every point in the hand including **both** teams' declarations, and a team taking no trick banks nothing. `:75-86` failed hand is `contractingTotal <= opposingTotal`. `:262-280` `determineMatchWinner` — both teams over target and **exactly tied** → the taker's team wins (D137).
- `scoring.go:228-256` `checkInstantWin` — all eight cards of the **trump** suit, live from `bidding.go:190` and `scoring.go:219`. `deferred-work.md:66` and `epic-12-context.md` both claim this is unimplemented; both are stale.

## Tasks & Acceptance

**Execution:**

- [x] `client/src/features/rules/content/types.ts` -- add an optional variant scope and an optional other-variant note to `RuleBlock`, plus a marker label on `RulesUi` -- widening the block union keeps `RuleSection.title`/`lede` single strings, so no section-level fork is introduced.
- [x] `client/src/features/rules/content/rulesContent.parity.test.ts` -- fold the variant scope and marker presence into `blockShape`, and assert both variants resolve a non-empty block list for every section -- without this a block scoped in one locale only passes the gate silently, which is the drift this test exists to stop.
- [x] `client/src/features/rules/content/en.ts` -- author the reconciled English spine: variant-scoped `basics` deal/bidding steps (both rounds, the Croatian round-2 self-reveal, both all-pass outcomes), variant-scoped `melds` declaration timing and the one-card-one-group / overlap rule, the D137 tied-tiebreaker sentence, a Capot statement, a variant-neutral `facts[2]` caption, other-variant notes on every scoped block, the game renamed to "Belote", and the `bela` meld's rule corrected to all eight trumps -- English is the reference locale the parity test measures the other three against, so it lands first and alone.
- [x] `client/src/features/rules/content/{mk,hr,sr}.ts` -- mirror the English structure exactly and translate idiomatically -- mk stays all-Cyrillic and keeps „Бељот" while its bonus and meld strings at `:20,23,175,181,204` survive untouched; hr/sr take "Bela" with correct case (accusative `:271`, genitive `:296`) and never mix ijekavica with ekavica.
- [x] `client/src/features/rules/RulesContext.ts` + `RulesPage.tsx` -- carry the active variant on the existing rules context, render the tab bar between hero and chapters with labels from `variantLabel`, and reset the scroll-spy's `activeId` when the tab flips -- `sections` identity changes on every flip, so the spy can otherwise leave `activeId` on a section the new tab does not render.
- [x] `client/src/features/rules/components/RuleBlock.tsx` -- filter by the active variant and render the difference marker per block kind, using a controlled `Tooltip` so it opens on tap as well as hover -- `p` and `note` have no header, so the marker sits inline at the end of their text; the uncontrolled trigger would be dead on a phone.
- [x] `client/src/features/match/components/RulesDialog.tsx` + `MatchPage.tsx` -- accept a variant prop, default the tab to `matchState.variant`, and mirror the filter and marker into the `DarkBlock` switch -- defaulting the tab is what closes the "rules contradict the table in front of you" finding.
- [x] `client/src/shared/i18n/{en,mk,hr,sr}.json` -- rewrite `lobby.createRoomModal.variantHint` to state the actual difference, and change the GAME-sense name in `landing.subtitle` and `landing.f2body` for en/hr/sr only -- every other mention of the name in these files is the brand.
- [x] `client/src/features/rules/RulesPage.test.tsx` + `client/src/features/match/components/RulesDialog.test.tsx` -- update the six game-name assertions and add coverage for the default tab, a tab switch swapping scoped blocks while a shared chapter is unchanged, the overlay pre-selecting a Croatian match's tab, and the marker opening on both hover and tap -- none of this has coverage today.
- [x] `_bmad-output/implementation-artifacts/deferred-work.md` -- correct the `:66` entry that claims the all-eight instant win has no engine support, and file the unimplemented any-suit variant plus moving the meld out of `DECLARATIONS_BASE` as new entries -- a false entry left standing is how this divergence survived two stories.

**Acceptance Criteria:**

- Given the Croatian tab, when `basics` and `melds` render, then they describe all eight cards dealt before bidding, a freely named trump suit with no candidate in both rounds, the round-2 self-reveal, the dealer's forced pick, a dedicated declaration phase before trick 1, and a card counting in more than one declaration — and no sentence mentions a flipped candidate or the 3+2 deal.
- Given the Bitola tab, when the same two chapters render, then they describe the 3+2 deal with a face-up candidate whose suit is adopted in round 1 and locked out in round 2, the picker taking that card into hand, a round-2 pass-out reshuffling and rotating the dealer, declarations announced during trick 1 and revealed at trick 2, and one card counting toward only one declaration.
- Given either tab, when `goal`, `cards`, `play` and `honour` render, then their text is identical between tabs.
- Given the `scoring` chapter in either tab, when it renders, then it states that a Capot replaces the last-trick bonus and takes every point in the hand including both teams' declarations, and that when both teams cross the target in the same hand with equal totals the taker's team wins.
- Given the `melds` chapter in either tab, when the all-eight entry renders, then it names the hand Belote / Бељот / Bela and describes all eight cards of the **trump** suit winning the match on the spot.
- Given each locale, when the rules page renders, then the game is named Belote (en) / Бељот (mk) / Bela (hr, sr) with correct inflection, and no locale still calls it Beljot in a game sense.
- Given `cd client && npx vitest run`, when it completes, then `rulesContent.parity.test.ts`, `i18n.parity.test.ts` and `i18n.test.ts` all pass with no locale exempted.
- Given `git diff --stat HEAD -- server/ client/src/features/match/lib/variantRules.ts`, when this story is complete, then it is empty.

## Spec Change Log

## Design Notes

**Why block-level scoping, not per-variant chapters.** Most of the rules on this page are identical, and the epic's audit insists identical rules must not be branched. Duplicating six chapters × four locales would quadruple a file that already needs a parity gate to stay honest, and every future shared-rule fix would need applying twice. Scoping at the block level makes the diff between tabs *be* the divergence list — switch tabs and four chapters visibly do not move.

**Why the marker text is authored, not derived.** A generic "this differs in the other variant" badge tells a player nothing. The useful sentence is what the *other* variant does — prose, which belongs in the locale files beside the block it annotates and is translated with it.

**The three-way name overlap is deliberate.** The game, the +20 K+Q bonus and the all-eight-trumps hand all carry one word per locale, because the game is named after the hand. The fix for ambiguity is sentence context, not new coinages: the melds grid entry is already tier 3 with its own "wins the match on the spot" treatment, so it reads as a hand rather than a synonym for the game.

## Verification

**Commands:**

- `cd client && npx vitest run` -- expected: all pass, including `rulesContent.parity.test.ts`, `RulesPage.test.tsx`, `RulesDialog.test.tsx`, `i18n.parity.test.ts`, `i18n.test.ts`.
- `cd client && npx tsc -p tsconfig.build.json --noEmit` -- expected: clean. Not run in CI — run it manually.
- `cd client && npx eslint . && npx prettier --check .` -- expected: clean.
- `cd server && go test ./...` -- expected: all pass, unchanged.
- `git diff --stat HEAD -- server/` -- expected: empty.
- `grep -n "Beljot" client/src/features/rules/content/*.ts` -- expected: only the four header comments.

**Manual checks:**

- Run `make dev`, open `/rules` in each locale and switch tabs: the four shared chapters must not change, `basics` and `melds` must, and every marker's tooltip must name the other variant's behaviour. A Playwright MCP browser is available and is the better tool here — screenshot both tabs' `basics` chapter in at least one non-English locale.
- With a Croatian match running, open the in-match overlay and confirm the Croatian tab is pre-selected; repeat in a Bitola match.
- In a touch-emulation viewport, tap a difference marker and confirm the tooltip opens and dismisses — hover-only would pass every unit test and still be dead on a phone.

## Suggested Review Order

**The content model — start here**

- One optional scope on the block union; sections never fork, so all six chapters survive both tabs.
  [`types.ts:56`](../../client/src/features/rules/content/types.ts#L56)

- Authored prose, not a derived badge — the marker says what the OTHER variant does.
  [`types.ts:52`](../../client/src/features/rules/content/types.ts#L52)

- The single narrowing point: `Variant` is now DERIVED from the runtime array.
  [`matchTypes.ts:17`](../../client/src/shared/types/matchTypes.ts#L17)

**The divergences, as content**

- Bitola's deal: 3+2, candidate flipped, round-2 lockout, and the pass-out reshuffle.
  [`en.ts:94`](../../client/src/features/rules/content/en.ts#L94)

- Croatian's deal: all eight up front, free suit both rounds, self-only reveal, forced pick.
  [`en.ts:126`](../../client/src/features/rules/content/en.ts#L126)

- Declaration timing — trick-1 announce vs a phase of its own.
  [`en.ts:222`](../../client/src/features/rules/content/en.ts#L222)

- The rule no locale documented: one-card-one-group vs the Croatian overlap.
  [`en.ts:242`](../../client/src/features/rules/content/en.ts#L242)

**Shared-rule corrections (both variants)**

- Capot replaces the +10 and takes both teams' declarations; a trickless team banks nothing.
  [`en.ts:288`](../../client/src/features/rules/content/en.ts#L288)

- D137: both teams over target and exactly tied goes to the taker.
  [`en.ts:293`](../../client/src/features/rules/content/en.ts#L293)

- The all-eight hand: trump suit, not any suit, and it names a winner rather than paying 1001.
  [`en.ts:61`](../../client/src/features/rules/content/en.ts#L61)

- The comment that propagated the falsehood through two stories.
  [`shared.ts:50`](../../client/src/features/rules/content/shared.ts#L50)

**Rendering — the marker had to reach both themes**

- Block filter plus one marker built once per block, then placed per kind.
  [`RuleBlock.tsx:170`](../../client/src/features/rules/components/RuleBlock.tsx#L170)

- Press-to-pin, so the note is reachable on a phone where hover does not exist.
  [`DiffMarker.tsx:72`](../../client/src/features/rules/components/DiffMarker.tsx#L72)

- Highest-risk stop: the overlay tier fix. Without it the note paints behind the panel.
  [`DiffMarker.tsx:99`](../../client/src/features/rules/components/DiffMarker.tsx#L99)

- The passthrough lands on the portalled positioner — the element whose tier actually competes.
  [`tooltip.tsx:46`](../../client/src/shared/components/ui/tooltip.tsx#L46)

- New tier above `Z.UTIL`, kept in the central table rather than a scattered class.
  [`zLayers.ts:62`](../../client/src/shared/lib/zLayers.ts#L62)

**The two tab bars**

- Page tabs on the shared primitive; the value is normalized, never cast.
  [`RulesPage.tsx:95`](../../client/src/features/rules/RulesPage.tsx#L95)

- Overlay keeps a felt-themed tablist but matches the primitive's keyboard contract.
  [`RulesDialog.tsx:269`](../../client/src/features/match/components/RulesDialog.tsx#L269)

- Reconstructed file — re-read this one first. The reset effect is load-bearing, not dead code.
  [`RulesDialog.tsx:70`](../../client/src/features/match/components/RulesDialog.tsx#L70)

- The wiring a deleted prop would have silently defaulted to Bitola.
  [`MatchPage.tsx:2270`](../../client/src/features/match/MatchPage.tsx#L2270)

**Gates and peripherals**

- Four new parity gates; each was proved to fail before being kept.
  [`rulesContent.parity.test.ts:131`](../../client/src/features/rules/content/rulesContent.parity.test.ts#L131)

- A section that scopes anything must cover both variants — no one-sided content holes.
  [`rulesContent.parity.test.ts:161`](../../client/src/features/rules/content/rulesContent.parity.test.ts#L161)

- Asserts a stacking tier, because jsdom cannot see paint order.
  [`DiffMarker.test.tsx:1`](../../client/src/features/rules/components/DiffMarker.test.tsx#L1)

- The hint now states the difference instead of restating the two option labels.
  [`en.json:657`](../../client/src/shared/i18n/en.json#L657)
