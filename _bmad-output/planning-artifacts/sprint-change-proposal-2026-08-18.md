# Sprint Change Proposal — Epic 12 Scope Correction & Card Deck Preference

**Date:** 2026-08-18
**Author:** Emilijan (via bmad-correct-course)
**Scope classification:** Moderate — planning-artifact-only change. Rewrite and split of one not-yet-started story, five new stories, three new FRs, one new binding design constraint, plus corrective edits to `project-context.md` and `architecture.md`. No implementation code is written by this proposal.

---

## Section 1 — Issue Summary

Epic 12 (Variant Expansion) was authored **2026-04-06**. Phase 3 is about to start, and a pre-flight audit of the epic against the shipped codebase found its central premise wrong.

**Story 12.1 claimed the Croatian variant is Bitola plus one bidding change:**

> **Given** a game is initialized with Croatian variant
> **When** the rules engine processes the bidding phase
> **Then** round 1 follows the same counter-clockwise PICK/PASS pattern as Bitola
> **And** in round 2, if the first 3 players pass, the last player (dealer) is FORCED to pick a trump suit — there is no reshuffle
>
> **Given** the Croatian variant is selected
> **When** card play and scoring proceed
> **Then** all other rules … are identical to Bitola variant

Both halves are false. A rules audit against a Croatian source (`tako.hr/clanci/kako-se-igra-bela`), **corrected line by line against the product owner's own knowledge of the game**, found **seven** divergences rather than one — and confirmed **thirty** other rules as genuinely identical.

The audit is recorded at `https://claude.ai/code/artifact/537ff7ff-90ab-403d-a322-eb72a20f3fe1` (revision 3, settled).

### The seven divergences

| # | Rule | Bitola (ships today) | Croatian |
|---|------|----------------------|----------|
| 1 | Deal shape | 3 + 2, candidate flipped, 11 cards held for post-pick distribution | 3 + 3 visible + 2 face-down; all eight dealt before bidding, no distribution stage |
| 2 | Trump candidate | Public flipped card; round 1 pick takes its suit and the card | None — trump is a bare named suit, freely chosen in both rounds; picker takes no card |
| 3 | Round 2 reveal | No reveal | Each player's two face-down cards turn up **to that player only** |
| 4 | All-pass outcome | Reshuffle + rotate dealer + re-deal | Dealer bids last in round 2 and **must** name a suit |
| 5 | Declaration overlap | One card, one group — `dedupBitola` drops the lower-value group | A card may count in more than one declaration |
| 6 | Declaration timing | During trick 1 per player; revealed at trick 2 | Dedicated phase between bidding and trick 1; revealed before trick 1 |
| 7 | Tied hand | *(must change)* → hanging points, carried over | All points to the taker's opponents — **already what ships** |

Divergence 7 is realized by changing **Bitola**, not Croatian. The interim stand-in documented in `deferred-work.md` and `project-context.md` was correct: the engine currently applies the Croatian tie rule to all variants on purpose.

### Evidence — verified in code at `3afec09`

- `server/internal/game/state.go:73` — `Variant` is stored on `GameState` but **never read**; there are zero variant-comparison sites in the engine today.
- `server/internal/game/state.go:254` `dealCards` — deals 3 then 2, flips a candidate, parks 11 cards in `gs.Deck`.
- `server/internal/game/bidding.go` `handlePickTrump` — hard-rejects with `ErrWrongPhase` when `TrumpCandidate == nil` or `len(Deck) != 11`; round 1 ignores `action.Suit` and uses the candidate's suit.
- `server/internal/game/declarations.go:94` — live `TODO(croatian-variant): skip dedup for the Croatian variant when added`.
- `server/internal/game/declarations.go:420,459` — declaration timing hardcoded to `TrickNumber == 1 / == 2`; there is no declaring phase in the `Phase` enum (`types.go:84-96`).
- `server/internal/game/scoring.go` `scoreHand` — `contractingTotal <= opposingTotal`, with an inline note that this is the Croatian rule applied to all variants as a stand-in.
- `server/internal/room/handler.go:27` — `validVariants = map[string]bool{"bitola": true}`; the server rejects `croatia` today.
- `client/src/features/room/CreateRoomModal.tsx:284` — the Croatia option ships with `disabled: true`.

