---
title: "Croatian variant enablement"
type: "feature"
created: "2026-08-20"
status: "done"
review_loop_iteration: 0
context: ["{project-root}/_bmad-output/implementation-artifacts/epic-12-context.md"]
baseline_commit: "bc83edef185235892585425c3d3a1374b0ec780b"
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Every Croatian rule is built (12.1, 12.5, 12.6) but the variant is unselectable, and eight defects sit behind that gate — all recorded as deferred and all reachable the moment it opens. Two of them deadlock a hand outright: a Croatian dealer who does not pick in round 2 (human timeout **or** bot) has the action rejected forever, because the engine forbids that pass and nothing ever names a suit instead.

**Approach:** Fix everything the gate is hiding, then open it. Teach the timeout path and the bot to name a suit when passing is forbidden, give the bot the two revealed face-down cards it currently bids blind without, correct the player-facing surfaces that assume a Bitola shape, add Croatian to the room allowlist and the create-room control, and prove it with a real four-player Croatian match rather than only unit tests.

## Boundaries & Constraints

**Always:**
- Read `VariantRules` fields — never compare `state.Variant` (D-VAR-1). This holds in the match layer and the bot as much as in the engine; `internal/bot` and `bot_driver.go` are clean today and must stay clean.
- Quick Play stays **Bitola-only**, as a documented and intentional limit for this epic.
- Every pre-existing Bitola test passes unchanged. Bitola is a regression surface, not a refactor target.
- A player's face-down cards never reach another player's snapshot, and never their own before the round-2 reveal. A *count* is not a card — surfacing how many cards an opponent holds is allowed; surfacing which ones is not.
- New i18n keys land in all four locales (en, mk, hr, sr) or the parity test fails. mk all-Cyrillic; em-dashes en-only; `„…"` quotes in mk/hr/sr; "contract" never appears in a user-visible string.
- Auto-actions taken on an absent player's behalf stay deterministic and pure, mirroring `AutoPlay`.

**Ask First:**
- Re-tuning the bot's `wantsTrump` thresholds. They were sized for a six-card bid hand; Croatian round 2 is eight. Feeding the correct eight cards is in scope — changing the numeric bar is a balance decision and is not.
- Adding a new `apperr` sentinel to distinguish "you must pick" from "bad suit". Hiding the illegal control is this story's fix; a new public error code is not.

**Never:**
- Do not touch the in-app rules reference. Reconciling it against real engine behaviour for both variants is split into its own spec (see `deferred-work.md`).
- Do not change the tie rule, and do not read `VariantRules.TieRule`. Story 12.7 is deferred; Bitola keeps borrowing the Croatian rule and the rules-reference tie sentence is correct as written.
- Do not design the "face-down pair deals to each seat" animation. Removing the dead trump-phase beat is in scope; inventing new deal choreography is not.
- Do not add a lobby variant filter.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Create Croatian room | `POST` room with `variant: "croatia"` | 201; room persists `croatia` | Unknown variant still 400 `INVALID_VARIANT` |
| Absent dealer, forced pick | Croatian, round 2, three passes, dealer's timer expires | A suit is auto-named for the dealer; bidding resolves; hand advances | Never emits `pass_trump` here, so `ErrInvalidBid` is never provoked |
| Bot dealer, forced pick | Same, dealer is a bot | Bot returns `pick_trump` with a suit | No rejected-action reschedule loop, no repeated rejection log |
| Bot bids Croatian round 2 | Round 1 passed out, face-down revealed | Bot evaluates all **eight** cards it knows | N/A |
| Bot bids Croatian round 1 | Six open cards, no candidate | Free-suit `pick_trump` **with** a suit, or `pass_trump` | N/A |
| Human dealer, forced pick | Croatian, round 2, three passes, dealer prompted | Prompt offers suits only — no Pass control; sibling seats' waiting copy does not promise a pass | Server still refuses a forged pass |
| Opponent card count during Croatian bidding | Opponent holds 6 open + 2 face-down | Their stack reads **8** | Face-down card identities still absent from the snapshot |
| Deal animation, no candidate | Croatian deal completes | No dead pause where the trump flip would be | N/A |
| Croatian room in lobby / room page / history | Any locale | Localized variant name in all four locales | No title-cased raw `"croatia"` anywhere |
| Quick Play queue | Croatian rooms exist | Never matched into one | N/A |

