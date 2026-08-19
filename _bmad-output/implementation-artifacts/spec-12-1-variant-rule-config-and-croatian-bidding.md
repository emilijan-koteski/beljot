---
title: "Variant rule configuration & Croatian dealing/bidding"
type: "feature"
created: "2026-08-18"
status: "done"
review_loop_iteration: 0
context: ["{project-root}/_bmad-output/implementation-artifacts/epic-12-context.md"]
baseline_commit: "b33e15afd302d69bfaee4e12fb6e728553995edc"
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Bitola's dealing and bidding rules are hardcoded in function bodies, so there is no way to express a second variant. Croatian Belot diverges in seven places, and the platform is meant to support both without either one's rules bending to fit the other.

**Approach:** Introduce a `VariantRules` value resolved **once** in `NewGame` and carried on `GameState`; every divergence reads a config field and no engine file ever compares the variant name (D-VAR-1). Then implement the two divergences this story owns — Croatian's deal-all-before-bidding with no trump candidate, and its bidding: a freely named suit in both rounds, the two face-down cards revealed to their owner alone when round 1 is passed out, and a dealer who must pick in round 2. Croatian stays unselectable; Story 12.8 exposes it.

## Boundaries & Constraints

**Always:**

- All seven divergence fields exist on `VariantRules` from day one, and each preset resolver returns a **fully populated** config — no field left to a zero value. Only this story's fields are *read*; 12.5/12.6/12.7 read the rest.
- `VariantRules` holds value types only — no pointer, slice, or map — so `cloneGameState`'s shallow struct copy stays correct without a new clone line.
- An unknown variant string resolves to the Bitola preset. Explicit, tested behaviour — never a zero-value config.
- The two face-down cards are **never serialized to any client but their owner**. They live in a `json:"-"` field and reach their owner only through a per-seat event, so they are absent from every `match_state` payload including their owner's.
- Every pre-existing Bitola test passes unchanged, and Bitola's bytes on the wire are identical to today.
- A new WS event completes the whole drift gate in **one commit**: `events.go`, the Go contract case + generated golden, `wsEvents.ts`, `wsEvents.schemas.ts` (schema + conformance witness + witness registry), the TS contract test, and dispatch.
- New i18n keys land in all four locales. Macedonian is all-Cyrillic; Croatian and Serbian forms are never mixed; nothing reads as a literal English calque. The word "contract" is banned — the trump caller is *the taker*.
- Go tests are table-driven with a `tests` slice, `tc` loop variable, lowercase prose subtest names, and no `t.Parallel()`. Game state comes from `testfixtures` factories, never raw struct literals.

**Ask First:**

- Adding `croatia` to the room-creation allowlist or the create-room UI — that is Story 12.8.
- Any change to what a Bitola game puts on the wire.
- Any change to the existing behaviour of broadcasting all four hands and the undealt deck to every player.

**Never:**

- Compare `state.Variant` (or any variant string) anywhere in `server/internal/game`. The preset resolver is the only variant-aware construct.
- Add the `declaring` phase, declaration-overlap behaviour, or the hanging-points tie rule. Their config fields exist; their behaviour is 12.5/12.6/12.7.
- Mask other players' hands or the undealt deck. That leak is pre-existing and deliberately out of scope — it is recorded in `deferred-work.md`.
- Mask the face-down cards on the client. Server-authoritative only.
- Restructure `handlePickTrump`'s Bitola stage-2 rotation or `reshuffleAndRedeal`'s dealer rotation.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Croatian deal | `NewGame(..., croatia, ...)` | 8 cards per seat: 6 in `Hand`, 2 in the hidden field. `TrumpCandidate` nil, `Deck` empty | N/A |
| Bitola deal | `NewGame(..., bitola, ...)` | Unchanged: 5 per seat, candidate flipped, `Deck` holds 11 | N/A |
| Croatian round-1 pick | `pick_trump{suit:"S"}` | `TrumpSuit`=S, picker takes no card, hidden cards merge into all hands, phase → `playing` | N/A |
| Croatian pick, no suit | `pick_trump{}` in either round | Rejected, state unchanged | `ErrInvalidBid` |
| Croatian pick, bad suit | `pick_trump{suit:"X"}` | Rejected, state unchanged | `ErrInvalidBid` |
| Croatian passed out round 1 | 4 passes in round 1 | Round 2 opens, `ActivePlayerSeat`=dealer+1, all four suits available, each seat's 2 hidden cards revealed **to that seat only** | N/A |
| Croatian dealer, round 2 | 3 passes in round 2, dealer to act | `pass_trump` rejected; `pick_trump` is the only legal action | `ErrInvalidBid` |
| Croatian all-pass round 2 | — | Unreachable: the forced dealer pick means `reshuffleAndRedeal` cannot be entered under this config | N/A |
| Bitola round-1 pick | `pick_trump{suit:"S"}` | Unchanged: `action.Suit` ignored, trump = candidate suit, candidate joins the picker's hand, stage-2 rotation identical | N/A |
| Bitola round-2 candidate suit | `pick_trump` naming the candidate's suit | Unchanged: rejected | `ErrInvalidBid` |
| Bitola passed out twice | 8 passes | Unchanged: `reshuffleAndRedeal`, dealer rotates, `PhaseDealing` | N/A |
| Reconnect mid round 2 | Croatian player reconnects after the reveal | Their own two revealed cards are re-sent to them alone | N/A |

