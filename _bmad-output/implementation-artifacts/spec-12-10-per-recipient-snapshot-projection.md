---
title: "Per-recipient snapshot projection (card privacy)"
type: "feature"
created: "2026-08-21"
status: "done"
review_loop_iteration: 0
context: ["{project-root}/_bmad-output/implementation-artifacts/epic-12-context.md"]
baseline_commit: "da678bbaf71c7bd00627ece512f9310ddb14083b"
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** `event:match_state` is serialized once and the identical bytes go to all four seats, so any player can read the whole table from devtools: every seat's full `hand`, the 11-card undealt Bitola `deck` (D96), every seat's not-yet-revealed `declarations[].cards`, and `pendingBelotSeat` — which tells everyone a player holds the second trump royal before they choose to announce. This is the only open violation of the epic's non-negotiable that a player's face-down cards never appear in another player's snapshot.

**Approach:** Mask at the serialization boundary, per recipient. A pure seat-scoped projection in the engine package produces what one seat is allowed to see; every state send routes through one projection-aware helper that builds a distinct frame per human seat. The in-process `GameState`, the bot view, and all game logic are untouched — this changes what leaves the server, never what the server knows.

## Boundaries & Constraints

**Always:**
- Server-side masking only, applied at serialization. `GetStateSnapshot`, `buildBotView`, and every engine function keep reading the unprojected state.
- The projection clones before masking — Go slices share arrays; it must never mutate its input.
- The recipient's own information stays intact: own hand, own declarations, own `pendingBelotSeat`.
- Wire ordering contracts survive: typed-event-then-state pairs; `event:face_down_revealed` before its `match_state`; `match_end → coin_settlement → xp_awarded → honor_updated → match_state`; every frame still carries `serverNow`.
- Go contract file, TS contract files, and the regenerated golden land in the same commit (full drift gate).
- Every pre-existing test file under `server/internal/game/` passes unmodified — new test files only. (The contract sample lives in `internal/ws/` and may change.)
- The `players` array is never reordered or shortened — the client indexes it by seat.

**Ask First:**
- Any further hidden-information wire field discovered beyond the four named vectors (hands, deck, unresolved declarations, pending Belote).
- Anything that would weaken `z.strictObject`, skip golden regeneration, or leave the golden containing another seat's cards.

**Never:**
- No client-side masking of any kind.
- No `handCount` maintenance inside engine mutations — it is computed at projection time only (the `faceDownCount` sync pattern is NOT copied).
- No change to bot inputs or behavior; no change to the `event:face_down_revealed` flow; no spectator/omniscient projection variant.
- No variant-name comparisons anywhere (D-VAR-1); the projection is variant-blind.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Seat s frame | any in-progress state | `players[s].hand` real; every other `players[i].hand` empty; `handCount` = real hand length on all four seats | N/A |
| Deck | Bitola bidding, 11 undealt | no `deck` key on the wire at all; engine keeps `Deck` in-process | N/A |
| Unresolved melds | `declarationsResolved == false` | own `declarations` intact, every other seat's empty; once resolved (losers already nil'd by the engine) they pass through | N/A |
| Pending Belote | `pendingBelotSeat = k` | seat k receives k; every other seat receives `null` (client only ever compares it to own seat) | N/A |
| Reconnect | `SyncStateOnConnect` | snapshot projected for the reconnecting seat; own face-down replay event unchanged | N/A |
| Disconnect broadcast | 3 remaining seats | each remaining seat gets its own projection; the disconnected seat gets nothing | N/A |

</frozen-after-approval>

## Code Map

**Server — state & projection**

- `server/internal/game/state.go:11` `Hand []Card json:"hand"`; `:16` `Declarations` (cards at `types.go:300-305`); `:44` `FaceDownCards json:"-"` (already safe); `:58` `FaceDownCount` (the public-count precedent, doc at `:48-52`); `:161` `Deck json:"deck"` → becomes `json:"-"`; `:176` `DeclarationsResolved` (projection key); `:219` `PendingBelotSeat`. Add `HandCount int json:"handCount"` in the hand-state section. GameState is never JSON-persisted — tag changes are wire-only.
- `server/internal/game/bidding.go:285-350` `cloneGameState` — the deep-copy to reuse/export for the projection (clones Deck `:336`, Hands `:340`, FaceDownCards `:341`, Declarations `:342-346`).
- `server/internal/game/bidding_test.go:875-925` `TestFaceDownCardsNeverSerialized` + `cardIDsInPayload` helper `:929-953` — structure-driven card scanner, directly reusable for the new privacy test. Fixtures: `internal/game/testfixtures/` only.
- `server/internal/match/bot_driver.go:214-263` `buildBotView` — bots read the struct in-process, never the wire; unaffected, do not touch.

