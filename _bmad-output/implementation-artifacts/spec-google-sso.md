---
title: 'Continue with Google — SSO login and register with safe account linking'
type: 'feature'
created: '2026-07-06'
status: 'done'
review_loop_iteration: 0
baseline_commit: '9c4fb91f7924f9de62f15f4457ec801b451cc944'
context:
  - '{project-root}/_bmad-output/project-context.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Players can only register/log in with email+password. We want one-click "Continue with Google" on the login and register pages, with safe linking when the Google email matches an existing password account — extensible to future providers (Facebook, etc.). Profile link/unlink UI is deferred (see deferred-work.md).

**Approach:** The Google Identity Services (GIS) button yields a Google ID token; the Go backend verifies it and issues the same JWT + refresh-cookie session as password login. Identities live in a provider-agnostic `user_identities` table behind a `Provider` interface registry. Email-matching accounts are never auto-linked — the user must confirm with the account's password.

## Boundaries & Constraints

**Always:**

- Verify ID tokens server-side (`google.golang.org/api/idtoken`): signature, audience == `BELJOT_GOOGLE_CLIENT_ID`, issuer `accounts.google.com`/`https://accounts.google.com`, expiry. Trust emails only when `email_verified == true`.
- Reuse `startSession` + `authResponseData` — SSO sessions identical to password sessions; SSO registration seeds wallet/streak exactly like `Register`.
- Adding a provider = new `Provider` impl + registry entry + i18n label; no schema/endpoint changes.
- Link-during-login requires the matched account's password (`CheckPassword`).
- New i18n keys in all four locales (en, hr, sr, mk — mk fully Cyrillic, natural phrasing). Errors via `internal/apperr`; house JSON envelopes, camelCase.

**Ask First:**

- Switching to redirect/authorization-code OAuth (needs console redirect URIs + client secret at runtime).
- Making `users.password_hash` nullable (plan uses empty-string sentinel = "no password set").
- Any change to refresh-token rotation semantics.

**Never:**

- No Google API scopes; never store Google access/refresh tokens — authentication only.
- Client secret never in client code or repo (stays in gitignored `docs/temp/`).
- No One Tap prompt. No auto-linking on email match alone. No profile UI/endpoints (deferred). No WS changes.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| SSO login | Valid credential; identity `(google, sub)` exists | 200 + session, like password login | N/A |
| SSO register | Valid credential, verified email, no identity, no user with email | Create user (generated unique username, empty password hash, wallet/streak seeded) + identity → 201 + session | N/A |
| Email collision | Verified email matches password account, no identity | 409 `SSO_LINK_REQUIRED`; client opens password dialog | No identity created |
| Link during login | Credential + correct password | Create identity → 200 + session; next SSO login is direct | Wrong password → 401 `INVALID_CREDENTIALS`, nothing linked |
| Link to passwordless acct | Matched account has empty hash | 401 `INVALID_CREDENTIALS` | Nothing linked |
| Unverified email | `email_verified=false`, no identity | 403 `SSO_EMAIL_UNVERIFIED` | No creation/matching |
| Bad credential | Expired/forged/wrong audience | 401 `SSO_INVALID_CREDENTIAL` | Generic message, details logged |
| Password login on SSO-only acct | Any password vs empty hash | 401 `INVALID_CREDENTIALS` (no panic) | N/A |
| Unknown provider | `POST /auth/sso/facebook` | 400 `SSO_UNKNOWN_PROVIDER` | N/A |

</frozen-after-approval>

## Code Map

- `server/cmd/api/main.go` -- public authGroup ~:113, handler DI ~:56
- `server/internal/auth/handler.go` -- `startSession` :113, `authResponseData` :152, login email normalization :252
- `server/internal/auth/service.go` -- `CheckPassword`; `handler_test.go` -- mockUserRepo + httptest pattern
- `server/internal/user/{model,repository,gorm_repo}.go` -- `User` (PasswordHash `not null`), 23505 mapping pattern
- `server/internal/{apperr/errors.go,config/config.go}` -- error vars :31-55; `BELJOT_` env loading + prod guards
- `server/migrations/` -- highest is 000013
- `client/src/features/auth/{LoginPage,RegisterPage,components/AuthCard}.tsx` -- forms, `SuitRule`
- `client/src/shared/hooks/mutations/useAuth.ts` (`setAuthState`), `client/src/shared/api/auth.ts` + `axiosClient.ts` (`axiosPublic`, FetchError wrap)
- `client/src/features/lobby/components/PasswordPromptDialog.tsx` -- template for password dialog
- `client/src/shared/i18n/{en,hr,sr,mk}.json` -- parity-tested locales

