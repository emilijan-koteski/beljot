---
title: 'Room option: play with or without declarations'
type: 'feature'
created: '2026-08-24'
status: 'done'
review_loop_iteration: 0
baseline_commit: '04629515621b98031f3db66921ba52d3b8e62e26'
context: []
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Melds (and the Belote/Rebelote bonus) are always on. Many tables play "bez zvanja" — pure card points — and there is no way to configure that. The room owner has no control, and joiners have no way to see which kind of game a room is.

**Approach:** Add a room-level `declarationsEnabled` setting (default ON) chosen in the create-room dialog, persisted on `rooms`, carried into the resolved `VariantRules` config at game init, and read by the engine so that when it is OFF **both** variants skip their declaration path entirely — Bitola's per-seat trick-1 prompt and Croatian's dedicated `PhaseDeclaring` — and the Belote prompt never fires. Surface it as a chip on the lobby card, the waiting room, and the in-match scoreboard, rendered only when declarations are OFF.

## Boundaries & Constraints

**Always:**
- D-VAR-1 holds: the switch is a named field on `VariantRules`, resolved once at game init and read as config. No engine file compares `state.Variant` or reads a room struct.
- Both variant presets in `RulesFor` populate the new field as `true`; the per-room choice is layered over the preset inside `NewGame`. This is the first room-level override of a rule-config field — the seam Epic 12 anticipated.
- OFF disables melds **and** Belote/Rebelote: no meld prompt, no `PhaseDeclaring`, no K+Q-of-trump prompt, no `+20`. `DeclarationPoints` and `BelotPoints` stay `[0,0]` all match.
- Every pre-existing Bitola and Croatian test passes unchanged. Declarations ON must be byte-identical to today's behaviour on the wire.
- The new plumbing parameters on `NewGame` and `StartMatch` are **positional**, not an options struct: a forgotten positional arg is a compile error, a forgotten struct field silently means OFF.
- `rooms.declarations_enabled` carries **no GORM `default` tag** (the `AllowNewPlayers` trap: GORM omits zero-valued fields that declare one, making `false` uninsertable). Both hand-built `&Room{}` sites set it explicitly.
- Client reads the flag with `=== false`, never truthiness — the hand-built QuickPlay `system:room_created` map is a known omitter, and absent must read as ON.
- i18n lands in all four locales (en, mk, hr, sr) in the same commit. Reuse each locale's existing declarations term (`Declarations` / `Zvanja` / the Cyrillic mk form) — read the locale files with the Read tool, not through the cp1251 Bash console.

**Ask First:**
- Any change to the tie rule, scoring formula, or `162`-point hand total.
- Persisting the flag on the `matches` table or surfacing it in match history (deliberately out of scope).

**Never:**
- No new WebSocket event and no new client→server action. The state snapshot carries the flag.
- Do not delete or refactor the declaration/Belote code paths — they are gated, not removed.
- Do not touch Quick Play's rule set: synthesized rooms are declarations-ON, explicitly.
- Do not edit the in-app rules reference (`features/rules/content/*`) — that is Story 12.9's surface.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Create room, toggle untouched | `POST /rooms` with `declarationsEnabled` omitted | Room persists `true`; no chip anywhere | N/A |
| Create room, declarations OFF | `POST /rooms` with `declarationsEnabled: false` | Room persists `false`; chip renders on lobby card, waiting room, scoreboard | N/A |
| Bitola, OFF, trick 1 | Seat holds a tierce, plays its first card | No meld prompt; `awaitingDeclaration` never true; turn advances normally | N/A |
| Croatian, OFF, bid resolves | `pick_trump` accepted | Phase goes straight to `playing`, trick 1, `trickNumber: 1` — `declaring` is never entered | N/A |
| Either variant, OFF, plays K of trump holding Q | `play_card` K of trump | Card resolves normally; `pendingBelotSeat` stays null; no `+20` | N/A |
| Either variant, OFF, stale client sends `declare` | `action:declare` | Rejected by the existing guards (`ErrActionRequired` / `ErrWrongPhase`); state unchanged | Engine returns an apperr; no state mutation |
| Hand 2+ with OFF | `scoreHand` resets per-hand flags | Declarations stay skipped every hand, not just hand 1 | N/A |
| Reconnect mid-match with OFF | Client reconnects | Snapshot carries `declarationsEnabled: false`; scoreboard chip restored | N/A |
| QuickPlay `system:room_created` | Payload omits the key | Lobby card renders as declarations-ON (correct — quick play is ON) | Absent reads as ON via `=== false` |

