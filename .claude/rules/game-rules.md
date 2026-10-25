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

- Play is counter-clockwise for dealing, bidding and tricks. Seats run 0, 1, 2, 3 counter-clockwise and the next seat is `(seat + 1) % 4`.
- The trick winner leads the next trick.
- Card IDs are rank plus suit: ranks `7 8 9 T J Q K A`, suits `S H D C` (for example `KS`, `TH`).

## Bidding

Select the path from a `VariantRules` field, never by comparing the variant name. Keep each outcome as its own code path.

- **Bitola:** two rounds. Round 1 offers the flipped candidate's suit, round 2 opens every other suit. Four passes in round 2 reshuffle and move the dealer.
- **Croatian:** one round with no candidate card. Players bid on their six visible cards, and the two face-down cards stay hidden from everyone, their owner included, until trump is set. Bidding starts to the dealer's right and the dealer speaks last. If the other three pass, the dealer must pick a suit. There is no second round and no reshuffle.

## Declarations

- **Bitola:** declared during trick 1 and revealed at the start of trick 2. A card can belong to only one declaration, so overlapping groups keep the higher one.
- **Croatian:** a dedicated declaration phase sits between bidding and trick 1, and one card may count in several declarations.
- The highest declaration decides the winning team. Only that team's declarations score, and the losing team's are never revealed.
- Belote and Rebelote (trump K and Q in one hand) score 20 only when announced. Announcing on the Queen is Belote, on the King is Rebelote. Declining on the first card forfeits the bonus.

## Card play (`validation.go`, mirrored in the client's `legalCards`)

1. Follow the led suit if you hold it. You must overplay only while the led suit can still win: it is trump, or no trump has cut the trick yet.
2. If void in the led suit but holding trump, you must cut, over-trumping the highest trump on the table when you can.
3. If void in both, any card is legal.

## Scoring

- Hand scoring and match scoring are separate functions in `scoring.go`.
- The taker's team must score strictly more than the opponents, declarations on both sides counted. A tie fails, and the opponents take the hand. Bitola will eventually carry tied points over; the current tie behavior is a deliberate stand-in, not a bug.
- Capot (+100) replaces the last-trick bonus (+10). It is not added to it.
- A fall or a Capot gives one side the whole hand. What happens to declarations then is the room's "Declarations on a fall" rule (`VariantRules.DeclarationsTransfer`), which `wholeHandAward` in `scoring.go` applies:
  - **On (default):** the side taking the hand banks every point, including both sides' declarations and Belote.
  - **Off:** that side banks all card points plus only its own declarations and Belote. The other side's are voided, and nobody scores them.
  - Belote follows the declarations, and a Capot obeys the rule just like a fall. A room with declarations off always stores the rule as on.
- If both teams cross the target in the same hand, the taker's team wins.
- On timer expiry, auto-play picks the first legal card sorted by suit, then rank.

## Wire and state

- A reconnecting or observing seat receives a projected state: its own hand is real, every other hand is `[]` with a `handCount`, and there is no deck. Raw state must never reach the wire.
- Deal and splash grace in `match/deal_grace.go` must match the client's `shared/lib/motion.ts`.
