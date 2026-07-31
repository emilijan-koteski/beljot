---
baseline_commit: b476334
---

# Spec: Honour System Redesign

Status: review

Source design: Claude Design project `8fb598bd-6702-4f1e-ada9-e6c4336901e9`, file `Honour System - Redesign.html` (+ `honour-redesign.css`), nine surfaces R1-R9.

## Why

Stories 9.7 and 9.8 shipped the honour system correct but **illegible**. Nothing anywhere said what honour measures; the tier word was `sr-only` at every width, so a declining score looked identical to a healthy one; the waiting room showed the gate nowhere at all even though the lobby card showed it; and a barred player discovered the gate by clicking Join and reading a toast.

This spec is presentation only. **The server contract, the five tier bands (95/85/70/50), the Beta(4,1) prior of 80 and the 5-match New Player floor are all unchanged** — verified against `server/internal/user/honor.go` before any code was written.

Not a story: Epic 9 is complete, and 9.7/9.8 are `done` and accurate about what they shipped. This follows the project's existing `spec-*.md` convention for cross-cutting work.

## Binding decisions

### D1 — Developed ON `feat/9-7-honor-score-system`, per user direction 2026-07-31

9.7 + 9.8 + this redesign ship as ONE feature/PR. 9-7 and 9-8 stay `done`: they describe what they shipped truthfully, and this spec is what changed afterwards.

### D2 — Per-seat honour OVERRIDES 9.8's Epic 11 deferral (PO decision 2026-07-31)

9.8's scope-guardrail table defers "Cross-player honor visibility (seeing a seatmate's honor in the room)" to **Epic 11, Story 11.3**. R6 builds it now. This is a deliberate reversal, not an oversight.

Rationale accepted: honour is already public — `ProfileResponse` exposes the same score and tier to anyone — so the roster reveals nothing new, and the roster is where the information finally does work (you can see who you are about to partner with, and which seat will block Start). Only score + tier cross the wire; the counts and trend deliberately do not.

### D3 — This redesign OVERRIDES 9.8's guardrails on 9.7's surfaces (PO decision 2026-07-31)

9.8's guardrails forbid touching `HonorPanel.tsx`, `honor.ts` and the honour display surfaces. This spec **deletes HonorPanel** and **rewrites honor.ts's presentation layer**. Those guardrails existed to stop 9.8 drifting into 9.7's territory; a deliberate redesign is the case they were not written for.

### D4 — Three drawn items are NOT built, because the client provably cannot compute them

| Dropped | Why | What it would need |
| --- | --- | --- |
| TopBar "2 / 5" newcomer counter | The auth envelope carries only `honorScore` / `honorTier` / `isNewPlayer` — not the counts (`auth/handler.go:60-62`). TopBar keeps the "New Player" label; the **profile** band does show "2 / 5", because `ProfileResponse` has the counts. | 2 struct fields + 2 assignments, no new query |
| Recovery estimate ("finish 4 more matches") | Underdetermined. The client knows only `s = 100(C+4)/(C+4+4A+1)`, which fixes the RATIO but not `C` — and the decayed weights plus `honorDecayedAt` are all `json:"-"` (`user/model.go:44,45,49`). Substituting the raw undecayed totals assumes no decay and would print a number contradicting the score on the same screen. | `honorMatchesToNextTier` server-side, or expose the weights |
| R2's "two finished matches keeps you Exemplary" | Same projection. Replaced with a plain standing line. | as above |

All three are recorded in `deferred-work.md` with the exact field required.

### D5 — Five factual errors in the design, corrected rather than implemented

1. **There is no XP row on the result overlay.** R7 claims "coins and XP already report themselves here". `MatchResult.tsx` has a coin row only; XP surfaces via `LevelUpDialog` outside the match. Honour is the **second** row, not the third.
2. **The abandonment overlay has no settlement rows at all** — it is `ReconnectOverlay` in abandoned mode with no coin row, so "same treatment" would mean building the list. Not done; the honour row is on `MatchResult` only.
3. **The abandoning player never receives `event:honor_updated`** (`Hub.SendToUser` is an unqueued no-op for an absent user). So "You dropped · 96 → 91" cannot be driven by the WS event. The stash therefore shows a movement only when the event actually arrives; the abandoning player sees none. Recorded as deferred.
4. **`DurationSlider` could not express tier ticks** — the interval was a hardcoded `v += 10`, so 0-100 gave 0,10,…,100, never 50/70/85/95; the fill was a hardcoded gradient; there was no marker API. Extended additively (see AC5).
5. **`bg-surface-2` is not a real utility** — `--surface-2` has no `--color-*` alias, so `HonorPanel`'s count tiles had been rendering transparent since 9.7. Not reproduced; the band uses real tokens.

### D6 — One shield component, not two exported maps

Tier colour was duplicated in `HonorPanel.tsx` and `TopBar.tsx` and the two **had already drifted** — they disagreed on `fair` — with nothing in TypeScript able to notice. This redesign adds five more consumers, so the duplication had to stop first. `HONOR_TIER_COLOR` now lives once in `honor.ts` (the coinGold.ts precedent) and is consumed through `<HonorShield>`, which pairs it with the tier's GLYPH. A component rather than two maps makes the drift structurally impossible.

### D7 — The tier ramp re-roots inside `.game-table` under the same names

`--h1`…`--h5` are declared in `:root` and REDECLARED with felt-lifted values inside `.game-table`, exactly as `--accent` and `--ink` already are. So one scope-blind map serves both parchment and felt, and R7's in-match surfaces theme themselves with no prop and no branch.

## Acceptance criteria