### Rules confirmed identical — must not be branched

Thirty rules were checked and match exactly: the 32-card deck; all trump and non-trump card point values; trump-beats-plain; trick-winner-leads; first bidder to the dealer's right; **two bidding rounds** (both variants have them); follow-suit; the overplay obligation **and** its lifting gate; must-trump-when-void; must-over-trump; every sequence and four-of-a-kind value; four 7s/8s scoring nothing; one-team-scores-melds; teammates' melds summing; the meld tie-break chain; identical melds still resolving with no cancel; melds forfeited by a team that scores nothing; the Belote bonus; the failed-hand threshold; Capot; the last-trick bonus; the 162-point hand total; and the 1001/501 match targets.

Two source claims were **rejected** rather than adopted. The article omits any must-trump-when-void or over-trump obligation, and omits the last-trick bonus while stating a 162-point hand total that only balances with it. Both were confirmed by the product owner as identical to Bitola. The article's Capot figure (+90 on top of the +10 last trick) is arithmetically identical to the engine's flat +100 replacing the +10 — a deliberate simplification already in the code, not a divergence.

### Second issue — card deck styles

A Croatian/German-suited card deck is wanted as a **player setting**, modelled on the language selector, not as a purchasable cosmetic. `FR50` (Epic 16, Phase 5) covers purchasable card backs and table themes, so this needs its own requirement and its own story inside Epic 12.

This is newly cheap: `spec-svg-card-deck.md` shipped 2026-08-18 (`d917b3a`), so card faces already render from vendored SVGs where the filename **is** the card ID, geometry is owned by `client/src/features/match/lib/cardFace.ts`, and the deck is regenerated through `scripts/recolor-cards.mjs`.

---

## Section 2 — Impact Analysis

### Epic impact

- **Epic 12 (Variant Expansion)** — scope corrected. Story 12.1 rewritten and split; Stories 12.4–12.8 added. Story 12.3 stays `done`. Story 12.2 stays retired in place. Phase 3 placement unchanged.
- **Epic 16 (Spectator, Achievements, Cosmetics & Tournaments)** — no scope change, but FR50 is annotated to draw the boundary against the new FR63. Purchasable cosmetics stay in Phase 5.
- **Epic 3 (Belot Rules Engine)** — no reopening. Its interim tie-rule stand-in was a documented, deliberate decision; Story 12.7 completes it as planned.
- No other epic's scope changes. No epic resequencing. No new epic — per this project's retire-and-never-renumber convention (Story 12.2, Epic 15, Epic 16), the card-deck work is Story 12.4 inside Epic 12 rather than a new epic.

### Story impact

| Story | Action |
|-------|--------|
| 12.1 Croatian Variant Rules Engine | **Rewritten and renamed** → *Variant Rule Configuration & Croatian Dealing/Bidding* |
| 12.2 | unchanged — retired in place 2026-06-11 |
| 12.3 In-App Rules Reference | unchanged — `done`; but its shipped locale content must be reconciled by 12.8 |
| 12.4 Card Deck Style Preference | **new** |
| 12.5 Croatian Declaration Overlap | **new** |
| 12.6 Croatian Declaration Phase | **new** |
| 12.7 Bitola Hanging-Points Tie Rule | **new** |
| 12.8 Croatian Variant Enablement | **new** |

Sequencing is load-bearing: **12.1 → 12.5 → 12.6 → 12.7 → 12.8**, with 12.4 independent of all of them. Enablement must be last — after 12.1 the variant is half-built, and exposing the room option earlier ships a broken game.

### Requirements impact

- **FR8** rewritten from a one-line stand-in to a six-clause description of the actual divergences.
- **FR63** new — card deck style as a free, persisted player preference.
- **FR64** new — the Bitola hanging-points tie rule, promoted from a `deferred-work.md` footnote to a citable requirement.
- **FR50** annotated to fence purchasable cosmetics off from FR63.
- **D-VAR-1** new binding design constraint on the Epic 12 card.

### Artifact conflicts

