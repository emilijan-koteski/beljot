# Story 4.5: Per-Move Timer & Auto-Play

Status: done

## Story

As a player,
I want a visible countdown timer on each move and automatic play when time runs out,
so that the game maintains pace and stalling is prevented.

## Acceptance Criteria

1. **Timer Display & Synchronization**
   Given the room is configured with a per-move timer (`timerStyle === "per-move"`),
   When it becomes a player's turn,
   Then the server sets `TurnExpiresAt` in GameState to an absolute UTC timestamp,
   And the `TimerRing` component renders on the active player's seat as a countdown overlay,
   And the timer display stays synchronized with server time within ±1 second.

2. **Timer Color — Normal State (>10 seconds)**
   Given the timer has more than 10 seconds remaining,
   When `TimerRing` renders,
   Then it displays in `text-secondary` color with a steady countdown number.

3. **Timer Color — Warning State (<=10 seconds)**
   Given the timer reaches <=10 seconds,
   When `TimerRing` updates,
   Then it transitions to amber (`warning` token, `#f59e0b`) with a pulse animation.

4. **Auto-Play on Timer Expiry**
   Given the timer reaches 0,
   When the server detects expiry in `session/timer.go`,
   Then `auto_play.go` selects the first legal card sorted by suit then rank,
   And the card is played automatically on behalf of the player via `ApplyAction`,
   And all clients receive `event:card_played` with an `autoPlayed: true` indicator,
   And a toast notification appears: "Auto-played: [card]" (info style, auto-dismiss 3s).

5. **Relaxed Timer Configuration**
   Given the room is configured with relaxed timer (`timerStyle === "relaxed"`),
   When it becomes a player's turn,
   Then no `TimerRing` is displayed and there is no auto-play trigger,
   And `TurnExpiresAt` is `null` in the GameState.

6. **Timer Cancellation on Card Play**
   Given a player plays a card (or takes any valid action) before the timer expires,
   When the action is processed by the session manager,
   Then the current timer is cancelled,
   And the next player's timer begins with a fresh `TurnExpiresAt`.

## Tasks / Subtasks