</frozen-after-approval>

## Code Map

**The gate and the tripwire**

- `server/internal/room/handler.go:27` `validVariants = map[string]bool{"bitola": true}` — **the gate**. `CreateRoom` at `:473-478` defaults `""`→`bitola` and rejects otherwise with `apperr.ErrInvalidVariant` (`apperr/errors.go:88`); persists at `:627`. DTO `:135` has no `IsQuickPlay` field, so clients cannot forge quick-play rooms.
- `server/internal/room/variant_allowlist_test.go:20` `TestValidVariantsIsBitolaOnly` — the tripwire, whose own doc comment (`:17-19`) names this story as the only one allowed to grow the map. Three assertions to invert: `:25`, `:31`, `:34`.
- Read-only evidence: `room/model.go:23` and `match/model.go:36` are plain `varchar(20)`; `migrations/000003_create_rooms.up.sql:6` and `000005_create_matches.up.sql:11` carry **no CHECK and no enum**. There is no `AutoMigrate` in the repo. **No migration is needed.**
- `room/handler_test.go:742` `TestCreateRoom_InvalidVariant` uses `"unknown"`, not `croatia` — it survives unchanged; add a positive `croatia` case beside it.

**Forced dealer pick — the two deadlocks**

- `server/internal/game/bidding.go:42-55` `handlePassTrump` returns `apperr.ErrInvalidBid` when `Rules.AllPassOutcome == AllPassDealerMustPick && BiddingRound == 2 && BiddingPassCount >= 3`. Pinned by `TestCroatianDealerCannotPassInRound2`. This rejection is correct and stays.
- `server/internal/match/live_match.go:1518-1538` the `PhaseBidding` auto-action is unconditionally `ActionPassTrump`, with the repo's only `TODO(croatian-enablement)` at `:1522`. **The TODO's reasoning is wrong** and must be corrected, not just deleted: the error path at `:1611-1618` calls `startTimerLocked` (`:1429-1435` → `armTurnTimerLocked` `:1400-1417`), which arms a **full fresh** window. There is no `time.Until` anywhere in `handleTimerExpiry`; the `time.Until(*oldState.TurnExpiresAt)` re-arm it cites is in `HandleAction`'s error path at `:424`, clamped by `max(remaining,0)+expiryGrace` (400ms, `:1398`). Real severity: a reject/re-arm loop at one timer period, forever, one `slog.Error` per cycle — the hand never advances, but no core is burned.
- `server/internal/game/auto_play.go:33-49` `AutoPlay` is the model to mirror — pure, deterministic, sorted by the package-level `suitOrder` (`:10-15`). A sibling trump-picker belongs here.
- **Trap:** at round 2 the revealed cards live in `Players[i].FaceDownCards`, **not** `Hand` — `mergeFaceDownCards` only runs later, at `handlePickTrump` (`bidding.go:167`). Any auto-pick that reads `Hand` alone chooses from six of eight cards.
- `server/internal/bot/bot.go:143-197` `decideBid` — **already nil-candidate safe**: `bidHand` at `:150-153` adds the candidate only when non-nil, the candidate branch `:155-163` is gated on non-nil, and the free-suit branch `:178-196` returns `pick_trump` **with** a suit. Croatian bidding works in both rounds today. Its only gap is `:196`, the unconditional `pass_trump` fallback, flagged out-of-scope at `:174-177`.
- `server/internal/match/bot_driver.go:200-211` logs `"bot: action rejected by engine; rescheduling seat"` and re-arms through `maybeScheduleBotAction` at the `botDelayMin` 1s floor (`live_match.go:163`) — a bounded-cadence livelock on the same rejected pass.

