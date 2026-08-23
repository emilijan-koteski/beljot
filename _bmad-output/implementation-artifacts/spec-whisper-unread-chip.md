---
title: 'Whisper unread as its own pink chip on the chat FAB'
type: 'feature'
created: '2026-08-23'
status: 'done'
baseline_commit: '4e3d43e57b8f9c961fff9c10afb005cde7e76c06'
review_loop_iteration: 0
context: []
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** On a closed chat dock the FAB shows ONE brass badge counting `unread + whisperUnreadTotal` (`ChatDock.tsx:363`), so a private whisper is arithmetically indistinguishable from public chat noise — the player sees "3" and cannot tell whether anyone whispered them.

**Approach:** Split that badge in two: brass keeps the public channel, a new pink chip carries the whisper total summed across all threads. The pair staggers and overlaps the way the Dealer and Trump chips stack next to a player's avatar in-match. The open dock keeps its per-friend tab badges, re-pointed to the same pink token so closed and open states read as one system.

## Boundaries & Constraints

**Always:**

- Pink chip = sum of `whisperUnread` over all threads (existing `whisperUnreadTotal`). Brass = primary channel only.
- Numeric only on the FAB — whisper text is never previewed there or in the peek bubble.
- With zero whisper unread the FAB looks exactly as today: one brass badge in its current position.
- Same behaviour in all three variants; the pink stays legible on the parchment skin AND the `.chat-dock-match` dark felt.
- New UI strings go into all four locales (`en`, `mk`, `hr`, `sr`) — `i18n.parity.test.ts` fails CI otherwise. `mk` all-Cyrillic and idiomatic.

**Ask First:**

- Touching `chatStore` whisper-unread semantics (`appendWhisper` / `setDockOpen` / `markThreadRead`), or which events count as unread. This change is presentation-only.
- Adding a sound or toast for whispers.

**Never:**

- No whisper chip on a player seat / avatar in-match — the seat chips are the styling *reference*, not the location.
- No structural change to the open dock: per-friend counts stay on the whisper tab strip.
- No server, WS-contract, or persistence changes.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Public only | 2 public msgs from others, dock closed | Brass chip `2` in today's position; no pink chip | N/A |
| Whisper only | 1 whisper from bob, dock closed | Pink chip `1` in the lone-chip position; no brass chip | N/A |
| Both, multi-friend | 2 public + bob(1) + carol(2), dock closed | Brass `2` and pink `3`, staggered/overlapping, pink in front | N/A |
| Overflow | 120 pending whispers | Pink chip renders `99+` | N/A |
| Own echo | I sent a whisper, dock closed | No pink chip | N/A |
| Dock opened | Whispers pending, dock opened | No FAB chips on screen; each friend's count on their whisper tab; active thread clears | N/A |

</frozen-after-approval>

## Code Map

- `client/src/features/chat/ChatDock.tsx` — sole consumer. `:129-132` `whisperUnreadTotal` memo (already the right sum); **`:363` `badgeCount = unread + whisperUnreadTotal` — the line to split**; `:392` FAB button, whose `aria-label` is the only label today; `:397-401` ring highlight gated on `badgeCount > 0`; `:403-410` brass badge (`data-testid={`${testIdRoot}-unread`}`, cap `99+`); `:468` per-thread `unread` prop; `:648-675` `ChannelTab` badge (`bg-(--whisper-name) text-white`, cap `9+`, hidden while active).
- `client/src/index.css:568-583` — `--whisper-*` tokens in `:root` and `.chat-dock-match`. The match skin's `--whisper-name: #f2b8d4` makes the tab badge's `text-white` low-contrast in-game; a shared badge token fixes that too.
- `client/src/features/match/components/PlayerSeat.tsx:66-104` (`StatusChip`) + `:333-338` (anchoring) — **read-only** stagger reference: equal `top`, `right` 10px apart, front chip wins z-order. Do not import or refactor; FAB chips are 20px in a different skin.
- `client/src/shared/stores/chatStore.ts:95,130-132,142-156` — whisper unread state/mutations. **Read-only, no change needed.**
- `client/src/features/chat/ChatDock.test.tsx` — `whisper()` factory `:24-35`, `resetChat()` `:34-50`, `renderOpenLobbyDock()` `:66-69`. Seed via real actions in `act()` (`appendWhisper(whisper({...}), 1)`) and render *without* clicking the FAB to keep `dockOpen` false. `:307` and `:323` assert whisper counts on `lobby-chat-unread` — both must move to the pink testid.
- `client/src/features/match/components/MatchChatDock.test.tsx:40-51` — `beforeEach` resets message slices but not whisper state; its five `match-chat-unread` tests (`:91-153`) remain valid as primary-only.
- `client/src/shared/i18n/{en,mk,hr,sr}.json` — `whisper.*` (en from `:355`) has 7 keys, identical in all four. No badge-label key exists. sr-only precedent: `HonorHeroBand.tsx:222`.

