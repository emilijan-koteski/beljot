# Sprint Change Proposal — Pre-Phase-3 Backlog Correction

**Date:** 2026-08-14
**Author:** Emilijan (via bmad-correct-course)
**Scope classification:** Moderate — planning-artifact-only change. Backlog reorganization of not-yet-started Phase-3/4 work, plus splitting one already-shipped story out of Epic 14. No implementation code is written by this proposal.

---

## Section 1 — Issue Summary

Phase 2 is complete — Epics 1–10 and 8.5 are all `done`, and Epic 9 (economy + honor, including the honour redesign) just merged (`dc14308`). Everything from Epic 11 onward is `backlog`. Before starting Phase 3, three corrections are needed:

1. **Google OAuth already shipped.** Google sign-in was built out incidentally during the Phase 1–2 work (provider-agnostic SSO handler, `BELJOT_GOOGLE_CLIENT_ID`, "Continue with Google" button), but it lives under a single backlog story `14-1-google-and-facebook-oauth` that bundles Facebook. Facebook has no production verifier. The story must be split so Google can be marked done and Facebook tracked separately.
2. **Epic 15 (Mobile Experience) PWA scope is not built and is being dropped.** The mobile-responsive layout + touch interaction (responsive `MatchPage`, `useVisualViewport`, 320px-safe cards) already shipped across Phase 1–2, but the PWA/native scope (manifest, service worker, installability, offline splash, push) was never implemented. Per decision, Epic 15 is removed rather than carried.
3. **Two new friend-social features need first-class specs in Epic 11** before Phase 3 begins: a private friend "whisper" chat, and friend room invites.

**Evidence (verified in code 2026-08-14):**
- Google wired: `server/internal/auth/sso_handler.go`, `server/internal/config/config.go` (`GoogleClientID`), `client/public/google.svg`, SSO handler tests.
- Facebook not wired: no Facebook config/verifier in production code; only a `"facebook"` provider path exercised in tests (handler is provider-agnostic).
- No PWA: no `manifest.json`/`.webmanifest`, no service worker, no `vite-plugin-pwa`, no `manifest`/`theme-color`/`apple-mobile` tags in `client/index.html`.

## Section 2 — Impact Analysis

