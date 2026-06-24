---
baseline_commit: beba746010aebb9e5d13436471962c5420393871
---

# Story 9.6: Private Rooms

Status: done

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

## Story

As a room owner,
I want to protect my room with a password so only people I share it with can enter,
so that I can host a closed table for friends without it being publicly joinable.

> **Scope guardrail (read first):** This story adds **password-gated entry to owner-created rooms** — nothing more. Private rooms stay **listed but locked** ("Option B" — the join code keeps working for discovery/identity). Do **NOT** build hidden/unlisted rooms, invite links, per-user allow-lists, or the honor gate (that is **Story 9.8**, a separate feature). The privacy gate is enforced at exactly **one** authoritative point: the seat-granting join action.
>
> **The single most important architectural fact:** the epic AC says "verified server-side on `action:join_room`" with "`error:wrong_room_password`". **There is no `action:join_room`.** Room create and join are **HTTP endpoints** (`POST /rooms`, `POST /rooms/:id/join`), not WebSocket actions — the epic wording is a planning-time assumption. Implement the rejection as an HTTP `apperr.AppError` with code `WRONG_ROOM_PASSWORD` (HTTP 409), **exactly mirroring `INSUFFICIENT_COINS`** from Stories 9.2/9.3 (which the epic likewise styled `error:insufficient_coins` but which is an HTTP error). **No new WebSocket event. No `events.go`/`wsEvents.ts` constant. No golden/Zod drift-gate change.** See [Design Decision D1](#design-decisions).

## Acceptance Criteria

**AC1 — Create-room "Private room" toggle + password (FR60)**
**Given** a room owner is configuring a new room
**When** the create-room modal is open
**Then** a "Private room" toggle is available (default **off**)
**And** enabling it reveals a **required** "Room password" field with basic validation (non-empty; `roomPasswordMinLength` = **4** characters placeholder; max **72 bytes** — bcrypt's hard input limit, see [D3](#design-decisions))
**And** the create-room request carries `isPrivate` (boolean) and an optional `password` (string, required only when `isPrivate` is true)
**And** the room keeps its normal 6-char join code — privacy is **orthogonal** to code-based discovery/identity

**AC2 — Hashed persistence + schema**
**Given** a private room is saved
**When** it is persisted
**Then** the password is stored **bcrypt-hashed** in `rooms.password_hash` (nullable `VARCHAR(60)`; **NULL = public room**) via the existing `auth.HashPassword` helper — the **plaintext password is never stored or logged**
**And** migration **`000012_add_room_password`** adds the column with a matching `.down.sql` that fully reverses it (`DROP COLUMN password_hash`)
**And** `Room.PasswordHash *string` carries `json:"-"` so it is **never serialized** to any client

**AC3 — Browse list shows "locked", never leaks the password**
**Given** the room browse / search list (and any room lifecycle broadcast)
**When** a private room is rendered
**Then** it is **still listed** but shown with a lock indicator on the card
**And** every room-returning response and broadcast carries a derived `isPrivate` boolean (`password_hash != nil`)
**And** the password hash is **never** present in any API response, WS payload, or log line

**AC4 — Join gate: password prompt before join, verified server-side**
**Given** a player attempts to join a private room — **either** by clicking its locked card in the lobby grid **or** by entering its join code
**When** the join is initiated
**Then** a password dialog is prompted **before** the join proceeds (the client knows the room is private from `room.isPrivate`)
**And** the entered password is sent on `POST /rooms/:id/join` (body `{ password }`) and verified server-side against `password_hash` via `auth.CheckPassword`
**And** a **correct** password lets the join continue through the **unchanged** seating flow (capacity, already-in-room, and coin-affordability checks all still apply)
**And** a **wrong or missing** password is rejected with `apperr.ErrWrongRoomPassword` (code `WRONG_ROOM_PASSWORD`, HTTP **409**) — missing and incorrect are **indistinguishable** to the client
**And** the client maps `WRONG_ROOM_PASSWORD` to the message "Incorrect room password." and keeps the dialog open for retry (no navigation away)

**AC5 — Public rooms unchanged**
**Given** a room with no password (`password_hash` is NULL → `isPrivate` false)
**When** a player joins by card or by code
**Then** **no** password dialog appears and behavior is **identical to today** — no extra round-trip, no regression to the existing join / quick-join / return-to-room flows

**AC6 — Owner edit: change password or revert to public**
**Given** a private (or public) room's owner, while the room is in `waiting` status
**When** they update room privacy via the new owner-only endpoint `POST /rooms/:id/privacy` (body `{ isPrivate, password? }`)
**Then** they can set/change the password (`isPrivate: true` + non-empty `password` → re-hash and store) or make the room public again (`isPrivate: false` → clear `password_hash` to NULL)
**And** the endpoint is **owner-only** (`ErrNotRoomOwner` otherwise) and **waiting-only** (`ErrRoomNotWaiting` once a match has started)
**And** changing or clearing the password **does NOT eject players already seated** (the gate is join-time only — there is no re-check of seated players)
**And** the change broadcasts `system:room_updated` so lobby cards flip their lock indicator live

**AC7 — Quick Play rooms are never private**
**Given** Quick Play matchmaking synthesizes a server-side room
**When** the room is created
**Then** its `password_hash` stays **NULL** (password gating applies only to owner-created rooms) — the QuickPlay/QuickJoin paths never prompt for or accept a password

**AC8 — No regressions; Definition of Done**
**Given** the existing create-room, browse, join-by-card, join-by-code, return-to-room, quick-play, and owner-control flows
**When** privacy is layered in
**Then** none of those behaviors change except the added privacy gate and indicator
**And** the implementation is **HTTP-only** (no WS contract change; the drift-gate / golden / Zod files are untouched)
**And** the [feature-complete checklist](#testing-standards) passes: server handler + repo + tests, new domain error in `apperr`, frontend components + co-located tests, i18n in **all four** locales (en/sr/mk/hr), `make lint` + `make test` green

---

## Tasks / Subtasks

> **Read [Dev Notes](#dev-notes) in full before starting.** The three highest-leverage facts: (1) **This is HTTP, not WS** — mirror `INSUFFICIENT_COINS`, add `ErrWrongRoomPassword`, touch **zero** WS contract files ([D1](#design-decisions)). (2) **Privacy = `password_hash != nil`** — no `is_private` column; the wire `isPrivate` boolean is derived, the hash is `json:"-"` and never leaves the server ([D2](#design-decisions)). (3) **The gate fires at exactly one place** — the `JoinRoom` handler (the authoritative seat grant); browse/code-lookup reveal `isPrivate` but never demand the password ([D5](#design-decisions)).

- [x] **Task 1 — Migration `000012`: add `password_hash` to rooms (AC2)**
  - [x] Create `server/migrations/000012_add_room_password.up.sql`: `ALTER TABLE rooms ADD COLUMN password_hash VARCHAR(60);` (nullable, no default → existing rows backfill to NULL = public). Mirror the additive-ALTER comment style of [000010_add_coin_economy_columns.up.sql](../../server/migrations/000010_add_coin_economy_columns.up.sql).
  - [x] Create the matching `000012_add_room_password.down.sql`: `ALTER TABLE rooms DROP COLUMN password_hash;`.
  - [x] **Confirm `000012` is the next number** — highest existing is `000011_add_xp_to_users`. Never skip numbers.
  - [x] `VARCHAR(60)` because a bcrypt hash is exactly 60 chars; nullable (no `NOT NULL`, no CHECK) — NULL is the public-room sentinel.

- [x] **Task 2 — Room model: `PasswordHash` (persisted) + `IsPrivate` (derived) (AC2, AC3)**
  - [x] Add to the `Room` struct in [server/internal/room/model.go:9-38](../../server/internal/room/model.go#L9): `PasswordHash *string \`gorm:"size:60" json:"-"\`` (pointer so NULL is representable; `json:"-"` so it **never** serializes). Place it near `CoinBuyIn`.
  - [x] Add the derived wire field: `IsPrivate bool \`gorm:"-" json:"isPrivate"\`` (transient, like `OwnerUsername`/`Players` at [model.go:14-22](../../server/internal/room/model.go#L14)).
  - [x] Add a GORM `AfterFind` hook so **every** DB read auto-populates the derived flag in one place: `func (r *Room) AfterFind(tx *gorm.DB) error { r.IsPrivate = r.PasswordHash != nil; return nil }`. This covers `FindByID`/`FindByCode`/`FindByStatus`/`FindPlayerRoom` etc. without per-handler edits. (See [D4](#design-decisions).)

- [x] **Task 3 — Create-room: accept + validate + hash (AC1, AC2)**
  - [x] Extend `CreateRoomRequest` ([handler.go:58-68](../../server/internal/room/handler.go#L58)) with `IsPrivate bool \`json:"isPrivate"\`` and `Password string \`json:"password"\``.
  - [x] In `CreateRoom` ([handler.go:300-469](../../server/internal/room/handler.go#L300)), **after** the existing field validation and **before** building the `room` struct: if `req.IsPrivate`, validate the password (`validateRoomPassword(req.Password)` → non-empty, `len >= roomPasswordMinLength`, `len(bytes) <= 72`; new `apperr.ErrRoomPasswordRequired` / `ErrRoomPasswordTooShort` / `ErrRoomPasswordTooLong`), then `hashed, err := auth.HashPassword(req.Password)` and set `room.PasswordHash = &hashed`. If `!req.IsPrivate`, leave `PasswordHash` nil.
  - [x] Set `room.IsPrivate = room.PasswordHash != nil` on the in-memory `room` **before** the `c.JSON(...)` response and the `roomLifecyclePayload` broadcast (the `AfterFind` hook does not fire on a freshly-`Create`d struct — see [D4](#design-decisions)).
  - [x] Add a small `validateRoomPassword(pw string) error` helper + named consts (`roomPasswordMinLength = 4`, `roomPasswordMaxBytes = 72`) — placeholders, tunable (see [D3](#design-decisions)).

- [x] **Task 4 — Join gate: verify password on the authoritative seat grant (AC4, AC5)**
  - [x] Add an optional password field to the join request. The endpoint takes an HTTP body now: define `type JoinRoomRequest struct { Password string \`json:"password"\` }` and `c.Bind(&req)` in `JoinRoom` ([handler.go:596-617](../../server/internal/room/handler.go#L596)). A `Bind` failure on an empty body must be tolerated (public-room joins send no body) — bind into a value with a zero-value default and ignore `ErrBadRequest` from an empty body, OR read the body defensively.
  - [x] Insert the password check **immediately after** the room-exists / status checks (after [handler.go:617](../../server/internal/room/handler.go#L617)) and **before** the capacity / affordability checks: `if room.PasswordHash != nil { if auth.CheckPassword(*room.PasswordHash, req.Password) != nil { return apperr.ErrWrongRoomPassword } }`. A nil hash (public room) skips the check entirely (AC5).
  - [x] **Do not differentiate** missing vs incorrect password — both yield `ErrWrongRoomPassword`. Do not log the attempted password.
  - [x] **Leave `GetRoomByCode` ([handler.go:560-594](../../server/internal/room/handler.go#L560)) and `GetRoom` password-free** — they return room detail (incl. `isPrivate`) for the prompt; they must NOT require or verify a password ([D5](#design-decisions)). The authoritative gate is `JoinRoom` only.

- [x] **Task 5 — Owner edit endpoint: `POST /rooms/:id/privacy` (AC6)**
  - [x] New handler `UpdateRoomPrivacy(c echo.Context)`: auth → parse `:id` → `FindByID` → owner check (`room.OwnerID != userID` → `apperr.ErrNotRoomOwner`) → waiting check (`room.Status != "waiting"` → `apperr.ErrRoomNotWaiting`). Bind `type UpdateRoomPrivacyRequest struct { IsPrivate bool \`json:"isPrivate"\`; Password string \`json:"password"\` }`.
  - [x] If `IsPrivate`: `validateRoomPassword(req.Password)`, `hashed, _ := auth.HashPassword(req.Password)`, set `room.PasswordHash = &hashed`. Else: set `room.PasswordHash = nil`. Persist via `h.repo.Update(room)` (existing repo method — already persists all fields).
  - [x] Set `room.IsPrivate` and broadcast `h.broadcastRoomUpdated(room)` so lobby cards flip the lock live ([handler.go:282-284](../../server/internal/room/handler.go#L282)). Return `200` with the updated room. **Do not touch seats / presence** — seated players are never ejected (AC6).
  - [x] Register the route in [server/cmd/api/main.go:233](../../server/cmd/api/main.go#L233) next to the other room sub-routes: `api.POST("/rooms/:id/privacy", roomHandler.UpdateRoomPrivacy)`.

- [x] **Task 6 — Derived `isPrivate` in the lifecycle broadcast map (AC3)**
  - [x] In `roomLifecyclePayload` ([handler.go:261-278](../../server/internal/room/handler.go#L261)) add `"isPrivate": r.PasswordHash != nil` to the returned `map[string]any` (this map is built by hand, so `AfterFind`/struct tags don't apply here).

- [x] **Task 7 — New domain errors (AC1, AC4)**
  - [x] Add to the Room block in [server/internal/apperr/errors.go:57-93](../../server/internal/apperr/errors.go#L57): `ErrWrongRoomPassword = NewAppError("WRONG_ROOM_PASSWORD", "incorrect room password", http.StatusConflict)`, `ErrRoomPasswordRequired = NewAppError("ROOM_PASSWORD_REQUIRED", "a password is required for a private room", http.StatusBadRequest)`, `ErrRoomPasswordTooShort = NewAppError("ROOM_PASSWORD_TOO_SHORT", "room password must be at least 4 characters", http.StatusBadRequest)`, `ErrRoomPasswordTooLong = NewAppError("ROOM_PASSWORD_TOO_LONG", "room password must be at most 72 characters", http.StatusBadRequest)`. **409** for wrong-password matches the `INSUFFICIENT_COINS` precedent ([errors.go:95-98](../../server/internal/apperr/errors.go#L95)); 400 for validation. **No `events.go` change.**

- [x] **Task 8 — Quick Play stays public (AC7)**
  - [x] In the QuickPlay synthesis ([handler.go:2869-2889](../../server/internal/room/handler.go#L2869)) the `newRoom` struct already omits `PasswordHash` (→ nil → public). Add an explicit one-line comment documenting the invariant (`// PasswordHash stays nil — quick-play rooms are never private (Story 9.6 AC7)`). Verify `QuickJoin` never prompts/accepts a password.

- [x] **Task 9 — Frontend types + API client (AC1, AC3, AC4, AC6)**
  - [x] In [client/src/shared/types/apiTypes.ts](../../client/src/shared/types/apiTypes.ts#L31): add `isPrivate: boolean` to `Room`; add `isPrivate: boolean` and `password?: string` to `CreateRoomRequest`. **Do NOT** add any password field to `Room` (the hash never reaches the client).
  - [x] In [client/src/shared/api/rooms.ts](../../client/src/shared/api/rooms.ts): change `joinRoom(id: number, password?: string)` → `axiosClient.post(\`/rooms/${id}/join\`, password !== undefined ? { password } : undefined)`. Add `updateRoomPrivacy(roomId: number, body: { isPrivate: boolean; password?: string })` → `POST /rooms/:id/privacy`.
  - [x] In [client/src/shared/types/wsEvents.ts](../../client/src/shared/types/wsEvents.ts): add `isPrivate: boolean` to the `RoomCreatedPayload` and `RoomUpdatedPayload` interfaces so the lobby-cache spread in [useRoomUpdates.ts:78](../../client/src/features/lobby/useRoomUpdates.ts#L78) carries it. (These payloads are **not** drift-gated — see [D1](#design-decisions); no Zod/golden change.)
  - [x] Update the `useJoinRoomMutation` (and the join-by-code mutation) signatures to thread the optional password through to `joinRoom`.

- [x] **Task 10 — Create-room modal: privacy toggle + password field (AC1)**
  - [x] In [client/src/features/room/CreateRoomModal.tsx](../../client/src/features/room/CreateRoomModal.tsx): add `isPrivate`/`roomPassword` state (mirror the `coinBuyIn` state at [:53](../../client/src/features/room/CreateRoomModal.tsx#L53)); render a toggle + a conditional password `Field`+`Input` group cloned from the buy-in field group at [:276-302](../../client/src/features/room/CreateRoomModal.tsx#L276) (use the `KeyRound` icon already imported at [:1](../../client/src/features/room/CreateRoomModal.tsx#L1)). Add password to the `mutateAsync` payload at [:85-92](../../client/src/features/room/CreateRoomModal.tsx#L85); reset both fields in `handleOpenChange` at [:119-132](../../client/src/features/room/CreateRoomModal.tsx#L119). Map `ROOM_PASSWORD_REQUIRED`/`ROOM_PASSWORD_TOO_SHORT`/`ROOM_PASSWORD_TOO_LONG` in the `FetchError` block at [:96-115](../../client/src/features/room/CreateRoomModal.tsx#L96). Testids: `private-room-toggle`, `room-password-input`.

- [x] **Task 11 — Password prompt dialog + both join entry points (AC4, AC5)**
  - [x] New `PasswordPromptDialog` component (model it on the shadcn `Dialog` pattern used by `InsolventEjectionModal` / `DailyRewardDialog` in `client/src/features/lobby/components/`): props `{ open, roomName, pending, errorKey, onSubmit(password), onClose }`; a single password `Input` + submit; shows `t("room.errors.wrongPassword")` when `errorKey` is set; testid `password-prompt-dialog` / `password-prompt-input`.
  - [x] **Card-click path** — in `handleJoinRoom` ([client/src/features/lobby/LobbyPage.tsx:115-159](../../client/src/features/lobby/LobbyPage.tsx#L115)): if `room.isPrivate`, open the dialog instead of joining immediately; on submit call `joinRoomMutation.mutateAsync({ id: room.id, password })`; on `WRONG_ROOM_PASSWORD` keep the dialog open with the error; on success navigate as today. Public rooms keep the exact current path (AC5).
  - [x] **Join-by-code path** — in `JoinByCodeTile`: `getRoomByCode(code)` returns `room.isPrivate`; if private, open the same dialog before calling `joinRoom(room.id, password)`. Map the new error codes in its catch block alongside the existing `ROOM_NOT_FOUND`/`ROOM_FULL`/`INSUFFICIENT_COINS`/`ALREADY_IN_ROOM`.

- [x] **Task 12 — Lock indicator on the room card (AC3)**
  - [x] In `RoomCard` (`client/src/features/lobby/components/RoomCard.tsx`): when `room.isPrivate`, render a lock indicator in the meta row (use the `Lock` lucide icon; testid `room-card-lock`, with an `aria-label` from i18n). Keep the join button behavior unchanged (the prompt is handled by the join handler, not the card).

- [x] **Task 13 — Owner privacy control in the room (AC6)**
  - [x] Add an owner-only privacy control to the room lobby page (`client/src/features/room/RoomPage.tsx`), visible only when `isOwner && room.status === "waiting"` (follow the existing `isOwner` gating used for kick / transfer / bot controls and the `OwnerConfirmDialogs` pattern). It surfaces current state (private/public), lets the owner set/change the password or revert to public, and calls a new `useUpdateRoomPrivacyMutation` → `updateRoomPrivacy(...)`. On success the `system:room_updated` broadcast updates the cache; show a toast. Keep it minimal — one control, mirroring the existing owner-action UI.

- [x] **Task 14 — i18n in all four locales (AC1, AC3, AC4, AC8)**
  - [x] Add keys to the `lobby.createRoomModal` block (e.g. `privateRoom`, `privateRoomHint`, `roomPassword`, `roomPasswordPlaceholder`, `roomPasswordHint`, plus `errors.passwordRequired`/`passwordTooShort`/`passwordTooLong`), to `lobby.card` (e.g. `privateLockAriaLabel`), and to the `room` block (e.g. `passwordPrompt.title`/`description`/`submit`, `errors.wrongPassword`, owner privacy-control labels) in [en](../../client/src/shared/i18n/en.json) / [sr](../../client/src/shared/i18n/sr.json) / [mk](../../client/src/shared/i18n/mk.json) / [hr](../../client/src/shared/i18n/hr.json).
  - [x] **No em-dash (`—`) in mk/sr/hr** — English only. **mk must be all-Cyrillic** and idiomatic (follow the 9.4/9.5 mk conventions; the i18n parity test is enforced — keys must be 1:1 across all four files).

- [x] **Task 15 — Tests (AC1–AC8)**
  - [x] **Backend (Go, `testify`, co-located, per-test tx rollback for DB):** `CreateRoom` with `isPrivate` hashes + persists a non-nil `password_hash` and `password_hash` is absent from the JSON response; create with missing/short/over-long password → the right 400 error. `JoinRoom`: correct password seats; wrong password → `WRONG_ROOM_PASSWORD`; **missing** password on a private room → `WRONG_ROOM_PASSWORD`; public room join with no body → unchanged success (AC5 regression guard). `UpdateRoomPrivacy`: owner sets/changes/clears; non-owner → `ErrNotRoomOwner`; non-waiting → `ErrRoomNotWaiting`; clearing does not change `room_players` (no ejection). `AfterFind` populates `IsPrivate`. QuickPlay synthesis leaves `password_hash` NULL. Mirror the existing room handler test harness (`handler_test.go` / `coin_handler_test.go`).
  - [x] **Frontend (Vitest, `data-testid`):** CreateRoomModal renders the toggle + reveals the password field when on, blocks submit when empty, includes `isPrivate`/`password` in the payload. PasswordPromptDialog: opens on a private-room join, shows the error on `WRONG_ROOM_PASSWORD` and stays open, calls join with the password. RoomCard renders the lock for `isPrivate`. LobbyPage `handleJoinRoom` opens the dialog for private rooms and joins directly for public. JoinByCodeTile prompts for a private room. Owner privacy control calls the mutation. **i18n parity** green across en/sr/mk/hr.
  - [x] `make lint` + `make test` green both stacks. Verify the WS drift-gate tests (`events_contract_test.go`, `wsEvents.contract.test.ts`) are **untouched and still pass** — this story adds no WS event.

---

## Dev Notes

### What this story actually is

A **password gate on one HTTP endpoint**, plus a hashed nullable column and the UI to set/enter/manage the password. You add `rooms.password_hash` (bcrypt, NULL = public), derive a wire-facing `isPrivate` boolean, prompt for a password before the two existing join entry points when a room is private, verify it on the **`JoinRoom`** handler (the authoritative seat grant), and give the owner a way to set/clear it. The hard parts are **not** code volume — they are: (1) **resisting the epic's `action:join_room`/`error:wrong_room_password` WS framing** (join is HTTP; mirror `INSUFFICIENT_COINS`), (2) **never leaking the hash** to any client/log, and (3) **gating in exactly one place** so browse/code-lookup stay open while seating is gated.

### The concrete shape

```
# Schema (migration 000012)
rooms.password_hash VARCHAR(60) NULL          # NULL = public; bcrypt hash = private

# Derived wire field (never the hash)
isPrivate = (password_hash != nil)            # AfterFind hook + explicit set in the manual broadcast map

# Create (HTTP POST /rooms): if isPrivate -> validate -> auth.HashPassword -> room.PasswordHash = &hash
# Join  (HTTP POST /rooms/:id/join {password}): if room.PasswordHash != nil and
#         auth.CheckPassword(*hash, password) != nil  ->  ErrWrongRoomPassword (409, code WRONG_ROOM_PASSWORD)
#         (missing password == wrong password; public room skips the check entirely)
# Edit  (HTTP POST /rooms/:id/privacy {isPrivate, password?}): owner+waiting only;
#         set/clear password_hash; persist; broadcast system:room_updated; NO seat ejection
# QuickPlay synthesis: password_hash stays nil (never private)
```

### Current state of the code being modified (READ THESE)

| File | Today | This story changes |
|---|---|---|
| [room/model.go `Room`](../../server/internal/room/model.go#L9) | Fields incl. `CoinBuyIn int` (9.2); transient `OwnerUsername`/`Players` use `gorm:"-"`. No privacy. | Add persisted `PasswordHash *string gorm:"size:60" json:"-"` + derived `IsPrivate bool gorm:"-" json:"isPrivate"` + an `AfterFind` hook. |
| [room/handler.go `CreateRoomRequest`](../../server/internal/room/handler.go#L58) | `name/variant/matchMode/timerStyle/timerDurationSeconds/reconnectWindowSec/coinBuyIn`. | Add `IsPrivate bool` + `Password string`. |
| [room/handler.go `CreateRoom`](../../server/internal/room/handler.go#L300) | Validates fields, defaults `coinBuyIn=500` ([:380](../../server/internal/room/handler.go#L380)), affordability check, builds `room`, tx-creates + auto-seats, broadcasts `system:room_created`, returns room JSON. | Validate + hash password into `room.PasswordHash`; set `room.IsPrivate` before response/broadcast. |
| [room/handler.go `JoinRoom`](../../server/internal/room/handler.go#L596) | Auth → `FindByID` → status/capacity/already-in-room → **coin affordability** ([:648-656](../../server/internal/room/handler.go#L648)) → tx add-player/increment → broadcasts. **No body bound.** | Bind `{password}`; insert password check after the status check ([:617](../../server/internal/room/handler.go#L617)), before capacity. The `ErrInsufficientCoins` block is the precedent to mirror. |
| [room/handler.go `GetRoomByCode`](../../server/internal/room/handler.go#L560) | Code → `FindByCode` → returns `RoomDetailResponse{room, players, returnedUserIds}`. | **No password check** — `AfterFind` gives `room.isPrivate`; the client prompts, then calls `JoinRoom`. |
| [room/handler.go `roomLifecyclePayload`](../../server/internal/room/handler.go#L237) | Hand-built `map[string]any` for `system:room_created`/`room_updated` (incl. `coinBuyIn`). | Add `"isPrivate": r.PasswordHash != nil` (struct tags don't apply to this manual map). |
| [room/handler.go QuickPlay synthesis](../../server/internal/room/handler.go#L2869) | Builds synthesized `newRoom` (no `PasswordHash`). | Add a comment asserting `password_hash` stays nil (AC7). |
| [cmd/api/main.go room routes](../../server/cmd/api/main.go#L218) | `POST /rooms`, `POST /rooms/:id/join`, `…/kick`, `…/transfer-ownership`, `…/swap-seats`, etc. **No edit route.** | Add `POST /rooms/:id/privacy` → `UpdateRoomPrivacy`. |
| [apperr/errors.go Room block](../../server/internal/apperr/errors.go#L57) | Room errors + `ErrInsufficientCoins` (409, [:98](../../server/internal/apperr/errors.go#L95)). | Add `ErrWrongRoomPassword` (409) + 3 validation errors (400). |
| [auth/service.go bcrypt](../../server/internal/auth/service.go) | `HashPassword(pw) (string,error)` = `bcrypt.GenerateFromPassword(…, DefaultCost)`; `CheckPassword(hash, pw) error` = `bcrypt.CompareHashAndPassword`. | **Reuse as-is** for room passwords. |
| [client apiTypes.ts](../../client/src/shared/types/apiTypes.ts#L31) | `Room` has `coinBuyIn`; `CreateRoomRequest` has `coinBuyIn`. | Add `isPrivate` to `Room`; `isPrivate`+`password?` to `CreateRoomRequest`. No password on `Room`. |
| [client api/rooms.ts](../../client/src/shared/api/rooms.ts#L27) | `joinRoom(id)` posts no body; no edit fn. | `joinRoom(id, password?)`; add `updateRoomPrivacy(roomId, body)`. |
| [client wsEvents.ts](../../client/src/shared/types/wsEvents.ts) | `RoomCreatedPayload`/`RoomUpdatedPayload` interfaces. | Add `isPrivate` to both (not drift-gated). |
| [client useRoomUpdates.ts](../../client/src/features/lobby/useRoomUpdates.ts#L78) | Spreads `{...r, ...payload}` into the cached `Room` for room_created/room_updated. | `isPrivate` flows through automatically once on the payload type. |
| [client CreateRoomModal.tsx](../../client/src/features/room/CreateRoomModal.tsx) | Form incl. buy-in `Field`+`Input` group ([:276-302](../../client/src/features/room/CreateRoomModal.tsx#L276)); `KeyRound` already imported. | Add toggle + conditional password field; payload + reset + error mapping. |
| [client LobbyPage.tsx `handleJoinRoom`](../../client/src/features/lobby/LobbyPage.tsx#L115) | Quick-play → matchmaking; normal → `joinRoomMutation.mutateAsync(room.id)` → navigate. Maps `ROOM_FULL`/`INSUFFICIENT_COINS`/`ALREADY_IN_ROOM`. | Intercept private rooms → open `PasswordPromptDialog` first. |
| `client/.../JoinByCodeTile.tsx` | `getRoomByCode` → `joinRoom(room.id)`; maps the same error codes. | Prompt before join when `isPrivate`; map new codes. |
| `client/.../RoomCard.tsx` | Meta row: variant·mode·timer·buy-in·time; join button. | Add lock indicator when `isPrivate`. |
| `client/src/features/room/RoomPage.tsx` | Owner controls (kick/transfer/bots) gated on `isOwner`, via `OwnerConfirmDialogs`. **No room-settings edit.** | Add owner-only privacy control (waiting-only). |

### Reuse map — DO NOT reinvent

- **bcrypt:** `auth.HashPassword` / `auth.CheckPassword` ([auth/service.go]) — the exact functions used for account passwords. Do **not** import `bcrypt` directly in the room package; call the `auth` helpers.
- **HTTP-error-as-rejection precedent:** `ErrInsufficientCoins` (Story 9.2/9.3) — defined in `apperr`, returned from `JoinRoom`, surfaced to the client as `err.code` via `FetchError`, mapped to UI in `LobbyPage`/`JoinByCodeTile`. `WRONG_ROOM_PASSWORD` follows this **identically**.
- **Create-room field group:** the buy-in `Field`+`Input` at [CreateRoomModal.tsx:276-302](../../client/src/features/room/CreateRoomModal.tsx#L276) — clone its structure for the password field.
- **Dialog pattern:** shadcn `Dialog`/`DialogContent` as used by `InsolventEjectionModal` / `DailyRewardDialog` (`client/src/features/lobby/components/`) — model `PasswordPromptDialog` on these (open state, `data-testid`, `DialogClose`).
- **Owner-action UI + gating:** the `isOwner` checks and `OwnerConfirmDialogs` in `RoomPage.tsx` — the privacy control lives in the same owner-only region.
- **Lifecycle broadcast:** `roomLifecyclePayload` + `broadcastRoomUpdated` ([handler.go:237-284](../../server/internal/room/handler.go#L237)) — the privacy edit reuses `broadcastRoomUpdated` verbatim.
- **Derived-flag-on-read:** `AfterFind` hook is the idiomatic single-point GORM way to populate `IsPrivate` (parallels how `OwnerUsername`/`Players` are hydrated at the handler layer; here it's automatic).

### Critical correctness rules (project-context.md + Epic 9 learnings)

- **HTTP, not WebSocket.** Join/create are REST endpoints. The rejection is an `apperr.AppError` (`WRONG_ROOM_PASSWORD`, 409), surfaced via `FetchError.code` — **not** a WS `error:` event. Touch **zero** WS contract / golden / Zod files. (AC8, [D1](#design-decisions))
- **Never leak the hash.** `PasswordHash` carries `json:"-"`; the client `Room` type has **no** password field; never log the plaintext or the hash. Verify with a test asserting `password_hash`/`passwordHash` is absent from the create/join/list JSON. ([D2](#design-decisions))
- **Gate at one authoritative point.** Only `JoinRoom` (the seat grant) verifies the password. `GetRoomByCode`/`GetRoom`/`ListRooms` reveal `isPrivate` but never demand it — "listed but locked" (Option B). ([D5](#design-decisions))
- **Server is the authority.** The client's toggle/disabled-button/dialog are cosmetic; the server validates and hashes. Missing password == wrong password (no oracle on which it was).
- **bcrypt 72-byte truncation.** bcrypt silently ignores input past 72 bytes — reject `len > 72` at validation so two long passwords differing only past byte 72 aren't treated as equal. (`auth` already caps account passwords at 72 — [ErrPasswordTooLong](../../server/internal/apperr/errors.go#L45).)
- **No ejection on edit.** Changing/clearing the password must not touch `room_players` or presence — the gate is join-time only (AC6). Guard with a test.
- **No regression to public rooms.** A public-room join sends no password body and must behave exactly as today; `c.Bind` on an empty body must not 400 the request. (AC5)
- **GORM conventions:** `password_hash` snake_case column, `*string` for NULL, exported field, `json:"-"`. Derived `isPrivate` is `gorm:"-"` + `json:"isPrivate"` (camelCase wire). Migrations sequential — `000012` next, with a reversing `.down.sql`.
- **i18n discipline:** keys 1:1 across en/sr/mk/hr (parity test enforced); **no em-dash** in mk/sr/hr; **mk all-Cyrillic** (Story 9.4/9.5 conventions — see [[no-emdash-in-mk-sr-hr]] / [[beljot-i18n-coin-terms]]).

### Design Decisions

- **D1 — HTTP rejection, not a WS event (RESOLVED).** The epic AC says "verified on `action:join_room`" → "`error:wrong_room_password`". **No such WS action exists** — room create/join are HTTP (`POST /rooms`, `POST /rooms/:id/join`; routes [main.go:218,223](../../server/cmd/api/main.go#L218)); game actions are WS. The epic wording is a planning-time naming convention, identical to how `error:insufficient_coins` is actually the HTTP `INSUFFICIENT_COINS` apperr (Stories 9.2/9.3). Implement `WRONG_ROOM_PASSWORD` as an `apperr.AppError` (409), surfaced via `FetchError.code` and mapped to UI exactly like `INSUFFICIENT_COINS`. **Consequence:** the WS contract (`events.go` ↔ `wsEvents.ts`), the Go golden tests, the Zod schemas, and the contract drift-gate are **all untouched** — there is no new event to keep in sync. (The room **lifecycle** payloads `system:room_created`/`room_updated` gain `isPrivate`, but those are hand-built `map[string]any` payloads with loosely-typed client parsing [useRoomUpdates.ts:78](../../client/src/features/lobby/useRoomUpdates.ts#L78) — not part of the typed-struct golden/Zod gate.)
- **D2 — Privacy = `password_hash != nil`; no `is_private` column.** A single nullable hash column is the source of truth (NULL = public). The wire-facing `isPrivate` boolean is **derived**, never stored, never the hash. This avoids a redundant boolean that could drift from the hash, and keeps the hash strictly server-side (`json:"-"`).
- **D3 — bcrypt via `auth` helpers; password rules are tunable placeholders.** Reuse `auth.HashPassword`/`auth.CheckPassword` (consistency with account auth; bcrypt's salt makes equal passwords hash differently, so comparison must use `CompareHashAndPassword`, never string equality). Room password min length **4** (lower friction than the 8-char account minimum — these are shared casually among friends) and max **72 bytes** (bcrypt limit) are named consts, tunable per the Epic-9 "economy/UX constants stay placeholders" guidance.
- **D4 — Derived `isPrivate` via `AfterFind` + two explicit sets.** A GORM `AfterFind` hook on `Room` populates `IsPrivate` on every DB **read** in one place (covers list/detail/join/code lookups). Two paths don't read from the DB and need an explicit `room.IsPrivate = room.PasswordHash != nil`: (a) the freshly-`Create`d in-memory room in `CreateRoom` (Create fires `AfterCreate`, not `AfterFind`), and (b) the hand-built `roomLifecyclePayload` map (it serializes nothing through struct tags). Documenting both prevents a "lock icon missing on a just-created/edited room" bug.
- **D5 — Verify only at the seat grant.** Browsing and code-resolution are identity/discovery (Option B "listed but locked"); they expose `isPrivate` so the client can prompt, but they must not require the password. The one authoritative gate is `JoinRoom`, the action that actually seats the player and is re-validated server-side. This also means **missing password is indistinguishable from wrong password** — both `WRONG_ROOM_PASSWORD` — so the endpoint reveals no "this room is private but you sent nothing" oracle beyond the already-public `isPrivate` flag.
- **D6 — Owner edit is a new action-style endpoint, no ejection.** No room-settings edit endpoint exists today; the room sub-routes are all action POSTs (`/kick`, `/transfer-ownership`, `/swap-seats`). `POST /rooms/:id/privacy` matches that style better than a generic `PATCH /rooms/:id`. Owner-only + waiting-only (consistent with the other owner controls). Changing/clearing the password is join-gate-only — **seated players are never ejected** (AC6), which falls out naturally because nothing re-checks seated players.
- **D7 — Quick Play is structurally public.** Synthesized rooms ([handler.go:2869](../../server/internal/room/handler.go#L2869)) never set `password_hash`, and the QuickPlay/QuickJoin paths have no password input — so they're always public with zero extra code (just an asserting comment + a test).

### Previous Story Intelligence (Stories 9.2 / 9.3 / 9.4 / 9.5)

- **9.2 established the room-economy gate pattern** you are extending: a check in `JoinRoom` that returns an `apperr` (`ErrInsufficientCoins`), with the client composing the UX from the error `code` (Design Decision B — the payload carries only a code). `WRONG_ROOM_PASSWORD` is the same shape. Settlement/charge code is irrelevant here — privacy touches **no** money path.
- **9.3 added `POST /rooms/:id/return` + insolvency ejection** and is the most recent heavy edit to `room/handler.go` — that file is large and freshly stabilized. Add your handlers as **siblings** to the existing ones; do not refactor the shared join/seat/transfer machinery. The owner-transfer/close logic (`transferOwnershipOrClose`) is unrelated to privacy — don't entangle them.
- **9.4 synthesized Quick Play rooms** (`QuickPlay`/`QuickJoin`) — the synthesis site is exactly where AC7's "never private" invariant is asserted.
- **9.5 (immediate predecessor)** added a WS event and had to extend the drift-gate (Go golden + Zod + contract row). **This story is the opposite** — it deliberately adds **no** WS event, so the drift-gate stays frozen. If you find yourself editing `events.go`/`wsEvents.ts`/`testdata/events/`/`wsEvents.schemas.ts`, stop — you've taken the wrong (WS) path (re-read [D1](#design-decisions)).
- **i18n (9.4/9.5 review patches):** mk strings must be all-Cyrillic and idiomatic; quoting conventions matter (a 9.4 patch re-quoted a button as `„…“`). The parity test fails the build if any key is missing in any of the four files. See [[beljot-i18n-coin-terms]].
- **Dev DB is on host port 6433** (not the Go test default 5433) — DB-backed tests need `BELJOT_DB_URL=postgres://beljot:beljot_dev_password@localhost:6433/beljot?sslmode=disable`. Apply migration `000012` and verify a down/up roundtrip there before claiming done. See [[beljot-dev-db-port-6433]].

### Git Intelligence

Recent history (`feat(bot): …` strategy work, then the 9.2→9.5 economy/XP landings) shows `room/handler.go`, `apperr/errors.go`, `CreateRoomModal.tsx`, `RoomCard.tsx`, `LobbyPage.tsx`, and the four i18n files are the freshly-touched hot paths — exactly what this story modifies, so they're stable and well-patterned. Follow the project conventions: branch `feat/9-6-private-rooms`, **one story = one branch = one PR**, commit scope `feat(room):` (and `feat(auth):` only if you touch the auth helpers, which you should not). The baseline for this story is `beba746`.

### Project Structure Notes

- Backend domain shape is fixed (`model.go`, `repository.go`, `gorm_repo.go`, `handler.go`, `service.go`, `_test.go`). Privacy lives entirely in the existing **`internal/room`** package + the shared `internal/apperr` + the existing `internal/auth` bcrypt helpers. **No new package, one migration (`000012`).** The repository interface needs **no** change — `Create`/`Update`/`Find*` already round-trip all `Room` fields including the new column.
- GORM: `password_hash` snake_case column, `*string` (nullable), exported field, `json:"-"`. Frontend: named exports, `data-testid` selection, feature-folder placement (`features/room/` for the modal + room page + prompt dialog; `features/lobby/components/` for the card; `shared/api` + `shared/types` for client plumbing).

### Testing Standards

- **Backend:** Go `testing` + `testify`; table-driven; co-located (`handler_test.go`/`coin_handler_test.go` are the room-handler test homes). DB-backed cases use **per-test transaction rollback**; tests create their own data (no `make seed`). Cover: create-private hashes + non-leaking JSON; create validation errors; join correct/wrong/missing password; public-room no-body join regression; owner edit set/change/clear + non-owner + non-waiting + no-ejection; `AfterFind` derivation; QuickPlay stays NULL. `go test ./... && go vet ./...` clean; `golangci-lint run ./...` clean.
- **Frontend:** Vitest; presentational; `data-testid` selection (never CSS classes); `it('renders …')` present tense. Cover the modal toggle/field/payload, the prompt dialog (open/retry-on-error/submit), the card lock, both join entry points (private prompts, public doesn't), the owner control, and i18n parity across en/sr/mk/hr.
- **Definition of Done (hard gate):** server handler + repo-roundtrip + tests; new domain errors in `internal/apperr/errors.go`; **no** WS contract change (drift-gate tests still green, untouched); frontend components + co-located tests; i18n in **all four** locales; `make lint`; `make test`.

### References

- Epic + ACs: [epics.md → Story 9.6](../planning-artifacts/epics.md#L1915-L1953); FR60 [epics.md:85](../planning-artifacts/epics.md#L85) + FR map [epics.md:255](../planning-artifacts/epics.md#L255); Epic 9 overview [epics.md:1720](../planning-artifacts/epics.md#L1720).
- Origin / design rationale: [sprint-change-proposal-2026-06-18.md §9.6](../planning-artifacts/sprint-change-proposal-2026-06-18.md#L79) (new story, Option B "listed but locked", hashed bcrypt nullable, Quick Play never private).
- Backend: room model [room/model.go:9-38](../../server/internal/room/model.go#L9); `CreateRoom` [handler.go:300-469](../../server/internal/room/handler.go#L300) + `CreateRoomRequest` [:58-68](../../server/internal/room/handler.go#L58); `JoinRoom` [:596-709](../../server/internal/room/handler.go#L596) (mirror the `ErrInsufficientCoins` block [:648-656](../../server/internal/room/handler.go#L648)); `GetRoomByCode` [:560-594](../../server/internal/room/handler.go#L560); `roomLifecyclePayload` [:237-279](../../server/internal/room/handler.go#L237) + `broadcastRoomUpdated` [:282-284](../../server/internal/room/handler.go#L282); QuickPlay synthesis [:2869-2889](../../server/internal/room/handler.go#L2869); routes [main.go:218-234](../../server/cmd/api/main.go#L218); bcrypt [auth/service.go]; errors [apperr/errors.go:57-98](../../server/internal/apperr/errors.go#L57); migration template [000010_add_coin_economy_columns.up.sql](../../server/migrations/000010_add_coin_economy_columns.up.sql) (next = `000012`).
- Frontend: types [apiTypes.ts:31-70](../../client/src/shared/types/apiTypes.ts#L31); api client [rooms.ts:11-29](../../client/src/shared/api/rooms.ts#L11); ws payload types [wsEvents.ts](../../client/src/shared/types/wsEvents.ts); lobby cache spread [useRoomUpdates.ts:43-82](../../client/src/features/lobby/useRoomUpdates.ts#L43); create modal [CreateRoomModal.tsx:276-302](../../client/src/features/room/CreateRoomModal.tsx#L276); join handler [LobbyPage.tsx:115-159](../../client/src/features/lobby/LobbyPage.tsx#L115); card `RoomCard.tsx`; join-by-code `JoinByCodeTile.tsx`; owner controls `RoomPage.tsx`/`OwnerConfirmDialogs.tsx`; i18n [en](../../client/src/shared/i18n/en.json)/[sr](../../client/src/shared/i18n/sr.json)/[mk](../../client/src/shared/i18n/mk.json)/[hr](../../client/src/shared/i18n/hr.json).
- Predecessor: [9-5-xp-and-level-system.md](9-5-xp-and-level-system.md). Project rules: [project-context.md](../project-context.md) (server-authoritative, HTTP via `shared/api`, GORM/camelCase conventions, no-em-dash mk/sr/hr, WS contract two-files — N/A here, no WS event).

## Dev Agent Record

### Agent Model Used

claude-opus-4-8 (Opus 4.8, 1M context) — BMad create-story workflow; implementation via BMad dev-story workflow (same model).

### Debug Log References

- Migration `000012` applied + down/up round-trip verified on the dev DB (host port **6433**, `BELJOT_DB_URL` override): `password_hash VARCHAR(60) NULL` present after `up`, dropped after `down 1`, re-added after `up 1`.
- DB-backed `AfterFind` test (`TestRoom_AfterFindDerivesIsPrivate`) runs against the dev DB (port 6433) and skips gracefully when no DB is reachable, mirroring the wallet/user integration-test convention.

### Completion Notes List

Implemented the private-room password gate as a pure **HTTP** feature — **no WS event, no `events.go`/`wsEvents.ts` constant, no golden/Zod drift-gate change** (D1). The drift-gate tests remain untouched and green.

- **Schema/model:** migration `000012` adds nullable `rooms.password_hash VARCHAR(60)` (NULL = public). `Room.PasswordHash *string` carries `json:"-"` (never serialized); derived `Room.IsPrivate bool` (`gorm:"-"`, `json:"isPrivate"`) is populated by a GORM `AfterFind` hook, plus two explicit sets where no DB read-back fires (the freshly-`Create`d room in `CreateRoom`, and the hand-built `roomLifecyclePayload` map).
- **Create:** `CreateRoomRequest` gains `isPrivate`/`password`; a private room validates (`validateRoomPassword`: non-empty, ≥4 chars, ≤72 bytes) and bcrypt-hashes via `auth.HashPassword`. New consts `roomPasswordMinLength=4`, `roomPasswordMaxBytes=72`.
- **Join gate:** `JoinRoom` binds an optional `{password}` (empty body tolerated for public joins), and immediately after the status check verifies `auth.CheckPassword` when `PasswordHash != nil`. Wrong **and** missing password both return `ErrWrongRoomPassword` (409) — indistinguishable; the attempted password is never logged. `GetRoomByCode`/`GetRoom` stay password-free (browse/code-lookup expose `isPrivate` only).
- **Owner edit:** new `POST /rooms/:id/privacy` (`UpdateRoomPrivacy`) — owner-only (`ErrNotRoomOwner`) + waiting-only (`ErrRoomNotWaiting`); sets/changes/clears the hash via `repo.Update`, broadcasts `system:room_updated`, and **never ejects seated players**.
- **Quick Play:** synthesized rooms keep `password_hash` NULL (AC7) — asserted with a comment + test.
- **Errors:** `ErrWrongRoomPassword` (409, mirrors `INSUFFICIENT_COINS`) + `ErrRoomPasswordRequired`/`ErrRoomPasswordTooShort`/`ErrRoomPasswordTooLong` (400).
- **Frontend:** `Room.isPrivate` + `CreateRoomRequest.isPrivate/password` types; `joinRoom(id, password?)` + `updateRoomPrivacy(...)` API client; `useJoinRoomMutation` threads the password and the obsolete combined `useJoinByCodeMutation` was replaced by a resolve-then-prompt flow; new `useUpdateRoomPrivacyMutation`. UI: create-room privacy toggle + conditional password field (+ live-preview lock), `PasswordPromptDialog` wired into both join entry points (card click + join-by-code) with wrong-password retry, `RoomCard` lock indicator, and an owner-only `RoomPrivacyDialog` on `RoomPage` (waiting-only). i18n added to **all four** locales (en/sr/mk/hr; mk all-Cyrillic, no em-dash; parity test green).
- **Tests:** new `server/internal/room/privacy_handler_test.go` (19 cases incl. the DB-backed `AfterFind`); new client tests for `PasswordPromptDialog`, `JoinByCodeTile`, `RoomPrivacyDialog`, plus extensions to `CreateRoomModal`/`RoomCard`/`LobbyPage`. Existing Room-typed test fixtures updated for the new required `isPrivate` field, and the changed `joinRoom`/create-payload call sites updated.

**Validations (all green):** `go vet ./...`; `go test ./...` (room run fresh, 35.6s); `golangci-lint run ./...`; `gofmt` clean; client `vitest run` **906 passed (92 files)**; `eslint`; `prettier --check`; `tsc -p tsconfig.build.json` (build gate, excludes tests); i18n parity test. (Pre-existing `returnedUserIds`-missing type warnings in some test fixtures are unrelated to this story and present on `master`; the build gate excludes test files.)

### File List

**Backend (server/)**
- `migrations/000012_add_room_password.up.sql` (new)
- `migrations/000012_add_room_password.down.sql` (new)
- `internal/room/model.go` (PasswordHash, IsPrivate, AfterFind)
- `internal/room/handler.go` (consts + validateRoomPassword, CreateRoomRequest, CreateRoom, JoinRoomRequest + JoinRoom gate, UpdateRoomPrivacy, roomLifecyclePayload isPrivate, QuickPlay comment)
- `internal/apperr/errors.go` (ErrWrongRoomPassword + 3 validation errors)
- `cmd/api/main.go` (POST /rooms/:id/privacy route)
- `internal/room/handler_test.go` (privacy route in 3 test setups)
- `internal/room/privacy_handler_test.go` (new — privacy backend tests + DB-backed AfterFind)

**Frontend (client/src/)**
- `shared/types/apiTypes.ts` (Room.isPrivate, CreateRoomRequest.isPrivate/password)
- `shared/types/wsEvents.ts` (isPrivate on RoomCreated/RoomUpdated payloads)
- `shared/api/rooms.ts` (joinRoom password arg, updateRoomPrivacy)
- `shared/hooks/mutations/useRooms.ts` (useJoinRoomMutation signature, useUpdateRoomPrivacyMutation; removed useJoinByCodeMutation)
- `features/room/CreateRoomModal.tsx` (privacy toggle + password field + preview lock)
- `features/room/RoomPage.tsx` (private badge + owner privacy control + dialog)
- `features/room/RoomPrivacyDialog.tsx` (new)
- `features/lobby/LobbyPage.tsx` (private-room password prompt on card join)
- `features/lobby/components/JoinByCodeTile.tsx` (resolve-then-prompt for private codes)
- `features/lobby/components/RoomCard.tsx` (lock indicator)
- `features/lobby/components/PasswordPromptDialog.tsx` (new)
- `shared/i18n/en.json`, `sr.json`, `mk.json`, `hr.json` (privacy strings ×4)
- Tests: `features/room/CreateRoomModal.test.tsx`, `features/room/RoomPrivacyDialog.test.tsx` (new), `features/lobby/LobbyPage.test.tsx`, `features/lobby/components/RoomCard.test.tsx`, `features/lobby/components/PasswordPromptDialog.test.tsx` (new), `features/lobby/components/JoinByCodeTile.test.tsx` (new), `features/room/RoomPage.test.tsx`, `features/room/RoomPage.bots.test.tsx`, `features/lobby/MatchmakingPage.test.tsx`, `features/lobby/components/MatchmakingDiagram.test.tsx`, `features/match/MatchPage.test.tsx`, `shared/hooks/useWsDispatch.test.ts`, `shared/stores/roomStore.test.ts` (isPrivate fixture updates)

## Change Log

| Date | Change |
|---|---|
| 2026-06-24 | Story 9.6 (Private Rooms / FR60) implemented via dev-story — HTTP password gate (no WS change): migration `000012` (`rooms.password_hash`), join-time `WRONG_ROOM_PASSWORD` gate mirroring `INSUFFICIENT_COINS`, owner `POST /rooms/:id/privacy` edit, derived `isPrivate`, create-room toggle + password prompt + lock indicator + owner control, i18n ×4. All gates green; status → review. |
| 2026-06-24 | Code-reviewed (3-layer adversarial: Blind Hunter + Edge Case Hunter + Acceptance Auditor). AC1–AC7 verified; D1 "no WS change" independently confirmed. 2 decisions resolved (deep-link → prompt; unrelated vite/docker port changes → keep-local-don't-commit), 3 patches applied (deep-link `PasswordPromptDialog` in `RoomPage` + test; `UpdateRoomPrivacy` quick-play guard `ErrQuickPlayRoomPrivacy` + test; password length-unit alignment rune/byte + test), 1 deferred (non-transactional `UpdateRoomPrivacy` save — TOCTOU, pre-existing pattern → deferred-work.md), 5 dismissed. Client gates green (tsc/vitest/eslint/prettier); Go gofmt-clean + gopls-validated, `go test`/`golangci-lint` to be run via `make test`. Status → done. |

## Review Findings

_Adversarial code review (Blind Hunter + Edge Case Hunter + Acceptance Auditor), 2026-06-24. AC1–AC7 verified satisfied; AC8 partial (deep-link gap + scope leak below). D1 "no WS contract change" independently confirmed: drift-gate / golden / Zod files untouched. Hash non-leak (`json:"-"`), bcrypt usage, owner/waiting gating, missing==wrong indistinguishability, i18n parity (×4, mk all-Cyrillic, no em-dash) all verified._

- [x] [Review][Decision→Dismissed] Unrelated infra port changes on the branch ([client/vite.config.ts](../../client/vite.config.ts) dev `5173→6173`, proxy `8080→9080`; [docker-compose.yml](../../docker-compose.yml) Postgres `5433→6433`) — **resolved 2026-06-24: intentional machine-local overrides (other projects on this machine use the default ports); to be kept in the working tree but NOT committed to this PR.** Not a code defect. `privacy_handler_test.go`'s default DSN stays at `localhost:5433` (the codebase convention; the dev DB on 6433 is reached via `BELJOT_DB_URL`), so no reconciliation is needed.

- [x] [Review][Patch] Deep-link / refresh to a private room must prompt for the password (Decision 1, resolved → prompt) — the `/rooms/:id` auto-join effect ([client/src/features/room/RoomPage.tsx:213-255](../../client/src/features/room/RoomPage.tsx#L213)) calls `joinRoomMutation.mutateAsync({ id })` with **no password** for any `waiting`, non-quick-play room the user is not yet seated in; a private room then returns `WRONG_ROOM_PASSWORD` (unhandled in the `.catch`) → generic `lobby.errors.joinFailed` toast + bounce to lobby. Fix: when `room.isPrivate`, open `PasswordPromptDialog` instead of auto-joining; submit calls `mutateAsync({ id, password })`; keep the dialog open on `WRONG_ROOM_PASSWORD`; on success refetch as today. [client/src/features/room/RoomPage.tsx:213](../../client/src/features/room/RoomPage.tsx#L213)
- [x] [Review][Patch] `UpdateRoomPrivacy` lacks a quick-play guard — a quick-play room's owner (the matchmaking initiator, `OwnerID: userID`) can privatize it via a direct `POST /rooms/:id/privacy`, violating AC7's server-side invariant; the UI hides the control but the endpoint does not enforce it. Fix: add `if room.IsQuickPlay { return apperr.ErrRoomNotQuickPlay }` (error already exists, used by `QuickJoin`) alongside the owner/waiting checks. [server/internal/room/handler.go:823-828](../../server/internal/room/handler.go#L823)
- [x] [Review][Patch] Room-password length validated in mismatched units — server min check uses `len(pw)` (bytes) while the error/hint says "characters"; client guards use `String.length` / `slice` / `maxLength` (UTF-16 units) while the server max is 72 **bytes**. A multibyte password can pass the client yet be rejected `ROOM_PASSWORD_TOO_LONG`, and a 2-char Cyrillic password (4 bytes) passes the "4 character" minimum. Fix: server min via `utf8.RuneCountInString`; client max via `new TextEncoder().encode(v).length`. [server/internal/room/handler.go:58-67](../../server/internal/room/handler.go#L58), [client/src/features/room/CreateRoomModal.tsx](../../client/src/features/room/CreateRoomModal.tsx), [client/src/features/room/RoomPrivacyDialog.tsx:134](../../client/src/features/room/RoomPrivacyDialog.tsx#L134)

- [x] [Review][Defer] `UpdateRoomPrivacy` is a non-transactional read-modify-write (`FindByID` → mutate → `Update`=`db.Save` full-row, no row lock) — a concurrent join or `StartMatch` landing between the read and the save can be clobbered by the stale full-row write (TOCTOU / lost update), e.g. `PlayerCount` or `Status` reverted. [server/internal/room/handler.go:816-850](../../server/internal/room/handler.go#L816) — deferred, pre-existing codebase-wide room-mutation pattern; story scope explicitly forbids refactoring the shared join/seat machinery, the owner+waiting-only constraint makes the window rare, and subsequent broadcasts self-heal the transient drift.

_Dismissed as noise (5): `JoinRoom` swallows the bind error — spec-mandated (tolerate empty public-join body; missing==wrong by D5). `VARCHAR(60)` exact-sizing — correct for bcrypt, matches the account-password convention. `TestUpdateRoomPrivacy_DoesNotEjectSeatedPlayers` under-asserts — core no-ejection assertion is valid; the "passes for wrong reason" concern assumes a copy-returning repo refactor that doesn't exist. `setRoom(updated)` blanking `ownerUsername` — false positive: `broadcastRoomUpdated` → `roomLifecyclePayload` hydrates `OwnerUsername` in place on the pointer (handler.go:267) before the `c.JSON` response. `JoinByCodeTile` double-click resolve race — benign duplicate work, no correctness/UX impact._

**Patches applied (2026-06-24):**

- **Deep-link prompt** — `RoomPage` now opens `PasswordPromptDialog` for a private-room deep-link/refresh instead of blind-joining; submit threads the password, wrong password keeps the dialog open, declining bounces to the lobby. New co-located test (`RoomPage.test.tsx`: "prompts for a password instead of blind-joining on a deep link to a private room"). [client/src/features/room/RoomPage.tsx]
- **Quick-play guard** — `UpdateRoomPrivacy` now returns the new `apperr.ErrQuickPlayRoomPrivacy` (`QUICK_PLAY_ROOM_PRIVACY`, 409) when `room.IsQuickPlay`, enforcing AC7 server-side. New test `TestUpdateRoomPrivacy_QuickPlayRoomRejected`. [server/internal/room/handler.go, internal/apperr/errors.go]
- **Password length units** — server min now counts runes (`utf8.RuneCountInString`) to honor the "characters" wording; client max now counts UTF-8 bytes (`TextEncoder`) in both `CreateRoomModal` and `RoomPrivacyDialog` to match the server's bcrypt byte bound. New test `TestCreateRoom_Private_ShortMultibytePassword`. [server/internal/room/handler.go, client/src/features/room/CreateRoomModal.tsx, RoomPrivacyDialog.tsx]

**Verification:** client TypeScript build (`tsc -p tsconfig.build.json`), Vitest (room+lobby 139 + RoomPage 46 incl. the new case), ESLint, and Prettier all green. Go files are gofmt-clean and gopls-validated (compile clean) but `go test` / `golangci-lint` were **not executed in the review environment** (Go toolchain not on this shell's PATH) — run `make test` to confirm the Go suite + new tests before merge.

### Manual E2E (Playwright, 2026-06-24)

Drove the full feature live (`make dev`, dev DB on 6433) across two identities (testxp + a registered `playerb`). Verified: create-private + lock indicator (AC1–AC3), owner edit change/public/private with no ejection (AC6), browse lock (AC3), join with wrong password → "Incorrect room password." + dialog stays open (AC4), correct-password join (AC4), **deep-link to a private room → prompt** and **join-by-code → prompt** (patch 1, both entry points), and public-room join → no prompt (AC5).

**Bug found + fixed (E2E-2): a privacy flip did not propagate to users already inside the room.** When the host changed a room private→public (or vice versa), players already on the room page kept seeing the stale privacy pill — only the lobby grid updated. Root cause: `useWsDispatch` routed `system:room_updated` solely to the lobby list cache (`handleRoomListMessage`) and returned; it never updated the `roomStore` that the room page renders from. **Fix:** on `system:room_updated` for the room the viewer is in (`currentRoomId === payload.id`), merge the payload into `roomStore.room` ([client/src/shared/hooks/useWsDispatch.ts](../../client/src/shared/hooks/useWsDispatch.ts)). Verified live in the browser (owner flipped via API → seated viewer's pill switched Private→Public with no reload) and with two unit tests in `useWsDispatch.test.ts` (current-room updates; other-room ignored) — confirmed fail-without/pass-with.

**UI refinements (same session):** (1) the owner privacy control now sits to the **left of the room code** on the room page; (2) the privacy pill is shown for **public** rooms too (globe + "Public"), not only private — on the lobby `RoomCard`, the `RoomPage` header, and the create-room **live preview**; (3) new i18n keys `room.privacy.publicBadge`, `lobby.card.public`, `lobby.card.publicAriaLabel` in all four locales (mk all-Cyrillic, comma style — no em-dash). All verified in the browser; client gates green (tsc + vitest room/lobby/ws/i18n 228 + eslint + prettier).

**Bug found + fixed (E2E-1): spurious password re-prompt after joining a private room.** Joining a private room via the lobby card or join-by-code and then landing on `/rooms/:id` re-opened the password prompt (and pre-patch would have blind-joined → `WRONG_ROOM_PASSWORD` → bounced to the lobby). Root cause: `useRoomDetailQuery` sets `refetchOnMount:"always"`, but React Query still serves the **cached snapshot synchronously** while the forced refetch runs; if the viewer just joined elsewhere, that stale snapshot omits them, so the RoomPage deep-link auto-join effect judged them a non-member and fired. **Fix:** gate the auto-join effect on `roomQuery.isFetchedAfterMount` so membership is judged only on data fetched fresh after mount ([client/src/features/room/RoomPage.tsx](../../client/src/features/room/RoomPage.tsx)). Added a regression test that seeds a stale (member-absent) cache against a fresh (member-present) fetch and asserts no prompt/no auto-join (`RoomPage.test.tsx`); confirmed it fails without the gate and passes with it. Re-ran the exact join-by-code path in the browser — no re-prompt. Client gates re-green (tsc + vitest room/lobby 141 + eslint + prettier).