**AC1 — Tier ramp.** Five tokens `--h1`…`--h5` (worst→best) plus `-soft` and `-line` variants, cool→warm: ember → amber → stone → felt → deep teal. Gold is NOT in the ramp (a gold top tier sat one hue from the amber warning tier and the two were mistakable at 16px); Exemplary keeps brass as a chip hairline, ornament only. Felt variants re-rooted in `.game-table` per D7. Colour never alone: `HONOR_TIER_ICON` varies the shield shape too.

**AC2 — One source of truth.** `honor.ts` exports `HONOR_TIER_BANDS` (derived from the floors, so gradient, ticks and bucketing cannot disagree), `HONOR_TIER_COLOR/_SOFT/_LINE`, `honorRoomIsGated`, `honorQualifies` and `HONOR_NEW_PLAYER_MIN_MATCHES`. The local maps in `HonorPanel` and `TopBar` are gone.

**AC3 — R1 TopBar chip.** Tier word visible from `lg`, `sr-only` below it; per-tier glyph; the chip is a `button` opening R2; brass hairline for Exemplary. The `lg` gate is the measured one from 9.7's E2E pass and is preserved — at 640-1023px the row already carries nav links plus the username pill, and a word there made the chip wrap.

**AC4 — R2 explainer.** Shared controlled dialog, same shell as the ejection modal, opened from the chip and the profile band. Four facts, including the two players actively get wrong: **surrender counts as finished**, and **old abandonments decay**. Deep-links to `/rules#honour`.

**AC5 — R5 slider.** `DurationSlider` gains `ticks`, `fillStyle`, `valueText`, `marker` — all optional and defaulted, so the timer call site is untouched. Honour uses ticks on the tier boundaries, a tier-coloured fill, 0 rendered as "Anyone", and the host's own score marked on the track (gated on `honorKnown`, so an absent envelope marks nothing). **Both hints deleted** — `minHonorHint` and `allowNewPlayersHint`. Open Question 6's steer survives in the ticks and the tier caption rather than in prose.

**AC6 — R6 waiting room.** `min_honor` / `allowNewPlayers` badges in the existing badges row. Per-seat shield + score on each `SeatTile`, showing THAT PLAYER'S tier — a seat at 71 is Fair and gets the stone shield, never a red X. Being under the room's bar is a property of the ROOM and is drawn by an ember ring on the tile instead. Start goes `disabled` with the reason in `title` and keeps its normal label.

**AC7 — R6 server, lenient.** `RoomPlayer` gains `HonorScore *int` / `HonorTier *string` (pointers, so a real 0 is distinguishable from "not read"), hydrated by `attachPlayerHonor` via the already-injected `HonorForUsers`. DELIBERATELY LENIENT, unlike the gate's reader: a failed read, a missing row or a nil service yields no shield, never a failed request. Bots are skipped. **Zero WS drift-gate touchpoints** — verified: `git diff` over `events_contract_test.go`, `testdata/events/`, `wsEvents.schemas.ts`, `wsEvents.contract.test.ts`, `ws/events.go` and `wsEvents.ts` is empty.

**AC8 — R4 lobby.** The honour chip's shield is tinted by the tier the REQUIREMENT falls in, not the viewer's standing, so it stays a property of the room. The verdict lives in the button: Join becomes **Locked** and fires no request. Numbers only in `title`. New "I qualify" filter chip, pure client.

**AC9 — R8 ejection.** To-scale comparison on a shared 0-100 axis with the numbers alongside, plus a **door out** — "Rooms I qualify for", routing to the lobby with the filter preset. No recovery estimate (D4).

**AC10 — R7 in-match.** Viewer-relative honour note on the reconnect overlay (reassurance when it is someone else, a reason to stay when it is you — the one screen where the cost is still avoidable). Honour row on `MatchResult`, driven by a client-side previous-score stash, reset in **both** the `match_end` and `match_abandoned` handlers exactly as `useWsDispatch`'s own comment demanded. Surrender dialog untouched by design.

**AC11 — R9 rules.** New `honour` section in all four of `content/{en,mk,hr,sr}.ts`, reusing existing block kinds. `rulesContent.parity.test.ts` enforces identical ids and order across locales.

**AC12 — i18n.** 886 keys × 4, 1:1 with matching interpolation sets, mk all-Cyrillic, **no em dash in mk/sr/hr** (verified: 0 in each, and the pre-existing count of 1 in each rules content file is unchanged).

## Gates (run, not inherited)

- `gofmt -l .` clean except the pre-existing `internal/auth/profile_identity_handler_test.go`
- `go vet ./...` clean · `golangci-lint run ./...` clean (v1.64.8) · `go test ./...` all packages ok, with the DB-backed honour tests **PASSING not skipping** (dev DB 5433)
- `tsc -p tsconfig.build.json --noEmit` clean · `eslint .` clean · `prettier --check .` clean
- `vitest run` **101 files / 1118 tests** (baseline 1096 → +22)
- i18n scripted check: 886 keys × 4, 0 missing/extra, 0 placeholder mismatches, 0 em dash in sr/mk/hr
- Drift gate: zero WS contract files touched

## Not done, and why

- **Manual E2E has NOT been run for this redesign.** Every gate above is automated. 9.6 found two real bugs in manual E2E *after* review passed, and this change touches nine surfaces including two responsive breakpoint decisions (the `lg` tier word, the 320px badges row) that automated tests cannot judge. **Run it before merge.**
- The design's own "Suggestions" section beyond the nine surfaces: grace warning before ejection, honour history sparkline, preset bars above the slider, bots-don't-count confirmation, honour-as-reward, and the regional rename ("Pouzdanost / Доверба"). The rename would touch every honour key in four locales and the domain vocabulary of two shipped stories — its own decision.
