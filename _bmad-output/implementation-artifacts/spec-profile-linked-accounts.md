---
title: 'Profile linked accounts — view, link, and unlink SSO providers'
type: 'feature'
created: '2026-07-07'
status: 'done'
review_loop_iteration: 0
baseline_commit: '528fb41eb0c16715028331a91157194d318410cd'
context:
  - '{project-root}/_bmad-output/project-context.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Google SSO login/register and the `user_identities` infrastructure shipped, but a logged-in player has no way to see whether a Google account is linked, link one from inside the app, or unlink it. This was explicitly deferred from `spec-google-sso.md`.

**Approach:** Add three authenticated, self-only endpoints under `/api/v1/users/:id/identities` (list / link / unlink) served by the existing `AuthHandler` (it already holds `identityRepo`, `providers`, `userRepo`), and a "Linked accounts" panel on the profile page that reuses the Google Identity Services button and the controlled-dialog pattern. Linking is gated by the JWT (the user is already authenticated) — never by password, unlike the login-collision link flow.

## Boundaries & Constraints

**Always:**

- Self-only authorization on all three endpoints: mirror `GetProfile`'s 3-step guard (`auth.GetUserID` → parse `:id` → `paramID == authUserID` else `ErrForbidden`; 401/400/403).
- Reuse `provider.Verify`, `validateSSOCredential`, `normalizeSSOEmail`, and `ssoProvider(c)` from the auth package — handlers stay provider-agnostic (no `"google"` literals in flow logic).
- Link accepts any Google account the user proves ownership of (verified credential, `email_verified == true`). It does NOT require the Google email to match the account's email — the JWT already proves account ownership.
- Unlink guard: an account must always retain at least one login method. Block unlink (409 `SSO_CANNOT_UNLINK_LAST`) when the account is passwordless (`PasswordHash == ""`) and removing this identity would leave zero identities.
- List DTO exposes only `provider`, `email`, `createdAt` plus a top-level `hasPassword` — never `id`, `userId`, or `providerUserId`.
- New i18n keys in all four locales (en, hr, sr, mk — mk fully Cyrillic); no em dash in mk/sr/hr. Errors via `internal/apperr`; house JSON envelopes, camelCase.

**Ask First:**

- Making `users.password_hash` nullable, or adding a "set password" flow for passwordless accounts (out of scope here; the unlink guard stands in for it).
- Adding any provider beyond Google, or any schema/migration change to `user_identities`.

**Never:**

- No password prompt on profile-initiated link (that is the login-collision path only). No new session issued by these endpoints — the caller is already authenticated.
- No Google API scopes; never store Google access/refresh tokens. No WS changes. No changes to the public `/auth/sso/*` endpoints.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| List identities | Authed, `:id` == self | 200 `{ hasPassword, identities: [{provider, email, createdAt}] }` | N/A |
| Link (new) | Authed, valid credential, verified email, subject not linked anywhere | Create identity for self → 201 `{ provider, email, createdAt }` | N/A |
| Link (idempotent) | Same Google subject already linked to THIS user | 200, same identity view (no error) | N/A |
| Link conflict | Google subject linked to a DIFFERENT user, or user already has this provider | 409 `SSO_IDENTITY_IN_USE` | Nothing created |
| Link unverified | `email_verified == false` | 403 `SSO_EMAIL_UNVERIFIED` | Nothing created |
| Link bad credential | Expired/forged/wrong audience | 401 `SSO_INVALID_CREDENTIAL` | Generic message, details logged |
| Unlink (has password) | Authed, provider linked, `PasswordHash != ""` | Hard-delete row → 204 | N/A |
| Unlink (passwordless, another identity remains) | `PasswordHash == ""`, 2+ identities | 204 | N/A |
| Unlink last login method | `PasswordHash == ""`, this is the only identity | 409 `SSO_CANNOT_UNLINK_LAST` | Nothing deleted |
| Unlink not linked | Provider has no identity for this user | 404 `SSO_IDENTITY_NOT_FOUND` | N/A |
| Unknown provider (link) | `POST .../identities/facebook` | 400 `SSO_UNKNOWN_PROVIDER` | N/A |
| Foreign `:id` | `:id` != authed user | 403 `FORBIDDEN` | N/A |

</frozen-after-approval>

## Code Map

