---
title: 'Clickable player names on profile surfaces'
type: 'feature'
created: '2026-08-22'
status: 'done'
review_loop_iteration: 0
context: []
baseline_commit: 'fb519d0a40c0063b36b1397fd8f2883f4461cf2d'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** In the profile match-history list (and the other profile panels where other players appear — Partner Spotlight, Rivalries), player names are inert text. A viewer who spots someone they played with cannot open that player's profile or send a friend request.

**Approach:** Make every real (human, non-subject) player name on profile surfaces a link to `/players/:userId` (the existing `PublicPlayerProfilePage`), keeping the current visuals plus a hover affordance.

## Boundaries & Constraints

**Always:**
- Clickable only when the player is a real user: `isBot !== true && userId > 0` (explicit comparisons — never truthiness on Go zero values).
- Bot seats, missing seats ("—"), and the profile subject's own chip stay non-interactive.
- Clicking a name must NOT toggle the match row's expand/collapse.
- Valid HTML: no `<a>`/`<button>` nested inside another interactive element. The row toggle keeps `aria-expanded`/`aria-controls` on a real `<button>`.
- Name links get a localized aria-label ("View {{username}}'s profile") present in all 4 locales (en, mk, hr, sr); mk all-Cyrillic.
- Client-side navigation (React Router `Link`/`navigate`), never full page loads.