</frozen-after-approval>

## Code Map

**Rules engine — `server/internal/game/`**

- `types.go:74-78` — `type Variant string` and the lone `VariantBitola` constant. `VariantCroatia` does not exist yet. `Suit`/`Card`/`Action` at `:6-16`, `:36`, `:125-129` (`Action.Suit *Suit`).
- `state.go:69-147` — `GameState`, with a documented section order at `:61-68`. `Variant` at `:73`, `TrumpCandidate` at `:83`, `Deck` at `:86` (the 11-card hold), `Players [4]PlayerState` at `:97`. `HandCompleteReady` at `:119` is the in-repo precedent for a server-only `json:"-"` field.
- `state.go:10-27` — `PlayerState`; `Hand []Card` at `:11`. **No per-card visibility concept exists anywhere.**
- `state.go:194-241` — `NewGame`: the only production `GameState` literal (`:202-215`) and the single place a config can be resolved once. Deals at `:237-239`.
- `state.go:254-280` — `dealCards`: batch of 3 (`:259-263`), batch of 2 (`:266-270`), candidate flip (`:275-277`), 11-card hold (`:280`). Called from three places — `state.go:239`, `scoring.go:206`, `bidding.go:167`.
- `bidding.go:16-32` — `handleBidding` dispatch. `handlePassTrump` at `:36-53` (round-1→2 transition at `:45-48`, reshuffle at `:51`). `handlePickTrump` at `:65-129`.
- `bidding.go:70` — **the guard to config-gate**: `TrumpCandidate == nil || len(Deck) != 11`. It is also Bitola's only protection against a slice panic in the stage-2 rotation at `:100-111`.
- `bidding.go:76-79` round-1 candidate binding; `:80-95` round-2 validation incl. the spent-suit rule; `:112` candidate joins the picker's hand; `:123-129` phase → playing.
- `bidding.go:138-173` — `reshuffleAndRedeal`, reached only from `:51`. `cloneGameState` at `:180-241` — hand-maintained deep copy; value-typed fields survive the shallow copy at `:184`.
- `scoring.go:158-218` — `startNewHand` re-deals without `NewGame`; `checkInstantWin` at `:223-235` falls back to `TrumpCandidate.Suit` and must keep tolerating nil.
- `declarations.go:94-96` — the unconditional `dedupBitola` call plus a live `TODO(croatian-variant)`. Config field only; behaviour is 12.5.

**Match layer — `server/internal/match/`**

- `live_match.go:217-265` — `StartMatch`, the only `NewGame` caller (`:233`, unvalidated `game.Variant(variant)` cast).
- `live_match.go:722-732` — where `action.Suit` is parsed off the wire; a missing suit silently yields nil.
- `live_match.go:864-888` — the `EventTrumpSelected` emit; `CardID` is sourced from `oldState.TrumpCandidate` at `:879` and the emit is **skipped entirely when the candidate is nil** (`:872-874`).
- `live_match.go:431-433` — the post-`ApplyAction` `dealing`→`bidding` transition; `:1277` `buildMessage` (no recipient parameter); `:1440-1445` bidding timeout auto-action is `pass_trump`.
- `reconnect.go:482-503` — `SyncStateOnConnect`, the one genuinely per-user send path.
- `bot_driver.go:199-207` — `buildBotView`, the existing seat-local redaction pattern. `bot/bot.go:120-161` — `decideBid` dereferences the candidate at `:126-129`.

