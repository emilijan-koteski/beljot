# Bot decision rules (Beljot, both variants)

This document describes exactly how the bot decides, across every scenario, what it
considers, and with examples, for both the Bitola and the Croatian variant. It reflects the
current logic including phases 1 and 2 of bot v2 (`BOT_V2_PROPOSAL.md` §4.7). Phase 1 is the new
memory facts, the hand-shape bidding table, the led-suit-secure take, the Ace cashed before the
Ten and the Ten under the Ace on the partner's trick. Phase 2 is the Monte Carlo search that
plays the cards from trick 3 (§5), with the rule layer as the fallback. The rule layer also
carries the earlier tuning: partner trump-draw with a declaration-inference abort when the
opponents provably hold the master, boss preservation, uncuttable-boss last-trick retention,
suppressing trump draws once the opponents are void of trump, leading into a partner's void,
smearing onto a partner's boss, seat-aware **secure trick takes** with a material-stake gate and
control-trump economy, the **boss-preserving smear** with its endgame/dead-suit exceptions, and
never fighting a partner who is **forced to win a trump-led trick**.

Suit notation: ♠ spades, ♥ hearts, ♦ diamonds, ♣ clubs. Ranks: A, K, Q, J, T (ten), 9, 8, 7.

---

## 1. Where the logic lives

| Concern | File | Notes |
|---|---|---|
| Card-play decision logic (pure, deterministic) | `server/internal/bot/bot.go` | `Decide`, `chooseCard`, declarations |
| Bidding (the hand-shape table) | `server/internal/bot/bidding.go` | `decideBid`, `matchesBidTable` |
| What the bot is allowed to know | `server/internal/bot/view.go` | Redacted, seat-local snapshot |
| What the bot remembers | `server/internal/bot/memory.go` | Played cards, voids, revealed declarations, located cards |
| Search layer: solver, belief sampler, settlement, chooser, sample schedule | `server/internal/bot/search/` | `solver.go`, `sampler.go`, `settle.go`, `pimc.go` (`Choose`, `SamplesForTrick`), `cards.go` |
| View to search input, when the search applies, the searched card | `server/internal/bot/search_bridge.go` | `SearchApplies`, `SearchCard`, `DecideWith` |
| Worker pool, search settings | `server/internal/match/search_pool.go` | `BotSearchSettings`, bounded queue, panic recovery |
| Search settings from the environment | `server/internal/config/config.go` | `BELJOT_BOT_SEARCH_*` (§5.6) |
| Timing, humanization, scheduling | `server/internal/match/bot_driver.go` | Think delay, when the bot acts, queueing and consuming search jobs |
| Card point and strength tables | `server/internal/game/types.go` | Tables every heuristic reads |
| Legality (the rules cage) | `server/internal/game/validation.go` | The bot only ever chooses among legal cards |
| Tests for all of the below | `server/internal/bot/bot_test.go`, `simulation_test.go`, `search_bridge_test.go`, `search/*_test.go`, `server/internal/match/bot_search_test.go` | Table tests, heuristic-vs-random simulation, the search gates |

`Decide(View) game.Action` is **pure and deterministic**: there is no randomness in card
choice. The only randomness is the think delay in the match layer. Every decision below is
fully reproducible and table-testable. The search (§5) is a separate entry point that `Decide`
never calls, and it is deterministic for a given view too.

---

## 2. Card values and strength

```
TRUMP      points:    J=20  9=14  A=11  T=10  K=4  Q=3  8=0  7=0
TRUMP      strength:  J > 9 > A > T > K > Q > 8 > 7
NON-TRUMP  points:    A=11  T=10  K=4  Q=3  J=2  9=0  8=0  7=0
NON-TRUMP  strength:  A > T > K > Q > J > 9 > 8 > 7
```

Note: in the non-trump order the **Ten outranks the King** (T=6, K=5), which matters for the
follow-suit / must-overplay rules.

---

## 3. Top-level decision order (`Decide`)

Every time it is the bot's turn, it checks these in strict priority:

1. **Partner proposed surrender** -> always accept. The bot never initiates surrender and
   never responds to an opponent's proposal.
2. **Bidding phase** -> `decideBid` (Scenario A).
3. **Belote / Rebelote pending** (+20) -> always announce. Unconditional value.
4. **Awaiting declaration** -> always declare. The engine auto-detects melds; declaring
   always maximizes points.
5. **Otherwise** -> `chooseCard` (Scenario B). From trick 3 the match layer first offers a card
   play to the search (§5); `chooseCard` then plays only when the search does not apply or does
   not deliver in time.

Scenarios 1, 3, 4 are "always do it" with nothing to tune. The real intelligence is in
**bidding** and **card play**.

---

## 4. Scenario A: Bidding (choosing trump)

The bot calls a suit when its **bid hand** fits one of twelve hand shapes in a table
(`matchesBidTable` in `server/internal/bot/bidding.go`). It reads the variant from
`View.HasTrumpCandidate`, never from a variant name. The rows come from the simulation
calibration in `BOT_V2_LAB_RESULTS.md` §3.

### 4.1 The bid hand and the two variants

- **Bitola** has a face-up candidate card. Whoever picks trump always receives it (the engine
  appends it in `handlePickTrump`, both rounds), so the bot evaluates the hand it would actually
  hold: `hand + candidate`, six cards. In round 1 only the candidate's suit is offered and the
  candidate counts toward its trump count. In round 2 the candidate's suit is locked out and the
  candidate is a side card, which can supply a side Ace.
- **Croatian** has no candidate. The bot evaluates its six visible cards (the two face-down cards
  are not counted), any suit may be called, and there is a single round.

### 4.2 The table

For one suit of the bid hand: **count** is the number of cards of that suit, **side Aces** is the
number of Aces in the other three suits, and **position** is the number of seats that have
already passed in the current round, counted from the seat after the dealer in the room's
direction of play (`seatsPassedBefore`): 0 for the first seat to speak,
3 for the dealer. The bot calls the suit when **any** row matches.

| # | Cards of the suit | Side Aces | Variant and position |
|---|---|---|---|
| 1 | Jack and 9 | any | both |
| 2 | Jack, 3 or more cards | any | both |
| 3 | the 9, 4 or more cards | any | both |
| 4 | the Ace, 4 or more cards | any | both |
| 5 | 4 or more cards | at least 1 | both |
| 6 | 5 or more cards | any | both |
| 7 | 9 and Ace, exactly 3 cards | at least 1 | Bitola only |
| 8 | Jack and Ace, exactly 2 cards | at least 1 | Bitola only |
| 9 | Jack, exactly 2 cards | at least 2 | Bitola only |
| 10 | the 9 but neither Jack nor Ace, exactly 3 cards | at least 1 | Bitola only |
| 11 | Jack, exactly 2 cards | at least 1 | Bitola only, third seat to speak (position 2) |
| 12 | Jack, exactly 2 cards | at least 1 | Croatian only, dealer's partner (position 1) |

Every other shape passes, for example the Jack with one small trump and no side Ace, three small
trumps without the Jack or the 9, or four small trumps with no side Ace.

| Hand (incl. candidate) | Variant, position | Call? | Why |
|---|---|---|---|
| J♥ 9♥ + 7♠ 8♠ 9♠ Q♦ | Bitola | yes | row 1, no side Ace needed |
| 7♥ 8♥ Q♥ K♥ + A♠ 7♣ | Bitola | yes | row 5, four trumps with a side Ace |
| 7♥ 8♥ Q♥ K♥ + 7♠ 8♠ | Croatian | no | four small trumps, no side Ace |
| 9♥ A♥ Q♥ + A♠ 7♣ 8♣ | Bitola | yes | row 7 |
| 9♥ A♥ Q♥ + A♠ 7♣ 8♣ | Croatian | no | row 7 is Bitola only |
| J♥ 7♥ + A♠ 7♣ 8♣ 9♦ | Bitola, first to speak | no | needs position 2 (row 11) |
| J♥ 7♥ + A♠ 7♣ 8♣ 9♦ | Bitola, third to speak | yes | row 11 |

