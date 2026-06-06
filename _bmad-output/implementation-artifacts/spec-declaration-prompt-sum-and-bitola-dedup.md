---
title: "Declaration prompt: show all groups with personal sum, Bitola single-use dedup"
type: "feature"
created: "2026-04-17"
status: "done"
context: []
baseline_commit: "8f05a5d6f127acbe39ef5eab829e7ea38d631d43"
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem (two parts):**

1. The declaration modal shows every detected group but does not display a **personal sum** — a player with a tierce (20) + a carré (100) sees "Tierce — 20 pts" and "Carré — 100 pts" but no "Total: 120". Players want an at-a-glance commit-or-skip decision.
2. The detection logic in both [server/internal/game/declarations.go](server/internal/game/declarations.go) and [client/src/features/game/lib/declarations.ts](client/src/features/game/lib/declarations.ts) allows a single card to contribute to two declarations (e.g. J♠ in both a 9♠-T♠-J♠-Q♠ sequence and a 4×J). This matches the **Croatian variant**. For the **Bitola variant** (currently the only supported variant), a card may only be part of one group — when two groups conflict on a shared card, keep the higher-value group, drop the other.

**Approach:**

- **Detection (server + client helpers):** after `detectDeclarations` returns the raw set, run a Bitola-variant deduplication pass that removes any group that shares at least one card with a higher-value group. Higher-value always wins; ties are impossible in practice (min FoaK = 100 > any sequence value unless quint = 100, but quint and a FoaK cannot share a card because sequence is single-suit and FoaK spans all 4 suits; the only overlap case is tierce/quarte vs FoaK, where FoaK always wins).
- **Modal UI:** keep the per-group rows (cards + label + per-group points) exactly as they are today. Add a bold "Total: {sum} pts" footer row inside the modal, above the Declare/Skip buttons. Sum = the de-duplicated set's values.

## Boundaries & Constraints

**Always:**

- Keep the server as the source of truth for stored declarations. `handleDeclare` must persist the **de-duplicated** set.
- The client-side `detectDeclarations` (added for the prompt preview) must apply the same dedup pass — the modal must not show a group the server will discard.
- Dedup rule: among groups that share at least one card, keep the one with the highest `value`. Remove the rest. If multiple max-value groups share a card, keep the first encountered (stable).
- `detectDeclarations` output still lists each surviving group separately — do not merge them into a single "combined" declaration.
- Total row must match the sum of values shown in the modal (no hidden rounding, no rule deviation).

**Ask First:**