**Wire contract**

- `server/internal/ws/events.go` — flat consts, no registry; `EventMatchState` at `:34`, `TrumpSelectedPayload` at `:136`, the "new event, not a widened payload" house rule at `:95-99`.
- `server/internal/ws/events_contract_test.go:34` — byte-exact golden gate, 20 cases at `:117-290`, `gameStateSample()` at `:37`, regenerate via `UPDATE_GOLDENS=1`. Missing golden hard-fails at `:313-319`.
- `client/src/shared/types/wsEvents.ts:83`, `:145-151` — const + `TrumpSelectedPayload`. `MatchStatePayload` at `:98` is `[key: string]: unknown`, so match-state shape lives in `matchTypes.ts` instead.
- `client/src/shared/types/wsEvents.schemas.ts` — all `z.strictObject`; `PlayerStateSchema:66`, `EventMatchStateSchema:90` (no conformance witness), `variant: z.string()` at `:94`, conformance block `:333-449`, witness registry `:451-468`.
- `client/src/shared/types/wsEvents.contract.test.ts:16-35`, `:66-87` — imports the Go goldens read-only and parses each through its schema.

**Client**

- `matchTypes.ts:10` — `export type Variant = "bitola"`, the union to widen. `MatchState` at `:104-119` (`trumpCandidate: Card | null` at `:115`).
- `TrumpPrompt.tsx:15-31` props (no `variant`); round-1/round-2 ternary at `:195`; round-1 renders **no suit grid** — a single `PICK` button at `:295-303` calling `onPick()` with no argument; round-2 grid at `:233-263` with `isLocked = trumpCandidate?.suit === suit` at `:235`; waiting-branch chips gated on `biddingRound === 2 && trumpCandidate` at `:118`. `SUITS`/`SUIT_SYMBOL`/`SUIT_COLOR`/`suitName()` at `:33`, `:35`, `:45`, `:312`.
- `TrumpPrompt.test.tsx:222-235` already pins that a null candidate renders no card and keeps all four suit buttons enabled.
- `TrumpReveal.tsx:134` bails when `cardId.length < 2`; `isFreePick = card.suit !== trumpSuit` at `:144` — invalid input when there is no candidate; candidate subline `:305-317`.
- `useWsDispatch.ts:414-425` — `EVENT_TRUMP_SELECTED` handler, drops payloads with `cardId.length < 2` at `:420`.
- `MatchPage.tsx:1092-1101` `handlePickTrump`/`handlePassTrump`; prompt render gate `:1963-1989`; `:1607` renders opponents as `player.hand.length`.
- `matchStore.ts:63`, `:70`, `:169-182` — `matchState`, `trumpReveal`, setters.
- i18n: `client/src/shared/i18n/{en,mk,hr,sr}.json`, line-aligned. `match.suits:786`, `match.trumpReveal:808`, `match.trumpPrompt:821`, `match.variants:901`. `i18n.parity.test.ts:57` requires identical key sets across all four.

**Tests**

- `testfixtures/fixtures.go` — factories at `:31` `NewGameJustDealt`, `:132` `NewGameMidPlay`, `:256` `NewGameFirstTrick`, `:335`, `:368` `NewGameLastTrick`, `:447`, `:530`, `:539`, `:551`, `:560`, `:573`, `:595` `NewGameMidBidding(passCount)` (clamped to 7). Seven raw `GameState` literals at `:31,132,256,335,368,447,595` — each needs the new config field.
- `bidding_test.go` — the Bitola regression surface: `:13` `TestPickTrumpRound1`, `:126` round-1→2 transition, `:145` `TestPickTrumpRound2`, `:221` candidate-suit rejection, `:270` reshuffle, `:404` `TestStateImmutability`, `:451` `TestMultipleReshuffles`, `:519` `TestPickTrumpStage2Rotation`, `:593` exact stage-2 deal, `:632` `TestRound1IgnoresActionSuit`. Helper `assertCardsAreFullDeck` at `:306` asserts 32-card conservation.
- `state_test.go:108` `TestGameStateJSONCamelCaseKeys` (wire-shape guard), `:142` `TestNewGame` (12 subtests).