- `epics.md` — Requirements Inventory (FR8 rewritten, +FR63, +FR64), FR Coverage Map (+2 entries, FR8 and FR50 annotated), Epic 12 overview card, Epic 12 body (1 story rewritten, 5 added).
- `sprint-status.yaml` — Epic 12 block: 1 story renamed, 5 added, header note, `last_updated`.
- **`project-context.md` — actively wrong and must be corrected.** Line 319 mandates *"Separate code paths in `bidding.go`, never a generic flow"* and line 303's anti-pattern table says *"Branch by variant from the start"*. Both instruct a dev agent to do the precise opposite of D-VAR-1. This file is read before every story; left uncorrected, the first agent to pick up 12.1 hardcodes `variant == "croatia"` and the preset design dies on contact. Its Croatian bidding description is also factually wrong (it describes a one-round forced pick).
- `architecture.md` — the phase transition table (~line 1015) has no slot for a declaration phase.
- `deferred-work.md` — the hanging-points item is promoted to Story 12.7; D67 is promoted to Story 12.5; the belot-all-eight-of-a-suit item stays deferred, annotated as applying to both variants.
- `spec-svg-card-deck.md` — `<frozen-after-approval>`, and its path contract (`/cards/{ID}.svg`) changes to `/cards/{deck}/{ID}.svg`. Per that spec's own terms this is a human renegotiation, recorded as a change-log entry rather than a silent override.
- **PRD — no change.** `epics.md` is the canonical living FR list, per the FR59 / FR60 / FR61 / FR62 precedent.
- **UX spec — no change required**, though 12.6 introduces a blocking four-player prompt with no existing pattern and 12.4 adds a Settings section; both are captured in story ACs.

### Technical impact

- **New WS event surface** in 12.1 (round-2 reveal) and 12.6 (declaration phase), each incurring the drift-gate touchpoints: Go golden files, testdata, Zod schemas, contract tests — `wsEvents.ts` and `events.go` in the same commit.
- **New game phase** in 12.6, reaching the session manager's timer handling and the full pause/disconnect/reconnect matrix.
- **New `GameState` fields** — the `VariantRules` config (12.1), per-player card visibility (12.1), and the hanging-points accumulator (12.7).
- **Information-security surface, unique to this epic.** The Croatian round-2 reveal is the only rule in the codebase where information must be withheld from a player about *their own cards*. It must live in the server-authoritative snapshot; client-side masking would be trivially defeated and would violate NFR8.
- **DB migration** `000020_add_card_deck_to_users` (12.4). The next free number is 000020 — `000019_create_friendships` is the current highest.
- **Known duplication bites again** (12.4): `axiosClient.ts`, `hooks/mutations/useAuth.ts`, and `hooks/useAuth.ts` each build a `User` field by field. Adding `cardDeckPreference` edits all three; folding the long-deferred unification into 12.4 is recommended while all three files are already open.
- **Asset procurement blocks 12.4.** The current deck is CC0 from `me.uk/cards`. A German/Croatian-suited set (žir / bundeva / zelje / srce) must be sourced under a compatible licence, generated through `scripts/recolor-cards.mjs`, and its provenance recorded in `docs/card-deck.md`. This is a procurement dependency, not a coding one.

---

## Section 3 — Recommended Approach

**Direct Adjustment** — modify and add stories within the existing plan.

- **Rollback: not viable and unnecessary.** No Croatian code exists to revert. Story 12.3 shipped correctly and stays.
- **MVP review: not needed.** This is Phase 3 work; the MVP is untouched.
- **Effort: Low** (planning artifacts only). **Risk: Low.**

The correction is unusually cheap right now for one specific reason: `state.Variant` is stored but **never read** anywhere in the engine. There is not a single variant-comparison site to unwind, so D-VAR-1 can be established on clean ground. Every month of Phase 3 that passes without it raises the cost.

**Trade-off considered and rejected:** keeping Story 12.1 as a single large story with corrected ACs. Rejected because the corrected scope spans the deal, both bidding rounds, per-player card visibility, meld detection, a new state-machine phase, cross-hand scoring state, and the client — one story would be the largest in the project and unreviewable as a unit.

