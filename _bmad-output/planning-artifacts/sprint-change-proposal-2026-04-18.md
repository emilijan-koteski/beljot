# Sprint Change Proposal — 2026-04-18

**Workflow:** `bmad-correct-course`
**User:** Emilijan
**Status:** Approved & Applied

## 1. Issue Summary

The founding product vision underwent a significant pivot across Phase 2 and all downstream phases. Triggers:

- **ELO / ranked competitive mode rejected** as the organising frame for progression. The user judged that ELO-based matchmaking gates (Level 5 unlock, placement matches, scaled penalties) would create a poor experience for a closed-circle app where room-based play is primary.
- **Coin-based economy replaces ranked** as the skill/stakes signal. Buy-in per room, per-match pot settlement, daily login rewards, and streak bonuses give every game tangible meaning without tying progression to a rigid ladder.
- **Honor system introduced** as the new trust signal. Public score based on match-completion behaviour (rage-quits, timeout abandons, unresolved disconnects) — visible on every profile, optionally gateable per room.
- **Phase sequencing changed:** Croatian variant, 501 mode, and in-app rules reference pushed from Phase 2 → Phase 3 (not foundational to the economy launch). Social login pushed from Phase 3 → Phase 4. Additional languages pulled forward from Phase 3 → Phase 2. A new Phase 5 was created to host Spectator / Achievements / Cosmetics / Tournaments.
- **New capabilities added** that had no prior slot: room owner pre-game kick + seat swap, and the coin economy / honor system.

Triggering story: None — this is a pre-implementation scope reshape ahead of Phase 2 development. No in-flight work was invalidated.

**Issue category:** Strategic pivot / scope redefinition.

## 2. Impact Analysis

### Epic Impact

| Epic (old)                                           | Status        | Destination                                                                                                                                                           |
| ---------------------------------------------------- | ------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Epic 1–7                                             | **Unchanged** | Phase 1, stays as-is                                                                                                                                                  |
| Epic 8 (Croatian + 501 + Rules + Surrender + Emotes) | **Split**     | 8.4 Surrender + 8.5 Emotes + NEW Kick/Seat → new Epic 8 (Phase 2). 8.1 Croatian + 8.2 501 + 8.3 Rules → new Epic 12 (Phase 3)                                         |
| Epic 9 (Progression + Ranked)                        | **Redefined** | XP/Level kept (no gating), Ranked queue / Placement / Scaled ELO penalty retired. New Epic 9 = Coin Economy + XP/Level + Honor. Seasonal rank → new Epic 13 (Phase 4) |
| Epic 10 (Social & Friend)                            | **Split**     | Additional Languages → new Epic 10 (Phase 2). Player Search + Friends + Public Profiles → new Epic 11 (Phase 3). Social Login → new Epic 14 (Phase 4)                 |
| Epic 11 (Spectator + Achievements + Cosmetics)       | **Moved**     | All three → new Epic 16 (Phase 5)                                                                                                                                     |
| Epic 12 (Tournaments + Mobile)                       | **Split**     | Tournaments → new Epic 16 (Phase 5). Mobile → new Epic 15 (Phase 4)                                                                                                   |

### Story Impact

- **No stories in Epics 1–6 affected.** Epic 6.2 (match-scoped chat) in review status was explicitly left untouched.
- **3 old stories retired** (ranked queue, placement matches, ELO-penalty career stats).
- **8 new stories created** in new Epic 8 (1 new), new Epic 9 (5 new economy/honor), new Epic 13 (1 new season rollover), new Epic 9 (1 honor rebuild).
- **14 old stories preserved verbatim** under new epic/story numbering (Croatian, 501, rules ref, surrender, emotes, social login, player search, friends, additional languages, spectator, achievements, cosmetics, tournaments, mobile).
- **4 old stories partially preserved** (XP/Level, rank tiers, leaderboard, public profiles) — ACs materially diverged, so old story files (had they existed) would have been stale. Current state: no old story files existed for Epic 8+, so no stale content is present.

### Artifact Impact

| Artifact                                 | Changes                                                                                                                                                                                                                                                                                                                                                                                      |
| ---------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| PRD (`prd.md`)                           | Phase 1–4 block rewritten to Phase 1–5. Goals "Soft Opening" phrasing updated (ranked → coin/honor). Croatian-variant Phase reference corrected (2 → 3). NFR capacity labels updated (Phase 2 → Phase 2–3 / beyond Phase 3).                                                                                                                                                                 |
| Epics (`epics.md`)                       | Epic List overview replaced (8–12 entries, added 13–16). Epic bodies for 8, 9, 10, 11, 12, 13, 14, 15, 16 fully rewritten. Requirements Inventory: FR34 modified, FR35/36/38 retired, FR37/39/40/42/43 reworded, FR53–FR58 added. NFR8 updated (ELO → SP/coin/honor). NFR12/13 phase labels corrected. Architecture Phase Scoping rewritten. FR Coverage Map remapped to new epic numbering. |
| Sprint Status (`sprint-status.yaml`)     | Epic 8+ block replaced. Epics 1–7 and Epic 6 (review) untouched.                                                                                                                                                                                                                                                                                                                             |
| Story files                              | **No-op:** Epic 8+ story files were never created as discrete artifacts — they existed only as content within `epics.md`. New story files will be generated fresh per the new epic bodies via `/bmad-create-story`.                                                                                                                                                                          |
| Architecture (`architecture.md`)         | **Not updated in this pass** — further review recommended: the persistence layer must add `wallet_balance`, `login_streak_days`, `last_login_at`, `rooms.coin_buy_in`, `rooms.min_honor`, `rooms.allow_new_players`, `users.honor_*` counters, `seasons` + `player_seasons` tables.                                                                                                          |
| UX Design (`ux-design-specification.md`) | **Not updated in this pass** — further review recommended: needs coverage for create-room modal coin-buy-in field, honor score display on profile, RankBanner seasonal tier (SP-based), insolvent-kick modal, honor-eject modal, room owner kick icon + drag-to-swap seat affordance, daily-reward toast.                                                                                    |

