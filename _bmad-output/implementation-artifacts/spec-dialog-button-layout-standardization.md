---
title: "Dialog button-layout standardization (action-left / status-quo-right)"
type: "refactor"
created: "2026-08-23"
status: "done"
review_loop_iteration: 0
baseline_commit: "a9897fcaa58fb2de7b494b30e4a387b5442ddb89"
context: []
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Dialog footers across the app disagree on button placement. Most confirm dialogs and every in-game prompt follow the shadcn default (status-quo button on the left, action on the right); a few use `justify-between` so the pair straddles the footer. There is no single rule, so muscle memory never forms.

**Approach:** Adopt the Windows-confirm convention app-wide — one footer row anchored bottom-right, with the **action-taking** button on the **left** and the **status-quo** button on its **right**. The X close control stays top-right where present. Dialogs whose buttons are both actions (no status-quo option) keep their own centered, stacked, full-width row with visually distinguished button tones. Single-button dialogs stay bottom-right. Purely a layout/ordering change: no handler, label, i18n, timer, or visual-tone change.

## Boundaries & Constraints

**Always:**
- Action button precedes status-quo button in DOM order; the footer row stays right-aligned (`justify-end`). This ordering also drives keyboard/tab order — that is intended.
- The auto-action timer ring (`ButtonTimerRing`) stays wrapped around the **status-quo** button (Pass / Skip / Decline) and keeps its exact props.
- Footers that stack on narrow screens stack in DOM order (action on top) — replace `flex-col-reverse` with `flex-col` so mobile order matches desktop left-to-right.
- Preserve every `data-testid`, `variant`, inline style, disabled condition, conditional-render branch, and surrounding comment. Move nodes; do not rewrite them.

**Ask First:**
- Any change that alters a button's label, tone/variant, handler, or a dialog's X-close presence.
- Adding an X close button to a dialog that currently has none.

**Never:**
- Do not change `window.confirm` at `client/src/features/match/MatchPage.tsx:968` — native browser dialog, unstyleable.
- Do not right-align the full-width celebration CTAs in `DailyRewardDialog` / `LevelUpDialog` (human decision: keep full-width).
- Do not restructure `MatchResult` — it is already the reference shape for the both-actions case.
- No new i18n keys, no server changes.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|---|---|---|---|
| Action + status quo | Confirm/Cancel, Declare/Skip, Announce/Decline, Pick/Pass, Accept/Decline | One right-aligned row: action left, status quo right | N/A |
| X present | `OwnerConfirmDialogs` | X stays top-right; footer pair swapped | N/A |
| Timer ring | Prompt with `showRing` true | Ring still wraps the status-quo button, now the right-hand one | N/A |
| Status-quo hidden | `TrumpPrompt` with `canPass === false` | `MustPickNote` (+ ring) renders where Pass would sit — right of the suit grid row; Pick absent in round 2 | N/A |
| Both actions | `PauseOverlay` owner + player controls | Centered stacked full-width column, distinct tones | N/A |
| One control only | `PauseOverlay` non-owner, or waiting paragraph | Same centered column, single child | N/A |
| Narrow screen | Footer below `sm` | Stacks action-on-top | N/A |

</frozen-after-approval>

## Code Map

Shared primitive — `client/src/shared/components/ui/dialog.tsx:78` `DialogFooter`: `-mx-4 -mb-4 flex flex-col-reverse gap-2 ... sm:flex-row sm:justify-end`. Change `flex-col-reverse` → `flex-col` only. Project rule permits editing `shared/components/ui/` for project-wide convention changes; this is one.

Rule 2 (no X) — six `DialogFooter` consumers, all ordered cancel-then-action; swap the two `<Button>` blocks:
- `client/src/features/lobby/components/PasswordPromptDialog.tsx:129`
- `client/src/features/room/RoomPrivacyDialog.tsx:169`
- `client/src/features/lobby/components/RoomInviteModal.tsx:237`
- `client/src/features/auth/components/LinkAccountDialog.tsx:135`
- `client/src/features/profile/components/UnlinkAccountDialog.tsx:71`
- `client/src/features/profile/components/RemoveFriendDialog.tsx:60`

