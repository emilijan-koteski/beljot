---
title: 'Mobile back-button history stack shaping'
type: 'feature'
created: '2026-07-19'
status: 'done'
review_loop_iteration: 0
context: []
baseline_commit: 'b4d68e4f9814af21a799e88ba0e73e45f6a2407a'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** On mobile browsers the back button walks through dead in-app history entries: after a match, back from the lobby lands on stale `/match`/`/matchmaking` URLs showing a "match has ended, redirecting" loader instead of exiting the app. Back is not meaningful anywhere.

**Approach:** Shape the history stack so `/lobby` is the app root and at most one entry sits above it: auth→lobby uses replace; lobby→X pushes; in-flow laterals (room↔matchmaking↔match) replace; returns to lobby pop back to the existing lobby entry (with replace fallback). Migrate MatchPage's sentinel `popstate` interceptor to React Router 7 `useBlocker` (requires data-router migration of App.tsx). Result: back in room/matchmaking/rules/profile/match-end → lobby; back in lobby → leaves the app; back mid-match → existing leave-confirm.

## Boundaries & Constraints

**Always:** Keep the mid-match back-press leave confirmation (reuse `match.leaveConfirm`, `window.confirm` parity). Preserve `{ fromRoom: true }` state on every navigation into `/match/:id`. Keep all existing error/guard redirects (`splashIssue`, room-closed, not-member) working. Pop-based return must fall back to `navigate("/lobby", { replace: true })` whenever the entry beneath is not provably the lobby (deep links, fresh reconnects). TypeScript strict; named exports; co-located tests.

**Ask First:** Any behavior change to server-side leave/abandon semantics; removing the leave confirmation entirely.

**Never:** No backend or WS-contract changes. No `beforeunload` traps or history-entry spamming to "eat" back presses. Don't try to `window.close()` the tab. Don't change guest access to `/rules` or landing/auth routing beyond the listed replaces.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Match ends, return to lobby, press back | Stack `[lobby, match]`, button pops | Browser leaves the app (lobby is bottom entry) | N/A |
| Back mid-match | Blocker armed (active match, POP) | Confirm dialog; OK → `clearGame()` + pop/replace to lobby; Cancel → stay on match | N/A |
| Back on match-end result overlay | `matchEndData`/`matchAbandonedData` set or phase `match_end` | Blocker disarmed; back goes straight to lobby; game store still cleared | N/A |
| Back in room / matchmaking / profile / rules | Pushed from lobby | Natural pop → lobby; room/queue unmount cleanup (auto-leave) still fires | N/A |
| Deep link straight into `/match/:id` or `/rooms/:id` | No lobby entry beneath | Returns use replace fallback → `[.., lobby]`; back then exits app | N/A |
| Fresh app open while match live (reconnect) | Redirect fires from `/lobby` | Push (not replace) so stack is `[lobby, match]` | Non-lobby origin keeps replace |

</frozen-after-approval>

## Code Map

- `client/src/App.tsx` -- `<BrowserRouter>`+`<Routes>`; migrate to data router
- `client/src/features/match/MatchPage.tsx` -- sentinel popstate interceptor ~L925-949; lobby returns L1148/1160/1197/1220; play-again L1167; guard replaces L754/826 (keep)
- `client/src/features/room/RoomPage.tsx` -- laterals→match L357/540/719; lobby returns L391/523; guard replaces L284/326/377/1553 (keep)
- `client/src/features/lobby/MatchmakingPage.tsx` -- lateral→match L105; lobby returns L155/190; guard replaces L119/135/145 (keep)
- `client/src/features/lobby/LobbyPage.tsx` -- lobby root; pushes L100/102/167 stay push
- `client/src/shared/hooks/{useMatchStartRedirect,useReconnectionRedirect,useInsolventEjectRedirect}.ts` -- always-mounted navigators
- `client/src/features/match/MatchPage.test.tsx` -- renders via plain MemoryRouter/BrowserRouter (helper L145-163 + rerenders ~L726/855); breaks with `useBlocker`

## Tasks & Acceptance

