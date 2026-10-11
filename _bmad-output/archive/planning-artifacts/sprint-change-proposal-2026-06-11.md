# Sprint Change Proposal — 2026-06-11

**Workflow:** `bmad-correct-course`
**User:** Emilijan
**Status:** Approved & Applied (changes requested explicitly by user; artifacts updated in this pass, uncommitted)
**Scope:** Two changes in one session — **Change 1:** Story 12.2 (501 mode) moved to Epic 10 as Story 10.2. **Change 2:** new Story 10.3 (Bot Players, new FR59) inserted after 10.2 and before Epic 9.

# Change 1 — 501-Point Match Mode moved to Epic 10

## 1. Issue Summary

Story 12.2 (501-Point Match Mode, FR15) is currently parked in Epic 12 (Variant Expansion, Phase 3) behind Story 12.1 (Croatian Variant Rules Engine). The user wants 501 mode available sooner and requested it be **moved into Epic 10 (in-progress, Phase 2), directly after Story 10.1**, making it the next story in line for development.

**Trigger:** Stakeholder reprioritization request (2026-06-11), not a defect. Direct quote of intent: *"I want story 12.2 501-Point Match Mode to be moved next in line for development instead of phase 3. Make it part of the epic 10 after 10-1."*

**Issue category:** Strategic resequencing / scope move between epics.

**Evidence the move is cheap and safe (codebase readiness):**

- `server/internal/game/scoring.go` — `matchTarget(mode)` already returns 501 for `"501"`; match-end resolution (including the taker-team tiebreaker) is already mode-agnostic.
- `server/internal/room/model.go` — rooms already persist `MatchMode` (default `"1001"`); `GameState` carries it through the engine and test fixtures.
- `server/internal/room/handler.go` — only gap server-side: `validMatchModes` accepts `"1001"` only.
- `client/src/features/room/CreateRoomModal.tsx` — the 501 option is already rendered, currently `disabled: true`.
- `client/src/shared/i18n/{en,sr,mk,hr}.json` — `matchMode501` keys already translated in all four languages.
- `client/src/features/match/components/ScorePanel.tsx` — already takes a `matchTarget` prop (defaults 1001); just needs the room's mode passed through.

All prerequisite stories are **done**: 2-1 (room configuration), 3-6 (match completion), 4-6 (score panel). Story 12.2 has **zero dependency** on Story 12.1 (Croatian variant) — the coupling in Epic 12 was thematic, not technical.

## 2. Impact Analysis

### Checklist Outcomes

| Section | Result |
| --- | --- |
| 1. Trigger & context | [x] Done — stakeholder resequencing request, evidence above |
| 2. Epic impact | [x] Done — Epic 10 grows by one story; Epic 12 shrinks to 12.1 + 12.3 |
| 3. Artifact conflicts | [x] Done — epics.md, prd.md, sprint-status.yaml need edits; architecture & UX need none |
| 4. Path forward | [x] Option 1: Direct Adjustment (effort: Low, risk: Low) |
| 5. Proposal components | [x] Done — this document |
| 6. Final review & handoff | [x] Done — sprint-status updated; next step is create-story 10.2 |

### Epic Impact

| Epic | Status | Change |
| --- | --- | --- |
| Epic 10 (Additional Languages) | **Expanded** | Gains Story 10.2 (501-Point Match Mode), verbatim ACs from old 12.2. Title/goal widened to "Additional Languages & 501 Match Mode". FRs covered: FR45 + FR15. Remains in-progress, Phase 2. |
| Epic 12 (Variant Expansion) | **Reduced** | Loses Story 12.2 and FR15. Remains viable as 12.1 (Croatian variant) + 12.3 (rules reference); both untouched. Story 12.3 keeps its number — no renumbering, so existing references in the readiness report and prior proposals stay valid. The variant tied-hand divergence note (hanging points for Bitola) stays in Epic 12 — it is unrelated to match target. |
| All other epics | Unchanged | Epic 9 stays queued after Epic 10 per the 2026-05-09 promotion. |

### Story Impact

- **1 story moved**: 12.2 → **10.2**, acceptance criteria unchanged. Becomes the next story in the sprint plan (Epic 10 is in-progress with 10-1 done, and the epic-10 block precedes epic-9 in sprint order).
- **0 stories retired, 0 invalidated, 0 in-flight affected.**
- A stub note remains at the old 12.2 position in epics.md pointing to 10.2.

### Artifact Conflicts