Rule 2, custom footer:
- `client/src/features/room/CreateRoomModal.tsx:634` — `<footer>` uses `justify-between` (Cancel far-left, Create far-right). Set `justify-end`, swap to Create-then-Cancel. No X (`showCloseButton={false}` at :312).

Rule 1 (X top-right):
- `client/src/features/room/OwnerConfirmDialogs.tsx:245` — footer already `justifyContent: "flex-end"`; swap the two `<DialogButton>` children. X is the absolute-positioned `DialogClose` at :162 — leave it.

Rule 1/2, in-game prompts (all `flex justify-end`, status-quo first, ring on status-quo):
- `client/src/features/match/components/TrumpPrompt.tsx:356` — inner `flex items-center gap-3.5` inside a `justify-between` row (left slot is the round-counter span, keep it). Move the Pick block (`:394`) above the Pass/`MustPickNote` branch (`:365`).
- `client/src/features/match/components/DeclarationPrompt.tsx:228` — move Declare (`:249`) above the `showRing ? <ButtonTimerRing>{skipButton}</ButtonTimerRing> : skipButton` ternary.
- `client/src/features/match/components/BelotPrompt.tsx:111` — move Announce (`:131`) above the decline/ring ternary.
- `client/src/features/match/components/SurrenderPrompt.tsx:78` — swap Decline (`:79`) and Accept (`:82`).
- `client/src/features/match/components/SurrenderButton.tsx:151` — swap Cancel and Confirm inside `flex justify-end`.

Rule 4 (both actions):
- `client/src/features/match/components/PauseOverlay.tsx:133` — replace the `flex items-center justify-between gap-3` row (owner override left, player action right) with a centered stacked full-width column. Order: player-level control first (Resume / Pause / waiting `<p>`), owner override second. Drop the `<span aria-hidden />` spacer — it only existed to hold `justify-between` apart. Tones already distinct (red-tinted owner vs brass primary); keep both, add `w-full` for the stacked shape.
- `client/src/features/match/components/MatchResult.tsx:242` — reference shape, no change.

Adjacent, minimal:
- `client/src/features/lobby/components/RoomEjectionModal.tsx:249` — order already action-left (`Browse qualifying`) / dismiss-right; only `flex-col-reverse` → `flex-col`.

Already compliant, do not touch: `SettingsDialog.tsx:259`, `RulesDialog.tsx:331`, `ScoreReveal.tsx:373`, `HonorExplainerDialog.tsx:206`, `ContactDialog.tsx`, `InviteFriendsDialog.tsx`, `DailyRewardDialog.tsx:257`, `LevelUpDialog.tsx:160`.

Tests exist for: RoomPrivacyDialog, RoomInviteModal, PasswordPromptDialog, CreateRoomModal, OwnerConfirmDialogs, TrumpPrompt, DeclarationPrompt, BelotPrompt, SurrenderPrompt, SurrenderButton, PauseOverlay, RoomEjectionModal. No test files for LinkAccountDialog, UnlinkAccountDialog, RemoveFriendDialog. `DeclarationPrompt.test.tsx:255` snapshots `querySelectorAll("button")` testids but only compares two renders to each other — order-agnostic, no update needed.

## Tasks & Acceptance

**Execution:**
- [x] `client/src/shared/components/ui/dialog.tsx` -- `flex-col-reverse` → `flex-col` in `DialogFooter` -- mobile stack must follow the new DOM order.
- [x] `client/src/features/lobby/components/PasswordPromptDialog.tsx`, `client/src/features/room/RoomPrivacyDialog.tsx`, `client/src/features/lobby/components/RoomInviteModal.tsx`, `client/src/features/auth/components/LinkAccountDialog.tsx`, `client/src/features/profile/components/UnlinkAccountDialog.tsx`, `client/src/features/profile/components/RemoveFriendDialog.tsx` -- swap the two footer buttons so the action precedes the cancel -- rule 2.
- [x] `client/src/features/room/CreateRoomModal.tsx` -- footer `justify-between` → `justify-end`; Create before Cancel -- rule 2 with a custom footer.
- [x] `client/src/features/room/OwnerConfirmDialogs.tsx` -- swap confirm/cancel `DialogButton`s; X untouched -- rule 1.
- [x] `client/src/features/match/components/TrumpPrompt.tsx`, `client/src/features/match/components/DeclarationPrompt.tsx`, `client/src/features/match/components/BelotPrompt.tsx` -- move the primary action above the status-quo/ring branch, leaving the ring on the status-quo button -- rule 1 for game prompts.
- [x] `client/src/features/match/components/SurrenderPrompt.tsx`, `client/src/features/match/components/SurrenderButton.tsx` -- swap accept/confirm before decline/cancel -- rule 1.
- [x] `client/src/features/match/components/PauseOverlay.tsx` -- convert the edge-justified row to a centered stacked full-width column, player control first -- rule 4.
- [x] `client/src/features/lobby/components/RoomEjectionModal.tsx` -- `flex-col-reverse` → `flex-col` -- mobile order consistency.
- [x] Add DOM-order assertions (`compareDocumentPosition` with `Node.DOCUMENT_POSITION_FOLLOWING`, keyed on existing `data-testid`s) to the existing test files for the changed dialogs; for `PauseOverlay` assert the owner override follows the player control. Do not create test files for the three dialogs that have none.

