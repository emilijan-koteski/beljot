---
paths:
  - "server/internal/game/**"
  - "server/internal/bot/**"
  - "server/internal/match/**"
  - "client/src/features/match/**"
  - "client/src/features/rules/**"
---

# Belot game rules

These are owner-confirmed rules. Do not "fix" them toward a generic Belot ruleset.

## Turn order and tricks

- Seats run 0, 1, 2, 3 counter-clockwise around the table, and they never move on screen.
- Play is counter-clockwise by default: the next seat is `(seat + 1) % 4`. The room's "Direction of play" rule (`VariantRules.Clockwise`, default off) makes it clockwise, so the next seat is `(seat + 3) % 4`. The flag is fixed for the whole match.
- Never hard-code the step. The engine moves through `VariantRules.NextSeat`, `SeatAfter` and `PlayDistance` in `types.go`, which all go through `game.StepSeat`. The direction covers the deal order, the first bidder, Bitola round 2, the reshuffle and between-hand dealer moves, the trick-1 leader, the turn advance (including after a Belote prompt), the declaration tiebreak distance, the bot's `seatsYetToPlay`, and the client's `dealSchedule.ts`.
- Partners (`seat + 2`), teams (`seat % 2`) and the trick winner do not depend on the direction.
- The trick winner leads the next trick.
- Card IDs are rank plus suit: ranks `7 8 9 T J Q K A`, suits `S H D C` (for example `KS`, `TH`).

## Bidding

Select the path from a `VariantRules` field, never by comparing the variant name. Keep each outcome as its own code path.

- **Bitola:** two rounds. Round 1 offers the flipped candidate's suit, round 2 opens every other suit. Four passes in round 2 reshuffle and move the dealer.
- **Croatian:** one round with no candidate card. Players bid on their six visible cards, and the two face-down cards stay hidden from everyone, their owner included, until trump is set. Bidding starts with the seat after the dealer in the direction of play (the dealer's right by default, the left in a clockwise room), and the dealer speaks last. If the other three pass, the dealer must pick a suit. There is no second round and no reshuffle.

## Declarations

- **Bitola:** `DeclarationTiming` is `turn_window`. In tricks 1 to N (`DeclarationLastTrick`, the room's "Declare until trick", 1–3, default 3), a seat holding a meld in its current hand is prompted before its card: Declare, or Continue (Skip in trick N). Declaring takes every meld the seat holds and opens the simultaneous declaring phase mid-trick, with the declarer already answered. Every other seat answers once, in Croatian's dialog and window. The contest then resolves at once, play resumes at the declarer's card, and nobody is prompted again that hand. A card can belong to only one declaration, so overlapping groups keep the higher one.
- **Croatian:** a dedicated declaration phase sits between bidding and trick 1, and one card may count in several declarations.
- The highest declaration decides the winning team. Only that team's declarations score, and the losing team's are never revealed.
- Belote (trump K and Q in one hand) scores 20 only when announced. Bitola calls it Belote when announced on the Queen and Rebelote on the King; Croatian calls it Belote on either. Declining on the first card forfeits the bonus.

## Card play (`validation.go`, mirrored in the client's `legalCards`)

1. Follow the led suit if you hold it. You must overplay only while the led suit can still win: it is trump, or no trump has cut the trick yet.
2. If void in the led suit but holding trump, you must cut, over-trumping the highest trump on the table when you can.
3. If void in both, any card is legal.

## Scoring

- Hand scoring and match scoring are separate functions in `scoring.go`.
- The taker's team must score strictly more than the opponents, declarations on both sides counted. Less is a fall, and the opponents take the hand. `game.SettleHand` in `scoring.go` is the one settlement rule; `scoreHand` and the bot search both call it, and it branches on `VariantRules.TieRule`:
  - **Croatian (`all_to_opponents`):** a tie is a fall too.
  - **Bitola (`hanging_points`):** on a tie the opponents bank their own total, the taker banks 0, and the taker's total hangs as `GameState.HangingPoints` `{amount, team}`. A tie is not a failed contract. At most one entry hangs, and every seat sees it.
  - The next scored hand pays the hanging points to whoever takes that hand: the Capot team, else the taker if it passes, else the taker's opponents (a fall or another tie). Only a tie creates a new entry. The payout is inside that hand's totals, so hand totals still add up to the match score, and it counts toward the target in the same end-of-hand check.
  - A reshuffle keeps the entry. Points still hanging when the match ends (target, all-eight-trumps deal, surrender, abandonment) are discarded, and a stop at target banks running totals without them.
- Capot (+100) replaces the last-trick bonus (+10). It is not added to it.
- A fall or a Capot gives one side the whole hand. What happens to declarations then is the room's "Declarations on a fall" rule (`VariantRules.DeclarationsTransfer`), which `wholeHandAward` in `scoring.go` applies:
  - **On (default):** the side taking the hand banks every point, including both sides' declarations and Belote.
  - **Off:** that side banks all card points plus only its own declarations and Belote. The other side's are voided, and nobody scores them.
  - Belote follows the declarations, and a Capot obeys the rule just like a fall. A room with declarations off always stores the rule as on.
- If both teams cross the target in the same hand, the higher total wins; an exact tie goes to the taker's team (`determineMatchWinner`).
- On timer expiry, auto-play picks the first legal card sorted by suit, then rank.
- Three per-move timer expiries in a row for one human seat forfeit the match exactly like a disconnect (`missed_turns.go`). The second miss sends only that player `event:missed_turn_warning`. A move of their own resets the count. The declaring-phase window (Croatian, or a Bitola declaration round) and the hand-complete pause do not count; a timed-out Bitola turn prompt does.

## Wire and state

- A reconnecting or observing seat receives a projected state: its own hand is real, every other hand is `[]` with a `handCount`, and there is no deck. Raw state must never reach the wire.
- Deal and splash grace in `match/deal_grace.go` must match the client's `shared/lib/motion.ts`.