---

## Section 4 — Detailed Change Proposals

### 4a. `epics.md` — Requirements Inventory

**FR8 — OLD:**

> FR8: The system enforces Croatian variant rules: 3+2 dealing sequence, forced trump selection by last player in bidding, counter-clockwise play, and variant-specific scoring

**FR8 — NEW:**

> FR8: The system enforces Croatian variant rules, which diverge from Bitola in six places: (a) **deal shape** — all eight cards are dealt before bidding as 3+3 visible to their holder plus 2 face-down, with no trump candidate flipped and no post-pick distribution stage; (b) **trump selection** — trump is a bare named suit, chosen freely in both rounds, and the picker takes no card into hand; (c) **round 2 reveal** — four passes in round 1 turns each player's two face-down cards up to that player, so round 2 is bid on a full eight-card hand; (d) **all-pass outcome** — the dealer, bidding last in round 2, must name a suit; there is no reshuffle-and-rotate; (e) **declaration overlap** — a single card may count in more than one declaration, with no Bitola single-use dedup; (f) **declaration timing** — a dedicated declaration phase runs between bidding and trick 1 in which all four players declare or skip, its result is revealed, and only then does trick 1 begin (Bitola declares during trick 1 and reveals at trick 2). All other rules are shared with Bitola: counter-clockwise play, follow-suit, the overplay obligation and its lifting gate, must-trump-when-void, over-trump, all card and declaration point values, the meld tie-break chain, Belote bonus, failed hands, last-trick bonus, Capot, and the tied-hand rule (which Bitola alone leaves — see FR64). Both variants are expressed as **named presets over an internal per-rule configuration** (see D-VAR-1), not as hardcoded variant branches.

**FR63 — NEW** (appended after FR62):

> FR63: Players can choose a card deck style (French-suited default, Croatian/German-suited alternative) as a persisted account preference, selectable from the in-game Settings dialog alongside language. The choice is purely visual, applies to every card surface in a match, is free of charge, and has no gameplay effect — it is not a purchasable cosmetic (see FR50) (added 2026-08-18)

**FR64 — NEW:**

> FR64: The Bitola variant applies the **hanging points** tie rule: when the taker's team total equals the opponents', neither team scores and the hand's points are carried over to the side that wins the next decisive hand. Interacts with the 1001/501 match target and match-end resolution. Croatian keeps all-points-to-opponents, which is what ships today for both variants as an interim stand-in. Split out of Story 3.5 (added 2026-08-18)

### 4b. `epics.md` — FR Coverage Map

| Entry | Change |
|-------|--------|
| FR8 | → `FR8: Epic 12 — Croatian variant rules engine (seven divergences; scope corrected 2026-08-18)` |
| FR50 | append `— purchasable cosmetics only; the free deck-style preference is FR63/Epic 12` |
| FR63 | **new** — `FR63: Epic 12 — Card deck style preference (free, Settings-level; added 2026-08-18)` |
| FR64 | **new** — `FR64: Epic 12 — Bitola hanging-points tie rule (added 2026-08-18)` |

### 4c. `epics.md` — Epic 12 overview card

Replaces the existing card in the Epic List section.