### Technical Impact

New server-side systems needed:

- `wallet` domain package (balance, daily reward, streak)
- Coin settlement engine in match lifecycle (stake hold, pot computation, refund paths)
- `honor` calculation hooks in match-end and abandonment handlers
- `seasons` scheduled job (quarterly rollover)
- SP calculation in match settlement
- Room state: `waiting` vs `in_progress` guards around kick/swap-seats

New frontend systems:

- Lobby wallet display + streak indicator
- Create-room modal: coin-buy-in field, min-honor field, allow_new_players toggle
- RoomCard: honor requirement badge, "Veterans only" indicator
- Profile: honor score + tier + raw counts + trend; prior-season rank archive section
- SeatLayout: owner kick icon + drag-to-swap (pre-game only)
- Post-match: `event:coin_settlement` toast, `event:insolvent_kick` modal, `event:honor_eject` modal

## 3. Recommended Approach

**Selected path: Option 1 — Direct Adjustment (+ Option 3 hybrid, MVP scope review)**

- **Direct adjustment** for epic scope restructuring — Epics 1–7 are unchanged and mostly done, so no rollback risk.
- **MVP scope review** applied at the phase level: Phase 2 is now leaner (economy + kick/seat + emotes + surrender + additional languages) and Phases 3–5 were resequenced around user value rather than technical dependencies.
- **No rollback needed** — no in-flight work was invalidated. Epic 6.2 (in review) is unaffected.

Effort estimate: **High** (net new functionality across coin economy, honor, seasonal rank — ~6 weeks of solo-dev time for Phase 2 alone).
Risk level: **Medium** — technical foundation is sound; primary risk is economy-tuning (daily bonus amounts, streak curve, honor weights) requiring live calibration.

Trade-offs considered:

- **Keeping ELO**: rejected — increases UI surface (placement flow, tier reveal, seasonal resets with ELO compression) for a benefit that doesn't match the closed-circle audience.
- **Coin + ELO coexistence**: rejected — two parallel competitive signals would confuse; honor + coin already provide enough differentiation.
- **Phasing coin and honor together vs. coin first**: bundled together in Phase 2 because honor's weight tuning depends on observable match-completion data that only materialises once people are playing matches with real stakes.

## 4. Detailed Change Proposals

All change proposals have been applied to the codebase. Reference artefacts:

- [prd.md](prd.md)
- [epics.md](epics.md)
- [sprint-status.yaml](../implementation-artifacts/sprint-status.yaml)

Key incremental edits applied in sequence:

1. PRD Phase strategy block (Phase 1–4 → Phase 1–5 with full content rewrite)
2. PRD cross-references (Goals, Risk Mitigation, NFR)
3. Epics.md overview (Epic List, entries 8–16)
4. Epics.md body for new Epic 8 (Game Table Enhancements)
5. Epics.md body for new Epic 9 (Player Economy & Progression — 7 stories)
6. Epics.md body for new Epic 10 (Additional Languages)
7. Epics.md body for new Epic 11 (Friends & Public Profiles — honor-enriched)
8. Epics.md bodies for new Epics 12–16 (Variant Expansion, Seasonal Rank & Leaderboard, Social Login, Mobile, Spectator/Achievements/Cosmetics/Tournaments)
9. Epics.md Requirements Inventory (FR34 modified, FR35/36/38 retired, FR37/39/40/42/43 reworded, FR53–58 added)
10. Epics.md NFR8/12/13, Architecture Phase Scoping, FR Coverage Map
11. sprint-status.yaml Epics 8+ block (Epic 6 untouched)

### Key parameter decisions locked

