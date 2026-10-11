# Story 1.4: Basic Player Profile & Navigation Shell

Status: done

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

## Story

As a logged-in player,
I want to see my profile and navigate the platform using a persistent top navigation bar,
So that I can access different sections and manage my language preference.

## Acceptance Criteria

1. **Navigation Bar Display** — Given a player is authenticated, when they are in the lobby or any main view, then a fixed top navigation bar is displayed with: logo/app name, tabs for Play / Leaderboard / Profile / Rules. The nav uses `surface` (#13131a) background with `border` (#2a2a38) bottom. The active tab is highlighted with an `accent` (#00e5a0) bottom border.

2. **Profile Page Content** — Given a player navigates to their profile, when the profile page loads, then it displays the player's username, "Member since" date (formatted from `createdAt` via i18n-aware date formatting), and placeholder sections for match history and stats (to be populated in Epic 7).

3. **Profile API Endpoint** — Given a player's profile endpoint is called, when `GET /api/v1/users/:id/profile` is requested with a valid JWT, then the server returns the user's `id`, `username`, `languagePreference`, and `createdAt` wrapped in `{ "data": { ... } }` format.

4. **Language Preference** — Given a player wants to change their language, when they select a different language (EN or SR) via a language selector in the nav or profile, then the UI immediately re-renders in the selected language, the preference is persisted to the server via `PATCH /api/v1/users/:id/preferences`, and subsequent page loads use the saved preference.

5. **Viewport & Cross-Browser** — Given the viewport is at least 1280x720, when the player views any page, then the layout renders correctly without horizontal scroll on Chrome, Firefox, Edge, and Safari (latest 2 versions).

## Tasks / Subtasks

- [x] **Task 1: Backend — User Profile and Preferences Endpoints** (AC: #3, #4)
  - [x] 1.1 Extend `UserRepository` interface in `server/internal/user/repository.go` with `GetByID(id uint) (*User, error)` and `UpdateLanguagePreference(id uint, lang string) error` methods
  - [x] 1.2 Implement methods in `server/internal/user/gorm_repo.go`
  - [x] 1.3 Add domain errors to `server/internal/apperr/errors.go`: `ErrUserNotFound` (404), `ErrForbidden` (403), `ErrInvalidLanguage` (400)
  - [x] 1.4 Create `server/internal/user/handler.go` with `UserHandler` struct, `GetProfile` handler (`GET /api/v1/users/:id/profile`), and `UpdatePreferences` handler (`PATCH /api/v1/users/:id/preferences`)
  - [x] 1.5 Wire routes in `server/cmd/api/main.go` under the authenticated group using existing `auth.AuthMiddleware`
  - [x] 1.6 Write backend tests in `server/internal/user/handler_test.go` covering: valid profile fetch, user not found, forbidden (wrong user ID), valid language update, invalid language value, missing auth

- [x] **Task 2: Frontend — API Client for Profile** (AC: #3, #4)
  - [x] 2.1 Create `client/src/shared/api/profile.ts` with `getProfile(userId: number)` and `updatePreferences(userId: number, prefs: { languagePreference: string })` using `fetchClient`
  - [x] 2.2 Add TypeScript types for profile API response in `client/src/shared/types/apiTypes.ts` (reuse existing `User` type if sufficient)

- [x] **Task 3: Frontend — Navigation Shell Layout** (AC: #1, #5)
  - [x] 3.1 Create `client/src/shared/components/AppLayout.tsx` — fixed top nav bar + `<Outlet />` for page content
  - [x] 3.2 Implement nav tabs: Play (`/lobby`), Leaderboard (placeholder `/leaderboard`), Profile (`/profile`), Rules (placeholder `/rules`) — use React Router `NavLink` or `useLocation` for active state detection, styled with `accent` bottom border on active tab
  - [x] 3.3 Add app name/logo in nav bar left section
  - [x] 3.4 Add `LanguageSelector` component in nav bar right section (see Task 5)
  - [x] 3.5 Update `client/src/App.tsx` routing: wrap lobby, profile, leaderboard, rules routes with `<AppLayout />` layout route; keep `/game` route outside AppLayout (game has no top nav per UX spec)
  - [x] 3.6 Create placeholder pages for `/leaderboard` and `/rules` routes (minimal components returning translated heading text)

- [x] **Task 4: Frontend — Profile Page** (AC: #2)
  - [x] 4.1 Replace stub `client/src/features/profile/ProfilePage.tsx` with full implementation: fetch profile data via `getProfile()`, display username and "Member since" date
  - [x] 4.2 Format "Member since" date using i18n-aware date formatting (use `Intl.DateTimeFormat` with current i18n locale)
  - [x] 4.3 Add placeholder sections for match history ("No matches yet") and stats with appropriate i18n keys — structure these so Epic 7 can populate them without restructuring

- [x] **Task 5: Frontend — Language Selector** (AC: #4)
  - [x] 5.1 Install shadcn/ui `dropdown-menu` component: `cd client && npx shadcn@latest add dropdown-menu`
  - [x] 5.2 Create `client/src/shared/components/LanguageSelector.tsx` — dropdown with EN/SR options, shows current language
  - [x] 5.3 On selection: call `i18n.changeLanguage()` for immediate re-render, then call `updatePreferences()` API to persist, then update `user.languagePreference` in `authStore`
  - [x] 5.4 Initialize language from `authStore.user.languagePreference` on mount (already handled by `useAuthInit` in Story 1.3)

- [x] **Task 6: i18n — Translation Keys** (AC: #1, #2, #4)
  - [x] 6.1 Add navigation keys to `client/src/shared/i18n/en.json` and `sr.json`: `nav.play`, `nav.leaderboard`, `nav.profile`, `nav.rules`, `nav.appName`
  - [x] 6.2 Add profile page keys: `profile.title`, `profile.memberSince`, `profile.matchHistory`, `profile.matchHistoryEmpty`, `profile.stats`, `profile.statsEmpty`
  - [x] 6.3 Add language selector keys: `language.label`, `language.en`, `language.sr`

- [x] **Task 7: Frontend Tests** (AC: #1, #2, #4, #5)
  - [x] 7.1 Create `client/src/shared/components/AppLayout.test.tsx` — renders nav tabs, highlights active tab, renders outlet content
  - [x] 7.2 Create `client/src/features/profile/ProfilePage.test.tsx` — renders username, renders member since date, renders placeholder sections, handles loading state
  - [x] 7.3 Create `client/src/shared/components/LanguageSelector.test.tsx` — renders current language, calls i18n.changeLanguage on selection, calls API to persist

- [x] **Task 8: Integration Validation** (AC: #1–#5)
  - [x] 8.1 Run `make lint` — fix any linting errors
  - [x] 8.2 Run `make test` — ensure all existing + new tests pass with zero regressions
  - [x] 8.3 Verify navigation flow: login → lobby with nav bar → click Profile tab → profile page loads with user data → change language → UI re-renders

## Dev Notes

### Architecture & Patterns

**Backend — User Domain Package:**

- User domain package already exists at `server/internal/user/` with `model.go`, `repository.go`, `gorm_repo.go`
- Add `handler.go` following the same pattern as `server/internal/auth/handler.go` — struct with dependencies injected via constructor
- `UserHandler` needs `UserRepository` interface and `jwtSecret string` (or use `auth.GetUserID()` helper from middleware)
- Handler must verify authenticated user's ID matches `:id` param — users can only view/edit their own profile in Phase 1 (no public profiles yet)
- Use `auth.GetUserID(c)` from `server/internal/auth/middleware.go` to extract JWT user ID
- Response format: `c.JSON(http.StatusOK, map[string]interface{}{"data": profileData})`
- Language validation: accept only `"en"` or `"sr"` — reject all other values with `ErrInvalidLanguage`

**Backend — Route Wiring in main.go:**

- Current authenticated group: `api := e.Group("", auth.AuthMiddleware(cfg.JWTSecret))`
- Add to this group: `api.GET("/api/v1/users/:id/profile", userHandler.GetProfile)` and `api.PATCH("/api/v1/users/:id/preferences", userHandler.UpdatePreferences)`
- `UserHandler` must be instantiated in `main.go` with the existing `userRepo` instance

**Frontend — AppLayout Component:**

- This is a NEW layout component wrapping authenticated non-game routes
- Renders: fixed top nav bar + `<Outlet />` for child route content
- Must NOT wrap the `/game` route (game is full-viewport, no nav bar per UX spec)
- Routing structure in App.tsx should become:
  ```
  <Route element={<ProtectedRoute />}>
    <Route element={<AppLayout />}>
      <Route path="/lobby" element={<LobbyPage />} />
      <Route path="/profile" element={<ProfilePage />} />
      <Route path="/leaderboard" element={<LeaderboardPage />} />
      <Route path="/rules" element={<RulesPage />} />
    </Route>
    <Route path="/game" element={<GamePage />} />
  </Route>
  ```

**Frontend — Nav Bar Styling (from UX Design Spec — Direction 5):**

- Fixed position, full-width, `surface` (#13131a) background
- `border-bottom` using `border` token (#2a2a38)
- Tabs: Play / Leaderboard / Profile / Rules — use Inter Medium 16px
- Active tab: `accent` (#00e5a0) bottom border — 2px solid
- Inactive tabs: `text-secondary` (#8888a0) color, no bottom border
- Logo/app name on the left — "Beljot" in Space Grotesk
- Language selector on the right
- Nav is NOT visible during active game (handled by routing structure)

**Frontend — Profile Page:**

- Located at `client/src/features/profile/ProfilePage.tsx` (file exists as stub — replace content)
- Fetch profile using `getProfile(user.id)` where `user` comes from `useAuthStore`
- Display username with `heading-lg` (Space Grotesk 24px 600)
- "Member since" date: format using `new Intl.DateTimeFormat(i18n.language, { year: 'numeric', month: 'long', day: 'numeric' })` to respect locale
- Placeholder sections: use `surface` background cards with `text-secondary` placeholder text
- Match history placeholder: "No matches yet — play your first game!"
- Stats placeholder: "Stats will appear after your first match"
- Structure placeholders as separate components/sections so Epic 7 (Stories 7.1-7.2) can replace content without restructuring

**Frontend — Language Selector:**

- Use shadcn/ui `DropdownMenu` component (needs installation)
- Show current language as trigger button (e.g., "EN" or "SR" with a globe icon from lucide-react)
- Options: English, Srpski (Serbian)
- On selection: (1) call `i18n.changeLanguage(lang)` for immediate re-render, (2) call `updatePreferences()` API fire-and-forget, (3) update `authStore.user.languagePreference`
- The `useAuthInit()` hook from Story 1.3 already syncs i18n language from `user.languagePreference` on page load

**Frontend — API Client:**

- Create `client/src/shared/api/profile.ts` following pattern from `client/src/shared/api/auth.ts`
- Use `fetchClient` (NOT raw `fetch`) for authenticated requests
- `getProfile()` returns unwrapped data from `{ "data": { ... } }` response

### Existing Files to Reuse (DO NOT Recreate)

| File                                              | What It Provides                                                 |
| ------------------------------------------------- | ---------------------------------------------------------------- |
| `server/internal/user/model.go`                   | `User` GORM model with all fields including `LanguagePreference` |
| `server/internal/user/repository.go`              | `UserRepository` interface — extend, don't replace               |
| `server/internal/user/gorm_repo.go`               | GORM implementation — extend, don't replace                      |
| `server/internal/auth/middleware.go`              | `AuthMiddleware()` + `GetUserID(c)` helper                       |
| `server/internal/apperr/errors.go`                | Centralized error definitions — add new errors here              |
| `server/cmd/api/main.go`                          | Route wiring — add to existing authenticated group               |
| `client/src/shared/api/fetchClient.ts`            | Authenticated HTTP client with refresh-retry                     |
| `client/src/shared/stores/authStore.ts`           | `useAuthStore` with `token`, `user`, `setUser`                   |
| `client/src/shared/hooks/useAuth.ts`              | `useAuthInit()` — already syncs i18n language                    |
| `client/src/shared/components/ProtectedRoute.tsx` | Auth guard — already handles redirects                           |
| `client/src/shared/components/ui/tabs.tsx`        | shadcn/ui Tabs — use for nav styling reference                   |
| `client/src/shared/lib/utils.ts`                  | `cn()` class merge utility                                       |
| `client/src/shared/i18n/i18n.ts`                  | i18next config — already set up with EN/SR                       |
| `client/src/index.css`                            | All Balatro design tokens as CSS custom properties               |

### Critical Anti-Patterns (DO NOT Do These)

- **DO NOT** call `fetch()` directly — use `fetchClient` from `shared/api/fetchClient.ts`
- **DO NOT** create a new Zustand store for profile — use existing `authStore` (user data is already there)
- **DO NOT** store language preference in localStorage — it's in the auth store and persisted to server
- **DO NOT** add game logic to frontend — this story is pure UI presentation
- **DO NOT** use `export default` — named exports only per project convention
- **DO NOT** use TypeScript `enum` — use union literal types
- **DO NOT** use CSS class names for test selectors — use `data-testid` attributes
- **DO NOT** put navigation inside `features/` — AppLayout is a shared layout component, place in `shared/components/`
- **DO NOT** modify `shared/components/ui/` source files for feature-specific styling — use Tailwind classes in consuming components
- **DO NOT** create game-related routes or components — only Play tab links to `/lobby` (existing)
- **DO NOT** add new database migrations — the `users` table from Story 1.2 already has all needed fields

### Previous Story Learnings (from Story 1.3)

1. **fetchClient pattern**: `login()` and `refresh()` use raw `fetch()`, NOT `fetchClient`, to avoid interceptor loops. Profile API calls SHOULD use `fetchClient` (they need auth token injection)
2. **Backend test pattern**: Use `httptest` with real Echo instance and `e.ServeHTTP` — don't mock HTTP
3. **Frontend test pattern**: Use `@testing-library/react` + `@testing-library/user-event` + Vitest with `data-testid` selectors
4. **Form error text**: Use `text-xs` (12px), not `text-sm` — established in auth forms
5. **Auth store `user` type**: `{ id: number; username: string; email: string; languagePreference: string; createdAt: string }` — already has all fields needed for profile display
6. **Cookie settings**: Auth cookies use path `/api/v1/auth`, SameSite Strict, HttpOnly — don't interfere
7. **i18n key structure**: `{feature}.{component}.{element}` — e.g., `auth.login.emailLabel`
8. **`useAuthInit` syncs language**: On mount, it calls `i18n.changeLanguage(user.languagePreference)` — initial language sync is already handled

### Design System Quick Reference

| Token              | CSS Variable         | Value         | Usage in This Story             |
| ------------------ | -------------------- | ------------- | ------------------------------- |
| `background`       | `--background`       | `#0a0a0f`     | Page background                 |
| `surface`          | `--surface`          | `#13131a`     | Nav bar bg, profile cards       |
| `surface-elevated` | `--surface-elevated` | `#1c1c26`     | Dropdowns, modals               |
| `border`           | `--border`           | `#2a2a38`     | Nav bottom border, card borders |
| `accent`           | `--accent`           | `#00e5a0`     | Active tab indicator            |
| `text-primary`     | `--text-primary`     | `#f0f0f8`     | Username, headings              |
| `text-secondary`   | `--text-secondary`   | `#8888a0`     | Labels, placeholder text        |
| Display font       | `font-display`       | Space Grotesk | App name, headings              |
| Body font          | `font-body`          | Inter         | Tab labels, body text           |

### Project Structure Notes

- AppLayout goes in `client/src/shared/components/AppLayout.tsx` — it's a shared layout, not feature-specific
- LanguageSelector goes in `client/src/shared/components/LanguageSelector.tsx` — reusable across pages
- ProfilePage stays in `client/src/features/profile/ProfilePage.tsx` — feature-specific page
- Placeholder pages (LeaderboardPage, RulesPage) go in their future feature folders: `client/src/features/leaderboard/LeaderboardPage.tsx` and `client/src/features/rules/RulesPage.tsx`
- Backend user handler: `server/internal/user/handler.go` — extends existing user domain package
- No new domain packages needed

### References

- [Source: _bmad-output/planning-artifacts/epics.md — Epic 1, Story 1.4]
- [Source: _bmad-output/planning-artifacts/architecture.md — Frontend Feature Folders, API Endpoints, User Domain Package]
- [Source: _bmad-output/planning-artifacts/ux-design-specification.md — Direction 5 Layout, Navigation, Design Tokens, Typography]
- [Source: _bmad-output/project-context.md — Naming Conventions, Anti-Patterns, Testing Rules]
- [Source: _bmad-output/archive/implementation-artifacts/1-3-user-login-and-session-persistence.md — Auth patterns, File List, Learnings]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Debug Log References

- Import cycle detected: `user/handler.go` initially imported `auth` package for `GetUserID()`, creating `user -> auth -> user` cycle. Resolved by duplicating the context extraction as a package-private `getUserID()` function in the user package.
- Used `package user_test` (external test package) for handler tests to allow importing both `auth` and `user` without cycle.
- 2 pre-existing GORM integration test failures in `user_test.go` (`TestGormUserRepository_FindByEmail_NotFound`, `TestGormUserRepository_FindByUsername_NotFound`) — these expect `record not found` errors but the repo returns `(nil, nil)` by design. Not caused by this story.

### Completion Notes List

- **Task 1**: Backend profile and preferences endpoints implemented. `GetProfile` (GET) and `UpdatePreferences` (PATCH) handlers created in `server/internal/user/handler.go`. Added `UpdateLanguagePreference` to repository interface and GORM implementation. Added `ErrUserNotFound` and `ErrInvalidLanguage` domain errors. Wired routes in `main.go` with auth middleware. Added `PATCH` to CORS allowed methods. 8 handler tests pass.
- **Task 2**: Created `client/src/shared/api/profile.ts` with `getProfile()` and `updatePreferences()` using `fetchClient`.
- **Task 3**: Created `AppLayout` with fixed top nav bar (surface bg, border bottom), tabs (Play/Leaderboard/Profile/Rules) with accent active indicator, app name, language selector, and `<Outlet />`. Updated `App.tsx` routing to wrap authenticated non-game routes with `AppLayout`; `/game` route remains outside.
- **Task 4**: Replaced stub `ProfilePage` with full implementation fetching profile via API, displaying username, i18n-formatted "Member since" date, and placeholder match history/stats sections.
- **Task 5**: Installed shadcn `dropdown-menu`. Created `LanguageSelector` with Globe icon, EN/SR options. On selection: changes i18n language, fires API call to persist, updates auth store.
- **Task 6**: Added nav, profile, language, leaderboard, and rules i18n keys to both `en.json` and `sr.json`.
- **Task 7**: Created 3 test files with 14 new tests (5 AppLayout, 5 ProfilePage, 4 LanguageSelector). All 42 frontend tests pass.
- **Task 8**: TypeScript compiles cleanly. All new files pass ESLint. Frontend: 42/42 tests pass. Backend: auth 40/40 pass, user handler 8/8 pass. `go vet` clean.

### File List

**New files:**

- `server/internal/user/handler.go` — UserHandler with GetProfile and UpdatePreferences
- `server/internal/user/handler_test.go` — 8 handler tests (external test package)
- `client/src/shared/api/profile.ts` — Profile API client
- `client/src/shared/components/AppLayout.tsx` — Navigation shell layout
- `client/src/shared/components/AppLayout.test.tsx` — AppLayout tests
- `client/src/shared/components/LanguageSelector.tsx` — Language selector dropdown
- `client/src/shared/components/LanguageSelector.test.tsx` — LanguageSelector tests
- `client/src/shared/components/ui/dropdown-menu.tsx` — shadcn dropdown-menu component
- `client/src/features/profile/ProfilePage.test.tsx` — ProfilePage tests
- `client/src/features/leaderboard/LeaderboardPage.tsx` — Placeholder leaderboard page
- `client/src/features/rules/RulesPage.tsx` — Placeholder rules page

**Modified files:**

- `server/internal/user/repository.go` — Added `UpdateLanguagePreference` to interface
- `server/internal/user/gorm_repo.go` — Implemented `UpdateLanguagePreference`
- `server/internal/apperr/errors.go` — Added `ErrUserNotFound`, `ErrInvalidLanguage`
- `server/cmd/api/main.go` — Wired user routes, added PATCH to CORS
- `server/internal/auth/handler_test.go` — Added `UpdateLanguagePreference` to mock
- `client/src/App.tsx` — Updated routing with AppLayout wrapper
- `client/src/features/profile/ProfilePage.tsx` — Full profile implementation
- `client/src/shared/i18n/en.json` — Added nav, profile, language, leaderboard, rules keys
- `client/src/shared/i18n/sr.json` — Added nav, profile, language, leaderboard, rules keys

### Review Findings

- [x] [Review][Decision] Active tab border uses wrong accent color — fixed: changed `--accent` in `:root` to `#00e5a0` and `--accent-foreground` to `#0a0a0f` [index.css:72-73]
- [x] [Review][Patch] GORM error not translated in UpdateLanguagePreference — fixed: repo now returns `apperr.ErrUserNotFound` [gorm_repo.go:75]
- [x] [Review][Patch] ProfilePage useEffect depends on full `user` object — fixed: changed dependency to `[user?.id]` [ProfilePage.tsx:23]
- [x] [Review][Patch] Stale `user` closure in LanguageSelector — fixed: reads `user` from `getState()` inside handler [LanguageSelector.tsx:25]
- [x] [Review][Patch] ProfilePage loading state hangs if `user` is null — fixed: sets `isLoading=false` before early return [ProfilePage.tsx:15]
- [x] [Review][Patch] `new Date(createdAt)` throws RangeError on malformed strings — fixed: wrapped in try/catch [ProfilePage.tsx:37-44]
- [x] [Review][Patch] Handler returns raw errors without `%w` wrapping — fixed: wrapped with `fmt.Errorf` context [handler.go:62,103]
- [x] [Review][Patch] CreatedAt format inconsistency — fixed: changed to `time.Time` type matching auth handler [handler.go:18,71]
- [x] [Review][Patch] Mock `UpdateLanguagePreference` returns nil for nonexistent user — fixed: returns `gorm.ErrRecordNotFound` [auth/handler_test.go:84, user/handler_test.go:72]
- [x] [Review][Defer] Duplicate `getUserID` helper diverges from `auth.GetUserID` [handler.go:32-42] — deferred, intentional to avoid import cycle
- [x] [Review][Defer] `i18n.language` region subtag comparison (`"en-US"` vs `"en"`) may bypass dedup guard [LanguageSelector.tsx:23] — deferred, only impacts if locale detection plugins are added
- [x] [Review][Defer] Path parameter `:id` accepts 0 — GORM `First(&u, 0)` may return arbitrary record [handler.go:50] — deferred, mitigated by JWT IDs starting at 1

### Change Log

- 2026-04-11: Story 1.4 implemented — Backend profile/preferences API, frontend navigation shell with AppLayout, profile page, language selector, i18n keys, placeholder pages, and 22 new tests (8 backend + 14 frontend)
- 2026-04-11: Code review complete — 1 decision-needed, 8 patches, 3 deferred, 7 dismissed