**Bot view — bidding blind on six of eight**

- `server/internal/bot/view.go:13-53` `View` carries `Hand` but **no** `FaceDownCards`, no `FaceDownRevealed`, and no rule-config field at all.
- `server/internal/match/bot_driver.go:219-250` `buildBotView` populates from `gs.Players[seat].Hand` alone. `LegalCards` is filled only under `PhasePlaying` (`:226`).
- `server/internal/bot/bot_test.go:17-49` `viewFromState` is a hand-written mirror of `buildBotView` — every `View` change lands in both.
- Do **not** merge the face-down cards into `Hand`: `bot.go:261` and `:1429` use `len(v.Hand) == 2` as the endgame marker.
- Stale comments claiming the view already reads state directly: `server/internal/match/face_down_reveal_test.go:120-123` and `:163-164`.
- Stale Bitola-shaped comments: `bot.go:146-149` ("the 5 dealt cards PLUS the candidate"); thresholds `bot.go:66-73`.

**Client — surfaces that assume a Bitola shape**

- `client/src/features/room/CreateRoomModal.tsx` — `variantOptions:280-287` with **`disabled: true` at `:285`**; the "Only Bitola for now" hint at `:363` (`lobby.createRoomModal.variantHint`); doc comment `:88-89`; state `:118`; reset `:263`; submit `:188-195`; preview label `:733-736`. No zod schema — only name-length checks at `:175-182`. `shared/components/ui/segmented.tsx:44-56` renders testid `variant-segmented-<value>`.
- `client/src/features/lobby/lib/roomLabels.ts:9-12` `variantLabel` maps **only** `bitola` and title-cases anything else → an untranslated "Croatia" in all four locales. Consumers: `lobby/components/RoomCard.tsx:120`, `lobby/components/MatchmakingDiagram.tsx:104`.
- `client/src/features/room/RoomPage.tsx:75-77` `variantKeys` holds **only** `bitola`; `:812` falls back to the raw string; badge `:1194` testid `badge-variant` inside `room-info-badges` (`:1192`).
- `client/src/features/profile/MatchHistory.tsx` renders **no** variant at all, though the server DTO carries it (`server/internal/user/handler.go:154`, populated `:908`) and so does the client type (`shared/api/matches.ts:38`). Slot a chip by the row header date block at `:444-452` or beside the markers at `:495-530`; testid convention `match-history-*` (`match-history-row:431`, `match-history-bots-marker:524`).
- `client/src/features/match/components/TrumpPrompt.tsx` renders Pass unconditionally and has no `canPass` prop; the sibling-seat `waitingRound2` copy still promises "to pick a suit or pass".
- `client/src/features/match/MatchPage.tsx:1607` passes `player.hand.length` as `cardCount` in every phase — reads 6 while a Croatian opponent holds 8. In-match variant label already works: `:1576` → `match.variants.<variant>` → `components/ScorePanel.tsx:300-313`.
- `client/src/features/match/components/DealAnimation.tsx` self-hides on `dealPhase === "done" && !trumpCandidate` but still spends `MOTION.DEAL_PHASE_TRUMP` with nothing on screen.
- `client/src/features/match/lib/variantRules.ts:12-19` holds only `declarationOverlap` — leave it alone; nothing here needs a new client-side rule fact.

**i18n**

- `client/src/shared/i18n/{en,mk,hr,sr}.json`. Already present in all four: `lobby.createRoomModal.variantBitola` (line 619), `variantCroatia` (644 — en "Croatian", mk „Хрватска", hr/sr „Hrvatska"), `match.variants.bitola` / `.croatia` (905/906), `lobby.card.variantBitola` (737).
- **Missing in all four: `lobby.card.variantCroatia`** — the gap behind both title-case fallbacks. `lobby.createRoomModal.variantHint` (656) must be rewritten or removed. Parity gates: `shared/i18n/i18n.test.ts:124` and `i18n.parity.test.ts`.

**Quick Play — already Bitola-only by construction**

