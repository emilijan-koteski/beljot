---
title: "Croatian declaration overlap"
type: "feature"
created: "2026-08-19"
status: "done"
review_loop_iteration: 0
context: ["{project-root}/_bmad-output/implementation-artifacts/epic-12-context.md"]
baseline_commit: "65e4966208e0883ca3f90549db9d40d05814ead3"
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** `detectDeclarations` always applies one-card-one-group dedup, so the Croatian rule — a card may count in more than one declaration — cannot be expressed. Story 12.1 shipped the `DeclarationOverlap` config field; nothing reads it, and a `TODO(declaration-overlap)` marks the spot. The client keeps its own copy of the detector and dedups unconditionally too, so a Croatian player's prompt would offer fewer melds than the server records.

**Approach:** Make the dedup pass config-selected on both sides — server reads `state.Rules.DeclarationOverlap`, the client resolves the same fact from the variant in one place — so a Croatian quarte and a four-of-a-kind sharing a Jack both survive and both count. While in that function, fix its equal-value tie so the survivor is the contender the clash comparison actually prefers.

## Boundaries & Constraints

**Always:**

- The overlap flag is an explicit parameter derived from `state.Rules.DeclarationOverlap` at both engine call sites — never a default, never a variant-name comparison inside `server/internal/game`.
- Dedup's equal-value tie keeps the **four-of-a-kind**, matching `declarationBeats` rule 2. This changes Bitola behaviour on one hand shape and is deliberate.
- The client detector stays a faithful mirror of the server's: same dedup algorithm, same tie rule, selected by config. The prompt must never list a group the server will discard, nor omit one it will keep.
- Exactly one place on the client maps a variant to rule facts; every consumer reads that resolver, with an unknown variant falling back to Bitola.
- Surviving groups stay listed separately with their own values; the displayed total is their plain sum.
- Declarations stay resolved during trick 1 in both variants under this story.

**Ask First:**

- If a hand can be constructed where two sequences, or two four-of-a-kinds, share a card — contradicting the maximal-run and rank-disjoint reasoning the tie fix rests on — stop before generalizing dedup to the full `declarationBeats` chain (which would need trump and seat, unavailable at detection time).
- If closing D67 turns out to need any change to the reveal's layout, centering, timing, or payload shape, stop: those are frozen by `spec-center-declaration-dialog.md` and `spec-declaration-reveal-cards-and-timing.md`.

**Never:**

- Don't restore seat anchoring, `compassOffset`, or `PANEL_POSITIONS` in the reveal, and don't touch its centering, 8s/1.5s reduced-motion timing, card size, fan geometry, max-height cap, or any `data-testid`. D67's "shows only the first / must anchor" premise is stale: the reveal already renders every meld and was deliberately centred.
- Don't widen `event:declarations_resolved` or any other payload, don't add an event, and don't serialize `Rules` or `DeclarationOverlap` to the client.
- Don't add i18n keys or change any locale copy — the tiebreaker line keeps its current `declarations.length > 1` gate.
- Don't merge surviving groups, change meld point values, or edit the `declarationBeats` chain itself.
- Don't move declaration timing (12.6), touch the tie rule (12.7), or make Croatian selectable (12.8).
- Don't sort `detectDeclarations` output to hide Go map-iteration order — make the tests order-agnostic instead.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Croatian J-overlap | quarte 9S-TS-JS-QS + four Jacks (share JS) | Both survive; team declaration total 250 | N/A |
| Bitola J-overlap | same hand | Unchanged: four Jacks only (200), one survivor | N/A |
| Bitola equal-value overlap | quinte TS-JS-QS-KS-AS (100) + four Tens (100), share TS | **New:** four Tens survives, not the sequence. Total still 100 | N/A |
| Croatian equal-value overlap | same hand | Both survive, total 200; the clash contender is the four-of-a-kind | N/A |
| Overlap-free hand | any hand with disjoint melds | Byte-identical output under either config | N/A |
| Prompt trigger | hand with any meld, either variant | Unchanged — dedup never empties a non-empty set, so the trigger predicate is invariant | N/A |
| Croatian prompt preview | player has melds, has not declared yet | Client lists both overlapping melds and sums them, matching what the server will record | N/A |
| Reveal, two melds one seat | resolved payload with a card in two entries | Both rows render, shared card appears in both, total is the sum | N/A |
| Bot, Croatian overlap hand | bot's turn to declare | Declares as today; memory records each shared card once | N/A |

</frozen-after-approval>

## Code Map

**Server rules engine — `server/internal/game/`**