> ### Epic 12: Variant Expansion
>
> Players can play the Croatian variant, choose the card deck style they play with, and access an in-app rules reference covering both variants. (The 501-point match mode originally planned here moved to Epic 10 as Story 10.2 on 2026-06-11.)
>
> **Scope corrected 2026-08-18** (sprint-change-proposal-2026-08-18.md). A rules audit against a Croatian source, corrected by a player, found Story 12.1's original premise wrong: Croatian is not "Bitola plus a forced dealer pick." It diverges in **seven** places — deal shape (3+3+2, no candidate card, no post-pick distribution), free suit choice in both bidding rounds, the round-2 reveal of each player's two face-down cards, the forced dealer pick, declaration overlap, a dedicated declaration phase before trick 1, and the tied-hand rule. Thirty other rules — every card and declaration value, the meld tie-break chain, all four play obligations, Belote, failed hands, last trick, and Capot — were verified **identical** and must not be branched.
>
> **D-VAR-1 — Variants are presets over a rule config, not branch conditions.** Every divergence above is a named field on a `VariantRules` struct resolved once at game initialization; `bitola` and `croatia` are preset resolvers returning a fully-populated config. The engine reads rule fields only — `state.Variant` is never compared inside `bidding.go`, `declarations.go`, `scoring.go`, or `validation.go`. Separate code paths per rule outcome are still required; the selector is the config field, not the variant string. The seven fields are: deal shape, trump candidate on/off, round-2 reveal on/off, all-pass outcome, declaration overlap, declaration timing, tie rule. This is forward-compatibility for a planned future story exposing these rules directly in room creation, with Bitola and Croatian demoted to preset buttons — that story must be reachable by adding a config source, not by rewriting the engine.
>
> **Tied hand — the divergence Bitola absorbs.** Croatian sends a tied hand's points to the taker's opponents, which is exactly what ships today for all variants as a deliberate interim stand-in (Story 3.5). **Bitola** is the side that moves, to hanging points (carry-over): on a tie nobody scores and the points carry to the side that wins the next decisive hand. Needs cross-hand state plus match-end interaction — Story 12.7.
>
> **FRs covered:** FR8, FR29, FR63, FR64
> **Phase:** 3

### 4d. `epics.md` — Epic 12 story bodies

#### Story 12.1: Variant Rule Configuration & Croatian Dealing/Bidding

*(replaces "Story 12.1: Croatian Variant Rules Engine" in full)*

As a player,
I want to play the Croatian variant's dealing and trump bidding with its authentic rules,
So that the platform supports both major Balkan Belot variants without either one's rules bending to fit the other.

**Acceptance Criteria:**

**Given** the rules engine initializes any game
**When** `NewGame` runs
**Then** a `VariantRules` config is resolved once from the variant name and carried on `GameState`
**And** `bitola` and `croatia` preset resolvers each return a fully-populated config — no field is left to a default
**And** no file in `internal/game` compares `state.Variant` to a variant name; every branch reads a config field (D-VAR-1)

**Given** a Croatian game is dealt
**When** `dealCards` runs
**Then** all eight cards per player are dealt before bidding — 3, then 3, then 2 face-down
**And** no trump candidate is flipped and `TrumpCandidate` stays `nil`
**And** `gs.Deck` is empty — there is no post-pick distribution stage

**Given** a Croatian game is in bidding round 1
**When** a player's state snapshot is built
**Then** that player sees six of their own cards and the two face-down cards are withheld **server-side**
**And** no player's face-down cards appear in any other player's snapshot at any point

**Given** a Croatian game is in bidding round 1
**When** a player picks trump
**Then** the trump is the suit named in `action.Suit` — freely chosen, not bound to any candidate card
**And** the picker takes no card into hand

**Given** all four players pass in Croatian bidding round 1
**When** round 2 opens
**Then** each player's two face-down cards become visible **to that player only**
**And** bidding resumes from the player after the dealer with all suits available

**Given** a Croatian game is in bidding round 2 and the first three players have passed
**When** it is the dealer's turn
**Then** `pass_trump` is rejected and only `pick_trump` is legal
**And** the hand proceeds to play — `reshuffleAndRedeal` is unreachable under the Croatian config

**Given** any Bitola game
**When** the full existing test suite runs
**Then** every Bitola test passes unchanged — the flipped candidate, stage-2 distribution, round-1 candidate-suit binding, and reshuffle-and-rotate all behave exactly as before

**Given** the client renders a Croatian bidding turn
**When** `TrumpPrompt` and `TrumpReveal` mount
**Then** they render a free four-suit choice with no candidate card, in both rounds
**And** the Bitola candidate presentation and round-2 candidate-disable behaviour are untouched

**Technical notes:** `handlePickTrump`'s `TrumpCandidate == nil || len(Deck) != 11` guard becomes config-gated. Hidden-card masking belongs in the snapshot builder, never the client — this is the epic's only rule where information must be withheld from a player about their own cards, and NFR8 makes it server-authoritative. The round-2 reveal event lands in `wsEvents.ts` and `events.go` in one commit.

