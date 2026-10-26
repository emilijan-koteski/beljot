---
title: 'Room invites for lobby players, with a friends-only opt-out'
type: 'feature'
created: '2026-10-26'
status: 'done'
route: 'dispatch'
review_loop_iteration: 0
context: []
baseline_commit: '11f394a88186660b8a41be2bb5692acc68d686a3'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Room invites (Story 11.5) reach friends only, so a host who wants a fourth player can't pull in someone idle in the lobby. Players also have no way to limit invites to people they know.

**Approach:** Let room members invite any non-friend who is online and idle in the lobby, through a new Lobby tab in the existing invite dialog. Add a per-user `allowNonFriendInvites` preference (default `true`) on the profile page. When it is off, the player is hidden from other people's Lobby tab and the server refuses non-friend invites to them.

## Boundaries & Constraints

**Always:**
- The server decides eligibility. "Idle in the lobby" means the same thing as a friend's "available": connected, not in a match, and not in a waiting or playing room.
- Friend invites are unchanged. The opt-out only governs invitees who are not friends with the inviter.
- Non-friend invites reuse the whole Story 11.5 pipeline: the `InviteRegistry`, the 60 s TTL, every auto-void, the `system:room_invite` popup, the host password-bypass grant, and the join-time honor gate, `allow_new_players` and seat-capacity checks. The WS contract does not change.
- The column defaults to `TRUE`, which backfills every existing user and every new account.
- The preference follows the audio-preference pattern: the same `PATCH /users/:id/preferences` endpoint, the auth envelope, and an optimistic store write that reverts on failure.

**Never:**
- Do not add the preference to `PublicProfileResponse`. Do not tell the inviter which players opted out; they are simply absent from the list.
- Do not list bots, the caller, players already in this room, or friends in the Lobby tab. Friends stay in the Friends tab.
- Do not rename the `friendUserId` wire field on `POST /rooms/:id/invite`.
- Do not add a decline cooldown or any new rate limit. The guard is one pending invite per room per player, plus the opt-out (owner decision).

**Decisions (owner, 2026-10-26):**
- **Who may invite non-friends:** the same members, under the same rules, as for friends. Any seated member can invite, in any room. A non-host's invitee still gets the password prompt in a password room.
- **"Invite back" in the Last-match dialog:** extended to non-friends. The client drops its friends-only gate, and the server enforces the opt-out and availability like any other invite.
- **Spec size:** kept as one spec, because lobby invites and the opt-out ship together.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Invite idle non-friend | target allows, is idle, room has a seat | 200, popup pushed, grant issued | N/A |
| Target opted out | non-friend, `allowNonFriendInvites=false` | 409 | `NOT_FRIENDS`; client text "only accepts invites from friends" |
| Friend opted out | friends, target `false` | 200, invite works as today | N/A |
| Non-friend busy | offline, in a match, or in a room | 409 | `FRIEND_NOT_AVAILABLE` |
| Lobby list | 3 idle non-friends, 1 opted out, 1 friend | 3 rows, sorted by username | N/A |
| Lobby list, room full | idle non-friends exist | every row `available:false`, `reason:"room_full"` | N/A |
| Lobby search | `?search=mar` | case-insensitive substring match, capped at 50 rows | N/A |
| Preference PATCH | `{"allowNonFriendInvites":false}` | 200, column written, echoed back | non-boolean → 400 |
| Invite back, non-friend | Last-match row for a non-friend who is not in the room | "Invite back" shown; POST follows the same rules as the Lobby tab | errors shown as a toast via `inviteFailure` |

</frozen-after-approval>

## Code Map

