# Epic 12 Context: Variant Expansion

<!-- Compiled from planning artifacts. Edit freely. Regenerate with compile-epic-context if planning docs change. -->

## Goal

Make Beljot a genuinely two-variant platform: the Croatian variant becomes playable with its authentic rules, players can choose the card deck they play with, and an in-app rules reference covers both variants. Scope was corrected on 2026-08-18 after a rules audit found the original premise wrong — Croatian is not "Bitola plus a forced dealer pick." It diverges in **seven** places, while **thirty** other rules were verified genuinely identical and must not be branched. So the deliverable is twofold: the Croatian rules themselves, plus a per-rule configuration layer that makes variants *presets* rather than hardcoded branches, so a future story can expose these rules directly in room creation without rewriting the engine. This is a rule-correctness epic — an incorrect rule hurts more than any UI defect, because players who know Belot detect it immediately.

## Stories

- Story 12.1: Variant Rule Configuration & Croatian Dealing/Bidding — **next to implement**
- Story 12.2: [retired] 501-Point Match Mode — moved to Epic 10 as Story 10.2; the number is never reused
- Story 12.3: In-App Rules Reference — **already `done`**
- Story 12.4: Card Deck Style Preference
- Story 12.5: Croatian Declaration Overlap
- Story 12.6: Croatian Declaration Phase
- Story 12.7: Bitola Hanging-Points Tie Rule
- Story 12.8: Croatian Variant Enablement

## Requirements & Constraints

**The seven divergences** — everything else is shared:

1. **Deal shape** — Croatian deals all eight cards before bidding: 3, then 3, then 2 face-down. Bitola deals 3+2, flips a candidate, holds 11 cards for post-pick distribution.
2. **Trump candidate** — Croatian has none; trump is a bare named suit chosen freely in both rounds, and the picker takes no card into hand.
3. **Round-2 reveal** — four passes in Croatian round 1 turns each player's two face-down cards up **to that player only**; round 2 is bid on a full eight-card hand.
4. **All-pass outcome** — the Croatian dealer bids last in round 2 and *must* name a suit; no reshuffle-and-rotate.
5. **Declaration overlap** — in Croatian one card may count in more than one declaration; Bitola keeps one-card-one-group dedup by higher value.
6. **Declaration timing** — Croatian runs a dedicated phase between bidding and trick 1: all four declare or skip, the result is revealed, then trick 1 begins. Bitola declares per player during trick 1, revealing at trick 2.
7. **Tied hand** — Croatian sends the points to the taker's opponents, which is exactly what ships today for *both* variants as a deliberate interim stand-in. **Bitola is the side that moves**, to hanging points: nobody scores and the pool carries to the side that wins the next decisive hand.

**Verified identical — do not branch:** the deck and every card value, trump-beats-plain, trick-winner-leads, first bidder to the dealer's right, two bidding rounds in *both* variants, all four play obligations (follow-suit, overplay and its lifting gate, must-trump-when-void, must-over-trump), every meld value and the tie-break chain, melds forfeited by a team scoring nothing, Belote-Rebelote, failed hands, last trick, Capot, the 162-point hand total, and the 1001/501 targets.

**Rule authority is the project owner, not published rule guides.** The audit source omitted must-trump-when-void, over-trump, and the last-trick bonus; all three were confirmed present in Croatian too and the omissions rejected. The engine's flat Capot value is an intentional arithmetic simplification, not a divergence. Where a planning doc states a rule, the doc wins over general Belot knowledge.

**Non-negotiables in every story:**

- Every pre-existing Bitola test passes unchanged — Bitola is a regression surface, not a refactor target.
- A player's face-down cards never appear in another player's snapshot, and never in their own before the round-2 reveal. This is the only rule that withholds information from a player about their *own* cards, so masking lives in the server-side snapshot builder; client-side masking is trivially defeated and breaks the server-authoritative security requirement.
- No engine file compares the variant name (D-VAR-1).
- Quick Play stays Bitola-only here — a documented, intentional limit.

**Localization:** four locales — en, mk, hr, sr. Macedonian is all-Cyrillic, the rest Latin; Croatian and Serbian forms are never mixed. Non-English strings must read idiomatically, never as literal English calques. The word **"contract" is banned** as a game term in every locale and in these docs: phrase outcomes as *pulled it off / went down / failed hand*, and call the trump caller *the taker*. New rules-reference and score-reveal copy in 12.6–12.8 lands in all four locale files.

## Technical Decisions

**D-VAR-1 (binding).** Variants are presets over a rule config, not branch conditions. Every divergence is a named field on a `VariantRules` struct resolved **once** at game initialization and carried on game state; `bitola` and `croatia` are preset resolvers each returning a *fully populated* config with no field left to a default. The engine reads config fields only — the variant string is never compared in bidding, declaration, scoring, or validation code. Separate code paths per rule outcome are still required; only the *selector* changes. The seven fields: deal shape, trump candidate on/off, round-2 reveal on/off, all-pass outcome, declaration overlap, declaration timing, tie rule. Success means the preset resolvers are the only variant-aware constructs in the engine.

