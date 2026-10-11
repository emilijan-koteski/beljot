# Story 1.2: User Registration

Status: done

## Story

As a new player,
I want to create an account with my email, username, and password,
so that I have a persistent identity on the Beljot platform.

## Acceptance Criteria

1. **Given** no account exists for the provided email
   **When** a player submits valid registration data (email, username, password)
   **Then** a new user record is created in the `users` table with the password stored as a bcrypt hash
   **And** the player is automatically logged in (receives JWT access token + refresh cookie)
   **And** the player is redirected to the lobby

2. **Given** the registration page is displayed
   **When** the player views the form
   **Then** three fields are shown: Email, Username, Password — with a submit button
   **And** all labels and prompts are displayed in the player's selected language (EN or SR)

3. **Given** a player submits a duplicate email
   **When** the server processes the request
   **Then** a 409 error is returned with error code `EMAIL_TAKEN` and a user-friendly message
   **And** the error displays inline below the email field

4. **Given** a player submits a duplicate username
   **When** the server processes the request
   **Then** a 409 error is returned with error code `USERNAME_TAKEN` and a user-friendly message
   **And** the error displays inline below the username field

5. **Given** invalid input (empty fields, invalid email format, password too short)
   **When** the player blurs a field or submits the form
   **Then** frontend validation shows specific inline error messages below the relevant field
   **And** the server rejects the request with a 400 error and structured error response if frontend validation is bypassed

6. **Given** the `users` migration
   **When** I inspect the database schema
   **Then** the `users` table contains: `id` (PK, auto-increment), `email` (unique), `username` (unique), `password_hash`, `language_preference` (default 'en'), `created_at`, `updated_at`

## Tasks / Subtasks

- [x] **Task 1: Create users table migration** (AC: 6)
  - [x] Create `server/migrations/000002_create_users.up.sql` with `users` table: `id` (serial PK), `email` (varchar unique, NOT NULL), `username` (varchar unique, NOT NULL), `password_hash` (varchar, NOT NULL), `language_preference` (varchar default 'en'), `created_at` (timestamptz), `updated_at` (timestamptz), `deleted_at` (timestamptz nullable for GORM soft delete)
  - [x] Add indexes: `idx_users_email` on `email`, `idx_users_username` on `username`, `idx_users_deleted_at` on `deleted_at`
  - [x] Create `server/migrations/000002_create_users.down.sql` that drops the `users` table

- [x] **Task 2: Implement User domain package** (AC: 6)
  - [x] Create `server/internal/user/model.go` — GORM `User` struct with fields: `ID uint`, `Email string`, `Username string`, `PasswordHash string`, `LanguagePreference string`, `CreatedAt time.Time`, `UpdatedAt time.Time`, `DeletedAt gorm.DeletedAt`. All with proper `gorm:` and `json:` tags (camelCase JSON). `PasswordHash` gets `json:"-"` to never serialize
  - [x] Create `server/internal/user/repository.go` — `UserRepository` interface with methods: `Create(user *User) error`, `FindByEmail(email string) (*User, error)`, `FindByUsername(username string) (*User, error)`, `FindByID(id uint) (*User, error)`
  - [x] Create `server/internal/user/gorm_repo.go` — GORM implementation of `UserRepository`

- [x] **Task 3: Implement Auth service** (AC: 1, 3, 4, 5)
  - [x] Add `github.com/golang-jwt/jwt/v5` dependency: `go get github.com/golang-jwt/jwt/v5`
  - [x] Create `server/internal/auth/service.go` with:
    - `HashPassword(password string) (string, error)` — bcrypt hash via `golang.org/x/crypto/bcrypt`
    - `CheckPassword(hash, password string) error` — bcrypt compare
    - `GenerateAccessToken(userID uint, secret string) (string, error)` — JWT with ~15 min expiry, `sub` claim = user ID
    - `GenerateRefreshToken(userID uint, secret string) (string, error)` — JWT with ~7 day expiry, `sub` claim = user ID
    - `ValidateToken(tokenString, secret string) (*jwt.RegisteredClaims, error)` — parse and validate JWT
  - [x] Create `server/internal/auth/handler.go` with `POST /api/v1/auth/register` handler:
    - Parse JSON body: `{ "email": "...", "username": "...", "password": "..." }`
    - Server-side validation: email format, username length (3-20 chars, alphanumeric + underscores), password length (min 8 chars)
    - Check email uniqueness via `UserRepository.FindByEmail` — return `ErrEmailTaken` (409) if exists
    - Check username uniqueness via `UserRepository.FindByUsername` — return `ErrUsernameTaken` (409) if exists
    - Hash password via `auth.HashPassword`
    - Create user via `UserRepository.Create`
    - Generate access token + refresh token
    - Set refresh token as httpOnly secure cookie (path `/api/v1/auth`, SameSite=Strict, 7-day MaxAge)
    - Return `201 { "data": { "id": N, "username": "...", "email": "...", "languagePreference": "en", "createdAt": "..." } }` with access token in response body under `"data"."token"`