**Server: invites** (`server/internal/room/invite_handler.go`)
- `:172-178` is the friendship gate. A non-friend passes only if the target's preference is `true`; otherwise it keeps returning `apperr.ErrNotFriends`.
- `availabilityReason` (`:289`), `roomIsFull` (`:311`), `requireRoomMember` (`:251`) and `InvitableFriendDTO` (`:102`) are all reused unchanged. The Lobby endpoint returns the same row shape.
- `ConnectionTracker` and `SessionTracker` (`:35`, `:41`) are used only by this handler. Add `ConnectedUserIDs()` and `InMatchUserIDs()` to them. `*ws.Hub` (`ws/hub.go:225`) and `*match.Manager` (`match/live_match.go:568`) already implement both.
- `RoomHandler.idleLobbyCount` (`quick_fill.go:53`) and `lobby.Handler.GetStats` (`lobby/lobby.go:59`) are the idle-set precedent. Use `repo.FindUserIDsByRoomStatus` for both `waiting` and `playing`.
- A new `PlayerDirectory` interface supplies `UserID`, `Username`, `AvatarURL` and `AllowsNonFriendInvites` for a set of IDs. It is implemented in `cmd/api/main.go:424` by the existing `inviteFriendDirectory` adapter via `users.FindManyByIDs`, and passed as a new `NewInviteHandler` argument (`main.go:304`). Routes are registered at `main.go:312-314`.
- Tests: `room/invite_handler_test.go` (`stubPresence` `:52`, `stubFriendDirectory` `:22`, `setupInviteTest` `:102`). `room/avatar_payload_test.go:199` and `cmd/api/invite_directory_test.go` also construct the handler or the adapter.

**Server: preference** (mirror `sound_enabled`)
- `user/model.go:15-19`; `user/repository.go:7` `PreferencesUpdate` and `IsEmpty` `:17`.
- `user/handler.go:86` `UpdatePreferencesRequest` must keep fields identical to `PreferencesUpdate`, because `PreferencesUpdate(req)` at `:545` is a struct conversion. Echo map at `:568`.
- `user/gorm_repo.go:129` `UpdatePreferences` column map.
- `ProfileResponse` `:26` (own-profile branch, `:325`).
- `auth/handler.go:31` `RegisterResponseData` and `authResponseData` `:153`.
- `migrations/000025_*` is the boolean-column precedent. The next migration number is `000033`.

**Client**
- `features/room/components/InviteFriendsDialog.tsx`: wrap the existing list as the Friends tab and add a Lobby tab (`shared/components/ui/tabs.tsx`; `RulesPage.tsx` uses it). `shared/hooks/useDebounce.ts` debounces the search.
- `shared/api/rooms.ts:34`, `shared/hooks/queries/useInvitableFriends.ts`, `shared/api/queryKeys.ts:7`, `shared/hooks/mutations/useRooms.ts:55`: add a sibling list call, query and key, and have the mutation invalidate both lists.
- `shared/lib/inviteFailure.ts` maps `NOT_FRIENDS` to the reworded message.
- `shared/components/matchStats/MatchPlayerActions.tsx:83-91`: `canReinvite` requires `status === "friends"`. Drop that condition, and drop the `useFriendshipStatus` read from the row if nothing else uses it. Update the row's doc comment to match. `MatchPlayerActions.test.tsx:140` ("no reinvite for a non-friend") must be inverted.
- Preference fields are hand-copied into `shared/types/apiTypes.ts` `User`, `shared/api/auth.ts:13`, `shared/hooks/mutations/useAuth.ts:20`, `shared/api/axiosClient.ts:91`, `shared/api/profile.ts:65/75`, and `test-utils.tsx` `makeUser`.
- `features/profile/components/AudioPanel.tsx` + `shared/lib/audioPreference.ts` are the Switch panel and persist helper to copy. Mount the new panel at `ProfilePage.tsx:152-158`.
- i18n `roomInvite.panel.*`, `roomInvite.errors.notFriends`, and a new `profile.invites.*` in all four locales.

## Tasks & Acceptance

