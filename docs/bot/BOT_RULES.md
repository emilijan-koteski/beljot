# Bot decision rules (Beljot, Bitola variant)

This document describes exactly how the bot decides, across every scenario, what it
considers, and with examples. It reflects the current logic including the latest tuning
(candidate-aware bidding, partner trump-draw, boss preservation, last-trick banking,
leading into a partner's void, and smearing onto a partner's boss).

Suit notation: ♠ spades, ♥ hearts, ♦ diamonds, ♣ clubs. Ranks: A, K, Q, J, T (ten), 9, 8, 7.

---

## 1. Where the logic lives

| Concern | File | Notes |
|---|---|---|
| All decision logic (pure, deterministic) | `server/internal/bot/bot.go` | Almost all tuning goes here |
| What the bot is allowed to know | `server/internal/bot/view.go` | Redacted, seat-local snapshot |
| What the bot remembers | `server/internal/bot/memory.go` | Played cards, voids, revealed declarations |
| Timing, humanization, scheduling | `server/internal/match/bot_driver.go` | Think delay, when the bot acts |
| Card point and strength tables | `server/internal/game/types.go` | Tables every heuristic reads |
| Legality (the rules cage) | `server/internal/game/validation.go` | The bot only ever chooses among legal cards |
| Tests for all of the below | `server/internal/bot/bot_test.go`, `simulation_test.go` | Table tests + heuristic-vs-random simulation |

`Decide(View) game.Action` is **pure and deterministic**: there is no randomness in card
choice. The only randomness is the think delay in the match layer. Every decision below is
fully reproducible and table-testable.

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
5. **Otherwise** -> `chooseCard` (Scenario B).

Scenarios 1, 3, 4 are "always do it" with nothing to tune. The real intelligence is in
**bidding** and **card play**.

---

## 4. Scenario A: Bidding (choosing trump)

Bidding happens on the 5-card stage-1 hand. There are two rounds.

### 4.0 The revealed candidate card is counted

Whoever picks the trump **always receives the face-up candidate card** (the engine appends it
in `handlePickTrump`, both rounds). So the bot evaluates the hand it would actually hold after
picking: `hand + candidate` (built in `decideBid`).

- **Round 1**: the candidate is a trump (its suit is the trump suit), so it adds to the trump
  count. The 3-card rules now apply to **2 in hand + candidate**; the 4-card rule to
  **3 in hand + candidate**.
- **Round 2**: the candidate is a side card (its suit is locked out), so it can supply a
  backing side Ace for rule 4.3.

Example: hand J♥ 9♥, candidate A♥ -> 3 trumps (J, 9, A) -> take.

### The hand-strength gate: `wantsTrump`

A single yes/no evaluator used in both rounds. For a given suit, the bot wants it as trump if:

- it holds **4 or more** cards of that suit -> yes (unconditional), or
- it holds **exactly 3** including the **Jack**, and **neither a 7 nor an 8** is among them
  (the other two trumps must both be 9 or higher), or
- it holds **exactly 3** with the **9 + Ace** pair, and also holds a **backed side Ace**
  (an Ace in another suit that has at least one more card of that suit), or
- otherwise: no.

#### 4.1 Four or more trumps -> unconditional take.

#### 4.2 Three trumps including the Jack (tightened)

Take only if the other two trumps are both 9 or higher. If a 7 **or** an 8 is among the three,
pass.

| 3 trumps (incl. candidate) | Take? | Why |
|---|---|---|
| J♥ 9♥ A♥ | yes | Jack, other two (9, A) are above 8 |
| J♥ K♥ Q♥ | yes | Jack, K and Q are above 8 |
| J♥ 9♥ 7♥ | no | contains a 7 |
| J♥ 8♥ K♥ | no | contains an 8 |
| J♥ 7♥ 8♥ | no | contains both 7 and 8 |

#### 4.3 Pair 9 + Ace with a BACKED side Ace (tightened)

A lone side Ace is not enough; that Ace must have **at least one more card of its suit**
(so it is not a singleton an opponent immediately ruffs). Logic: `hasBackedSideAce`.

| Hand (incl. candidate) | Take? | Why |
|---|---|---|
| 9♥ A♥ Q♥ + A♣ T♣ | yes | pair 9+A, side A♣ backed by T♣ |
| 9♥ A♥ Q♥ + A♣ 7♣ | yes | any second card of the suit counts as backup |
| 9♥ A♥ Q♥ + A♣ K♦ | no | A♣ is a singleton (no other club) |

### Round 1 (`decideBid`)

Only one option: the candidate's suit. The bot calls
`wantsTrump(hand + candidate, candidate.Suit)` and picks or passes.

### Round 2

If all four passed in round 1, the candidate's suit is locked out. The bot evaluates **every
other suit** with the same gate, and among those that pass, picks the **highest-scoring** one
via `trumpSuitScore`:

```
score(suit) = sum over cards of that suit of ( trumpPoints[rank] + 10 )
```