### 4.3 Choosing the suit, and the forced pick

- **Bitola round 1:** only the candidate's suit is offered. The bot picks it exactly when the
  table matches, otherwise it passes.
- **Bitola round 2 and Croatian:** the bot evaluates every allowed suit (all but the candidate's,
  when there is one) and, among the suits that match, picks the one with the highest
  `trumpSuitScore`:

  ```
  score(suit) = sum over cards of that suit of ( trumpPoints[rank] + 10 )
  ```

  The per-card `trumpLengthBonus` (currently 10) biases toward a longer suit over raw points.
  Ties go to the first suit in the order ♠ ♥ ♦ ♣.
- **Forced pick:** when the engine forbids a pass (`View.MustPickTrump`, the Croatian dealer after
  three passes) and no suit matches, the bot picks the suit with the highest `trumpSuitScore`
  whatever the rows say. It never passes there, because the engine rejects the pass and the bot
  timer would re-arm forever. Bitola has no forced pick: if no suit qualifies in round 2, all
  four pass and the deck reshuffles.

### 4.4 Bidding tuning levers

- The rows of `matchesBidTable`, including the position conditions on rows 11 and 12: the
  biggest dial for aggressive vs. conservative. Re-derive them with the lab, not by feel.
- `trumpLengthBonus` (currently 10): raise to favor length, lower to favor points when several
  suits match or the pick is forced.
- The table considers only the bid hand and the seat's position. It considers no partner or
  match score.

---

## 5. The search layer (from trick 3)

From the third trick on, a bot seat does not run `chooseCard`. It plays the card found by a Monte
Carlo search: draw complete deals that agree with everything the seat knows, solve each one
exactly as if every hand were open, settle the result the way the engine scores a hand, and play
the card with the best average for the seat's team. Bidding, declarations, Belote, surrender and
tricks 1 and 2 stay with the rule layer (§3, §4, §6), and the rule layer is also the fallback
whenever the search does not deliver a card. The search is the phase-2 design of
`BOT_V2_PROPOSAL.md` §4.7; the lab measurements behind the start at trick 3 are in
`BOT_V2_LAB_RESULTS.md` §2.2 (searching tricks 1 and 2 as well was worth between 0 and +5 at 12
samples and costs seconds per decision with the current solver).

`Decide` never calls the search. The search is a separate entry point in
`server/internal/bot/search_bridge.go` (`SearchApplies`, `SearchCard`, `DecideWith`) that the
match layer drives (5.5), so `Decide` stays pure and the rule layer needs no search to be tested.

### 5.1 When the search plays (`SearchApplies`)

All of these must hold, otherwise the rule layer plays:

- the first searched trick (`BELJOT_BOT_SEARCH_FROM_TRICK`, default 3) is above 0 and the current
  trick (`View.TrickNumber`, 1 to 8) has reached it;
- the phase is playing, the trump suit is known and it is the seat's own turn;
- nothing else is pending for the seat: no Belote announcement, no declaration, no partner
  surrender proposal;
- the seat has more than one legal card;
- when the room stops at the match target (`StopAtTarget`), neither team can still reach the
  target inside this hand. The test adds each team's match score, hand points and declaration
  points, and compares the gap to the target with every card point still to play plus the +10 for
  the last trick. The search has no race objective yet, so such hands go to the rule layer.

### 5.2 What is sampled

`informationFrom` turns the `View` into what the sampler may use:

| Input | Value |
|---|---|
| own cards | the seat's hand |
| gone | cards played this hand and the cards in the current trick |
| known per seat | the seat's declared and located cards that are still unplayed (`KnownCards`), minus the seat's own hand |
| cannot hold per seat | every card of a suit the seat is known void in (including a trump void proven by a discard), plus the cards just outside a revealed run (`cardsProvablyAbsent`), minus the known cards |
| need per seat | the cards the seat still holds (the hand size, one less for a seat that already played to this trick) minus its known cards |
| unseen | the whole deck minus own, gone and every known card |

`search.Sample` then deals the unseen cards to the seats' remaining counts. It first shuffles and
deals up to 200 times, rejecting a deal that puts a card where `cannot hold` forbids it. This is
uniform over the consistent deals. If every draw is rejected it builds a deal card by card, up to
200 times, giving each card to a random seat that still needs cards and may hold it. That
fallback is not exactly uniform. When no deal can be built the search stops with the samples it
has, and with none it reaches no decision and the rules play.

Every deal that satisfies the hard facts is equally likely. The sampler does not weigh deals by
how the seats bid or by what they led or discarded (see 5.9).

### 5.3 How a sample is solved and settled

- **Solve** (`search.Solver`): an exact double-dummy search for the fixed trump suit and the room's
  direction of play, alpha-beta with a transposition table at trick boundaries over the four
  hands as 32-bit masks. Legality mirrors `game/validation.go`: follow the suit, overplay the led
  suit while able, cut when void, overtrump. It returns, for every legal card of the seat, the
  best-play value of the rest of the hand as team A minus team B points, including the +10 for
  the last trick or the Capot bonus. `search/solver_test.go` checks it against a brute-force
  minimax and against the engine's legality and trick winner; the lab prototype's speed table is
  in `BOT_V2_LAB_RESULTS.md` §1.1.
- **Settle** (`search.Settle`): the remainder value becomes the award the engine would bank,
  mirroring `scoreHand`. It adds the hand points already banked and both teams' declaration and
  Belote points. A Capot gives the whole hand and the Capot bonus to the sweeping team. A caller
  who does not beat the defenders (a tie loses) concedes the hand: the defenders take all card
  points plus either both teams' declarations (`DeclarationsTransfer`) or only their own.
  `TestSettleMatchesEngineAtTheLastTrick` pins the agreement with the engine.
- **Choose** (`search.Choose`): for each legal card, the settled values are averaged over the
  samples, signed for the seat's team, and the best average is played. Equal sums go to the lower
  card index, so ties are deterministic.

### 5.4 Samples per trick and the early stop

| Trick | Samples (at most) |
|---|---|
| 1 and 2 | none (rule layer) |
| 3 | 16 |
| 4 | 30 |
| 5 | 40 |
| 6, 7 and 8 | 60 |

The search stops early once at least 8 samples are in and the best card leads the runner-up by
more than one standard error. The standard error is that of the per-sample difference between
the two cards (the same deals are scored for both, so the comparison is paired). A clear choice
therefore can stop after 8 solves, and a close one runs the full count.

### 5.5 Deadline, worker pool and fallback

The search runs in the match layer (`match/bot_driver.go`, `match/search_pool.go`):

- When the bot's think timer is armed for a card play (`maybeScheduleBotAction`), the match
  layer builds the seat's view under the session lock and queues a search job. It does so only
  when the seat's turn is a plain card play: playing phase, no pending Belote or declaration, and
  no open surrender proposal from anyone. The job itself runs on a pool worker without the lock.
  The pool has a fixed number of workers and a bounded queue (four jobs per worker). A full queue
  drops the new job, so a burst of bot turns never grows memory or blocks a timer goroutine.
- The job has a deadline of `BELJOT_BOT_SEARCH_DEADLINE` (default 500 ms) counted from when it
  was queued, so time spent waiting in the queue counts. The deadline is checked before every
  sample. When it has passed, all samples are discarded and the search reports no decision. A
  partial answer is never played.
