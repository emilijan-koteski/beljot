# Epic 12 Context: Variant Expansion

<!-- Compiled from planning artifacts. Edit freely. Regenerate with compile-epic-context if planning docs change. -->

## Goal

Make Beljot a genuinely two-variant platform: the Croatian variant becomes playable with its authentic rules, players can choose the card deck they play with, and an in-app rules reference covers both variants. Scope was corrected on 2026-08-18 after a rules audit found the original premise wrong — Croatian is not "Bitola plus a forced dealer pick." It diverges in **seven** places, while **thirty** other rules were verified genuinely identical and must not be branched. So the deliverable is twofold: the Croatian rules themselves, plus a per-rule configuration layer that makes variants *presets* rather than hardcoded branches, so a future story can expose these rules directly in room creation without rewriting the engine. This is a rule-correctness epic — an incorrect rule hurts more than any UI defect, because players who know Belot detect it immediately. The rule config, Croatian dealing/bidding, declaration overlap and the dedicated declaration phase are built and awaiting review; what remains is enablement and the deck preference. The seventh divergence — Bitola's tie rule — was deliberately dropped on 2026-08-20: Bitola keeps borrowing the Croatian tie rule, so only six divergences ship.

## Stories

- Story 12.1: Variant Rule Configuration & Croatian Dealing/Bidding — `review`
- Story 12.2: [retired] 501-Point Match Mode — moved to Epic 10 as Story 10.2; the number is never reused
- Story 12.3: In-App Rules Reference — `done`
- Story 12.4: Card Deck Style Preference — `backlog`
- Story 12.5: Croatian Declaration Overlap — `review`
- Story 12.6: Croatian Declaration Phase — `review`
- Story 12.7: Bitola Hanging-Points Tie Rule — **deferred indefinitely** (owner decision 2026-08-20; not a blocker for anything)
- Story 12.8: Croatian Variant Enablement — `backlog`, **next to implement**, ships last

## Requirements & Constraints

**The divergences** — everything else is shared. Six ship; the seventh (the tie rule) was dropped:

1. **Deal shape** — Croatian deals all eight cards before bidding: 3, then 3, then 2 face-down. Bitola deals 3+2, flips a candidate, holds 11 cards for post-pick distribution. *(shipped)*
2. **Trump candidate** — Croatian has none; trump is a bare named suit chosen freely in both rounds, and the picker takes no card into hand. *(shipped)*
3. **Round-2 reveal** — four passes in Croatian round 1 turns each player's two face-down cards up **to that player only**; round 2 is bid on a full eight-card hand. *(shipped)*
4. **All-pass outcome** — the Croatian dealer bids last in round 2 and *must* name a suit; no reshuffle-and-rotate. *(shipped)*
5. **Declaration overlap** — in Croatian one card may count in more than one declaration; Bitola keeps one-card-one-group dedup by higher value. *(shipped)*
6. **Declaration timing** — Croatian runs a dedicated phase between bidding and trick 1: all four seats declare or skip, the result is revealed, then trick 1 begins. Bitola declares per player during trick 1, revealing at trick 2. *(shipped)*
7. **Tied hand** — Croatian sends the points to the taker's opponents, which is what ships for *both* variants. **This divergence is deliberately NOT implemented**: Bitola keeps borrowing the Croatian rule indefinitely. *(dropped 2026-08-20 — Story 12.7 deferred)*

**The tie rule is a non-divergence for now (owner decision, 2026-08-20).** A tied hand — the taker's team total exactly equal to the opponents' — is a failed hand for the taker in **both** variants, and the whole pool transfers to the opponents. That is the Croatian rule, and Bitola borrows it. Hanging points (nobody scores, the pool carries to the winner of the next decisive hand) is Bitola's authentic rule and is recorded as such in `VariantRules.TieRule`, but the engine does **not** read that field and must not be changed to. Do not treat Bitola's tie behaviour as a bug, do not "fix" the rules-reference sentence "Fall short — or even tie — and the hand is lost" (it is correct for both variants), and do not resurrect the story without a fresh owner decision. See `deferred-work.md` for the planning findings.

**Rule authority is the project owner, not published rule guides.** The audit source omitted must-trump-when-void, over-trump, and the last-trick bonus; all three were confirmed present in Croatian too. The engine's flat Capot value is an intentional arithmetic simplification, not a divergence. Where a planning doc states a rule, the doc wins over general Belot knowledge.

**Verified identical — do not branch:** the deck and every card value, trump-beats-plain, trick-winner-leads, first bidder to the dealer's right, two bidding rounds in *both* variants, all four play obligations (follow-suit, overplay and its lifting gate, must-trump-when-void, must-over-trump), every meld value and the tie-break chain, melds forfeited by a team scoring nothing, Belote-Rebelote, failed hands, last trick, Capot, the 162-point hand total, and the 1001/501 targets.

**Non-negotiables in every story:**

- Every pre-existing Bitola test passes unchanged — Bitola is a regression surface, not a refactor target.
- A player's face-down cards never appear in another player's snapshot, and never in their own before the round-2 reveal. Masking lives in the server-side snapshot builder; client-side masking is trivially defeated and breaks the server-authoritative requirement.
- No engine file compares the variant name (D-VAR-1).
- Quick Play stays Bitola-only here — a documented, intentional limit.

**Localization:** four locales — en, mk, hr, sr. Macedonian is all-Cyrillic, the rest Latin; Croatian and Serbian forms are never mixed. Non-English strings must read idiomatically, never as literal English calques; em-dashes are en-only and quotes are `„…"` in mk/hr/sr. The word **"contract" is banned** as a game term in every locale and in these docs: phrase outcomes as *pulled it off / went down / failed hand*, and call the trump caller *the taker* (software/wire "contract" is unaffected, and code identifiers are never renamed).

