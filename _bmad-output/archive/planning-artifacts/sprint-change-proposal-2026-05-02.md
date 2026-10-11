# Sprint Change Proposal — 2026-05-02

**Workflow:** `bmad-correct-course`
**User:** Emilijan
**Status:** Approved & Applied

## 1. Issue Summary

Epics 1–8 are complete. Epic 9 (Player Economy & Progression) is the next required body of work and touches the same hot paths that have accumulated 60+ open deferred items in `_bmad-output/implementation-artifacts/deferred-work.md`. The deferred backlog is mixed: most entries are RESOLVED or explicitly ACCEPTED-for-Phase-2, but ~30 are open issues — bugs, races, type asymmetries, and shared-pattern UX inconsistencies. Building Epic 9 (coin wallets, buy-in escrow, XP/level/honor settlement at match end) on top of these untouched issues compounds the same risk class the issues represent (atomicity at match end, denormalized counter drift, broadcast-before-persist races, unbounded per-user state).

**Triggering moment:** Epic 8 closed (`8-3-in-game-emotes` done 2026-04-28). Before kicking off `9-1-coin-wallet-foundation`, the user requested a triage pass over deferred-work.md to convert the open items into discrete, plannable stories.

**Issue category:** Hardening / debt consolidation ahead of a foundation-level epic.

## 2. Impact Analysis

### Epic Impact

| Epic                                | Status            | Change                                                                                                       |
| ----------------------------------- | ----------------- | ------------------------------------------------------------------------------------------------------------ |
| Epic 1–7                            | **Unchanged**     | Done; not touched.                                                                                           |
| Epic 8 (Game Table Enhancements)    | **Unchanged**     | All three stories (8.1, 8.2, 8.3) done; deferred items from review reports flow into the new Epic 8.5.       |
| **Epic 8.5 (NEW) — Pre-Phase-2 Hardening** | **Inserted**  | Three stories (8.5-1, 8.5-2, 8.5-3) batching open deferred-work.md items. No new FRs. Phase 2.               |
| Epic 9 (Player Economy & Progression) | **Unchanged scope; gated** | Stays as-is. Becomes runnable after Epic 8.5 completes (recommended) or in parallel from Story 9.1 (allowed). |
| Epic 10–16                          | **Unchanged**     | No downstream impact.                                                                                        |

### Story Impact

- **3 new stories** (8.5-1, 8.5-2, 8.5-3) added to the backlog. Sourced from `deferred-work.md`. Each story carries its own Acceptance Criteria mapped 1:1 to the deferred IDs it resolves.
- **0 retired stories.** No prior work is invalidated.
- **0 in-flight stories affected.** Epic 8 is closed; Epic 9 has not started.
- **deferred-work.md** is annotated, not rewritten — each item adopted into 8.5 will be flagged `→ folded into Story 8.5-X` once the story is created via `/bmad-create-story`. Items not selected (the ACCEPTED-for-Phase-1 backlog and a handful of low-value polish gaps) stay where they are.

### Artifact Impact

| Artifact                                 | Changes                                                                                                                                                                                                  |
| ---------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Epics (`epics.md`)                       | Insert Epic 8.5 in the Epic List (between Epic 8 and Epic 9 entries) and as a body section between Epic 8 and Epic 9 with three story bodies. **No FRs claimed.** **Phase: 2.**                          |
| Sprint Status (`sprint-status.yaml`)     | Insert `epic-8.5: backlog` block with three `8.5-1`, `8.5-2`, `8.5-3` story entries between the Epic 8 and Epic 9 blocks.                                                                                |
| Deferred-work index (`deferred-work.md`) | No structural change. After `/bmad-create-story` produces each 8.5 story, the corresponding D-IDs in deferred-work.md should gain a `→ Story 8.5-X` reference inline (a one-liner per item).             |
| PRD (`prd.md`)                           | **No-op.** Epic 8.5 introduces no new requirements; it converts open code-review findings into discrete work. No FR additions, retires, or rewordings.                                                   |
| Architecture (`architecture.md`)         | **No-op for the proposal.** Story 8.5-2 may surface architectural changes (typed `OutcomeReason` enum, `RemoveUser(userID)` hook on `session.Manager`); those land as ADR notes during story dev, not now. |
| UX Design (`ux-design-specification.md`) | **No-op.** Story 8.5-3 polish work touches existing components only — no new flows.                                                                                                                      |

