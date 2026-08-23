---
title: "Room last-match stats with friend & reinvite actions"
type: "feature"
created: "2026-08-23"
status: "done"
review_loop_iteration: 0
baseline_commit: "60e8a889e6746a59c4f188dcb01223c9603623d1"
context: ["{project-root}/_bmad-output/project-context.md", "{project-root}/_bmad-output/implementation-artifacts/spec-dialog-button-layout-standardization.md"]
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** After a match the group lands back in the room lobby with no way to look at what just happened, and no path from "we just played together" to "let's play again" or "let's be friends" — the rich per-hand breakdown already built for the profile page is unreachable from the room and from the end-of-match dialog.

**Approach:** Add one room-scoped endpoint returning the room's most recent match in the existing viewer-relative match DTO, extract the profile's single-match renderer into a shared component, and mount it in two places: a dialog opened from the room action bar (with per-player Add-friend and Reinvite actions) and a collapsible section inside the end-of-match dialog (Add-friend only).

## Boundaries & Constraints

**Always:**
- Match participation IS the authorization gate: the endpoint returns the room's last match only if the caller occupied one of its four seats. This also guarantees `viewerSeat` is real, so the "us / them" projection is never relative to a stranger's seat.
- One shared card component renders the match in both surfaces — no duplicated hand-breakdown markup.
- Reinvite reuses the existing `POST /rooms/:id/invite` unchanged; it stays friends-only.
- New i18n keys land in all four locale files. Non-English strings must read naturally to a native speaker, not as literal English calques; `mk.json` is all-Cyrillic.
- `match-result-actions` in `MatchResult.tsx` is the named reference implementation of the both-actions footer pattern — insert above it, never restructure it.

**Ask First:**
- Relaxing the friendship requirement on room invites.
- Any change to the `GET /users/:id/matches` response shape (it is shared).

**Never:**
- No list of past matches, no pagination, no room match-history page — the most recent match only.
- No reinvite control in the end-of-match dialog (everyone is still present; "Return to room" already covers it).
- No new WebSocket events; both surfaces read over REST.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|---|---|---|---|
| Happy path | Caller played the room's last match | `200 {data: MatchListItem}` with hands ordered by `handNumber`, `viewerSeat` = caller's seat | N/A |
| Room never hosted a match | No `matches` row for `room_id` | `404 NOT_FOUND` | Room button hidden; dialog section hidden |
| Caller not a participant | Joined the room after the match | `404 NOT_FOUND` | Same as above — no data leak |
| Bot seat in match | `isBot === true` | Rendered as a localized bot name, no action buttons | N/A |
| Non-friend co-player | friendship `none` | Add-friend only; no Reinvite control | N/A |
| Friend already seated in room | Friend in `room.players` | Reinvite hidden | N/A |
| Reinvite rejected | Room full / invite pending / not friends | Toast with the mapped reason; row stays interactive | Map `ROOM_FULL`, `INVITE_ALREADY_PENDING`, `FRIEND_NOT_AVAILABLE`, `NOT_FRIENDS` |
| Last match unreadable | Persist failed at match end, or the row is otherwise absent | Surface degrades: no stats section in the end-of-match dialog, unavailable line in the room dialog | A 404 is terminal, never retried — "this room has no match yet" is the common case; transient errors retry twice; never a crash |

</frozen-after-approval>

## Code Map

**Backend**
- `server/internal/match/repository.go:43` -- `MatchRepository` interface; add the room-scoped read here.
- `server/internal/match/gorm_repo.go:163` -- `GetMatchesForUser` is the query template: status allowlist, 4-way `playerN_id` OR, `Preload("Hands", ORDER BY hand_number ASC)`, `Order("completed_at ...").Order("id ...")`.
- `server/internal/user/handler.go:759` -- `ListMatches`; `:856` `loadUsernamesForMatches`; `:899` `buildMatchListItem` (unexported, same package — reuse directly, do not rename). DTOs at `:173` `MatchPlayer`, `:181` `MatchHandView`, `:199` `MatchListItem`.
- `server/cmd/api/main.go:157` -- `userHandler` already holds `userRepo` + `matchRepo`; room routes at `:301-318`.
- `server/internal/apperr/errors.go:33` -- `ErrNotFound`; `:36` `ErrBadRequest`.
- Mocks to extend: `server/internal/user/handler_test.go:84`, `server/internal/match/manager_test.go:60` (the only two `MatchRepository` mocks).
- Read-only evidence: `server/internal/match/live_match.go:1311-1330` persists the match BEFORE `event:match_end`; `server/internal/match/reconnect.go:690-753` broadcasts `match_abandoned` BEFORE persisting — that ordering is why the client query retries.
- Auth precedent: `ListMatches` (`handler.go:760`) and `GetRoom` are authenticated-only with no membership check. `room.requireRoomMember` (`invite_handler.go:300`) is NOT reusable — it 404s any room whose status is not `waiting`.