**Ask First:** Redirecting a self-click (viewer clicks their own name on someone else's history) to `/profile` instead of `/players/:selfId` — current public page already self-guards the FriendButton, so plain `/players/:id` is assumed OK.

**Never:** No server/API changes (all needed `userId`s are already in the payloads). No changes to lobby/room `SeatChip` variants. No new profile-preview popover — navigation only.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Human teammate/opponent chip | `{userId: 7, username: "ana", isBot: false}` | Chip is a link → `/players/7`; click does NOT toggle the row | N/A |
| Bot seat | `{userId: 0, username: "", isBot: true}` | Localized bot name, non-interactive (unchanged) | N/A |
| Missing seat | no player at seat | "—" chip, non-interactive | N/A |
| Subject's own chip | first chip (`username` prop) | Non-interactive (self = viewer; public = page already shown) | N/A |
| Row background click | outside links / toggle button | Expand/collapse detail (existing behavior) | N/A |
| Partner/rival row | `{userId: 9, username: "vlade"}` | Avatar+username is a link → `/players/9` | N/A |
| Keyboard user | Tab through a match row | Name links AND toggle button reachable; toggle keeps `aria-expanded` | N/A |

</frozen-after-approval>

## Code Map

- `client/src/features/profile/MatchHistory.tsx` — `MatchRow` (l.400) wraps the whole row header in `<button onClick={onToggle}>` (l.435-549); SeatChips render inside it (l.469-487) via `seatChipProps()` (l.409) which already reads `match.players` but currently drops `userId`. Restructure: header becomes a clickable `<div>`; the chevron (l.544) becomes the real toggle `<button>` carrying the existing `aria-expanded`/`aria-controls`/`aria-label`.
- `client/src/features/profile/components/SeatChip.tsx` — team-tinted pill, used ONLY by MatchHistory (lobby has its own SeatChip). Add optional `userId?: number`; when set, render as `Link` to `/players/:userId` with `onClick` `stopPropagation`, hover affordance, and `aria-label` `t("friends.viewProfileAria", {username})`.
- `client/src/shared/api/matches.ts` — `MatchPlayer` (l.11-17): `userId`, `username`, `isBot` all already on the wire. Bot seats: `{userId: 0, username: "", isBot: true}`. Read-only.
- `client/src/features/profile/components/PartnerSpotlight.tsx` — featured partner (l.54-68) + rest list (l.74-92); `PartnerStat` has `userId`. Wrap avatar+username in a `Link`.
- `client/src/features/profile/components/Rivalries.tsx` — rival rows (l.36-57); `RivalStat` has `userId`. Same `Link` treatment. Server guarantees no bots in either panel (`teammate_id IS NOT NULL`).
- `client/src/shared/i18n/{en,mk,hr,sr}.json` — `friends.viewProfileAria` (l.286) already exists in all 4 locales with exactly the needed copy; REUSE it, no new keys needed.
- `client/src/features/profile/MatchHistory.test.tsx` — toggle tests use `within(row).getByRole("button")` (l.325, 369, 396): still valid if the row contains exactly ONE button (links have role `link`). Add coverage for link vs non-link chips.
- `client/src/App.tsx` (l.94) — `/players/:id` route exists; `PublicPlayerProfilePage` self-guards the FriendButton. Read-only.

## Tasks & Acceptance

**Execution:**
- [x] `client/src/features/profile/components/SeatChip.tsx` — add optional `userId`; render `Link` variant (stopPropagation, hover underline/affordance, `friends.viewProfileAria` aria-label) when set, unchanged `span` otherwise.
- [x] `client/src/features/profile/MatchHistory.tsx` — pass `userId` through `seatChipProps` for human seats only (`isBot !== true && userId > 0`); restructure the row header: outer `<div>` with `onClick={onToggle}` + `cursor-pointer`, chevron wrapped in the toggle `<button>` (keeps `aria-expanded`, `aria-controls`, `aria-label`, chevron rotation). Subject chip gets no `userId`.
- [x] `client/src/features/profile/components/PartnerSpotlight.tsx` — link featured + rest usernames to `/players/:userId` with the same aria-label.
- [x] `client/src/features/profile/components/Rivalries.tsx` — link rival usernames to `/players/:userId` with the same aria-label.
- [x] `client/src/features/profile/MatchHistory.test.tsx` — cover the I/O matrix: human chip navigates (link href), bot/missing/subject chips render no link, link click does not toggle detail, toggle button still exposes `aria-expanded`.

**Acceptance Criteria:**
- Given a match against humans, when the viewer clicks an opponent's name in match history, then the app navigates client-side to that player's public profile (where the FriendButton lives).
- Given the restructured row, when a keyboard user tabs through it, then name links and the toggle button are each reachable and the toggle announces expanded state.
- Given the Partner Spotlight / Rivalries panels (self and public profile), when a listed player is clicked, then their public profile opens.

## Verification

**Commands:**
- `cd client && npx vitest run src/features/profile` — expected: all profile suites pass, incl. new chip-link tests.
- `make lint` — expected: clean.
- `make test` — expected: both stacks green.

## Suggested Review Order

**Link eligibility — who gets a profile link**

- Entry point: only live humans (`isBot !== true && userId > 0 && username !== ""`) get a `userId`; soft-deleted users arrive as `{userId>0, username:""}`.
  [`MatchHistory.tsx:414`](../../client/src/features/profile/MatchHistory.tsx#L414)

- The chip itself: `userId` present → `Link` with stopPropagation + viewProfileAria; absent → the original inert span.
  [`SeatChip.tsx:54`](../../client/src/features/profile/components/SeatChip.tsx#L54)

**Row restructure — links inside a clickable row without nested interactives**

- Header becomes a convenience-click `<div>`; interactive semantics move out of it.
  [`MatchHistory.tsx:452`](../../client/src/features/profile/MatchHistory.tsx#L452)

- The chevron is now the real toggle `<button>`, keeping `aria-expanded`/`aria-controls` and stopping its own bubble.
  [`MatchHistory.tsx:577`](../../client/src/features/profile/MatchHistory.tsx#L577)

**Sidebar panels — same guard, same aria pattern**

- Featured partner links only on non-empty username; shared `FeaturedPartner` avoids duplicating the block.
  [`PartnerSpotlight.tsx:83`](../../client/src/features/profile/components/PartnerSpotlight.tsx#L83)

- Rest-list partner rows: link vs plain span on the same username guard.
  [`PartnerSpotlight.tsx:108`](../../client/src/features/profile/components/PartnerSpotlight.tsx#L108)

- Rival rows mirror the partner-row pattern with the "Them" palette.
  [`Rivalries.tsx:50`](../../client/src/features/profile/components/Rivalries.tsx#L50)

**Cross-profile navigation hygiene**

- `key={validId}` remounts the public page on subject change — cached destinations skip the skeleton, so state would otherwise carry over.
  [`PublicPlayerProfilePage.tsx:128`](../../client/src/features/profile/PublicPlayerProfilePage.tsx#L128)

**Tests**

- Chip link matrix: human links + hrefs, subject/bot/missing inert, no-toggle-on-link-click, single toggle button, mk aria-label.
  [`MatchHistory.test.tsx:345`](../../client/src/features/profile/MatchHistory.test.tsx#L345)

- Deleted-user seat renders an inert "—" chip, never a dead link.
  [`MatchHistory.test.tsx:424`](../../client/src/features/profile/MatchHistory.test.tsx#L424)

- Partner (featured AND rest row) + rival links with hrefs — two-partner fixture exercises the otherwise-unrendered rest path.
  [`ProfilePage.test.tsx:344`](../../client/src/features/profile/ProfilePage.test.tsx#L344)
