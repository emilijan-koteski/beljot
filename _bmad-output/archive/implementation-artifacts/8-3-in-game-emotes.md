# Story 8.3: In-Game Emotes

Status: done

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

## Story

As a player in an active match,
I want to express reactions using a preset emote palette,
so that I can communicate emotions and reactions beyond text chat without leaving the table.

## Acceptance Criteria

1. **Given** a player is in an active match (room status `in_progress`, GameState present), **when** they click the emote button rendered near the match-chat toggle, **then** an `EmotePicker` popover opens with a grid of the six preset emotes — `thumbs_up`, `clap`, `laugh`, `thinking`, `facepalm`, `heart`. The picker is reachable by keyboard (Tab to button → Enter/Space → arrow keys move focus inside the grid → Enter selects, Escape closes), and the button is hidden / not rendered when there is no active match (i.e. lobby, room-lobby, post-`match_end` overlays).

2. **Given** a player clicks an emote tile, **when** the click handler fires, **then** the client sends `action:emote` with payload `{ "emote": "<emote_id>" }` over the multiplexed WS connection. The picker closes immediately (no waiting for a server echo). `<emote_id>` is the canonical `snake_case` string from the preset set.

3. **Given** a valid `action:emote` arrives at the server from a user currently in an active match (resolved via `session.Manager.IsUserInGame`), **when** the emote string is in the preset whitelist AND the sender's last emote was sent more than `3000 ms` ago, **then** the server broadcasts `system:emote` to all four match participants (sender included for the own-echo) with payload `{ "playerSeat": <0..3>, "emote": "<emote_id>" }`. The broadcast goes through the existing `ws.Hub.BroadcastToUsers` API; multi-event ordering rules apply (separate JSON message, never batched with another event).

4. **Given** the server receives `action:emote` with any of the following violations — sender not in an active match, unknown emote string (not in the whitelist), empty payload, or sender's last emote within the last `3000 ms` — **when** the handler runs, **then** the message is silently dropped: no broadcast, no error event, log at `slog.Info` level for observability. (Mirrors the existing chat-handler silent-drop pattern; no `error:` event is added.)

5. **Given** a client receives `system:emote` for a participant in their current match, **when** `useWsDispatch` dispatches the event, **then** an `EmoteBubble` appears anchored to that player's `PlayerSeat`. The bubble: (a) has a pop-in animation (`motion-safe:zoom-in-95 motion-safe:duration-150`); (b) auto-dismisses after `2000 ms` (`1000 ms` when `prefers-reduced-motion: reduce` is set); (c) is purely visual / non-interactive (`pointer-events-none`, `aria-live="polite"`, no focus trap). A new emote arriving for the same seat **replaces** the active one — only the most recent is shown.

6. **Given** an emote bubble is displayed, **when** the auto-dismiss timer expires OR the player navigates away from the game page OR the match ends (`event:match_end` / `event:match_abandoned`), **then** the bubble unmounts and the per-seat slot in the store is cleared. No bubble survives across matches — the store wipes on `clearGame()` (existing pattern).

7. **Given** a player has just sent an emote, **when** they try to send another within `3000 ms` from the **client side**, **then** the picker tile is visually disabled (opacity reduced, `disabled` attribute set, `aria-disabled="true"`) and no `action:emote` is sent. This mirrors the server-side rate limit so the user sees instant feedback rather than an apparent silent drop. Server-side check is the authoritative gate; client throttle is UX only.

8. **Given** the WS contract is touched, **when** the change is committed, **then** [server/internal/ws/events.go](server/internal/ws/events.go) AND [client/src/shared/types/wsEvents.ts](client/src/shared/types/wsEvents.ts) both ship the new constants and payload types in the **same commit** (project rule). The two new constants are `ActionEmote` / `ACTION_EMOTE` (`"action:emote"`) and `SystemEmote` / `SYSTEM_EMOTE` (`"system:emote"`). The preset emote whitelist is defined once on each side as exported string-literal union (`type EmoteID = "thumbs_up" | "clap" | "laugh" | "thinking" | "facepalm" | "heart"`) — **no TypeScript `enum`**.

9. **Given** the new feature ships, **when** translations are checked by [client/src/shared/i18n/i18n.test.ts](client/src/shared/i18n/i18n.test.ts) (`flattenKeys` parity test), **then** every new English `game.emote.*` key has a Serbian counterpart in [sr.json](client/src/shared/i18n/sr.json). Required keys: `game.emote.button` (aria/tooltip), `game.emote.picker.title`, `game.emote.picker.close`, `game.emote.names.thumbsUp`, `.clap`, `.laugh`, `.thinking`, `.facepalm`, `.heart`. **No emote ID strings appear in user-facing copy** — IDs are protocol values, names come from i18n.

10. **Given** an opponent's emote arrives, **when** the bubble renders on the receiving client, **then** the position uses the **compass-relative** seat layout (the existing `compassOffset(seat, myPlayerSeat)` helper from [GamePage.tsx:52-54](client/src/features/game/GamePage.tsx#L52-L54)) — i.e. the bubble appears at the absolute screen position of _that player's_ seat from the _receiver's_ perspective. Sender always sees their own bubble at South.

11. **Given** the match is `paused` OR a reconnect overlay is up OR a match-end / match-abandoned overlay is up OR the receiving client has not yet derived `myPlayerSeat`, **when** a `system:emote` event arrives, **then** the bubble does not render (the picker button is also hidden in these states). The store can still record the latest emote, but the visual is suppressed; existing overlays own the screen.

12. **Given** a player disconnects and later reconnects mid-match, **when** the reconnection state snapshot is delivered (`event:game_state` from `Manager.GetStateSnapshot`), **then** active emote bubbles are NOT replayed — emotes are ephemeral, not part of GameState. The reconnecting client's first emote-render will be the next live `system:emote` they receive after reconnect. (Decision documented in Dev Notes; no GameState field for emotes.)

13. **Given** the sender's connection is lost between sending `action:emote` and the broadcast going out, **when** the sender reconnects, **then** the original emote is NOT re-broadcast — it is dropped. Server does not buffer in-flight emotes. (Consistent with chat handler — no replay queue.)

## Tasks / Subtasks