**Frontend**
- `client/src/features/profile/MatchHistory.tsx` -- extraction source. `formatDuration` 37-60, `OutcomeChip` 62-96, `HandNotes` 121-179, `HandsGrid` 181-389 (mobile stack 233-310 / desktop grid 314-388), `MatchRow` 400-604. Scaffolding 606-754 stays.
- `client/src/features/profile/components/SeatChip.tsx` -- imported only by `MatchHistory.tsx`; moves with the card.
- `client/src/features/profile/components/FriendButton.tsx` -- all friendship states; carries a profile-only `my-5` rhythm.
- `client/src/shared/hooks/queries/useMatches.ts`, `client/src/shared/api/matches.ts:76`, `client/src/shared/api/queryKeys.ts:29` -- TanStack Query conventions (`axiosClient` unwraps the `{data}` envelope).
- `client/src/features/room/RoomPage.tsx` -- action bar 1243-1315 (Invite-friends button 1273-1284, gated on `room.status === "waiting"`); dialog stack 1567-1645; `InviteFriendsDialog` mounted unconditionally at 1619. Room players via `players` (802), viewer via `useAuthStore` (150).
- `client/src/features/room/components/InviteFriendsDialog.tsx` -- the dialog pattern to mirror, incl. `useInviteToRoomMutation` and the reset-on-`open` effect (49-54).
- `client/src/features/match/components/MatchResult.tsx` -- duration `<p>` ends at 240, `match-result-actions` starts at 242: insert between. Panel is `ClassicPanel width={520}` on dark felt.
- `client/src/features/match/MatchPage.tsx:2252` -- MatchResult mount site; `roomIdNum` at 225-231.
- `client/src/shared/i18n/{en,hr,sr,mk}.json` + `i18n.parity.test.ts` (key sets must match exactly, no empty leaves).

## Tasks & Acceptance

