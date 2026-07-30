---
baseline_commit: a3631ef6ff507e6c376c90b3f8aaabf6299821c5
---

# Story 9.8: Honor-Gated Rooms

Status: done

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

## Story

As a room owner,
I want to optionally require a minimum honor score and decide whether unproven players may enter,
so that I can self-select into a table of people who finish the matches they start.

## Task 0 — branch prerequisite (do this before anything else)

Story 9.7 is `done` but **not merged**: `feat/9-7-honor-score-system` is 3 commits ahead of `master` (`a3631ef`, `ee556df`, `50be7d6`) and `master` is still at `310b4e6`. Every symbol this story consumes — `user.HonorScore`, `user.IsNewPlayer`, `user.NewHonorSnapshot`, the six `users.honor_*` columns, migration `000017` — lives only on that branch.

- Merge 9.7 to `master` first, then cut `feat/9-8-honor-gated-rooms` from `master`.
- If 9.7's PR is not merged yet, cut `feat/9-8-honor-gated-rooms` from `feat/9-7-honor-score-system` and re-target the PR after 9.7 lands.
- **Do not branch from `master` as it stands** — nothing in this story will compile.

Migration `000017` is the current highest, so this story owns **`000018`**. Confirm that before naming the file; never skip numbers.

## Scope guardrails — read before Task 1

**This story gates room access on the honor 9.7 already computes. It does not change how honor is calculated, stored, decayed or displayed.** Do not touch `server/internal/user/honor.go`, migration `000017`, `HonorPanel.tsx`, `client/src/shared/lib/honor.ts`'s math, the honor trend, or `event:honor_updated`. If you find yourself editing the formula, you have left the story.

**Explicitly out of scope — do NOT build:**