- `declarations.go:30` `detectDeclarations(hand []Card)` — sequences at `:33-73` (maximal per-suit runs, so runs within a suit are disjoint), four-of-a-kind at `:75-92` (rank-disjoint; 4x7 and 4x8 silently dropped by the `ok` guard at `:82`). `:94-98` is the `TODO(declaration-overlap)` and the unconditional `dedupOneCardOneGroup` call.
- `declarations.go:106-146` `dedupOneCardOneGroup` — greedy over a `sort.SliceStable` by `Value` desc; the tie rule at `:105`/`:111-117` is "earlier index wins", and sequences are appended before four-of-a-kinds, which is exactly the inversion to fix. Returns the input slice unfiltered when `len <= 1` (`:107-109`).
- `declarations.go:149-151` `hasDeclarableCombinations` — `len(detectDeclarations(hand)) > 0`; provably invariant under a dedup skip.
- Call sites (the only two, both with `*GameState` in scope): `declarations.go:289` in `handleDeclare` (stamps `PlayerSeat` at `:296-299`, assigns the slice at `:300`), and `declarations.go:470` in `checkDeclarationPrompt`.
- `declarations.go:171-210` `resolveDeclarations` — keeps one best per team in `bestByTeam [2]*teamBest` (`:178`) but awards `teamDeclarationTotal` (`:263-273`), which sums **all** of the winning team's melds. Already handles several melds per player and per team. `declarationBeats` at `:214-249`; rule 2 (four-of-a-kind beats sequence at equal value) is `:221-223`.
- `declarations.go:441-457` `resolveDeclarationsForHand` — assigns `DeclarationPoints`, nils the losing team's melds. Fires from the `TrickNumber == 2 && !DeclarationsResolved` gate at `:423`.
- `types.go:163` `DeclarationOverlap bool` (croatia `true` at `:192`, bitola/fallback `false` at `:202`); `types.go:269-274` `Declaration`; `state.go:16` `PlayerState.Declarations []Declaration`; `state.go:103` `Rules VariantRules` (`json:"-"`).
- `bidding.go:255-320` `cloneGameState` — two-level clone of declarations at `:312-316`. `Declaration.Cards` are freshly allocated per meld (`declarations.go:57-58`, `:83-84`), so overlap introduces no aliasing.
- `scoring.go:56-59` — declaration points reach scoring only through `DeclarationPoints`, a separate lane from `HandPoints`; no card-point double counting is possible.

**Tests & fixtures**

- `declarations_test.go` is `package game_test` — reach detection through `ApplyAction` only, per project rules. `TestDedupBitola:865` subtests `:866` (20 vs 150) and `:888` (50 vs 200) must keep asserting one survivor; `:910`, `:930`, `:953` are overlap-free.
- `testfixtures/fixtures.go:258` `NewGameFirstTrick` is the declaration workhorse (hand layout documented `:250-256`; dedup tests overwrite `Players[0].Hand`). `:338` `NewGameWithDeclarations` injects melds. 12.1's Croatian factories `:651` and `:746` both stop at bidding — **there is no Croatian playing-phase fixture**. Every literal sets `Variant` and `Rules: game.RulesFor(...)` together (`:36-37`, `:264-265`, `:656-657`); `state_test.go:396-399` explains why the zero-value config is not Bitola.

**Wire (no shape change needed)**

- `match/live_match.go:1110-1138` `broadcastDeclarationsResolvedIfTransition` — flat double loop over all seats x all their melds, each entry carrying its own `playerSeat`, so N melds per seat already serialize. Fire-once latch at `:1111`.
- `ws/events_contract_test.go:102-113` sample, case at `:182`, golden `ws/testdata/events/declarations_resolved.json` (one entry today). `UPDATE_GOLDENS=1` regenerates; a missing golden hard-fails at `:322-328`. `event_match_state.json` must stay byte-identical.
- `client/src/shared/types/wsEvents.ts:179-187` + `wsEvents.schemas.ts:235-247` (`z.strictObject`, unbounded array) already accept the multi-meld payload; witnesses at `:386-390` and registry `:480-481`.

**Client**

- `features/match/lib/declarations.ts:52` mirror of the Go detector; `:114-116` stale `TODO(croatian-variant)` + unconditional `dedupBitola`; `:125-151` the dedup, same algorithm and same tie inversion as the server.
- `features/match/MatchPage.tsx:2009-2019` — prompt uses `myPlayer.declarations` when non-empty, else the local `detectDeclarations(myPlayer.hand)`; `matchState.variant` is typed at `shared/types/matchTypes.ts:107` and read today only at `MatchPage.tsx:1539`. The client cannot read `Rules` (`json:"-"`), so it must resolve overlap from the variant.
- `features/match/components/DeclarationReveal.tsx:198` maps every declaration; `:68` `declarations[0]` is an emptiness guard only; `:103` centring; `:87` tiebreaker gate; `:243-244` per-row `declaration-reveal-declarer` with `data-seat`. Tests already cover one seat with two overlapping melds (`DeclarationReveal.test.tsx:205-226`) and two teammates (`:228-250`); centring is locked at `:126-166`, timing at `:316-357`.
- `features/match/lib/declarations.test.ts:53-88` — five Bitola dedup tests whose assertions must not change.
- `bot/memory.go:59-67` `ObserveDeclarations` concatenates each meld's cards with no dedup, so overlap records a shared card twice, breaking the "exact cards" invariant `spec-bot-declaration-memory.md` documented. Consumers are all set/existence based today, so this is hardening, not a live bug. The maximal-run soundness argument at `bot/bot.go:695-703` still holds — overlap does not change run maximality.