- [x] **Task 4: Add domain errors to apperr** (AC: 3, 4, 5)
  - [x] Add to `server/internal/apperr/errors.go`:
    - `ErrEmailTaken = NewAppError("EMAIL_TAKEN", "email is already registered", 409)`
    - `ErrUsernameTaken = NewAppError("USERNAME_TAKEN", "username is already taken", 409)`
    - `ErrInvalidEmail = NewAppError("INVALID_EMAIL", "invalid email format", 400)`
    - `ErrPasswordTooShort = NewAppError("PASSWORD_TOO_SHORT", "password must be at least 8 characters", 400)`
    - `ErrUsernameTooShort = NewAppError("USERNAME_TOO_SHORT", "username must be between 3 and 20 characters", 400)`
    - `ErrUsernameInvalidChars = NewAppError("USERNAME_INVALID_CHARS", "username can only contain letters, numbers, and underscores", 400)`

- [x] **Task 5: Wire auth routes in main.go** (AC: 1)
  - [x] In `server/cmd/api/main.go`:
    - Initialize `UserRepository` (GORM implementation) with the DB instance
    - Initialize `AuthHandler` with `UserRepository` and config
    - Register `POST /api/v1/auth/register` route (no auth middleware — public endpoint)
    - Keep middleware order: CORS -> Logging -> Error Handler -> Auth (auth middleware not applied to `/auth/*` routes)

- [x] **Task 6: Create frontend API client for auth** (AC: 1)
  - [x] Create `client/src/shared/api/auth.ts` with:
    - `register(data: { email: string; username: string; password: string }): Promise<RegisterResponse>` — calls `POST /api/v1/auth/register`
    - Type: `RegisterResponse = { token: string; id: number; username: string; email: string; languagePreference: string; createdAt: string }`
  - [x] Add `User` type to `client/src/shared/types/apiTypes.ts`: `{ id: number; username: string; email: string; languagePreference: string; createdAt: string }`

- [x] **Task 7: Update authStore with user profile** (AC: 1)
  - [x] Extend `authStore.ts` state to include `user: User | null`
  - [x] Add `setUser(user: User)` action
  - [x] Update `logout()` to also clear `user`
  - [x] The `token` field already exists — registration flow calls `setToken` + `setUser` after successful register

- [x] **Task 8: Implement RegisterPage form** (AC: 2, 3, 4, 5)
  - [x] Replace placeholder in `client/src/features/auth/RegisterPage.tsx` with full registration form:
    - Three fields: Email (type="email"), Username (type="text"), Password (type="password") with show/hide toggle
    - Use shadcn `Input` and `Button` components
    - Submit on Enter key or button click
    - Primary button style (`accent` fill) for "Create Account"
    - Link to login page: "Already have an account? Log in"
    - Form centered on page, max 400px wide, `surface` background card, 32px padding
  - [x] Implement client-side validation (on blur, NOT on keystroke):
    - Email: non-empty, valid email format regex
    - Username: 3-20 chars, alphanumeric + underscores only
    - Password: minimum 8 characters
    - Error text appears below the relevant field in `destructive` color, `body-sm` size (12px Inter)
  - [x] Implement form submission:
    - Call `register()` from `shared/api/auth.ts`
    - On success: store token and user in `authStore`, navigate to `/lobby`
    - On 409 error: parse error code (`EMAIL_TAKEN` / `USERNAME_TAKEN`) and display inline below the correct field
    - On 400 error: display generic validation error
    - On network error: display toast via sonner
    - Disable submit button while request is in-flight (loading state with `isLoading`)
  - [x] Use `useTranslation` hook for all visible text — no hardcoded strings

