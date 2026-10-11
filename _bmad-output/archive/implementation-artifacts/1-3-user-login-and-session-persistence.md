# Story 1.3: User Login & Session Persistence

Status: done

## Story

As a registered player,
I want to log in and remain authenticated across page refreshes,
so that I don't need to re-enter my credentials every time I visit the platform.

## Acceptance Criteria

1. **Given** a player has a registered account
   **When** they submit correct email and password on the login page
   **Then** the server returns a JWT access token (~15 min lifetime) in the response body
   **And** a refresh token (~7 day lifetime) is set as an httpOnly secure cookie
   **And** the Zustand auth store is populated with the access token and user profile
   **And** the player is redirected to the lobby

2. **Given** invalid credentials are submitted
   **When** the server processes the login request
   **Then** a 401 error is returned with a generic "Invalid email or password" message (no information leakage about which field is wrong)

3. **Given** a player is authenticated and refreshes the page
   **When** the app initializes
   **Then** the client calls `POST /api/v1/auth/refresh` (httpOnly cookie auto-sent)
   **And** the server returns a new access token and user profile
   **And** the Zustand auth store is repopulated and the player remains on their current page

4. **Given** the refresh token has expired
   **When** the client calls `/api/v1/auth/refresh`
   **Then** the server returns 401
   **And** the player is redirected to the login page with auth store cleared

5. **Given** a player clicks logout
   **When** the logout action is triggered
   **Then** the Zustand auth store is cleared, the refresh cookie is invalidated server-side, and the player is redirected to the login page

6. **Given** an unauthenticated user navigates to a protected route (lobby, profile, game)
   **When** the route guard evaluates
   **Then** the user is redirected to the login page

## Tasks / Subtasks

- [x] **Task 1: Implement login endpoint** (AC: 1, 2)
  - [x] Add `POST /api/v1/auth/login` handler to `server/internal/auth/handler.go`
  - [x] Define `LoginRequest` struct: `Email string`, `Password string`
  - [x] Define `LoginResponseData` struct: reuse `RegisterResponseData` shape (ID, Username, Email, LanguagePreference, CreatedAt, Token)
  - [x] Normalize email: `strings.ToLower(strings.TrimSpace(input))` — same as registration
  - [x] Look up user by email via `UserRepository.FindByEmail`
  - [x] If user not found OR password check fails: return `apperr.ErrInvalidCredentials` (401) — single error for both cases to prevent enumeration
  - [x] On success: generate access token + refresh token, set refresh cookie (same pattern as Register), return 200 with user data + access token

- [x] **Task 2: Implement refresh endpoint** (AC: 3, 4)
  - [x] Add `POST /api/v1/auth/refresh` handler to `server/internal/auth/handler.go`
  - [x] Read `refresh_token` from httpOnly cookie via `c.Cookie("refresh_token")`
  - [x] Validate token via `ValidateToken` — check signature AND audience claim equals `"refresh"`
  - [x] If invalid/expired/missing: clear the refresh cookie (set MaxAge=-1) and return 401
  - [x] Extract user ID from token `Subject` claim
  - [x] Look up user by ID via `UserRepository.FindByID` — return 401 if user not found (deleted/banned)
  - [x] Generate new access token (do NOT rotate refresh token for Phase 1 — rotation adds complexity without benefit at this scale)
  - [x] Return 200 with new access token + user profile data

- [x] **Task 3: Implement logout endpoint** (AC: 5)
  - [x] Add `POST /api/v1/auth/logout` handler to `server/internal/auth/handler.go`
  - [x] Clear the refresh cookie: set `refresh_token` cookie with `MaxAge: -1`, same path/domain/flags
  - [x] Return 200 with `{ "data": { "message": "logged out" } }`
  - [x] This endpoint is public (no auth middleware) — logout should always succeed even with expired tokens

