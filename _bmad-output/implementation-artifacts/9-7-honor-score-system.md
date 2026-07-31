---
baseline_commit: 310b4e6a65840e03bab29a4e5a0627577f58cb71
---

# Story 9.7: Honor Score System

Status: done

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

## Story

As a player,
I want a public honor score that reflects how reliably I finish the matches I start, and that forgives old mistakes over time,
so that I can decide whom to play with, and so that one bad night does not brand me forever.

## Scope guardrails — read before Task 1

**This story computes and displays honor. It does NOT gate anything.** Room access gating (`min_honor`, `allow_new_players`, `event:honor_eject`) is Story 9.8. Cross-player profile visibility is Epic 11 Story 11.3. Do not build either.

**Three epic ACs are deliberately overridden by PO decision 2026-07-29.** The epic's Story 9.7 text predates the system as built. The overrides are D1, D2 and D3 below and they are binding — implement the ACs in *this* file, not the ones in `epics.md`.

**Do not "fix" `prd.md:150`.** It says the New Player floor is 20 completed matches. It is stale; the floor is **5** (`sprint-change-proposal-2026-06-18.md#Section 4`). `epics.md` is the canonical FR list.

## Design decisions (binding)

### D1 — The four-bucket formula collapses to two buckets. (Overrides epic AC1)

The epic specifies `honor = 100 × completed / (completed + 2.0·rage_quits + 1.5·timeout_abandons + 0.3·dc_abandons)`. Three of those buckets do not correspond to distinct events in the system as built:

| Epic bucket | Reality in this codebase |
| --- | --- |
| `rage_quits` | A rage quit reaches the server as a socket close, byte-identical to a network drop. There is no in-match leave action (grep `action:leave` in `server/internal/ws/events.go` → zero hits), and `POST /rooms/:id/leave` returns `apperr.ErrMatchAlreadyStarted` mid-match (`server/internal/room/handler.go:1310`). Close codes are read then discarded (`server/internal/ws/client.go:88-97`). |
| `dc_abandons` | Same event as above. Any disconnect arms a per-seat reconnect timer; expiry ends the match. |
| `timeout_abandons` | Does not exist. Per-move timer expiry **auto-plays a card and continues the match** (`server/internal/match/live_match.go:1362`). It never disconnects or abandons anyone. There is no strike counter and no threshold constant anywhere. |

PO ruling (2026-07-29): *"rage quit and abandonment of time out is basically the same way — each time someone disconnects, a timer of 2 mins is activated and when that passes then the match finishes... If timer of throwing a card passes, the game just plays for the player, it doesn't disconnect a player."*

**Therefore there are exactly two buckets: `completed` and `abandoned`.** Do not create columns, constants, or code paths for `rage_quits` or `timeout_abandons`. Do not add a new abandonment trigger — `spec-abandonment-per-player-results.md` freezes this: *"No new abandonment triggers — sole trigger stays reconnect-window expiry."*

### D2 — Honor decays with time. (New requirement, not in the epic)

PO requirement: *"the honour system should recover from very old rage quits/abandonment."*

Each match contributes a recency weight `w = 0.5 ^ (age_days / HONOR_HALF_LIFE_DAYS)`. A 90-day-old event counts half; a year-old event counts ~6%.

### D3 — Honor is stored on `users`, as decayed weights + a score snapshot. (Refines epic AC1's "counters persist as integers")

PO requirement: *"later the room entrance will be guarded by this score... just need to make sure we update it all the time in all scenarios so it is a valid one and even later some admin user can forgive/reset the honour of some players. If it is by matches computed, then it cannot be forgiven with a later cmd."*

Two reasons the score must be stored rather than derived from `matches` on read:

1. **Forgiveness.** An admin must be able to pardon or reset a player. A pure function over immutable match history cannot be overridden.
2. **Story 9.8's join gate** reads honor on every join attempt; it must be a column read, not an aggregate query.

**The decay/storage tension resolves exactly, not approximately.** Because every term decays by the same factor:

```text
Σ 0.5^((now − tᵢ)/H)  =  0.5^((now − last)/H) · Σ 0.5^((last − tᵢ)/H)
```

So "decay the stored running weight forward, then add the new event" is *algebraically identical* to summing per-match weights from scratch. Store the running weights and the decay reference timestamp; both incremental update and point-in-time read are exact.

**Which value is authoritative — this must not be ambiguous.** Decay means the score changes as time passes even when nothing happens, so a stored score is stale by design.