#### Story 12.4: Card Deck Style Preference

As a player,
I want to choose which card deck the game draws with,
So that I can play with the deck I grew up with rather than the one the platform defaults to.

**Acceptance Criteria:**

**Given** a registered player
**When** their account is created or migrated
**Then** a `card_deck_preference` column on `users` defaults to `french`
**And** migration `000020_add_card_deck_to_users` ships with a reversing `.down.sql`

**Given** a player opens the Settings dialog
**When** the deck section renders
**Then** it presents the available decks the same way the language section presents languages — a labelled radio set with the current choice marked
**And** selecting a deck applies it immediately, with no page reload and no match interruption

**Given** a player changes their deck
**When** the selection is made
**Then** it is persisted via the same `PATCH /users/:id/preferences` endpoint that carries `languagePreference`
**And** an unreachable or failing endpoint reverts the UI to the previous choice, matching how the language handler already behaves

**Given** a player signs in on a different browser
**When** the auth envelope resolves
**Then** their deck preference is applied before the first card renders

**Given** any deck is active
**When** a card renders anywhere in a match
**Then** every card surface uses that deck — hand, trick area, card flight, deal animation, trump prompt and reveal, declaration prompt and reveal, Belote prompt and reveal
**And** `cardFaceUrl` derives the path from the deck and the card ID with no lookup table
**And** card IDs, geometry, states, glow, the face-down back, and all `data-testid`s are unchanged

**Given** a screen-reader user with the Croatian deck active
**When** a card is focused
**Then** the `aria-label` names the suit as that deck depicts it, in the player's language

**Given** the marketing landing page
**When** it renders its decorative cards
**Then** it is untouched — `features/landing/components/PlayingCard.tsx` stays on the French deck for unauthenticated visitors

**Given** the deck assets
**When** they are added
**Then** they live under `client/public/cards/{deck}/`, are generated through `scripts/recolor-cards.mjs` rather than hand-edited, and their provenance and licence are recorded in `docs/card-deck.md`

**Dependencies:** artwork must be sourced under a compatible licence before this story starts; `spec-svg-card-deck.md`'s frozen path contract needs a change-log entry; the three-way auth-envelope duplication should be unified here.

#### Story 12.5: Croatian Declaration Overlap

As a Croatian-variant player,
I want a card to count toward every declaration it belongs to,
So that a hand holding four jacks and a sequence through a jack scores both, as it does at a real table.

**Acceptance Criteria:**

**Given** a Croatian game resolves declarations
**When** `detectDeclarations` returns groups that share a card
**Then** all of them survive — the `dedupBitola` pass is skipped by config, not by a variant-name check
**And** a hand with 9-10-J-Q of spades and all four jacks scores the sequence **and** the four-of-a-kind

**Given** a Bitola game resolves declarations
**When** two groups share a card
**Then** the higher-value group is kept and the other dropped, exactly as today, with every existing dedup test passing unchanged

**Given** a Croatian player holds two surviving declarations
**When** the declaration result is revealed
**Then** the reveal renders every surviving meld for that player rather than only the first
**And** the panel anchors correctly when one player contributes multiple melds — closes deferred **D67**

**Given** both partners on the winning team hold declarations
**When** the team total is computed
**Then** all their melds sum, unchanged from today's `teamDeclarationTotal` behaviour

#### Story 12.6: Croatian Declaration Phase

As a Croatian-variant player,
I want to declare after trump is set and before the first card is played,
So that declarations resolve the way the Croatian game plays them rather than being folded into trick one.

**Acceptance Criteria:**

**Given** a Croatian game where trump has just been set
**When** bidding completes
**Then** the game enters a dedicated declaration phase before any card is played
**And** all four players are prompted to declare or skip
**And** the phase resolves only once every player has answered

**Given** the Croatian declaration phase resolves
**When** the result is determined
**Then** it is revealed to all four players
**And** only then does trick 1 begin

**Given** a Bitola game
**When** a hand is played
**Then** declarations still fire per player during trick 1 and reveal at the start of trick 2 — no Bitola behaviour changes