## Tasks & Acceptance

**Execution:**

- [x] `server/internal/game/declarations.go` -- thread an explicit overlap parameter through `detectDeclarations` and `hasDeclarableCombinations`, sourced from `state.Rules.DeclarationOverlap` at both call sites (`:289`, `:470`); skip the dedup when it is true and delete the TODO; make dedup's equal-value tie keep the four-of-a-kind -- the story's rule change plus the clash-contender fix.
- [x] `server/internal/game/testfixtures/fixtures.go` -- add a Croatian first-trick factory deriving from `NewGameFirstTrick` with `Variant` **and** `Rules` set to the Croatian preset -- no Croatian playing-phase fixture exists and raw state literals are banned in tests.
- [x] `server/internal/game/declarations_test.go` -- table-driven cases through `ApplyAction` for the Croatian overlap hands, the Bitola equal-value tie, and an overlap-free hand producing identical results under both configs; assertions must be order-agnostic.
- [x] `server/internal/bot/memory.go` -- record each observed declaration card once per seat -- restores the exact-cards invariant that overlap would otherwise break.
- [x] `server/internal/ws/events_contract_test.go` + `internal/ws/testdata/events/declarations_resolved.json` -- extend the sample with a second entry for the same seat sharing a card, regenerate with `UPDATE_GOLDENS=1` -- contract-level proof the existing shape carries overlap; payload shape unchanged.
- [x] `client/src/features/match/lib/variantRules.ts` -- new: the client's single variant-to-rule-facts resolver mirroring the Go presets, unknown falling back to Bitola -- keeps exactly one variant comparison on the client for later Epic 12 stories to extend.
- [x] `client/src/features/match/lib/declarations.ts` -- take the overlap flag, gate `dedupBitola` on it, mirror the equal-value four-of-a-kind tie fix, drop the stale TODO -- the mirror must agree with the server on both variants.
- [x] `client/src/features/match/MatchPage.tsx` -- resolve the flag from `matchState.variant` and pass it into the prompt's `detectDeclarations` fallback (`:2013-2016`).
- [x] `client/src/features/match/lib/declarations.test.ts` -- add a Croatian overlap describe plus the tie case; the five existing Bitola assertions stay as they are.
- [x] `client/src/features/match/components/DeclarationReveal.test.tsx` -- add one regression test for a single seat with two overlapping melds: both rows render, the shared card resolves via `getAllByTestId`, two declarer rows carry the right `data-seat`, total is the sum.
- [x] `_bmad-output/implementation-artifacts/deferred-work.md` -- append a dated closure note to the D67 entry recording it closed by verification, citing the centring decision and the two existing multi-meld tests.

**Acceptance Criteria:**

- Given `server/internal/game`, when searched for a comparison of `state.Variant` or a variant constant outside the preset resolver, then there are none.
- Given the full pre-existing test suite, when it runs, then every Bitola assertion passes unchanged except the newly specified equal-value tie, and `event_match_state.json` is byte-identical to `HEAD`.
- Given the same overlapping hand, when it is detected under each config, then the Croatian result keeps both melds and the Bitola result keeps one, and the client detector returns exactly what the server would.
- Given a Croatian hand whose overlapping melds are the only ones at the table, when the hand is scored, then the team receives the sum of both meld values and card points are unaffected.
- Given the client, when searched for a mapping from variant to rule behaviour, then `variantRules.ts` is the only one. The remaining variant-string uses are label lookups and the create-room selector (`roomLabels.ts:10`, `CreateRoomModal.tsx:734`, `MatchPage.tsx:1540`); their Croatian branch belongs to 12.8 per this spec's Never list, so they are deliberately left alone.
- Given the four locale files, when the parity test runs, then their key sets are unchanged and identical.
- Given the create-room surface and the server variant allowlist, when this story is complete, then Croatian is still not selectable, and declarations still resolve during trick 1 in both variants.

## Spec Change Log

## Design Notes

