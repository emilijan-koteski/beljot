---
title: "Join Room by Code"
type: "feature"
created: "2026-04-12"
status: "done"
baseline_commit: "a9dab56"
context: ["_bmad-output/project-context.md"]
---

<frozen-after-approval reason="human-owned intent -- do not modify unless human renegotiates">

## Intent

**Problem:** Players who receive a room code (e.g., shared via WhatsApp/Viber) have no way to join directly -- they must browse the room list and search for it. If the room is full or in-progress, it won't appear in the "waiting" list at all.

**Approach:** Add a dedicated "Join by Code" input on the lobby options view and a backend endpoint to look up a room by its unique code, then join it in one flow.

## Boundaries & Constraints

**Always:** Reuse existing `joinRoom` POST endpoint for the actual join. New backend endpoint only resolves code -> room detail. Follow existing lobby card styling for the join-by-code UI. Add translations for both EN and SR.

**Ask First:** Whether to allow joining rooms that are `playing` status (reconnect scenario) vs. only `waiting` rooms.

**Never:** Do not modify the existing browse/search flow. Do not add URL-based deep linking (deferred). Do not change room code generation logic.

## I/O & Edge-Case Matrix

| Scenario                 | Input / State                    | Expected Output / Behavior                          | Error Handling                         |
| ------------------------ | -------------------------------- | --------------------------------------------------- | -------------------------------------- |
| Valid code, room waiting | 6-char code, room status=waiting | Room found, join succeeds, navigate to `/rooms/:id` | N/A                                    |
| Valid code, room full    | 6-char code, room 4/4 players    | Join attempt returns 409                            | Toast: "Room is full" (reuse existing) |
| Invalid/expired code     | Non-existent code string         | 404 from lookup                                     | Toast: "Room not found"                |
| Empty input              | Submit with blank field          | Button disabled, no request                         | N/A                                    |
| Already in room          | Valid code, user already joined  | Join returns 409 ALREADY_IN_ROOM                    | Toast: reuse existing error            |

</frozen-after-approval>

## Code Map

- `server/internal/room/handler.go` -- Add `GetRoomByCode` handler
- `server/internal/room/repository.go` -- Add `FindByCode(code string)` to interface
- `server/internal/room/repository_impl.go` -- Implement `FindByCode`
- `server/cmd/api/main.go` -- Register `GET /rooms/code/:code` route
- `client/src/shared/api/rooms.ts` -- Add `getRoomByCode(code)` API function
- `client/src/features/lobby/JoinByCode.tsx` -- New component: code input + join button
- `client/src/features/lobby/LobbyPage.tsx` -- Render `JoinByCode` in options view
- `client/src/shared/i18n/en.json` -- Add `lobby.joinByCode.*` keys
- `client/src/shared/i18n/sr.json` -- Add `lobby.joinByCode.*` keys

## Tasks & Acceptance

**Execution:**

- [x] `server/internal/room/repository.go` -- Add `FindByCode(code string) (*Room, error)` to `Repository` interface
- [x] `server/internal/room/gorm_repo.go` -- Implement `FindByCode` using GORM `Where("code = ?", code).First(&room)`
- [x] `server/internal/room/handler.go` -- Add `GetRoomByCode(c echo.Context) error` handler: extract `:code` param, call `FindByCode`, return 404 if not found, else return `RoomDetailResponse`
- [x] `server/cmd/api/main.go` -- Register `GET /rooms/code/:code` before the `/:id` route to avoid param conflict
- [x] `client/src/shared/api/rooms.ts` -- Add `getRoomByCode(code: string): Promise<RoomDetail>` calling `GET /rooms/code/:code`
- [x] `client/src/features/lobby/JoinByCode.tsx` -- Create component: Input (uppercase, max 6 chars) + "Join" Button. On submit: call `getRoomByCode`, then `joinRoom(room.id)`, navigate to `/rooms/:id`. Error toasts on failure. Button disabled when input empty or request in-flight.
- [x] `client/src/features/lobby/LobbyPage.tsx` -- Add `JoinByCode` component below the three option cards in the options view, with a subtle divider/label "Have a room code?"
- [x] `client/src/shared/i18n/en.json` -- Add keys: `lobby.joinByCode.label`, `lobby.joinByCode.placeholder`, `lobby.joinByCode.join`, `lobby.joinByCode.notFound`
- [x] `client/src/shared/i18n/sr.json` -- Add matching Serbian translations
- [x] `server/internal/room/handler_test.go` -- Add `FindByCode` to mock repository (interface compliance)

**Acceptance Criteria:**

- Given a player on the lobby page, when they see the options view, then a "Join by Code" input is visible below the option cards
- Given a player enters a valid room code and clicks Join, when the room exists and has space, then they are navigated to the room lobby
- Given a player enters an invalid code, when they click Join, then a "Room not found" toast appears and they remain on the lobby page

## Verification

**Commands:**

- `cd server && go build ./...` -- expected: compiles without errors
- `cd server && go test ./internal/room/...` -- expected: existing tests pass
- `cd client && npx tsc --noEmit` -- expected: no type errors
- `cd client && npx vitest run` -- expected: existing tests pass

**Manual checks:**

- Open lobby, enter a valid room code, verify join + navigation works
- Enter garbage code, verify error toast appears

## Suggested Review Order

**Backend: room lookup by code**

- New repo interface method — single line addition for code-based lookup
  [`repository.go:7`](../../../server/internal/room/repository.go#L7)

- GORM implementation — parameterized query, nil-on-not-found pattern matches FindByID
  [`gorm_repo.go:47`](../../../server/internal/room/gorm_repo.go#L47)

- Handler: validates length, uppercases, filters non-waiting rooms, reuses RoomDetailResponse
  [`handler.go:203`](../../../server/internal/room/handler.go#L203)

- Route registration — placed before `:id` wildcard to avoid param conflict
  [`main.go:88`](../../../server/cmd/api/main.go#L88)

**Frontend: join-by-code UI**

- API client function — matches backend route, encodes code for safety
  [`rooms.ts:24`](../../../client/src/shared/api/rooms.ts#L24)

- JoinByCode component — input (uppercase, 6-char limit), two-step lookup+join, error toasts
  [`JoinByCode.tsx:1`](../../client/src/features/lobby/JoinByCode.tsx#L1)

- LobbyPage integration — divider + component below option cards
  [`LobbyPage.tsx:147`](../../../client/src/features/lobby/LobbyPage.tsx#L147)

**i18n**

- English translations — label, placeholder, join button, not-found toast
  [`en.json:89`](../../../client/src/shared/i18n/en.json#L89)

- Serbian translations — matching keys
  [`sr.json:89`](../../../client/src/shared/i18n/sr.json#L89)

**Tests**

- Backend handler tests — success, not-found, non-waiting rejection, invalid length, case normalization, unauthorized
  [`handler_test.go:710`](../../../server/internal/room/handler_test.go#L710)

- Frontend component tests — render, disabled state, uppercase, max length, success nav, error toasts, Enter key
  [`JoinByCode.test.tsx:1`](../../client/src/features/lobby/JoinByCode.test.tsx#L1)
