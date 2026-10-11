---
title: "Center the declaration reveal dialog"
type: "bugfix"
created: "2026-04-29"
status: "done"
route: "one-shot"
---

# Center the declaration reveal dialog

## Intent

**Problem:** The declaration reveal panel anchored to the winning declarer's seat (one of four compass-relative positions). This made the panel feel like it "moved" toward a specific player, which the user found visually noisy and inconsistent with treating the reveal as a shared, table-centred announcement.

**Approach:** Replace the per-seat `PANEL_POSITIONS` map and `compassOffset` lookup with a single centred container (`absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2`) and drop the now-unused `myPlayerSeat` prop end-to-end. `BelotReveal` is intentionally untouched.

## Suggested Review Order

**Centering change**

- Container now centers via classic translate-50% trick instead of per-seat anchor.
  [`DeclarationReveal.tsx:44`](../../client/src/features/game/components/DeclarationReveal.tsx#L44)

- `myPlayerSeat` removed from props (was only used by the old compass logic).
  [`DeclarationReveal.tsx:9`](../../client/src/features/game/components/DeclarationReveal.tsx#L9)

**Call-site cleanup**

- GamePage no longer threads `myPlayerSeat` into the reveal.
  [`GamePage.tsx:640`](../../client/src/features/game/GamePage.tsx#L640)

**Test lock-in**

- New test asserts the centering classes are applied regardless of the winning declarer's seat — locks the new behavior against silent regressions.
  [`DeclarationReveal.test.tsx:57`](../../client/src/features/game/components/DeclarationReveal.test.tsx#L57)

- Existing tests stripped of the obsolete `myPlayerSeat={0}` arg.
  [`DeclarationReveal.test.tsx:47`](../../client/src/features/game/components/DeclarationReveal.test.tsx#L47)