- `server/internal/identity/repository.go` + `gorm_repo.go` -- add `FindByUserID(userID) ([]Identity, error)`, `DeleteByUserProvider(userID, provider) (int64, error)` (hard delete; no soft-delete on this table)
- `server/internal/auth/handler.go:57` -- `AuthHandler` already holds `userRepo`, `identityRepo`, `providers`; `ssoProvider` :271, passwordless idiom `u.PasswordHash == ""`
- `server/internal/auth/sso_handler.go:197` -- `SSOLink` idempotency/conflict logic to mirror; `validateSSOCredential` :263, `normalizeSSOEmail` :298
- `server/internal/auth/middleware.go:52` -- `GetUserID(c)` exported helper; :14 `AuthMiddleware`
- `server/internal/apperr/errors.go:53-62` -- add `ErrSSOCannotUnlinkLast` (409), `ErrSSOIdentityNotFound` (404)
- `server/cmd/api/main.go:135` -- register 3 routes on the authed `api` group
- `client/src/shared/api/axiosClient.ts:170` -- `axiosClient` (authed, unwraps envelope, throws `FetchError`)
- `client/src/shared/api/profile.ts` -- thin authed-client template to mirror
- `client/src/shared/api/queryKeys.ts:8` -- add `identities.detail(userId)`
- `client/src/features/profile/components/{SidePanel,LinkAccountDialog}.tsx` -- section container + controlled-dialog templates
- `client/src/features/auth/components/GoogleSignInButton.tsx` -- reused for the credential
- `client/src/features/profile/ProfilePage.tsx:92` -- sidebar `<aside>` insertion point
- `client/src/shared/i18n/{en,hr,sr,mk}.json` -- `profile.linkedAccounts.*`; parity test `i18n.parity.test.ts`

## Tasks & Acceptance

**Execution:**

- [x] `server/internal/apperr/errors.go` -- add `ErrSSOCannotUnlinkLast` (`SSO_CANNOT_UNLINK_LAST`, 409) and `ErrSSOIdentityNotFound` (`SSO_IDENTITY_NOT_FOUND`, 404)
- [x] `server/internal/identity/{repository,gorm_repo}.go` -- add `FindByUserID` (order by `created_at`) and `DeleteByUserProvider` (returns `RowsAffected`)
- [x] `server/internal/auth/profile_identity_handler.go` -- new file: `ListIdentities`, `LinkIdentity`, `UnlinkIdentity` methods on `AuthHandler` per the matrix; self-only guard; `IdentityView` DTO; reuse `ssoProvider`/`validateSSOCredential`/`normalizeSSOEmail`; mirror `SSOLink` conflict/idempotency handling; no session issued
- [x] `server/cmd/api/main.go` -- register `GET/POST/DELETE /users/:id/identities(/:provider)` on the authed `api` group
- [x] `server/internal/auth/profile_identity_handler_test.go` -- cover every matrix row via the `setupUserHandlerWithMatches`-style real-middleware + JWT pattern; extend `mockIdentityRepo` with the two new methods
- [x] `server/internal/identity/gorm_repo_test.go` -- per-test-tx integration tests for `FindByUserID` + `DeleteByUserProvider` (incl. delete affects only the target user/provider)
- [x] `client/src/shared/api/identities.ts` -- `getIdentities`/`linkIdentity`/`unlinkIdentity` via `axiosClient` (mirror `profile.ts`, no manual FetchError mapping)
- [x] `client/src/shared/api/queryKeys.ts` + `client/src/shared/hooks/queries/useIdentities.ts` + `client/src/shared/hooks/mutations/useIdentities.ts` -- query key, `useIdentitiesQuery`, link/unlink mutations invalidating `identities.detail(userId)` on success
- [x] `client/src/features/profile/components/LinkedAccounts.tsx` -- SidePanel-style section: per-provider row showing linked email or "not linked"; Link renders `GoogleSignInButton` → link mutation; Unlink opens confirm dialog; unlink disabled with hint when it would be the last login method; toast on success, discriminate errors by `FetchError.code`
- [x] `client/src/features/profile/components/UnlinkAccountDialog.tsx` -- confirm dialog modeled on `LinkAccountDialog` (no password field; non-dismissable while pending)
- [x] `client/src/features/profile/ProfilePage.tsx` -- render `<LinkedAccounts userId={user?.id} />` in the sidebar `<aside>`, independent of the career query
- [x] `client/src/shared/i18n/{en,hr,sr,mk}.json` -- `profile.linkedAccounts.*` (title/eyebrow/description, provider label, linkedAs/notLinked, link/unlink, unlinkDialog.*, errors.*, toasts, passwordless hint); parity green
- [x] `client/src/features/profile/components/LinkedAccounts.test.tsx` -- linked & not-linked render, link flow, unlink confirm flow, unlink-disabled when passwordless+last, error toasts; add `@/shared/api/identities` mock to `ProfilePage.test.tsx`

**Acceptance Criteria:**

- Given a logged-in player whose account has a password, when they open their profile, then the Linked accounts panel shows Google as not linked with a Link action; linking via Google shows the linked email and an enabled Unlink.
- Given a passwordless (SSO-only) account with Google as its sole identity, when viewing the panel, then Unlink is disabled with a hint, and a forced DELETE returns 409 `SSO_CANNOT_UNLINK_LAST` (nothing removed).
- Given a Google account already linked to another user, when linking, then the server returns 409 and the UI shows an "already linked elsewhere" message; no row is created.
- `make test` and `make lint` pass on both stacks; i18n parity green.