### Technical Impact

Three coherent themes mapped to three stories:

**8.5-1 — Bug & race triage** (server: timer, room handler, session manager, store). Highest blast radius; sequence first.

**8.5-2 — Server-side concurrency & type hardening** (server: types, session manager broadcast plumbing, per-user rate-limit state, ID truncation). Compounds with Epic 9 — fixing now means coin wallet inherits correct primitives.

**8.5-3 — UX polish & shared-pattern refactors** (client: shared hooks, reveal flows, overlays, prompts, locale review). Lowest risk; can run in parallel with 8.5-1 / 8.5-2 if dev capacity allows. Includes one shared-pattern extraction (`useReducedMotion`) that prevents repeating the D68/D122 pattern across Epic 9's reveal flows (daily-reward toast, settlement overlay, etc.).

## 3. Recommended Approach

**Selected path: Option 1 — Direct Adjustment** (insert Epic 8.5 between Epic 8 and Epic 9).

Rationale:

- **No rollback needed** — no in-flight work invalidated.
- **No PRD/MVP rewrite** — Epic 8.5 is engineering quality, not product scope.
- **Cleanest sequencing for Epic 9** — Epic 9 introduces new atomic transitions (coin settlement at match end, XP grant, honor delta) that share code paths with the open issues (D101 match_end ordering, D29 counter drift, D109 stale participants). Fixing first means Epic 9 inherits a stable substrate; doing both in parallel risks landing Epic 9 stories on top of code that's about to change.

Effort estimate: **Medium** — three stories, ~1.5–2 weeks of solo-dev time.
Risk level: **Low** — well-scoped fixes against working code with existing test coverage.

Trade-offs considered:

- **Option A: skip the hardening epic, fold fixes opportunistically into Epic 9 stories.** Rejected — opportunistic fixes diffuse responsibility, make code review reports inconsistent (which deferred ID was fixed where?), and cross-cut Epic 9's commit graph with unrelated bug fixes.
- **Option B: file individual `spec-*.md` files per item (matching the existing 19-spec pattern).** Considered viable. Reserved as a fallback if 8.5-1's Acceptance Criteria grows past ~7 items; in that case 8.5-1 splits into individual specs.
- **Option C: defer everything to "Phase 2 hardening" alongside the explicit ACCEPTED-for-Phase-1 items.** Rejected — D101, D109, D113, and the LeaveRoom race are bugs today, not deferred polish; pairing them with rate-limiter / chat-moderation work delays fixes that materially affect Epic 9.

## 4. Detailed Change Proposals

### 4.1 — `epics.md` — Add Epic 8.5 to Epic List

**Insert between line 308 (current Epic 8 entry, ending `**Phase:** 2`) and line 310 (current `### Epic 9: Player Economy & Progression`):**

```markdown
### Epic 8.5: Pre-Phase-2 Hardening

Engineering-quality epic batching open code-review deferreds before Phase 2 economy work touches the same hot paths. Covers timer/race fixes, server-side concurrency & type hardening, and shared-pattern UX cleanups extracted from `deferred-work.md`.

**FRs covered:** none (technical debt epic — see deferred-work.md for source items)
**Phase:** 2
```

### 4.2 — `epics.md` — Add Epic 8.5 body

**Insert between line 1527 (last line of Epic 8 — Story 8.3 trailing AC) and line 1528 (current `## Epic 9: Player Economy & Progression`):**

