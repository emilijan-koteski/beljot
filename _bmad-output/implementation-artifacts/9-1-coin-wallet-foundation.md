---
baseline_commit: ec5b2471ce53e0cb39114b6d53f4e1a953c48ad3
---

# Story 9.1: Coin Wallet Foundation

Status: done

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

## Story

As a player,
I want a coin wallet that grows with daily activity,
so that I have an ongoing currency to spend on entering games.

## Acceptance Criteria

> Source: [epics.md#Story 9.1](../planning-artifacts/epics.md) (lines 1724-1761). Story 9.1 is **unchanged** by the 2026-06-18 Epic 9 restructure — see [sprint-change-proposal-2026-06-18.md](../planning-artifacts/sprint-change-proposal-2026-06-18.md) (row 9.1 = "unchanged"). Two delivery details were refined with the stakeholder during story planning (2026-06-18) and are called out inline below: the reward notification is a **persistent dialog** (not a toast), and the day-1 bonus starts the **calendar day after registration** (registration itself only seeds 5 000).

1. **New-player wallet seed** — Given a new player registers, when the user record is created, then their coin balance is initialized to **5 000**, and a `wallet_balance` column is persisted on the user (integer, non-negative). **No daily bonus is granted at registration** — registration also stamps `last_login_at = <registration UTC date>` and `login_streak_days = 0` so the **day-1 bonus first becomes available on the next calendar day** (stakeholder decision; see Q1-resolved).

2. **Daily login bonus + streak** — Given a player starts a session on a **new calendar day (UTC)**, when the session-init runs, then:
   - the server checks their `last_login_at` date;
   - if it was **exactly the prior calendar day**, the streak counter increments; otherwise it resets to **1**;
   - a daily bonus is credited: `1000 + (streak_day - 1) × 162`, **capped at 3 100** (the cap is first reached on day 14);
   - `last_login_at` is updated to today;
   - the client shows the amount granted and the new streak value in a **persistent lobby dialog that stays open until the player closes it** (stakeholder decision — overrides the epic's "lobby toast" wording).

3. **One bonus per day** — Given a player has already started a session today, when the session-init runs again in the same UTC day, then no additional bonus is granted (enforced atomically — see Task 4/5).

4. **Balance + streak display** — Given a player views their profile or the lobby header, when the UI renders, then their current coin balance is displayed with a coin icon, and the current streak (if > 1) is shown as a small indicator (e.g., "Day 7").

5. **Wallet schema migration** — Given the wallet migration, when the schema is inspected, then `users` has columns: `wallet_balance` (integer, default 5000, non-negative constraint), `last_login_at` (date, nullable), `login_streak_days` (integer, default 0).

6. **Wallet backend package** — Given the wallet backend package, when structured, then a `wallet` domain package exists with model, repository, service, handler, and `_test.go` per the feature-complete checklist, and **wallet mutations are atomic and transactional**.

---

## ⚠️ Design Decision — Daily-Reward Trigger & Delivery (read before coding)

The epic AC says the client receives `event:daily_reward` as a "lobby toast" on login. That framing has **two flaws** for this codebase, both resolved here:

### A. Trigger: session bootstrap, NOT the `/auth/login` endpoint

The refresh token lasts **7 days** ([auth/service.go:35-44](../../server/internal/auth/service.go#L35-L44)); on every app load `useAuthInit` calls `POST /auth/refresh` (cookie auto-sent) to mint a fresh access token ([useAuth.ts:41-56](../../client/src/shared/hooks/useAuth.ts#L41-L56)). A returning daily player is **auto-logged-in via the cookie and almost never hits `POST /auth/login`**. If the daily bonus fired only inside `Login`, daily players would essentially never receive it — defeating the whole "wallet that grows with daily activity" goal.

**Therefore the daily-login check must run on the path that executes on every app entry — the session bootstrap — not on the explicit-login action.**

### B. Transport: a dedicated HTTP endpoint, NOT WebSocket

`event:` is reserved for server→client **game state** ([project-context.md] WS prefixes); a daily reward is an account/platform event, and the WS isn't even connected at bootstrap time. So a WS event is the wrong mechanism.

**Decision (matches the stakeholder's "handle it in the init endpoint for fetching the user, without WS"):** add **`POST /api/v1/wallet/daily-login`** (authenticated, idempotent, atomic). The client calls it **once per app session on bootstrap** — right after auth is established (covers both explicit login AND refresh-token auto-login uniformly). It grants at most once per UTC day and returns the outcome:

```jsonc
// 200 { "data": { granted, amount, streakDay, newBalance, loginStreakDays } }
{ "granted": true, "amount": 1000, "streakDay": 1, "newBalance": 6000, "loginStreakDays": 1 }
// when already claimed today:
{ "granted": false, "amount": 0, "streakDay": 7, "newBalance": 5320, "loginStreakDays": 7 }
```

When `granted === true`, the client updates `authStore.user` (balance + streak) and opens the **persistent dialog**. This is the `wallet` package's handler (satisfies AC #6).

**Consequences:** `auth.Login` / `Register` / `Refresh` do **NOT** grant the bonus — they only echo the current `walletBalance` + `loginStreakDays` in their responses for immediate header display. **No WS contract files (`events.go` / `wsEvents.ts`) are touched by this story.** Because the grant is a state mutation, it is a **POST**, not a side-effecting GET.

---

## Tasks / Subtasks

### Backend — Schema & Model

- [x] **Task 1: Migration `000009_add_wallet_columns_to_users` (AC: #1, #5)**
  - [x] Create `server/migrations/000009_add_wallet_columns_to_users.up.sql` and `.down.sql`. **Highest existing number is `000008` — use `000009`, never skip** ([project-context.md] migrations rule).
  - [x] `.up.sql`: `ALTER TABLE users ADD COLUMN wallet_balance INTEGER NOT NULL DEFAULT 5000 CHECK (wallet_balance >= 0);` `ADD COLUMN last_login_at DATE;` (nullable, no default) `ADD COLUMN login_streak_days INTEGER NOT NULL DEFAULT 0;`
  - [x] `.down.sql`: drop the three columns in reverse. The down must fully reverse the up ([project-context.md]). Mirror the additive style of [000007_add_bot_players.up.sql](../../server/migrations/000007_add_bot_players.up.sql).
  - [x] Existing rows (none in prod): `DEFAULT 5000` backfills balances; `last_login_at` stays NULL, so an existing user's first bootstrap is treated as a fresh streak (resets to 1) — acceptable. Comment this in the migration.
  - [x] Run `make migrate`; confirm columns exist.

- [x] **Task 2: Extend `user.User` model (AC: #1, #5)**
  - [x] Add to [user/model.go](../../server/internal/user/model.go): `WalletBalance int` (`gorm:"not null;default:5000"`, `json:"walletBalance"`), `LastLoginAt *time.Time` (`gorm:"column:last_login_at"`, `json:"lastLoginAt,omitempty"`), `LoginStreakDays int` (`gorm:"column:login_streak_days;not null;default:0"`, `json:"loginStreakDays"`).
  - [x] `LastLoginAt` is a **pointer** — it's nullable, and `time.Time`'s zero value serializes as `"0001-01-01T00:00:00Z"`, not `null` ([project-context.md] Go rules). DB column is `DATE`; GORM reads/writes `time.Time` fine.
  - [x] Place new fields after `LanguagePreference`, before the GORM magic timestamps.

### Backend — Wallet Domain Package

> Standard domain-package shape: `model.go`, `repository.go`, `gorm_repo.go`, `service.go`, `handler.go`, `wallet_test.go` ([project-context.md] Echo rules). Templates: [passwordreset](../../server/internal/passwordreset/) (atomic-guard repo) and [user](../../server/internal/user/) (handler/getUserID pattern).

- [x] **Task 3: `wallet.Repository` + GORM impl — atomic daily-login (AC: #2, #3, #6)**
  - [x] `server/internal/wallet/repository.go`: `Repository` interface. Wallet state lives **on the `users` table** (AC #5), so the repo operates on `user.User` rows. Method: `ProcessDailyLogin(userID uint, today time.Time) (DailyLoginResult, error)`.
  - [x] `server/internal/wallet/gorm_repo.go`: `GormRepository{ db *gorm.DB }`, `NewGormRepository(db)`. Mirror [user/gorm_repo.go](../../server/internal/user/gorm_repo.go) error handling.
  - [x] **Atomicity (AC #3, #6):** wrap the read-modify-write in `r.db.Transaction(func(tx *gorm.DB) error { ... })`, selecting the user row `FOR UPDATE` (`tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&u, userID)` — import `gorm.io/gorm/clause`). Read `last_login_at`, compute the new streak/bonus (Task 4 helpers), then update only when a grant is due. The row lock makes the once-per-day guard race-free — two concurrent bootstraps cannot both grant. This is the [project-context.md] "wallet mutations are atomic and transactional" + concurrency-safety expectation; precedent is [passwordreset MarkUsed](../../server/internal/passwordreset/gorm_repo.go#L36-L47).
  - [x] **Import direction:** `wallet` may import `user` (for `user.User`); `user` must **not** import `wallet`.

- [x] **Task 4: `wallet.Service` — streak + bonus math (AC: #2, #3)**
  - [x] `server/internal/wallet/service.go`: `Service` with `ProcessDailyLogin(userID uint) (DailyLoginResult, error)` that computes the current UTC day and delegates the atomic write to the repo. Keep the pure date/bonus logic in standalone helpers so they're DB-free table-testable.
  - [x] **Date comparison uses UTC calendar-date components, not duration math.** Read `last_login_at` as `.UTC()` and compare by `(Year, Month, Day)` (or format both as `"2006-01-02"`). Do **not** use `Truncate(24*time.Hour)` or subtract durations — that breaks across DST / non-UTC server zones. "Exactly the prior day" = `today` is the calendar day immediately after `dateOf(last_login_at)`.
  - [x] Streak rule:
    - `last_login_at == nil` (legacy/no stamp) → `streak = 1`, grant.
    - `dateOf(today) == dateOf(last_login_at)` → **already today → no grant** (AC #3); return current balance/streak, `Granted: false`.
    - `dateOf(today) == dateOf(last_login_at) + 1 day` → `streak = last_streak + 1`, grant. (Registration stamped streak 0 → first next-day grant yields streak 1 = day-1.)
    - otherwise (gap > 1 day, or `today < last_login` clock-skew) → `streak = 1`, grant.
  - [x] Bonus: `amount := min(1000 + (streak-1)*162, 3100)`. Verify the curve: day1=1000, day2=1162, day13=2944, **day14=3106→capped 3100**, day15+=3100. The **streak counter keeps incrementing past 14** (for "Day N" display); only the bonus amount is capped.
  - [x] On grant: `new_balance = balance + amount`, set `login_streak_days = streak`, `last_login_at = today`. On no-grant: touch nothing.
  - [x] `type DailyLoginResult struct { Granted bool; Amount int; StreakDay int; NewBalance int; LoginStreakDays int }`. Centralize constants: `const StartingBalance = 5000; DailyBase = 1000; DailyStep = 162; DailyCap = 3100` (keep `StartingBalance` in sync with the migration default — comment the link).

- [x] **Task 5: `wallet.Handler` — `POST /api/v1/wallet/daily-login` (AC: #2, #3, #6)**
  - [x] `server/internal/wallet/handler.go`: `WalletHandler` with `ProcessDailyLogin(c echo.Context) error` → reads the authed user via the `getUserID(c)` pattern ([user/handler.go:156](../../server/internal/user/handler.go#L156)), calls `service.ProcessDailyLogin`, returns `{ "data": DailyLoginResult }` ([project-context.md] API envelope). Idempotent — safe to call on every bootstrap and to retry.
  - [x] Register in [main.go](../../server/cmd/api/main.go) authed `api` group (near the `/users/...` routes, ~line 119): `walletRepo := wallet.NewGormRepository(db)`, `walletService := wallet.NewService(walletRepo)`, `walletHandler := wallet.NewWalletHandler(walletService)`, `api.POST("/wallet/daily-login", walletHandler.ProcessDailyLogin)`.

### Backend — Auth & Profile Responses (read-only wallet fields)

- [x] **Task 6: Echo wallet fields in auth responses + registration stamping (AC: #1, #4)**
  - [x] Extend `RegisterResponseData` ([auth/handler.go:29](../../server/internal/auth/handler.go#L29)) — shared by **Register, Login, Refresh** — with `WalletBalance int json:"walletBalance"` and `LoginStreakDays int json:"loginStreakDays"`. Populate all three handlers from the loaded/created `user.User`. **No daily-reward grant in any auth handler** — those stay pure auth (the grant is the wallet endpoint, Task 5).
  - [x] **Register** ([auth/handler.go:115-124](../../server/internal/auth/handler.go#L115-L124)): before `Create`, explicitly set `u.WalletBalance = 5000`, `u.LoginStreakDays = 0`, and `u.LastLoginAt = &today` (today = `time.Now().UTC()` date). Stamping `last_login_at` at registration is what makes the same-day bootstrap a no-grant and the **next day** the day-1 grant (AC #1, Q1-resolved). Set `WalletBalance` explicitly rather than relying on the GORM `default` tag, so the response and the inserted row agree.
  - [x] **Login** / **Refresh**: just set the two response fields from the loaded `u`. Do not call the wallet service.

- [x] **Task 7: Wallet fields on the profile (AC: #4)**
  - [x] Add `WalletBalance int json:"walletBalance"` + `LoginStreakDays int json:"loginStreakDays"` to `ProfileResponse` ([user/handler.go:15](../../server/internal/user/handler.go#L15)); populate from the loaded `u` in `GetProfile`.
  - [x] **Privacy:** these are private figures. `GetProfile` is self-only today (`paramID == authUserID`, [user/handler.go:179](../../server/internal/user/handler.go#L179)) so there's no leak now — but when **Epic 11** adds public player profiles, the public DTO must **not** include `walletBalance` / `loginStreakDays`. Never put them on a shared/public response shape.

### Frontend — Types, State, Bootstrap

- [x] **Task 8: Extend `User` type + hydrate all auth sites (AC: #4)**
  - [x] [apiTypes.ts](../../client/src/shared/types/apiTypes.ts) `User`: add `walletBalance: number;` and `loginStreakDays: number;`.
  - [x] [api/auth.ts](../../client/src/shared/api/auth.ts): add `walletBalance: number; loginStreakDays: number;` to `RegisterResponse` and `RefreshResponse` (and thus `LoginResponse`, which aliases register).
  - [x] **All three User-hydration sites must carry the new fields — miss one and the header blanks out in that flow** (regression trap):
    1. `setAuthState` (login + register) — [mutations/useAuth.ts:12-28](../../client/src/shared/hooks/mutations/useAuth.ts#L12-L28): add `walletBalance` + `loginStreakDays` to `setUser({...})` and to the function's param type.
    2. Reload bootstrap — [useAuthInit in useAuth.ts:44-51](../../client/src/shared/hooks/useAuth.ts#L44-L51): same two fields in its `setUser({...})`.
  - [x] **Never use JS truthiness on these numbers** — `0` balance / `0` streak are real Go zero values, not "missing". Use explicit comparisons; show the streak chip only when `loginStreakDays > 1` (AC #4), not `if (loginStreakDays)` ([project-context.md] TS rules).

- [x] **Task 9: Wallet API client + bootstrap gate (AC: #2, #3)**
  - [x] New `client/src/shared/api/wallet.ts`: `claimDailyLogin(): Promise<DailyLoginResult>` → `axiosClient.post("/wallet/daily-login")`, unwrapping the `{ data }` envelope and mapping errors via `FetchError` (mirror [api/auth.ts](../../client/src/shared/api/auth.ts)). `DailyLoginResult = { granted: boolean; amount: number; streakDay: number; newBalance: number; loginStreakDays: number }`.
  - [x] **One bootstrap gate fires the call once per authenticated app session** — covering explicit-login, register, and refresh-token auto-login uniformly. Implement a hook/component (e.g. `useDailyRewardGate`) mounted in the **authenticated shell** ([AppLayout.tsx](../../client/src/shared/components/AppLayout.tsx)) that, when `authStore.token` is present and not loading, calls `claimDailyLogin()` exactly once. Guard against React 18 **StrictMode** double-invoke with a ref; the endpoint is idempotent server-side regardless (a second call returns `granted:false`), so no double-grant is possible.
  - [x] On a successful response: update `authStore` via `setUser({ ...user, walletBalance: res.newBalance, loginStreakDays: res.loginStreakDays })` (immutable replace). If `res.granted === true`, open the dialog (Task 10).
  - [x] Use a TanStack-query mutation or a guarded `useEffect`; do not call `fetch()` in a component ([project-context.md]).

### Frontend — Display

- [x] **Task 10: Persistent daily-reward dialog (AC: #2)**
  - [x] New `DailyRewardDialog` (e.g. `client/src/features/lobby/components/DailyRewardDialog.tsx`), rendered in the authenticated shell. Use the shadcn **controlled** Dialog: `<Dialog open={open} onOpenChange={setOpen}>` with `DialogContent`/`DialogHeader`/`DialogTitle`/`DialogDescription` from [shared/components/ui/dialog](../../client/src/shared/components/ui/dialog.tsx) — same imports as [ContactDialog.tsx](../../client/src/features/landing/components/ContactDialog.tsx).
  - [x] **Persistent until the player closes it** (AC #2): drive `open` from gate state, **no auto-dismiss timer**. To prevent accidental dismissal, pass `onInteractOutside={(e) => e.preventDefault()}` (and optionally `onEscapeKeyDown`) on `DialogContent`, and provide an explicit "Collect"/"Close" button that sets `open=false`. Do **not** use a `DialogTrigger` — it's opened programmatically by the gate.
  - [x] Content: coin icon (lucide `Coins`), `+{{amount}}` coins, "Day {{streakDay}}", and the new balance — all via i18n.
  - [x] `data-testid="daily-reward-dialog"` for tests; never select by CSS class ([project-context.md]).

- [x] **Task 11: Coin balance + streak in header & profile (AC: #4)**
  - [x] [TopBar.tsx](../../client/src/shared/components/TopBar.tsx): read `useAuthStore((s) => s.user)` (already present); render a coin pill (lucide `Coins` + `walletBalance`) after the `LanguageSelector`, before the user menu. When `loginStreakDays > 1`, render a small "Day N" indicator. `data-testid="coin-balance"`. Format with `toLocaleString()` (locale-aware).
  - [x] Profile: surface balance + streak in the hero pills ([IdentityHero.tsx](../../client/src/features/profile/components/IdentityHero.tsx)) alongside games/wins/losses/capots. Source from `authStore.user` or the augmented profile response (Task 7). Keep the **login** streak distinct from the existing **win/loss** streak in `StreakCallout` — different concepts. Update the profile response type in [api/profile.ts](../../client/src/shared/api/profile.ts) if you read it from there.

### i18n & Tests

- [x] **Task 12: i18n keys in ALL four locale files (AC: #2, #4)**
  - [x] Add a `rewards`/`wallet` namespace to **all** of `en.json`, `sr.json`, `mk.json`, `hr.json` in [client/src/shared/i18n/](../../client/src/shared/i18n/) — missing one fails the feature-complete checklist ([project-context.md]). Keys: `{feature}.{component}.{element}`, e.g. `rewards.dailyReward.title`, `.amount` (`"+{{amount}} coins"`), `.streak` (`"Day {{streak}}"`), `.collect`, `wallet.balanceLabel`. Serbian = Latin script.

- [x] **Task 13: Backend tests (AC: all)**
  - [x] `server/internal/wallet/wallet_test.go`: **table-driven** ([project-context.md]) tests for the streak/bonus pure helpers — first session (nil last_login), same-day (no grant), consecutive day (++), gap > 1 day (reset to 1), and the bonus curve incl. the day-14 cap and day-15+ staying at 3100.
  - [x] Transaction/concurrency test: two concurrent `ProcessDailyLogin` for the same user on a new day grant **exactly once** (AC #3). PostgreSQL integration tests use a **per-test transaction with rollback** and create their own data — never seed data ([project-context.md]).
  - [x] `auth/handler_test.go`: login/register/refresh responses include `walletBalance` + `loginStreakDays`; **registration stamps balance 5000 / streak 0 / last_login = today and grants nothing**; a same-day registration→bootstrap yields no grant.
  - [x] `user/handler_test.go`: profile response includes the wallet fields.

- [x] **Task 14: Frontend tests (AC: #2, #4)**
  - [x] `TopBar.test.tsx`: renders balance from the store; shows "Day N" only when `loginStreakDays > 1`; renders correctly at balance `0`.
  - [x] `DailyRewardDialog` test: opens and stays open when the gate reports `granted` (no auto-close); the Collect button closes it; it does not open when `granted:false`. Present-tense `it(...)` descriptions.

### Definition of Done (Feature-Complete Checklist — hard gate)

- [x] Server handler + repository layer + tests ✔ (wallet pkg `POST /wallet/daily-login`, auth/user/profile response changes)
- [x] Domain errors in `internal/apperr/errors.go` **only if** a new user-facing wallet error is introduced — 9.1 likely needs none (the endpoint either grants or returns `granted:false`).
- [x] WebSocket events — **N/A** (HTTP `POST /wallet/daily-login` by design — see Design Decision). Do not touch `wsEvents.ts` / `events.go`.
- [x] Frontend component + co-located test ✔
- [x] API client function in `shared/api/wallet.ts` ✔
- [x] i18n strings in **all** four translation files ✔
- [x] Linter passes (`make lint`)
- [x] All existing tests pass (`make test`)

---

## Dev Notes

### Why this story matters (Epic context)

Story 9.1 is the **foundation** of Epic 9 (Player Economy & Progression). Downstream stories depend on the wallet existing and being mutated atomically:

- **9.2 Room Buy-In & Settlement** — deducts a stake at match start, splits the pot; needs transactional `Deduct`/`Credit` primitives. **Design Task 3's transaction/locking pattern so 9.2 can reuse it.**
- **9.3 Insolvency Ejection** — re-validates `balance >= coin_buy_in` atomically inside `StartMatch`.
- **9.4 Quick Play Coin Bracketing** — brackets by balance band. **9.5 XP & Level** — adds more `users` columns alongside the wallet fields.

(Source: [epics.md#Epic 9](../planning-artifacts/epics.md) lines 1720-2029; [sprint-change-proposal-2026-06-18.md](../planning-artifacts/sprint-change-proposal-2026-06-18.md).)

### Economy constants are placeholders (locked for 9.1)

Per the 2026-06-18 change proposal §"Deferred (by decision)": starting balance (5000), daily curve (`1000 + (n-1)×162`, cap 3100), and all economy numbers are **placeholders tuned per story**. For 9.1 the AC values are the locked constants — centralize them in the `wallet` package (Task 4) so later tuning is one edit. The DB default (5000) duplicates `StartingBalance`; keep them in sync and comment the link.

### Source tree — files to touch

**Backend (new):** `server/migrations/000009_add_wallet_columns_to_users.{up,down}.sql`; `server/internal/wallet/{model.go, repository.go, gorm_repo.go, service.go, handler.go, wallet_test.go}`

**Backend (modify):** [user/model.go](../../server/internal/user/model.go) (3 fields) · [user/handler.go](../../server/internal/user/handler.go) (`ProfileResponse`) · [auth/handler.go](../../server/internal/auth/handler.go) (`RegisterResponseData` fields + registration stamping) · [cmd/api/main.go](../../server/cmd/api/main.go) (wallet repo/service/handler + route)

**Frontend (modify):** [apiTypes.ts](../../client/src/shared/types/apiTypes.ts) · [api/auth.ts](../../client/src/shared/api/auth.ts) · [hooks/mutations/useAuth.ts](../../client/src/shared/hooks/mutations/useAuth.ts) · [hooks/useAuth.ts](../../client/src/shared/hooks/useAuth.ts) · [components/TopBar.tsx](../../client/src/shared/components/TopBar.tsx) · [components/AppLayout.tsx](../../client/src/shared/components/AppLayout.tsx) (mount the gate + dialog) · [features/profile/components/IdentityHero.tsx](../../client/src/features/profile/components/IdentityHero.tsx) · [api/profile.ts](../../client/src/shared/api/profile.ts) · all four `shared/i18n/*.json`

**Frontend (new):** `shared/api/wallet.ts`; the daily-reward gate hook/component; `features/lobby/components/DailyRewardDialog.tsx`

### Reading the code being modified — current behavior to preserve

- **`AuthHandler.Login`** ([auth/handler.go:155](../../server/internal/auth/handler.go#L155)): normalizes email (lowercase + NFC) → `FindByEmail` → `CheckPassword` → mints access+refresh tokens → sets refresh cookie → returns `RegisterResponseData`. **Preserve every step**; only add the two response fields. Don't change token issuance, cookies, or the `ErrInvalidCredentials` paths.
- **`RegisterResponseData` is shared by Register, Login, and Refresh.** Adding fields touches all three responses (intended — balance must survive reload via Refresh). Populate correctly per handler: Register = 5000/0/last_login=today (no grant), Login & Refresh = current values from the loaded user.
- **`useAuthInit`** ([useAuth.ts:16-72](../../client/src/shared/hooks/useAuth.ts#L16-L72)): the reload path — `refresh()` → `setUser(...)` → `i18n.changeLanguage`. Preserve the abort-controller + language-switch behavior; only add the two wallet fields to `setUser`. The daily-login **grant** is NOT done here — it's the AppLayout gate (Task 9), which runs once auth is established (covers this path too).
- **`logout()`** ([authStore.ts:26-40](../../client/src/shared/stores/authStore.ts#L26-L40)): wipes session stores then clears `user`. No change — clearing `user` drops the balance. **Do not** add a separate wallet store; balance lives on `authStore.user` (the persistent-across-app store per [project-context.md] partitioning).
- **shadcn Dialog** ([ContactDialog.tsx](../../client/src/features/landing/components/ContactDialog.tsx)) is the canonical usage; for a programmatic persistent modal use the controlled `open`/`onOpenChange` form (no `DialogTrigger`).

### Architecture & convention guardrails (must follow)

- **GORM three-convention bridge:** DB `wallet_balance` (snake) ↔ Go `WalletBalance` (Pascal) ↔ JSON `walletBalance` (camel). Explicit `gorm:"column:..."` + `json:"camelCase"` tags. ([project-context.md])
- **JSON wire format is always camelCase**, both directions.
- **Wallet mutations atomic + transactional** — one DB transaction with a row lock; on failure roll back + log via slog; user-facing messages stay generic (AC #6 + [project-context.md]).
- **`*time.Time` for the nullable date** so `null` serializes correctly.
- **No economy values via `os.Getenv`** outside config — 9.1's constants are code-level in the `wallet` package, not env-driven ([config.go](../../server/internal/config/config.go)).
- **Middleware order** ([main.go:73](../../server/cmd/api/main.go#L73)) is load-bearing; the new route goes in the existing authed `api` group — add no middleware.
- **Frontend:** balance on `authStore.user` (persists across app), immutable `setUser({...user, ...})` updates only; no `fetch()` in components; named exports only, filename matches export. ([project-context.md])

### Testing standards summary

- Go: `testing` + `testify`, co-located `_test.go`, **table-driven** for streak/bonus, **per-test transaction + rollback** for PostgreSQL, tests create their own data (no seed dependency).
- Frontend: Vitest, co-located `.test.tsx`, `data-testid` selectors, present-tense `it(...)`, cover the `0`-balance, `streak === 1` (no chip), and persistent-dialog (no auto-close) cases.

### Project Structure Notes

- The new `wallet` package fits `server/internal/<domain>/` (peer to `user`, `room`, `passwordreset`). **Variance from the usual pattern:** wallet's persistent state lives **on the `users` table** (AC #5), not its own table — so the repo reads/writes `user.User` rows rather than owning a table+model. The package still exists for cohesion (atomic mutation + math + handler) per AC #6. Document this in the package doc comment. Avoid the `user`→`wallet` import (cycle); `wallet`→`user` is fine.
- The wallet handler is a **POST** (`/wallet/daily-login`) because it mutates — not a side-effecting GET. It is the single grant point; reads come from the login/refresh/profile responses.

### References

- [Source: epics.md#Story 9.1](../planning-artifacts/epics.md) — ACs (1724-1761), Epic 9 overview (1720).
- [Source: sprint-change-proposal-2026-06-18.md](../planning-artifacts/sprint-change-proposal-2026-06-18.md) — 9.1 unchanged; economy constants are placeholders.
- [Source: project-context.md](../../_bmad-output/project-context.md) — GORM tag bridge, camelCase wire format, atomic wallet mutations, `*time.Time` nullable dates, store partitioning, JS-truthiness-on-Go-zero-values, migration numbering, feature-complete checklist, i18n-all-locales.
- [Source: server/internal/auth/{handler,service}.go](../../server/internal/auth/) — Login/Register/Refresh + shared `RegisterResponseData`; 7-day refresh token (the reason the grant is bootstrap-triggered, not login-triggered).
- [Source: server/internal/user/{model,handler,gorm_repo}.go](../../server/internal/user/) — User model, ProfileResponse, repo error/getUserID patterns.
- [Source: server/internal/passwordreset/gorm_repo.go](../../server/internal/passwordreset/gorm_repo.go) — atomic-guard precedent (`MarkUsed`, RowsAffected check).
- [Source: server/cmd/api/main.go](../../server/cmd/api/main.go) — DI wiring + route registration order.
- [Source: server/migrations/000007_add_bot_players.up.sql](../../server/migrations/000007_add_bot_players.up.sql) — additive `ALTER TABLE` migration style.
- [Source: client/src/shared/hooks/{useAuth.ts, mutations/useAuth.ts}](../../client/src/shared/hooks/) — the three User-hydration sites; refresh-token bootstrap.
- [Source: client/src/features/landing/components/ContactDialog.tsx](../../client/src/features/landing/components/ContactDialog.tsx) — shadcn Dialog usage pattern.
- [Source: client/src/shared/components/{TopBar.tsx, AppLayout.tsx}](../../client/src/shared/components/) — header render slot + authenticated shell mount point.

### Previous-story / Git intelligence

- Epic 9 is the first epic after Epics 1–8, 8.5, 10, and 12-3 all reached **done**. Story 9.1 is the **first story in Epic 9** — no prior 9.x story to inherit. Epic-9 was flipped `backlog → in-progress` when this story was created.
- Freshest shipped work (git log): the Epic 9 restructure (`feat(epics)`), the **password-reset flow** (`feat(auth)` — the `passwordreset` package is the freshest domain-package + atomic-guard + new-migration template; copy its shape), and return-to-room v1/v2 + room-ownership fixes. The password-reset PR added `000008` migration, confirming the additive-migration convention used here.
- Bot-players (10-3) added nullable player FKs + `isBot` flags — relevant later for 9.2 settlement (bots don't stake), not for 9.1.

## Dev Agent Record

### Agent Model Used

claude-opus-4-8 (Claude Code Dev Story workflow)

### Debug Log References

- `make migrate` against the dev DB (port 6433) applied `000007`→`000009` cleanly; verified `000009` down→up reverses with no residue (column add/drop round-trip).
- Backend integration + concurrency tests run against the live dev DB (`BELJOT_DB_URL=…:6433`): 4 concurrent `ProcessDailyLogin` calls on a fresh day granted exactly once (balance 5000→6000, streak→1).

### Completion Notes List

- **Wallet domain package** created (`server/internal/wallet/`) with `model.go` (DailyLoginResult + package doc on the on-`users`-table variance), `repository.go`, `service.go` (centralized constants `StartingBalance/DailyBase/DailyStep/DailyCap` + pure `evaluateDailyLogin`/`bonusForStreak` helpers), `gorm_repo.go` (transactional `FOR UPDATE` row-lock for race-free once-per-day grant), `handler.go` (`POST /api/v1/wallet/daily-login`).
- **Date logic** uses UTC calendar-date components (`utcDate` + `AddDate(0,0,1)`), never `Truncate(24h)`/duration math — DST/zone-safe per Dev Notes.
- **Migration 000009** adds `wallet_balance` (NOT NULL DEFAULT 5000 CHECK ≥0), `last_login_at` (DATE, nullable), `login_streak_days` (NOT NULL DEFAULT 0). Default kept in sync with `wallet.StartingBalance` (commented both places).
- **Auth handlers**: `RegisterResponseData` (shared by Register/Login/Refresh) gained `walletBalance`+`loginStreakDays`; Register seeds 5000/streak 0/`last_login=today` and grants nothing; Login/Refresh only echo. Profile (`ProfileResponse`) carries the private fields (self-only; never on a public/shared shape).
- **Frontend**: `User` type + `RegisterResponse`/`RefreshResponse` extended; **three** `setUser` hydration sites updated — `setAuthState`, `useAuthInit`, AND `axiosClient.doRefresh` (the 401-retry refresh; not listed in the task but it is a real third site — missing it would blank the header coin pill after a mid-session refresh).
- **Daily-reward gate** (`useDailyRewardGate` + `DailyRewardGate`) mounted in `AppLayout`, fires `claimDailyLogin()` once per session (StrictMode-safe ref guard, no cleanup-cancel so the reward still surfaces on remount; endpoint idempotent regardless). Persistent `DailyRewardDialog` uses Base UI's `disablePointerDismissal` + a no-op `onOpenChange` so only the explicit Collect button closes it (no auto-dismiss timer).
- **Display**: coin pill + "Day N" streak chip (shown only when `loginStreakDays > 1`) in `TopBar`; balance + login-streak pills in the profile `IdentityHero` (sourced from `authStore.user`, kept live by the gate). All numeric checks explicit — no JS truthiness on Go zero values.
- **i18n**: `rewards.dailyReward.*` + `wallet.*` added to all four locales (en/sr-Latin/mk-Cyrillic/hr); i18n parity test passes.
- **Tests**: wallet table-driven streak/bonus + DB integration + 4-way concurrency (exactly-once); auth register-stamps-and-echoes + login/refresh echo; profile includes wallet fields; `TopBar` coin pill (balance, zero balance, streak chip >1 only); `DailyRewardDialog` (opens/persists/Collect-closes/hidden when not granted). Updated ~13 existing User/auth-response fixtures for the new required fields.
- **Verification**: backend `go test ./...` (incl. live DB) all pass; `gofmt`/`go vet` clean. Frontend `vitest run` — 81 files / 828 tests pass; `eslint` 0 errors; `prettier --check` clean; build-scope `tsc` (tsconfig.build.json) clean. **`golangci-lint` is not installed locally** (CI runs it) — verified via `gofmt -l` + `go vet` instead. No WS contract files touched (HTTP-by-design). No new `apperr` errors needed.

### File List

**Backend (new):**

- `server/migrations/000009_add_wallet_columns_to_users.up.sql`
- `server/migrations/000009_add_wallet_columns_to_users.down.sql`
- `server/internal/wallet/model.go`
- `server/internal/wallet/repository.go`
- `server/internal/wallet/service.go`
- `server/internal/wallet/gorm_repo.go`
- `server/internal/wallet/handler.go`
- `server/internal/wallet/wallet_test.go`

**Backend (modified):**

- `server/internal/user/model.go`
- `server/internal/user/handler.go`
- `server/internal/user/handler_test.go`
- `server/internal/auth/handler.go`
- `server/internal/auth/handler_test.go`
- `server/cmd/api/main.go`

**Frontend (new):**

- `client/src/shared/api/wallet.ts`
- `client/src/features/lobby/hooks/useDailyRewardGate.ts`
- `client/src/features/lobby/components/DailyRewardGate.tsx`
- `client/src/features/lobby/components/DailyRewardDialog.tsx`
- `client/src/features/lobby/components/DailyRewardDialog.test.tsx`

**Frontend (modified):**

- `client/src/shared/types/apiTypes.ts`
- `client/src/shared/api/auth.ts`
- `client/src/shared/api/axiosClient.ts`
- `client/src/shared/hooks/mutations/useAuth.ts`
- `client/src/shared/hooks/useAuth.ts`
- `client/src/shared/components/AppLayout.tsx`
- `client/src/shared/components/TopBar.tsx`
- `client/src/features/profile/components/IdentityHero.tsx`
- `client/src/features/profile/ProfilePage.tsx`
- `client/src/shared/i18n/en.json`, `sr.json`, `mk.json`, `hr.json`

**Frontend (test fixtures updated for the new required `User`/auth-response fields):**

- `client/src/shared/components/TopBar.test.tsx` (new coin-pill tests + fixture)
- `client/src/shared/components/AppLayout.test.tsx` (wallet API mock + fixture)
- `client/src/shared/components/LanguageSelector.test.tsx`
- `client/src/shared/stores/authStore.test.ts`
- `client/src/features/auth/LoginPage.test.tsx`
- `client/src/features/auth/RegisterPage.test.tsx`
- `client/src/features/profile/ProfilePage.test.tsx`
- `client/src/features/lobby/MatchmakingPage.test.tsx`
- `client/src/features/match/MatchPage.test.tsx`
- `client/src/features/room/RoomPage.test.tsx`, `RoomPage.bots.test.tsx`, `RoomPage.diamond.test.tsx`, `RoomPage.locale.test.tsx`

### Change Log

| Date       | Change                                                                                                                                   |
| ---------- | ---------------------------------------------------------------------------------------------------------------------------------------- |
| 2026-06-18 | Implemented Story 9.1 Coin Wallet Foundation: migration 000009, `wallet` domain package (atomic daily-login), auth/profile wallet fields, frontend wallet client + bootstrap gate + persistent daily-reward dialog + header/profile display, i18n in all four locales, backend + frontend tests. Status → review. |
| 2026-06-18 | Code review (3-layer adversarial) + live Playwright E2E verification (all 5 scenarios + bonus-curve + idempotency confirmed against the dev DB). Patch: registration stamps the date-truncated UTC date into `last_login_at`. Status → done. |
| 2026-06-18 | Post-review UX tweaks (stakeholder): mk wording `Монети`→`Парички` and the non-Macedonian `Дневен низ`→`Денови по ред`; hr/sr streak label `Dnevni niz`→`Dana zaredom` (more idiomatic). Removed the "Day N" streak chip from the `TopBar` header — the login streak now shows only in the daily-reward dialog and the profile (AC #4 display narrowed); `TopBar.test.tsx` updated accordingly. |
| 2026-06-18 | Profile chip polish (stakeholder): `IdentityHero` coins pill shows the lucide `Coins` icon + amount (no text label), restyled to match the header pill exactly (neutral white surface, border, ink number, brass-deep icon); streak pill shows a 🔥 + count on the same white surface. Both keep the localized label as a hover `title` + accessible name (`Парички`/`Coins`, `Денови по ред`/`Day streak`). Header: coins pill moved to the left of the language selector. |
| 2026-06-18 | Further UX (stakeholder): added `wallet.streakTooltip` (all 4 locales) so the streak chip's hover explains the streak with a **person-neutral** phrasing that also reads correctly on other players' profiles (Epic 11) — mk "Игра {{days}} дена по ред", en "On a {{days}}-day play streak", hr/sr "Igra {{days}} dana zaredom" (no 2nd-person "you log in") — via a new `HeroPill` `titleText` prop (the accessible name stays the short label). Coin icon recoloured to an **off-theme gold** `COIN_GOLD` (`#D4A017`) in `client/src/shared/lib/coinGold.ts` — a documented palette exception mirroring `StreakCallout`'s `ICE` blue, used by both the header and profile coin icons. Language selector collapses to an icon-only globe button below `md` (lang code + chevron hidden), matching the hamburger. |

---

## Resolved Decisions (confirmed with stakeholder, 2026-06-18)

1. **Day-1 bonus at registration → NO.** Registration only seeds the wallet to 5 000 (and stamps `last_login_at = registration date`, `login_streak_days = 0`). The day-1 bonus first becomes claimable on the **next calendar day**. Reflected in AC #1 and Task 6.
2. **Daily-reward transport → HTTP, on the session-init path, not WebSocket.** A dedicated authenticated `POST /api/v1/wallet/daily-login`, called once on app bootstrap (covers refresh-token returning players, not just explicit login). Reflected in the Design Decision and Tasks 5/9.
3. **Notification UI → persistent dialog, not toast.** The reward shows in a modal that stays open until the player closes it. Reflected in AC #2 and Task 10.
4. **`last_login_at` granularity → UTC calendar days, not 24-hour windows.** Confirmed; date comparison uses calendar-date components (Task 4).

---

## Review Findings

_Code review 2026-06-18 — three adversarial layers (Blind Hunter, Edge Case Hunter, Acceptance Auditor). **AC #1–#6 all confirmed Met — no acceptance-blocking gap.** 7 findings dismissed as noise/by-design (getUserID 401-vs-500, redundant `streakDay`/`loginStreakDays` wire fields, refresh stale-balance race, `ProfileResponse` private-field exposure [verified self-only], `bonusForStreak` overflow [unreachable], legacy NULL `last_login_at` grant [documented], Base UI `disablePointerDismissal` vs spec's Radix props [justified]). Diff reviewed: uncommitted working tree vs `HEAD` (`ec5b247`) + new untracked files._

### Decision needed

- [x] [Review][Decision] **RESOLVED (2026-06-18, option a):** The `docker-compose.yml` (`5433`→`6433`) and `client/vite.config.ts` (`5173`→`6173`, `8080`→`9080`) edits are **local-machine-only** overrides (other project holds the default ports; dev needs them to run `make dev`). **Repo defaults stay** (`5433`/`8080`/`5173`); these two files must be **excluded from the Story 9.1 commit** and left modified in the working tree so the dev's local env keeps working. `wallet_test.go`'s `5433` DSN fallback matches the default and is correct as committed. No repo change needed — commit-hygiene only.
  - _Original finding: out-of-scope, internally-inconsistent dev-port changes swept into a wallet PR — committed `6433`/`9080`/`6173` in compose+vite while config.go/.env.example/Makefile/wallet_test.go stayed on `5433`/`8080`; on a fresh clone the frontend couldn't reach the API and the AC#3 integration tests would silently `t.Skip`._

### Patch

- [x] [Review][Patch] **FIXED (2026-06-18):** Registration now stamps the UTC calendar date (`time.Date(now.Year(), now.Month(), now.Day(), …, time.UTC)`) into `last_login_at` instead of the raw `time.Now().UTC()` instant, so the auth write path agrees with the wallet daily-login path (both date-truncated) rather than relying on the `DATE` column to silently truncate. `go vet` + `gofmt` clean; auth tests pass (incl. `TestRegister_SeedsWalletAndStampsLastLogin`). [server/internal/auth/handler.go]

### Deferred

- [x] [Review][Defer] **`useDailyRewardGate` re-POSTs `/wallet/daily-login` on every `AppLayout` remount, not once per app session.** Leaving the lobby for `/match/:roomId` (a route outside `AppLayout`) and returning unmounts/remounts the gate, resetting its ref guard and re-firing the claim. Server is idempotent (same-day repeat → `granted:false`), so no double-grant and no UX impact — only redundant traffic. "Once per session" is effectively "once per mount." [client/src/features/lobby/hooks/useDailyRewardGate.ts; client/src/shared/components/AppLayout.tsx] — deferred (D153), pre-existing-style minor efficiency, no correctness/UX impact.