- Any change to how tiebreaks resolve between teams (existing `declarationBeats` chain in [server/internal/game/declarations.go:167](server/internal/game/declarations.go#L167) is out of scope here).
- Any change to the carré point values or sequence point values.
- Any rendering change in the _reveal_ flow (that's a separate spec — goal 2).

## Rule Reference

- **Bitola variant:** one card, one group. On conflict, higher-value group wins.
- **Croatian variant (deferred):** one card may appear in multiple groups; no dedup. Documented in [project-context.md](_bmad-output/project-context.md) (see companion docs update) — implementation deferred until Croatian variant work begins.
- Winning-team / losing-team comparison unchanged: per-team best declaration (by value, then tiebreakers) decides the team winner, winning team's team declarations are summed, losing team gets 0.

## Implementation Outline

**Server — [server/internal/game/declarations.go](server/internal/game/declarations.go):**

- Add `dedupBitola(decls []Declaration) []Declaration` below `detectDeclarations`. Pure function. Algorithm:
  1. Sort declarations by value descending (stable).
  2. Iterate in order, track a set of already-used card identities (rank+suit).
  3. Keep a declaration only if none of its cards appear in the used set. Add its cards to the used set on keep.
- In `detectDeclarations`, after building the raw list, return `dedupBitola(raw)`. No variant switch yet — the function is unconditionally Bitola because the server only supports Bitola today. Add a TODO comment pointing to the Croatian variant work.

**Client — [client/src/features/game/lib/declarations.ts](client/src/features/game/lib/declarations.ts):**

- Add the same `dedupBitola` pass after building the raw list. Same algorithm. No variant switch.

**Client — [client/src/features/game/components/DeclarationPrompt.tsx](client/src/features/game/components/DeclarationPrompt.tsx):**

- Compute `total = declarations.reduce((s, d) => s + d.value, 0)`.
- Render a new footer row between the declarations list and the button row. Structure:
  ```
  <div className="flex items-center justify-between border-t border-border pt-3 mb-4">
    <span className="font-body text-sm text-text-secondary">{t("game.declaration.total")}</span>
    <span className="text-accent font-display text-lg font-bold">{total} {t("game.declaration.pts")}</span>
  </div>
  ```
- New i18n keys: `game.declaration.total` → "Total" (EN) / localized equivalents. Update all existing locale files.

## Files

**New:** none.

**Modified:**

- `server/internal/game/declarations.go` — add `dedupBitola`, call it from `detectDeclarations`.
- `server/internal/game/declarations_test.go` — add tests: (a) tierce + FoaK sharing a J — FoaK kept, tierce dropped; (b) tierce + quarte where tierce ⊂ quarte — quarte kept (already held by current logic, verify); (c) two FoaK of different ranks — both kept (no card overlap); (d) a sequence with no shared card vs a FoaK — both kept.
- `client/src/features/game/lib/declarations.ts` — add `dedupBitola`.
- `client/src/features/game/lib/declarations.test.ts` — NEW FILE with mirror tests.
- `client/src/features/game/components/DeclarationPrompt.tsx` — add total footer.
- `client/src/features/game/components/DeclarationPrompt.test.tsx` — add assertion that the total appears and equals the sum of the shown groups' values.
- `client/src/shared/i18n/en.json`, `sr.json` (and any other locales) — add `game.declaration.total` key.

## Acceptance

- **Given** a hand with J♠,9♠,T♠,Q♠ and J♥,J♦,J♣ (quarte of spades valuing 50, FoaK jacks valuing 200, sharing J♠)
  **When** `detectDeclarations` runs
  **Then** only the FoaK is returned; the sequence is dropped.
- **Given** a hand with 7♠,8♠,9♠ (tierce 20) and 9♦,9♥,9♣,9♠ (FoaK 9s, 150, sharing 9♠)
  **When** `detectDeclarations` runs
  **Then** only the FoaK is returned.
- **Given** the modal shows two non-overlapping declarations summing to 70
  **Then** the footer reads "Total: 70 pts".
- **Given** only one declaration is detected
  **Then** the footer still shows it, reading the same value as the group row.
- **Given** `detectDeclarations` returns empty
  **Then** the modal is not rendered at all (existing behavior, confirm).

## Test Plan

- `go test ./server/internal/game/...` — all pass, new dedup tests green.
- `npx vitest run src/features/game/lib/declarations.test.ts` — new test file passes.
- `npx vitest run src/features/game/components/DeclarationPrompt.test.tsx` — existing tests still pass, new total-footer assertion passes.
- `npx tsc --noEmit` — clean.
- Manual: deal a hand with overlap (hard to force naturally) or write a test fixture. The modal shows only the winning-value group and the total matches.

</frozen-after-approval>

## Suggested Review Order

**Dedup rule (source of truth)**

- Server entry point — `detectDeclarations` now funnels through dedup before returning.
  [`declarations.go:108`](../../server/internal/game/declarations.go#L108)

- Bitola dedup algorithm — stable sort by value desc, track used card keys, keep first non-conflict.
  [`declarations.go:115`](../../server/internal/game/declarations.go#L115)

- Client mirror — identical contract, same output shape, keeps prompt and server in lock-step.
  [`declarations.ts:110`](../../client/src/features/game/lib/declarations.ts#L110)

- Client entry point — matches the server's single call site.
  [`declarations.ts:101`](../../client/src/features/game/lib/declarations.ts#L101)

**Prompt UI — personal sum**

- Total computed from the same `declarations` prop driving the per-group rows.
  [`DeclarationPrompt.tsx:28`](../../client/src/features/game/components/DeclarationPrompt.tsx#L28)

- Footer row rendered between groups and buttons per spec JSX.
  [`DeclarationPrompt.tsx:81`](../../client/src/features/game/components/DeclarationPrompt.tsx#L81)

**i18n**

- EN label "Total" added; reuses existing `pts` key.
  [`en.json:176`](../../client/src/shared/i18n/en.json#L176)

- SR label "Ukupno".
  [`sr.json:176`](../../client/src/shared/i18n/sr.json#L176)

**Tests**

- Server dedup cases cover all four spec scenarios plus the quarte-subsumes-tierce sanity check.
  [`declarations_test.go:775`](../../server/internal/game/declarations_test.go#L775)

- Client mirror tests — same scenarios, vitest-style.
  [`declarations.test.ts:53`](../../client/src/features/game/lib/declarations.test.ts#L53)

- Prompt total assertions — multi-decl sum + single-decl parity.
  [`DeclarationPrompt.test.tsx:47`](../../client/src/features/game/components/DeclarationPrompt.test.tsx#L47)