**Execution:**
- [x] `client/src/shared/hooks/useLobbyReturn.ts` -- NEW: `useMarkLobbyRoot()` stores react-router `window.history.state.idx` for `/lobby` in sessionStorage; `useLobbyReturn()` returns `returnToLobby()` = `navigate(-1)` when current idx-1 equals stored lobby idx, else `navigate("/lobby", { replace: true })`. Co-located test.
- [x] `client/src/features/lobby/LobbyPage.tsx` -- call `useMarkLobbyRoot()` on mount.
- [x] Auth entries -- `LoginPage.tsx:72`, `RegisterPage.tsx:104`, `useGoogleSso.ts:56,87` add `{ replace: true }`.
- [x] Laterals to replace -- `RoomPage.tsx:357,540,719`, `MatchmakingPage.tsx:105`, `MatchPage.tsx:1167` add `{ replace: true }` (keep existing `state`); `useMatchStartRedirect.ts:29` and `useReconnectionRedirect.ts:24` use push when `pathname === "/lobby"`, replace otherwise.
- [x] Lobby returns -- `MatchPage.tsx:1148,1160,1197,1220`, `RoomPage.tsx:391,523`, `MatchmakingPage.tsx:155,190`, `useInsolventEjectRedirect.ts:26`, `RulesFooter.tsx:35` switch to `returnToLobby()`.
- [x] `client/src/App.tsx` -- data-router migration: root layout component runs `useAuthInit`/`useTokenRefresh` + isLoading gate + `<Outlet/>`; `createBrowserRouter(createRoutesFromElements(...))` preserving the exact route tree; `<RouterProvider>`; Toaster placement unchanged.
- [x] `client/src/features/match/MatchPage.tsx` -- delete sentinel block (~L925-949, `historyPushedRef` incl.); add `useBlocker(({ historyAction }) => matchActive && historyAction === "POP")` where `matchActive` = matchState set ∧ phase ≠ `match_end` ∧ no end/abandon data; blocked → `window.confirm(t("match.leaveConfirm"))`: OK → `clearGame()` + (`proceed()` if lobby beneath else `reset()` + replace `/lobby`), Cancel → `reset()`. Ensure game store is cleared when unmounting in end-state via pop (verify existing wipe-on-navigation semantics; add unmount cleanup only if missing).
- [x] `client/src/shared/components/TopBar.tsx` -- nav links `replace` when `pathname !== "/lobby"`; brand (authed) and "Play" link to `/lobby` intercept click → `returnToLobby()` (keep `href` for a11y/middle-click).
- [x] `client/src/features/match/MatchPage.test.tsx` -- migrate render helper + rerenders to `createMemoryRouter` + `RouterProvider`.

**Acceptance Criteria:**
- Given a full flow lobby→room→match→match end→return to lobby, when the user presses back once in the lobby, then the browser exits the app (no dead `/match`/`/matchmaking` entries, no "redirecting" loader).
- Given an active match, when the user presses back and cancels the confirm, then they remain on `/match/:id` with game state intact and can keep playing.
- Given the match-end result overlay, when the user presses back, then they land on a live lobby with no confirm dialog and no reconnection bounce-back into the match.
- Given a room, matchmaking queue, profile, or rules page entered from the lobby, when the user presses back, then they land on the lobby (and room/queue membership is released).
- Given quick-play auto-start from the lobby (`LobbyPage.tsx:100`), when the match ends and the player returns, then the stack is `[lobby]` again.

## Spec Change Log

## Design Notes

- Lobby-root detection reads `window.history.state.idx` (react-router-maintained, semi-internal). Failure mode is benign: `canPopToLobby` false → replace → at worst one duplicate lobby entry, never a dead match entry.
- `useBlocker` throws outside data routers — the App.tsx migration is a hard prerequisite.
- Blocked-POP + `proceed()` continues the original pop onto the live lobby entry beneath — the pop itself is the navigation.
- Back out of room/matchmaking relies on existing unmount auto-leave (`hasLeftRef` guards) — no explicit leave calls.

## Verification

