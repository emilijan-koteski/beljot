---
title: 'Password Reset (Forgot Password) flow with localized email'
type: 'feature'
created: '2026-06-17'
status: 'done'
baseline_commit: '0cefdf63d4b88155752c585cbd3b9e0d97815f8d'
context:
  - '{project-root}/_bmad-output/project-context.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Users who forget their password have no way to recover their account — login only succeeds with a remembered password, and there is no "Forgot?" affordance or any email-sending capability in the system.

**Approach:** Add a self-service reset flow: a "Forgot?" link on login opens `/forgot-password` (enter email), which mints a DB-backed single-use token and emails a `/reset-password?token=…` link in the user's own language; that page sets a new password. Email is sent via `github.com/wneessen/go-mail` through a new injectable `mailer` package. New auth pages reuse the existing `AuthLayout` + `AuthCard` chrome so branding and field positions match login/register exactly.

## Boundaries & Constraints

**Always:**
- Anti-enumeration: `POST /auth/forgot-password` returns the SAME generic success for existing and non-existing emails; never reveal whether an account exists.
- Reset token is single-use, 1-hour expiry, stored as a SHA-256 hash (never plaintext) in `password_reset_tokens`; the raw token lives only in the emailed link.
- Email subject + body are localized to the target user's stored `languagePreference` (en/sr/mk/hr); unknown/empty language falls back to `en`.
- Server is authoritative: re-validate password (8–72 chars) and token validity server-side. New frontend i18n keys must be added to ALL four locale files (parity test gates this); `mk` must be all-Cyrillic and read naturally.
- Config only via `config.Config` + DI (no `os.Getenv` outside config). Secrets never committed — `.env.example` carries placeholders only.

**Ask First:**
- Any change to the existing `NewAuthHandler` signature or existing auth routes (prefer a separate `PasswordResetHandler` to avoid churn).
- Adding a confirm-password mismatch rule beyond "must match" / changing password length limits.

**Never:**
- No account enumeration via timing or differing responses/status codes.
- No auto-login after reset — redirect to `/login` with a success toast.
- No session invalidation of existing access/refresh JWTs on reset (deferred — logged in `deferred-work.md`).
- No email visual design — simple HTML only, structured so copy can be edited later.
- Do not touch the dev `docker-compose.yml` (Postgres-only; Go runs on host and reads `.env` via the Makefile).

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Forgot — known email | `{email}` matches a user | 200 generic success; prior tokens for that user deleted; new token row created; email sent in user's language | Mailer error logged, response still 200 |
| Forgot — unknown email | `{email}` matches nobody | 200 identical generic success; no token created; no email | N/A |
| Forgot — malformed email | empty / not an email | 200 generic success (no enumeration); no token | N/A |
| Reset — valid token | `{token, password}` token unused & unexpired | password_hash updated; token marked used; 200 success | N/A |
| Reset — expired/used/unknown token | invalid token | no change | 400 `INVALID_RESET_TOKEN` (single generic code) |
| Reset — bad password | valid token, password <8 or >72 | no change | 400 `PASSWORD_TOO_SHORT` / `PASSWORD_TOO_LONG` |
| Reset page — missing/blank `token` query param | open `/reset-password` with no token | render invalid-link state + link to `/forgot-password`; submit disabled | N/A |

</frozen-after-approval>

## Code Map