## Technical Decisions

**D-VAR-1 (binding).** Variants are presets over a rule config, not branch conditions. Every divergence is a named field on a `VariantRules` struct resolved **once** at game initialization and carried on game state; `bitola` and `croatia` are preset resolvers each returning a *fully populated* config with no field left to a default. The engine reads config fields only — the variant string is never compared in bidding, declaration, scoring, or validation code. Separate code paths per rule outcome are still required; only the *selector* changes. The seven fields: deal shape, trump candidate on/off, round-2 reveal on/off, all-pass outcome, declaration overlap, declaration timing, tie rule. Six are read; `TieRule` is populated but deliberately unread, and stays that way.

- **The `declaring` phase (shipped).** Selected by the declaration-timing config field, entered from a resolved bid instead of starting trick 1. It prompts seats **one at a time** counter-clockwise from the trick-1 leader, reusing the existing awaiting-declaration prompt — so it added **no WS event and no client→server action**; the phase field on the state snapshot carries it. Only meld-holding seats are prompted, the rest count as answered, and resolution runs the same path Bitola uses at trick 2 before handing off to `playing` at trick 1. It is a full turn-taking phase for every session-manager concern (turn timer with auto-skip, pause, surrender, bots, disconnect/reconnect), and every phase switch in the match layer must name it — each `default` arm is a silent stall, not an error.
- **Phase transition table is the contract.** The architecture doc's phase table and phase-error table are the session-manager/engine contract and must stay in sync with any phase or scoring change; instant-win on a pick is checked before either post-bid branch.
- **Wire contract.** The round-2 reveal event was the epic's one new event surface. Any payload change needed to surface held-over points must update the TypeScript and Go event contract files in the *same commit*, and incurs the full drift gate: Go golden files, testdata, Zod schemas, contract tests. The client mirrors of the rule config and declaration logic are prose-only mirrors today — no generated contract binds them.
- **State fields.** New fields (the rule config and per-player card visibility from 12.1) go in their existing sections of the state struct, not appended arbitrarily. Engine tests use the fixture factories, never raw struct literals, and go through the public apply-action entry point.
- **Deck preference (12.4):** a users column defaulting to the French deck via migration `000020` with a reversing down migration, persisted through the existing preferences endpoint alongside language, deriving asset paths from deck + card ID with no lookup table. The alternative deck is Croatian/German-suited, free, purely visual, with no gameplay effect — a player setting, explicitly not a purchasable cosmetic.

## UX & Interaction Patterns

- **Croatian bidding** renders a free four-suit choice with no candidate card in *both* rounds; Bitola's candidate presentation and round-2 candidate-disable behaviour stay untouched. The round-2 reveal is visible to its owner only.
- **The Croatian declaration phase** prompts the active seat only and cannot be dismissed or closed by backdrop, consistent with other game prompts; an expired timer auto-skips and the phase continues; reconnecting restores the player into the phase with their own answer intact. Multiple melds per player must render in the reveal, anchored correctly when one player contributes several.
- **Deck picker** mirrors the language section: a labelled radio set with the current choice marked, applying immediately with no reload or match interruption, reverting on persistence failure. Every card surface follows the active deck — hand, trick area, card flight, deal animation, and every trump, declaration, and Belote prompt and reveal — while card IDs, geometry, states, glow, the face-down back, and all test IDs stay unchanged. Screen-reader labels name suits as the active deck depicts them, in the player's language; the unauthenticated landing page keeps the French deck.
- **Rules reference** (shipped) is a lobby nav tab and, in-match, a persistent bottom-right icon opening a dismissible overlay that does not interrupt play. **Croatian rooms must be identifiable** in the lobby, room preview, and match history.

## Cross-Story Dependencies

- **Sequencing is load-bearing: 12.1 → 12.5 → 12.6 → 12.8, with 12.4 fully independent.** The first three are built and 12.7 is dropped, so enablement (12.8) is next — and it still ships **last**, because exposing the room option before the variant is complete ships a broken game.
- **No rule *divergence* remains to build.** Every Croatian rule is already implemented. What 12.8 still owes the engine is one deliberately deferred decision: what to auto-pick for an absent dealer who is forbidden to pass under `AllPassDealerMustPick`. Today that auto-pass is rejected and the match layer re-arms on an already-elapsed deadline, so it HOT SPINS — unreachable only because the variant is unselectable. See `TODO(croatian-enablement)` in the session manager's timer-expiry switch.
- 12.8 needs 12.1 / 12.5 / 12.6 shipped (12.7 is not a dependency — hanging points was only ever a Bitola change, and Croatian's tie rule is what already ships). It reconciles the already-`done` rules-reference locale content against real engine behaviour, enables the variant server-side and in the create-room dropdown, and verifies bots bid with no candidate, answer the declaration phase, and read the config rather than a variant name.
- 12.4 is **blocked on procurement, not engineering**: Croatian/German-suited artwork must be licensed compatibly and generated through the existing recolour script with provenance recorded. It also renegotiates a frozen card-path contract (a change-log entry, not a silent override) and is the moment to unify the three-way duplication in auth-envelope user construction, since all three files are open anyway.
- The multiple-meld reveal defect was absorbed by 12.5. The hanging-points rule returned to deferred on 2026-08-20 and is no longer scheduled. "Belote across all eight of a suit" stays deferred and applies to *both* variants — rules-page text only, unimplemented for Bitola too.