## Tasks & Acceptance

**Execution:**

- [x] `server/internal/game/types.go` -- add `VariantCroatia`, the `VariantRules` struct with all seven value-typed fields, named string types for the multi-outcome rules (deal shape, all-pass outcome, declaration timing, tie rule), and a `RulesFor(Variant) VariantRules` resolver with fully-populated `bitola`/`croatia` presets and an explicit Bitola fallback for unknown input -- the D-VAR-1 foundation every later Epic 12 story reads.
- [x] `server/internal/game/state.go` -- carry `Rules VariantRules` on `GameState` (`json:"-"`, in the Match-metadata section) resolved once in `NewGame`; add the server-only hidden-card field; make `dealCards` deal per the config's deal shape -- Croatian deals 3+3+2-face-down with no candidate and an empty `Deck`.
- [x] `server/internal/game/bidding.go` -- config-gate the `bidding.go:70` candidate/deck guard; make round 1 either bind to the candidate (Bitola) or take `action.Suit` freely (Croatian); skip stage-2 distribution and the candidate-into-hand step when there is no candidate; reveal-and-merge the hidden cards when bidding resolves; on the fourth round-1 pass mark the round-2 reveal; reject `pass_trump` from the dealer in round 2 under the forced-pick config -- the story's bidding divergences, with Bitola's paths untouched.
- [x] `server/internal/game/scoring.go` -- ensure `startNewHand` re-deals through the same config-driven path so hand 2 of a Croatian match is not dealt Bitola-style -- `dealCards` has three callers and only one is `NewGame`.
- [x] `server/internal/game/testfixtures/fixtures.go` -- set the Bitola-resolved config on all seven `GameState` literals, and add a `NewGameCroatianJustDealt` / `NewGameCroatianMidBidding(passCount)` pair -- the zero-value config is not Bitola, so every fixture must be explicit.
- [x] `server/internal/ws/events.go` -- add the round-2 hidden-card reveal event const and its payload struct (seat + the two card IDs), per the new-event-not-widened-payload rule; document that it is sent per-seat, never broadcast.
- [x] `server/internal/ws/events_contract_test.go` -- add the contract case for the new payload, then generate its golden with `UPDATE_GOLDENS=1` -- a missing golden hard-fails the gate.
- [x] `server/internal/match/live_match.go` -- emit the reveal event to each seat's own user only when bidding enters round 2 under the reveal config; make the `EventTrumpSelected` emit work with no candidate (empty `cardId`) instead of suppressing itself -- otherwise a Croatian take fires no reveal at all.
- [x] `server/internal/match/reconnect.go` -- re-send a reconnecting player's own revealed cards in `SyncStateOnConnect` -- the reveal is a one-shot per-seat event and would otherwise be lost.
- [x] `server/internal/bot/bot.go` -- guard `decideBid` against a nil trump candidate so it cannot panic once 12.8 enables the variant -- panic guard only; Croatian bot strategy is 12.8's.
- [x] `client/src/shared/types/{wsEvents.ts,wsEvents.schemas.ts,wsEvents.contract.test.ts}` -- mirror the new event: const, payload interface, strict schema, conformance witness plus registry entry, golden import and contract-table row.
- [x] `client/src/shared/types/matchTypes.ts` -- widen `Variant` to include the Croatian value.
- [x] `client/src/shared/{hooks/useWsDispatch.ts,stores/matchStore.ts}` -- handle the reveal event into a store slice holding the viewer's own revealed cards, cleared when the phase leaves bidding; merge it into the viewer's rendered hand.
- [x] `client/src/features/match/components/TrumpPrompt.tsx` -- render the free four-suit grid in **both** rounds when there is no candidate, with nothing locked and no candidate card; extract the suit-tile markup rather than adding a third copy -- Bitola's candidate presentation and round-2 lock must be untouched.
- [x] `client/src/features/match/components/TrumpReveal.tsx` -- render a candidate-less reveal when no card accompanies the pick, instead of returning null on the short-`cardId` guard.
- [x] `client/src/shared/i18n/{en,mk,hr,sr}.json` -- add the Croatian variant label plus free-pick prompt and reveal copy to all four locales, idiomatically, with Macedonian in Cyrillic.
- [x] `server/internal/game/bidding_test.go` + `server/internal/game/state_test.go` -- table-driven tests for every row of the I/O matrix, including the unknown-variant fallback, 32-card conservation under the Croatian deal, and that no hidden card is ever reachable through a marshalled snapshot.
- [x] `client/src/features/match/components/{TrumpPrompt,TrumpReveal}.test.tsx` -- cover the candidate-less branches in both rounds and assert the existing Bitola assertions still hold.