- A finished search is stored for the seat only if the session is still open and the seat's
  action generation still equals the one the timer was armed with.
- When the seat's current timer fires (`handleBotActionTimer`), the stored result is taken and
  cleared. It is used only if it was made for the same generation and the same decision context
  (phase, hand number, pending Belote, awaiting declaration, surrender pending). `DecideWith`
  then plays the searched card if it is still legal and nothing else is pending, and otherwise
  calls `Decide`.
- The bot never waits for the search. If the job is still running when the think delay ends (by
  default 1 to 2.5 s for a play), if it was dropped, timed out, or panicked (the pool recovers and
  logs it), or if the position changed, the rule layer plays the card. A late result is
  discarded.

Only `server/cmd/api/main.go` enables the search, through `Manager.SetBotSearch`. A `Manager`
that never calls it plays every card by the rules.

### 5.6 Settings

Read once in `server/internal/config` and passed to `SetBotSearch`:

| Variable | Default | Meaning |
|---|---|---|
| `BELJOT_BOT_SEARCH_WORKERS` | `0` | worker count; 0 or less means automatic, `max(1, min(4, NumCPU/2))` |
| `BELJOT_BOT_SEARCH_DEADLINE` | `500ms` | time one search may take, from queueing; 0 or less means the default |
| `BELJOT_BOT_SEARCH_FROM_TRICK` | `3` | first trick the search plays; 0 or less turns the search off |

A value below 3 behaves like 3 for the cards played, because no sample count is defined for
tricks 1 and 2: the job is queued, reaches no decision, and the rules play (the log line is the
same "search timed out" message as for a missed deadline).

### 5.7 Determinism

For a given view and without a deadline hit, `SearchCard` is deterministic. The random stream is
seeded from a hash (FNV-64a) of the information state: seat, own cards, unseen cards, known and
cannot-hold masks and need counts. The same information gives the same samples and the same card,
and any new fact (a void, a revealed card) changes the seed. Reproducing a bot's play therefore
needs its view, not a recorded seed. The only nondeterminism left is timing: a missed deadline,
or a job still running when the think timer fires, hands the card to the rule layer, so a
loaded machine can turn a search into a rule-layer play.

### 5.8 Measured strength and the gates

Measured on this branch (phase 2, the settings above):

| Measurement | Result |
|---|---|
| search seats 0/2 vs the random baseline, 60 hands (`TestSimulation_SearchBeatsRandomBaseline`) | 75 to 79% of all points; gate: at least 60% |
| duplicate search vs the rule layer, 40 deals (`TestSimulation_SearchDoesNotLoseToTheRules`) | +6.0 ± 5.4 per deal; gate: mean at least −3.0 |
| the same duplicate comparison, 400 deals | +10.05 ± 1.76 per deal |
| trick-3 search over 20 random positions (`TestSearchTrick3StaysUnderTheDeadline`) | mean about 40 ms, max about 150 ms; gate: mean under 250 ms, max under 500 ms |

The two simulation gates run only when `BELJOT_BOT_SIM=1` is set (CI sets it; `make test` skips
them to stay fast), take about 9 s together, and need no database. The deadline test always runs.
A deal that passes out in bidding scores 0 in the duplicate comparison, so the per-deal figures
include passed-out deals.

### 5.9 What the search does not model

- **No bidding or play inference beyond the hard facts of 5.2.** Deals are not weighed by how the
  seats bid, led, discarded or signalled. The lab measured the bidding-consistency weighting at
  −1.9 ± 1.9 and left it out (`BOT_V2_LAB_RESULTS.md` §2.6).
- **Open-hand solving of each sample.** Every sampled deal is solved as if all four players saw
  all cards, for the opponents too. A card that is best only because the seat would know the
  exact deal later (strategy fusion) can be overrated, and a card that hides information is not
  valued.
- **No partner conventions** and no objective other than the settled hand score (no match-score
  or race awareness; the stop-at-target hands within reach go to the rules, 5.1).
- **No bidding, declaration, Belote or surrender decisions**, and no search on tricks 1 and 2.

---

## 6. Scenario B: Card play

Entry: `chooseCard`. Decision tree:

```
1. Only one legal card?            -> play it (forced).
2. Endgame retention (dix de der)? -> retainLastTrickWinner (hold a guaranteed trick-8 winner
   for +10, unless the trump-led trick in front of us is worth more than the bonus).
3. Trick is empty (leading)?       -> chooseLead.
4. Trick in progress (following)?  -> chooseFollow.
```

Everything operates only on the legal-card set. The bot never reasons about legality: the
engine already filtered (follow suit, must-overplay / iber, must-cut, over-trump).

### B1. Endgame retention "dix de der" (+10)

Fires only at the second-to-last trick (exactly 2 cards in hand). The forced 8th trick goes to
whoever retains the best card; its winner gets +10. The bot must hold the **master trump**
(no opponent can beat it) — a guaranteed trick-8 winner under **any** lead: it wins whether led
or forced in as a follow. It normally spends its other card now and banks the master for trick 8.
Defers (back to normal heuristics) unless all hold: exactly 2 cards left; the bot's best trump
is the master; the card it would spend is legal this trick; and the partner is not already known
to hold a higher trump (do not fight the partner).

Example: trump ♠, hand J♠ + 7♥, all other trumps gone -> play 7♥ now, keep J♠ for trick 8.

**Why retention does NOT look at the trick in front of it — including a fat trump-led pot.**
Retention runs **before** `chooseFollow` (see `chooseCard`), so at exactly two cards it, not
`trumpEconomyTake`, chooses the card. On a trump lead the overplay rule forces a trump out, so
spending the spare can hand the current trick to the opponents while the bot sits on the master.
That *looks* like the trump-lead leak fixed below, and a guard to "contest" a fat trick-7 pot was
tried — and measured **worse at every threshold**.

The reason is that the master takes exactly **one** trick either way, so it should take the trick
that is worth more — and trick 8 carries the **+10** plus the opponents' last cards:

| Trick 7, trump ♥, hand J♥ T♥, partner led Q♥, opponent 7♥, seat 1 holds 9♥ A♥ | Trick 7 | Final |
|---|---|---|
| **Retain** (spend T♥, J♥ takes trick 8 + the +10) | 0 / 24 | **44 / 24** |
| Contest (spend J♥ now, T♥ loses trick 8 + the +10) | 34 / 0 | 34 / 34 |

Conceding 24 on trick 7 is repaid with interest on trick 8. Wider measurement agreed: double-dummy
over 5,374 two-card information sets lost **2.54 points per changed decision**, minimax over 200k
fully legal hands was 11 better / 23 worse, and sweeping the threshold showed *never contesting*
scored best. `TestDecide_TrickSevenRetentionBeatsContesting` pins this — **do not add a contest
guard here.** Known gaps that a future change might legitimately address (capot, leading trick 7,
the non-trump over-ruff) are logged in `deferred-work.md`.

analogue — a spare that gets over-ruffed — is logged in `deferred-work.md`, not fixed here.

**Generalization — an uncuttable non-trump boss is also a guaranteed trick-8 winner**
(`isUncuttableBoss`): a non-trump card that is the boss of its suit **and** cannot be ruffed
(the opponents are out of trump, so no one can cut it — `opponentsOutOfTrump`). Such a card wins
trick 8 **only when the bot leads trick 8**. So when the bot holds the master trump **plus** an
uncuttable boss, both cards win the last trick, and the bot spends the **master trump first**
(leading the trump wins trick 7 and hands the bot the lead into trick 8, where the boss then
wins) and keeps the boss for last. The outcome is identical to keeping the master, but the trump
is led first and the side winner is held in reserve.