- `room/handler.go:3694` `QuickPlay`, `:3893` `QuickJoin`, room synthesis `:3763-3790` with **`Variant: "bitola"` hardcoded at `:3767`**; `IsQuickPlay` set true in exactly one place, `:3775`.
- `room/gorm_repo.go:226-246` `FindQuickPlayRoomExcluding`, filter `:233` — `is_quick_play AND status AND player_count < 4 AND coin_buy_in`, **no variant predicate**. Since no Croatian room can ever carry `is_quick_play = true`, this is already safe; the limit needs a test and a comment, and a variant predicate only as defence in depth.

**Tests**

- `server/internal/bot/simulation_test.go:235-297` already runs a full **Croatian** hand with four `bot.Decide` seats — but `croatianBotDriver` (`:306-320`) **cheats the forced pick**, substituting `AllSuits[0]` when `mustPickTrump` (`:326-331`) says the pass would be rejected. **Deleting that substitution is the acceptance test.** Bitola sim at `:24-71`.
- `server/internal/bot/bot_test.go:69-274` `TestDecide_Bidding` is entirely candidate-based (`NewGameJustDealt` always sets candidate `AH`, `:75`); **no nil-candidate case exists**, and `:265-267` asserts a round-1 pick carries no suit — true only for the candidate variant.
- Fixtures: `testfixtures/fixtures.go:651` `NewGameCroatianJustDealt`, `:746` `NewGameCroatianMidBidding(passCount)` — `(7)` is the forced-pick state.
- Declaration phase needs nothing: `bot_driver.go:100-107`, `bot.go:25-46`, and tests `bot_test.go:294-323`, `simulation_test.go:235-297`, `match/declaration_phase_test.go:514-540` all landed with 12.6.
- `client/.../CreateRoomModal.test.tsx:67` only asserts the control exists; `:100,121,145,164,188,284` assert `variant: "bitola"` payloads. **No client test asserts the option is disabled** — nothing to flip, but an enabled-option test is needed.

## Tasks & Acceptance

**Execution:**

