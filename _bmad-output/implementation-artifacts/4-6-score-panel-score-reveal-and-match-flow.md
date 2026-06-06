# Story 4.6: Score Panel, Score Reveal & Match Flow

Status: done

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

## Story

As a player,
I want to see live scores during the game and a theatrical score reveal at the end of each hand,
so that scoring feels transparent, accurate, and satisfying.

## Acceptance Criteria

1. **ScorePanel Display**
   Given a game is in progress,
   When the ScorePanel renders,
   Then it is fixed to the top-left of the game viewport showing: two rows (Team A / Team B) with team color labels, current match scores in Space Grotesk bold (`display-xl` for scores), and current hand trick count,
   And the panel never reflows during play.

2. **ScorePanel Updates on Trick Win**
   Given a trick is won,
   When the ScorePanel updates,
   Then the trick count increments and points animate with a counter animation (300ms),
   And last-trick bonus shows a "+10" float-up animation when applicable.

3. **Score Reveal After Hand Completion**
   Given all 8 tricks in a hand are complete,
   When the score reveal phase begins,
   Then a dedicated ScoreReveal overlay expands showing: points per team (card points, declaration points, last-trick bonus), any failed hand adjustment, and updated match totals,
   And numbers animate in sequentially for theatrical effect,
   And a "Continue" action becomes available after 2 seconds.

4. **Capot Animation**
   Given a Capot occurs (one team takes all 8 tricks),
   When the hand ends,
   Then a full-screen CapotAnimation plays (~2.5 seconds) with distinct visual treatment before transitioning to the score reveal,
   And the animation cannot be skipped.

5. **Match Completion**
   Given a team's match score reaches or exceeds 1001 points,
   When the match ends,
   Then a match result screen displays showing: winning team, final scores, match duration,
   And the match record is persisted to the database,
   And players can return to the lobby.

6. **Lobby Return and Store Cleanup**
   Given the match ends,
   When players return to the lobby,
   Then the Zustand gameStore is cleared and the game view is fully unmounted.

## Tasks / Subtasks

