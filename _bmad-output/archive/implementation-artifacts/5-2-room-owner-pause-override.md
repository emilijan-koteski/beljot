# Story 5.2: Room Owner Pause Override

Status: done

## Story

As a room owner,
I want to override all active pauses and resume the game,
so that I can keep the game moving if a pause is taking too long.

## Acceptance Criteria

1. **Owner Unpause — Clears All Pauses**
   Given the game is paused with one or more active player pauses,
   When the room owner submits `action:owner_unpause`,
   Then ALL active pauses are cleared regardless of who initiated them,
   And the game resumes to the previous phase,
   And all clients receive `event:game_resumed` with `ownerOverride: true`.

2. **Non-Owner Rejected**
   Given a non-owner player submits `action:owner_unpause`,
   When the action is processed,
   Then an `error:not_room_owner` event is returned and the state is unchanged.

3. **Owner Sees "Resume All" Button**
   Given the game is paused,
   When the room owner views the pause overlay,
   Then they see an additional "Resume All" button (destructive ghost style) that other players do not see.

4. **Toast Notification on Owner Override**
   Given the room owner uses the override,
   When other players see the resume,
   Then a toast notification appears: "Room owner resumed the game".

## Tasks / Subtasks

- [x] Task 1: Backend — Add `OwnerSeat` to GameState (AC: #1, #2)
  - [x] 1.1 In `server/internal/game/state.go`, add `OwnerSeat int` field to GameState struct in the **Match metadata** section (after `Phase`):
    ```go
    OwnerSeat int `json:"ownerSeat"` // Seat index of the room owner (for pause override)
    ```
  - [x] 1.2 In `client/src/shared/types/gameTypes.ts`, add `ownerSeat: number` to the `GameState` interface (after `phase`).

- [x] Task 2: Backend — Wire owner info through StartGame (AC: #1, #2)
  - [x] 2.1 Update `GameStarter` interface in `server/internal/room/handler.go` to add `ownerID uint` parameter:
    ```go
    StartGame(roomID uint, variant string, matchMode string, players [4]PlayerSeatInfo, timerStyle string, timerDurationSec int, ownerID uint) error
    ```
  - [x] 2.2 Update both `StartGame` call sites in `room/handler.go`:
    - `StartGame` handler (line ~809): pass `updatedRoom.OwnerID`
    - `QuickPlay` auto-start (line ~705): pass `autoStartRoom.OwnerID`
  - [x] 2.3 Update `session/manager.go` `StartGame` method signature to accept `ownerID uint`. Map owner to seat:
    ```go
    ownerSeat := -1
    for i, uid := range playerIDs {
        if uid == ownerID {
            ownerSeat = i
            break
        }
    }
    gs.OwnerSeat = ownerSeat
    ```
  - [x] 2.4 Update any test mocks/stubs of the `GameStarter` interface to match the new signature.

- [x] Task 3: Backend — Implement `handleOwnerUnpause` in rules engine (AC: #1, #2)
  - [x] 3.1 In `server/internal/game/pause.go`, add a new function:
    ```go
    func handleOwnerUnpause(state *GameState, action Action) (*GameState, error) {
        if state.Phase != PhasePaused {
            return nil, apperr.ErrNotPaused
        }
        if action.PlayerSeat != state.OwnerSeat {
            return nil, apperr.ErrNotRoomOwner
        }
        newState := cloneGameState(state)
        // Clear ALL active pauses
        newState.PausedPlayers = [4]bool{}
        newState.Phase = newState.PreviousPhase
        newState.PreviousPhase = ""
        return newState, nil
    }
    ```
  - [x] 3.2 In `server/internal/game/rules_engine.go`, update `ApplyAction` to route `ActionOwnerUnpause` to `handleOwnerUnpause` instead of `handleUnpause`:
    ```go
    if action.Type == ActionUnpause {
        return handleUnpause(state, action)
    }
    if action.Type == ActionOwnerUnpause {
        return handleOwnerUnpause(state, action)
    }
    ```

- [x] Task 4: Backend — Add error WS constant for not_room_owner (AC: #2)
  - [x] 4.1 In `server/internal/ws/events.go`, add:
    ```go
    const ErrorNotRoomOwner = "error:not_room_owner"
    ```
  - [x] 4.2 In `client/src/shared/types/wsEvents.ts`, add:
    ```ts
    export const ERROR_NOT_ROOM_OWNER = "error:not_room_owner" as const;
    ```
  - [x] 4.3 Add `ERROR_NOT_ROOM_OWNER` to `GAME_ERROR_TYPES` set in `useWsDispatch.ts` so the error triggers a toast.
  - [x] 4.4 Add i18n key mapping in `GamePage.tsx` error handler:
    ```ts
    "error:not_room_owner": "game.errors.notRoomOwner",
    ```

- [x] Task 5: Backend tests — Owner unpause scenarios (AC: #1, #2)
  - [x] 5.1 Add test fixture in `server/internal/game/testfixtures/fixtures.go`:
    - `NewGamePausedWithOwner(pausedBySeat int, ownerSeat int) *GameState` — returns a paused game state with `OwnerSeat` set.
  - [x] 5.2 Add table-driven tests in `server/internal/game/pause_test.go`:
    - **Owner unpause clears all pauses** — owner submits `owner_unpause`, all `PausedPlayers` cleared, phase resumes to `PreviousPhase`
    - **Owner unpause clears stacked pauses** — 2+ players paused, owner clears all at once
    - **Non-owner rejected** — non-owner submits `owner_unpause`, returns `ErrNotRoomOwner`, state unchanged
    - **Owner unpause when not paused** — returns `ErrNotPaused`
    - **Owner can also use regular unpause** — owner who has an active pause can use `action:unpause` (only clears their own)
    - **Owner unpause with no active pause by owner** — owner didn't pause but can still override all
  - [x] 5.3 Update existing `cloneGameState` test (if any) to verify `OwnerSeat` is preserved through clone.
  - [x] 5.4 Update all test fixtures that create `GameState` structs to include `OwnerSeat` field (default 0 is acceptable for most existing tests since seat 0 is a valid player).

- [x] Task 6: Frontend — Update PauseOverlay with owner "Resume All" button (AC: #3)
  - [x] 6.1 Add new props to `PauseOverlay`:
    - `isRoomOwner: boolean`
    - `onOwnerResume: () => void`
  - [x] 6.2 Add "Resume All" button visible ONLY when `isRoomOwner` is true:
    - Style: destructive ghost — `border border-red-500/50 text-red-400 hover:bg-red-500/10` (destructive ghost per UX spec)
    - Position: below the existing resume/stack pause buttons
    - Text: `t("game.pause.resumeAll")`
    - `data-testid="pause-owner-resume-button"`
    - The button is always visible to the owner regardless of whether they personally have an active pause
  - [x] 6.3 The existing resume/stack pause buttons for the owner should still work as before (owner can use their own pause/unpause independently)

- [x] Task 7: Frontend — Wire owner unpause in GamePage (AC: #3, #4)
  - [x] 7.1 Add `handleOwnerUnpause` callback:
    ```tsx
    const handleOwnerUnpause = useCallback(() => {
      sendMessage(ACTION_OWNER_UNPAUSE, {});
    }, [sendMessage]);
    ```
  - [x] 7.2 Import `ACTION_OWNER_UNPAUSE` from wsEvents.
  - [x] 7.3 Compute `isRoomOwner`:
    ```tsx
    const isRoomOwner =
      myPlayerSeat !== null && gameState.ownerSeat === myPlayerSeat;
    ```
  - [x] 7.4 Pass `isRoomOwner` and `onOwnerResume={handleOwnerUnpause}` to PauseOverlay.

- [x] Task 8: Frontend tests — PauseOverlay owner button (AC: #3)
  - [x] 8.1 In `PauseOverlay.test.tsx`, add tests:
    - **Owner sees "Resume All" button** — render with `isRoomOwner=true`, assert button visible
    - **Non-owner does not see "Resume All" button** — render with `isRoomOwner=false`, assert button absent
    - **Owner "Resume All" fires onOwnerResume** — click the button, assert callback fired
    - **Owner still sees personal resume button when they have an active pause** — owner with active pause sees both Resume and Resume All

- [x] Task 9: i18n keys (AC: #3, #4)
  - [x] 9.1 In `client/src/shared/i18n/en.json`, add under `game.pause`:
    - `"resumeAll": "Resume All"`
  - [x] 9.2 Add under `game.errors`:
    - `"notRoomOwner": "Only the room owner can do this"`
  - [x] 9.3 In `client/src/shared/i18n/sr.json`, add under `game.pause`:
    - `"resumeAll": "Nastavi sve"`
  - [x] 9.4 Add under `game.errors`:
    - `"notRoomOwner": "Samo vlasnik sobe moze ovo da uradi"`
  - [x] 9.5 Note: `game.pause.ownerResumedToast` already exists in both language files (added in Story 5.1). Do NOT duplicate.

- [x] Task 10: Validation and quality gates (AC: all)
  - [x] 10.1 Run `make lint` — both Go and TypeScript must pass
  - [x] 10.2 Run `make test` — all existing + new tests must pass
  - [x] 10.3 Verify no regressions in existing pause tests (Story 5.1 tests must still pass)
  - [x] 10.4 Verify WS contract files are in sync (`wsEvents.ts` + `events.go`)

### Review Findings

- [x] [Review][Patch] OwnerSeat zero-value (0) collision with valid seat index — `OwnerSeat int` defaults to 0, which is a valid seat. If `ownerID` is not found in `playerIDs` during `StartGame`, seat 0 gets false owner privileges. Fix: initialize to -1 sentinel when no match found. [server/internal/session/manager.go:77-82]
- [x] [Review][Defer] Room ownership transfer during game not synced to active session — if room ownership changes via `LeaveRoom` during an active game, `OwnerSeat` in `GameState` is stale. Pre-existing design decision (OwnerSeat fixed at game start). [server/internal/session/manager.go]
- [x] [Review][Defer] No integration test for broadcast `OwnerOverride: true` — `broadcastActionResult` sets `OwnerOverride: true` for `ActionOwnerUnpause` (pre-existing from Story 5.1), but no test asserts the broadcast payload. Pre-existing, broadcast code unchanged. [server/internal/session/manager.go:462-471]
- [x] [Review][Defer] No WS-layer test for `error:not_room_owner` event delivery — rules engine returns `ErrNotRoomOwner` but `sendGameError` routes all errors as `error:invalid_action` (D53). Pre-existing system-wide issue. [server/internal/session/manager.go:528]

## Dev Notes

### What Already Exists — Do NOT Recreate

Critical: The following were scaffolded in Story 5.1. They already exist and MUST NOT be duplicated:

| Item                                                    | Location                                           | Status |
| ------------------------------------------------------- | -------------------------------------------------- | ------ |
| `ActionOwnerUnpause = "owner_unpause"`                  | `server/internal/game/types.go:105`                | Exists |
| `const ActionOwnerUnpause = "action:owner_unpause"`     | `server/internal/ws/events.go:27`                  | Exists |
| `ACTION_OWNER_UNPAUSE`                                  | `client/src/shared/types/wsEvents.ts:41`           | Exists |
| `"owner_unpause"` in `ActionType` union                 | `client/src/shared/types/gameTypes.ts:32`          | Exists |
| `GameResumedPayload.OwnerOverride bool`                 | `server/internal/ws/events.go:58`                  | Exists |
| `GameResumedPayload.ownerOverride`                      | `client/src/shared/types/wsEvents.ts:153`          | Exists |
| `ErrNotRoomOwner`                                       | `server/internal/apperr/errors.go:60`              | Exists |
| `game.pause.ownerResumedToast` i18n key                 | `en.json:232`, `sr.json`                           | Exists |
| Toast on `ownerOverride`                                | `client/src/shared/hooks/useWsDispatch.ts:182-185` | Exists |
| Session manager timer handling for `ActionOwnerUnpause` | `server/internal/session/manager.go:170`           | Exists |
| `broadcastActionResult` case for `ActionOwnerUnpause`   | `server/internal/session/manager.go:462-471`       | Exists |

### What Must Be Changed

1. **`pause.go`** — Add new `handleOwnerUnpause` function (separate from `handleUnpause`). `handleUnpause` stays unchanged for regular unpause.

2. **`rules_engine.go`** — Split routing: `ActionUnpause` → `handleUnpause`, `ActionOwnerUnpause` → `handleOwnerUnpause`. Currently both go to `handleUnpause` (line 16-17), which is incorrect for owner unpause because it requires the acting player to have an active pause and only clears their pause.

3. **`state.go`** — Add `OwnerSeat int` field to `GameState`. Place in Match metadata section after `Phase`.

4. **`session/manager.go`** — Update `StartGame` to accept `ownerID` and map to seat. The manager already handles `ActionOwnerUnpause` for timer restoration correctly (line 170).

5. **`room/handler.go`** — Update `GameStarter` interface and both call sites to pass `ownerID`.

6. **`PauseOverlay.tsx`** — Add `isRoomOwner` and `onOwnerResume` props. Add "Resume All" button.

7. **`GamePage.tsx`** — Wire `handleOwnerUnpause` callback, compute `isRoomOwner` from `gameState.ownerSeat`.

8. **`gameTypes.ts`** — Add `ownerSeat: number` to `GameState` interface.

### Architecture Patterns to Follow

- **Rules engine is a pure function**: `handleOwnerUnpause` must clone state, validate, mutate clone, return. No side effects.
- **Clone slices before mutation**: Use `cloneGameState(state)` — the existing clone function handles `[4]bool` arrays correctly (Go arrays are value types, not reference types, so struct copy is sufficient).
- **Session manager orchestration**: Timer management for owner unpause is already handled in `manager.go:170` — the existing `(action.Type == game.ActionUnpause || action.Type == game.ActionOwnerUnpause) && newState.Phase != game.PhasePaused` branch correctly restores the timer.
- **WebSocket contract sync**: Both `wsEvents.ts` and `events.go` must be updated in the same commit.
- **Centralized errors**: `ErrNotRoomOwner` already exists in `apperr/errors.go` — use it directly, don't create a new one.
- **Domain package file pattern**: Add `handleOwnerUnpause` to existing `pause.go`, tests to existing `pause_test.go`.
- **Test fixtures**: Use factory functions from `testfixtures/fixtures.go`. Add `NewGamePausedWithOwner` factory.

### Session Manager — No Timer Changes Needed

The session manager's `HandleAction` already handles `ActionOwnerUnpause` correctly for timer management at line 170:

```go
} else if (action.Type == game.ActionUnpause || action.Type == game.ActionOwnerUnpause) && newState.Phase != game.PhasePaused {
    // Game resumed — restore timer from preserved remaining time
```

This branch fires when the game leaves `PhasePaused`, which is exactly what `handleOwnerUnpause` does. No changes needed to timer logic.

The `broadcastActionResult` already handles `ActionOwnerUnpause` at line 462:

```go
case game.ActionUnpause, game.ActionOwnerUnpause:
    resumed := ws.GameResumedPayload{
        ResumedBy:     action.PlayerSeat,
        OwnerOverride: action.Type == game.ActionOwnerUnpause,
    }
```

This correctly sets `OwnerOverride: true` for owner unpause. No changes needed.

### Room Owner Detection — Design Decision

The room owner is tracked by `Room.OwnerID` (uint) in the database. To make this available to the rules engine:

- Add `OwnerSeat int` to `GameState` (mapped from `OwnerID` to seat index during `StartGame`)
- This is the cleanest approach since the rules engine operates on seat indices, not user IDs
- The client receives `ownerSeat` in every `event:game_state` broadcast, so it always knows who the owner is

### UX Design Requirements

Per UX spec (`ux-design-specification.md`):

- **"Resume All" button style**: Destructive ghost — border and text in a subdued red/warning tone, not a filled destructive button. It's a powerful action but not dangerous.
- **Visual tone**: Composed, calm — consistent with the existing PauseOverlay design (surface-elevated card, no red alerts)
- **Toast**: Already wired — `useWsDispatch.ts:182-185` shows `game.pause.ownerResumedToast` ("Room owner resumed the game") when `ownerOverride` is true. No additional work needed.
- **Button hierarchy**: "Resume All" is below the existing personal pause/unpause controls. Owner sees both their personal controls AND the override button.

### Frontend Owner Detection

The client determines `isRoomOwner` from `gameState.ownerSeat === myPlayerSeat`. This is derived from server state — no separate API call needed. The `roomLobbyStore.room.ownerId` is also available but using `gameState.ownerSeat` is more reliable since it's always in sync with the active game session.

### Previous Story Intelligence (Story 5.1)

Key learnings from Story 5.1 implementation:

- `handlePause` accepts `PhasePaused` as source phase for stacking — `handleOwnerUnpause` doesn't need this (it just clears everything)
- Unpause routing was moved before phase-specific dispatch in `ApplyAction()` — maintain this pattern
- Timer remaining floor of 3 seconds is enforced in session manager — applies to owner unpause resume too (already handled)
- PauseOverlay uses `data-testid` attributes for testing — follow same pattern for new button
- 4 existing test files needed GameState pause field updates — adding `OwnerSeat` may require similar updates in test files that construct raw `GameState` structs (check `testfixtures/fixtures.go`)
- `cloneGameState` is a shallow struct copy — `OwnerSeat int` is a value type, copies correctly

### Cross-Story Context

- **Story 5.3/5.4** (Disconnect Detection & Reconnection) will use `OwnerSeat` for disconnect-during-pause scenarios. The deferred item D54 ("Disconnect while paused leaves game frozen") is partially addressed by this story — the owner can now force-resume even if the pausing player disconnects.
- **Deferred D53**: `sendGameError` routes all errors as `error:invalid_action` — the `error:not_room_owner` type will also be affected. The error toast will work via the generic `error:invalid_action` path in practice. Adding the typed constant is still correct for future fixes.

### Project Structure Notes

**Modified files (expected):**

- `server/internal/game/state.go` — Add `OwnerSeat` field
- `server/internal/game/pause.go` — Add `handleOwnerUnpause` function
- `server/internal/game/rules_engine.go` — Split `ActionOwnerUnpause` routing
- `server/internal/game/pause_test.go` — Add owner unpause test cases
- `server/internal/game/testfixtures/fixtures.go` — Add `NewGamePausedWithOwner` factory
- `server/internal/session/manager.go` — Update `StartGame` signature to accept `ownerID`
- `server/internal/room/handler.go` — Update `GameStarter` interface and call sites
- `server/internal/ws/events.go` — Add `ErrorNotRoomOwner` constant
- `client/src/shared/types/gameTypes.ts` — Add `ownerSeat` to `GameState`
- `client/src/shared/types/wsEvents.ts` — Add `ERROR_NOT_ROOM_OWNER` constant
- `client/src/features/game/components/PauseOverlay.tsx` — Add owner button
- `client/src/features/game/components/PauseOverlay.test.tsx` — Add owner tests
- `client/src/features/game/GamePage.tsx` — Wire owner unpause
- `client/src/shared/hooks/useWsDispatch.ts` — Add error type to set
- `client/src/shared/i18n/en.json` — Add i18n keys
- `client/src/shared/i18n/sr.json` — Add i18n keys

**No new files expected** — all changes fit within existing files.

### References

- [Source: _bmad-output/planning-artifacts/epics.md — Story 5.2 acceptance criteria]
- [Source: _bmad-output/planning-artifacts/architecture.md — Game state machine phases, paused phase valid actions: unpause, owner_unpause]
- [Source: _bmad-output/planning-artifacts/architecture.md — Session manager orchestrator pattern]
- [Source: _bmad-output/planning-artifacts/architecture.md — WebSocket event naming conventions]
- [Source: _bmad-output/planning-artifacts/ux-design-specification.md — ReconnectOverlay pattern and calm visual tone]
- [Source: _bmad-output/planning-artifacts/ux-design-specification.md — Modal/overlay patterns: surface-elevated bg, no red alerts]
- [Source: _bmad-output/planning-artifacts/ux-design-specification.md — Darko (Room Owner) persona: needs control without complexity]
- [Source: _bmad-output/project-context.md — Rules engine pure function contract, clone slices before mutation]
- [Source: _bmad-output/project-context.md — WS contract files updated in same commit, no exceptions]
- [Source: _bmad-output/project-context.md — Timer sync via absolute timestamps]
- [Source: _bmad-output/archive/implementation-artifacts/5-1-player-pause-system.md — Previous story: scaffolded constants, timer integration, PauseOverlay design]
- [Source: server/internal/game/pause.go — Existing handlePause/handleUnpause implementations]
- [Source: server/internal/game/rules_engine.go:16-17 — Current routing sends both unpause types to handleUnpause]
- [Source: server/internal/session/manager.go:170 — Timer restoration already handles ActionOwnerUnpause]
- [Source: server/internal/session/manager.go:462-471 — Broadcast already sets OwnerOverride flag]
- [Source: server/internal/room/model.go — Room.OwnerID field]
- [Source: server/internal/apperr/errors.go:60 — ErrNotRoomOwner already defined]
- [Source: client/src/shared/hooks/useWsDispatch.ts:182-185 — Toast on ownerOverride already wired]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Debug Log References

- Updated existing `owner_unpause` test in `TestUnpause` table to test `ErrNotRoomOwner` (non-owner path) instead of previous behavior where owner_unpause routed through handleUnpause
- Fixed import sort order for `ACTION_OWNER_UNPAUSE` in GamePage.tsx (eslint simple-import-sort)
- Updated 11 `StartGame` calls in `manager_test.go` to include new `ownerID` parameter (default `10`, matching seat 0's UserID)

### Completion Notes List

- All 4 acceptance criteria satisfied
- 5 new table-driven test cases in `TestOwnerUnpause` covering: owner clears single pause, owner clears stacked pauses, non-owner rejected, owner without active pause can override, owner can use regular unpause for own pause only
- 2 additional error path tests in `TestUnpause` for owner_unpause: non-owner returns ErrNotRoomOwner, not-paused returns ErrNotPaused
- 4 new PauseOverlay tests: owner sees Resume All, non-owner doesn't see it, click fires callback, owner sees both Resume and Resume All buttons
- All 284 frontend tests pass, all Go tests pass, no regressions
- i18n keys added for both English and Serbian (resumeAll, notRoomOwner)
- WS contract files in sync (events.go + wsEvents.ts both have ErrorNotRoomOwner)

### Change Log

- 2026-04-13: Implemented room owner pause override (Story 5.2)

### File List

**Modified files:**

- `server/internal/game/state.go` — Added `OwnerSeat int` field to GameState
- `server/internal/game/pause.go` — Added `handleOwnerUnpause` function
- `server/internal/game/rules_engine.go` — Split routing: ActionUnpause → handleUnpause, ActionOwnerUnpause → handleOwnerUnpause
- `server/internal/game/pause_test.go` — Added TestOwnerUnpause (5 cases) + updated TestUnpause (2 new error cases)
- `server/internal/game/testfixtures/fixtures.go` — Added `NewGamePausedWithOwner` factory
- `server/internal/session/manager.go` — Updated `StartGame` signature to accept `ownerID`, map to seat
- `server/internal/session/manager_test.go` — Updated 11 `StartGame` calls with ownerID parameter
- `server/internal/room/handler.go` — Updated `GameStarter` interface and both call sites to pass `ownerID`
- `server/internal/ws/events.go` — Added `ErrorNotRoomOwner` constant
- `client/src/shared/types/gameTypes.ts` — Added `ownerSeat` to `GameState` interface
- `client/src/shared/types/wsEvents.ts` — Added `ERROR_NOT_ROOM_OWNER` constant
- `client/src/features/game/components/PauseOverlay.tsx` — Added `isRoomOwner`, `onOwnerResume` props + "Resume All" button
- `client/src/features/game/components/PauseOverlay.test.tsx` — Added 4 owner tests + updated all existing tests with new props
- `client/src/features/game/GamePage.tsx` — Added `handleOwnerUnpause` callback, `isRoomOwner` computation, wired to PauseOverlay
- `client/src/shared/hooks/useWsDispatch.ts` — Added `ERROR_NOT_ROOM_OWNER` to imports and `GAME_ERROR_TYPES` set
- `client/src/shared/i18n/en.json` — Added `game.pause.resumeAll`, `game.errors.notRoomOwner`
- `client/src/shared/i18n/sr.json` — Added `game.pause.resumeAll`, `game.errors.notRoomOwner`
