---
title: "Fix declaration reveal never firing — broadcast on card-play transition"
type: "bugfix"
created: "2026-04-17"
status: "done"
context: ["spec-declaration-reveal-cards-and-timing.md"]
baseline_commit: "5fd5303fea82d8a7960eb146ce8dd11bc9dc2724"
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** `event:declarations_resolved` is wired to broadcast only inside the `ActionDeclare`/`ActionSkipDeclare` switch case in [server/internal/session/manager.go:446-471](server/internal/session/manager.go#L446-L471). In the Bitola variant, `DeclarationsResolved` only flips true when trick 1 finishes — i.e. during an `ActionPlayCard` that triggers `resolveTrickWithDeclarations` → `resolveDeclarationsForHand` ([declarations.go:415-453](server/internal/game/declarations.go#L415-L453)). Because the transition never occurs during declare/skip-declare actions, the broadcast is dead code. Clients never receive the event, `declarationReveal` in the store stays null, and no one sees the reveal at start of trick 2. The prior 3 commits reworked the client reveal UI, but the event never reached the client.

**Approach:** Extract the transition broadcast into a small helper on `Manager` and call it from both the `ActionPlayCard` branch (where the Bitola transition actually happens) and the existing declare/skip-declare branch (kept for future variants). No payload-shape or client changes.

## Boundaries & Constraints

**Always:**

- Fire exactly once per hand — guard on `newState.DeclarationsResolved && !oldState.DeclarationsResolved`.
- Broadcast to all four player user IDs — the seat-anchored reveal renders on every client, including both teammates of the winner.
- Preserve existing broadcast ordering inside `ActionPlayCard`: `event:card_played` → `event:trick_resolved` (if any) → `event:declarations_resolved` (new placement) → `event:hand_scored` (if any) → `event:match_end` (if any) → `event:game_state`. Declarations-resolved must precede the authoritative state sync so the client handler sets `declarationReveal` before any follow-up state logic runs.
- Payload shape unchanged — `{ winnerTeam, declarations: [{ playerSeat, type, value, cards }] }`. Iterate `newState.Players[*].Declarations`; only the winning team's remain after `resolveDeclarationsForHand`.

**Ask First:** None — this is a localized server-side fix to an unreachable code path.

**Never:**

- Modify the client `DeclarationReveal` component or the i18n strings — existing unit tests prove the component renders cards correctly given a valid payload.
- Change when `DeclarationsResolved` flips server-side (game-rule territory, out of scope).
- Broadcast multiple times or to a subset of users.

## I/O & Edge-Case Matrix

| Scenario                                                                                         | Input / State                                                                                                          | Expected Broadcast                                                                  | Notes                                                                         |
| ------------------------------------------------------------------------------------------------ | ---------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------- | ----------------------------------------------------------------------------- |
| Trick 1 ends, Team B declared FoaK, Team A declared nothing                                      | oldState: `DeclarationsResolved=false`, newState: `DeclarationsResolved=true`, only Team B players have `Declarations` | `event:declarations_resolved` with `winnerTeam=1`, Team B's declarations with cards | Fires from `ActionPlayCard` branch                                            |
| Trick 1 ends, both teams declared, Team B wins, Team A's cleared by `resolveDeclarationsForHand` | same as above but oldState had Team A declarations too                                                                 | `event:declarations_resolved` with only Team B's declarations                       | Losing-team clear happens server-side already; iteration naturally skips nils |
| Trick 1 ends, no one declared                                                                    | `winnerTeam` stays `nil`, all `Declarations` nil                                                                       | Event still broadcast with `winnerTeam=null`, empty declarations                    | Client early-returns on `winnerTeam===null` — unchanged                       |
| Second card of trick 1 played (not the 4th)                                                      | `DeclarationsResolved` not yet flipped                                                                                 | No broadcast                                                                        | Transition guard blocks it                                                    |
| Declare/skip-declare action fired                                                                | `DeclarationsResolved` cannot flip here in current rules                                                               | No broadcast; helper still safe to call                                             | Future-proofing for Croatian variant                                          |

</frozen-after-approval>

## Code Map

- `server/internal/session/manager.go` — action dispatcher; hosts `broadcastActionResult`; currently contains dead-code broadcast at lines 446-471.
- `server/internal/game/declarations.go` — `resolveTrickWithDeclarations` (the actual transition site for Bitola) and `resolveDeclarationsForHand` (clears losing team's declarations).
- `server/internal/ws/events.go` — event constant `EventDeclarationsResolved`; no change.
- `client/src/shared/hooks/useWsDispatch.ts:185-190` — client already handles the event correctly; verifies the fix end-to-end.
- `client/src/features/game/components/DeclarationReveal.tsx` — already correct given a valid payload; no change.

## Tasks & Acceptance

**Execution:**

- [x] `server/internal/session/manager.go` — add private method `(m *Manager) broadcastDeclarationsResolvedIfTransition(oldState, newState *game.GameState, userIDs []uint)` that encapsulates the transition guard + payload build + broadcast. Call it inside `case game.ActionPlayCard:` after the `event:trick_resolved` broadcast and before `event:hand_scored`. Replace the inline block in `case game.ActionDeclare, game.ActionSkipDeclare:` with a call to the same helper.

**Acceptance Criteria:**

- Given a session with 4 players, trump called, declarations made on trick 1 by Team B, when player 4 plays the card that resolves trick 1, then `event:declarations_resolved` is broadcast to all 4 user IDs with `winnerTeam=1` and Team B's declarations (each with a non-empty `cards` array).
- Given the same scenario but with no declarations by anyone, when trick 1 resolves, then `event:declarations_resolved` is broadcast with `winnerTeam=null` and empty `declarations`.
- Given the broadcast has fired once in a hand, when any subsequent card is played, then no further `event:declarations_resolved` is sent for that hand (the transition guard holds).
- Given `go test ./internal/session/... ./internal/game/...` runs from `server/`, then all existing tests pass (no regressions).
- Given manual play with a declaration winner, when trick 2 starts, then every connected client renders `DeclarationReveal` anchored to the winning player's seat with the winning team's cards, for ~4s, on all four screens including both winning teammates.

## Verification

**Commands:**

- `cd server && go build ./...` — builds clean.
- `cd server && go test ./internal/session/... ./internal/game/...` — all green.
- `cd server && golangci-lint run ./...` (if available locally) — clean. CI covers this otherwise.

**Manual checks:**

- Launch dev server and client, seat 4 players, play a hand where at least one player has a sequence/FoaK declaration, confirm reveal panel appears at start of trick 2 on every client with correct seat anchoring and card rendering.

## Suggested Review Order

- Entry point: new call site wires the missing broadcast into the card-play flow, strictly between trick_resolved and hand_scored so the reveal lands before state sync.
  [`manager.go:394`](../../server/internal/session/manager.go#L394)

- Helper definition: transition-guarded, idempotent broadcast; empty `decls` emits `make([]..., 0)` so JSON stays `[]` not `null`.
  [`manager.go:510`](../../server/internal/session/manager.go#L510)

- Call from declare/skip-declare: kept as a safety net for future variants (Croatian) where the flag could transition here.
  [`manager.go:455`](../../server/internal/session/manager.go#L455)

- Transition site on the game side (unchanged) — confirms why the broadcast had to move out of the declare branch in the first place.
  [`declarations.go:415`](../../../server/internal/game/declarations.go#L415)

- Client event handler (unchanged) — proves the payload reaches `gameStore.setDeclarationReveal`, which mounts the already-correct `DeclarationReveal` component.
  [`useWsDispatch.ts:185`](../../../client/src/shared/hooks/useWsDispatch.ts#L185)