- [x] **Task 4: Add auth domain errors** (AC: 2)
  - [x] Add to `server/internal/apperr/errors.go`:
    - `ErrInvalidCredentials = NewAppError("INVALID_CREDENTIALS", "invalid email or password", 401)`
  - [x] Note: `ErrUnauthorized` (401) already exists and is sufficient for middleware/refresh token failures. Do NOT add separate `ErrTokenExpired` / `ErrTokenInvalid` — the client does not need to differentiate these; all trigger the same refresh-or-redirect flow

- [x] **Task 5: Implement auth middleware** (AC: 6)
  - [x] Create `server/internal/auth/middleware.go`
  - [x] Implement `AuthMiddleware(jwtSecret string) echo.MiddlewareFunc` that:
    - Extracts token from `Authorization: Bearer <token>` header
    - Validates token via `ValidateToken` from `service.go`
    - Checks audience contains `"access"` — `ValidateToken` returns `*jwt.RegisteredClaims` where `Audience` is type `jwt.ClaimStrings` (a `[]string`). Check via: `!claims.Audience.Contains("access")` — this is NOT string equality, it's a method call on the slice type
    - On success: sets user ID in Echo context via `c.Set("userID", userID)` (parse Subject as uint via `strconv.ParseUint`)
    - On failure: returns `apperr.ErrUnauthorized` (401)
  - [x] Export a helper: `GetUserID(c echo.Context) (uint, error)` to extract user ID from context safely

- [x] **Task 6: Wire routes and middleware in main.go** (AC: 1, 6)
  - [x] Register `POST /api/v1/auth/login` — public (no auth middleware)
  - [x] Register `POST /api/v1/auth/refresh` — public (cookie-based auth, not Bearer)
  - [x] Register `POST /api/v1/auth/logout` — public (must work with expired tokens)
  - [x] Create an authenticated route group: `api.Group("", auth.AuthMiddleware(cfg.JWTSecret))`
  - [x] Move future protected endpoints to this group (no protected endpoints exist yet in this story, but the group must be ready)
  - [x] Maintain middleware order: CORS → Logging → Error Handler → routes (auth middleware applied per-group, not globally)

- [x] **Task 7: Make refresh cookie Secure flag environment-aware** (AC: 3, fixes D5)
  - [x] Add `Environment` field to `config.Config` struct — read from `BELJOT_ENV` env var, default `"development"`
  - [x] Create a helper method on `AuthHandler`: `setRefreshCookie(c echo.Context, token string)` and `clearRefreshCookie(c echo.Context)`
  - [x] Set `Secure: true` only when `Environment != "development"` — allows HTTP in local dev
  - [x] Update the Register handler to use the same `setRefreshCookie` helper (DRY up the existing cookie-setting code)

