# Sprint Change Proposal — Epic 9 Restructure (Player Economy & Progression)

**Date:** 2026-06-18
**Author:** Emilijan (via bmad-correct-course)
**Scope classification:** Moderate — backlog reorganization of a not-yet-started epic. No implementation code exists yet, so this is a planning-artifact change only. Mostly a reordering + AC rewrites + one new story.

---

## Section 1 — Issue Summary

Epic 9 is the last open Phase-2 epic and the clear next thread (all of Epics 1–8, 8.5, 10 are done). Before kicking off `bmad-create-story` for it, we ran a pre-implementation review of the planned stories and found several gaps and one set of requirements that conflicts with behavior already shipped:

- The coin-settlement pot rules (Story 9.2) refunded an abandoning player's teammate and didn't account for bots (which don't stake).
- The insolvency story (Story 9.3) was written before the **Return to Room** feature shipped (`spec-return-to-room-after-match.md`, `spec-return-to-room-presence-v2.md`), so its "checked when the room prepares for a next match" framing no longer matches how returning actually works.
- A desired **private (password-protected) rooms** feature had no story at all.
- The honor "New Player" floor of 20 completed matches was judged too high.
- Story ordering didn't reflect the desired priority (coins first; private rooms before the honor cluster; XP/level ahead of room-access work).

## Section 2 — Impact Analysis

**Epic impact:** Epic 9 only. Re-sequenced, two stories rewritten, one new story added, honor floor lowered. No other epic's scope changes.

**Story impact:**
- Story 9.2 (Room Buy-In & Settlement) — settlement ACs rewritten.
- Story 9.3 (Insolvency Ejection) — rewritten to gate the existing Return-to-Room flow; **partially supersedes** spec-return-to-room-presence-v2's "no auto-transfer of ownership from an absent owner" rule, for the insolvency / owner-leave case only.
- Story 9.6 (Private Rooms) — **new**.
- Stories renumbered: Honor Score 9.6 → 9.7; Honor-Gated Rooms 9.7 → 9.8. XP & Level stays 9.5.
- Honor floor 20 → 5 completed matches (Honor Score + Honor-Gated + the Epic 11 public-profile reference).

**Artifact conflicts / updates:**
- `epics.md` — Requirements Inventory (added FR60), Phase-2 summary, Epic 9 overview + body, FR Coverage Map, and two cross-references (Epic 8 surrender pot-math pointer corrected to Story 9.2; Epic 11 honor-label pointer corrected to Story 9.7).
- `sprint-status.yaml` — added `9-6-private-rooms`; honor stories renumbered to `9-7`/`9-8`.
- PRD — no change needed; epics.md is the canonical/living FR list (FR59 precedent).
- Architecture/UX docs — no change needed; the new `rooms.password_hash` column is a minor additive schema detail captured in the story AC.

**Technical impact:** None yet (no code). Future implementation touches the wallet/room/match-settlement hot paths and the already-shipped return-to-room handler + presence registry.

## Section 3 — Recommended Approach

**Direct Adjustment** — modify/add stories within the existing plan. No rollback (nothing built), no MVP-goal change. Effort is limited to planning artifacts; the only noteworthy downstream note is that Story 9.3 will revise a small piece of already-shipped return-to-room behavior, which is called out explicitly in its ACs.

## Section 4 — Detailed Change Proposals

### Re-sequenced Epic 9

| # | Story | Change |
|---|-------|--------|
| 9.1 | Coin Wallet Foundation | unchanged |
| 9.2 | Room Buy-In & Match Settlement | settlement rewrite |
| 9.3 | Insolvency Ejection & Room Persistence | rewrite for return-to-room gate |
| 9.4 | Quick Play Coin Bracketing | unchanged (bracket bands = tuning time) |
| 9.5 | XP & Level System | unchanged |
| 9.6 | **Private Rooms** | NEW |
| 9.7 | Honor Score System | floor 20 → 5 |
| 9.8 | Honor-Gated Rooms | renumbered; floor 20 → 5 |

### 9.2 — Settlement (pot rules)

- **OLD:** Pot = 4S always; abandonment refunds the abandoner's teammate (net 0) and winners split a reduced 3S pot.
- **NEW:**
  - Pot = sum of **human** stakes only; bots never stake or receive.
  - Winning team's **human** players split the pot equally.
  - Abandonment = full loss for the **entire abandoning team** (both forfeit −S, no teammate refund).
  - Surrender = normal loss (unchanged).
  - Edge: winning team with no human (all-bot winners) → losing humans' stakes are a **coin sink** (house keeps them).
- **Rationale:** Bots have no wallet; refunding an abandoner's partner removed the incentive to pick reliable teammates. Treating abandonment/surrender/loss identically also simplifies the settlement code.

### 9.3 — Insolvency Ejection & Room Persistence

- **OLD:** Generic "when the room prepares for a next match, check each seated wallet and `event:insolvent_kick`."
- **NEW:**
  - Affordability checked at **`POST /rooms/:id/return`** (the moment a player clicks "Return to room"). Insufficient balance → reject with `error:insufficient_coins`, free the seat, route to lobby with a clear modal.
  - Players who haven't acted yet stay **held as seated** until they return or leave.
  - Owner barred/leaving → ownership transfers to the first seated player who is **present (returned) AND solvent**; if none, the room **closes** (`event:room_closed_insolvent`).
  - The authoritative money guard is the atomic balance re-validation inside `StartMatch`'s stake deduction; an insolvent-at-start player is routed through the same ejection flow. No redundant third UI check.
- **Rationale:** Aligns insolvency with the shipped return-to-room flow; the present-AND-solvent ownership rule avoids handing a room to an absent owner (consistent with v1/v2 intent) while still letting solvent players carry on.
- **Note:** This **supersedes** spec-return-to-room-presence-v2's "no auto-transfer from an AWOL owner" for the insolvency / owner-leave case.

### 9.6 — Private Rooms (NEW, FR60)

- Owner toggles "Private" at create time and sets a password (stored **hashed**, bcrypt, in `rooms.password_hash`, nullable).
- Private rooms remain **listed but locked** (Option B) — the join **code stays as-is** for discovery/identity.
- Any join attempt to a private room — via locked card **or** via code — prompts a **password dialog before** joining; verified server-side on `action:join_room`; wrong password → `error:wrong_room_password`.
- Owner can change the password or revert to public; password never returned by the API; Quick Play rooms are never private.
- **Rationale:** Closed tables for friends without a fully hidden/unfindable room. Placed before the honor cluster per priority.

### Honor floor 20 → 5

- "New Player" label / score-suppression now applies under **5** completed matches (was 20), in Honor Score, Honor-Gated join logic, and the Epic 11 public-profile reference.

### Deferred (by decision)

- **Economy constants** (starting balance, daily curve, default buy-in, bracket bands, level curve, honor weights) stay as placeholders, tuned during each story's planning.
- **FR42 career statistics** — no new Epic 9 story; it's an Epic 11 display surface fed by data recorded as later epics implement it.

## Section 5 — Implementation Handoff

- **Scope:** Moderate (backlog reorg, no code). Artifacts updated: `epics.md`, `sprint-status.yaml`, this proposal.
- **Next action:** `bmad-create-story` for **9-1-coin-wallet-foundation** (fresh context window). Stories then proceed 9.1 → 9.8 in order.
- **Watch items for story planning:** lock economy constants per story; when 9.3 is built, treat it as an explicit amendment to the return-to-room presence behavior; 9.2 settlement should be the single source of pot math (Epic 8 surrender now points here).
- **Success criteria:** epics.md reflects the 8-story sequence with FR60 registered; sprint-status lists 9-1…9-8; no dangling references to the old Story 9.6/9.7 honor numbering or the old 5433/teammate-refund rules.