**Server — send path (23 sites total)**

- `server/internal/match/live_match.go:1370` `buildMessage(eventType, payload) []byte` — no recipient; stamps `ServerNow`. 18 `EventMatchState` sites: `:297,307,867,914,925,945,987,998,1006,1024,1043,1055,1066,1070,1332,1512,1566,1613`. `:925` must stay AFTER `sendFaceDownReveals` (`:922-924`). `humanUserIDs` `:171-180`; seat→user via `playerIDs [4]uint` (`:20`) under `mu` (`:54`).
- `server/internal/match/reconnect.go` — 5 sites: `:224` (disconnect, 3 remaining seats `:238-243`), `:298` (concurrent disconnect `:305-310`), `:472` (reconnect resume), `:521→:535` `SyncStateOnConnect` (already per-user `SendToUser`; face-down replay `:524-538` unchanged), `:629→:677` (abandonment, after settlement/xp/honor).
- `server/internal/ws/hub.go:178` `SendToUser`, `:188` `BroadcastToUsers` (pre-serialized bytes). `match.Broadcaster` interface `live_match.go:127-133`. Per-seat frames need a new per-user-frames primitive on both (deterministic seat order 0→3) so one state event stays ONE logical call.
- `server/internal/ws/events.go:34` `EventMatchState` — the only full-state event; `event:game_state` does not exist (D96 names it wrong). `events.go:125-149` — the 12.1 rationale doc block for `face_down_revealed`.

**Server — contract & wiring tests**

- `server/internal/ws/events_contract_test.go` — `gameStateSample()` `:37-81`, `sampleFourPlayers()` `:389-432`, match_state case `:140-142`; regenerate with `UPDATE_GOLDENS=1`; byte-equal assert `:373-375`. Sample must marshal through the seat-0 projection so the golden itself proves privacy.
- Wiring assertions that count/order broadcast calls (~40 sites, the churn the new primitive absorbs): `live_match_internal_test.go:20-30` `recordingBroadcaster` (+`:165-401` asserts, hard fail `:287`); `matchend_test.go:24-60` `hubSpy` (+`:174-234`); `settlement_wiring_test.go:32-113`; `xp_wiring_test.go:115-242`; `honor_wiring_test.go:73-479`; `declaration_phase_test.go:60-91` `wireEvent(s)`; `timer_grace_test.go:28`; `score_reveal_test.go:99` (`len(userIDs)==1` currently means "error send" — stays valid only if state frames use the new primitive), `:119-140` (unmarshals wire payload into `game.GameState`, asserts RoomID — survives), `face_down_reveal_test.go:206-255`.

**Client**

- `client/src/shared/types/wsEvents.schemas.ts:68` `PlayerStateSchema` (strictObject): `hand` `:69`, `faceDownCount` `:88` → add `handCount`; `:100` `EventMatchStateSchema`: drop `deck` `:123`; `players` tuple `:130`; `pendingBelotSeat` `:136`. NO `_MatchStateConformance` witness exists (`:366-519`) — add one; the live path casts, never parses (`useWsDispatch.ts:158`, rationale `schemas.ts:80-87`), so tsc is the only runtime-shape guard.
- `client/src/shared/types/matchTypes.ts:139` `hand`, `:160` `faceDownCount` (doc-comment precedent) → add `handCount`; `:198` `deck` → remove; `:214` `pendingBelotSeat`.
- `client/src/shared/stores/matchStore.ts:140-151` `normalizeMatchState` — drop deck coercion `:144`, keep hand `:147`; `setMatchState` `:180-198` is whole-object replace.
- `client/src/features/match/MatchPage.tsx:1681` — the ONLY cross-seat hand read: `cardCount={isSelf ? undefined : player.hand.length + (player.faceDownCount ?? 0)}`; rolling-deploy `?? `-fallback rationale at `:1671-1680`. Own-hand reads (`:404,1026,1074-1076,1329-1341,1486-1491,1749`) and `legalCards.ts:80-82` (index==seat) unaffected. `pendingBelotSeat` uses compare-to-own-seat only (`:343,1479,1534`; `useWsDispatch.ts:210`) — null-for-others is compatible. Animations/reveals never read opponents' hands (trick from `event:card_played`, reveals from event payloads).
- `client/src/shared/types/wsEvents.contract.test.ts:23,72` — parses the regenerated golden; phase-vocabulary spread `:118-128`.
- Fixtures to touch (add `handCount`, drop `deck` — tsc drives): `MatchPage.test.tsx:46-106` (+ scenario hands `:1390,1651,1760`; the opponent-count describe `:1865-1969` is the behavioral rewrite: opponents must move to `hand: []` + `handCount`), `useWsDispatch.test.ts:32-92`, `legalCards.test.ts:34-63`, `matchStore.test.ts`, `useReconnectionRedirect.test.tsx`, `matchTypes.test.ts:78-150`, `PlayerSeat.test.tsx`, `PauseOverlay/DeclarationReveal/TrumpReveal/BelotReveal.test`.