**Execution:**
- [x] `server/internal/match/repository.go` -- add `GetLastMatchForRoomAndUser(roomID, userID uint) (*Match, error)` to `MatchRepository` -- one query does room scoping, the participation gate, and recency.
- [x] `server/internal/match/gorm_repo.go` -- implement it: `room_id = ?`, `status IN ("completed","abandoned")`, 4-way `playerN_id = ?` OR, `Preload("Hands")` ordered by `hand_number ASC`, `ORDER BY completed_at DESC, id DESC LIMIT 1`; return `(nil, nil)` when absent.
- [x] `server/internal/user/room_last_match.go` -- new `UserHandler.GetRoomLastMatch`: parse `:id` (`ErrBadRequest`), call the repo with the authenticated caller, `ErrNotFound` on nil, else reuse `loadUsernamesForMatches` + `buildMatchListItem` and return `{data: MatchListItem}` -- keeps the DTO single-sourced; a room-package handler would need a user repo it does not have.
- [x] `server/cmd/api/main.go` -- register `api.GET("/rooms/:id/last-match", userHandler.GetRoomLastMatch)` after the room routes, with a comment noting why `userHandler` owns a `/rooms` path.
- [x] `server/internal/user/handler_test.go`, `server/internal/match/manager_test.go` -- extend both `mockMatchRepo`s; add handler tests for the matrix rows (participant, non-participant, no match, bad id).
- [x] `server/internal/match/gorm_repo_test.go` -- real-DB test: picks the newest row for the room, excludes other rooms, excludes non-participants, hands ordered.
- [x] `client/src/shared/components/matchStats/MatchStatsCard.tsx` (+ move `SeatChip.tsx` here) -- extract `formatDuration`/`OutcomeChip`/`HandNotes`/`HandsGrid`/`MatchRow` verbatim; props `{ match, subjectIsSelf?, isOpen, onToggle, handsLayout?: "auto" | "stacked", footer?: ReactNode }`; derive the subject name from `match.viewerSeat`; render `footer` inside the expanded detail below the hands.
- [x] `client/src/features/profile/MatchHistory.tsx` -- consume `MatchStatsCard`; keep list/filter/pagination and the lifted `openIds` set. Existing testids and behaviour must not change.
- [x] `client/src/shared/api/matches.ts`, `queryKeys.ts`, `hooks/queries/useMatches.ts` -- add `getRoomLastMatch(roomId)`, `matches.lastByRoom(roomId)`, and `useRoomLastMatchQuery(roomId, enabled)` with `retry: 2` and no retry on a settled 404.
- [x] `client/src/shared/components/matchStats/MatchPlayerActions.tsx` -- one row per human seat except the viewer: `FriendButton` always; a Reinvite button only when `showReinvite` and friendship is `friends` and that user is not in `playersInRoom`. Reinvite calls `useInviteToRoomMutation` and maps error codes to toasts.
- [x] `client/src/features/profile/components/FriendButton.tsx` -- add `compact?: boolean` dropping the `my-5` rhythm; default unchanged.
- [x] `client/src/features/room/components/LastMatchDialog.tsx` -- shadcn Dialog mirroring `InviteFriendsDialog` (`{ open, roomId, onOpenChange }`, reset-on-`open`): `MatchStatsCard` expanded by default with `MatchPlayerActions showReinvite` as its footer.
- [x] `client/src/features/room/RoomPage.tsx` -- run `useRoomLastMatchQuery` when the room is `waiting` and not quick-play; add a ghost button beside Invite-friends that renders only when the query has data; mount `LastMatchDialog` in the dialog stack.
- [x] `client/src/features/match/components/MatchResult.tsx` -- accept `roomId?: number`; between the duration and `match-result-actions`, add a chevron toggle whose panel holds `MatchStatsCard handsLayout="stacked"` plus `MatchPlayerActions` (no reinvite) on a light parchment inset. Collapsed by default; nothing renders while the query has no data.
- [x] `client/src/features/match/MatchPage.tsx` -- pass `roomId={roomIdNum ?? undefined}` to `MatchResult`.
- [x] `client/src/shared/i18n/{en,hr,sr,mk}.json` -- add keys in all four locales. Namespaces settled during review: `matchStats.*` for the shared card and player actions (a `room.*` namespace would have let room-dialog copy edits silently rewrite the match overlay), `room.lastMatch.*` for the room dialog chrome only, `match.matchResult.stats*` for the overlay toggle, and the reinvite failures reuse the existing `roomInvite.errors.*` rather than duplicating them.
- [x] Co-located Vitest files for `MatchStatsCard`, `MatchPlayerActions`, `LastMatchDialog`, plus RoomPage and MatchResult coverage for the new entry points.

**Acceptance Criteria:**
- Given a room whose last match I played, when I open the room lobby, then a last-match button appears and its dialog shows the same header + per-hand breakdown the profile shows.
- Given the dialog is open, when a co-player is a friend who is not currently in the room, then I see a Reinvite control that issues a real room invite; when they are not a friend, I see only Add-friend.
- Given a match just ended, when the end-of-match dialog appears, then a collapsed stats section sits above the two footer actions, and expanding it shows the same breakdown plus Add-friend per human opponent.
- Given I open the room lobby of a room whose last match I did not play, then no last-match button is shown and the endpoint returns 404.
- Given the profile page, when I expand a match row, then its rendering and testids are unchanged from before the extraction.

## Design Notes

**Why the participation gate.** `MatchListItem` is viewer-relative: `buildMatchListItem` derives `viewerSeat` by scanning the four seats for the caller and silently falls back to seat 0. Serving a non-participant would therefore render a confident but wrong "us / them". Scoping the SQL to `room_id AND caller-is-a-seat` makes the gate and the correctness guarantee the same predicate, and needs no room-membership lookup — which matters because `requireRoomMember` rejects any room not in `waiting`, and the end-of-match dialog fires while the room is `completed`.