**Given** a player's move timer expires during the Croatian declaration phase
**When** the timer fires
**Then** the player is auto-skipped and the phase continues, consistent with how auto-play resolves an expired turn elsewhere

**Given** a player disconnects or the match is paused during the declaration phase
**When** they reconnect
**Then** the restored snapshot places them back in the declaration phase with their own answer state intact

**Given** the declaration phase exists
**When** its events cross the wire
**Then** they are defined in `wsEvents.ts` and `events.go` in the same commit, and the phase transition table in `architecture.md` is updated

**Note:** the largest story in the epic — it adds a phase to the state machine, reaching the session manager's timer handling and the full pause/reconnect matrix, plus a blocking four-player prompt with no existing UI precedent (`DeclarationPrompt` today is per-card, single-player, during trick 1).

#### Story 12.7: Bitola Hanging-Points Tie Rule

As a Bitola player,
I want a tied hand's points to hang and carry to the next decisive hand,
So that a tie is held over the way we play it, rather than handed to the opponents.

**Acceptance Criteria:**

**Given** a Bitola hand where the taker's team total equals the opponents'
**When** the hand is scored
**Then** neither team's match score changes
**And** the hand's full point pool is held as hanging points on the game state

**Given** hanging points are outstanding
**When** the next hand resolves decisively
**Then** the hanging points are added to that hand's winning side and the accumulator resets to zero

**Given** hanging points are outstanding
**When** the next hand also ties
**Then** both pools accumulate and carry forward together

**Given** a Bitola hand ties
**When** the match-end condition is checked
**Then** no team crosses the target on that hand — hanging points count toward nobody until they resolve

**Given** a match ends by surrender or abandonment with hanging points outstanding
**When** the match is finalized
**Then** the hanging points are discarded and the existing surrender/abandonment settlement is unchanged

**Given** a Croatian hand ties
**When** it is scored
**Then** all points transfer to the taker's opponents, exactly as ships today — no Croatian behaviour changes

**Given** a hand ties or resolves hanging points
**When** the score reveal and hand-results table render
**Then** the held-over state is visible to players, in all four locales

**Given** the rules reference
**When** it describes Bitola scoring
**Then** the hanging-points rule is documented in all four locale files, and the Croatian pages continue to describe all-points-to-opponents

**Resolved ambiguity — record this in the story file.** "The side that wins the next decisive hand" has two candidate readings: (a) the team with the higher hand total, or (b) the taker's team if they succeed, else the opponents. They are **provably equivalent** across every reachable outcome — a successful take means scoring strictly more; a failed take transfers everything to the opponents; a Capot gives its team the higher total by definition; and a tie is not decisive. Implement either, but do not let a future refactor invent a third meaning.

#### Story 12.8: Croatian Variant Enablement

As a player,
I want to actually create and play Croatian rooms,
So that the variant is available rather than merely implemented.

**Acceptance Criteria:**

**Given** a player creates a room
**When** they choose the Croatian variant
**Then** the server accepts it — `validVariants` includes `croatia`
**And** the create-room dropdown option is no longer `disabled`

**Given** a Croatian room with bot players
**When** the match runs
**Then** bots bid correctly with no trump candidate in both rounds, answer the declaration phase, and play tricks under the shared obligations
**And** bot behaviour reads the rule config, never a variant name

**Given** Quick Play matchmaking
**When** a player queues
**Then** they are matched into Bitola rooms only, as a documented and intentional limit for this epic

**Given** the in-app rules reference
**When** a player reads either variant's pages
**Then** the content matches what the engine actually does, in all four locales, reconciled against everything 12.1 / 12.5 / 12.6 / 12.7 shipped

**Given** a room appears in the lobby, room preview, or match history
**When** it is Croatian
**Then** the variant is visible to the player

**Note:** must ship last. After 12.1 the variant is half-built; enabling the room option earlier ships a broken game.

### 4e. `sprint-status.yaml`

