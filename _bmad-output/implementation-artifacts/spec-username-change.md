---
title: 'Change Username from Profile Page'
type: 'feature'
created: '2026-07-10'
status: 'done'
review_loop_iteration: 0
baseline_commit: '512db3fd5f1bc7eb5a9ec8870a0b85675a7e5f4b'
context:
  - '{project-root}/_bmad-output/project-context.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Users cannot change their username after registration. The profile page shows the username as static text with no way to edit it.

**Approach:** Add inline edit-in-place to the profile identity header (pencil icon → input + confirm/cancel), backed by a new self-only `PATCH /users/:id/username` endpoint. Enforce the same validation as registration, keep usernames unique (case-sensitive), and rate-limit changes to once every 30 days via a new `username_changed_at` column.

## Boundaries & Constraints

**Always:**
- Validation identical to registration (trim, 3–20 chars, `^[a-zA-Z0-9_]+$`) via ONE extracted `user.ValidateUsername(raw) (string, error)`, called by both the register flow and the new handler.
- Server is authoritative for validation, uniqueness, and cooldown; client checks are UX-only. Endpoint is self-only (`paramID == authUserID`, else `ErrForbidden`).
- On success update BOTH `authStore.user.username` and the profile React Query cache, or `TopBar` (avatar initial, nav pill, "signed in as") goes stale.
- Cooldown length is server constant `UsernameChangeCooldownDays = 30`, mirrored client-side as a documented manual-sync pair (no shared type generation exists).
- New i18n keys in all four files; no em dash (`—`) in mk/sr/hr. New migration `000016` with a fully-reversing `.down.sql`.

**Ask First:**
- Changing the 30-day cooldown value, or dropping the cooldown entirely.
- Making uniqueness case-insensitive (currently case-sensitive to match registration/SSO).

**Never:**
- No email or password change in this scope.
- No public exposure of `walletBalance`/`loginStreakDays`/XP beyond the existing self-only profile DTO.
- Do not import `auth` from `user` (would cycle; `auth` already imports `user`).
- Do not reset the cooldown clock on a no-op (username unchanged).

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Valid change | Available name, outside cooldown, self | 200 `{data:{username, usernameChangedAt}}`; row updated, `username_changed_at=now` | N/A |
| Taken name | Name owned by another live user | No change | 409 `USERNAME_TAKEN` |
| Unique-index race | Passes pre-check, loses race on write | No change | 409 `USERNAME_TAKEN` (repo maps pg 23505 on `username`) |
| Too short/long | <3 or >20 chars | No change | 400 `USERNAME_TOO_SHORT` / `USERNAME_TOO_LONG` |
| Invalid chars | Contains non `[a-zA-Z0-9_]` | No change | 400 `USERNAME_INVALID_CHARS` |
| Unchanged | New == current (after trim) | No write, cooldown untouched | 400 `USERNAME_UNCHANGED` |
| Within cooldown | `username_changed_at` < 30 days ago | No change | 429 `USERNAME_CHANGE_TOO_SOON` |
| Foreign / no auth | `:id` != auth user / no token | No change | 403 `FORBIDDEN` / 401 `UNAUTHORIZED` |

</frozen-after-approval>

## Code Map