```markdown
## Epic 8.5: Pre-Phase-2 Hardening

Engineering-quality epic. Three stories that fold the open items from `_bmad-output/implementation-artifacts/deferred-work.md` into discrete, reviewable work before Epic 9 economy work begins. Source: triage pass on 2026-05-02. Items explicitly ACCEPTED-for-Phase-1 in deferred-work.md remain deferred — this epic only tackles the open items where (a) the issue is a bug or latent race today, or (b) the fix establishes a primitive Epic 9 will reuse.

### Story 8.5-1: Bug & Race Triage

As an engineer,
I want the open bugs and races discovered across Epics 1–8 code reviews resolved before Phase 2 economy work begins,
So that Epic 9's match-end settlement, wallet, and counter operations build on a stable substrate.

**Acceptance Criteria:**

**Given** the per-move timer is cancelled via `cancelTurnTimer()` (e.g. on pause)
**When** a goroutine spawned by `time.AfterFunc` is already past the timer trigger and blocked on `session.mu.Lock()`
**Then** the goroutine's staleness check trips because `session.timerGeneration` was bumped at cancel time
**And** no work runs after the lock is acquired (defensive against future code added above the `handleTimerExpiry` switch)
[Resolves D — `cancelTurnTimer()` does not increment `timerGeneration` (server/internal/session/manager.go:861-919)]

**Given** the auto-start path (`SelectSeat` 4th-seat or QuickPlay last-seat) attempts `gameStarter.StartGame`
**When** `StartGame` returns an error
**Then** `system:game_started` is NOT broadcast
**And** the room's status is NOT flipped to `playing`
**And** clients receive a clear error toast instead of navigating to a non-existent `/game/{id}`
[Resolves the auto-start failure leak in `server/internal/room/handler.go:670-731, :1287-1298`]

**Given** a player calls `LeaveRoom` while another player is in the auto-start tx for the same room
**When** the leave runs concurrently with `gameStarter.StartGame`
**Then** the leave path re-checks `room.Status == "waiting"` under transaction lock and aborts (`ErrAlreadyStarted`) if start has begun
**And** `StartGame` never observes a `seatInfo` containing a player who has already left
[Resolves the LeaveRoom / auto-start race in `server/internal/room/handler.go:443-542`]

**Given** the match-end goroutine fires `event:match_end`
**When** the broadcast is sent
**Then** the match record (and any settlement-relevant rows the broadcast implies exist) are persisted to the DB BEFORE `BroadcastToUsers` runs
**And** a client immediately calling a hypothetical match-record API after receiving `match_end` finds the row
[Resolves D101 in `server/internal/session/manager.go:handleMatchEnd`]

**Given** the `player_count` denormalized counter on `rooms` may drift under concurrent join/leave
**When** `pickFirstEmptySeat` runs as part of the QuickPlay retry loop
**Then** an `ErrRoomFull` outcome triggers retry on a different/new room (not just `ErrRoomCodeTaken`/`ErrRoomNameTaken`)
**And** counter drift cannot surface as an opaque 5xx to the user
[Resolves D29 + the `pickFirstEmptySeat` fallback gap in `server/internal/room/handler.go:1126-1135`]

**Given** the 401 interceptor in `fetchClient` calls `authStore.logout()` after a refresh failure
**When** logout runs
**Then** `gameStore.clearGame()` is also invoked
**And** subsequent `useReconnectionRedirect` reads find a clean store and do not redirect to a finished game's `/game/{id}` page
[Resolves D66 — stale `gameState` in gameStore on re-login]

**Outcomes:**
- All six items above resolved with tests (table-driven Go tests for race paths; Vitest for the auth-init store-clear).
- `deferred-work.md` updated with `→ Story 8.5-1` annotations on each adopted ID.

### Story 8.5-2: Server-Side Concurrency & Type Hardening

As an engineer,
I want session-manager broadcast plumbing, per-user state lifecycles, and wire-format types hardened before Phase 2 introduces wallet/XP/honor at match end,
So that the new Epic 9 surfaces inherit correct primitives instead of inheriting the same risk shapes.

**Acceptance Criteria:**

**Given** `TeamStringForIndex` (Go) and `teamStringForIndex` (TS) receive a team index
**When** the index is outside `{0, 1}`
**Then** the Go variant returns a sentinel string (`""` or named constant), never panics
**And** the TS variant returns `null` for out-of-range
**And** Zod schemas at the wire boundary tighten to `z.union([z.literal(0), z.literal(1)])` so contract tests reject garbage at parse time
[Resolves D113 — `TeamStringForIndex` panic on out-of-range]

**Given** the WS `OutcomeReason` field is emitted by the server
**When** the client parses an `event:match_end`
**Then** the Go field is typed as `OutcomeReason` (named string type with constants), not raw `string`
**And** the TS Zod schema accepts the documented set (e.g. `"surrender" | "timeout" | "abandonment" | "natural"`) — Epic 9 will append `"insolvency"` and `"honor_eject"` per its new events
[Resolves D116 — `OutcomeReason` Go/TS asymmetry]

**Given** a user is removed from `session.Manager` (match end, leave, disconnect window expiry)
**When** `RemoveSession` runs (or a new `RemoveUser(userID)` hook fires alongside it)
**Then** the per-user `lastEmoteAt` entry in `emote.Handler` is cleared
**And** future cross-match emotes from that user respect a fresh rate-limit window in the next match
**And** the per-process `lastEmoteAt` map's growth is bounded by current-active users, not lifetime users
[Resolves D105 + D106 — `lastEmoteAt` unbounded growth + cross-match bleed-through]

**Given** `session.Manager.MatchParticipantsByUser` returns a participant slice
**When** `RemoveSession` (write lock) interleaves between the slice capture and the broadcast
**Then** the broadcast either re-checks session liveness OR holds the read lock through `BroadcastToUsers`
**And** orphaned tail-of-match emote / chat / event broadcasts to torn-down sessions are eliminated (or formally documented as accepted with a tracking comment)
[Resolves D109 — stale participants slice after RemoveSession]

**Given** the auth `:id` route parameter is parsed via `strconv.ParseUint(..., 10, 64)`
**When** the result is cast to `uint`
**Then** the cast is replaced with `uint64` end-to-end, OR the build is enforced 64-bit
**And** a forged `:id = 4294967297` cannot equate to auth user `1` on any supported platform
[Resolves D86 — `ParseUint` → `uint` truncation]

**Outcomes:**
- All five items above resolved.
- New `RemoveUser(userID)` hook (or equivalent) is reusable for Epic 9's per-user state (wallet rate-limit, daily-claim cooldown).
- `deferred-work.md` updated with `→ Story 8.5-2` annotations on each adopted ID.

### Story 8.5-3: UX Polish & Shared-Pattern Refactors

As a player,
I want reveals, overlays, prompts, and team labels to behave consistently across reconnect, phase transition, and OS-level setting changes,
So that the in-match experience is coherent before Phase 2 introduces more reveal/overlay surfaces (settlement, daily-reward, insolvent-eject).

**Acceptance Criteria:**

**Given** the OS-level "Reduce motion" setting toggles mid-session
**When** any reveal component (`BelotReveal`, `DeclarationReveal`, `RoomLobby`, `MatchResult`, `DealAnimation`) renders next
**Then** it reads the live value via a shared `useReducedMotion` hook subscribed to the media-query `change` event
**And** the snapshot-once `useMemo([])` pattern is removed from all five call sites
[Resolves D68 + D122 — shared `prefersReducedMotion` extraction]

**Given** a player disconnects during the 4s belot reveal window or mid-declaration-reveal animation
**When** they reconnect inside the window
**Then** the server-side `event:game_state` snapshot includes either the active reveal payload OR a clear "no active reveal" state
**And** the client's `setGameState` resets `declarationReveal` and any in-flight reveal fields on reconnect, so a stale reveal is never re-rendered on top of the reconnected state
[Resolves D69 + D71 — reveal events not replayed / stale `declarationReveal` surviving reconnect]

**Given** the centered declaration reveal is active
**When** a `PauseOverlay` or `ReconnectOverlay` mounts
**Then** the reveal timer pauses (or the reveal queues) instead of being silently consumed behind the tinted overlay
**And** the reveal resumes (or fires) once the overlay dismisses
[Resolves D112]

**Given** the player views a `MatchResult`, `ReconnectOverlay`, `MatchHistory`, or in-game `TrumpIndicator`
**When** team identity is rendered
**Then** all four surfaces honor the same convention (viewer-team-first columns, "Us"/"Them" labels) — `TrumpIndicator` is no longer the neutral "Team A"/"Team B" outlier
[Resolves D114 + D115]

**Given** the round-2 trump prompt renders on a mobile-landscape viewport (~360px height)
**When** the candidate card + 4 suit buttons + PASS button stack
**Then** the dialog body has `max-h-[90vh] overflow-y-auto` (or the suit grid restructures to 2x2) so the PASS button stays reachable
[Resolves the round-2 D97]

**Given** any overlay mounts above the dealer-indicator pill
**When** `MatchResult`, `ReconnectOverlay`, or disconnected-phase overlay is active
**Then** the dealer pill hides (or the overlay's z-index is unambiguously higher across all states)
**And** dealer/trump-caller name spans clamp via `max-w-[8rem] truncate` + `min-w-0` on the parent pill so a long username can't push the pill into the seat/score area
[Resolves dealer-indicator D97 + D98]

**Given** `SurrenderPrompt` (and its sibling `BelotPrompt`) is open
**When** the user presses `Escape`
**Then** the prompt closes via its decline handler
**And** the prompt's container uses `fixed` positioning (not `absolute`) so it cannot leak through scrolled/zoomed parent stacking contexts
[Resolves D102 + D103]

**Given** the emote picker is mounted across phase transitions (`playing` → `match_end` → next `dealing`)
**When** the picker is unmounted and remounted by the parent's allowlist gate
**Then** `lastSentAt` is preserved across remount (lifted to `useRef` in `GamePage` or to `gameStore`)
**And** the cooldown is computed via `performance.now()` (monotonic) so a system-clock backwards jump cannot lock tiles for arbitrary time
[Resolves D107 + D108]

**Given** Serbian-language strings "Tim A", "Tim B", "Mi", "Oni" render in the lobby and in-game
**When** a native Beljot-playing reviewer evaluates them
**Then** the strings are confirmed natural by a native speaker, OR replaced with idiomatic equivalents (e.g. "naša ekipa" / "njihova ekipa")
**And** the change ships in a single i18n update without code-side renames
[Resolves D125]

**Outcomes:**
- New `useReducedMotion` hook in `client/src/shared/hooks/`, used by ≥5 call sites.
- All eight items above resolved.
- `deferred-work.md` updated with `→ Story 8.5-3` annotations on each adopted ID.

### Bucket D items (test/doc gaps): folded into adjacent stories

The following items are NOT discrete ACs in 8.5-1/2/3 but are claimed by whichever story touches the relevant code:
- D87, D88 (stats / match-list status-filter desync) → 8.5-2 if the type-hardening pass touches `match/gorm_repo.go`; otherwise skipped.
- D100 (`time.Sleep` in manager_test) → 8.5-1 (the timer/race fixes will need stable async-test primitives anyway).
- D117, D118, D119, D120 (tests that can't fail) → 8.5-3 (touches the same component test files).
- D94 (Story 4.3 spec inverted compass labels), D111 (belot-reveal spec stale claim), D126 (Red/Blue prose in Go test comments) → 8.5-3 doc-cleanup pass.
- D124 (lockfile script self-exclusion) → 8.5-2 if a CI failure surfaces it; otherwise skipped.
```