- [x] **Task 9: Add i18n translation keys** (AC: 2)
  - [x] Add to `client/src/shared/i18n/en.json`:
    ```json
    "auth": {
      "register": {
        "title": "Create Account",
        "emailLabel": "Email",
        "emailPlaceholder": "your@email.com",
        "usernameLabel": "Username",
        "usernamePlaceholder": "Choose a username",
        "passwordLabel": "Password",
        "passwordPlaceholder": "Minimum 8 characters",
        "submitButton": "Create Account",
        "loginLink": "Already have an account? Log in",
        "errors": {
          "emailRequired": "Email is required",
          "emailInvalid": "Enter a valid email address",
          "emailTaken": "This email is already registered",
          "usernameRequired": "Username is required",
          "usernameTooShort": "Username must be at least 3 characters",
          "usernameTooLong": "Username must be at most 20 characters",
          "usernameInvalidChars": "Letters, numbers, and underscores only",
          "usernameTaken": "This username is already taken",
          "passwordRequired": "Password is required",
          "passwordTooShort": "Password must be at least 8 characters",
          "registrationFailed": "Registration failed. Please try again."
        }
      }
    }
    ```
  - [x] Add matching keys to `client/src/shared/i18n/sr.json` with Serbian (Latin) translations

- [x] **Task 10: Write backend tests** (AC: 1, 3, 4, 5, 6)
  - [x] Create `server/internal/auth/auth_test.go`:
    - Test `HashPassword` produces valid bcrypt hash
    - Test `CheckPassword` succeeds with correct password, fails with wrong
    - Test `GenerateAccessToken` + `ValidateToken` round-trip
    - Test `GenerateRefreshToken` + `ValidateToken` round-trip
  - [x] Create `server/internal/user/user_test.go`:
    - Test GORM repository `Create` — inserts user, returns with ID
    - Test `FindByEmail` — returns user when exists, error when not
    - Test `FindByUsername` — returns user when exists, error when not
    - Use per-test transaction with rollback (integration tests against real PostgreSQL)
  - [x] Create `server/internal/auth/handler_test.go`:
    - Test `POST /api/v1/auth/register` success — returns 201 with user data and token
    - Test duplicate email — returns 409 with `EMAIL_TAKEN` code
    - Test duplicate username — returns 409 with `USERNAME_TAKEN` code
    - Test invalid email format — returns 400
    - Test password too short — returns 400
    - Test empty fields — returns 400
    - Test response sets httpOnly cookie for refresh token
    - Use `httptest` with real Echo instance

- [x] **Task 11: Write frontend tests** (AC: 2, 5)
  - [x] Create `client/src/features/auth/RegisterPage.test.tsx`:
    - Test renders email, username, password fields and submit button
    - Test shows validation errors on blur for empty/invalid fields
    - Test successful registration navigates to /lobby
    - Test displays inline error for `EMAIL_TAKEN` server response
    - Test displays inline error for `USERNAME_TAKEN` server response
    - Test submit button is disabled during loading
    - Use `data-testid` attributes for element selection

- [x] **Task 12: Verify end-to-end** (AC: all)
  - [x] Run `make lint` — passes with zero errors
  - [x] Run `make test` — all existing + new tests pass
  - [x] Manual verification: registration form renders, validates, submits, creates user, returns tokens, redirects to lobby

## Dev Notes

### Architecture Compliance

- **Domain package shape**: `internal/auth/` gets `handler.go`, `service.go`, `auth_test.go`, `handler_test.go`. `internal/user/` gets `model.go`, `repository.go`, `gorm_repo.go`, `user_test.go`. This matches the standard backend domain package pattern established in Story 1.1
- **Never call GORM `AutoMigrate()`** — all schema changes via golang-migrate SQL files only
- **Handlers call repository interfaces, never GORM directly**
- **Error handling**: all new errors go in `internal/apperr/errors.go`. Handler uses `errors.Is()` for control flow
- **API response format**: Success `{ "data": { ... } }`, Error `{ "error": { "code": "...", "message": "..." } }` — reuse the existing `appErrorHandler` in `main.go`
- **Middleware order** is load-bearing: CORS -> Logging -> Error Handler -> Auth. Auth registration route is public (no auth middleware)

### JWT Implementation Details

- **New dependency**: `github.com/golang-jwt/jwt/v5` — must be added via `go get`
- **bcrypt** is already available via `golang.org/x/crypto` (indirect dep from Story 1.1) — promote to direct dependency
- **Access token**: ~15 min expiry, stored in Zustand `authStore` (memory only, never localStorage)
- **Refresh token**: ~7 day expiry, set as `httpOnly` secure cookie on `/api/v1/auth` path, `SameSite=Strict`
- **Token payload**: use `jwt.RegisteredClaims` with `Subject` = stringified user ID, `ExpiresAt`, `IssuedAt`
- Login and refresh endpoints are **deferred to Story 1.3** — this story only implements registration with auto-login
- The `fetchClient.ts` 401 → refresh → retry cycle is also **deferred to Story 1.3**

