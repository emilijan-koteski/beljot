---
title: 'Remove-friend and decline actions on the public player profile'
type: 'feature'
created: '2026-08-18'
status: 'done'
review_loop_iteration: 0
context: []
baseline_commit: 'd917b3ad2dd7508bd6c2bfaa53fe61fbd8224fb9'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** The public profile's `FriendButton` covers only half the friendship lifecycle: an incoming request can be Accepted but not Declined (decline lives only in the lobby), and an accepted friendship cannot be ended anywhere — no unfriend endpoint exists (Story 11.2 anticipated one: "hard-delete on decline/unfriend").

**Approach:** Add a party-agnostic `DELETE /api/v1/friends/:id` unfriend endpoint (`:id` = friendship row id, atomic conditional delete like Accept/Decline), and extend `FriendButton` so `pending_incoming` shows Accept + Decline (existing mutation) and `friends` adds "Remove friend" behind a confirm dialog (new mutation).

## Boundaries & Constraints

**Always:**
- Server-authoritative: caller must be a party to the row (`user_id=? OR friend_id=?`) AND `status='accepted'`, enforced in ONE atomic conditional DELETE with rows-affected check; `rows==0` → uniform 404 (don't leak existence vs. authz). Caller id = `getUserID(c)`, never client-supplied.
- Hard-delete (matches decline; no soft-delete column).
- Confirm dialog before unfriending, modeled on `UnlinkAccountDialog` (destructive confirm; not dismissable in flight).
- New strings in all four locales; `mk` all-Cyrillic; no em dash in `mk`/`sr`/`hr`; reuse `friends.decline` for the decline label.
- Precise cache invalidation: remove → `friends.list()` + `friends.status(userId)`; decline keeps its existing invalidations.
- Explicit `requestId !== null` checks — never JS truthiness.

**Ask First:** Any WS contract change (none planned); any surface beyond `FriendButton` (e.g. remove in lobby `FriendList`).

**Never:**
- No WS push to the other party on unfriend — their surfaces refresh on next load, and whisper/invite already re-check `AreFriends` server-side per action (don't touch those paths).
- No migrations, no third `status` value, no changes to lobby `FriendRequests`/`FriendList`.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Unfriend happy path | DELETE /friends/:id, caller either party, row accepted | Row deleted; 200 `{data:{id,status:"removed"}}` | N/A |
| Row is pending | `status='pending'` | 404 (decline is the pending path) | `FRIENDSHIP_NOT_FOUND` |
| Caller not a party | Accepted row, third-party caller | 404, row survives | uniform 404 |
| Race: both unfriend | Concurrent deletes | First wins; second 404 | toast `friends.errors.removeFailed` |
| Bad id | `:id` = 0 / non-numeric | 400 | `apperr.ErrBadRequest` |
| Profile: remove flow | `friends` state → Remove friend → confirm | Mutation with row id; on success button flips to "Add friend" | 404 → toast, no crash |
| Profile: decline flow | `pending_incoming` → Decline | Existing mutation; button flips to "Add friend" | existing `declineFailed` toast |
| Defensive null | `requestId === null` in either state | Action disabled — never fire without a row id | N/A |

</frozen-after-approval>

## Code Map

- `server/internal/friend/{repository.go,gorm_repo.go}` -- Repository interface + GORM impl; atomic `Accept`/`Delete` idiom to mirror (gorm_repo.go:66-79)
- `server/internal/friend/handler.go` -- `Decline` (:253) is the template; `getUserID`/`parseIDParam` helpers exist
- `server/internal/friend/{handler_test.go,gorm_repo_test.go}` -- test idioms (spy notifier, per-test tx)
- `server/internal/apperr/errors.go` -- friend error block (:158)
- `server/cmd/api/main.go` -- friend routes (:350-355)
- `client/src/shared/api/friends.ts` -- 1:1 API client
- `client/src/shared/hooks/mutations/useFriendMutations.ts` -- send/accept/decline mutations to mirror
- `client/src/features/profile/components/FriendButton.tsx` (+ its test) -- the state machine to extend
- `client/src/features/profile/components/UnlinkAccountDialog.tsx` -- confirm-dialog pattern
- `client/src/shared/i18n/{en,sr,mk,hr}.json` -- `friends.*` block (`friends.decline` exists)

## Tasks & Acceptance

**Execution:**
- [x] `server/internal/apperr/errors.go` -- add `ErrFriendshipNotFound` (`FRIENDSHIP_NOT_FOUND`, 404) -- the request-oriented code is semantically wrong for an accepted friendship
- [x] `server/internal/friend/repository.go` + `gorm_repo.go` -- add `Unfriend(id, userID uint) (int64, error)`: `DELETE ... WHERE id=? AND (user_id=? OR friend_id=?) AND status='accepted'` -- party-agnostic, atomic
- [x] `server/internal/friend/handler.go` -- add `Unfriend(c)` mirroring `Decline`; rows==0 → `ErrFriendshipNotFound`; 200 `{data:{id,status:"removed"}}`
- [x] `server/cmd/api/main.go` -- register `api.DELETE("/friends/:id", friendHandler.Unfriend)` in the friend block (distinct method → no collision)
- [x] `server/internal/friend/{gorm_repo_test.go,handler_test.go}` -- cover the I/O matrix backend rows (happy path both directions, pending row, third party, missing, bad id)
- [x] `client/src/shared/api/friends.ts` -- add `removeFriend(id: number): Promise<void>` → `axiosClient.delete`
- [x] `client/src/shared/hooks/mutations/useFriendMutations.ts` -- add `useRemoveFriendMutation` (vars `{requestId, userId?}`), invalidates per Boundaries, onError toast `friends.errors.removeFailed`
- [x] `client/src/features/profile/components/RemoveFriendDialog.tsx` -- new confirm dialog per `UnlinkAccountDialog` pattern (testids: `remove-friend-dialog`, confirm, cancel); interpolates the subject's username
- [x] `client/src/features/profile/components/FriendButton.tsx` -- `friends`: keep disabled "Friends" chip + add "Remove friend" button opening the dialog; `pending_incoming`: add Decline (X icon) beside Accept wired to the existing decline mutation with `{requestId, userId}`
- [x] `client/src/features/profile/components/FriendButton.test.tsx` -- new cases: remove affordance renders + confirm calls `removeFriend` with row id; Decline renders + calls mutation; null `requestId` disables actions
- [x] `client/src/shared/i18n/{en,sr,mk,hr}.json` -- add `friends.removeFriend`, `friends.removeDialog.{title,description,cancel,confirm,submitting}`, `friends.errors.removeFailed` -- parity test stays green

**Acceptance Criteria:**
- Given two accepted friends, when either calls `DELETE /friends/:id` with their row id, then the row is deleted and a repeat call returns 404.
- Given a viewer on a friend's profile, when they click Remove friend and confirm, then the dialog closes and the button flips to "Add friend" without manual refresh.
- Given a profile with `pending_incoming`, when the viewer clicks Decline, then the request is removed and the button flips to "Add friend".
- Given the confirm dialog is cancelled, then no request fires and the friendship is intact.
- Given `make lint` and `make test`, then both pass, including the i18n parity test.

## Design Notes

- `GetStatus` already returns the row id as `requestId` for both `friends` and `pending_incoming` — no new query needed on the profile.
- The party-agnostic guard is the one deliberate divergence from Accept/Delete's recipient-only guard: an accepted friendship belongs to both sides equally.
- Unfriend leaves both parties free to re-request later (`FindByPair` → nothing → "none") — no tombstone.

## Verification

**Commands:**
- `make lint` -- expected: clean on both stacks
- `make test` -- expected: all green (Go friend package + client vitest incl. i18n parity)

**Manual checks (if no CLI):**
- Friend's profile: Remove friend → confirm → "Add friend"; lobby friend list drops them after refetch.
- Profile with incoming request: Decline → "Add friend"; lobby requests section drops the row.

## Suggested Review Order

**Unfriend endpoint (server)**

- Entry point — the design in one screen: either party, uniform 404, no WS push.
  [`handler.go:281`](../../server/internal/friend/handler.go#L281)

- The one deliberate divergence: party-agnostic guard in a single atomic conditional DELETE.
  [`gorm_repo.go:86`](../../server/internal/friend/gorm_repo.go#L86)

- Interface contract documenting what rows==0 means.
  [`repository.go:31`](../../server/internal/friend/repository.go#L31)

- Distinct 404 code — request-oriented sibling was semantically wrong for accepted friendships.
  [`errors.go:174`](../../server/internal/apperr/errors.go#L174)

- Route registration; DELETE method avoids any collision with the sibling routes.
  [`main.go:358`](../../server/cmd/api/main.go#L358)

**Profile actions (UI)**

- `friends` state: chip + Remove friend; dialog closes on settled so the 404 race can't strand it.
  [`FriendButton.tsx:77`](../../client/src/features/profile/components/FriendButton.tsx#L77)

- `pending_incoming`: Decline beside Accept, cross-disabled so the two can't race one row.
  [`FriendButton.tsx:123`](../../client/src/features/profile/components/FriendButton.tsx#L123)

- Confirm dialog; pending locks every dismissal path (modeled on UnlinkAccountDialog).
  [`RemoveFriendDialog.tsx:31`](../../client/src/features/profile/components/RemoveFriendDialog.tsx#L31)

- Username threaded through so the dialog names who is being removed.
  [`PublicPlayerProfilePage.tsx:162`](../../client/src/features/profile/PublicPlayerProfilePage.tsx#L162)

**Client data layer**

- Remove mutation: onSuccess AND onError both resync — a 404 means the friendship is already gone.
  [`useFriendMutations.ts:86`](../../client/src/shared/hooks/mutations/useFriendMutations.ts#L86)

- 1:1 API function on the friend domain client.
  [`friends.ts:36`](../../client/src/shared/api/friends.ts#L36)

**Peripherals**

- Endpoint matrix: either party, repeat 404, pending 404, third party 404, bad id, 401.
  [`handler_test.go:483`](../../server/internal/friend/handler_test.go#L483)

- Integration proof incl. the both-unfriend race and re-request after unfriend (no index residue).
  [`gorm_repo_test.go:171`](../../server/internal/friend/gorm_repo_test.go#L171)

- Invalidation proven end-to-end: successful remove flips the button to "Add friend".
  [`FriendButton.test.tsx:127`](../../client/src/features/profile/components/FriendButton.test.tsx#L127)

- New `removeFriend`/`removeDialog` strings — mirrored in sr/mk/hr, parity test green.
  [`en.json:297`](../../client/src/shared/i18n/en.json#L297)
