---
baseline_commit: 2169392d1ee9467b796caf7e17b0e1320e02e864
---

# Story 13.1: Season Points (SP) & Tier Climb

Status: done

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

## Story

As a player,
I want to earn Season Points for every match I play and watch my rank tier advance,
so that I have an active competitive goal distinct from lifetime level.

## Scope guardrail (read first)

This story delivers **SP accrual + a derived 8-tier ladder + the lobby RankBanner + the season schema**. It is the FIRST story of Epic 13 and it must be shippable on its own.

**Do NOT build:**

- The **leaderboard** (list, endpoint, nav tab, lobby panel) — that is **Story 13.2**.
- The **rollover scheduler**, the prior-season **profile archive**, or any cron/job infrastructure — that is **Story 13.3**. This story ships no scheduler. It makes 13.3 an optimisation, not a prerequisite, via the lazy season resolver in [Design Decision D1](#design-decisions).
- **ELO / hidden MMR / placement matches / LP / sub-ranks (Silver II) / rank-reveal modal / SP decay.** The PRD's older journey prose (`prd.md:225`) and the product brief (`product-brief-beljot-2026-02-21.md:233`) describe an ELO + placement-match + I/II/III sub-rank model. **That model is dead.** The canonical spec is the epic AC (SP formula, flat 8 tiers, no placements) as reset by `sprint-change-proposal-2026-04-18.md:130`. See [Design Decision D6](#design-decisions).
- Any **gating** on tier or SP. Nothing unlocks. Same rule Story 9.5 held for level.

## Acceptance Criteria

**AC1 — SP is calculated and accrued at match end**

**Given** a match completes
**When** SP is calculated for each player
**Then** the formula is `SP_earned = 50 (completion) + (100 if team won else 0) + floor(team_game_points / 10) + (50 if Capot or instant-win occurred)`
**And** abandoning players earn 0 SP
**And** SP is accumulated into the player's current season record

**AC2 — Tier advances and is announced**

**Given** a player's cumulative SP crosses a tier threshold
**When** the next match ends
**Then** their tier updates and a tier-up toast is shown
**And** the 8 tiers and thresholds are: Iron (0), Bronze (500), Silver (1 500), Gold (3 000), Platinum (5 500), Diamond (8 500), Immortal (12 500), Radiant (18 000)

**AC3 — RankBanner renders in the lobby**

**Given** a player views their rank
**When** the RankBanner renders in the lobby
**Then** it shows: tier badge (tier-specific colour + glow), tier name, current SP, progress bar to next tier, days remaining in season

**AC4 — Season schema exists**

**Given** the season schema migration
**When** I inspect the database
**Then** a `seasons` table exists with: `id`, `name`, `started_at`, `ends_at`
**And** a `player_seasons` table exists with: `id`, `user_id`, `season_id`, `sp`, `rank_tier`, `games_played`, `games_completed`

## Design Decisions

These resolve ambiguities in the epic AC. Each states the chosen default and why. **Implement the default.** The two marked **CONFIRM** diverge from a sibling system the dev will otherwise copy by reflex — flag them in the completion notes so the PO ratifies them at review, but do not block on an answer.

### D1 — Where the current season comes from

No `seasons` row exists today, and the rollover job is Story 13.3. If "current season" is a plain `SELECT ... WHERE started_at <= now < ends_at`, this story ships a system that silently stops accruing SP the day the first season ends — before 13.3 exists to extend it.

**Decision:** the season resolver is **lazily self-healing**. `SeasonService.CurrentSeason(now)` reads the row covering `now`; on miss it computes the calendar quarter containing `now` and inserts it idempotently (`INSERT ... ON CONFLICT (started_at) DO NOTHING`, then re-read). The migration also seeds the quarter containing the deploy so the table is never empty.

Quarters are calendar quarters in UTC: Q1 = Jan 1–Apr 1, Q2 = Apr 1–Jul 1, Q3 = Jul 1–Oct 1, Q4 = Oct 1–Jan 1 (`started_at` inclusive, `ends_at` exclusive — no gap, no overlap). `name` is `"YYYY QN"` (e.g. `"2026 Q3"`) — a machine-stable token, **not** a display string; the client renders it verbatim as an identifier and never translates it.

Story 13.3 then owns *scheduled* rollover (creating the next window ahead of time, and whatever archive/notification work it needs), not correctness of the current window.

### D2 — The Capot / instant-win +50 is MATCH-level, not team-level

The epic formula scopes its other terms explicitly (`if team won`, `team_game_points`) and pointedly does **not** scope this one. Read it literally: **if a Capot or an instant-win occurred anywhere in the match, all four human seats get +50** — winners and losers alike.

This is consistent with the established progression philosophy: `xp_award.go` states "XP is a participation reward, not zero-sum" and awards the losing team too. A spectacular match rewards the table.

Do NOT restrict the bonus to the team that made the Capot. It is +50 once, not +50 per Capot hand — a match with two Capot hands still yields +50.

### D3 — Instant-win needs a NEW server-only flag; do not infer it

`checkInstantWin` (`scoring.go:409`) sets `WinnerTeam` + `Phase = PhaseMatchEnd` and **leaves no trace**. It fires right after a deal, so an instant-win match can have zero `handResults` and `TeamScores` of `[0,0]` — but it can equally fire on hand 5 of a match at 500:300. There is no signal at the match layer.

**Follow the `StoppedAtTarget` precedent exactly** (`state.go:206-219` — a field that exists for this identical reason, verbatim: "so the match layer can state the outcome instead of inferring it"):

- Add `WonByInstantWin bool` with tag `json:"-"` to `GameState`, in the same block as `StoppedAtTarget`, with a comment that mirrors its rationale.
- Set it `true` at **both** `checkInstantWin` call sites: `handlePickTrump` (`bidding.go:195`) and `startNewHand` (`scoring.go:398`).
- Clear it in `startNewHand` **next to the existing `state.StoppedAtTarget = false`** so it can never outlive its hand.
- `json:"-"` is deliberate: the client learns nothing new here, and keeping it off the wire leaves the `match_state` contract and its golden untouched.

**Do NOT** add an `OutcomeReasonInstantWin`. Widening `MatchEndPayload`'s enum is a client-contract change this story's ACs do not ask for, and `OutcomeReason` is a raw `string` on the Go side against a strict Zod literal on the client (deferred item **D116**) — adding a value there breaks stale tabs.

### D4 — Capot is read from the buffered hand results, and the copy must be HOISTED

`session.handResults` (each row carries `Capot bool`) is the source. `capotOccurred = any(hr.Capot)`.

**The trap:** in `handleMatchEnd` the `handResults` copy is taken under `session.mu.RLock()` at `live_match.go:1400-1403`, which is **after** the settlement/XP/honor block at `1343-1366` where SP must slot in. Reading `session.handResults` unlocked at line ~1367 is a data race that `-race` will not necessarily catch (`make test` does not pass `-race`).

**Do this:** hoist the existing RLock'd `handsCopy` block from `1400-1403` to **above** the `settleMatch` call at `1343`, and pass `capotOccurred` down. One copy, used by both SP and `CreateWithHands`. Do not add a second lock acquisition.

On the abandonment path `handsCopy` is already snapshotted under the session lock at `reconnect.go:646-647`, **before** the unlock at `:663` — derive `capotOccurred` there and pass it through; no hoist needed.

### D5 — CONFIRM: absence forfeits SP per-SEAT, not per-TEAM

The epic says "**abandoning players** earn 0 SP" — plural, player-scoped. Take it literally, and map "abandoning players" onto the presence gate that already exists and has survived two review passes: `computeHonorEvents` (`honor_record.go:67-79`) charges the seat whose reconnect window expired **plus any other seat absent at the terminal end**.

**Decision:** every human seat **present** at the terminal end earns the full formula. Every **absent** seat earns 0. The abandoner's teammate, if present, **still earns**.

**This deliberately differs from XP and coins**, which forfeit team-wide — and the dev will be copying `xp_award.go`, so this is the single easiest thing to get wrong:

| System | Abandonment forfeit |
| --- | --- |
| Coins (9.2) | Whole abandoning **team** |
| XP (9.5) | Whole abandoning **team** (PO override 2026-06-22, overrode that epic's AC) |
| Honor (9.7) | **Per-seat** presence |
| **SP (13.1)** | **Per-seat** presence — reuses honor's gate |

Rationale: SP is a *competitive ladder*. Zeroing a present player's ranked progress because their partner's network dropped is a harsher punishment than the XP case, and the epic AC does not ask for it. Note the presence gate **fails open** (`true` does not reliably mean present — see the comment at `reconnect.go:627-637`), so it under-charges rather than over-charges. That is the correct direction for a ladder.

Flag at review for PO ratification.

### D6 — The UX RankBanner spec is stale; SP replaces LP/placement

`ux-design-specification.md:753-758` specifies RankBanner states as `unranked` / `placement ("Placement: X/3")` / `ranked (LP + progress)`. That is the retired ELO model. `sprint-change-proposal-2026-04-18.md:51` explicitly lists "seasonal RankBanner in SP mode" as an outstanding UX update that was never made.

**The epic AC3 is canonical.** Build exactly the five elements it names: tier badge (colour + glow), tier name, current SP, progress bar to next tier, days remaining in season. There is no unranked state and no placement state — a player at 0 SP is **Iron**, which is a real tier and renders normally.

This is the same resolution Story 9.5 recorded as its D2 (`9-5-xp-and-level-system.md:231`), which called RankBanner "the Epic-13 surface". This story is that surface.

### D7 — `rank_tier` is stored but NEVER authoritative

Mirrors the `honor_score` column precedent (`000017_add_honor_to_users.up.sql`, the "DENORMALIZED SNAPSHOT" header): the column exists **only** so operators and Story 13.2's leaderboard query can sort/filter in SQL. The authoritative tier is always `season.TierForSP(sp)` — pure arithmetic over a value you already loaded.

Unlike honor, SP has **no decay** (PRD: "No decay") and SP is monotonic, so stored and derived can never actually disagree. Keep the derived call as the single source anyway, so no future reader learns the wrong habit. Refresh the column on every SP write.

### D8 — Package placement and the import-direction rule

New package `server/internal/season/`. The match manager declares its own narrow `SPAwarder` interface **in the match package** and receives an implementation via `SetSPAwarder` in `main.go`.

This is not ceremony — it is the rule that `xp_award.go` and `honor_record.go` both call out in capitals: **`user` imports `match`, so `match` must never import `user`** (Story 9.5 D1 / 9.7 D4). `season` will need to grow toward match/user territory later (13.2's leaderboard joins users; 13.3's archive reads seasons per player). Declaring the interface in `match` keeps that door open. Copy the `HonorRecorder` shape: the interface returns a **precomputed snapshot** so the manager never runs tier math it cannot see.

Nil-tolerant like all three siblings: `m.spAwarder == nil` → SP is skipped entirely (no mutation, no event). Match end must never break because SP is unwired.

### D9 — RankBanner data arrives by dedicated endpoint + WS invalidation

XP and honor ride the **auth envelope** (`authResponseData`, `auth/handler.go:174`) into `authStore.user`. Do **not** extend that path here: `authResponseData` takes only a `*user.User` and would force a season dependency into `AuthHandler` and every one of its call sites, for data only one surface reads.

**Instead:** `GET /api/v1/seasons/current` returns the season window plus the viewer's own record, consumed by `useCurrentSeasonQuery` (model it on `useLobbyStats.ts`). The `event:season_points_awarded` dispatch handler then calls `queryClient.invalidateQueries({ queryKey: queryKeys.season.current() })` — the established WS→query bridge (`useWsDispatch.ts:610` does exactly this for friends). Story 13.2's leaderboard endpoint sits beside it.

**No `refetchInterval`.** The banner is push-invalidated; polling it would be waste. (`useLobbyStats` polls because its counts have no push path — that reason does not apply here.)

### D10 — `games_played` vs `games_completed`

- `games_played` — **+1 for every human seat in the match**, on both finalizers, present or not.
- `games_completed` — **+1 only for seats present at the terminal end** (the same gate as SP eligibility — see D5 above).

So `games_completed` is exactly "matches where this player earned SP", and `games_played - games_completed` is their in-season absence count. 13.2's leaderboard renders `games_played`.

Bot seats and empty seats (`playerIDs[seat] == 0`) increment neither. Use the exact guard from `settlement.go` / `xp_award.go`: `if botSeats[seat] || playerIDs[seat] == 0 { continue }`.

## Tasks / Subtasks

- [x] **Task 1 — Migration `000024`: seasons + player_seasons** (AC: 4)
  - [x] `server/migrations/000024_create_seasons_and_player_seasons.up.sql` + `.down.sql`. **Verify `000023` is still the highest** before naming; never skip a number.
  - [x] `seasons`: `id SERIAL PK`, `name VARCHAR(32) NOT NULL`, `started_at TIMESTAMPTZ NOT NULL`, `ends_at TIMESTAMPTZ NOT NULL`, `CHECK (ends_at > started_at)`, `UNIQUE (started_at)` (the conflict target D1's upsert needs), `created_at`/`updated_at` per GORM.
  - [x] `player_seasons`: `id SERIAL PK`, `user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE`, `season_id INTEGER NOT NULL REFERENCES seasons(id) ON DELETE CASCADE`, `sp BIGINT NOT NULL DEFAULT 0 CHECK (sp >= 0)`, `rank_tier VARCHAR(16) NOT NULL DEFAULT 'iron'`, `games_played INTEGER NOT NULL DEFAULT 0 CHECK (games_played >= 0)`, `games_completed INTEGER NOT NULL DEFAULT 0 CHECK (games_completed >= 0)`, `created_at`/`updated_at`.
  - [x] `CREATE UNIQUE INDEX idx_player_seasons_user_season ON player_seasons (user_id, season_id);` — the upsert conflict target, and the atomic backstop against two concurrent match-end writes both inserting.
  - [x] `CREATE INDEX idx_player_seasons_season_sp ON player_seasons (season_id, sp DESC);` — Story 13.2's leaderboard read. Added here because the schema AC lives here.
  - [x] `sp` is **BIGINT**: it is an accumulator summed by a 64-bit Go `int`. This is the exact width trap the 9.5 review caught when `total_xp` shipped as `INTEGER` and had to be widened.
  - [x] Seed the quarter containing deploy time (D1). Idempotent (`ON CONFLICT (started_at) DO NOTHING`) so it is safe under a down/up cycle.
  - [x] `.down.sql` drops `player_seasons` then `seasons` (FK order). **No backfill in the up migration** — SP starts at zero for everyone by design (a season is a fresh competitive window; there is no historical SP to reconstruct). Say so in the header, so nobody later "fixes" the omission.

- [x] **Task 2 — `season` package: tier curve** (AC: 2)
  - [x] `server/internal/season/tier.go`. Single source of truth for the ladder. Every function **pure**: no DB, no clock reads (time is always a parameter). Header comment naming `client/src/shared/lib/seasonTier.ts` as the one permitted mirror, per the manual-sync convention.
  - [x] `SeasonTiers` — ordered token list `["iron","bronze","silver","gold","platinum","diamond","immortal","radiant"]`.
  - [x] Thresholds as **named consts or one ordered table**, not scattered literals: 0 / 500 / 1500 / 3000 / 5500 / 8500 / 12500 / 18000. A retune must be a one-place change (the `levelCurveCoefficient` / `honorHalfLifeDays` convention).
  - [x] `TierForSP(sp int) string` — highest tier whose floor `<= sp`. `sp <= 0` → `"iron"`. Integer arithmetic only.
  - [x] `TierProgress(sp int) (tier string, spIntoTier, spForNextTier int)` — mirrors `LevelProgress` (`level.go:44`). At **Radiant** there is no next tier: return `spForNextTier = 0` and document that the client renders a full/terminal bar. `LevelProgress` can rely on a strictly-increasing quadratic; a finite table cannot, so this case is real — cover it in tests.
  - [x] `tier_test.go` — Go **table-driven** (`[]struct{name string; ...}` + `t.Run`). Cases: 0, each threshold exactly, each threshold −1, negative, above 18000, Radiant's `spForNextTier == 0`.

- [x] **Task 3 — `season` package: model + repository** (AC: 1, 4)
  - [x] `model.go` — `Season`, `PlayerSeason`. GORM default naming (`snake_case`, auto-plural). All JSON tags `camelCase`. Every JSON-bound field exported.
  - [x] `repository.go` — the interface. `gorm_repo.go` — the implementation. Handlers/services call the **interface**, never GORM directly.
  - [x] `CurrentSeason(now time.Time) (*Season, error)` — read the covering row; on miss compute the calendar quarter (UTC), `INSERT ... ON CONFLICT (started_at) DO NOTHING`, re-read (D1).
  - [x] `ApplySeasonPoints(seasonID uint, awards map[uint]SPAward) (map[uint]PlayerSeasonSnapshot, error)` — **one transaction** for all four seats. Per user: upsert `player_seasons` on `(user_id, season_id)`, `sp = sp + delta`, `games_played = games_played + 1`, `games_completed = games_completed + N`, `rank_tier = TierForSP(new sp)`. Return post-write snapshots. Mirror the `AddXP` transaction shape (`user/gorm_repo.go:212-256`).
  - [x] Return the **pre-award SP** in the snapshot (or the delta) so the caller can compute `tieredUp` without a second read — the same problem `event:honor_updated` solved by reading `before` off the store, done properly here.
  - [x] `gorm_repo_test.go` — DB tests use a **per-test transaction with rollback** (`tx.Begin()` + `t.Cleanup(Rollback)`). Tests create all their own data; never touch `make seed` data or another test's rows.

- [x] **Task 4 — Engine: instant-win signal** (AC: 1)
  - [x] `game/state.go` — add `WonByInstantWin bool \`json:"-"\`` beside `StoppedAtTarget`, with the mirrored rationale comment (D3).
  - [x] `game/bidding.go:195` and `game/scoring.go:398` — set it `true` at both `checkInstantWin` success branches.
  - [x] `game/scoring.go` — clear it in `startNewHand` next to `state.StoppedAtTarget = false`.
  - [x] Engine tests through **`ApplyAction` only**. Build states with `internal/game/testfixtures/` factories — **never** a raw `GameState{}` literal, even if it compiles. If no factory fits, add one.
  - [x] Do **not** touch `RefreshDerivedFlags`. `StoppedAtTarget` is recomputed from `Rules`; `WonByInstantWin` is an **event record**, not a config mirror, and must not be recomputed at `ApplyAction`'s exit.

- [x] **Task 5 — WS contract: `event:season_points_awarded`** (AC: 2)
  - [x] `ws/events.go` — `EventSeasonPointsAwarded = "event:season_points_awarded"` + `SeasonPointsAwardedPayload`: `spEarned`, `newSeasonSp`, `rankTier` (string token), `tieredUp` (bool), `seasonName`. Document the ordering slot and why it is a new event type rather than fields on an existing one (strict Zod on the client breaks stale tabs when a payload widens — see the `EventHonorUpdated` header).
  - [x] `rankTier` crosses the wire as a **stable machine token**, never a display string. Non-negotiable: `HonorUpdatedPayload.HonorTier`'s comment states the rule.
  - [x] **Walk all six drift-gate touchpoints — a partial pass fails CI:**
    1. `server/internal/ws/events.go` — const + struct
    2. `server/internal/ws/events_contract_test.go` — new case in the table (`goldenFile: "season_points_awarded.json"`)
    3. `server/internal/ws/testdata/events/season_points_awarded.json` — generate with `UPDATE_GOLDENS=1 go test ./internal/ws/`
    4. `client/src/shared/types/wsEvents.ts` — `EVENT_SEASON_POINTS_AWARDED` const + `SeasonPointsAwardedPayload` interface
    5. `client/src/shared/types/wsEvents.schemas.ts` — `z.strictObject` schema + the type-equality assertion
    6. `client/src/shared/types/wsEvents.contract.test.ts` — golden import + table row

- [x] **Task 6 — Match wiring: award SP at both finalizers** (AC: 1, 2)
  - [x] `match/live_match.go` — `SPAwarder` interface + `spAwarder` field + `SetSPAwarder`, modelled on `HonorRecorder` (D8). Declare the `SPAward` / `PlayerSeasonSnapshot` DTOs **in the match package** so a `season.Service` satisfies the interface without `match` importing `season`.
  - [x] `match/sp_award.go` — `computeSPAwards(...) [4]int` (pure, table-tested) + `(m *Manager) awardSeasonPoints(...) []spAwardMsg`. Copy `xp_award.go`'s structure and its best-effort degradation: an error is `slog.Error`'d and the events are skipped, but the caller **still** fires `match_end` / `match_abandoned` and `match_state`. Clients must never be stranded on the table.
  - [x] `handleMatchEnd` (`live_match.go:1338+`): **hoist the `handsCopy` RLock block from `:1401-1404` to above `:1343`** (D4), derive `capotOccurred`, then insert the SP call after `recordHonor` at `:1366`. `abandonedSeat = -1`, all four `connected` true by construction on this path.
  - [x] `handleReconnectTimeout` (`reconnect.go:575+`): insert after `recordHonor` at `:695`. Pass the real `abandonedSeat`, the lock-snapshotted `connected` array, the snapshotted `[2]int{teamAScore, teamBScore}`, and `capotOccurred` from the already-copied `handsCopy`.
  - [x] Send `spAwardMsgs` **after** the `honorMsgs` loop and **before** the trailing state broadcast, on both paths. The extended contract:

    ```
    match_end | match_abandoned
      -> coin_settlement
      -> xp_awarded
      -> honor_updated
      -> season_points_awarded   <- NEW
      -> match_state
    ```

    This does not break `honor_wiring_test.go`, whose assertion is `LAST(honor) < match_state` — still true. Verify, don't assume.
  - [x] The win bonus uses the **same `winnerTeam` the finalizer already computed**: `*finalState.WinnerTeam` on the natural path, `1 - game.TeamForSeat(abandonedSeat)` on the abandonment path. Do not re-derive it from scores — surrender and stop-at-target both route through the natural path with the winner already resolved.
  - [x] On an instant-win, `TeamScores` may be `[0,0]` and `handResults` empty. `floor(0/10) == 0` is a **legitimate** term, not a bug. Winners get `50 + 100 + 0 + 50 = 200`; losers `50 + 0 + 0 + 50 = 100`.
  - [x] `sp_award_test.go` — table-driven over `computeSPAwards`: normal win/loss; Capot; instant-win; both; bot seats; empty seats; abandonment (absent seat 0, present teammate earns — D5); multiple absent seats.
  - [x] `sp_wiring_test.go` — model on `honor_wiring_test.go`. Stub awarder; assert the ordering slot with the existing `firstIndexOfType` / `lastIndexOfType` helpers; assert nil-awarder is a clean no-op; assert an awarder error still lets `match_end` and `match_state` through.

- [x] **Task 7 — HTTP: `GET /api/v1/seasons/current`** (AC: 3)
  - [x] `season/handler.go`. Response `{ "data": { ... } }` (never a bare object): `seasonName`, `endsAt` (**ISO 8601 absolute timestamp** — never a relative "days remaining"; the client computes the countdown), `sp`, `rankTier`, `spIntoTier`, `spForNextTier`, `gamesPlayed`, `gamesCompleted`.
  - [x] A player with no `player_seasons` row yet returns the **zero state** (`sp: 0`, `rankTier: "iron"`), not 404 and not a lazily-created row. Reads must not write.
  - [x] Register in `main.go` beside the other authed `api.GET` routes (`main.go:154-371`). Auth-gated — it returns the caller's own record, keyed off the JWT subject, never a path/query id.
  - [x] `handler_test.go` — the zero state, a populated state, and a mid-tier progress decomposition.

- [x] **Task 8 — Wire the service in `main.go`** (AC: 1, 3)
  - [x] `seasonRepo := season.NewGormRepository(db)`; `seasonService := season.NewService(seasonRepo)`; `sessionManager.SetSPAwarder(seasonService)`; season handler for Task 7. Follow the `honorService` comment block (`main.go:207-216`) — one instance, several narrow consumers.
  - [x] Middleware order in `main.go` is load-bearing (**CORS → Logging → Error Handler → Auth**). Adding routes does not change it; do not reorder anything.

- [x] **Task 9 — Client: tier mirror + API client** (AC: 2, 3)
  - [x] `client/src/shared/lib/seasonTier.ts` — the **only** client copy of the ladder. Header must state it mirrors `server/internal/season/tier.go` and that the server is authoritative for tier. Export: `SEASON_TIERS`, threshold table, `seasonTierForSp`, `normalizeSeasonTier(tier, sp)` (version-skew guard — an unrecognised token falls back to the SP's own bucket, which is why the Zod schema types `rankTier` as a plain `string`), `seasonBarFill`, and `SEASON_TIER_COLOR` / `SEASON_TIER_SOFT` / `SEASON_TIER_LINE` maps. Model on `honor.ts` — read its header on why the colour map must be centralised (`HonorPanel` and `TopBar` had already drifted on `fair` before it was).
  - [x] `seasonTier.test.ts` — thresholds, boundaries, Radiant terminal case, unknown-token fallback.
  - [x] `shared/api/season.ts` + `shared/hooks/queries/useCurrentSeason.ts` + `queryKeys.season.current()`. API client files map 1:1 to backend domain packages.
  - [x] Add the season response type to `shared/types/apiTypes.ts`.
  - [x] **Eight tier colours do not exist in `index.css`.** The honour ramp (`--h1`…`--h5`, `index.css:197-217` light / `:412-428` dark) is five tokens and is honour's. Declare a new `--rt1`…`--rt8` ramp (+ `-soft` / `-line`) in **both** the light `:root` block and the dark block, following the honour ramp's exact structure. Tailwind v4 is **CSS-first** — `@theme` directives in `index.css`, and there is no `tailwind.config.js` to touch.

- [x] **Task 10 — Client: RankBanner + tier-up toast** (AC: 2, 3)
  - [x] `client/src/features/lobby/components/RankBanner.tsx` + `.test.tsx`. Named export, filename matches component, no default export.
  - [x] Renders exactly AC3's five elements. Reuse `XpBar` (`shared/components/XpBar.tsx`) for the progress bar if its accent fill can be re-coloured per tier; otherwise write a sibling in the same shape (`role="progressbar"` + `aria-valuemin/max/now` + `aria-label`). **Do not** add a shadcn `progress` primitive — `ui/` has none, and primitives are only ever added via `npx shadcn@latest add`, never hand-written.
  - [x] Days remaining is derived from `endsAt` client-side. Use the existing `relativeTime.ts` / `timeTick.ts` helpers rather than a new interval.
  - [x] Mount in `LobbyPage.tsx` between `<HeroBlock>` and `<FriendList>` (`LobbyPage.tsx:213-226`). **Note:** the real lobby is a single-column stack (`HeroBlock` → `FriendList` → `FilterRail` → `RoomGrid`) — it has no right panel, despite `architecture.md:640` listing a flat `features/lobby/RankBanner.tsx` and 13.2's AC naming a "right panel". Real components live under `features/lobby/components/`; follow the real tree.
  - [x] `useWsDispatch.ts` — handle `EVENT_SEASON_POINTS_AWARDED`. Type-guard every field with `Number.isInteger` / `typeof`, **never truthiness**: Go zero values are real values, and `spEarned: 0` and `tieredUp: false` are both legitimate and both falsy. Then `queryClient.invalidateQueries({ queryKey: queryKeys.season.current() })`.
  - [x] Tier-up toast on `tieredUp` (AC2). The AC says **toast**, not dialog — use `sonner` (`toast` is already imported in `LobbyPage.tsx:4`). Do **not** copy `levelUpStore` + `LevelUpDialog`: that store exists because a *dialog* must survive the navigation that wipes `gameStore`, and a toast has no such need. Fire it from the dispatch handler so it lands whether the player is mid-navigation or already in the lobby.
  - [x] `.test.tsx` selects via `data-testid`, never CSS classes or DOM structure. Test names present tense: `it("renders the tier badge")`.

- [x] **Task 11 — i18n across all four locales** (AC: 2, 3)
  - [x] New `season` namespace in **`en.json`, `sr.json`, `hr.json`, `mk.json`** — `i18n.parity.test.ts` gates key parity; a missing key in one locale fails the suite.
  - [x] Keys follow `{feature}.{component}.{element}`: the eight tier names, `season.banner.*` (sp / progress label / days remaining / a11y label), `season.tierUp.*`.
  - [x] **Never use an em dash (—) in `mk`, `sr`, or `hr`.** English only.
  - [x] One word per concept per locale. **Never transliterate `mk` from `sr`/`hr`** — they are different languages. If a term already exists in the file, reuse it verbatim rather than coining a synonym.
  - [x] `seasonName` (`"2026 Q3"`) is **not** translated — it is an identifier rendered verbatim.
  - [x] Edit the locale JSONs with the file tools, not shell heredocs: this console is cp1251 and printing Cyrillic or diacritics can crash mid-edit and leave a multi-file change half-applied.

- [x] **Task 12 — Gates** (AC: all)
  - [x] `make lint` clean — TS side is `tsc --noEmit` (**tests included**) + ESLint + Prettier `--check`; Go side is `golangci-lint run ./...` (plus `gofmt`). Import order: Go = stdlib / third-party / internal; TS = React / third-party / `shared/` / relative, blank line between groups.
  - [x] `make test` clean — `go test ./...` **and** `npx vitest run`.
  - [x] Re-verify the WS golden pair: `go test ./internal/ws/` then `npx vitest run wsEvents.contract`.
  - [x] Feature-Complete checklist (`project-context.md`): handler + repository + tests / domain errors in `apperr` if any new cases / WS event in **both** contract files / component + co-located test / API client in `shared/api/` / i18n in **all four** locales / lint / all existing tests pass.

## Dev Notes

### Read these files before writing code

`xp_award.go` is the closest analogue to what you are building; `honor_record.go` is the closest analogue to how you wire it. Read both in full.

| File | Why | What must not break |
| --- | --- | --- |
| `server/internal/match/xp_award.go` | **The structural template.** Pure `compute*` + `Manager` method returning prepared per-user messages; bot/empty-seat guard; best-effort error degradation. | — (reference only) |
| `server/internal/match/honor_record.go:67-79` | The presence gate D5 reuses. | Do not modify `computeHonorEvents`. Read the same `connected` array. |
| `server/internal/match/live_match.go:1338-1435` | Natural-end finalizer. SP slots after `recordHonor`. | Persist-then-broadcast order (8.5-1 AC4); `match_end` fires even if persistence failed; `match_state` last. |
| `server/internal/match/reconnect.go:575-700` | Abandonment finalizer. **Broadcasts BEFORE it persists** — the inverse of the natural path. Never read the match row back here; it does not exist yet. | The under-lock snapshot block; the `connected` fail-open semantics. |
| `server/internal/game/state.go:206-219` | `StoppedAtTarget` — the precedent D3 copies verbatim. | Its own behaviour. |
| `server/internal/game/scoring.go:332-406` | `startNewHand` — clears per-hand flags, then calls `checkInstantWin`. | Existing resets; hand/dealer rotation. |
| `server/internal/user/level.go` | Threshold-curve + `*Progress` decomposition shape. | — (reference only) |
| `server/internal/user/gorm_repo.go:212-256` | `AddXP`: multi-user atomic accumulate in one transaction. | — (reference only) |
| `client/src/shared/lib/honor.ts` | Tier-token mirror + colour-map centralisation + version-skew normaliser. | — (reference only) |
| `client/src/shared/hooks/useWsDispatch.ts:337-368` | The `event:xp_awarded` handler — type-guard discipline. | Every existing branch. Add a new `if (type === ...)` block; touch nothing else. |
| `client/src/features/lobby/LobbyPage.tsx:213-226` | Real lobby composition. | Existing order and the modals mounted at the end. |

### Non-negotiables from `project-context.md`

- **Server-authoritative.** Tier and SP are decided server-side. The client mirror is display math and never makes a decision. This is NFR8 territory.
- **Rules engine stays pure.** `ApplyAction(state, action) -> (state, error)`, zero side effects. Task 4 sets a flag on the state the engine already owns — it must not reach for a clock, a DB, or a service.
- **Clone before mutation** in the engine — `slices.Clone()` / `copy()`. Go slices share backing arrays.
- **`ws/router.go` does type-based dispatch only.** No SP logic there. Side effects live in the session manager.
- **Multi-event sequences are separate ordered messages**, never batched. Frontend animation depends on ordering.
- **Absolute timestamps, never relative durations** on the wire. `endsAt`, not `daysRemaining`.
- **`*time.Time` for optional timestamps** — `time.Time`'s zero value serialises as `"0001-01-01T00:00:00Z"`, not `null`.
- **Wrap errors with `%w`**, never `%v`. Control flow via `errors.Is` against `internal/apperr`.
- **No `os.Getenv` outside `config`.** Nothing here needs config.
- **GORM soft deletes are automatic.** If raw SQL is unavoidable, filter `deleted_at IS NULL`.
- **Union literal types in TS**, not `enum`. `type SeasonTier = "iron" | ...` mirrors the Go string constants without drift.
- **Named exports only.** No `export default`.
- **Components never call `fetch()`** — always through `shared/api/` via `axiosClient`/`fetchClient`.
- **Immutable Zustand updates.** Replace objects; never mutate.
- **Branch on `VariantRules` config fields, never `state.Variant`.** SP is variant-agnostic, so nothing here should read either — if you find yourself needing the variant, stop and re-read the ACs.

### Testing standards

- **Go:** `testing` + `testify`. Tests co-located. Rules-engine tests are **table-driven** and go through `ApplyAction` only, with `testfixtures/` factories exclusively. DB tests use per-test transaction + rollback and create their own data.
- **Vitest:** co-located `.test.tsx`. `data-testid` selectors only. Present-tense test names. Components are presentational — assert rendering given server state, not logic.
- **Session-manager tests** must not regress reconnection coverage. If you touch the finalizers, the existing snapshot-projection expectations (Story 12.10: own hand real, every other seat masked to `[]` with `handCount`, no `deck` key) still hold.
- **Test files ARE typechecked.** `make lint` runs `npx tsc --noEmit` against `tsconfig.json`, whose `include` is `["src"]` — tests included. (The Story 11.5 deferred note claiming test type errors are "structurally invisible" is **stale**; that gap has since been closed.) So a stale `.test.tsx` fixture is a **lint failure**, not a silent pass: if you widen a shared type, fix every fixture that constructs it. `tsconfig.build.json` is the *narrower* one — it excludes tests — so it is not the gate to check against.
- **`make test` does not pass `-race`** (verified: `Makefile:17-19` is `npx vitest run` + `go test ./...`). Do not let that lull you on D4's hoist — reason about the lock; the suite will not catch it for you.

### Project Structure Notes

- **New package** `server/internal/season/` follows the mandated domain shape: `model.go`, `repository.go`, `gorm_repo.go`, `handler.go`, `service.go`, `{domain}_test.go`. `internal/wallet/` and `internal/friend/` are the two closest existing examples.
- **Variance from `architecture.md`:** the doc maps Progression (FR33–FR40) to `internal/user/` (extended) and a flat `features/lobby/RankBanner.tsx`. Both are stale. Backend goes in its **own** `internal/season/` package (D8's import-direction rule makes cramming it into `user` actively harmful), and the real client tree puts lobby components under `features/lobby/components/`. The doc also lists a `ui/progress.tsx` that does not exist. Follow the real tree; the doc predates Epic 9.
- **Branch:** `feat/E13-S1-season-points-tier-climb`. One story = one branch = one PR. If you find a bug en route, file it in `deferred-work.md` — do not fix it here unless it blocks the story.
- **Commits:** `{type}({scope}): {description}`, under 72 chars, scope = backend package or frontend feature (`feat(season): ...`, `feat(match): ...`).
- **`WS event contract lands in both files in the same commit`** — no exceptions.

### Cross-story context

- **13.2 (Leaderboard)** consumes `player_seasons` ordered by `sp` — Task 1's `idx_player_seasons_season_sp` is for it. It also needs a right-hand lobby panel that does not exist yet; that layout work is 13.2's, not yours.
- **13.3 (Rollover & archive)** needs: a scheduler (none exists anywhere in the codebase — verified), the next-quarter `seasons` row, and a per-player prior-season list. D1's lazy resolver means 13.3 is not a correctness dependency for you. Leave `player_seasons` rows immutable across seasons — the soft reset is "a new `season_id`", never an update or a compression of the old row.
- **Epic 9 systems this sits beside:** wallet/coins (9.2), XP/level (9.5), honor (9.7). All four now fire at match end. SP is the fourth event in that burst; keep it in its slot.
- **Recent branch context:** the last merged work added `Room.StopAtTarget` ("dosta") and `matches.stop_at_target`/`declarations_enabled` rule-flag columns (migrations `000021`–`000023`), plus `OutcomeReasonTargetReached`. A stop-at-target match ends mid-hand through the **natural** path with `WinnerTeam` resolved — so it needs no special handling from you, but it is the reason `StoppedAtTarget` exists as the precedent D3 copies.

### External / version context

No new dependencies. Everything needed is already pinned: Go (exact version in `go.mod`), Echo **v4** (do **not** upgrade to v5 — deferred until Dec 2026+), GORM + PostgreSQL, `nhooyr.io/websocket` (import path is `nhooyr.io/websocket`; the repo lives at `github.com/coder/websocket` after the rebrand — not gorilla), golang-migrate (**CLI only**, via `make migrate`; never embedded as a library), testify, React 19, Vite 6, Tailwind CSS v4 (CSS-first, no config file), Zustand, TanStack Query, react-i18next, Vitest, `sonner` for toasts.

Add no library for the tier ladder, the quarter math, or the countdown. Go's `time` package and the existing `relativeTime.ts` / `timeTick.ts` / `clockSync.ts` helpers cover all three. `npx shadcn@latest add` is the only way a `ui/` primitive may appear.

### Open questions for the PO (do not block on these)

1. **D5** — SP absence forfeit is **per-seat** (honour's gate), diverging from XP's and coins' team-wide forfeit. Reads the epic AC literally; harsher team rule would need an explicit override like 9.5's.
2. **D2** — The Capot / instant-win +50 goes to **all four seats**, not just the team that earned it. Literal reading of the formula, consistent with "XP is a participation reward, not zero-sum".
3. **Season length vs. `name`** — quarters are calendar-aligned UTC with a machine-stable `"YYYY QN"` name. If marketing wants themed season names ("Season 1: Ember"), that is a display-name column on `seasons`, added when it is asked for.

## References

- [Source: _bmad-output/planning-artifacts/epics.md#Epic-13-Seasonal-Rank-Leaderboard] — canonical ACs (lines 2566-2644)
- [Source: _bmad-output/planning-artifacts/epics.md#L62] — FR37, the 8-tier ladder
- [Source: _bmad-output/planning-artifacts/prd.md#L168] — SP formula, quarterly seasons, no decay, soft reset
- [Source: _bmad-output/planning-artifacts/prd.md#L225] — **STALE** ELO/placement/Silver-II journey prose; superseded (D6)
- [Source: _bmad-output/planning-artifacts/sprint-change-proposal-2026-04-18.md#L130] — SP tier thresholds as reset; #L50 flags the un-done architecture pass for `seasons`/`player_seasons`; #L51 flags the un-done "RankBanner in SP mode" UX pass
- [Source: _bmad-output/planning-artifacts/ux-design-specification.md#L753] — **STALE** RankBanner states (unranked/placement/LP); superseded (D6). #L331 — rank badges use tier-specific colours, Radiant = accent + glow
- [Source: _bmad-output/planning-artifacts/architecture.md#L887] — Progression FR33-FR40 mapping (stale, see Project Structure Notes); #L640 — flat RankBanner path (stale); #L199 — data architecture (PostgreSQL/GORM, no caching in Phase 1, design queries caching-ready)
- [Source: _bmad-output/implementation-artifacts/9-5-xp-and-level-system.md#L17] — the scope guardrail that deferred RankBanner/rank/LP/season to Epic 13; #L231 (D2) — epic AC overrides PRD prose, no gating
- [Source: _bmad-output/implementation-artifacts/9-7-honor-score-system.md] — presence gate, tier tokens, stored-vs-authoritative split, client mirror convention
- [Source: _bmad-output/project-context.md] — full rule set: stack versions, naming, testing, anti-patterns, Feature-Complete checklist
- [Source: server/internal/match/xp_award.go] — structural template (D2's participation-reward philosophy, bot/empty guard, best-effort degradation)
- [Source: server/internal/match/honor_record.go#L67] — `computeHonorEvents` presence gate (D5)
- [Source: server/internal/match/live_match.go#L79] — `XPAwarder` / #L119 `HonorRecorder`: the import-direction rule (D8); #L1338 natural finalizer; #L1400-L1403 the `handsCopy` block to hoist (D4)
- [Source: server/internal/match/reconnect.go#L575] — abandonment finalizer, broadcast-before-persist, `connected` fail-open note
- [Source: server/internal/game/state.go#L206] — `StoppedAtTarget`, the D3 precedent
- [Source: server/internal/game/scoring.go#L409] — `checkInstantWin`; #L398 and server/internal/game/bidding.go#L195 — its two call sites
- [Source: server/internal/user/level.go] — threshold curve + `LevelProgress` shape
- [Source: server/internal/user/gorm_repo.go#L212] — `AddXP` transactional multi-user accumulate
- [Source: server/internal/ws/events.go#L63] — `EventXPAwarded`; #L85 `EventHonorUpdated` (the full ordering contract + why a new event type beats a widened payload)
- [Source: server/internal/ws/events_contract_test.go#L18] — `UPDATE_GOLDENS=1` golden workflow
- [Source: server/migrations/000019_create_friendships.up.sql] — table-creating migration template (FK cascade, unique-index-as-atomic-backstop)
- [Source: server/migrations/000017_add_honor_to_users.up.sql] — the "denormalized snapshot may lag, never render or gate on it" header (D7)
- [Source: client/src/shared/lib/honor.ts] — tier mirror, centralised colour map, version-skew normaliser
- [Source: client/src/shared/lib/xpLevel.ts] — the single-client-copy curve convention
- [Source: client/src/shared/hooks/useWsDispatch.ts#L337] — `event:xp_awarded` type-guard discipline; #L610 — the `invalidateQueries` WS-to-query bridge (D9)
- [Source: client/src/shared/hooks/queries/useLobbyStats.ts] — query-hook template
- [Source: client/src/index.css#L197] — honour colour ramp structure to mirror for `--rt1`…`--rt8`

## Dev Agent Record

### Agent Model Used

claude-opus-5[1m]

### Debug Log References

- **CORRECTION (review round).** An earlier draft of this record claimed `make lint`'s Go half was broken repo-wide. **That was wrong.** `golangci-lint` on PATH resolves to `mise/installs/go/1.26.4/bin/golangci-lint.exe` at **v1.64.8**, which reads the v1-format `server/.golangci.yml` fine: `make lint` exits **0** with zero Go findings (re-verified 2026-08-27 after the review fixes). The v2.12.2 binary mise also installs is the one that refuses the config, and it does not win PATH resolution. CI is safe for a structural reason: `ci.yml:52` installs `github.com/golangci/golangci-lint/cmd/golangci-lint@latest`, a **v1-only module path** (v2 lives at `.../golangci-lint/v2/cmd/...`). The two-binary hazard is recorded in `deferred-work.md` as a latent local-PATH exposure, NOT as a broken gate; the config was deliberately left alone.
- Migration verified against the live dev DB (docker `beljot-postgres-1`, port 5433): `24/u` applies, seeds exactly one row (`2026 Q3`, `2026-07-01` → `2026-10-01`), and a full `down 1` → `up` cycle re-seeds exactly one row (the `ON CONFLICT (started_at) DO NOTHING` guard holds). `\d player_seasons` confirms `sp BIGINT`, both CHECKs, both FK cascades, and both indexes.
- The WS golden was generated with `UPDATE_GOLDENS=1 go test ./internal/ws/ -run Contract`; `git status` confirms **only** `season_points_awarded.json` was added and no existing golden changed.

### Completion Notes List

**Two decisions flagged for PO ratification at review** (both implemented as the story's default):

1. **D5 — SP absence forfeits PER-SEAT, not per-team.** Every human seat present at the terminal end earns the full formula; every absent seat earns 0. The abandoner's teammate, if present, still earns. This diverges from coins (9.2) and XP (9.5), which forfeit team-wide, and reuses honor's (9.7) presence gate. `spSeatPresent` in `sp_award.go` expresses the rule; `TestSPSeatPresent_MatchesTheHonorPresenceGate` asserts it agrees with `computeHonorEvents` over the same inputs so the two cannot drift apart silently. A team-wide rule would need an explicit override like 9.5's.
2. **D2 — the Capot / instant-win +50 goes to ALL FOUR seats**, winners and losers alike, once per match rather than once per Capot hand. Literal reading of the formula (which scopes its other terms and pointedly does not scope this one), consistent with "XP is a participation reward, not zero-sum".

**Implementation notes worth carrying forward:**

- **D3's flag was needed exactly as predicted.** `checkInstantWin` leaves no trace at the match layer, so `GameState.WonByInstantWin` (`json:"-"`, set at both call sites, cleared in `startNewHand`) is the only signal. It is an *event record*, not a config mirror, so `RefreshDerivedFlags` was deliberately left alone — `TestWonByInstantWin_SurvivesApplyActionsDerivedFlagRefresh` locks that in, and `TestWonByInstantWin_IsNotSerialised` locks the `match_state` contract staying untouched. No `OutcomeReasonInstantWin` was added.
- **D4's hoist is in.** The `handsCopy` RLock block moved from just above `CreateWithHands` to just above `settleMatch`; one copy now feeds both the SP award and the persist call. Verified no code between the old and new snapshot points can append to `session.handResults`, and that `bufferHandResultIfScored` precedes `handleMatchEnd` on every call site where buffering is not a documented no-op (`live_match.go:650/668`, `:1813/1820`, `:2125/2140`, and the stop-at-target path at `:1760` which nils `LastHandResult`).
- **A zero SP award is NOT a no-op**, which is the one place this diverges structurally from `xp_award.go`. `awardXP` skips zero deltas; `awardSeasonPoints` must not, because `games_played` increments for every human seat while `games_completed` increments only for present ones (D10). Skipping the zero-delta seats would silently lose the in-season absence record.
- **`spForNextTier == 0` at Radiant is load-bearing on both sides.** `TierProgress` returns 0 (a finite table has a top, unlike `LevelProgress`'s quadratic) and `seasonBarFill` renders that as a FULL bar — a naive guard returning 0 would show the top of the ladder as empty. Covered in `tier_test.go` and `seasonTier.test.ts`.
- **The rank ramp is a new `--rt1`…`--rt8` family**, declared in both palette scopes. Note the "dark block" in this project is the `.game-table` felt scope, not a `prefers-color-scheme` block — there is no theme toggle — so the ramp mirrors the honour ramp's exact two-block structure (`:root` + `.game-table`). `--rt8` (Radiant) points at `var(--accent)` per the UX spec's "Radiant = accent + glow", so it re-roots to lime on felt for free.
- **`XpBar` was NOT reused.** It hardcodes `bg-accent` and the rank bar must take the tier's own colour, so `RankBanner` carries a sibling with the identical a11y contract (`role="progressbar"` + `aria-valuemin/max/now` + `aria-label`). No shadcn `progress` primitive was hand-written.
- **i18next pluralization was deliberately avoided** for "days remaining". The parity test asserts identical key SETS across locales, and en/mk (one/other) and sr/hr (one/few/other) have different plural categories — suffixed plural keys would either fail parity or force wrong agreement. `season.banner.daysLeft` therefore uses a single compact `{{days}}` form, which is the convention `wallet.streakTooltip` and `ageCompact` already follow. No em dash appears in `mk`/`sr`/`hr`; `mk` reuses the existing `поени` (matching `xp.progress`) rather than coining a synonym, and `sr`/`hr` keep the `SP` abbreviation the way they keep `XP`.
- Left for 13.2/13.3 as scoped: no leaderboard, no scheduler, no prior-season archive. `idx_player_seasons_season_sp` ships here for 13.2's read; `player_seasons` rows are immutable across seasons so 13.3's archive can read them back unchanged.

**Review round - 11 patch findings applied (P1-P11).** All fixes mutation-verified where behaviour changed.

- **P1 (bug, latent).** The 000024 seed computed `+ INTERVAL '3 months'` and `to_char(..., 'YYYY "Q"Q')` on a **timestamptz**, so both were evaluated in the SESSION TimeZone. Reproduced against the dev DB: under `TimeZone='America/New_York'` the Q4 window came out named `2026 Q3` and ended `2026-12-31T01:00Z` instead of `2027-01-01T00:00Z` (the EDT->EST shift) - a misnamed row plus a one-hour hole the resolver's half-open lookup falls through. Fixed by keeping the quarter start NAIVE for both the arithmetic and the formatting and casting only the two output columns. Verified by extracting the file's verbatim seed expression and running it under `UTC`, `America/New_York` and `Pacific/Kiritimati`: identical name and identical absolute instants in all three.
- **P2.** `resolveSeason` now guards the `(nil, nil)` the `Repository.CurrentSeason` contract forbids but does not enforce, and BOTH public methods route through it, so neither can regain the dereference. A nil there would have panicked inside a match finalizer; it now degrades to `awardSeasonPoints`' best-effort path. Two tests added using `newMockRepo(nil)`.
- **P3.** `awardSeasonPoints` takes `now time.Time`; both finalizers stamp `finalizedAt := time.Now().UTC()` and thread it down, so the `SPAwarder` doc is now a fact rather than a comment. `assertFinalizerStamp` asserts non-zero, `time.UTC`, and within a second of the test clock on both paths - mutation-verified (passing `time.Time{}` fails it).
- **P4 - PARTIAL, AND THE DEVIATION IS DELIBERATE.** The requested test ("reach `PhaseMatchEnd` through `startNewHand` with a stacked deck") **cannot be written, because that call site is unreachable dead code.** `startNewHand` builds and shuffles its own deck via `math/rand/v2` (no seeding, no injection seam), and neither shipped deal shape can satisfy `checkInstantWin`: `DealShapeCandidate` leaves 5-card hands (8 trumps arithmetically impossible) and `DealShapeAllBeforeBidding` sets `TrumpCandidate = nil` with `TrumpSuit` nil, so it returns at the `default` arm with no suit to count. Verified empirically over 6000 deals across both variants: zero hits, max 5 trumps in any hand under the candidate shape, no trump reference at all under the other. **So no test can make that assignment execute, and inverting it will keep passing no matter what is written.** Delivered instead: (a) `TestWonByInstantWin_SetMidMatchWithPointsOnTheBoard` - the mid-match instant win at hand 5 with TeamScores 500:300, which is the real scenario the field exists for (it lives at the **bidding** call site, since every hand passes through bidding) and is exactly the case that pays the +50 SP with no inference available; mutation-verified against the `bidding.go` assignment. (b) `TestWonByInstantWin_StartNewHandSiteCannotFireUnderEitherDealShape` - pins the two structural invariants that make the other site dead, so a future variant dealing eight open cards alongside a trump reference fails there and tells its author the branch just came alive. The `scoring.go` assignment is kept (correct if ever reached, and removing it would be the same latent trap in reverse), with the reachability finding documented at the test.
- **P5.** `TestAbandonment_CapotPaysThePresentSeats` added - the abandonment finalizer derives `spectacularMatch` independently of the natural-end path, so the Capot argument there was previously free to be a hardcoded `false`. Mutation-verified (hardcoding `false` now fails).
- **P6.** Six tests appended to the existing `useWsDispatch.test.ts` (the repo's own dispatch-test harness). Mutation-verified all three named behaviours: dropping `invalidateQueries` fails 2 tests, inverting the `tieredUp` guard fails 4, removing the object guard fails 1.
- **P7.** `TestGetCurrentSeason_WirePayloadKeysAreExact` unmarshals into `map[string]any` and asserts the exact 8-key set plus JSON types and that `endsAt` parses as RFC 3339. Mutation-verified: renaming the `spForNextTier` tag now fails (it previously passed every Go and TS test while silently rendering every player's bar 100% full).
- **P8.** `!payload || typeof payload !== "object"` added as the first condition in the new branch only; sibling handlers left alone as instructed.
- **P9.** (a) The "to next tier" clause dropped from `season.banner.progress` in all four locales, aligning to `xp.progress`'s bare fraction - the visible line was labelling the band WIDTH as the remainder. The aria label keeps its correct "into the next tier" wording. (b) `mk` now renders SP as `SP` rather than `поени`, removing the collision with the existing `xp` block; flagged to the owner as their call, no new Macedonian term coined. Parity green, no em dash in mk/sr/hr.
- **P10.** `--rt8-line` now derives from `var(--accent)` via `color-mix` in both scopes, so all three Radiant tokens track the accent. The ramp comment records why hardcoding the RGB defeated the indirection it sits under.
- **P11.** Both comment corrections applied: the `gorm_repo.go` upsert now explains that `TierForSP(award.SP)` is correct only on the INSERT branch and that deleting the follow-up UPDATE would freeze every returning player's stored tier; the "READS MUST NOT WRITE" claims in `repository.go` and `service.go` are reworded to "a read never writes `player_seasons`", stating explicitly that the read path CAN create the `seasons` row via the lazy resolver.

Left alone as instructed: the double `rank_tier` write, the `getTestDB` DB-skip, `queryKeys.season.current()` as a function, `useCurrentSeasonQuery`'s `enabled` parameter, and `server/.golangci.yml`.

**Verification (post-review).** `make lint` exits **0** (client `tsc --noEmit` + ESLint + Prettier; server `golangci-lint run ./...` with zero findings). `make test` exits **0**: `npx vitest run` **1818/1818** across 131 files, and `go test ./...` green in every package. `gofmt -l ./internal ./cmd` empty; `go vet ./...` clean. Migration re-applied and `down 1` -> `up` cycled again after the P1 fix. WS golden pair re-verified. `honor_wiring_test.go`'s `LAST(honor) < match_state` assertion still holds.

**Verification (initial round):** `go build ./...` + `go vet ./...` clean; `go test ./...` all 20 packages pass (season DB tests run against the live dev DB on 5433 with per-test transaction + rollback); `npx tsc --noEmit` clean (tests included); `npx eslint .` clean; `npx prettier --check .` clean; `npx vitest run` 1812/1812 pass across 131 files. WS golden pair re-verified in both directions. `honor_wiring_test.go`'s `LAST(honor) < match_state` assertion still holds with SP slotted between (verified, not assumed).

### File List

**New — server**

- `server/migrations/000024_create_seasons_and_player_seasons.up.sql`
- `server/migrations/000024_create_seasons_and_player_seasons.down.sql`
- `server/internal/season/tier.go`
- `server/internal/season/tier_test.go`
- `server/internal/season/quarter.go`
- `server/internal/season/quarter_test.go`
- `server/internal/season/model.go`
- `server/internal/season/repository.go`
- `server/internal/season/gorm_repo.go`
- `server/internal/season/gorm_repo_test.go`
- `server/internal/season/service.go`
- `server/internal/season/handler.go`
- `server/internal/season/handler_test.go`
- `server/internal/match/sp_award.go`
- `server/internal/match/sp_award_test.go`
- `server/internal/match/sp_wiring_test.go`
- `server/internal/game/instant_win_flag_test.go`
- `server/internal/ws/testdata/events/season_points_awarded.json`

**New — client**

- `client/src/shared/lib/seasonTier.ts`
- `client/src/shared/lib/seasonTier.test.ts`
- `client/src/shared/api/season.ts`
- `client/src/shared/hooks/queries/useCurrentSeason.ts`
- `client/src/features/lobby/components/RankBanner.tsx`
- `client/src/features/lobby/components/RankBanner.test.tsx`

**Modified — server**

- `server/internal/game/state.go` — `WonByInstantWin bool \`json:"-"\`` beside `StoppedAtTarget`
- `server/internal/game/bidding.go` — set the flag at `checkInstantWin`'s success branch
- `server/internal/game/scoring.go` — set it in `startNewHand`'s instant-win branch, clear it beside `StoppedAtTarget = false`
- `server/internal/match/live_match.go` — `SPAward` / `PlayerSeasonSnapshot` / `SPAwarder` + `spAwarder` field + `SetSPAwarder`; hoisted the `handsCopy` RLock block above `settleMatch`; SP award after `recordHonor`; `spMsgs` loop after `honorMsgs`, before the trailing state broadcast
- `server/internal/match/reconnect.go` — `spectacularMatch` derived under the session lock; SP award after `recordHonor`; `spMsgs` loop after `honorMsgs`, before `SendFrames`
- `server/internal/ws/events.go` — `EventSeasonPointsAwarded` + `SeasonPointsAwardedPayload`
- `server/internal/ws/events_contract_test.go` — new golden table case
- `server/cmd/api/main.go` — `season` import; `seasonRepo` / `seasonService`; `sessionManager.SetSPAwarder`; `api.GET("/seasons/current", ...)`

**Modified — client**

- `client/src/shared/types/wsEvents.ts` — `EVENT_SEASON_POINTS_AWARDED` + `SeasonPointsAwardedPayload`
- `client/src/shared/types/wsEvents.schemas.ts` — `SeasonPointsAwardedPayloadSchema` + conformance assertion + witness entry
- `client/src/shared/types/wsEvents.contract.test.ts` — golden import + table row
- `client/src/shared/types/apiTypes.ts` — `CurrentSeasonResponse`
- `client/src/shared/api/queryKeys.ts` — `queryKeys.season.current()`
- `client/src/shared/hooks/useWsDispatch.ts` — `EVENT_SEASON_POINTS_AWARDED` handler (type guards, query invalidation, tier-up `sonner` toast)
- `client/src/features/lobby/LobbyPage.tsx` — `useCurrentSeasonQuery` + `<RankBanner>` between `<HeroBlock>` and `<FriendList>`
- `client/src/index.css` — `--rt1`…`--rt8` (+ `-soft` / `-line`) in `:root` and `.game-table`
- `client/src/shared/i18n/en.json`, `hr.json`, `sr.json`, `mk.json` — new `season` namespace (8 tier names, `season.banner.*`, `season.tierUp.toast`)

## Suggested Review Order

**The SP formula and its two decisions**

- Entry point: the whole formula, and why absence forfeits per-seat and Capot pays the table
  [`sp_award.go:122`](../../server/internal/match/sp_award.go#L122)

- The presence gate D5 reuses from honor; fails open, which under-charges a ladder
  [`sp_award.go:82`](../../server/internal/match/sp_award.go#L82)

- Best-effort degradation: an error skips the events but never strands clients on the table
  [`sp_award.go:202`](../../server/internal/match/sp_award.go#L202)

**Wiring into the two finalizers**

- The interface declared in `match`, so `match` never imports `season`
  [`live_match.go:178`](../../server/internal/match/live_match.go#L178)

- The D4 hoist: one RLock'd copy above settlement, feeding both SP and the persist call
  [`live_match.go:1397`](../../server/internal/match/live_match.go#L1397)

- Natural end; SP slots after honor, before the trailing state broadcast
  [`live_match.go:1458`](../../server/internal/match/live_match.go#L1458)

- Abandonment path; snapshots taken under the session lock, real `abandonedSeat`
  [`reconnect.go:722`](../../server/internal/match/reconnect.go#L722)

**The engine signal**

- Server-only event record, deliberately outside `RefreshDerivedFlags`
  [`state.go:240`](../../server/internal/game/state.go#L240)

**The ladder, in one table**

- Thresholds and tokens in one ordered table so a retune is a one-place change
  [`tier.go:54`](../../server/internal/season/tier.go#L54)

- Radiant has no next tier; a finite table has a top, unlike the XP quadratic
  [`tier.go:116`](../../server/internal/season/tier.go#L116)

**Persistence**

- The lazily self-healing resolver that makes Story 13.3 an optimisation, not a prerequisite
  [`gorm_repo.go:34`](../../server/internal/season/gorm_repo.go#L34)

- One transaction for all four seats; returns pre-award SP so `tieredUp` needs no second read
  [`gorm_repo.go:129`](../../server/internal/season/gorm_repo.go#L129)

- Nil guard so a repository contract breach is an error, not a panic inside a finalizer
  [`service.go:51`](../../server/internal/season/service.go#L51)

- Quarter math in UTC; half-open windows so consecutive seasons neither gap nor overlap
  [`quarter.go:22`](../../server/internal/season/quarter.go#L22)

**Schema**

- Denormalized `rank_tier` plus the two counters that make absence countable
  [`000024...up.sql:45`](../../server/migrations/000024_create_seasons_and_player_seasons.up.sql#L45)

- Seed computed on the naive UTC quarter, so the row cannot depend on session TimeZone
  [`000024...up.sql:100`](../../server/migrations/000024_create_seasons_and_player_seasons.up.sql#L100)

**The wire**

- New event type rather than a widened payload, so stale tabs survive
  [`events.go:164`](../../server/internal/ws/events.go#L164)

- The TS mirror; `rankTier` stays a plain string for version skew
  [`wsEvents.ts:296`](../../client/src/shared/types/wsEvents.ts#L296)

- The read endpoint: absolute `endsAt`, zero state for a player with no row yet
  [`handler.go:69`](../../server/internal/season/handler.go#L69)

**Client**

- The five AC3 elements, and the terminal-bar case at Radiant
  [`RankBanner.tsx:36`](../../client/src/features/lobby/components/RankBanner.tsx#L36)

- WS-to-query bridge plus the tier-up toast; guards on type, never truthiness
  [`useWsDispatch.ts:430`](../../client/src/shared/hooks/useWsDispatch.ts#L430)

- Version-skew normaliser: an unknown token falls back to the SP's own bucket
  [`seasonTier.ts:83`](../../client/src/shared/lib/seasonTier.ts#L83)

- Pushed, not polled; invalidated by the event rather than a `refetchInterval`
  [`useCurrentSeason.ts:21`](../../client/src/shared/hooks/queries/useCurrentSeason.ts#L21)

- Mounted between the hero block and the friends card in the real single-column lobby
  [`LobbyPage.tsx:232`](../../client/src/features/lobby/LobbyPage.tsx#L232)

**Peripherals**

- New `--rt1`…`--rt8` ramp; Radiant derives from `--accent` in both scopes
  [`index.css:250`](../../client/src/index.css#L250)

- The HTTP DTO mirror, hand-synced and now pinned by a wire-key assertion
  [`apiTypes.ts:287`](../../client/src/shared/types/apiTypes.ts#L287)

- Client mirror of the ladder, display-only, never a decision
  [`seasonTier.ts:35`](../../client/src/shared/lib/seasonTier.ts#L35)