**Why `handsLayout="stacked"` in the overlay.** `HandsGrid` picks its layout from a viewport media query, not container width. Inside `ClassicPanel width={520}` on a desktop viewport it would pick the ~476px-minimum desktop grid and overflow. Forcing the stacked layout there is cheaper and steadier than widening the panel or adding horizontal scroll.

## Verification

**Commands:**
- `make lint` -- expected: clean for both stacks.
- `make test` -- expected: all Go and Vitest suites pass, including `i18n.parity.test.ts` and the untouched `MatchHistory.test.tsx`.
- `cd server && go test ./internal/match/... ./internal/user/...` -- expected: new repo and handler tests pass (repo test skips without a reachable dev DB).

**Manual checks:**
- Play a match to completion, expand the stats section in the end-of-match dialog, return to the room, and confirm the room dialog shows the same match with working Add-friend and Reinvite.

## Suggested Review Order

**The read and its gate**

- Entry point: one query does room scoping, participation, and recency at once.
  [`gorm_repo.go:229`](../../server/internal/match/gorm_repo.go#L229)

- Why participation IS the authorization gate, not a check bolted beside it.
  [`repository.go:72`](../../server/internal/match/repository.go#L72)

- Handler reuses the existing viewer-relative DTO; nil row becomes a 404.
  [`room_last_match.go:31`](../../server/internal/user/room_last_match.go#L31)

- A `/rooms` path deliberately served by `userHandler` — the comment says why.
  [`main.go:326`](../../server/cmd/api/main.go#L326)

**Cache identity — the subtlest part of the change**

- Three layers stop the overlay ever painting the previous match.
  [`useMatches.ts:58`](../../client/src/shared/hooks/queries/useMatches.ts#L58)

- Removal, not invalidation: invalidate keeps serving the stale row while refetching.
  [`useWsDispatch.ts:308`](../../client/src/shared/hooks/useWsDispatch.ts#L308)

- Final guard: only paint a row matching this match's own end payload.
  [`MatchResult.tsx:92`](../../client/src/features/match/components/MatchResult.tsx#L92)

**Shared card extraction**

- The single-match renderer, now with `linkPlayers`, `handsLayout`, and a `footer` slot.
  [`MatchStatsCard.tsx:417`](../../client/src/shared/components/matchStats/MatchStatsCard.tsx#L417)

- What survived: 754 lines down to 162 of list/filter/pagination scaffolding.
  [`MatchHistory.tsx:25`](../../client/src/features/profile/MatchHistory.tsx#L25)

**Per-player actions**

- One row per human co-player; reinvite gated on friendship and absence from the room.
  [`MatchPlayerActions.tsx:49`](../../client/src/shared/components/matchStats/MatchPlayerActions.tsx#L49)

- Invite failures mapped once and shared with the existing invite dialog.
  [`inviteFailure.ts:35`](../../client/src/shared/lib/inviteFailure.ts#L35)

**Mount points**

- Room dialog mirrors `InviteFriendsDialog`, card expanded by default.
  [`LastMatchDialog.tsx:33`](../../client/src/features/room/components/LastMatchDialog.tsx#L33)

- Button gated on both waiting status and data — a disabled query keeps its cache.
  [`RoomPage.tsx:1322`](../../client/src/features/room/RoomPage.tsx#L1322)

- Overlay section sits above the footer actions, never restructuring them.
  [`MatchResult.tsx:283`](../../client/src/features/match/components/MatchResult.tsx#L283)

- The one line that enables the whole match-side feature.
  [`MatchPage.tsx:2261`](../../client/src/features/match/MatchPage.tsx#L2261)

**Containment and theming**

- Height cap plus internal scroll so footer actions stay reachable — helps every overlay.
  [`ClassicPanel.tsx:52`](../../client/src/features/match/components/overlay/ClassicPanel.tsx#L52)

- Parchment island inside a felt scope, so the shared card needs no fork.
  [`index.css:445`](../../client/src/index.css#L445)
