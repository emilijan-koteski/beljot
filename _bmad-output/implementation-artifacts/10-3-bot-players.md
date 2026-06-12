---
baseline_commit: a9999fc2a39617bdcb29a8028642bc9388d93ca5
---

# Story 10.3: Bot Players

Status: done

<!-- Added 2026-06-11 via bmad-correct-course (new FR59) — see
     _bmad-output/planning-artifacts/sprint-change-proposal-2026-06-11.md (Change 2).
     Largest Phase 2 story. If dev finds it oversized mid-flight, shard per the epics.md
     note (10.3 foundation: seating + legal play + timing; 10.4 strategy hardening) —
     do NOT thin acceptance criteria. -->

## Story

As a room owner,
I want to seat bots on the empty seats of my room,
so that a match can start and be played even when fewer than four humans are available.

## Acceptance Criteria

1. **Seating.** Given a room in `waiting` status with at least one empty seat, when the room owner adds a bot to an empty seat, then the bot occupies that seat with a distinct, localized bot identity (name + bot badge) visible to everyone in the room lobby; the owner can seat 1, 2, or 3 bots (any combination of free seats); the owner can remove a seated bot while the room is still in `waiting`; the owner can swap seats between any two occupants — human ↔ bot included — or move a bot to an empty seat, while the room is still in `waiting`; add/remove/swap from non-owners is rejected server-side; bot seating works in both 1001 and 501 match modes.
2. **Normal flow.** Given all four seats filled by any mix of humans and bots, when the owner starts the game, then the match proceeds through the normal flow (dealing, bidding, declarations, card play, scoring, match end) with bots acting autonomously on their seats.
3. **Humanized legal actions.** Given it is a bot's turn at any decision point (trump bidding, declarations, Belote/Rebelote announcement, card play), the action is produced server-side and validated by the same rules-engine path as human actions (always legal), lands after a randomized think delay of roughly 1–2.5 s (never instantaneous), and the delay always resolves within the per-move timer so a bot never triggers timeout auto-play.
4. **Competent strategy.** Bots bid from hand-strength evaluation (trump length and honors), play cards with trump management, remember played cards, support their partner, and maximize points won or denied; they announce declarations and Belote/Rebelote whenever they score points; automated simulation tests show the heuristic is measurably stronger than a random-legal-move baseline.
5. **Persistence + marking.** Matches with at least one bot are flagged bot-inclusive in the database; match previews and match history visibly mark the game as played with bots; bot seats are recorded with their bot identity WITHOUT creating user accounts; bots accrue no XP, coins, honor, or stats.
6. **Resilience isolation.** Bots never pause, disconnect, or abandon; human reconnect/pause flows are unaffected; bots are never used to replace a disconnected human — the existing reconnect/abandon flow is unchanged (architecture constraint #6 as amended 2026-06-11).

## Tasks / Subtasks

- [x] Task 1 — Migration 000007: bot seats + bot-inclusive matches (AC: 1, 5)
  - [x] 1.1 Highest existing migration is `000006_create_hand_results` — create `server/migrations/000007_add_bot_players.up.sql` / `.down.sql` (never skip numbers; down must fully reverse).
  - [x] 1.2 Up: `CREATE TABLE room_bots (id SERIAL PRIMARY KEY, room_id INTEGER NOT NULL REFERENCES rooms(id) ON DELETE CASCADE, seat INTEGER NOT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW());` + `CREATE UNIQUE INDEX idx_room_bots_room_seat ON room_bots(room_id, seat);` + `CREATE INDEX idx_room_bots_room_id ON room_bots(room_id);`
  - [x] 1.3 Up (matches): `ALTER TABLE matches ALTER COLUMN player1_id DROP NOT NULL` (×4 for player1–4_id — FKs still validate non-NULL values); `ADD COLUMN player1_is_bot BOOLEAN NOT NULL DEFAULT FALSE` (×4); `ADD COLUMN has_bots BOOLEAN NOT NULL DEFAULT FALSE`; `CREATE INDEX idx_matches_has_bots ON matches(has_bots);`
  - [x] 1.4 Down: drop `room_bots`; on matches — `DELETE FROM matches WHERE has_bots = TRUE` first (their NULL player IDs block NOT NULL restore; hand_results cascade), then drop the five new columns and restore `NOT NULL` on player1–4_id. Verified: up → down → up all clean against dev DB (port 5433).
- [x] Task 2 — Server room domain: bot seating (AC: 1)
  - [x] 2.1 `server/internal/room/model.go`: add `RoomBot{ID uint, RoomID uint, Seat int, CreatedAt time.Time}` GORM model (json camelCase tags). Add `IsBot bool \`gorm:"-" json:"isBot"\`` to `RoomPlayer` — bots are NOT room_players rows (FK to users forbids it); they are merged into the wire `players` array as synthetic entries `{id:0, roomId, userId:0, username:"", seat, team, isBot:true}`. Humans always serialize `isBot:false`.
  - [x] 2.2 `server/internal/room/repository.go` + `gorm_repo.go`: add `AddBot(roomID uint, seat int) error`, `RemoveBot(roomID uint, seat int) error` (return rows-affected so handler can 404), `FindBotsByRoomID(roomID uint) ([]RoomBot, error)`, `FindBotsByRoomIDs(roomIDs []uint) (map[uint][]RoomBot, error)` (for lobby previews, mirrors `FindPlayersByRoomIDs` at gorm_repo.go:~240).
  - [x] 2.3 `server/internal/room/handler.go`: new endpoints `POST /rooms/:id/bots` (body `{"seat": n}`) and `DELETE /rooms/:id/bots/:seat`. Validation order mirrors KickPlayer (handler.go:~1019): room exists → `room.OwnerID == userID` else `apperr.ErrNotRoomOwner` → `room.Status == "waiting"` → `!room.IsQuickPlay` (new `apperr.ErrBotsNotAllowed`) → seat in 0–3 else `ErrInvalidSeat` → seat empty (no human via `FindPlayerBySeat` AND no bot) else `ErrSeatTaken`. Remove: bot exists on seat else new `apperr.ErrNoBotOnSeat`. Run inside `repo.RunInTransaction` with `FindByIDForUpdate` (the story-8.5-1 row-lock pattern, handler.go:545/921) to serialize against concurrent seat takes and auto-start.
  - [x] 2.4 Broadcast after commit: `system:bot_added` `{roomId, seat, team}` / `system:bot_removed` `{roomId, seat}` to room participants (`broadcastToRoom`), then `broadcastRoomSeatSnapshot` (handler.go:183) so lobby browsers get the refreshed `system:room_updated`. Team derives from seat parity like humans (handler.go:642-647).
  - [x] 2.5 Merge bots into EVERY payload path that builds a players array: `roomLifecyclePayload` (handler.go:136-171 — `system:room_created`/`system:room_updated`), `GetRoom` (REST RoomDetail), `GetRoomByCode`, `ListRooms` previews (via `FindBotsByRoomIDs`). Write one helper (e.g. `mergeBotPlayers(players []RoomPlayer, bots []RoomBot) []RoomPlayer`) and use it everywhere — do not hand-merge at each site.
  - [x] 2.6 ⚠️ Regression guard — bot seats must read as OCCUPIED in `SelectSeat` (handler.go:~686): it currently checks only `FindPlayerBySeat`, so a human could silently take a bot's seat. Add the bot-seat check → `ErrSeatTaken`. (Owner seat rearrangement goes through SwapSeats, which DOES support bots — 2.7.)
  - [x] 2.7 `SwapSeats` (handler.go:~1249) gains bot awareness — the owner can rearrange any occupant combination inside the same `RunInTransaction` + `FindByIDForUpdate` envelope: human ↔ human (existing, untouched); human ↔ bot (update the human's `room_players` seat/team AND the bot's `room_bots.seat` atomically); bot → empty seat (move the `room_bots` row); bot ↔ bot (observable no-op — bot identity is seat-derived, so return success without state change or broadcasts). Teams stay seat-parity-derived for both kinds of occupant. Broadcasts for bot-involved swaps: `system:seat_updated` for each HUMAN participant (existing payload shape with `previousSeat`) plus `system:bot_removed` `{roomId, seat: old}` + `system:bot_added` `{roomId, seat: new, team}` for the bot, then the lobby seat snapshot — the client store already handles each event individually (Task 8.2/8.3), so no new event types are needed.
  - [x] 2.8 Start validation: `StartMatch` (handler.go:1496-1503) and `autoStartIfFull` (handler.go:932-939) currently gate on `seatedCount >= 4` humans. Change StartMatch to: every seat 0–3 covered by exactly one human or bot (quickplay rooms can't have bots, so autoStartIfFull only needs the defensive read). Build `seatInfo [4]match.PlayerSeatInfo` (handler.go:1527-1534) with `{IsBot: true, Seat: i}` for bot seats (UserID 0, Username "").
  - [x] 2.9 `server/internal/apperr/errors.go`: add `ErrBotsNotAllowed` ("BOTS_NOT_ALLOWED", 409 or 400) and `ErrNoBotOnSeat` ("NO_BOT_ON_SEAT", 404). Reuse `ErrNotRoomOwner`, `ErrSeatTaken`, `ErrInvalidSeat` — check the room block (errors.go:55-71) before adding anything else.
  - [x] 2.10 Room-lifecycle invariants (verified in current code — pin them with tests, no source change expected): `LeaveRoom`'s owner-transfer and empty-room close read `FindPlayersByRoomID` (humans only, handler.go:571-587) — when the sole human leaves a waiting room with bots seated, the room flips to "completed" (bots never keep a room alive) and ownership can never transfer to a bot; `TransferOwnership` targets room_players → bots automatically ineligible; `PlayerCount` stays humans-only (it drives the join gate; bots occupy seats, not capacity — a 1-human + 3-bot room correctly shows 1 player with 4 seats taken).
  - [x] 2.11 Handler tests (`handler_test.go`): owner adds bot → 201 + bot in players payload with `isBot:true`; non-owner → `NOT_ROOM_OWNER`; occupied seat (human, then bot) → `SEAT_TAKEN`; non-waiting room rejected; quickplay room → `BOTS_NOT_ALLOWED`; remove → seat frees; remove empty seat → `NO_BOT_ON_SEAT`; human `SelectSeat` onto bot seat → `SEAT_TAKEN`; owner swaps human ↔ bot → both records move, teams follow parity, correct event sequence (`seat_updated` + `bot_removed`/`bot_added`); owner moves bot → empty seat; bot ↔ bot swap → success, no state change, no broadcasts; non-owner swap involving a bot still `NOT_ROOM_OWNER`; start with 1/2/3 bots succeeds and passes correct `seatInfo`; start with 3 humans + 1 empty still fails; sole human leaves waiting room with 3 bots → room "completed".
- [x] Task 3 — WS event contract, BOTH files same commit (AC: 1)
  - [x] 3.1 `server/internal/ws/events.go`: add `SystemBotAdded = "system:bot_added"`, `SystemBotRemoved = "system:bot_removed"` next to the room-lobby block (events.go:204-222).
  - [x] 3.2 `client/src/shared/types/wsEvents.ts`: add the two event constants + `BotAddedPayload {roomId, seat, team}` / `BotRemovedPayload {roomId, seat}` interfaces beside `SeatUpdatedPayload` (wsEvents.ts:~321).
  - [x] 3.3 `client/src/shared/types/apiTypes.ts:59-67`: `RoomPlayer` gains `isBot: boolean`.
  - [x] 3.4 Match snapshot: `PlayerState` gains `isBot` — `client/src/features/match/types/matchTypes.ts` AND the strict zod players schema in `client/src/shared/types/wsEvents.schemas.ts` (it is `.strict()` — server emitting `isBot` without the schema entry hard-fails every snapshot; this MUST land in the same commit as the Go field, Task 4.2).
- [x] Task 4 — Session manager: bot actors (AC: 2, 3, 6)
  - [x] 4.1 `server/internal/match/live_match.go`: `PlayerSeatInfo` gains `IsBot bool`. `StartMatch` (line 92): skip `m.userToRoom[p.UserID] = roomID` for bot seats (UserID 0 must NEVER enter that map — `HandleDisconnect` and action routing key on it); when broadcasting, filter `playerIDs` to non-zero (hub tolerates unknown IDs but don't rely on it).
  - [x] 4.2 `server/internal/game/state.go`: `PlayerState` (lines 10-18) gains `IsBot bool \`json:"isBot"\``; set from seat info in `NewGame` (state.go:183-187 — extend its per-seat inputs); bots get `Connected: true` permanently so no disconnect UI ever shows for them. testfixtures default `IsBot:false` — factories are the single update point; touch `testfixtures/fixtures.go` only if a constructor signature changes.
  - [x] 4.3 New `server/internal/match/bot_driver.go`: `maybeScheduleBotAction(session *LiveMatch)`. Decision-seat resolution from gameState: `PhaseBidding` → `ActivePlayerSeat`; `PhasePlaying` + `PendingBelotSeat != nil` → `*PendingBelotSeat`; `PhasePlaying` + `AwaitingDeclaration` → `ActivePlayerSeat`; `PhasePlaying` → `ActivePlayerSeat`; `PhaseHandComplete` → every bot seat that has not yet acked `continue` (~1 s delay each; humans + the existing 14 s fallback still pace the reveal); surrender pending (`SurrenderProposerSeat != nil`) and the proposer is the bot's PARTNER → the bot accepts (bots never initiate surrender, never respond to opponents' proposals — surrender is team-internal); `PhasePaused`/`PhaseDisconnected`/`PhaseMatchEnd` → never schedule. If the resolved seat is a bot: `time.AfterFunc(randomized delay)` capturing `session.timerGeneration` (the established staleness pattern, live_match.go:1068-1092); on fire take the session lock, re-verify generation + that it is STILL that bot's decision point, build the action, and run it through the SAME apply-and-broadcast path the timer expiry uses (live_match.go:1156-1215 — extract/reuse, do not duplicate; chained-prompt handling at 1233-1299 must keep working).
  - [x] 4.4 Think delay: uniform random 1.0–2.5 s. Make the range injectable on `Manager` (e.g. `botDelayMin/Max time.Duration` defaulted in the constructor) so tests set ~1 ms — without this every manager test with bots sleeps for real. Per-move timer minimum is 10 s (`ErrTimerDurationOutOfRange`, 10–120 s), so 2.5 s always resolves inside it; KEEP the turn timer armed on bot turns — if a bot ever errors, timeout auto-play is the safety net (and a test should pin that the bot acts well before expiry, i.e. `autoPlayed` flag stays false on bot actions... see 4.6).
  - [x] 4.5 Schedule call sites — invoke `maybeScheduleBotAction` after EVERY point the game state is replaced and broadcast: end of `StartMatch` (first bidder may be a bot), end of the success path in `HandleAction` (after `broadcastActionResult`, line ~333), after timer-expiry auto-actions and the chained loop, after unpause/owner-unpause resume, after reconnect resume, after `ForceAdvanceHandComplete` auto-continue, after reshuffle/redeal (all-pass bidding) and new-hand transitions. Missing one call site = a silently stalled match; sweep them all (grep for `broadcastActionResult` + every `gameState =` assignment).
  - [x] 4.6 Bot actions are ordinary `game.Action`s applied via `game.ApplyAction` — they carry NO `autoPlayed` marker (that flag stays reserved for timeout auto-play; AC3 says bots never trigger it). `event:card_played` etc. look identical to human plays.
  - [x] 4.7 Resilience isolation checks: `HandleDisconnect` keys on `userToRoom` — bots absent by 4.1; pause is seat-keyed but only arrives via client actions — bots have no socket, nothing to do; abandonment timers are per-seat reconnect timers started only from `HandleDisconnect` — unreachable for bots. Add manager tests pinning each: human disconnect/reconnect mid bot-match works unchanged; bot keeps acting after a human reconnects; bot seats never get reconnect timers.
  - [x] 4.8 Manager tests (httptest + real WS client per project rule where the socket matters): match with 3 bots progresses Dealing→Bidding→Playing→HandComplete→next hand without any client action except the human's; bots ack `continue`; bot acts within delay bounds and before timer expiry; stale bot timer (generation bump) never double-fires; bot responds to belote prompt; bot partner accepts human surrender.
- [x] Task 5 — `internal/bot` package: heuristic strategy (AC: 3, 4)
  - [x] 5.1 `server/internal/game/validation.go`: export `LegalCards(state *GameState, seat int) []Card` as a thin wrapper over the unexported `legalCards` (line 16-55). Exposure only — zero rule changes; engine tests still go through `ApplyAction`.
  - [x] 5.2 New package `server/internal/bot/` (per sprint-change-proposal §Technical Impact): `view.go` — `View` struct built by the match layer containing ONLY what a human in that seat could know: `Seat, Hand, LegalCards, Phase, BiddingRound, TrumpCandidate, TrumpSuit, TrumpCallerSeat, DealerSeat, CurrentTrick, LeadSuit, ActivePlayerSeat, AwaitingDeclaration, PendingBelot bool, TeamScores, HandPoints, TricksWon, PlayedCards []game.Card, KnownVoids [4][4]bool`. The redacted View makes no-peeking STRUCTURAL — `Decide` never receives other players' hands even though the server state has them. Build it in `bot_driver.go` (`buildBotView(gs, seat, mem)`).
  - [x] 5.3 `bot.go` — `Decide(v View) game.Action`, a pure deterministic function (humanized randomness lives in the match-layer delay, NOT in card choice — deterministic decisions are table-testable):
    - Bidding round 1: accept the candidate suit when holding ≥4 cards of it, or 3 including the J or the 9+A pair; else pass. Round 2: evaluate the three non-candidate suits with the same evaluator; pick the best suit that clears the threshold, else pass. Score with the exported tables `game.TrumpCardPoints` / `TrumpRankOrder` (types.go:150-185) — keep the evaluator's constants in one place for tuning.
    - Declarations: when `AwaitingDeclaration` for own seat → always `ActionDeclare` (engine auto-detects from hand, declarations.go:42-108 — there is nothing to choose). Belote: `PendingBelot` → always `ActionAnnounceBelot` (+20 is unconditional value).
    - Card play, choosing among `LegalCards` only: leading — if own team called trump and trumps remain unseen (memory), lead high trump (J/9) to draw them; else lead the strongest side suit (A first). Following — if partner currently wins the trick (`currentTrickWinnerSeat` logic re-derivable from View), play the highest-point card that does not waste a winner (smear); if an opponent wins and the bot can take it, win as cheaply as possible; otherwise discard the lowest-value card, preserving trump. Memory informs "can anyone still beat this": trumps exhausted / suit voids per `PlayedCards` + `KnownVoids`.
    - Surrender accept (partner proposed): return `ActionSurrenderAccept`.
  - [x] 5.4 `memory.go` — per-match `Memory`: cards seen this hand (every resolved `play_card`), inferred voids (seat didn't follow led suit ⇒ void in it; followed with trump ⇒ void in led suit), reset on each new hand. The session manager updates it on every successful `ActionPlayCard` (old state's `LeadSuit` + played card tell the void story) — `GameState` keeps NO trick history (`CurrentTrick` is cleared on trick resolution, playing.go:139), so this lives on `LiveMatch` (e.g. `botMemory *bot.Memory`, allocated only when the match has bots).
  - [x] 5.5 Unit tests (table-driven, co-located): bidding evaluator cases (strong trump → pick; junk → pass; round-2 best-suit selection); follow-suit smear vs cheap-win vs discard cases; trump-drawing lead; memory void inference. Build states with `testfixtures` factories then derive Views — no raw `GameState{}` literals.
- [x] Task 6 — Simulation proof vs random baseline (AC: 4)
  - [x] 6.1 `server/internal/bot/simulation_test.go`: drive full hands purely through `game.NewGame`/`ApplyAction` (no session manager, no WS): seats 0+2 use `bot.Decide`, seats 1+3 use a random-legal baseline (`game.LegalCards` + seeded `rand.New(rand.NewPCG(...))` — math/rand/v2 has no global Seed; deck shuffles stay nondeterministic, which is fine at this sample size).
  - [x] 6.2 Run ≥200 hands; assert the heuristic team accumulates ≥60% of all points awarded. The margin is deliberately far below the expected true rate so CI never flakes; if the heuristic can't clear 60%, the strategy fails AC4 — tune the heuristic, don't lower the bar. Keep runtime in check (pure-Go hand ≈ instant; cap at a few seconds total).
  - [x] 6.3 Also assert zero `ApplyAction` errors across the entire simulation — this doubles as the "always legal" proof for AC3.
- [x] Task 7 — Match persistence + history API (AC: 5)
  - [x] 7.1 `server/internal/match/model.go` (lines 14-33): `Player1ID`–`Player4ID` become `*uint` (nullable for bot seats — FK forbids fake users); add `Player1IsBot`–`Player4IsBot bool` and `HasBots bool` (all `not null;default:false` GORM tags, camelCase json). Sweep EVERY construction/read of these fields — `handleMatchEnd` record build (live_match.go:930-979), repo queries, and `user/handler.go` history scans — the compiler finds most, but query strings (`player1_id = ?`) need a grep.
  - [x] 7.2 `handleMatchEnd`: populate per-seat IDs (nil for bots), per-seat bot flags from the session's seat info, `HasBots = any`. Persisted via the existing `CreateWithHands` transaction unchanged.
  - [x] 7.3 `server/internal/user/handler.go` history (lines 375-416, `MatchListItem` 71-86): `MatchPlayer` gains `IsBot bool` and a nullable/zero `UserID` for bot seats; `MatchListItem` gains `HasBots bool`; viewer-seat derivation (line ~521) must skip NULL player IDs (the viewer is always human). Username hydration for bot seats: skip the users lookup, leave `Username: ""` — the client renders the localized bot name.
  - [x] 7.4 XP/coins/honor/stats: none of these systems exist yet (Epic 9 follows this story). The per-seat `IsBot` flags + `HasBots` ARE the deliverable — Epic 9 inherits "ignore bot seats" as a clean rule (sprint-change-proposal §2). Do not build placeholder accrual logic.
  - [x] 7.5 Tests: match-end persistence with 2 bots → record has nil IDs + flags + `HasBots`; history endpoint returns `hasBots` + per-player `isBot`; abandoned bot-inclusive match persists the same way.
- [x] Task 8 — Client: room lobby add/remove + bot identity (AC: 1)
  - [x] 8.1 `client/src/shared/api/rooms.ts`: `addBot(roomId, seat)` → `POST /rooms/${roomId}/bots`; `removeBot(roomId, seat)` → `DELETE /rooms/${roomId}/bots/${seat}`. Mutations in `client/src/shared/hooks/mutations/useRooms.ts` following the kick/swap pattern.
  - [x] 8.2 `client/src/shared/stores/roomStore.ts`: new actions `addBot({seat, team})` → insert synthetic `RoomPlayer {id:0, userId:0, username:"", seat, team, isBot:true}`; `removeBotBySeat(seat)`. ⚠️ Existing store matching is userId-keyed (roomStore.ts:46-74) — all bots share `userId 0`, so bot mutations MUST match by seat, never by userId; human paths stay untouched.
  - [x] 8.3 `client/src/shared/hooks/useWsDispatch.ts` (system routing, lines ~405-477): handle `system:bot_added`/`system:bot_removed` with the same defensive `typeof` checks the other room-lobby events use (they are NOT zod-validated — see wsEvents.schemas.ts coverage note, lines 22-26).
  - [x] 8.4 `client/src/features/room/components/` SeatTile + `RoomPage.tsx` (seat grid, lines ~991-1105): empty seat → owner in `waiting` rooms sees an "add bot" affordance alongside "take this seat" (non-owners see no change); bot-occupied seat → localized bot name + `<Badge>` bot marker (reuse `shared/components/ui/badge.tsx` tones — `neutral` or `brass`, no new primitive) + owner-only remove control with the same confirm-dialog pattern as kick (`OwnerConfirmDialogs.tsx`). Bots never show "You"/"Host" badges. Quickplay rooms: no bot affordances at all. Verify the owner's start-button enablement (RoomPage derives readiness from the store's players array) counts merged bot entries — it should work for free since bots carry a `seat`, but pin it with a test. Bot tiles participate in the owner's existing swap flow (`handleSwapTarget`, RoomPage.tsx:~418-455) as both source and target — the owner can click a bot then a human/empty seat (and vice versa) exactly like swapping humans; the swap UI affordances ("Move here"/"To swap" states) must treat bot tiles as occupied seats, not empty ones.
  - [x] 8.5 Display-name rule (single source): a shared helper (e.g. `botDisplayName(seat, t)` or component-level) renders `t("bots.seatName", {n: seat + 1})` whenever `player.isBot` — seat-derived, stable, distinct ("Bot 1"–"Bot 4"). Apply in SeatTile, RoomCard lobby preview (`features/lobby/components/RoomCard.tsx` `seatOf`, lines ~30-34), and the room-detail preview (story 2-6 surface) — anywhere `username` renders from a players array, empty username + `isBot` must never leak through as a blank. Note: because identity is seat-derived, a swapped/moved bot changes name ("Bot 2" moved to seat 3 becomes "Bot 4") — accepted consequence, not a bug.
  - [x] 8.6 Tests: SeatTile renders bot name + badge; owner sees add/remove controls, non-owner doesn't; add-bot mutation fires with right payload; store add/remove by seat with two bots present; owner swap flow with a bot as source and as target fires the swap mutation (and the store applies the resulting `seat_updated` + `bot_removed`/`bot_added` sequence correctly); RoomCard preview shows localized bot names (mk locale per 10-2 precedent).
- [x] Task 9 — Client: match table + match history marking (AC: 2, 5)
  - [x] 9.1 Match table: `features/match/components/PlayerSeat.tsx` (+ any other `username` consumers in MatchPage — score reveal rows, trick area labels, surrender/pause dialogs, declaration reveal) render the localized bot name when `player.isBot`. Bots never show connection-lost UI (server keeps `connected: true`).
  - [x] 9.2 `client/src/shared/api/matches.ts` (MatchListItem 28-43, MatchPlayer ~7): add `hasBots: boolean` and per-player `isBot: boolean` (+ `userId` may be 0/absent for bots — keep the explicit-comparison rule, no truthiness).
  - [x] 9.3 `features/profile/MatchHistory.tsx` (MatchRow ~389-499): bot-inclusive rows get a visible localized marker (small badge/chip near the mode label, e.g. `t("profile.matchHistory.withBots")`); SeatChip for a bot player renders the localized bot name. The expanded hand-breakdown needs no change.
  - [x] 9.4 Tests: MatchRow shows the bots marker when `hasBots`, hides otherwise; bot seat shows localized name; PlayerSeat bot-name render test.
- [x] Task 10 — i18n: all four locales, same commit (AC: 1, 5)
  - [x] 10.1 New keys (suggested — adjust idiom, see Dev Notes i18n rules): `bots.seatName` en "Bot {{n}}" / sr "Bot {{n}}" / mk "Бот {{n}}" / hr "Bot {{n}}"; `bots.badge` en "Bot" / mk "Бот"; `room.addBot` en "Add a bot" / sr "Dodaj bota" / mk "Додај бот" / hr "Dodaj bota"; `room.removeBot` + confirm-dialog title/body following the kick-dialog key shapes; `profile.matchHistory.withBots` en "With bots" / sr "Sa botovima" / mk "Со ботови" / hr "S botovima".
  - [x] 10.2 `i18n.parity.test.ts` fails the build on any missing/empty key in any locale — all four files land together. mk all-Cyrillic and idiomatic (not Serbian carryover); hr/sr forms never mixed (hr ijekavica "s", sr "sa"); em-dashes only in en; „quotes" in mk/hr/sr; literal UTF-8, never \uXXXX.
- [x] Task 11 — Verification (Definition of Done)
  - [x] 11.1 `make lint` clean; `make test` green (Go + Vitest incl. parity test + simulation test). `make migrate` runs 000007 up AND down cleanly against a dev DB.
  - [x] 11.2 Manual flow (live-match debug harness from the reference memory works here — 1 browser + bots): create room → add 3 bots (names + badges visible in another browser too) → remove one → re-add → swap yourself with a bot, then move a bot to the freed seat (names/teams update everywhere, second browser included) → start → full match plays out with visibly paced bot turns (~1–2.5 s, no timer expiry on bot seats) → bots declare/announce when holding combos → human pauses + reconnects mid-match, bots unaffected → match ends → history row shows the bots marker and bot names; DB row has `has_bots`, per-seat flags, NULL bot player IDs.
  - [x] 11.3 Regression sweep: a pure-human match end-to-end unchanged (no bot affordances for non-owners/quickplay; snapshots still validate against the strict zod schema).
- [x] Task 12 — Live E2E verification with MCP Playwright (AC: 1-6) — gate before code review
  - [x] 12.1 Environment: `make dev` starts everything (Postgres in Docker + Go on :8080 + Vite on :5173 — if startup fails, check for orphaned processes on 5173/8080 first). App at `http://localhost:5173`. DB inspection via `psql "postgres://beljot:beljot_dev_password@localhost:5433/beljot?sslmode=disable"` (dev DB is on port 5433). If the dev server was restarted mid-session, hard-refresh the browser tab before trusting any "still broken" observation — stale tabs run frozen JS.
  - [x] 12.2 Drive the REAL UI with the MCP Playwright browser tools (`browser_navigate`, `browser_snapshot`, `browser_click`, `browser_fill_form`, …) as the human player: register a fresh account, create a room with **Bitola variant + 501 match mode**, and play with bots. Check MULTIPLE scenarios — not exhaustive, but more than the happy path. Suggested set (adapt to what the deals actually present):
    - **Seating:** add 3 bots (localized names + badges render); remove one and re-add it; swap yourself with a bot and move a bot to an empty seat (names/teams update; bot name follows the seat).
    - **Match flow:** start the match; bots bid and play with visible ~1–2.5 s pacing (never instant, never hitting the turn timer); tricks resolve; ScorePanel shows `… / 501`; score reveal appears at hand end and advances without waiting the full 14 s fallback (bots ack).
    - **Prompts:** when the deal presents them — a bot declares at trick 1 and/or announces Belote/Rebelote; verify via the reveal UI or the hand-score breakdown.
    - **Surrender shortcut (also a scenario):** propose team surrender with a bot partner — the bot accepts after its think delay and the match ends; this doubles as the fast path to a completed match record.
    - **Persistence:** after match end, `psql` the latest row — `SELECT id, has_bots, player1_id, player1_is_bot, player2_id, player2_is_bot, player3_id, player3_is_bot, player4_id, player4_is_bot FROM matches ORDER BY id DESC LIMIT 1;` → `has_bots` true, bot seats NULL + flagged; `SELECT * FROM room_bots;` consistent with the lobby. Match history page shows the bots marker and localized bot names.
    - **Isolation spot-check:** reload the tab mid-match (reconnect path) — state restores and bots keep playing, unaffected.
  - [x] 12.3 Fix-and-retest loop: any scenario that misbehaves → return to implementation, fix, then RE-RUN the failed scenario plus any scenario the fix could plausibly touch. Repeat until the checked set is clean.
  - [x] 12.4 When everything passes: the session is done — mark the story ready for code review (status → review per the dev-story workflow) and record the scenarios exercised + outcomes in the Dev Agent Record.

### Review Findings

Code review 2026-06-12 (Blind Hunter + Edge Case Hunter + Acceptance Auditor; all layers completed). 3 decision-needed (resolved 2026-06-12: 1 dismissed as intended, 2 converted to patches), 14 patch, 2 defer, 7 dismissed.

- [x] [Review][Decision] Stale surrender proposal survives the hand boundary — bot partner auto-accepts at the next hand's first bidding tick — RESOLVED: dismissed as intended behavior. A surrender proposal may legitimately be accepted in the next hand; the bot partner honoring it there is by design. No change. [server/internal/game/surrender.go:48-55; server/internal/match/bot_driver.go:86-87]
- [x] [Review][Patch] Unseated human members stranded at start + bot-filled rooms stay joinable — FIXED (both parts): `StartMatch` requires every room member to hold a seat (`ErrNotAllSeated`), restoring the pre-bots "started ⇒ every member seated" invariant; `JoinRoom` counts bot-covered seats toward capacity (`ROOM_FULL`); `AddBot` enforces the same invariant from the other side (a bot that would over-commit the four seats → `ROOM_FULL`). Tests: `TestJoinRoom_BotCoveredSeatsCountTowardCapacity`, `TestAddBot_RejectedWhenMembersNeedTheSeats`, `TestStartMatch_UnseatedMemberBlocksStart`. [server/internal/room/handler.go]
- [x] [Review][Patch] Server-restart reconcile silently drops bot-inclusive abandoned matches — FIXED: `StaleRoomRepository` gained `FindBotSeatsByRoomID` (adapter delegates to `FindBotsByRoomID`); reconcile counts bot seats as seated and persists the abandoned record with nil IDs + per-seat flags + `HasBots` via `matchSeatColumns`. Test: `TestReconcileStaleRooms_BotInclusiveRoomPersistsFlaggedRecord`. [server/internal/match/reconcile.go; server/internal/room/stale_room_adapter.go]
- [x] [Review][Patch] Human↔bot dual seat occupancy race — FIXED: `SelectSeat` and `StartMatch` now open their transactions with `FindByIDForUpdate`, serializing against the bot endpoints' row lock. [server/internal/room/handler.go]
- [x] [Review][Patch] Desktop declaration banner renders a blank bot name — FIXED: desktop DeclareBanner call site now uses `playerDisplayName(t, player) ?? player.username`, matching the compact call site. [client/src/features/match/MatchPage.tsx]
- [x] [Review][Patch] Engine-rejected bot action is dropped with no reschedule — FIXED: the rejection path re-invokes `maybeScheduleBotAction` so the seat re-arms; relaxed rooms no longer depend on a turn timer that doesn't exist. [server/internal/match/bot_driver.go]
- [x] [Review][Patch] Bots-load failure after the start commit proceeds with corrupted seatInfo — FIXED: a `FindBotsByRoomID` error now skips `StartMatch` exactly like the players-load failure path instead of degrading to `bots = nil`. [server/internal/room/handler.go]
- [x] [Review][Patch] Bot decision applied outside the verification lock — FIXED: new `applyAndBroadcastActionWith(session, build)` runs generation check, decision-point re-verify, and `bot.Decide` inside the SAME critical section that applies the action; `applyAndBroadcastAction` is now a thin wrapper over it for the human path. [server/internal/match/live_match.go; server/internal/match/bot_driver.go]
- [x] [Review][Patch] `trumpsRemainUnseen` double-counts trumps sitting in the current trick — FIXED: now derived from `unseenCards` (map-deduped) instead of a naive three-source count. [server/internal/bot/bot.go]
- [x] [Review][Patch] WS bot-event guards pass NaN/fractional/out-of-range seats — FIXED: `Number.isInteger` + 0–3 range guards on both `system:bot_added`/`system:bot_removed` handlers. The `currentRoomId !== null` semantics were deliberately kept — they match every sibling room-lobby event (seat_updated, match_started). [client/src/shared/hooks/useWsDispatch.ts]
- [x] [Review][Patch] AddBot validates seat range before the owner check — FIXED: range check moved inside the transaction after owner/waiting/quickplay gates (KickPlayer order); non-owner probing now gets `NOT_ROOM_OWNER`. Test: `TestAddBot_NonOwnerWithJunkSeatGetsNotRoomOwner`. [server/internal/room/handler.go]
- [x] [Review][Patch] Missing Task 8.6 coverage — FIXED: added the human-source→bot-target RoomPage swap test, the store test applying the full `seat_updated` + `bot_removed` + `bot_added` sequence, and a stale-`bot_added` occupant-guard store test. [client/src/features/room/RoomPage.bots.test.tsx; client/src/shared/stores/roomStore.test.ts]
- [x] [Review][Patch] Cross-phase reuse of a pending bot timer can act ~10 ms after a new decision point opens — FIXED: `botDecisionContext` (phase, hand, belote/declare/surrender flags) is captured at arm time; on fire, a changed context re-arms a fresh delay instead of acting, preserving the ≥1 s humanization floor. [server/internal/match/bot_driver.go]
- [x] [Review][Patch] `roomStore.addBot` dedupes only against bots — FIXED: guard now rejects insertion while ANY occupant holds the seat (the server's swap ordering frees the seat before `bot_added` arrives; a still-occupied seat means a stale event). [client/src/shared/stores/roomStore.ts]
- [x] [Review][Patch] `mergeBotPlayers` discards `RoomBot.CreatedAt` — FIXED: the synthetic wire entry now carries the real seat time from the `room_bots` row. [server/internal/room/handler.go]
- [x] [Review][Defer] Strict zod `PlayerStateSchema` + required `isBot` breaks every mixed-version window (stale tab on old bundle × new server, and the reverse) [client/src/shared/types/wsEvents.schemas.ts:73] — deferred, pre-existing: the `.strict()` schema architecture and the same-commit contract rule predate this story; version-skew tolerance is a platform-wide decision (D142)
- [x] [Review][Defer] Remove-bot confirm dialog's `pending` spinner is unreachable (dialog closed before `await mutateAsync`) [client/src/features/room/RoomPage.tsx:497-503] — deferred, pre-existing: faithfully copies the kick-dialog house pattern which has the identical dead `pending` wiring; fix both together (D143)

## Dev Notes

### The three design decisions that shape everything (settled — do not relitigate)

1. **Bots are NOT users, NOT room_players rows, NOT non-null match player IDs.** `room_players.user_id` and `matches.player1–4_id` are `NOT NULL REFERENCES users(id)` (migrations 000004/000005), and AC5 forbids creating accounts. Hence: `room_bots` table for seating, nullable match player IDs + per-seat bot flags for persistence, and synthetic `{userId:0, isBot:true}` entries merged into wire payloads. Username hydration everywhere is an INNER JOIN to users (gorm_repo.go:138-139,197-198) — bots never pass through those queries; the merge helper is the single place bots enter a players array.
2. **Bot identity is client-rendered, server-flagged.** Server stores/sends only `isBot` + seat; every display surface renders `t("bots.seatName", {n: seat+1})`. This is what makes the identity localized (AC1) — a server-side "Bot 2" string would freeze one language into snapshots and history. Empty `username` + `isBot:true` is the wire signature of a bot.
3. **Bots are server-side actors driven by the session manager — no WebSocket, no client, no auth.** Decision logic lives in `internal/bot` as a pure function over a redacted `View`; scheduling, delay, memory upkeep, and applying via `ApplyAction` live in the match layer. The rules engine stays pure and untouched except the one-line `LegalCards` export.

### Hard scope boundaries — do NOT touch

- **Engine rules unchanged.** `internal/game` gets exactly two additions: `PlayerState.IsBot` (a field, not a rule) and the `LegalCards` export. No new actions, no validation changes, no scoring changes. Bots send the SAME actions humans send.
- **No disconnect fill-in, ever.** Architecture constraint #6 (amended 2026-06-11) keeps its second half: bots are seated pre-game only. Do not add any "replace with bot" affordance to the disconnect/abandon flow.
- **Quickplay excluded.** Quickplay rooms auto-start on 4 humans (handler.go:1618 hardcodes their config) — reject add-bot there (`ErrBotsNotAllowed`). Matchmaking with bots is not in the ACs.
- **Bot swaps go through SwapSeats only.** The owner rearranges bots via the existing swap flow (Task 2.7) — human ↔ bot, bot → empty, all in one transaction. `SelectSeat` (a player seating THEMSELVES) still treats bot seats as occupied (`SEAT_TAKEN`); only the owner's swap can displace a bot.
- **No XP/coins/honor/stats code.** Those systems don't exist until Epic 9; this story only persists the flags Epic 9 will honor (sprint-change-proposal Change 2 §2, Epic Impact).
- **Chat/emotes:** bots never produce them; no UI for "bot chat". Nothing to build.
- **Bitola only in practice** (the only variant shipped), but write the bot against the engine surface, not variant internals — round-2 bidding already differs by variant inside the engine, and `Decide` only ever sees legal options.
- One story = one branch = one PR. Suggested branch: `feat/E10-S3-bot-players`. Unrelated finds → deferred-work.md (D137–D141 already live there; don't fix them here).

### Architecture & project-context compliance (the rules that bite here)

- **WS contract files move together** — `events.go` + `wsEvents.ts` in the same commit (two new system events + the `isBot` payload fields). The zod snapshot schema is `.strict()`: the Go `PlayerState.IsBot` field and the schema entry MUST be one commit or every client hard-fails on the next snapshot.
- **Multi-event ordering preserved** — bot actions reuse the exact broadcast path; never batch.
- **Session manager owns side effects** — delay timers, memory updates, scheduling are match-layer; `ws/router.go` stays dispatch-only; `internal/bot.Decide` is pure (no clock, no rand, no logging).
- **Timer truth** — absolute `TurnExpiresAt` stays armed on bot turns as the safety net; bot delay (≤2.5 s) always beats the 10 s minimum per-move duration. Never send relative durations.
- **Generation-guarded timers** — the bot `AfterFunc` follows the `timerGeneration` staleness pattern (live_match.go:24, 1068-1092); on fire: lock → re-verify → act. A stale bot action that slips through still fails safely in `ApplyAction` (wrong seat/phase), but the error path re-arms timers (live_match.go:229-231) — don't rely on it, guard properly.
- **Concurrency** — add/remove bot uses `RunInTransaction` + `FindByIDForUpdate` (8.5-1 pattern) against races with seat selection and start; two simultaneous add-bot calls on one seat must yield one winner + one `SEAT_TAKEN`.
- **Counter-clockwise turn order, trick winner leads** — the bot driver derives nothing itself; it reads `ActivePlayerSeat`/`PendingBelotSeat` from the state the engine produced.
- **No truthiness on Go-sourced fields** — `isBot === true` checks client-side; `userId` 0 is a real value, not null.
- **Errors** — new entries in `apperr/errors.go` only (`ErrBotsNotAllowed`, `ErrNoBotOnSeat`); everything else reuses the existing room block.
- **Naming** — `room_bots` table, `idx_room_bots_room_seat`, `internal/bot` package, `bots.*` / `room.*` / `profile.matchHistory.*` i18n keys, REST `POST /rooms/:id/bots` + `DELETE /rooms/:id/bots/:seat`.

### How the pieces talk (read this before writing bot_driver.go)

```
owner HTTP POST /rooms/:id/bots ─→ room handler (tx, locks, validates)
        └─ broadcasts system:bot_added + system:room_updated (players incl. synthetic bot)
owner starts → seatInfo[4]{IsBot} → match.StartMatch → game.NewGame (PlayerState.IsBot)
        └─ maybeScheduleBotAction(session)   ←──────────────────────────────┐
              └─ decision seat is bot? → AfterFunc(1–2.5s, generation-guarded)│
                    └─ lock; re-verify; buildBotView(gs, seat, memory)        │
                          └─ bot.Decide(view) → game.Action                   │
                                └─ same apply+broadcast path as humans ───────┘
                                      (updates botMemory on play_card)
match end → handleMatchEnd → Match{PlayerNID:*uint, PlayerNIsBot, HasBots} → history API → UI marker
```

### Key code facts (verified 2026-06-11, line refs are navigation hints)

- `Manager` keyed by roomID, dual-layer mutex (`live_match.go:57-65`); `LiveMatch.timerGeneration` staleness counter (`:24`); `HandleAction` entry (`:154-353`); timer expiry decision table per phase (`:1156-1215`); chained prompt auto-resolution loop (`:1233-1299`); `expiryGrace = 400ms` (`:1055`); hand-complete 14 s fixed-deadline auto-continue (`:1094-1106`).
- Engine: `ApplyAction(state, action) (*GameState, error)` (`rules_engine.go:9`); action constants (`types.go:100-122`); `Action{Type, PlayerSeat, Card, Suit}` (`types.go:125-130`); `legalCards` unexported (`validation.go:16-55`); `AutoPlay` exported, suit-then-rank-sorted first legal (`auto_play.go:33-49`); declarations auto-detected + Bitola dedup (`declarations.go:42-155`); belote prompt = `PendingBelotSeat`, turn does NOT advance until resolved (`declarations.go:333-405`); point/rank tables exported (`types.go:148-198`); `GameState` has NO played-cards history — `CurrentTrick` clears on resolution (`playing.go:139`), hence the match-layer `Memory`.
- Room: status values "waiting"/"playing"/"completed" (`handler.go:27`); seat-parity team rule (`:642-647`); start gates `seatedCount >= 4` (`:932-939, :1496-1503`); seatInfo build (`:1527-1534`); lifecycle payload (`:136-171`); seat snapshot broadcast (`:183-190`).
- Persistence: `Match` model (`match/model.go:14-33`); `CreateWithHands` transactional (`gorm_repo.go:43-56`); history endpoint + `MatchListItem` (`user/handler.go:375-416, 71-86`); viewer-relative outcome (`:521-526`).
- Client: roomStore userId-keyed mutations (`roomStore.ts:46-74`); dispatch routing (`useWsDispatch.ts:~405-477`); room-lobby events use defensive typeof checks, NOT zod (`wsEvents.schemas.ts:22-26` coverage note); seat grid (`RoomPage.tsx:991-1105`); badge tones (`ui/badge.tsx:5,14-45`); history row (`MatchHistory.tsx:389-499`); rooms API client (`shared/api/rooms.ts`).

### Previous story intelligence (10.2, done 2026-06-11)

- Red-green per task worked well; keep it — especially "human takes bot seat → SEAT_TAKEN" and the zod-strict snapshot test, which fail loudly before the fix.
- i18n wording is the most-reviewed surface; the Task 10 values are starting points, not gospel. mk went through three review rounds on 10.1 for Serbian carryover — budget for it.
- Review patterns that recur: pin exact boundaries (10.2 got dinged for not pinning score==501 — here, pin "delay ≥1 s" and "bot acts before timer expiry"); keep the File List complete including sprint-status.yaml; page-level tests for both branches (bot and non-bot), not just the new one.
- `data-testid` + i18n-key-driven assertions, not literal UI strings.
- The /tmp WS bot harness (`/tmp/beljot-501-uibots-v3.mjs`) is a TEST CLIENT, not a design template — this story's bots are server-side actors, but the harness remains useful for manual verification as the human-side driver and shows the full action vocabulary a seat needs.

### Git intelligence

- `a9999fc` (HEAD) shipped 10.2 — `room/handler.go`, `MatchPage.tsx`, locale files, and `scoring_test.go` were all just touched; rebase pain is low but real if hotfixes land first.
- House pattern: feature commits ship co-located tests in the same commit; scopes here will be `feat(room):`, `feat(bot):`, `feat(match):`, `feat(profile):` ≤72 chars.

### Web research

Not applicable — no new libraries. Stack stays Go stdlib (`math/rand/v2` — note: it has NO global `Seed`; use `rand.New(rand.NewPCG(...))` where the simulation needs a seeded source) + GORM + Echo v4 + React 19 + Zustand + Vitest + testify, all pinned.

### Testing standards summary

- Go: stdlib `testing` + testify, co-located `_test.go`, table-driven; engine behavior via `ApplyAction` effects with `testfixtures` factories only (no raw `GameState{}` literals); WS-touching manager tests use `httptest.Server` + real WS client; DB-touching tests use per-test transaction rollback; tests never depend on `make seed`.
- `internal/bot` is a NEW package: unit tests for `Decide`/`Memory` are direct (it's pure — that's the point); the simulation test is the AC4 evidence and the AC3 always-legal proof.
- Frontend: Vitest co-located, `data-testid` selection, present-tense descriptions; i18n parity test gates all four locales.
- Gates: `make lint` + `make test` green before review; migration up AND down verified; PLUS the Task 12 live MCP Playwright session (real account, Bitola 501 room, multiple bot scenarios, fix-and-retest loop) — the story is not review-ready until that session is clean.

### Project Structure Notes

- New: `server/migrations/000007_add_bot_players.{up,down}.sql`, `server/internal/bot/` (`bot.go`, `view.go`, `memory.go`, + tests), `server/internal/match/bot_driver.go`. Everything else lands in existing domain locations.
- `internal/bot` deliberately does NOT follow the `model/repository/handler` domain-package shape — it has no persistence and no HTTP surface; it is a pure decision library consumed by `internal/match` (precedent: `internal/game` is also shape-exempt). Seating persistence lives in the room domain where the table lives.
- Cross-cutting atomic units: the two WS contract files; the four locale JSONs; the Go `IsBot` field + zod schema. Each set is one commit.

### References

- Epics: `_bmad-output/planning-artifacts/epics.md` § "Story 10.3: Bot Players" (lines 2035-2090); Epic 10 overview (1977-1979); FR59 (line 84); sizing/sharding note (line 2037)
- Provenance + technical impact: `_bmad-output/planning-artifacts/sprint-change-proposal-2026-06-11.md` Change 2 (§§1-5 — `internal/bot/` suggestion, flagging requirement, Epic 9 inheritance, sizing warning)
- PRD: `_bmad-output/planning-artifacts/prd.md` Phase 2 bots bullet (line 156)
- Architecture: `_bmad-output/planning-artifacts/architecture.md` constraint #6 as amended (line 68); WS contract process rule (line 424); DoD checklist (lines 542-583)
- Project rules: `_bmad-output/project-context.md` (turn order CCW, declarations timing, belote announcement, three-layer validation, auto-play rule, timer absolutes, testing rules, DoD)
- Schema ground truth: `server/migrations/000004_create_room_players.up.sql`, `000005_create_matches.up.sql` (the NOT NULL + FK constraints that force this design)
- Localization: memory `reference_localization-terminology.md` (terminology table, banned "contract"); 10.1/10.2 stories for the locale review history

## Dev Agent Record

### Agent Model Used

claude-fable-5 (Claude Fable 5)

### Debug Log References

- Migration 000007 verified up → down → up against the dev DB (port 5433) before any Go work.
- `internal/ws` golden drift gate (`TestEventsJSONContract/EventMatchState`) failed after `PlayerState.IsBot` landed — regenerated `testdata/events/event_match_state.json` with `UPDATE_GOLDENS=1`; the client zod schema (updated in the same change) validates the regenerated golden.
- Required `isBot` field on `RoomPlayer`/`PlayerState`/`MatchPlayer` broke ~20 client test fixtures + 2 source constructors (`useRoomUpdates.ts`, `useWsDispatch.ts` player_joined) — swept all with `isBot: false`.
- Live E2E found one real gap: "swap yourself with a bot" was blocked by the pre-existing own-seat-cancels-swap guard in `RoomPage.handleSwapTarget`. Fixed: the owner's own seat is a valid swap target when the SOURCE is a bot (human sources keep the cancel semantics); pinned with a new RoomPage test, re-ran the scenario live — green.
- Incidental live confirmation of the 2.10 invariant: a hard page reload in the waiting room fired the page-hide leave beacon; as the sole human left, the bot-filled room flipped to "completed" (bots never keep a room alive).

### Completion Notes List

- **Server**: `room_bots` table + nullable match player IDs (migration 000007); room domain bot seating (add/remove/swap/move endpoints, merge helper, SelectSeat occupancy guard, start-gate coverage by humans-or-bots); session-manager bot driver (`match/bot_driver.go`) with per-seat generation-guarded think-delay timers (1–2.5 s, injectable for tests), riding the SAME apply-and-broadcast path as human actions (extracted `applyAndBroadcastAction` from `HandleAction`); pure decision library `internal/bot` (redacted View — structural no-peeking; deterministic `Decide`; per-match `Memory` with void inference); persistence with per-seat bot flags + `HasBots`; history API marks bots.
- **Bots never enter** `userToRoom`, broadcast recipient lists, reconnect windows, or the users-table lookups; they stay `Connected: true` forever; surrender is team-internal (bot accepts its partner's proposal only); bot actions carry no `autoPlayed` marker and the turn timer stays armed as the safety net.
- **AC4 evidence**: `internal/bot/simulation_test.go` — heuristic team takes 68–76% of all points vs a seeded random-legal baseline over 200 full hands (bar: ≥60%); zero `ApplyAction` rejections across every simulated action doubles as the AC3 always-legal proof.
- **Client**: bot identity rendered seat-derived + localized via single-source helpers (`botDisplayName`/`playerDisplayName`) applied to every username surface (seat tiles, roster, legend, lobby card chips, match table, reveals, pause/surrender/disconnect dialogs, history rows); owner add/remove affordances + remove-confirm dialog (kick pattern); bots participate in the swap flow as source and target; seat-keyed bot store mutations (all bots share userId 0); two new WS events dispatched with defensive guards; history rows show the "With bots" marker.
- **i18n**: `bots.*`, `room.addBot`/`removeBot*`, `room.errors.*bot*`, `profile.matchHistory.withBots` in all four locales (mk all-Cyrillic; sr/hr forms kept distinct; em-dashes only in en); parity test green.
- **Live E2E session (MCP Playwright, fresh account, Bitola 501)**: seated 3 bots (names+badges), removed/re-added, swapped self↔bot, moved bot→empty (name follows seat); started match; bots bid (passed weak round 1, picked qualifying suits in round 2) and played with visible ~1–2.5 s pacing and zero timer expiries; trick resolution, declaration reveal of a BOT's tierce, score reveal advancing without the 14 s fallback, next hand dealt; surrender accepted by bot partner (twice — both matches completed); persistence verified via psql (`has_bots=t`, NULL+flagged bot seats, human ID intact, `room_bots` consistent); history page shows marker + bot names; mid-match hard reload restored state and bots kept playing.
- **Gates**: `make lint` clean; `make test` green (789 client tests incl. 19 new; full Go suite incl. 22 new room handler tests, 10 bot-driver manager tests, 16 bot unit tests + simulation); migration up/down verified.

### Change Log

- 2026-06-12 (code review): adversarial review (Blind Hunter + Edge Case Hunter + Acceptance Auditor) produced 14 patches, all applied — see Review Findings. Headlines: room row-lock coverage extended to SelectSeat/StartMatch (human↔bot seat race); humans+bots≤4 capacity invariant enforced across JoinRoom/AddBot/StartMatch (no stranded unseated members); bot decisions now verify AND apply inside one critical section (`applyAndBroadcastActionWith`); engine-rejected bot actions re-schedule (relaxed-room stall guard); stale cross-phase bot timers re-arm instead of acting instantly; restart reconcile persists bot-inclusive abandoned matches with flags; desktop declaration banner renders the localized bot name; `trumpsRemainUnseen` deduped. Surrender-across-hand-boundary behavior reviewed and confirmed INTENDED (a proposal may be accepted in the next hand — bot partner honoring it is by design). Two pre-existing items deferred as D142/D143. `make lint` + `make test` green (792 client + full Go).
- 2026-06-12 (UI polish, user feedback): bot avatars render the Bot glyph instead of a name initial on EVERY surface — room seat tiles, roster dropdown, remove-bot dialog (via a new `icon` slot on the shared Avatar), the in-match seat discs and trump-reveal taker, the profile match-history chips, and the lobby room-card seat chips. All bot glyphs share one optical rule: a 5% upward shift, because the lucide Bot glyph is geometrically centered but its bottom-heavy visual mass (robot head low in the viewBox) reads as off-center, especially at chip sizes. The add-bot button icon was also enlarged to 14px with the same nudge.
- 2026-06-12: Story 10.3 implemented end to end (bot seating, server-side bot actors, heuristic strategy + simulation proof, persistence/history marking, client UI, four-locale i18n). One adjacent UI behavior change shipped as part of AC1: the owner's own seat is now a valid swap target when the swap source is a bot (previously any own-seat click cancelled swap mode; human-source behavior unchanged).

### File List

New:
- server/migrations/000007_add_bot_players.up.sql
- server/migrations/000007_add_bot_players.down.sql
- server/internal/bot/bot.go
- server/internal/bot/view.go
- server/internal/bot/memory.go
- server/internal/bot/bot_test.go
- server/internal/bot/simulation_test.go
- server/internal/match/bot_driver.go
- server/internal/match/bot_driver_test.go
- server/internal/room/bot_handler_test.go
- client/src/shared/lib/botName.ts
- client/src/features/room/RoomPage.bots.test.tsx
- _bmad-output/implementation-artifacts/10-3-bot-players.md

Modified (server):
- server/cmd/api/main.go
- server/internal/apperr/errors.go
- server/internal/game/state.go
- server/internal/game/validation.go
- server/internal/game/state_test.go
- server/internal/game/scoring_test.go
- server/internal/lobby/lobby_test.go
- server/internal/match/export_test.go
- server/internal/match/live_match.go
- server/internal/match/model.go
- server/internal/match/reconcile.go
- server/internal/match/reconcile_test.go
- server/internal/room/stale_room_adapter.go
- server/internal/match/reconnect.go
- server/internal/match/timer.go
- server/internal/room/gorm_repo.go
- server/internal/room/handler.go
- server/internal/room/handler_test.go
- server/internal/room/lobby_disconnect_test.go
- server/internal/room/model.go
- server/internal/room/repository.go
- server/internal/user/handler.go
- server/internal/user/handler_test.go
- server/internal/ws/events.go
- server/internal/ws/testdata/events/event_match_state.json

Modified (client):
- client/src/App.test.tsx
- client/src/shared/components/ui/avatar.tsx
- client/src/features/profile/components/SeatChip.tsx
- client/src/features/lobby/components/SeatChip.tsx
- client/src/features/lobby/LobbyPage.test.tsx
- client/src/features/lobby/MatchmakingPage.test.tsx
- client/src/features/lobby/components/MatchmakingDiagram.test.tsx
- client/src/features/lobby/components/RoomCard.tsx
- client/src/features/lobby/components/RoomCard.test.tsx
- client/src/features/lobby/useRoomUpdates.ts
- client/src/features/match/MatchPage.tsx
- client/src/features/match/MatchPage.test.tsx
- client/src/features/match/components/BelotReveal.tsx
- client/src/features/match/components/BelotReveal.test.tsx
- client/src/features/match/components/DeclarationReveal.tsx
- client/src/features/match/components/DeclarationReveal.test.tsx
- client/src/features/match/components/PauseOverlay.tsx
- client/src/features/match/components/PauseOverlay.test.tsx
- client/src/features/match/components/PlayerSeat.tsx
- client/src/features/match/components/PlayerSeat.test.tsx
- client/src/features/match/components/TrumpReveal.tsx
- client/src/features/match/components/TrumpReveal.test.tsx
- client/src/features/match/lib/legalCards.test.ts
- client/src/features/profile/MatchHistory.tsx
- client/src/features/profile/MatchHistory.test.tsx
- client/src/features/room/CreateRoomModal.test.tsx
- client/src/features/room/OwnerConfirmDialogs.tsx
- client/src/features/room/RoomPage.tsx
- client/src/features/room/RoomPage.test.tsx
- client/src/features/room/RoomPage.diamond.test.tsx
- client/src/features/room/RoomPage.locale.test.tsx
- client/src/features/room/components/SeatTile.tsx
- client/src/shared/api/matches.ts
- client/src/shared/api/rooms.ts
- client/src/shared/hooks/mutations/useRooms.ts
- client/src/shared/hooks/useReconnectionRedirect.test.tsx
- client/src/shared/hooks/useWsDispatch.ts
- client/src/shared/hooks/useWsDispatch.test.ts
- client/src/shared/i18n/en.json
- client/src/shared/i18n/sr.json
- client/src/shared/i18n/mk.json
- client/src/shared/i18n/hr.json
- client/src/shared/stores/roomStore.ts
- client/src/shared/stores/roomStore.test.ts
- client/src/shared/stores/matchStore.test.ts
- client/src/shared/types/apiTypes.ts
- client/src/shared/types/matchTypes.ts
- client/src/shared/types/matchTypes.test.ts
- client/src/shared/types/wsEvents.ts
- client/src/shared/types/wsEvents.schemas.ts

Modified (tracking):
- _bmad-output/implementation-artifacts/sprint-status.yaml