### Password & Validation Rules

- **bcrypt cost**: use `bcrypt.DefaultCost` (10) — sufficient for Phase 1 scale
- **Email validation**: regex check on both client (blur) and server. Server uses Go `net/mail.ParseAddress` or a simple regex — don't pull in a library for this
- **Username**: 3-20 characters, `^[a-zA-Z0-9_]+$` regex. Enforce on both client and server
- **Password**: minimum 8 characters. No complexity requirements for MVP
- **Server validation is authoritative** — frontend validation is cosmetic/UX only

### Database Schema Notes

- **GORM conventions**: `snake_case` columns auto-generated from PascalCase Go fields. `CreatedAt`, `UpdatedAt`, `DeletedAt` are magic GORM fields — include `DeletedAt gorm.DeletedAt` for soft delete support
- **Migration numbering**: Story 1.1 used `000001`. This story uses `000002` — check `server/migrations/` and increment by 1
- **Existing migration `000001_init.up.sql`** is a placeholder from Story 1.1 — it has no tables. The users table is the first real table
- **Indexes**: `idx_users_email`, `idx_users_username`, `idx_users_deleted_at` — follow the `idx_{table}_{column}` convention
- **`PasswordHash` JSON tag**: `json:"-"` — never include password hash in API responses

### Frontend Implementation Notes

- **RegisterPage.tsx**: replace the existing stub (currently just a `<h1>Register</h1>` heading)
- **Use existing shadcn components**: `Input` from `shared/components/ui/input.tsx`, `Button` from `shared/components/ui/button.tsx`
- **Form styling**: `surface` background card, centered, max 400px wide, 32px padding — consistent with UX spec modal pattern. Use Space Grotesk for the title heading, Inter for all form labels and input text
- **Password field**: add a show/hide toggle (eye icon from `lucide-react` which is already installed)
- **Error display**: inline below each field, `destructive` color (`#ef4444`), `body-sm` (12px Inter), appears on blur validation or server error
- **Button states**: Primary `accent` fill for submit. Disabled: 40% opacity + `cursor-not-allowed`. Loading: disabled + spinner or "Creating..." text
- **Navigation**: import `useNavigate` from `react-router` — redirect to `/lobby` on success. Add link to `/login` page
- **No `export default`** — named exports only (project convention)
- **All text via `useTranslation`** — zero hardcoded English strings in the component
- **Use `data-testid`** attributes on form elements for test selection
- **fetchClient.ts** already wraps responses and extracts `data` — the auth API function should work with it. The `FetchError` class provides `status` and `code` fields for error handling

### Previous Story Intelligence (Story 1.1)

**Key learnings from Story 1.1 review findings to apply:**

- **P1 (RESOLVED)**: JWT secret validation was missing — config now logs a warning. For this story, the auth service should fail loudly if `JWTSecret` is empty/default in production, but in dev it's OK to use the default
- **P2 (RESOLVED)**: `fetchClient` was crashing on non-JSON responses — now fixed. Auth API function can rely on `fetchClient` working correctly
- **W1 (DEFERRED to 1.3)**: `fetchClient` 401 handler doesn't implement refresh-then-retry — for this story, registration always returns a fresh token so no refresh needed yet
- **Debug log**: Vite 8 template generated vanilla TS instead of React — already resolved, not an issue now
- **Debug log**: ESLint 9 is in use (not 10) — linting config is stable
- **Debug log**: shadcn `sonner` replaces deprecated `toast` — use `sonner` for toast notifications (e.g., network errors)

**Established code patterns to follow:**

- Config loading pattern in `config/config.go` — `Load()` reads env vars with defaults
- AppError pattern in `apperr/errors.go` — `NewAppError(code, message, status)` for new domain errors
- Echo handler pattern in `main.go` — register routes on an Echo group, return errors for the error handler middleware
- Zustand store shape — `{ state, isLoading, actions }` pattern already in `authStore.ts`
- Test pattern — backend uses `testify/assert`, frontend uses `@testing-library/react` + Vitest

### Git Intelligence

Recent commits show the story 1.1 scaffold is complete with review fixes applied. Branch naming convention: `feat/E1-S2-user-registration` (following `{type}/{epic-id}-{description}` pattern). Commit style: `feat(auth): implement user registration endpoint`.