| Not in this story | Where it belongs |
| --- | --- |
| An owner endpoint to edit `min_honor` / `allow_new_players` after creation | Follow-up story. 9.6 shipped `POST /rooms/:id/privacy` because its AC6 demanded it; 9.8's ACs demand create-time config only. See Open Question 1. |
| Filtering gated rooms out of the browse list | AC5 — gated rooms stay **listed and labelled**, exactly like 9.6's "listed but locked" private rooms. |
| A lobby filter/sort by honor requirement | Not requested. `FilterRail` stays as-is. |
| Cross-player honor visibility (seeing a seatmate's honor in the room) | Epic 11, Story 11.3. |
| Any admin / forgiveness surface | Story 9.7 D7 — still no admin system in this project. |
| A new abandonment trigger, or any change to what drops honor | Frozen by `spec-abandonment-per-player-results.md`. |

**Four epic ACs are overridden by decisions in this file (D1-D4 below). They are binding — implement the ACs in *this* file, not the ones in `epics.md`.**

**Do not trust `prd.md:150`** ("< 20 completed matches labeled New Player") or the epic's "< 5 **completed** matches". Both are stale. The floor is `completed + abandoned < 5` — experience, not successes — amended by PO decision 2026-07-29 during 9.7's review specifically because a 0-completed / 20-abandoned account (real score **5**, "problematic") was hiding behind the newcomer chip, *and because this story's gate reads the same flag*. See D2.

## Design decisions (binding)

### D1 — The two gates are INDEPENDENT. (Overrides the epic's `min_honor > 0` scoping)

The epic nests both rejections under **"Given a player attempts to join a room with `min_honor > 0`"**, which literally means `allow_new_players = false` does nothing in a room with `min_honor = 0`. That contradicts the same epic's own AC1 ("a **separate** 'Allow New Players' toggle") and AC3 (the "Veterans only" card indicator is not conditioned on `min_honor`).

**The gate is therefore this function, and nothing else:**

```text
gateOK(minHonor, allowNewPlayers, isNewPlayer, honorScore):
    if isNewPlayer:  return allowNewPlayers          // the score is NOT consulted
    else:            return honorScore >= minHonor   // minHonor 0 => always true
```

Read the two branches carefully, because the non-obvious half is the one a reviewer will check first:

- **A New Player is never score-checked.** With `allow_new_players = true` a newcomer enters a `min_honor = 95` room even though their score is the 80 prior. That is the entire purpose of the toggle: it is the owner's explicit "I'll take an unknown" switch. Removing that branch would make the toggle meaningless.
- **A room is "ungated" only when `minHonor == 0 AND allowNewPlayers == true`.** That is the overwhelmingly common case, so every call site must short-circuit on it and perform **no honor read at all** (D5).

### D2 — "New Player" means `completed + abandoned < 5`, via `user.IsNewPlayer`. (Overrides epic AC2's "< 5 completed matches")

Call `user.IsNewPlayer(completedTotal, abandonedTotal)` — never re-derive the floor, never compare `honorCompletedTotal` alone, and never read `honorNewPlayerMinMatches` (it is unexported for exactly this reason).

Why this matters here more than anywhere else: the account this story exists to keep out — someone who abandons repeatedly — is the account a completions-only floor labels a New Player forever. Under the stale floor, `allow_new_players = true` (the default) would admit them into every gated room while suppressing their real score of 5. 9.7's review closed that bypass twice; do not reopen it.

### D3 — The mid-session re-check fires at BOTH insolvency checkpoints, not just at start

Epic AC5 says the re-check runs "when a new match within the same room is about to start … **alongside the insolvency check (Story 9.3)**". Story 9.3 has **two** insolvency checkpoints, and the phrase points at that mechanism as a whole:

1. **`ReturnToRoom`** (`handler.go:1450-1458`) — the return-time gate.
2. **`StartMatch`** (`handler.go:2850-2907`) — the authoritative pre-charge gate.

Both get the honor check. The reason is not symmetry, it is *when honor actually moves*: honor changes only at match end, and the abandoning player's `room_players` row survives (9.3 AC3 holds them as seated). So the realistic sequence is **abandon → reconnect → "Return to room" → gate fires**. Gating only at `StartMatch` would leave the ejected-to-be player sitting in the room until the owner clicks Start, at which point the match refuses to start and the owner has to click again for no visible reason.

Note also that a *completed* match raises honor, and decay lowers it only over months. **Abandonment is effectively the only thing that trips the mid-session gate.** Write the tests around that scenario, not around a hypothetical slow drift.

### D4 — The mid-session push is `system:honor_ejected`, NOT `event:honor_eject` — zero drift-gate touchpoints

The epic writes `event:honor_eject`. That prefix is wrong for this event and would drag in work that is not needed:

- Its sibling is `system:insolvent_ejected` (`ws/events.go:301`) — a pre-match, room-lifecycle, per-user push. `event:` is reserved for in-match game-state events, and the ordering contract in `handleMatchEnd` / `handleSeatReconnectTimeout` has nothing to do with this.
- **`system:*` payloads are not in the WS drift gate.** `events_contract_test.go`'s `cases` table contains only `event:*` typed payloads — `InsolventEjectedPayload`, `RoomClosedInsolventPayload` and `PlayerReturnedPayload` are all absent (verified). So there is **no golden, no Zod schema, no `MutualExtends` witness, no `cases` row** for this event. Naming it `event:honor_eject` would silently pull all six touchpoints into scope.

Two files only: `ws/events.go` (const + typed payload) and `wsEvents.ts` (const + interface). Same as Story 9.3 did for `system:insolvent_ejected`.

**Also**: `epics.md:1616` predicts Epic 9 will append `"honor_eject"` to the `match_abandoned` `reason` union. It will not. That union describes how a **live match** ended; an honor ejection happens pre-match, at the room level, and never produces a `match_abandoned`. 9.3 correctly never appended `"insolvency"` either. Leave the union alone.

### D5 — The gate reads the AUTHORITATIVE recomputed score, and only when the room is gated

Never read `users.honor_score` (the `User.HonorScoreSnapshot` field). It is a denormalized, deliberately-lagging column for SQL filtering only; 9.7's model comment says so in capitals, and a decayed score moves with wall-clock time even when nothing is written. Go through `user.NewHonorSnapshot(...)`, which recomputes from three columns of a row already in hand.

And short-circuit hard: `JoinRoom` is a hot path and almost every room is ungated. `if room.MinHonor == 0 && room.AllowNewPlayers { /* skip the honor read entirely */ }`.

### D6 — `room` reads honor through a locally-declared interface; `room` may import `user`

`WalletService` (`handler.go:118-123`) is the precedent: a narrow interface declared in `room`, satisfied by a concrete service, injected from `main.go`, nil-tolerant.

The import direction is the one thing to get right. `user` **must never import `room`**, because `room -> auth -> user` already exists and `user -> room` would close the cycle. Therefore the DTO crossing the interface must be a **`user`-owned type**, and `room` imports `user`:

```go
// room/handler.go — mirrors WalletService above it.
type HonorService interface {
    HonorForUsers(userIDs []uint) (map[uint]user.HonorSnapshot, error)
}
```

Verified acyclic: `room -> user -> match` and `match` does not import `room` (`grep -rn "internal/room" internal/user internal/match` -> zero hits). `internal/room/privacy_handler_test.go` already imports `user`, so test stubs are unproblematic.

**No new `UserRepository` method is needed** — and therefore no mock churn in `user/handler_test.go`, `user/xp_service_test.go`, `match/*_test.go`. `FindManyByIDs(ids) ([]User, error)` already returns every honor column; `HonorForUsers` is a pure addition to `*user.HonorService` that maps those rows through `NewHonorSnapshot`. Adding it to the concrete service, not to the repository interface, is what keeps the blast radius at zero.

### D7 — The owner must satisfy their own gate at create time

Not in the epic, and a real hole. An owner who sets `min_honor = 95` while sitting at 60, or who sets `allow_new_players = false` while being a New Player themselves, has barred *themselves* from their own room. `CreateRoom` auto-seats the creator (`handler.go:483-490`), so at the first `StartMatch` the mid-session re-check (AC6) would eject the owner from a room they made one minute earlier, then transfer ownership or close the room.

`CreateRoom` already blocks a creator who cannot afford their own buy-in (`handler.go:427-435`) for the identical reason. Mirror it exactly: reject with the same code the join gate would have returned.

### D8 — `ejectInsolventAtStart` / `ejectInsolventReturner` are GENERALIZED, not duplicated

Both are ~140 lines of review-hardened transaction + ownership-transfer + best-effort fan-out. Copy-pasting them for honor would double the surface of the two functions most likely to regress. Parameterize instead: the tx body, the ownership move, the revert-to-`waiting`, the `player_left` / `room_updated` / `room_closed_insolvent` fan-out and the failure logging all stay **byte-identical**; only the per-user WS push and the returned `apperr` become parameters. See Task 5 for the exact shape.

## Authoritative spec

### The gate, as a truth table

`isNewPlayer` and `honorScore` both come from `user.HonorSnapshot`. Every row must have a test.

| # | minHonor | allowNewPlayers | isNewPlayer | score | Result |
| --- | --- | --- | --- | --- | --- |
| 1 | 0 | true | any | any | **admit** — ungated room, no honor read performed |
| 2 | 0 | false | true | any | **reject** `NEW_PLAYER_NOT_ALLOWED` (D1: independent of minHonor) |
| 3 | 0 | false | false | 5 | **admit** — experienced, and `5 >= 0` |
| 4 | 80 | true | true | 80 | **admit** — a New Player is never score-checked (D1) |
| 5 | 80 | true | true | 19 | **admit** — same rule, at the worst score a newcomer can hold |
| 6 | 80 | false | true | 80 | **reject** `NEW_PLAYER_NOT_ALLOWED` |
| 7 | 80 | true | false | 79 | **reject** `HONOR_TOO_LOW` |
| 8 | 80 | true | false | 80 | **admit** — boundary is inclusive (`>=`) |
| 9 | 100 | true | false | 99 | **reject** `HONOR_TOO_LOW` — see the `min_honor = 100` note below |
| 10 | 80 | false | false | 95 | **admit** — the newcomer bar does not apply to an experienced player |

Precedence when both branches could apply: `isNewPlayer` is evaluated first, so a New Player in a `min_honor = 80` / `allow_new_players = false` room gets `NEW_PLAYER_NOT_ALLOWED`, never `HONOR_TOO_LOW` (row 6).

**Two arithmetic facts that constrain which fixtures are legal — get these wrong and you will write a test for a state the system cannot produce:**

- **A New Player's score can never be below 19.** The floor caps a newcomer at 4 finished matches, so their worst case is 0 completed / 4 abandoned: `100 × 4 / (4 + 4·4 + 1) = 400/21 = 19`. Row 5 uses 19 for that reason. A fixture pairing `isNewPlayer = true` with a score of, say, 5 is unreachable — reaching 5 takes ~19 abandonments, which puts the account far past the floor.
- **`min_honor = 100` is very nearly a closed room.** The Beta(4,1) prior means `100 × (C+4) / (C+5)` only rounds to 100 once the decayed completed weight `C ≥ 195` — roughly 195 finished matches inside a single 90-day half-life. Do not "fix" row 9; a 100-gate genuinely excludes almost everyone, which is why the field accepts it but the create-modal hint should say what a realistic threshold looks like. Raised with the PO as Open Question 6.

### Where the gate runs, and in what order

| Call site | Gate? | Notes |
| --- | --- | --- |
| `CreateRoom` | yes — on the **creator** (D7) | After the existing buy-in affordability check, before hashing the password. |
| `JoinRoom` | yes | **Appended last**, after the existing coin check (`handler.go:731-739`). Do not reorder the password / capacity / already-in-room / coin checks — a hardened sequence, and appending keeps the diff reviewable. A player who is both broke and short on honor gets `INSUFFICIENT_COINS`; that is fine and intentional. |
| `ReturnToRoom` | yes (D3) | After the existing insolvency gate (`handler.go:1450-1458`). |
| `StartMatch` | yes (D3) | **BEFORE** the whole coin block (`handler.go:2850`). This ordering is load-bearing: `ChargeStakes` commits money, so an honor ejection discovered after it would need a refund. Gate first, charge second. |
| `QuickPlay` / `QuickJoin` | no | Synthesized rooms are ungated by construction (AC12). No new error code, no bracket interaction. |
| `SelectSeat` / `LeaveSeat` / `SwapSeats` / `AddBot` / `KickPlayer` / `TransferOwnership` | no | Seat mechanics inside a room the player already passed the gate for. |

### New errors (`internal/apperr/errors.go`)

Add next to the private-room block (`:119-132`), which is the closest sibling — an HTTP-only join gate with no WS error event.

| Var | Code | Status | Returned from |
| --- | --- | --- | --- |
| `ErrHonorTooLow` | `HONOR_TOO_LOW` | 409 | join gate, return gate, `CreateRoom` self-gate, `StartMatch` (to the owner) |
| `ErrNewPlayerNotAllowed` | `NEW_PLAYER_NOT_ALLOWED` | 409 | same four sites |
| `ErrInvalidMinHonor` | `INVALID_MIN_HONOR` | 400 | `CreateRoom` validation |

409 (not 402/403) matches `INSUFFICIENT_COINS` and `WRONG_ROOM_PASSWORD`. Per 9.2 Decision B and 9.6's precedent, the error payload carries **only the code** — the client composes the "requires honor >= X, yours is Y" copy locally from the room object and its own auth envelope. Do not put numbers in the error message.

### `system:honor_ejected`

```go
// ws/events.go, beside SystemInsolventEjected (:296-301)
const SystemHonorEjected = "system:honor_ejected"

type HonorEjectedPayload struct {
    RoomID   uint `json:"roomId"`
    MinHonor int  `json:"minHonor"`
    Honor    int  `json:"honor"`
}
```

`Honor` is the ejected player's authoritative recomputed score. Not drift-gated (D4).

**The room-close event is REUSED, not renamed.** If the owner is honor-ejected and no eligible heir remains, the room closes and every still-seated member receives the existing `system:room_closed_insolvent`. Its payload is `{roomId}` and its client copy (`roomClosedTitle` / `roomClosedBody`) is already reason-agnostic, so it is behaviourally correct for an honor close. Renaming the wire const would break stale tabs for zero user-visible gain. Add one line to its doc comment recording that it now covers honor closes too — do not touch the string.

## Acceptance Criteria

**AC1 — Schema (migration `000018_add_honor_gate_to_rooms`).**
Given the up migration
When I inspect `rooms`
Then it has `min_honor SMALLINT NOT NULL DEFAULT 0 CHECK (min_honor BETWEEN 0 AND 100)` and `allow_new_players BOOLEAN NOT NULL DEFAULT TRUE`
And `SMALLINT` is chosen because the value is bounded 0-100 (same reasoning as `users.honor_score` in `000017`)
And `DEFAULT TRUE` on `allow_new_players` backfills every existing room as open, so no live room silently becomes veterans-only at deploy
And **no backfill statement is needed** — unlike `000017`, both defaults are correct for every existing row
And `.down.sql` drops both columns in reverse order with a `-- Reverse 000018 by ...` header
And the roundtrip is verified on the dev DB (port **5433**, docker `beljot-postgres-1`): `make migrate` -> `migrate down 1` -> `migrate up`, inspecting column state and CHECKs at each step

**AC2 — Room model, and the GORM default-value trap.**
Given `server/internal/room/model.go`
When the columns are added to the `Room` struct
Then `MinHonor int` carries `gorm:"not null;default:0" json:"minHonor"`
And `AllowNewPlayers bool` carries **`gorm:"not null"` with NO `default` tag** and `json:"allowNewPlayers"`
And a comment on the field states why, because this is a silent-data-corruption trap and not a style preference:

> GORM does not send a zero-valued field (`0`, `""`, `false`) in an `INSERT` when that field declares a `default` tag — it lets the database apply the default instead. With `gorm:"default:true"` it would be **impossible to create a room with `allow_new_players = false`**: the value would silently flip to `true`. (Confirmed against the GORM docs: "zero values like 0, '', false won't be saved into the database for those fields defined default value … use a pointer type or Scanner/Valuer to avoid this.") Omitting the tag makes GORM send the real boolean every time; the DB-side `DEFAULT TRUE` still covers the migration backfill and any raw insert.

And the inverse trap is closed by construction: with no GORM default, a hand-built `&Room{...}` that *forgets* the field inserts `false` (veterans-only). **Both** `&Room{}` sites therefore set it explicitly — `CreateRoom` (`handler.go:458`) and the Quick Play synthesis (`handler.go:3027`)
And two tests pin both directions: `allow_new_players = false` survives a create-and-read roundtrip (this is the test that fails if someone adds `default:true`), and a room created with the field omitted from the request comes back `true`
And **that roundtrip test MUST be DB-backed** — reuse `getRoomTestDB(t)` (`privacy_handler_test.go:355-367`: `BELJOT_DB_URL`, per-test transaction, `t.Skip` when no DB, dev DB on port **5433**). A `mockRoomRepo` test cannot catch this bug at all: the trap lives in GORM's SQL generation, and an in-memory mock stores whatever Go value it is handed. A green mock-level test here is worse than no test, because it certifies the exact thing it cannot observe
And rejected alternatives are recorded in the comment: `*bool` (drags nil-handling into every read for one write-path problem) and inverting the column to `veterans_only` (deviates from the AC-mandated column name and installs a permanent double negative)

**AC3 — Create-room configuration.**
Given the create-room request
When `CreateRoomRequest` is extended
Then it gains `MinHonor *int json:"minHonor"` and `AllowNewPlayers *bool json:"allowNewPlayers"` — **both pointers**, mirroring `CoinBuyIn *int` (`handler.go:94`), so "omitted" is distinguishable from an explicit `0` / `false`
And nil `MinHonor` defaults to `0`; a value outside `[0,100]` rejects with `ErrInvalidMinHonor`
And nil `AllowNewPlayers` defaults to **`true`**
And the creator must satisfy their own gate (D7): evaluate `gateOK` against the creator's own honor and reject with `ErrHonorTooLow` / `ErrNewPlayerNotAllowed`, placed immediately after the existing buy-in affordability check (`handler.go:427-435`) and before the password hashing
And the server is the authority — the modal's disabled-submit guard is cosmetic, exactly as it is for `coinBuyIn` and the password

**AC4 — Join gate.**
Given a player calls `POST /rooms/:id/join` on a room where `min_honor > 0` or `allow_new_players = false`
When the request is processed
Then the gate is evaluated per the D1 truth table using `user.HonorSnapshot` values obtained through `HonorService.HonorForUsers`
And a New Player barred by the toggle is rejected with `ErrNewPlayerNotAllowed`; an experienced player below the threshold is rejected with `ErrHonorTooLow`
And the check is appended **after** the existing coin check, leaving the password / capacity / already-in-room / coin sequence untouched
And an ungated room (`min_honor == 0 && allow_new_players`) performs **no honor read at all** — asserted by a test that fails if the stub's call counter moves
And a nil `honorService` skips the gate entirely (mirroring the nil-`walletService` affordance), and a honor-read error fails the join with a wrapped 500 rather than silently admitting — a read failure must never open a closed door
And every row of the D1 truth table has a test

**AC5 — Browse and detail visibility.**
Given the lobby room list or a room detail response
When a gated room is rendered
Then `minHonor` and `allowNewPlayers` ride the existing `Room` JSON on every endpoint that serializes the struct (list, detail, by-code, create) — no handler-by-handler plumbing
And **`roomLifecyclePayload` (`handler.go:269-314`) is extended too**, because it is a hand-built `map[string]any`, not the struct: `system:room_created` and `system:room_updated` flow through it, so without those two keys a live lobby card would show a gated room as ungated until the next full refetch. This is the identical trap 9.6 hit with `isPrivate`, which is why that map carries an explicit comment at `:305-307` — add both keys beside it
And the room card shows the requirement only when it exists: an honor chip (e.g. "80+" with a shield glyph) when `minHonor > 0`, and a separate "Veterans only" indicator when `allowNewPlayers === false`
And an ungated room's card is **visually unchanged** from today
And gated rooms are **not filtered out** of the list — mirroring 9.6's "listed but locked" decision, the player sees the requirement and decides
And colour is never the only signal (UX spec: *"No information conveyed exclusively through colour"*) — each chip carries text plus a glyph
And the chips expose `data-testid="room-card-min-honor"` / `data-testid="room-card-veterans-only"` so tests never key on i18n wording
And the create-room modal's live preview card mirrors both chips, keeping its "what the lobby will show" promise honest

**AC6 — Mid-session re-check and ejection.**
Given a seated player's honor has dropped below the room's threshold (in practice: they abandoned the previous match in this room)
When they call `POST /rooms/:id/return`, or the owner calls `POST /rooms/:id/start`
Then the honor gate is re-evaluated and every failing seat is ejected through the **same** flow Story 9.3 uses for insolvency: seat freed and broadcast, ownership transferred or the room closed, `system:room_updated` + `system:player_left` fan-out, and a per-user `system:honor_ejected` push carrying `minHonor` and the player's own score
And the ejected player's client routes to the lobby and shows the ejection modal with "Your honor has dropped below this room's threshold"
And on the return path the caller receives `ErrHonorTooLow` / `ErrNewPlayerNotAllowed` (409)
And on the start path the match **does not start**: the room reverts to `waiting` (unless it closed) and the owner receives the same 409 — matching how `ejectInsolventAtStart` behaves today
And the start-path gate runs **before** the coin block, so no stake is ever charged and refunded for a player who was about to be ejected
And a room that is ungated skips both re-checks with no honor read

**AC7 — Ownership eligibility includes the honor gate.**
Given the owner is honor-ejected and ownership must move
When `transferOwnershipOrClose` picks an heir
Then the eligibility predicate requires the candidate to **also pass the honor gate**, on top of the existing per-path requirements (present-AND-solvent on the return path, solvent-only on the start path)
And if no eligible heir remains the room closes and every still-seated member receives `system:room_closed_insolvent` (reused per the spec above)
And this is not hypothetical: an abandonment charges **every absent human seat** (9.7 AC3, review pass 2), so multiple seats can drop below the threshold from a single match end — a test must cover an owner plus one other seat failing together

**AC8 — WS contract.**
Given `system:honor_ejected` is added
Then `server/internal/ws/events.go` gains the const and `HonorEjectedPayload`, and `client/src/shared/types/wsEvents.ts` gains the matching const and interface
And **no golden, no Zod schema, no conformance witness and no `cases` row are added** — `system:*` payloads are outside the drift gate (D4). A contributor who adds them has misread the gate's scope
And `useWsDispatch.ts` validates `roomId`, `minHonor` and `honor` with `typeof === "number"` before use — never JS truthiness, since a real score of `0` is a legitimate Go value
And `SystemRoomClosedInsolvent`'s doc comment records that it now also covers honor closes, with its wire string unchanged

**AC9 — Client join-error handling at all three entry points.**
Given a join is rejected by the honor gate
Then all **three** join entry points surface it: the lobby card (`LobbyPage.joinRoomFlow`), join-by-code (`JoinByCodeTile.joinResolvedRoom`), and the deep-link / refresh auto-join (`RoomPage`, which has **two** call paths — the public auto-join and `joinDeepLinkPrivate`)
And `HONOR_TOO_LOW` composes the message locally from the room's `minHonor` and the viewer's own `honorScore`; `NEW_PLAYER_NOT_ALLOWED` shows the veterans-only copy
And join-by-code, which has no room object in scope for the public path, uses a param-less generic variant — the precedent `room.errors.insufficientCoinsGeneric` already set (`JoinByCodeTile.tsx:42-45`)
And the viewer's own score is read through `honorScoreOrPrior(...)`, never `user.honorScore || 80` — a real 0 must survive (this exact bug was patched in 9.7's review)
And 9.6's lesson is honoured: **check every entry point, not just the primary one** — its deep-link path needed the identical fix the main path got, and was found only in manual E2E

