---
title: "Belote / Rebelote: rank-specific prompt and seat-anchored card reveal"
type: "feature"
created: "2026-04-17"
status: "done"
context: ["spec-declaration-reveal-cards-and-timing.md"]
baseline_commit: "c0945b9"
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem (three parts):**

1. The Belote prompt currently reads a generic "Announce Belote?" regardless of whether the player is about to play the **Queen** or the **King** of trump. The rule distinguishes: playing the Queen first → "Belote"; playing the King first → "Rebelote".
2. The Belote announcement is silent client-side — [client/src/shared/hooks/useWsDispatch.ts:191-194](client/src/shared/hooks/useWsDispatch.ts#L191-L194) handles `event:belot_announced` as a no-op. Other players get no feedback about who just announced or which card triggered it.
3. The `event:belot_announced` payload ([server/internal/ws/events.go — BelotAnnouncedPayload](server/internal/ws/events.go)) carries only `{playerSeat, team}`. The card ID is needed to render the reveal; extend the event.

**Approach:**

1. **Prompt label:** the prompt title and button text must reflect the card being played. When the player is about to play the **Queen of trump**, prompt says "Announce Belote?". When about to play the **King of trump**, prompt says "Announce Rebelote?". Terminology is rank-bound, not play-order-bound.
2. **Server payload:** extend `event:belot_announced` to include `cardId` (the K or Q being announced). Session manager already knows — it fires off the `ActionAnnounceBelot` action, whose last-played card is the triggering card.
3. **Reveal:** Belote's reveal stays seat-anchored to the announcer (PANEL_POSITIONS in BelotReveal); the declaration reveal centers on the table since 2026-04-29 (spec-center-declaration-dialog). The Belote panel shows the single card (the K or Q of trump that triggered the announcement), with the text "Belote!" or "Rebelote!" above it. Auto-dismisses after 4s (1.5s reduced-motion). Non-blocking.

## Boundaries & Constraints

**Always:**

- Prompt for announce fires **only** on playing the first of K/Q of trump (existing `shouldPromptBelot` guard — `hasBelot(handBeforePlay)` requires both K+Q in hand before play). No prompt on the second card — already correct. Confirmed via [server/internal/game/declarations.go:294-310](server/internal/game/declarations.go#L294-L310).
- If the player skips the first K/Q prompt, they cannot announce later — already correct.
- The reveal shows **only the card that triggered the announcement**, not both K+Q. The other card is still in the announcer's hand and stays hidden until naturally played.
- Reveal is non-blocking: `pointer-events-none`, no modal, no focus trap.
- Reveal placement: same anchoring pattern as declaration reveal (seat-anchored via `compassOffset`). If both a declaration reveal and a Belote reveal could theoretically overlap in time, they won't in practice — declarations resolve at trick 2 start, Belote announce fires mid-trick on a K/Q play. No z-index conflict expected.
- Points still flow through existing `handleAnnounceBelot` → `HandPoints[team] += 20`. The reveal is purely visual; does not mutate score paths. ScorePanel's "+X this hand" line reflects the pending +20 naturally.
- `prefers-reduced-motion` reduces the reveal duration to 1.5s and drops slide-in animation.

**Ask First:**

- Any change to the announce/skip action semantics (still the same two actions).
- Any change to the point value (+20 for K+Q of trump held by same player).
- Any change that would reveal the second K/Q before it's actually played.

## Rule Reference

- **Belote (bela):** announced when playing the **Queen** of trump, provided the player also holds the King of trump in hand. +20 to the announcer's team.
- **Rebelote (re-bela):** announced when playing the **King** of trump, provided the player also holds the Queen of trump in hand. +20 to the announcer's team.
- The player picks one moment to announce — whichever K/Q they play first. If they decline at that moment, the bonus is forfeit; they cannot re-claim it when playing the other card.
- Points remain pending until hand end (failed-hand rules can transfer them).
- **Both variants (Bitola + Croatian future) behave identically** for Belote / Rebelote.

## Implementation Outline

**Server — [server/internal/ws/events.go](server/internal/ws/events.go):**

- Extend `BelotAnnouncedPayload` to include `CardID string` (JSON: `cardId`).

**Server — [server/internal/session/manager.go](server/internal/session/manager.go):**

- In the `case game.ActionAnnounceBelot, game.ActionSkipBelot:` branch (around line 477), include the triggering card ID in the `belot` map. The card is the last entry of `oldState.CurrentTrick` immediately before the announce action (the K/Q that was just played and whose post-play flow was paused on the Belote prompt). Capture it from `oldState.CurrentTrick[len(oldState.CurrentTrick)-1].Card.String()`.

**Client — [client/src/features/game/components/BelotPrompt.tsx](client/src/features/game/components/BelotPrompt.tsx):**

- Accept a new prop `isKing: boolean` (or `cardRank: "Q" | "K"`). Switch title from a single i18n key to two: `game.belot.titleBelot` ("Announce Belote?") vs `game.belot.titleRebelot` ("Announce Rebelote?"). Do the same for the description/button label where it reads "Announce".
- Caller (GamePage) determines the rank from `state.currentTrick`'s last played card (the K/Q that triggered the prompt).

**Client — [client/src/features/game/GamePage.tsx](client/src/features/game/GamePage.tsx):**

- When `showBelotPrompt` is true, compute `isKing` from the last played card in the current trick. Pass to `BelotPrompt`.

**Client — NEW [client/src/features/game/components/BelotReveal.tsx](client/src/features/game/components/BelotReveal.tsx):**

- Pattern after `DeclarationReveal`: absolute-positioned panel anchored near the announcing seat via `compassOffset`.
- Props: `{ playerSeat: number, myPlayerSeat: number, cardId: string, isKing: boolean, onComplete: () => void }`.
- Shows a large label "Belote!" or "Rebelote!" plus one `<PlayingCard size="sm" state="default" />` rendered from the `cardId`.
- Auto-dismiss after 4s (1.5s reduced-motion). `pointer-events-none`.
- `data-testid="belot-reveal"`.

**Client — [client/src/features/game/components/BelotReveal.test.tsx](client/src/features/game/components/BelotReveal.test.tsx):** NEW FILE.

- Mirror `DeclarationReveal.test.tsx`: (a) renders label and card; (b) auto-dismisses at 4s; (c) prefers-reduced-motion shortens to 1.5s; (d) `onComplete` fires; (e) correct label for King vs Queen.

**Client — [client/src/shared/stores/gameStore.ts](client/src/shared/stores/gameStore.ts):**

- Add `belotReveal: BelotAnnouncedPayload | null` state and `setBelotReveal(payload)` setter. Pattern mirrors `declarationReveal`.

**Client — [client/src/shared/hooks/useWsDispatch.ts](client/src/shared/hooks/useWsDispatch.ts):**

- Replace the no-op `EVENT_BELOT_ANNOUNCED` branch with `store.setBelotReveal(message.payload as BelotAnnouncedPayload)`.

**Client — [client/src/features/game/GamePage.tsx](client/src/features/game/GamePage.tsx):**

- Render `<BelotReveal>` when `belotReveal` is set (similar to `declarationReveal`).

**Client — [client/src/shared/types/wsEvents.ts](client/src/shared/types/wsEvents.ts):**

- Extend `BelotAnnouncedPayload` with `cardId: string` to match the server.

**i18n:**

- `game.belot.titleBelot` / `game.belot.titleRebelot`.
- `game.belot.announceBelot` / `game.belot.announceRebelot`.
- `game.belot.reveal.belot` → "Belote!".
- `game.belot.reveal.rebelot` → "Rebelote!".
- Update EN and SR locale files.

## Files

**New:**

- `client/src/features/game/components/BelotReveal.tsx`.
- `client/src/features/game/components/BelotReveal.test.tsx`.

**Modified:**

- `server/internal/ws/events.go` — extend `BelotAnnouncedPayload`.
- `server/internal/session/manager.go` — include `cardId` in belot broadcast.
- `client/src/shared/types/wsEvents.ts` — extend type.
- `client/src/features/game/components/BelotPrompt.tsx` — rank-aware label.
- `client/src/features/game/components/BelotPrompt.test.tsx` — assert correct label per rank.
- `client/src/features/game/GamePage.tsx` — rank detection, mount `BelotReveal`.
- `client/src/shared/stores/gameStore.ts` — `belotReveal` state + setter.
- `client/src/shared/hooks/useWsDispatch.ts` — populate `belotReveal`.
- `client/src/shared/i18n/en.json`, `sr.json` — new keys.

## Acceptance

- **Given** the player holds K♣+Q♣ of trump and their turn comes to play the **Queen** first
  **When** they click Q♣
  **Then** the Belote prompt reads "Announce Belote?" with "Announce Belote" and "Skip" buttons.
  **And** on Announce, all four players see a small panel near the announcing seat showing "Belote!" and the Q♣ card. The panel auto-dismisses after 4s.
  **And** +20 is added to the announcing team's pending hand total.
  **And** when the announcer later plays K♣ in a subsequent trick, no prompt fires and no reveal appears.

- **Given** the player holds K♣+Q♣ and plays the **King** first
  **When** they click K♣
  **Then** the prompt reads "Announce Rebelote?" and the reveal shows "Rebelote!" with the K♣ card.

- **Given** the player skips the prompt on their first K/Q play
  **When** they later play the other K/Q
  **Then** no prompt appears and no Belote points are awarded.

- **Given** the announcer's the taker's team later fails the hand
  **When** hand scoring runs
  **Then** the +20 is part of the "all points to opponent" transfer (already correct server-side; reveal has no effect on scoring path).

## Test Plan

- `go test ./server/internal/game/... ./server/internal/session/... ./server/internal/ws/...` — new payload field covered.
- `npx vitest run` — all existing + new tests green.
- `npx tsc --noEmit` — clean.
- Manual/Playwright: play through a hand where the dealer forces K+Q of trump to the human player. Announce Belote on Q-first play. Verify the prompt label, the reveal card, the auto-dismiss duration, the pending score bump, and the absence of any prompt/reveal on the second card. Repeat with K-first.

</frozen-after-approval>

## Suggested Review Order

**Wire contract (server → client)**

- Typed payload struct replaces the loose map, carrying the triggering card id.
  [`events.go:49`](../../server/internal/ws/events.go#L49)

- Broadcast reads the K/Q from the last trick entry before the announce action.
  [`manager.go:477`](../../server/internal/session/manager.go#L477)

- Client mirror of the payload gains `cardId`.
  [`wsEvents.ts:131`](../../client/src/shared/types/wsEvents.ts#L131)

- Dispatch now populates the store (with a `cardId` shape guard mirroring `card_played`).
  [`useWsDispatch.ts:191`](../../client/src/shared/hooks/useWsDispatch.ts#L191)

**Rank-aware prompt**

- Prompt title/button copy branches on `isKing` instead of fixed keys.
  [`BelotPrompt.tsx:12`](../../client/src/features/game/components/BelotPrompt.tsx#L12)

- GamePage derives `isKing` from the last card in `currentTrick` (the triggering K/Q).
  [`GamePage.tsx:338`](../../client/src/features/game/GamePage.tsx#L338)

- Prompt mounted with the new prop.
  [`GamePage.tsx:494`](../../client/src/features/game/GamePage.tsx#L494)

**Seat-anchored reveal**

- New `BelotReveal` component mirrors `DeclarationReveal`'s pattern (compass panel + single card + auto-dismiss).
  [`BelotReveal.tsx:30`](../../client/src/features/game/components/BelotReveal.tsx#L30)

- Mounted with `key` derived from payload so back-to-back announcements remount cleanly.
  [`GamePage.tsx:512`](../../client/src/features/game/GamePage.tsx#L512)

- Reveal state mirrors the existing `declarationReveal` store slot.
  [`gameStore.ts:78`](../../client/src/shared/stores/gameStore.ts#L78)

**i18n**

- New `titleBelot`/`titleRebelot`/`announceBelot`/`announceRebelot`/`reveal.belot`/`reveal.rebelot` keys.
  [`en.json:186`](../../client/src/shared/i18n/en.json#L186)

- Matching SR translations.
  [`sr.json:186`](../../client/src/shared/i18n/sr.json#L186)

**Tests**

- Prompt asserts both Belote and Rebelote copy variants.
  [`BelotPrompt.test.tsx:45`](../../client/src/features/game/components/BelotPrompt.test.tsx#L45)

- Reveal asserts label/card for both ranks and the 4s / 1.5s auto-dismiss paths.
  [`BelotReveal.test.tsx:22`](../../client/src/features/game/components/BelotReveal.test.tsx#L22)