- `server/migrations/000016_add_username_changed_at_to_users.up.sql` / `.down.sql` -- new: nullable `username_changed_at TIMESTAMPTZ`
- `server/internal/user/model.go` -- add `UsernameChangedAt *time.Time` (mirror `LastLoginAt`)
- `server/internal/user/validate.go` -- new: `ValidateUsername`, `usernameRegex`, min/max consts, `UsernameChangeCooldownDays`
- `server/internal/auth/handler.go` -- refactor `validateRegisterRequest` username block to call `user.ValidateUsername`
- `server/internal/apperr/errors.go` -- add `ErrUsernameChangeTooSoon` (429), `ErrUsernameUnchanged` (400)
- `server/internal/user/repository.go` + `gorm_repo.go` -- add `UpdateUsername(id, username)` (sets username + username_changed_at; maps pg 23505 → `ErrUsernameTaken`)
- `server/internal/user/handler.go` -- add `UpdateUsername` handler; add `UsernameChangedAt` to `ProfileResponse`
- `server/cmd/api/main.go` -- register `api.PATCH("/users/:id/username", userHandler.UpdateUsername)`
- `server/internal/user/handler_test.go` -- add `UpdateUsername` to mock repo + route in test setup; add cases
- `client/src/shared/api/profile.ts` -- add `updateUsername`; add `usernameChangedAt?` to `ProfileResponse`
- `client/src/shared/lib/usernameChange.ts` -- new: cooldown constant + helpers (mirror of server)
- `client/src/features/profile/components/EditableUsername.tsx` -- new: inline-edit component (mutation + store/cache update)
- `client/src/features/profile/components/IdentityHero.tsx` -- render `EditableUsername` in place of static `<h1>`; add `userId`, `usernameChangedAt` props
- `client/src/features/profile/ProfilePage.tsx` -- pass `userId` + `usernameChangedAt` into `IdentityHero`
- `client/src/shared/i18n/{en,sr,mk,hr}.json` -- add `profile.editUsername.*`

## Tasks & Acceptance

**Execution:**
- [x] `server/migrations/000016_*.{up,down}.sql` -- add/drop nullable `username_changed_at TIMESTAMPTZ` on `users`
- [x] `server/internal/user/model.go` -- add `UsernameChangedAt *time.Time` with `column:username_changed_at` tag
- [x] `server/internal/user/validate.go` -- extract `ValidateUsername` (+ regex, consts, `UsernameChangeCooldownDays`); refactor `auth` register to use it
- [x] `server/internal/apperr/errors.go` -- add the two new errors
- [x] `server/internal/user/repository.go` + `gorm_repo.go` -- add `UpdateUsername`; map 23505 on `username` → `ErrUsernameTaken`
- [x] `server/internal/user/handler.go` -- `UpdateUsername` handler (self-only → validate → unchanged/cooldown/taken checks → repo); include `UsernameChangedAt` in `ProfileResponse`
- [x] `server/cmd/api/main.go` -- register the PATCH route
- [x] `server/internal/user/handler_test.go` -- cover all I/O Matrix rows (table-driven)
- [x] `client/src/shared/api/profile.ts` -- `updateUsername` + `usernameChangedAt` field
- [x] `client/src/shared/lib/usernameChange.ts` -- cooldown constant + `usernameChangeAvailableAt`/`isInCooldown` helpers
- [x] `client/src/features/profile/components/EditableUsername.tsx` (+ `.test.tsx`) -- inline edit; on success update authStore + profile cache; map 409/429/400 codes to inline messages; Enter=save, Escape=cancel
- [x] `client/src/features/profile/components/IdentityHero.tsx` -- integrate `EditableUsername`; update `IdentityHero.test.tsx`
- [x] `client/src/features/profile/ProfilePage.tsx` -- thread new props
- [x] `client/src/shared/i18n/{en,sr,mk,hr}.json` -- `profile.editUsername.*` (button/save/cancel labels, placeholder, per-error messages, cooldown-with-date, success)

**Acceptance Criteria:**
- Given I am on my own profile outside cooldown, when I click the pencil, edit to an available valid name, and press Enter/confirm, then the header, TopBar pill, and avatar initial all reflect the new name without a page reload and a success toast appears.
- Given I press Escape or click cancel while editing, then the input reverts to the current username with no request sent.
- Given my last change was under 30 days ago, when I view the profile, then the edit control is disabled with a hint stating when I can next change it, and a direct API attempt returns 429 `USERNAME_CHANGE_TOO_SOON`.
- Given I submit a name another user holds, then an inline "already taken" message shows and the displayed username is unchanged.
- Given `make lint` and `make test` run, then both stacks pass including the new Go and Vitest cases, and existing auth register tests still pass after the validator refactor.

## Design Notes

Handler order (short-circuit before the cooldown clock): self-auth → bind → `ValidateUsername` → load user → unchanged? `ErrUsernameUnchanged` → cooldown? `ErrUsernameChangeTooSoon` → taken? (`FindByUsername`, different id) `ErrUsernameTaken` → `repo.UpdateUsername` (which also maps the pg 23505 race to `ErrUsernameTaken`).