The per-card `trumpLengthBonus` (currently 10) biases toward a longer suit over raw points.

### Bidding tuning levers

- The whole `wantsTrump` threshold logic: the biggest dial for aggressive vs. conservative.
  It currently considers no partner, score, or dealer position.
- `trumpLengthBonus` (currently 10): raise to favor length, lower to favor points.
- There is no forced pick in round 2: if no suit qualifies, all pass and the deck reshuffles.

---

## 5. Scenario B: Card play

Entry: `chooseCard`. Decision tree:

```
1. Only one legal card?            -> play it (forced).
2. Endgame retention (dix de der)? -> retainLastTrickWinner (hold the master for +10).
3. Trick is empty (leading)?       -> chooseLead.
4. Trick in progress (following)?  -> chooseFollow.
```

Everything operates only on the legal-card set. The bot never reasons about legality: the
engine already filtered (follow suit, must-overplay / iber, must-cut, over-trump).

### B1. Endgame retention "dix de der" (+10)

Fires only at the second-to-last trick (exactly 2 cards in hand). The forced 8th trick goes to
whoever retains the best card; its winner gets +10. If the bot holds the **master trump**
(no opponent can beat it), it spends its other card now and banks the master for trick 8.

Defers (back to normal heuristics) unless all hold: exactly 2 cards left; the bot's best trump
is the master; the spare is legal this trick; and the partner is not already known to hold a
higher trump (do not fight the partner).

Example: trump ♠, hand J♠ + 7♥, all other trumps gone -> play 7♥ now, keep J♠ for trick 8.

This rule is unchanged.

### B2. Leading a trick (`chooseLead`)

In priority order:

1. **Draw trumps with the master**: lead the highest trump, but only if all three hold: my
   team called trump, trumps still sit with opponents, and **I still hold the master trump**.