</frozen-after-approval>

## Code Map

**Engine (`server/internal/game/`)**
- `types.go:148-200` -- `VariantRules` struct + `RulesFor`. Add `DeclarationsEnabled bool`; both presets return `true`.
- `state.go:154-163` -- `Rules VariantRules json:"-"`; `state.go:146-152` carries the Story 12.10 per-seat visibility triage note — the new wire field is **public**.
- `state.go:319-330` -- `NewGame(...)`. Add trailing `declarationsEnabled bool`; assign `gs.Rules = RulesFor(variant)` then override the field, and seed `DeclarationsResolved` when OFF.
- `rules_engine.go:38-40` -- `RefreshDerivedFlags`; mirror `Rules.DeclarationsEnabled` onto the wire field here so it can never drift.
- `bidding.go:200-215` -- the post-bid branch that opens `PhaseDeclaring` vs trick 1. Gate the dedicated-phase arm and the `checkDeclarationPrompt` call.
- `declarations.go:707-721` -- `checkDeclarationPrompt` (already early-returns on `DeclarationsResolved`).
- `declarations.go:561-577` -- `shouldPromptBelot`; add the config gate as its first check.
- `declarations.go:636-650` -- `resolveTrickWithDeclarations`; its `!DeclarationsResolved` guards suppress the no-op resolve.
- `scoring.go:170-190` -- per-hand reset (`DeclarationsResolved = false` at :179). Must re-seed `true` when OFF, or hand 2 regresses.
- `projection.go:52` -- masks declarations while unresolved; harmless with empty melds, but note the seeded flag changes the branch taken.
- `testfixtures/fixtures.go` -- factories set `Rules` via `RulesFor`; add a declarations-off factory. Never raw `GameState{}` literals.

**Match layer (`server/internal/match/`)**
- `live_match.go:230-246` -- `StartMatch(...)` → `game.NewGame(...)`. Add the trailing param and thread it.
- `live_match.go:1691-1820` -- timer-expiry auto-action arms for `skip_declare` / `skip_belot`. Unreachable when OFF; confirm no `default` arm stalls.
- `bot_driver.go:69,108,246,294` -- bot view fields `awaitingDecl` / `AwaitingDeclaration` / `DeclarationsResolved`. No change expected; verify bots don't stall.
- `server/internal/bot/bot.go:42-50` -- `PendingBelot` and `AwaitingDeclaration` arms; both go dead when OFF.

**Room domain (`server/internal/room/`)**
- `model.go` -- add `DeclarationsEnabled bool` beside `AllowNewPlayers` (:~79); copy that field's no-`default`-tag rationale.
- `handler.go:137-159` -- `CreateRoomRequest`; add `DeclarationsEnabled *bool` (nil → `true`), mirroring `AllowNewPlayers`.
- `handler.go:~485-500` -- validation block; resolve the pointer to a concrete bool.
- `handler.go:627-648` -- the `&Room{}` literal; set explicitly.
- `handler.go:395-420` -- `roomLifecyclePayload` map; add `"declarationsEnabled"`.
- `handler.go:3766-3790` -- QuickPlay `&Room{}` synthesis; set `true` explicitly.
- `handler.go:3853-3866` -- QuickPlay `system:room_created` map; add the key (closes the omission caveat for this field).
- `handler.go:163-165` -- `MatchStarter` interface; widen `StartMatch`.
- `handler.go:2485`, `handler.go:3654` -- the two `StartMatch` call sites; pass `room.DeclarationsEnabled`.