**AC10 — The ejection notice is generalized, not duplicated.**
Given the client already has a single-notice ejection pipeline for insolvency
When honor ejection is added
Then `roomStore.insolventEjection` is **renamed** to `roomEjection` with type `RoomEjection`, its `reason` widened from `"ejected" | "roomClosed"` to `"insolvent" | "roomClosed" | "honor"`, and honor's `minHonor` / `honor` numbers added
And `InsolventEjectionModal` -> `RoomEjectionModal`, `useInsolventEjectRedirect` -> `useRoomEjectRedirect`, with the `reset()`-preserves-the-notice behaviour and the always-mounted redirect unchanged
And the rename lands as its **own commit with no behaviour change**, before the honor branch is added, so the honor diff stays reviewable
And a field named `insolventEjection` holding `reason: "honor"` is not acceptable: 9.7's review pass 2 spent ten patches on comments and names that had stopped telling the truth, two of which would have walked this story's implementer straight back into a closed bug

**AC11 — i18n x4.**
Given honor-gate strings are added
Then all four of `en.json` / `sr.json` / `mk.json` / `hr.json` gain 1:1 keys for: the create-room "Minimum honor" field + hint, the "Allow new players" toggle + its two option labels, the two create-time self-gate errors, the card's honor chip and "Veterans only" indicator (plus `aria-label`s), the two join rejection toasts and the generic by-code variant, and the ejection modal's honor title + body
And the `lobby.insolventEjection.*` block is renamed to `lobby.roomEjection.*` with reason-specific sub-keys, in step with AC10
And `mk.json` copy is **all-Cyrillic**
And **no em dash (`—`) appears in the mk / sr / hr strings** (English only)
And no existing `string` key is being replaced by an `object` (JSON cannot hold both — this collision bit Story 7-2)
And `i18n.parity.test.ts` passes

**AC12 — Quick Play stays ungated.**
Given Quick Play matchmaking
When a room is synthesized (`handler.go:3027`)
Then it sets `MinHonor: 0` and `AllowNewPlayers: true` **explicitly**, with a comment mirroring the existing `PasswordHash stays nil` note (`handler.go:3045-3047`)
And `QuickJoin` / `QuickPlay` gain no honor gate and no new error code
And the matchmaking pool query (`FindQuickPlayRoom` / `FindQuickPlayRoomExcluding`, keyed on `buyIn`) is **not** widened — there is no honor bracket
And a test asserts a synthesized quick-play room comes back ungated (this is the test that catches the AC2 trap in the wild: a missing explicit `AllowNewPlayers: true` would make every Quick Play table veterans-only)

## Tasks / Subtasks

- [x] **Task 0 — Branch** (see "Task 0" above)
  - [x] Merge 9.7 to `master`, then cut `feat/9-8-honor-gated-rooms` from `master` (fallback: branch from `feat/9-7-honor-score-system`)
  - [x] Confirm `000017` is the highest migration before naming `000018`

- [x] **Task 1 — Migration `000018_add_honor_gate_to_rooms`** (AC: 1)
  - [x] Up: two `ALTER TABLE rooms ADD COLUMN` statements, each with a `--` comment block on value semantics and type choice, following `000012_add_room_password.up.sql`'s prose style
  - [x] Header must state: no backfill needed (both defaults are correct for existing rows), and that `DEFAULT TRUE` is what keeps live rooms open at deploy
  - [x] Down: drop both in reverse order with a `-- Reverse 000018 by ...` header
  - [x] Verify on dev DB 5433: `make migrate` -> `migrate down 1` -> `migrate up`; inspect columns + CHECK at each step and record it in the Debug Log

- [x] **Task 2 — Room model** (AC: 2, 12)
  - [x] Add `MinHonor` / `AllowNewPlayers` to `room.Room` (`model.go:9-49`), placed next to `CoinBuyIn` / `PasswordHash` so the economy+access block stays together
  - [x] Write the GORM default-tag warning comment from AC2 verbatim in substance
  - [x] Set `AllowNewPlayers` explicitly at **both** `&Room{}` sites: `CreateRoom` (`handler.go:458`) and Quick Play synthesis (`handler.go:3027`)
  - [x] `AfterFind` (`model.go:56`) needs **no change** — these are real columns, not derived like `IsPrivate`
  - [x] Add both keys to `roomLifecyclePayload` (`handler.go:293-313`), beside the `isPrivate` comment. A missing key here is invisible in every HTTP test and only shows up as a stale lobby card on a live `room_updated` (AC5)

- [x] **Task 3 — Errors + the pure gate function** (AC: 4)
  - [x] Three errors in `apperr/errors.go` beside the private-room block (`:119-132`), each with a comment naming the sites that return it
  - [x] One unexported pure helper in the `room` package — `honorGateError(room *Room, snap user.HonorSnapshot) error` returning `nil` / `ErrNewPlayerNotAllowed` / `ErrHonorTooLow` — so all four call sites share one implementation and one table-driven test covering all ten D1 rows
  - [x] A sibling `func (r *Room) honorGated() bool { return r.MinHonor > 0 || !r.AllowNewPlayers }` for the short-circuit, so no call site re-spells the condition