| Parameter                    | Value                                                                                                        |
| ---------------------------- | ------------------------------------------------------------------------------------------------------------ |
| Registration coin grant      | 5 000                                                                                                        |
| Daily login bonus            | 1 000 on day 1, +162/day linear up to 3 100 on day 14                                                        |
| Streak reset                 | On any missed day                                                                                            |
| Room buy-in range            | Min 0, no maximum (owner freeform)                                                                           |
| Default Quick Play stake     | 500 coins, balance-bracketed below 500                                                                       |
| Pot math (normal)            | 4× stake pot; winners split (each net +S); losers net −S                                                     |
| Pot math (abandonment)       | 3× stake pot (teammate refunded); winners split (each net +0.5S); abandoner net −S                           |
| Pot math (surrender)         | Identical to normal loss for surrendering team                                                               |
| Settlement cadence           | Per-match, not per-hand                                                                                      |
| Insolvency check             | Between matches; mid-match balance drops do not eject                                                        |
| Honor formula                | `100 × completed / (completed + 2.0·rage_quits + 1.5·timeout_abandons + 0.3·dc_abandons)`                    |
| Honor "New Player" threshold | < 20 completed matches                                                                                       |
| Honor tiers                  | Exemplary 95+, Trusted 85–94, Fair 70–84, Unreliable 50–69, Problematic < 50                                 |
| SP formula                   | 50 (completion) + 100 (win) + floor(team_game_points/10) + 50 (Capot/instant-win); 0 for abandoners          |
| SP tier thresholds           | Iron 0, Bronze 500, Silver 1 500, Gold 3 000, Platinum 5 500, Diamond 8 500, Master 12 500, Grandmaster 18 000 |
| Season length                | 3 months (quarterly)                                                                                         |
| Season reset                 | Soft — all players start at Iron with 0 SP; prior-season records archived                                    |
| Prior-season archive display | Every season with ≥ 1 game played; zero-game seasons omitted                                                 |
| Level curve                  | Placeholder quadratic: Level N requires `50 × N²` total XP                                                   |
| XP from match                | `floor(team_game_points / 10)` per completed match                                                           |
| Honor-gated rooms            | Optional per-room `min_honor` (0–100); separate `allow_new_players` toggle (default true)                    |
| Room owner kick/seat-swap    | Pre-game only (room status = `waiting`); disabled once `in_progress`                                         |

## 5. Implementation Handoff

### Scope classification

**Moderate** — requires coordinated backlog reorganisation across multiple epics and downstream architecture/UX work, but no fundamental rebuild of foundational systems.

### Routing

| Recipient               | Responsibilities                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                           |
| ----------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| **Bob (Scrum Master)**  | Sprint planning refresh (`/bmad-sprint-planning`) against the rewritten `sprint-status.yaml`. Next-story pick after current 6.2 review completes.                                                                                                                                                                                                                                                                                                                                                                                                                                          |
| **Winston (Architect)** | Architecture update pass for: new `wallet`, `seasons`, `player_seasons` tables; `rooms` column additions (coin_buy_in, min_honor, allow_new_players); `users` column additions (wallet_balance, login_streak_days, last_login_at, total_xp, honor counters); new WS events (`action:room_kick_player`, `action:room_swap_seats`, `event:coin_settlement`, `event:insolvent_kick`, `event:honor_eject`, `event:daily_reward`, `event:room_closed_insolvent`, error codes). Recommended: run `/bmad-create-architecture` to produce an Architecture ADR supplement covering the new domains. |
| **Sally (UX Designer)** | UX updates: create-room modal (new fields), profile honor display, insolvent/honor-eject modals, owner kick icon + drag-swap, daily-reward toast, seasonal RankBanner in SP mode.                                                                                                                                                                                                                                                                                                                                                                                                          |
| **Amelia (Dev)**        | Story implementation starts with new Epic 8 Story 8.1 (Room Owner Pre-Game Controls) after `/bmad-create-story` generates the first spec for it. Coin system (Epic 9) gates economy rollout — suggest implementation order: 9.1 (wallet foundation) → 9.2 (settlement) → 9.3 (insolvency) → 9.5 (XP/level, independent) → 9.4 (Quick Play brackets) → 9.6 (honor) → 9.7 (honor gates).                                                                                                                                                                                                     |

### Pre-development blockers (FR28, FR43)

The partial-XP-on-abandonment formula (FR43) and the timeout-abandon threshold (FR28) still need product tuning before Epic 9 Story 9.5 lands. Recommendation: start Epic 9.1 (wallet) and Epic 8 (kick/surrender/emotes) in parallel while FR28/FR43 constants are decided.

### Success criteria

- Sprint Change Proposal accepted by user (done).
- All artefact changes applied without invalidating in-flight work (done — Epic 6.2 preserved).
- Sprint-status.yaml reflects the new epic structure (done).
- Next `/bmad-create-story` invocation picks up the new epic structure cleanly.
- Epic 1–5 test suites continue to pass (must verify on next dev session — no code changed in this workflow).

### Outstanding follow-ups

- `/bmad-create-architecture` — supplement ADR for new domains (wallet, seasons, honor, economy)
- `/bmad-create-ux-design` — UX for create-room modal additions, honor display, insolvent/honor modals, drag-to-swap, daily-reward toast
- `/bmad-create-story` — generate the 8 net-new story specs listed in §2
- Product decisions on FR28 timeout threshold and FR43 partial-XP formula

---

**Workflow complete, Emilijan!**