**Migration**
- `server/migrations/000021_add_declarations_to_rooms.{up,down}.sql` -- `ALTER TABLE rooms ADD COLUMN declarations_enabled BOOLEAN NOT NULL DEFAULT TRUE;` / `DROP COLUMN`. Pattern and commentary style: `000018_add_honor_gate_to_rooms.*`. `DEFAULT TRUE` is the backfill — every existing room stays as it plays today.

**Wire contract (both files, same commit)**
- `server/internal/game/state.go` (Go side) and `client/src/shared/types/wsEvents.schemas.ts:112-150` (`EventMatchStateSchema`, `z.strictObject` — a Go-side key fails the parse until this lands).
- `client/src/shared/types/matchTypes.ts:194-240` -- `MatchState` interface.
- `server/internal/ws/testdata/events/event_match_state.json` -- golden; regenerate with `UPDATE_GOLDENS=1 go test ./internal/ws/...` (`events_contract_test.go:146`).
- `client/src/shared/types/wsEvents.contract.test.ts` -- reads the golden; `wsEvents.schemas.ts:403-413` conformance types.
- `client/src/shared/types/wsEvents.ts:333-400` -- `RoomCreatedPayload` / `RoomUpdatedPayload`; add the field and record the QuickPlay-map status in the existing caveat comment style.

**Client UI**
- `client/src/features/room/CreateRoomModal.tsx` -- state at :118-128, option lists at :283-295, submit at :175+, reset at :265-275. Add a `Segmented` control next to variant/match-mode, default `true`, and include it in the mutation payload and the live-preview pane.
- `client/src/shared/types/apiTypes.ts:117-188` -- `Room` and `CreateRoomRequest`.
- `client/src/features/lobby/components/RoomCard.tsx:117-150` -- the meta chip row (`variantLabel · modeLabel`, then dotted chips). Add an OFF-only chip; see the absent-field precedent at :63-67.
- `client/src/features/room/RoomPage.tsx:828-832, 1205-1240` -- waiting-room badge row (`badge-match-mode`, `badge-min-honor`).
- `client/src/features/match/components/ScorePanel.tsx:290-316` -- header meta band (`score-meta`); `MatchPage.tsx:1585-1593` passes the props.
- `client/src/shared/i18n/{en,mk,hr,sr}.json` -- new keys under `lobby.createRoomModal.*`, `lobby.card.*`, `match.score.*`; `i18n.parity.test.ts` gates all four.

## Tasks & Acceptance