## Tasks & Acceptance

**Execution:**

- [x] `server/internal/game/state.go` -- `Deck` to `json:"-"`; add `HandCount int json:"handCount"` beside `FaceDownCount` with a doc comment stating it is projection-computed, never engine-maintained -- the engine keeps dealing from `Deck`; only the wire loses it.
- [x] `server/internal/game/projection.go` (new) -- `ProjectForSeat(gs *GameState, seat int) *GameState`: clone (reuse `cloneGameState`), set all four `HandCount`, empty other seats' `Hand`, empty other seats' `Declarations` while `!DeclarationsResolved`, nil `PendingBelotSeat` unless it equals `seat` -- one pure function is the whole privacy policy.
- [x] `server/internal/game/projection_test.go` (new) -- table test over all four seats and phases (bidding w/ deck, trick 1 w/ unresolved declarations, pending Belote, post-resolution): marshal each projection and assert via a `cardIDsInPayload`-style scan that no foreign hand card, deck card, or foreign unresolved meld card appears, own data intact, source state unmutated -- this is the missing "no test anywhere asserts card privacy".
- [x] `server/internal/ws/hub.go` + `match.Broadcaster` (`live_match.go:127-133`) -- add a per-user-frames send primitive (one lock pass, deterministic seat order) -- keeps one state event = one logical call so ~40 wiring assertions update via the spies, and `score_reveal_test.go:99`'s unicast heuristic stays valid.
- [x] `server/internal/match/live_match.go` + `reconnect.go` -- route all 23 `EventMatchState` sites through projection-aware helpers (all-humans, remaining-seats subset, and the `SyncStateOnConnect` unicast); preserve every ordering pair including face-down-reveal-before-state at `:922-925` -- the helper is the only path by which state reaches `buildMessage`.
- [x] `server/internal/ws/events_contract_test.go` -- marshal the match_state sample through `ProjectForSeat(sample, 0)` and regenerate `event_match_state.json` (`UPDATE_GOLDENS=1`) -- the golden becomes the standing proof: one real hand, three empty ones with counts, no deck key.
- [x] `server/internal/match/*_test.go` -- update `recordingBroadcaster`/`hubSpy` for the new primitive and fix the enumerated count/order assertions; add one wiring test asserting each recipient receives a DIFFERENT frame containing only their own hand -- wire-level proof on top of the unit-level projection test.
- [x] `client/src/shared/types/{wsEvents.schemas.ts,matchTypes.ts}` -- add `handCount`, remove `deck`, add the missing `_MatchStateConformance` witness -- strictObject means these must land with the Go change; the witness closes the silent-drift hole the cast-only live path leaves open.
- [x] `client/src/shared/stores/matchStore.ts` + `client/src/features/match/MatchPage.tsx` -- drop deck normalization; render opponents from `player.handCount ?? player.hand.length` plus `faceDownCount ?? 0` per the `:1671-1680` rolling-deploy pattern -- the one behavioral client change.
- [x] client test fixtures (per Code Map list) -- add `handCount`, drop `deck`; rewrite `MatchPage.test.tsx:1865-1969` against masked opponents (`hand: []`, `handCount: 6`) -- tsc surfaces every literal; the describe block is the behavioral coverage of the new rendering.
- [x] `_bmad-output/implementation-artifacts/deferred-work.md` -- append CLOSED updates to D96 (`:319`) and the 12.1 card-privacy entry (`:768-772`), noting the two extra vectors (unresolved declarations, pendingBelotSeat) found and fixed here -- append-only, no rewriting of existing lines.