- [x] **Task 4 — `HonorService` interface + `HonorForUsers` + wiring** (AC: 4, 6)
  - [x] Declare `room.HonorService` beside `WalletService` (`handler.go:108-123`), with the same doc-comment shape: what it is for, why an interface, that nil means no enforcement, and the import-direction note from D6
  - [x] Add `HonorForUsers(userIDs []uint) (map[uint]user.HonorSnapshot, error)` to `*user.HonorService` (`user/honor_service.go`) — built on the existing `FindManyByIDs`, stamping `time.Now().UTC()` once and mapping each row through `NewHonorSnapshot`. Empty input -> empty map, no DB round-trip (mirroring `TotalXPForUsers`). Do **not** add it to the `UserRepository` interface (D6: that is what keeps the mocks untouched)
  - [x] Add the `honorService` field + constructor parameter; update **all 8 call sites**: `cmd/api/main.go:247`, `room/coin_handler_test.go:82,103`, `room/handler_test.go:376,406,463,479,2322`
  - [x] `main.go`: `user.NewHonorService(userRepo)` — the same value already passed to `sessionManager.SetHonorRecorder` (`main.go:189`); construct it once into a local and pass it to both
  - [x] Test stub `stubHonor` in the room test package with a call counter (AC4's "no read on an ungated room" assertion needs it), mirroring `stubWallet`'s shape (`coin_handler_test.go:19-76`)

- [x] **Task 5 — Generalize the two eject helpers** (AC: 6, 7, D8)
  - [x] Introduce an unexported `ejectNotice` describing one seat's ejection (kind + the numbers its push needs) and a small method that turns it into `(msgType string, payload any)`
  - [x] `ejectInsolventReturner` (`handler.go:948`) -> takes a notice and the apperr to return. Body unchanged
  - [x] `ejectInsolventAtStart` (`handler.go:1085`) -> takes `map[uint]ejectNotice` instead of `insolventIDs + balances`. Body unchanged, including the tx-failure `UpdateStatus` un-bricking (`:1153-1164`) and the `txErr == nil && !roomClosed` guard on the `player_left` fan-out (`:1193`)
  - [x] Widen both eligibility closures (`ownerEligible` `:959-986`, `startEligible` `:1091-1100`) with the honor predicate; read the seated humans' snapshots **once, before the transaction** — no honor read inside a row lock, exactly as `GetBalances` is hoisted today
  - [x] Re-run the existing 9.3 insolvency tests unchanged — they are the regression net for this refactor. If any needs editing, the refactor changed behaviour and is wrong

- [x] **Task 6 — `CreateRoom`** (AC: 3, 12, D7)
  - [x] `CreateRoomRequest` += `MinHonor *int`, `AllowNewPlayers *bool` with the "pointer so omitted != explicit zero" comment
  - [x] Range-validate; default nil -> `0` / `true`
  - [x] Creator self-gate immediately after the buy-in affordability check
  - [x] Persist both onto the new `&Room{}`

- [x] **Task 7 — `JoinRoom`** (AC: 4)
  - [x] Append the gate after the coin check (`handler.go:731-739`), guarded by `room.honorGated()`
  - [x] Comment stating the ordering rationale from the spec table, so it does not read as arbitrary

- [x] **Task 8 — `ReturnToRoom` + `StartMatch`** (AC: 6, 7)
  - [x] `ReturnToRoom` (`:1450-1458`): after the insolvency gate, re-evaluate and route through the generalized returner eject
  - [x] `StartMatch`: insert the honor prefilter **before** `if updatedRoom.CoinBuyIn > 0 && h.walletService != nil` (`:2850`). Comment the "gate before charge, never charge then refund" reason
  - [x] Both paths: `room.honorGated()` short-circuit first

- [x] **Task 9 — WS event** (AC: 8)
  - [x] `ws/events.go`: const + `HonorEjectedPayload` beside `SystemInsolventEjected` (`:296-322`), with a comment stating it is deliberately `system:` and therefore outside the drift gate
  - [x] `wsEvents.ts`: const + interface, mirroring the `InsolventEjectedPayload` entry
  - [x] One line added to `SystemRoomClosedInsolvent`'s comment (`:289-294`) about honor closes. **Wire string unchanged**
  - [x] Do NOT touch `events_contract_test.go`, `testdata/events/`, `wsEvents.schemas.ts` or `wsEvents.contract.test.ts`

- [x] **Task 10 — Client store rename (own commit, zero behaviour change)** (AC: 10)
  - [x] `roomStore.ts`: `InsolventEjection` -> `RoomEjection`, `insolventEjection` -> `roomEjection`, `setInsolventEjection` -> `setRoomEjection`, `reason: "ejected"` -> `"insolvent"`, `+ minHonor?/honor?`. Keep the `reset()` preservation (`:161`) and its comment
  - [x] Rename `InsolventEjectionModal.tsx` -> `RoomEjectionModal.tsx` and `useInsolventEjectRedirect.ts` -> `useRoomEjectRedirect.ts` (+ their tests)
  - [x] Update every consumer — this is the complete list, verified by grep: `shared/providers/WebSocketProvider.tsx:3,18` (where the redirect hook is mounted); `MatchPage.tsx:232,1235,1237,1270`; `RoomPage.tsx:161,401,405,408` (`:401` is a *comment* naming the hook — 9.7's review spent patches on stale comments, so fix it); `useWsDispatch.ts:705,723`; `LobbyPage.tsx:9,246`; `roomStore.test.ts:8-10,324-351`, `useWsDispatch.test.ts:1118-1166`, `InsolventEjectionModal.test.tsx`
  - [x] Rename the `lobby.insolventEjection.*` i18n block to `lobby.roomEjection.*` x4 in the same commit
  - [x] Verify green before starting Task 11: `npx tsc -p tsconfig.build.json --noEmit`, `npx vitest run`

- [x] **Task 11 — Client honor-ejection branch** (AC: 6, 8, 10)
  - [x] `useWsDispatch.ts`: handle `SYSTEM_HONOR_EJECTED` next to the insolvency handler (`:695-712`), validating all three numbers, setting `reason: "honor"`
  - [x] `RoomEjectionModal`: a third copy branch (icon: a shield rather than `Coins`/`DoorOpen`), interpolating `minHonor` and `honor`
  - [x] No new redirect logic — `useRoomEjectRedirect` already fires on any non-null notice

- [x] **Task 12 — Types + create-room modal + card** (AC: 2, 3, 5)
  - [x] `apiTypes.ts`: `Room` += `minHonor: number`, `allowNewPlayers: boolean`; `CreateRoomRequest` += the same two, with the "Go zero values are real 0s / false — compare explicitly" caveat
  - [x] `CreateRoomModal.tsx`: a "Minimum honor" `Input type=number min=0 max=100` (mirroring the `coin-buy-in` field's shape, `:315-350`) and an "Allow new players" `Segmented` (mirroring the privacy toggle, `:352-377`); reset both in `handleOpenChange` (`:158-174`); map the three new error codes in the `catch` (`:124-155`); disable submit on a cosmetic self-gate failure, mirroring `buyInExceedsBalance` (`:437`)
  - [x] The self-gate read uses `honorScoreOrPrior(user.honorScore)` + `honorIsNewPlayer(user.isNewPlayer)` from `shared/lib/honor.ts` — do not re-implement either
  - [x] `PreviewCard` (`:503-580`) += both props and chips
  - [x] `RoomCard.tsx`: conditional chips in the meta row after the private/public chip (`:107-125`). The row is already `flex-wrap`, so no layout rework — but check 320px width, since a one-line-overflow regression is what commit `ee556df` had to fix on the top bar
  - [x] Reuse existing tokens and the `Badge`/chip vocabulary. Do not add a new colour token

- [x] **Task 13 — Join-error handling at every entry point** (AC: 9)
  - [x] `LobbyPage.tsx` `joinRoomFlow` (`:163-195`)
  - [x] `JoinByCodeTile.tsx` `toastError` (`:38-48`) — generic variants
  - [x] `RoomPage.tsx` **both** paths: the public auto-join `catch` (`:266-287`) and `joinDeepLinkPrivate`'s `catch` (`:303-329`)
  - [x] A test per entry point asserting the right toast for each of the two codes

- [x] **Task 14 — i18n x4** (AC: 11)
  - [x] All keys from AC11 in `en/sr/mk/hr`; mk all-Cyrillic; no em dash in mk/sr/hr
  - [x] Copy follows the house voice — calm, non-punitive, no dead ends (cf. `lobby.insolventEjection.ejectedBody`). The honor-eject body is a statement of fact plus a way forward, not an accusation
  - [x] Check for a pre-existing `string` key where an `object` is now needed

- [x] **Task 15 — Tests**
  - [x] Go: new `internal/room/honor_handler_test.go` (sibling to `coin_handler_test.go` / `privacy_handler_test.go`), reusing `setupCoinTestBC`-style wiring, `mockRoomRepo`, `mockBroadcaster` and `PresenceRegistry`. Table-driven for the ten D1 rows
  - [x] Go (mock-level, `mockRoomRepo`): all ten truth-table rows; ungated room performs zero honor reads; honor-read error -> 500 not admit; create-time self-gate (both codes); `StartMatch` ejects before charging (assert `stubWallet.chargeCalls == 0`); owner + one other seat both failing -> transfer to the one eligible heir; no eligible heir -> close + `room_closed_insolvent`; return-path 409 + `system:honor_ejected` payload; quick-play synthesis ungated
  - [x] Go (**DB-backed**, `getRoomTestDB`): the `allow_new_players = false` create-and-read roundtrip and the omitted-field -> `true` case. These two cannot be mock-level (AC2), and they are the only tests that can catch the GORM default-tag trap
  - [x] Use the D1 truth table's exact fixture values — row 5 is `score = 19`, not a lower number (see the two arithmetic facts under the table); a fixture the system cannot produce is a test that proves nothing
  - [x] Vitest: card chips render only when gated (via `data-testid`); modal honor branch; the three (four call-path) join entry points; create-modal validation + self-gate disable; store rename regression
  - [x] Selection by `data-testid` only, never CSS classes; present-tense `it(...)`; assert computed numbers via `data-*` attributes, not text, so tests stay i18n-independent

- [x] **Task 16 — Gates**
  - [x] `cd server` (mise-shimmed go 1.26): `go vet ./...`, `gofmt -l .`, `go test ./...`, `golangci-lint run ./...` (v1.64.8, matching CI)
  - [x] `cd client`: `npx tsc -p tsconfig.build.json --noEmit`, `npx vitest run`, `npx eslint .`, `npx prettier --check .`
  - [x] `npx prettier --write` on changed files before committing — a single missing space blocked CI during 9.5
  - [x] `make test` + `make lint` green. Baseline to beat: vitest **101 files / 1053 tests**
  - [x] Known pre-existing noise, **do not fix here**: `gofmt -l` flags `server/internal/auth/profile_identity_handler_test.go`; `tsc` noise on `RoomDetail.returnedUserIds` mocks in `MatchmakingPage.test.tsx` / `RoomPage.bots.test.tsx`

- [x] **Task 17 — Docs**
  - [x] Update `deferred-work.md` with anything consciously deferred
  - [x] Epic 9 is complete after this story — note it in the sprint-status entry so the retrospective is on the radar

### Review Findings

3-layer adversarial code review (Blind Hunter + Edge Case Hunter + Acceptance Auditor, all three green first run, `git diff a3631ef..HEAD`). 29 raw findings → 22 unique after dedup: 1 decision, 15 patches, 6 deferred, 0 dismissed. All three layers independently converged on the same top defect. The Acceptance Auditor's conformance table returned AC6 PARTIAL and every other AC1-AC12 plus all of D1-D8 SATISFIED; it independently re-verified AC8's zero-drift-gate claim (both halves), the ten truth-table rows, the DB-backed GORM-trap tests running rather than skipping, and that 9.3's insolvency tests are unedited.

- [x] [Review][Decision→Patch] **A New Player owner passes their own create-time gate against any `min_honor`, then is ejected from their own room on graduation** — RESOLVED by PO decision 2026-07-30: **cap a New Player creator's `min_honor` at the newcomer prior (80).** A New Player may still gate their room, but only at or below the score they themselves currently hold, so graduation can never drop them under their own bar. Chosen over (b) a modal-only warning, which leaves the ejection live for anyone who ignores it or uses a non-browser client; (c) exempting the owner from their own room's gate, which fixes more cases but lets a gated room be hosted by someone who fails it; and (d) accept-as-is. The fix is create-time only and does NOT touch D1's join semantics — a New Player joining someone else's room is still never score-checked. Applies server-side in the `CreateRoom` self-gate plus the cosmetic `CreateRoomModal` mirror, with an i18n hint ×4. `TestCreateRoom_NewPlayerCreatorMaySetHighThreshold` pins the old behaviour and must be rewritten. Original finding: D7 exists to stop an owner barring themselves; D1 mandates that a New Player is never score-checked. For a New Player creator the two collide and D1 wins, so the self-gate is a no-op for exactly the accounts whose score is about to move. Worked case: an account at 0 completed / 4 abandoned (New Player, score 19) creates a `min_honor = 80` room; **one** completed match makes them experienced at `100x5/22 = 23`, and the next return/start ejects them from their own room, transferring ownership or closing it. A brand-new 0/0 account creating `min_honor = 95` fails the same way after five clean matches (score 90). The client mirror has the identical hole (`failsOwnHonorGate = !meIsNewPlayer && meHonor < effectiveMinHonor`), so the owner is never warned. The code is spec-conformant and `TestCreateRoom_NewPlayerCreatorMaySetHighThreshold` pins the current behaviour as intended — this is a gap in the specified interaction of D1 and D7, not a coding slip, which is why it needs a PO call rather than a patch. [server/internal/room/handler.go:554-565, honorGateError:109-120, CreateRoomModal.tsx:127-129]

- [x] [Review][Patch] StartMatch leaves the room bricked in `"playing"` when the honor read fails — the one post-commit exit that skips the revert its two siblings perform [server/internal/room/handler.go:3340]
- [x] [Review][Patch] `MatchPage`'s return-time catch has no honor branch, so the checkpoint D3 calls "the one that fires in practice" leaves the player on a dead result overlay with a misleading toast and uncleaned match state [client/src/features/match/MatchPage.tsx:1227]
- [x] [Review][Patch] `en.json` `roomClosedBody` states a coin reason for an honor close, contradicting the reuse justification in `ws/events.go` and `wsEvents.ts`; sr/mk/hr are genuinely reason-agnostic, English is the only locale that misinforms [client/src/shared/i18n/en.json:470]
- [x] [Review][Patch] `RoomPage.handleStartGame` has no honor branch, so the owner gets "try again" — advice that fails identically on retry — where insolvency has a purpose-built non-disclosing key; no `honorTooLowStart` sibling exists [client/src/features/room/RoomPage.tsx:736]
- [x] [Review][Patch] `ejectReturner`'s `ownerEligible` keeps the nil-balances shape that `startEligible` was deliberately fixed for, so it can still judge every heir ineligible and close a room with valid heirs [server/internal/room/handler.go:1390]
- [x] [Review][Patch] `ReturnToRoom` performs the heir-candidate honor read (and can 500 on it) even when the returner is not the owner, where `ejectReturner` never consults the predicate it feeds [server/internal/room/handler.go:1918]
- [x] [Review][Patch] Stale comment names `ejectInsolventAtStart`, renamed to `ejectSeatsAtStart` in this story — the exact class AC10 forbids and Task 10 called out on the client side [server/internal/room/handler.go:2222]
- [x] [Review][Patch] `RoomCreatedPayload` / `RoomUpdatedPayload` were not extended with `minHonor` / `allowNewPlayers` even though `roomLifecyclePayload` now sends them, so the declared wire contract under-describes the real one and diverges from 9.6's `isPrivate` precedent in the same interfaces [client/src/shared/types/wsEvents.ts:311-349]
- [x] [Review][Patch] `CreateRoomModal` hard-disables "Veterans only" whenever `isNewPlayer` is absent from the auth envelope — `honorIsNewPlayer(undefined)` is `true` by design, which is the right default for display suppression but converts "unknown" into "denied" for a capability gate the server would have allowed [client/src/features/room/CreateRoomModal.tsx:81,127]
- [x] [Review][Patch] `honorSnapshotFor(0)` returns a zero-value snapshot that reads as an experienced player with score 0 and clears a veterans-only gate — verbatim the hazard the function's own doc comment guards against for missing rows [server/internal/room/handler.go:1068]
- [x] [Review][Patch] `TestStartMatch_HonorReadErrorDoesNotEject` asserts 500, no ejection and no push but never room status, so it already reproduces the bricked-room defect and certifies around it [server/internal/room/honor_handler_test.go:725-740]
- [x] [Review][Patch] Join-error test matrix is 3/4 — the private deep-link path has no `NEW_PLAYER_NOT_ALLOWED` case, against Task 13's stated per-code-per-entry-point matrix [client/src/features/room/RoomPage.test.tsx:516]
- [x] [Review][Patch] `useRoomEjectRedirect`'s doc comment still enumerates only the two insolvency events as its signal sources [client/src/shared/hooks/useRoomEjectRedirect.ts]
- [x] [Review][Patch] `JoinByCodeTile`'s comment justifies the generic variant on a false premise — `HONOR_TOO_LOW` can only originate inside `joinResolvedRoom(room, ...)`, which does hold the room; a lookup failure yields `ROOM_NOT_FOUND`. The generic copy is what AC9 asked for, so only the stated reason is wrong [client/src/features/lobby/components/JoinByCodeTile.tsx:46-50]
- [x] [Review][Patch] `honorOf` / `newPlayerHonorOf` carry decorative totals that contradict the score they are handed (`newPlayerHonorOf(80)` holds 0C/4A, which computes to 19). Every `(isNewPlayer, score)` pair used is reachable and no assertion depends on the totals, but the inconsistency cost a reviewer time to clear [server/internal/room/honor_handler_test.go]

- [x] [Review][Defer] `ReturnToRoom`'s gates run before the room-status check, so a stale `/return` on a `playing` room could free a seat mid-match [server/internal/room/handler.go:1905] — deferred, pre-existing: the insolvency gate directly above it has the identical shape and predates this story; both reviewers labelled the stale-client trigger unproven
- [x] [Review][Defer] On a rolled-back eject transaction the per-user ejection push still fans out, so players who were NOT actually ejected redirect to the lobby while still seated server-side [server/internal/room/handler.go:1600-1608] — deferred, pre-existing: unchanged 9.3 code (`player_left` is correctly guarded on `txErr == nil` at :1627, the per-user push deliberately is not); honor only makes the path more travelled
- [x] [Review][Defer] Two other hand-built room payloads omit the honor keys [server/internal/room/handler.go:3615-3628, server/internal/room/lobby_disconnect.go:222-236] — deferred, pre-existing: both maps already drop `coinBuyIn` / `isPrivate` / `players` / `ownerUsername`, so the hole is broader and older than 9.8; quick-play rooms are genuinely ungated so `undefined` renders correctly, and the disconnect payload's consumer merges rather than replaces
- [x] [Review][Defer] `ejectNotice` carries only `kind: honor`, never which of the two gates failed, so a `NEW_PLAYER_NOT_ALLOWED` ejection would render "your honor dropped below the minimum of 0 — it's now 80" [server/internal/room/handler.go:1146,1935 -> RoomEjectionModal.tsx:130-139] — deferred: confirmed **unreachable** on this branch (`IsNewPlayer` only goes true->false, the join gate blocks New Players at entry, D7 blocks a New Player owner from setting veterans-only, and there is no owner-edit endpoint). Widen the seam together with the deferred owner-edit endpoint, which is what makes it live
- [x] [Review][Defer] The insolvency return path passes `nil` `extraEligible`, so an insolvent owner's seat can transfer to an heir who does not pass the room's honor gate [server/internal/room/handler.go:1890] — deferred: not an AC7 violation (its Given is scoped to an honor-ejected owner) and consistent with the lazy-check design already deferred as Open Question 4; the heir is ejected at the next return/start
- [x] [Review][Defer] A stranger who joins and seats between the hoisted candidate read and the eject transaction is treated as an ineligible heir, closing a room that had one [server/internal/room/handler.go:1856 vs :1221] — deferred, pre-existing class: structurally identical to the hoisted-`balances` race directly above it, fails conservatively (closes rather than mis-assigns), and the Edge Case Hunter could not construct a realistic trigger

#### Review resolution (2026-07-30)

All 16 patches applied. Notes on the four things that did not go exactly as the finding list predicted:

1. **The New Player cap rule was corrected during implementation.** The decision was recorded as "cap at the newcomer prior (80)", but the rationale attached to it was "so graduation can never drop them under their own bar" — and those are different rules. Capping at a fixed 80 does NOT close the reproduction: the 0-completed/4-abandoned account holds 19, could still set 80, and one finished match puts them at 23. The implemented rule is therefore **the creator's own current score**, which yields exactly "<= 80" for a 0/0 newcomer (their score IS 80) and correctly yields "<= 19" for the 0/4 case. Server (`handler.go` CreateRoom self-gate) and the cosmetic modal mirror both apply it; D1 at the join gate is untouched, pinned by a new `TestJoinRoom_NewPlayerStillNotScoreCheckedAtJoin`.
2. **Two existing tests pinned the behaviour the decision reversed** and were rewritten rather than deleted: Go `TestCreateRoom_NewPlayerCreatorMaySetHighThreshold` -> `TestCreateRoom_NewPlayerCreatorCannotSetBarAboveOwnScore` (now table-driven, five rows, including the two the fixed-80 reading would have got wrong), and Vitest "lets a new-player owner set a high threshold as long as newcomers are welcome" -> a disabled-submit assertion plus a new at-or-below-own-score case. The Vitest one failed on the first full run, which is how it was caught.
3. **The bricked-room fix is proven, not asserted.** The new `assert.Equal(t, "waiting", persisted.Status)` was verified to FAIL with the revert disabled (`actual: "playing"`) and pass with it — the same discipline 9.7's review used for the decay clamp.
4. **`honorKnown` was added to `CreateRoomModal`** so the cosmetic self-gate is skipped entirely when the auth envelope carries no honor. This fixes the reported "unknown becomes denied" defect and also covers the score half of the same problem, since `honorScoreOrPrior`'s 80 default had the identical flaw in a gate context. New test: "does not gate the owner when the auth envelope carries no honor".

One finding was deliberately implemented only in part: `ejectReturner`'s nil-`balances` shape was aligned to `startEligible`'s, but its `GetBalances` **error** path still returns 500 rather than degrading the way `honorPrefilterAtStart` does. That error behaviour is unchanged 9.3 code shared with the insolvency caller, so changing it would alter 9.3 semantics — out of scope for a review patch, and recorded here rather than silently widened.

#### Manual E2E (Playwright, 2026-07-30, post-patch)

Ran against the real app — Go server rebuilt from source on 8080, Vite on 5173, dev DB 5433 at migration 18. Five accounts with SQL-set honor weights (97 / 96 / 30 / 80-newcomer / 80-newcomer). **No new defect found**; all four flows behaved as specified, and two things the review could only reason about were confirmed in the wild:

- **AC2's GORM trap, in production code paths.** A room created through the UI with the veterans-only toggle persisted `allow_new_players = false` — the value the `default:true` tag would have silently flipped to `true`.
- **D5, incidentally and decisively.** User 1876's `honor_score` snapshot column still read `80` while the TopBar and the gate both used the recomputed `29`. The lagging column is genuinely not consulted.
- **D1 both halves.** A room with `min_honor = 0` + veterans-only was created and rejected a New Player with `NEW_PLAYER_NOT_ALLOWED` (the epic's nesting would have admitted them); and a New Player on the 80 prior was ADMITTED to a `min_honor = 85` room, score never consulted.
- **Both patched user-facing messages rendered.** Owner start-failure: "A player doesn't meet this room's honor requirement — the match didn't start." (patch #8's new key, replacing the futile "try again"). New Player create cap: 409 at `min_honor 95`, 201 at exactly 80 (patch #2 — this returned 201 before the fix).
- **AC6 end-to-end at the start path:** newcomer ejected, seat freed, room reverted to `waiting` (not stranded in `playing`), zero matches created, owner got the 409. **At the return path:** 409 + seat freed. The ejection modal rendered from a genuine `system:honor_ejected` push with shield icon and interpolated 90/29 via `data-min-honor`/`data-honor`.
- **All three join entry points:** lobby card composed "asks for honor 85 … Yours is 30"; join-by-code used the param-less generic; the deep link bounced to `/lobby` on a 409.
- **Task 12's 320px concern, previously UNVERIFIED, is closed.** A maximally-chipped card (timer + buy-in + `95+` + Veterans only) at 320px wraps across three lines with `documentElement.scrollWidth` 310 < 320 and **zero** overflowing elements.
- Zero server-side `ERROR` log lines and zero JS exceptions across the whole run.

Two things this pass did NOT cover, stated rather than implied: **patch #7's `MatchPage` return-catch branch** specifically (the WS-push route to the modal is verified, but reaching the match-result overlay needs a real four-player match played to completion/abandonment — the branch is unit-tested), and **patch #1's revert in the live app** (it needs a DB failure injected mid-start; it is instead proven by a unit test verified to fail without the fix, `actual: "playing"`). Incidental non-9.8 observation: Echo's logging middleware records apperr responses as `status 200` (a 401 probe and the 409 gate rejections all logged 200 while the client received the real code) — pre-existing, affects every apperr, not introduced here.

**Gates re-run after the patches** (not inherited from the dev record): `gofmt -l` clean except the documented pre-existing `internal/auth/profile_identity_handler_test.go`; `go vet ./...` clean; `golangci-lint run ./...` clean (v1.64.8); `go test ./...` all 18 packages ok **with the DB-backed honor tests PASSING not skipping** (dev DB 5433, `BELJOT_DB_URL` from `.env.example`) — the four of them verified individually; `tsc -p tsconfig.build.json --noEmit` clean; `vitest run` **101 files / 1099 tests** (up from 1096: +3 net across the rewritten and added cases); `eslint .` clean (one `simple-import-sort` error autofixed in `MatchPage.tsx`); `prettier --check .` clean. i18n verified by script: **861 keys x 4, zero missing/extra, zero placeholder-set mismatches, zero em dashes in sr/mk/hr, mk all-Cyrillic** (the 20 residual Latin hits in mk are all pre-existing language names, unit suffixes and placeholder artifacts — none from this review).

## Dev Notes

### Where the code goes

| Concern | Path | New/Update |
| --- | --- | --- |
| Migration | `server/migrations/000018_add_honor_gate_to_rooms.{up,down}.sql` | NEW |
| Room columns | `server/internal/room/model.go` | UPDATE |
| Errors | `server/internal/apperr/errors.go` | UPDATE |
| Gate fn, interface, 4 call sites, 2 eject helpers | `server/internal/room/handler.go` | UPDATE |
| Honor batch read | `server/internal/user/honor_service.go` | UPDATE |
| DI wiring | `server/cmd/api/main.go` (`:247`) | UPDATE |
| WS event | `server/internal/ws/events.go` | UPDATE |
| Gate tests | `server/internal/room/honor_handler_test.go` | NEW |
| Handler ctor call sites | `server/internal/room/{coin_handler_test,handler_test}.go` | UPDATE |
| WS types | `client/src/shared/types/wsEvents.ts` | UPDATE |
| Store (renamed) | `client/src/shared/stores/roomStore.ts` | UPDATE |
| Modal (renamed) | `client/src/features/lobby/components/RoomEjectionModal.tsx` | RENAME + UPDATE |
| Redirect hook (renamed) | `client/src/shared/hooks/useRoomEjectRedirect.ts` | RENAME |
| Dispatch | `client/src/shared/hooks/useWsDispatch.ts` | UPDATE |
| Create modal + preview | `client/src/features/room/CreateRoomModal.tsx` | UPDATE |
| Room card | `client/src/features/lobby/components/RoomCard.tsx` | UPDATE |
| Join entry points | `LobbyPage.tsx`, `JoinByCodeTile.tsx`, `RoomPage.tsx` | UPDATE |
| Types | `client/src/shared/types/apiTypes.ts` | UPDATE |
| i18n x4 | `client/src/shared/i18n/{en,sr,mk,hr}.json` | UPDATE |

### Current state of the files you are modifying

**`server/internal/room/handler.go` — `JoinRoom` (`:662`).** Check order today: parse id -> tolerant `c.Bind` (an empty body must never 400, `:673-676`) -> `FindByID` -> `Status != "waiting"` -> **password** (`:696-700`) -> `PlayerCount >= 4` -> bots+humans capacity (`:702-716`) -> `FindPlayerRoom` already-in-room -> **coin affordability** (`:731-739`) -> tx (AddPlayer + IncrementPlayerCount + re-fetch) -> presence -> `player_joined` -> `room_updated`. **Preserve all of it.** Your insertion is one block after `:739`.

**`StartMatch` (`:2709`).** Two phases. Phase 1 is a row-locked tx that validates owner/status/seating and flips the room to `playing`. Phase 2 (post-commit) builds `seatInfo`, then — only when `CoinBuyIn > 0 && walletService != nil` — runs `GetBalances` prefilter -> `ejectInsolventAtStart` on any insolvent seat -> the atomic `ChargeStakes` (which can still surface ONE more insolvent user via TOCTOU) -> `matchStarter.StartMatch`, with refund-on-failure. Insert the honor prefilter at `:2850`, before that whole block. Note the room is **already** `playing` at this point, which is why the eject helper owns the revert-to-`waiting`.

**`ReturnToRoom` (`:1409`).** Membership check -> insolvency gate (`:1450-1458`) -> row-locked reopen tx (status `completed` -> `waiting`, clears bots) -> post-commit broadcasts (only when this call performed the reopen) -> presence add + `player_returned` -> response with `ReturnedUserIds`. Your insertion is immediately after `:1458`.

**`transferOwnershipOrClose` (`:887`).** Runs **inside** the caller's tx and performs **no broadcasts** — it returns values the caller fans out post-commit. Eligibility is the caller's policy, passed as a closure. That is exactly the seam AC7 needs; do not move policy into the helper.

**`ejectInsolventAtStart` (`:1085`) is the more delicate of the two helpers.** It tolerates a concurrent leave (`ErrNotInRoom` -> continue, `:1116-1121`), un-bricks the room out-of-tx if its own tx rolls back (`:1153-1164`), and gates the `player_left` fan-out on `txErr == nil && !roomClosed` (`:1193`). Every one of those behaviours must survive the generalization untouched.

**`server/internal/user/honor.go`.** Pure, no clock reads, `now` always a parameter. `NewHonorSnapshot(completedWeight, abandonedWeight, decayedAt, completedTotal, abandonedTotal, now)` is the single read-path entry point — use it, don't call `HonorScore` + `IsNewPlayer` separately. `honorNewPlayerMinMatches` is unexported deliberately.

**`server/internal/user/model.go:64-73`.** `HonorScoreSnapshot` is named `...Snapshot` precisely so that gating on it reads wrong at the call site. Its comment says NEVER gate on it. That comment was written for this story.

**`client/src/shared/lib/honor.ts`.** Already exports every guard you need: `honorScoreOrPrior` (never `|| 80`), `honorIsNewPlayer` (defaults to suppressed), `honorCountOrZero`, `normalizeHonorTier`. All exist because 9.7's review found the unguarded versions rendering a confident "80 / Fair" for accounts the server had said nothing about. Reuse them; add nothing.

**`client/src/features/lobby/components/RoomCard.tsx:88-128`.** The meta row already carries variant/mode, timer, buy-in, private/public and relative age, separated by `<Dot />`. It wraps, so adding conditional chips is safe — but the chips must be conditional, or every ungated card grows two more chips for no information.

### Previous-story intelligence

**9.6 (private rooms) is the structural blueprint.** Same shape: an additive `rooms` column, an HTTP-only join gate returning a 409 `apperr`, a derived/visible flag on the card, and a prompt-or-reject client flow. Copy it and you will be close.

What its work and its review established, that applies directly:

1. **A UI-only guard is not a guard.** Every check must be enforced server-side; the modal's disabled button is cosmetic.
2. **Check *every* entry point.** 9.6's deep-link path needed the same fix as the lobby path and was caught only in manual E2E — then a *second* bug (a stale React Query cache re-opening the password prompt) was found in E2E after review had passed. RoomPage has **two** join call paths; both need the new error codes.
3. **HTTP, not WS.** The epic phrases join gates as `action:join_room` / `error:*`; the actual mechanism is an HTTP `apperr` surfaced through `FetchError.code`. 9.6 confirmed this and explicitly changed nothing in the WS drift gate.
4. **Server/client validation units must agree.** 9.6 shipped a min in runes and a max in bytes and needed a review patch to align them. Your bound is a plain integer range, so keep it symmetric and boring: `[0,100]` on both sides.

From **9.7**: all six drift-gate touchpoints in one commit *when the event is `event:*`* — and this story's is `system:*`, so the correct number is **zero**. Do not cargo-cult the checklist.

From **9.5** (via 9.7's review): a lifetime accumulator's column width must match the Go type; a `match_abandoned` handler that forgets what `match_end` resets is a real bug class; and Prettier alone has blocked CI.

From **9.3**: the ejection flow you are extending was hardened by a review that found a charge-time TOCTOU wrongly **closing a room that had solvent heirs** — because a one-entry balances map made every candidate look ineligible. When you widen the eligibility closures, make sure the honor snapshot map covers **every seated human**, not just the failing one. That is the same bug waiting to happen again.

### Testing requirements

- Go: standard `testing` + `testify`, co-located. The pure gate function gets a **table-driven** test over all ten D1 rows. Handler tests use the existing `mockRoomRepo` / `mockBroadcaster` / `PresenceRegistry` harness — do not reinvent it, and do not reach for a real DB (the gate has no SQL of its own).
- `stubHonor` mirrors `stubWallet` (`coin_handler_test.go:19-76`), including a call counter so "an ungated room performs no honor read" is provable rather than asserted by inspection.
- The 9.3 insolvency tests are the regression net for Task 5. They must pass **unedited**.
- Vitest: co-located, `data-testid` selection only, present-tense `it(...)`, computed values asserted through `data-*` attributes so tests never depend on i18n wording.
- Assert the *absence* of gate output too: an ungated room's card must not render either chip, and a nil honor service must not reject.

### Project Structure Notes

Everything lands in existing folders. No new packages, no new feature directories, no new API endpoints (`min_honor` / `allow_new_players` ride `POST /rooms` and every existing room response). The one structural addition is `room -> user`, which is new but acyclic and justified in D6; if a future story needs to break it, the seam is the single `HonorService` interface.

The client rename (AC10) is the only non-additive change. It is confined to one store field, one modal and one hook, and it is what keeps the codebase honest about what the notice now means.

NFR8 applies: the gate decision is server-side. The client copy of the honor bands stays display-only, exactly as 9.7 D6 requires.

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Story 9.8: Honor-Gated Rooms] — base ACs (the `min_honor > 0` scoping, the "< 5 completed" floor, and `event:honor_eject` overridden by D1/D2/D4)
- [Source: _bmad-output/planning-artifacts/epics.md#Story 9.6: Private Rooms] — the structural blueprint for an additive room column + HTTP join gate
- [Source: _bmad-output/planning-artifacts/epics.md#Story 9.3: Insolvency Ejection & Room Persistence] — the ejection flow this story extends, and its two checkpoints
- [Source: _bmad-output/planning-artifacts/epics.md#Additional Requirements] — FR57 (min_honor + allow_new_players), NFR8 (server-authoritative)
- [Source: _bmad-output/planning-artifacts/epics.md#Story 8.5-1] — predicts a `"honor_eject"` `match_abandoned` reason; D4 explains why that does not apply
- [Source: _bmad-output/planning-artifacts/prd.md#Phase 2] — **stale** on the New Player floor (says 20); ignore per D2
- [Source: _bmad-output/planning-artifacts/sprint-change-proposal-2026-06-18.md#Section 4] — New Player floor 20 -> 5; honor weights are placeholders
- [Source: _bmad-output/planning-artifacts/ux-design-specification.md#Accessibility Considerations] — no information conveyed exclusively through colour
- [Source: _bmad-output/implementation-artifacts/9-7-honor-score-system.md] — the honor system this story gates on; D2's floor amendment; the `system:`-vs-`event:` drift-gate scope note
- [Source: _bmad-output/implementation-artifacts/9-6-private-rooms.md] — HTTP-only gate precedent; enforce server-side; check every entry point; the E2E-found deep-link bugs
- [Source: _bmad-output/implementation-artifacts/9-3-insolvency-ejection-and-room-persistence.md] — `transferOwnershipOrClose`, the two eject helpers, and the balances-map TOCTOU finding
- [Source: _bmad-output/implementation-artifacts/spec-abandonment-per-player-results.md] — frozen: no new abandonment triggers
- [Source: _bmad-output/implementation-artifacts/deferred-work.md] — 9.3's non-transactional room-mutation TOCTOU, the refund-durability item, D82/D87/D142
- [Source: _bmad-output/project-context.md] — migration numbering, i18n parity, `data-testid`, atomic mutation, branch/commit conventions
- GORM zero-value-with-`default`-tag behaviour (AC2's trap), confirmed against the current GORM docs: *"any zero value fields like 0, '', false won't be saved into the database for those fields defined default value … you might want to use a pointer type or Scanner/Valuer to avoid this"* — [gorm.io Create / Default Values](https://gorm.io/docs/create.html#Default-Values)

## Open questions for the PO (non-blocking)

1. **No owner edit surface.** 9.6 shipped `POST /rooms/:id/privacy` so an owner could change the password mid-room; 9.8's ACs ask only for create-time config, so `min_honor` / `allow_new_players` are fixed once the room exists. Confirm that is acceptable, or schedule the edit endpoint (it would reuse the mid-session re-check for the "tightened the gate on seated players" case, which is a decision of its own: eject them, or grandfather them in?).
2. **Both gates are independent (D1).** An owner can set `min_honor = 0` with `allow_new_players = false` — "anyone experienced, any score". Confirm that reads as intended.
3. **A New Player bypasses the score check entirely (D1, row 4).** With the default `allow_new_players = true`, a `min_honor = 95` room still admits newcomers. That is what makes the toggle meaningful, but it does mean the default configuration of a gated room is softer than the number on the card suggests. Should the card say something like "80+ · newcomers welcome" so the two settings read together?
4. **Ejection happens at return/start, not the instant honor drops.** A player whose honor falls below the threshold stays a room member until they try to come back or the owner starts. Pushing them out at match end would need honor to reach back into room membership from the match finalizers, which crosses a package boundary this codebase deliberately keeps one-way. Confirm the lazy check is fine.
5. **Nothing shows an owner *why* a room won't start.** On an honor ejection at start the owner gets a 409 and sees a seat disappear, with no statement that it was an honor gate — identical to today's insolvency behaviour. Worth a follow-up owner-facing notice?
6. **The usable range of `min_honor` is much narrower than 0-100.** Because of the Beta(4,1) prior, a fresh account sits at 80, a flawless 20-match record caps at 96, and 100 needs ~195 completions inside one 90-day half-life. So `min_honor` above ~95 is effectively "nobody", and anything at or below 80 admits every newcomer by default. The field accepts the full 0-100 per AC1; the question is whether the create-modal hint should steer owners toward the meaningful band (say 85-95) and whether values above 95 should warn. Retuning `honorPriorCompleted` / `honorPriorAbandoned` would move this, and 9.7's Open Questions 1-3 already flag those constants as unvalidated placeholders.

## Dev Agent Record

### Agent Model Used

claude-opus-5 (Claude Opus 5, 1M context) via Claude Code / bmad-dev-story

### Debug Log References

**Task 0 — branch (fallback taken deliberately).** Developed ON TOP of `feat/9-7-honor-score-system`, not from `master`. Per user direction 2026-07-30: 9.7 and 9.8 ship as ONE full-scope feature/PR, so 9.7 is deliberately NOT merged to master first. This is the story's own documented fallback. `000017` confirmed as the highest migration before naming `000018`.

**Task 1 — migration roundtrip on dev DB 5433 (`beljot-postgres-1`).** `migrate version` → 17. `migrate up` → `18/u add_honor_gate_to_rooms`. Column inspection: `min_honor smallint NOT NULL DEFAULT 0`, `allow_new_players boolean NOT NULL DEFAULT true`; `pg_constraint` shows `rooms_min_honor_check CHECK ((min_honor >= 0) AND (min_honor <= 100))`. `migrate down 1` → version 17, both columns gone (`information_schema` count 0) and the CHECK gone (count 0). `migrate up` → version 18, identical column state and CHECK restored. Roundtrip clean.

**AC2's GORM trap — proven, not asserted.** Temporarily added `gorm:"not null;default:true"` to `Room.AllowNewPlayers` and re-ran the DB-backed tests: `TestRoom_AllowNewPlayersFalseSurvivesRoundtrip` and `TestCreateRoom_VeteransOnlyPersistsToDB` both FAILED ("Should be false" on both the struct read and the raw `SELECT allow_new_players`), confirming GORM omitted the zero-valued field and the DB applied `true`. Tag removed; both tests pass. This is exactly why AC2 mandates a DB-backed test — a `mockRoomRepo` test stores whatever Go value it is handed and cannot observe the SQL generation.

**Four things the story did not predict.**

1. `ejectInsolventAtStart` had **FOUR** call sites, not the two Task 5 lists. `startAutoStartedMatch` (the QuickPlay auto-start path) calls it twice as a defensive insolvency net (`handler.go` ~1854/1870 pre-change). All four were migrated to the generalized `ejectSeatsAtStart`; the two quick-play sites pass `nil` for `extraEligible` because a synthesized room is ungated by construction (AC12).
2. Heir-candidate honor reads must be **lenient** where the gate subject is **strict**. My first cut used one strict batch reader for both, and `TestReturnToRoom_HonorEjectsReturner` returned 500 instead of 409: the ejected owner's peer had no stub row, and "missing row = error" aborted the whole ejection, leaving a barred player seated. Split into `honorSnapshotsFor` (strict — a missing row for the gate SUBJECT must fail the request, because a zero-value snapshot reads as an experienced player with score 0 and would sail through a `min_honor 0` / veterans-only room) and `honorCandidateSnapshots` (lenient — a missing CANDIDATE row means "ineligible"). Pinned by `TestReturnToRoom_MissingCandidateHonorTreatsHeirAsIneligible`.
3. `startEligible` now treats a **nil** `balances` map as "solvency not evaluated" rather than "everyone is broke". The honor-eject path passes nil on a free room or when no wallet is wired; the original `balances[candidateID] < buyIn` shape would have made every candidate look insolvent and closed a room that had valid heirs — 9.3's review finding in a new costume. Every insolvency caller still passes a freshly-read non-nil map, so their behaviour is byte-identical.
4. The pre-9.8 mock seed helpers (`seedFinishedRoom`, `seedMixedRoom`, …) build `&room.Room{}` without `AllowNewPlayers`, so every mock-seeded room reads as **veterans-only** (Go's zero `false`) and therefore gated. This is the inverse of the GORM trap and is harmless in production — both real `&Room{}` sites set the field explicitly and the DB `DEFAULT TRUE` covers raw inserts — but it made two "ungated room performs no honor read" fixtures wrong until spelled out. The shared helpers were left untouched so the 9.3 tests stay unedited; the honor tests set the field on the returned pointer, as those tests already do for `CoinBuyIn`.

**Task 5 regression net held.** The 9.3 insolvency tests pass **unedited** after the generalization, which is the story's own stated criterion for the refactor being behaviour-preserving. Only the two shared setup helpers changed, and only to pass `nil` for the new constructor parameter.

**AC8 independently verified.** `git diff --name-only` over `events_contract_test.go`, `testdata/events/`, `wsEvents.schemas.ts` and `wsEvents.contract.test.ts` returns empty, and none of those files contains `honor_ejected`/`HonorEjected`. Exactly two contract files touched: `ws/events.go` + `wsEvents.ts`. Zero drift-gate touchpoints, as D4 requires (contrast 9.7, where the correct number was six).

**Two Vitest failures diagnosed rather than patched around.** (a) `submits the configured honor gate` failed because the default test owner sits at honor 80 and the test asked for `minHonor: 90` — the D7 self-gate correctly disabled submit. Fixture raised to 96. (b) `surfaces the server's INVALID_MIN_HONOR rejection` then failed as a **cascade**: `vi.clearAllMocks()` does not drain `mockResolvedValueOnce` queues, so the value (a) never consumed was picked up by the next test's submit, turning a rejection path into a success path. Fixing (a) fixed both.

**Gates.** `gofmt -l` clean except the pre-existing `internal/auth/profile_identity_handler_test.go` (documented known noise). `go vet ./...` clean. `golangci-lint run ./...` clean (v1.64.8, matching CI). `go test ./...` — all 18 packages ok, with the DB-backed honor tests **passing, not skipping** (dev DB 5433). `tsc -p tsconfig.build.json --noEmit` clean. `vitest run` — 101 files / **1096** tests. `eslint .` clean (3 import-order errors autofixed). `prettier --check .` clean. `make lint` + `make test` green.

**Baseline correction.** Task 16 names "vitest 101 files / 1053 tests" as the baseline to beat. The actual count on this branch before any 9.8 test was written is **1058** — verified by running the full suite immediately after the behaviour-neutral rename commit, which added no tests. The story's figure is stale by 5. Final count 1096 (+38 tests).

**i18n verified by script, not by eye.** 18 new keys × 4 locales, asserted 1:1 present and non-empty, with matching `{{interpolation}}` placeholder sets against `en` (a mismatch renders a literal `{{minHonor}}`), no em dash anywhere in sr/mk/hr, and every mk letter outside a placeholder inside the Cyrillic block. Run from a script file rather than inline because the Bash console here is cp1251 and printing Cyrillic can half-apply a multi-file edit loop.

### Completion Notes List

- **The gate is one pure function.** `honorGateError(room, snapshot)` plus a `Room.honorGated()` short-circuit, shared verbatim by all four call sites, so the D1 semantics cannot drift between them. All ten truth-table rows are pinned by a table-driven test exercised through the real `POST /rooms/:id/join` endpoint rather than against the helper in isolation.
- **D1 honored, including the counter-intuitive half.** The two gates are independent, `isNewPlayer` is evaluated first, and a New Player is **never** score-checked — so a newcomer enters a `min_honor 95` room when `allow_new_players` is true (row 4), and a New Player in a gated veterans-only room gets `NEW_PLAYER_NOT_ALLOWED` and never `HONOR_TOO_LOW` (row 6). Fixtures use only reachable states: row 5 is score **19**, the worst a newcomer can hold (0 completed / 4 abandoned → 400/21).
- **D2 honored.** The gate reads `IsNewPlayer(completedTotal, abandonedTotal)` through `user.NewHonorSnapshot`. `TestHonorForUsers_NewPlayerFloorCountsExperienceNotSuccesses` pins the account this story exists to keep out: 0 completed / 20 abandoned is **not** a New Player and its real score of 5 reaches the gate.
- **D5 honored and provable.** `stubHonor` carries a call counter, so "an ungated room performs no honor read at all" is asserted (0 calls) rather than eyeballed — and the same counter proves the batched start-path read is exactly **one** call for the whole table. `HonorForUsers` never touches `users.honor_score`; `TestHonorService_HonorForUsers_RecomputesRatherThanReadingTheSnapshotColumn` feeds a deliberately-wrong snapshot column (5) and asserts the recomputed 96 wins.
- **A failed honor read never opens a closed door.** A read error fails the request with a wrapped 500 at every gate; on the start path it ejects nobody and starts nothing. A missing row for the gate subject is likewise an error, because the zero-value snapshot would read as an experienced player with a score of 0.
- **StartMatch gates before it charges.** The honor prefilter sits before the entire coin block, asserted by `stubWallet.chargeCalls == 0` in `TestStartMatch_HonorGateRunsBeforeCharging`. No stake is ever charged and refunded for a player who was about to be ejected.
- **AC7's multi-seat case is real, not hypothetical.** Because an abandonment charges every absent human seat (9.7 AC3, review pass 2), one match end can push several seats under at once. `TestStartMatch_HonorEjectsOwnerAndPeerTransfersToLoneHeir` covers owner + two peers failing with a single eligible heir surviving, and asserts each ejected seat gets its own per-user push; a sibling test covers all-four-fail → room closes with the reused `system:room_closed_insolvent`.
- **D8 honored.** The two eject helpers were generalized and renamed (`ejectReturner`, `ejectSeatsAtStart`), parameterized by `ejectNotice` + the apperr to return + an optional `extraEligible`. The renames are deliberate: a function named `ejectInsolventReturner` that ejects for honor is exactly the name-stopped-telling-the-truth failure AC10 forbids on the client. Every hardened behaviour survives untouched — the concurrent-leave tolerance, the out-of-tx un-bricking `UpdateStatus`, and the `txErr == nil && !roomClosed` guard on the `player_left` fan-out. Tests do not call these unexported helpers, so no 9.3 test needed editing.
- **AC10's rename shipped as its own commit** (`573182f`), behaviour-neutral, verified green (tsc + 101 files/1058 tests) before a single line of honor branching was added.
- **Every join entry point covered, all four call paths.** `LobbyPage.joinRoomFlow`, `JoinByCodeTile.toastError` (param-less generic variant — the public by-code path genuinely has no room object in scope), and **both** RoomPage paths, refactored through one shared `joinFailureMessage(code, room)` so a future error code cannot be added to one path and forgotten on the other. That is 9.6's lesson made structural rather than remembered.
- **A real 0 survives everywhere.** `honorScoreOrPrior(...)` at every viewer-score read (never `|| 80`), `typeof === "number"` in the WS guard (never truthiness), explicit `minHonor > 0` / `allowNewPlayers === false` comparisons on the card. Pinned by tests at each layer.
- **Scope guardrails respected.** No change to `user/honor.go`'s math, migration `000017`, `HonorPanel.tsx`, the honor trend, or `event:honor_updated`. No owner edit endpoint, no lobby filtering of gated rooms, no `match_abandoned` reason-union change, no admin surface.
- **Epic 9 is complete after this story.** `epic-9-retrospective` is now on the radar; noted in the sprint-status entry.
- **Four items deferred** to `deferred-work.md` (Open Questions 1, 4, 5, 6), all raised by the story itself as non-blocking.
- **One scope-guardrail exception, taken on the PO's explicit direction.** `000017_add_honor_to_users.down.sql` advertised the up migration's backfill as "written to be re-runnable as a reconciliation recipe" -- flatly contradicting the up header that 9.7's review pass 2 had corrected to "DEPLOY-ONLY. DO NOT RE-RUN IT". I flagged it and left it alone (9.8's guardrails forbid touching `000017`); the PO chose to fix it here, since both migrations ship in the same feature. The down header now documents the reversal as LOSSY, states that a down-then-up cycle re-runs the deploy-only backfill by construction, enumerates what that destroys (operator pardons; and the concurrent-double-disconnect rule, since `matches` carries no per-seat presence and a second absent seat gets re-credited a completion), and records the stale claim so nobody reinstates it. Comment-only -- `git diff` confirms not one SQL statement changed, and the six `DROP COLUMN`s were executed against the live dev schema inside a rolled-back transaction rather than via a real `down 2; up 2` cycle, precisely because that cycle would itself re-run the backfill the comment warns about. Dev DB left at version 18, all six columns intact, not dirty.

### File List

**New**

- `server/migrations/000018_add_honor_gate_to_rooms.up.sql`
- `server/migrations/000018_add_honor_gate_to_rooms.down.sql`
- `server/internal/room/honor_handler_test.go`

**Modified — server**

- `server/internal/room/model.go`
- `server/internal/room/handler.go`
- `server/internal/apperr/errors.go`
- `server/internal/user/honor_service.go`
- `server/internal/user/honor_service_test.go`
- `server/internal/ws/events.go`
- `server/cmd/api/main.go`
- `server/internal/room/coin_handler_test.go`
- `server/internal/room/handler_test.go`

**Modified — client**

- `client/src/shared/types/wsEvents.ts`
- `client/src/shared/types/apiTypes.ts`
- `client/src/shared/stores/roomStore.ts`
- `client/src/shared/stores/roomStore.test.ts`
- `client/src/shared/hooks/useWsDispatch.ts`
- `client/src/shared/hooks/useWsDispatch.test.ts`
- `client/src/shared/providers/WebSocketProvider.tsx`
- `client/src/features/lobby/LobbyPage.tsx`
- `client/src/features/lobby/LobbyPage.test.tsx`
- `client/src/features/lobby/components/RoomCard.tsx`
- `client/src/features/lobby/components/RoomCard.test.tsx`
- `client/src/features/lobby/components/JoinByCodeTile.tsx`
- `client/src/features/lobby/components/JoinByCodeTile.test.tsx`
- `client/src/features/room/CreateRoomModal.tsx`
- `client/src/features/room/CreateRoomModal.test.tsx`
- `client/src/features/room/RoomPage.tsx`
- `client/src/features/room/RoomPage.test.tsx`
- `client/src/shared/i18n/en.json`
- `client/src/shared/i18n/sr.json`
- `client/src/shared/i18n/mk.json`
- `client/src/shared/i18n/hr.json`

**Renamed — client (AC10, own commit)**

- `client/src/features/lobby/components/InsolventEjectionModal.tsx` → `RoomEjectionModal.tsx`
- `client/src/features/lobby/components/InsolventEjectionModal.test.tsx` → `RoomEjectionModal.test.tsx`
- `client/src/shared/hooks/useInsolventEjectRedirect.ts` → `useRoomEjectRedirect.ts`

**Modified — docs**

- `_bmad-output/implementation-artifacts/sprint-status.yaml`
- `_bmad-output/implementation-artifacts/deferred-work.md`
- `_bmad-output/implementation-artifacts/9-8-honor-gated-rooms.md`

### Change Log

| Date | Change |
| --- | --- |
| 2026-07-30 | Story implemented on `feat/9-7-honor-score-system` (Task 0 fallback, per user direction: 9.7 + 9.8 ship as one feature). Migration 000018, room columns, three apperrs, one pure gate function shared by four call sites, `HonorService` interface + `HonorForUsers`, generalized eject helpers, `system:honor_ejected` (zero drift-gate touchpoints), client store rename as its own commit, honor branch, card + create-modal UI, four join call paths, i18n ×4. |
| 2026-07-30 | Three design corrections made during implementation: lenient heir-candidate honor reads split from the strict subject read; nil `balances` map treated as "solvency not evaluated" rather than "everyone broke"; the quick-play auto-start path added as the third and fourth `ejectSeatsAtStart` call sites the story had not enumerated. |
| 2026-07-30 | Fixed a pre-existing 9.7 contradiction at the PO's direction, overriding 9.8's scope guardrail on migration `000017`: its down-migration header no longer advertises the deploy-only backfill as re-runnable, and now documents the reversal as lossy. Comment-only; SQL unchanged and re-verified against the live dev schema in a rolled-back transaction. |
| 2026-07-30 | Gates green: gofmt/vet/golangci-lint clean, `go test ./...` 18/18 packages with DB-backed honor tests passing, tsc clean, vitest 101 files / 1096 tests, eslint + prettier clean, `make lint` + `make test` green. Status → review. |