**Execution:**
- [x] `server/migrations/000021_add_declarations_to_rooms.{up,down}.sql` -- add `declarations_enabled BOOLEAN NOT NULL DEFAULT TRUE` with a reversing down -- persistence, with the default as the backfill.
- [x] `server/internal/room/model.go` -- add `DeclarationsEnabled bool` with no GORM `default` tag -- avoids the documented uninsertable-`false` trap.
- [x] `server/internal/game/types.go` -- add `DeclarationsEnabled` to `VariantRules`, `true` in both presets -- D-VAR-1 config field, fully populated presets.
- [x] `server/internal/game/state.go` + `rules_engine.go` -- `NewGame` trailing `declarationsEnabled bool` param, override on `Rules`, public `declarationsEnabled` wire field mirrored in `RefreshDerivedFlags`, seed `DeclarationsResolved` when OFF -- one flag makes the downstream guards fall out.
- [x] `server/internal/game/bidding.go` + `declarations.go` -- gate the `PhaseDeclaring` arm, the trick-1 prompt, and `shouldPromptBelot` -- the actual skip, in both variants.
- [x] `server/internal/game/scoring.go` -- re-seed `DeclarationsResolved` on the per-hand reset when OFF -- otherwise only hand 1 skips.
- [x] `server/internal/match/live_match.go` -- widen `StartMatch`, thread to `NewGame` -- compile-error-enforced plumbing.
- [x] `server/internal/room/handler.go` -- request field, validation, both `&Room{}` sites, both payload maps, `MatchStarter` interface, both `StartMatch` call sites -- end-to-end HTTP + lobby broadcast.
- [x] `server/internal/game/testfixtures/fixtures.go` -- declarations-off factory for both variants -- factories are the single update point.
- [x] `server/internal/game/no_declarations_test.go` (NEW FILE, in place of appending to `declarations_test.go` + `bidding_test.go`) -- table-driven cases for every I/O Matrix row, through `ApplyAction` only, plus two negative controls -- edge-case coverage. A dedicated file rather than two large existing suites: the feature is one cross-cutting gate, and splitting its cases across the meld and bidding suites would have hidden the pairing between each skip and its control.
- [x] `server/internal/room/declarations_handler_test.go` (NEW FILE) + `handler_test.go` (`fakeMatchStarter` widened) -- default-true, explicit-false, quick-play-is-true, StartMatch plumbing, and two DB-backed column cases -- request/persistence contract.
- [x] `server/internal/ws/testdata/events/event_match_state.json` -- regenerate golden -- wire drift gate.
- [x] `client/src/shared/types/{matchTypes,wsEvents,wsEvents.schemas,apiTypes}.ts` -- add the field across type, Zod schema, and room payloads -- strict-object parse fails until all land.
- [x] `client/src/features/room/CreateRoomModal.tsx` (+ `.test.tsx`) -- toggle defaulting ON, submitted and previewed -- the user-facing control.
- [x] `client/src/features/lobby/components/RoomCard.tsx`, `client/src/features/room/RoomPage.tsx`, `client/src/features/match/components/ScorePanel.tsx` + `MatchPage.tsx` (+ tests) -- OFF-only chip via `=== false` -- joiners and players can see the rule.
- [x] `client/src/shared/i18n/{en,mk,hr,sr}.json` -- new keys in all four locales -- parity test gates it.

- [x] `server/internal/match/no_declarations_session_test.go` (NEW FILE, not in the original plan) -- asserts the flag reaches the running session in both variants and that a Croatian session never opens `PhaseDeclaring` -- the StartMatch seam had no coverage of its own.

**Acceptance Criteria:**
- Given the create-room dialog is opened, when the owner submits without touching the new control, then the room is created with `declarationsEnabled: true` and no surface shows a declarations chip.
- Given a Bitola room with declarations OFF, when the match reaches trick 1 and a seat holds a valid meld, then no declaration prompt is shown, `awaitingDeclaration` is never `true`, and the hand scores on card points, last trick and Capot alone.
- Given a Croatian room with declarations OFF, when trump is picked, then the state moves directly from `bidding` to `playing` with `trickNumber: 1` and `phase` is never `declaring`.
- Given either variant with declarations OFF, when a player plays the King of trump while holding the Queen, then no Belote prompt appears and `belotPoints` stays `[0, 0]`.
- Given a room with declarations OFF, when a player views the lobby card, the waiting room, or the in-match scoreboard, then a localized "no declarations" chip is visible on each in all four locales.
- Given a room with declarations ON, when any match is played end to end, then every existing declaration and Belote behaviour and every `event:match_state` field other than the new one is unchanged.
- Given a match with declarations OFF, when a player disconnects and reconnects, then the restored snapshot carries `declarationsEnabled: false` and the scoreboard chip reappears.

## Spec Change Log

No loopback occurred: the review found no intent_gap and no bad_spec, so the frozen
intent and the non-frozen sections stand as approved. Recorded here because the
review DID find real defects, all resolved as patches on the existing plan:

- **Two code defects, both in hand-built payload maps and neither in the spec's Code Map.**
  `lobby_disconnect.go`'s `broadcastRoomUpdated` is a FOURTH room-payload key list
  (the Code Map named three) and omitted `declarationsEnabled`, so a bez-zvanja room
  lost its lobby chip whenever any lobby player disconnected. `bot_driver.go`'s
  `observeBotMemory` gates on `DeclarationsResolved` alone, which the seed makes
  permanently true, so it recorded a "reveal" on every action; now also gated on
  `Rules.DeclarationsEnabled`. KEEP: the seeded-flag design itself was sound — the
  defects were in consumers of that flag the Code Map had not enumerated. A future
  spec touching room payloads should enumerate FOUR maps, not three.