| Artifact | Change |
| --- | --- |
| `epics.md` | Phase-scoping lines (Phase 2 gains FR15, Phase 3 drops it); guardrail "must NOT implement Croatian variant **or 501 mode** before Phase 3" loses the 501 clause; FR Coverage Map FR15 → Epic 10; Epic List entries for Epics 10 and 12; Epic 10 body gains Story 10.2; Epic 12 body keeps a moved-stub at 12.2. |
| `prd.md` | Phase 2 feature list gains a 501-mode bullet; Phase 3 list drops "501-point match mode". The Phase 1 "Explicitly Deferred" list keeps its entry (historically accurate). FR15 itself is unchanged. |
| `sprint-status.yaml` | `10-2-501-point-match-mode: backlog` inserted after 10-1; `12-2-501-point-match-mode` removed from the epic-12 block with a provenance comment; header comments updated. |
| `architecture.md` | **No change.** Architecture never gated 501 to Phase 3 — its journey-validity table already describes 501 as Phase 2, so this move resolves that pre-existing inconsistency rather than creating one. Its only prohibition ("not in Phase 1") remains true. |
| `ux-design-specification.md` | **No change** — no 501 references; ScorePanel target is already parameterized. |
| `project-context.md` | **No change** — its tiebreaker rule already reads "1001/501". |

### Technical Impact

Implementation surface when 10.2 is developed: add `"501"` to `validMatchModes`, un-disable the modal option, pass the room's mode through to `ScorePanel`'s `matchTarget`, and add table-driven engine tests at the 501 threshold (factories already accept `MatchMode`). The in-app rules pages (`client/src/features/rules/content/*.ts`) describe "race to 1001" — the story's dev pass should decide whether to footnote 501 there; flagged for the create-story context, not added as a new AC here.

## 3. Recommended Approach

**Direct Adjustment** (Option 1). Move the story; no rollback (nothing to revert) and no MVP review (MVP shipped; this is Phase 2/3 sequencing only).

- **Effort:** Low — documentation/backlog move now; the story itself is small (most plumbing already exists).
- **Risk:** Low — no in-flight work touched; no FR text changes; no architecture impact.
- **Timeline:** Positive — 501 mode ships a full phase earlier; Epic 12 shrinks, de-risking Phase 3.
- **Precedent:** consistent with prior resequencing (Epic 8.5 insertion 2026-05-02; Epic 10 promotion ahead of Epic 9, 2026-05-09).

## 4. Detailed Change Proposals

### 4.1 epics.md — Phase scoping (Additional Requirements)

```
OLD: - Phase 2: Coin economy (FR53–55), ..., additional languages MK+HR (FR45)
NEW: - Phase 2: ..., additional languages MK+HR (FR45), 501 mode (FR15 — moved from Phase 3 on 2026-06-11)

OLD: - Phase 3: Player search (FR5), friends (FR6), public profiles (FR47), Croatian variant (FR8), 501 mode (FR15), in-app rules reference (FR29)
NEW: - Phase 3: Player search (FR5), friends (FR6), public profiles (FR47), Croatian variant (FR8), in-app rules reference (FR29)

OLD: - Agents must NOT implement Croatian variant or 501 mode before Phase 3
NEW: - Agents must NOT implement the Croatian variant before Phase 3 (501 mode was moved to Phase 2 / Epic 10 on 2026-06-11)
```

### 4.2 epics.md — FR Coverage Map

```
OLD: FR15: Epic 12 — 501-point match mode
NEW: FR15: Epic 10 — 501-point match mode (moved from Epic 12 on 2026-06-11)
```

### 4.3 epics.md — Epic List entries

Epic 10 entry: title → "Additional Languages & 501 Match Mode"; goal mentions the shorter casual match target; FRs covered → FR45, FR15.
Epic 12 entry: goal drops "501-point matches"; FRs covered → FR8, FR29.

### 4.4 epics.md — Epic bodies

- Epic 10 heading/goal updated; **Story 10.2: 501-Point Match Mode** appended after Story 10.1 with the four acceptance criteria from old 12.2, verbatim.
- Epic 12 heading text updated; Story 12.2 section replaced by a moved-stub referencing 10.2 and this proposal.

### 4.5 prd.md — Phase 2 / Phase 3 lists

```
Phase 2 (added bullet):
- **501-point match mode:** shorter match target for casual rooms (moved up from Phase 3;
  decoupled from the Croatian variant — engine, room config, and UI plumbing already exist).

Phase 3 (removed bullet):
- 501-point match mode
```

### 4.6 sprint-status.yaml

```
Epic 10 block: add  10-2-501-point-match-mode: backlog   (after 10-1, with move comment)
Epic 12 block: remove  12-2-501-point-match-mode: backlog  (replaced by provenance comment)
```

**Rationale (all):** user-directed reprioritization; the story is technically independent of Epic 12 and nearly free to implement given existing plumbing.

## 5. Implementation Handoff

- **Scope classification:** **Moderate** — backlog reorganization (PO/DEV), no replan needed.
- **Applied in this pass:** all edits in §4 (epics.md, prd.md, sprint-status.yaml) plus this proposal document. Nothing committed to git — commit timing is the user's call.
- **Next step:** run `/bmad-create-story` (fresh context) — the sprint plan now resolves Story **10.2** as the next story. The story-creation pass should pull in the codebase-readiness notes from §1 and the rules-content question from §2 Technical Impact.
- **Success criteria:** story file `10-2-501-point-match-mode.md` created and validated; implementation passes all four ACs; `make test` green including new 501-threshold engine tests; room creation offers both modes end-to-end.