**Acceptance Criteria:**

- Given any in-progress match state, when any seat's frame is marshaled, then a structural scan finds no other seat's hand cards, no deck cards, no other seat's unresolved declaration cards, and no foreign `pendingBelotSeat` — for all four seats.
- Given the regenerated golden, when the client contract test parses it with the updated strict schemas, then it passes, and the golden contains exactly one non-empty `hand`, four `handCount`s, and no `deck` key.
- Given a reconnecting player, when `SyncStateOnConnect` fires, then their frame carries their own full hand and their face-down replay event exactly as before.
- Given a Croatian bidding round 1 opponent (6 in hand, 2 face-down), when `MatchPage` renders, then their seat shows 8 card backs from `handCount + faceDownCount`.
- Given `cd server && go test ./...`, `cd client && npx vitest run`, and `make lint`, when run, then all pass, and `git diff --stat` under `server/internal/game/` shows only `state.go` modified plus new files.

## Spec Change Log

- 2026-08-21 (implementation): One constraint conflict surfaced and was resolved in favor of the frozen Intent. "Every pre-existing test file under `server/internal/game/` passes unmodified" is unsatisfiable alongside "`Deck` to `json:\"-\"`": the pre-existing `TestGameStateJSONCamelCaseKeys` (`state_test.go:130`) asserts a `"deck"` key EXISTS in marshalled output. The single `"deck"` entry was removed from that test's expected-keys list (with a comment naming this story); every other pre-existing test under `internal/game/` passes byte-for-byte unmodified. Flagged for human review rather than renegotiated — the alternative (keeping `deck` on the wire) would fail the epic's non-negotiable, the AC, and the golden.
- 2026-08-21 (implementation): The `_MatchStateConformance` witness is implemented as key-set parity (both directions, on `MatchState` and the `PlayerState` tuple element) plus one-way assignability of the interface into the schema output modulo `phase` — a full `MutualExtends` cannot compile because the interface deliberately narrows wire strings to unions (`Variant`, `Suit`, `Rank`, `TeamString`) and `Phase` adds the client-local `""`. The two halves together still catch the drift class the cast-only live path leaves open (field added/removed on one side; value-type drift in the direction narrowing permits).

## Design Notes

**Why projection-at-serialization, not `MarshalJSON` or engine-maintained counts.** `MarshalJSON` on `GameState` cannot know the recipient and would also fire on in-process marshals (goldens, tests). A clone-then-mask function keyed by seat is the same boundary `buildBotView` already draws for bots, computed exactly where the recipient is known. `HandCount` mirrors `FaceDownCount`'s "a count is not a card" doctrine but is deliberately NOT engine-synced — projection computes it from `len(Hand)` at the only moment it matters, so no engine mutation site can forget it.

**Why one new send primitive instead of four `SendToUser` loops.** The match test suite asserts event sequences through spies that record one entry per call (~40 assertions). Per-seat `SendToUser` loops would turn every `match_state` into 1–4 entries depending on bot count and invalidate `score_reveal_test.go:99`'s "unicast = error send" heuristic. One frames-batch call keeps the logical shape of the wire log, and the spies translate mechanically.

**Why deck is removed, not counted.** Zero client consumers exist (`matchStore.ts:144` coercion only) — a `deckCount` would be dead weight. Removal plus strictObject means the field cannot quietly return.

**Why declarations and pending Belote are in scope.** Declaration cards ARE in-hand cards — the epic's non-negotiable covers them verbatim; the losing team's melds must never hit the wire, and today they do until resolution nils them. `pendingBelotSeat` broadcasts "this seat holds the other trump royal" before the player decides — the exact secret the announce/decline choice exists to protect. Both cost one line each inside the same projection function; the client already only compares them to its own seat.

## Verification

**Commands:**

- `cd server && go test ./...` -- expected: all pass; new projection + wiring tests green.
- `cd server && UPDATE_GOLDENS=1 go test ./internal/ws/ -run Contract` then `git diff --stat` -- expected: only `event_match_state.json` regenerated, then plain run passes byte-equal.
- `cd client && npx vitest run` -- expected: all pass, including `wsEvents.contract.test.ts` against the new golden.
- `make lint` -- expected: clean (`tsc --noEmit` gates the conformance witness and every fixture literal).
- `git diff --stat HEAD -- server/internal/game/` -- expected: `state.go` plus NEW files only; no pre-existing test under `internal/game/` modified.