**Execution:**
- [x] `server/migrations/000033_add_allow_non_friend_invites_to_users.{up,down}.sql` -- `BOOLEAN NOT NULL DEFAULT TRUE`, drop on down -- the default is the backfill.
- [x] `server/internal/user/{model,repository,gorm_repo,handler}.go` -- add the field to the model, `PreferencesUpdate`, `IsEmpty`, the column map, the request, the echo and `ProfileResponse` -- one PATCH, one envelope.
- [x] `server/internal/auth/handler.go` -- carry the field in the auth envelope -- the profile switch reads it from the auth store.
- [x] `server/internal/room/invite_handler.go` -- relax the friendship gate, extend the trackers, add `PlayerDirectory` and `ListInvitablePlayers` (`GET /rooms/:id/invitable-players?search=`) -- the server owns eligibility.
- [x] `server/cmd/api/main.go` -- implement the directory on the adapter, wire it and register the route.
- [x] Go tests -- every I/O Matrix row, the stub updates, and the preference PATCH and repo cases.
- [x] Client plumbing (files in the Code Map) -- the new field, API call, query, key and invalidation.
- [x] `client/src/features/room/components/InviteFriendsDialog.tsx` -- add Friends and Lobby tabs, with debounced search and empty, loading and error states on the Lobby tab.
- [x] `client/src/shared/components/matchStats/MatchPlayerActions.tsx` + its test -- show "Invite back" to non-friends -- owner decision; the server is the gate.
- [x] `client/src/features/profile/components/InvitePrivacyPanel.tsx` + `shared/lib/invitePreference.ts` + `ProfilePage.tsx` -- a Switch panel with optimistic persist and revert.
- [x] `client/src/shared/i18n/{en,sr,hr,mk}.json` -- new and reworded keys in all four locales, with no em dash in sr, hr or mk.
- [x] Client tests -- the dialog tabs, the panel (including revert), `inviteFailure`, and that the panel never mounts on `PublicPlayerProfilePage`.

**Acceptance Criteria:**
- Given a room member opens the invite dialog, when they switch to Lobby, then idle non-friends who allow it are listed and can be invited exactly like friends.
- Given a player turns the switch off on their profile, when another player's Lobby tab refreshes, then they are absent and a direct invite POST is refused.
- Given an existing account before the migration, when the migration runs, then it accepts non-friend invites.
- Given `make lint` and `make test`, when run, then both pass.

## Implementation Notes

- **No GORM `default` tag on `AllowNonFriendInvites`.** `server/CLAUDE.md` forbids it, because GORM would then drop `false` from INSERTs. So unlike `sound_enabled`, both user-creation paths (`auth/handler.go` Register and `auth/sso_handler.go`) set `AllowNonFriendInvites: true` explicitly. Any future path that creates a user must do the same, or the account starts opted out. `TestGormUserRepository_AllowNonFriendInvitesDefaultTrueOnRawInsert` and `TestRegister_AcceptsNonFriendInvitesByDefault` pin the default.
- **The lobby list makes more queries than the Design Note says:** one per room status (waiting and playing), a friends list (one friendships query plus one users query), then the candidate-users query. That is still cheap at lobby scale.
- **Wording:** the dialog's open button and title changed from "Invite friends" to "Invite players" in all four locales. The mk, sr and hr strings are the implementer's own and await owner review. The new hr `lobbyLoadFailed` uses "ponovo" to match its pre-existing neighbours in `roomInvite.panel`; most of hr.json uses "ponovno".
- **Verification gotcha:** `internal/user` integration tests read `BELJOT_DB_URL`, falling back to the dev DB on 5433, not `BELJOT_TEST_DB_URL`. Setting only the latter skips them. Both variables were set for the verification run against a throwaway Postgres 16 container.
- **Local environment:** installed prettier is 3.8.1 while the lockfile pins 3.9.10, so `make lint` flags three untouched files. `npx prettier@3.9.10 --check .` is clean. RankChangeDialog's Grandmaster test fails locally because the host locale is mk-MK; it is a known non-regression.

## Spec Change Log

## Review Triage Log

Pass 1 (2026-10-26). Reviewers: Blind Hunter (BH), Edge Case Hunter (EC) and Verification Gap (VG). Routes: **patch** = fixed in this pass; **reject** = no change. No intent_gap or bad_spec findings, so no loopback.