- `server/internal/auth/handler.go` -- existing auth handlers (Register/Login pattern to mirror: bind, normalize email, JSON envelope)
- `server/internal/auth/handler_test.go` -- `mockUserRepo` + `httptest` setup the new tests reuse
- `server/internal/user/repository.go` + `gorm_repo.go` -- `UpdateLanguagePreference` is the template for `UpdatePasswordHash`
- `server/internal/config/config.go` -- `Load()` + `getEnv`; extend with SMTP + app base URL
- `server/internal/apperr/errors.go` -- centralized errors; reuse `ErrPasswordTooShort/Long`, add `ErrInvalidResetToken`
- `server/cmd/api/main.go` -- DI wiring + public `/api/v1/auth` route group
- `server/migrations/000007_*` -- highest existing migration; next is `000008`
- `client/src/features/auth/{AuthLayout,LoginPage}.tsx` + `components/AuthCard.tsx` -- card chrome; `Field` has a right-aligned `hint` slot for the "Forgot?" link
- `client/src/shared/components/ui/field.tsx` -- `hint` prop = right of label (no change needed)
- `client/src/shared/api/auth.ts` + `hooks/mutations/useAuth.ts` -- API + mutation patterns (axiosPublic, FetchError)
- `client/src/App.tsx` -- routes; new pages go under `GuestRoute` → `AuthLayout`
- `client/src/shared/i18n/{en,sr,mk,hr}.json` -- `auth.*` keys; parity test enforces all four

## Tasks & Acceptance