### 4.3 — `sprint-status.yaml` — Insert Epic 8.5 block

**Update the `last_updated` field at top to `"2026-05-02"`.**

**Insert between line 110 (`epic-8-retrospective: optional`) and line 112 (`# --- Epic 9: Player Economy & Progression (Phase 2) ---`):**

```yaml

  # --- Epic 8.5: Pre-Phase-2 Hardening (Phase 2) ---
  # Inserted 2026-05-02 via bmad-correct-course; consolidates open deferred-work.md items
  # before Epic 9 economy work touches the same hot paths.
  epic-8.5: backlog
  8.5-1-bug-and-race-triage: backlog
  8.5-2-server-concurrency-and-type-hardening: backlog
  8.5-3-ux-polish-and-shared-pattern-refactors: backlog
  epic-8.5-retrospective: optional
```

### 4.4 — `deferred-work.md` — Annotation pass (deferred to story creation)

When `/bmad-create-story` generates each Epic 8.5 story file, append a `→ Folded into Story 8.5-X` marker to the corresponding deferred-work.md entries. Not done in this proposal — done as part of each story's first commit so the trail is git-traceable to the story branch.

## 5. Implementation Handoff

### Scope classification

**Moderate** — adds a new epic with three stories and updates two artifacts. No PRD/Architecture/UX rewrites. No FR additions or retires.