- [x] `server/internal/game/auto_play.go` -- add a pure deterministic trump picker for the active seat, mirroring `AutoPlay`: longest suit across the seat's **`Hand` plus `FaceDownCards`**, ties broken by the existing `suitOrder` -- reading `Hand` alone would pick from six of eight cards at round 2, and a pure engine helper keeps the policy testable and out of the session manager.
- [x] `server/internal/game/auto_play_test.go` -- table-driven cases for the picker, including a Croatian round-2 state where the face-down pair changes the answer -- that is the whole reason the helper is not a one-liner.
- [x] `server/internal/match/live_match.go` -- in the `PhaseBidding` auto-action arm, when `Rules.AllPassOutcome == AllPassDealerMustPick` and this pass would be round 2's fourth, emit `pick_trump` with the auto-picked suit instead of `pass_trump`; **rewrite** the `TODO(croatian-enablement)` comment's factually wrong hot-spin reasoning as part of the fix -- the current path rejects forever and the comment misdirects the next reader.
- [x] `server/internal/bot/view.go` + `server/internal/match/bot_driver.go` -- add the seat's revealed face-down cards as their own `View` field (never merged into `Hand`, which is the endgame marker) and a boolean saying a pass is forbidden this turn, computed in `buildBotView` from the rule config and the bidding counters -- the bot must bid on all eight cards it knows and must never offer an action the engine will reject, without ever learning a variant name.
- [x] `server/internal/bot/bot.go` -- include the face-down cards in the bid hand, and return `pick_trump` with the best-scoring suit when passing is forbidden; correct the stale "5 dealt cards PLUS the candidate" comment -- `:196` is currently the only fallback and it livelocks the hand.
- [x] `server/internal/bot/bot_test.go` -- mirror the `View` change in `viewFromState`; add nil-candidate round-1 and round-2 rows on the Croatian fixtures asserting a suit **is** carried, a forced-dealer row asserting `pass_trump` is never returned, and scope the existing "round-1 pick carries no suit" assertion to the candidate variant -- that assertion encodes a Bitola-only rule as universal and would block the first Croatian row.
- [x] `server/internal/bot/simulation_test.go` -- delete `croatianBotDriver`'s forced-pick substitution and its `mustPickTrump` helper so the Croatian hand runs on `bot.Decide` alone -- this is the acceptance test for the two deadlocks.
- [x] `server/internal/match/bot_driver_test.go` -- a bot dealer at three round-2 passes advances the hand with no rejected-action reschedule -- the livelock is in the driver, not the engine, so an engine-level test cannot see it.
- [x] `server/internal/match/live_match_internal_test.go` -- a Croatian dealer's expired round-2 timer resolves bidding instead of looping -- covers the timeout path independently of bots.
- [x] `server/internal/match/face_down_reveal_test.go` -- correct the two stale comments claiming `buildBotView` reads the face-down state directly -- they assert something that only becomes true in this story.
- [x] `server/internal/game/state.go` -- add a per-seat **count** of face-down cards to `PlayerState`, JSON-tagged -- opponents' stacks read 6 while a Croatian player holds 8, and a count leaks nothing while the card identities stay out of the snapshot.
- [x] `server/internal/ws/testdata/events/event_match_state.json` + `client/src/shared/types/wsEvents.schemas.ts` -- regenerate the golden with `UPDATE_GOLDENS=1` and add the field to the `PlayerStateSchema` strictObject -- strict parsing rejects unknown keys, so a missing schema entry breaks every `match_state`.
- [x] `client/src/shared/types/matchTypes.ts` + `client/src/features/match/MatchPage.tsx` -- carry the new count and render each seat's stack as hand length plus face-down count -- server-authoritative, so no client-side variant branch is introduced.
- [x] `server/internal/room/handler.go` + `server/internal/room/variant_allowlist_test.go` -- add `croatia` to `validVariants` and invert the tripwire's three assertions into "both variants are creatable" -- the test's own comment names this story as its owner.
- [x] `server/internal/room/handler_test.go` -- add a positive Croatian create case beside the existing invalid-variant test -- the latter uses `"unknown"` and does not cover the new value.
- [x] `server/internal/room/gorm_repo.go` + a quick-play test -- add a Bitola predicate to the quick-play room lookup with a comment saying it is defence in depth, and assert a Croatian room is never matched -- Bitola-only is currently true by construction only, which no test states.
- [x] `client/src/features/room/CreateRoomModal.tsx` -- drop `disabled` from the Croatian option, update the stale doc comment, and replace the "Only Bitola for now" hint -- the hint becomes false the moment the option works.
- [x] `client/src/features/lobby/lib/roomLabels.ts` + `client/src/features/room/RoomPage.tsx` -- map `croatia` in both, so neither falls through to title-casing -- these are the two surfaces that would render an untranslated "Croatia".
- [x] `client/src/features/profile/MatchHistory.tsx` -- render the variant on each match row -- the AC requires it in history and it is the one listed surface showing nothing today, though both the DTO and the client type already carry it.
- [x] `client/src/features/match/components/TrumpPrompt.tsx` -- accept a "may pass" prop, hide the Pass control for the forced dealer, and fix the sibling-seat waiting copy that promises a pass -- offering a control the server refuses is the defect; a new error sentinel is explicitly not the fix.
- [x] `client/src/features/match/components/DealAnimation.tsx` -- skip the trump-flip beat when there is no candidate -- every Croatian deal currently spends that duration on an empty table centre. Do not design new face-down choreography.
- [x] `client/src/shared/i18n/{en,mk,hr,sr}.json` -- add `lobby.card.variantCroatia` and revise the create-room variant hint, in all four locales -- the missing key is the root cause of both title-case fallbacks, and parity is enforced.
- [x] `client/.../CreateRoomModal.test.tsx`, `RoomCard`/`RoomPage`/`MatchHistory`/`TrumpPrompt` tests -- assert the Croatian option is selectable and submits `croatia`, the localized label renders on all three surfaces, and the forced dealer sees no Pass control -- none of these have coverage today.