**Acceptance Criteria:**

- Given the whole `server/internal/game` package, when it is searched for a comparison of `state.Variant` against any variant name, then there are none — every branch reads a `VariantRules` field (D-VAR-1).
- Given either preset resolver, when it returns, then all seven fields are populated, and an unknown variant string yields the Bitola preset.
- Given a Croatian game at any point before bidding resolves, when any player's `match_state` payload is marshalled, then no player's two face-down cards appear in it — including that player's own payload.
- Given Croatian bidding round 2 has opened, when the reveal is delivered, then each seat receives only its own two cards and no other seat's.
- Given the full existing test suite, when it runs, then every pre-existing Bitola test passes unchanged and the `event:match_state` golden is byte-identical to today.
- Given the create-room surface and the server variant allowlist, when this story is complete, then Croatian is still not selectable.
- Given the four locale files, when the parity test runs, then their key sets are identical and no leaf is empty.

## Design Notes

**Why the hidden cards are a `json:"-"` field plus a per-seat event, not a masked snapshot.**

`match_state` is serialized once and the identical bytes go to all four seats — `buildMessage` (`live_match.go:1277`) takes no recipient and `BroadcastToUsers` (`ws/hub.go:188`) takes pre-serialized bytes. Satisfying "visible to that player only" by masking would mean introducing a per-recipient projection and rewriting ~21 broadcast sites, with real Bitola regression risk.

Keeping the two cards out of `Hand` entirely and delivering them through a per-seat event means they are never serialized into *anyone's* snapshot, which is strictly stronger than masking: other clients never receive the data, so there is nothing to defeat client-side. The state then converges naturally — when bidding resolves, the hidden cards merge into `Hand` and every seat holds eight, matching today's behaviour for the rest of the hand.

Consequence to accept deliberately: while Croatian bidding is in progress, other seats' snapshots show six cards for a player who physically holds eight. This is invisible to players until 12.8 enables the variant, since Croatian is unselectable until then. Note that `MatchPage.tsx:1607` does pass `cardCount` in every phase, so once 12.8 lands the count will read 6 for other seats during Croatian bidding — 12.8 should decide whether to surface the two face-down cards as backs.

**Config shape.** Booleans only where the rule is genuinely binary (candidate on/off, round-2 reveal on/off, declaration overlap). The other four use named string types so the code reads as an outcome rather than a flag, e.g. an all-pass outcome of *reshuffle-and-rotate* versus *dealer must pick*. Every field is a value type, which is what lets `cloneGameState` (`bidding.go:180-241`) keep working without a new clone line.

## Verification

**Commands:**

- `cd server && go test ./internal/game/...` -- expected: all pass, including every pre-existing Bitola test unchanged.
- `cd server && go test ./internal/ws/... -run Contract` -- expected: pass with the new golden committed; the `event_match_state.json` golden must be unchanged from `HEAD`.
- `cd server && go test ./...` -- expected: no regressions in `match`, `bot`, or `ws`.
- `cd server && golangci-lint run ./...` -- expected: clean.
- `cd server && gofmt -l .` -- expected: no output.
- `cd client && npx vitest run` -- expected: all pass, including `wsEvents.contract.test.ts` and `i18n.parity.test.ts`.
- `cd client && npx tsc -p tsconfig.build.json --noEmit` -- expected: clean; this is the only gate on the schema conformance witnesses and it does **not** run in CI.
- `cd client && npx eslint . && npx prettier --check .` -- expected: clean.
- `cd server && git diff --stat HEAD -- internal/ws/testdata/events/event_match_state.json` -- expected: empty, proving Bitola's wire shape did not move.

**Manual checks (if no CLI):**

- Grep `server/internal/game` for the variant constants and for `.Variant` — expected: writes and the preset resolver only, no comparisons.
- Confirm the create-room variant allowlist and the create-room UI still offer Bitola alone.

## Suggested Review Order

**Rule-config foundation (start here)**