| # | Finding | Verdict | Evidence | Route |
|---|---------|---------|----------|-------|
| EC1 | Reopening the dialog on the Lobby tab fetches with the stale debounced term and can show "No lobby players match '<old>'" under a blank input | low | Confirmed: the open effect resets `search`, but `useDebounce` returns the previous term for 250 ms and `tab` is not reset. Direct one-line fix. | patch (G1) |
| BH8 | Same stale-results-on-reopen defect | low | Same root cause as EC1. | patch (G1) |
| VG-other | Same stale debounced term on reopen | low | Same root cause as EC1. | patch (G1) |
| VG4 | The search reset on reopen is never tested | low | Pre-verified: `renderDialog` always renders `open`, and no test rerenders closed then open. | patch (G1) |
| VG1 | The supersede-guard test passes with the guard deleted | low | Pre-verified: with two toggles the stale `previous` equals the latest value, so a third toggle is needed to tell them apart. | patch (G2) |
| VG2 | No test covers the friends list in a full room after the move to `markUnavailableWhenFull` | low | Pre-verified: no `doListInvitable` caller seeds a full room. | patch (G3) |
| VG3 | No request-contract test pins the Lobby URL and `search` param | low | Pre-verified: the dialog test mocks `@/shared/api/rooms`, the server helper builds its own URL, and there is no `rooms.test.ts`. | patch (G4) |
| VG5 | No handler test asserts `avatarUrl` on Lobby rows | low | Pre-verified: `stubPlayerDirectory` never sets `AvatarURL`, and only the friends list has an avatar test. | patch (G5) |
| BH15a | Same missing Lobby avatar test | low | Same as VG5. | patch (G5) |
| BH3 | Both migration comments misstate facts | low | Confirmed: `sound_enabled` and `music_enabled` do carry `default:true`, so the up comment's reason is wrong. With the code rolled back, non-friend invites don't exist, so the down comment's consequence is wrong. Direct comment correction. | patch (G6) |
| BH1 | `InviteToRoom` checks the opt-out before availability, so any member can probe any user ID's opt-out, even offline users | low | Real. The frozen matrix maps an opted-out non-friend to `NOT_FRIENDS`, so the refusal is specified. The proposed reorder would be worse: busy or offline opted-out users would answer `FRIEND_NOT_AVAILABLE` and idle ones `NOT_FRIENDS`, newly leaking an opted-out player's presence. Today an opted-out player is exactly as visible as before the feature, apart from the setting bit. Probing needs hand-crafted POSTs. | reject |
| BH2 | The server `NOT_FRIENDS` message is now wrong | false | `ErrNotFriends` is used only by the invite handler (`invite_handler.go:245`). The refusal still happens because the invitee is not a friend, so "you are not friends with this player" remains true. The client shows its own reworded text. | reject |
| BH4 | Default TRUE relies on each creation path remembering it | low | Only two production creation paths exist (Register and SSO); both set it, and tests pin it. `server/CLAUDE.md` forbids the GORM default tag. A constructor would add public surface for a hypothetical future path, which Implementation Notes already document. | reject |
| BH5 | Third copy of the "idle in lobby" rule; `idleLobbyCount` ignores playing rooms | low | Players in a playing room are already excluded through `InMatchUserIDs`, so the copies differ only in a transient window. Consolidating them is a refactor beyond a direct correction. | reject |
| EC4 | Full candidate scan per keystroke; an `IN` list over 65535 params would fail | low | Bounded by concurrently connected users, far below 65535 at this game's scale, so the per-request cost is small. Pushing filter, sort and limit into SQL is more than a direct correction. | reject |
| BH6 | Same scan cost; `SELECT *` loads the password hash; `search` has no length cap | low | Same as EC4. The hash never leaves the adapter. An oversized `search` only costs a lowercase and substring check, inside echo's URL limits. | reject |
| BH7 | The 50-row cap is silent, with no "refine your search" hint | low | The cap is specified in the frozen matrix. Over 50 idle non-friends is unlikely at current scale, and a hint needs new UI and four locales. | reject |
| EC2 | Two overlapping toggles that both fail leave the switch OFF while the server is ON | low | Real: the latest failure reverts to its own `previous`. It needs two toggles within one round trip and both requests to fail, and it is the same pattern as `audioPreference` and `cardDeckPreference`, which the frozen spec says to follow. A fix needs baseline tracking, more than a direct correction. | reject |
| EC3 | Rapid toggles that both succeed out of order leave the server on the older value | low | Same pre-existing persist pattern. Out-of-order arrival of two PATCHes sent milliseconds apart is rare, and serialising writes adds complexity. | reject |
| EC5 | No cooldown after a decline lets strangers loop invite popups | false | Out of scope by owner decision. The frozen Never clause excludes any decline cooldown or new rate limit. | reject |
| EC6 | A pending non-friend invite stays usable for 60 s after the invitee opts out | false | The grant can be consumed only by the invitee choosing to accept it. Nobody else gains anything, and they can decline. | reject |
| BH9 | The tabs have no `tabpanel`, so the tablist controls nothing | low | Real but minor: content stays reachable and the tabs announce their selected state. Same pattern as `RulesPage`. `tabs.tsx` exports no panel part, so the fix adds a UI-kit export. | reject |
| BH10 | A failed privacy-switch save reverts silently | low | The frozen spec prescribes the audio pattern (optimistic, revert on failure), and the revert itself is visible. Failure needs a network or server error. A toast needs new strings in four locales. | reject |
| BH11 | The invite popup lacks friend context and an opt-out shortcut | low | An enhancement, not a defect. The frozen spec keeps the WS contract unchanged. | reject |
| BH12 | The Lobby lists strangers who fail the honor, new-player or buy-in rules, and doesn't mark pending invites | low | Matches the specified Story 11.5 behaviour of checking at join time. A pending re-invite already resolves to "Invited" via `INVITE_ALREADY_PENDING`. Pre-filtering is new logic. | reject |
| BH13 | Names still say "friend" for things that now cover any player | low | Developer-only naming drift. Renaming across both tiers and test IDs is broad churn. | reject |
| BH14 | Locale wording inconsistent (formal vs informal, hr "sa", mk "лобито", missing full stop) | false | Each string follows its own section: profile is formal ("Odaberite…", "Изберете…") and the room panel informal. hr `lobbyNoMatch` mirrors the existing `lobby.playerSearch.empty` ("sa „{{query}}“"), and the en full stop matches it too. mk.json already uses both "лоби" and "лобито". | reject |
| BH15b | No test for `/invitable-players` on a quick-play or non-waiting room | low | That path is the shared `requireRoomMember`, already covered by the friends-endpoint tests. | reject |
| BH15c | `h.players` has no nil guard, and a nil `connections` means different things in two places | false | Production always wires non-nil collaborators (`main.go`). A nil tracker is a test-only seam, so no reachable path panics or misreports. | reject |

## Design Notes

Lobby set: `ConnectedUserIDs` minus `InMatchUserIDs` minus users in waiting or playing rooms, minus the caller and the caller's friends. Resolve the rest through `PlayerDirectory` and drop opted-out players. Then trim the search term, filter case-insensitively, sort by lower-cased username and cap at `lobbyInviteListLimit = 50`. The set comes from in-memory presence plus one room query and one users query, so it stays cheap at lobby scale.

## Verification

**Commands:**
- `make lint` -- expected: tsc, eslint, prettier and golangci-lint all clean.
- `make test` -- expected: vitest (including i18n parity) and `go test -tags nodynamic ./...` green.

**Manual checks:**
- With two accounts in the lobby and one in a room, invite the non-friend from the Lobby tab and accept. Then switch the invitee's preference off and confirm they disappear from the tab.
