---
title: 'Mobile dialog font scaling + score-reveal i18n fixes'
type: 'bugfix'
created: '2026-07-19'
status: 'done'
route: 'one-shot'
---

# Mobile dialog font scaling + score-reveal i18n fixes

## Intent

**Problem:** On phone screens the classic-overlay dialogs render at desktop type sizes, so the score-reveal match strip runs out of room and "1064 · 1006 / 1001" wraps ugly next to the Continue button; the mk score-reveal subtitles also used neuter "Извадено/Паднато" (should be feminine, agreeing with implied „рака") and the "твојот тим зеде {{suit}}" phrasing read poorly.

**Approach:** Step the shared dialog chrome down one type/padding notch below Tailwind's `sm` breakpoint — ClassicPanel (title, subtitle, header/body padding) and ClassicButton (font, padding) — so every dialog shrinks on mobile; tighten the ScoreReveal strip and make the score line `whitespace-nowrap` (with `flex-wrap` + `ml-auto` as graceful ultra-narrow fallback). Fix mk grammar (Извадена/Падната) and reword the held-subtitles in all four locales to the "you took trump on {{suit}}" form (mk „вие зедовте адут на", hr/sr „vi ste zvali adut"). Desktop stays pixel-identical (arbitrary px utilities emit no line-height).

## Suggested Review Order

1. [ClassicPanel.tsx](../../client/src/features/match/components/overlay/ClassicPanel.tsx) — shared chrome: responsive title/subtitle sizes, header padding, responsive default body padding (explicit `bodyPadding` still opts out at all breakpoints).
2. [ClassicButton.tsx](../../client/src/features/match/components/overlay/ClassicButton.tsx) — font/padding moved from inline style to responsive classes; inline `style` overrides (PauseOverlay) still win.
3. [ScoreReveal.tsx](../../client/src/features/match/components/ScoreReveal.tsx) — brass-strip tightening, `whitespace-nowrap` on the score line, `flex-wrap`/`ml-auto` fallback, responsive row type, refreshed wording comments.
4. [mk.json](../../client/src/shared/i18n/mk.json) — Извадена/Падната + „вие зедовте адут на {{suit}}" / „тие зедоа адут на {{suit}}".
5. [en.json](../../client/src/shared/i18n/en.json), [hr.json](../../client/src/shared/i18n/hr.json), [sr.json](../../client/src/shared/i18n/sr.json) — same subtitle rewording per locale.
6. [ScoreReveal.test.tsx](../../client/src/features/match/components/ScoreReveal.test.tsx) — i18n mock updated to mirror the new en wording.