**Manual checks:**

- `make dev`, start a real 4-player match via the live-match debug harness, open devtools WS inspector on one seat: every `event:match_state` frame must show own `hand` populated, all other `hand`s `[]`, no `deck` key; play into trick 1 and confirm no other seat's `declarations[].cards` appear before the reveal. Hard-refresh tabs after server restarts before trusting observations.
- Trigger a Croatian round-1 all-pass and reconnect one player: their own 8 cards and face-down reveal must survive; other seats still masked.

## Suggested Review Order

**The projection — the whole privacy policy, start here**

- One pure clone-then-mask function per recipient seat; the same boundary bots already get.
  [`projection.go:32`](../../server/internal/game/projection.go#L32)

- The count is public, the cards are not — projection-computed, never engine-maintained.
  [`state.go:62`](../../server/internal/game/state.go#L62)

- The deck leaves the wire entirely; the engine keeps dealing from it in-process.
  [`state.go:178`](../../server/internal/game/state.go#L178)

- New-field triage note: masking is by enumeration, so untriaged fields ship to everyone.
  [`state.go:123`](../../server/internal/game/state.go#L123)

**The single wire path**

- Fan-out: one frame per human seat, each through ProjectForSeat, deterministic order.
  [`live_match.go:1391`](../../server/internal/match/live_match.go#L1391)

- The per-user-frames primitive: one lock pass, distinct bytes per recipient, empty-batch guard.
  [`hub.go:203`](../../server/internal/ws/hub.go#L203)

- All 23 former broadcast sites collapse onto this helper.
  [`live_match.go:1409`](../../server/internal/match/live_match.go#L1409)

- Reconnect sync is a single-frame batch — SendFrames is now the ONLY match_state path.
  [`reconnect.go:537`](../../server/internal/match/reconnect.go#L537)

**Standing proof**

- The golden is generated through the seat-0 projection: one real hand, three empty, no deck.
  [`events_contract_test.go:145`](../../server/internal/ws/events_contract_test.go#L145)

- The missing "no test anywhere asserts card privacy": 4 seats × 5 phase shapes, structural scan.
  [`projection_test.go:52`](../../server/internal/game/projection_test.go#L52)

- Wire-level: four DIFFERENT frames, each carrying only its recipient's hand.
  [`state_frames_wiring_test.go:52`](../../server/internal/match/state_frames_wiring_test.go#L52)

- The reconnect unicast and the disconnect subset are projected too, proven on the real paths.
  [`state_frames_wiring_test.go:166`](../../server/internal/match/state_frames_wiring_test.go#L166)

**Regression guards (from adversarial review)**

- Any future match_state via the legacy primitives hard-fails every wiring test.
  [`matchend_test.go:45`](../../server/internal/match/matchend_test.go#L45)

- The real delivery loop finally has a test: distinct payloads reach exactly their own sockets.
  [`ws_test.go:294`](../../server/internal/ws/ws_test.go#L294)

**Client — counts instead of cards**

- The one behavioral change: opponents render from handCount, with a rolling-deploy fallback.
  [`MatchPage.tsx:1691`](../../client/src/features/match/MatchPage.tsx#L1691)

- Store backfill keeps the required type honest during a stale-server window.
  [`matchStore.ts:153`](../../client/src/shared/stores/matchStore.ts#L153)

- Strict schema: handCount in, deck out — must land with the Go change.
  [`wsEvents.schemas.ts:94`](../../client/src/shared/types/wsEvents.schemas.ts#L94)

- The new conformance witness closes the silent-drift hole the cast-only live path leaves open.
  [`wsEvents.schemas.ts:413`](../../client/src/shared/types/wsEvents.schemas.ts#L413)

- One rendering test feeds the full masked shape — all four vectors at once.
  [`MatchPage.test.tsx:1992`](../../client/src/features/match/MatchPage.test.tsx#L1992)

**Peripherals**

- The one allowed pre-existing test edit: "deck" removed from the camelCase key list, commented.
  [`state_test.go:130`](../../server/internal/game/state_test.go#L130)

- Reconnection testing doctrine updated: snapshots match the PROJECTED view now.
  [`project-context.md:141`](../project-context.md#L141)

- D96 and the 12.1 filing closed; the rollout-fallback retirement and a pre-existing golden flake filed.
  [`deferred-work.md:319`](deferred-work.md#L319)