### Routing

| Recipient                | Responsibilities                                                                                                                                                                                                                                                                                                                                                                                                       |
| ------------------------ | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Bob (Scrum Master)**   | Re-run `/bmad-sprint-planning` after this proposal applies, so `sprint-status.yaml` becomes the active sprint plan with Epic 8.5 in scope. Then `/bmad-create-story 8.5-1` for the first story.                                                                                                                                                                                                                       |
| **Amelia (Dev)**         | Implement Epic 8.5 stories in order (8.5-1 → 8.5-2 → 8.5-3 recommended; 8.5-3 may be parallelised if capacity allows). Each story closes its own deferred-work.md IDs (annotated `→ Story 8.5-X`). Implementation order inside Epic 9 is unchanged from the existing plan: 9.1 → 9.2 → 9.3 → 9.5 → 9.4 → 9.6 → 9.7. |
| **Winston (Architect)**  | Review Story 8.5-2's typed `OutcomeReason` enum decision and the `RemoveUser(userID)` hook signature before Story 9.1 builds on top. ADR-light: a paragraph in `architecture.md` is sufficient.                                                                                                                                                                                                                       |
| **Sally (UX Designer)**  | No action required. Story 8.5-3 polishes existing components only. UX work for Epic 9 (Story 9.1+) is unchanged from the 2026-04-18 proposal.                                                                                                                                                                                                                                                                         |
| **Paige (Tech Writer)**  | Optional: refresh stale doc claims in 8.5-3 (D94 compass labels in `4-3-game-table-ui-layout-seats-and-cards.md`, D111 belot-reveal mirror claim in `spec-belot-rebelot-prompt-and-reveal.md`).                                                                                                                                                                                                                        |

