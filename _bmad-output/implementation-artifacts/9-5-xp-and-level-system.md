---
baseline_commit: 71fbe97af9cf39ef6ed33f6d5fab7d9a0bc40bc2
---

# Story 9.5: XP & Level System

Status: done

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

## Story

As a player,
I want to earn XP from matches and see my level grow over time,
so that I have a lifetime career signal independent of seasonal competitive standing.

> **Scope guardrail (read first):** This story delivers **lifetime XP + a derived level only**. It is a career *signal* with **NO gating behavior** of any kind. Do **NOT** build (or re-introduce) the Level-5 "unlock ranked mode" gate, the rank/LP/season-countdown `RankBanner`, ELO, or any ranked surface — those are the deferred **Epic 13** competitive system. The PRD's older journey prose mentions a "Level 5 gate"; the **epic AC is canonical and explicitly overrides it** ("the level is a lifetime value — it never resets, and has no gating behavior attached"). See [Design Decision D2](#design-decisions).

## Acceptance Criteria

**AC1 — Normal-completion XP award (FR33)**
**Given** a match completes normally (natural end or accepted surrender)
**When** XP is calculated
**Then** each **human** player earns XP proportional to the game points **their own team** scored across the match: `xpEarned = floor(teamGamePoints / xpPerGamePointDivisor)` with `xpPerGamePointDivisor = 10` (placeholder constant)
**And** the earned XP is added to that player's `total_xp` and persisted atomically
**And** **bots earn no XP** (a bot seat is never charged, paid, or XP-credited — same exclusion as coin settlement)
**And** both teammates on a team receive the same XP (their shared team's points ÷ 10); the losing team still earns XP for the points they scored (XP is a participation/progress reward, **not** zero-sum)

**AC2 — Lifetime level derived from total XP (FR34)**
**Given** a player accumulates XP
**When** their `total_xp` crosses a level threshold (placeholder quadratic: **Level N requires `50 × N²` total XP**)
**Then** their level is the largest `N` such that `50 × N² ≤ total_xp` (a new player at 0 XP is **Level 0**)
**And** `level` is **derived/computed from `total_xp` — it is NOT a stored column**
**And** the level is a lifetime value: it never resets and has **no gating behavior** attached anywhere in the system

**AC3 — Abandonment XP (FR43) — whole abandoning team forfeits (PO override 2026-06-22)**
**Given** a match ends by abandonment (a player does not reconnect within the window, or is auto-played to match end)
**When** XP outcomes are applied
**Then** the **entire abandoning team** (both members) receives **0 XP**
**And** the **non-abandoning team** (both opponents) each receive `floor(teamScores[nonAbandoningTeam] / 10) × abandonPartialXPFactor` XP — the **normal-end formula** (`abandonPartialXPFactor` defaults to **1.0**, a tunable const); partial-ness emerges naturally from the lower point total at the early end (PO chose "points-so-far" 2026-06-22 — see [D4](#design-decisions))
**And** this **aligns with coin settlement's team-based forfeit** — the abandoning team taking no XP is the fair punishment (PO decision 2026-06-22; this **overrides** the epic AC's original "only the abandoning player gets 0, the other three get partial" wording — see [Design Decision D4](#design-decisions))
**And** note this still differs from a **normal** loss, where the losing team **does** earn XP (AC1); abandonment is a punishment, a normal loss is not

**AC4 — Profile shows level + XP progress**
**Given** a player views their own profile
**When** the profile renders
**Then** their current **level** and **XP progress-to-next-level** (a labelled progress bar, e.g. "Level 3 · 1,240 / 1,800 XP") are shown
**And** the layout leaves room alongside the (not-yet-built) honor score and prior-season rank archive — render only the level/XP surface now; do **not** stub fake honor/rank values

**AC5 — Lobby banner shows level + XP bar**
**Given** a logged-in player is anywhere the top-nav is visible
**When** the top-nav renders
**Then** the player's **level + an XP progress bar** are shown in the top-nav zone (next to the existing coin balance pill)
**And** when XP is awarded at match end, the banner reflects the **new** level + XP without a full reload (driven by the WS award event updating the auth store, mirroring how `event:coin_settlement` live-updates the coin balance)

**AC6 — Schema & persistence**
**Given** the level system migration
**When** I inspect the database schema
**Then** the `users` table has a `total_xp` column: `INTEGER NOT NULL DEFAULT 0 CHECK (total_xp >= 0)`
**And** there is **no** `level` column — level is derived
**And** the migration has a matching `.down.sql` that fully reverses it

**AC7 — Match-end event delivery & ordering**
**Given** XP is awarded at match end (normal or abandonment)
**When** the server broadcasts results
**Then** a new per-human `event:xp_awarded` (added to **both** WS contract files) carries that player's `xpEarned`, `newTotalXp`, `newLevel`, and `leveledUp`
**And** it is slotted into the existing match-end broadcast sequence **after** `event:coin_settlement` and **before** the trailing `event:match_state`, preserving the Story 8.5-1 ordering contract
**And** XP persistence failures are logged and must **not** strand clients on the table (the match-end broadcasts still fire) — mirroring the settlement best-effort philosophy

**AC8 — No regressions; Definition of Done**
**Given** the existing match-end, settlement, abandonment, reconnect, and profile flows
**When** XP is layered in
**Then** none of those behaviors change (settlement deltas, match persistence, room-status flip, broadcast ordering all unchanged except the inserted XP event)
**And** the [feature-complete checklist](#testing-standards) passes: server changes + tests, WS event on **both** contract files, frontend components + co-located tests, i18n in **all four** locales, `make lint` + `make test` green

---

## Tasks / Subtasks

> **Read [Dev Notes](#dev-notes) in full before starting.** The two highest-leverage facts: (1) **XP hooks into the existing match-end flow exactly where coin settlement already runs** — `handleMatchEnd` ([match/live_match.go:1062](../../server/internal/match/live_match.go#L1062)) and the abandonment path ([match/reconnect.go:590](../../server/internal/match/reconnect.go#L590)) — reuse the `botSeats` guard and the inject-via-interface pattern, do not invent a new orchestration point. (2) **Abandonment forfeits XP for the whole abandoning team** (PO decision 2026-06-22 — overrides the epic's per-player text): both abandoning-team members get 0, only the non-abandoning team earns partial XP. This now mirrors coin settlement's team-based forfeit (but note a *normal* loss still earns XP — abandonment is the punishment).

- [x] **Task 1 — Migration `000011`: add `total_xp` to users (AC6)**
  - [x] Create `server/migrations/000011_add_xp_to_users.up.sql`: `ALTER TABLE users ADD COLUMN total_xp INTEGER NOT NULL DEFAULT 0 CHECK (total_xp >= 0);` — mirror the exact constraint style of [000009_add_wallet_columns_to_users.up.sql](../../server/migrations/000009_add_wallet_columns_to_users.up.sql) (`wallet_balance` is the template).
  - [x] Create the matching `000011_add_xp_to_users.down.sql`: `ALTER TABLE users DROP COLUMN total_xp;`.
  - [x] **Confirm `000011` is the next number** — highest existing is `000010_add_coin_economy_columns`. Never skip numbers.
  - [x] **No `level` column** (derived). **No per-match XP-delta columns** on the `matches` table — explicitly out of scope (see [Design Decision D5](#design-decisions)); do not add a second migration.

- [x] **Task 2 — User model field (AC1, AC6)**
  - [x] Add `TotalXP int` to the `User` struct in [server/internal/user/model.go:9-27](../../server/internal/user/model.go#L9), after `LoginStreakDays`, with tags `gorm:"not null;default:0" json:"totalXp"`. Mirror the `WalletBalance` field exactly (GORM default + camelCase JSON tag). Do **not** add a `Level` field.

- [x] **Task 3 — Level curve: pure, table-tested function (AC2)**
  - [x] Add `server/internal/user/level.go` with pure functions: `LevelForXP(totalXP int) int` (largest `N` with `50·N² ≤ totalXP`; returns `0` at `0` XP) and a helper for progress, e.g. `LevelProgress(totalXP int) (level, xpIntoLevel, xpForNextLevel int)`. Use a named const `levelCurveCoefficient = 50` (placeholder, tunable — see [D3](#design-decisions)).
  - [x] **Avoid float precision bugs at exact thresholds.** Prefer an integer approach (increment `N` while `levelCurveCoefficient*(N+1)*(N+1) <= totalXP`) over `math.Sqrt`. If you use `Sqrt`, you MUST verify exact-threshold boundaries with tests.
  - [x] Table-driven test `level_test.go`: assert boundaries **0→L0, 49→L0, 50→L1, 199→L1, 200→L2, 449→L2, 450→L3, 1250→L5**, and a large value. Assert `LevelProgress` returns sane `xpIntoLevel`/`xpForNextLevel` at and between thresholds.

- [x] **Task 4 — Atomic XP award (repository + service) (AC1, AC3)**
  - [x] Extend `UserRepository` ([server/internal/user/repository.go](../../server/internal/user/repository.go)) with `AddXP(awards map[uint]int) (map[uint]int, error)` — adds each delta to `total_xp` atomically and returns each user's **new** total. Implement in [user/gorm_repo.go] mirroring the wallet repo's pattern: one `db.Transaction`, `clause.Locking{Strength: "UPDATE"}`, **ascending userID order** (see [wallet/gorm_repo.go:77-114 `ChargeStakes`](../../server/internal/wallet/gorm_repo.go#L77) as the template). Skip zero-amount entries. Return `apperr.ErrUserNotFound` on a missing row.
  - [x] Add a thin XP service method (on the existing user service, or a small `user.XPService`) `ApplyXPAwards(awards map[uint]int) (newTotals map[uint]int, err error)` that calls the repo. This is what the match manager will be given. (See [Design Decision D1](#design-decisions) for why XP lives in `internal/user`, not a new package.)
  - [x] Update **all `UserRepository` mocks/fakes** surfaced by `grep -rn "UserRepository\|FindByID" server/internal | grep -i mock` so they implement `AddXP`.

- [x] **Task 5 — Match-end XP integration: the `XPAwarder` interface + computation (AC1, AC3, AC7)**
  - [x] In the `match` package, define an `XPAwarder` interface + `SetXPAwarder` injector, **mirroring `WalletSettler`/`SetWalletSettler`** ([match/live_match.go:62-71, 130-134](../../server/internal/match/live_match.go#L62)): `type XPAwarder interface { ApplyXPAwards(awards map[uint]int) (map[uint]int, error) }`. Add an `xpAwarder XPAwarder` field to `Manager`. Nil → XP is skipped (deltas still 0; no event), same nil-tolerant philosophy as `walletSettler`. **[Dev note: interface also carries `LevelForXP(totalXP int) int` so the manager can compute `NewLevel`/`LeveledUp` without importing `user` — `user` imports `match`, so `match` must never import `user` (D1). Cycle-free realization of D1's intent.]**
  - [x] Add `awardXP(...)` to a new `match/xp_award.go` (parallel to [match/settlement.go](../../server/internal/match/settlement.go)). It computes the seat→delta map (pure helper `computeXPAwards`), calls `m.xpAwarder.ApplyXPAwards`, reads back new totals, and returns prepared per-human `event:xp_awarded` messages (mirror the `coinSettlementMsg` struct + builder in settlement.go).
  - [x] **`computeXPAwards(playerIDs, botSeats, teamScores [2]int, abandonedSeat int) (deltas [4]int)`** — pure, table-tested (normal split, bot/empty → 0, whole abandoning team → 0, non-abandoning earns, normal-loss-vs-abandonment contrast). Exact bot/empty guard from settlement.go reused.
  - [x] Wire `awardXP` into **both** finalize paths, **after** `settleMatch` returns (`handleMatchEnd` with `abandonedSeat = -1` + `finalState.TeamScores`; abandonment path with the real `abandonedSeat` + `[2]int{teamAScore, teamBScore}`).
  - [x] **Broadcast ordering (AC7):** in **both** paths, send the per-human `event:xp_awarded` after the `event:coin_settlement` loop and before the trailing `event:match_state`. Net order: `match_end`/`match_abandoned` → `coin_settlement`(s) → `xp_awarded`(s) → `match_state`.

- [x] **Task 6 — WS event contract on BOTH files (AC7)**
  - [x] Backend: add `const EventXPAwarded = "event:xp_awarded"` + `type XPAwardedPayload struct {...}` to [server/internal/ws/events.go](../../server/internal/ws/events.go). `NewLevel`/`LeveledUp` computed in `awardXP` via the `XPAwarder.LevelForXP` method.
  - [x] Frontend: add the mirror `EVENT_XP_AWARDED` constant + `XpAwardedPayload` interface to [client/src/shared/types/wsEvents.ts](../../client/src/shared/types/wsEvents.ts).

- [x] **Task 7 — Backend: profile endpoint returns level + XP (AC4)**
  - [x] In [server/internal/user/handler.go](../../server/internal/user/handler.go) add `TotalXP`/`Level`/`XPIntoLevel`/`XPForNextLevel` to `ProfileResponse`; populate in `GetProfile` via `LevelProgress(u.TotalXP)`.
  - [x] Auth `RegisterResponseData` (Register/Login/Refresh) now carries `totalXp` + `level` (`user.LevelForXP(u.TotalXP)`) so the banner has them on load.

- [x] **Task 8 — Frontend types + store update on award (AC5)**
  - [x] Add `level`/`totalXp` to the `User` interface ([apiTypes.ts](../../client/src/shared/types/apiTypes.ts)); add `totalXp`/`level`/`xpIntoLevel`/`xpForNextLevel` to `ProfileResponse` ([profile.ts](../../client/src/shared/api/profile.ts)); add `totalXp`/`level` to the auth response types + the two `setUser` mappings (`useAuth.ts`, `axiosClient.ts`, `mutations/useAuth.ts`).
  - [x] In [useWsDispatch.ts](../../client/src/shared/hooks/useWsDispatch.ts), add the `EVENT_XP_AWARDED` handler mirroring `EVENT_COIN_SETTLEMENT`: validate (explicit int/bool guards, no truthiness), `setUser({ ...user, totalXp, level })`, stash on `matchStore.xpAward`; cleared on `match_end`.

- [x] **Task 9 — Frontend: level + XP bar in top-nav (AC5) and on profile (AC4)**
  - [x] **Top-nav (AC5):** level + XP bar beside the coin pill in [TopBar.tsx](../../client/src/shared/components/TopBar.tsx) (`xp-level` + `xp-bar` testids), reading `user.level`/`user.totalXp`. No shadcn `Progress` present → new reusable `XpBar` Tailwind bar (accent fill, progressbar a11y).
  - [x] **Profile (AC4):** brass level pill (`Sparkles` icon, `tone="brass"`) + labelled XP bar in [IdentityHero.tsx](../../client/src/features/profile/components/IdentityHero.tsx) (`profile-level`/`profile-xp`/`profile-xp-bar`); server-provided `xpIntoLevel`/`xpForNextLevel`. Honor/rank space left empty (no stubs).
  - [x] **Progress math:** single client curve helper [xpLevel.ts](../../client/src/shared/lib/xpLevel.ts) mirroring the server `50·N²` const with a documented keep-in-sync comment (used by the top-nav; the profile uses server-provided progress — curve in exactly one client location per D6).
  - [x] **Optional (done):** "You earned {{amount}} XP" + level-up flourish in [MatchResult.tsx](../../client/src/features/match/components/MatchResult.tsx) (`match-result-xp`), fed by `matchStore.xpAward`.

- [x] **Task 10 — i18n in all four locales (AC8)**
  - [x] Added an `xp` block (`short`, `levelLabel`, `progress`, `progressLabel`, `xpGained`, `levelUp`) to [en](../../client/src/shared/i18n/en.json)/[sr](../../client/src/shared/i18n/sr.json)/[mk](../../client/src/shared/i18n/mk.json)/[hr](../../client/src/shared/i18n/hr.json).
  - [x] No em-dash in mk/sr/hr; mk is all-Cyrillic (XP rendered as „поени/искуствени поени“); i18n parity test green.

- [x] **Task 11 — Wiring (AC1, AC3)**
  - [x] In [server/cmd/api/main.go](../../server/cmd/api/main.go#L152) `sessionManager.SetXPAwarder(user.NewXPService(userRepo))` next to `SetWalletSettler`.

- [x] **Task 12 — Tests (AC1–AC8)**
  - [x] **Curve:** `level_test.go` boundary table tests.
  - [x] **`computeXPAwards` (pure):** `xp_award_test.go` — normal split, loser earns, bot/empty → 0, whole abandoning team → 0, non-abandoning earns, normal-loss-vs-abandonment contrast, zero scores.
  - [x] **`AddXP` repo:** `xp_repo_test.go` — atomic add + new totals, skip zero, empty no-op, missing user → `ErrUserNotFound` + rollback (per-test tx rollback harness).
  - [x] **Match-manager integration:** `xp_wiring_test.go` — `stubXPAwarder` + hub spy; normal-end (free match still awards), no-awarder no-event, bot-seat exclusion, abandonment (team forfeit + ordering after coin_settlement, before match_state). `AbandonSeatForTest` export hook added.
  - [x] **Profile handler:** `TestGetProfile_IncludesXPAndLevel`.
  - [x] **Frontend (Vitest, `data-testid`):** TopBar (level + bar from store); `useWsDispatch` XP dispatch (4 cases); IdentityHero (level + progress); MatchResult (XP + level-up); `xpLevel.test.ts` (client↔server curve sync); i18n parity green.
  - [x] `make test` green both stacks (server `go test ./...` + `go vet`; client `vitest` 884 tests). ESLint + Prettier clean on changed files; client build-config `tsc` clean. `gofmt` clean. `golangci-lint` v1.64.8 (matching CI) installed mid-review and `golangci-lint run ./...` is **clean**.

---

## Dev Notes

### What this story actually is

A **lifetime, non-gating career signal.** You add one nullable-free integer column (`users.total_xp`), a **pure level curve** (`level = ⌊√(total_xp/50)⌋`, computed never stored), and you award XP at the two existing match-end exits — reusing the proven inject-an-interface + per-human-event + broadcast-after-settlement machinery from Story 9.2. Then you surface level + an XP bar in two places: the profile and the top-nav. That's it. **No ranked, no ELO, no LP, no season, no Level-5 unlock, no honor.** The hard part is not the code volume — it's resisting scope creep from the PRD/UX prose that describes the full competitive system (deferred to Epic 13), and getting the **abandonment per-player XP rule** right (it is NOT the coin rule).

### The concrete algorithm

```
# Level curve (pure, server-authoritative; placeholder constant = 50)
threshold(N) = 50 * N^2
LevelForXP(xp) = largest N with threshold(N) <= xp          # 0 XP -> Level 0
  e.g. 0..49 -> L0 | 50..199 -> L1 | 200..449 -> L2 | 450..799 -> L3 | 800..1249 -> L4 | 1250+ -> L5

# XP award at match end (per HUMAN seat; bots & empty seats always 0)
normal end:
  delta(seat) = floor(teamScores[TeamForSeat(seat)] / 10)   # both teammates equal; losers still earn
abandonment (abandonedSeat known) -- abandoning TEAM forfeits (PO override, mirrors coins):
  delta(seat on abandoning team)     = 0
  delta(seat on non-abandoning team) = floor(teamScores[nonAbandTeam] / 10) * abandonPartialXPFactor  # placeholder, see D4

# Persist atomically, then broadcast per-human:
total_xp[user] += delta            # FOR UPDATE, ascending userID (mirror wallet ChargeStakes)
event:xp_awarded {xpEarned, newTotalXp, newLevel=LevelForXP(newTotalXp), leveledUp}
# ordering: match_end/abandoned -> coin_settlement(s) -> xp_awarded(s) -> match_state
```

### Current state of the code being modified (READ THESE)

| File | Today | This story changes |
|---|---|---|
| [match/live_match.go `handleMatchEnd`](../../server/internal/match/live_match.go#L1045) | Settles coins (line 1062), builds + persists `matchRecord`, flips room status, broadcasts `match_end → coin_settlement(s) → match_state` (lines 1116-1120), removes session. `botSeats` already computed at 1051-1054; `finalState.TeamScores[0/1]` available. | Add `awardXP(...)` call right after `settleMatch`; insert `xp_awarded` broadcast between the settlement loop and `match_state`. No other change. |
| [match/reconnect.go (abandon path)](../../server/internal/match/reconnect.go#L590) | On reconnect-timeout abandonment: settles coins against the **non-abandoning team** as winner (line 589-590), broadcasts `match_abandoned → coin_settlement(s) → match_state` (594-598), persists `Status:"abandoned"`. `abandonedSeat`, `botSeats`, `teamAScore/teamBScore` all in scope. | Add `awardXP(..., abandonedSeat, ...)` after line 590; insert `xp_awarded` between settlement loop and `match_state`. **Abandoning team=0, non-abandoning team=partial** (PO override) — reuse the same team-based forfeit shape as coin settlement. |
| [match/settlement.go](../../server/internal/match/settlement.go) | `settleMatch`/`computeSettlement` + `coinSettlementMsg`. The bot/empty guard at line 67 (`botSeats[seat]` or `playerIDs[seat]==0` → `continue`) and the prepared-msg pattern are the **templates to copy** for `match/xp_award.go`. | **No change** — copy its shape into a sibling `xp_award.go`. |
| [match/live_match.go `WalletSettler`/`SetWalletSettler`](../../server/internal/match/live_match.go#L62) | Interface subset of `*wallet.Service`, injected, nil-tolerant; `Manager.walletSettler` field. | Add a parallel `XPAwarder` interface, `Manager.xpAwarder` field, `SetXPAwarder`. |
| [user/model.go `User`](../../server/internal/user/model.go#L9) | `WalletBalance int gorm:"not null;default:5000"` + `LastLoginAt`/`LoginStreakDays` (Story 9.1). | Add `TotalXP int gorm:"not null;default:0" json:"totalXp"`. No `Level` field. |
| [user/repository.go `UserRepository`](../../server/internal/user/repository.go) | `Create/FindByEmail/FindByUsername/FindByID/FindManyByIDs/Count/UpdateLanguagePreference/UpdatePasswordHash`. | Add `AddXP(awards map[uint]int) (map[uint]int, error)` + gorm impl + update all mocks. |
| [user/handler.go `ProfileResponse`/`GetProfile`](../../server/internal/user/handler.go#L174) | Returns id/username/lang/walletBalance/loginStreakDays/createdAt + win/loss/abandoned stats. | Add `totalXp`/`level`/`xpIntoLevel`/`xpForNextLevel`. |
| [ws/events.go](../../server/internal/ws/events.go#L51) | `EventCoinSettlement` + `CoinSettlementPayload` are the per-human match-end event template. | Add `EventXPAwarded` + `XPAwardedPayload`. |
| [wallet/gorm_repo.go `ChargeStakes`](../../server/internal/wallet/gorm_repo.go#L77) | Atomic per-user mutation: one tx, `FOR UPDATE`, ascending userID, `ErrUserNotFound`. | **No change** — the template for `AddXP`. |
| [cmd/api/main.go](../../server/cmd/api/main.go#L152) | `sessionManager := match.NewManager(...)`; `SetRoomUpdater`; `SetWalletSettler(walletService)` (156). `userRepo` built earlier for the user handler. | Add `sessionManager.SetXPAwarder(xpService)` built from `userRepo`. |
| [client TopBar.tsx](../../client/src/shared/components/TopBar.tsx#L109) | Coin balance pill reads `user.walletBalance` from auth store. | Add a level + XP-bar element beside it. |
| [client IdentityHero.tsx](../../client/src/features/profile/components/IdentityHero.tsx) | Hero-pill row (games/wins/losses/capots/coins/streak). | Add a brass level pill + XP progress bar. |
| [client useWsDispatch.ts](../../client/src/shared/hooks/useWsDispatch.ts) | `EVENT_COIN_SETTLEMENT` updates `authStore.user.walletBalance`. | Add `EVENT_XP_AWARDED` → update `user.totalXp`/`user.level`. |
| [client apiTypes.ts `User`](../../client/src/shared/types/apiTypes.ts#L14), [profile.ts](../../client/src/shared/api/profile.ts), [wsEvents.ts](../../client/src/shared/types/wsEvents.ts) | User has walletBalance/loginStreakDays. No XP. | Add `level`/`totalXp` (+ progress on ProfileResponse) + the XP event types. |

### Reuse map — DO NOT reinvent

- **Inject-an-interface at match end:** `WalletSettler` + `SetWalletSettler` ([live_match.go:62-71, 130-134](../../server/internal/match/live_match.go#L62)). Clone the exact shape for `XPAwarder`.
- **Per-human prepared event:** `coinSettlementMsg{userID, msg}` + `buildMessage(ws.Event..., payload)` ([settlement.go:13-16, 75-76](../../server/internal/match/settlement.go#L13)).
- **Bot/empty-seat guard:** `if botSeats[seat] || playerIDs[seat] == 0 { continue }` ([settlement.go:67](../../server/internal/match/settlement.go#L67)). `humanUserIDs(...)` and `TeamForSeat(seat)` helpers already exist.
- **Atomic per-user balance mutation:** `wallet/gorm_repo.go ChargeStakes` ([:77-114](../../server/internal/wallet/gorm_repo.go#L77)) — `db.Transaction`, `clause.Locking{Strength:"UPDATE"}`, ascending IDs, `ErrUserNotFound`.
- **Live store update from a match-end event:** the `EVENT_COIN_SETTLEMENT` handler in `useWsDispatch.ts` (updates `authStore.user.walletBalance`) — mirror it for `totalXp`/`level`.
- **Hero pill + progress UI:** `HeroPill` in `IdentityHero.tsx`; coin pill in `TopBar.tsx`; the shadcn `Progress` component (UX `Progress` ↔ "XP level bar").

### Critical correctness rules (project-context.md + Story 9.2/9.3 learnings)

- **NO gating, ever.** Level unlocks nothing. Do not add a Level-5 check anywhere. (AC2, [D2](#design-decisions))
- **Level is derived, not stored.** Single source of truth = `total_xp`; compute level on read. No `level` column, no level cache to keep in sync.
- **Abandonment XP mirrors abandonment coins (team-based forfeit; PO override 2026-06-22).** The **whole abandoning team** gets 0 XP; only the **non-abandoning team** earns partial XP. This intentionally overrides the epic AC's per-player wording. It still differs from a **normal** loss, where the losing team **does** earn XP — abandonment is a punishment, a normal loss is not. Guard the team-forfeit + the normal-loss-still-earns cases with dedicated tests. (AC3, [D4](#design-decisions))
- **Bots accrue nothing** — Story 10.3 contract ("bots accrue no XP, coins, honor, or stats"). Same `botSeats[seat] || playerIDs[seat]==0` guard as settlement.
- **XP mutation is atomic/transactional** (FOR UPDATE, ascending userID) — even though XP can't go negative, keep parity with wallet so concurrent match-ends and the same-user-in-back-to-back-matches case are race-free.
- **Best-effort, never strand clients.** An `ApplyXPAwards` failure is logged (`slog.Error`) and the XP event is skipped — but `match_end`/`match_abandoned` and `match_state` still broadcast. Mirror settlement's degradation philosophy ([settlement.go:24-28, 41-56](../../server/internal/match/settlement.go#L24)).
- **Broadcast order is load-bearing** (Story 8.5-1): `match_end`/`match_abandoned` → `coin_settlement`(s) → **`xp_awarded`(s)** → `match_state`. Multi-event match-end sequences are separate ordered messages, never batched.
- **WS contract = two files in one commit:** `events.go` + `wsEvents.ts` together, no exceptions.
- **Wire format `camelCase`** (`totalXp`, `newTotalXp`, `xpEarned`); GORM column `snake_case` (`total_xp`). Go zero values serialize as real values — on the client use explicit comparisons, never truthiness, for numeric XP fields.

### Previous Story Intelligence (Story 9.4 — immediate predecessor; and 9.2/9.3)

From `9-4-quick-play-coin-bracketing.md` + `9-2`/`9-3` Dev Agent Records:
- **9.4 explicitly deferred XP to this story** ("this story adds no XP/honor code"; AC3 there: a free Quick Play match "still counts for XP/honor per the other Epic 9 stories — XP = Story 9.5"). So Quick Play / free-bracket matches **must award XP normally** here — XP is independent of `coinBuyIn` (a 0-coin match still scores game points → still earns XP). Add a test for the 0-buy-in case.
- The match-end settlement injection (`SetWalletSettler`) and the per-human `event:coin_settlement` + 8.5-1 broadcast ordering are battle-tested across 9.2/9.3/9.4 — you are adding one more event in the same slot, not redesigning anything.
- **Test harness:** the wallet side uses a `stubWallet` + broadcast-capture (`setupCoinTestBC`, `broadcastsOfType`) in the `room` package; the `match` manager tests have their own hub spy + match-end fixtures. Add a `stubXPAwarder` the same way. Tests are co-located, table-driven, create their own data (no seed dependency).
- **i18n discipline (9.4 review patch):** mk strings must be all-Cyrillic and idiomatic; a 9.4 review patch re-quoted a button name as `„Брза игра“` to match mk prose convention — follow the same quoting/idiom conventions for any mk/hr/sr strings. i18n parity test is enforced.

### Git Intelligence

The last several commits are bot-strategy refinements (`feat(bot): …`) plus the matchmaking/economy landings (`9-2`, `9-3`, `9-4`, "match stake HUD"). The economy hot paths this story touches — `match/live_match.go`, `match/reconnect.go`, `match/settlement.go`, `ws/events.go`, `user/*`, `useWsDispatch.ts`, `TopBar.tsx`, `IdentityHero.tsx` — are freshly stabilized. Bots are now real seat occupants at match end (`finalState.Players[i].IsBot` / `botSeats`), which is exactly why the bot-exclusion guard is non-negotiable. Follow the convention: branch `feat/9-5-xp-and-level-system`, one story = one branch = one PR; commit scope `feat(user):` / `feat(match):` per the touched package.

### Project Structure Notes

- Backend domain shape is fixed (`model.go`, `repository.go`, `gorm_repo.go`, `handler.go`, `service.go`, `_test.go`). XP lives in the **existing `internal/user`** package ([D1](#design-decisions)); the match-end glue is a new `internal/match/xp_award.go` sibling to `settlement.go`. **No new top-level package, one migration only.**
- GORM conventions: `total_xp` snake_case column, `json:"totalXp"` camelCase tag, exported field. Tests co-located; Go table-driven; rules-engine factories irrelevant (no engine change). Frontend: named exports, `data-testid` selection, feature-folder placement.

### Design Decisions

- **D1 — XP code lives in `internal/user`, not a new `internal/xp` package.** The architecture FR-map puts Progression (FR33–FR40) in `internal/user/` ([architecture.md:887](../planning-artifacts/architecture.md#L887)). `total_xp` is a `users` column and the level curve is consumed by the profile handler (in `user`). A separate `internal/xp` package would import `user` for the model, while `user/handler.go` would import `xp` for `LevelForXP` → **circular import**. Keeping the pure curve + `AddXP` in `user` avoids the cycle and matches the doc. The match manager stays decoupled via the structural `XPAwarder` interface (it never imports `user`). *(The wallet package precedent was considered; it doesn't apply because nothing in `user` calls back into `wallet`, so there's no cycle there.)*
- **D2 — No gating; epic AC overrides PRD prose (RESOLVED).** PRD FR34 / journey text describe a "Level 5 unlocks ranked" gate and a rank/LP/season `RankBanner`. The **canonical epic AC** ([epics.md:1898](../planning-artifacts/epics.md#L1898)) states the level "has no gating behavior attached." Ranked/ELO/LP/season is **Epic 13 (deferred)**. Build the lifetime level + XP bar only. The UX `RankBanner` spec ([ux-design-specification.md:753](../planning-artifacts/ux-design-specification.md#L753)) is the Epic-13 surface — the "top-nav zone" XP bar here is the lightweight lifetime-level element, not the full rank banner.
- **D3 — Level curve `50·N²` is a tunable placeholder (per 2026-06-18 reorg: "level curve … stays a placeholder, tuned during each story's planning").** Implement as a named const so a future tuning pass is a one-line change. Sanity check: a 1001-point win ≈ `⌊1001/10⌋ ≈ 100` XP, a loss ≈ 60–90 XP → Level 5 (1250 XP) in ~13–18 matches, consistent with the PRD journey's "~20 games to Level 5."
- **D4 — Abandonment forfeits XP for the whole abandoning team; the non-abandoning team earns "points-so-far" XP (both RESOLVED by PO, 2026-06-22).** The epic AC and PRD originally said "the abandoning **player** gets 0, the **other three** get partial XP." The PO (Emilijan) overrode this: **the entire abandoning team forfeits all XP** (the fair punishment), mirroring coin settlement's team-based forfeit; only the **non-abandoning team** earns. For the non-abandoning team's amount, the PO chose the **points-so-far** formula (Option A): each non-abandoning-team player earns `floor(teamScores[nonAbandoningTeam] / 10) × abandonPartialXPFactor`, with `abandonPartialXPFactor` a named const defaulting to **1.0** — i.e. **exactly the normal-end formula**; "partial proportional to game progress" emerges naturally because the match ended early with fewer points on the board. The abandoning team gets 0. **Implementation elegance:** compute the normal-end XP for all four seats, then **zero the abandoning team's two seats** — one code path, no separate partial branch. Keep `abandonPartialXPFactor` as a single tunable const (a future discount is then a one-number change). Both prior open questions are now closed: the formula basis is points-so-far (not hands-played / completion-%), and the canonical epic AC ([epics.md → Story 9.5 abandonment block](../planning-artifacts/epics.md#L1900)) has been **synced** to the team-forfeit wording (2026-06-22).
- **D5 — No per-match XP-delta columns (scope boundary).** Coin deltas are persisted on the `matches` row; XP deliberately is **not** (the AC requires only `users.total_xp`). The durable record is `total_xp`; the per-match earned amount rides the `event:xp_awarded` payload for display. Avoids a second migration and match-model churn. If match-history XP display is wanted later, it's a separate story.
- **D6 — XP-bar fill: server is authoritative for level; the bar % is display math.** `total_xp` and `level` are server-authoritative (level is sent, never derived on the client for any decision). The progress-bar *fill* is cosmetic. Primary approach: profile endpoint returns `xpIntoLevel`/`xpForNextLevel`; the top-nav uses a tiny client curve helper that mirrors the `50·N²` const (documented `// keep in sync with server` comment, the established manual-sync convention). Either is acceptable; do not duplicate the curve in more than one client location.

### Testing Standards

- **Backend:** Go `testing` + `testify`; table-driven; co-located (`level_test.go`, `xp_award_test.go` next to source); tests create their own data (no `make seed` dependency). Curve + `computeXPAwards` are **pure** → exhaustive table tests including exact thresholds and the abandonment/bot branches. `AddXP` repo test uses per-test transaction-rollback. Match-manager integration uses a `stubXPAwarder` + hub spy asserting event presence, per-human targeting, and slot ordering. `go test ./... && go vet ./...` clean.
- **Frontend:** Vitest; presentational; `data-testid` selection (never CSS classes); `it('renders …')` present tense. Assert: TopBar renders level + bar from store; `useWsDispatch` updates `authStore.user` on `event:xp_awarded`; IdentityHero renders level pill + progress; i18n parity across en/sr/mk/hr.
- **Definition of Done (hard gate):** server handler/repo + tests; WS event on **both** contract files; frontend components + co-located tests; i18n in **all four** locales; `make lint`; `make test`. No new domain errors expected (XP can't fail a user-facing validation); if one is added, it goes in `internal/apperr/errors.go`.

### References

- Epic + ACs: [epics.md → Story 9.5](../planning-artifacts/epics.md#L1882-L1912); Epic 9 overview [epics.md:1720](../planning-artifacts/epics.md#L1720); cross-story XP note (9.4) [epics.md:1880](../planning-artifacts/epics.md#L1880); bots-no-XP (10.3) [epics.md:2128](../planning-artifacts/epics.md#L2128).
- FRs: FR33 (XP from completed matches), FR34 (lifetime level, **no gating** per epic), FR43 (partial XP on abandonment) — [epics.md:228-238](../planning-artifacts/epics.md#L228). FR42 career stats display is Epic 11 ([2026-06-18 proposal §Deferred](../planning-artifacts/sprint-change-proposal-2026-06-18.md#L94)).
- Architecture: Progression → `internal/user/` extended ([architecture.md:887](../planning-artifacts/architecture.md#L887)); FR43 formula undefined ([architecture.md:1003-1005](../planning-artifacts/architecture.md#L1003)); naming-conversion chain + GORM tag bridge ([project-context.md](../project-context.md)).
- UX: `Progress` ↔ "XP level bar", `Badge` ↔ "Player level display" ([ux-design-specification.md:647-648](../planning-artifacts/ux-design-specification.md#L647)); RankBanner is the **deferred Epic-13** surface ([ux-design-specification.md:753](../planning-artifacts/ux-design-specification.md#L753)).
- Match-end hooks: `handleMatchEnd` [live_match.go:1045-1123](../../server/internal/match/live_match.go#L1045); abandonment [reconnect.go:560-653](../../server/internal/match/reconnect.go#L560); settlement template [settlement.go](../../server/internal/match/settlement.go); injector pattern [live_match.go:62-134](../../server/internal/match/live_match.go#L62); atomic mutation template [wallet/gorm_repo.go:77-114](../../server/internal/wallet/gorm_repo.go#L77); wiring [main.go:152-156](../../server/cmd/api/main.go#L152).
- Schema: migrations dir (next = `000011`); template [000009_add_wallet_columns_to_users.up.sql](../../server/migrations/000009_add_wallet_columns_to_users.up.sql); user model [user/model.go:9-27](../../server/internal/user/model.go#L9); profile [user/handler.go:174-216](../../server/internal/user/handler.go#L174).
- Frontend: [TopBar.tsx:109-119](../../client/src/shared/components/TopBar.tsx#L109), [IdentityHero.tsx](../../client/src/features/profile/components/IdentityHero.tsx), [useWsDispatch.ts](../../client/src/shared/hooks/useWsDispatch.ts), [authStore.ts](../../client/src/shared/stores/authStore.ts), [apiTypes.ts:14-24](../../client/src/shared/types/apiTypes.ts#L14), [profile.ts](../../client/src/shared/api/profile.ts), [wsEvents.ts](../../client/src/shared/types/wsEvents.ts), [MatchResult.tsx](../../client/src/features/match/components/MatchResult.tsx), i18n [en](../../client/src/shared/i18n/en.json)/[sr](../../client/src/shared/i18n/sr.json)/[mk](../../client/src/shared/i18n/mk.json)/[hr](../../client/src/shared/i18n/hr.json).
- Predecessor: [9-4-quick-play-coin-bracketing.md](9-4-quick-play-coin-bracketing.md). Project rules: [project-context.md](../project-context.md) (server-authoritative, atomic mutations, camelCase wire, GORM conventions, no-em-dash in mk/sr/hr, WS contract two-files).

## Dev Agent Record

### Agent Model Used

claude-opus-4-8 (Opus 4.8, 1M context) — BMad create-story workflow; implementation via BMad dev-story workflow (same model).

### Debug Log References

- Dev env uses Postgres on host port **6433** (`docker-compose.yml` remapped from 5433, which is occupied by an unrelated container). The Go test default DSN points at 5433, so DB-backed tests were run with `BELJOT_DB_URL=postgres://beljot:beljot_dev_password@localhost:6433/beljot?sslmode=disable`. Migration `000011` applied + down/up roundtrip verified against that DB.
- `golangci-lint` installed mid-review via `go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest` (v1.64.8, matching CI). `golangci-lint run ./...`, `gofmt`, `go build ./...`, and `go vet ./...` all clean.

### Completion Notes List

- **Architecture (cycle avoidance):** `user/handler.go` imports `match`, so `match` must never import `user`. The level curve lives in `internal/user` (D1), and the match manager reaches it through the injected `XPAwarder` interface, which carries both `ApplyXPAwards` and a `LevelForXP(int) int` method — letting `awardXP` compute `NewLevel`/`LeveledUp` for `event:xp_awarded` without importing `user`. This realizes D1's "match stays decoupled, never imports user" intent (the story's one-method interface sketch would have forced a `match → user` import cycle).
- **Abandonment XP (PO override 2026-06-22):** the whole abandoning team forfeits XP (0); only the non-abandoning team earns, at the points-so-far amount (`abandonPartialXPFactor` default 1.0). A normal loss still earns — guarded by a dedicated contrast test.
- **XP is coin-independent:** `computeXPAwards` takes no buy-in; a free (0-coin) Quick Play match still awards XP (covered by `TestHandleMatchEnd_AwardsXP` on a 0-buy-in match).
- **Best-effort, ordered:** an `ApplyXPAwards` failure logs and skips the event but never blocks `match_end`/`match_abandoned`/`match_state`. `event:xp_awarded` is slotted after `coin_settlement` and before the trailing `match_state` in both finalize paths.
- **Level derived, never stored:** single source of truth is `users.total_xp`; level is computed on read (server) and the bar fill is cosmetic client display math (one client curve copy in `xpLevel.ts`, kept in sync with the server `50·N²` const).
- **Scope held:** lifetime level + XP bar only — no ranked/ELO/LP/season/Level-5 gate (Epic 13). Honor/rank space left visually open on the profile with nothing stubbed.
- **No new domain errors** were needed (XP cannot fail a user-facing validation); `AddXP` reuses `apperr.ErrUserNotFound`.
- **Validation:** server `go test ./...` + `go vet` green; client `vitest` 884 tests green (incl. i18n parity, wsEvents contract); client build-config `tsc` clean; ESLint + Prettier clean on changed files. The 4 `UserRepository` mocks/fakes (auth, chat, lobby, user-handler) updated for the new `AddXP` method; widely-used `User`/auth-response test fixtures updated for the new required `totalXp`/`level` fields. (Pre-existing `RoomDetail.returnedUserIds` test-fixture type drift is unrelated and left untouched — it does not affect `vitest` or the build-config `tsc` gate.)

### File List

**Server — created**

- `server/migrations/000011_add_xp_to_users.up.sql`
- `server/migrations/000011_add_xp_to_users.down.sql`
- `server/internal/user/level.go`
- `server/internal/user/level_test.go`
- `server/internal/user/xp_service.go`
- `server/internal/user/xp_repo_test.go`
- `server/internal/match/xp_award.go`
- `server/internal/match/xp_award_test.go`
- `server/internal/match/xp_wiring_test.go`

**Server — modified**

- `server/internal/user/model.go` (TotalXP field)
- `server/internal/user/repository.go` (AddXP interface method)
- `server/internal/user/gorm_repo.go` (AddXP impl)
- `server/internal/user/handler.go` (ProfileResponse + GetProfile XP/level/progress)
- `server/internal/user/handler_test.go` (mock AddXP + TestGetProfile_IncludesXPAndLevel)
- `server/internal/auth/handler.go` (RegisterResponseData totalXp/level + populate)
- `server/internal/auth/handler_test.go` (mock AddXP)
- `server/internal/chat/handler_test.go` (stub AddXP)
- `server/internal/lobby/lobby_test.go` (fake AddXP)
- `server/internal/ws/events.go` (EventXPAwarded + XPAwardedPayload)
- `server/internal/match/live_match.go` (XPAwarder interface + field + SetXPAwarder; awardXP wired into handleMatchEnd + ordering)
- `server/internal/match/reconnect.go` (awardXP wired into abandonment finalize + ordering)
- `server/internal/match/export_test.go` (AbandonSeatForTest hook)
- `server/cmd/api/main.go` (SetXPAwarder wiring)

**Client — created**

- `client/src/shared/lib/xpLevel.ts`
- `client/src/shared/lib/xpLevel.test.ts`
- `client/src/shared/components/XpBar.tsx`
- `client/src/features/profile/components/IdentityHero.test.tsx`

**Client — modified**

- `client/src/shared/types/wsEvents.ts` (EVENT_XP_AWARDED + XpAwardedPayload)
- `client/src/shared/types/apiTypes.ts` (User.totalXp/level)
- `client/src/shared/api/profile.ts` (ProfileResponse XP fields)
- `client/src/shared/api/auth.ts` (RegisterResponse/RefreshResponse totalXp/level)
- `client/src/shared/api/axiosClient.ts` (refresh re-hydrate totalXp/level)
- `client/src/shared/hooks/useAuth.ts` (bootstrap setUser totalXp/level)
- `client/src/shared/hooks/mutations/useAuth.ts` (login/register setAuthState totalXp/level)
- `client/src/shared/hooks/useWsDispatch.ts` (EVENT_XP_AWARDED handler)
- `client/src/shared/hooks/useWsDispatch.test.ts` (XP dispatch tests + fixture)
- `client/src/shared/stores/matchStore.ts` (xpAward state + setter)
- `client/src/shared/components/TopBar.tsx` (level + XP bar)
- `client/src/shared/components/TopBar.test.tsx` (XP tests + fixture)
- `client/src/features/profile/components/IdentityHero.tsx` (level pill + XP bar)
- `client/src/features/profile/ProfilePage.tsx` (thread level/progress props)
- `client/src/features/profile/ProfilePage.test.tsx` (fixtures)
- `client/src/features/match/components/MatchResult.tsx` (XP flourish)
- `client/src/features/match/components/MatchResult.test.tsx` (XP tests)
- `client/src/features/match/MatchPage.tsx` (pass xpAward to MatchResult)
- `client/src/shared/i18n/en.json`, `sr.json`, `mk.json`, `hr.json` (xp block)
- `client/src/features/auth/LoginPage.test.tsx`, `RegisterPage.test.tsx` (auth-response fixtures)
- `client/src/features/room/RoomPage.test.tsx`, `RoomPage.locale.test.tsx`, `RoomPage.diamond.test.tsx`, `RoomPage.bots.test.tsx`, `CreateRoomModal.test.tsx` (User fixtures)
- `client/src/features/match/MatchPage.test.tsx` (User fixtures)
- `client/src/features/lobby/MatchmakingPage.test.tsx`, `LobbyPage.test.tsx` (User fixtures)

### Change Log

| Date       | Change                                                                                          |
| ---------- | ----------------------------------------------------------------------------------------------- |
| 2026-06-22 | Implemented Story 9.5 (XP & lifetime level): migration 000011, derived level curve, atomic XP award at match end (normal + abandonment team-forfeit), `event:xp_awarded` on both WS contract files, profile + top-nav level/XP bar + MatchResult flourish, i18n ×4. Status → review. |
| 2026-06-22 | Code review (3-layer adversarial): 4 patches applied — WS drift-gate coverage for `event:xp_awarded` (Go golden + Zod schema + conformance witness + Go/TS contract rows), Prettier fix in LoginPage.test.tsx, match_abandoned store reset of xpAward/coinSettlement, profile XP-bar fallback via client curve. 2 deferred, 8 dismissed. Full client vitest (885) + go test (match/ws) + tsc + eslint + prettier green. Status → done. |

## Review Findings

> 3-layer adversarial code review (Blind Hunter + Edge Case Hunter + Acceptance Auditor), 2026-06-22. All eight ACs and design decisions D1–D6 verified satisfied in code (incl. the PO-override abandonment team-forfeit AC3 and the no-import-cycle XPAwarder, D1). Outcome: 0 decision-needed, 5 patch, 1 defer, 8 dismissed as noise/false-positive/by-design. (The `total_xp` width item was promoted from deferred to a fixed patch — migration 000011 is unshipped.)

**Patch (fixable, unambiguous):**

- [x] [Review][Patch] `event:xp_awarded` missing from the cross-language WS contract drift-gate — `coin_settlement` (the template event) has a Go golden test case, a `testdata/events/*.json` golden, a Zod schema, and a TS contract row; `xp_awarded` has none, so the new event has no protection against `events.go`↔`wsEvents.ts` field-name drift (the project's #1 WS rule). MEDIUM. [server/internal/ws/events_contract_test.go, server/internal/ws/testdata/events/, client/src/shared/types/wsEvents.schemas.ts, client/src/shared/types/wsEvents.contract.test.ts]
- [x] [Review][Patch] Prettier violation introduced — `createdAt:"..."` lost its space after the colon (HEAD had it); `prettier --check` fails on this file → `make lint`/CI blocks merge, contradicting the AC8 "Prettier clean on changed files" claim. Fix: `npx prettier --write`. LOW (merge-blocking). [client/src/features/auth/LoginPage.test.tsx:106,186]
- [x] [Review][Patch] `EVENT_MATCH_ABANDONED` handler does not clear stale `xpAward`/`coinSettlement` — the `EVENT_MATCH_END` handler resets both before per-human events arrive; the abandoned handler does not. Abandoning-team players (who by design receive no follow-up `xp_awarded`) could retain stale store state; masked today by `matchStore.reset()` on navigation. Fix: mirror match_end. LOW. [client/src/shared/hooks/useWsDispatch.ts:447]
- [x] [Review][Patch] Profile-loading transient shows a non-zero level with an empty "0 / 0 XP" bar — `level` falls back to `user.level` while `xpIntoLevel`/`xpForNextLevel` fall back to `0`, so the bar reads empty/contradictory until the profile query resolves. Fix: fall back to `xpBarFill(user.totalXp, user.level)` (client curve) or render the bar only once profile data is present. LOW. [client/src/features/profile/ProfilePage.tsx:62-64]
- [x] [Review][Patch] `total_xp` column widened from `INTEGER` to `BIGINT` to match the 64-bit Go `int` accumulator — promoted from deferred and fixed here since migration 000011 is unshipped (an `INTEGER` capped at ~2.1B would silently fail the additive `UPDATE` for a long-lived player; `wallet_balance` stays `INTEGER` as a bounded balance). Migration 000011 down/up roundtrip re-verified on the dev DB; user-pkg DB tests green. LOW. [server/migrations/000011_add_xp_to_users.up.sql, server/internal/user/model.go]

**Deferred (real, latent — not actionable now):**

- [x] [Review][Defer] `abandonPartialXPFactor` float truncation is untested for non-1.0 values [server/internal/match/xp_award.go:61] — deferred, latent (factor is 1.0 today; only matters when tuned)

**Dismissed (8):** server suppresses 0-delta `xp_awarded` while client has defensive 0-handling + test — behavior correct, completed matches always score ≥10/team, abandoning-team forfeit is by-design invisible; profile React-Query cache not invalidated on award — mirrors the accepted `coin_settlement` precedent, not introduced here; `LeveledUp` reconstructs prior level via `newTotal − earned` — correct given the repo's `new = old + delta` contract; `xpBarFill` clamps and could mask curve drift — explicitly by-design (D6 manual-sync, server-authoritative level); migration `down.sql` `DROP COLUMN` destroys XP on rollback — standard reversible down-migration required by AC6; `RegisterResponseData`/`ProfileResponse` gofmt field-alignment churn — cosmetic, JSON order-independent; no guard against duplicate user IDs in the awards map — defends an impossible state (one user, one seat); `mockUserRepo.AddXP` range-copy mutation — false positive, `m.users` is `[]*user.User` (pointer slice, mutates in place).