- The D-VAR-1 contract: seven value-typed fields, so cloning stays a shallow copy.
  [`types.go:145`](../../server/internal/game/types.go#L145)

- The only variant-aware construct in the engine; unknown input falls back to Bitola.
  [`types.go:185`](../../server/internal/game/types.go#L185)

- Config rides on state but is `json:"-"`, so no wire payload moved.
  [`state.go:103`](../../server/internal/game/state.go#L103)

**Croatian dealing**

- All eight cards dealt before bidding: no candidate flipped, reserve left empty.
  [`state.go:348`](../../server/internal/game/state.go#L348)

- The two withheld cards live off `Hand` entirely — never serialized to anyone.
  [`state.go:28`](../../server/internal/game/state.go#L28)

**Croatian bidding**

- The candidate/reserve guard is config-gated both ways, not deleted.
  [`bidding.go:109`](../../server/internal/game/bidding.go#L109)

- Round 1 either adopts the candidate's suit or takes the freely named one.
  [`bidding.go:119`](../../server/internal/game/bidding.go#L119)

- The forced dealer pick is what makes reshuffle-and-rotate unreachable under this config.
  [`bidding.go:43`](../../server/internal/game/bidding.go#L43)

- Hidden cards fold into every hand once bidding resolves, converging the state.
  [`bidding.go:191`](../../server/internal/game/bidding.go#L191)

- The fourth round-1 pass is what arms the owner-only reveal.
  [`bidding.go:73`](../../server/internal/game/bidding.go#L73)

**Per-seat delivery — the privacy mechanism**

- One `SendToUser` per human seat; bot seats skipped, never broadcast.
  [`live_match.go:1066`](../../server/internal/match/live_match.go#L1066)

- Reconnect replays only the returning seat's own two cards.
  [`reconnect.go:509`](../../server/internal/match/reconnect.go#L509)

- A candidate-less take now fires a reveal; the nil-candidate warning survives for Bitola.
  [`live_match.go:864`](../../server/internal/match/live_match.go#L864)

- A new event rather than a widened payload, so stale tabs simply ignore it.
  [`events.go:125`](../../server/internal/ws/events.go#L125)

**Client wiring**

- Keyed on "bidding resolved", so a pause or disconnect cannot wipe the reveal.
  [`matchStore.ts:197`](../../client/src/shared/stores/matchStore.ts#L197)

- Merge extracted to be testable; refuses a foreign seat and malformed ids.
  [`faceDownCards.ts:29`](../../client/src/features/match/lib/faceDownCards.ts#L29)

- Shared card-id validator stops a 3-char id slicing into a plausible wrong card.
  [`cardId.ts:16`](../../client/src/shared/lib/cardId.ts#L16)

- Strict schema plus an own-seat check before anything reaches the store.
  [`useWsDispatch.ts:439`](../../client/src/shared/hooks/useWsDispatch.ts#L439)

- The merged hand is what the viewer actually sees rendered.
  [`MatchPage.tsx:1446`](../../client/src/features/match/MatchPage.tsx#L1446)

- Free four-suit grid in both rounds with nothing locked when no candidate exists.
  [`TrumpPrompt.tsx:65`](../../client/src/features/match/components/TrumpPrompt.tsx#L65)

- The seal carries its own glow when it is the hero, not the card.
  [`TrumpReveal.tsx:263`](../../client/src/features/match/components/TrumpReveal.tsx#L263)

**Guards, copy and peripherals**

- Makes "Croatian stays unselectable" a deliberate gate, naming 12.8 to unblock it.
  [`variant_allowlist_test.go:12`](../../server/internal/room/variant_allowlist_test.go#L12)

- Structural privacy assertion; the old substring form was field-order dependent.
  [`bidding_test.go:872`](../../server/internal/game/bidding_test.go#L872)

- Records the forced-dealer timeout as a hot spin, not a quiet stall, for 12.8.
  [`live_match.go:1510`](../../server/internal/match/live_match.go#L1510)

- Terminology canon: take / земи / zvati, and no em-dash outside English.
  [`en.json:827`](../../client/src/shared/i18n/en.json#L827)

- Strict payload schema mirroring the Go struct, with its conformance witness.
  [`wsEvents.schemas.ts:334`](../../client/src/shared/types/wsEvents.schemas.ts#L334)