**Acceptance Criteria:**

- Given `server/internal/game`, `server/internal/bot` and `server/internal/match/bot_driver.go`, when searched for a variant-name comparison, then the only hits are `RulesFor` and test `NewGame` arguments.
- Given the Croatian bot simulation with its forced-pick substitution removed, when a full hand runs `-count=5`, then bidding resolves, the declaration phase completes, eight tricks play and the hand scores — with no stall, no rejected-action log, and no panic.
- Given a real four-player Croatian match driven through the WS harness, when round 1 is passed out and the dealer neither picks nor is a bot, then the hand still advances and the server logs no repeated rejection.
- Given the full pre-existing suite, when it runs, then every Bitola assertion passes unchanged.
- Given a Croatian room, when it renders in the lobby, the room page, the in-match HUD and match history, then each shows a localized variant name in all four locales and none shows a raw or title-cased `"croatia"`.
- Given Quick Play with Croatian rooms present, when a player queues repeatedly, then they only ever land in Bitola rooms.
- Given the four locale files, when the parity tests run, then key sets are identical.
- Given `client/src/features/rules/` and `server/internal/game/scoring.go`, when this story is complete, then neither is modified.

## Spec Change Log

**2026-08-20 — one added wire field: `GameState.MustPickTrump` (implementation).**
The Tasks list gave TrumpPrompt a "may pass" prop but named no source for it. The
only source available without a new field was a client-side inference — "no trump
candidate during round 2 plus three passes plus the active seat is the dealer" —
which silently couples two independent rule facts (candidate-absence and
all-pass-outcome) and would rot the moment a variant paired no candidate with a
reshuffle. Since the story already widens the wire for `faceDownCount` and
already regenerates the golden, the boolean rides along instead: `MustPickTrump`
is derived at ApplyAction's single exit from the engine's own exported predicate,
carried on `match_state`, and read by the prompt as `canPass={!mustPickTrump}`.
No client-side variant branch is introduced and `variantRules.ts` is untouched,
per the Code Map. Landed with the golden, the Zod `strictObject`, and
`matchTypes.ts` in the same change; covered by
`TestMustPickTrumpWireFlag` (game) and the TrumpPrompt `canPass` suite (client).

**2026-08-20 — second added wire value: `ws.AutoActionPickTrump` (review round 1).**
`autoActionTypeFor` had no `ActionPickTrump` case, so the forced auto-pick was the
only timer auto-action that fired no `event:auto_action` — the one auto-action
that fixes trump for a whole hand was silent to all four seats. Added
`AutoActionPickTrump` to `internal/ws`, mapped it, widened the client allowlist
and the Zod union, and added `match.timer.autoPickedTrump` in all four locales
(the other three auto-actions DECLINE something; this one commits the hand, so it
gets its own copy). Pinned by `auto_action_pick_trump.json` on both sides of the
contract gate, by the `event:auto_action` assertions in
`TestHandleTimerExpiry_ForcedDealerPickResolvesBidding`, and by an i18n-key spy in
`useWsDispatch.test.ts`. The pre-existing `TestAutoActionTypeFor` row asserting
"pick_trump is not a timeout action" was inverted — it encoded "no config can
force a pick", which is precisely what this story changes.

**2026-08-20 — seat parameters on `MustPickTrump` and `AutoPickTrumpSuit` (review round 1).**
Both described the ACTIVE seat while taking none, so every caller had to remember
to scope them and `AutoPickTrumpSuit` could pick for a different seat than the
action was stamped for. Both now take the seat explicitly.

## Design Notes

**Two deadlocks, one policy.** The absent-human path and the bot path fail for the same reason — something emits `pass_trump` where the engine allows only `pick_trump` — so they want one shared answer, not two. Putting the picker in `auto_play.go` beside `AutoPlay` gives the session manager a pure function to call and gives the bot a scoring path it already has; what neither may do is learn that "croatia" means anything. The bot decides from a config-derived boolean handed to it in its view, exactly as it already receives a nil candidate rather than a variant name.

