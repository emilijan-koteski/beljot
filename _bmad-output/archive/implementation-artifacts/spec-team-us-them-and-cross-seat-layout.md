---
title: "Team Rename red/blue → teamA/teamB, Us/Them Labels, Cross Seat Layout"
type: "feature"
created: "2026-05-01"
status: "done"
baseline_commit: "d8e8c0f7f406442ca9253f009c27165175c635f6"
context:
  - "{project-root}/_bmad-output/project-context.md"
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** The codebase uses `red`/`blue` everywhere — UI text, TS literal types, Zustand state, WS payload field names, Go constants and struct fields with gorm/json tags, DB columns, CSS tokens, Tailwind utility classes. The follow-up task switches team colors to gold/silver, so "Red"/"Blue" labels and identifiers become misleading at every layer. The room-lobby 2x2 seat grid also doesn't visually convey that partners sit across the table.

**Approach:** One sweep, three goals:

1. **Data-model rename** `red`/`blue` → `teamA`/`teamB` end-to-end. The string literal value is the full word `"teamA"` / `"teamB"` (Winston's grep-ability rule — never bare `"a"`/`"b"`). Compound identifiers use the bare suffix in their casing convention: `team_a_score` (SQL), `teamAScore` (camel), `TeamAScore` (Pascal), `team-a` (CSS kebab). Numeric team index `0`/`1` is unchanged. Go constants `TeamRed`/`TeamBlue` → `TeamA`/`TeamB`.
2. **UI relabel** to viewer-relative "Us"/"Them" anywhere the viewer has skin in the game (active match, plus the viewer's own past matches in history) and neutral "Team A"/"Team B" everywhere else (room lobby pre- and post-seat, room browse, hypothetical spectator views). The label flip happens at game-start (deal animation), not at seat-selection — staging stays neutral.
3. **Cross/diamond seat layout** in the room lobby — viewer rotates to bottom on seating. Animated 300ms reorientation with a "your seat" micro-label confirming the click. Mobile portrait (<640px) falls back to a vertical 2-column stack with colored borders.

DB migrations 000006 and 000009 are edited in place (user is solo dev, no other environments, code lands on master directly, no production yet). CSS color hex values stay unchanged this task — the variable rename `--color-team-red` → `--color-team-a` happens now; the hex flip to gold/silver is the next task. A `data-team="teamA"|"teamB"` attribute hook is added to all team-bearing elements now so the next task can attach a non-color partnership signal cleanly.

## Boundaries & Constraints

**Always:**

- Single canonical naming. String literal value: `"teamA"` / `"teamB"` (full word, never bare). Compound identifiers use the bare suffix per language convention: `team_a_score` (SQL), `teamAScore` (camel/JSON), `TeamAScore` (Go field), `team-a` / `team-b` (CSS kebab and Tailwind utility names). Numeric team index `0`/`1` unchanged.
- Wire format is pinned: `PlayerState.team` is the string `"teamA"` / `"teamB"`. Team-index payload fields (`winnerTeam`, `capotTeam`, `lastTrickTeam`, `contractingTeam`, `surrenderProposerTeam`) stay integers `0` / `1`. Conversion lives in exactly one Go function and one TS function — every other site goes through it. The conversion is `TeamStringForIndex(0) = "teamA"` / `TeamStringForIndex(1) = "teamB"` (and inverse).
- Go constants rename: `TeamRed = 0` → `TeamA = 0`, `TeamBlue = 1` → `TeamB = 1`. Same numeric values.
- Project rule: WS event contract files (`client/src/shared/types/wsEvents.ts` and `server/internal/ws/events.go`) ship in the same commit. With the new contract test below, this becomes machine-verified instead of social.
- DB migrations `000006_create_matches.up.sql/.down.sql` and `000009_create_hand_results.up.sql/.down.sql` are edited in place (justified by solo-dev / single-environment / pre-production state). `000004_create_room_players.up.sql` column shape unchanged — only stored values change from `"red"`/`"blue"` to `"teamA"`/`"teamB"` (still fits `VARCHAR(10)`).
- "Us"/"Them" UI text appears wherever the viewer has skin in the game: live game (`ScorePanel`, `ScoreReveal`, `MatchResult`, `DeclarationReveal`, in-game match-abandon toast) and the viewer's own past matches in `MatchHistory`. "Team A"/"Team B" UI text appears in staging/neutral views: `RoomLobby` (pre- and post-seat — no flip during seat selection), `RoomDetailPreview`, and any future spectator/admin view.
- DeclarationReveal viewer-relative logic keys off **team membership**, not seat equality: viewer sees "Us" when `team === viewerTeam` (i.e., `(viewerSeat + declarerSeat) % 2 === 0`). Both partners see "Us" when either of them declares.
- All team-bearing tiles, score blocks, and labels carry `data-team="teamA"` or `data-team="teamB"` so the next task (gold/silver palette + non-color partnership signal for color-blind users) has a clean attribute hook with no further structural change.
- Existing `data-testid="player-seat-{N}"` (where `{N}` is the **seat index 0–3**, server-canonical, rotation-independent) stays stable. A new `data-testid="seat-position-{south|north|east|west}"` is added on the same elements so tests can assert visual position separately from seat index.
- English (`en.json`) and Serbian Latin (`sr.json`) updated together. Serbian is `sr-Latn` only this task — no Cyrillic. Strings: "Mi" / "Oni" / "Tim A" / "Tim B".
- Mobile portrait <640px (Tailwind `sm` breakpoint) — diamond layout collapses to a vertical 2-column stack with the same colored borders and same `data-testid` values. Partnership reads from team-color borders on the stack.
- Diamond rotation transition: 300ms ease-out reorientation when the viewer takes a seat. A "your seat" micro-label fades in at the bottom slot once rotation completes (≤200ms after rotation). `prefers-reduced-motion: reduce` users get a snap (no animation) plus an immediate "your seat" label.

**Ask First:**

- If the rename surfaces a `red`/`blue` literal that is NOT a team name (e.g. an unrelated CSS color, an icon variant), surface it before changing.
- If the WS JSON contract test (see Tasks) requires choosing between Zod / io-ts / a hand-rolled discriminator on the TS side, stop and ask which the project should adopt — that's a project-wide decision, not a local one.

**Never:**

- Don't keep both names ("backward-compatibility shim", "deprecated alias"). Single source of truth.
- Don't add a brand-new migration file — edit existing ones in place.
- Don't change CSS color hex values this task. Only rename the token names. Hex flip to gold/silver is the next task.
- Don't add an `us` / `them` field to game state. Derive viewer team purely client-side from `myPlayerSeat`.
- Don't introduce bare `"a"` / `"b"` as string literal values anywhere. Always full `"teamA"` / `"teamB"`.

## I/O & Edge-Case Matrix

| Scenario                                           | Input / State                                         | Expected Output / Behavior                                                                                                                                                                                |
| -------------------------------------------------- | ----------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Score panel during live match                      | viewer seat 1 (teamB), aScore 80, bScore 120          | Row labelled "Them 80" in team-A color, "Us 120" in team-B color; both rows carry `data-team` attributes                                                                                                  |
| Match end overlay                                  | `winnerTeam=0`, viewer seat 0 (teamA)                 | "Us Wins!" in team-A color                                                                                                                                                                                |
| Capot animation                                    | `capotTeam=0`, any viewer                             | "CAPOT!" in team-A color, no team text                                                                                                                                                                    |
| DeclarationReveal — viewer's partner declared      | viewer seat 1 (teamB), declarer seat 3 (teamB)        | "Us declared"                                                                                                                                                                                             |
| DeclarationReveal — opponent declared              | viewer seat 1 (teamB), declarer seat 2 (teamA)        | "Them declared"                                                                                                                                                                                           |
| Room lobby pre-seat                                | viewer hasn't seated                                  | Diamond layout with default rotation (seat 0 at bottom, 2 at top, 1+3 sides), headers "Team A" / "Team B"                                                                                                 |
| Room lobby — viewer takes seat 2                   | seat 2 click                                          | 300ms ease-out rotation: seat 2 to bottom, seat 0 to top, seats 1+3 to sides; "your seat" label fades in at bottom slot. Headers stay "Team A" / "Team B" — no flip                                       |
| Room lobby on mobile portrait <640px               | viewer of any seat state                              | Vertical 2-column stack (same as today), team-color borders preserved, same testids                                                                                                                       |
| RoomDetailPreview (lobby browse)                   | any room                                              | Headers "Team A" / "Team B"; never "Us"/"Them"                                                                                                                                                            |
| Game starts (deal animation)                       | match begins                                          | ScorePanel takes over; first time "Us"/"Them" labels appear                                                                                                                                               |
| Match history — viewer's own past match            | viewer participated, was teamA                        | Row shows "Us 1010 — Them 640", failed-hand chip and last-trick chip use Us/Them                                                                                                                      |
| Match history — admin viewing someone else's match | edge case, not in current product                     | Falls back to "Team A" / "Team B" — covered by viewer-participation predicate                                                                                                                             |
| Match-abandoned toast                              | aFinalScore 200, bFinalScore 180, viewer participated | "Final: Us 200 : Them 180" (in-game toast — viewer is participant)                                                                                                                                        |
| Backend persists hand result                       | hand finishes                                         | Row inserted into `hand_results` with columns `a_card_points`, `b_card_points`, `a_decl_points`, `b_decl_points`, `a_hand_total`, `b_hand_total`                                                          |
| WS payload for `event:hand_scored`                 | server broadcasts                                     | JSON contains `aCardPoints`, `bCardPoints`, `aDeclPoints`, `bDeclPoints`, `aHandTotal`, `bHandTotal`, `aMatchScore`, `bMatchScore`. Old `red*`/`blue*` keys absent. Enforced by JSON golden contract test |
| Serbian locale                                     | any view above                                        | Renders "Mi"/"Oni"/"Tim A"/"Tim B"; no text overflow on diamond seat tiles (asserted in test)                                                                                                             |

</frozen-after-approval>

## Code Map

**Note (2026-05-01 follow-up):** The original rename pass shipped compound names with bare `a`/`b` suffixes (`a_card_points`, `ACardPoints`, `aHandTotal`, etc.). Per Winston's grep-ability rule the prefix must extend into compound names too — so the live names today are `team_a_card_points` (SQL), `TeamACardPoints` (Go), `teamACardPoints` (JSON/TS), and so on. The Code Map below preserves the original-pass names for the historical record; the Naming convention summary table in **Design Notes** documents the current canonical form.

**Backend — game core, ws, match domain (commit 1 territory):**

- `server/internal/game/state.go` — `PlayerState.Team` literal updated to `"teamA"`/`"teamB"`; `HandResult` struct fields are `TeamACardPoints`, `TeamBCardPoints`, `TeamADeclPoints`, `TeamBDeclPoints`, `TeamAHandTotal`, `TeamBHandTotal` with matching `json:"teamA*"` / `"teamB*"` tags; constants are `TeamA`/`TeamB`; add `TeamStringForIndex(int) string` and `TeamIndexForString(string) int` as the single conversion point
- `server/internal/game/scoring.go` + `scoring_test.go` + `scoring_internal_test.go` — field name updates; comments updated
- `server/internal/game/declarations_test.go`, `state_test.go` — assertions and fixtures
- `server/internal/game/testfixtures/fixtures.go` + `fixtures_test.go` — `Team: "teamA"`/`"teamB"`; ensure factories use named struct fields, not positional args (Amelia's note)
- `server/internal/match/model.go` — fields `TeamAScore`/`TeamBScore`; gorm column tags `team_a_score`/`team_b_score`; JSON tags `teamAScore`/`teamBScore`
- `server/internal/match/hand_result.go` — fields `TeamACardPoints` etc.; gorm columns `team_a_card_points`, `team_b_card_points`, `team_a_decl_points`, `team_b_decl_points`, `team_a_hand_total`, `team_b_hand_total`; JSON tags mirror
- `server/internal/session/manager.go` + test, `session/reconnect.go` — broadcast map keys use `"teamACardPoints"` etc.; references to `game.TeamA`/`game.TeamB`
- `server/internal/room/handler.go` + test — `getTeamForSeat` returns `"teamA"`/`"teamB"`; all test fixtures
- `server/internal/user/handler.go` + test — match-history response struct fields and JSON tags
- `server/migrations/000006_create_matches.up.sql` + `.down.sql` — columns `team_a_score`, `team_b_score`
- `server/migrations/000009_create_hand_results.up.sql` + `.down.sql` — columns `team_a_card_points`, `team_b_card_points`, `team_a_decl_points`, `team_b_decl_points`, `team_a_hand_total`, `team_b_hand_total`

**WS contract — paired commit (commit 2):**

- `server/internal/ws/events.go` — fields `TeamAFinalScore`, `TeamBFinalScore` on `MatchEndPayload` and `MatchAbandonedPayload`; JSON tags mirror
- `client/src/shared/types/wsEvents.ts` — payload fields `teamACardPoints`, `teamBCardPoints`, `teamADeclPoints`, `teamBDeclPoints`, `teamAHandTotal`, `teamBHandTotal`, `teamAMatchScore`, `teamBMatchScore`, `teamAFinalScore`, `teamBFinalScore`

**Frontend — types, stores, dispatch (commit 3):**

- `client/src/shared/types/gameTypes.ts` — `team: "teamA" | "teamB"`; add `TeamString = "teamA" | "teamB"` exported type; add `teamStringForIndex(i: 0 | 1): TeamString` helper
- `client/src/shared/api/matches.ts` — response type field renames mirror backend
- `client/src/shared/stores/gameStore.ts` + test, `roomLobbyStore.ts` + test — literal updates
- `client/src/shared/hooks/useWsDispatch.ts` + test — payload field reads use new names
- `client/src/shared/hooks/useReconnectionRedirect.test.tsx` — fixtures use new literals

**Frontend — styles & i18n (commit 4):**

- `client/src/index.css` — CSS variables `--color-team-a` / `--color-team-b` (hex unchanged from prior rename)
- `client/src/shared/i18n/en.json` — add shared `team` namespace (`us`, `them`, `a`, `b`); legacy `lobby.roomDetail.team*`, `lobby.roomLobby.team*`, `game.score.*`, `game.declaration.team*`, `game.disconnect.matchAbandonedScores`, `profile.matchHistory.hand.failedContractTeam*` keys consume the new namespace (the historical red/blue-suffixed keys have been removed)
- `client/src/shared/i18n/sr.json` — mirror in Serbian Latin: `Mi`, `Oni`, `Tim A`, `Tim B`

**Frontend — UI components, label routing, viewer-team derivation (commit 5):**

- `client/src/features/game/GamePage.tsx` + test — derive `viewerTeam: TeamString | null` from `myPlayerSeat`; pass to all team-aware children
- `client/src/features/game/components/ScorePanel.tsx` + test — accept `viewerTeam`; relabel rows Us/Them; preserve team colors; props are `aScore`/`bScore`; add `data-team` attribute on each row
- `client/src/features/game/components/ScoreReveal.tsx` + test — accept `viewerTeam`; team-name helper returns Us/Them; failed-hand sentence uses Us/Them; payload reads
- `client/src/features/game/components/MatchResult.tsx` + test — accept `viewerTeam`; winner banner + final-score columns use Us/Them; payload reads
- `client/src/features/game/components/DeclarationReveal.tsx` + test — accept `viewerTeam`; "{{team}} declared" uses Us/Them based on `team === viewerTeam` (covers both partners)
- `client/src/features/game/components/CapotAnimation.tsx` + test, `TrumpIndicator.tsx` + test, `PlayerSeat.tsx` + test — Tailwind class renames; remove any legacy literal team-string reads; add `data-team`
- `client/src/features/lobby/RoomDetailPreview.tsx` + test — Team A / Team B headers; literal updates
- `client/src/features/profile/MatchHistory.tsx` + test — relabel R/B chips, failed-hand chip, score line; **predicate: viewer participated → Us/Them; otherwise Team A/B** (refined per Sally); field reads use new names; Tailwind rename; add `data-team`

**Frontend — cross/diamond seat layout (commit 6):**

- `client/src/features/lobby/RoomLobby.tsx` + test — replace `SEAT_LAYOUT` 2x2 grid with diamond layout (`top` / `right` / `bottom` / `left` slots); compute display rotation so the viewer's seat sits at the bottom once seated, default rotation puts seat 0 at bottom; headers always read "Team A"/"Team B" (no flip); 300ms ease-out reorientation animation on seat selection; "your seat" micro-label appears at bottom slot ≤200ms after rotation completes; `prefers-reduced-motion: reduce` users get an instant snap with the label; preserve `data-testid="player-seat-{N}"` (seat index, server-canonical) and add `data-testid="seat-position-{south|north|east|west}"` for visual position; mobile portrait <640px collapses to vertical 2-column stack with same testids and team-color borders; add `data-team` attribute on each tile

**New tests — contract + invariants (commit 7):**

- `server/internal/ws/events_contract_test.go` (new) — Go test that marshals one of every event payload struct with representative fixtures, writes the JSON envelopes to `server/internal/ws/testdata/events/*.json` (golden files). Failure mode: any future field/tag change updates the goldens and fails the diff.
- `client/src/shared/types/wsEvents.contract.test.ts` (new) — Vitest test that loads each Go-produced golden from `server/internal/ws/testdata/events/`, runs them through a runtime parser (Zod or hand-rolled discriminator — see Ask First) for `WSEvent`, asserts each parses cleanly. Failure mode: any TS-side type drift fails the parse.
- `client/src/shared/i18n/i18n.parity.test.ts` (new) — single Vitest assertion that `Object.keys(en)` deep-equals `Object.keys(sr)`. Catches missing translation keys.
- `client/src/features/lobby/RoomLobby.diamond.test.tsx` (new) — four parameterized cases (viewerSeat ∈ {0,1,2,3}); for each, render lobby in seated state and assert via `getBoundingClientRect`: seat at `seat-position-south` = viewer; `seat-position-north` = partner; `seat-position-east` and `west` = opponents; partner seat shares team with viewer. Tests observable invariant, not the rotation algorithm.
- `client/src/features/lobby/RoomLobby.locale.test.tsx` (new) — for both `en` and `sr` locales, assert no element inside the seat tiles has `scrollWidth > clientWidth` (no text overflow). Catches Serbian-only layout breaks.
- `.github/workflows/ci.yml` (or equivalent) + `scripts/lockfile-old-team-tokens.sh` (new) — CI step that greps for the enumerated old identifiers (full list in spec verification section); fails the build if any are reintroduced. Permanent gate.

## Tasks & Acceptance

**Execution — commit-by-commit, build green at each commit boundary** (Amelia's order):

- [x] **Commit 1 — backend rename + DB column rename** (atomic). Go code, gorm tags, JSON tags, struct fields, constants, all server tests, testfixtures, migrations 000006 + 000009 (in place), `go test ./...` green. After this commit: server compiles, all Go tests pass, DB schema matches new code on a fresh `make migrate`.
- [x] **Commit 2 — WS contract paired**. `server/internal/ws/events.go` JSON tags + struct fields renamed. `client/src/shared/types/wsEvents.ts` field renames. No other client code touched yet — TS will fail to compile until commit 3 lands. (Acceptable because solo-dev / master / no live sessions per user.)
- [x] **Commit 3 — frontend types, stores, hooks, dispatch + their tests**. `gameTypes.ts`, `matches.ts`, all stores, all hooks, all tests using new literals + new payload field names. `tsc --noEmit` green; `npx vitest run` green for non-component layers.
- [x] **Commit 4 — CSS tokens + Tailwind utility renames + i18n**. `index.css`, all Tailwind class references in component files, `en.json` + `sr.json` shared `team` namespace. Visual: nothing user-visible has flipped yet — i18n keys now resolve to "Us"/"Them"/"Team A"/"Team B" but components still need to consume them. Actually visible relabel happens once components are updated in commit 5. Treat this commit as "tokens and strings only".
- [x] **Commit 5 — UI relabel + viewer-team derivation + `data-team` attributes**. `GamePage.tsx` derives `viewerTeam`; passes to `ScorePanel`, `ScoreReveal`, `MatchResult`, `DeclarationReveal`. `RoomDetailPreview` uses Team A/B. `MatchHistory` uses Us/Them when viewer participated, Team A/B otherwise. All affected components emit `data-team` attribute. All component tests updated. `make test` fully green.
- [x] **Commit 6 — cross/diamond seat layout in `RoomLobby.tsx`**. Diamond positions (top/right/bottom/left), viewer-rotation, 300ms transition, "your seat" micro-label, mobile <640px stack fallback, both testid layers (`player-seat-{N}` server-canonical + `seat-position-{south|north|east|west}` visual). Headers stay Team A/B (no flip). Component test updated.
- [x] **Commit 7 — new tests + CI lockfile**. `events_contract_test.go` + golden fixtures, `wsEvents.contract.test.ts`, `i18n.parity.test.ts`, `RoomLobby.diamond.test.tsx` (4 parameterized cases), `RoomLobby.locale.test.tsx`, CI lockfile script for forbidden tokens.
- [x] **Commit 8 — final sweep + manual smoke**. Run the lockfile script locally; confirm zero hits across `client/src`, `server/internal`, `server/migrations`, `*.json`, `*.md`, `*.sql`, `__snapshots__/*.snap`, `*.stories.*`, `*.mdx`. Manual two-client smoke: full hand to declared winner, one client refreshes mid-hand to exercise reconnect/state-restore, switch language to `sr` and verify lobby + game labels.

**Acceptance Criteria (numbered):**

- **AC-001** Project-wide regex `team[_-]?(red|blue)|[Tt]eam(Red|Blue)|red(Card|Decl|Hand|Match|Final)Points?|red(Score)|blue(Card|Decl|Hand|Match|Final)Points?|blue(Score)|red(Hand|Match)Total|blue(Hand|Match)Total|red_(card|decl|hand)_(points|total)|blue_(card|decl|hand)_(points|total)|team_(red|blue)_score|--color-team-(red|blue)|text-team-(red|blue)|border-team-(red|blue)|bg-team-(red|blue)` returns zero hits across `client/src`, `server/internal`, `server/migrations`, `*.json`, `*.md`, `*.sql`, `__snapshots__/*.snap`, `*.stories.*`, `*.mdx`.
- **AC-002** `cd client && npx tsc --noEmit` clean.
- **AC-003** `make lint` clean (Go `golangci-lint` + ESLint + Prettier).
- **AC-004** `make test` green (full Vitest + Go test).
- **AC-005** Drop local DB and `make migrate` succeeds; `\d matches` shows columns `team_a_score`/`team_b_score`; `\d hand_results` shows `a_card_points`, `b_card_points`, `a_decl_points`, `b_decl_points`, `a_hand_total`, `b_hand_total`.
- **AC-006** `events_contract_test.go` produces golden JSON files; `wsEvents.contract.test.ts` parses every golden through the TS runtime parser without error. (Drift gate.)
- **AC-007** `i18n.parity.test.ts` passes — every key in `en.json` exists in `sr.json` and vice versa.
- **AC-008** `RoomLobby.diamond.test.tsx` — for each viewerSeat ∈ {0,1,2,3}: `data-testid="seat-position-south"` element matches viewer's username; `north` is partner; `east`+`west` are opponents; partner shares viewer's team via `data-team` attribute.
- **AC-009** `RoomLobby.locale.test.tsx` — in `en` and `sr`, no seat-tile text element has `scrollWidth > clientWidth`.
- **AC-010** Manual smoke: viewer on teamA sees ScorePanel row labelled "Us" in team-A color; viewer on teamB sees their row labelled "Us" in team-B color. DeclarationReveal shows "Us declared" when partner declares.
- **AC-011** Manual smoke: viewer takes a non-default seat; observes 300ms reorientation animation; "your seat" label appears at bottom slot. With `prefers-reduced-motion: reduce` set in the browser, observes instant snap with immediate label.
- **AC-012** Manual smoke: resize browser to <640px portrait; observes vertical 2-column stack with team-color borders; same `data-testid="player-seat-{N}"` selectors resolve.
- **AC-013** Manual smoke: language switched to `sr`; lobby + ScorePanel + MatchResult + MatchHistory all render translated strings; no visible text overflow on the diamond.
- **AC-014** Manual smoke: full hand played to scored hand; reconnect-mid-hand from one client; state restores correctly with new field names.
- **AC-015** CI lockfile script (`scripts/lockfile-old-team-tokens.sh`) wired into the CI workflow; manually run and confirm pass on this branch and fail when a deliberately-injected `team_red` is reintroduced (verify locally).

## Design Notes

**Naming convention summary (2026-05-01 follow-up — full `team_`/`team`/`Team` prefix on compound names):**

| Form                 | Token                                                         |
| -------------------- | ------------------------------------------------------------- |
| String literal value | `"teamA"` / `"teamB"`                                         |
| Go constant          | `TeamA` / `TeamB` (numeric `0` / `1`)                         |
| Go struct field      | `TeamAScore`, `TeamACardPoints`, `TeamAHandTotal`             |
| JSON / TS camelCase  | `teamAScore`, `teamACardPoints`, `teamAHandTotal`             |
| SQL column           | `team_a_score`, `team_a_card_points`, `team_a_hand_total`     |
| CSS variable         | `--color-team-a` / `--color-team-b`                           |
| Tailwind utility     | `text-team-a` / `text-team-b` / `border-team-a` / `bg-team-a` |
| HTML attribute hook  | `data-team="teamA"` / `"teamB"`                               |

**Wire format pin:** `PlayerState.team` is the string `"teamA"` / `"teamB"`. Team-index payload fields (`winnerTeam`, `capotTeam`, `lastTrickTeam`, `contractingTeam`, `surrenderProposerTeam`) stay integers `0` / `1`. Conversion lives in:

- Go: `game.TeamStringForIndex(int) string` and `game.TeamIndexForString(string) int`
- TS: `teamStringForIndex(i: 0 | 1): TeamString`

Every team-string ↔ team-index conversion site goes through these. Grep for any other site after rename.

**Diamond layout (post-seat, viewer at bottom):**

```text
        ┌─ top ─┐         ← partner
        │       │
┌─left─┐         ┌─right─┐  ← opponents
│      │         │       │
        ┌bottom─┐         ← viewer
        │       │
```

Vertical axis = viewer's team; horizontal axis = opponents. Each tile keeps `border-team-a`/`border-team-b` based on its team and carries `data-team="teamA"|"teamB"`. Pre-seat default rotation: seat 0 at bottom, 2 at top, 1+3 sides. Headers always read "Team A" / "Team B" while in the lobby (no flip during seat selection — Sally's UX rule).

**Viewer team derivation (single rule, frontend only):**

```ts
const viewerTeam: TeamString | null =
  myPlayerSeat === null ? null : myPlayerSeat % 2 === 0 ? "teamA" : "teamB";
```

**Label resolution rule:**

- Viewer participated in the rendered context (active match, own past match): `team === viewerTeam ? t('team.us') : t('team.them')`
- Viewer is observer / no participation context (lobby, room browse): `team === 'teamA' ? t('team.a') : t('team.b')`

**DeclarationReveal — both partners see "Us":** keys off team match, not seat equality. `payload.winnerTeam` is the integer index; convert via `teamStringForIndex` and compare to `viewerTeam`. When viewer's partner declares, their team matches → "Us declared". When opponents declare → "Them declared".

**Diamond rotation animation:** CSS `transition: transform 300ms ease-out` on each seat tile (or grid-area). Trigger: `viewerSeat` change. Post-rotation, fade-in the "your seat" label (`opacity 0 → 1, 200ms`). Reduced-motion: `transition: none` and label opacity goes to 1 instantly.

**Mobile portrait fallback (<640px):** Tailwind `sm:` breakpoint. Below `sm`, the diamond grid (with `grid-template-areas: "_ top _" "left _ right" "_ bottom _"` or similar) collapses to `grid-template-columns: 1fr 1fr; grid-template-rows: auto auto;` — back to the original 2x2. Same testids resolve. Team color borders carry the partnership signal.

**`data-team` attribute hook:** every team-bearing element (seat tiles, score rows, score reveal team columns, match-result winner banner, declaration-reveal banner, capot animation text, trump indicator team chip, match-history hand chips and score columns) carries `data-team="teamA"` or `"teamB"`. The next task uses these as the single styling hook to attach gold/silver palette + a non-color partnership signal (pattern, icon, border style) for color-blind users.

**Migration edit-in-place — single-environment scoped exception:** justified by user's solo-dev / master-only / pre-production state. Drop and re-run `make migrate` on local DB to validate. Do not adopt this pattern post-launch.

**Color hex unchanged this task:** `--color-team-a` keeps the value `#ff4d4d` (formerly Red), `--color-team-b` keeps `#4d9fff` (formerly Blue). Hex flip to gold/silver + non-color partnership signal is the next task. Visual continuity is intentional — the rename should be invisible to a user playing one match across the boundary.

## Verification

**Commands:**

- `cd client && npx prettier --write .` — clean exit (per memory rule before any commit)
- `cd client && npx tsc --noEmit` — 0 errors
- `make lint` — 0 ESLint / Prettier / golangci-lint errors
- `make test` — all Vitest + Go tests pass (including new contract / parity / diamond / locale tests)
- `make migrate` against fresh DB — schema applies clean with new column names
- `bash scripts/lockfile-old-team-tokens.sh` — exits 0 (no forbidden tokens). Verify by deliberately reintroducing `team_red` in a scratch branch — script must fail.
- `grep -rIE "team[_-]?(red|blue)|[Tt]eam(Red|Blue)|red(Card|Decl|Hand|Match|Final)Points?|blueScore|red_card_points|--color-team-(red|blue)|text-team-(red|blue)" client/src server/internal server/migrations` — 0 hits
- 2026-05-01 follow-up: `grep -rIEn "\b(ACardPoints|BCardPoints|ADeclPoints|BDeclPoints|AHandTotal|BHandTotal|AFinalScore|BFinalScore|aCardPoints|bCardPoints|aDeclPoints|bDeclPoints|aHandTotal|bHandTotal|aMatchScore|bMatchScore|aFinalScore|bFinalScore|a_card_points|b_card_points|a_decl_points|b_decl_points|a_hand_total|b_hand_total)\b" client/src server/internal server/migrations` — 0 hits (lockfile script extended to enforce this going forward).

**Manual checks (run in order):**

1. `make dev`, log in as test users A and B, both join the same room. A takes seat 0; B takes seat 2 (same team). A's lobby shows seat 0 at bottom, seat 2 at top, headers "Team A" / "Team B". B's lobby shows seat 2 at bottom, seat 0 at top, headers also "Team A" / "Team B". Both observe 300ms rotation animation when seating; "your seat" label appears at bottom slot.
2. Have a third user C take seat 1. Game auto-starts. ScorePanel: A and B see their team row labelled "Us" in team-A color; C sees their team row labelled "Us" in team-B color.
3. Play a full hand. At hand-end ScoreReveal: each viewer sees Us/Them rows correctly. If a player declares, DeclarationReveal shows "Us declared" to both partners and "Them declared" to the opposing pair.
4. Refresh client A mid-hand. State restores; new payload field names (`aCardPoints` etc.) deserialize correctly; no console errors.
5. Resize browser to <640px portrait. Lobby seat layout collapses to 2-column stack; team colors visible; testids still resolve.
6. Switch language to Serbian (`sr`). All labels translate: lobby = "Tim A" / "Tim B", in-game = "Mi" / "Oni" / "Mi pobeđuje", match history = "Mi" / "Oni" for own matches.
7. Inspect a `event:hand_scored` WS frame in browser devtools; confirm field names are `aCardPoints`, `bCardPoints`, etc.
8. Set `prefers-reduced-motion: reduce` in browser dev tools; take a seat; confirm instant snap (no animation) with immediate "your seat" label.