Example: trump ♠, hand J♠ + A♥, opponents out of trump -> lead J♠ now (win trick 7), keep A♥ for
trick 8 (it wins uncut). Previously the bot led A♥ first and kept J♠.

**Why the non-master case is *not* re-ordered**: if the bot holds a *lower* trump plus an
uncuttable boss, spending that trump first would only hand the lead — and the master trump — to
the partner, who then leads an unknown card into trick 8. Cashing the boss now is at least as
good: the partner still controls trick 8 either way, so the +10 stays with the team. Only the
master-trump holder gains from re-ordering. This case therefore defers, exactly as before.

### B2. Leading a trick (`chooseLead`)

In priority order:

1. **Draw trumps with the master**: lead the highest trump, but only if all four hold: my
   team called trump, trumps still sit with opponents, **I still hold the master trump**, and
   the **opponents are not already out of trump** (`opponentsOutOfTrump`) — see the note below.

2. **Draw trumps for the partner** (`partnerDrawTrump`): when the **partner** called trump and
   I do **not** hold the master, still lead a trump to strip opponents. The partner is assumed
   to hold the top trumps (J/9) and overtrumps to win. Card chosen, in order:
   **Q, then K, then T, then A** (sacrifice the weakest honor first, keep the stronger ones);
   if no honor, the **lowest trump that is not the 9** (7/8). **Never lead the 9** (a
   near-master kept as a winner, so the partner's Jack is not stripped by our own draw). The
   Jack never appears here (holding it makes the bot the master, handled by step 1).

   Skipped if: the partner is void in trump (no overtrump to set up), a **known** opponent
   holds a trump above the bot's best (the "partner has the top" assumption is disproven), the
   only trump available to lead would be the 9 (then the bot does not draw), the **opponents
   are already out of trump** (see the note below), or the **opponents provably hold the master
   trump** (`opponentHoldsMasterTrump`; see "Declaration-inference draw abort" below).

   > **Opponents out of trump -> do not draw, let the partner lead** (`opponentsOutOfTrump`):
   > once **both** opponents are known void in trump (or every trump is otherwise accounted
   > for), the remaining trumps sit only with the bot and its partner. Leading a trump then
   > "draws" nothing — it only strips the partner's own trump control — so both draw steps are
   > skipped and the bot leads a side suit instead (cash a boss, feed a ruff, or lead safe),
   > handing the lead to the partner who holds the trump control. This matters because
   > `trumpsRemainUnseen` cannot tell the **partner's unknown** trumps from an opponent's, so on
   > its own it would still report trumps outstanding and keep the bot drawing at its own partner.
   > Example: the partner leads trump twice, the opponents cannot follow the second time (marking
   > them void), the bot wins with a bigger trump — now on lead, the bot leads a **side suit**,
   > not another trump.

   | Trumps in hand (partner called) | Leads | Why |
   |---|---|---|
   | 9♥ 8♥ | 8♥ | no honor, lead low, keep the 9 |
   | Q♥ K♥ | Q♥ | weakest honor first |
   | T♥ A♥ | T♥ | T before A in the order |
   | 9♥ A♥ | A♥ | A is an honor; the 9 is kept |
   | only 9♥ | does not draw | never lead a lone 9; move to the next step |

   > **Declaration-inference draw abort -> do not draw into the opponents' master**
   > (`opponentHoldsMasterTrump`): the "partner holds the top trumps" assumption above is an
   > optimistic default. It can be disproven not only by a known opponent holding a higher
   > trump, but by a **negative** inference from the partner's own declaration
   > (`cardsProvablyAbsent`): a revealed run is MAXIMAL (consecutive in natural rank order), so
   > the rank immediately above its top and immediately below its bottom cannot be in that
   > hand — either would have extended the run. The inference holds for every seat and every
   > suit; `opponentHoldsMasterTrump` reads it for the partner's trump suit. When the top
   > OUTSTANDING trump (not played, not in the bot's own hand) is provably absent from the
   > partner this way, or is already known to sit with an opponent, drawing would only feed a
   > trick to that master — the bot aborts the draw entirely and falls through to cashing a
   > boss, feeding a partner ruff, or leading safe.
   >
   > Example (declaration inference): trump ♥, partner called and declared the tierce
   > 8♥ 9♥ T♥. The run is maximal, so the partner cannot hold J♥ (it would have shown
   > J-T-9-8); the bot lacks it too, so an opponent holds the master. The bot does NOT
   > draw — it cashes a side boss / leads safe instead of donating a trick to the J♥.

3. **Cash a side-suit boss**: a non-trump card no opponent can beat. Prefers the highest-value
   boss (Aces first).

   The Ace is cashed first; the Ten follows on a later lead only while it is still the boss
   of its suit.

4. **Lead into the partner's void** (`leadIntoPartnerVoid`): with no boss to cash, if the
   partner is known void in a side suit (and not known void in trump, so it can still ruff),
   lead the lowest card of that suit so the partner ruffs and wins. With several void suits,
   the cheapest sacrifice is chosen.

5. **Lead safe**: no boss and no ruff to feed -> lead the lowest-value non-trump (a 0-point
   7/8/9 when held). Never burn a trump here.

6. **Only trumps left** -> lead the highest trump **only if it is the master** (a winner worth
   cashing); otherwise lead the **lowest** (weakest) trump. Leading a high non-master trump just
   donates it to whoever holds the master above it, so the stronger trumps are kept back.
   Example: only trumps left, hand 9♥ 8♥ 7♥ with the J♥ still out -> lead **7♥** (9♥ is not the
   master); but hand J♥ 9♥ 7♥ -> lead **J♥** (the master).

Example (step 2): trump ♥, partner called. Hand Q♥ K♥ A♠ 7♣ -> lead **Q♥** (weakest honor),
keep the K; the partner overtrumps with J/9.

Example (step 4): trump ♥, opponent called. Hand 7♦ K♦ 9♣, partner void in ♦ -> lead **7♦**
so the partner ruffs.

### B3. Following a trick (`chooseFollow`)

First it computes who currently wins. Branches:

**My partner currently wins:**

- If the win is **safe** (no opponent can still beat it): **smear** points onto the guaranteed
  trick via `bestSmear` — the highest-point non-overtaking card that is **boss-safe** (see "The
  boss-preserving smear" below). If every legal card would overtake (e.g. a forced over-play),
  play the **strongest** card, unless it is the boss/master of its suit, then play the
  **second strongest** and keep the boss for later (`strongestPreservingBoss`). **Ace + Ten
  exception**: if that boss is a non-trump **Ace** and the bot **also holds that suit's Ten**,
  smear the **Ace** instead — spending it surrenders no control (the Ten becomes the suit's new
  boss), so the higher points are banked now onto the already-won trick. Example: partner safely
  won, bot forced over holding A♠ + T♠ -> play **A♠** (the Ten stays as the new boss).
- If the win is **not provably safe**, but the partner holds the **boss of a non-trump led
  suit** (only a ruff could beat it) and the one remaining opponent cannot certainly ruff:
  **risk-smear** high points anyway (`shouldSmearOntoPartnerBoss`), again picking the card via
  `bestSmear`. See the rule below.
- Otherwise: keep points home with the lowest value — with one exception, the **secure take**:
  when **every legal card overtakes** (a forced overplay or forced ruff leaves nothing to duck)
  **and** there are points at stake (trick points + the donation's own points > 0), picking the
  donation by points alone can hand the trick away. If some legal card **takes the trick beyond
  doubt** (`securelyWins`, below), the bot calls `trumpEconomyTake` (see "Trump economy on the
  take" below) to secure it while preserving the control trumps **J/9** on a ruff — on a **trump
  lead** there is no control to preserve, so it secures whenever the trick has points on it; this
  is the forced-overplay/forced-ruff shape of the same take used against a winning opponent; if
  nothing is provably secure, it donates the cheapest as before.

  Example (the classic leak this fixes): trump ♥, partner leads **Q♥**, opponent plays 8♥, bot
  third holds K♥ A♥ 9♥ J♥ with the **T♥ unseen**. The overplay rule forces all four; the old
  bot played K♥ (cheapest) and the T♥ holder took the trick. Now it plays **A♥** — the cheapest
  card the unseen Ten cannot beat, and not a control trump, so `trumpEconomyTake` takes it
  immediately — keeping 9♥ and J♥ as masters.

**The "secure winner" concept (`securelyWins`)** — a card takes the trick beyond doubt when it
overtakes the current winner AND either the bot closes the trick, or **no yet-to-play OPPONENT
could still play a card that beats it**. The check is per remaining opponent seat:

- cards that seat **provably holds** (known: revealed or located, minus already-played) count;
- **unseen** cards count only for suits that seat is **not known void in** (`KnownVoids`);
- a seat known (revealed or located) to still hold a **led-suit card must follow suit**
  (`seatKnownHoldsSuit`) — its trumps are unplayable this trick and do not veto security;
- the partner is never a threat, and neither are cards pinned to a seat that already acted.

So: forced over the partner's Q♥ holding K♥ A♥ J♥ with the one remaining opponent **known void
in trump**, the bot plays **K♥** — it is already guaranteed; the Jack is not burnt.

**The material-stake gate** — the secure upgrade only fires when securing wins something:
current trick points plus the cheap take's own points must be **> 0**. On a pointless trick
(e.g. forced ruff of a 7♦ lead holding J♥ + 8♥) the bot ruffs **cheap** (8♥) and accepts the
gamble — the master is never burnt to secure nothing.

**Trump economy on the take** (`trumpEconomyTake`) — used at **both** secure-take call sites
above: the forced-overplay/forced-ruff take over a partner, and the "otherwise, take the
trick" case over a winning opponent. Once the material-stake gate has passed, the bot does not
simply grab the cheapest provably-secure card (`cheapestSecureWinning`) — **when ruffing a side
suit** it holds back the **control trumps**, the Jack and the 9 (`isControlTrump`), whenever a
cheaper path to the trick exists. On a **trump lead** there is no cheaper path worth anything (the
overplay rule spends a trump either way), so the reservation does not apply there — only a
points-on-the-table test does. It computes two candidates and chooses between them by pot size:

- `secure`: the cheapest legal card that provably `securelyWins` (may or may not be a control
  trump).
- On a **ruff** of a non-trump lead only: `preserve`, the highest-point non-control trump that
  overtakes the current winner and beats every **KNOWN** over-ruff threat
  (`knownSafeRuff` / `highestExpendableKnownSafeRuff`) — a revealed higher trump a yet-to-play
  opponent holds, or an unseen higher trump from a seat **known void** in the led suit that may
  still hold trump. A seat not yet proven void in the led suit is assumed to have to follow
  suit, so its unseen trumps are not counted here — a lighter bar than `securelyWins`, which
  treats any not-provably-void seat's unseen trump as a live threat. A card can therefore be
  "known safe" here while not being strictly `securelyWins`.

Then: (1) if `secure` is **not** a control trump, take it immediately — nothing to preserve.
**Note there is no points gate on this step**, which is why `retainLastTrickWinner` must decide
trick 7 for itself rather than routing through this function (see B1). (2) Else, on a **TRUMP
lead** with points on the table, take `secure` — see "No reservation on a trump lead" below.
(3) Else, when ruffing and `preserve` exists, bank `preserve` instead — **unless** the pot
already justifies spending the control trump (below), in which case the mathematically
guaranteed control-trump win is taken over the merely known-safe cheaper trump. (4) Else, if
`secure` is a control trump and the pot is worth **at least its own value**
(`trickPoints(v, trump) >= controlTrumpValue`: the Jack needs >= 20, the 9 needs >= 14), spend
it. (5) Otherwise `trumpEconomyTake` returns nil: the caller falls through to its cheap
take/duck and accepts the speculative over-ruff.

Step (3) is the only genuinely ruff-gated step. Steps (4) and (5) still run on a trump lead —
in fact step (5)'s `return nil` is what keeps the master back on a **pointless** trump-led trick,
since step (2) declines it and `0 >= 14` fails at step (4).

**No reservation on a trump lead** — when the led suit IS trump, the overplay rule forces a trump
out no matter what, so declining to secure does not *preserve* the control trump: it donates a
**cheaper trump** into a trick the opponents then take. `controlTrumpValue` prices a control trump
by its own card points, but on a win those points land in **our own** pile — the wrong quantity to
weigh. So the reservation is dropped and replaced by a weaker test: **are there points on the
table** (`trickPoints(v, trump) > 0`)?

That test is load-bearing, and the callers' material-stake gate does **not** substitute for it.
That gate is `trickPoints + cardPoints(fallback) > 0`, where the fallback is the *cheapest legal
card* — so a cheap-but-scoring fallback (a Queen or King, when no 0-point trump is legal) opens it
on a worthless trick. Verified: without `trickPoints > 0`, hand J♥ Q♥ over an 8♥ lead burns the
J♥ to win nothing.

Because step (2) fires on any non-zero pot, a Jack **can** be spent for a pot of 3 — that is
deliberate: the alternative on a trump lead is donating a cheaper trump plus the pot, not keeping
a card for free. This is why step (5)'s old blanket claim that the master is "never burnt for less
than it is worth" no longer holds as a statement about the whole function; it describes step (4).

Legal sets on a trump lead come in three shapes, and only the first can reach step (2):
strictly-higher trumps (every card overtakes); all held trumps with **none** overtaking; or —
when void in trump — the whole all-non-trump hand. In the latter two nothing can beat the
trump-led winner, so nothing `securelyWins`, `secure` is nil, and the step is inert.

Example (the reported leak): trump ♥, the bot is third; its partner led **Q♥** and an opponent
played 7♥ — **3** points showing. Forced above the Q♥ holding T♥ 9♥ J♥ with the **A♥ unseen**,
the old value gate refused the 9♥ (3 < 14) and ducked with the **T♥**, which the last opponent
took with the A♥ — **24 points donated**. The 9♥ is beaten only by the J♥ the bot itself holds,
so it now plays **9♥**; end-to-end it banks 28 instead of conceding 24.

Counter-example (the pot must be real): same trump lead, but seat 3 opened the **8♥** (0 pts) and
the bot holds only J♥ + Q♥. Both are forced above the 8♥ and the Q♥ loses to any of the four
unseen higher trumps — but there is nothing on the table to win, so the bot donates the **Q♥** and
keeps the master. Add points (a void opponent discarding the A♠ onto the same lead) and it spends
the **J♥** instead.

Example (bank the ace): trump ♥, opponent leads T♦ (10 pts), bot void in ♦ (must cut) holding
J♥ A♥ 7♣, early hand — legal cards are J♥ and A♥. No opponent is known void in ♦ or in trump,
so an unseen 9♥ could in principle over-ruff the A♥: only the J♥ satisfies `securelyWins`. But
the A♥ beats every KNOWN threat — nobody is revealed to hold a bigger trump, and nobody is
provably void in ♦ to make the unseen 9♥ a live threat — so the bot cuts with the **A♥**,
banking 11 points and keeping the J♥ master. Old behavior cut with the J♥.

Example (value-gated master): the bot is third; partner led A♠ (11), an opponent ruffed T♥
(10) — 21 points on the table. Forced to over-ruff with only J♥ and A♥ available, the A♥ is
again only known-safe (an unseen 9♥ is unresolved), so the sole `securelyWins` candidate is the
J♥. Since 21 >= 20 (the Jack's own value), the bot spends the master **J♥**. With only 10 on
the table (10 < 20) it would keep the J♥ and over-ruff with the **A♥** instead, accepting the
gamble.

- **Risk-smear detail** (`shouldSmearOntoPartnerBoss`): when the partner leads a non-trump
  **Ace** (or the current top of the suit), an opponent follows suit, and the bot sits third
  with one opponent left, the only thing that can beat the partner is a ruff from that last
  opponent. The bot smears its highest points, accepting the **unknown** ruff risk. It keeps
  points home only when that last opponent is **known void** in the led suit **and** could
  still hold a trump (a near-certain ruff). If that opponent is also out of trump (known void
  in trump, or **every trump is already accounted for**, via `opponentMayHoldTrump`), it cannot
  ruff, so the bot still smears. When void in led + void in trump, the smear comes from another
  suit; when void in led + holding trump, the engine forces a ruff (Bitola has no partner
  exemption) and there is nothing to smear.

  | Partner has the boss, bot follows third | Behavior |
  |---|---|
  | last opponent's void unknown | smear high points |
  | last opponent known void in led, could hold trump | keep points home (likely ruff) |
  | last opponent known void in led, but out of trump | smear (cannot ruff) |
  | every trump already played | smear (cannot ruff) |
  | partner winning a non-boss card | keep points home |

**An opponent currently wins:**

- If the partner is yet to play and is **guaranteed** to take the trick (`partnerTakesTrick`):
  smear via `bestSmear`; if every legal card overtakes (the card lands in the team's trick
  anyway), bank high while preserving a boss/master via `strongestPreservingBoss` — never dump
  the minimum into a guaranteed team trick.

  `partnerTakesTrick` covers two shapes now:
  - **Non-trump lead** (unchanged): the partner must be **provably void** in the led suit (so
    it is forced to ruff, not follow into a loss) and hold a trump beater no opponent-reachable
    card beats.
  - **Trump lead** (new): no void proof needed — the partner must follow trump, and the
    overplay rule **forces** its known threat-proof beater out. Example: opponent leads **K♥**,
    partner declared **J♥** and plays last; bot holds 9♥ + T♥ (both forced overplays). The J♥
    wins no matter what, so the bot dumps **T♥** (banks 10 into the partner's trick) and keeps
    the 9♥ — it never "secures" with the 14-point 9. A declared beater that is **not**
    threat-proof (say the partner declared only A♥ while J♥/9♥ are unseen) does NOT trigger
    this — the bot takes normally.
- **Last to play and can win by following the led non-trump suit**: bank the
  **highest-points** led-suit winner instead of the cheapest (`highestPointsLedSuitWinner`).
  Banked into this trick (guaranteed as last player) it is safe; kept and led next trick it
  risks a ruff. Ruff wins (void in led) and trump-led tricks fall through unchanged.
- Otherwise: take the trick — with points at stake (trick + cheap winner > 0), the bot calls
  `trumpEconomyTake` (see "Trump economy on the take" below), which secures the trick while
  preserving the control trumps **J/9** whenever the pot doesn't justify spending one. After
  `trumpEconomyTake` declines (nothing is provably secure), or when the trick is pointless, the
  led-suit-secure take below applies; only after that does the bot take **as cheaply as
  possible** (`cheapestWinning`).
- **Plain suit led, an opponent still to play:** before falling back to the cheapest take, the
  bot takes with the lowest-point led-suit card that overtakes and that **no unseen led-suit
  card, nor one an opponent is known to hold,** outranks (`cheapestLedSecureWinner`). A ruff
  would beat the Ace too, so unseen trumps are not an argument for a lower card. The rule stands
  down when an opponent still to play is known void in the led suit, because a ruff is then
  likely and the cheap card loses less. Example: the partner leads 7♠, the right-hand opponent
  plays 8♠, the bot is third holding A♠ Q♠ with the K♠ and T♠ unseen and the left-hand opponent
  last -> play **A♠**; the Queen would lose to the last player's King or Ten and the kept Ace would
  be ruffed later. Once the K♠ and T♠ are gone the Queen suffices. A trump cut, or a trump lead,
  leaves it inert.
- Cannot win: **discard** the lowest value, preserving trump.

**The boss-preserving smear (`bestSmear` + `bossWorthGuarding`)** — used at all three smear
sites (safe smear, risk-smear, partner-takes-trick). Among non-overtaking legal cards it picks
the highest-point card that is **boss-safe**; on a points tie it smears the **weaker** rank
(keep the stronger card). A card is NOT boss-safe when it is the boss/master of its suit
(`cardIsBoss`) **without a backup** — another held card of the same suit that is also a boss
once it leaves (`heldSameSuitBoss`; the Ace backed by the Ten, generalized to any promoted
top). An unprotected boss is kept home and the best non-boss card is smeared instead. Two
exceptions un-guard a non-trump boss whose "control" is illusory:

- **Endgame** (2 cards in hand): if the opponents are **not provably out of trump**, a hoarded
  boss just dies to the trick-8 ruff — smear it now and bank the points. Only a provably
  **uncuttable** boss is kept for trick 8.
- **Dead suit**: an opponent **known void** in the boss's suit that **may still hold a trump**
  (`opponentMayHoldTrump`) means any lead of that suit gets ruffed — the boss can never cash,
  so it is banked, not hoarded.

Trump **masters** are always guarded (nothing ruffs a trump). When **every** candidate is an
unprotected boss (one must be given), the bot smears the **highest-point** one — the plain
old behavior, by explicit product decision.

**Ten under the Ace.** When the chosen smear is a non-trump Ace and the Ten of the same suit is
legal and does not overtake, the bot discards the **Ten** instead and keeps the Ace. That is the
table convention: it tells a human partner "I hold the Ace of this suit, return it". The cost is
one point on the partner's trick (the Ten is worth 10, the Ace 11; the lab measured at
most 0.02 points per hand), and the Ace stays home as the suit's boss. Without the Ten the Ace
rules above apply unchanged. The swap applies only while the Ace can still cash: when an opponent
can ruff the suit, or in the two-card endgame with a live opponent trump (`bossWorthGuarding`),
the Ace is smeared for its 11 points and the Ten kept.

Example (boss-guard): partner trumped and safely won, bot last holding **A♠ Q♠ 7♠** (no T♠) ->
smear **Q♠** and keep the Ace as the master of spades. Old behavior threw the Ace.

Example (protected boss): same trick, holding **A♠ + T♠** -> smear **T♠** (Ten under the Ace;
the Ace stays boss and shows the partner it is held).

Example (endgame exception): trick 7, hand **A♠ + 8♣**, partner safely won, an opponent may
still hold a trump -> smear **A♠** (bank 11; keeping it donates it to the trick-8 ruff). With
both opponents provably out of trump -> smear 8♣ and keep A♠ to win trick 8.

Example (smear, trump master kept): trump ♥, partner's **J♥** ruff safely won the closed trick,
bot void in the led suit holding 9♥ Q♥ 8♥ (forced to cut) -> smear **Q♥**; the 9♥ — the master
once the Jack cashes — is kept. Holding 9♥ + A♥ instead (each protects the other): smear **9♥**,
the canonical 9-under-the-partner's-Jack.

Example (preserve boss, 5.1.1): partner wins with Q♠, bot forced over with K♠ and Q♠ (A♠/T♠
gone) -> K♠ is the boss, so play **Q♠** and keep K♠. (With A♠ + T♠ instead, the Ace + Ten
exception applies: play **A♠** and keep T♠ as the new boss.)

Example (risk-smear, Rule 8): partner leads A♠, opponent follows low, bot third with T♠ J♠ 7♠,
last opponent's void unknown -> play **J♠** (2 points smeared); the T♠ — promoted to boss once
the Ace cashes — is kept, and 7♠ stays for a later duck.

Example (bank highest as last, Rule 6): opponent wins with ♠K, bot is last holding A♠ and T♠ ->
play **A♠** now (banks 11 safely) instead of T♠ (which would later lead into a ruff).

---

## 7. What the bot knows: information model

The bot's intelligence is bounded by the View (`buildBotView`), redacted so the bot only sees
what a human in that seat could:

- **Own hand and legal cards** for this turn.
- **Cards played this hand and inferred voids** (from `Memory`). A player who does not follow
  the led suit is marked void in it (`ObservePlay`). A seat that plays neither the led suit nor
  a trump is void in trump as well: it would have had to cut. Voids are tracked for **all 4
  seats** (including the partner, used by the lead-into-void and smear rules).
- **Known cards per seat** (`KnownCards` = declared ∪ located). Declared cards
  (`DeclaredCards`) come from the public declaration reveal, and only the winning declaration
  team's cards; the run inference below reads these alone. Located cards are public too: the
  candidate card is with the caller (in both Bitola rounds), and a Belote announcement puts the
  King and Queen of trump with the announcer. This is the only "x-ray" and it is legal
  information.
- **Update points**: the match layer updates the memory on every state replacement — card
  plays, the trump pick, the Belote announcement and the declaration reveal, including when
  the declaration window times out — not only on player actions.
- **Scores, hand points, tricks won**: present in the View but **not used** by any rule-layer
  heuristic. The search reads the hand points, declaration points and tricks won to settle a
  sample, and the match scores to spot stop-at-target hands within reach (§5.1, §5.3).

Reasoning helpers (all pure, in `bot.go`):

- `unseenCards`: the full deck minus everything placed (played, own hand, current trick,
  known holdings).
- `threats`: unseen cards **plus** cards a known opponent holds. A partner's known card is
  never a threat. Most heuristics (`holdsMasterTrump`, `isSuitBoss`, `trumpsRemainUnseen`,
  `partnerWinIsSafe`, `shouldSmearOntoPartnerBoss`) scan `threats`, so once the partner has
  declared the high cards they leave the threat set and the bot stops fighting the partner.
- `securelyWins` goes **further** than the raw `threats` scan: it reasons **per yet-to-play
  opponent seat** — known holdings per seat, unseen cards filtered by that seat's known voids,
  and the follow-suit pin (`seatKnownHoldsSuit`: a seat known to hold the led suit cannot
  play its trumps this trick). Cards pinned to seats that already acted are not threats.
- `cardsProvablyAbsent(v, seat)` is a **negative** inference from the same reveal: a revealed
  run is MAXIMAL, so the rank just below its bottom and just above its top cannot be in that
  seat's hand — either would have extended the run. It holds for every seat and every suit.
  `opponentHoldsMasterTrump` consumes it for the partner's trump suit (plus a direct
  known-held check) to prove the top outstanding trump sits with an opponent even when no card
  has been directly revealed in an opponent's own hand, aborting the partner trump-draw in
  `chooseLead`.

What the search sampler consumes (§5.2): the same View and nothing more. It reads the cards played
and those in the current trick, the void matrix (including trump voids), the known cards per seat
(declared and located cards, which it puts into that seat's hand and removes from the unseen
pool), the run neighbours from `cardsProvablyAbsent` (which it forbids in that seat's hand) and
each seat's remaining card count. These are hard facts: a deal that contradicts one is never
drawn. Everything else is unknown, and the sampler treats it as uniform. Which unseen cards sit
with which seat is equally likely under every deal that fits the facts, so no conclusion is drawn
from how a seat bid or passed, what it led or discarded, or what it signalled, and the scores do
not shape the samples.

---

## 8. Timing and humanization (`bot_driver.go`)

No bearing on which card, only when:

- Game decisions: uniform random delay in `[botDelayMin, botDelayMax]`.
- Score-reveal acknowledgements: a single short beat.
- Generation/context guards ensure the bot never double-acts or acts on a stale decision point.
- Card plays from trick 3: the search job is queued when the think timer is armed, so the think
  delay is the time the search has to finish. A result that is ready and still valid when the
  timer fires is played, otherwise the rule layer plays (§5.5).

---

## 9. Tuning levers and blind spots

By impact:

1. `matchesBidTable`: bidding aggressiveness (the biggest dial).
2. `trumpSuitScore` + `trumpLengthBonus`: suit preference when several suits match or the pick is forced (length vs. points).
3. `chooseLead` priority: when to draw trumps (with the master or for the partner, and
   `opponentsOutOfTrump` stops the draw once opponents are void), cash a boss (Ace first),
   feed a partner ruff, or lead safe. The partner draw also aborts on
   `opponentHoldsMasterTrump` / `cardsProvablyAbsent` — a revealed run's maximal-run boundary
   that proves the opponents (not the partner) hold the top trump.
4. `chooseFollow`: smear, risk-smear, banking, and point management.
5. `securelyWins` + the material-stake gate (`trickPoints + cardPoints > 0`): when the bot
   pays up for a guaranteed take vs. contests cheaply. The `> 0` threshold is the dial.
   `trumpEconomyTake` layers control-trump economy on top: the control-trump set **{J, 9}**
   (`isControlTrump`), the known-over-ruff signal (`knownSafeRuff` /
   `highestExpendableKnownSafeRuff`) that lets it bank a cheaper trump instead, and the value
   gate `trickPoints(v, trump) >= controlTrumpValue` that decides whether a pot is fat enough to
   spend a control trump at all. Those two levers apply to **ruffs**; on a trump lead they are
   reachable only when `trickPoints == 0`, where both comparisons fail anyway. The trump-lead dial
   is `trickPoints(v, trump) > 0` — raise it to make the bot stingier with the J/9. It applies to
   3+ card hands only: at two cards `retainLastTrickWinner` decides first and has no such dial,
   deliberately (B1).
6. `bestSmear` / `bossWorthGuarding`: what counts as control worth hoarding (backup test,
   endgame and dead-suit exceptions, the all-unprotected fallback).
7. `partnerDrawTrump`: the Q/K/T/A order and the "never the 9" rule.
8. `retainLastTrickWinner` + `isUncuttableBoss`: capturing the +10 in the last trick.
9. The search: the `BELJOT_BOT_SEARCH_*` settings, `search.SamplesForTrick` and the early-stop
   constants (`searchEarlyStopAfter`, `searchEarlyStopSE`) trade playing strength from trick 3
   against CPU time (§5.4, §5.6). Change them only with the simulation gates and the duplicate
   comparison of §5.8.

Blind spots if you want a stronger bot. These belong to the rule layer, which from trick 3 plays
only as the fallback; the search's own limits are in §5.9. (The first four are logged in
`_bmad-output/implementation-artifacts/deferred-work.md` as a simulation-gated tuning pass.)

- **Addressed** — the stake gate used to have no cost/benefit weighing (any > 0 stake could
  spend a 20-point master to secure a 3-point trick). `trumpEconomyTake` now gates a control
  trump (J/9) spend on `trickPoints(v, trump) >= controlTrumpValue` (J needs >= 20, 9 needs
  >= 14); below that threshold it banks a cheaper known-safe trump (`knownSafeRuff`) or accepts
  the speculative over-ruff instead.
- **Addressed** — that value gate then over-applied on a **trump lead**, where the overplay rule
  spends a trump regardless: reserving the J/9 for a fatter pot donated a *cheaper* trump into a
  trick the opponents took (forced over a partner's Q♥ with 3 showing, the bot ducked the T♥ into
  the unseen A♥ — 24 points). It is now ruff-only, replaced on a trump lead by `trickPoints > 0`.
  The asymmetry is deliberate: on a **ruff** the bot still declines a control trump below the
  threshold, because there it genuinely keeps the card.
- **Investigated, deliberately NOT changed** — the same shape at **two cards** is decided by
  `retainLastTrickWinner`, which banks the master and spends the spare. Two attempts to make it
  "contest" a fat trump-led pot were both reverted: the master takes one trick either way, so it
  should take trick 8 with its +10, and measurement put every contest threshold below never
  contesting. See B1 and `TestDecide_TrickSevenRetentionBeatsContesting`.
- There is **no** general arbitration between spending a master mid-hand and last-trick control —
  the two sites simply do not interact, because retention short-circuits before `chooseFollow` at
  two cards. `retainLastTrickWinner` also ignores **capot** (a swept hand makes the last trick
  worth +100, not +10) and can lead a spare into an over-ruff; both are in `deferred-work.md`.
- The endgame boss-guard ignores **who leads trick 8** — a hoarded uncuttable boss converts
  for sure only when the bot itself leads it.
- `partnerWinIsSafe`'s trump-threat leg is still a **raw unseen scan** (not seat-aware like
  `securelyWins`), so the bot sometimes ducks a smear onto a provably safe partner ruff.
- No arbitration between a mid-hand secure-spend of the master trump and the trick-7 retention
  logic that prices last-trick control at +10.
- `TeamScores` / `HandPoints` / `TricksWon` are in the View but **never consulted by the rule
  layer**: no "we are behind, play aggressive" or "the contract is secured, coast" logic. The
  search uses the hand points and tricks won only to score a sample, not to change its objective.
- No signalling or inference from the **partner's** discards (only voids are used).
- No forced pick in Bitola round 2 (weak hands just reshuffle).
- Belote, declarations, and accepting surrender are unconditional (correct, nothing to tune).

---

## 10. Where to change things (quick map)

| Behavior | Function | File |
|---|---|---|
| Bidding aggressiveness | `matchesBidTable` | `server/internal/bot/bidding.go` |
| Suit preference when several suits match or the pick is forced | `trumpSuitScore`, `trumpLengthBonus` | `server/internal/bot/bidding.go` |
| Draw trumps for partner (Q/K/T/A, never 9) | `partnerDrawTrump` + block in `chooseLead` | `server/internal/bot/bot.go` |
| Don't draw into the opponents' master (declaration inference) | `opponentHoldsMasterTrump`, `cardsProvablyAbsent` + block in `chooseLead` | `server/internal/bot/bot.go` |
| Stop drawing once opponents are void of trump | `opponentsOutOfTrump` + blocks in `chooseLead` | `server/internal/bot/bot.go` |
| Cash a side boss (Ace first; the Ten follows on a later lead) | boss block in `chooseLead` | `server/internal/bot/bot.go` |
| Only trumps left: highest if master, else lowest | `chooseLead` only-trumps branch, `isTrumpMaster` | `server/internal/bot/bot.go` |
| Preserve the boss on a forced overtake (Ace/Ten: smear the Ace) | `strongestPreservingBoss`, `findRankOfSuit` | `server/internal/bot/bot.go` |
| Secure take: cheapest guaranteed winner when points are at stake | `securelyWins`, `cheapestSecureWinning`, `trickPoints`, `seatKnownHoldsSuit` | `server/internal/bot/bot.go` |
| Preserve control trumps on the take (bank the ace, value-gate the master) — **ruff-only** | `trumpEconomyTake`, `isControlTrump`, `controlTrumpValue`, `knownSafeRuff`, `highestExpendableKnownSafeRuff` | `server/internal/bot/bot.go` |
| Secure the take on a trump lead when points are on the table | `trumpEconomyTake` (trump-lead branch), `trickPoints` | `server/internal/bot/bot.go` |
| Trick 7: bank the master unconditionally (do NOT add a contest guard) | `retainLastTrickWinner`, `TestDecide_TrickSevenRetentionBeatsContesting` | `server/internal/bot/bot.go` |
| Boss-preserving smear (backup test, endgame/dead-suit exceptions) | `bestSmear`, `bossWorthGuarding`, `heldSameSuitBoss` | `server/internal/bot/bot.go` |
| Ten under the Ace on the partner's safe trick | `bestSmear` (the Ten preference), `tenUnderAce`, `findRankOfSuit` | `server/internal/bot/bot.go` |
| Never fight a partner forced to win a trump-led trick | `partnerTakesTrick` (trump-led branch) | `server/internal/bot/bot.go` |
| Bank the high card as last player | `highestPointsLedSuitWinner` + `chooseFollow` | `server/internal/bot/bot.go` |
| Take with the cheapest card no led-suit card can beat | `cheapestLedSecureWinner` + `chooseFollow` | `server/internal/bot/bot.go` |
| Lead into the partner's void | `leadIntoPartnerVoid` | `server/internal/bot/bot.go` |
| Smear onto the partner's boss (risk) | `shouldSmearOntoPartnerBoss`, `opponentMayHoldTrump` | `server/internal/bot/bot.go` |
| Endgame +10 retention (master trump or uncuttable boss) | `retainLastTrickWinner`, `isUncuttableBoss` | `server/internal/bot/bot.go` |
| When the search plays (trick, turn, pending actions, stop-at-target reach) | `SearchApplies`, `withinStopAtTargetReach` | `server/internal/bot/search_bridge.go` |
| What the sampler may know (View to information) | `informationFrom`, `cannotHold`, `cardsHeldBy` | `server/internal/bot/search_bridge.go` |
| Draw deals consistent with the information; the seed | `Sample`, `Information.Seed` | `server/internal/bot/search/sampler.go` |
| Solve a deal exactly | `Solver.RootValues` | `server/internal/bot/search/solver.go` |
| Settle a sample's remainder as the engine scores a hand | `Settle` | `server/internal/bot/search/settle.go` |
| Samples per trick, the average, the early stop, the deadline check | `SamplesForTrick`, `Choose`, `searchEarlyStopAfter`, `searchEarlyStopSE` | `server/internal/bot/search/pimc.go`, `search_bridge.go` |
| Queue the search, store and consume its result, fall back to the rules | `submitBotSearch`, `runBotSearch`, `takeBotSearchResult`, `DecideWith` | `server/internal/match/bot_driver.go`, `server/internal/bot/search_bridge.go` |
| Pool size, deadline, first searched trick | `BotSearchSettings`, `DefaultBotSearchSettings`, `Manager.SetBotSearch` | `server/internal/match/search_pool.go`, `live_match.go`, `server/internal/config/config.go` |
| Unit tests for all of the above | `TestDecide_*` | `server/internal/bot/bot_test.go` |
| Strength check (bot vs. random) | `TestSimulation_*` | `server/internal/bot/simulation_test.go` |
| Search strength and time gates | `TestSimulation_Search*` (need `BELJOT_BOT_SIM=1`), `TestSearchTrick3StaysUnderTheDeadline` | `server/internal/bot/simulation_test.go`, `server/internal/bot/search/bench_test.go` |