**Commands:**
- `cd client && npx vitest run` -- expected: all suites pass, incl. migrated MatchPage.test.tsx and new useLobbyReturn test
- `make lint` -- expected: clean

**Manual checks (if no CLI):**
- On a mobile device/emulator: play a quick-play match to completion, return to lobby, press back → app exits; mid-match back shows confirm; back from rules/profile/room → lobby.

## Suggested Review Order

**Core mechanism — the lobby-root contract**

- The whole design in one file: mark the lobby's history index, pop when it sits beneath, replace otherwise.
  [`useLobbyReturn.ts:45`](../../client/src/shared/hooks/useLobbyReturn.ts#L45)

- Re-entrancy guard (review finding): `navigate(-1)` is not idempotent — a double-tap must not pop past the lobby.
  [`useLobbyReturn.ts:75`](../../client/src/shared/hooks/useLobbyReturn.ts#L75)

- LobbyPage records itself as the root on every mount.
  [`LobbyPage.tsx:71`](../../client/src/features/lobby/LobbyPage.tsx#L71)

**Data-router migration (useBlocker prerequisite)**

- Route tree unchanged, now under `createBrowserRouter`; built lazily inside `App()` to avoid import-time side effects.
  [`App.tsx:64`](../../client/src/App.tsx#L64)

- `AppShell` root layout carries the old `AppRoutes` auth bootstrap + loading gate.
  [`App.tsx:45`](../../client/src/App.tsx#L45)

**Mid-match back confirmation (sentinel → useBlocker)**

- Blocker armed only for POPs while the match is genuinely live — result overlays exit freely.
  [`MatchPage.tsx:942`](../../client/src/features/match/MatchPage.tsx#L942)

- Accept path decides via `blocker.location`, not `history.state` probing (review finding: revert race, multi-entry jumps).
  [`MatchPage.tsx:953`](../../client/src/features/match/MatchPage.tsx#L953)

- StrictMode-safe unmount cleanup wipes an ended match only after a real navigation away.
  [`MatchPage.tsx:982`](../../client/src/features/match/MatchPage.tsx#L982)

**Call-site sweep — push/replace/pop discipline**

- Laterals into the match replace, preserving `fromRoom` splash state.
  [`RoomPage.tsx:361`](../../client/src/features/room/RoomPage.tsx#L361)

- Always-mounted navigators: push only when leaving from `/lobby`, replace elsewhere.
  [`useMatchStartRedirect.ts:32`](../../client/src/shared/hooks/useMatchStartRedirect.ts#L32)

- Match-end returns pop back to the lobby root.
  [`MatchPage.tsx:1182`](../../client/src/features/match/MatchPage.tsx#L1182)

- Auth → lobby entries replace, so the lobby lands at the stack bottom.
  [`LoginPage.tsx:74`](../../client/src/features/auth/LoginPage.tsx#L74)

- Guests keep the old push on the Rules CTA (review finding: history erasure); authed users pop.
  [`RulesFooter.tsx:17`](../../client/src/features/rules/components/RulesFooter.tsx#L17)

**TopBar link semantics**

- Cross-page nav replaces; lobby-targeted links pop via click interception, `href` kept for a11y/middle-click.
  [`TopBar.tsx:68`](../../client/src/shared/components/TopBar.tsx#L68)

**Tests**

- Blocker behavior: decline stays, accept proceeds onto the lobby, end-state pops through unblocked.
  [`MatchPage.test.tsx:1207`](../../client/src/features/match/MatchPage.test.tsx#L1207)

- Hook unit tests incl. double-tap guard and empty-string index hardening.
  [`useLobbyReturn.test.tsx:1`](../../client/src/shared/hooks/useLobbyReturn.test.tsx#L1)

- TopBar shaping: replace between pages, pop on Play, modified-click passthrough.
  [`TopBar.test.tsx:129`](../../client/src/shared/components/TopBar.test.tsx#L129)

- Redirect hook push-vs-replace asserted behaviorally through a data router.
  [`useMatchStartRedirect.test.tsx:84`](../../client/src/shared/hooks/useMatchStartRedirect.test.tsx#L84)