## Tasks & Acceptance

**Execution:**

- [x] `client/src/index.css` -- add `--whisper-badge` (fill) + `--whisper-badge-ink` (text) to both whisper blocks -- one pink pair legible on parchment and felt, shared by FAB chip and tab badge.
- [x] `client/src/features/chat/ChatDock.tsx` -- replace `badgeCount` with two independent chips (brass `unread`, pink `whisperUnreadTotal` at `data-testid={`${testIdRoot}-whisper-unread`}`), stagger when both present and keep today's anchor when only one is; gate the ring on either count; localize both counts into the FAB button's own `aria-label` (see Design Notes — an sr-only span inside the button cannot be announced); re-point `ChannelTab`'s badge to the shared token.
- [x] `client/src/shared/i18n/en.json`, `mk.json`, `hr.json`, `sr.json` -- add `whisper.unreadCount` and `chat.unreadCount`, each with `{{count}}`, for the FAB's accessible name.
- [x] `client/src/features/chat/ChatDock.test.tsx` -- retarget the two whisper-on-FAB tests to the pink testid; add public-only (brass, no pink), whisper-only (pink, no brass), both-present with separate numbers summed across two friends, and the `99+` cap.
- [x] `client/src/features/match/components/MatchChatDock.test.tsx` -- extend the `beforeEach` reset with whisper state so the existing assertions stay primary-only; add one test that the pink chip renders on the match FAB.

**Acceptance Criteria:**

- Given whispers from two friends plus unread public messages, when the dock is closed, then brass shows only the public count and pink shows the summed whisper count as a separate chip.
- Given only public unread, when the dock is closed, then exactly one brass chip renders in its pre-change position.
- Given pending whispers, when the dock is opened, then no FAB chip is on screen and each friend's count sits on their own whisper tab.
- Given the match dock on dark felt, then the pink chip and tab badge fill/text stay legible.
- Given counts pending on either channel, when the FAB is reached by a screen reader, then its accessible name carries the open label plus a localized clause for each non-zero count.
- Given both chips render, then neither the pair's width nor its overflow form overhangs the 56px FAB.

## Spec Change Log

- **Chip stagger reimplemented as a flex row (step-04 review).** The Design Notes originally sketched two absolutely positioned chips at `right: -6` / `right: 4`. Both offsets are fixed while a chip's width grows with its digit count, so the front chip covered the rear chip's centre — the brass digit was ~60% hidden at a single digit and fully hidden at two. Amended to a shrink-wrapped `absolute -top-0.5 -right-0.5 flex items-center` container with a `-ml-1.5` overlap, which is width-independent. **KEEP on any re-derivation:** the lone-chip case must stay on the untouched `-top-0.5 -right-0.5` anchor so a whisper-free FAB is pixel-identical to its pre-change self, and the pink chip must stay in front.
- **Count labels moved from sr-only spans to the button's `aria-label` (step-04 review).** The Execution task asked for an "sr-only localized label per chip"; implemented that way, the label was inert — an `aria-label` on the FAB `<button>` replaces the accessible name of its whole subtree, so no nested text is ever announced. Known-bad state avoided: shipping a localized string, and a key in four locale files, that no screen reader can reach. Amended to compose both counts into the button's own label, which also closed the asymmetry where only the whisper count would have been announced.
- **Paired-chip overflow form capped at two characters (step-04 review).** Two `99+` chips are ~62px against a 56px button. Over 99, a chip shows `9+` when paired and keeps `99+` when alone; precision below 100 is unchanged. Known-bad state avoided: the chip row overhanging the FAB and reaching the message glyph.
- **Badge fill darkened to `#b0416f` (step-04 review).** The inherited `#c2568a` + white measured 4.21:1, under WCAG AA, and this change extended that pair from the tab badge onto a new FAB chip while citing legibility as the reason for splitting the token. The felt pair (`#3d0f26` on `#f2b8d4`, 9.72:1) was already sound. **KEEP:** quantify contrast in the token comment rather than asserting legibility.

## Design Notes

Stagger mirrors `PlayerSeat`'s dealer/caller pair — overlapping pair, pink in front as the rarer, higher-signal event. **Implemented as a flex row, not the two absolute `right` offsets first sketched here:** a chip grows with its digit count, so any fixed offset that looks right at `2` buries the brass digit at `12` (`PlayerSeat` gets away with it because its chips are fixed-width glyph circles). A negative margin keeps the overlap at 6px whatever the widths — enough to read as a stack, never enough to clip a number.