## Tasks & Acceptance

**Execution:**

- [x] `server/migrations/000014_create_user_identities.{up,down}.sql` -- id, user_id FK (CASCADE), provider VARCHAR(20), provider_user_id VARCHAR(255), email, timestamps; UNIQUE(provider, provider_user_id), UNIQUE(user_id, provider)
- [x] `server/internal/identity/{model,repository,gorm_repo}.go` -- `Identity` model; iface `Create`, `FindByProviderSubject`; 23505 → conflict
- [x] `server/internal/identity/{provider,google}.go` -- `Provider` iface `{Name(); Verify(ctx, credential) (ExternalIdentity, error)}` → {Subject, Email, EmailVerified, DisplayName}; registry map; Google impl via `idtoken.Validate` + issuer check
- [x] `server/internal/apperr/errors.go` -- `ErrSSOInvalidCredential`(401), `ErrSSOLinkRequired`(409), `ErrSSOEmailUnverified`(403), `ErrSSOIdentityInUse`(409), `ErrSSOUnknownProvider`(400)
- [x] `server/internal/config/config.go` + `.env.example` -- `GoogleClientID` (`BELJOT_GOOGLE_CLIENT_ID`) + non-dev empty warning
- [x] `server/internal/auth/sso_handler.go` -- `POST /auth/sso/:provider` {credential} and `POST /auth/sso/:provider/link` {credential, password} per matrix; username from Google name/email → `[a-zA-Z0-9_]{3,20}`, numeric-suffix uniquified; new `AuthHandler` deps: identity repo, registry
- [x] `server/cmd/api/main.go` + `go.mod` -- wire deps + both public routes; add `google.golang.org/api` (idtoken)
- [x] `server/internal/auth/sso_handler_test.go` (+ mocks) -- fake Provider, cover every matrix row; `server/internal/identity/gorm_repo_test.go` -- per-test-tx (constraints, cascade)
- [x] `client/.env.example` (`VITE_GOOGLE_CLIENT_ID`, ensure `.env` gitignored) + `client/src/shared/lib/googleIdentity.ts` -- idempotent GSI script loader, typed `google.accounts.id`, initialize/renderButton wrapper (locale from i18n)
- [x] `client/src/features/auth/components/GoogleSignInButton.tsx` -- div-ref GIS button; hidden when client ID unset; `onCredential` prop
- [x] `client/src/shared/api/auth.ts` + `client/src/shared/hooks/mutations/useAuth.ts` -- `ssoLogin`/`ssoLink` (axiosPublic) + mutations with `setAuthState` onSuccess
- [x] `client/src/features/auth/components/LinkAccountDialog.tsx` -- password-confirm dialog after `PasswordPromptDialog` (shows matched email, inline wrong-password error)
- [x] `client/src/features/auth/LoginPage.tsx` + `RegisterPage.tsx` -- `SuitRule` "or" divider + button in `AuthCard`; `SSO_LINK_REQUIRED` → LinkAccountDialog → `ssoLink`; success mirrors password-login language reconciliation + navigate `/lobby`; register page adds ToS/privacy small-print
- [x] `client/src/shared/i18n/{en,hr,sr,mk}.json` -- `auth.sso.*` (divider, consent note, dialog title/lede/errors), parity green
- [x] `client/src/features/auth/LoginPage.test.tsx` + `RegisterPage.test.tsx` -- mock `googleIdentity` + api: SSO success→store+navigate, link-required→dialog→success, wrong password inline error

**Acceptance Criteria:**

- Given `make dev` with real client IDs, when clicking the Google button on /login with a fresh Google account, then a user is created and lands in /lobby with a working session (refresh included).
- Given an existing password account with the same email, when continuing with Google, then the password dialog appears; correct password links + logs in; the next Google login succeeds directly.
- Given a future provider, then support requires only a `Provider` impl + registry entry + i18n label (no `"google"` literals in handler flow logic).
- `make test` and `make lint` pass on both stacks.

## Design Notes

- **GIS ID-token flow, not redirect/code flow:** the OAuth client has JS origins (localhost:5173, beljot.online) but zero redirect URIs; ID-token verification needs no secret and no callback route — dev and prod work unchanged.
- **Empty-string `password_hash` sentinel** avoids a `*string` ripple; `CheckPassword("", x)` fails safely; passwordless users can set a password via the existing reset flow.
- `SSO_LINK_REQUIRED` is disclosed only to a Google-verified owner of that email, so account-existence disclosure is acceptable.