This is cheap to establish now precisely because the variant field is stored but never read — zero comparison sites to unwind. Standing agent guidance previously said the opposite ("branch by variant from the start"); that has been corrected and is void.

- **New phase.** A `declaring` phase exists only under the Croatian config, accepting declare/skip and transitioning to play once all four answer. Croatian bidding *always* reaches it because the forced dealer means bidding cannot fail — so reshuffle-and-redeal is unreachable under that config. Bitola resolves declarations inside the playing phase. The architecture doc's phase transition table is the session-manager/engine contract and must stay in sync.
- **Config-gated guards.** The existing pick-trump guard rejecting a nil candidate or wrong-sized deck becomes config-gated, not removed.
- **Wire contract.** Two new event surfaces — the round-2 reveal (12.1) and the declaration phase (12.6). The TypeScript and Go event contract files are updated in the *same commit*, no exceptions, and each event incurs the full drift gate: Go golden files, testdata, Zod schemas, contract tests.
- **New state fields:** the rule config and per-player card visibility (12.1), the hanging-points accumulator (12.7). Fields go in their existing sections, not appended arbitrarily. Tests use the fixture factories, never raw struct literals.
- **Deck preference (12.4):** a users column defaulting to the French deck via migration `000020` with a reversing down migration, persisted through the existing preferences endpoint alongside language, deriving asset paths from deck + card ID with no lookup table.

## UX & Interaction Patterns

- **Croatian bidding** renders a free four-suit choice with no candidate card, in *both* rounds. Bitola's candidate presentation and round-2 candidate-disable behaviour stay untouched. The round-2 reveal is visible to its owner only.
- **The Croatian declaration phase has no UI precedent** — a blocking prompt for all four players at once, where today's is per-card, single-player, inside trick 1. Game prompts cannot be dismissed or closed by backdrop. An expired timer auto-skips and the phase continues, consistent with auto-play elsewhere; reconnecting restores the player into the phase with their own answer intact.
- **Multiple melds per player must render.** The reveal currently shows only the first, and the panel must anchor correctly when one player contributes several — this closes a known deferred defect.
- **Deck picker** mirrors the language section: a labelled radio set with the current choice marked, applying immediately with no reload or match interruption, reverting on persistence failure. Every card surface follows the active deck — hand, trick area, card flight, deal animation, and every trump, declaration, and Belote prompt and reveal — while card IDs, geometry, states, glow, the face-down back, and all test IDs stay unchanged. Screen-reader labels name suits as the active deck depicts them, in the player's language. The unauthenticated landing page keeps the French deck.
- **Rules reference** (shipped) is a lobby nav tab and, in-match, a persistent bottom-right icon opening a dismissible overlay that does not interrupt play.
- **Hanging points must be visible** in the score reveal and hand-results table, in all four locales. **Croatian rooms must be identifiable** in the lobby, room preview, and match history.

## Cross-Story Dependencies

- **Sequencing is load-bearing: 12.1 → 12.5 → 12.6 → 12.7 → 12.8, with 12.4 fully independent.** Enablement (12.8) ships **last** — after 12.1 the variant is half-built, and exposing the room option earlier ships a broken game.
- 12.1 establishes the rule config every later story reads; nothing else can be done cleanly first.
- 12.6 is the largest story: adding a state-machine phase reaches the session manager's timer handling and the whole pause / disconnect / reconnect matrix.
- 12.7 interacts with the match target and match-end resolution — a tie can never carry a team across the target, and hanging points are discarded on surrender or abandonment with existing settlement unchanged. "Wins the next decisive hand" is settled and must not be reinterpreted: highest hand total and taker-succeeds-else-opponents are provably equivalent across all reachable outcomes.
- 12.8 needs 12.1 / 12.5 / 12.6 / 12.7 shipped, reconciles the already-`done` rules-reference locale content against real engine behaviour, enables the variant server-side and in the create-room dropdown, and verifies bots bid with no candidate, answer the declaration phase, and read the config rather than a variant name.
- 12.4 is **blocked on procurement, not engineering**: Croatian/German-suited artwork must be licensed compatibly and generated through the existing recolour script with provenance recorded. It also renegotiates a frozen card-path contract (a change-log entry, not a silent override) and is the moment to unify the three-way duplication in auth-envelope user construction, since all three files are open anyway.
- Two deferred items are absorbed: the multiple-meld reveal defect by 12.5, the hanging-points rule by 12.7. "Belote across all eight of a suit" stays deferred and applies to *both* variants — rules-page text only, unimplemented for Bitola too.