- **Authoritative, always:** `HonorScore(completedWeight, abandonedWeight, decayedAt, now)` — pure arithmetic on three columns of the user row you have already loaded. Every read path (profile, auth envelope, `event:honor_updated`, and Story 9.8's join gate) recomputes this. No extra query, no staleness.
- **`honor_score` column:** a denormalized snapshot refreshed on every honor write, existing *only* so operators and future features can filter/sort in SQL (`WHERE honor_score < 50`). It is allowed to lag. **Never render it and never gate on it.**

Write that distinction as a comment on both the column (migration header) and the struct field, or someone will gate on the snapshot in six months.

### D4 — Honor lives in `internal/user`, injected into `match` via an interface declared in `match`

`server/internal/user/handler.go` imports `match` (it holds a `match.MatchRepository`; wired at `server/cmd/api/main.go:133`). **Therefore `match` must never import `user`.** Mirror Story 9.5's `XPAwarder` exactly: declare `HonorRecorder` in the `match` package, satisfy it with a `user.HonorService`, inject from `main.go`. Nil-tolerant.

### D5 — Emit `event:honor_updated`, updating all six drift-gate touchpoints in one commit

PO chose "Profile + TopBar, with a WS event". A **new** event const + new schema is version-skew safe (old bundles ignore unknown event types). Do **not** instead add honor fields to an existing `z.strictObject` payload — that is deferred item D142's hard-failure mode for stale tabs.

### D6 — The pure honor math has exactly one client copy, and it is display-only

Mirror `xpLevel.ts` / `level.go`: server is authoritative, the client copy computes only cosmetic presentation (tier bucket + bar fill) and never a gating decision. Keep it to one file (`client/src/shared/lib/honor.ts`).

### D7 — 9.7 makes forgiveness *possible*, it does not build an admin surface

Ship `ResetHonor(userID)` on the repository + a documented SQL recipe in the migration header. **No admin endpoint, no admin role, no admin UI** — there is no admin system in this project and building one is out of scope.

## The formula (authoritative spec)

### Constants — named, documented tunables

Declared once in `server/internal/user/honor.go`. These are placeholders per `sprint-change-proposal-2026-06-18.md#Section 4` (*"honor weights stay as placeholders, tuned during each story's planning"*) — they must be named consts so retuning never touches logic.

| Const | Value | Meaning |
| --- | --- | --- |
| `honorHalfLifeDays` | `90.0` | Days for an event's weight to halve |
| `honorAbandonPenalty` | `4.0` | One abandonment offsets four completed matches |
| `honorPriorCompleted` | `4.0` | Bayesian pseudo-count (numerator + denominator) |
| `honorPriorAbandoned` | `1.0` | Bayesian pseudo-count (denominator only) |
| `honorNewPlayerMinMatches` | `5` | Below this many raw matches PLAYED (completed + abandoned) → "New Player". Amended 2026-07-29 (review pass 2); it counts experience, not successes. |
| `honorTrendWindow` | `20` | Matches in the recent-trend window |
| `honorTrendThreshold` | `2` | Min \|delta\| to render up/down rather than flat |

### Score

```text
f      = 0.5 ^ (daysSince(decayedAt) / honorHalfLifeDays)     // decay-forward factor
C      = completedWeight × f
A      = abandonedWeight × f
honor  = round( 100 × (C + 4) / (C + 4 + 4·A + 1) )            // clamped to [0,100]
```

The `+4 / +1` prior is Bayesian pseudo-counting (Beta(4,1)) — the standard remedy for "1-of-1 reads as 100%". It puts a fresh player at 80 and makes Exemplary earned rather than default. Adding pseudo-successes/failures is the same technique as the Jeffreys/Laplace-smoothed proportion.

**Worked examples — use these verbatim as the table-test cases:**

| completedWeight | abandonedWeight | honor | Tier |
| --- | --- | --- | --- |
| 0 | 0 | 80 | (New Player — suppressed) |
| 5 | 0 | 90 | Trusted |
| 20 | 0 | 96 | Exemplary |
| 50 | 0 | 98 | Exemplary |
| 20 | 1 | 83 | Fair |
| 20 | 2 | 73 | Fair |
| 20 | 4 | 59 | Unreliable |
| 10 | 10 | 25 | Problematic |
| 20 | 0.06 (1yr-old abandon) | 95 | Exemplary — recovered |

The last two rows were originally written as 26 and 95→96; both were arithmetic slips (`1400/55 = 25.45 → 25`, `2400/25.24 = 95.09 → 95`) and are corrected in place here so this table stays the normative source AC2 points at. The tier column was unaffected. Independently recomputed during the 2026-07-29 code review; all nine rows now match the formula and the shipped tests.

Note the decay direction: an inactive player's `C` and `A` both shrink, so honor drifts back toward the `100×4/5 = 80` prior. That is intended — honor answers *"is this player reliable right now"*.

### Tiers

Server returns the tier as a stable machine token; the client maps token → i18n label + colour. **Never send a display string over the wire.**

| Score | Token | i18n label | Tone |
| --- | --- | --- | --- |
| 95–100 | `exemplary` | "Exemplary" | brass |
| 85–94 | `trusted` | "Trusted" | accent |
| 70–84 | `fair` | "Fair" | neutral |
| 50–69 | `unreliable` | "Unreliable" | muted |
| 0–49 | `problematic` | "Problematic" | danger |

### Trend

> **AMENDED by PO decision 2026-07-29 (code review).** This section originally compared the last-20 window against the **lifetime** score. That was wrong: the Beta(4,1) prior adds 4 pseudo-completions whose drag depends on sample size, so a flawless 20-match window caps at `100×24/25 = 96` while an active player's lifetime score reaches 98-100. Every engaged, never-abandoned player therefore rendered a permanent red "Slipping −3", and a player idle for a year rendered "Improving +12". The corrected definition below compares two **equal-size adjacent windows**, where the prior drag cancels.

Same formula over the **last 20 matches, undecayed** (the window *is* the recency mechanism), compared against the **20 matches immediately before those**:

```text
trendDelta = honorOverLast20 − honorOverThe20Before
trendDirection = up   if trendDelta ≥ +2
                 down if trendDelta ≤ −2
                 flat otherwise
```

The comparison renders **flat unless both windows hold the same number of matches**. A player therefore needs 40 finished matches before any trend appears; below that the honest answer is "not enough evidence", and flat is how that is shown. Never compare unequal windows — that reintroduces the exact prior-drag mismatch this amendment removes.

The windows are the one part that must query `matches` (the stored weights are aggregates and cannot be windowed). One bounded `2×20` fetch split by `ROW_NUMBER`, no `OFFSET` (deferred item D82). Compute it in the profile read path only — **not** on the auth/TopBar path, and **not** in the join gate.

### New Player suppression

> **AMENDED by PO decision 2026-07-29 (code review).** The floor originally counted **completions only**. That let the worst possible actor hide behind the newcomer chip indefinitely: 0 completed / 20 abandoned is a real score of **5** ("problematic"), yet it suppressed identically to a genuine first-timer, and Story 9.8's gate reads `isNewPlayer` off the same envelope. The floor now counts **experience**.

`isNewPlayer = (honorCompletedTotal + honorAbandonedTotal) < 5`, using the **raw lifetime integers**, never the decayed weights — otherwise a returning veteran gets relabelled "New Player". When `isNewPlayer` is true the client hides the numeric score and the tier, shows a "New Player" chip, and **still shows the raw counts** (PO decision).

The server still returns the real `honorScore` and `honorTier` when `isNewPlayer` is true (9.8's gate needs them; suppression is presentation-only).

## Acceptance Criteria

**AC1 — Schema.**
Given migration `000017`
When I inspect `users`
Then it has `honor_completed_weight NUMERIC(14,6) NOT NULL DEFAULT 0 CHECK (>= 0)`, `honor_abandoned_weight NUMERIC(14,6) NOT NULL DEFAULT 0 CHECK (>= 0)`, `honor_decayed_at TIMESTAMPTZ` (nullable; NULL = never decayed, treat factor as 1), `honor_completed_total BIGINT NOT NULL DEFAULT 0 CHECK (>= 0)`, `honor_abandoned_total BIGINT NOT NULL DEFAULT 0 CHECK (>= 0)`, `honor_score SMALLINT NOT NULL DEFAULT 80 CHECK (BETWEEN 0 AND 100)`
And the up migration **backfills all six columns from existing `matches` rows** so existing players do not all reset to the 80 prior
And `.down.sql` drops all six in reverse order
And there are **no** `rage_quits` or `timeout_abandons` columns (D1)

**AC2 — Pure score function.**
Given `server/internal/user/honor.go`
When the score is computed
Then `HonorScore(completedWeight, abandonedWeight float64, decayedAt *time.Time, now time.Time) int` implements the formula above, clamped to `[0,100]`
And `HonorTier(score int) string` returns one of the five tokens
And `IsNewPlayer(completedTotal, abandonedTotal int64) bool` compares `completedTotal + abandonedTotal` against `honorNewPlayerMinMatches` (amended 2026-07-29 — see "New Player suppression")
And `DecayFactor(decayedAt *time.Time, now time.Time) float64` returns `1.0` when `decayedAt` is nil
And every function is pure — no DB, no clock reads (time is a parameter), no side effects
And all worked examples in the table above pass as table-driven test cases

**AC3 — Counters update in all five match-end scenarios, and only those.**
Given a match reaches a terminal state
When honor is recorded
Then exactly this matrix applies:

| Scenario | Code path | Effect |
| --- | --- | --- |
| Natural end (win/loss) | `handleMatchEnd` | every human seat: `completed +1` |
| Surrender accepted | `handleMatchEnd` (same path, `Status` stays `completed`) | every human seat: `completed +1` |
| Abandonment (reconnect-window expiry) | `handleSeatReconnectTimeout` | EVERY human seat that was not at the table: `abandoned +1` (the seat whose window expired, plus any other absent seat); human seats still **connected**: `completed +1` (amended 2026-07-29 — see below) |
| Boot reconcile of a stale room (`abandoned_by IS NULL`) | `reconcile.go` | **no honor change for anyone** — server fault, not a player signal |
| Disconnect resolved inside the window | (no terminal state) | **no honor event at all**; the match continues to natural end and scores `completed` there |

And bot seats never accrue honor — reuse the guard verbatim: `if botSeats[seat] || playerIDs[seat] == 0 { continue }`
And in a concurrent double-disconnect, EVERY absent human seat is charged `abandoned` — not merely the one whose reconnect timer fired; a human seat that was **still connected** gets `completed`
And each scenario has its own test

> **AMENDED by PO decision 2026-07-29 — THE RULE IS PRESENCE, and this took two review passes to settle.** The clause originally read "every other human seat gets `completed`". `handleConcurrentDisconnectLocked` opens a fresh full window plus its own timer per drop, so two or more overlapping windows is a normal state — and crediting a `completed` to a seat that had walked out and never returned **raised** their honor, making quitting *second* strictly better than quitting first: a gameable bypass of the signal Story 9.8 gates room access on.
>
> **Pass 1 made those seats neutral (no event) and that was wrong too.** Because nothing is written, `honor_completed_total + honor_abandoned_total` never increments, so a repeat second-quitter stayed pinned at the 80 prior with `isNewPlayer = true` **forever** — and 9.8's gate reads both off the auth envelope. It also silently cost an honest player their completion when a socket blipped just before another seat's window expired. And the rule was unrepresentable on the two read paths: `matches` stores a single `abandoned_by` and no per-seat presence, so the trend query and the 000017 backfill both kept crediting the seat regardless.
>
> **Pass 2 therefore charges every absent seat `abandoned`.** Monotonic, no ordering incentive, and representable. Pass 1 had rejected this as creating a second abandonment trigger against `spec-abandonment-per-player-results.md`; on re-reading, that freeze governs what ENDS a match and what lands in `abandoned_by`, neither of which changes here — only the honor bucket. The presence gate applies only on the abandonment path; a natural end or an accepted surrender reached a real terminal state and still credits every human seat. Note `connected` fails OPEN (see deferred-work.md), so the gate under-charges rather than over-charges.

**AC4 — Atomic, exact, decay-correct persistence.**
Given honor counters are written
When the repository applies them
Then it runs in **one transaction**, taking `clause.Locking{Strength: "UPDATE"}` on each row in **ascending userID order** (the deadlock-avoidance contract shared with wallet settlement and `AddXP`)
And for each user it decays the stored weights forward by `DecayFactor(honor_decayed_at, now)` **before** adding the new event's weight of `1.0`
And it then sets `honor_decayed_at = now`, increments the matching raw total, and refreshes the `honor_score` snapshot
And a failure logs via `slog.Error` and skips the honor events but **never blocks** `match_end` / `match_abandoned` / `match_state` (best-effort, mirroring settlement and XP)

**AC5 — `event:honor_updated`.**
Given a match ends
When honor events are broadcast
Then each human player receives a per-user `event:honor_updated` carrying `honorScore`, `honorTier`, `honorCompletedTotal`, `honorAbandonedTotal`, `isNewPlayer`
And it is slotted **after** `event:xp_awarded` and **before** the trailing `event:match_state`, in **both** finalizers, preserving the Story 8.5-1 ordering contract
And all six drift-gate touchpoints are updated in the same commit (see the checklist in Dev Notes)
And the client dispatcher validates every field with `Number.isInteger` / `typeof === "string"` / `typeof === "boolean"` before use — never JS truthiness (Go zero values serialize as real `0` / `false`)

**AC6 — Profile display.**
Given a player views their profile
When it renders
Then an honor surface shows the numeric score, the tier label, the raw completed/abandoned counts, and the trend indicator
And when `isNewPlayer` is true it shows a "New Player" chip **instead of** the score and tier, while still showing the raw counts
And colour is never the only signal — the tier word and the numeric value always accompany the tone (UX spec: *"No information conveyed exclusively through colour"*)
And any transition respects `prefers-reduced-motion` via the existing `useReducedMotion` hook
And the component exposes `data-honor`, `data-tier`, `data-new-player`, `data-trend-direction` so tests never key on tier wording or i18n

**AC7 — TopBar display.**
Given the top nav renders
When the user is authenticated
Then honor score + tier tone appear beside the existing coin pill and level/XP bar
And the value is available on first paint (it rides the auth envelope, like `level` / `totalXp`)
And it live-updates from `event:honor_updated` via `authStore.setUser({ ...user, ... })`

**AC8 — API surface.**
Given `GET /users/:id/profile`
When it responds
Then `ProfileResponse` gains `honorScore`, `honorTier`, `honorCompletedTotal`, `honorAbandonedTotal`, `isNewPlayer`, `honorTrendDelta`, `honorTrendDirection`
And these fields are documented in-code as **public-safe** — unlike their `walletBalance` / `totalXp` neighbours — so Epic 11's public DTO can lift them without re-litigating
And the auth envelope (`RegisterResponseData`, shared by Register/Login/Refresh/SSO) gains `honorScore`, `honorTier`, `isNewPlayer` only (not the trend — that needs the windowed query)
And the `:id == authUserID` check is **not** relaxed (that is Story 11.3's scope)

**AC9 — Forgiveness hook.**
Given an operator must pardon a player
When `ResetHonor(userID)` is called on the repository
Then the weights, totals and `honor_decayed_at` reset to their defaults and `honor_score` returns to the 80 prior, in one transaction
And the migration header documents the equivalent SQL recipe
And **no admin endpoint or UI is built** (D7)

**AC10 — i18n.**
Given honor strings are added
Then a `profile.honor.*` block exists in **all four** of `en.json` / `sr.json` / `mk.json` / `hr.json` with 1:1 keys
And `mk.json` copy is **all-Cyrillic**
And **no em dash (`—`) appears in the mk / sr / hr strings** (English only)
And `i18n.parity.test.ts` passes

## Tasks / Subtasks

- [x] **Task 1 — Migration `000017_add_honor_to_users`** (AC: 1, 9)
  - [x] Confirm `000016` is still the highest in `server/migrations/` before naming the file; never skip numbers
  - [x] Up: six `ALTER TABLE users ADD COLUMN` statements, one per line, each with a `--` comment block explaining the value semantics and type choice (follow `000011_add_xp_to_users.up.sql` prose style)
  - [x] Type rationale to write into the header: weights are `NUMERIC(14,6)` because decay produces fractional values and float drift in a money-adjacent trust signal is unacceptable; totals are `BIGINT` because they are monotonic lifetime accumulators summed by a 64-bit Go `int` (this is exactly the trap the 9.5 review caught when `total_xp` shipped as `INTEGER`); `honor_score` is `SMALLINT` because it is bounded 0–100
  - [x] Backfill in the up migration from `matches`, using `power(0.5, EXTRACT(EPOCH FROM (NOW() - completed_at)) / (90 * 86400))` for the weight; `completed` = `status='completed'` (any seat) plus `status='abandoned' AND abandoned_by IS NOT NULL AND abandoned_by <> users.id`; `abandoned` = `status='abandoned' AND abandoned_by = users.id`; **exclude** `abandoned_by IS NULL` rows entirely
  - [x] Set `honor_decayed_at = NOW()` and `honor_score` from the backfilled weights in the same migration
  - [x] Down: drop all six in reverse order, with a `-- Reverse 000017 by ...` header
  - [x] Verify on the dev DB (port **5433**): `make migrate` → `down 1` → `up 1`, inspecting column state at each step

- [x] **Task 2 — `server/internal/user/honor.go`** (AC: 2)
  - [x] Seven named consts per the table above, each with a one-line comment
  - [x] `DecayFactor`, `HonorScore`, `HonorTier`, `IsNewPlayer` — all pure, `now` passed in
  - [x] Header comment stating this is the single source of truth and naming the client mirror (`client/src/shared/lib/honor.ts`), matching `level.go`'s convention
  - [x] `honor_test.go`: table-driven, every worked example from the spec table, plus tier boundaries (49/50, 69/70, 84/85, 94/95), a nil `decayedAt`, a negative-input defensive case, and a decay case proving a 365-day-old abandonment recovers to Exemplary

- [x] **Task 3 — Repository + service** (AC: 4, 9)
  - [x] `UserRepository`: add `ApplyHonorEvents(events map[uint]HonorEvent, now time.Time) (map[uint]HonorSnapshot, error)` and `ResetHonor(userID uint) error`, each with an interface-level doc comment
  - [x] Implement in `gorm_repo.go` copying `AddXP`'s lock discipline exactly (`gorm_repo.go:169`): one `db.Transaction`, `clause.Locking{Strength:"UPDATE"}`, `slices.Sort(ids)` ascending, `apperr.ErrUserNotFound` rolls the whole batch back
  - [x] Decay-forward-then-add inside the locked read, per AC4
  - [x] `user/honor_service.go`: `HonorService` wrapping the repo, mirroring `xp_service.go`'s thin-passthrough shape
  - [x] **Update all `UserRepository` mocks** or the build breaks: `user/handler_test.go` `mockUserRepo`, `user/xp_service_test.go` `fakeLevelRepo`. Find them with `grep -rn "UserRepository" server/internal | grep -i mock`
  - [x] `honor_repo_test.go`: DB-backed with per-test transaction rollback and `t.Skip` when no DB, following `xp_repo_test.go`; cover decay-forward correctness (write, advance `now`, write again, assert the sum equals the from-scratch sum), zero-event no-op, missing-user rollback, and `ResetHonor`

- [x] **Task 4 — `HonorRecorder` interface + match-end glue** (AC: 3, 4, 5)
  - [x] Declare `HonorRecorder` in `server/internal/match/live_match.go` beside `XPAwarder` (~`:81-89`), with the same import-cycle comment (D4)
  - [x] Add `Manager.honorRecorder` field (~`:108`) and nil-tolerant `SetHonorRecorder` (~`:157`)
  - [x] Wire in `server/cmd/api/main.go` immediately after `SetXPAwarder` (`:184`): `sessionManager.SetHonorRecorder(user.NewHonorService(userRepo))`
  - [x] New file `server/internal/match/honor_record.go`, sibling to `xp_award.go`: pure `computeHonorEvents(playerIDs [4]uint, botSeats [4]bool, connected [4]bool, abandonedSeat int) map[uint]HonorEvent` (the `connected` presence snapshot was added by review pass 2 — see the AC3 amendment) + impure `(m *Manager) recordHonor(...) []honorUpdateMsg`
  - [x] Bot/empty guard verbatim: `if botSeats[seat] || playerIDs[seat] == 0 { continue }`
  - [x] Best-effort degradation: `slog.Error` + return nil on failure, never block broadcasts
  - [x] Call site 1 — `live_match.go` **immediately after line 1116** (`xpMsgs := m.awardXP(...)`), before `matchRecord` is built; pass `abandonedSeat = -1`
  - [x] Call site 2 — `reconnect.go` **immediately after line 598** (`xpMsgs := m.awardXP(...)`); pass the real `abandonedSeat`
  - [x] Send the messages in both finalizers after the `xpMsgs` loop and before the trailing `match_state` broadcast (`live_match.go:1174-1177`, `reconnect.go:606-609`)
  - [x] Confirm `reconcile.go` is untouched — it has no live session and no `Manager`, so it is safe by construction. Add an explicit test asserting a boot-reconcile row changes nobody's honor

- [x] **Task 5 — Trend query** (AC: 6, 8)
  - [x] Add `GetHonorTrendWindowsForUser(userID uint, limit int) (HonorTrendWindows, error)` to `match.MatchRepository` + `gorm_repo.go`. (Originally specified as a single-window `GetRecentHonorWindowForUser`; replaced by review pass 2 — see the Trend amendment.)
  - [x] Reuse the canonical viewer gate **verbatim**: `abandoned_by IS NOT NULL AND abandoned_by <> ?`. Rows with `abandoned_by IS NULL` are excluded entirely
  - [x] Order by `completed_at DESC, id DESC`, fetch `2 × limit` in ONE bounded query and split it with `ROW_NUMBER()` into `rn <= limit` (recent) and `rn > limit` (prior); note deferred item D82 (offset pagination duplicates rows under concurrent completions) — use `LIMIT` without `OFFSET`
  - [x] Adding a method to `MatchRepository` breaks its stubs — update `match/matchend_test.go` `timestampedRepo` and `user/handler_test.go` `mockMatchRepo`
  - [x] DB-backed test in the existing `match/gorm_repo_test.go` harness (do not assert this SQL at mock level — that is deferred item D87)

- [x] **Task 6 — WS contract, all six touchpoints in one commit** (AC: 5)
  - [x] `server/internal/ws/events.go`: `const EventHonorUpdated = "event:honor_updated"` + `type HonorUpdatedPayload struct{...}` with camelCase json tags, plus a comment stating its slot in the ordering contract
  - [x] `client/src/shared/types/wsEvents.ts`: `EVENT_HONOR_UPDATED` const + `HonorUpdatedPayload` interface
  - [x] `server/internal/ws/events_contract_test.go`: append a row to the `cases` table with a non-zero sample and `goldenFile: "honor_updated.json"`
  - [x] `server/internal/ws/testdata/events/honor_updated.json`: generate with `UPDATE_GOLDENS=1 go test ./internal/ws/... -run Contract` (a missing golden hard-fails; it is not auto-bootstrapped)
  - [x] `client/src/shared/types/wsEvents.schemas.ts`: type-import, `HonorUpdatedPayloadSchema = z.strictObject({...})` with `.int()` on every numeric, **and** the `MutualExtends` conformance witness registered in the `_conformanceWitnesses` export
  - [x] `client/src/shared/types/wsEvents.contract.test.ts`: golden import + schema import + a row in the `cases` tuple
  - [x] Verify green: `go test ./internal/ws/...`, `npx vitest run wsEvents`, `npx tsc -p tsconfig.build.json --noEmit`

- [x] **Task 7 — Server API surface** (AC: 8)
  - [x] Extend `ProfileResponse` (`server/internal/user/handler.go:19-41`) with the seven honor fields; add a comment marking them **public-safe**, in contrast to the existing "these are PRIVATE self-only figures" note
  - [x] Populate in `GetProfile`, calling `HonorScore`/`HonorTier`/`IsNewPlayer` plus the trend query
  - [x] Extend `RegisterResponseData` (`server/internal/auth/handler.go:37-52`) with `honorScore`, `honorTier`, `isNewPlayer` — all four of Register/Login/Refresh/SSO inherit it
  - [x] Handler test asserting the profile payload, mirroring `TestGetProfile_IncludesXPAndLevel`

- [x] **Task 8 — Client plumbing** (AC: 7)
  - [x] `client/src/shared/lib/honor.ts` — display-only tier bucketing + a `honorBarFill`-style helper if needed; header comment naming `server/internal/user/honor.go` as the source of truth and stating this is the ONLY client copy (D6). Co-located `honor.test.ts`
  - [x] `client/src/shared/types/apiTypes.ts` — add honor fields to `User` with the "Go zero values are real 0s, compare explicitly" caveat comment
  - [x] `client/src/shared/api/profile.ts` + `auth.ts` — extend the response interfaces
  - [x] `client/src/shared/hooks/useWsDispatch.ts` — handle `EVENT_HONOR_UPDATED` after the XP handler (~`:305-337`); validate every field before `authStore.setUser({ ...user, ... })`
  - [x] **Reset any per-match honor stash in BOTH the `match_end` and `match_abandoned` handlers** — the 9.5 review caught exactly this asymmetry, and abandoning-team members are the ones who receive no follow-up event

- [x] **Task 9 — Profile UI** (AC: 6)
  - [x] New `client/src/features/profile/components/HonorPanel.tsx` + co-located test
  - [x] Slot it into the space `IdentityHero.tsx:201-204` explicitly reserves ("*Leaves room for the not-yet-built honor / prior-season rank surfaces*"), or as a full-width section between `StreakCallout` and `StatsGrid` in `ProfilePage.tsx` — pick one and say which in Completion Notes
  - [x] Reuse existing vocabulary: `SectionHeader` / `SidePanel` / the `StatTile` shape from `StatsGrid.tsx`, and `Badge` for the tier chip. Do not invent a new visual system
  - [x] Tier tones from existing tokens — brass (`--brass-deep` / `Badge tone="brass"`), accent, neutral, muted, danger (`--danger` / `Badge tone="danger"`). Do **not** add a new gold token; if true gold is required, follow the `coinGold.ts` single-const-with-rationale precedent
  - [x] Score number uses `font-display` (Space Grotesk) per the UX spec typography rule
  - [x] Loading transient: do not mix a real fallback with a `0` fallback — that was a 9.5 review patch (`ProfilePage.tsx:45-48`)
  - [x] Emit the four `data-*` attributes from AC6
  - [x] Extend `ProfilePage.test.tsx`'s `profileFixture` with the new fields or its existing tests fail typecheck

- [x] **Task 10 — TopBar** (AC: 7)
  - [x] Add the honor chip to `client/src/shared/components/TopBar.tsx` beside the coin pill and XP bar; mirror the `xp-level` / `xp-bar` testid pattern with `honor-score` / `honor-tier`
  - [x] Suppress to a "New Player" state when `isNewPlayer`
  - [x] Extend `TopBar.test.tsx`

- [x] **Task 11 — i18n ×4** (AC: 10)
  - [x] Add `profile.honor.*` to all four locale files: score label, the five tier labels, raw-count labels, trend up/flat/down, "New Player" chip, and an explanatory tooltip string
  - [x] Check there is no existing `profile.honor` **string** key before adding an **object** — JSON cannot hold both, and this exact collision bit Story 7-2
  - [x] mk all-Cyrillic; no em dash in mk/sr/hr
  - [x] New-player copy follows the house invitational voice (cf. *"No games yet — [Quick Play] to get started"*), e.g. "Complete 5 matches to earn an honor score"

- [x] **Task 12 — Fixture blast radius**
  - [x] Adding required fields to `User` detonates fixtures across ~12 test files (9.5's `totalXp`/`level` did, 9.6's `isPrivate` did again). Sweep with `npx tsc -p tsconfig.build.json --noEmit` and fix all
  - [x] Prefer a centralized fixture helper so the next additive change touches one place (7-2's `profileFixture()` precedent)
  - [x] Known pre-existing tsc noise — **do not fix in this story**: `RoomDetail.returnedUserIds` missing from mocks in `MatchmakingPage.test.tsx` and `RoomPage.bots.test.tsx`

- [x] **Task 13 — Gates**
  - [x] `cd server` (mise-shimmed go 1.26) → `go vet ./...`, `gofmt -l .`, `go test ./...`, `golangci-lint run ./...` (v1.64.8, matching CI)
  - [x] `cd client` → `npx tsc -p tsconfig.build.json --noEmit`, `npx vitest run`, `npx eslint .`, `npx prettier --check .`
  - [x] Run `npx prettier --write` on changed files before committing — a single missing space after a colon blocked CI during 9.5
  - [x] `make test` + `make lint` green

- [x] **Task 14 — Docs**
  - [x] Update `deferred-work.md` with anything consciously deferred
  - [x] Correct the two stale in-code comments that call honor "Story 9.6": `server/internal/match/live_match.go:1078` and `server/internal/match/model.go:20`

## Dev Notes

### Where the code goes

| Concern | Path | New/Update |
| --- | --- | --- |
| Migration | `server/migrations/000017_add_honor_to_users.{up,down}.sql` | NEW |
| Pure honor math | `server/internal/user/honor.go` + `honor_test.go` | NEW |
| Service | `server/internal/user/honor_service.go` | NEW |
| Repo interface | `server/internal/user/repository.go` | UPDATE |
| Repo impl | `server/internal/user/gorm_repo.go` | UPDATE |
| User model columns | `server/internal/user/model.go` | UPDATE |
| Profile DTO | `server/internal/user/handler.go` | UPDATE |
| Auth envelope | `server/internal/auth/handler.go` | UPDATE |
| Match-end glue | `server/internal/match/honor_record.go` + `honor_record_test.go` | NEW |
| Interface + setter + 2 call sites | `server/internal/match/live_match.go` (after `:1116`), `reconnect.go` (after `:598`) | UPDATE |
| Trend query | `server/internal/match/repository.go`, `gorm_repo.go` | UPDATE |
| DI wiring | `server/cmd/api/main.go` (after `:184`) | UPDATE |
| WS contract ×6 | see Task 6 | UPDATE |
| Client math | `client/src/shared/lib/honor.ts` + test | NEW |
| Profile UI | `client/src/features/profile/components/HonorPanel.tsx` + test | NEW |
| Profile page / hero | `client/src/features/profile/ProfilePage.tsx`, `components/IdentityHero.tsx` | UPDATE |
| TopBar | `client/src/shared/components/TopBar.tsx` | UPDATE |
| Types / API / dispatch | `apiTypes.ts`, `api/profile.ts`, `api/auth.ts`, `hooks/useWsDispatch.ts` | UPDATE |
| i18n ×4 | `client/src/shared/i18n/{en,sr,mk,hr}.json` | UPDATE |

### Current state of the files you are modifying

**`server/internal/match/live_match.go` — `handleMatchEnd` (`:1092`).** Side-effect order today: derive `botSeats` → `settleMatch` (`:1109`) → `awardXP` (`:1116`) → build `matchRecord` (`:1118`) → snapshot hand results → `CreateWithHands` (`:1151`) → `UpdateRoomStatus` → broadcasts (`:1169-1177`) → `RemoveSession`. **Preserve all of it.** Your insertion is one call after `:1116` and one send loop between the `xpMsgs` loop and the trailing `match_state`. Note this path persists *before* broadcasting.

**`server/internal/match/reconnect.go` — `handleSeatReconnectTimeout` (`:511`).** Guards (generation staleness `:516`, `Connected` re-check `:526`) → capture `abandonedSeat`/`abandonedPlayerID` (`:531`) → phase to `PhaseMatchEnd` → cancel timers → snapshot under lock → build messages → **unlock (`:582`)** → `winningTeam` (`:590`) → `settleMatch` (`:591`) → `awardXP` (`:598`) → broadcasts (`:601-609`) → persist (`:620-655`) → `RemoveSession`. Unlike the natural path, this one **broadcasts before persisting** (open deferred item from 8.5-1). Consequence for you: compute honor from in-memory session data, **never** by reading back the just-persisted match row — it may not exist yet.

**`server/internal/user/gorm_repo.go` — `AddXP` (`:169`).** Copy its structure exactly. It does read-under-`FOR UPDATE`, compute in Go, write the absolute value — **not** `gorm.Expr`. `gorm.Expr` appears in only two places repo-wide, both on `rooms.player_count`; it is not the precedent here. The `slices.Sort(ids)` ascending-lock order is load-bearing: it is what stops a concurrent wallet settlement from deadlocking.

**`server/internal/user/handler.go:15-31`.** Carries an explicit warning that `WalletBalance` / `LoginStreakDays` / `TotalXP` are private self-only figures that must never reach a public DTO. Honor is the **opposite** privacy class — annotate it as public-safe so Epic 11 does not have to re-derive that conclusion.

**`client/src/features/profile/components/IdentityHero.tsx:201-204`.** Literally reserves your slot: *"Leaves room for the not-yet-built honor / prior-season rank surfaces (render nothing for them)."* Story 9.5 was explicitly forbidden from stubbing fake honor values; you are the story that fills it.

### Previous-story intelligence (9.5 is the blueprint)

Story 9.5 (XP & level) is the same shape: new user column + derived value + match-end hook + WS event + profile UI + TopBar. Copy it structurally and you will be right.

What its code review caught, that you should not repeat:

1. **Column width.** `total_xp` shipped as `INTEGER` and was widened to `BIGINT` in review because a lifetime monotonic accumulator must match the 64-bit Go `int` summing into it. Your raw totals are the same class.
2. **The WS event shipped with only 2 of 6 drift-gate touchpoints** and needed four patches. Do all six up front. This is described in the story record as "the project's #1 WS rule".
3. **`match_abandoned` did not reset the per-human stash** that `match_end` reset. Abandoning-team members are precisely the players who get no follow-up event, so stale state is their failure mode.
4. **Profile loading transient** mixed a real fallback (`user.level`) with a `0` fallback (progress), rendering a non-zero level with an empty bar.
5. **Prettier.** A single missing space blocked CI.
6. **`abandonPartialXPFactor` float truncation is untested for non-1.0** (open deferred item). Your decay factor is a float in a similar position: decide round-vs-floor explicitly and test a non-identity value from day one.

From 9.6 (private rooms): a UI-only guard is not a guard — enforce server-side. And check *every* entry point, not just the primary one (its deep-link path needed the same fix as the main path).

### The import-cycle rule, stated once more

`user` imports `match`. Therefore **`match` must never import `user`.** The `XPAwarder` interface exists solely to satisfy this: it is declared in `match`, structurally satisfied by a `user` type, and injected from `main.go`. Note that `XPAwarder` carries `LevelForXP` on the interface *specifically* so the match package can compute a derived value for the payload without importing `user` — you need the same trick for `HonorScore`/`HonorTier` if the event payload carries them. Declare them on `HonorRecorder`.

### Drift-gate checklist (all six, one commit)

1. `server/internal/ws/events.go` — const + payload struct
2. `client/src/shared/types/wsEvents.ts` — const + interface
3. `server/internal/ws/events_contract_test.go` — `cases` row
4. `server/internal/ws/testdata/events/honor_updated.json` — golden (`UPDATE_GOLDENS=1`)
5. `client/src/shared/types/wsEvents.schemas.ts` — `z.strictObject` schema **+** `MutualExtends` witness registered in `_conformanceWitnesses`
6. `client/src/shared/types/wsEvents.contract.test.ts` — golden import + schema import + `cases` row

The Go test marshals the sample and diffs the golden; the TS test parses the *same* golden through Zod. Schemas are `z.strictObject`, so an added Go key fails with "Unrecognized key(s)" and a removed one fails with "Required". The witness catches a schema that parses but drifted from the hand-written TS interface.

Note what is **not** gated, so you do not over-apply this: `system:*` events and hand-built `map[string]any` payloads have no schema or golden (Story 9.3's precedent). Only typed-struct `event:*` payloads are in the gate.

### Testing requirements

- Go: standard `testing` + `testify`, co-located. Pure functions get **table-driven** tests; DB-backed repo tests use **per-test transaction rollback** with `t.Skip` when no DB is reachable (dev DB on port **5433**).
- Match-manager tests: reuse the existing harness rather than reinventing — `hubSpy` / `timestampedRepo` / `containsType` in `match/matchend_test.go`, `indexOfTypeAfter` in `settlement_wiring_test.go`, `stubXPAwarder` / `countTypeBetween` / `firstIndexOfType` in `xp_wiring_test.go`, and `AbandonSeatForTest` in `export_test.go` (drives abandonment deterministically without waiting on a real timer). **Synchronous assertions only — never `time.Sleep`.**
- Ordering must be asserted, not assumed: prove `honor_updated` lands after `xp_awarded` and before `match_state` in both finalizers.
- Vitest: co-located, `data-testid` selection only (never CSS classes), present-tense `it(...)`. Assert computed numbers via `data-*` attributes, not text — that is the established idiom (`data-value`, `data-rate`, `data-streak-kind`) and it keeps tests independent of i18n.
- Every one of AC3's five scenarios needs its own test, including the boot-reconcile no-op.

### Known limitations to record, not fix

- **An AFK player who lets the per-move timer auto-play every card still scores `completed`.** That follows directly from D1 (timer expiry is not an abandonment) and is consistent with how the system actually behaves. If this becomes a farming vector, it is a new story.
- **`rage_quits` and `timeout_abandons` are not modelled at all.** If a future story adds an explicit in-match leave action or an AFK-strike threshold, honor gains a bucket then. The formula's single penalty weight is easy to split later.
- **In-app navigation away from a live match keeps the WebSocket open**, so the match limps to natural end via auto-play and nobody is charged. Pre-existing behavior, out of scope.

### Project Structure Notes

Everything lands in existing folders — no new packages, no new feature directories. Honor is user-domain data (`internal/user/`, per the architecture's "Progression → `internal/user/` (extended Phase 2)" mapping), invoked by the match-end orchestrator through an injected interface. It is **not** rules-engine logic and must not go in `internal/game/`.

One conscious variance from precedent: Story 7-2 chose per-request aggregation over denormalized counters specifically to avoid two-writer drift, and this story denormalizes onto `users` anyway. The justification is D3 — admin forgiveness and 9.8's per-join gate both require mutable stored state that a pure function over `matches` cannot provide. The drift risk is real and is mitigated by (a) writing honor in the same best-effort band as XP and coins, from the same two finalizers, and (b) the migration's backfill being re-runnable as a reconciliation recipe. State this tradeoff in Completion Notes.

NFR8 applies: honor calculation is explicitly named as server-authoritative. The client copy is presentation only.

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Story 9.7: Honor Score System] — base ACs (AC1's four-bucket formula and counter-storage clause overridden by D1/D3)
- [Source: _bmad-output/planning-artifacts/epics.md#Story 9.8: Honor-Gated Rooms] — the downstream consumer; do not build it here
- [Source: _bmad-output/planning-artifacts/epics.md#Story 11.3: Public Player Profiles] — where cross-player visibility lands
- [Source: _bmad-output/planning-artifacts/epics.md#Additional Requirements] — NFR8 server-authoritative honor calculation
- [Source: _bmad-output/planning-artifacts/sprint-change-proposal-2026-04-18.md#4] — honor formula origin; the never-executed UX/architecture follow-ups
- [Source: _bmad-output/planning-artifacts/sprint-change-proposal-2026-06-18.md#Section 4] — New Player floor 20 → 5; honor renumbered 9.6 → 9.7; weights are placeholders
- [Source: _bmad-output/planning-artifacts/sprint-change-proposal-2026-06-11.md#Change 2] — bots accrue no XP, coins, honor or stats
- [Source: _bmad-output/planning-artifacts/prd.md#Phase 2] — **stale**, says floor 20; ignore
- [Source: _bmad-output/planning-artifacts/ux-design-specification.md#Accessibility Considerations] — no information conveyed exclusively through colour
- [Source: _bmad-output/planning-artifacts/ux-design-specification.md#Typography System] — score numbers use Space Grotesk
- [Source: _bmad-output/implementation-artifacts/9-5-xp-and-level-system.md] — the structural blueprint and its review learnings
- [Source: _bmad-output/implementation-artifacts/9-6-private-rooms.md] — HTTP-only precedent; enforce server-side; check every entry point
- [Source: _bmad-output/implementation-artifacts/5-5-match-abandonment-on-timeout.md] — reconnect-window expiry is the sole abandonment trigger
- [Source: _bmad-output/implementation-artifacts/spec-abandonment-per-player-results.md] — frozen: no new abandonment triggers; the canonical `abandoned_by IS NOT NULL AND abandoned_by <> viewer` gate
- [Source: _bmad-output/implementation-artifacts/7-2-expanded-player-profile.md] — profile stat-tile and empty-state conventions; the derived-vs-denormalized tradeoff this story inverts
- [Source: _bmad-output/implementation-artifacts/deferred-work.md] — D82 (offset pagination), D87/D88 (status-filter desync, untested raw SQL), D142 (strict-Zod version skew), D153 (AppLayout remount re-fire)
- [Source: _bmad-output/project-context.md] — migration numbering, i18n parity, atomic mutation, `data-testid`, commit/branch conventions
- Formula grounding: exponential decay with half-life is the standard reputation-aging form ([half-life](https://en.wikipedia.org/wiki/Half_time_(physics)), [exponential decay models](https://math.libretexts.org/Bookshelves/Precalculus/Book:_Precalculus__An_Investigation_of_Functions_(Lippman_and_Rasmussen)/04:_Exponential_and_Logarithmic_Functions/4.06:_Exponential_and_Logarithmic_Models)); the `+4/+1` pseudo-count prior is Bayesian smoothing of a proportion, the standard remedy for extreme values from tiny samples ([Wilson score / Jeffreys prior comparison](https://davidzhao1015.github.io/blog/2025/benchmark-interval-prop/), [Wilson score interval](https://statisticsfundamentals.com/confidence-intervals/wilson-score-interval/)); the "abandonment is the harshest penalty, recovery takes ~20-30 clean matches" calibration matches shipped behaviour-score systems ([Dota 2 behaviour score](https://steamcommunity.com/app/570/discussions/0/4436564907307381047/), [Wild Rift player behaviour system](https://wildrift.leagueoflegends.com/en-us/news/game-updates/player-behavior-system/))

## Open questions for the PO (non-blocking)

1. **Half-life of 90 days** — an abandonment is half-forgiven in 3 months, effectively gone in a year. Retune the const if that feels wrong once there is real data.
2. **Abandonment penalty of 4.0** — one abandonment offsets four completed matches, so one abandon in 20 drops Trusted → Fair. Harsher (6.0) pushes repeat abandoners down faster.
3. **Inactivity drifts honor back toward 80.** A veteran with 500 clean matches who stops playing for two years returns at ~80 "Trusted" rather than 98 "Exemplary". This is intentional for a *current* reliability signal, but confirm it reads as fair.
4. ~~**The innocent three in an abandoned match are credited `completed`.**~~ **RESOLVED 2026-07-29 (review pass 2):** only seats that were still CONNECTED are credited. Any seat absent at match end is charged an abandonment, regardless of whose reconnect timer fired — see the AC3 amendment.
5. **Admin forgiveness has no UI.** 9.7 ships the repo method and a SQL recipe only. When do you want the operator surface?

## Dev Agent Record

### Agent Model Used

claude-opus-5[1m] (Claude Code, bmad-dev-story workflow)

### Debug Log References

- **Migration verified on the dev DB** (docker `beljot-postgres-1`, port 5433): `make migrate` → `migrate down 1` → `migrate up`. All six columns present with the intended types and CHECKs; `down` drops all six; the re-run `up` reproduces the same backfilled values (tiny weight deltas only, from the extra seconds of decay — which is the correct behaviour). Spot-checked user 1: `C=15.2839, A=0.7707 → 100×19.284/23.367 = 82.5 → 83`, matching the Go formula.
- **Two arithmetic slips in the story's worked-example table.** The table lists `26` for `(C=10, A=10)` and `96` for `(C=20, A=0.06)`. The normative formula gives `1400/55 = 25.45 → 25` and `2400/25.24 = 95.09 → 95`. Seven of the nine rows match the formula exactly, and both slipped rows still land in the tier the story claims (Problematic / Exemplary), so the formula was treated as authoritative and the table tests use the arithmetically correct values. Documented inline in `honor_test.go`.
- **Two of my own test expectations were wrong before they were right**, and the corrections are worth recording because they are counter-intuitive: (1) decay does NOT universally "forgive" — it pulls every inactive player toward the 80 prior, so an above-prior player (83) DROPS to 81 after a year idle. `TestHonorScore_Decay` now pins both directions. (2) `DecayFactor` at 365 days is `0.0601`, not `0.0596`.
- **`TestAbandonment_ChargesOnlyTheExpiredSeat` first failed on the ORDERING assertion, not the honor one:** with zero team scores `computeXPAwards` returns all-zero deltas, so no `event:xp_awarded` fires and "honor must follow xp" had nothing to follow. Fixed by seeding non-zero team scores, mirroring `TestAbandonment_AwardsXP`.
- **DB-backed repo tests hit `varchar(20)`** on `users.username` — fixture names must stay under 20 characters.
- **Pre-existing `gofmt -l` hit** on `server/internal/auth/profile_identity_handler_test.go` (untouched by this story; confirmed unmodified in `git status`). It does not block CI — `make lint` runs `golangci-lint run ./...`, which has no gofmt linter enabled. Recorded in `deferred-work.md`.

### Completion Notes List

**Where the honor surface went (Task 9 asked for an explicit choice):** a full-width `HonorPanel` section between `StreakCallout` and `StatsGrid` in `ProfilePage.tsx`, NOT the reserved slot in `IdentityHero.tsx:201-204`. The surface carries a score, a tier badge, a meter, a trend indicator and two count tiles — that needs the width of a section with its own `SectionHeader`, and the hero slot is a narrow column beside the XP bar. `IdentityHero`'s reservation comment is left in place for the prior-season rank surface.

**The denormalization tradeoff, stated as the story asked.** Story 7-2 deliberately chose per-request aggregation over denormalized counters to avoid two-writer drift; 9.7 denormalizes onto `users` anyway. The justification is D3: admin forgiveness and Story 9.8's per-join gate both need mutable stored state that a pure function over `matches` cannot provide. The drift risk is real and is mitigated by (a) writing honor in the same best-effort band as coins and XP, from the same two finalizers, and (b) the 000017 backfill being written as a re-runnable reconciliation recipe. Recorded in `deferred-work.md`.

**Authoritative vs. snapshot, enforced by naming.** The Go field for the `honor_score` column is `User.HonorScoreSnapshot` (explicit `gorm:"column:honor_score"`), so gating on it reads wrong at the call site. Every read path instead goes through `user.NewHonorSnapshot(...)`, which recomputes from three storage columns of a row already in hand — no extra query, never stale. The distinction is written into the migration header AND the struct field, as D3 demanded.

**No per-match honor stash exists, so there is nothing to reset in the two finalizer handlers.** The 9.5 review caught `match_abandoned` failing to reset what `match_end` reset; honor sidesteps that class of bug entirely by writing only to `authStore.user` (navigation-surviving) and having no end-of-match dialog. A comment in `useWsDispatch.ts` records that a future honor flourish MUST reset in both handlers, since the abandoning player is exactly the one who gets no follow-up event.

**All six drift-gate touchpoints landed in this commit** (the 9.5 review's "project's #1 WS rule" finding): `ws/events.go` const+struct, `wsEvents.ts` const+interface, `events_contract_test.go` cases row, `testdata/events/honor_updated.json` golden (generated with `UPDATE_GOLDENS=1`), `wsEvents.schemas.ts` strict schema + `MutualExtends` witness registered in `_conformanceWitnesses`, and `wsEvents.contract.test.ts` golden+schema+cases row. Both contract tests are green.

**`honorTier` is typed as a plain `z.string()`, not a union of the five tokens.** Deliberate: a future server-side tier retune must not hard-fail a stale client bundle (deferred item D142's failure mode). `normalizeHonorTier()` falls back to the score's own band for an unknown token, and both the panel and the TopBar chip are tested against that path.

**Column widths follow the 9.5 review's lesson.** The raw lifetime totals are `BIGINT` (matching the 64-bit Go `int` accumulating into them — the exact trap that forced `total_xp` INTEGER→BIGINT in review), the decayed weights are `NUMERIC(14,6)` rather than a float because binary drift in an access-gating trust signal is unacceptable, and `honor_score` is `SMALLINT` because it is bounded 0-100.

**Rounding was decided explicitly rather than inherited.** `math.Round` (half away from zero) in Go, matching Postgres `ROUND(numeric)` in the backfill, so the migration and the runtime can never disagree by one. `honor_test.go` pins non-identity decay factors and an exactness proof that decay-forward-then-add equals the from-scratch weighted sum — closing out the 9.5 review's "test a non-identity float from day one" note for honor's own float path.

**Fixture blast radius (Task 12).** Adding three required fields to `User` broke ~20 fixtures across 14 test files. Rather than patching each, `test-utils.tsx` now exports `makeUser(overrides)` and every one of those fixtures routes through it, so the next additive field is a one-line change. Three of the fixed files (`AppLayout.test.tsx`, `LanguageSelector.test.tsx`, `authStore.test.ts`) were ALREADY type-broken from Story 9.5's `totalXp`/`level` — fixed here too, since they were the same defect. The known `RoomDetail.returnedUserIds` noise in `MatchmakingPage.test.tsx` / `RoomPage.bots.test.tsx` was left alone per the task instruction.

**Scope held.** No gating, no `min_honor`, no `allow_new_players`, no `event:honor_eject` (Story 9.8). No cross-player profile visibility (Epic 11 / 11.3). No admin endpoint or UI (D7). No `rage_quits` / `timeout_abandons` columns, constants or code paths (D1). `reconcile.go` was confirmed honor-free by construction and is pinned by an explicit test.

**Gates:** `go vet ./...` clean, `golangci-lint run ./...` clean, `go test ./...` all packages ok (incl. DB-backed repo tests against the dev DB on 5433), `npx tsc -p tsconfig.build.json --noEmit` clean, `npx eslint .` clean, `npx prettier --check .` clean, `npx vitest run` 101 files / 1041 tests passed, `i18n.parity.test.ts` green. `make lint` and `make test` both exit 0.

### File List

**New — server**

- `server/migrations/000017_add_honor_to_users.up.sql`
- `server/migrations/000017_add_honor_to_users.down.sql`
- `server/internal/user/honor.go`
- `server/internal/user/honor_test.go`
- `server/internal/user/honor_service.go`
- `server/internal/user/honor_service_test.go`
- `server/internal/user/honor_repo_test.go`
- `server/internal/match/honor_record.go`
- `server/internal/match/honor_record_test.go`
- `server/internal/match/honor_wiring_test.go`
- `server/internal/ws/testdata/events/honor_updated.json`

**New — client**

- `client/src/shared/lib/honor.ts`
- `client/src/shared/lib/honor.test.ts`
- `client/src/features/profile/components/HonorPanel.tsx`
- `client/src/features/profile/components/HonorPanel.test.tsx`

**Modified — server**

- `server/cmd/api/main.go`
- `server/internal/auth/handler.go`
- `server/internal/auth/handler_test.go`
- `server/internal/chat/handler_test.go`
- `server/internal/lobby/lobby_test.go`
- `server/internal/match/gorm_repo.go`
- `server/internal/match/gorm_repo_test.go`
- `server/internal/match/live_match.go`
- `server/internal/match/manager_test.go`
- `server/internal/match/matchend_test.go`
- `server/internal/match/model.go`
- `server/internal/match/reconcile.go`
- `server/internal/match/reconnect.go`
- `server/internal/match/repository.go`
- `server/internal/user/gorm_repo.go`
- `server/internal/user/handler.go`
- `server/internal/user/handler_test.go`
- `server/internal/user/model.go`
- `server/internal/user/repository.go`
- `server/internal/user/xp_service_test.go`
- `server/internal/ws/events.go`
- `server/internal/ws/events_contract_test.go`

**Modified — client**

- `client/src/features/lobby/LobbyPage.test.tsx`
- `client/src/features/lobby/MatchmakingPage.test.tsx`
- `client/src/features/match/MatchPage.test.tsx`
- `client/src/features/profile/ProfilePage.tsx`
- `client/src/features/profile/ProfilePage.test.tsx`
- `client/src/features/profile/components/EditableUsername.test.tsx`
- `client/src/features/room/CreateRoomModal.test.tsx`
- `client/src/features/room/RoomPage.bots.test.tsx`
- `client/src/features/room/RoomPage.diamond.test.tsx`
- `client/src/features/room/RoomPage.locale.test.tsx`
- `client/src/features/room/RoomPage.test.tsx`
- `client/src/features/auth/LoginPage.test.tsx`
- `client/src/features/auth/RegisterPage.test.tsx`
- `client/src/features/profile/components/IdentityHero.tsx`
- `client/src/shared/api/auth.ts`
- `client/src/shared/api/axiosClient.ts`
- `client/src/shared/api/profile.ts`
- `client/src/shared/components/AppLayout.test.tsx`
- `client/src/shared/components/LanguageSelector.test.tsx`
- `client/src/shared/components/TopBar.tsx`
- `client/src/shared/components/TopBar.test.tsx`
- `client/src/shared/hooks/useAuth.ts`
- `client/src/shared/hooks/mutations/useAuth.ts`
- `client/src/shared/hooks/useWsDispatch.ts`
- `client/src/shared/hooks/useWsDispatch.test.ts`
- `client/src/shared/i18n/en.json`
- `client/src/shared/i18n/sr.json`
- `client/src/shared/i18n/mk.json`
- `client/src/shared/i18n/hr.json`
- `client/src/shared/stores/authStore.test.ts`
- `client/src/shared/types/apiTypes.ts`
- `client/src/shared/types/wsEvents.ts`
- `client/src/shared/types/wsEvents.schemas.ts`
- `client/src/shared/types/wsEvents.contract.test.ts`
- `client/src/test-utils.tsx`

**Modified — docs**

- `_bmad-output/implementation-artifacts/deferred-work.md`
- `_bmad-output/implementation-artifacts/sprint-status.yaml`

### Review Findings

Code review 2026-07-29 (bmad-code-review, 3-layer adversarial: Blind Hunter + Edge Case Hunter + Acceptance Auditor, all three green on first run). 22 raw findings normalized to 16 unique after 6 merges. Zero dismissed at triage — each layer self-filtered its cleared suspicions (~30 reported as chased-and-cleared, incl. lock ordering vs `AddXP`/`ChargeStakes`/`ApplySettlement`, Go `math.Round` vs Postgres `ROUND(numeric)` on a 17-point grid, the NUMERIC(14,6)↔float64 round-trip, the decay-forward exactness claim, all six drift-gate touchpoints verified individually 6/6, and every Go zero-value path on the TS side).

> **SUPERSEDED IN PART BY PASS 2.** Decision 1 below ("skip absent seats") was replaced on the same day — review pass 2 found it was only representable in the live write path and that abstaining froze a serial second-quitter at the 80 prior as a permanent New Player. The rule is now "charge every absent seat". See "Review Findings — pass 2" further down. Decisions 2 and 3 stand as written.

**Decisions resolved 2026-07-29 (PO: Emilijan) — all three became patches**

1. **Absent seat → skip entirely.** No honor event for a seat that is disconnected at match end but whose own window never expired. Chosen over charging them `abandoned` because that would create a second abandonment trigger, which `spec-abandonment-per-player-results.md` freezes. Removes both the perverse reward and the quit-second incentive.
2. **Trend → window vs preceding window.** Last 20 vs matches 21-40, so both sides carry the same sample size and therefore the same Beta(4,1) prior drag and are genuinely comparable. Avoid `OFFSET` (deferred item D82); use a keyset or a second bounded window instead.
3. **`isNewPlayer` → floor on `completed + abandoned`.** Suppress below 5 matches of EXPERIENCE rather than 5 successes, so a 0-completed/20-abandoned account's score becomes visible while a genuine first-timer stays protected. This is a deliberate override of AC2/AC6's literal `honorCompletedTotal < 5`, recorded here in the same spirit as D1-D3.

- [x] [Review][Decision→Patch] A seat that is still disconnected when ANOTHER seat's timer fires is credited a completion, so an absent player's honor RISES [server/internal/match/honor_record.go:41] — `handleConcurrentDisconnectLocked` (reconnect.go:280-283) opens a fresh full per-seat window + its own timer for every subsequent drop, so 2+ overlapping windows is a first-class state, not a race artifact. `computeHonorEvents` decides purely on seat index, so whichever timer fires first charges that seat `abandoned` and gives every other human seat — including one who walked out 5s later and never returned — `completed +1` weight and `completed_total++`. Quitting second is therefore strictly better than quitting first, which is a gameable bypass of the very signal Story 9.8 gates room access on; worst case (all four drop) credits three absent players. `gs.Players[seat].Connected` is already in the snapshot the caller walks (reconnect.go:565-567 reads `IsBot` in the same loop shape) and is never consulted. NOTE: the current behavior is deliberate and reasoned — `TestComputeHonorEvents_ConcurrentDoubleDisconnect` (honor_record_test.go:130-143) pins it with the rationale "the match ended out from under them", so any fix must update that test and its comment. Options: (a) skip absent-but-unexpired seats entirely (no honor event — spec-compliant, since charging them `abandoned` would create a second abandonment trigger, which `spec-abandonment-per-player-results.md` freezes); (b) charge every disconnected seat `abandoned` (needs PO signoff against that freeze); (c) keep as-is and accept the incentive.
- [x] [Review][Decision] The trend indicator is systematically inverted: a 20-match-capped window score is compared against an uncapped lifetime score [server/internal/user/handler.go:263] — `HonorScoreForCounts(20, 0)` maxes out at `100×24/25 = 96` because the Beta(4,1) prior's 4 pseudo-completions are a fixed drag at n=20, while the lifetime side is computed from an unbounded decayed weight and reaches 98-100. The two numbers are not on the same scale by construction. Crossover is a decayed `C ≈ 45` (~45 completions inside one 90-day half-life — six weeks of daily play), after which a player who has NEVER abandoned a match permanently renders `TREND_COLOR.down`, a `TrendingDown` icon, the word "Slipping" and −2/−3/−4. Inversely, 20 clean matches then a year idle renders "Improving +12". `honorTrendThreshold = 2` cannot absorb a structural gap. `TestGetProfile_IncludesHonor` picks the one pair where the comparison happens to work (83 vs 96 → +13 up), so no test covers a lifetime score above the window cap. Options: (a) window-vs-window (last 20 vs the preceding 20 — same sample size, same prior drag, genuinely comparable; needs a second query or an OFFSET, and D82 warns about OFFSET); (b) compare undecayed raw lifetime counts instead (still unbounded, only narrows the gap); (c) compare abandonment RATE rather than two smoothed scores; (d) drop the trend from 9.7 and re-scope it.
- [x] [Review][Decision] `isNewPlayer` floors on completions only, so a pure abandoner hides behind the "New Player" chip indefinitely [server/internal/user/honor.go:235] — `IsNewPlayer(completedTotal)` never consults `honor_abandoned_total`. A player with 0 completed / 20 abandoned has a real score of 5 ("problematic") and both surfaces (TopBar.tsx:254, HonorPanel.tsx:119) suppress the number and render the newcomer chip, identical to a genuine first-timer. The worst possible actor stays behind that chip forever simply by never finishing a fifth match — and this is exactly the population the feature exists to surface, with 9.8's gate reading `isNewPlayer` off the same envelope. `TestIsNewPlayer` only varies `completedTotal`. AC-mandated as written ("`isNewPlayer = honorCompletedTotal < 5`"), so changing it is a spec deviation and the PO's call. Options: (a) floor on `completed + abandoned < 5` (experience, not just success); (b) suppress only when BOTH totals are below the floor; (c) keep as-is and let 9.8's `allow_new_players` toggle carry the risk.

**Patches**

- [x] [Review][Patch] Honor chip is `display:none` below the `sm` breakpoint — AC7 unmet on phones, with no mobile treatment at all [client/src/shared/components/TopBar.tsx:236]
- [x] [Review][Patch] A missing/NaN honor score renders as the WORST tier ("Problematic", `var(--danger)`) on the unvalidated HTTP path; `HONOR_PRIOR_SCORE` documents a render fallback that was never wired (zero production consumers) [client/src/shared/lib/honor.ts:32,39]
- [x] [Review][Patch] Two tiles labelled "Abandoned" with contradictory values on the same profile page after any boot reconcile — `GetStatsForUser` counts `abandoned_by IS NULL` rows for every participant, honor excludes them [client/src/features/profile/ProfilePage.tsx:87-109]
- [x] [Review][Patch] Dead i18n key `profile.honor.topBarLabel` shipped in all four locales, and the chip has no accessible "Honor" label — the only "Honor" text is a `title` on a non-interactive div [client/src/shared/i18n/en.json:92]
- [x] [Review][Patch] `honor_decayed_at` is rewritten BACKWARDS when the stored stamp is in the future, silently over-decaying every later write [server/internal/user/gorm_repo.go:290]
- [x] [Review][Patch] TopBar comment claims the chip "shows a dash instead of a number" for a newcomer; the code renders the words "New Player" [client/src/shared/components/TopBar.tsx:231-233]
- [x] [Review][Patch] `IdentityHero`'s reservation comment still calls honor "not-yet-built" one commit after honor shipped — the exact drift Task 14 exists to clean up [client/src/features/profile/components/IdentityHero.tsx:201-204]
- [x] [Review][Patch] Eight auth-response mocks were never extended with the three new required envelope fields, so login/register tests inject `honorScore: undefined` into the store; `tsc` structurally cannot catch it because the mocks are bare `vi.fn()` [client/src/features/auth/LoginPage.test.tsx:126,206,253,328; RegisterPage.test.tsx:180,214,381,397]
- [x] [Review][Patch] `TopBar.test.tsx` still hand-builds an inline `User` literal instead of routing through `makeUser()`, contradicting the Completion Note's "every one of those fixtures routes through it" — in the one file most likely to break on the next additive `User` field [client/src/shared/components/TopBar.test.tsx:30-46]
- [x] [Review][Patch] The worked-example table still carries the two arithmetically wrong rows (26 → 25, 96 → 95), so AC2's "all worked examples pass as table-driven test cases" is unsatisfiable as written — the correction lives only in a Debug Log paragraph and a test comment [_bmad-output/implementation-artifacts/9-7-honor-score-system.md:125-126]
- [x] [Review][Patch] Trend query uses `abandoned_by IS DISTINCT FROM ?` instead of the canonical `abandoned_by IS NOT NULL AND abandoned_by <> ?` gate — semantically equivalent here (verified), but a future grep for the canonical string will not find this site, and the neighbouring `GetStatsForUser` does use the canonical form [server/internal/match/gorm_repo.go:119]

### Review Findings — pass 2 (2026-07-29)

Second full 3-layer adversarial pass over the patched diff (74 files, 5896 diff lines). The Acceptance Auditor found **no functional defect** in the three amendments or in any of AC1-AC10, verified all six drift-gate touchpoints individually 6/6, and spot-checked all eleven pass-1 patches as genuinely applied and correct. The blind layers, however, found that **Amendment 1 is only half-implemented** — see the decision below. Pass 2 also caught a stray `zz_probe_review_test.go` that a pass-1 subagent left in `server/internal/user/` and that would have shipped; it has been deleted (it used `getTestDB`, so it left no rows in the dev DB).

**Decisions needed**

- [x] [Review][Decision] Amendment 1's withheld-completion rule exists ONLY in the live write path; the trend query and the 000017 backfill both still credit the absent seat, and the "neutral" bucket freezes a serial second-quitter at the 80 prior as a permanent New Player [server/internal/match/honor_record.go:59 vs server/internal/match/gorm_repo.go:130 vs server/migrations/000017_add_honor_to_users.up.sql:101,118] — Found independently by BOTH blind layers, one of them verified empirically against the dev DB. `matches` records a single `abandoned_by` and no per-seat presence, so neither read path can reproduce the skip: the trend window counts the absent seat as `completed` on every profile load, and the backfill grants it a completion at deploy — including the re-run the migration header advertises as a reconciliation recipe. Worse, because the skip writes NOTHING, `honor_completed_total + honor_abandoned_total` never increments, so a repeat second-quitter stays at exactly the 80 prior with `isNewPlayer = true` **forever** — and both fields ride the auth envelope Story 9.8's join gate reads. The same missing distinction also costs an honest player: a mobile blip a second before another seat's window expires reads `connected == false`, and that player silently loses the completion they had earned before this fix. NOTE for the record: pass 1 rejected "charge every disconnected seat `abandoned`" as creating a second abandonment trigger against `spec-abandonment-per-player-results.md`. On re-reading, that freeze governs what ENDS a match and what lands in `abandoned_by` — not which honor bucket a seat falls into — so that option was probably ruled out too cautiously. Options: (a) charge the absent seat `abandoned` after all; (b) keep it neutral but count it toward the experience floor (needs a third raw total — migration 000017 is unshipped, so amending it is allowed, per the 9.5 `total_xp` INTEGER→BIGINT precedent); (c) record per-seat presence on the match row so all three paths can agree; (d) accept that the stored weights and the `matches`-derived trend are two different estimators, and document it.
- [x] [Review][Decision] `ResetHonor` zeroes the raw totals, so pardoning a veteran relabels them a "New Player" — and the migration's own advertised re-runnable backfill destroys every pardon [server/internal/user/gorm_repo.go:350-351; server/migrations/000017_add_honor_to_users.up.sql:42-44 vs :29-40] — `IsNewPlayer(0, 0)` is true, so a forgiven 500-match veteran is shown the newcomer chip and "Play 5 matches to earn an honor score" while the untouched `StatsGrid` still displays their real history — reintroducing exactly the contradiction the pass-1 label rescope removed. Separately, the up-migration header advertises its backfill as "re-runnable as a full reconciliation recipe", but that `UPDATE … FROM agg` writes absolute values derived purely from `matches`, so re-running it silently reverts every `ResetHonor` pardon. The `.down.sql` header warns that forgiveness is unrecoverable; the up header, which is the one recommending the re-run, does not. This defeats the stated reason honor is stored rather than derived (`up.sql:14-16`). Options: preserve experience across a reset (zero the weights and the abandoned total, keep the completed total); add a forgiveness marker the backfill excludes; or drop the reconciliation-recipe claim and say plainly that the backfill is deploy-only.

**Patches**

- [x] [Review][Patch] Nine authoritative comments still document pre-amendment semantics, and two of them read as directives a Story 9.8 implementer would follow straight back into the bug [server/internal/user/model.go:50-51; server/migrations/000017_add_honor_to_users.up.sql:64; server/internal/user/honor.go:62-64; server/internal/user/handler.go:48-49; server/internal/ws/events.go:111; server/internal/match/live_match.go:103; client/src/shared/types/wsEvents.ts:232; client/src/features/profile/components/HonorPanel.tsx:79; server/internal/user/honor_repo_test.go:56]
- [x] [Review][Patch] The spec still contradicts itself in five places after the amendments: the normative constant table still defines the floor as completions-only, Task 5 names a method that no longer exists, Task 4 omits the `connected` parameter that IS Amendment 1, open question #4 asks the PO to confirm a rule they already overruled, and the File List omits three files pass-1 patches modified [_bmad-output/implementation-artifacts/9-7-honor-score-system.md:99,291,300,482,572-605]
- [x] [Review][Patch] Version-skew hardening covers `honorScore` only — `isNewPlayer`, the two counts and the trend delta are raw on the same untyped HTTP responses, so an absent envelope renders a confident 80/"Fair" for every account including the newcomers the flag exists to suppress [client/src/shared/components/TopBar.tsx:252,271; client/src/features/profile/components/HonorPanel.tsx:63]
- [x] [Review][Patch] `trendTooltip` hardcodes "20" in all four locales, defeating the exported `HonorTrendWindow()` constant added to prevent exactly that drift [client/src/shared/i18n/en.json:89]
- [x] [Review][Patch] `HonorScore` guards NaN but not ±Inf, and the Inf path lands on the WORST tier via `int(NaN)` → negative → clamp-to-0, asymmetric with the NaN case that is explicitly tested to degrade to the 80 prior [server/internal/user/honor.go:178-189]
- [x] [Review][Patch] The 000017 backfill's decay weight is not clamped at 1.0 for a future `completed_at`, so app-host clock skew ahead of Postgres `NOW()` backfills weights ABOVE the real match count — the exact case `DecayFactor` guards on the Go side in a file that claims the two agree exactly [server/migrations/000017_add_honor_to_users.up.sql:102]
- [x] [Review][Patch] The pass-1 forward clamp on `honor_decayed_at` has no ceiling, so one far-future stamp freezes decay indefinitely with nothing logged — it traded a double-decay bug for an unbounded no-decay window [server/internal/user/gorm_repo.go:299-301]
- [x] [Review][Patch] The TopBar chip's own width-budget comment assumes an icon plus two digits below `sm`, but the New Player branch renders "New Player" / "Нов играч" — the widest content in the row — for every account's first five matches, in a non-wrapping flex bar with no `min-w-0` or overflow handling [client/src/shared/components/TopBar.tsx:272-274]
- [x] [Review][Patch] Tautological assertion: the natural-end wiring test re-derives the expected tier from the same `user.HonorTier` the stub used to build it, so it passes for any implementation including an inverted one [server/internal/match/honor_wiring_test.go:141]
- [x] [Review][Patch] The presence snapshot's comment claims `connected == false` means "sitting inside their own still-open window", but the converse does not hold — state the precondition honestly [server/internal/match/reconnect.go:570-574]

**Deferred**

- [x] [Review][Defer] The presence snapshot fails OPEN: `Players[i].Connected` is only maintained for drops observed in four phases, so a socket reaped during dealing / trick_resolving / hand_scoring leaves the seat reading connected for the rest of the match [server/internal/match/reconnect.go:105-108] — deferred, pre-existing: the `[F3]` phase gate is deliberate and predates honor; widening it changes disconnect/pause behaviour well beyond this story. Consequence here is lenient (the seat is credited, i.e. the pre-fix behaviour), and transient phases are brief enough that deliberately targeting them is impractical.
- [x] [Review][Defer] No index on `matches.completed_at`, so the trend query materialises and sorts the user's entire participation set before its `LIMIT 40` applies [server/internal/match/gorm_repo.go:128-152] — deferred, pre-existing: identical shape to `GetMatchesForUser`, which has the same missing index. Flagged only because the story's prose implies the fetch is bounded end-to-end.
- [x] [Review][Defer] Absent players never receive their own `event:honor_updated`, and no path re-syncs it [server/internal/match/reconnect.go:623] — deferred, pre-existing: `Hub.SendToUser` is a silent no-op with no queue, and coins + XP already behave identically from the same loop.
- [x] [Review][Defer] Honor commits in its own transaction BEFORE the match row, so a crash in that window leaves counters the documented reconciliation recipe silently reverts [server/internal/match/live_match.go:1168] — deferred, pre-existing: the broadcast-before-persist ordering is an open 8.5-1 item, and settlement + XP sit in the same band.

## Change Log

| Date | Change |
| --- | --- |
| 2026-07-29 | Story 9.7 code-review PASS 2 (bmad-code-review, second full 3-layer adversarial pass over the patched diff). The Acceptance Auditor found NO functional defect in the three pass-1 amendments or in AC1-AC10, re-verified all six drift-gate touchpoints 6/6, hand-checked the trend query's 13-placeholder binding order, and confirmed all eleven pass-1 patches. The blind layers independently found pass 1's Amendment 1 was HALF-IMPLEMENTED, which drove two further PO decisions. (1) **The absent-seat rule is now PRESENCE, charging `abandoned`** — replacing pass 1's abstain. Abstaining wrote nothing, so the raw totals never incremented and a repeat second-quitter sat at the 80 prior as a permanent New Player, which 9.8's gate reads; it also cost honest players a completion on a socket blip; and it was unrepresentable on the trend query and the 000017 backfill, which have no per-seat presence in `matches` and kept crediting the seat. Pass 1 had excluded this option believing it created a second abandonment trigger — that freeze actually governs only what ends a match and what lands in `abandoned_by`, neither of which changes. (2) **`ResetHonor` now preserves `honor_completed_total`** — zeroing it turned a pardoned veteran into a "New Player" — and the migration header no longer advertises its backfill as re-runnable, because that `UPDATE ... FROM agg` silently reverts every pardon. Ten further patches: nine pre-amendment comments corrected (two of which, `model.go:50` "always compare against this" and the migration column doc, would have walked 9.8's implementer straight back into the closed bypass); five spec self-contradictions fixed (normative constant table, Task 4 signature, Task 5 method name, resolved open question, File List +3); version-skew guards extended from `honorScore` to `isNewPlayer`/counts/delta via `honorCountOrZero` + `honorIsNewPlayer` (absent `isNewPlayer` was falsy and rendered a confident 80/"Fair" for newcomers); `trendTooltip` now interpolates `{{window}}` from a mirrored `HONOR_TREND_WINDOW` in all four locales instead of hardcoding 20; `HonorScore` guards ±Inf so it lands on the prior rather than the worst tier; the backfill weight is `LEAST(1, ...)`-clamped against app-host-ahead-of-Postgres skew; the decay clamp gained a `honorMaxClockSkew` ceiling with a `slog.Warn` so one absurd stamp cannot freeze decay forever; the TopBar New Player words go sr-only below `sm` (they are the chip's widest content, on every account's first five matches, in a non-wrapping row); and a tautological wiring assertion was replaced with literals. Two items deferred (presence snapshot fails open for drops in transient phases — pre-existing `[F3]` phase gate; no index on `matches.completed_at` — precedent-consistent with `GetMatchesForUser`), one dismissed. Pass 2 also deleted `server/internal/user/zz_probe_review_test.go`, scaffolding a PASS-1 subagent left behind that would have shipped. Gates: `gofmt` clean except the pre-existing `profile_identity_handler_test.go`, `go vet` clean, `golangci-lint v1.64.8` clean, `go test ./...` all 18 packages ok with DB-backed honor tests passing not skipping, `tsc` clean, `vitest` 101 files / **1053 tests**, `eslint` clean, `prettier` clean, i18n 19 keys × 4 with parity + mk all-Cyrillic + no em dash in mk/sr/hr. Status stays `done`. |
| 2026-07-29 | Story 9.7 code-reviewed (bmad-code-review, 3-layer adversarial). 22 raw findings → 16 unique; 3 decisions resolved by PO and 11 patches applied, 2 deferred, 0 dismissed. Three ACs amended in place: AC3's concurrent-double-disconnect clause (an absent-but-unexpired seat now gets NO event instead of a `completed` credit that raised a quitter's honor and made quitting second strictly better), the Trend definition (two equal-size adjacent 20-match windows instead of window-vs-lifetime, which capped the window at 96 against a lifetime of 98-100 and made flawless active players read "Slipping" forever), and the New Player floor (`completed + abandoned < 5` instead of completions only, which hid a 0/20 abandoner behind the newcomer chip). Also: `honor_decayed_at` clamped forward so a future stamp cannot roll back and double-decay; the trend query rewritten to one `ROW_NUMBER`-split bounded fetch using the canonical viewer gate verbatim; TopBar honor chip made visible at phone widths (was `sm:flex`-only, leaving AC7 unmet on mobile) and given an accessible name via the previously-dead `topBarLabel` key; `honorScoreOrPrior` wired so an absent HTTP score renders the 80 prior instead of falling through to the worst tier; honor count labels rescoped ("Finished by you" / "Abandoned by you") so they stop contradicting the StatsGrid tile after a boot reconcile; the two stale worked-example rows corrected in the AC table itself; 8 auth-response mocks given the three required envelope fields; `TopBar.test.tsx` routed through `makeUser`; 2 stale comments fixed. Gates re-run: `go vet` clean, `golangci-lint v1.64.8` clean, `gofmt` clean except the pre-existing `profile_identity_handler_test.go`, `go test ./...` all packages ok with DB-backed honor tests passing (not skipping), `tsc` clean, `vitest` 101 files / 1048 tests, `eslint` clean, `prettier` clean. Status → done. |
| 2026-07-29 | Story 9.7 implemented (dev-story). Migration 000017 adds six honor columns to `users` with a backfill from `matches` (down/up roundtrip verified on the dev DB). Pure decay/score/tier math in `user/honor.go`; atomic decay-forward-then-add persistence in `user/gorm_repo.go` under the shared ascending-userID `FOR UPDATE` lock discipline; `ResetHonor` forgiveness hook. `HonorRecorder` declared in `match`, satisfied by `user.HonorService`, injected from `main.go`; honor recorded from both finalizers (natural end / surrender → every seat `completed`; abandonment → only the expired seat charged). New `event:honor_updated` with all six drift-gate touchpoints. Profile DTO gains seven public-safe honor fields incl. the 20-match trend; auth envelope gains score/tier/isNewPlayer. New `HonorPanel` profile section, TopBar honor chip, and `profile.honor.*` i18n ×4 (mk all-Cyrillic; no em dash in mk/sr/hr). `makeUser()` test fixture centralizes the `User` blast radius. Status → review. |