**Acceptance Criteria:**
- Given any dialog with an action and a status-quo button, when it renders at `sm` or wider, then both sit in one right-aligned footer row with the action element preceding the status-quo element in the DOM.
- Given a prompt whose timer ring is visible, when the footer order is swapped, then the ring still wraps the status-quo button and its `turnExpiresAt` / `totalDuration` / `onExpire` props are unchanged.
- Given `TrumpPrompt` with `canPass === false`, when it renders, then `MustPickNote` still occupies the status-quo slot and the round-counter span still sits at the row's left edge.
- Given `PauseOverlay` viewed by a room owner with an active pause, when it renders, then the player Resume control and the owner override are stacked full-width in a centered column, Resume first, with their existing distinct tones.
- Given `make lint` and `make test`, when run, then both pass with no new failures.

## Spec Change Log

## Verification

**Commands:**
- `cd client && npx vitest run` -- expected: all suites pass.
- `make lint` -- expected: clean (ESLint + Prettier).

**Manual checks:**
- Confirm no `flex-col-reverse` remains in a dialog footer: `grep -rn "flex-col-reverse" client/src` returns nothing under a dialog footer.
- Confirm every touched footer resolves to `justify-end` (or the centered rule-4 column) and that no `data-testid` was added, removed, or renamed: `git diff` shows moved blocks only.

## Suggested Review Order

**The convention itself**

- The one-class change that makes DOM order authoritative on phones too, not just desktop.
  [`dialog.tsx:91`](../../client/src/shared/components/ui/dialog.tsx#L91)

- Simplest instance of the new rule: action `type="submit"` first, ghost cancel second.
  [`PasswordPromptDialog.tsx:133`](../../client/src/features/lobby/components/PasswordPromptDialog.tsx#L133)

**Footers that also changed alignment**

- `justify-between` → `justify-end`; the pair no longer straddles the footer.
  [`CreateRoomModal.tsx:634`](../../client/src/features/room/CreateRoomModal.tsx#L634)

- The only rule-1 dialog: footer pair swapped, absolute top-right X untouched.
  [`OwnerConfirmDialogs.tsx:261`](../../client/src/features/room/OwnerConfirmDialogs.tsx#L261)

**Game prompts — the timer ring must not move**

- Hardest case: three status-quo branches (Pass, ring-wrapped Pass, MustPickNote) now trail Pick.
  [`TrumpPrompt.tsx:364`](../../client/src/features/match/components/TrumpPrompt.tsx#L364)

- Declare leads; the ring stays wrapped around Skip, which is the auto-action target.
  [`DeclarationPrompt.tsx:233`](../../client/src/features/match/components/DeclarationPrompt.tsx#L233)

**Rule 4 — both buttons take action**

- Edge-justified row becomes a centred stacked column matching MatchResult's anatomy.
  [`PauseOverlay.tsx:138`](../../client/src/features/match/components/PauseOverlay.tsx#L138)

- Order already compliant; only the mobile stack direction changed.
  [`RoomEjectionModal.tsx:250`](../../client/src/features/lobby/components/RoomEjectionModal.tsx#L250)

**Enforcement**

- Strict position equality, not a bitwise AND — the AND form passed on nested nodes.
  [`test-utils.tsx:154`](../../client/src/test-utils.tsx#L154)