```yaml
  # --- Epic 12: Variant Expansion (Phase 3) ---
  # Scope corrected 2026-08-18 via bmad-correct-course (sprint-change-proposal-2026-08-18.md):
  # rules audit found SEVEN divergences, not one. 12.1 rewritten + split; 12.4-12.8 added.
  epic-12: backlog
  12-1-variant-rule-config-and-croatian-bidding: backlog
  # 12-2 501-point match mode moved to Epic 10 as 10-2 on 2026-06-11 (bmad-correct-course)
  12-3-in-app-rules-reference: done
  12-4-card-deck-style-preference: backlog
  12-5-croatian-declaration-overlap: backlog
  12-6-croatian-declaration-phase: backlog
  12-7-bitola-hanging-points-tie-rule: backlog
  12-8-croatian-variant-enablement: backlog
  epic-12-retrospective: optional
```

Header `last_updated` → `2026-08-18`.

### 4f. Ripple edits outside the epic files

| File | Change |
|------|--------|
| `project-context.md` **:319** | *"Trump bidding branches by variant … Separate code paths in `bidding.go`, never a generic flow"* → restate as D-VAR-1: branch on `VariantRules` config fields, never the variant name. Correct the Croatian description — it currently describes a one-round forced pick; Croatian has two rounds, no candidate card, free suit choice, and a round-2 reveal. |
| `project-context.md` **:303** | Anti-pattern row *"Implement generic trump bidding → Branch by variant from the start"* → *"Compare `state.Variant` in the engine → Branch on `VariantRules` config fields"* |
| `project-context.md` declaration + tie notes | Promote the three "(deferred)" Croatian asides to specified rules, pointing at Stories 12.5 / 12.6 / 12.7 |
| `architecture.md` (~line 1015) | Add the Croatian declaration phase to the phase transition table |
| `deferred-work.md` | Hanging-points item → promoted to Story 12.7. D67 → promoted to Story 12.5. Belot-all-eight-of-a-suit → stays deferred, annotated as applying to **both** variants (not implemented for Bitola either; rules-page text only). |
| `spec-svg-card-deck.md` | Change-log entry renegotiating the frozen `/cards/{ID}.svg` path to `/cards/{deck}/{ID}.svg` |

**The `project-context.md` edits are the highest-value item in this proposal.** That file is loaded before every story. Lines 303 and 319 currently instruct a dev agent to do the exact opposite of D-VAR-1, and its Croatian bidding description is factually wrong.

---

## Section 5 — Implementation Handoff

**Scope classification: Moderate** — backlog reorganization plus corrective edits to standing agent guidance. No code.

### Immediate — planning artifacts

| Deliverable | Owner |
|-------------|-------|
| `epics.md` — 4a, 4b, 4c, 4d | PO / Dev |
| `sprint-status.yaml` — 4e | PO / Dev |
| `project-context.md`, `architecture.md`, `deferred-work.md`, `spec-svg-card-deck.md` — 4f | PO / Dev |

### Then — story cycle

Run `bmad-create-story` for **12.1** first. Sequence is **12.1 → 12.5 → 12.6 → 12.7 → 12.8**, with **12.4 parallel and independent**.

### Blocking dependency

**Story 12.4 cannot start until Croatian/German-suited card artwork is sourced** under a licence compatible with the project, and its provenance is recorded in `docs/card-deck.md`. This is procurement, not engineering — start it now so it does not gate the story later.

### Success criteria

1. No file in `server/internal/game` compares `state.Variant` to a variant name.
2. Every pre-existing Bitola test passes unchanged across all seven stories.
3. A player's face-down cards never appear in another player's state snapshot, and never in their own before the round-2 reveal.
4. The rules reference in all four locales matches engine behaviour once 12.8 closes.
5. `bitola` and `croatia` are the only variant-aware constructs outside the preset resolvers — proving the future room-configurable-rules story is reachable by adding a config source.

### Open items carried forward

- Quick Play remains Bitola-only. Extending matchmaking to variants is deliberately **not** in this epic and needs its own story when wanted.
- The future "room-configurable rules, variants as presets" story is not written here. D-VAR-1 exists to keep it cheap; it should be raised when Phase 3 completes.

---

## Approval

**Change trigger:** Epic 12 pre-flight rules audit, 2026-08-18.
**Recommended path:** Direct Adjustment.
**Approved by:** Emilijan — 2026-08-18.