- [x] Task 1: Backend timer infrastructure (AC: #1, #4, #5, #6)
  - [x] 1.1 Create `server/internal/session/timer.go` — goroutine-based timer lifecycle
  - [x] 1.2 Create `server/internal/game/auto_play.go` — first legal card selection (suit then rank order)
  - [x] 1.3 Add `auto_play_test.go` — table-driven tests for auto-play card selection across game states
  - [x] 1.4 Integrate timer start/cancel into `session/manager.go` `HandleAction()` and `broadcastActionResult()`
  - [x] 1.5 Pass room timer config (`TimerStyle`, `TimerDurationSeconds`) into session on `StartGame()`
  - [x] 1.6 Set `TurnExpiresAt` in GameState on every turn transition (or nil for relaxed)
  - [x] 1.7 On timer expiry: call `auto_play.go`, then `ApplyAction`, then broadcast result with auto-play flag
- [x] Task 2: WebSocket event contract updates (AC: #4)
  - [x] 2.1 Add `autoPlayed boolean` field to `CardPlayedPayload` Go struct in `events.go` (TypeScript side already has it in `wsEvents.ts` — verify it matches)
  - [x] 2.2 Verify both contract files are consistent (same commit rule applies for any changes)
- [x] Task 3: Frontend TimerRing component (AC: #1, #2, #3, #5)
  - [x] 3.1 Create `TimerRing.tsx` + `TimerRing.test.tsx` in `features/game/components/`
  - [x] 3.2 Implement countdown calculation from `turnExpiresAt` using `Date.now()` interval
  - [x] 3.3 Implement normal state (>10s): `text-secondary` color, steady countdown
  - [x] 3.4 Implement warning state (<=10s): amber `warning` token, pulse animation
  - [x] 3.5 Implement expired state: brief red flash before auto-play resolves
  - [x] 3.6 Respect `prefers-reduced-motion`: pulse reduces to instant, no decorative motion
- [x] Task 4: Integrate TimerRing into PlayerSeat and GamePage (AC: #1, #5)
  - [x] 4.1 Add TimerRing overlay to `PlayerSeat.tsx` for the active player's seat
  - [x] 4.2 Conditionally render TimerRing only when `turnExpiresAt !== null` (per-move mode)
  - [x] 4.3 Update `GamePage.tsx` to pass timer-relevant props
- [x] Task 5: Auto-play toast notification (AC: #4)
  - [x] 5.1 Detect `autoPlayed: true` in `useWsDispatch.ts` when handling `EVENT_CARD_PLAYED`
  - [x] 5.2 Show Sonner info toast: "Auto-played: [card]" with auto-dismiss 3s
  - [x] 5.3 Add i18n keys for auto-play toast to both `en.json` and `sr.json`
- [x] Task 6: Backend tests (AC: #1, #4, #5, #6)
  - [x] 6.1 Timer lifecycle tests in `session/manager_test.go` (existing test file): start, cancel on action, expiry triggers auto-play
  - [x] 6.2 Auto-play integration: verify correct card selected and played via `ApplyAction`
  - [x] 6.3 Relaxed timer mode: verify no timer started, `TurnExpiresAt` is nil
  - [x] 6.4 Timer reset on action: verify old timer cancelled, new timer started for next player
- [x] Task 7: Frontend tests (AC: #1, #2, #3, #5)
  - [x] 7.1 TimerRing renders countdown from `turnExpiresAt`
  - [x] 7.2 TimerRing transitions to warning state at <=10s
  - [x] 7.3 TimerRing not rendered when `turnExpiresAt` is null (relaxed mode)
  - [x] 7.4 Auto-play toast shown when `autoPlayed === true` in card_played event
  - [x] 7.5 All animations respect `prefers-reduced-motion`
- [x] Task 8: i18n and final integration
  - [x] 8.1 Add i18n key: `game.timer.autoPlayed` (no `game.timer.warning` needed — warning state is CSS-only, no user-facing text)
  - [x] 8.2 Verify `make lint` passes
  - [x] 8.3 Verify `make test` passes (all existing + new tests)

### Review Findings

- [x] [Review][Patch] P1-HIGH: Timer fires during bidding but no auto-action — FIXED: added auto-pass in handleTimerExpiry
- [x] [Review][Patch] P2-HIGH: Data race on session.turnTimer — FIXED: all timer ops now under session.mu via startTimerLocked
- [x] [Review][Patch] P3-HIGH: Timer callback fires after session removal — FIXED: added closed flag, checked in callback
- [x] [Review][Patch] P4-HIGH: broadcastActionResult mutates newState.Phase outside lock — FIXED: moved transition into HandleAction under lock
- [x] [Review][Patch] P5-MED: Timer not restarted after dealing→bidding — FIXED: transition + setTurnExpiry now inside lock
- [x] [Review][Patch] P6-MED: Concurrent HandleAction/timer process wrong turn — FIXED: timerGeneration counter + expectedSeat check
- [x] [Review][Patch] P7-MED: Error path no timer restart — FIXED: startTimerLocked in all error paths
- [x] [Review][Patch] P8-MED: broadcastAutoPlayResult DRY violation — FIXED: merged into broadcastActionResult with autoPlayed param
- [x] [Review][Patch] P9-MED: JS falsy check on timerDuration — FIXED: explicit !== undefined && > 0
- [x] [Review][Patch] P10-LOW: TimerRing progress overflow — FIXED: clamped to [0, 1]
- [x] [Review][Patch] P11-LOW: Dead dealing→bidding code — FIXED: removed (transition in callers now)
- [x] [Review][Patch] P12-LOW: Missing concurrent test — FIXED: added TestPerMoveTimer_ConcurrentActionAndExpiry
- [x] [Review][Patch] P13-LOW: CardPlayedPayload struct missing — FIXED: added to events.go
- [x] [Review][Defer] D1: GetStateSnapshot returns mutable pointer — pre-existing design issue [manager.go:172-174]
- [x] [Review][Defer] D2: legalCards dereferences TrumpSuit without nil check — pre-existing [validation.go:18]
- [x] [Review][Defer] D3: Client clock skew — no server-client time sync — future enhancement

## Dev Notes

### Architecture Compliance

- **Server-authoritative**: Timer expiry detection and auto-play selection happen ONLY on the server. The client countdown is purely visual — it reads `turnExpiresAt` from GameState and renders a local countdown. The client CANNOT trigger auto-play.
- **Absolute timestamps**: Server sends `TurnExpiresAt` as an ISO 8601 UTC string (e.g., `"2026-04-12T15:04:05Z"`) — this is Go's `*time.Time` default JSON serialization. Client computes remaining time as `new Date(turnExpiresAt).getTime() - Date.now()`. NEVER send relative durations like "30 seconds" or raw Unix integers.
- **Pure function contract**: `auto_play.go` must be a pure function: `AutoPlay(state GameState) (Card, error)` — no side effects. The session manager calls it on timer expiry, then calls `ApplyAction(state, playCardAction)` to process the move. Auto-play does NOT bypass the rules engine.
- **Session manager is the orchestrator**: Timer goroutine lives in `session/timer.go`. On expiry, it calls back into the session manager, which calls `auto_play.go` → `ApplyAction` → broadcast. Side effects (broadcasting, timer management) live in the session manager, NOT in the rules engine.

### Backend Implementation Details

#### `server/internal/session/timer.go`

- Use `time.AfterFunc(duration, callback)` for the per-move timer goroutine
- Store `*time.Timer` in the Session struct for cancellation
- `StartTurnTimer(duration time.Duration, callback func())` — starts timer, stores reference
- `CancelTurnTimer()` — stops current timer if active (safe to call when nil)
- Timer callback must acquire the session mutex before modifying state (avoid race conditions)
- On expiry callback: call session manager method that runs auto-play → ApplyAction → broadcast
- **Thread safety**: Timer goroutine fires asynchronously. The callback MUST lock the session mutex before accessing game state. The `Session` struct already has `mu sync.RWMutex` — reuse it, do NOT add a second mutex.

#### `server/internal/game/auto_play.go`

- Pure function: `AutoPlay(state *GameState) (string, error)` — returns card ID (e.g., "KS")
- Selection logic: get legal cards for active player → sort by suit (S, H, D, C) then rank (7, 8, 9, T, J, Q, K, A) → return first
- Use existing `legalCards()` (unexported, same `game` package) from `validation.go` to determine playable cards — do NOT create an exported wrapper
- If no legal cards (shouldn't happen in valid state), return error
- This is the same as project-context rule: "Auto-play on timer expiry: first legal card sorted by suit then rank"

#### Session Manager Integration (`server/internal/session/manager.go`)

- Add `timerStyle string` and `timerDurationSecs int` fields to `Session` struct (from room config)
- Add `turnTimer *time.Timer` to `Session` struct (the `mu sync.RWMutex` already exists — do NOT add a duplicate)
- In `StartGame()`: receive room timer config, store in session
- After every action that changes the active player:
  1. Cancel existing timer (`CancelTurnTimer()`)
  2. Set `state.TurnExpiresAt` to `time.Now().Add(duration)` (or nil if relaxed)
  3. Start new timer (`StartTurnTimer(...)`) if per-move mode
- In `HandleAction()`: acquire `session.mu.Lock()` first, then cancel timer as the first step inside the lock — this prevents a race where the timer fires between cancel and lock acquisition
- Auto-play flow on expiry:
  1. Timer callback fires → acquire `session.mu.Lock()` (blocks if HandleAction holds it)
  2. Call `auto_play.AutoPlay(gameState)` → get card ID
  3. Build `PlayCardAction` with auto-play flag
  4. Call `ApplyAction(state, action)`
  5. Broadcast `event:card_played` with `autoPlayed: true`
  6. Start next player's timer

#### Room Config Propagation

- `Room` model already has `TimerStyle string` and `TimerDurationSeconds *int`
- When game starts (handler calls `manager.StartGame()`), pass these values
- `StartGame()` signature needs room timer config parameters
- Default: `TimerDurationSeconds` is 10-120 seconds range (validated in room handler)

### Frontend Implementation Details

#### `TimerRing.tsx` Component

- **Props**: `turnExpiresAt: string | null`, `isActive: boolean`
- **Rendering**: Circular SVG ring around the player's seat avatar
  - SVG `<circle>` with `stroke-dasharray` and `stroke-dashoffset` animated via CSS transition
  - Center text shows remaining seconds as integer
- **Countdown logic**: `useEffect` with `setInterval(1000)` — compute `Math.max(0, Math.ceil((new Date(turnExpiresAt).getTime() - Date.now()) / 1000))`
- **States**:
  - Normal (>10s): `stroke` = `text-secondary` CSS variable, steady animation
  - Warning (<=10s): `stroke` = `warning` token (`#f59e0b`), `motion-safe:animate-pulse`
  - Expired (0s): brief red flash, then component waits for next state update
- **Cleanup**: Clear interval on unmount and when `turnExpiresAt` changes
- **Reduced motion**: `prefers-reduced-motion` → no pulse, instant transitions
- **Conditional render**: Return `null` when `turnExpiresAt` is `null` (relaxed mode)

#### PlayerSeat Integration

- `PlayerSeat.tsx` currently renders active state with `border-accent shadow-[0_0_16px...]`
- Add `<TimerRing>` as an overlay child positioned absolute over the seat avatar
- Only render when `isActive && turnExpiresAt !== null`
- TimerRing is compact variant — overlays the avatar circle

#### Auto-Play Toast in `useWsDispatch.ts`

- In `EVENT_CARD_PLAYED` handler: check payload for `autoPlayed === true`
- If auto-played: call `toast.info(t('game.timer.autoPlayed', { card: payload.cardId }))` (use existing `cardId` field — no separate `autoPlayedCard` field needed)
- Toast auto-dismisses in 3 seconds (Sonner default or explicit `duration: 3000`)
- Import `toast` from `sonner` (already installed and configured)

#### GameState Already Has Timer Fields

- `gameTypes.ts` line 91: `turnExpiresAt: string | null` — already in the interface
- `gameStore.ts`: No changes needed to the store — timer state flows through `gameState.turnExpiresAt`
- Components read timer state directly from gameStore's `gameState` object

### WebSocket Event Contract Changes

**Both files updated in the same commit — no exceptions.**

In `wsEvents.ts` — `CardPlayedPayload` already has `autoPlayed: boolean`. Verify it matches:

```typescript
export interface CardPlayedPayload {
  playerSeat: number; // EXISTING — do NOT rename to "seat"
  cardId: string; // EXISTING — use this for auto-play toast display
  autoPlayed: boolean; // EXISTING — true if timer-triggered
}
```

In `events.go` — create `CardPlayedPayload` Go struct (does not exist yet):

```go
type CardPlayedPayload struct {
    PlayerSeat int    `json:"playerSeat"`
    CardID     string `json:"cardId"`
    AutoPlayed bool   `json:"autoPlayed"`
}
```

**CRITICAL**: The field name is `playerSeat` (not `seat`) — match the existing TypeScript interface and session manager broadcast code in `manager.go` which uses `"playerSeat"` as the JSON key.

No new event types needed — timer state is conveyed via `TurnExpiresAt` in the GameState struct (already broadcast via `event:game_state`). Auto-play is conveyed via the `autoPlayed` flag on `event:card_played`.

### i18n Keys

```json
// en.json
{
  "game": {
    "timer": {
      "autoPlayed": "Auto-played: {{card}}"
    }
  }
}

// sr.json
{
  "game": {
    "timer": {
      "autoPlayed": "Automatski odigrano: {{card}}"
    }
  }
}
```

### Testing Strategy

**Backend (Go):**

- `auto_play_test.go`: Table-driven tests using `testfixtures/` factory functions
  - Test with various hand compositions (trump only, mixed suits, single card)
  - Verify sort order: suit priority S > H > D > C, then rank 7 > 8 > 9 > T > J > Q > K > A
  - Test when must follow suit vs. free to play any
- `manager_test.go` additions (existing file — `session_test.go` does NOT exist):
  - Timer starts on turn transition when `timerStyle == "per-move"`
  - Timer does NOT start when `timerStyle == "relaxed"`
  - Timer cancels when player acts before expiry
  - Timer expiry triggers auto-play and broadcasts result
  - Concurrent timer expiry and player action (race condition test)
  - Use `time.AfterFunc` with short durations (10ms) in tests for fast execution

**Frontend (Vitest):**

- `TimerRing.test.tsx`: Use `vi.useFakeTimers()` for deterministic countdown testing
  - Renders countdown from turnExpiresAt
  - Shows warning state at <=10s (amber color, pulse class)
  - Returns null when turnExpiresAt is null
  - Respects prefers-reduced-motion (mock `window.matchMedia`)
- `useWsDispatch` integration: verify toast fires on autoPlayed events
- `PlayerSeat` integration: TimerRing rendered for active player with timer

### Project Structure Notes

**New files to create:**

```text
server/internal/session/timer.go          # Timer lifecycle management
server/internal/game/auto_play.go         # Auto-play card selection (pure function)
server/internal/game/auto_play_test.go    # Table-driven auto-play tests
client/src/features/game/components/TimerRing.tsx       # Countdown ring component
client/src/features/game/components/TimerRing.test.tsx   # TimerRing tests
```

**Files to modify:**

```text
server/internal/session/manager.go        # Timer integration, auto-play trigger
server/internal/session/manager_test.go   # Timer lifecycle tests (existing file)
server/internal/room/handler.go           # Pass timer config to StartGame() (caller update)
server/internal/ws/events.go              # CardPlayedPayload Go struct with autoPlayed field
client/src/shared/types/wsEvents.ts       # Verify autoPlayed field consistency (already exists)
client/src/features/game/components/PlayerSeat.tsx  # TimerRing overlay integration
client/src/shared/hooks/useWsDispatch.ts  # Auto-play toast on card_played event
client/src/shared/i18n/en.json            # Timer i18n keys
client/src/shared/i18n/sr.json            # Timer i18n keys
```

**Alignment with project structure**: All paths follow existing conventions — backend domain packages in `internal/`, frontend feature components in `features/game/components/`, shared types in `shared/types/`, hooks in `shared/hooks/`.

### Previous Story Intelligence (from 4-4)

**Patterns to follow:**

- Named exports only (`export function TimerRing(...)`, not `export default`)
- CSS-only animations via Tailwind keyframe extensions (no framer-motion, no react-spring)
- `motion-safe:animate-*` prefix for all animations; `motion-reduce:` fallback
- All user-facing strings via `useTranslation()` hook
- `data-testid` on all interactive/testable elements
- Compass seat mapping: `compassOffset = (seat - myPlayerSeat + 4) % 4`
- GamePage manages WS lifecycle; components receive props/store state
- Error display via Sonner toast system (already configured)
- `vi.useFakeTimers()` for animation/timing tests
- Mock `window.matchMedia("(prefers-reduced-motion: reduce)")` for reduced motion tests

**Anti-patterns from 4-4 to avoid:**

- Do NOT validate game logic client-side (timer expiry is server-only)
- Do NOT use `export default`
- Do NOT hardcode user-facing strings
- Do NOT use animation libraries (CSS only)
- Do NOT maintain parallel game state in component local state
- Do NOT use `Dialog` from shadcn for game overlays
- Tailwind variant ordering: `motion-safe:hover:` (prefix first, NOT `hover:motion-safe:`)

**Deferred issues from 4-4 still open:**

- W1: `playableCardIds` includes ALL hand cards (no legal-move filtering) — pre-existing from 4.3
- W2: `PlayerState.username` missing from server Go struct — pre-existing from 4.3
- W3: `TrumpSelectedPayload.trumpSuit` unsafe `as` cast — pre-existing from 4.2
- W4: `window.confirm()` for back-button interception — pre-existing from 4.3

### References

- [Source: _bmad-output/planning-artifacts/epics.md — Epic 4, Story 4.5]
- [Source: _bmad-output/planning-artifacts/architecture.md — Timer synchronization, Session manager, Auto-play]
- [Source: _bmad-output/planning-artifacts/ux-design-specification.md — TimerRing component, PlayerSeat states, Toast patterns]
- [Source: _bmad-output/project-context.md — Anti-patterns, Testing rules, Game rules]
- [Source: server/internal/game/state.go — GameState struct with TurnExpiresAt field]
- [Source: server/internal/room/model.go — Room model with TimerStyle, TimerDurationSeconds]
- [Source: server/internal/session/manager.go — Session/Manager structs, HandleAction, broadcastActionResult]
- [Source: client/src/shared/types/gameTypes.ts — Frontend GameState with turnExpiresAt]
- [Source: client/src/shared/types/wsEvents.ts — Event constants, CardPlayedPayload]
- [Source: client/src/features/game/components/PlayerSeat.tsx — Active player rendering]
- [Source: client/src/shared/components/ui/sonner.tsx — Toast system configuration]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Debug Log References

- Auto-play test initially failed due to import cycle (package game vs game_test) — resolved by using external test package
- Auto-play sort test had wrong expected value for TS vs QS — rank order 3 < 5, so TS comes first
- Timer expiry auto-play failed with "pending action must be resolved first" — fixed by adding declaration/belot auto-skip handling in handleTimerExpiry
- ESLint import sort required auto-fix for new i18n/sonner imports in useWsDispatch
- gofmt formatting fix needed for manager.go and handler.go (alignment)

### Completion Notes List

- Backend: Created `session/timer.go` (goroutine-based timer using `time.AfterFunc`), `game/auto_play.go` (pure function selecting first legal card sorted by suit then rank), integrated timer lifecycle into `session/manager.go` with proper mutex-based thread safety
- Timer handles all game states on expiry: auto-plays cards, auto-skips declarations, auto-skips belot announcements
- Session manager's `StartGame` now accepts `timerStyle` and `timerDurationSec` from room config; `GameStarter` interface and room handler updated accordingly
- Added `TimerDurationSec` field to `GameState` (Go + TypeScript) so frontend can compute ring progress without cross-referencing room data
- Frontend: Created `TimerRing.tsx` SVG ring component with normal (>10s, text-secondary), warning (<=10s, amber pulse), and expired (red) states
- Integrated TimerRing as overlay in `PlayerSeat.tsx` for the active player when `turnExpiresAt` is set
- Auto-play toast via Sonner info toast in `useWsDispatch.ts` when `autoPlayed === true` on card_played events
- i18n keys added to both en.json and sr.json: `game.timer.autoPlayed`
- All 220 frontend tests pass (9 new TimerRing tests), all Go tests pass (7 new auto_play tests + 4 timer lifecycle tests)
- All animations use `motion-safe:` prefix with `motion-reduce:` fallbacks

### Change Log

- 2026-04-12: Story 4.5 implementation complete — per-move timer and auto-play

### File List

**New files:**

- server/internal/session/timer.go
- server/internal/game/auto_play.go
- server/internal/game/auto_play_test.go
- client/src/features/game/components/TimerRing.tsx
- client/src/features/game/components/TimerRing.test.tsx

**Modified files:**

- server/internal/session/manager.go
- server/internal/session/manager_test.go
- server/internal/room/handler.go
- server/internal/game/state.go
- client/src/shared/types/gameTypes.ts
- client/src/shared/types/wsEvents.ts (verified — autoPlayed already existed)
- client/src/shared/hooks/useWsDispatch.ts
- client/src/features/game/components/PlayerSeat.tsx
- client/src/features/game/GamePage.tsx
- client/src/shared/i18n/en.json
- client/src/shared/i18n/sr.json
- client/src/features/game/GamePage.test.tsx (fixture update)
- client/src/shared/hooks/useWsDispatch.test.ts (fixture update)
- client/src/shared/stores/gameStore.test.ts (fixture update)
- client/src/shared/types/gameTypes.test.ts (fixture update)