- [x] **Task 1: WS contract — both files in same commit (AC #8)**
  - [x] 1.1. [server/internal/ws/events.go](server/internal/ws/events.go) — add `const ActionEmote = "action:emote"`, `const SystemEmote = "system:emote"`. Add `EmoteID` typed string with the six values (`emoteThumbsUp = "thumbs_up"` etc.) plus a `var ValidEmoteIDs = map[EmoteID]struct{}{...}` for O(1) whitelist checks. Add `EmoteRequest` struct (`Emote string`) and `EmotePayload` struct (`PlayerSeat int`, `Emote EmoteID`).
  - [x] 1.2. [client/src/shared/types/wsEvents.ts](client/src/shared/types/wsEvents.ts) — add `ACTION_EMOTE`, `SYSTEM_EMOTE` `as const` exports. Export `type EmoteID = "thumbs_up" | "clap" | "laugh" | "thinking" | "facepalm" | "heart"`. Export `EMOTE_IDS: readonly EmoteID[]` (frozen array — single source of truth for the picker UI to iterate). Add `EmoteRequest` and `EmotePayload` interfaces.

- [x] **Task 2: Server-side emote handler (AC #3, #4)**
  - [x] 2.1. Create new package `server/internal/emote/` with `handler.go`. **DO NOT** add the handler to `internal/game/` — emotes are not a rules-engine concern (no GameState mutation, no Phase impact). Mirror `internal/chat/handler.go` shape: `Handler` struct depending on `Broadcaster` (subset of `*ws.Hub`), `GameMembership` (`IsUserInGame(uint) bool` + `MatchParticipants(uint) ([4]uint, bool)`), and `userRepo` for sender username **only if needed** — current AC #3 payload does not include username, so **omit the userRepo dependency** (the client resolves username from `gameState.players[playerSeat].username`).
  - [x] 2.2. `Handler.HandleAction(client *ws.Client, msg ws.WSMessage)` early-returns when `msg.Type != ws.ActionEmote`. Decode `EmoteRequest`. Verify `req.Emote` is in `ws.ValidEmoteIDs` — silently drop and `slog.Info("emote: unknown id", ...)` if not.
  - [x] 2.3. Resolve sender's room via `GameMembership.IsUserInGame(client.UserID)`. If not in game, drop. Resolve participants via a new `MatchParticipantsByUser(userID uint) ([4]uint, int /*seat*/, bool)` method on `session.Manager` (to be added in Task 3) — handler needs both the four participants AND the sender's seat index to populate `EmotePayload.PlayerSeat`. (`MatchParticipants` currently takes a `matchID`; the chat handler resolves matchID from the request payload, but emote has no matchID in the payload, so a userID→participants helper is cleaner than re-deriving the matchID via `userToRoom` outside the manager.)
  - [x] 2.4. Rate-limit: maintain `lastEmoteAt map[uint]time.Time` guarded by `sync.Mutex`. On request, lock, check `time.Since(last) < 3 * time.Second`, drop if too soon, else update timestamp and unlock. Use `time.Now()` (no injection needed for this story; tests assert via direct handler calls and a `Clock` interface can be added later if flakiness emerges).
  - [x] 2.5. Build `EmotePayload` and broadcast via `hub.BroadcastToUsers(participants[:], buildMessage(ws.SystemEmote, payload))`. Reuse the small `buildMessage` helper pattern from [chat/handler.go:251-263](server/internal/chat/handler.go#L251-L263) — copy the closure into the emote package (do NOT export from chat — small enough that a copy is cheaper than coupling the two packages).
  - [x] 2.6. Add `server/internal/emote/handler_test.go` covering: (a) unknown emote silently dropped — no `hubSpy` calls; (b) sender not in game silently dropped; (c) valid emote broadcasts to exactly the four participants with correct seat index; (d) two emotes from same user within 3 s — only first broadcasts; (e) two emotes from same user 3.5 s apart — both broadcast; (f) two **different** users emoting within 3 s of each other — both broadcast (rate limit is per-user, not global). Use `hubSpy` + `fakeGame` patterns from [chat/handler_test.go:43-99](server/internal/chat/handler_test.go#L43-L99).

- [x] **Task 3: Session manager helper for emote handler (Task 2 dep)**
  - [x] 3.1. [server/internal/session/manager.go](server/internal/session/manager.go) — add `MatchParticipantsByUser(userID uint) (participants [4]uint, seat int, ok bool)`. Take `m.mu.RLock()`, look up `roomID := m.userToRoom[userID]`, look up `session := m.sessions[roomID]`, scan `session.playerIDs` for `userID` to find seat, return `(session.playerIDs, seat, true)`. Return `([4]uint{}, -1, false)` when not in game.
  - [x] 3.2. Extend the emote-package `GameMembership` interface to require `MatchParticipantsByUser`. Wire `*session.Manager` as the implementer in `main.go`.
  - [x] 3.3. Add a focused unit test in [server/internal/session/manager_test.go](server/internal/session/manager_test.go): `TestMatchParticipantsByUser` covering present-user / absent-user / no-session paths.

- [x] **Task 4: Composite action-handler wiring (AC #2, #3)**
  - [x] 4.1. [server/cmd/api/main.go](server/cmd/api/main.go) — extend the existing composite at [main.go:121-127](server/cmd/api/main.go#L121-L127). Add an emote handler instance and route `ws.ActionEmote` to it before falling through to `sessionManager.HandleAction`. Pattern after the existing `if msg.Type == ws.ActionChatMessage { ... return }` block.
  - [x] 4.2. Construct: `emoteHandler := emote.NewHandler(hub, sessionManager)`. Add the second `if` arm: `if msg.Type == ws.ActionEmote { emoteHandler.HandleAction(client, msg); return }`.
  - [x] 4.3. **Critical**: do **NOT** route `action:emote` into `sessionManager.HandleAction` — `parseAction` would treat `emote` as a rules-engine action type and `ApplyAction` would reject it with `ErrInvalidAction`, polluting the WS error stream. The early-return is load-bearing.

- [x] **Task 5: GameStore — transient emote slot (AC #5, #6, #11)**
  - [x] 5.1. [client/src/shared/stores/gameStore.ts](client/src/shared/stores/gameStore.ts) — add `activeEmotes: Record<number, { emote: EmoteID; receivedAt: number } | null>` keyed by seat index (0..3). Initial value: `{ 0: null, 1: null, 2: null, 3: null }`. Include in `initialState` so `clearGame()` and `reset()` zero it.
  - [x] 5.2. Add setter `setActiveEmote: (seat: number, emote: EmoteID | null) => void` that produces a new object (no mutation) and stamps `receivedAt: Date.now()` when emote is non-null. Use `set((state) => ({ activeEmotes: { ...state.activeEmotes, [seat]: emote === null ? null : { emote, receivedAt: Date.now() } } }))`.
  - [x] 5.3. Bump the gameStore unit test [client/src/shared/stores/gameStore.test.ts](client/src/shared/stores/gameStore.test.ts) with: setter writes per-seat, `clearGame()` zeroes all four slots, replacing the same seat overwrites without leaking the previous value.

- [x] **Task 6: WS dispatch — `system:emote` branch (AC #5)**
  - [x] 6.1. [client/src/shared/hooks/useWsDispatch.ts](client/src/shared/hooks/useWsDispatch.ts) — import `EMOTE_IDS`, `SYSTEM_EMOTE`, `EmotePayload`. Add a branch in `dispatchSystemEvent` (mirror the `SYSTEM_CHAT_MESSAGE` block).
  - [x] 6.2. Validate the payload defensively: `typeof payload?.playerSeat === "number"` AND `payload.playerSeat >= 0 && < 4` AND `EMOTE_IDS.includes(payload.emote)`. Drop malformed payloads with `console.warn` (mirrors `SYSTEM_CHAT_MESSAGE` malformed-drop pattern at [useWsDispatch.ts:357-366](client/src/shared/hooks/useWsDispatch.ts#L357-L366)).
  - [x] 6.3. Defence in depth — only commit the emote when the user is in an active match: `if (useGameStore.getState().gameState === null) return;` (Story 8.1 dispatcher hardening pattern).
  - [x] 6.4. Call `useGameStore.getState().setActiveEmote(payload.playerSeat, payload.emote)`.
  - [x] 6.5. Update [client/src/shared/hooks/useWsDispatch.test.ts](client/src/shared/hooks/useWsDispatch.test.ts) with cases: valid emote sets store slot; malformed (bad seat / bad emote / non-string) ignored; arrival when `gameState === null` ignored.

- [x] **Task 7: `EmotePickerButton` component (AC #1, #2, #7)**
  - [x] 7.1. Create `client/src/features/game/components/EmotePickerButton.tsx`. Props: `onSend: (emote: EmoteID) => void`, `disabled?: boolean` (parent supplies based on phase). Internal state: `isOpen: boolean`, `lastSentAt: number` (initial `0`).
  - [x] 7.2. Render a circular icon button (Lucide `Smile` icon — already available via lucide-react used in `MatchChatSidebar.tsx`). Position handled by parent (GamePage). `data-testid="emote-toggle"`. `aria-label={t("game.emote.button")}`. `aria-pressed={isOpen}`.
  - [x] 7.3. On click toggle `isOpen`. While open, render an absolute-positioned grid of the six emotes (3 columns × 2 rows). Each tile is a `<button type="button" data-testid={`emote-tile-${id}`}>` with `aria-label={t(`game.emote.names.${camelCaseId}`)}` and the visual emoji glyph as content (`👍`, `👏`, `😂`, `🤔`, `🤦`, `❤️`). The glyph→ID mapping is hardcoded in the component (six entries — no need for a separate config file).
  - [x] 7.4. Click-outside dismissal: `useEffect` with `mousedown`/`touchstart` listener referencing a ref on the popover root. Dismiss on Escape via the same effect.
  - [x] 7.5. Tile click handler: compute `cooldownRemaining = 3000 - (Date.now() - lastSentAt)`. If `cooldownRemaining > 0`, no-op (defence in depth — tile is already disabled when this is true). Else: `setLastSentAt(Date.now())`, `onSend(id)`, `setIsOpen(false)`.
  - [x] 7.6. Tile `disabled` flag: `cooldownRemaining > 0 || disabled`. While `cooldownRemaining > 0` re-render once per second so the disabled state lifts at the right moment — use `useEffect` with `setTimeout(cooldownRemaining)` to force a state change at the cooldown-end moment (a single timer, not per-tile).
  - [x] 7.7. Co-located test `EmotePickerButton.test.tsx`: opens picker on click; calls `onSend` with the right ID; second click within 3 s does NOT call `onSend`; second click after 3 s does; closes on Escape and on outside click; tiles render in deterministic order matching `EMOTE_IDS`.

- [x] **Task 8: `EmoteBubble` component (AC #5, #6, #10)**
  - [x] 8.1. Create `client/src/features/game/components/EmoteBubble.tsx`. Props: `emote: EmoteID`, `compassPosition: 0 | 1 | 2 | 3`, `onDismiss: () => void`, `receivedAt: number` (used as React `key` from the parent so a new emote remounts cleanly).
  - [x] 8.2. Position via `SEAT_POSITIONS[compassPosition]` — extract the same `Record<number, string>` shape used by [GamePage.tsx:60-65](client/src/features/game/GamePage.tsx#L60-L65) into a small shared util OR re-declare locally (declare locally — duplication is cheaper than a new shared file for four lines of CSS).
  - [x] 8.3. Render the emote glyph in a small bubble with `pointer-events-none`, `aria-live="polite"`, `data-testid={`emote-bubble-${compassPosition}`}`, animation classes `motion-safe:animate-in motion-safe:zoom-in-95 motion-safe:duration-150`.
  - [x] 8.4. Auto-dismiss: `useEffect` schedules `setTimeout(onDismiss, prefersReducedMotion ? 1000 : 2000)`. Use `window.matchMedia('(prefers-reduced-motion: reduce)').matches`. Cleanup the timeout on unmount.
  - [x] 8.5. Co-located test `EmoteBubble.test.tsx`: renders glyph for each emote ID; calls `onDismiss` after 2000 ms (`vi.useFakeTimers`); calls `onDismiss` after 1000 ms with reduced-motion media query mocked; cleans up timer on unmount before timeout.

- [x] **Task 9: GamePage integration (AC #1, #5, #10, #11)**
  - [x] 9.1. [client/src/features/game/GamePage.tsx](client/src/features/game/GamePage.tsx) — import `EmotePickerButton`, `EmoteBubble`, `ACTION_EMOTE`, `EmoteID`, `useGameStore` selectors for `activeEmotes` and `setActiveEmote`.
  - [x] 9.2. Render `EmotePickerButton` inside an absolute-positioned wrapper near the chat toggle: `top-32 right-2 z-30` (one slot below `MatchChatSidebar`'s `top-16 right-2` button). Hide when `gameState.phase === "match_end"` OR `matchAbandonedData !== null` OR `gameState.phase === "disconnected"` OR `myPlayerSeat === null` OR `gameState.phase === "paused"` (mirror the surrender control gate but extend it to include `dealing` — emotes are allowed during dealing animation; allow phases: `dealing`, `bidding`, `playing`).
  - [x] 9.3. Wire `onSend`: a new `handleSendEmote = useCallback((emote: EmoteID) => { sendMessage(ACTION_EMOTE, { emote }); }, [sendMessage])`.
  - [x] 9.4. For each of the four `gameState.players`, when `activeEmotes[player.seat] !== null`, render an `EmoteBubble` with: `key={`${player.seat}-${activeEmotes[player.seat]!.receivedAt}`}` (forces clean remount when same seat fires a second emote), `compassPosition={compassOffset(player.seat, myPlayerSeat)}`, `emote={activeEmotes[player.seat]!.emote}`, `receivedAt={...}`, `onDismiss={() => setActiveEmote(player.seat, null)}`. Suppress all bubble rendering when `matchEndData !== null || matchAbandonedData !== null || gameState.phase === "paused"`.
  - [x] 9.5. Update [client/src/features/game/GamePage.test.tsx](client/src/features/game/GamePage.test.tsx) with: (a) emote toggle renders during `playing`; (b) emote toggle hidden during `match_end`; (c) clicking an emote tile fires `action:emote` over the WS mock; (d) `system:emote` arriving for opponent renders a bubble at the correct compass position; (e) bubble suppressed under MatchResult overlay; (f) bubble for self renders at South.

- [x] **Task 10: i18n strings (AC #9)**
  - [x] 10.1. [client/src/shared/i18n/en.json](client/src/shared/i18n/en.json) — under the existing `game` block, add the `emote` sub-block:
    ```jsonc
    "emote": {
      "button": "Send emote",
      "picker": { "title": "Pick an emote", "close": "Close emote picker" },
      "names": {
        "thumbsUp": "Thumbs up",
        "clap": "Clap",
        "laugh": "Laugh",
        "thinking": "Thinking",
        "facepalm": "Facepalm",
        "heart": "Heart"
      }
    }
    ```
  - [x] 10.2. [client/src/shared/i18n/sr.json](client/src/shared/i18n/sr.json) — mirror with Serbian Latin translations (e.g. `"thumbsUp": "Palac gore"`, `"clap": "Aplauz"`, `"laugh": "Smeh"`, `"thinking": "Razmišljanje"`, `"facepalm": "Glavom u ruke"`, `"heart": "Srce"`). Confirm the i18n parity test [client/src/shared/i18n/i18n.test.ts](client/src/shared/i18n/i18n.test.ts) passes.

- [x] **Task 11: Quality gates**
  - [x] 11.1. `make test` — both stacks green.
  - [x] 11.2. `make lint` — Go (`gofmt`, `golangci-lint`) + frontend (ESLint + Prettier) clean.
  - [x] 11.3. **Run `npx prettier --write .` from `client/` BEFORE committing client changes** — CI has failed repeatedly on this rule across 7.x and 8.x stories.
  - [x] 11.4. **Run `gofmt -w` against any new Go file** before staging.
  - [x] 11.5. Verify branch follows project convention (`feat/E8-S3-in-game-emotes`).
  - [x] 11.6. Commit message format: `feat(game): add in-game preset emotes (Story 8.3)`.

## Dev Notes

### Big Picture — Emote Is a Social Broadcast, NOT a Rules Engine Action

This is the architecturally important framing the dev agent must internalise up-front: **emotes do not mutate `GameState`**. They are ephemeral social broadcasts on the same multiplexed WebSocket as chat. The closest precedent is **`internal/chat/handler.go`**, not `internal/game/surrender.go`.

| Concern                                      | Pause / Surrender (Story 5.1, 8.2)              | Chat (Story 6.2)                     | **Emote (this story)**                          |
| -------------------------------------------- | ----------------------------------------------- | ------------------------------------ | ----------------------------------------------- |
| Mutates `GameState`?                         | Yes (Phase, Used flags, ProposerSeat)           | No                                   | **No**                                          |
| Goes through `game.ApplyAction`?             | Yes                                             | No                                   | **No**                                          |
| Lives in `internal/game/`?                   | Yes                                             | No (`internal/chat/`)                | **No (`internal/emote/`)**                      |
| Persisted to DB?                             | Yes (matches.surrendered_by)                    | No                                   | **No**                                          |
| Server-side rate limit?                      | n/a (one-shot per-seat)                         | n/a (length cap only)                | **Yes — per-user 3 s window**                   |
| Reuses `IsUserInGame` / `MatchParticipants`? | n/a                                             | Yes                                  | **Yes**                                         |
| Phase gating in handler?                     | Yes (PhasePlaying / PhaseBidding / PhasePaused) | Phase-agnostic (uses `IsUserInGame`) | **Phase-agnostic — `IsUserInGame` is the gate** |

The dev agent's primary anti-pattern risk on this story is **over-fitting the surrender precedent**: adding a `GameState.LastEmoteAt`, an `ActionEmote` rules-engine action type, an apperr error, a migration. **None of those are needed.** Read the chat handler closely; do not read the surrender handler for inspiration on this feature.

### What Already Exists — Do NOT Recreate

| Item                                                 | Location                                                                                                                        | Notes                                                                                                                                                                                                                                              |
| ---------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `chat.Handler` action handler                        | [server/internal/chat/handler.go](server/internal/chat/handler.go)                                                              | The exact shape and dependency wiring to mirror — `Broadcaster` interface (subset of `*ws.Hub`), `GameMembership` interface, silent-drop on validation failures, `buildMessage` helper. **Read this whole file before writing the emote handler**. |
| `chat.Handler` test scaffolding                      | [server/internal/chat/handler_test.go](server/internal/chat/handler_test.go)                                                    | `hubSpy`, `fakeGame`, `fakeRoom`, `userRepoStub` — copy the spy/fake patterns; do not invent new ones.                                                                                                                                             |
| Composite action-handler wiring                      | [server/cmd/api/main.go:121-127](server/cmd/api/main.go#L121-L127)                                                              | The single-closure `if msg.Type == ws.ActionChatMessage { ... }; sessionManager.HandleAction(...)` pattern. Extend with a second `if` arm — do NOT introduce a router abstraction or move dispatch into `ws.Router`.                               |
| `ws.Hub.BroadcastToUsers`                            | [server/internal/ws/hub.go:174-182](server/internal/ws/hub.go#L174-L182)                                                        | All emote broadcasts go through this. Disconnected user IDs are silently skipped — exactly the desired behaviour during a transient disconnect.                                                                                                    |
| `Manager.IsUserInGame` / `Manager.MatchParticipants` | [server/internal/session/manager.go:316-335](server/internal/session/manager.go#L316-L335)                                      | Existing helpers. The new `MatchParticipantsByUser` is a sibling helper using the same lock pattern.                                                                                                                                               |
| `compassOffset(seat, myPlayerSeat)`                  | [client/src/features/game/GamePage.tsx:52-54](client/src/features/game/GamePage.tsx#L52-L54)                                    | Existing helper. Use as-is for `EmoteBubble` positioning — do not duplicate the math.                                                                                                                                                              |
| `SEAT_POSITIONS` Tailwind class map                  | [client/src/features/game/GamePage.tsx:60-65](client/src/features/game/GamePage.tsx#L60-L65)                                    | The four absolute-positioning class strings keyed by compass index. The `EmoteBubble` reuses this exact mapping.                                                                                                                                   |
| `useFocusTrap`                                       | `client/src/shared/hooks/useFocusTrap.ts`                                                                                       | Used by `SurrenderPrompt` / `BelotPrompt`. **Do NOT** apply to `EmotePickerButton`'s popover — it is a non-modal popover, not a dialog; trapping focus would block the picker close-via-outside-click flow.                                        |
| `useGameStore.clearGame()`                           | [client/src/shared/stores/gameStore.ts:105](client/src/shared/stores/gameStore.ts#L105)                                         | Wipes all transient slots back to `initialState`. Adding `activeEmotes` to `initialState` is the entire mechanism that satisfies AC #6 (no per-game cleanup logic needed beyond the existing teardown).                                            |
| `useWsDispatch` defensive payload validation         | [useWsDispatch.ts:357-366](client/src/shared/hooks/useWsDispatch.ts#L357-L366)                                                  | The `SYSTEM_CHAT_MESSAGE` malformed-payload guard. Mirror this exact shape for `SYSTEM_EMOTE`.                                                                                                                                                     |
| Lucide `Smile` icon (and friends) via `lucide-react` | npm package already on the dependency tree (used by `MatchChatSidebar`, others)                                                 | Import `import { Smile } from "lucide-react"`. **Do NOT** install any new icon library.                                                                                                                                                            |
| `motion-safe:` Tailwind v4 modifier                  | Used throughout the codebase ([SurrenderPrompt.tsx:25](client/src/features/game/components/SurrenderPrompt.tsx#L25) and others) | Use the same `motion-safe:animate-in motion-safe:zoom-in-95 motion-safe:duration-150` pattern; reduced-motion will skip the animation automatically.                                                                                               |
| i18n parity test                                     | [client/src/shared/i18n/i18n.test.ts](client/src/shared/i18n/i18n.test.ts)                                                      | CI-enforced. Every English `game.emote.*` key must have a Serbian counterpart.                                                                                                                                                                     |

### What Must Be Created

1. **One new Go package**: `server/internal/emote/` with `handler.go` + `handler_test.go`.
2. **Two new React components**: `EmotePickerButton.tsx` + `EmoteBubble.tsx`, each with co-located `.test.tsx`.
3. **WS contract additions**: `ActionEmote`, `SystemEmote`, `EmoteID` enum-like union, `EmoteRequest`, `EmotePayload` — added to BOTH [events.go](server/internal/ws/events.go) AND [wsEvents.ts](client/src/shared/types/wsEvents.ts) in the same commit.

**No new SQL migration.** **No new apperr.** **No new `error:` event.** **No new GameState field.** **No new rules-engine action type or handler.** If the dev agent finds itself adding any of those, **stop and re-read this section** — the design is deliberately scoped narrower than 8.2.

### What Must Be Modified

1. [server/internal/ws/events.go](server/internal/ws/events.go) — add 2 constants + 1 typed union + 1 whitelist map + 2 payload structs.
2. [server/internal/session/manager.go](server/internal/session/manager.go) — add `MatchParticipantsByUser` helper.
3. [server/internal/session/manager_test.go](server/internal/session/manager_test.go) — add `TestMatchParticipantsByUser` (3 cases).
4. [server/cmd/api/main.go](server/cmd/api/main.go) — wire the emote handler into the composite action handler.
5. [client/src/shared/types/wsEvents.ts](client/src/shared/types/wsEvents.ts) — add 2 constants + `EmoteID` union + `EMOTE_IDS` array + 2 interfaces.
6. [client/src/shared/stores/gameStore.ts](client/src/shared/stores/gameStore.ts) — add `activeEmotes` slot + `setActiveEmote` setter + extend `initialState`.
7. [client/src/shared/stores/gameStore.test.ts](client/src/shared/stores/gameStore.test.ts) — extend with emote-slot cases.
8. [client/src/shared/hooks/useWsDispatch.ts](client/src/shared/hooks/useWsDispatch.ts) — add `SYSTEM_EMOTE` branch in `dispatchSystemEvent`.
9. [client/src/shared/hooks/useWsDispatch.test.ts](client/src/shared/hooks/useWsDispatch.test.ts) — add 3 emote cases (valid / malformed / no-active-game).
10. [client/src/features/game/GamePage.tsx](client/src/features/game/GamePage.tsx) — render picker + bubbles, wire send handler.
11. [client/src/features/game/GamePage.test.tsx](client/src/features/game/GamePage.test.tsx) — add 6 integration cases per Task 9.5.
12. [client/src/shared/i18n/en.json](client/src/shared/i18n/en.json) + [sr.json](client/src/shared/i18n/sr.json) — `game.emote.*` block.

**No changes expected:**

- `internal/game/*` — emote is not a rules-engine concern.
- `internal/match/*` — no persistence.
- `internal/apperr/errors.go` — no new errors (silent drops).
- `server/internal/session/reconnect.go` — emotes not in GameState, not part of reconnection state.
- `server/migrations/*` — no schema change.
- Existing tests for surrender, pause, reconnect, match-end, abandonment — unchanged.

### Architecture Patterns to Follow

- **Domain placement** — emote is a **social** concern, not a game concern. New package `internal/emote/`. Reusing `internal/chat/` would be acceptable (both are social broadcasts gated by match membership) but creates a multi-feature package and a confusing name. New package is the right call. (See [architecture.md L877](_bmad-output/planning-artifacts/architecture.md): chat lives in `internal/chat/`; emote is its sibling, not a sub-feature.)
- **WS event prefix discipline** — `action:emote` is client→server; `system:emote` is server→client. **Not** `event:emote` — `event:` is reserved for game-state changes (project-context [L80](server/../_bmad-output/project-context.md#L80) and architecture L335). Emotes are platform/social events, identical category to `system:chat_message`.
- **Same-commit WS contract sync** — server/client constant pair MUST land together (project-context L80, L286).
- **Pure-function rules engine, untouched** — no `ActionEmote` constant in `internal/game/types.go`, no `case ActionEmote` in `ApplyAction`, no `handleEmote` in any rules-engine file. The emote handler is wired alongside `sessionManager.HandleAction` in main.go's action-router closure.
- **Multi-event WS broadcasts as separate ordered messages** — `system:emote` is a single message; no follow-up `event:game_state` is needed (no state changed). Do NOT broadcast `event:game_state` after an emote — it would re-render the table for no reason.
- **Server is the authority** — client throttle is UX only; server's per-user 3 s window is the authoritative rate limit.
- **Silent drops over `error:` events** — chat handler precedent. Failed emote sends do NOT generate user-visible toasts. Logging at `slog.Info` is sufficient observability.
- **Defensive payload validation in `useWsDispatch`** — explicit `typeof` checks before commit, mirroring the chat-message guard. Drops with `console.warn` rather than crashing the dispatcher.
- **`data-testid` over text/Tailwind queries** — every component exposes stable `data-testid`s.
- **No TypeScript `enum`** — `EmoteID` is a string-literal union (project-context L63). `EMOTE_IDS` is a `readonly EmoteID[]`.
- **Explicit equality on numeric/boolean wire fields** (project-context L64) — `payload.playerSeat >= 0` not truthiness checks.
- **Frontend feature folder discipline** — components live under `features/game/components/` next to existing siblings (PlayerSeat, BelotPrompt, etc.). No new sub-folder.

### Previous Story Intelligence (Story 8.2 — done, 2026-04-27)

Carried-forward learnings that shape this story:

- **Prettier-before-commit is non-negotiable** (memory rule + 8.x review pattern). Task 11.3 enforces it. CI has failed repeatedly across Stories 7.1, 7.2, 8.1, 8.2 on this single rule.
- **`gofmt` is enforced** by `make lint`; commit `b16920f` ("apply gofmt across the codebase") shows the project tolerates zero drift. Task 11.4 enforces it.
- **Defence-in-depth on the dispatcher** (Story 8.1 + 8.2 review patches) — only commit a payload to a store when the user is in an active match (`gameState !== null`). Apply to `SYSTEM_EMOTE` branch (Task 6.3).
- **`disabled` (real attribute) over `pointer-events-none`** for keyboard accessibility (Story 8.1 review). Apply to disabled emote tiles (Task 7.6).
- **`data-testid` discipline** is fully baked (Stories 7.1, 7.2, 8.1, 8.2). Every new component must expose stable testids. No tests should query by Tailwind class or visible string.
- **`useFocusTrap` is for modal dialogs only** — `SurrenderPrompt` uses it (modal); `EmotePickerButton`'s popover does NOT (non-modal popover; outside click dismisses). Story 8.2 review note: focus traps swallow Tab navigation when overlapped by a higher-z overlay; non-modal popovers must allow focus to escape.
- **i18n parity test** (`flattenKeys`) catches every divergence — no hand-checking needed.
- **No emoji in source files unless explicit project decision** — emoji glyphs ARE the emote payload here, so they appear deliberately in `EmotePickerButton.tsx`'s glyph→ID mapping. This is a content choice for emote rendering, not a code-comment choice.
- **8.2 mentioned 8.3 will use a "BelotPrompt-style overlay"** — that note was speculative and is **not load-bearing**. The actual design is per-seat ephemeral bubbles + a non-modal picker popover, NOT a centred modal overlay.

### Cross-Story Context

- **Story 6.2 (done) — Match-Scoped Chat.** Direct architectural template for the server-side emote handler. The `IsUserInGame` + `MatchParticipants` interfaces, the `Broadcaster` minimal hub abstraction, and the silent-drop validation pattern are all reused verbatim.
- **Story 8.2 (done — 2026-04-27) — Team Surrender.** The most recent precedent in the same epic. Surrender is an **anti-precedent** for this story: it shows what NOT to do (don't put emotes in the rules engine, don't add a GameState field, don't persist them). Read 8.2 to absorb the WS contract sync rule and review-cycle discipline; do not copy its handler shape.
- **Story 4.5 (done) — Per-Move Timer.** The active player's turn timer is **untouched** by emotes; emotes do not advance, cancel, or extend the turn timer. The session manager never sees the action.
- **Story 5.1 / 5.4 (done) — Pause / Reconnect.** Emotes are dropped during `paused` phase (UI gate, AC #11) and not replayed on reconnect (AC #12). Pause itself does NOT block the emote handler server-side — `IsUserInGame` is true throughout pause — but the client picker is hidden during pause for UX coherence.
- **Story 4.7 (done) — Room Lobby WS Wiring.** `system:` event prefix precedent for non-game-state broadcasts. Emote follows the same prefix convention.
- **Story 8.1 (done) — Room Owner Pre-Game Controls.** Established the **Story 8.x shape** (small additive feature, ~10 review patches per cycle). Defensive guard pattern from 8.1 (`gameState !== null` before dispatcher commits) is reused.
- **Story 9.x (future — Phase 2) — XP / coins / honor.** Emotes do **not** affect XP, coins, or honor. No coupling. Future Story 9.6 (honor system) will not touch the emote handler.
- **Story 16.x (future — Phase 5) — Cosmetics.** A future cosmetic emote pack would extend `EmoteID` with new IDs and `EMOTE_IDS` array. The current six-emote whitelist is the minimum complete set; do not pre-architect for extensibility (project rule: don't design for hypothetical future requirements).

### Recent Codebase Signals (git log — last 10 commits)

- `257c6ce feat(game): add team surrender mechanic (Story 8.2)` — predecessor in this epic. Adds `SurrenderProposerSeat *int`, `SurrenderUsed [4]bool`, three rules-engine handlers, three new WS events, MatchEndPayload retrofit. Touchpoints to be aware of when reading manager.go and events.go.
- `34a8d95 feat(room): add owner kick + seat-swap pre-game controls (Story 8.1)` — added shadcn Dialog usage pattern. **Note**: 8.2's dev notes claimed an `alert-dialog` shadcn component was installed by 8.1; the actual repo only has `dialog.tsx` (no `alert-dialog.tsx`). 8.2's `SurrenderButton` uses the basic `Dialog` component. **For 8.3**: the emote picker does NOT need a Dialog primitive at all — it's a custom-positioned popover with click-outside dismissal.
- `b16920f chore(server): apply gofmt across the codebase` — gofmt is the canonical Go formatter; new files must conform.
- `588b6eb chore: add .gitattributes to enforce LF line endings` — Windows ↔ Linux normalisation. New files must be LF; let git's normalisation handle it.
- Pre-8.x commits — game-rules / UI fixes, not relevant to emote.

**Signal: `internal/chat/handler.go` is quiet since 6.2 landed; `cmd/api/main.go`'s composite action handler grew once for chat and is about to grow again for emote.** Touchpoint risk is **low** — the handler is a leaf module, the composite is a single closure with a clean precedent.

### Backend Flow — `action:emote`

1. Client sends `{ "type": "action:emote", "payload": { "emote": "thumbs_up" } }` over WS.
2. `ws.Hub.handleMessage` extracts prefix `action`, calls the registered action handler closure.
3. Composite action handler closure ([main.go:121-127](server/cmd/api/main.go#L121-L127)) inspects `msg.Type`. New arm: `if msg.Type == ws.ActionEmote { emoteHandler.HandleAction(client, msg); return }`.
4. `emote.Handler.HandleAction` decodes `EmoteRequest`. Validates `req.Emote` is in `ws.ValidEmoteIDs` map. Drop + log on unknown.
5. Resolves participants + seat via `sessionManager.MatchParticipantsByUser(client.UserID)`. Drop + log on `!ok` (sender not in game).
6. Acquires `lastEmoteAt` mutex, checks `time.Since(last) < 3*time.Second`. Drop + log on rate-limit. Else updates timestamp.
7. Builds `EmotePayload{PlayerSeat: seat, Emote: req.Emote}`, `json.Marshal` via `buildMessage`, calls `hub.BroadcastToUsers(participants[:], msgBytes)`.
8. Returns. No follow-up event. No DB write. No state change.

### Frontend Flow — Sending and Receiving an Emote

| Role                     | What They See                                                                                                                                                                                    | Driving State                                                                                                  |
| ------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ | -------------------------------------------------------------------------------------------------------------- |
| **Sender**               | Click emote toggle → grid pops open → click tile → grid closes immediately → ~150 ms later the bubble pops in at their own (South) seat. Re-clicking the toggle within 3 s shows disabled tiles. | Local `lastSentAt` for client throttle; `activeEmotes[mySeat]` set by the round-trip `system:emote` broadcast. |
| **Other 3 participants** | A bubble pops in at the sender's compass position with the emote glyph; auto-dismisses after 2 s (1 s with reduced motion).                                                                      | `activeEmotes[senderSeat]` set by `useWsDispatch`'s `SYSTEM_EMOTE` branch.                                     |
| **Spectator (future)**   | n/a — Phase 5.                                                                                                                                                                                   | n/a                                                                                                            |

The sender does NOT optimistically render their own bubble — they wait for the WS round-trip, identical to surrender / pause UI patterns (server is the authority, client renders received state). This guarantees the sender sees their own bubble at the same moment everyone else does, eliminating the cross-client desync that an optimistic render would risk.

### Project Structure Notes

**Modified files:** ~12 (per "What Must Be Modified" above).
**New files:** ~6 (1 Go handler, 1 Go test, 2 React components × 2 files = 4).
**Total LOC estimate:** ~600–800 lines including tests.

**Alignment with unified project structure:**

- Backend: new `internal/emote/` package matches the package-per-domain pattern (architecture L356-368).
- Frontend: components live under `features/game/components/` next to siblings (no new folder).
- WS contract: both sides updated in the same commit (project rule).
- i18n: keys nested under existing `game.*` block, not a new top-level block.
- Naming: `snake_case` for the emote IDs in wire format; `camelCase` for i18n key names; `PascalCase.tsx` for component file names.

### Alignment Checks / Detected Conflicts

- **Epic AC names align with implementation.** `action:emote`, `system:emote` are taken verbatim from epics.md AC text (lines 1505-1511). No prefix-discipline reinterpretation needed.
- **No "FR28a-style" reinterpretation** — the epic's "max 1 emote per 3 seconds per player" is implemented exactly as written: 3000 ms server-side window, per-user. Client throttle is a UX shadow of the same value.
- **Persisted vs ephemeral.** Emotes are ephemeral — explicitly NOT in `GameState`, NOT in any database table, NOT replayed on reconnect (AC #12). This is the deliberate counterpoint to surrender's persistence model.
- **`event:` vs `system:` prefix.** Architecture (L335) reserves `event:` for game-state changes. Emotes are NOT game state — `system:` is the correct prefix. This matches the epic spec literally and the chat-message precedent (`system:chat_message`).
- **Picker placement.** AC says "near chat toggle". The chat toggle is at `top-16 right-2` ([MatchChatSidebar.tsx:58](client/src/features/game/components/MatchChatSidebar.tsx#L58)); the picker goes one slot below at `top-32 right-2`. Both are within the existing top-right control cluster.
- **Phase gating asymmetry (server vs client).** Server uses `IsUserInGame` (loose — true throughout the entire match including `paused`). Client UI hides the picker during `paused` / `disconnected` / `match_end` (tight — UX coherence). This is intentional: server stays simple, client owns the look-and-feel.

### Edge Cases & Anti-Patterns to Avoid

- **Do NOT** add `ActionEmote` to [server/internal/game/types.go](server/internal/game/types.go) or `case ActionEmote` to `game.ApplyAction`. The emote handler runs _outside_ the rules engine.
- **Do NOT** add an `activeEmotes` field to the Go `GameState` struct. Emotes are not part of the snapshot, not part of reconnection state, not persisted.
- **Do NOT** add an `error:emote_*` event. All failure modes are silent drops with `slog.Info` — mirrors chat handler.
- **Do NOT** add a new shadcn primitive (`popover`, `alert-dialog`, etc.). The picker is a hand-rolled absolute-positioned grid with click-outside dismissal — six tiles do not warrant a UI library install.
- **Do NOT** persist the emote whitelist in a database table. The six IDs are hard-coded constants in `events.go` and `wsEvents.ts`. A future cosmetic emote pack (Phase 5, Epic 16.3) will extend this; for now, hard-coded is correct.
- **Do NOT** broadcast `event:game_state` after a `system:emote`. No state changed; an extra broadcast wastes bandwidth and triggers spurious re-renders.
- **Do NOT** apply `useFocusTrap` to the emote picker popover. It is non-modal; trapping focus prevents click-outside dismissal and breaks keyboard navigation to other controls.
- **Do NOT** replay queued emotes on reconnect. Emotes are fire-and-forget; a reconnecting client's first `system:emote` after reconnect is the next live one. (AC #12, #13.)
- **Do NOT** rate-limit globally (per-room). Rate limit is **per-user**: simultaneous emotes from different participants are all allowed. (AC #4 test case f.)
- **Do NOT** include a username field in `EmotePayload`. The client resolves username from `gameState.players[playerSeat].username` — payload stays small. (Different from `ChatMessagePayload` which carries username because chat predates the gameState.players username denormalisation.)
- **Do NOT** focus-trap the bubble. The bubble is purely visual: `pointer-events-none`, `aria-live="polite"`, no interactive elements.
- **Do NOT** suppress reduced-motion users entirely. They still see emotes — animation is shorter (1 s instead of 2 s) and the zoom-in transition is dropped (Tailwind `motion-safe:` handles this). Emote display is functional, not just decorative.
- **Do NOT** broadcast emotes during `dealing` if it complicates the deal animation timing. Pragmatic choice: allow `dealing` so players can react to bidding-end / hand-end transitions. If integration tests show emote bubbles colliding with `DealAnimation`, the gate is one line in GamePage to flip.
- **Counter-clockwise seat order is irrelevant to emote.** The bubble is anchored to the sender's compass position from the receiver's perspective via `compassOffset` — same math as everything else; no rotation logic needed.
- **Server-side timestamp is unnecessary in the wire payload.** AC requires no `timestamp` field in `EmotePayload`. The bubble's auto-dismiss timer is started client-side on receipt; clock skew is irrelevant for a 2 s ephemeral animation.

### References

- [\_bmad-output/planning-artifacts/epics.md#L1493-L1511](_bmad-output/planning-artifacts/epics.md#L1493-L1511) — Epic 8 / Story 8.3 ACs
- [\_bmad-output/planning-artifacts/epics.md#L209](_bmad-output/planning-artifacts/epics.md#L209) — FR32 maps to Epic 8 (in-game emotes)
- [\_bmad-output/planning-artifacts/epics.md#L51](_bmad-output/planning-artifacts/epics.md#L51) — FR32: "Players can express reactions during a match using preset in-game emotes"
- [\_bmad-output/planning-artifacts/architecture.md#L327-L336](_bmad-output/planning-artifacts/architecture.md#L327-L336) — WS event prefix discipline
- [\_bmad-output/planning-artifacts/architecture.md#L877](_bmad-output/planning-artifacts/architecture.md#L877) — communication packages map to `internal/chat/`
- [\_bmad-output/planning-artifacts/ux-design-specification.md#L117](_bmad-output/planning-artifacts/ux-design-specification.md#L117) — UX phase 2: in-game preset emotes (rate-limited, visible to all 4 seats)
- [\_bmad-output/planning-artifacts/prd.md#L136](_bmad-output/planning-artifacts/prd.md#L136) — PRD: "In-game preset emotes: rate-limited (max 1 per 3s per player), visible to all 4 seats"
- [\_bmad-output/planning-artifacts/prd.md#L349](_bmad-output/planning-artifacts/prd.md#L349) — FR32 PRD definition
- [\_bmad-output/project-context.md](_bmad-output/project-context.md) — global project rules (TS no-`enum`, WS contract sync, i18n parity, etc.)
- [server/internal/chat/handler.go](server/internal/chat/handler.go) — direct architectural template
- [server/internal/chat/handler_test.go](server/internal/chat/handler_test.go) — test scaffolding template
- [server/internal/ws/events.go](server/internal/ws/events.go) — WS contract (server side)
- [server/internal/ws/hub.go](server/internal/ws/hub.go) — `BroadcastToUsers`, `IsConnected`
- [server/internal/session/manager.go#L316-L335](server/internal/session/manager.go#L316-L335) — `IsUserInGame` / `MatchParticipants`
- [server/cmd/api/main.go#L121-L127](server/cmd/api/main.go#L121-L127) — composite action handler closure
- [client/src/shared/types/wsEvents.ts](client/src/shared/types/wsEvents.ts) — WS contract (client side)
- [client/src/shared/stores/gameStore.ts](client/src/shared/stores/gameStore.ts) — Zustand transient slot pattern
- [client/src/shared/hooks/useWsDispatch.ts](client/src/shared/hooks/useWsDispatch.ts) — WS event dispatcher with malformed-payload guards
- [client/src/features/game/GamePage.tsx](client/src/features/game/GamePage.tsx) — table layout, `compassOffset`, `SEAT_POSITIONS`
- [client/src/features/game/components/MatchChatSidebar.tsx](client/src/features/game/components/MatchChatSidebar.tsx) — chat-toggle button placement reference
- [client/src/shared/i18n/i18n.test.ts](client/src/shared/i18n/i18n.test.ts) — parity test the new keys must satisfy
- [\_bmad-output/archive/implementation-artifacts/8-2-team-surrender.md](_bmad-output/archive/implementation-artifacts/8-2-team-surrender.md) — predecessor story (anti-precedent for emote architecture; precedent for review discipline)

## Dev Agent Record

### Agent Model Used

claude-opus-4-7 (Opus 4.7, 1M context)

### Debug Log References

- Server `go test ./...` — all packages green, including the new `internal/emote` package and the extended session manager test.
- Client `npx vitest run` — 56 files / 520 tests green.
- `npx eslint .` and `npx prettier --check .` — clean.
- gofmt drift check (`gofmt -l .`) — clean.

### Completion Notes List

- WS contract additions land in **both** `events.go` and `wsEvents.ts` in the same commit (project rule).
- Emote handler placed in a **new** `server/internal/emote/` package, mirroring the chat handler shape — no rules-engine coupling, no GameState mutation, no DB write.
- Per-user 3 s server-side rate limit guarded by a `sync.Mutex` map. Client throttle is a UX shadow of the same value (3000 ms).
- White-box tests in the emote package directly manipulate `lastEmoteAt` to exercise the post-cooldown branch without sleeping (avoids slowing CI).
- `MatchParticipantsByUser` added to `session.Manager` because the emote payload carries no matchID — the userID→roomID lookup keeps the handler decoupled from the manager's internal indexing.
- `composite action handler` in `main.go` adds the emote arm **before** falling through to `sessionManager.HandleAction` to keep `parseAction` from rejecting `action:emote` as an unknown rules-engine type.
- gameStore gains `activeEmotes: Record<0|1|2|3, ActiveEmote | null>` initialised in `initialState`, so `clearGame()` automatically wipes it on navigation away (AC #6).
- `useWsDispatch` `SYSTEM_EMOTE` branch defensively validates `playerSeat` (number, 0..3) and `emote` (must be in `EMOTE_IDS`); also early-returns if `gameState === null` (Story 8.1 hardening).
- `EmotePickerButton` is a hand-rolled non-modal popover with click-outside + Escape dismissal — `useFocusTrap` is intentionally NOT applied (would break outside-click). Tile cooldown uses a single `setTimeout` keyed on `lastSentAt`, no per-tile timers.
- `EmoteBubble` auto-dismisses after 2000 ms (1000 ms on `prefers-reduced-motion: reduce`); cleans its timer on unmount.
- GamePage renders the picker at `top-32 right-2` (one slot below `MatchChatSidebar`'s toggle), gated to `dealing | bidding | playing` and suppressed under match-end / match-abandoned overlays.
- Bubbles are keyed on `${seat}-${receivedAt}` so back-to-back emotes from the same seat remount cleanly. Compass position is computed via the existing `compassOffset` helper.
- i18n keys added under `game.emote.*` in both `en.json` and `sr.json` — parity test (`flattenKeys`) green.

### File List

**New files:**

- `server/internal/emote/handler.go`
- `server/internal/emote/handler_test.go`
- `client/src/features/game/components/EmotePickerButton.tsx`
- `client/src/features/game/components/EmotePickerButton.test.tsx`
- `client/src/features/game/components/EmoteBubble.tsx`
- `client/src/features/game/components/EmoteBubble.test.tsx`

**Modified files:**

- `server/internal/ws/events.go`
- `server/internal/session/manager.go`
- `server/internal/session/manager_test.go`
- `server/cmd/api/main.go`
- `client/src/shared/types/wsEvents.ts`
- `client/src/shared/stores/gameStore.ts`
- `client/src/shared/stores/gameStore.test.ts`
- `client/src/shared/hooks/useWsDispatch.ts`
- `client/src/shared/hooks/useWsDispatch.test.ts`
- `client/src/features/game/GamePage.tsx`
- `client/src/features/game/GamePage.test.tsx`
- `client/src/shared/i18n/en.json`
- `client/src/shared/i18n/sr.json`
- `_bmad-output/implementation-artifacts/sprint-status.yaml`

### Change Log

| Date       | Description                                                                                                                                                                                                                                                                        | Author       |
| ---------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------ |
| 2026-04-28 | Story drafted with comprehensive context — chat-handler-style architecture (NOT rules engine), six-emote preset, 3 s rate limit, ephemeral per-seat bubbles                                                                                                                        | Bob (SM)     |
| 2026-04-28 | Implementation complete — WS contract + emote package + session helper + composite-handler arm + gameStore slot + dispatcher branch + picker/bubble components + GamePage integration + i18n. All quality gates green (go test, vitest, eslint, prettier, gofmt). Status → review. | Amelia (Dev) |

### Review Findings

_Code review on 2026-04-28. Three layers: Blind Hunter (diff-only), Edge Case Hunter (diff + project read), Acceptance Auditor (diff + spec). 4 patches, 6 deferred, 17 dismissed as noise / by-design / false-positive._

- [x] [Review][Patch] Bubble dismiss timer reset on every parent re-render — `[EmoteBubble.tsx:33-42]` — `useEffect(..., [onDismiss])` re-runs on every GamePage re-render because the inline arrow `() => setActiveEmote(player.seat, null)` creates a new function identity each render. During active gameplay (every gameState update from the WS triggers a re-render), the 2 s timer is cleared and rescheduled continuously, so the bubble may persist far longer than spec'd. Tests pass only because they mount the bubble in isolation with a stable `onDismiss`. **Fixed:** captured `onDismiss` in a ref so the timer effect runs once on mount; added regression test for the parent-rerender scenario. Source: edge-case.
- [x] [Review][Patch] `role="dialog"` on the emote popover contradicts non-modal Dev Notes — `[EmotePickerButton.tsx:120]` — Dev Notes explicitly characterise the picker as "a non-modal popover, not a dialog" when arguing against `useFocusTrap`. ARIA `role="dialog"` triggers screen-reader modal semantics that conflict with the actual non-modal behaviour (outside-click dismissal, no focus trap). **Fixed:** changed to `role="group"`. Source: auditor.
- [x] [Review][Patch] Arrow-key navigation between picker tiles missing — `[EmotePickerButton.tsx:117-145]` — AC #1: "arrow keys move focus inside the grid → Enter selects". Only Escape is wired; Tab traversal is the only inbound focus path. **Fixed:** added roving-tabindex-style ArrowUp/Down/Left/Right handler on the grid; auto-focus the first tile when the picker opens so keyboard users have an arrow-key starting point. Source: auditor.
- [x] [Review][Patch] `game.emote.picker.close` i18n key unused — `[i18n/en.json, i18n/sr.json]` — Task 10.1 added `picker.close` ("Close emote picker") in both locales but no consumer exists; outside-click and Escape are the only dismiss paths. **Fixed:** removed the orphan key from both locales (i18n parity preserved). Source: auditor.
- [x] [Review][Defer] `lastEmoteAt` map grows unbounded for the lifetime of the process — `[emote/handler.go:40, :99-105]` — deferred, leaks one entry per unique user who ever emoted; mitigation is small (eviction on session end / periodic sweep) but not blocking.
- [x] [Review][Defer] Cross-match rate-limit bleed-through silently drops first emote — `[emote/handler.go:98-106]` — deferred, first emote in a fresh match is silently dropped if the user emoted within 3 s of the previous match ending; rate-limit state is global per user, not session-scoped.
- [x] [Review][Defer] Picker `lastSentAt` resets on phase remount — `[EmotePickerButton.tsx:36, :92-100; GamePage.tsx:678-686]` — deferred, phase transitions unmount the picker; remount has fresh `lastSentAt=0` so tiles re-enable inside the 3 s window. Server still rate-limits, so the user gets a silent server-side drop with no UX feedback.
- [x] [Review][Defer] System clock backwards-jump locks picker for arbitrary time — `[EmotePickerButton.tsx:43, :94-95]` — deferred, NTP step or laptop sleep/resume makes `Date.now() - lastSentAt` go negative, producing huge `cooldownRemaining` values. Use `performance.now()` or clamp to `COOLDOWN_MS`.
- [x] [Review][Defer] Stale `participants` slice broadcast after concurrent `RemoveSession` — `[emote/handler.go:73-92; session/manager.go:629-646]` — deferred, race window between `MatchParticipantsByUser` (RLock) returning and `RemoveSession` (Lock) running before broadcast. Late-arriving emote frame can leak briefly to clients that just transitioned. Mitigated by the client's `gameState === null` guard.
- [x] [Review][Defer] Stale slot survives match-end overlay — `[gameStore.ts; GamePage.tsx:691-707]` — deferred, bubble unmounts under match-end overlay without firing `onDismiss`, leaving the seat slot non-null. The spec acknowledges this design ("the store still records the latest emote so that re-emergence renders the next live one"), but a brief overlay clear before `clearGame()` could replay a stale bubble. Add a freshness check (`Date.now() - slot.receivedAt < DURATION_MS`) when this becomes user-visible.