**Why a count and not the cards.** Showing an opponent's stack as 6 when they hold 8 is the kind of error a Belot player spots instantly, but fixing it by shipping the face-down cards would break the one rule this epic guards hardest. A per-seat integer is the whole fix: the client renders `hand.length + faceDownCount` and still cannot name a single hidden card. It does move `event_match_state.json`, which is expected and is what the golden regeneration is for — unlike Story 12.6, this story deliberately widens the wire.

**The bot's bar is left alone on purpose.** `wantsTrump` was calibrated against a six-card bid hand. Croatian round 1 is coincidentally also six, so only round 2 changes, and it changes from six cards to the eight the bot actually holds — strictly more information against an unchanged threshold. That will make bots somewhat keener to take trump in Croatian round 2. Correcting the *input* is this story's job; re-deriving the *threshold* is a balance change that deserves its own decision, so it is on the Ask First list rather than guessed at here.

## Verification

**Commands:**

- `cd server && go test ./...` -- expected: all pass.
- `cd server && go test ./internal/bot/... -run Simulation -count=5` -- expected: stable; no `unexpected phase`, no rejection log, no panic.
- `cd server && UPDATE_GOLDENS=1 go test ./internal/ws/... && go test ./internal/ws/...` -- expected: regenerates, then passes.
- `cd server && git diff --stat HEAD -- internal/game/scoring.go` -- expected: empty.
- `cd server && golangci-lint run ./... && gofmt -l .` -- expected: clean.
- `cd client && npx vitest run` -- expected: all pass, including `wsEvents.contract.test.ts`, `i18n.parity.test.ts`, `i18n.test.ts`.
- `cd client && npx tsc -p tsconfig.build.json --noEmit` -- expected: clean. Not run in CI — run it manually.
- `cd client && npx eslint . && npx prettier --check .` -- expected: clean.
- `cd client && git diff --stat HEAD -- src/features/rules/ src/features/match/lib/variantRules.ts` -- expected: empty.

**Live match (required, not optional).** Start local services with `make dev` (Docker Postgres + Vite on 5173 + Go on 8080; if startup fails, check for orphaned processes on those ports first).

Drive a real four-player Croatian match per the project's WS bot harness — register, create with `variant: "croatia"`, join and seat four clients, authenticate over `ws`, match server errors on the `error:` type prefix, and send `action:continue` at `hand_complete`. Force a round-1 pass-out so the run exercises the round-2 reveal **and** the forced dealer pick, including one run where the dealer simply never acts. Expected: bidding resolves, the declaration phase completes, the hand scores, and the server log carries no repeated rejection.

A Playwright MCP browser is available and is the better tool for the player-facing rows of the matrix — seat one human client in it and confirm, in the real UI: the create-room Croatian option is selectable, the lobby card / room page / match-history rows all show a localized variant name (check at least one non-English locale), the forced dealer's prompt offers suits with **no** Pass control, and each opponent's stack reads 8 during Croatian bidding. Take a screenshot of the Croatian bidding table and the forced-dealer prompt as evidence.

**Manual checks (if no CLI):**

- Grep the bot and match packages for the variant constants -- expected: none outside `RulesFor` and test `NewGame` calls.
- Grep the four locale files for a `lobby.card.variantCroatia` value -- expected: present and localized in all four.

## Suggested Review Order

**Start here — the rule the whole story turns on**

