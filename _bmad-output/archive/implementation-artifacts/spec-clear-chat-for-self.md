---
title: "Per-user clear chat (icon + /cc command)"
type: "feature"
created: "2026-04-29"
status: "done"
route: "one-shot"
---

# Per-user clear chat (icon + /cc command)

## Intent

**Problem:** Players had no way to wipe a noisy chat panel for themselves — the existing `clear*` store actions only fire on system events (match end, room exit), so a user stuck with a flooded lobby/match/room chat had to live with it until navigation.

**Approach:** Added two purely client-side affordances to the shared `ChatPanel`: a trash-icon button in the header and a `/cc` slash command (case-insensitive, exact match after trim). Both call the existing channel-scoped `clearGlobal` / `clearMatch` / `clearRoom` actions — local-only, no broadcast, other users keep their view.

## Suggested Review Order

**Entry point — clear behaviour**

- Slash-command branch: bare `/cc` clears, `/cc hello` is sent literally.
  [`ChatPanel.tsx:91`](../../client/src/features/chat/ChatPanel.tsx#L91)

- Helper bundles clear + draft reset + focus handoff so keyboard users don't strand on `<body>`.
  [`ChatPanel.tsx:59`](../../client/src/features/chat/ChatPanel.tsx#L59)

**Store wiring**

- Channel-aware `clearChannel` selector — same pattern as `messages` / `markSent` selectors above.
  [`ChatPanel.tsx:46`](../../client/src/features/chat/ChatPanel.tsx#L46)

**UI affordance**

- Trash icon button in the header; auto-disables when `messages.length === 0`.
  [`ChatPanel.tsx:135`](../../client/src/features/chat/ChatPanel.tsx#L135)

- `inputRef` attached so the post-clear refocus actually lands.
  [`ChatPanel.tsx:181`](../../client/src/features/chat/ChatPanel.tsx#L181)

**i18n**

- New `clearAriaLabel` + `clearTooltip` keys (English).
  [`en.json:80`](../../../client/src/shared/i18n/en.json#L80)

- Serbian counterparts.
  [`sr.json:80`](../../../client/src/shared/i18n/sr.json#L80)

**Tests**

- Button clears the active channel only — global + room untouched when match is cleared.
  [`ChatPanel.test.tsx:296`](../../client/src/features/chat/ChatPanel.test.tsx#L296)

- Bare `/cc` clears; `/cc hello` is sent as a normal message.
  [`ChatPanel.test.tsx:237`](../../client/src/features/chat/ChatPanel.test.tsx#L237)

- Clear resets the latched `hasSent*` placeholder back to the channel invitation.
  [`ChatPanel.test.tsx:335`](../../client/src/features/chat/ChatPanel.test.tsx#L335)

- Clear button hands focus back to the input (a11y regression guard).
  [`ChatPanel.test.tsx:367`](../../client/src/features/chat/ChatPanel.test.tsx#L367)