**Epic impact:**
- **Epic 11 (Friends & Public Profiles)** — two new stories appended (11.4, 11.5); existing 11.1–11.3 unchanged and not renumbered. Two new FRs: **FR61, FR62**.
- **Epic 14 (Social Login)** — Story 14.1 split into **14.1 Google OAuth (done)** + **14.2 Facebook OAuth (backlog)**; epic status → `in-progress` (delivered ahead of its Phase-4 slot).
- **Epic 15 (Mobile Experience)** — **removed** (tombstoned in place, consistent with the project's retire-in-place convention for Story 12.2 and FR35/36/38). **FR52** marked `[descoped]`.
- No other epic's scope changes; no epic resequencing.

**Story impact:**
- New: `11-4-friend-whisper-chat`, `11-5-friend-room-invites` (both depend on `11-2` friend model + server-side presence).
- Split: `14-1-google-and-facebook-oauth` → `14-1-google-oauth` (done), `14-2-facebook-oauth` (backlog).
- Withdrawn: `15-1-pwa-mobile-layout-and-touch-interaction`.

**Artifact conflicts / updates:**
- `epics.md` — Requirements Inventory (+FR61, +FR62), Phase 3/4 scoping bullets, FR Coverage Map (+FR61/FR62; FR3 annotated; FR52 descoped), Epic 11/14/15 overview cards, Epic 11 body (+2 stories), Epic 14 body (split), Epic 15 body (tombstone).
- `sprint-status.yaml` — Epic 11 +2 stories; Epic 14 story split + epic → `in-progress`; Epic 15 block removed; header note.
- PRD — no change (epics.md is the canonical living FR list, per FR59/FR60 precedent).
- Architecture/UX — no change needed; new WS events (`event:whisper`, `event:room_invite`) and presence reads are additive and captured in story ACs / technical notes.

**Technical impact:** None yet (no code). For later dev: 11.4 and 11.5 each introduce new `event:*` WS contracts and therefore incur the WS drift-gate touchpoints (Go golden files + testdata + Zod schemas + contract tests) that the honor stories tracked; both read the server-side presence registry already used by return-to-room / honor; 11.5's password bypass must be a server-authorized one-time grant, never a client flag; 11.5's honor gate (Story 9.8) still applies to invitees.

## Section 3 — Recommended Approach

**Direct Adjustment** — modify/add stories within the existing plan. No rollback (Google is already shipped and stays; nothing else built). No MVP-goal change. Effort limited to planning artifacts. The only "reversal" is that Epic 15's PWA ambition is formally descoped; the responsive layout it would have delivered already exists.

## Section 4 — Detailed Change Proposals

### 4a. Epic 14 — split Story 14.1

| # | Story | Status | Note |
|---|-------|--------|------|
| 14.1 | Google OAuth | **done** | GIS ID-token credential verified against `GoogleClientID`; provider-agnostic SSO handler creates/links account |
| 14.2 | Facebook OAuth | backlog | handler already provider-agnostic; needs Facebook token verifier + client button |

Epic 14 → `in-progress` (Story 14.1 delivered early, ahead of Phase 4).

### 4b. Epic 15 — removed

Tombstoned in `epics.md` (overview card + body). `FR52` → `[descoped 2026-08-14] — Epic 15 removed; mobile-responsive layout delivered incidentally across Phase 1–2, PWA/native dropped`. Epic 16 retains its number (no renumber).

### 4c. New FRs

- **FR61** — Friends can exchange private one-on-one "whisper" messages, initiated with the `/w <username>` chat command from the lobby, a room, or a match. Whispering is blocked to any friend currently in the sender's same active room or match (teammate or opponent) to prevent card-information collusion; the block is enforced server-side. Whispers are real-time only (recipient must be online) and ephemeral (not persisted), rendering visually distinct (pink) with Valorant-style channel switching between the primary chat and each whisper thread. → Epic 11 / Story 11.4
- **FR62** — Room members can invite "available" friends (online, in the lobby, not in any room or match) into a `waiting` room. A host (owner) invite bypasses the room password via a server-authorized one-time grant; a non-host member's invite still requires the invitee to enter the room password if one is set. Invites are delivered as a popup, expire on timeout, and auto-void if the room fills/closes or the friend leaves the lobby. The honor gate (Story 9.8) and seat capacity still apply to all invitees. → Epic 11 / Story 11.5

### 4d. Story 11.4 — Friend Whisper Chat (NEW)

Full Given/When/Then acceptance criteria written into `epics.md` under Epic 11. Highlights: `/w <friendUsername> <message>` → private `event:whisper`; `error:not_friends` for non-friends; **anti-collusion** `error:whisper_blocked_in_game` for a target in the same active room/match (server-authoritative via presence registry); pink bubbles + Tab-key channel switching; `error:whisper_recipient_offline` (real-time only); ephemeral (no history).

### 4e. Story 11.5 — Friend Room Invites (NEW)

Full Given/When/Then acceptance criteria written into `epics.md` under Epic 11. Highlights: invite available friends from a `waiting` room via `event:room_invite` popup; **host** invite → auto-join bypassing password via server-issued one-time grant; **non-host** invite + private room → Story 9.6 `PasswordPromptDialog` (`error:wrong_room_password`); honor gate + `allow_new_players` + capacity still apply to all invitees; graceful failure on full/closed/expired.

**Confirmed decision:** invites bypass **only** the password (host case). The honor gate is **not** bypassed for any invitee (host or non-host). (Confirmed by Emilijan 2026-08-14.)

### 4f. Reprioritization

Backlog is otherwise already in phase order; no epic resequencing needed. Epic 11 story order: 11-1 search → 11-2 friends → 11-3 profiles → **11-4 whisper** → **11-5 invites** (both after their 11-2 dependency). Epic 14 now sits `in-progress` ahead of its Phase-4 slot.

## Section 5 — Implementation Handoff

- **Scope:** Moderate (no code). Artifacts updated: `epics.md`, `sprint-status.yaml`, this proposal.
- **Recipients:** Product Owner / Developer — backlog reorganization only.
- **Next action:** normal Phase-3 entry — `bmad-create-story` for `11-1-player-search` in a fresh context window. Stories proceed 11-1 → 11-5 in order.
- **Watch items for story planning:** whisper + invites depend on Story 11.2's friend model and the presence registry; both add `event:*` WS contracts (drift-gate touchpoints); 11.5's password bypass must be server-authorized; 11.5 must not bypass the Story 9.8 honor gate; Facebook (14.2) reuses the existing provider-agnostic SSO path.
- **Success criteria:** epics.md registers FR61/FR62, splits Story 14.1, and tombstones Epic 15/FR52; sprint-status lists 11-1…11-5, 14-1 (done) + 14-2 (backlog), and no longer lists Epic 15; no dangling FR52→Epic 15 or "14-1-google-and-facebook-oauth" references remain.