```tsx
// shrink-wrapped container keeps today's anchor, so a whisper-free FAB is unchanged
<span className="absolute -top-0.5 -right-0.5 flex items-center">
  {hasBrass && <span className={cn(chipBase, brassSkin)}>…</span>}
  {hasWhisper && <span className={cn(chipBase, pinkSkin, hasBrass && "z-10 -ml-1.5")}>…</span>}
</span>
```

**Accessible name:** the count rides on the FAB `<button>`'s own `aria-label` (`"Open lobby chat, Unread whispers: 2"`), not an sr-only span inside it — an `aria-label` replaces the accessible name of its whole subtree, so nested text is never announced. The brass count stays unannounced, as it was before this change.

Tokens: `:root` → `--whisper-badge: #c2568a; --whisper-badge-ink: #fff` (today's parchment look). `.chat-dock-match` → `--whisper-badge: #f2b8d4; --whisper-badge-ink: #3d0f26` — keeps the bright felt pink, takes dark ink, and repairs the existing white-on-light-pink tab badge in-game.

## Verification

**Commands:**

- `cd client && npx vitest run src/features/chat src/shared/stores/chatStore.test.ts src/features/match/components/MatchChatDock.test.tsx src/shared/i18n src/shared/hooks/useWsDispatch.test.ts` -- expected: all pass (64 were green pre-change, plus new ones)
- `make lint` -- expected: clean

**Manual checks:**

- Lobby, dock closed, whisper pending: pink chip present, brass only for public unread, neither clipping the FAB glyph or the screen edge.
- Same in-match: pink chip legible on felt; tab badge legible once opened.

## Suggested Review Order

**The split itself**

- Entry point: the two channel flags that replaced the single summed `badgeCount`.
  [`ChatDock.tsx:371`](../../client/src/features/chat/ChatDock.tsx#L371)

- One shrink-wrapped anchor holds both chips, so the pair's right edge never moves.
  [`ChatDock.tsx:444`](../../client/src/features/chat/ChatDock.tsx#L444)

- Overlap and overflow narrowing share one `bothChips` flag, so they cannot disagree.
  [`ChatDock.tsx:390`](../../client/src/features/chat/ChatDock.tsx#L390)

- Hoisted chip class — both chips draw identical geometry from one string.
  [`ChatDock.tsx:52`](../../client/src/features/chat/ChatDock.tsx#L52)

**Accessibility**

- Counts ride the button's own label; nested text inside an `aria-label` is never announced.
  [`ChatDock.tsx:397`](../../client/src/features/chat/ChatDock.tsx#L397)

**Pink as one system**

- Fill, ink and halo as a token trio, with contrast ratios measured rather than asserted.
  [`index.css:587`](../../client/src/index.css#L587)

- Pink chip consumes all three tokens, so the felt skin re-points every one.
  [`ChatDock.tsx:461`](../../client/src/features/chat/ChatDock.tsx#L461)

- The open dock's per-friend badge now draws from the same pair — repairs a 1.67:1 in-game contrast.
  [`ChatDock.tsx:729`](../../client/src/features/chat/ChatDock.tsx#L729)

- Two count strings, four locales; `Label: {{count}}` form dodges the parity test's plural ban.
  [`en.json:354`](../../client/src/shared/i18n/en.json#L354)

**Tests worth reading**

- Pins the stagger: shared anchor, DOM order, overlap classes only when paired.
  [`ChatDock.test.tsx:486`](../../client/src/features/chat/ChatDock.test.tsx#L486)

- Pins the privacy half of the contract — a whisper raises no peek bubble.
  [`ChatDock.test.tsx:546`](../../client/src/features/chat/ChatDock.test.tsx#L546)

- Paired chips stay two characters wide; sub-100 precision is untouched.
  [`ChatDock.test.tsx:450`](../../client/src/features/chat/ChatDock.test.tsx#L450)

- Both label clauses and their order.
  [`ChatDock.test.tsx:381`](../../client/src/features/chat/ChatDock.test.tsx#L381)

- Guards the token re-point that would otherwise revert silently.
  [`ChatDock.test.tsx:526`](../../client/src/features/chat/ChatDock.test.tsx#L526)

- Same split on the felt skin, with the public message authored by someone else.
  [`MatchChatDock.test.tsx:161`](../../client/src/features/match/components/MatchChatDock.test.tsx#L161)