# Change 2 — Bot Players (new Story 10.3, new FR59)

## 1. Issue Summary

The user requested a new, higher-priority capability sequenced **after Story 10.2 and before Epic 9**: room owners can seat server-controlled bots on the empty seats of their room (1, 2, or 3 bots — any combination of free seats), and those seats are then played by bots for the whole match. Requirements stated by the user:

- Bots must be **smart** — competent play, not random-legal.
- Bots must not act robotically — a humanized think delay of ~1–2 seconds before acting, never instant.
- (Follow-up, same session) Matches with **at least one bot must be flagged in the DB** and visibly marked in match previews/history.

**Issue category:** New requirement from stakeholder (scope addition + resequencing).

No existing FR covers bots, so this adds **FR59** to the requirements inventory (epics.md is the living FR list — FR53–58 precedent). It also amends an explicit architecture constraint (see below).

## 2. Impact Analysis

### Epic Impact

| Epic | Change |
| --- | --- |
| Epic 10 | Title widened to "Additional Languages, 501 Mode & Bot Players"; gains Story 10.3 (FR59). Order inside the epic: 10.1 (done) → 10.2 → 10.3 → Epic 9 follows, exactly as requested. |
| Epic 9 | Unchanged in scope; now runs after 10.3. Note: Epic 9's coin settlement and honor/XP logic must ignore bot seats — captured in 10.3's ACs (bots accrue no XP, coins, honor, or stats), so Epic 9 inherits a clean rule rather than a conflict. |
| All others | Unchanged. |

### Artifact Conflicts

| Artifact | Change |
| --- | --- |
| `epics.md` | FR59 added to Requirements Inventory and FR Coverage Map; Epic 10 list entry + body updated; Story 10.3 added with full ACs. |
| `prd.md` | Phase 2 list gains a bot-players bullet (with the DB-flag requirement and the no-fill-in clarification). |
| `architecture.md` | **Hard constraint #6 amended.** It read "**No AI/bot opponents** — disconnection = pause or abandon, never AI fill-in". The disconnect half stays; the blanket "no bot opponents" half is lifted for owner-seated bots. PRD Phase 1's "no AI fill-ins" rule is preserved: bots are seated pre-game only and **never** replace a disconnected human mid-match. |
| `sprint-status.yaml` | `10-3-bot-players: backlog` added after 10-2. |

### Technical Impact (for the create-story pass)

- **Server:** new bot decision module (suggested `internal/bot/`) producing actions consumed by the session manager through the same `ApplyAction` path as human actions — the pure rules engine is untouched. Bot turns are driven by the session manager (humanized randomized delay ~1–2.5 s, always resolving inside the per-move timer so timeout auto-play never fires for bots). Bots have no WebSocket connection — they are server-side actors on a seat.
- **Seating:** room model gains bot-seat representation (no user account behind the seat); add/remove bot is an owner-only room-lobby action broadcast like seat changes; room start validation accepts bot-filled seats.
- **Persistence/flagging:** match record gains a bot-inclusive flag (new migration); match previews and match-history UI surface a "played with bots" marker; bot seats stored with bot identity, excluded from XP/coins/honor/stats accrual.
- **Strategy:** heuristic engine — hand-strength bidding, trump management, card memory, partner support, point maximization/denial; declarations and Belote/Rebelote announced whenever they score. Simulation tests must show it beats a random-legal baseline.
- **i18n:** bot names/badges localized in all four languages (see localization terminology reference).

### Sizing Warning

This is the largest Phase 2 story. The epics.md entry carries an explicit note: if story validation flags it oversized, **shard it** (10.3 foundation: seating + legal play + humanized timing; 10.4 strategy hardening) rather than thinning the acceptance criteria.

## 3. Recommended Approach

**Direct Adjustment** — add the story where the user wants it. Effort: High (relative to other Phase 2 stories — new domain module + migration + UI). Risk: Medium, contained by the ACs that wall bots off from economy/honor/resilience systems and by the architecture amendment that keeps the disconnect rule intact.

## 4. Detailed Change Proposals

Applied edits (all in this pass):

1. `epics.md` — FR59 appended to Requirements Inventory; `FR59: Epic 10` in coverage map; Epic 10 list entry and body retitled; Story 10.3 added with six AC groups (seating, normal flow, humanized legal actions, smart strategy, DB flag + history marking, resilience isolation).
2. `prd.md` — Phase 2 bullet added.
3. `architecture.md` — constraint #6 amended (owner-seated bots allowed; no disconnect fill-in preserved).
4. `sprint-status.yaml` — `10-3-bot-players: backlog` inserted; header comments updated.

## 5. Implementation Handoff

- **Scope classification:** **Moderate** — backlog addition + one architecture constraint amendment; no replan.
- **Sequence:** create-story 10.2 → dev 10.2 → create-story 10.3 → dev 10.3 → Epic 9.
- **Success criteria for 10.3:** all six AC groups pass; simulation tests prove bots beat the random baseline; matches with bots are flagged in DB and marked in previews/history; `make test` green; no change to human reconnect/pause behavior.