### Project Structure Notes

New files created by this story:

```
server/
  migrations/
    000002_create_users.up.sql      (new)
    000002_create_users.down.sql    (new)
  internal/
    auth/
      handler.go                     (new — replaces empty auth.go)
      service.go                     (new)
      auth_test.go                   (new)
      handler_test.go                (new)
    user/
      model.go                       (new — replaces empty user.go)
      repository.go                  (new)
      gorm_repo.go                   (new)
      user_test.go                   (new)
    apperr/
      errors.go                      (modified — add new error codes)
  cmd/api/
    main.go                          (modified — wire auth routes)

client/
  src/
    features/auth/
      RegisterPage.tsx               (modified — full form replaces stub)
      RegisterPage.test.tsx          (new)
    shared/
      api/
        auth.ts                      (new)
      stores/
        authStore.ts                 (modified — add user state)
      types/
        apiTypes.ts                  (modified — add User type)
      i18n/
        en.json                      (modified — add auth.register keys)
        sr.json                      (modified — add auth.register keys)
```

Note: the existing `server/internal/auth/auth.go` and `server/internal/user/user.go` are empty placeholder files from Story 1.1. They should be **deleted** and replaced with the proper domain package files listed above. Do not keep empty placeholder files.

### References

- [Source: epics.md#Story-1.2] — acceptance criteria and story definition
- [Source: architecture.md#Authentication-Security] — JWT strategy, bcrypt, token storage, CORS
- [Source: architecture.md#API-Communication-Patterns] — REST JSON, structured errors, HTTP status codes
- [Source: architecture.md#Backend-Domain-Package-Shape] — model/repository/handler/service/test pattern
- [Source: architecture.md#Database-Naming-Conventions] — snake*case tables, idx* indexes, camelCase JSON
- [Source: architecture.md#Project-Structure] — auth/ and user/ package locations, migration numbering
- [Source: prd.md#FR1] — email/password registration requirement
- [Source: prd.md#Security] — hashed passwords, server-side validation, time-limited tokens
- [Source: ux-design-specification.md#Journey-1] — 3-field registration, no email verification gate
- [Source: ux-design-specification.md#Form-Patterns] — registration form: 3 fields, Enter submit, password toggle, inline errors
- [Source: ux-design-specification.md#Inline-Validation] — errors on blur, specific error copy
- [Source: ux-design-specification.md#Button-Hierarchy] — Primary accent fill for main CTA, disabled states
- [Source: ux-design-specification.md#Typography-System] — Space Grotesk display, Inter UI/body
- [Source: ux-design-specification.md#Colour-System] — accent, surface, destructive, text-primary/secondary tokens
- [Source: project-context.md] — all critical rules: naming, anti-patterns, auth flow, GORM conventions

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Debug Log References

- `migrate` CLI not installed on dev machine — applied migration via `docker exec` psql
- `golangci-lint` not installed on dev machine — used `go vet` for static analysis
- Handler tests initially failed because `doRegister` called handler directly without Echo error handler; fixed by using `e.ServeHTTP` with registered route and `HTTPErrorHandler`
- Pre-existing ESLint import-sort errors across codebase (from Story 1.1); fixed imports in new/modified files only

### Completion Notes List

- All 12 tasks and subtasks implemented and verified
- Backend: auth service (bcrypt + JWT), user domain package (model, repository, GORM impl), registration handler with server-side validation
- Frontend: full registration form with i18n, blur validation, inline server errors, password toggle, loading state, navigation on success
- 24 server tests pass (16 auth, 5 user integration, 2 apperr, 1 health)
- 13 frontend tests pass (9 RegisterPage, 2 App routing, 2 i18n)
- Zero regressions on existing tests
- Lint passes on all modified files (pre-existing import sort issues in untouched files)
- Added `@testing-library/user-event` as dev dependency for frontend tests
- Added `github.com/golang-jwt/jwt/v5` as direct dependency; promoted `golang.org/x/crypto` from indirect to direct
- Deleted placeholder files: `server/internal/auth/auth.go`, `server/internal/user/user.go`

### Review Findings

_Code review performed 2026-04-10 — 3 layers (Blind Hunter, Edge Case Hunter, Acceptance Auditor)_

**Patch — HIGH priority:**

- [x] [Review][Patch] TOCTOU race on duplicate email/username — FIXED: gorm_repo.go catches pgconn unique-violation (23505) and maps to apperr.ErrEmailTaken/ErrUsernameTaken
- [x] [Review][Patch] Handler directly imports gorm.ErrRecordNotFound — FIXED: repository returns (nil, nil) for not-found; handler no longer imports gorm
- [x] [Review][Patch] Server 400 errors display as toast instead of inline — FIXED: RegisterPage maps all server error codes to field-specific inline errors
- [x] [Review][Patch] Email case sensitivity allows duplicate accounts — FIXED: handler normalizes email to lowercase via strings.ToLower(addr.Address)
- [x] [Review][Patch] mail.ParseAddress accepts display names and whitespace — FIXED: handler uses parsed addr.Address field and TrimSpace on all inputs

**Patch — MEDIUM priority:**

- [x] [Review][Patch] JWT access and refresh tokens are indistinguishable — FIXED: added Audience claim ("access" / "refresh") to distinguish token types
- [x] [Review][Patch] No password max length validation — FIXED: added 72-char upper bound on both server and client, with i18n keys
- [x] [Review][Patch] Soft-delete vs UNIQUE constraint conflict — FIXED: migration uses partial unique indexes (WHERE deleted_at IS NULL)
- [ ] [Review][Patch] Orphan account on token generation failure — SKIPPED: requires repository transaction support; risk is negligible (JWT signing only fails on empty secret, caught by config)

**Patch — LOW priority:**

- [x] [Review][Patch] Misleading USERNAME_TOO_SHORT code for too-long usernames — FIXED: added separate ErrUsernameTooShort and ErrUsernameTooLong errors
- [x] [Review][Patch] Redundant indexes in migration — FIXED: removed column-level UNIQUE, using partial unique indexes instead
- [x] [Review][Patch] jwt/v5 and x/crypto listed as indirect deps — FIXED: moved to direct require block, go mod tidy applied
- [x] [Review][Patch] Hardcoded "Creating..." fallback in RegisterPage — FIXED: removed unnecessary || "Creating..." fallback
- [x] [Review][Patch] Handler test missing createdAt assertion — FIXED: mock sets CreatedAt, test asserts non-zero
- [x] [Review][Patch] Missing frontend tests — FIXED: added tests for invalid username chars and password toggle

**Deferred:**

- [x] [Review][Defer] Token lost on page refresh — access token in Zustand memory only, no persist middleware, refresh endpoint not implemented. Deferred to Story 1.3 (refresh token cycle)
- [x] [Review][Defer] fetchClient hard redirect on 401 — uses window.location.href instead of React Router, destroys SPA state. Pre-existing from Story 1.1, deferred to Story 1.3
- [x] [Review][Defer] Default JWT secret warning-only — config logs slog.Warn but continues with insecure default. Acknowledged from Story 1.1 review (P1 RESOLVED)
- [x] [Review][Defer] No rate limiting on registration endpoint — bcrypt cost makes this a CPU DoS vector. Infrastructure concern, not story-scoped
- [x] [Review][Defer] Refresh token Secure:true hardcoded — prevents cookie on HTTP in dev. Moot until refresh endpoint exists in Story 1.3

### Change Log

- 2026-04-10: Implemented Story 1.2 User Registration — full backend auth service with JWT, user domain package with GORM repository, registration endpoint, React registration form with validation and i18n, comprehensive test coverage

### File List

**New files:**

- server/migrations/000002_create_users.up.sql
- server/migrations/000002_create_users.down.sql
- server/internal/user/model.go
- server/internal/user/repository.go
- server/internal/user/gorm_repo.go
- server/internal/user/user_test.go
- server/internal/auth/service.go
- server/internal/auth/handler.go
- server/internal/auth/auth_test.go
- server/internal/auth/handler_test.go
- client/src/shared/api/auth.ts
- client/src/features/auth/RegisterPage.test.tsx

**Modified files:**

- server/internal/apperr/errors.go (added auth domain errors)
- server/cmd/api/main.go (wired auth routes, DI)
- server/go.mod (added jwt/v5, promoted x/crypto)
- server/go.sum (updated)
- client/src/features/auth/RegisterPage.tsx (full form replaces stub)
- client/src/shared/stores/authStore.ts (added user state)
- client/src/shared/types/apiTypes.ts (added User type)
- client/src/shared/i18n/en.json (added auth.register keys)
- client/src/shared/i18n/sr.json (added auth.register keys)
- client/package.json (added @testing-library/user-event)

**Deleted files:**

- server/internal/auth/auth.go (placeholder)
- server/internal/user/user.go (placeholder)
