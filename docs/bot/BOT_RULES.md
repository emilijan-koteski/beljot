# Bot decision rules (Beljot, Bitola variant)

This document describes exactly how the bot decides, across every scenario, what it
considers, and with examples. It reflects the current logic including the latest tuning
(candidate-aware bidding, unbacked side-Ace and two-trump Jack+9 bids, partner trump-draw,
boss preservation, uncuttable-boss last-trick retention, Ace/Ten boss handling, suppressing
trump draws once the opponents are void of trump, leading into a partner's void, smearing
onto a partner's boss, seat-aware **secure trick takes** with a material-stake gate, the
**boss-preserving smear** with its endgame/dead-suit exceptions, and never fighting a partner
who is **forced to win a trump-led trick**).

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
  side Ace for rules 4.3 / 4.4.

Example: hand J♥ 9♥, candidate A♥ -> 3 trumps (J, 9, A) -> take.
Example: hand J♥ + side A♣, candidate 9♥ -> 2 trumps (J, 9) + a side Ace -> take (rule 4.4).

### The hand-strength gate: `wantsTrump`

A single yes/no evaluator used in both rounds. For a given suit, the bot wants it as trump if:

- it holds **4 or more** cards of that suit -> yes (unconditional), or
- it holds **exactly 3** including the **Jack** with **neither a 7 nor an 8** among them (the
  other two trumps both 9 or higher); or exactly 3 including the **Jack** with a 7 or 8 among
  them **plus a side Ace** (the outside winner rescues the weaker holding), or
- it holds **exactly 3** with the **9 + Ace** pair, and also holds a **side Ace** (any Ace in
  another suit; it need not be backed by a second card of its suit), or
- it holds **exactly 2** trumps that are the **Jack and the 9** (the two strongest trumps),
  and also holds a **side Ace**, or
- otherwise: no.

#### 4.1 Four or more trumps -> unconditional take.

#### 4.2 Three trumps including the Jack (tightened, with a side-Ace rescue)

Take if the other two trumps are both 9 or higher. If a 7 **or** an 8 is among the three the
hand is weaker — but it **still calls when the hand holds a side Ace** (an Ace in another suit,
an outside winner). With a 7/8 and no side Ace, pass.

| 3 trumps (incl. candidate) | Side Ace? | Take? | Why |
|---|---|---|---|
| J♥ 9♥ A♥ | — | yes | Jack, other two (9, A) are above 8 |
| J♥ K♥ Q♥ | — | yes | Jack, K and Q are above 8 |
| J♥ 9♥ 7♥ | no | no | contains a 7, no outside Ace |
| J♥ 8♥ K♥ | no | no | contains an 8, no outside Ace |
| J♥ 7♥ 8♥ | no | no | contains both 7 and 8, no outside Ace |
| J♥ 8♥ 7♥ + A♠ | yes | yes | weak three, but the side A♠ rescues it |

#### 4.3 Pair 9 + Ace with a side Ace

The 9 + Ace trump pair calls with any **side Ace** — an Ace in another suit. The Ace does
**not** need a second card of its suit to back it: a lone side Ace is enough. Logic:
`hasSideAce`.

| Hand (incl. candidate) | Take? | Why |
|---|---|---|
| 9♥ A♥ Q♥ + A♣ T♣ | yes | pair 9+A, side A♣ |
| 9♥ A♥ Q♥ + A♣ K♦ | yes | a lone side A♣ is enough (no backing needed) |
| 9♥ A♥ Q♥ + K♣ K♦ | no | no side Ace at all |

#### 4.4 Two trumps: the Jack and the 9 with a side Ace

Exactly two trumps call **only** when they are the **Jack and the 9** — the two strongest
trumps (J=20, 9=14) — and the hand also holds a **side Ace**. Any other two-trump holding
passes. Logic: the `count == 2` branch in `wantsTrump` (`hasJack && hasNine && hasSideAce`).

| Hand (incl. candidate) | Take? | Why |
|---|---|---|
| J♥ 9♥ + A♣ | yes | the two strongest trumps, backed by a side Ace |
| J♥ 9♥ | no | no side Ace |
| J♥ A♥ + A♣ | no | two trumps but not the Jack **and** the 9 |
| J♥ T♥ + A♣ | no | the second trump is the Ten, not the 9 |