- **A required/optional incoherence the spec's own "always compare `=== false`" rule
  produced.** That rule is right for `Room` and the room WS payloads (no schema
  validates them) and wrong for `MatchState` (a `z.strictObject` requires the key, so
  the guard was unreachable and its test had to cast past the type). Resolved by
  surface class rather than uniformly: strict-schema surfaces read the field directly,
  unvalidated surfaces keep the guard. Documented on the type itself.
- **A near-tautological test, replaced.** The original card-points assertion used a
  one-card-per-seat fixture in which no meld can exist, so its ON/OFF equality proved
  little. Replaced with a full eight-trick hand holding a real quarte and two tierces
  plus K+Q of trump, and a paired ON control asserting those melds DO score when the
  toggle is on. Writing it also corrected a wrong assertion of mine: raw card points
  total 152, and 162 is that plus the last-trick bonus.
- Also patched: the documented "no spurious `declarations_resolved`" claim now has an
  assertion; `roomLifecyclePayload`'s new key has a test that provably fails without it;
  card privacy under the seeded flag is asserted rather than assumed; English used both
  "declarations" and "melds" for one concept; the mk/hr/sr hint was restructured away
  from a plural/singular agreement question and a comma splice; the create-room hint is
  now shown in both toggle states (it carries the one fact the label cannot); and the
  segmented options narrow to `"on" | "off"` instead of widening to `string`.

Two findings were deferred to `deferred-work.md` rather than patched, both excluded by
the frozen intent: persisting the flag on `matches` / surfacing it in history ("Ask
First"), and the in-match rules reference still teaching melds at a declarations-off
table ("Never" — Story 12.9 owns that surface).

## Design Notes

**Why one seeded flag plus two gates, rather than gating every call site.** Setting `DeclarationsResolved = true` at hand start when the option is OFF makes three existing guards do the work for free: `checkDeclarationPrompt` early-returns on it (`declarations.go:710`), `resolveTrickWithDeclarations` skips both resolve arms (`:641`, `:647`), and no `false → true` transition ever occurs, so `broadcastDeclarationsResolvedIfTransition` never emits a no-op reveal. Only two things still need an explicit config read: the `PhaseDeclaring` arm in `bidding.go:205` (which does not consult the flag) and `shouldPromptBelot` (Belote is not a meld). It must be seeded in **both** `NewGame` and `scoreHand`'s per-hand reset.

**Why positional params, not an options struct.** `NewGame` and `StartMatch` are already long, and a struct would read better — but `DeclarationsEnabled`'s zero value is `false`, the destructive setting. A forgotten struct field silently ships a declarations-less match; a forgotten positional argument does not compile. Same reasoning the codebase already applies to `Room.AllowNewPlayers`.

## Verification

**Commands:**
- `make lint` -- expected: clean, both stacks.
- `make test` -- expected: all Go and Vitest suites pass, including the pre-existing Bitola and Croatian engine suites with no edits.
- `UPDATE_GOLDENS=1 go test ./internal/ws/...` then `go test ./internal/ws/...` -- expected: golden regenerated once, then passes clean.
- `npx vitest run src/shared/types/wsEvents.contract.test.ts src/shared/i18n` -- expected: contract and locale-parity suites pass.
- `make migrate` -- expected: `000021` applies; re-running the down and up leaves `rooms` unchanged.

**Manual checks:**
- Create one Bitola and one Croatian room with declarations OFF, play a hand with a known meld and a K+Q of trump in hand: no prompts, no `+20`, scoreboard shows only card points.
- Same two rooms with declarations ON: prompts, reveal and Belote all behave exactly as before.