- One seat-scoped predicate; the engine, the bot and the client cannot disagree.
  [`bidding.go:60`](../../server/internal/game/bidding.go#L60)

**The forced dealer pick — both deadlocks**

- Placed before the general bidding arm, so the rejected pass is never emitted.
  [`live_match.go:1519`](../../server/internal/match/live_match.go#L1519)

- Pure picker mirroring AutoPlay; counts the face-down pair, not just Hand.
  [`auto_play.go:82`](../../server/internal/game/auto_play.go#L82)

- Bot names its best suit rather than passing; kills the driver livelock.
  [`bot.go:192`](../../server/internal/bot/bot.go#L192)

- Without this the one auto-action that names trump was silent to everyone.
  [`events.go:225`](../../server/internal/ws/events.go#L225)

- Maps it, so the toast reaches all four seats like every sibling auto-action.
  [`live_match.go:1824`](../../server/internal/match/live_match.go#L1824)

**The wire — this story deliberately widens it**

- A count, never the cards: fixes stacks reading 6 while a seat holds 8.
  [`state.go:58`](../../server/internal/game/state.go#L58)

- Derived at two real call sites; reshuffle and hand reset covered via dealCards.
  [`state.go:72`](../../server/internal/game/state.go#L72)

- Refreshed at ApplyAction's single exit, so a new handler cannot forget it.
  [`rules_engine.go:18`](../../server/internal/game/rules_engine.go#L18)

- Both keys added to the strictObject in the same change, or snapshots hard-fail.
  [`wsEvents.schemas.ts:87`](../../client/src/shared/types/wsEvents.schemas.ts#L87)

**Bot bids on the hand it actually holds**

- Own field, gated on the reveal; kept out of Hand, which is the playable set.
  [`view.go:33`](../../server/internal/bot/view.go#L33)

- Populated per seat, so round 2 sees eight cards instead of six.
  [`bot_driver.go:249`](../../server/internal/match/bot_driver.go#L249)

**The gate**

- The allowlist is the only thing that made Croatian unselectable.
  [`handler.go:31`](../../server/internal/room/handler.go#L31)

- Quick Play stays Bitola-only; the predicate is documented defence in depth.
  [`gorm_repo.go:235`](../../server/internal/room/gorm_repo.go#L235)

**Client surfaces**

- The prop defaults to true, so this one line is the whole forced-dealer UI fix.
  [`MatchPage.tsx:2042`](../../client/src/features/match/MatchPage.tsx#L2042)

- Nullish guard: the live path casts rather than parses, so absence meant NaN.
  [`MatchPage.tsx:1666`](../../client/src/features/match/MatchPage.tsx#L1666)

- Announced, not merely visual — a vanishing button is invisible to a reader.
  [`TrumpPrompt.tsx:134`](../../client/src/features/match/components/TrumpPrompt.tsx#L134)

- Was the title-case fallback that rendered an untranslated "Croatia".
  [`roomLabels.ts:11`](../../client/src/features/lobby/lib/roomLabels.ts#L11)

- Same gap on the room page, via a map that held only bitola.
  [`RoomPage.tsx:77`](../../client/src/features/room/RoomPage.tsx#L77)

- History showed no variant at all, though the DTO always carried it.
  [`MatchHistory.tsx:461`](../../client/src/features/profile/MatchHistory.tsx#L461)

- Option un-disabled; the "Only Bitola for now" hint became false on ship.
  [`CreateRoomModal.tsx:280`](../../client/src/features/room/CreateRoomModal.tsx#L280)

- Skips the dead trump-flip beat when the variant flips no candidate.
  [`DealAnimation.tsx:51`](../../client/src/features/match/components/DealAnimation.tsx#L51)

**Peripherals — the tests that make the fix hold**

- The Bitola-only tripwire, inverted by the story it named as its owner.
  [`variant_allowlist_test.go:22`](../../server/internal/room/variant_allowlist_test.go#L22)

- Deterministic tail hand replaced a probabilistic assertion that flaked ~1 in 3,500.
  [`simulation_test.go:336`](../../server/internal/bot/simulation_test.go#L336)

- Pins the canPass wiring; deleting the prop previously left 620 tests green.
  [`MatchPage.test.tsx:1861`](../../client/src/features/match/MatchPage.test.tsx#L1861)

- Pins the count at the deal; removing the sync previously broke nothing.
  [`state_test.go:455`](../../server/internal/game/state_test.go#L455)

- Asserts a Croatian room actually starts a Croatian match — the headline behaviour.
  [`handler_test.go:2483`](../../server/internal/room/handler_test.go#L2483)