- [x] **Task 8: Create frontend auth API functions** (AC: 1, 3, 5)
  - [x] Add to `client/src/shared/api/auth.ts`:
    - `login(data: { email: string; password: string }): Promise<LoginResponse>` — POST `/api/v1/auth/login`
    - `refresh(): Promise<RefreshResponse>` — POST `/api/v1/auth/refresh` with `credentials: "include"` (cookie auto-sent)
    - `logout(): Promise<void>` — POST `/api/v1/auth/logout` with `credentials: "include"`
  - [x] Define types: `LoginResponse` (same shape as `RegisterResponse`), `RefreshResponse: { token: string; id: number; username: string; email: string; languagePreference: string; createdAt: string }`
  - [x] `login()` goes through `fetchClient` — this is a normal API call; `fetchClient` already parses errors into `FetchError` with `status`, `code`, `message` fields. LoginPage handles `FetchError` by checking `error.status === 401` for credentials error, and falls back to toast for other failures
  - [x] `refresh()` must NOT go through `fetchClient` — it calls `fetch` directly to avoid the 401-intercept loop (fetchClient's 401 handler calls refresh, which would call fetchClient, which would call refresh...). Must manually parse JSON response and throw on non-200
  - [x] `logout()` uses `fetch` directly — no auth header needed, just the cookie. Fire-and-forget (don't await response)

- [x] **Task 9: Implement fetchClient refresh-then-retry cycle** (AC: 3, fixes W1 + D2)
  - [x] Refactor `client/src/shared/api/fetchClient.ts`:
    - On 401 response: call `refresh()` from `auth.ts` (NOT fetchClient — direct fetch)
    - If refresh succeeds: store new token in authStore, retry the original request with the new token **exactly once** — if the retry also returns 401, do NOT attempt another refresh; proceed to logout/redirect
    - If refresh fails: clear authStore, redirect to `/login` via a callback (not `window.location.href`)
    - Use a module-level `refreshPromise` variable to deduplicate concurrent refresh calls — if a refresh is already in-flight, other 401s await the same promise
    - Track retry state: pass an internal `_isRetry?: boolean` flag to prevent infinite loops — if `_isRetry` is true and response is 401, skip refresh and throw immediately
  - [x] Replace `window.location.href = "/login"` with a configurable redirect callback
  - [x] Export `setAuthRedirect(fn: () => void)` from `fetchClient.ts` — called by `useAuthInit()` hook (Task 10) which has access to React Router's `navigate`

- [x] **Task 10: Implement app initialization with session restore** (AC: 3, 4)
  - [x] Create `client/src/shared/hooks/useAuth.ts` with:
    - `useAuthInit()` hook — called once in App.tsx on mount
    - On mount: if no token in authStore, call `refresh()` to attempt session restore from cookie
    - If refresh succeeds: populate authStore with token + user, sync i18n language via `i18n.changeLanguage(user.languagePreference)`, app renders normally
    - If refresh fails: clear authStore (user stays on public routes or gets redirected by route guard)
    - Set `isLoading: true` during the attempt, `false` when done
    - Wire up the auth redirect callback here: `setAuthRedirect(() => navigate("/login"))` — this is the **single canonical location** for setting the redirect callback
  - [x] In App.tsx: call `useAuthInit()` and show a loading state (simple centered spinner or blank) while `authStore.isLoading` is true — prevents route flash

- [x] **Task 11: Implement ProtectedRoute component** (AC: 6)
  - [x] Create `client/src/shared/components/ProtectedRoute.tsx`
  - [x] Component checks `authStore.token` — if null and not loading, redirect to `/login` via `<Navigate to="/login" replace />`
  - [x] If loading (session restore in progress), render nothing or a minimal loader
  - [x] If authenticated, render `<Outlet />`
  - [x] Update `App.tsx` routes: wrap `/lobby`, `/profile`, `/game` inside a `<Route element={<ProtectedRoute />}>` parent

- [x] **Task 12: Implement LoginPage** (AC: 1, 2)
  - [x] Replace stub in `client/src/features/auth/LoginPage.tsx` with full login form:
    - Two fields: Email (type="email"), Password (type="password") with show/hide toggle
    - Use shadcn `Input` and `Button` components — same styling as RegisterPage
    - Submit on Enter key or button click
    - Primary `accent` fill button: "Log In"
    - Link to register page: "Don't have an account? Create one"
    - Form centered on page, max 400px wide, `surface` background card, 32px padding
  - [x] Implement client-side validation (on blur):
    - Email: non-empty, valid email format
    - Password: non-empty
    - Error text below field in `destructive` color, `body-sm` (12px Inter)
  - [x] Implement form submission:
    - Call `login()` from `shared/api/auth.ts`
    - On success: store token + user in authStore, navigate to `/lobby`
    - On 401 error: display generic "Invalid email or password" below the form (NOT field-specific — prevents enumeration)
    - On network error: display toast via sonner
    - Disable submit button while request in-flight
  - [x] Use `useTranslation` hook for all visible text
  - [x] Use `data-testid` attributes on form elements

- [x] **Task 13: Update authStore with logout action** (AC: 5)
  - [x] Extend `authStore.ts`:
    - Update `logout()` to call the backend `POST /api/v1/auth/logout` (fire-and-forget — don't block on response) before clearing state
    - The redirect to `/login` is handled by `setAuthRedirect` callback, not by the store

- [x] **Task 14: Add i18n translation keys** (AC: 1, 2)
  - [x] Add to `client/src/shared/i18n/en.json` under `auth.login`:
    ```json
    "login": {
      "title": "Log In",
      "emailLabel": "Email",
      "emailPlaceholder": "your@email.com",
      "passwordLabel": "Password",
      "passwordPlaceholder": "Enter your password",
      "submitButton": "Log In",
      "registerLink": "Don't have an account? Create one",
      "errors": {
        "emailRequired": "Email is required",
        "emailInvalid": "Enter a valid email address",
        "passwordRequired": "Password is required",
        "invalidCredentials": "Invalid email or password"
      }
    }
    ```
  - [x] Add matching keys to `client/src/shared/i18n/sr.json` with Serbian (Latin) translations

- [x] **Task 15: Write backend tests** (AC: 1, 2, 3, 4, 5, 6)
  - [x] Add to `server/internal/auth/handler_test.go`:
    - Test `POST /api/v1/auth/login` success — returns 200 with user data, access token, and sets refresh cookie
    - Test login with wrong password — returns 401 with INVALID_CREDENTIALS
    - Test login with non-existent email — returns 401 with INVALID_CREDENTIALS (same error as wrong password)
    - Test login normalizes email to lowercase
    - Test `POST /api/v1/auth/refresh` success — returns 200 with new access token and user profile
    - Test refresh with expired cookie — returns 401
    - Test refresh with missing cookie — returns 401
    - Test refresh with access token (wrong audience) — returns 401
    - Test refresh for deleted user — returns 401
    - Test refresh clears cookie on failure
    - Test `POST /api/v1/auth/logout` — returns 200 and clears cookie
  - [x] Add to `server/internal/auth/auth_test.go`:
    - Test `ValidateToken` rejects token with wrong audience
  - [x] Create `server/internal/auth/middleware_test.go`:
    - Test middleware allows valid access token — sets userID in context
    - Test middleware rejects expired token — returns 401
    - Test middleware rejects missing Authorization header — returns 401
    - Test middleware rejects malformed Bearer token — returns 401
    - Test middleware rejects refresh token (wrong audience) — returns 401
    - Test `GetUserID` helper extracts user ID from context

- [x] **Task 16: Write frontend tests** (AC: 1, 2, 6)
  - [x] Create `client/src/features/auth/LoginPage.test.tsx`:
    - Test renders email and password fields with submit button
    - Test shows validation errors on blur for empty fields
    - Test successful login navigates to /lobby
    - Test displays generic error for 401 response (not field-specific)
    - Test submit button disabled during loading
    - Test password toggle works
    - Test "Create one" link navigates to /register
  - [x] Create `client/src/shared/components/ProtectedRoute.test.tsx`:
    - Test redirects to /login when no token
    - Test renders children when token present
    - Test shows loader during auth initialization
  - [x] Add authStore logout test to an appropriate test file:
    - Test `logout()` calls backend `/auth/logout` endpoint
    - Test `logout()` clears token and user from store

- [x] **Task 17: Verify end-to-end** (AC: all)
  - [x] Run `make lint` — passes with zero errors
  - [x] Run `make test` — all existing + new tests pass
  - [x] Manual verification: login form renders, validates, submits, authenticates, redirects to lobby
  - [x] Manual verification: page refresh calls /refresh and restores session
  - [x] Manual verification: protected routes redirect to login when not authenticated
  - [x] Manual verification: logout clears session and redirects to login

## Dev Notes

### Architecture Compliance

- **Domain package shape**: All new backend code goes in existing `internal/auth/` package — `handler.go` (add login/refresh/logout handlers), `middleware.go` (new file), `service.go` (no changes needed — `ValidateToken`, `CheckPassword`, `GenerateAccessToken`, `GenerateRefreshToken` already exist)
- **Never call GORM directly from handlers** — use `UserRepository` interface
- **All errors in `internal/apperr/errors.go`** — add `ErrInvalidCredentials` only; reuse existing `ErrUnauthorized` for all token failures
- **API response format**: Success `{ "data": { ... } }`, Error `{ "error": { "code": "...", "message": "..." } }`
- **Middleware order is load-bearing**: CORS → Logging → Error Handler → routes. Auth middleware is applied per-group, not globally
- **No `export default`** — named exports only (TypeScript convention)
- **All text via `useTranslation`** — zero hardcoded strings
- **`data-testid`** on interactive elements for test selection

### JWT Implementation Details (Already Built in Story 1.2)

These functions exist in `server/internal/auth/service.go` — reuse them, do NOT reimplement:

- `HashPassword(password string) (string, error)` — bcrypt
- `CheckPassword(hash, password string) error` — bcrypt compare
- `GenerateAccessToken(userID uint, secret string) (string, error)` — 15 min, audience="access"
- `GenerateRefreshToken(userID uint, secret string) (string, error)` — 7 day, audience="refresh"
- `ValidateToken(tokenString, secret string) (*jwt.RegisteredClaims, error)` — parse + validate

**Audience claim enforcement**: Story 1.2 already adds `Audience: jwt.ClaimStrings{"access"}` or `jwt.ClaimStrings{"refresh"}` to tokens. `jwt.ClaimStrings` is a `[]string` type with a `.Contains(string) bool` method. The middleware MUST check `claims.Audience.Contains("access")` — do NOT use `claims.Audience[0] == "access"` (index could panic) or string equality (wrong type). The refresh endpoint likewise checks `.Contains("refresh")`.

### Cookie Handling Pattern

Story 1.2's Register handler already sets the refresh cookie. Extract this into a shared helper on `AuthHandler` to DRY up across Register, Login, Refresh, and Logout:

```go
func (h *AuthHandler) setRefreshCookie(c echo.Context, token string) {
    c.SetCookie(&http.Cookie{
        Name:     "refresh_token",
        Value:    token,
        Path:     "/api/v1/auth",
        HttpOnly: true,
        Secure:   h.env != "development",
        SameSite: http.SameSiteStrictMode,
        MaxAge:   7 * 24 * 60 * 60,
    })
}

func (h *AuthHandler) clearRefreshCookie(c echo.Context) {
    c.SetCookie(&http.Cookie{
        Name:     "refresh_token",
        Value:    "",
        Path:     "/api/v1/auth",
        HttpOnly: true,
        Secure:   h.env != "development",
        SameSite: http.SameSiteStrictMode,
        MaxAge:   -1,
    })
}
```

Update `NewAuthHandler` to accept environment string from config.

### Login Security — Anti-Enumeration

The login endpoint MUST return the same error (`ErrInvalidCredentials`) for both "user not found" and "wrong password" cases. Never reveal whether an email is registered. This is the standard defense against email enumeration attacks.

The `UserRepository.FindByEmail` returns `(nil, nil)` for not-found (established in Story 1.2). Check: if user is nil OR `CheckPassword` fails → same 401 error.

### fetchClient Refresh-Retry Architecture

The refresh cycle must avoid infinite loops and handle concurrent requests:

```
Request → 401 → Is this a retry (_isRetry flag)?
  ├─ Yes → throw immediately (do NOT refresh again — prevents infinite loop)
  └─ No  → Check: is refresh already in-flight?
             ├─ Yes → await existing refreshPromise
             └─ No  → set refreshPromise = refresh()
                          ├─ Success → store new token → retry original request (with _isRetry=true)
                          └─ Failure → clear authStore → redirect to /login
```

**Critical**: The `refresh()` API call must use raw `fetch()`, NOT `fetchClient` — otherwise a 401 from the refresh endpoint triggers another refresh attempt → infinite loop.

**Concurrent deduplication**: Use a module-level `let refreshPromise: Promise<...> | null = null` in `fetchClient.ts`. When the first 401 sets it, all subsequent 401s during that window await the same promise. After resolution (success or failure), reset to null.

### Session Restore on Page Refresh

When the app mounts, the access token is gone (Zustand memory-only — intentional per architecture). The `useAuthInit()` hook:

1. Sets `authStore.isLoading = true`
2. Calls `refresh()` (the httpOnly cookie is auto-sent)
3. If success: `setToken(newToken)` + `setUser(userProfile)` + `isLoading = false`
4. If failure (401 or network error): `logout()` + `isLoading = false`

**App.tsx must gate rendering on `!isLoading`** — otherwise ProtectedRoute will see `token === null` during the refresh attempt and flash-redirect to login before the token arrives.

### Auth Redirect Callback Pattern (Fixes D2)

Story 1.2 left a deferred issue: `fetchClient.ts` uses `window.location.href = "/login"` which destroys SPA state. Fix:

- Export `setAuthRedirect(fn: () => void)` from `fetchClient.ts`
- In `useAuthInit()` (inside App.tsx context): `setAuthRedirect(() => navigate("/login"))`
- `fetchClient` and `authStore.logout()` call the registered callback instead of `window.location.href`
- Fallback: if callback not yet registered (edge case during app init), use `window.location.href` as last resort

### ProtectedRoute Pattern

Use React Router v7's nested route pattern with `<Outlet />`:

```tsx
// App.tsx routes
<Route element={<ProtectedRoute />}>
  <Route path="/lobby" element={<LobbyPage />} />
  <Route path="/profile" element={<ProfilePage />} />
  <Route path="/game" element={<GamePage />} />
</Route>
```

The `ProtectedRoute` component:

- Reads `token` and `isLoading` from `authStore`
- If `isLoading`: render nothing (or minimal spinner) — session restore in progress
- If `!token && !isLoading`: `<Navigate to="/login" replace />`
- If `token`: `<Outlet />`

### Deferred Items Resolved by This Story

| ID  | Issue                                    | Resolution                                                              |
| --- | ---------------------------------------- | ----------------------------------------------------------------------- |
| D1  | Token lost on page refresh               | Session restore via `useAuthInit()` → `POST /auth/refresh` on app mount |
| D2  | fetchClient hard redirect breaks SPA     | `setAuthRedirect` callback using React Router navigate                  |
| D5  | Refresh cookie Secure:true hardcoded     | Environment-aware via `BELJOT_ENV` config                               |
| W1  | fetchClient 401 lacks refresh-then-retry | Full refresh cycle with concurrent deduplication                        |

### Deferred Items NOT Addressed (Out of Scope)

| ID  | Issue                              | Why Deferred                                                         |
| --- | ---------------------------------- | -------------------------------------------------------------------- |
| W2  | apperr.Wrap() wraps raw error      | Utility concern, not auth-specific                                   |
| W3  | ErrorBoundary retry loop           | UX concern, not auth-specific                                        |
| D3  | JWT secret default only warns      | Infrastructure concern, acceptable for Phase 1                       |
| D4  | No rate limiting on auth endpoints | Infrastructure concern (nginx/Caddy rate limiting), not story-scoped |

### Previous Story Intelligence (Story 1.2)

**Key patterns established that MUST be followed:**

- Email normalization: `strings.ToLower(strings.TrimSpace())` — apply to login input too
- Cookie settings: path `/api/v1/auth`, httpOnly, SameSite=Strict — keep identical
- Handler tests use `httptest` with real Echo instance and `e.ServeHTTP` — do NOT call handlers directly
- Frontend tests use `@testing-library/react` + `@testing-library/user-event` + Vitest
- Frontend form pattern: centered card, max 400px, `surface` bg, 32px padding, Space Grotesk title, Inter body
- Error display: inline below fields for validation, toast for network errors
- Button: primary `accent` fill, disabled 40% opacity + cursor-not-allowed, loading text variant
- Password field: show/hide toggle via lucide-react eye icon

**Review fixes from Story 1.2 that inform this story:**

- TOCTOU race on duplicate checks → already fixed in `gorm_repo.go` with pgconn constraint handling
- JWT audience claim → already added (access/refresh distinction) — middleware must enforce it
- Email case sensitivity → already fixed with `strings.ToLower` — apply same in login
- `net/mail.ParseAddress` returns `addr.Address` field — use the parsed address, not raw input

### Git Intelligence

Recent commits follow pattern: `feat(auth): implement user registration with code review fixes`. For this story:

- Branch name: `feat/E1-S3-user-login`
- Commit style: `feat(auth): implement user login and session persistence`

### Project Structure Notes

Files created/modified by this story:

```
server/
  internal/
    auth/
      handler.go                     (modified — add Login, Refresh, Logout handlers + cookie helpers)
      middleware.go                   (new — JWT auth middleware + GetUserID helper)
      middleware_test.go              (new)
      handler_test.go                (modified — add login/refresh/logout tests)
      auth_test.go                   (modified — add audience validation test)
    apperr/
      errors.go                      (modified — add ErrInvalidCredentials, ErrTokenExpired, ErrTokenInvalid)
    config/
      config.go                      (modified — add Environment field)
  cmd/api/
    main.go                          (modified — wire login/refresh/logout routes, add authenticated group)

client/
  src/
    features/auth/
      LoginPage.tsx                  (modified — full form replaces stub)
      LoginPage.test.tsx             (new)
    shared/
      api/
        auth.ts                      (modified — add login(), refresh(), logout() functions)
        fetchClient.ts               (modified — add refresh-retry cycle, setAuthRedirect)
      hooks/
        useAuth.ts                   (new — useAuthInit hook for session restore)
      stores/
        authStore.ts                 (modified — update logout to call backend)
      components/
        ProtectedRoute.tsx           (new)
        ProtectedRoute.test.tsx      (new)
      i18n/
        en.json                      (modified — add auth.login keys)
        sr.json                      (modified — add auth.login keys)
    App.tsx                          (modified — wrap protected routes, call useAuthInit)
```

### References

- [Source: epics.md#Story-1.3] — acceptance criteria and story definition
- [Source: architecture.md#Authentication-Security] — JWT strategy, access/refresh tokens, httpOnly cookie, WebSocket auth flow
- [Source: architecture.md#API-Communication-Patterns] — REST JSON, structured errors, HTTP status codes
- [Source: architecture.md#Frontend-Architecture] — Zustand partitioned stores, authStore lifecycle
- [Source: architecture.md#Backend-Domain-Package-Shape] — handler/service/middleware pattern
- [Source: architecture.md#Project-Structure] — auth/ package, middleware.go, useAuth.ts, fetchClient.ts
- [Source: prd.md#FR2] — login and session persistence requirement
- [Source: prd.md#Security] — time-limited tokens, secure refresh mechanisms
- [Source: ux-design-specification.md#Journey-1] — registration + first login flow
- [Source: ux-design-specification.md#Form-Patterns] — form styling, inline errors, button states
- [Source: project-context.md#Authentication-Flow] — JWT in memory, refresh in httpOnly cookie, fetchClient owns 401 cycle
- [Source: project-context.md#Anti-Patterns] — never store JWT in localStorage
- [Source: 1-2-user-registration.md#Review-Findings] — deferred items D1, D2, D5, W1 scoped to this story

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Debug Log References

- `jwt.ClaimStrings` in golang-jwt/jwt/v5 does not have a `.Contains()` method. Used `slices.Contains([]string(claims.Audience), "access")` instead.
- `TestRegister_SetsRefreshCookie` needed update: `Secure` flag is now `false` in development environment (intentional change from Task 7).

### Completion Notes List

- All 17 tasks completed successfully
- Backend: 38 tests pass (auth package) — login, refresh, logout handlers + auth middleware + service tests
- Frontend: 28 tests pass across 6 test files — LoginPage, ProtectedRoute, authStore, App routing, RegisterPage, i18n
- TypeScript compiles with zero errors
- Deferred items D1, D2, D5, W1 from Story 1.2 resolved
- Auth middleware ready for protected routes (authenticated group wired in main.go)
- Session restore on page refresh implemented via useAuthInit hook
- fetchClient refresh-retry cycle with concurrent deduplication implemented

### Change Log

- 2026-04-11: Implemented user login, token refresh, logout, auth middleware, session persistence, protected routes, and LoginPage (Story 1.3)

### File List

**Backend (modified):**

- server/internal/auth/handler.go — added Login, Refresh, Logout handlers + setRefreshCookie/clearRefreshCookie helpers; refactored AuthHandler to accept env param
- server/internal/auth/handler_test.go — added 11 new tests for login/refresh/logout; updated setupHandler and cookie test
- server/internal/auth/auth_test.go — added audience validation test
- server/internal/apperr/errors.go — added ErrInvalidCredentials
- server/internal/config/config.go — added Environment field
- server/cmd/api/main.go — wired login/refresh/logout routes + authenticated group

**Backend (new):**

- server/internal/auth/middleware.go — JWT auth middleware + GetUserID helper
- server/internal/auth/middleware_test.go — 7 middleware tests

**Frontend (modified):**

- client/src/shared/api/auth.ts — added login(), refresh(), logout() functions + types
- client/src/shared/api/fetchClient.ts — added refresh-retry cycle, setAuthRedirect, concurrent deduplication
- client/src/shared/stores/authStore.ts — logout calls backend; isLoading defaults to true
- client/src/features/auth/LoginPage.tsx — full login form replacing stub
- client/src/App.tsx — added useAuthInit, isLoading gate, ProtectedRoute wrapper
- client/src/shared/i18n/en.json — added auth.login translation keys
- client/src/shared/i18n/sr.json — added auth.login Serbian translations
- client/src/App.test.tsx — updated for new LoginPage (uses data-testid + i18n)

**Frontend (new):**

- client/src/shared/hooks/useAuth.ts — useAuthInit hook for session restore
- client/src/shared/components/ProtectedRoute.tsx — route guard component
- client/src/features/auth/LoginPage.test.tsx — 8 LoginPage tests
- client/src/shared/components/ProtectedRoute.test.tsx — 3 ProtectedRoute tests
- client/src/shared/stores/authStore.test.ts — 2 authStore logout tests

### Review Findings

- [x] [Review][Decision] **Login 401 triggers unnecessary refresh attempt** — Resolved: `login()` now uses raw `fetch()` directly, bypassing fetchClient's 401 intercept. [client/src/shared/api/auth.ts]
- [x] [Review][Patch] **`useAuthInit` effect has `[i18n]` dependency instead of `[]`** — Fixed: changed to empty deps `[]`. [client/src/shared/hooks/useAuth.ts:42]
- [x] [Review][Patch] **Form-level error uses `text-sm` (14px) instead of spec's `text-xs` (12px)** — Fixed: changed to `text-xs`. [client/src/features/auth/LoginPage.tsx:156]
- [x] [Review][Patch] **Login handler: empty password triggers expensive bcrypt** — Fixed: added `req.Password == ""` early return. [server/internal/auth/handler.go:152]
- [x] [Review][Patch] **Expired token tests use wrong-secret, not actual expired tokens** — Fixed: added `generateExpiredAccessToken`/`generateExpiredRefreshToken` helpers, new `TestRefresh_ExpiredToken` and `TestMiddleware_RejectsWrongSecret` tests. [server/internal/auth/middleware_test.go, handler_test.go]
- [x] [Review][Defer] **Logout fire-and-forget can orphan refresh cookie server-side** [authStore.ts:23, auth.ts:62] — deferred, Phase 1 design decision (no server-side token revocation)
- [x] [Review][Defer] **Refresh tokens stateless with no revocation mechanism** [auth/handler.go:196] — deferred, requires token blocklist or DB session store (Phase 2+)
- [x] [Review][Defer] **Same signing secret for access and refresh tokens** [auth/service.go] — deferred, defense-in-depth improvement for Phase 2+
- [x] [Review][Defer] **Mutable `var` AppError sentinels shared across goroutines** [apperr/errors.go:31-48] — deferred, pre-existing from Story 1.1
- [x] [Review][Defer] **`apperr.Wrap()` wraps raw error instead of AppError, breaking error chain** [apperr/errors.go:26-28] — deferred, pre-existing from Story 1.2
- [x] [Review][Defer] **Default JWT secret only warns, doesn't abort in production** [config/config.go:26] — deferred, pre-existing (D3)
- [x] [Review][Defer] **No Unicode normalization on email addresses** [auth/handler.go:151] — deferred, pre-existing (affects registration too)