2. **Draw trumps for the partner** (`partnerDrawTrump`): when the **partner** called trump and
   I do **not** hold the master, still lead a trump to strip opponents. The partner is assumed
   to hold the top trumps (J/9) and overtrumps to win. Card chosen, in order:
   **Q, then K, then T, then A** (sacrifice the weakest honor first, keep the stronger ones);
   if no honor, the **lowest trump that is not the 9** (7/8). **Never lead the 9** (a
   near-master kept as a winner, so the partner's Jack is not stripped by our own draw). The
   Jack never appears here (holding it makes the bot the master, handled by step 1).

   Skipped if: the partner is void in trump (no overtrump to set up), a **known** opponent
   holds a trump above the bot's best (the "partner has the top" assumption is disproven), or
   the only trump available to lead would be the 9 (then the bot does not draw).

   | Trumps in hand (partner called) | Leads | Why |
   |---|---|---|
   | 9♥ 8♥ | 8♥ | no honor, lead low, keep the 9 |
   | Q♥ K♥ | Q♥ | weakest honor first |
   | T♥ A♥ | T♥ | T before A in the order |
   | 9♥ A♥ | A♥ | A is an honor; the 9 is kept |
   | only 9♥ | does not draw | never lead a lone 9; move to the next step |

3. **Cash a side-suit boss**: a non-trump card no opponent can beat. Prefers the highest-value
   boss (Aces first).

4. **Lead into the partner's void** (`leadIntoPartnerVoid`): with no boss to cash, if the
   partner is known void in a side suit (and not known void in trump, so it can still ruff),
   lead the lowest card of that suit so the partner ruffs and wins. With several void suits,
   the cheapest sacrifice is chosen.

5. **Lead safe**: no boss and no ruff to feed -> lead the lowest-value non-trump (a 0-point
   7/8/9 when held). Never burn a trump here.

6. **Only trumps left** -> lead the highest trump.

Example (step 2): trump ♥, partner called. Hand Q♥ K♥ A♠ 7♣ -> lead **Q♥** (weakest honor),
keep the K; the partner overtrumps with J/9.

Example (step 4): trump ♥, opponent called. Hand 7♦ K♦ 9♣, partner void in ♦ -> lead **7♦**
so the partner ruffs.

### B3. Following a trick (`chooseFollow`)

First it computes who currently wins. Branches:

**My partner currently wins:**

- If the win is **safe** (no opponent can still beat it): **smear** the highest-point card that
  does not overtake the partner (dump an Ace/Ten onto the guaranteed trick). If every legal
  card would overtake (e.g. a forced over-play), play the **strongest** card, unless it is the
  boss/master of its suit, then play the **second strongest** and keep the boss for later
  (`strongestPreservingBoss`).
- If the win is **not provably safe**, but the partner holds the **boss of a non-trump led
  suit** (only a ruff could beat it) and the one remaining opponent cannot certainly ruff:
  **risk-smear** the high points anyway (`shouldSmearOntoPartnerBoss`). See the rule below.
- Otherwise: keep points home with the lowest value.

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
  smear the highest-point non-overtaking card, else lowest value.
- **Last to play and can win by following the led non-trump suit**: bank the
  **highest-points** led-suit winner instead of the cheapest (`highestPointsLedSuitWinner`).
  Banked into this trick (guaranteed as last player) it is safe; kept and led next trick it
  risks a ruff. Ruff wins (void in led) and trump-led tricks fall through unchanged.
- Otherwise: take the trick **as cheaply as possible** (`cheapestWinning`).
- Cannot win: **discard** the lowest value, preserving trump.

Example (smear): trump ♥, partner led A♥ (safe). Hand ♥T ♥7 ♣K -> play **♥T** (10 points onto
the partner's trick), not ♥7.

Example (preserve boss, 5.1.1): partner wins with Q♠, bot forced over with A♠ and T♠ -> A♠ is
the boss, so play **T♠** and keep A♠.

Example (risk-smear, Rule 8): partner leads A♠, opponent follows low, bot third with T♠ and a
7♠, last opponent's void unknown -> play **T♠** (smear), not 7♠.

Example (bank highest as last, Rule 6): opponent wins with ♠K, bot is last holding A♠ and T♠ ->
play **A♠** now (banks 11 safely) instead of T♠ (which would later lead into a ruff).

---

## 6. What the bot knows: information model

The bot's intelligence is bounded by the View (`buildBotView`), redacted so the bot only sees
what a human in that seat could:

- **Own hand and legal cards** for this turn.
- **Cards played this hand and inferred voids** (from `Memory`). A player who does not follow
  the led suit is marked void in it (`ObservePlay`). Voids are tracked for **all 4 seats**
  (including the partner, used by the lead-into-void and smear rules).
- **Known cards per seat**: only from the public declaration reveal, and only the winning
  declaration team's cards. This is the only "x-ray" and it is legal information.
- **Scores, hand points, tricks won**: present in the View but currently **not used** by any
  heuristic.

Reasoning helpers (all pure, in `bot.go`):

- `unseenCards`: the full deck minus everything placed (played, own hand, current trick,
  known holdings).
- `threats`: unseen cards **plus** cards a known opponent holds. A partner's known card is
  never a threat. Most heuristics (`holdsMasterTrump`, `isSuitBoss`, `trumpsRemainUnseen`,
  `partnerWinIsSafe`, `shouldSmearOntoPartnerBoss`) scan `threats`, so once the partner has
  declared the high cards they leave the threat set and the bot stops fighting the partner.

---

## 7. Timing and humanization (`bot_driver.go`)

No bearing on which card, only when:

- Game decisions: uniform random delay in `[botDelayMin, botDelayMax]`.
- Score-reveal acknowledgements: a single short beat.
- Generation/context guards ensure the bot never double-acts or acts on a stale decision point.

---

## 8. Tuning levers and blind spots

By impact:

1. `wantsTrump` and `hasBackedSideAce`: bidding aggressiveness (the biggest dial).
2. `trumpSuitScore` + `trumpLengthBonus`: round-2 suit preference (length vs. points).
3. `chooseLead` priority: when to draw trumps (with the master or for the partner), cash a
   boss, feed a partner ruff, or lead safe.
4. `chooseFollow`: smear, risk-smear, banking, and point management.
5. `partnerDrawTrump`: the Q/K/T/A order and the "never the 9" rule.
6. `retainLastTrickWinner`: capturing the +10 in the last trick.

Blind spots if you want a stronger bot:

- `TeamScores` / `HandPoints` / `TricksWon` are in the View but **never consulted**: no
  "we are behind, play aggressive" or "the contract is secured, coast" logic.
- No signalling or inference from the **partner's** discards (only voids are used).
- No forced pick in round 2 (weak hands just reshuffle).
- Belote, declarations, and accepting surrender are unconditional (correct, nothing to tune).

---

## 9. Where to change things (quick map)

| Behavior | Function | File |
|---|---|---|
| Bidding aggressiveness | `wantsTrump`, `hasBackedSideAce` | `server/internal/bot/bot.go` |
| Round-2 suit preference | `trumpSuitScore`, `trumpLengthBonus` | `server/internal/bot/bot.go` |
| Draw trumps for partner (Q/K/T/A, never 9) | `partnerDrawTrump` + block in `chooseLead` | `server/internal/bot/bot.go` |
| Preserve the boss on a forced overtake | `strongestPreservingBoss` | `server/internal/bot/bot.go` |
| Bank the high card as last player | `highestPointsLedSuitWinner` + `chooseFollow` | `server/internal/bot/bot.go` |
| Lead into the partner's void | `leadIntoPartnerVoid` | `server/internal/bot/bot.go` |
| Smear onto the partner's boss (risk) | `shouldSmearOntoPartnerBoss`, `opponentMayHoldTrump` | `server/internal/bot/bot.go` |
| Endgame +10 retention | `retainLastTrickWinner` | `server/internal/bot/bot.go` |
| Unit tests for all of the above | `TestDecide_*` | `server/internal/bot/bot_test.go` |
| Strength check (bot vs. random) | `TestSimulation_*` | `server/internal/bot/simulation_test.go` |