**Execution:**
- [x] `server/go.mod` -- add `github.com/wneessen/go-mail` via `go get` (pin in go.mod/go.sum)
- [x] `server/migrations/000008_create_password_reset_tokens.up.sql` / `.down.sql` -- create `password_reset_tokens` (id, user_id FK→users, token_hash VARCHAR(64), expires_at, used_at NULL, created_at) + index on token_hash and user_id; down drops the table
- [x] `server/internal/passwordreset/model.go` + `repository.go` + `gorm_repo.go` -- `PasswordResetToken` struct (no Updated/DeletedAt) + `Repository` interface (`Create`, `FindValidByHash` excludes used/expired, `MarkUsed`, `DeleteByUserID`) + GORM impl
- [x] `server/internal/mailer/mailer.go` + `templates.go` + `smtp.go` + `log.go` -- `Mailer` interface `SendPasswordReset(ctx, to, lang, link) error`; `templates.go` localized subject+HTML per lang (en/sr/mk/hr, en fallback); `SMTPMailer` (go-mail, STARTTLS:587, LOGIN/PLAIN auth); `LogMailer` logs the link via slog for dev when SMTP unconfigured
- [x] `server/internal/config/config.go` -- add `SMTPHost/Port/Username/Password/From/FromName`, `AppBaseURL`; env `BELJOT_SMTP_*`, `BELJOT_APP_BASE_URL` (dev default `http://localhost:5173`)
- [x] `server/internal/apperr/errors.go` -- add `ErrInvalidResetToken` ("INVALID_RESET_TOKEN", 400)
- [x] `server/internal/user/repository.go` + `gorm_repo.go` -- add `UpdatePasswordHash(id, hash) error` (mirror `UpdateLanguagePreference`, updates `updated_at`)
- [x] `server/internal/auth/password_reset.go` -- `PasswordResetHandler` with `NewPasswordResetHandler(userRepo, resetRepo, mailer, appBaseURL, resetTTL)`; `ForgotPassword` (generic success, raw token = 32 rand bytes base64url, store sha256 hex, build link, send in user's lang) + `ResetPassword` (hash incoming token, find valid, validate password, update hash, mark used)
- [x] `server/cmd/api/main.go` -- build mailer (SMTP if host+username set, else LogMailer), `passwordreset` GORM repo, `PasswordResetHandler`; register `authGroup.POST("/forgot-password", …)` + `POST("/reset-password", …)`
- [x] `server/internal/auth/password_reset_test.go` -- table-driven tests for all I/O Matrix backend rows using a fake mailer + in-memory reset repo; add `UpdatePasswordHash` to `mockUserRepo`
- [x] `server/internal/mailer/mailer_test.go` -- assert subject/body picked per lang and en fallback for unknown lang
- [x] `client/src/shared/api/auth.ts` -- `forgotPassword({email})` + `resetPassword({token,password})` via `axiosPublic`, FetchError mapping
- [x] `client/src/shared/hooks/mutations/useAuth.ts` -- `useForgotPasswordMutation` + `useResetPasswordMutation` (no auth-state writes)
- [x] `client/src/features/auth/ForgotPasswordPage.tsx` -- `AuthCard` email form; on success swap card to generic "check your inbox" state; `AltLink` back to `/login`
- [x] `client/src/features/auth/ResetPasswordPage.tsx` -- read `token` via `useSearchParams`; new + confirm password (show/hide toggle); success → toast + `navigate("/login")`; invalid/expired token → inline error + link to `/forgot-password`; missing token → invalid-link state
- [x] `client/src/features/auth/LoginPage.tsx` -- pass `hint={<Link to="/forgot-password">…forgotLink</Link>}` to the password `Field` (right of PASSWORD label, per design photo)
- [x] `client/src/App.tsx` -- add `/forgot-password` + `/reset-password` routes inside the `GuestRoute` → `AuthLayout` group
- [x] `client/src/shared/i18n/{en,sr,mk,hr}.json` -- add `auth.login.forgotLink`, `auth.forgotPassword.*`, `auth.resetPassword.*` (idiomatic; mk all-Cyrillic)
- [x] `client/src/features/auth/ForgotPasswordPage.test.tsx` + `ResetPasswordPage.test.tsx` -- render, submit, success + error/invalid-token states (mock the mutations)
- [x] `.env.example` -- add `BELJOT_SMTP_*` + `BELJOT_APP_BASE_URL` placeholders
- [x] `docker-compose.prod.yml` -- add `BELJOT_SMTP_*` + `BELJOT_APP_BASE_URL` to the `api` service `environment` block
- [x] `.env` (local, gitignored) -- add the same vars for dev (request permission if the tool blocks it; otherwise hand the user the exact lines)
- [x] `_bmad-output/implementation-artifacts/deferred-work.md` -- append a "session invalidation on password reset" deferred entry

**Acceptance Criteria:**
- Given a logged-out user on `/login`, when they click "Forgot?", then they land on `/forgot-password` rendered in the same `AuthLayout`/`AuthCard` chrome as login.
- Given SMTP is unconfigured in dev, when a known user requests a reset, then the server logs the full reset link (testable without a real mailbox) and the endpoint returns success.
- Given a reset email, when the user opens the link and submits a valid matching password, then their stored hash changes (old password fails, new password logs in) and reusing the same link fails with `INVALID_RESET_TOKEN`.
- Given any new i18n key, when `npx vitest run i18n.parity` runs, then all four locales have the key as a non-empty string.

## Design Notes

- **Token:** `raw = base64url(32 random bytes)`; persist `sha256hex(raw)`; lookups hash the inbound token. `DeleteByUserID` before `Create` so only the latest link is live.
- **go-mail sketch** (Gmail = smtp.gmail.com:587 STARTTLS):
  ```go
  m := mail.NewMsg()
  m.From(fromAddr); m.To(to); m.Subject(subj)
  m.SetBodyString(mail.TypeTextHTML, body)
  c, _ := mail.NewClient(host, mail.WithPort(port),
      mail.WithSMTPAuth(mail.SMTPAuthLogin),
      mail.WithUsername(user), mail.WithPassword(pass))
  return c.DialAndSendWithContext(ctx, m)
  ```
- **Secret placement (answers the GitHub question):** runtime secret, not CI. Prod = add the `BELJOT_SMTP_*` lines to the `PROD_ENV_FILE` GitHub *Environment* secret (Settings → Environments → `production`), which `deploy.yml` writes to `/opt/beljot/.env`; also add the env keys to `docker-compose.prod.yml`'s `api` service. Dev = `.env`. (Documented at implement time, not code.)

## Verification

**Commands:**
- `cd server && go build ./... && go test ./...` -- expected: compiles, all tests pass (new auth + mailer tests included)
- `cd server && golangci-lint run ./...` -- expected: clean
- `cd client && npx vitest run` -- expected: pass, including i18n parity + new page tests
- `cd client && npx eslint . && npx prettier --check .` -- expected: clean

**Manual checks:**
- `make dev`, open `/login` → "Forgot?" → submit a seeded email → confirm the reset link is logged by the Go server; open it → set a new password → redirected to `/login` → log in with the new password.

## Suggested Review Order

**Token lifecycle & security — start here**

- Entry point: anti-enumeration (generic response for any email) + mint-then-async-send
  [`password_reset.go:69`](../../server/internal/auth/password_reset.go#L69)
- The reset itself: token validity wins over password shape; atomic single-use consume before password update; soft-deleted-user → invalid-link
  [`password_reset.go:141`](../../server/internal/auth/password_reset.go#L141)
- Atomic single-use guard (`used_at IS NULL` + RowsAffected) — the concurrent-reuse fix
  [`gorm_repo.go:36`](../../server/internal/passwordreset/gorm_repo.go#L36)
- Valid-token lookup excludes used/expired rows
  [`gorm_repo.go:22`](../../server/internal/passwordreset/gorm_repo.go#L22)
- 256-bit `crypto/rand` token; only its SHA-256 hash is stored
  [`password_reset.go:213`](../../server/internal/auth/password_reset.go#L213)
- Schema: hashed token, expiry, nullable `used_at`; reversible down migration
  [`000008…up.sql:1`](../../server/migrations/000008_create_password_reset_tokens.up.sql#L1)

**Email delivery — localized, async**

- Minimal HTML body + localized subject per language, en fallback, escaped link
  [`templates.go:73`](../../server/internal/mailer/templates.go#L73)
- mk copy is all-Cyrillic; "Beljot" brand stays Latin
  [`templates.go:43`](../../server/internal/mailer/templates.go#L43)
- go-mail STARTTLS send; client built per send
  [`smtp.go:33`](../../server/internal/mailer/smtp.go#L33)
- Mailer selection: real SMTP when configured, else dev log-fallback
  [`main.go:59`](../../server/cmd/api/main.go#L59)

**Config & secrets**

- SMTP + app-base-URL config; Gmail app-password whitespace stripped
  [`config.go:55`](../../server/internal/config/config.go#L55)
- Loud warning if base URL is localhost/unset in non-dev (dead reset links)
  [`config.go:69`](../../server/internal/config/config.go#L69)

**Frontend flow**

- Forgot page: validates email, always shows generic "check your inbox"
  [`ForgotPasswordPage.tsx:26`](../../client/src/features/auth/ForgotPasswordPage.tsx#L26)
- Reset page: token from query, invalid-link state, success → `/login`
  [`ResetPasswordPage.tsx:46`](../../client/src/features/auth/ResetPasswordPage.tsx#L46)
- "Forgot?" link in the password field `hint` slot (matches design photo)
  [`LoginPage.tsx:148`](../../client/src/features/auth/LoginPage.tsx#L148)
- Two public routes under the shared `AuthLayout`
  [`App.tsx:48`](../../client/src/App.tsx#L48)
- API client via `axiosPublic` + `FetchError` mapping
  [`auth.ts:99`](../../client/src/shared/api/auth.ts#L99)

**Peripherals — tests, i18n, infra**

- Backend handler tests: anti-enum, single-use, async send, validation order
  [`password_reset_test.go:153`](../../server/internal/auth/password_reset_test.go#L153)
- Localized email-template test
  [`mailer_test.go:8`](../../server/internal/mailer/mailer_test.go#L8)
- New i18n keys in all four locales (parity-gated)
  [`en.json:788`](../../client/src/shared/i18n/en.json#L788)
- Env template (SMTP + base URL placeholders)
  [`.env.example:15`](../../.env.example#L15)
- Prod compose: SMTP env wired into the api service
  [`docker-compose.prod.yml:48`](../../docker-compose.prod.yml#L48)