Client mirrors `LanguageSelector`'s optimistic-store pattern: on success `setUser({ ...user, username })` + `queryClient.setQueryData(queryKeys.profile.detail(userId), ...)`; revert both on error, detecting cases via `FetchError.code`. `IdentityHero` stays presentational for the avatar; only the title becomes smart. Use `Input`, `Button size="icon-sm"`, lucide `Pencil`/`Check`/`X`.

## Verification

**Commands:**
- `make test` -- expected: `go test ./...` + `npx vitest run` both green (new + existing)
- `make lint` -- expected: golangci-lint + ESLint/Prettier clean
- `make migrate` -- expected: `000016` applies and reverses cleanly

**Manual checks:**
- Run `make dev`, open your profile: pencil → edit → confirm updates the name everywhere live; retry immediately shows the disabled/cooldown state.

## Suggested Review Order

**Endpoint & check order (start here)**

- Self-only handler; ordered checks short-circuit before the cooldown clock.
  [`handler.go:408`](../../server/internal/user/handler.go#L408)

**Validation (single source of truth)**

- Extracted validator shared by register + change-username.
  [`validate.go:34`](../../server/internal/user/validate.go#L34)

- Register now delegates to it — no forked rules.
  [`handler.go:476`](../../server/internal/auth/handler.go#L476)

**Persistence & atomic cooldown**

- WHERE-clause cooldown makes concurrent changes race-safe; maps pg 23505 → taken.
  [`gorm_repo.go:129`](../../server/internal/user/gorm_repo.go#L129)

- Interface contract — returns the persisted stamp.
  [`repository.go:33`](../../server/internal/user/repository.go#L33)

**Schema & model**

- Nullable cooldown column (migration 000016).
  [`000016.up.sql:7`](../../server/migrations/000016_add_username_changed_at_to_users.up.sql#L7)

- Model field mirrors LastLoginAt (pointer → null).
  [`model.go:34`](../../server/internal/user/model.go#L34)

**Errors & route**

- New 429/400 errors for the cooldown + no-op cases.
  [`errors.go:79`](../../server/internal/apperr/errors.go#L79)

- Route registered alongside the other self-only user routes.
  [`main.go:139`](../../server/cmd/api/main.go#L139)

**Frontend edit-in-place**

- Inline editor: validation, cooldown gate, Enter/Escape, error mapping.
  [`EditableUsername.tsx:40`](../../client/src/features/profile/components/EditableUsername.tsx#L40)

- Swapped into the hero title in place of the static h1.
  [`IdentityHero.tsx:152`](../../client/src/features/profile/components/IdentityHero.tsx#L152)

- Props threaded from the page (self id + cooldown stamp).
  [`ProfilePage.tsx:62`](../../client/src/features/profile/ProfilePage.tsx#L62)

**Frontend data & cooldown**

- Mutation updates BOTH the profile cache and the auth store (TopBar).
  [`useProfile.ts:22`](../../client/src/shared/hooks/mutations/useProfile.ts#L22)

- Cooldown + validation mirror of the server (UX-only).
  [`usernameChange.ts:27`](../../client/src/shared/lib/usernameChange.ts#L27)

- API client + response type.
  [`profile.ts:48`](../../client/src/shared/api/profile.ts#L48)

**i18n**

- New editUsername keys (en; sr/mk/hr mirror, no em dash).
  [`en.json:41`](../../client/src/shared/i18n/en.json#L41)

**Tests (peripherals)**

- Backend table-driven cases incl. cooldown / taken / race.
  [`handler_test.go:759`](../../server/internal/user/handler_test.go#L759)

- Component interaction tests.
  [`EditableUsername.test.tsx:1`](../../client/src/features/profile/components/EditableUsername.test.tsx#L1)

- Pure cooldown/validation helpers.
  [`usernameChange.test.ts:1`](../../client/src/shared/lib/usernameChange.test.ts#L1)