**Why fixing the tie needs no new parameters.** Overlap is only ever possible between a sequence and a four-of-a-kind: sequences are maximal per-suit runs, so two sequences of one suit are disjoint and two of different suits share no card; two four-of-a-kinds are rank-disjoint. So every dedup comparison is sequence-vs-four-of-a-kind, and `declarationBeats` rule 2 alone settles it — the later chain steps needing trump and seat (unavailable at detection time, `PlayerSeat` is stamped after detection) are unreachable here. The only reachable tie is value 100: a quinte-or-longer against a four-of-a-kind of A/T/K/Q.

**Why D-VAR-1 is a server-package rule.** `GameState.Rules` is `json:"-"` by design, so the client cannot read the config and must derive overlap from the variant string. Concentrating that in one resolver preserves the spirit of D-VAR-1 — a single variant-aware construct — without leaking the config onto the wire.

**Test determinism.** `detectDeclarations` iterates Go maps by suit and by rank, so the order of several survivors varies between runs. Overlap makes multi-survivor sets common, so assert with element matching or sort first; never index into `Declarations[0]`. Use `assert.Empty`/`Len` rather than nil checks — the codebase is inconsistent about empty-vs-nil declaration slices.

## Verification

**Commands:**

- `cd server && go test ./internal/game/...` -- expected: all pass, including the untouched `TestDedupBitola` overlap subtests.
- `cd server && go test ./internal/game/... -run Declaration -count=5` -- expected: stable across runs, proving the new assertions are order-agnostic.
- `cd server && go test ./internal/ws/... -run Contract` -- expected: pass with the regenerated declarations golden committed.
- `cd server && go test ./...` -- expected: no regressions in `match`, `bot`, or `ws`.
- `cd server && golangci-lint run ./... && gofmt -l .` -- expected: clean, no output.
- `cd server && git diff --stat HEAD -- internal/ws/testdata/events/event_match_state.json` -- expected: empty.
- `cd client && npx vitest run` -- expected: all pass, including `wsEvents.contract.test.ts` and `i18n.parity.test.ts`.
- `cd client && npx tsc -p tsconfig.build.json --noEmit` -- expected: clean; the only gate on the schema conformance witnesses.
- `cd client && npx eslint . && npx prettier --check .` -- expected: clean.
- `cd client && git diff --stat HEAD -- src/shared/i18n/` -- expected: empty.

**Manual checks (if no CLI):**

- Grep `server/internal/game` for the variant constants and for `.Variant` — expected: the preset resolver only, no comparisons.
- Grep `client/src/features/match` for the variant strings — expected: `lib/variantRules.ts` only.
- Confirm the create-room variant option for Croatian is still `disabled` and the server allowlist still rejects it.

## Suggested Review Order

**Rule selection (start here)**

- The story's rule: the dedup is skipped by config, never by variant name.
  [`declarations.go:102`](../../server/internal/game/declarations.go#L102)

- The equal-value tie now keeps the four-of-a-kind, matching `declarationBeats` rule 2.
  [`declarations.go:136`](../../server/internal/game/declarations.go#L136)

- Both call sites source the flag from resolved state, so nothing defaults.
  [`declarations.go:315`](../../server/internal/game/declarations.go#L315)

**Croatian coverage**

- The first Croatian playing-phase fixture; `Variant` and `Rules` always agree.
  [`fixtures.go:790`](../../server/internal/game/testfixtures/fixtures.go#L790)

- The premise the tie fix rests on, now swept rather than asserted in a comment.
  [`declarations_test.go:1207`](../../server/internal/game/declarations_test.go#L1207)

- Both teams declaring: an overlap-inflated sum still loses to a stronger single meld.
  [`declarations_test.go:1422`](../../server/internal/game/declarations_test.go#L1422)

- The engine itself sets the prompt flag identically under both configs.
  [`declarations_test.go:1134`](../../server/internal/game/declarations_test.go#L1134)

**Client mirror**

- The only place on the client that maps a variant to behaviour; unknown strings fall back to Bitola.
  [`variantRules.ts:43`](../../client/src/features/match/lib/variantRules.ts#L43)

- The mirrored gate and the mirrored tie, so the prompt shows exactly what the server will record.
  [`declarations.ts:122`](../../client/src/features/match/lib/declarations.ts#L122)

- The wiring test a hardcoded `false` would have silently satisfied before review.
  [`MatchPage.test.tsx:1621`](../../client/src/features/match/MatchPage.test.tsx#L1621)

**Peripherals**

- A card shared by two melds is remembered once, keeping the bot's exact-cards invariant.
  [`memory.go:71`](../../server/internal/bot/memory.go#L71)

- Two same-seat entries pin payload and schema shape, not the producer loop.
  [`declarations_resolved.json`](../../server/internal/ws/testdata/events/declarations_resolved.json)

Note: the `## Code Map` anchors above are pre-change positions, captured during planning; the doc comments this story added shifted them by a few lines.