## Verification

**Commands:**

- `cd server && go build ./... && go test ./...` -- expected: green
- `cd client && npx vitest run` -- expected: green incl. i18n parity
- `make lint` -- expected: clean

**Manual checks (if no CLI):**

- `make dev` with both client-ID env vars from the `docs/temp/` JSON; exercise fresh-Google register and the email-collision password-link flow end-to-end.

## Suggested Review Order

**Entry point — SSO decision tree**

- The whole flow: verify credential → identity hit → email match → register; start here for design intent
  [`sso_handler.go:61`](../../server/internal/auth/sso_handler.go#L61)
- Registry built from config — google registered only when configured; routes are two lines
  [`main.go:66`](../../server/cmd/api/main.go#L66)

**Provider abstraction (extensibility seam)**

- The contract every future provider (Facebook, …) implements; handlers never see "google"
  [`provider.go:22`](../../server/internal/identity/provider.go#L22)
- Google impl: `idtoken.Validate` + explicit issuer pin + 10s timeout; validate fn injectable for tests
  [`google.go:19`](../../server/internal/identity/google.go#L19)

**Safe linking (security core)**

- Password-gated link-during-login: no-oracle 401s, idempotent retry, race-safe conflict mapping
  [`sso_handler.go:197`](../../server/internal/auth/sso_handler.go#L197)
- SSO error taxonomy + disclosure rationale (why 409 SSO_LINK_REQUIRED is acceptable)
  [`errors.go:53`](../../server/internal/apperr/errors.go#L53)

**Session parity & registration robustness**

- `issueSession` delegates to `startSession`/`authResponseData` — SSO sessions identical to password sessions
  [`sso_handler.go:282`](../../server/internal/auth/sso_handler.go#L282)
- Compensating soft-delete if the identity insert fails — partial unique index frees the email
  [`sso_handler.go:139`](../../server/internal/auth/sso_handler.go#L139)
- `user_identities` schema: unique (provider, subject) and (user, provider), FK cascade
  [`000014_create_user_identities.up.sql:1`](../../server/migrations/000014_create_user_identities.up.sql#L1)

**Frontend SSO flow**

- One shared hook owns credential handling, link-dialog state, navigation — used by both auth pages
  [`useGoogleSso.ts:35`](../../client/src/features/auth/useGoogleSso.ts#L35)
- Error discrimination by `code`, never `status` — expired Google token ≠ wrong password
  [`useGoogleSso.ts:89`](../../client/src/features/auth/useGoogleSso.ts#L89)
- Idempotent GSI script loader; failure not cached; UTF-8-safe email decode
  [`googleIdentity.ts:63`](../../client/src/shared/lib/googleIdentity.ts#L63)
- Container-measured button width (mobile-safe) + stale-initialize guard across page switches
  [`GoogleSignInButton.tsx:37`](../../client/src/features/auth/components/GoogleSignInButton.tsx#L37)
- Password-confirm dialog: dismissal locked while the link request is in flight
  [`LinkAccountDialog.tsx:38`](../../client/src/features/auth/components/LinkAccountDialog.tsx#L38)
- Page wiring: button + consent small-print on the login page too (it can register accounts)
  [`LoginPage.tsx:195`](../../client/src/features/auth/LoginPage.tsx#L195)

**Peripherals**

- Server config: `BELJOT_GOOGLE_CLIENT_ID` as verification audience
  [`config.go:34`](../../server/internal/config/config.go#L34)
- API client functions + shared `toFetchError` mapping
  [`auth.ts:129`](../../client/src/shared/api/auth.ts#L129)
- i18n: `auth.sso.*` across en/hr/sr/mk (mk Cyrillic)
  [`en.json:855`](../../client/src/shared/i18n/en.json#L855)
- Backend tests: every edge-case-matrix row plus race/idempotency/compensation pins
  [`sso_handler_test.go:1`](../../server/internal/auth/sso_handler_test.go#L1)
- Identity integration tests: unique constraints + cascade against real Postgres
  [`gorm_repo_test.go:1`](../../server/internal/identity/gorm_repo_test.go#L1)
- Frontend tests: SSO success, link flow, expired credential, pending guard, consent render
  [`LoginPage.test.tsx:1`](../../client/src/features/auth/LoginPage.test.tsx#L1)