### Pre-development blockers

None. Epic 8.5 work is fully specified by the deferred-work.md source items it adopts.

### Success criteria

- Sprint Change Proposal accepted by user.
- `epics.md` updated with Epic 8.5 entry + body — Epic 1–8 and Epic 9–16 content unchanged.
- `sprint-status.yaml` updated with `epic-8.5: backlog` block and `last_updated: "2026-05-02"`.
- Next `/bmad-create-story` invocation picks up `8.5-1-bug-and-race-triage` cleanly.
- Epic 1–8 test suites continue to pass (must verify on next dev session — no code changed in this workflow).
- After Epic 8.5 stories ship, ≥18 deferred-work.md IDs gain `→ Story 8.5-X` annotations.

### Outstanding follow-ups

- `/bmad-sprint-planning` — refresh the sprint plan against the rewritten `sprint-status.yaml`.
- `/bmad-create-story` — generate the three Epic 8.5 story specs.
- (Optional) `/bmad-retrospective` for Epic 8 before starting Epic 8.5 if any patterns from this triage repeat lessons-learned themes.

---

**Workflow complete, Emilijan!**

Applied edits:

- `epics.md` — Epic 8.5 entry inserted in Epic List (after Epic 8) + Epic 8.5 body section inserted before Epic 9 with three story bodies.
- `sprint-status.yaml` — `last_updated: "2026-05-02"`; `epic-8.5: backlog` block with `8.5-1`, `8.5-2`, `8.5-3` story entries inserted between Epic 8 and Epic 9.
- `deferred-work.md` — annotation pass deferred to per-story commits (will be `→ Story 8.5-X` markers added by the dev when each story branch lands).
