---
title: 'Croatian single-round bidding (pod mus)'
type: 'bugfix'
created: '2026-08-22'
status: 'done'
review_loop_iteration: 0
context: []
baseline_commit: '26caf0b84e080f30507ecfd371792e9c68c3c261'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** The shipped Croatian bidding is a two-round flow (bid on 6 cards; four passes reveal each seat's 2 face-down cards to their owner and open round 2 on 8 cards, where the dealer is forced). The real rule is a **single round**: each player, seeing only their 6 cards, names a suit or passes ("dalje"), starting to the dealer's right and going counter-clockwise; the dealer speaks last and, if the other three passed, **must** call ("pod mus"); only **after** trump is set do the 2 face-down cards turn up, giving everyone 8 cards. Rule authority: the owner (2026-08-22) — this supersedes epic-12-context.md and project-context.md, which still describe the two-round rule.

**Approach:** Make the forced-dealer predicate fire in round 1 (drop its `BiddingRound == 2` conjunct), which makes the round-1→2 transition unreachable under `AllPassDealerMustPick` — Croatian bidding then never leaves round 1. Remove the now-purposeless mid-bidding reveal: the `RevealFaceDownOnRound2` config field, the `FaceDownRevealed` state flag, and the entire `event:face_down_revealed` wire surface (server emit, reconnect replay, client schema/dispatch/store/merge). The post-pick `mergeFaceDownCards` + per-recipient snapshot already deliver the 8-card hand. Rewrite the Croatian rules-reference steps in all four locales.

## Boundaries & Constraints

**Always:**

- Bitola is byte-identical on the wire and every pre-existing Bitola test passes unchanged; `event_match_state.json` golden does not move.
- D-VAR-1 holds: no engine code compares the variant name; the change is expressed through existing `VariantRules` fields (`AllPassOutcome`, `HasTrumpCandidate`) — no new field.
- Face-down cards stay invisible to everyone (including their owner) until trump is resolved; the structural privacy test keeps covering the whole single round including the dealer-on-clock state.
- The forced dealer (human timeout auto-pick AND bot) chooses trump from the 6 visible cards only — never from `FaceDownCards`.
- Removing the WS event completes the full drift gate in the same change: `events.go`, contract case + golden deletion, `wsEvents.ts`, `wsEvents.schemas.ts` (+ witness + registry), contract test row, dispatch, store.
- Rules-reference edits keep `rulesContent.parity.test.ts` green (equal step counts per variant, both-direction `otherVariantNote`s) across en/mk/hr/sr; mk all-Cyrillic, hr/sr idiomatic, "contract" banned, the taker terminology.
- Correct the two stale docs: `epic-12-context.md` (divergences 3–4, "two bidding rounds in both variants") and `project-context.md` (Croatian bidding paragraph).

**Ask First:**

- Any change to Bitola behavior or its wire bytes.
- Renaming/removing any other `VariantRules` field beyond `RevealFaceDownOnRound2`.

**Never:**

- Keep `event:face_down_revealed` as dormant dead code.
- Add bidding UI variant branches client-side — the client keeps reading `mustPickTrump` and `trumpCandidate === null` only.
- Touch declaration phase, scoring, tie rule, or Quick Play scope.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Croatian passes 1–3 | `pass_trump` from dealer+1, +2, +3 | Accepted; `BiddingRound` stays 1; after the 3rd, active seat = dealer and `mustPickTrump` = true on the wire | N/A |
| Dealer tries to pass | 3 passes, then dealer `pass_trump` | Rejected, state unchanged, round 1 persists | `ErrMustPickTrump` |
| Dealer forced pick | 3 passes, dealer `pick_trump{suit}` | Trump set, face-down cards merge (8 each), phase → `declaring` | N/A |
| Any earlier pick | seat names a suit before 3 passes | Same resolution path; picker takes no card | N/A |
| Dealer timeout | forced dealer's timer expires | Auto-pick via `AutoPickTrumpSuit` over `Hand` only (6 cards); `event:auto_action` = `pick_trump` | N/A |
| Non-dealer timeout | any other bidder's timer expires | Auto-pass, as today | N/A |
| Reconnect mid-bidding | Croatian player reconnects at any pass count | Snapshot: own 6-card hand, `faceDownCount` 2; **no** reveal replay | N/A |
| Snapshot privacy | marshal any seat's projection during bidding | No face-down card ID appears anywhere, incl. owner's own payload | N/A |
| Bitola both rounds | full Bitola pass-out ×2 | Unchanged: round 2, candidate lock, reshuffle+rotate | N/A |

</frozen-after-approval>

## Code Map

**Server engine — `server/internal/game/`**

- `types.go:156` — delete `RevealFaceDownOnRound2` from `VariantRules` + both presets in `RulesFor` (`:185-203`). Croatian stays `AllPassDealerMustPick`, `HasTrumpCandidate:false`.
- `bidding.go:60-66` — `MustPickTrump`: drop the `BiddingRound == 2` conjunct (keep `AllPassOutcome == AllPassDealerMustPick && BiddingPassCount >= 3`); update doc `:41-59`. This alone forces the dealer in round 1 and makes pass #4 — hence the round-1→2 transition at `:82-96` — unreachable for Croatian.
- `bidding.go:93-94` — delete the `RevealFaceDownOnRound2` arming; `:228` and `reshuffleAndRedeal:265` drop `FaceDownRevealed` writes (keep the face-down pooling `:249-250`).
- `state.go:185` — delete `FaceDownRevealed bool` (json:"-"); `scoring.go:172` reset goes too. Keep `FaceDownCards`/`FaceDownCount`/`syncFaceDownCounts`/`mergeFaceDownCards` — merge on pick (`bidding.go:187`) is now the one reveal moment.
- `auto_play.go:92-94` — `AutoPickTrumpSuit` stops folding `FaceDownCards` into the scan (dealer knows 6 cards). Only caller is the forced-dealer timer branch, so Bitola untouched.
- `rules_contract_test.go:36-57` + `testdata/contract/variant_rules.json` — if the deleted field is a fact, remove and regenerate; mirror `client/src/features/match/lib/variantRules.ts`.

**Match layer — `server/internal/match/`**

- `live_match.go:927-928` reveal emission; helpers `sendFaceDownReveals:1086`, `sendFaceDownRevealToSeat:1099`, `buildFaceDownRevealMsg:1108` — delete. Forced-dealer timer branch `:1579-1619` works unchanged; refresh round-2 wording in its rationale comments.
- `reconnect.go:546-555` — delete the reveal replay; keep `RefreshDerivedFlags` at `:471`.
- `bot_driver.go:248-250` — delete the `FaceDownRevealed` gate; `bot/view.go:25` `FaceDownCards` field and `bot/bot.go:164-171` bidHand inclusion go with it (bots bid on `Hand` (+candidate for Bitola) only).

**Wire contract**

- `ws/events.go:125-159` — delete `EventFaceDownRevealed` + `FaceDownRevealedPayload` + rationale block. Keep `ErrorMustPickTrump`, `AutoActionPickTrump`.
- `ws/events_contract_test.go:336-342` — delete case; delete golden `ws/testdata/events/face_down_revealed.json`. `event_match_state.json` sample (`BiddingRound: 2`, Bitola-valid) must NOT change.
- Client mirrors: `wsEvents.ts:95-177` const+payload+docs; `wsEvents.schemas.ts:357-371` (`CardIdSchema` has no other consumer) + witness `:539-543` + registry `:571`; `wsEvents.contract.test.ts:24,47,95`.

**Client**

- `useWsDispatch.ts:80,440-474` handler; `matchStore.ts:79,121,167,202,217` slice/setter/retention; `features/match/lib/faceDownCards.ts` + test — delete all. `MatchPage.tsx:90,250,1486-1491` → `myHand = myPlayer?.hand ?? []`; `:1760` collapses to `faceDownCount={myFaceDownCount}`.
- `TrumpPrompt.tsx` — only two `biddingRound === 2` sites (`:180,:217`), both already OR-ed with `trumpCandidate === null`: leave. Gate the footer round label `:346-349` ("Round {n} / 2") on `trumpCandidate !== null` — no round counter for the single-round variant. `MustPickNote`/`canPass` machinery already correct.
- `matchTypes.ts:199-202` — reword `mustPickTrump` doc (single round, dealer 4th). `MatchPage.test.tsx:1886` fixture `biddingRound: 2` → 1.
- Rules content `features/rules/content/{en,mk,hr,sr}.ts` — Croatian `steps` items in `basics` (en anchors): keep "Deal all eight up front" `:115-121`; rewrite `:129-135` as the single round (dealer's right first, on your 6 cards, name a suit or "dalje"); replace `:143-149` (round-2 reveal) with the dealer-forced step — dealer speaks last and must call, "pod mus", no reshuffle; rewrite `:157-163` as the post-trump reveal (your last two turn up, everyone plays with 8). Keep 4 items (parity `:249`). Rewrite the four Bitola items' `otherVariantNote`s (`:110,:124,:138,:152`) to match; Croatian items' Bitola-notes stay. `RulesPage.test.tsx:129-134,204-255` literal-string assertions updated.
- i18n JSONs: no key changes required (`mustPick`, `waitingForcedPick`, `titleFreePick`, `autoPickedTrump` all still fit); `roundLabel` remains Bitola-only.

**Tests to rewrite (server)**

- `testfixtures/fixtures.go:762-789` — `NewGameCroatianMidBidding(passCount)`: clamp 0–3; 3 = dealer on clock; delete round-2 states.
- `bidding_test.go` Croatian suite `:659-1184` — re-target: passed-out-reveals test becomes "3rd pass puts dealer on clock, round stays 1, cards stay hidden"; dealer-cannot-pass moves to round 1; bad-suit round-2 rows → passCount 3; `TestFaceDownCardsNeverSerialized` rows cover dealer-on-clock; `TestCroatianFullBiddingFromNewGame` = 3 passes → forced pick → `PhaseDeclaring`, `BiddingRound` 1 throughout; `TestMustPickTrumpWireFlag` round-1 semantics, still never true for Bitola.
- `match/face_down_reveal_test.go` — delete reveal tests; relocate `TestTrumpSelected_EmittedWithEmptyCardIDWhenNoCandidate:261`. `live_match_internal_test.go:390-506`, `reconnect_test.go:650-689`, `auto_action_test.go`, `bot_driver_test.go:672-698`, `bot/bot_test.go:73-410`, `projection_test.go:64-67`, `scoring_internal_test.go:208,317` — update fixtures from round-2 to pass-count-3 states.

## Tasks & Acceptance

**Execution:**

- [x] `server/internal/game/{types.go,bidding.go,state.go,scoring.go,auto_play.go}` -- single-round engine change per Code Map -- the whole rule falls out of `MustPickTrump` losing its round-2 conjunct.
- [x] `server/internal/game/testfixtures/fixtures.go` + `bidding_test.go` + other game tests -- re-target Croatian fixtures/tests to the single round; cover every I/O-matrix row.
- [x] `server/internal/game/rules_contract_test.go` (+ golden) + `client/src/features/match/lib/variantRules.ts` -- drop the deleted config fact from both sides if present.
- [x] `server/internal/match/{live_match.go,reconnect.go,bot_driver.go}` + `server/internal/bot/{bot.go,view.go}` -- remove reveal emission/replay and face-down bot visibility; update match tests.
- [x] `server/internal/ws/events.go` + contract test + golden deletion -- retire `event:face_down_revealed`.
- [x] `client/src/shared/{types,hooks,stores}` + `features/match/lib/faceDownCards.ts` + `MatchPage.tsx` -- remove the client reveal machinery; hand re-renders from the post-pick snapshot.
- [x] `client/src/features/match/components/TrumpPrompt.tsx` (+ test) -- hide the round counter for candidate-less bidding; move forced-pick fixtures to round 1.
- [x] `client/src/features/rules/content/{en,mk,hr,sr}.ts` + `RulesPage.test.tsx` -- rewrite the four Croatian steps and the four Bitola mirror-notes to the single-round rule.
- [x] `_bmad-output/implementation-artifacts/epic-12-context.md` + `_bmad-output/project-context.md` -- correct the recorded rule (divergences 3–4; the "two bidding rounds in both variants" claim; the Croatian bidding paragraph).

**Acceptance Criteria:**

- Given a Croatian game, when bidding runs to any conclusion, then `BiddingRound` is 1 in every reachable state and `reshuffleAndRedeal` is never entered.
- Given the server tree, when grepped for `FaceDownRevealed`, `RevealFaceDownOnRound2`, or `face_down_revealed`, then there are no hits (code, tests, goldens).
- Given a Bitola match, when the full suite runs, then every pre-existing Bitola test passes unchanged and `event_match_state.json` is byte-identical to HEAD.
- Given the rules page under the Croatian tab, when read in each locale, then it describes: bid on 6, dealer's right first, single round, dealer forced ("pod mus"), 2 cards revealed after trump, 8 cards in play — and `rulesContent.parity.test.ts` passes.

## Spec Change Log

## Verification

**Commands:**

- `cd server && go test ./...` -- expected: all pass.
- `cd server && golangci-lint run ./... && gofmt -l .` -- expected: clean, no output.
- `cd client && npx vitest run` -- expected: all pass incl. contract, parity, rules-page suites.
- `cd client && npx tsc -p tsconfig.build.json --noEmit && npx eslint . && npx prettier --check .` -- expected: clean.
- `cd server && git diff --stat HEAD -- internal/ws/testdata/events/event_match_state.json` -- expected: empty.
- `grep -rn "FaceDownRevealed\|RevealFaceDownOnRound2\|face_down_revealed" server/ client/src/` -- expected: no hits.

## Suggested Review Order

**The rule itself — one predicate change**

- Entry point: forced pick now belongs to the free-suit stage — round 1 when candidate-less.
  [`bidding.go:67`](../../server/internal/game/bidding.go#L67)

- The dealer's own pass is what gets refused; pass #4 can never land for Croatian.
  [`bidding.go:80`](../../server/internal/game/bidding.go#L80)

- The round-1→2 transition survives untouched — reachable only by Bitola now.
  [`bidding.go:94`](../../server/internal/game/bidding.go#L94)

- The post-pick merge is the one reveal moment left; snapshot delivers the 8-card hand.
  [`bidding.go:225`](../../server/internal/game/bidding.go#L225)

- `RevealFaceDownOnRound2` deleted; six fields remain, presets still fully populated.
  [`types.go:181`](../../server/internal/game/types.go#L181)

- Doc now says the second round never opens only when candidate-less.
  [`types.go:109`](../../server/internal/game/types.go#L109)

**Blind pick fairness — the dealer knows six cards**

- Timeout auto-pick scans `Hand` only; the hidden pair cannot sway it.
  [`auto_play.go:82`](../../server/internal/game/auto_play.go#L82)

- Bots bid on the same six cards humans see; face-down fold removed.
  [`bot.go:160`](../../server/internal/bot/bot.go#L160)

**Reveal surface retired — wire and client**

- `event:face_down_revealed` gone from the contract; `trump_selected` still fires candidate-less.
  [`events.go:39`](../../server/internal/ws/events.go#L39)

- Candidate-less take emits with empty `cardId`; the per-seat reveal emission is deleted.
  [`live_match.go:897`](../../server/internal/match/live_match.go#L897)

- Client hand renders straight from the projected snapshot — merge machinery deleted.
  [`MatchPage.tsx:1479`](../../client/src/features/match/MatchPage.tsx#L1479)

- Round counter is candidate-gated: no "Round n / 2" in a single-round variant.
  [`TrumpPrompt.tsx:350`](../../client/src/features/match/components/TrumpPrompt.tsx#L350)

**Rules reference — four locales**

- The single-round step: dealer's right first, "dalje" to pass, six cards visible.
  [`en.ts:133`](../../client/src/features/rules/content/en.ts#L133)

- The forced call named and explained — "pod mus", no reshuffle, every deal plays.
  [`en.ts:147`](../../client/src/features/rules/content/en.ts#L147)

- hr wording qualifies the trump call to avoid the reserved declarations term.
  [`hr.ts:145`](../../client/src/features/rules/content/hr.ts#L145)

**Deal shape (unchanged, context)**

- Croatian still deals 6 + 2 face-down before bidding; only the reveal timing moved.
  [`state.go:434`](../../server/internal/game/state.go#L434)

**Tests — the new pins**

- Third pass puts the dealer on the clock inside round 1.
  [`bidding_test.go:766`](../../server/internal/game/bidding_test.go#L766)

- Dealer's pass rejected with `ErrMustPickTrump`; round 1 persists.
  [`bidding_test.go:803`](../../server/internal/game/bidding_test.go#L803)

- Full game walk: 3 passes → forced pick → declaring, `BiddingRound` 1 throughout.
  [`bidding_test.go:1086`](../../server/internal/game/bidding_test.go#L1086)

- The combined candidate+must-pick config pinned: force belongs to round 2 there.
  [`bidding_test.go:846`](../../server/internal/game/bidding_test.go#L846)

- Privacy holds through the dealer-on-clock state; no hidden card in any payload.
  [`bidding_test.go:926`](../../server/internal/game/bidding_test.go#L926)

- Post-pick projection: each seat's own merged 8, nobody else's pair.
  [`projection_test.go:247`](../../server/internal/game/projection_test.go#L247)

- Fixture clamps to 0–3 passes; round-2 states no longer constructible.
  [`fixtures.go:762`](../../server/internal/game/testfixtures/fixtures.go#L762)

- Forced pick announces itself with an empty `cardId`.
  [`trump_selected_test.go:36`](../../server/internal/match/trump_selected_test.go#L36)

- Bitola's timer-driven round-1 passout still opens round 2 — match-layer proof restored.
  [`auto_action_test.go:262`](../../server/internal/match/auto_action_test.go#L262)

- Page-level: the viewer's own two backs render during bidding, none after trump.
  [`MatchPage.test.tsx:1932`](../../client/src/features/match/MatchPage.test.tsx#L1932)
