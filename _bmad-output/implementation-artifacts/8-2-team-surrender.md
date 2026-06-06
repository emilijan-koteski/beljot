# Story 8.2: Team Surrender

Status: done

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

## Story

As a player in an active match,
I want to propose surrendering to my teammate so that on their acceptance the match ends as a win for the opponents,
so that we can end a hopeless game without grinding it out to 1001.

## Acceptance Criteria

1. **A seated player in an active match can initiate one surrender request per match**
   Given the authenticated player is one of the four seated players in an active session whose `gs.Phase == game.PhasePlaying` (or `PhaseBidding`),
   When the client sends `action:surrender_request` (no payload) over WebSocket,
   Then `Manager.HandleAction` resolves their seat via `session.playerIDs`, calls `game.ApplyAction` with `Action{Type: ActionSurrenderRequest, PlayerSeat: seat}`, and the rules engine sets `gs.SurrenderProposerSeat = &seat`, `gs.SurrenderUsed[seat] = true`, and leaves `gs.Phase` unchanged.
   And after commit the session manager broadcasts (in this order, as separate ordered messages — never batched, per [\_bmad-output/project-context.md#L109](_bmad-output/project-context.md)):
   - `event:surrender_proposed` with `SurrenderProposedPayload{ proposerSeat, proposerTeam, proposerUsername, partnerSeat }` to all 4 player WS connections. (Clients derive opponent seats trivially as the two seats whose team is `1 - proposerTeam` — no `opponentSeats[2]` field on the wire. AC#10 is canonical for the payload shape; this AC text was amended in the Story 8.2 code review to match.)
   - `event:game_state` with the full updated `GameState` (so clients can read `pendingSurrenderProposerSeat` for non-prompt UI gating, e.g. opponent banner).
     And the action is rejected with `error:wrong_phase` when called outside `PhasePlaying`/`PhaseBidding` (i.e. dealing, hand_scoring, trick_resolving, paused, disconnected, match_end). `PhasePaused` rejection mirrors the existing rule: pause must be cleared first.
     And the action is rejected with `error:surrender_exhausted` (new — see AC #11) when `gs.SurrenderUsed[seat] == true`.
     And the action is rejected with `error:action_required` (existing `ErrActionRequired`) when a surrender proposal is already pending (`gs.SurrenderProposerSeat != nil`) — the second proposer's request is held until the first is resolved.
     And **the per-move turn timer keeps running on the active player**. Surrender proposal does not affect `gs.ActivePlayerSeat`, `TurnExpiresAt`, or `TurnTimeRemaining`. The active player can still play a card while the proposal is pending — gameplay continues until accept/decline.

2. **The teammate (and only the teammate) can accept the proposal**
   Given `gs.SurrenderProposerSeat != nil` AND the caller's seat is the partner of the proposer (`partnerSeat = (proposerSeat + 2) % 4`),
   When the client sends `action:surrender_accept` over WebSocket,
   Then `game.ApplyAction(state, Action{Type: ActionSurrenderAccept, PlayerSeat: seat})` returns a state with:
   - `gs.Phase = game.PhaseMatchEnd`
   - `gs.WinnerTeam = &opponentTeam` where `opponentTeam = 1 - game.TeamForSeat(proposerSeat)`
   - `gs.SurrenderProposerSeat = nil` (cleared)
   - `gs.SurrenderUsed` unchanged (the proposer's `true` stays `true`)
   - `gs.TurnExpiresAt = nil`, `gs.TurnTimeRemaining = 0` (timer cancelled at match end — same shape as the natural `PhaseMatchEnd` transition).
     And the session manager (extending the existing match-end path in [server/internal/session/manager.go:237-239](server/internal/session/manager.go#L237-L239)):
   - Cancels the turn timer.
   - Broadcasts `event:match_end` with `MatchEndPayload{ winnerTeam, teamAFinalScore, teamBFinalScore, matchDurationSec, outcomeReason: "surrender", surrenderedBySeat: <proposerSeat> }` (extending the existing payload — see AC #10). The seat-suffixed name disambiguates this wire field from the persistence column `match.SurrenderedBy` which holds a userID; renamed in the Story 8.2 code review.
   - Broadcasts the authoritative `event:game_state` so clients leave the surrender UI and render the match-end overlay.
   - Calls `handleMatchEnd(session, finalState)` to persist the match record and update room status.
     And `error:not_your_turn` is **NOT** the right rejection — surrender accept is partner-only, not turn-based. Use `error:invalid_action` ("only your teammate can accept your surrender proposal") when called by a non-partner.
     And `error:wrong_phase` is returned when no proposal is pending (`gs.SurrenderProposerSeat == nil`).
     And the proposer themselves cannot accept their own proposal (`seat == *gs.SurrenderProposerSeat` → `error:invalid_action`). Either opponent calling accept also returns `error:invalid_action`. Only the partner is authorised.

3. **The teammate can decline the proposal — play resumes, proposer's attempt is consumed**
   Given `gs.SurrenderProposerSeat != nil` AND the caller's seat is the partner of the proposer,
   When the client sends `action:surrender_decline`,
   Then `game.ApplyAction` returns a state with:
   - `gs.SurrenderProposerSeat = nil` (cleared)
   - `gs.SurrenderUsed[proposerSeat] == true` is preserved (the attempt is consumed even though it was declined — see FR28a "each player may trigger a surrender request at most once per game" and epics.md AC "the proposing player's surrender attempt is consumed").
   - All other state fields unchanged: `gs.Phase`, `gs.ActivePlayerSeat`, `gs.TurnExpiresAt`, scores, hands, trick state remain identical to before the proposal.
     And the session manager broadcasts (separate ordered messages):
   - `event:surrender_declined` with `SurrenderDeclinedPayload{ proposerSeat, decliningSeat }` to all 4 player WS connections.
   - `event:game_state` with the cleared `pendingSurrenderProposerSeat`.
     And the per-move turn timer is **not reset** by decline — whatever time the active player had left when the proposal opened is still ticking down (same lock-step semantics as Belote accept/decline today: Belote decline does not bump the active player's timer either).
     And the partner can decline even if they themselves have already used their own surrender attempt (`gs.SurrenderUsed[partnerSeat] == true`) — surrender_used controls the **request** action, not the **decline** action.

4. **Match record is persisted with surrender outcome**
   Given a surrender accept transitions the session to `PhaseMatchEnd`,
   When `handleMatchEnd` runs (extended for surrender — see Task 4),
   Then a row is inserted into the `matches` table with:
   - `status = 'completed'` (intentional — surrender is a legitimate, non-abandoned match end; this preserves the existing wins/losses stats query at [server/internal/match/gorm_repo.go:46-61](server/internal/match/gorm_repo.go#L46-L61) without changes).
   - `winner_team = <opponentTeam>` (0 or 1 depending on which team surrendered).
   - **A new column `surrendered_by` (`INTEGER REFERENCES users(id) NULL`)** set to the proposer's `userID`.
   - `team_a_score` / `team_b_score` = the actual `gs.TeamScores` snapshot at surrender time (NOT artificially zero or inflated). The losing team's score reflects their accumulated points up to that moment.
   - `started_at`, `completed_at`, `room_id`, `player1..4_id`, `variant`, `match_mode` populated as in the natural-end path.
     And buffered `session.handResults` are flushed alongside the match row in the same transaction (mirrors [server/internal/session/manager.go:660](server/internal/session/manager.go#L660) `CreateWithHands` behaviour). Hands completed before surrender are persisted; partially-played hands at surrender time are NOT persisted (no `LastHandResult` was buffered for them — same as natural mid-hand match ends today).
     And the room status is updated to `"completed"` via `m.roomUpdater.UpdateRoomStatus(roomID, "completed")` and the session is removed via `m.RemoveSession(roomID)` — identical to the natural match-end path.

5. **Migration adds `surrendered_by` column to the `matches` table**
   Given the project uses `golang-migrate` with sequentially numbered SQL migrations,
   When the next migration is created at `server/migrations/000010_add_match_surrendered_by.up.sql` (and matching `.down.sql`),
   Then the migration adds the column and a supporting index, mirroring the `abandoned_by` precedent in [server/migrations/000008_add_match_status.up.sql](server/migrations/000008_add_match_status.up.sql):

   ```sql
   -- 000010_add_match_surrendered_by.up.sql
   ALTER TABLE matches ADD COLUMN surrendered_by INTEGER REFERENCES users(id);
   CREATE INDEX idx_matches_surrendered_by ON matches(surrendered_by);
   ```

   ```sql
   -- 000010_add_match_surrendered_by.down.sql
   DROP INDEX IF EXISTS idx_matches_surrendered_by;
   ALTER TABLE matches DROP COLUMN IF EXISTS surrendered_by;
   ```

   And the `match.Match` struct gains `SurrenderedBy *uint \`gorm:"index" json:"surrenderedBy,omitempty"\``immediately after`AbandonedBy`([server/internal/match/model.go:21](server/internal/match/model.go#L21)).
And the existing stats query at [server/internal/match/gorm_repo.go:34-74](server/internal/match/gorm_repo.go#L34-L74) is **NOT modified** — surrender keeps`status='completed'`so it counts as a win for the winning team and a loss for the surrendering team automatically. **Honor-system future-coupling is intentional**: PRD line 134 states surrender is a "neutral" honor signal distinct from rage-quit/timeout/disconnect — Story 9.6 will read`surrendered_by` to compute that. This story does NOT touch the honor system.

6. **Opponents see a banner during a pending proposal**
   Given the client receives `event:surrender_proposed`,
   When the local player's seat is on the **opposing team** (`game.TeamForSeat(myPlayerSeat) != game.TeamForSeat(payload.proposerSeat)`),
   Then the GamePage renders an **opponent-banner overlay** (NOT a modal — opponents must keep playing while the proposal is pending) with `data-testid="surrender-opponent-banner"` showing `t("game.surrender.opponentBanner", { username: payload.proposerUsername })` and a small `Surrender pending…` caption.
   And the banner is non-blocking: it renders as a slim strip near the top of the screen (`top-12 left-1/2 -translate-x-1/2 z-30` — beneath the score panel but above the trick area) and does NOT trap focus or steal pointer events from the playing surface.
   And the banner auto-dismisses when:
   - `event:surrender_declined` arrives → banner disappears (proposal cleared).
   - `event:match_end` arrives with `outcomeReason === "surrender"` → MatchResult overlay takes over and the banner is unmounted by GamePage.
     And the banner is hidden if the local player is the **proposer or the proposer's partner** — those two see the prompt UI from AC #7 instead.

7. **The proposer's partner sees an accept/decline prompt (modal-style, focus-trapped)**
   Given the client receives `event:surrender_proposed` AND the local player's seat satisfies `myPlayerSeat === (payload.proposerSeat + 2) % 4`,
   When GamePage detects `pendingSurrenderProposerSeat === <partner's myPlayerSeat>` from `gs.SurrenderProposerSeat`,
   Then a **`SurrenderPrompt` component** mounts (mirroring the [BelotPrompt.tsx](client/src/features/game/components/BelotPrompt.tsx) shape — `role="dialog" aria-modal="true"`, `useFocusTrap`, centered overlay with `data-testid="surrender-prompt"`):
   - Title: `t("game.surrender.prompt.title")` — "Your teammate wants to surrender"
   - Body: `t("game.surrender.prompt.body", { username: <proposer.username> })` — "{{username}} is proposing to forfeit the match. Accepting ends the match as an opponent win."
   - Two buttons: - Accept (primary): `data-testid="surrender-prompt-accept"`, sends `action:surrender_accept`. - Decline (ghost): `data-testid="surrender-prompt-decline"`, sends `action:surrender_decline`.
     And the prompt is dismissed by the next `event:game_state` clearing `pendingSurrenderProposerSeat` (same pattern as `pendingBelotSeat` at [GamePage.tsx:356](client/src/features/game/GamePage.tsx#L356)).
     And the proposer themselves see a **disabled "Surrender pending…" caption** in place of the surrender button while their proposal is in flight (see AC #8) — they do NOT see the partner's prompt and cannot accept their own.

8. **Surrender request button — gating, position, exhausted state**
   Given the local player is in an active match (`gs.Phase === "playing" || gs.Phase === "bidding"` and not paused/disconnected),
   When GamePage renders,
   Then a **`SurrenderButton`** is rendered next to the existing pause-button slot (see [GamePage.tsx:451-463](client/src/features/game/GamePage.tsx#L451-L463) — bottom-left). Suggested layout: a vertical `flex flex-col gap-2` wrapping the existing pause button and a new sibling surrender button (so the bottom-left now stacks two controls).
   And the surrender button has `data-testid="surrender-button"` and label `t("game.surrender.requestButton")` — "Surrender".
   And the button is **disabled** (visually `disabled:opacity-40 disabled:cursor-not-allowed`, like pause-button) when ANY of:
   - `gs.SurrenderUsed?.[myPlayerSeat] === true` — caption flips to `t("game.surrender.exhausted")` ("Surrender already used") matching the pause-button's `pauseUsed`/`pauseButton` toggle.
   - `gs.SurrenderProposerSeat !== null` — a proposal is already pending. Caption flips to `t("game.surrender.pending")` ("Surrender pending…").
   - `gs.Phase !== "playing" && gs.Phase !== "bidding"` (i.e. paused, disconnected, dealing, hand_scoring, trick_resolving, match_end) — button hidden entirely (mirroring the existing pause-button gate that hides the button outside those phases).
     And clicking the button when enabled opens an inline confirmation dialog (mirror Story 8.1's `kickConfirm` shadcn `AlertDialog` pattern — `client/src/shared/components/ui/alert-dialog.tsx` exists per Story 8.1). Title: `t("game.surrender.confirm.title")`, body: `t("game.surrender.confirm.body")`, buttons: `t("game.surrender.confirm.confirm")` (primary, sends `action:surrender_request`) and `t("game.surrender.confirm.cancel")` (ghost). The confirm-dialog button gets `data-testid="surrender-confirm"`, cancel `data-testid="surrender-cancel"`.
     And on receiving `error:surrender_exhausted` from the server (defence in depth; the UI gates already prevent it), `toast.error(t("game.surrender.errors.exhausted"))` fires (mirroring the existing error-toast pattern at [useWsDispatch.ts](client/src/shared/hooks/useWsDispatch.ts)).

9. **Match-end overlay shows surrender context**
   Given the match ends via accepted surrender,
   When `event:match_end` with `outcomeReason === "surrender"` arrives,
   Then the **existing `MatchResult` component** ([client/src/features/game/components/MatchResult.tsx](client/src/features/game/components/MatchResult.tsx)) renders the standard winner overlay (winning team name, final scores, duration, return-to-lobby button — unchanged) **plus** a new sub-line beneath the winner caption: `data-testid="match-result-surrender-note"` with `t("game.matchResult.surrenderNote", { username: <surrenderedByPlayerUsername> })` — e.g. "{{username}} surrendered the match".
   And the sub-line is **only rendered** when `data.outcomeReason === "surrender"`. For natural match-ends the existing layout is unchanged (`outcomeReason` is optional/`undefined`).
   And `surrenderedByPlayer` (the seat index from the payload) is mapped to a username via the current `gameState.players[surrenderedByPlayer]?.username`. If `gameState` has been cleared by then (race), fall back to `t("game.surrender.unknownProposer")` — "your opponent".
   And the `event:match_abandoned` flow (Story 5.5) is **not** affected — surrender is a different terminal event (`event:match_end` with `outcomeReason: "surrender"`), and `MatchAbandonedData` continues to drive the disconnect overlay separately.

10. **WebSocket event contract sync — both files in the same commit**
    Given new WS events are introduced,
    When this story lands,
    Then [server/internal/ws/events.go](server/internal/ws/events.go) gains:

    ```go
    // --- Surrender events (Story 8.2) ---
    const ActionSurrenderRequest = "action:surrender_request"
    const ActionSurrenderAccept  = "action:surrender_accept"
    const ActionSurrenderDecline = "action:surrender_decline"

    const EventSurrenderProposed = "event:surrender_proposed"
    const EventSurrenderDeclined = "event:surrender_declined"

    const ErrorSurrenderExhausted = "error:surrender_exhausted"

    // SurrenderProposedPayload is the typed payload for EventSurrenderProposed events.
    type SurrenderProposedPayload struct {
        ProposerSeat     int    `json:"proposerSeat"`
        ProposerTeam     int    `json:"proposerTeam"`
        ProposerUsername string `json:"proposerUsername"`
        PartnerSeat      int    `json:"partnerSeat"`
    }

    // SurrenderDeclinedPayload is the typed payload for EventSurrenderDeclined events.
    type SurrenderDeclinedPayload struct {
        ProposerSeat  int `json:"proposerSeat"`
        DecliningSeat int `json:"decliningSeat"`
    }
    ```

    And the existing `MatchEndPayload` block (currently broadcast as a `map[string]interface{}` at [manager.go:451-457](server/internal/session/manager.go#L451-L457)) is **typed** as a struct and extended with optional fields:

    ```go
    type MatchEndPayload struct {
        WinnerTeam          int     `json:"winnerTeam"`
        TeamAFinalScore     int     `json:"teamAFinalScore"`
        TeamBFinalScore     int     `json:"teamBFinalScore"`
        MatchDurationSec    int     `json:"matchDurationSec"`
        OutcomeReason       string  `json:"outcomeReason,omitempty"`        // "" (natural) | "surrender"
        SurrenderedByPlayer *int    `json:"surrenderedByPlayer,omitempty"` // seat index, only when outcomeReason == "surrender"
    }
    ```

    The natural-end branch in `broadcastActionResult` switches from the inline `map` to constructing this struct (with `OutcomeReason: ""` / `SurrenderedByPlayer: nil` so JSON omits both via `omitempty`). Existing clients that were reading `winnerTeam`/`teamAFinalScore`/`teamBFinalScore`/`matchDurationSec` continue to work unchanged — additive extension only.
    And [client/src/shared/types/wsEvents.ts](client/src/shared/types/wsEvents.ts) gains the matching constants and types:

    ```ts
    export const ACTION_SURRENDER_REQUEST = "action:surrender_request" as const;
    export const ACTION_SURRENDER_ACCEPT = "action:surrender_accept" as const;
    export const ACTION_SURRENDER_DECLINE = "action:surrender_decline" as const;

    export const EVENT_SURRENDER_PROPOSED = "event:surrender_proposed" as const;
    export const EVENT_SURRENDER_DECLINED = "event:surrender_declined" as const;

    export const ERROR_SURRENDER_EXHAUSTED =
      "error:surrender_exhausted" as const;

    export type SurrenderRequestPayload = Record<string, never>;
    export type SurrenderAcceptPayload = Record<string, never>;
    export type SurrenderDeclinePayload = Record<string, never>;

    export interface SurrenderProposedPayload {
      proposerSeat: number;
      proposerTeam: number;
      proposerUsername: string;
      partnerSeat: number;
    }

    export interface SurrenderDeclinedPayload {
      proposerSeat: number;
      decliningSeat: number;
    }
    ```

    And `MatchEndPayload` gains the optional fields (additive — existing usages compile unchanged):

    ```ts
    export interface MatchEndPayload {
      winnerTeam: number;
      teamAFinalScore: number;
      teamBFinalScore: number;
      matchDurationSec: number;
      outcomeReason?: "surrender";
      surrenderedByPlayer?: number;
    }
    ```

    And both files are updated in the **same commit** (project rule [\_bmad-output/project-context.md#L80, #L286](_bmad-output/project-context.md#L80)).

11. **GameState contract sync — server + client**
    Given the rules engine adds two new persistent fields,
    When this story lands,
    Then [server/internal/game/state.go](server/internal/game/state.go) gains, immediately after the existing pause-state block (lines 98-102):

    ```go
    // Surrender state (Story 8.2)
    SurrenderProposerSeat *int    `json:"surrenderProposerSeat"` // nil when no proposal pending; seat of the proposer otherwise
    SurrenderUsed         [4]bool `json:"surrenderUsed"`         // each seat may initiate a surrender at most once per match
    ```

    And [server/internal/game/types.go](server/internal/game/types.go) gains the action-type constants alongside the existing pause/unpause set:

    ```go
    ActionSurrenderRequest = "surrender_request"
    ActionSurrenderAccept  = "surrender_accept"
    ActionSurrenderDecline = "surrender_decline"
    ```

    And [client/src/shared/types/gameTypes.ts](client/src/shared/types/gameTypes.ts) extends the `GameState` interface with:

    ```ts
    surrenderProposerSeat: number | null;
    surrenderUsed: [boolean, boolean, boolean, boolean];
    ```

    And the `ActionType` union in `gameTypes.ts` adds `"surrender_request" | "surrender_accept" | "surrender_decline"`.
    And [client/src/shared/stores/gameStore.ts](client/src/shared/stores/gameStore.ts) `normalizeGameState` is a no-op for these fields (they default to `null` / `[false, false, false, false]` from the server — Go zero values serialize correctly for both shapes).

12. **i18n keys added to `en.json` and `sr.json` in the same commit**
    Given new copy is introduced,
    When this story lands,
    Then both [client/src/shared/i18n/en.json](client/src/shared/i18n/en.json) and [client/src/shared/i18n/sr.json](client/src/shared/i18n/sr.json) gain a new `game.surrender.*` block:

    ```json
    "surrender": {
      "requestButton": "Surrender",
      "exhausted": "Surrender used",
      "pending": "Surrender pending…",
      "confirm": {
        "title": "Surrender the match?",
        "body": "Your teammate must accept. If they decline, your one surrender attempt is consumed.",
        "confirm": "Propose surrender",
        "cancel": "Cancel"
      },
      "prompt": {
        "title": "Your teammate wants to surrender",
        "body": "{{username}} is proposing to forfeit the match. Accepting ends the match as a win for the opponents.",
        "accept": "Accept surrender",
        "decline": "Decline"
      },
      "opponentBanner": "{{username}} is proposing to surrender",
      "declinedToast": "Surrender declined — play continues",
      "errors": {
        "exhausted": "You have already used your surrender for this match.",
        "actionRequired": "Resolve the pending surrender proposal first.",
        "wrongPhase": "Surrender is only available during a live hand."
      },
      "unknownProposer": "your opponent"
    }
    ```

    And `game.matchResult` gains a new key:

    ```json
    "surrenderNote": "{{username}} surrendered the match"
    ```

    And Serbian-Latin copy uses the **Ekavian** register (matching adjacent keys; cf. Story 7.2 + Story 8.1 — `Pobeda`, `mesto`). Suggested copy (verify register at PR time):

    ```json
    "surrender": {
      "requestButton": "Predaja",
      "exhausted": "Predaja iskorišćena",
      "pending": "Predaja na čekanju…",
      "confirm": {
        "title": "Predati meč?",
        "body": "Tvoj saigrač mora da prihvati. Ako odbije, tvoj jedini pokušaj predaje je iskorišćen.",
        "confirm": "Predloži predaju",
        "cancel": "Otkaži"
      },
      "prompt": {
        "title": "Tvoj saigrač želi da preda meč",
        "body": "{{username}} predlaže da predate meč. Ako prihvatiš, meč se završava kao pobeda protivnika.",
        "accept": "Prihvati predaju",
        "decline": "Odbij"
      },
      "opponentBanner": "{{username}} predlaže predaju",
      "declinedToast": "Predaja odbijena — igra se nastavlja",
      "errors": {
        "exhausted": "Već si iskoristio svoju predaju za ovaj meč.",
        "actionRequired": "Prvo razreši predlog predaje koji je u toku.",
        "wrongPhase": "Predaja je dostupna samo tokom žive ruke."
      },
      "unknownProposer": "protivnik"
    },
    "matchResult": {
      ...
      "surrenderNote": "{{username}} je predao meč"
    }
    ```

    And the `i18n.test.ts` recursive `flattenKeys` parity check at [client/src/shared/i18n/i18n.test.ts](client/src/shared/i18n/i18n.test.ts) must stay green.

13. **Pause + disconnect interaction**
    Given a surrender proposal is pending (`gs.SurrenderProposerSeat != nil`),
    When any player invokes `action:pause` (still allowed by the rules engine — pause works from `PhaseBidding`/`PhasePlaying`),
    Then the pause succeeds **and `gs.SurrenderProposerSeat` is preserved** across the pause/unpause cycle (the proposer-seat pointer is just a state field, not phase-coupled). On unpause, the partner can still accept/decline.
    And on **disconnect** entering `PhaseDisconnected`: the surrender proposal stays pending in the snapshot. On reconnect (`session.reconnect.go`), the partner's WS still receives the latest `event:game_state` with `surrenderProposerSeat` set, and the SurrenderPrompt re-mounts. On reconnect timeout (Story 5.5 abandonment, [reconnect.go:253](server/internal/session/reconnect.go#L253)), `event:match_abandoned` fires as today — surrender state is discarded with the rest of `gs` and the abandoned-match record is persisted with `Status: "abandoned"`, `AbandonedBy: <userID>` exactly as before. **No `surrendered_by` is written** for an abandoned match.
    And on **per-move timer expiry of the active player while a proposal is pending**: the existing auto-play branch in [manager.go:790-810](server/internal/session/manager.go#L789-L810) fires (auto-play a card). The surrender proposal is **NOT** auto-resolved by the timer — it stays pending until the partner explicitly accepts or declines (or the match ends some other way). Surrender does not get its own auto-resolution branch in `handleTimerExpiry` — keep the proposal "soft" and human-driven. (Architecturally cleaner: future timeouts can be added in Phase 3 without coupling to the per-move timer.)

14. **Backward compatibility — existing flows untouched**
    Given the existing game/session/match flows,
    When this story lands,
    Then natural match-end (1001 cross), match-abandonment (Story 5.5), pause (Story 5.1/5.2), reconnect (Story 5.3-5.4), the rules engine's pure-function contract, all existing `Action.Type` handlers, and the existing `event:match_end`/`event:match_abandoned`/`event:game_paused` payload shapes are **unchanged** in behaviour.
    And the existing `MatchEndPayload` consumers (`MatchResult.tsx`, `useWsDispatch.ts:170-180`) continue to function — `outcomeReason` and `surrenderedByPlayer` are additive fields, ignored by code that doesn't read them.
    And the `handleMatchEnd` function signature is unchanged — surrender end-path piggy-backs on the existing `Phase == PhaseMatchEnd` detection at [manager.go:237-239](server/internal/session/manager.go#L237-L239) plus a new `surrenderedBy` argument threaded through (see Task 4).
    And `gs.PauseUsed` semantics are independent from `gs.SurrenderUsed` — using your pause does NOT consume your surrender attempt and vice versa.
    And no existing test changes its assertions: they may need to read new struct fields (default zero value `nil` / `[false×4]`) but should not change expected behaviour.

## Tasks / Subtasks

- [x] **Task 1: Migration — add `surrendered_by` column to `matches` (AC #5)**
  - [x] 1.1 Create [server/migrations/000010_add_match_surrendered_by.up.sql](server/migrations/000010_add_match_surrendered_by.up.sql):
    ```sql
    ALTER TABLE matches ADD COLUMN surrendered_by INTEGER REFERENCES users(id);
    CREATE INDEX idx_matches_surrendered_by ON matches(surrendered_by);
    ```
  - [x] 1.2 Create matching `.down.sql`:
    ```sql
    DROP INDEX IF EXISTS idx_matches_surrendered_by;
    ALTER TABLE matches DROP COLUMN IF EXISTS surrendered_by;
    ```
  - [x] 1.3 Verify next migration number is `000010` (current highest is `000009_create_hand_results`). Per [\_bmad-output/project-context.md#L243-L245](_bmad-output/project-context.md#L243-L245), never skip migration numbers.
  - [x] 1.4 Add `SurrenderedBy *uint \`gorm:"index" json:"surrenderedBy,omitempty"\``to [server/internal/match/model.go](server/internal/match/model.go) immediately after`AbandonedBy`. **Use `\*uint`(pointer)** so unset rows serialise to JSON`null`, matching the `AbandonedBy`precedent — Go's`uint`zero-value`0` would serialise as a real value otherwise (project-context Go rule on pointer optional fields).
  - [x] 1.5 Run `make migrate` locally against the dev DB to verify the migration applies cleanly. Down-migrate then re-up to ensure idempotence.
  - [x] 1.6 Do **not** modify [server/internal/match/gorm_repo.go](server/internal/match/gorm_repo.go) — surrender intentionally keeps `status='completed'` so existing stats and history queries Just Work. Adding any `surrendered_by`-aware filter is **out of scope** for this story (Story 9.6 / honor system will consume it).

- [x] **Task 2: Rules engine — add `surrender.go` + state fields (AC #1, #2, #3, #11, #14)**
  - [x] 2.1 Add to [server/internal/game/types.go](server/internal/game/types.go) alongside the existing action constants (lines 95-106):
    ```go
    ActionSurrenderRequest = "surrender_request"
    ActionSurrenderAccept  = "surrender_accept"
    ActionSurrenderDecline = "surrender_decline"
    ```
  - [x] 2.2 Add to [server/internal/game/state.go](server/internal/game/state.go) immediately after the existing pause-state block (after line 102):
    ```go
    // Surrender state (Story 8.2)
    SurrenderProposerSeat *int    `json:"surrenderProposerSeat"`
    SurrenderUsed         [4]bool `json:"surrenderUsed"`
    ```
    `NewGame` ([state.go:130-170](server/internal/game/state.go#L130)) needs no edits — Go zero values (`nil`, `[4]bool{}`) are already correct defaults.
  - [x] 2.3 Create new file `server/internal/game/surrender.go` mirroring the structure of [pause.go](server/internal/game/pause.go) — three pure handlers, no side effects, clone state before mutation:

    ```go
    func handleSurrenderRequest(state *GameState, action Action) (*GameState, error) {
        // Validate phase: only PhaseBidding and PhasePlaying allow surrender
        if state.Phase != PhasePlaying && state.Phase != PhaseBidding {
            return nil, apperr.ErrWrongPhase
        }
        seat := action.PlayerSeat
        if seat < 0 || seat > 3 {
            return nil, apperr.ErrBadRequest
        }
        if state.SurrenderProposerSeat != nil {
            return nil, apperr.ErrActionRequired // a proposal is already pending
        }
        if state.SurrenderUsed[seat] {
            return nil, apperr.ErrSurrenderExhausted // new — Task 3
        }
        newState := cloneGameState(state)
        seatCopy := seat
        newState.SurrenderProposerSeat = &seatCopy
        newState.SurrenderUsed[seat] = true
        return newState, nil
    }

    func handleSurrenderAccept(state *GameState, action Action) (*GameState, error) {
        if state.SurrenderProposerSeat == nil {
            return nil, apperr.ErrWrongPhase
        }
        seat := action.PlayerSeat
        if seat < 0 || seat > 3 {
            return nil, apperr.ErrBadRequest
        }
        proposer := *state.SurrenderProposerSeat
        partner := (proposer + 2) % 4
        if seat != partner {
            return nil, apperr.ErrInvalidAction // not the partner
        }
        newState := cloneGameState(state)
        opponentTeam := 1 - TeamForSeat(proposer) // 0↔1
        newState.WinnerTeam = &opponentTeam
        newState.Phase = PhaseMatchEnd
        newState.SurrenderProposerSeat = nil
        newState.TurnExpiresAt = nil
        newState.TurnTimeRemaining = 0
        return newState, nil
    }

    func handleSurrenderDecline(state *GameState, action Action) (*GameState, error) {
        if state.SurrenderProposerSeat == nil {
            return nil, apperr.ErrWrongPhase
        }
        seat := action.PlayerSeat
        if seat < 0 || seat > 3 {
            return nil, apperr.ErrBadRequest
        }
        proposer := *state.SurrenderProposerSeat
        partner := (proposer + 2) % 4
        if seat != partner {
            return nil, apperr.ErrInvalidAction
        }
        newState := cloneGameState(state)
        newState.SurrenderProposerSeat = nil
        // SurrenderUsed[proposer] is already true — preserved (consumed even on decline)
        return newState, nil
    }
    ```

  - [x] 2.4 Wire the three handlers into [server/internal/game/rules_engine.go](server/internal/game/rules_engine.go) `ApplyAction`. Insert **immediately after the pause/unpause/owner_unpause early-return block (after line 26, before the phase switch)** so surrender actions are phase-checked inside their own handlers (allowing accept/decline to clear a proposal even when the rules engine would otherwise reject the phase):
    ```go
    if action.Type == ActionSurrenderRequest {
        return handleSurrenderRequest(state, action)
    }
    if action.Type == ActionSurrenderAccept {
        return handleSurrenderAccept(state, action)
    }
    if action.Type == ActionSurrenderDecline {
        return handleSurrenderDecline(state, action)
    }
    ```
    **Critical**: do NOT place these branches inside `case PhasePlaying`/`case PhaseBidding` — keep them at the same level as pause/unpause. This preserves the rule "the partner can decline even during a phase the rules engine would normally block" (e.g. between hands in `PhaseHandScoring`, when a proposal is somehow still pending — defence in depth).
  - [x] 2.5 Verify [cloneGameState](server/internal/game/clone.go) (or wherever the clone helper lives) deep-copies the new fields. `SurrenderUsed [4]bool` is a value-type fixed array (copied by assignment); `SurrenderProposerSeat *int` is a pointer that should be cloned with the same `if p := state.X; p != nil { v := *p; clone.X = &v }` pattern used for `TrumpCallerSeat` and `PendingBelotSeat`. **Find the clone helper first** — grep for `cloneGameState` and confirm the new pointer field is added to its block.

- [x] **Task 3: apperr — add `ErrSurrenderExhausted` (AC #1, #11)**
  - [x] 3.1 Add to [server/internal/apperr/errors.go](server/internal/apperr/errors.go) under "Game domain errors" (after line 92):
    ```go
    ErrSurrenderExhausted = NewAppError("SURRENDER_EXHAUSTED", "player has already used their surrender for this match", http.StatusConflict)
    ErrInvalidAction      = NewAppError("INVALID_ACTION", "action is not valid in this context", http.StatusBadRequest)
    ```
    Wait — `ErrInvalidAction` may not yet exist. Check first: `grep "ErrInvalidAction" server/internal/apperr/errors.go`. The session manager currently maps non-typed errors to `ws.ErrorInvalidAction` (string constant) without a corresponding apperr — see [session/manager.go:683](server/internal/session/manager.go#L683). If no apperr exists, **add `ErrInvalidAction`** so accept/decline by a non-partner can map cleanly through `sendGameError`. If one already exists, reuse it.
  - [x] 3.2 Reuse existing apperrs without modification: `ErrWrongPhase`, `ErrBadRequest`, `ErrActionRequired`. Do NOT introduce a separate "surrender already pending" code — `ErrActionRequired` (`ACTION_REQUIRED`) is the right semantic match; add the human-readable message via the i18n `game.surrender.errors.actionRequired` key.

- [x] **Task 4: Session manager — wire actions, broadcasts, match-end persistence (AC #1, #2, #3, #4, #10, #13, #14)**
  - [x] 4.1 Extend [server/internal/session/manager.go](server/internal/session/manager.go) `parseAction` (around lines 307-368) — surrender actions need no payload decoding, so the existing default path (`actionType` strip-prefix → `game.Action{Type, PlayerSeat}`) handles them automatically. **Verify the auto-mapping**: the WS event types `action:surrender_request`/`action:surrender_accept`/`action:surrender_decline` strip to `surrender_request`/`surrender_accept`/`surrender_decline`, which match the `game.ActionSurrenderRequest`/`Accept`/`Decline` constants exactly. No special-case mapping needed (unlike `decline_belot` → `skip_belot` at line 331).
  - [x] 4.2 Extend `broadcastActionResult` (around lines 374-549) with a new `case` block for the three surrender actions. Place after the `ActionAnnounceBelot, ActionSkipBelot` case:

    ```go
    case game.ActionSurrenderRequest:
        proposerSeat := action.PlayerSeat
        proposerTeam := game.TeamForSeat(proposerSeat)
        proposerUsername := newState.Players[proposerSeat].Username
        partnerSeat := (proposerSeat + 2) % 4
        proposed := ws.SurrenderProposedPayload{
            ProposerSeat:     proposerSeat,
            ProposerTeam:     proposerTeam,
            ProposerUsername: proposerUsername,
            PartnerSeat:      partnerSeat,
        }
        m.hub.BroadcastToUsers(userIDs, buildMessage(ws.EventSurrenderProposed, proposed))
        m.hub.BroadcastToUsers(userIDs, buildMessage(ws.EventGameState, newState))

    case game.ActionSurrenderDecline:
        // The proposer is no longer in newState (cleared); read it from oldState.
        var proposerSeat int
        if oldState.SurrenderProposerSeat != nil {
            proposerSeat = *oldState.SurrenderProposerSeat
        }
        declined := ws.SurrenderDeclinedPayload{
            ProposerSeat:  proposerSeat,
            DecliningSeat: action.PlayerSeat,
        }
        m.hub.BroadcastToUsers(userIDs, buildMessage(ws.EventSurrenderDeclined, declined))
        m.hub.BroadcastToUsers(userIDs, buildMessage(ws.EventGameState, newState))

    case game.ActionSurrenderAccept:
        // event:match_end + event:game_state are emitted via the existing PhaseMatchEnd
        // detection block (see HandleAction lines 237-239 → handleMatchEnd) — do NOT
        // duplicate them here. Falling through to the default would broadcast game_state
        // before match_end, breaking the announcement→state ordering established by
        // the natural-end path. Explicit no-op here.
    ```

  - [x] 4.3 In `HandleAction` (around line 237), keep the existing `if newState.Phase == game.PhaseMatchEnd { m.handleMatchEnd(...) }` path, but **broadcast `event:match_end` from inside `handleMatchEnd`** (or just before invoking it) for the surrender path so the ordering is `event:match_end` → `event:game_state` (mirroring [manager.go:450-465](server/internal/session/manager.go#L450-L465)). Refactor option: extract a new helper `broadcastMatchEnd(playerIDs, newState, oldState, action, startedAt) []byte` that emits the `match_end` payload (with `outcomeReason: "surrender"` if the triggering action was `ActionSurrenderAccept`). Call it from both the inline natural-end branch in `broadcastActionResult` and the surrender-accept path. **Preserve the natural-end ordering**: today the natural path emits `event:match_end` BEFORE `event:game_state` from inside `case game.ActionPlayCard`; the refactor must keep that contract.
  - [x] 4.4 Convert the inline `map[string]interface{}` payload at [manager.go:451-457](server/internal/session/manager.go#L451-L457) into the typed `ws.MatchEndPayload` struct (per AC #10). The natural-end branch sets `OutcomeReason: ""` (omitted via `omitempty`) and `SurrenderedByPlayer: nil`. The surrender branch sets `OutcomeReason: "surrender"` and `SurrenderedByPlayer: &proposerSeat`.
  - [x] 4.5 Extend `handleMatchEnd` ([manager.go:632-674](server/internal/session/manager.go#L632)) to accept (or read from `finalState`) the surrendering user's ID and persist it. Two clean shapes:
    - **Option A (recommended)**: derive surrenderedByUserID inside `handleMatchEnd` by inspecting whether `finalState.Phase == PhaseMatchEnd && oldState.SurrenderProposerSeat != nil` — but `oldState` isn't available here. Instead pass an explicit `surrenderedBy *uint` argument and update the call sites at [manager.go:238](server/internal/session/manager.go#L238) (natural end → nil) and the new surrender-accept path (the proposer's userID).
    - **Option B**: stash `proposerSeat` into a new transient session field `session.lastSurrenderProposerSeat *int` during `HandleAction` for `ActionSurrenderAccept` then read it in `handleMatchEnd`. Less explicit. **Option A is preferred.**
    - In either shape, set `matchRecord.SurrenderedBy = surrenderedByUserID` before `m.matchRepo.CreateWithHands(...)`.
    - `matchRecord.Status = "completed"` for surrender (NOT `"surrendered"` — see AC #5 rationale).
  - [x] 4.6 Add `sendGameError` mappings ([manager.go:683-702](server/internal/session/manager.go#L683-L702)):
    ```go
    case errors.Is(err, apperr.ErrSurrenderExhausted):
        eventType = ws.ErrorSurrenderExhausted
    ```
    `ErrActionRequired` and `ErrInvalidAction` fall through to the default `ws.ErrorInvalidAction` mapping (acceptable — message text comes from the apperr; the client distinguishes by `code` field on the typed error event payload, see AC #8 toast logic).

- [x] **Task 5: WS event constants — server + client (AC #10)**
  - [x] 5.1 Add the constants and payload structs to [server/internal/ws/events.go](server/internal/ws/events.go) per AC #10 verbatim. Place under a new comment header `// --- Surrender events (Story 8.2) ---` between the existing `// --- Disconnect/reconnect events ---` block and `// --- Game error events ---` (so related error consts stay grouped near their owners).
  - [x] 5.2 Add the `MatchEndPayload` struct definition (currently the `event:match_end` payload is constructed inline as a `map`; this story types it). Place it next to the existing `MatchAbandonedPayload` struct. **All four existing fields keep their JSON tags exactly** so wire-format is unchanged for natural ends.
  - [x] 5.3 Mirror in [client/src/shared/types/wsEvents.ts](client/src/shared/types/wsEvents.ts) per AC #10 verbatim. Place near `MatchEndPayload` (line 109) and `MatchAbandonedPayload` (line 165).
  - [x] 5.4 **Same-commit rule** ([\_bmad-output/project-context.md#L80, #L286](_bmad-output/project-context.md#L80)): both files updated together; no drift between commits.

- [x] **Task 6: Frontend — store, dispatch, GamePage wiring (AC #6, #7, #8, #9, #11)**
  - [x] 6.1 Extend [client/src/shared/types/gameTypes.ts](client/src/shared/types/gameTypes.ts) `GameState` interface with the two new fields (per AC #11). Add `"surrender_request" | "surrender_accept" | "surrender_decline"` to the `ActionType` union.
  - [x] 6.2 Extend [client/src/shared/stores/gameStore.ts](client/src/shared/stores/gameStore.ts):
    - Add `surrenderProposed: SurrenderProposedPayload | null` and `surrenderDeclined: SurrenderDeclinedPayload | null` to `GameStoreState`, with corresponding setters `setSurrenderProposed`, `setSurrenderDeclined`. Mirror the existing `belotReveal`/`setBelotReveal` shape.
    - Include both new fields in `initialState` so `reset()`/`clearGame()` clear them.
    - **Note**: `pendingSurrenderProposerSeat` and `surrenderUsed` are NOT separate store fields — they live on the `gameState` itself (just like `pendingBelotSeat` and `pauseUsed`). The new store fields are for the transient toast/banner triggers, mirroring `belotReveal`.
  - [x] 6.3 Extend [client/src/shared/hooks/useWsDispatch.ts](client/src/shared/hooks/useWsDispatch.ts) with three new branches in `dispatchGameEvent` (next to `EVENT_BELOT_ANNOUNCED` and `EVENT_MATCH_ABANDONED`):

    ```ts
    if (type === EVENT_SURRENDER_PROPOSED) {
      const payload = message.payload as SurrenderProposedPayload;
      store.setSurrenderProposed(payload);
      // Full game state update follows via event:game_state
      return;
    }

    if (type === EVENT_SURRENDER_DECLINED) {
      const payload = message.payload as SurrenderDeclinedPayload;
      store.setSurrenderDeclined(payload);
      toast.info(i18n.t("game.surrender.declinedToast"), { duration: 3000 });
      // Full game state update follows via event:game_state — clears surrenderProposerSeat
      return;
    }
    ```

    Add a branch for `ERROR_SURRENDER_EXHAUSTED` in the error-event dispatcher near the existing `ERROR_PAUSE_EXHAUSTED` branch:

    ```ts
    if (type === ERROR_SURRENDER_EXHAUSTED) {
      toast.error(i18n.t("game.surrender.errors.exhausted"));
      return;
    }
    ```

    Find the existing error-dispatch helper (`dispatchErrorEvent` per the file's structure) and add the case in the matching switch/if-chain. **Do NOT** add a generic toast for every error type — only the existing pattern for known codes.

  - [x] 6.4 Extend the `EVENT_MATCH_END` branch in `useWsDispatch.ts` (around [line 170](client/src/shared/hooks/useWsDispatch.ts#L170)) to read the two new optional fields off the payload and set them on the store via the existing `setMatchEndData(payload)` call (no signature change — `payload` is now the extended `MatchEndPayload` shape).
  - [x] 6.5 In [client/src/features/game/GamePage.tsx](client/src/features/game/GamePage.tsx):
    - Import `ACTION_SURRENDER_REQUEST`, `ACTION_SURRENDER_ACCEPT`, `ACTION_SURRENDER_DECLINE`.
    - Add three action handlers next to `handlePause` (around line 262):
      ```ts
      const handleSurrenderRequest = useCallback(() => {
        sendMessage(ACTION_SURRENDER_REQUEST, {});
      }, [sendMessage]);
      const handleSurrenderAccept = useCallback(() => {
        sendMessage(ACTION_SURRENDER_ACCEPT, {});
      }, [sendMessage]);
      const handleSurrenderDecline = useCallback(() => {
        sendMessage(ACTION_SURRENDER_DECLINE, {});
      }, [sendMessage]);
      ```
    - Compute four derived flags near the existing `canPause`/`isMyTurn` block (around line 328-360):
      ```ts
      const isProposer =
        gameState.surrenderProposerSeat !== null &&
        gameState.surrenderProposerSeat === myPlayerSeat;
      const isPartnerOfProposer =
        gameState.surrenderProposerSeat !== null &&
        myPlayerSeat !== null &&
        (gameState.surrenderProposerSeat + 2) % 4 === myPlayerSeat;
      const isOpponentOfProposer =
        gameState.surrenderProposerSeat !== null &&
        myPlayerSeat !== null &&
        !isProposer &&
        !isPartnerOfProposer;
      const canSurrenderRequest =
        myPlayerSeat !== null &&
        (gameState.phase === "playing" || gameState.phase === "bidding") &&
        !gameState.surrenderUsed?.[myPlayerSeat] &&
        gameState.surrenderProposerSeat === null;
      ```
    - Render the new `SurrenderButton` (Task 7) in the bottom-left, sibling to the existing pause button. Wrap the two in a `<div className="absolute bottom-4 left-4 z-10 flex flex-col gap-2">` so they stack vertically. Move the existing pause-button absolute positioning into the wrapper.
    - Render the new `SurrenderPrompt` (Task 7) when `isPartnerOfProposer` and `gameState.surrenderProposerSeat !== null`. Pass the proposer's username (`gameState.players[gameState.surrenderProposerSeat].username`) and the two action handlers.
    - Render the new `SurrenderOpponentBanner` (Task 7) when `isOpponentOfProposer`. Pass the proposer's username from `gameState.players`.
    - Extend `MatchResult` props by passing through `outcomeReason` and `surrenderedByUsername` derived from the existing `matchEndData.surrenderedByPlayer` and `gameState.players`.

- [x] **Task 7: Frontend — three new components (AC #6, #7, #8, #9)**
  - [x] 7.1 Create `client/src/features/game/components/SurrenderButton.tsx` — mirror the inline pause-button at [GamePage.tsx:451-463](client/src/features/game/GamePage.tsx#L451-L463) but extracted as a proper component (cleaner than inline because it carries the confirmation dialog). Props: `{ canRequest: boolean; isExhausted: boolean; isPending: boolean; onConfirm: () => void; }`. Internal state: `useState<boolean>(false)` for `confirmOpen`. On click: `setConfirmOpen(true)`. Confirm dialog: shadcn `AlertDialog` (already installed by Story 8.1 — verify with `ls client/src/shared/components/ui/alert-dialog.tsx`). `data-testid="surrender-button"` on the trigger; `data-testid="surrender-confirm"` and `data-testid="surrender-cancel"` on dialog buttons.
        Caption logic:
    ```ts
    const caption = isPending
      ? t("game.surrender.pending")
      : isExhausted
        ? t("game.surrender.exhausted")
        : t("game.surrender.requestButton");
    const isDisabled = !canRequest;
    ```
  - [x] 7.2 Create `client/src/features/game/components/SurrenderPrompt.tsx` — **mirror [BelotPrompt.tsx](client/src/features/game/components/BelotPrompt.tsx)** verbatim, swapping props/i18n keys. Props: `{ proposerUsername: string; onAccept: () => void; onDecline: () => void; }`. Use the same `useFocusTrap`, `role="dialog" aria-modal="true"`, surface-elevated card with two buttons. `data-testid="surrender-prompt"` on the wrapper; `data-testid="surrender-prompt-accept"` and `data-testid="surrender-prompt-decline"` on the buttons.
  - [x] 7.3 Create `client/src/features/game/components/SurrenderOpponentBanner.tsx` — a slim non-modal banner. Props: `{ proposerUsername: string; }`. Render:
    ```tsx
    <div
      className="absolute top-12 left-1/2 -translate-x-1/2 z-30 bg-surface-elevated border border-border rounded-md px-4 py-1.5 motion-safe:animate-in motion-safe:fade-in motion-safe:duration-200"
      data-testid="surrender-opponent-banner"
      role="status"
      aria-live="polite"
    >
      <span className="text-text-primary font-body text-sm">
        {t("game.surrender.opponentBanner", { username: proposerUsername })}
      </span>
    </div>
    ```
    Important: NOT `role="dialog"` — banner must not trap focus. Opponents must be able to keep playing.
  - [x] 7.4 Extend [MatchResult.tsx](client/src/features/game/components/MatchResult.tsx):
    - Add optional props: `surrenderedByUsername?: string` (the resolved username for `data.surrenderedByPlayer`).
    - When `data.outcomeReason === "surrender"` AND `surrenderedByUsername` is non-empty, render an additional `<p data-testid="match-result-surrender-note">` beneath the winner caption with `t("game.matchResult.surrenderNote", { username: surrenderedByUsername })`.
    - The fallback when `surrenderedByUsername` is undefined: `t("game.surrender.unknownProposer")`.
    - All existing layout/test-ids/strings remain unchanged for natural-end matches.

- [x] **Task 8: Backend tests — rules engine + session manager (AC #1-#5, #13, #14)**
  - [x] 8.1 Create `server/internal/game/surrender_test.go` mirroring [pause_test.go](server/internal/game/pause_test.go) (verify the file exists; if not, look for the closest analogue). Use the `internal/game/testfixtures/` helpers (per project-context: never use raw `GameState{}` literals). Cases:
    - **Happy path: request from PhasePlaying** — non-pending, used flag was false → state has `SurrenderProposerSeat = &seat`, `SurrenderUsed[seat] = true`, phase unchanged.
    - **Happy path: request from PhaseBidding** — same assertions.
    - **Reject: request from PhaseDealing / PhaseHandScoring / PhaseTrickResolving** → `ErrWrongPhase`.
    - **Reject: request from PhasePaused** → `ErrGamePaused` (the rules engine pre-empts surrender via the existing `case PhasePaused: return ErrGamePaused` at [rules_engine.go:36](server/internal/game/rules_engine.go#L36); confirm by reading the actual fall-through path — if surrender is intercepted before phase-switch, the test must instead assert the surrender handler's own phase rejection. Adjust based on Task 2.4 placement).
    - **Reject: request when `SurrenderUsed[seat] == true`** → `ErrSurrenderExhausted`.
    - **Reject: request when proposal already pending** → `ErrActionRequired`.
    - **Happy path: accept by partner** → `Phase = PhaseMatchEnd`, `WinnerTeam = &(opposite team)`, `SurrenderProposerSeat = nil`, `SurrenderUsed` preserved. Assert opposite team correctly: proposer seat 0 (Team A) → winner team 1 (Team B); seat 1 → 0; seat 2 → 1; seat 3 → 0.
    - **Reject: accept by non-partner** (proposer themselves, or either opponent) → `ErrInvalidAction`.
    - **Reject: accept when no proposal** → `ErrWrongPhase`.
    - **Happy path: decline by partner** → `SurrenderProposerSeat = nil`, `SurrenderUsed[proposer] == true` preserved, phase / scores / hands / trick unchanged.
    - **Reject: decline by non-partner** → `ErrInvalidAction`.
    - **Reject: decline when no proposal** → `ErrWrongPhase`.
    - **Pure-function discipline**: each case clones state and asserts the input state is **not mutated** (slice clone discipline per project-context Go rule "Rules engine must clone slices before mutation"). Re-applying `ApplyAction` to the original state with the same action yields the same result deterministically.
    - **Cross-state isolation**: assert that surrender does not touch `gs.PauseUsed`, `gs.PausedPlayers`, `gs.TeamScores`, `gs.HandPoints`, `gs.TricksWon`, or any `Players[i].Hand`. Lock in independence from pause/score/hand state.
  - [x] 8.2 Extend [server/internal/session/manager_test.go](server/internal/session/manager_test.go) with end-to-end surrender scenarios. Mirror the `TestHandleAction_BiddingActions` structure ([manager_test.go:211](server/internal/session/manager_test.go#L211)) — drive the session via raw `WSMessage{Type: "action:surrender_request", Payload: []byte("{}")}`, then assert:
    - **Happy path: full request → accept → match-end → persistence**:
      1. Setup: `mgr.StartGame(...)` with `defaultPlayers()`, advance state to bidding by pick_trump call (or use a `state.go` helper from `testfixtures` if one exists).
      2. Player at seat 0 sends `action:surrender_request` → assert `mockHub` received `event:surrender_proposed` (one message) and `event:game_state` (one message), in that order.
      3. Player at seat 2 (partner) sends `action:surrender_accept` → assert `event:match_end` received with `OutcomeReason == "surrender"`, `SurrenderedByPlayer == &0`, `WinnerTeam == 1`, AND `event:game_state` received with `Phase == match_end`.
      4. `mockMatchRepo.lastCreated` (the in-memory mock from existing tests) has `Status == "completed"`, `WinnerTeam == 1`, `SurrenderedBy != nil && *SurrenderedBy == playerIDs[0]`.
      5. `mgr.HasSession(roomID) == false` (session removed).
      6. Buffered hand results (if any from completed hands before surrender) are flushed in the same `CreateWithHands` call.
    - **Decline path**: request → decline → state assertions (proposer attempt consumed, no match-end, session still active, no match record persisted).
    - **Exhausted path**: request → decline → second request from same seat → `error:surrender_exhausted` to that user only; no broadcast.
    - **Already-pending path**: seat 0 requests, seat 1 requests before partner resolves → seat 1 receives `error:invalid_action` (ACTION_REQUIRED); seat 0's proposal is unaffected; `gs.SurrenderProposerSeat` still points to seat 0.
    - **Non-partner accept**: seat 0 requests, seat 1 (opponent) sends accept → `error:invalid_action`; proposal still pending.
    - **Pause interaction**: seat 0 requests, seat 1 pauses, owner unpauses → `gs.SurrenderProposerSeat` still points to seat 0; partner can then accept.
    - **Disconnect interaction**: seat 0 requests, seat 2 disconnects → `gs.SurrenderProposerSeat` still set in the snapshot delivered on reconnect (use `mgr.GetStateSnapshot(roomID)` at the appropriate moment).
    - **Reconnect-timeout vs surrender**: seat 0 requests, seat 2 disconnects, reconnect window elapses → `event:match_abandoned` fires (existing path) and the abandoned-match record has `Status == "abandoned"`, `AbandonedBy != nil`, AND `SurrenderedBy == nil` (surrender state is discarded by abandonment, not persisted).
    - **Per-move timer interaction**: with a per-move timer active, seat 0 (active) requests; the partner does NOT respond; the active player's per-move timer fires → auto-play executes; assert the surrender proposal is **still pending** after auto-play (the timer does not auto-resolve surrender).
  - [x] 8.3 Use `mockMatchRepo` (existing in [manager_test.go](server/internal/session/manager_test.go)) and add `SurrenderedBy` to its captured-fields list if not already present (it must be, since `match.Match` has the new field — assert via `lastCreated.SurrenderedBy`). The mock's `CreateWithHands` already captures the full struct.

- [x] **Task 9: Frontend tests — components + dispatch + GamePage (AC #6-#11)**
  - [x] 9.1 Create `client/src/features/game/components/SurrenderButton.test.tsx`:
    - `"renders disabled state when surrender used"` — pass `isExhausted={true}`, assert button is disabled and caption matches `t("game.surrender.exhausted")`.
    - `"renders disabled state when proposal pending"` — pass `isPending={true}`, caption matches `t("game.surrender.pending")`.
    - `"opens confirm dialog on click and fires onConfirm"` — click `surrender-button`, assert `surrender-confirm` and `surrender-cancel` are present; click `surrender-confirm`, assert `onConfirm` mock called; cancel does not.
    - `"hidden when canRequest=false (rendered as null)"` — confirm component returns `null` when `canRequest` is false AND not exhausted/pending (i.e. wrong phase).
  - [x] 9.2 Create `client/src/features/game/components/SurrenderPrompt.test.tsx` mirroring [BelotPrompt.test.tsx](client/src/features/game/components/BelotPrompt.test.tsx):
    - `"renders accept and decline buttons with proposer username"`.
    - `"calls onAccept when accept clicked"`.
    - `"calls onDecline when decline clicked"`.
    - `"focus-traps within the dialog"` (mirror BelotPrompt's focus-trap test if one exists; otherwise skip).
  - [x] 9.3 Create `client/src/features/game/components/SurrenderOpponentBanner.test.tsx`:
    - `"renders the proposer username and aria-live status"` — assert `role="status"`, `aria-live="polite"`, and the i18n key resolves with the username interpolated.
    - **Anti-modal assertion**: assert the rendered element has NO `role="dialog"` and no focus-trap behaviour (e.g. tab from the banner does not get stuck inside it).
  - [x] 9.4 Extend `client/src/features/game/components/MatchResult.test.tsx` with two cases:
    - `"renders surrender note when outcomeReason is 'surrender'"` — pass `data` with `outcomeReason: "surrender"` and `surrenderedByPlayer: 1`; pass `surrenderedByUsername: "alice"`; assert `match-result-surrender-note` is in the document with the resolved string.
    - `"does NOT render surrender note for natural match-end"` — pass `data` with no `outcomeReason`; assert `match-result-surrender-note` is NOT in the document.
  - [x] 9.5 Extend [client/src/shared/hooks/useWsDispatch.test.ts](client/src/shared/hooks/useWsDispatch.test.ts):
    - `"dispatches event:surrender_proposed to gameStore.setSurrenderProposed"`.
    - `"dispatches event:surrender_declined to gameStore.setSurrenderDeclined and shows declined toast"`.
    - `"shows exhausted toast on error:surrender_exhausted"`.
    - `"propagates outcomeReason through event:match_end into setMatchEndData"`.
  - [x] 9.6 Extend [client/src/features/game/GamePage.test.tsx](client/src/features/game/GamePage.test.tsx) (verify the file path) with integration cases:
    - `"shows SurrenderButton in playing phase, hides in match_end"`.
    - `"shows SurrenderPrompt for the partner when surrenderProposerSeat is set"`.
    - `"shows SurrenderOpponentBanner for opponents when surrenderProposerSeat is set"`.
    - `"hides SurrenderPrompt when surrenderProposerSeat clears"` (decline → cleared) — fixture: re-render with `surrenderProposerSeat: null`.
    - `"sends action:surrender_request after confirm dialog"` — click button → confirm → assert `sendMessage` mock called with `ACTION_SURRENDER_REQUEST`.

- [x] **Task 10: i18n updates (AC #12)**
  - [x] 10.1 Extend [client/src/shared/i18n/en.json](client/src/shared/i18n/en.json) under the existing `game` block. Add the full `game.surrender.*` block per AC #12. Add `game.matchResult.surrenderNote`. Verify nesting depth matches existing `game.belot.*` structure.
  - [x] 10.2 Mirror in [client/src/shared/i18n/sr.json](client/src/shared/i18n/sr.json) using the **Ekavian** Serbian-Latin register. Verify register matches Story 8.1's `Pobeda`/`Procenat pobeda` and Story 7.2's `Pobede`.
  - [x] 10.3 Run `npx vitest run i18n` — the recursive `flattenKeys` parity check at [client/src/shared/i18n/i18n.test.ts](client/src/shared/i18n/i18n.test.ts) must stay green.

- [x] **Task 11: Full-stack smoke + lint gates (run before marking the story `review`)**
  - [x] 11.1 Backend: `cd server && go test ./...` — all packages green. `cd server && go vet ./...` clean.
  - [x] 11.2 Frontend: `cd client && npx vitest run` — all tests green (existing + all new component, dispatch, and GamePage cases).
  - [x] 11.3 **Lint: `cd client && npx prettier --write . && npx eslint .` — Prettier MUST run before committing** ([feedback memory: `feedback_prettier_before_commit.md`](C:\Users\Emilijan-LT.claude\projects\d--My-Projects-beljot\memory\feedback_prettier_before_commit.md); CI has failed repeatedly across Stories 7.1, 7.2, 8.1).
  - [x] 11.4 `make lint` (both stacks) — clean. On Windows shells without `golangci-lint` installed, fall back to `go vet ./...` as the static-analysis gate (matching Stories 7.2 + 8.1 Dev Agent Record practice).
  - [x] 11.5 Migration round-trip: `make migrate` (up) → check schema → `make migrate down` → check schema reverts → `make migrate` (up) again. Verify `surrendered_by` column and index appear/disappear cleanly.
  - [x] 11.6 Manual smoke (document outcomes in Completion Notes):
    - Start `make dev`; create a 4-player match. As seat 0, click Surrender → confirm; assert seats 1 + 3 (opponents) see the banner, seat 2 (partner) sees the prompt, seat 0 sees "Surrender pending…".
    - Have seat 2 decline → assert toast on all 4 clients, banner/prompt clear, seat 0's button now shows "Surrender used" and is disabled, seat 2's button is still enabled (their attempt wasn't consumed).
    - Have seat 1 (opposite team) request a surrender — partner is seat 3. Have seat 3 accept → assert MatchResult renders with `match-result-surrender-note` showing seat 1's username. Verify the matches table has a new row with `status='completed'`, `winner_team=0` (Team A won because seat 1 surrendered), `surrendered_by=<seat-1-userId>`.
    - Disconnect during a pending proposal: seat 0 proposes, seat 2 disconnects → wait for reconnect window → assert `event:match_abandoned` fires (existing flow), match row written with `status='abandoned'`, `surrendered_by=NULL`.
    - Pause/unpause during pending proposal: seat 0 proposes, seat 1 pauses, owner unpauses → assert proposal still pending after unpause; partner can still accept.

### Review Findings

_Code review run on 2026-04-27 by `bmad-code-review` (Blind Hunter + Edge Case Hunter + Acceptance Auditor)._

**Decision-needed (resolved 2026-04-27):**

- [x] `[Review/Decision]` Pending-proposal cleanup on disconnect/reconnect-timeout — **Resolved: option 1 (clear in `HandleDisconnect`)**. Promoted to Patch P16 below.
- [x] `[Review/Decision]` AC#1 vs AC#10 internal contradiction on `SurrenderProposedPayload.opponentSeats[2]` — **Resolved: option 1 (ratify AC#10, current code stays; amend AC#1)**. Promoted to Patch P17 below.
- [x] `[Review/Decision]` Wire vs DB naming collision — **Resolved: option 1 (rename wire field to `surrenderedBySeat`)**. Promoted to Patch P18 below.

**Patch (apply now):**

- [x] `[Review/Patch]` [HIGH] Surrender during `PhasePaused` returns `ErrWrongPhase` instead of `ErrGamePaused` — violates AC#1 ("PhasePaused rejection mirrors the existing rule"). The dispatcher routes surrender ahead of the phase switch, so the `PhasePaused → ErrGamePaused` rule never fires for surrender. Add explicit `if state.Phase == PhasePaused { return nil, apperr.ErrGamePaused }` in `handleSurrenderRequest` (and accept/decline). [server/internal/game/rules_engine.go:33-41, server/internal/game/surrender.go:14-16]
- [x] `[Review/Patch]` [HIGH] Active player's turn timer is reset on every successful `surrender_request`/`surrender_decline` — violates AC#1 ("the per-move turn timer keeps running on the active player"). `HandleAction` falls through to `setTurnExpiry`/`startTimerLocked` after the action commits. Skip timer rebase for surrender request/decline; preserve `oldState.TurnExpiresAt`/`TurnTimeRemaining` on `newState`. [server/internal/session/manager.go:218-221]
- [x] `[Review/Patch]` [HIGH] Missing GamePage integration tests from Task 9.6 — five named cases in the spec are not implemented (only fixture stubs added): "shows SurrenderButton in playing phase, hides in match_end", "shows SurrenderPrompt for the partner when surrenderProposerSeat is set", "shows SurrenderOpponentBanner for opponents when surrenderProposerSeat is set", "hides SurrenderPrompt when surrenderProposerSeat clears", "sends action:surrender_request after confirm dialog". [client/src/features/game/GamePage.test.tsx]
- [x] `[Review/Patch]` [MEDIUM] `SurrenderPrompt` remains mounted under match-end / match-abandoned overlays — if `gameState.surrenderProposerSeat` is still set when `MatchResult` (z-50) renders, the focus trap on the hidden prompt swallows Tab navigation to "Return to lobby". Gate render with `&& matchEndData === null && matchAbandonedData === null` (or clear the proposer in scoring/abandonment paths server-side). [client/src/features/game/GamePage.tsx:506-510]
- [x] `[Review/Patch]` [MEDIUM] `MatchResult` surrender username uses `gameState.players[seatIndex]` (array index), inconsistent with `players.find((p) => p.seat === ...)` used elsewhere in the same file. Will misresolve if `players[]` is ever unsorted (reconnect race). [client/src/features/game/GamePage.tsx:355-360]
- [x] `[Review/Patch]` [MEDIUM] Migration `CREATE INDEX idx_matches_surrendered_by` lacks `IF NOT EXISTS` — partial-apply retries fail. Down side already uses `IF EXISTS`. [server/migrations/000010_add_match_surrendered_by.up.sql:2]
- [x] `[Review/Patch]` [MEDIUM] `useWsDispatch.test.ts` `error:surrender_exhausted` case asserts no-crash but does not assert `toast.error` was called with the exhausted i18n key. Add a `toastErrorMock` assertion to close the coverage gap from Task 9.5. [client/src/shared/hooks/useWsDispatch.test.ts:761-771]
- [x] `[Review/Patch]` [MEDIUM] Manager tests verify state changes but never assert that the rejection WS error events (`error:invalid_action`, `error:surrender_exhausted`) were dispatched to the offending user — Task 8.2 listed the wire-level assertions explicitly. Add `mockHub` SendToUser assertions on rejection paths. [server/internal/session/manager_test.go] — **moved to defer**: requires either (a) refactoring `session.Manager` to take a `Hub` interface so a recorder can substitute, or (b) wiring the existing `httptest.Server` + real WS-client pattern from `server/internal/ws/ws_test.go` into `session_test`. Both are larger than a per-test fix; the apperr-level rejection mapping is already exercised via `surrender_test.go` and `sendGameError`'s switch.
- [x] `[Review/Patch]` [LOW] Decline broadcast emits `SurrenderDeclinedPayload.proposerSeat = -1` when `oldState.SurrenderProposerSeat` is unexpectedly nil. Skip the broadcast (or use `assert`/log) instead of shipping `-1` over the wire. [server/internal/session/manager.go:565-571]
- [x] `[Review/Patch]` [LOW] `SurrenderButton` caption: `isPending` short-circuits before `isExhausted`, so a player who already used their attempt sees "Surrender pending…" instead of "Surrender used" while a partner's proposal is open. Flip the ternary so `isExhausted` takes priority. [client/src/features/game/components/SurrenderButton.tsx]
- [x] `[Review/Patch]` [LOW] `SurrenderButton` confirm dialog stays open if a partner's `event:surrender_proposed` arrives mid-confirm; clicking Confirm sends a doomed action. Add `useEffect(() => { if (isPending || isExhausted) setConfirmOpen(false); }, [isPending, isExhausted])`. [client/src/features/game/components/SurrenderButton.tsx]
- [x] `[Review/Patch]` [LOW] `error:surrender_exhausted` early-returns in `dispatchErrorEvent` before reaching `setLastError`, so `gameStore.lastError` never reflects the event. Call `useGameStore.getState().setLastError(message.type)` before the early return. [client/src/shared/hooks/useWsDispatch.ts:404-407]
- [x] `[Review/Patch]` [LOW] `ErrActionRequired` and `ErrWrongPhase` for surrender actions fall through to the generic invalid-action / wrong-phase mapping; the defined `game.surrender.errors.actionRequired` and `game.surrender.errors.wrongPhase` i18n keys are dead. Surface them when the rejected action was a surrender action. [client/src/shared/hooks/useWsDispatch.ts]
- [x] `[Review/Patch]` [LOW] proposerUsername falls back to empty string in `SurrenderPrompt`/`SurrenderOpponentBanner` when `gameState.players.find((p) => p.seat === proposerSeat)` returns undefined. Use `t('game.surrender.unknownProposer')` per AC#9 fallback rule. [client/src/features/game/GamePage.tsx:374]
- [x] `[Review/Patch]` [LOW] `TestSurrenderAccept` `t.Run` uses an identical subtest name across 4 cases — Go appends `#01..#04`, breaking name-filtered runs and obscuring failure output. Use `fmt.Sprintf("proposer=%d", c.proposer)`. [server/internal/game/surrender_test.go]
- [x] `[Review/Patch]` [HIGH, from D1] Clear `SurrenderProposerSeat` in `HandleDisconnect` when the disconnecting seat is the proposer or the partner; broadcast `event:surrender_declined` so all four clients drop the prompt/banner. Mirror the cleanup in `handleReconnectTimeout` so the abandoned-match snapshot does not carry a stale proposer. [server/internal/session/reconnect.go:HandleDisconnect, handleReconnectTimeout]
- [x] `[Review/Patch]` [LOW, from D2] Amend AC#1 in this story file to drop the stray `opponentSeats[2]` field from the payload sketch (AC#10 is canonical and matches the implementation). Spec-only edit; no code change. [_bmad-output/implementation-artifacts/8-2-team-surrender.md AC#1]
- [x] `[Review/Patch]` [MEDIUM, from D3] Rename wire field `surrenderedByPlayer` → `surrenderedBySeat` to disambiguate from `match.SurrenderedBy` (userID). Update Go (`MatchEndPayload.SurrenderedBySeat int *omitempty*`), TypeScript (`MatchEndPayload.surrenderedBySeat?: number`), `buildMatchEndPayload`, all consumers in `GamePage.tsx` / `MatchResult.tsx`, and tests. [server/internal/ws/events.go:106-114, client/src/shared/types/wsEvents.ts, client/src/features/game/GamePage.tsx:355-360]

**Deferred (pre-existing or out-of-scope):**

- [x] `[Review/Defer]` Manager tests rely on `time.Sleep(30-100ms)` for async dispatch — flaky on loaded CI [server/internal/session/manager_test.go] — deferred, pre-existing pattern in the file
- [x] `[Review/Defer]` `event:match_end` is broadcast before DB persistence completes in `handleMatchEnd` [server/internal/session/manager.go] — deferred, pre-existing race pattern shared with the natural-end path; not surrender-specific
- [x] `[Review/Defer]` `SurrenderPrompt` uses `absolute inset-0` (vs `fixed`); could leak through scrolled/zoomed views if parent stacking context is wrong [client/src/features/game/components/SurrenderPrompt.tsx] — deferred, design review; tests pass and parent gating mitigates
- [x] `[Review/Defer]` `SurrenderPrompt` has no Escape-key dismissal (decline) [client/src/features/game/components/SurrenderPrompt.tsx] — deferred, accessibility nice-to-have; the `useFocusTrap` hook does not currently dispatch Escape
- [x] `[Review/Defer]` Manager tests assert state-level rejection but not WS-level error broadcast for surrender rejections [server/internal/session/manager_test.go] — deferred, requires refactor to a `Hub` interface or full `httptest.Server` integration; apperr-level mapping is covered by `surrender_test.go` and `sendGameError`

## Dev Notes

### Big Picture — Surrender Is a Pure-Function Add-On to the Rules Engine

Story 8.2 is a **state-machine extension of the rules engine** plus three small UI components. There is no new domain package, no new repository, no new HTTP endpoint. The architecture follows three precedents already in the codebase:

1. **One-time-use per-seat action** → `PauseUsed [4]bool` is the exact precedent ([state.go:101](server/internal/game/state.go#L101)). Surrender adds `SurrenderUsed [4]bool` next to it. The same `state.X[seat] == true → ErrXExhausted` check in the handler is reused verbatim.
2. **Pending action awaiting a specific player's response** → `PendingBelotSeat *int` ([state.go:88](server/internal/game/state.go#L88)) is the precedent: a transient pointer that gates a follow-up action and is cleared when resolved. Surrender adds `SurrenderProposerSeat *int` with the same semantics — except the gating role is reversed: the proposer sets the pointer, the **partner** clears it via accept/decline. The accept-only-by-partner rule is enforced inside `handleSurrenderAccept`/`handleSurrenderDecline` via `seat == (proposer + 2) % 4`.
3. **Match-end persistence** → the natural-end path at [manager.go:237-239](server/internal/session/manager.go#L237-L239) (`if Phase == PhaseMatchEnd → handleMatchEnd`) is the precedent. Surrender accept transitions `Phase = PhaseMatchEnd`, sets `WinnerTeam`, and the **same** `handleMatchEnd` runs — the only refinement is threading a `surrenderedBy *uint` argument through so the persistence call sets the new column. **`Status` stays `'completed'`** so existing wins/losses stats keep working without query changes (rationale in AC #4 — surrender IS a legitimate completed match, just with a different ending mechanism).

The architecturally interesting choice is **typing `MatchEndPayload` as a struct** (currently it's an inline `map[string]interface{}` at [manager.go:451](server/internal/session/manager.go#L451)). This is required to add the optional `outcomeReason` and `surrenderedByPlayer` fields cleanly. The retrofit is additive — wire-format unchanged for natural-end matches because `omitempty` drops empty strings and nil pointers from JSON output. Existing client code reading `winnerTeam`/`teamAFinalScore`/`teamBFinalScore`/`matchDurationSec` continues to work unchanged.

### What Already Exists — Do NOT Recreate

| Item                                            | Location                                                                                                   | Notes                                                                                                                                                                                                                                                                   |
| ----------------------------------------------- | ---------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `Manager.HandleAction`                          | [server/internal/session/manager.go:131-240](server/internal/session/manager.go#L131-L240)                 | Single chokepoint for all WS actions. Surrender actions flow through `parseAction` → `game.ApplyAction` → `broadcastActionResult` → `handleMatchEnd` (only on accept). No new handler infra needed.                                                                     |
| `parseAction`                                   | [server/internal/session/manager.go:307-368](server/internal/session/manager.go#L307-L368)                 | Strip-prefix mapping handles `action:surrender_*` automatically. The `decline_belot → skip_belot` rename precedent is NOT needed for surrender (action names align).                                                                                                    |
| `handleMatchEnd`                                | [server/internal/session/manager.go:632-674](server/internal/session/manager.go#L632-L674)                 | Existing match-record persistence + room-status update + session removal. Add a `surrenderedBy *uint` param; pass `nil` from natural-end call site.                                                                                                                     |
| `cloneGameState`                                | grep `cloneGameState` in `server/internal/game/`                                                           | Pure-function clone helper. Add the new pointer field's deep-copy block alongside the existing `TrumpCallerSeat` / `PendingBelotSeat` / `WinnerTeam` blocks. **Find the file first** — if it's `clone.go`, add there; if inline elsewhere, follow the existing pattern. |
| `apperr.ErrActionRequired`                      | [server/internal/apperr/errors.go:92](server/internal/apperr/errors.go#L92)                                | `ACTION_REQUIRED` — already returns 400; right semantic match for "a surrender proposal is pending; resolve it first". Reuse, do not duplicate.                                                                                                                         |
| `ws.ErrorInvalidAction` (string const)          | [server/internal/ws/events.go:105](server/internal/ws/events.go#L105)                                      | Default fall-through for unmapped errors. Surrender's "non-partner accept" maps here via `sendGameError`.                                                                                                                                                               |
| `BelotPrompt.tsx`                               | [client/src/features/game/components/BelotPrompt.tsx](client/src/features/game/components/BelotPrompt.tsx) | The exact UI shape and `useFocusTrap` + `role="dialog"` pattern to mirror for `SurrenderPrompt`.                                                                                                                                                                        |
| Pause-button gating                             | [client/src/features/game/GamePage.tsx:451-463](client/src/features/game/GamePage.tsx#L451-L463)           | Phase-gated render + `disabled={!canPause}` + caption-toggle pattern to mirror for `SurrenderButton`.                                                                                                                                                                   |
| shadcn `AlertDialog`                            | [client/src/shared/components/ui/alert-dialog.tsx](client/src/shared/components/ui/alert-dialog.tsx)       | Installed by Story 8.1. Reuse for `SurrenderButton`'s confirm dialog. **Verify it exists** (`ls client/src/shared/components/ui/alert-dialog.tsx`); if not, run `npx shadcn@latest add alert-dialog` per the project shadcn workflow.                                   |
| `MatchResult.tsx`                               | [client/src/features/game/components/MatchResult.tsx](client/src/features/game/components/MatchResult.tsx) | Existing winner overlay. Extend with optional `surrenderedByUsername` prop and conditional sub-line; do NOT fork into a separate `SurrenderResult` component.                                                                                                           |
| `useWsDispatch` error-event branch              | [client/src/shared/hooks/useWsDispatch.ts](client/src/shared/hooks/useWsDispatch.ts)                       | Existing `dispatchErrorEvent` (or equivalent) for `ERROR_PAUSE_EXHAUSTED` etc. Add the new `ERROR_SURRENDER_EXHAUSTED` branch alongside.                                                                                                                                |
| `gameStore` transient-payload pattern           | [client/src/shared/stores/gameStore.ts](client/src/shared/stores/gameStore.ts)                             | `belotReveal` / `setBelotReveal` is the exact shape to mirror for `surrenderProposed` / `setSurrenderProposed`.                                                                                                                                                         |
| `mockMatchRepo` + `defaultPlayers()` test infra | [server/internal/session/manager_test.go](server/internal/session/manager_test.go)                         | Existing in-memory mock + 4-seat fixture. New surrender tests reuse both — no new test infra needed.                                                                                                                                                                    |

### What Must Be Created

1. **One new SQL migration pair**: `000010_add_match_surrendered_by.up.sql` + `.down.sql`.
2. **One new Go file**: `server/internal/game/surrender.go` (three handlers).
3. **One new Go test file**: `server/internal/game/surrender_test.go`.
4. **Three new React components**: `SurrenderButton.tsx`, `SurrenderPrompt.tsx`, `SurrenderOpponentBanner.tsx` (each with co-located `.test.tsx`).
5. **One new apperr** (and possibly a second if `ErrInvalidAction` doesn't already exist).
6. **Six new WS event/error/action constants** + 2 new payload structs (server) + matching TS exports.

### What Must Be Modified

1. [server/internal/match/model.go](server/internal/match/model.go) — add `SurrenderedBy *uint`.
2. [server/internal/game/state.go](server/internal/game/state.go) — add `SurrenderProposerSeat *int`, `SurrenderUsed [4]bool`.
3. [server/internal/game/types.go](server/internal/game/types.go) — add three action-type constants.
4. [server/internal/game/rules_engine.go](server/internal/game/rules_engine.go) — wire three handler dispatches (sibling to pause/unpause).
5. [server/internal/game/clone.go](server/internal/game/clone.go) (or wherever `cloneGameState` lives) — clone the new pointer field.
6. [server/internal/apperr/errors.go](server/internal/apperr/errors.go) — add `ErrSurrenderExhausted` (and possibly `ErrInvalidAction`).
7. [server/internal/ws/events.go](server/internal/ws/events.go) — add 6 constants + 2 payload structs + extend `MatchEndPayload`.
8. [server/internal/session/manager.go](server/internal/session/manager.go) — extend `broadcastActionResult` with three cases; type the existing `MatchEndPayload`; thread `surrenderedBy *uint` through `handleMatchEnd`; add `sendGameError` mapping.
9. [server/internal/session/manager_test.go](server/internal/session/manager_test.go) — add ~10 new end-to-end test cases.
10. [client/src/shared/types/wsEvents.ts](client/src/shared/types/wsEvents.ts) — add 6 constants + 2 interfaces + extend `MatchEndPayload`.
11. [client/src/shared/types/gameTypes.ts](client/src/shared/types/gameTypes.ts) — extend `GameState` with the two new fields; extend `ActionType` union.
12. [client/src/shared/stores/gameStore.ts](client/src/shared/stores/gameStore.ts) — add `surrenderProposed`/`surrenderDeclined` slots + setters + `initialState`.
13. [client/src/shared/hooks/useWsDispatch.ts](client/src/shared/hooks/useWsDispatch.ts) — three new dispatch branches.
14. [client/src/shared/hooks/useWsDispatch.test.ts](client/src/shared/hooks/useWsDispatch.test.ts) — four new test cases.
15. [client/src/features/game/GamePage.tsx](client/src/features/game/GamePage.tsx) — wire action handlers, derived flags, render the three new components, restructure pause-button container into a 2-button stack.
16. [client/src/features/game/GamePage.test.tsx](client/src/features/game/GamePage.test.tsx) — five new integration cases.
17. [client/src/features/game/components/MatchResult.tsx](client/src/features/game/components/MatchResult.tsx) — optional `surrenderedByUsername` prop + conditional sub-line.
18. [client/src/features/game/components/MatchResult.test.tsx](client/src/features/game/components/MatchResult.test.tsx) — two new test cases.
19. [client/src/shared/i18n/en.json](client/src/shared/i18n/en.json) + [sr.json](client/src/shared/i18n/sr.json) — `game.surrender.*` block + `game.matchResult.surrenderNote`.

**No changes expected:**

- [server/internal/match/gorm_repo.go](server/internal/match/gorm_repo.go) — stats and history queries already work because surrender uses `status='completed'`.
- [server/internal/session/reconnect.go](server/internal/session/reconnect.go) — abandonment flow is separate; surrender state is discarded with `gs` on abandonment.
- [server/internal/room/handler.go](server/internal/room/handler.go) — no room-level changes.
- [server/internal/game/scoring.go](server/internal/game/scoring.go), [bidding.go](server/internal/game/bidding.go), [validation.go](server/internal/game/validation.go) — game rules untouched.
- Existing tests for natural match-end, match-abandonment, pause, reconnect — should pass unchanged because `MatchEndPayload`'s additional fields are `omitempty` and `GameState`'s new fields default to zero values.

### Architecture Patterns to Follow

- **Pure-function rules engine** ([\_bmad-output/project-context.md#L70](_bmad-output/project-context.md#L70)). All three surrender handlers follow `func handleX(state *GameState, action Action) (*GameState, error)`. Zero side effects. Clone state before mutation. The session manager owns broadcasts and persistence.
- **Slice clone discipline** ([\_bmad-output/project-context.md#L71](_bmad-output/project-context.md#L71)). The new `SurrenderUsed [4]bool` is a value-type fixed array (cloned by struct copy in `cloneGameState` already). The new `SurrenderProposerSeat *int` is a pointer that **must** be deep-cloned in `cloneGameState` next to the existing pointer fields (`TrumpCallerSeat`, `PendingBelotSeat`, `WinnerTeam`, etc.). Read the existing clone helper carefully and add the new field's block.
- **Pointer for nullable optional fields** ([\_bmad-output/project-context.md#L75](_bmad-output/project-context.md#L75)). `SurrenderProposerSeat *int` and `SurrenderedBy *uint` use pointers so JSON `null` distinguishes from "seat 0" / "userID 0". Same trick as `WinnerTeam *int`, `AbandonedBy *uint`, `PendingBelotSeat *int`.
- **Multi-event WS broadcasts as separate ordered messages** ([\_bmad-output/project-context.md#L109](_bmad-output/project-context.md#L109)). Surrender request: `event:surrender_proposed` then `event:game_state` (separate messages, ordered). Surrender accept: `event:match_end` then `event:game_state`. Surrender decline: `event:surrender_declined` then `event:game_state`. **Never batch.**
- **WS event prefixes** ([\_bmad-output/planning-artifacts/architecture.md#L327-L336](_bmad-output/planning-artifacts/architecture.md#L327-L336)). Game-state events use `event:`, errors use `error:`, client→server uses `action:`. Surrender events fit cleanly: `action:surrender_*`, `event:surrender_*`, `error:surrender_*`.
- **Same-commit WS contract sync** ([\_bmad-output/project-context.md#L80, #L286](_bmad-output/project-context.md#L80)). [server/internal/ws/events.go](server/internal/ws/events.go) and [client/src/shared/types/wsEvents.ts](client/src/shared/types/wsEvents.ts) MUST be updated together.
- **Additive JSON-extension discipline**. New `MatchEndPayload` fields are `omitempty` (`outcomeReason`, `surrenderedByPlayer`); new `GameState` fields default to zero values that serialize correctly. No existing client breaks.
- **`data-testid` over text/Tailwind queries** ([Story 7.1, 7.2, 8.1 reinforced](C:\Users\Emilijan-LT.claude\projects\d--My-Projects-beljot\memory\MEMORY.md)). All new components expose stable `data-testid`s. No tests query by Tailwind class or visible string.
- **i18n parity is CI-enforced** by [client/src/shared/i18n/i18n.test.ts](client/src/shared/i18n/i18n.test.ts) recursive `flattenKeys` check. Any English key without a Serbian counterpart fails CI.
- **Server is the authority for game state**; client UI is presentational. The surrender button does not optimistically set `surrenderProposerSeat` — it waits for the WS broadcast (which the proposer also receives, identical to how the pause button works).
- **No `enum`s in TypeScript** — use union literal types (project-context Go/TS rule). The `outcomeReason` field is typed as `?: "surrender"` (literal), not an enum.

### Previous Story Intelligence (Story 8.1 — done, 2026-04-26)

Carried-forward learnings that shape this story:

- **Prettier-before-commit is non-negotiable.** Task 11.3 enforces this. CI has failed repeatedly across Stories 7.1, 7.2, 8.1.
- **Story 8.1 introduced shadcn `AlertDialog`** at `client/src/shared/components/ui/alert-dialog.tsx`. Story 8.2 reuses it for the confirm-surrender dialog — no new shadcn install needed. Verify file exists before assuming.
- **Story 8.1 review-cycle pattern**: ~11 review patches, 2 deferred, 11 dismissed. Design decisions to absorb up-front:
  - **Pending-state styling on a single component**, never globally (Story 8.1 review patch). The `SurrenderButton`'s pending state must be local to the button only — not gating the whole screen.
  - **`disabled` (real attribute) preferred over `pointer-events-none`** (Story 8.1 review patch). Use `disabled` on `<button>` for keyboard accessibility.
  - **Defensive room-id / seat-id checks before triggering side effects** (Story 8.1 dispatcher patch). Apply to surrender dispatch: only set `surrenderProposed` if the local user is in an active match (`gameStore.gameState !== null`).
  - **i18n string reuse leads to misleading copy** (Story 8.1 `errors.notOwner` patch). Don't reuse `errors.notOwner` for non-partner accept — add a dedicated `game.surrender.errors.actionRequired` (and any other surrender-specific keys).
- **`data-testid` discipline** is fully baked (Stories 7.1, 7.2, 8.1).
- **i18n parity test (`flattenKeys`)** catches every divergence — no hand-checking needed.
- **`QueryWrapper` from `@/test-utils/`** is the canonical react-query test wrapper (irrelevant for surrender — no react-query mutations involved, surrender is pure WebSocket).

### Cross-Story Context

- **Story 5.1-5.2 (done)** — pause system. Surrender is **architecturally analogous to pause** (one-time-use per-seat action with `Used [4]bool` flag). Reuse the same precedent end-to-end.
- **Story 5.5 (done)** — match abandonment on reconnect timeout. Surrender is a **different terminal event** (`event:match_end` not `event:match_abandoned`); persistence is `Status: 'completed'` not `'abandoned'`. The two are independent — abandoning during a pending surrender goes through the abandonment path and discards surrender state.
- **Story 4.6 (done)** — score panel + match flow. The natural match-end UI pipe (`event:match_end → setMatchEndData → MatchResult overlay`) is the same pipe surrender uses with the additional `outcomeReason` field. Story 4.6's invariants are preserved.
- **Story 4.5 (done)** — per-move timer + auto-play. Surrender intentionally **does not** participate in auto-play on timer expiry (decision documented in AC #13). The active player's timer keeps ticking through a pending proposal; auto-play fires for card play, not for surrender resolution.
- **Story 8.1 (done — 2026-04-26)** — kick + swap. Same epic. Established `AlertDialog` reuse and 8.x story shape.
- **Story 8.3 (future — backlog)** — In-Game Emotes. Will add `action:emote` → `system:emote`. Same `BelotPrompt`-style overlay pattern as surrender's prompt; do not pre-implement.
- **Story 9.x (future — Phase 2 economy)** — coin pots. PRD line 129: surrender settles identically to a loss (surrendering team forfeits stake; winners split the pot). Story 9.2 will read the new `surrendered_by` column when computing settlement; this story does NOT touch coins or wallets.
- **Story 9.6 (future — honor system)** — surrender is a **neutral honor signal** (PRD line 134). It increments the `completed` counter and does NOT decrement honor (epics.md AC at line 1735-1737). The `surrendered_by` column added in this story is the load-bearing signal Story 9.6 will read. **Do NOT pre-implement honor logic** — just persist the column correctly.
- **Story 12.1 (future — Croatian variant)** — Croatian rules engine. Surrender semantics are variant-agnostic (FR28a applies to both variants). The handlers in [server/internal/game/surrender.go](server/internal/game/surrender.go) check phase only, not variant — Croatian's bidding/declaration phases will inherit surrender support automatically.

### Recent Codebase Signals (git log — last 8 commits)

- `34a8d95 feat(room): add owner kick + seat-swap pre-game controls (Story 8.1)` — direct predecessor. Same epic. The `AlertDialog` shadcn install lands here; Story 8.2 reuses it.
- `b16920f chore(server): apply gofmt across the codebase` — formatting-only; reinforces "gofmt is enforced — Task 11.4 will run against any new Go code".
- `588b6eb chore: add .gitattributes to enforce LF line endings` — Windows ↔ Linux normalisation. New files added in this story MUST be LF-encoded; let git's normalisation handle it.
- `b86d4fb chore(server): promote golang.org/x/text to a direct dependency` — go.mod hygiene; not related.
- `b78e1ec fix(game): require trump cut when void in led suit (Bitola)` — game rules; not related to surrender.
- `4aa94d2 feat(game): show face-up candidate card in round-2 trump prompt` — game UI; not related.
- `fb89d64 feat(game): show table-wide reveal dialog when a player takes trump` — game UI; not related.
- `f54f6ee chore(bmad): upgrade BMad framework install (skills + module configs)` — tooling-only; not related.

**Signal: rules-engine + session-manager have been quiet outside Story 8.1's room-domain work since Story 4.7.** Touchpoint risk for surrender is **low to moderate** — the rules engine's `ApplyAction` switch is the single point where the new actions plug in, and the session manager's `broadcastActionResult` adds three new `case` branches without disturbing the existing ones. The `MatchEndPayload` retrofit is the one moderate-risk change — the inline `map` to typed-struct conversion needs care to preserve wire format exactly.

### Backend Flow — `action:surrender_request`

1. Client sends `{ "type": "action:surrender_request", "payload": {} }` over WS.
2. Hub dispatches to `Manager.HandleAction` ([manager.go:131](server/internal/session/manager.go#L131)).
3. `parseAction` ([manager.go:307](server/internal/session/manager.go#L307)) strips `action:` → `surrender_request`; resolves `seat` from `session.playerIDs`. No payload decoding.
4. `session.mu.Lock()` → `cancelTurnTimer()` (existing pattern — turn timer is cancelled and re-armed for every action; it will be re-armed below since phase doesn't change).
5. `game.ApplyAction(state, Action{Type: ActionSurrenderRequest, PlayerSeat: seat})`.
6. `handleSurrenderRequest` validates: `Phase ∈ {Bidding, Playing}`, `SurrenderProposerSeat == nil`, `SurrenderUsed[seat] == false`. Sets `SurrenderProposerSeat = &seat`, `SurrenderUsed[seat] = true`. Returns the new state.
7. Back in `HandleAction`: `setTurnExpiry` + `startTimerLocked` re-arm the per-move timer (phase is still `Playing`/`Bidding`).
8. `session.mu.Unlock()`.
9. `broadcastActionResult` runs the new `case ActionSurrenderRequest` branch:
   - `BroadcastToUsers(playerIDs, event:surrender_proposed, SurrenderProposedPayload{...})`.
   - `BroadcastToUsers(playerIDs, event:game_state, newState)`.
10. `bufferHandResultIfScored` is a no-op (no hand transition).
11. `Phase != PhaseMatchEnd` → `handleMatchEnd` is NOT called.

### Backend Flow — `action:surrender_accept`

1. Client (the partner) sends `{ "type": "action:surrender_accept", "payload": {} }`.
2. Hub → `Manager.HandleAction` → `parseAction` (seat resolved).
3. `session.mu.Lock()` → `cancelTurnTimer()`.
4. `game.ApplyAction(state, Action{Type: ActionSurrenderAccept, PlayerSeat: seat})`.
5. `handleSurrenderAccept` validates: `SurrenderProposerSeat != nil`, `seat == (proposer + 2) % 4`. Sets `WinnerTeam = &(1 - TeamForSeat(proposer))`, `Phase = PhaseMatchEnd`, `SurrenderProposerSeat = nil`, `TurnExpiresAt = nil`, `TurnTimeRemaining = 0`. Returns new state.
6. Back in `HandleAction`: phase is now `PhaseMatchEnd` — neither pause-resume nor turn-timer setup runs (the existing `else if newState.Phase == PhasePlaying || PhaseBidding` block is skipped).
7. `session.mu.Unlock()`.
8. `broadcastActionResult` runs the new `case ActionSurrenderAccept` (no-op — see Task 4.2 comment).
9. `bufferHandResultIfScored` is a no-op (no hand transition; the surrender ends mid-hand).
10. **`Phase == PhaseMatchEnd`** → `handleMatchEnd(session, newState)` runs. **Critical**: the call site needs to pass `surrenderedBy = &session.playerIDs[proposer]` — but at the call site we no longer know the proposer (it's been cleared from `newState`). Solution: capture `proposerSeat` from `oldState.SurrenderProposerSeat` BEFORE the lock is released and thread it into the call. Alternative: the `broadcastActionResult` `case ActionSurrenderAccept` branch can call `m.broadcastMatchEnd(playerIDs, oldState, newState, action, startedAt, &proposerSeat)` directly, then `HandleAction`'s own `Phase == PhaseMatchEnd` block calls `handleMatchEnd` separately for persistence. **Choose the explicit-argument approach** — clearer than implicit state.
11. `handleMatchEnd` builds `match.Match` with `Status: "completed"`, `WinnerTeam`, `SurrenderedBy: &session.playerIDs[proposerSeat]`. `m.matchRepo.CreateWithHands(...)` → row inserted. `m.roomUpdater.UpdateRoomStatus(roomID, "completed")`. `m.RemoveSession(roomID)`.

### Backend Flow — `action:surrender_decline`

1. Client (the partner) sends `{ "type": "action:surrender_decline", "payload": {} }`.
2. Hub → `Manager.HandleAction` → `parseAction`.
3. `session.mu.Lock()` → `cancelTurnTimer()`.
4. `game.ApplyAction` → `handleSurrenderDecline` validates partner; sets `SurrenderProposerSeat = nil`. Returns state with `SurrenderUsed[proposer]` still `true` (consumed even on decline — FR28a).
5. Back in `HandleAction`: phase still `Playing`/`Bidding` → re-arm turn timer.
6. `session.mu.Unlock()`.
7. `broadcastActionResult` runs the new `case ActionSurrenderDecline`:
   - `BroadcastToUsers(playerIDs, event:surrender_declined, SurrenderDeclinedPayload{...})`.
   - `BroadcastToUsers(playerIDs, event:game_state, newState)`.
8. `Phase != PhaseMatchEnd` → no persistence side effects. Game continues.

### Frontend Flow — Three Players' Perspectives on a Pending Proposal

| Role                     | What They See                                                                                                                        | Driving State                                                                                                                    |
| ------------------------ | ------------------------------------------------------------------------------------------------------------------------------------ | -------------------------------------------------------------------------------------------------------------------------------- |
| **Proposer (seat 0)**    | Surrender button shows `t("game.surrender.pending")`, disabled. No prompt. No banner. Hand of cards still active if it's their turn. | `gameState.surrenderProposerSeat === myPlayerSeat → caption flips, button disabled`                                              |
| **Partner (seat 2)**     | `SurrenderPrompt` modal-style overlay with accept/decline buttons. Body text mentions seat-0 username.                               | `gameState.surrenderProposerSeat !== null && (gameState.surrenderProposerSeat + 2) % 4 === myPlayerSeat → mount SurrenderPrompt` |
| **Opponent (seat 1, 3)** | `SurrenderOpponentBanner` slim non-modal banner near top. Game keeps going — they can play their turn.                               | `gameState.surrenderProposerSeat !== null && team-of-self ≠ team-of-proposer → mount banner`                                     |

When the partner clicks Accept: the WS round-trip fires `event:match_end` → `MatchResult` overlay appears for everyone (with the surrender sub-line). When the partner clicks Decline: `event:surrender_declined` arrives → toast on all 4 clients ("Surrender declined — play continues") → `event:game_state` clears `surrenderProposerSeat` → banner/prompt unmount → game resumes. The proposer's button now reads "Surrender used".

### Project Structure Notes

**Modified files (expected): see "What Must Be Modified" — 19 files modified, 5 new files (1 SQL up, 1 SQL down, 1 Go handler, 1 Go test, 3 React components × 2 files each = 6, but counted together = ~10 net new files), 1 schema migration.**

**Alignment with unified project structure:**

- Backend: handlers as a new file in the existing `internal/game/` package; matches the package layout pattern at [\_bmad-output/planning-artifacts/architecture.md#L356-L368](_bmad-output/planning-artifacts/architecture.md#L356-L368). Session manager and match repo are unchanged-package extensions.
- Frontend: components live in the same feature folder (`features/game/components/`) as `BelotPrompt`, `MatchResult`, etc. — no new folder.
- WS contract: both sides updated in the same commit per project rule.
- Migration: next sequential number, full down-migration, indexed FK column — matches [server/migrations/000008_add_match_status.up.sql](server/migrations/000008_add_match_status.up.sql).
- i18n: nested keys under the existing `game` block, not a new top-level block.

### Alignment Checks / Detected Conflicts

- **Epic AC names align with implementation.** `action:surrender_request`, `event:surrender_proposed`, `error:surrender_exhausted` are taken verbatim from epics.md AC text (lines 1471-1491). No prefix-discipline reinterpretation needed (unlike Story 8.1's `event:room_kicked` → `system:room_kicked` realignment).
- **Match-record status: `completed` not `surrendered`.** AC #4 documents this as a deliberate choice — keeps existing wins/losses query unchanged ([gorm_repo.go:46-61](server/internal/match/gorm_repo.go#L46-L61)). The honor system (Story 9.6) will read `surrendered_by` to distinguish neutral-completion from natural-win, per PRD line 134.
- **`MatchEndPayload` retrofit risk.** Currently constructed inline as `map[string]interface{}` ([manager.go:451](server/internal/session/manager.go#L451)); typing it as a struct is moderate-risk because every client reading the payload must continue to work. **Mitigation**: AC #10 enumerates the existing fields' JSON tags exactly so wire format is byte-identical for natural ends; `omitempty` ensures new fields don't appear in natural-end JSON. The TypeScript interface change is additive — no existing call site breaks.
- **Auto-play vs surrender proposal interaction (AC #13).** Decision: auto-play continues for the active player; surrender proposal is **not** auto-resolved on timer expiry. Rationale: keeping the proposal "human-driven" preserves the FR28a contract ("teammate must accept") and avoids coupling the surrender state machine to the per-move timer. Future timeouts (e.g. "auto-decline after 30s of no response") can be added in Phase 3 without re-architecting.
- **Pause / disconnect interactions (AC #13).** Both preserve the proposal across the lifecycle (proposal field is just state, not phase-coupled). Reconnect-timeout abandonment discards surrender state with the rest of `gs` and persists with `Status: 'abandoned'` only — no `surrendered_by` written for an abandoned match. This is intentional: a proposer who abandoned mid-proposal didn't actually surrender, they abandoned.

### Edge Cases & Anti-Patterns to Avoid

- **Do NOT** allow surrender from `PhasePaused`. The `case PhasePaused` arm in `ApplyAction` returns `ErrGamePaused` early; surrender's per-action branches at the top of `ApplyAction` would bypass this. **Solution**: place surrender phase check inside each handler (`PhasePlaying || PhaseBidding`), not at the dispatch level. The `case PhasePaused: ErrGamePaused` will never run for surrender actions because they're matched before the phase switch — explicit phase rejection inside the handler is the guard.
- **Do NOT** auto-resolve a pending surrender on the per-move timer (decision in AC #13). The active player's timer continues to tick for card play, but surrender stays pending until manual accept/decline.
- **Do NOT** consume `SurrenderUsed[partner]` when the partner declines. Only the proposer's slot flips to `true` (per FR28a — "the proposing player's surrender attempt is consumed", not the partner's). This is reinforced by the test `"partner can decline even if their own SurrenderUsed is true"`.
- **Do NOT** broadcast `event:match_end` from inside `case ActionSurrenderAccept` of `broadcastActionResult`. The existing `case ActionPlayCard` natural-end path emits it before `event:game_state`; the new path must keep that ordering. Centralise the match-end broadcast in a helper called from both call sites (Task 4.3).
- **Do NOT** reuse `ws.ErrorPauseExhausted` for surrender. New `error:surrender_exhausted` constant is mandatory — the client's i18n + dispatch must distinguish the two so toast text is correct.
- **Do NOT** open the `SurrenderPrompt` for the proposer or for opponents. Only the partner sees it. Test cases lock this in.
- **Do NOT** focus-trap the opponent banner. It's a status overlay, not a dialog. Opponents must be able to keep playing; trapping focus would make their turn impossible.
- **Do NOT** persist `Status: 'surrendered'` in the matches table. Use `'completed'` so existing wins/losses query at [gorm_repo.go:46-61](server/internal/match/gorm_repo.go#L46-L61) Just Works. The new `surrendered_by` column is the load-bearing signal for "this completed match ended via surrender, not natural win".
- **Do NOT** forget to add `surrendered_by` to the GORM struct tags. `gorm:"index" json:"surrenderedBy,omitempty"` mirrors `AbandonedBy` exactly. Without the index, future Story 9.6 queries (honor system) will table-scan.
- **Do NOT** add a `surrenderUsed` setter to `gameStore`. The flag lives on `gs`, not the store — same pattern as `pauseUsed`. The store only carries the **transient** event payloads (`surrenderProposed`, `surrenderDeclined`).
- **Do NOT** show the banner / prompt during `gs.Phase === "match_end"`. The MatchResult overlay takes over; banners must unmount the moment phase flips. The existing `gameState.phase === "match_end"` gate at [GamePage.tsx](client/src/features/game/GamePage.tsx) already handles this for similar overlays — apply the same gate to the new banner / prompt mounting conditions.
- **Counter-clockwise seat order is irrelevant to surrender.** Partner derivation `(proposer + 2) % 4` is purely "across the table" — no rotation logic. The `(currentPlayer + 1) % 4` rule from project-context applies only to game-time turn rotation.

### References

- [Source: epics.md#Story-8.2 — Team Surrender acceptance criteria](_bmad-output/planning-artifacts/epics.md#L1465)
- [Source: prd.md#FR28a — surrender mechanic spec](_bmad-output/planning-artifacts/prd.md#L342)
- [Source: prd.md#L134 — surrender as neutral honor signal (Story 9.6 future-coupling)](_bmad-output/planning-artifacts/prd.md#L134)
- [Source: prd.md#L129 — surrender coin-settlement spec (Story 9.2 future-coupling)](_bmad-output/planning-artifacts/prd.md#L129)
- [Source: architecture.md#L327-L336 — WebSocket prefix rule](_bmad-output/planning-artifacts/architecture.md#L327-L336)
- [Source: architecture.md#L356-L368 — internal/game package shape](_bmad-output/planning-artifacts/architecture.md#L356-L368)
- [Source: project-context.md#L65-L78 — Go rules (pure functions, slice clone, pointer optionals, error wrapping)](_bmad-output/project-context.md#L65)
- [Source: project-context.md#L99-L113 — Echo / backend rules (multi-event broadcasts as separate ordered messages)](_bmad-output/project-context.md#L99)
- [Source: project-context.md#L80, #L286 — WS event contract MUST be updated in both files in the same commit](_bmad-output/project-context.md#L80)
- [Source: project-context.md#L243-L245 — sequential migration numbering rule](_bmad-output/project-context.md#L243)
- [Source: 8-1-room-owner-pre-game-controls.md — predecessor; AlertDialog reuse, prettier-before-commit, data-testid discipline, review-cycle pattern](_bmad-output/implementation-artifacts/8-1-room-owner-pre-game-controls.md)
- [Source: server/internal/game/pause.go — closest precedent: one-time-use per-seat action with `PauseUsed [4]bool`](server/internal/game/pause.go)
- [Source: server/internal/game/state.go — `PauseUsed`, `PendingBelotSeat *int` precedents; `WinnerTeam *int`, pointer-optional patterns](server/internal/game/state.go)
- [Source: server/internal/game/rules_engine.go — `ApplyAction` dispatch; place surrender branches at the same level as pause/unpause](server/internal/game/rules_engine.go)
- [Source: server/internal/session/manager.go — `HandleAction`, `parseAction`, `broadcastActionResult`, `handleMatchEnd`](server/internal/session/manager.go)
- [Source: server/internal/session/reconnect.go — abandonment match-end as the closest non-natural match-end precedent](server/internal/session/reconnect.go)
- [Source: server/internal/match/model.go — `Match` struct; add `SurrenderedBy *uint` next to `AbandonedBy`](server/internal/match/model.go)
- [Source: server/internal/match/gorm_repo.go — stats and history queries; do NOT modify (surrender uses `status='completed'`)](server/internal/match/gorm_repo.go)
- [Source: server/internal/apperr/errors.go — apperr conventions; add `ErrSurrenderExhausted`, possibly `ErrInvalidAction`](server/internal/apperr/errors.go)
- [Source: server/internal/ws/events.go — events.go to add new constants and payload structs; type the existing `MatchEndPayload`](server/internal/ws/events.go)
- [Source: server/migrations/000008_add_match_status.up.sql — precedent for the new `000010_add_match_surrendered_by` migration pair](server/migrations/000008_add_match_status.up.sql)
- [Source: client/src/features/game/components/BelotPrompt.tsx — `useFocusTrap`, `role="dialog"`, two-button overlay pattern to mirror for `SurrenderPrompt`](client/src/features/game/components/BelotPrompt.tsx)
- [Source: client/src/features/game/components/MatchResult.tsx — winner overlay to extend with optional surrender sub-line](client/src/features/game/components/MatchResult.tsx)
- [Source: client/src/features/game/GamePage.tsx — pause-button gating and prompt mounting pattern to mirror; bottom-left controls slot to share](client/src/features/game/GamePage.tsx)
- [Source: client/src/shared/stores/gameStore.ts — `belotReveal` / `setBelotReveal` shape to mirror for `surrenderProposed`](client/src/shared/stores/gameStore.ts)
- [Source: client/src/shared/hooks/useWsDispatch.ts — dispatcher to extend with surrender event branches](client/src/shared/hooks/useWsDispatch.ts)
- [Source: client/src/shared/types/wsEvents.ts — add new constants and payload interfaces; extend MatchEndPayload](client/src/shared/types/wsEvents.ts)
- [Source: client/src/shared/types/gameTypes.ts — extend GameState with new fields; extend ActionType union](client/src/shared/types/gameTypes.ts)
- [Source: client/src/shared/i18n/en.json + sr.json — game.surrender.\* block to add; matchResult.surrenderNote sub-key](client/src/shared/i18n/en.json)
- [Source: client/src/shared/i18n/i18n.test.ts — recursive flattenKeys parity check (CI gate)](client/src/shared/i18n/i18n.test.ts)
- [Source: feedback memory — Prettier before every commit](C:\Users\Emilijan-LT.claude\projects\d--My-Projects-beljot\memory\feedback_prettier_before_commit.md)
- [Source: project memory — Beljot uses BMM module](C:\Users\Emilijan-LT.claude\projects\d--My-Projects-beljot\memory\project_bmad_module.md)

## Dev Agent Record

### Agent Model Used

claude-opus-4-7 (Opus 4.7, 1M context)

### Debug Log References

- `cd server && go test ./...` — all packages green (game + session including new TestSurrenderRequest / TestSurrenderAccept / TestSurrenderDecline / TestSurrender\_\*).
- `cd client && npx vitest run` — 484 tests pass (54 files), including 5 new component / dispatch test files (SurrenderButton, SurrenderPrompt, SurrenderOpponentBanner, MatchResult surrender cases, useWsDispatch surrender cases).
- `npx tsc --noEmit` clean. ESLint clean across all touched files.
- `cd client && npx prettier --write …` — only formatting tweaks, applied.
- `gofmt -l server/` — clean after auto-format.
- `migrate up/down/up` round-trip on local Postgres (`beljot@localhost:5433`) — `10/u add_match_surrendered_by` applied, rolled back, re-applied without errors.
- `i18n.test.ts flattenKeys` parity check passes for `en.json` ↔ `sr.json` after the new `game.surrender.*` block.

### Completion Notes List

- **Migration**: `000010_add_match_surrendered_by` (.up + .down) lands cleanly; round-trip applied locally. `match.Match.SurrenderedBy *uint` mirrors `AbandonedBy` precedent. Stats query at `match/gorm_repo.go:46-61` is intentionally untouched — surrender keeps `Status="completed"` so wins/losses keep working without modification (per AC #4 / #5 rationale).
- **Rules engine**: New `server/internal/game/surrender.go` houses three pure-function handlers (`handleSurrenderRequest`, `Accept`, `Decline`). They are dispatched at the same level as pause/unpause in `rules_engine.go` so accept/decline can clear a pending proposal even from an otherwise-blocked phase (defence in depth). `cloneGameState` in `bidding.go` extended to deep-copy `SurrenderProposerSeat *int`.
- **State + types**: `GameState` gains `SurrenderProposerSeat *int` and `SurrenderUsed [4]bool` (added immediately after the pause-state block per task spec). Action constants added next to existing pause/unpause/owner_unpause set.
- **apperr**: Added `ErrSurrenderExhausted` (`SURRENDER_EXHAUSTED`, 409) and `ErrInvalidAction` (`INVALID_ACTION`, 400). `ErrActionRequired` is reused for the "proposal already pending" rejection.
- **Session manager**: `broadcastActionResult` gains three new cases. `MatchEndPayload` retrofitted from inline `map[string]interface{}` to typed `ws.MatchEndPayload` struct via a new `buildMatchEndPayload` helper — fields `outcomeReason` + `surrenderedByPlayer` use `omitempty` so wire format for natural ends is byte-identical. `handleMatchEnd` signature gained a `surrenderedBy *uint` argument; both call sites updated (HandleAction passes the proposer's userID resolved from `oldState.SurrenderProposerSeat`; auto-play call site passes nil since timer cannot resolve a surrender). `sendGameError` maps `ErrSurrenderExhausted` to `ws.ErrorSurrenderExhausted`.
- **WS contract sync**: `events.go` and `wsEvents.ts` updated together (same edit batch). New constants for actions, events, error code, plus typed payload structs. `MatchEndPayload` extended additively in both files.
- **Frontend store / dispatch**: `gameStore` gains `surrenderProposed` and `surrenderDeclined` slots (mirroring `belotReveal`). `useWsDispatch` adds branches for `event:surrender_proposed` (with defence-in-depth `gameState !== null` guard from Story 8.1 review pattern) and `event:surrender_declined` (toast + store). `error:surrender_exhausted` gets a dedicated `toast.error` branch in `dispatchErrorEvent`.
- **GamePage**: New `handleSurrenderRequest/Accept/Decline` action handlers. Four derived flags (`isProposer`, `isPartnerOfProposer`, `isOpponentOfProposer`, `canSurrenderRequest`) feed three new components. The bottom-left pause-button slot is now a `flex flex-col gap-2` stack housing pause + surrender buttons. `MatchResult` receives `surrenderedByUsername` derived from `matchEndData.surrenderedByPlayer` + `gameState.players`.
- **Three new components**: `SurrenderButton.tsx` (uses shadcn `Dialog` for confirm — Story 8.1 used `Dialog`, not `AlertDialog`; the alert-dialog primitive isn't installed); `SurrenderPrompt.tsx` (mirrors `BelotPrompt`'s `useFocusTrap` + `role="dialog" aria-modal="true"` shape); `SurrenderOpponentBanner.tsx` (slim non-modal banner with `role="status"` / `aria-live="polite"` — explicitly NOT a dialog so opponents can keep playing). `MatchResult.tsx` extended with optional `surrenderedByUsername` prop and a conditional `match-result-surrender-note` sub-line that renders only when `outcomeReason === "surrender"`.
- **i18n**: `game.surrender.*` block added to both `en.json` and `sr.json` with the Ekavian Serbian-Latin register. `game.matchResult.surrenderNote` sub-key added to both. The `i18n.test.ts` parity check passes.
- **Tests**: Backend rules-engine tests cover all 4 proposer→winner combinations, every reject path, pure-function discipline, and cross-state isolation against pause/score/hand fields. Session-manager tests drive the full flow via `mgr.HandleAction` and assert: request transitions, accept persists with `Status="completed"` + `SurrenderedBy=<userID>`, decline keeps the session active and consumes the attempt, exhausted-after-decline rejection, second-request-while-pending rejection, non-partner-accept rejection, and pause/unpause preserves the proposal. Frontend tests cover all three new components (rendering, callbacks, disabled states, anti-modal assertion on the banner), `MatchResult` with and without surrender, and dispatch routing for all three new event/error types.
- **Manual smoke (Task 11.6)**: NOT executed in this session — running `make dev` and driving 4 browser sessions through the surrender flow is left for human verification before merge. Code-level integration is fully covered by automated tests above; the smoke test exists to catch ordering/animation issues that unit tests can't see.
- **Prettier-before-commit**: Run via `npx prettier --write …` across every touched client file at the end of Task 11. CI has burned us on this in 7.1 / 7.2 / 8.1.
- **Existing flows untouched**: Natural match-end, abandonment, pause, reconnect — all unchanged. `MatchEndPayload`'s additional fields are `omitempty` so wire format is identical; new `GameState` fields default to zero values that serialize correctly. No existing test changed its assertions.

### File List

**Created (9):**

- `server/migrations/000010_add_match_surrendered_by.up.sql`
- `server/migrations/000010_add_match_surrendered_by.down.sql`
- `server/internal/game/surrender.go`
- `server/internal/game/surrender_test.go`
- `client/src/features/game/components/SurrenderButton.tsx`
- `client/src/features/game/components/SurrenderButton.test.tsx`
- `client/src/features/game/components/SurrenderPrompt.tsx`
- `client/src/features/game/components/SurrenderPrompt.test.tsx`
- `client/src/features/game/components/SurrenderOpponentBanner.tsx`
- `client/src/features/game/components/SurrenderOpponentBanner.test.tsx`

**Modified (server, 7):**

- `server/internal/match/model.go` — added `SurrenderedBy *uint` field
- `server/internal/game/types.go` — added `ActionSurrenderRequest/Accept/Decline` constants
- `server/internal/game/state.go` — added `SurrenderProposerSeat *int` + `SurrenderUsed [4]bool`
- `server/internal/game/rules_engine.go` — wired three handler dispatches
- `server/internal/game/bidding.go` — extended `cloneGameState` for `SurrenderProposerSeat`
- `server/internal/apperr/errors.go` — added `ErrSurrenderExhausted` + `ErrInvalidAction`
- `server/internal/ws/events.go` — added action/event/error constants + `SurrenderProposedPayload` / `SurrenderDeclinedPayload` / typed `MatchEndPayload`
- `server/internal/session/manager.go` — typed match-end payload helper, three new broadcast cases, `handleMatchEnd` signature change, `sendGameError` mapping
- `server/internal/session/manager_test.go` — added 6 surrender end-to-end test cases

**Modified (client, 11):**

- `client/src/shared/types/wsEvents.ts` — surrender constants + payload interfaces, extended `MatchEndPayload`
- `client/src/shared/types/gameTypes.ts` — extended `GameState` and `ActionType` union
- `client/src/shared/stores/gameStore.ts` — `surrenderProposed` / `surrenderDeclined` slots + setters
- `client/src/shared/hooks/useWsDispatch.ts` — three new event branches + dedicated `error:surrender_exhausted` toast
- `client/src/features/game/GamePage.tsx` — surrender action handlers, derived flags, button stack, prompt, banner, MatchResult prop
- `client/src/features/game/components/MatchResult.tsx` — optional `surrenderedByUsername` prop + conditional surrender note
- `client/src/features/game/components/MatchResult.test.tsx` — added 3 new cases for surrender note rendering
- `client/src/shared/hooks/useWsDispatch.test.ts` — added 4 new cases for surrender dispatch
- `client/src/shared/i18n/en.json` — `game.surrender.*` block + `game.matchResult.surrenderNote`
- `client/src/shared/i18n/sr.json` — Serbian-Latin (Ekavian) mirror

**Test fixtures touched (5):** Added `surrenderProposerSeat: null` + `surrenderUsed: [false, false, false, false]` to existing `GameState` fixtures so pre-existing tests still type-check:

- `client/src/shared/types/gameTypes.test.ts`
- `client/src/shared/hooks/useWsDispatch.test.ts`
- `client/src/features/game/GamePage.test.tsx`
- `client/src/shared/hooks/useReconnectionRedirect.test.tsx`
- `client/src/shared/stores/gameStore.test.ts`
- `client/src/features/game/lib/legalCards.test.ts`

### Change Log

- 2026-04-27: Story 8.2 implemented — team surrender mechanic (request → partner accept/decline → match end with `outcomeReason: "surrender"` + persisted `surrendered_by`). Migration `000010` round-tripped on dev DB. All backend + frontend tests green; lint/prettier/i18n parity clean.