Note: this rule targets **exactly two** trumps; adding a third trump makes it a *three*-trump
hand judged by rule 4.2. With the side-Ace rescue in 4.2, the two rules now agree — J♥ 9♥ +
side Ace calls (4.4), and so does J♥ 9♥ 7♥ + side Ace (4.2). A Jack plus a side Ace is enough
whether the hand holds two or three trumps.

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
2. Endgame retention (dix de der)? -> retainLastTrickWinner (hold a guaranteed trick-8 winner
   for +10).
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
   only trump available to lead would be the 9 (then the bot does not draw), or the **opponents
   are already out of trump** (see the note below).

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

3. **Cash a side-suit boss**: a non-trump card no opponent can beat. Prefers the highest-value
   boss (Aces first).

   **Ace + Ten exception**: when the chosen boss is the **Ace** and the bot **also holds that
   suit's Ten**, it cashes the **Ten** first (the lower one) and keeps the Ace. Holding the Ace
   makes the Ten a boss too, so the Ten still wins the trick, and the Ace stays back as the
   suit's guaranteed master. This applies **only** to the Ace + Ten pair — not to other touching
   honors (e.g. holding K + Q, it still cashes the King, the general highest-value rule).
   Example: opponents called, hand A♠ T♠ 7♣ -> lead **T♠**, keep A♠.

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
  doubt** (`securelyWins`, below), the bot secures it with the **cheapest such card**; if
  nothing is provably secure, it donates the cheapest as before.

  Example (the classic leak this fixes): trump ♥, partner leads **Q♥**, opponent plays 8♥, bot
  third holds K♥ A♥ 9♥ J♥ with the **T♥ unseen**. The overplay rule forces all four; the old
  bot played K♥ (cheapest) and the T♥ holder took the trick. Now it plays **A♥** — the cheapest
  card the unseen Ten cannot beat — keeping 9♥ and J♥ as masters.

**The "secure winner" concept (`securelyWins`)** — a card takes the trick beyond doubt when it
overtakes the current winner AND either the bot closes the trick, or **no yet-to-play OPPONENT
could still play a card that beats it**. The check is per remaining opponent seat:

- cards that seat **provably holds** (declaration reveal, minus already-played) count;
- **unseen** cards count only for suits that seat is **not known void in** (`KnownVoids`);
- a seat revealed to still hold a **led-suit card must follow suit** (`seatKnownHoldsSuit`) —
  its trumps are unplayable this trick and do not veto security;
- the partner is never a threat, and neither are cards pinned to a seat that already acted.

So: forced over the partner's Q♥ holding K♥ A♥ J♥ with the one remaining opponent **known void
in trump**, the bot plays **K♥** — it is already guaranteed; the Jack is not burnt.

**The material-stake gate** — the secure upgrade only fires when securing wins something:
current trick points plus the cheap take's own points must be **> 0**. On a pointless trick
(e.g. forced ruff of a 7♦ lead holding J♥ + 8♥) the bot ruffs **cheap** (8♥) and accepts the
gamble — the master is never burnt to secure nothing.

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
- Otherwise: take the trick — with points at stake (trick + cheap winner > 0), prefer the
  **cheapest secure winner** (`cheapestSecureWinning`); if nothing is provably secure, or the
  trick is pointless, take **as cheaply as possible** (`cheapestWinning`) as before.
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

Example (boss-guard): partner trumped and safely won, bot last holding **A♠ Q♠ 7♠** (no T♠) ->
smear **Q♠** and keep the Ace as the master of spades. Old behavior threw the Ace.

Example (protected boss): same trick, holding **A♠ + T♠** -> smear **A♠** (the Ten stays boss).

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
- `securelyWins` goes **further** than the raw `threats` scan: it reasons **per yet-to-play
  opponent seat** — known holdings per seat, unseen cards filtered by that seat's known voids,
  and the follow-suit pin (`seatKnownHoldsSuit`: a seat revealed to hold the led suit cannot
  play its trumps this trick). Cards pinned to seats that already acted are not threats.

---

## 7. Timing and humanization (`bot_driver.go`)

No bearing on which card, only when:

- Game decisions: uniform random delay in `[botDelayMin, botDelayMax]`.
- Score-reveal acknowledgements: a single short beat.
- Generation/context guards ensure the bot never double-acts or acts on a stale decision point.

---

## 8. Tuning levers and blind spots

By impact:

1. `wantsTrump` and `hasSideAce`: bidding aggressiveness (the biggest dial).
2. `trumpSuitScore` + `trumpLengthBonus`: round-2 suit preference (length vs. points).
3. `chooseLead` priority: when to draw trumps (with the master or for the partner, and
   `opponentsOutOfTrump` stops the draw once opponents are void), cash a boss (Ace/Ten handling),
   feed a partner ruff, or lead safe.
4. `chooseFollow`: smear, risk-smear, banking, and point management.
5. `securelyWins` + the material-stake gate (`trickPoints + cardPoints > 0`): when the bot
   pays up for a guaranteed take vs. contests cheaply. The `> 0` threshold is the dial.
6. `bestSmear` / `bossWorthGuarding`: what counts as control worth hoarding (backup test,
   endgame and dead-suit exceptions, the all-unprotected fallback).
7. `partnerDrawTrump`: the Q/K/T/A order and the "never the 9" rule.
8. `retainLastTrickWinner` + `isUncuttableBoss`: capturing the +10 in the last trick.

Blind spots if you want a stronger bot (the first four are logged in
`_bmad-output/implementation-artifacts/deferred-work.md` as a simulation-gated tuning pass):

- The stake gate has **no cost/benefit weighing** — any > 0 stake can spend a 20-point master
  to secure a 3-point trick.
- The endgame boss-guard ignores **who leads trick 8** — a hoarded uncuttable boss converts
  for sure only when the bot itself leads it.
- `partnerWinIsSafe`'s trump-threat leg is still a **raw unseen scan** (not seat-aware like
  `securelyWins`), so the bot sometimes ducks a smear onto a provably safe partner ruff.
- No arbitration between a mid-hand secure-spend of the master trump and the trick-7 retention
  logic that prices last-trick control at +10.
- `TeamScores` / `HandPoints` / `TricksWon` are in the View but **never consulted**: no
  "we are behind, play aggressive" or "the contract is secured, coast" logic.
- No signalling or inference from the **partner's** discards (only voids are used).
- No forced pick in round 2 (weak hands just reshuffle).
- Belote, declarations, and accepting surrender are unconditional (correct, nothing to tune).

---

## 9. Where to change things (quick map)

| Behavior | Function | File |
|---|---|---|
| Bidding aggressiveness | `wantsTrump`, `hasSideAce` | `server/internal/bot/bot.go` |
| Round-2 suit preference | `trumpSuitScore`, `trumpLengthBonus` | `server/internal/bot/bot.go` |
| Draw trumps for partner (Q/K/T/A, never 9) | `partnerDrawTrump` + block in `chooseLead` | `server/internal/bot/bot.go` |
| Stop drawing once opponents are void of trump | `opponentsOutOfTrump` + blocks in `chooseLead` | `server/internal/bot/bot.go` |
| Cash a side boss (Ace/Ten: cash the Ten, keep the Ace) | boss block in `chooseLead`, `findRankOfSuit` | `server/internal/bot/bot.go` |
| Only trumps left: highest if master, else lowest | `chooseLead` only-trumps branch, `isTrumpMaster` | `server/internal/bot/bot.go` |
| Preserve the boss on a forced overtake (Ace/Ten: smear the Ace) | `strongestPreservingBoss` | `server/internal/bot/bot.go` |
| Secure take: cheapest guaranteed winner when points are at stake | `securelyWins`, `cheapestSecureWinning`, `trickPoints`, `seatKnownHoldsSuit` | `server/internal/bot/bot.go` |
| Boss-preserving smear (backup test, endgame/dead-suit exceptions) | `bestSmear`, `bossWorthGuarding`, `heldSameSuitBoss` | `server/internal/bot/bot.go` |
| Never fight a partner forced to win a trump-led trick | `partnerTakesTrick` (trump-led branch) | `server/internal/bot/bot.go` |
| Bank the high card as last player | `highestPointsLedSuitWinner` + `chooseFollow` | `server/internal/bot/bot.go` |
| Lead into the partner's void | `leadIntoPartnerVoid` | `server/internal/bot/bot.go` |
| Smear onto the partner's boss (risk) | `shouldSmearOntoPartnerBoss`, `opponentMayHoldTrump` | `server/internal/bot/bot.go` |
| Endgame +10 retention (master trump or uncuttable boss) | `retainLastTrickWinner`, `isUncuttableBoss` | `server/internal/bot/bot.go` |
| Unit tests for all of the above | `TestDecide_*` | `server/internal/bot/bot_test.go` |
| Strength check (bot vs. random) | `TestSimulation_*` | `server/internal/bot/simulation_test.go` |