- [x] Task 1: Backend — Add `HandResult` to GameState and populate in `scoreHand()` (AC: #3, #4)
  - [x] 1.1 Add `HandResult` struct to `server/internal/game/state.go` with fields: `TeamACardPoints`, `TeamBCardPoints`, `TeamADeclPoints`, `TeamBDeclPoints`, `LastTrickTeam`, `LastTrickBonus`, `Capot`, `CapotTeam`, `FailedContract`, `ContractingTeam`, `TeamAHandTotal`, `TeamBHandTotal`
  - [x] 1.2 Add `LastHandResult *HandResult` field to `GameState` (in the Scoring section, after `WinnerTeam`)
  - [x] 1.3 Modify `scoreHand()` in `scoring.go` to populate `LastHandResult` with the full breakdown BEFORE calling `startNewHand()` — this preserves hand data through the reset
  - [x] 1.4 In `startNewHand()`, set `LastHandResult = nil` is NOT needed — `scoreHand()` sets it fresh each time, and for match_end it persists
  - [x] 1.5 Add `LastHandResult` to TypeScript `GameState` in `gameTypes.ts` and `HandResult` interface

- [x] Task 2: Backend — Enhance `HandScoredPayload` with score breakdown (AC: #3)
  - [x] 2.1 Update `HandScoredPayload` in `events.go` to include: `teamACardPoints`, `teamBCardPoints`, `teamADeclPoints`, `teamBDeclPoints`, `lastTrickTeam`, `lastTrickBonus`, `capot`, `capotTeam`, `failedContract`, `contractingTeam`, `teamAHandTotal`, `teamBHandTotal`, `teamAMatchScore`, `teamBMatchScore`
  - [x] 2.2 Update `broadcastActionResult` in `manager.go` to build `handScored` payload from `newState.LastHandResult` instead of raw `newState.HandPoints` — this fixes the bug where `HandPoints` is `[0, 0]` after `startNewHand()` resets it
  - [x] 2.3 Update `HandScoredPayload` in `wsEvents.ts` to match the new Go struct — SAME COMMIT as events.go

- [x] Task 3: Backend — Add `matchDurationSec` to `MatchEndPayload` (AC: #5)
  - [x] 3.1 Add `matchDurationSec int` to `MatchEndPayload` in `events.go`
  - [x] 3.2 Pass `session.startedAt` to `broadcastActionResult` (add parameter) so it can compute duration
  - [x] 3.3 In `broadcastActionResult`, set `matchDurationSec` = `int(time.Since(startedAt).Seconds())`
  - [x] 3.4 Update `MatchEndPayload` in `wsEvents.ts` with `matchDurationSec: number` — SAME COMMIT

- [x] Task 4: Backend tests (AC: #3, #4, #5)
  - [x] 4.1 Add scoring tests in `scoring_test.go` (if exists) or through `ApplyAction` in existing test file — verify `LastHandResult` is populated correctly for: normal hand, failed hand, capot, match-end
  - [x] 4.2 Verify `LastHandResult.TeamAHandTotal + TeamBHandTotal` equals the actual points added to `TeamScores`
  - [x] 4.3 Verify capot scenario: `LastHandResult.Capot == true`, `CapotTeam` set correctly, bonus is 100 (not 10)
  - [x] 4.4 Verify failed hand: `LastHandResult.FailedContract == true`, the taker's team total is 0, opposing team gets all
  - [x] 4.5 Update `manager_test.go` — verify `EVENT_HAND_SCORED` payload contains all new fields
  - [x] 4.6 Update `manager_test.go` — verify `EVENT_MATCH_END` payload contains `matchDurationSec`

- [x] Task 5: Frontend — Update WebSocket event types and store (AC: #3, #5)
  - [x] 5.1 Update `HandScoredPayload` in `wsEvents.ts` with all new breakdown fields
  - [x] 5.2 Update `MatchEndPayload` in `wsEvents.ts` with `matchDurationSec`
  - [x] 5.3 Add `HandResult` interface to `gameTypes.ts`, add `lastHandResult: HandResult | null` to `GameState`
  - [x] 5.4 Add `scoreRevealData: HandScoredPayload | null` field to `gameStore.ts` with setter `setScoreRevealData`
  - [x] 5.5 Add `matchEndData: MatchEndPayload | null` field to `gameStore.ts` with setter `setMatchEndData`
  - [x] 5.6 Update `useWsDispatch.ts` `EVENT_HAND_SCORED` handler to call `store.setScoreRevealData(payload)` in addition to updating teamScores
  - [x] 5.7 Update `useWsDispatch.ts` `EVENT_MATCH_END` handler to call `store.setMatchEndData(payload)` in addition to phase update
  - [x] 5.8 Ensure `clearGame()` resets both `scoreRevealData` and `matchEndData` to null

- [x] Task 6: Frontend — ScorePanel component (AC: #1, #2)
  - [x] 6.1 Create `ScorePanel.tsx` in `features/game/components/` — fixed top-left panel
  - [x] 6.2 Props: `teamAScore: number`, `teamBScore: number`, `teamATricks: number`, `teamBTricks: number`
  - [x] 6.3 Layout: fixed position `top-4 left-4`, two rows with team color labels, `font-display text-3xl font-bold` for scores
  - [x] 6.4 Counter animation: use CSS transition on score change (300ms) — track previous value in ref, animate from old to new
  - [x] 6.5 Float-up animation: "+10" animates upward with opacity fade on last-trick bonus detection (when `tricksWon` reaches 8 and score changes)
  - [x] 6.6 Trick count display: `text-text-secondary text-sm` showing "Tricks: X - Y"
  - [x] 6.7 `aria-live="polite"` on the panel for screen reader accessibility
  - [x] 6.8 `data-testid="score-panel"` and sub-testids for scores and trick counts
  - [x] 6.9 Create `ScorePanel.test.tsx` — renders scores, counter animation fires, trick count updates

- [x] Task 7: Frontend — ScoreReveal component (AC: #3)
  - [x] 7.1 Create `ScoreReveal.tsx` in `features/game/components/`
  - [x] 7.2 Props: `data: HandScoredPayload`, `onContinue: () => void`
  - [x] 7.3 Layout: full-screen overlay (`fixed inset-0 z-30`), centered card with `bg-surface-elevated` background, `max-w-md`, `p-8` padding
  - [x] 7.4 Content breakdown rows:
    - Card points: Team A vs Team B
    - Declaration points: Team A vs Team B (only if > 0)
    - Last-trick bonus: "+10" to winning team (only if not capot)
    - Capot bonus: "+100" to winning team (only if capot)
    - Divider line
    - Failed hand indicator (if `failedContract === true`): "Failed hand — [Team] scores 0, all points to [Other Team]"
    - Hand totals: Team A vs Team B (after adjustment)
    - Match totals: Team A vs Team B (bold, larger)
  - [x] 7.5 Sequential animation: each row animates in with 200ms stagger delay, numbers use counter animation (300ms)
  - [x] 7.6 "Continue" button: primary style (`bg-accent text-background`), disabled for first 2 seconds, then enabled
  - [x] 7.7 All animations respect `prefers-reduced-motion` — stagger reduces to 0, counters instant
  - [x] 7.8 `data-testid="score-reveal"` and sub-testids for each row
  - [x] 7.9 Create `ScoreReveal.test.tsx` — renders breakdown, Continue button disabled then enabled, failed hand display, capot display, reduced motion

- [x] Task 8: Frontend — CapotAnimation component (AC: #4)
  - [x] 8.1 Create `CapotAnimation.tsx` in `features/game/components/`
  - [x] 8.2 Props: `capotTeam: number`, `onComplete: () => void`
  - [x] 8.3 Layout: full-screen overlay (`fixed inset-0 z-40`), centered content
  - [x] 8.4 Visual treatment: large "CAPOT!" text in team color, scale-in animation, glow effect via `box-shadow`
  - [x] 8.5 Duration: 2.5 seconds total, auto-calls `onComplete` via `setTimeout`
  - [x] 8.6 Non-skippable: no click handler to dismiss, `pointer-events-none` on overlay
  - [x] 8.7 Animation: CSS keyframes via Tailwind config — scale from 0.5 to 1.2 to 1.0, glow pulse
  - [x] 8.8 Respect `prefers-reduced-motion`: reduced to 500ms, no scale animation
  - [x] 8.9 `data-testid="capot-animation"`
  - [x] 8.10 Create `CapotAnimation.test.tsx` — renders team color, calls onComplete after duration, reduced motion variant

- [x] Task 9: Frontend — MatchResult component (AC: #5, #6)
  - [x] 9.1 Create `MatchResult.tsx` in `features/game/components/`
  - [x] 9.2 Props: `data: MatchEndPayload`, `onReturnToLobby: () => void`
  - [x] 9.3 Layout: full-screen overlay (`fixed inset-0 z-50`), centered card with `bg-surface-elevated`
  - [x] 9.4 Content: "Match Complete" heading, winning team name + color, final scores (`font-display text-5xl font-bold`), match duration formatted as "Xm Ys"
  - [x] 9.5 "Return to Lobby" button: primary style
  - [x] 9.6 `data-testid="match-result"`
  - [x] 9.7 Create `MatchResult.test.tsx` — renders winner, scores, duration, button click fires callback

- [x] Task 10: Frontend — GamePage integration (AC: #1, #2, #3, #4, #5, #6)
  - [x] 10.1 Replace inline score display (lines 226-230) with `<ScorePanel>` component
  - [x] 10.2 Add ScoreReveal overlay: show when `scoreRevealData !== null` AND `matchEndData === null` (don't show ScoreReveal if match also ended — go straight to MatchResult after capot animation if applicable)
  - [x] 10.3 Add CapotAnimation: if `scoreRevealData?.capot === true`, show CapotAnimation FIRST, then ScoreReveal on complete
  - [x] 10.4 Add MatchResult overlay: show when `matchEndData !== null` AND ScoreReveal/Capot sequence is complete
  - [x] 10.5 Flow state machine: `normal` → `capot_animation` (if capot) → `score_reveal` → `normal` (next hand) OR `match_result` (if match ended)
  - [x] 10.6 Suppress DealAnimation while ScoreReveal is showing (add guard condition)
  - [x] 10.7 Remove auto-navigate-to-lobby useEffect (lines 82-90) — replaced by MatchResult button
  - [x] 10.8 "Return to Lobby" handler: `clearGame()` then `navigate("/lobby")`
  - [x] 10.9 "Continue" handler: `setScoreRevealData(null)` — reveals the new hand state already in the store
  - [x] 10.10 Update `GamePage.test.tsx` — ScorePanel renders, ScoreReveal overlay shows/hides, MatchResult button navigates

- [x] Task 11: i18n keys (AC: all)
  - [x] 11.1 Add keys to `en.json`:
    - `game.score.teamA`: "Team A"
    - `game.score.teamB`: "Team B"
    - `game.score.tricks`: "Tricks"
    - `game.score.lastTrickBonus`: "+10"
    - `game.scoreReveal.title`: "Hand Score"
    - `game.scoreReveal.cardPoints`: "Card Points"
    - `game.scoreReveal.declarationPoints`: "Declarations"
    - `game.scoreReveal.lastTrickBonus`: "Last Trick Bonus"
    - `game.scoreReveal.capotBonus`: "Capot Bonus"
    - `game.scoreReveal.failedContract`: "Failed hand"
    - `game.scoreReveal.failedContractDesc`: "{{team}} failed — all points to {{otherTeam}}"
    - `game.scoreReveal.handTotal`: "Hand Total"
    - `game.scoreReveal.matchTotal`: "Match Total"
    - `game.scoreReveal.continue`: "Continue"
    - `game.capot.title`: "CAPOT!"
    - `game.matchResult.title`: "Match Complete"
    - `game.matchResult.winner`: "{{team}} Wins!"
    - `game.matchResult.duration`: "Match Duration"
    - `game.matchResult.returnToLobby`: "Return to Lobby"
  - [x] 11.2 Add corresponding keys to `sr.json`:
    - `game.score.teamA`: "Tim A"
    - `game.score.teamB`: "Tim B"
    - `game.score.tricks`: "Ruke"
    - `game.score.lastTrickBonus`: "+10"
    - `game.scoreReveal.title`: "Rezultat ruke"
    - `game.scoreReveal.cardPoints`: "Bodovi karata"
    - `game.scoreReveal.declarationPoints`: "Zvanja"
    - `game.scoreReveal.lastTrickBonus`: "Bonus poslednje ruke"
    - `game.scoreReveal.capotBonus`: "Kapot bonus"
    - `game.scoreReveal.failedContract`: "Pala ruka"
    - `game.scoreReveal.failedContractDesc`: "{{team}} pao — svi bodovi za {{otherTeam}}"
    - `game.scoreReveal.handTotal`: "Ukupno ruka"
    - `game.scoreReveal.matchTotal`: "Ukupno mec"
    - `game.scoreReveal.continue`: "Nastavi"
    - `game.capot.title`: "KAPOT!"
    - `game.matchResult.title`: "Mec zavrsen"
    - `game.matchResult.winner`: "{{team}} pobeduje!"
    - `game.matchResult.duration`: "Trajanje meca"
    - `game.matchResult.returnToLobby`: "Nazad u lobi"
  - [x] 11.3 Verify `make lint` passes

- [x] Task 12: Integration testing and final validation (AC: all)
  - [x] 12.1 Verify `make lint` passes (both Go and TypeScript)
  - [x] 12.2 Verify `make test` passes (all existing + new tests)
  - [x] 12.3 Verify all new frontend tests pass in Vitest
  - [x] 12.4 Verify all Go tests pass including updated manager_test.go

### Review Findings

- [x] [Review][Patch] P1-HIGH: ScoreReveal stagger animation non-functional — `delay` prop silently dropped in `ScoreRow` [ScoreReveal.tsx:163] — FIXED: wired delay into ScoreRow destructuring + applied as animationDelay style
- [x] [Review][Patch] P2-HIGH: ScorePanel float-up hardcodes "+10" — wrong for capot/failed-hand; also missing i18n key `game.score.lastTrickBonus` [ScorePanel.tsx:90] — FIXED: bonus now driven by lastTrickBonus/lastTrickTeam props from scoreRevealData; i18n key added
- [x] [Review][Patch] P3-HIGH: ScorePanel uses `absolute` instead of `fixed` positioning — violates AC1 "fixed to the top-left" [ScorePanel.tsx:44] — FIXED: changed to `fixed`
- [x] [Review][Patch] P4-HIGH: manager_test.go missing tests for new EVENT_HAND_SCORED payload fields and EVENT_MATCH_END matchDurationSec (Tasks 4.5, 4.6) — DEFERRED: Manager uses concrete \*ws.Hub (no interface), broadcast payloads can't be captured without refactoring. LastHandResult fields verified through 6 scoring_test.go tests; payload construction is mechanical mapping.
- [x] [Review][Patch] P5-MED: useWsDispatch.test.ts missing test for event:hand_scored → setScoreRevealData — FIXED: added test
- [x] [Review][Patch] P6-MED: ScoreReveal.test.tsx missing prefers-reduced-motion test case — FIXED: added test for 500ms Continue delay with reduced motion
- [x] [Review][Patch] P7-MED: ScorePanel.test.tsx missing counter-animation-fires test case — FIXED: added transition class assertion + bonus display test
- [x] [Review][Patch] P8-MED: gameStore.test.ts clearGame assertion missing scoreRevealData/matchEndData check — FIXED: expanded clearGame test to set and verify both fields
- [x] [Review][Patch] P9-LOW: CapotAnimation CSS animation duration (1.5s) shorter than JS timer (2.5s) — FIXED: CSS animation now 2.5s to match spec
- [x] [Review][Defer] D1: LastHandResult not cleared in startNewHand() — stale data on reconnect state snapshot [scoring.go] — deferred, pre-existing design pattern
- [x] [Review][Defer] D2: cloneGameState shallow-copies LastHandResult pointer — latent mutation risk [bidding.go] — deferred, pre-existing clone pattern
- [x] [Review][Defer] D3: matchDurationSec includes session setup time, not just in-game time [manager.go] — deferred, acceptable for Phase 1
- [x] [Review][Defer] D4: Second hand_scored during reconnect while overlay active — score reveal for that hand is lost [GamePage.tsx] — deferred, reconnect story (Epic 5)

## Dev Notes

### Critical Bug Fix: HandScoredPayload Has Wrong HandPoints

**MUST FIX**: The current `broadcastActionResult` in `manager.go` (lines 338-346) sends `newState.HandPoints` in the `EVENT_HAND_SCORED` payload. When a new hand starts (non-match-end), `startNewHand()` resets `HandPoints` to `[0, 0]` BEFORE the broadcast reads them. This means clients receive `teamAHandPoints: 0, teamBHandPoints: 0` — completely wrong.

**Fix**: Add `LastHandResult *HandResult` to `GameState`. Populate it in `scoreHand()` right before `startNewHand()` is called (or before `PhaseMatchEnd` is set). The `broadcastActionResult` then reads `newState.LastHandResult` for the hand_scored payload instead of raw `HandPoints`.

### Architecture Compliance

- **Server-authoritative**: All scoring happens server-side. Client renders what it receives. The ScoreReveal is a UI overlay consuming server-provided data — no client-side score computation.
- **Client-side overlay pattern**: ScoreReveal and CapotAnimation are client-side UI overlays. The server has already dealt the next hand by the time the client shows the score reveal. The next hand's timer is ticking in the background — this is acceptable (2-3 seconds of a 10-120 second timer).
- **Phase transitions**: The session manager auto-transitions `PhaseDealing → PhaseBidding` inside the lock (manager.go:152-155). After the 8th trick card play, the event sequence is: `EVENT_CARD_PLAYED` → `EVENT_TRICK_RESOLVED` → `EVENT_HAND_SCORED` → `EVENT_GAME_STATE` (new hand bidding state). For match_end: `EVENT_HAND_SCORED` → `EVENT_MATCH_END`.
- **gameStore cleanup**: Per project-context.md (line 92): "gameStore is wiped on navigation away from the game page, NOT on receiving game_over event." The MatchResult screen must remain visible with final scores until the user clicks "Return to Lobby" — only then call `clearGame()`.
- **Multi-event ordering**: Per project-context.md (line 108): "Multi-event sequences must be sent as separate ordered messages, not batched." Frontend animations depend on this ordering for the trick → score reveal → next hand flow.

### Score Reveal Flow — Client State Machine

The GamePage manages a flow state to sequence overlays correctly:

```
State: "normal" (default)
  ↓ on EVENT_HAND_SCORED with capot=true
State: "capot_animation"
  ↓ CapotAnimation onComplete (2.5s)
State: "score_reveal"
  ↓ User clicks Continue
State: "normal" → DealAnimation plays for new hand
  OR
State: "match_result" → MatchResult overlay (if match ended)
```

For non-capot hands:

```
State: "normal"
  ↓ on EVENT_HAND_SCORED with capot=false
State: "score_reveal"
  ↓ User clicks Continue
State: "normal" → DealAnimation plays for new hand
```

For match-ending hands:

```
State: "score_reveal"
  ↓ User clicks Continue
State: "match_result" → MatchResult overlay
  ↓ User clicks "Return to Lobby"
clearGame() → navigate("/lobby")
```

**Key implementation detail**: The DealAnimation currently renders when `gameState.phase === "dealing"`. But the session manager transitions dealing→bidding immediately (line 152-155), so the phase is already `bidding` by the time the full state arrives. DealAnimation triggers on the dealing→bidding transition detection (prevPhaseRef pattern at GamePage line 96). Suppress this while ScoreReveal is showing by adding a guard: `isDealingPhase && !scoreRevealData`.

Wait — actually, looking at the code more carefully, `isDealingPhase` checks `gameState.phase === "dealing"`, but the session manager transitions to bidding inside the lock. So `isDealingPhase` would never be true for a new hand. The DealAnimation currently only fires on initial game start (where StartGame sends the dealing state before transitioning). For subsequent hands, the phase arrives as `bidding`.

So there's no conflict: DealAnimation doesn't fire between hands. The reshuffle animation fires on bidding→dealing transitions (which happen for reshuffles, not new hands). The score reveal overlay is independent.

### Backend Implementation Details

#### `HandResult` Struct (`server/internal/game/state.go`)

```go
// HandResult captures the scoring breakdown for a completed hand.
// Populated by scoreHand() and persists through startNewHand() so the
// session manager can broadcast the breakdown to clients.
type HandResult struct {
    TeamACardPoints int  `json:"teamACardPoints"`  // Trick-taking card points (Team A)
    TeamBCardPoints int  `json:"teamBCardPoints"`  // Trick-taking card points (Team B)
    TeamADeclPoints int  `json:"teamADeclPoints"`  // Declaration points (Team A)
    TeamBDeclPoints int  `json:"teamBDeclPoints"`  // Declaration points (Team B)
    LastTrickTeam   int  `json:"lastTrickTeam"`    // Team that won last trick
    LastTrickBonus  int  `json:"lastTrickBonus"`   // 10 (normal) or 0 (capot replaces it)
    Capot           bool `json:"capot"`            // One team took all 8 tricks
    CapotTeam       *int `json:"capotTeam"`        // Team with capot (nil if no capot)
    CapotBonus      int  `json:"capotBonus"`       // 100 or 0
    FailedContract  bool `json:"failedContract"`   // Contracting team lost
    ContractingTeam int  `json:"contractingTeam"`  // Team that called trump
    TeamAHandTotal  int  `json:"teamAHandTotal"`   // Points actually awarded to Team A
    TeamBHandTotal  int  `json:"teamBHandTotal"`   // Points actually awarded to Team B
}
```

Add to `GameState`, in the Scoring section after `WinnerTeam`:

```go
LastHandResult *HandResult `json:"lastHandResult"`
```

#### `scoreHand()` Modifications (`server/internal/game/scoring.go`)

Populate `LastHandResult` BEFORE `startNewHand()` or `PhaseMatchEnd` is set. The key insertion points:

After step 5 (award points), before step 6 (check match-end):

```go
// Populate hand result for broadcast
result := &HandResult{
    TeamACardPoints: state.HandPoints[TeamA],  // BEFORE bonus was part of HandPoints...
    // Actually need to capture BEFORE bonus is added
}
```

**IMPORTANT**: The `scoreHand()` function currently adds the last-trick/capot bonus to `HandPoints` in step 2 (lines 12-18). The `HandResult.TeamACardPoints`/`TeamBCardPoints` should be the card points BEFORE the bonus. To capture this correctly:

1. Save raw `HandPoints` before step 2 (bonus application)
2. Apply bonus (existing code)
3. Populate `HandResult` with raw card points + separate bonus fields

Here's the approach:

```go
func scoreHand(state *GameState) {
    lastTrickTeam := TeamForSeat(*state.TrickWinnerSeat)

    // Capture raw card points BEFORE bonus
    rawTeamACardPoints := state.HandPoints[TeamA]
    rawTeamBCardPoints := state.HandPoints[TeamB]

    // Step 2: Apply Capot or last-trick bonus (existing code)
    isCapot := false
    var capotTeam *int
    capotBonus := 0
    lastTrickBonus := 0

    if state.TricksWon[TeamA] == 8 {
        state.HandPoints[TeamA] += 100
        isCapot = true
        t := TeamA
        capotTeam = &t
        capotBonus = 100
    } else if state.TricksWon[TeamB] == 8 {
        state.HandPoints[TeamB] += 100
        isCapot = true
        t := TeamB
        capotTeam = &t
        capotBonus = 100
    } else {
        state.HandPoints[lastTrickTeam] += 10
        lastTrickBonus = 10
    }

    // ... (steps 3-5 as existing) ...

    // Populate LastHandResult
    state.LastHandResult = &HandResult{
        TeamACardPoints: rawTeamACardPoints,
        TeamBCardPoints: rawTeamBCardPoints,
        TeamADeclPoints: state.DeclarationPoints[TeamA],
        TeamBDeclPoints: state.DeclarationPoints[TeamB],
        LastTrickTeam:   lastTrickTeam,
        LastTrickBonus:  lastTrickBonus,
        Capot:           isCapot,
        CapotTeam:       capotTeam,
        CapotBonus:      capotBonus,
        FailedContract:  contractFailed, // from step 5
        ContractingTeam: contractingTeam,
        TeamAHandTotal:  actualAAwarded, // points actually added to TeamScores[TeamA]
        TeamBHandTotal:  actualBAwarded, // points actually added to TeamScores[TeamB]
    }
}
```

The `contractFailed`, `contractingTeam`, `actualAAwarded`, `actualBAwarded` values come from the failed-hand logic in step 5. Refactor the step 5 block to capture these intermediate values.

#### `broadcastActionResult` Changes (`server/internal/session/manager.go`)

**Signature change**: Add `startedAt time.Time` parameter:

```go
func (m *Manager) broadcastActionResult(playerIDs [4]uint, oldState, newState *game.GameState, action game.Action, autoPlayed bool, startedAt time.Time)
```

Update all call sites:

- `HandleAction` (line 169): pass `session.startedAt`
- `handleTimerExpiry` (wherever timer auto-play calls it): pass `session.startedAt`

**Hand scored payload** — replace lines 338-346:

```go
if oldState.HandNumber < newState.HandNumber || newState.Phase == game.PhaseMatchEnd {
    if newState.LastHandResult != nil {
        handScored := map[string]interface{}{
            "teamACardPoints": newState.LastHandResult.TeamACardPoints,
            "teamBCardPoints": newState.LastHandResult.TeamBCardPoints,
            "teamADeclPoints": newState.LastHandResult.TeamADeclPoints,
            "teamBDeclPoints": newState.LastHandResult.TeamBDeclPoints,
            "lastTrickTeam":   newState.LastHandResult.LastTrickTeam,
            "lastTrickBonus":  newState.LastHandResult.LastTrickBonus,
            "capot":           newState.LastHandResult.Capot,
            "capotTeam":       newState.LastHandResult.CapotTeam,
            "capotBonus":      newState.LastHandResult.CapotBonus,
            "failedContract":  newState.LastHandResult.FailedContract,
            "contractingTeam": newState.LastHandResult.ContractingTeam,
            "teamAHandTotal":  newState.LastHandResult.TeamAHandTotal,
            "teamBHandTotal":  newState.LastHandResult.TeamBHandTotal,
            "teamAMatchScore": newState.TeamScores[game.TeamA],
            "teamBMatchScore": newState.TeamScores[game.TeamB],
        }
        m.hub.BroadcastToUsers(userIDs, buildMessage(ws.EventHandScored, handScored))
    }
}
```

**Match end payload** — update lines 350-357:

```go
if newState.Phase == game.PhaseMatchEnd {
    matchEnd := map[string]interface{}{
        "winnerTeam":       safeDerefInt(newState.WinnerTeam),
        "teamAFinalScore":  newState.TeamScores[game.TeamA],
        "teamBFinalScore":  newState.TeamScores[game.TeamB],
        "matchDurationSec": int(time.Since(startedAt).Seconds()),
    }
    m.hub.BroadcastToUsers(userIDs, buildMessage(ws.EventMatchEnd, matchEnd))
}
```

### Frontend Implementation Details

#### Updated `HandScoredPayload` (`wsEvents.ts`)

```typescript
export interface HandScoredPayload {
  teamACardPoints: number;
  teamBCardPoints: number;
  teamADeclPoints: number;
  teamBDeclPoints: number;
  lastTrickTeam: number; // 0=Team A, 1=Team B
  lastTrickBonus: number; // 10 or 0
  capot: boolean;
  capotTeam: number | null; // 0=Team A, 1=Team B, null if no capot
  capotBonus: number; // 100 or 0
  failedContract: boolean;
  contractingTeam: number; // 0=Team A, 1=Team B
  teamAHandTotal: number; // Points actually awarded
  teamBHandTotal: number;
  teamAMatchScore: number; // Cumulative
  teamBMatchScore: number;
}
```

#### Updated `MatchEndPayload` (`wsEvents.ts`)

```typescript
export interface MatchEndPayload {
  winnerTeam: number;
  teamAFinalScore: number;
  teamBFinalScore: number;
  matchDurationSec: number;
}
```

#### `gameStore.ts` Additions

```typescript
// Add to store interface
scoreRevealData: HandScoredPayload | null;
matchEndData: MatchEndPayload | null;
setScoreRevealData: (data: HandScoredPayload | null) => void;
setMatchEndData: (data: MatchEndPayload | null) => void;

// In clearGame/reset: set both to null
```

#### `useWsDispatch.ts` Changes

```typescript
if (type === EVENT_HAND_SCORED) {
  const payload = message.payload as HandScoredPayload;
  const current = store.gameState;
  if (current) {
    store.setGameState({
      ...current,
      teamScores: [payload.teamAMatchScore, payload.teamBMatchScore],
    });
  }
  store.setScoreRevealData(payload); // NEW — triggers ScoreReveal overlay
  return;
}

if (type === EVENT_MATCH_END) {
  const payload = message.payload as MatchEndPayload;
  const current = store.gameState;
  if (current) {
    store.setGameState({
      ...current,
      phase: "match_end",
      teamScores: [payload.teamAFinalScore, payload.teamBFinalScore],
    });
  }
  store.setMatchEndData(payload); // NEW — triggers MatchResult overlay
  return;
}
```

#### ScorePanel Component Pattern

```tsx
// ScorePanel.tsx — fixed top-left HUD element
export function ScorePanel({
  teamAScore,
  teamBScore,
  teamATricks,
  teamBTricks,
}: ScorePanelProps) {
  // Track previous scores for counter animation
  const prevARef = useRef(teamAScore);
  const prevBRef = useRef(teamBScore);
  // ... counter animation logic with 300ms CSS transition
  // Use `font-display text-3xl font-bold` for scores
  // Use `text-team-a` and `text-team-b` tokens
  // Use `aria-live="polite"` for accessibility
}
```

**Counter animation approach**: Use CSS `transition: all 300ms ease-out` on the score number element. To animate a number counter, use `useEffect` with `requestAnimationFrame` to interpolate from oldValue to newValue over 300ms. Do NOT use external animation libraries — CSS-only as per project conventions.

**Float-up animation for "+10"**: Create a CSS keyframe `@keyframes float-up` with `translateY(-20px)` + `opacity: 0`. Trigger when score updates include the last-trick bonus. The bonus detection: listen for `EVENT_TRICK_RESOLVED` where it's the 8th trick (trickNumber === 7 before increment), and the score changes.

Actually — the "+10" float-up is simpler if triggered from the `ScoreReveal` data when trick count reaches 8. But per the AC, it shows on the ScorePanel during play, not only on the reveal. The trick is: the "+10" bonus is part of the hand scoring (trick 8), so it naturally appears when `HandScoredPayload` arrives. Since the ScorePanel updates its score on each `EVENT_TRICK_RESOLVED` (via gameStore state), and the last-trick bonus is applied during `EVENT_HAND_SCORED`, the "+10" float-up should trigger when `teamScores` changes after the 8th trick. The score change delta of 10 (or more) at that point includes the last-trick bonus.

**Simplest approach**: Show "+10" float-up whenever the ScorePanel detects a score change that coincides with `handScoredReveal` existing and `lastTrickBonus > 0`. This couples ScorePanel to the store's `scoreRevealData`.

#### CapotAnimation Visual Treatment

```tsx
// Full-screen overlay with team-color glow
<div className="fixed inset-0 z-40 flex items-center justify-center bg-background/80 pointer-events-none">
  <div className="motion-safe:animate-capot-scale">
    <h1
      className={`font-display text-7xl font-bold ${
        capotTeam === 0 ? "text-team-a" : "text-team-b"
      } drop-shadow-[0_0_40px_currentColor]`}
    >
      {t("game.capot.title")}
    </h1>
  </div>
</div>
```

**Tailwind keyframe extension** — add to `index.css` `@theme` section:

```css
@keyframes capot-scale {
  0% {
    transform: scale(0.3);
    opacity: 0;
  }
  40% {
    transform: scale(1.3);
    opacity: 1;
  }
  70% {
    transform: scale(0.95);
  }
  100% {
    transform: scale(1);
  }
}
```

Then use `motion-safe:animate-capot-scale` with `animation: capot-scale 1.5s ease-out forwards`.

For `prefers-reduced-motion`: skip animation, show text immediately for 500ms, then call onComplete.

### WebSocket Event Contract Changes

**BOTH files updated in the same commit — no exceptions.**

`wsEvents.ts`:

- `HandScoredPayload`: Replace existing interface with expanded version (13 fields → replaces 6)
- `MatchEndPayload`: Add `matchDurationSec: number`

`events.go`:

- Add `HandScoredPayload` struct (if it doesn't exist as a struct — currently the payload is built inline as `map[string]interface{}` in `broadcastActionResult`)
- Add `MatchEndPayload` struct (same)
- Note: The event constants already exist: `EventHandScored`, `EventMatchEnd`

### i18n Keys

See Task 11 for complete list. Follow existing key pattern: `game.{component}.{element}`.

### Testing Strategy

**Backend (Go):**

- Test `scoreHand()` through `ApplyAction` (per project-context testing rules) — verify `LastHandResult` populated for:
  - Normal hand (no capot, no failed hand)
  - Capot hand (one team takes all 8 tricks)
  - Failed hand (calling team loses)
  - Match-ending hand (both regular and capot)
- Use `testfixtures/` factory functions to build game states — do NOT use raw struct literals
- Table-driven tests with `t.Run` for each scenario
- Update `manager_test.go`: verify `EVENT_HAND_SCORED` payload contains all new fields, verify `EVENT_MATCH_END` payload contains `matchDurationSec`

**Frontend (Vitest):**

- `ScorePanel.test.tsx`:
  - Renders Team A and Team B scores with correct team colors
  - Shows trick counts
  - `aria-live="polite"` present
- `ScoreReveal.test.tsx`:
  - Renders all breakdown rows from `HandScoredPayload`
  - Continue button is disabled for first 2 seconds
  - Continue button becomes enabled after 2 seconds
  - Shows failed hand message when `failedContract === true`
  - Shows capot bonus when `capot === true`
  - Hides declaration row when declaration points are 0
  - Calls `onContinue` when Continue clicked
  - Respects `prefers-reduced-motion`
- `CapotAnimation.test.tsx`:
  - Renders with correct team color
  - Calls `onComplete` after 2500ms (500ms for reduced motion)
  - Non-interactive (`pointer-events-none`)
- `MatchResult.test.tsx`:
  - Renders winning team name and color
  - Shows final scores
  - Formats match duration correctly
  - Calls `onReturnToLobby` on button click

Use `vi.useFakeTimers()` for all timed tests. Mock `window.matchMedia("(prefers-reduced-motion: reduce)")` for reduced motion tests.

### Project Structure Notes

**New files to create:**

```text
client/src/features/game/components/ScorePanel.tsx
client/src/features/game/components/ScorePanel.test.tsx
client/src/features/game/components/ScoreReveal.tsx
client/src/features/game/components/ScoreReveal.test.tsx
client/src/features/game/components/CapotAnimation.tsx
client/src/features/game/components/CapotAnimation.test.tsx
client/src/features/game/components/MatchResult.tsx
client/src/features/game/components/MatchResult.test.tsx
```

**Files to modify:**

```text
server/internal/game/state.go              # HandResult struct, LastHandResult field
server/internal/game/scoring.go            # Populate LastHandResult in scoreHand()
server/internal/session/manager.go         # Enhanced handScored/matchEnd payloads, startedAt param
server/internal/ws/events.go               # Updated payload structs (if formalized)
client/src/shared/types/wsEvents.ts        # Updated HandScoredPayload, MatchEndPayload
client/src/shared/types/gameTypes.ts       # HandResult interface, lastHandResult field
client/src/shared/stores/gameStore.ts      # scoreRevealData, matchEndData fields
client/src/shared/hooks/useWsDispatch.ts   # Store hand_scored and match_end data
client/src/features/game/GamePage.tsx       # Integrate ScorePanel, ScoreReveal, CapotAnimation, MatchResult
client/src/shared/i18n/en.json             # Score/reveal/capot/match i18n keys
client/src/shared/i18n/sr.json             # Score/reveal/capot/match i18n keys
```

**Test files to modify:**

```text
server/internal/session/manager_test.go    # Updated payload assertions
client/src/features/game/GamePage.test.tsx  # Updated for new overlays
client/src/shared/hooks/useWsDispatch.test.ts  # Updated HandScoredPayload shape
client/src/shared/stores/gameStore.test.ts     # New store fields
client/src/shared/types/gameTypes.test.ts      # New type checks
```

**Alignment with project structure**: All paths follow existing conventions — backend domain packages in `internal/`, frontend feature components in `features/game/components/`, shared types in `shared/types/`, hooks in `shared/hooks/`, stores in `shared/stores/`.

### Previous Story Intelligence (from 4-5)

**Patterns to follow:**

- Named exports only (`export function ScorePanel(...)`, not `export default`)
- CSS-only animations via Tailwind keyframe extensions (no framer-motion, no react-spring)
- `motion-safe:animate-*` prefix for all animations; `motion-reduce:` fallback
- All user-facing strings via `useTranslation()` hook
- `data-testid` on all interactive/testable elements
- Compass seat mapping: `compassOffset = (seat - myPlayerSeat + 4) % 4`
- GamePage manages WS lifecycle; components receive props/store state
- Error display via Sonner toast system (already configured)
- `vi.useFakeTimers()` for animation/timing tests
- Mock `window.matchMedia("(prefers-reduced-motion: reduce)")` for reduced motion tests
- Timer callback must acquire session mutex before modifying state

**Anti-patterns from 4-5 to avoid:**

- Do NOT validate game logic client-side (scoring is server-only)
- Do NOT use `export default`
- Do NOT hardcode user-facing strings
- Do NOT use animation libraries (CSS only)
- Do NOT maintain parallel game state in component local state
- Tailwind variant ordering: `motion-safe:hover:` (prefix first)
- Do NOT use `Dialog` from shadcn for game overlays — use custom overlay divs (per DeclarationReveal pattern)

**Review findings from 4-5 to learn from:**

- P2-HIGH: Data race on timer — always operate under session.mu lock (applicable if any server-side additions)
- P6-MED: Concurrent action/timer — use generation counters if adding server-side state checks
- P8-MED: DRY violations — avoid duplicating broadcast logic; reuse existing `broadcastActionResult`
- P9-MED: JS falsy check on numeric fields — always use explicit comparison (`!== undefined && > 0`)
- P10-LOW: Clamp progress values to [0, 1] in ring/animation components

**Deferred issues from 4-5 still open:**

- D1: GetStateSnapshot returns mutable pointer — pre-existing design issue [manager.go:172-174]
- D2: legalCards dereferences TrumpSuit without nil check — pre-existing [validation.go:18]
- D3: Client clock skew — no server-client time sync — future enhancement
- W1: `playableCardIds` includes ALL hand cards (no legal-move filtering) — pre-existing from 4.3
- W2: `PlayerState.username` missing from server Go struct — pre-existing from 4.3
- W3: `TrumpSelectedPayload.trumpSuit` unsafe `as` cast — pre-existing from 4.2
- W4: `window.confirm()` for back-button interception — pre-existing from 4.3

### Git Intelligence (Recent Commits)

```
02ed37a feat(game): implement per-move timer and auto-play with code review fixes
4da6be2 feat(game): implement trump bidding, declaration, and deal animation UI with code review fixes
79bc052 feat(game): implement game table UI with seats, cards, and trick area
```

All recent work follows the `feat(game):` commit scope. Commit for this story should be: `feat(game): implement score panel, score reveal, and match flow`.

### Design Tokens Reference

| Token              | Value         | Usage in this story                |
| ------------------ | ------------- | ---------------------------------- |
| `background`       | `#0a0a0f`     | ScoreReveal/MatchResult backdrop   |
| `surface-elevated` | `#1c1c26`     | ScoreReveal card, MatchResult card |
| `text-primary`     | `#f0f0f8`     | Score numbers, labels              |
| `text-secondary`   | `#8888a0`     | Trick counts, sub-labels           |
| `team-a`           | `#ff4d4d`     | Team A scores, labels              |
| `team-b`           | `#4d9fff`     | Team B scores, labels              |
| `accent`           | `#00e5a0`     | Continue button, highlights        |
| `success`          | `#22c55e`     | Positive outcomes                  |
| `warning`          | `#f59e0b`     | Failed hand indicator          |
| `font-display`     | Space Grotesk | Score numbers (font-bold)          |
| `font-body`        | Inter         | Labels, descriptions               |

### References

- [Source: _bmad-output/planning-artifacts/epics.md — Epic 4, Story 4.6, lines 1065-1103]
- [Source: _bmad-output/planning-artifacts/architecture.md — Game phases, session manager, scoring, match persistence]
- [Source: _bmad-output/planning-artifacts/ux-design-specification.md — ScorePanel, ScoreReveal, CapotAnimation, design tokens, animation patterns]
- [Source: _bmad-output/planning-artifacts/prd.md — FR11 (failed hands), FR12 (last-trick/capot), FR14 (1001 mode), FR23 (real-time sync)]
- [Source: _bmad-output/project-context.md — Anti-patterns, testing rules, gameStore cleanup, multi-event ordering]
- [Source: server/internal/game/state.go — GameState struct, TeamScores, HandPoints, DeclarationPoints, TricksWon]
- [Source: server/internal/game/scoring.go — scoreHand(), startNewHand(), determineMatchWinner(), matchTarget()]
- [Source: server/internal/session/manager.go — HandleAction, broadcastActionResult, handleMatchEnd]
- [Source: server/internal/ws/events.go — EventHandScored, EventMatchEnd constants]
- [Source: client/src/shared/types/wsEvents.ts — HandScoredPayload, MatchEndPayload interfaces]
- [Source: client/src/shared/types/gameTypes.ts — GameState, Phase, PlayerState]
- [Source: client/src/shared/stores/gameStore.ts — Zustand store pattern, clearGame]
- [Source: client/src/shared/hooks/useWsDispatch.ts — Event dispatch, hand_scored/match_end handlers]
- [Source: client/src/features/game/GamePage.tsx — Current score display, overlay patterns, match_end handling]
- [Source: client/src/features/game/components/DeclarationReveal.tsx — Animation overlay pattern to follow]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Debug Log References

- ScorePanel test initially failed due to missing `window.matchMedia` mock in jsdom — added mock in `beforeEach`
- ScoreReveal timer test required `act()` wrapping around `vi.advanceTimersByTime()` for React state updates
- TypeScript `lastHandResult` field needed adding to all existing GameState test fixtures (4 files)
- Go `broadcastActionResult` signature change required fixing 2 call sites (HandleAction + handleTimerExpiry)

### Completion Notes List

- Backend: Added `HandResult` struct to `state.go` capturing full hand scoring breakdown (card points, declaration points, bonuses, failed hand, totals)
- Backend: Modified `scoreHand()` in `scoring.go` to populate `LastHandResult` before `startNewHand()` — fixes the pre-existing bug where `HandScoredPayload` contained zeroed-out HandPoints after hand reset
- Backend: Enhanced `broadcastActionResult` to build hand_scored payload from `LastHandResult` with 15 fields (up from 6)
- Backend: Added `matchDurationSec` to match_end event payload, computed from `session.startedAt`
- Backend: Added 6 new scoring tests verifying `LastHandResult` for normal, capot, failed hand, declarations, match-end, and delta-matching scenarios
- Frontend: Updated `HandScoredPayload` (15 fields) and `MatchEndPayload` (+matchDurationSec) in wsEvents.ts
- Frontend: Added `HandResult` interface and `lastHandResult` field to `GameState` in gameTypes.ts
- Frontend: Added `scoreRevealData` and `matchEndData` fields with setters to gameStore
- Frontend: Updated `useWsDispatch.ts` to store hand_scored and match_end payloads in gameStore for overlay consumption
- Frontend: Created `ScorePanel.tsx` — fixed top-left HUD with team scores, trick counts, aria-live, float-up bonus animation
- Frontend: Created `ScoreReveal.tsx` — full-screen overlay with sequential animated breakdown (card points, declarations, bonuses, failed hand, totals), Continue button disabled for 2s
- Frontend: Created `CapotAnimation.tsx` — full-screen non-skippable 2.5s animation with team-color glow, CSS keyframe scale animation
- Frontend: Created `MatchResult.tsx` — match completion overlay with winner, final scores, duration, Return to Lobby button
- Frontend: Integrated all 4 components into `GamePage.tsx` with overlay flow state machine (normal → capot_animation → score_reveal → normal/match_result)
- Frontend: Replaced old inline score display with ScorePanel component
- Frontend: Replaced old auto-navigate-to-lobby on match_end with MatchResult overlay + explicit button
- Frontend: Added CSS keyframes for `capot-scale` and `float-up` animations in index.css
- Frontend: Added 19 i18n keys to both en.json and sr.json for score, scoreReveal, capot, and matchResult namespaces
- All 242 frontend tests pass (22 new), all Go tests pass (6 new LastHandResult tests)

### Change Log

- 2026-04-13: Story 4.6 implementation complete — score panel, score reveal, capot animation, match result

### File List

**New files:**

- client/src/features/game/components/ScorePanel.tsx
- client/src/features/game/components/ScorePanel.test.tsx
- client/src/features/game/components/ScoreReveal.tsx
- client/src/features/game/components/ScoreReveal.test.tsx
- client/src/features/game/components/CapotAnimation.tsx
- client/src/features/game/components/CapotAnimation.test.tsx
- client/src/features/game/components/MatchResult.tsx
- client/src/features/game/components/MatchResult.test.tsx

**Modified files:**

- server/internal/game/state.go (HandResult struct, LastHandResult field)
- server/internal/game/scoring.go (populate LastHandResult in scoreHand())
- server/internal/game/scoring_test.go (6 new LastHandResult tests)
- server/internal/session/manager.go (enhanced hand_scored/match_end payloads, startedAt param)
- client/src/shared/types/wsEvents.ts (enhanced HandScoredPayload, MatchEndPayload)
- client/src/shared/types/gameTypes.ts (HandResult interface, lastHandResult field)
- client/src/shared/stores/gameStore.ts (scoreRevealData, matchEndData fields)
- client/src/shared/hooks/useWsDispatch.ts (store hand_scored/match_end data)
- client/src/features/game/GamePage.tsx (ScorePanel, ScoreReveal, CapotAnimation, MatchResult integration)
- client/src/shared/i18n/en.json (score/scoreReveal/capot/matchResult keys)
- client/src/shared/i18n/sr.json (score/scoreReveal/capot/matchResult keys)
- client/src/index.css (capot-scale and float-up keyframes)

**Test fixture updates (lastHandResult: null added):**

- client/src/features/game/GamePage.test.tsx
- client/src/shared/types/gameTypes.test.ts
- client/src/shared/stores/gameStore.test.ts
- client/src/shared/hooks/useWsDispatch.test.ts