## Design Notes

- **Handler placement:** `AuthHandler` on the authed `api` group, not `UserHandler` — it already owns `identityRepo`, `providers`, `userRepo`, and the SSO helpers; `UserHandler` would need those injected. New file keeps the public SSO flow and authed management visually separate.
- **hasPassword must come from the server:** the client `User` type carries no password signal today, so the list endpoint exposes `hasPassword` — it drives both the disabled-Unlink UX and is the server-side source of truth for the unlink guard.
- **Hard delete is correct here:** `user_identities` has no `DeletedAt`; deleting the row frees the `(provider, provider_user_id)` unique index so the same Google account can be relinked later.
- **Idempotent link** (mirror `SSOLink:241`): on `ErrSSOIdentityInUse`, re-fetch by `(provider, subject)`; if it already belongs to the caller, return success — a retried/double-tapped link is not an error.

## Verification

**Commands:**

- `cd server && go build ./... && go test ./...` -- expected: green
- `cd client && npx vitest run` -- expected: green incl. i18n parity
- `make lint` -- expected: clean

**Manual checks:**

- `make dev` with `VITE_GOOGLE_CLIENT_ID` / `BELJOT_GOOGLE_CLIENT_ID` set: on /profile, link a Google account (see linked email), unlink it; confirm a fresh Google login still registers/logs in.

## Suggested Review Order

**Backend — endpoints & authorization (entry point)**

- Start here: the three self-managed identity handlers capture the whole design (self-only, JWT link, unlink guard)
  [`profile_identity_handler.go:60`](../../server/internal/auth/profile_identity_handler.go#L60)
- Shared self-only guard — a byte-for-byte mirror of GetProfile's 401/400/403 ladder
  [`profile_identity_handler.go:43`](../../server/internal/auth/profile_identity_handler.go#L43)
- Link: JWT-gated (no password), idempotent re-link, provider-agnostic conflict mapping, user-existence guard
  [`profile_identity_handler.go:98`](../../server/internal/auth/profile_identity_handler.go#L98)
- Unlink: the load-bearing "cannot remove the last sign-in method" guard for passwordless accounts
  [`profile_identity_handler.go:218`](../../server/internal/auth/profile_identity_handler.go#L218)
- Routes wired on the authed group beside the other `/users/:id` endpoints
  [`main.go:142`](../../server/cmd/api/main.go#L142)

**Backend — persistence & errors**

- New repo contract: list-by-user (oldest first) + hard-delete-by-provider returning RowsAffected
  [`repository.go:15`](../../server/internal/identity/repository.go#L15)
- GORM delete surfaces RowsAffected so the handler distinguishes a lost race from a real delete
  [`gorm_repo.go:53`](../../server/internal/identity/gorm_repo.go#L53)
- Two new SSO errors: cannot-unlink-last (409), identity-not-found (404)
  [`errors.go:69`](../../server/internal/apperr/errors.go#L69)

**Frontend — panel & flows**

- The LinkedAccounts panel: linked/not-linked row, `canUnlink` gate mirroring the server guard, link/unlink handlers
  [`LinkedAccounts.tsx:44`](../../client/src/features/profile/components/LinkedAccounts.tsx#L44)
- Unlink confirm dialog (no password; non-dismissable while pending), modeled on LinkAccountDialog
  [`UnlinkAccountDialog.tsx:32`](../../client/src/features/profile/components/UnlinkAccountDialog.tsx#L32)
- Panel dropped into the profile sidebar, independent of the career query
  [`ProfilePage.tsx:94`](../../client/src/features/profile/ProfilePage.tsx#L94)
- Thin authed API client (mirrors profile.ts — envelope + errors handled by axiosClient)
  [`identities.ts:26`](../../client/src/shared/api/identities.ts#L26)
- Link/unlink mutations invalidate the identities query key on success
  [`useIdentities.ts:6`](../../client/src/shared/hooks/mutations/useIdentities.ts#L6)
- `profile.linkedAccounts.*` copy added across all four locales (mk Cyrillic)
  [`en.json:150`](../../client/src/shared/i18n/en.json#L150)

**Peripherals — tests**

- Handler tests: every I/O-matrix row + the user-not-found guard, via real AuthMiddleware + JWT
  [`profile_identity_handler_test.go:73`](../../server/internal/auth/profile_identity_handler_test.go#L73)
- Repo integration tests (per-test tx): list + scoped-delete semantics
  [`gorm_repo_test.go:107`](../../server/internal/identity/gorm_repo_test.go#L107)
- Component tests: linked/not-linked render, link, unlink-confirm, passwordless-disabled, error toasts
  [`LinkedAccounts.test.tsx:56`](../../client/src/features/profile/components/LinkedAccounts.test.tsx#L56)
