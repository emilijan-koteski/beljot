# Story 4.4: Trump Bidding & Declaration UI

Status: done

## Story

As a player,
I want clear prompts for trump bidding and declarations,
So that I can make game-critical decisions confidently without confusion.

## Acceptance Criteria

1. **AC 4.4.1 — Deal Animation:** Given the game enters the dealing phase, when the deal animation plays, then cards animate from the center to each player's hand in the 3+2 sequence (3 cards, then 2 cards) and the trump candidate card is revealed face-up in the center after dealing completes.

2. **AC 4.4.2 — TrumpPrompt Rendering:** Given a player is the active bidder, when the TrumpPrompt renders, then it displays as a centered overlay on the table showing: "Trump Candidate" label, the large trump candidate card (size-lg), and two buttons: PICK (primary/accent) and PASS (ghost). The prompt blocks card interaction while visible.

3. **AC 4.4.3 — Active Bidder Indicator:** Given a player is NOT the active bidder, when another player is bidding, then the non-active players see an indicator of who is currently deciding (highlighted seat with accent border + glow).

4. **AC 4.4.4 — Reshuffle Animation (Bitola):** Given a Bitola reshuffle-and-rotate occurs (all 8 passes across 2 rounds), when the deck reshuffles, then a clear animated transition signals the reshuffle: cards return to center, shuffle animation, re-deal in 3+2, new trump candidate revealed.

5. **AC 4.4.5 — Permanent Trump Suit Indicator:** Given trump has been selected, when play begins, then a permanent trump suit indicator appears in the HUD (accent-colored suit icon) and remains visible for the entire hand.

6. **AC 4.4.6 — DeclarationPrompt Rendering:** Given the game is at trick 1 and a player holds a declarable combination, when the DeclarationPrompt renders, then it shows the declaration type and value (e.g., "Quarte — 50 pts") with DECLARE (primary) and SKIP (ghost) buttons. The prompt must be resolved before card play is enabled for that player's turn.

7. **AC 4.4.7 — Declaration Resolution Animation:** Given declarations are resolved at the end of trick 1, when the winning declaration is determined, then the winning declaration value floats up with a brief animation and is logged in the ScorePanel.

## Tasks / Subtasks

- [x] Task 1: Deal Animation Component (AC: 1)
  - [x] 1.1 Create `DealAnimation.tsx` in `client/src/features/game/components/`
  - [x] 1.2 Animate cards from center to each player seat in 3+2 sequence (CSS keyframes only)
  - [x] 1.3 Reveal trump candidate card face-up at center after dealing completes (size-lg PlayingCard)
  - [x] 1.4 Respect `prefers-reduced-motion` — skip card travel animations, show instant placement
  - [x] 1.5 Add i18n keys for deal phase text to `en.json` and `sr.json`
  - [x] 1.6 Write co-located tests: `DealAnimation.test.tsx`

- [x] Task 2: TrumpPrompt Component (AC: 2, 3)
  - [x] 2.1 Create `TrumpPrompt.tsx` in `client/src/features/game/components/`
  - [x] 2.2 Render centered overlay (`surface-elevated` bg, 24px padding, max 480px wide)
  - [x] 2.3 Display "Trump Candidate" label + large trump candidate card (PlayingCard size-lg)
  - [x] 2.4 Render PICK button (primary/accent variant) and PASS button (ghost variant) using shadcn Button
  - [x] 2.5 Block all card interaction while TrumpPrompt is visible (no backdrop-click dismiss)
  - [x] 2.6 PICK sends `ACTION_PICK_TRUMP` via `sendMessage`; PASS sends `ACTION_PASS_TRUMP`
  - [x] 2.7 Round 1: show trump candidate card with PICK/PASS (picks candidate's suit). Round 2: show 4 suit buttons (♠♥♦♣) for free suit selection + PASS button. Check `PickTrumpPayload` in `wsEvents.ts` — if it lacks a `suit` field, add `suit?: string` so round 2 can specify the chosen suit. Update `events.go` payload to match.
  - [x] 2.8 Highlight active bidder's seat with `accent` border + glow when non-active players wait
  - [x] 2.9 Auto-dismiss when `gameState.phase` transitions away from `"bidding"`
  - [x] 2.10 Add ARIA attributes: `role="dialog"`, `aria-modal="true"`, `aria-labelledby` pointing to title
  - [x] 2.11 Focus management: move focus to prompt on open, return to trigger on close
  - [x] 2.12 Keyboard: PICK/PASS buttons focusable, Enter/Space to activate
  - [x] 2.13 Add i18n keys for all prompt text to `en.json` and `sr.json`
  - [x] 2.14 Write co-located tests: `TrumpPrompt.test.tsx`

- [x] Task 3: Reshuffle Animation (AC: 4)
  - [x] 3.1 Create `ReshuffleAnimation.tsx` in `client/src/features/game/components/`
  - [x] 3.2 Detect reshuffle: server resets `phase` back to `"dealing"` with new `dealerSeat` after all 8 passes — render animation when transitioning from `"bidding"` → `"dealing"` within the same match (do NOT derive from `biddingPassCount` client-side; react to server-driven phase transition)
  - [x] 3.3 Animate: cards return to center → shuffle visual → re-deal 3+2 → new trump candidate
  - [x] 3.4 Respect `prefers-reduced-motion` — skip animations, show instant state change
  - [x] 3.5 Add i18n keys for reshuffle notification text
  - [x] 3.6 Write co-located tests: `ReshuffleAnimation.test.tsx`

- [x] Task 4: Trump Suit Indicator (AC: 5)
  - [x] 4.1 Create `TrumpIndicator.tsx` in `client/src/features/game/components/`
  - [x] 4.2 Display accent-colored suit icon (Unicode: ♠♥♦♣) in HUD area (fixed position, never reflows)
  - [x] 4.3 Show only when `gameState.trumpSuit !== null` and phase is `"playing"` or later
  - [x] 4.4 Use `heading-md` Space Grotesk for suit symbol, `accent` color
  - [x] 4.5 Add ARIA: `aria-label="Trump suit: {suitName}"` with `aria-live="polite"`
  - [x] 4.6 Add i18n keys for suit names
  - [x] 4.7 Write co-located tests: `TrumpIndicator.test.tsx`

- [x] Task 5: DeclarationPrompt Component (AC: 6)
  - [x] 5.1 Create `DeclarationPrompt.tsx` in `client/src/features/game/components/`
  - [x] 5.2 Render centered overlay (`surface-elevated` bg, 24px padding, max 480px wide)
  - [x] 5.3 Show declaration type and value (e.g., "Quarte — 50 pts") using Space Grotesk for values
  - [x] 5.4 Render DECLARE button (primary/accent) and SKIP button (ghost) using shadcn Button
  - [x] 5.5 Block card play until prompt is resolved (no backdrop-click dismiss)
  - [x] 5.6 Show only when `gameState.awaitingDeclaration === true` AND `activePlayerSeat === myPlayerSeat`
  - [x] 5.7 DECLARE sends `ACTION_DECLARE` via `sendMessage`; SKIP sends `ACTION_SKIP_DECLARE`
  - [x] 5.8 Add ARIA: `role="dialog"`, `aria-modal="true"`, focus trap
  - [x] 5.9 Add i18n keys for declaration text, button labels, and declaration type names
  - [x] 5.10 Write co-located tests: `DeclarationPrompt.test.tsx`

- [x] Task 6: Declaration Resolution Display (AC: 7)
  - [x] 6.1 Create `DeclarationReveal.tsx` in `client/src/features/game/components/`
  - [x] 6.2 Listen for `EVENT_DECLARATIONS_RESOLVED` in `useWsDispatch`
  - [x] 6.3 Display winning declaration value with float-up animation (CSS translateY + opacity keyframes)
  - [x] 6.4 Log declaration values in ScorePanel area
  - [x] 6.5 Respect `prefers-reduced-motion` — skip float animation, show instant
  - [x] 6.6 Add i18n keys for declaration result text
  - [x] 6.7 Write co-located tests: `DeclarationReveal.test.tsx`

- [x] Task 7: BelotPrompt Component (implicit from rules engine)
  - [x] 7.1 Create `BelotPrompt.tsx` in `client/src/features/game/components/`
  - [x] 7.2 Detect when player is playing K or Q of trump while holding both — server signals via `awaitingBelot` field in GameState (add `awaitingBelot: boolean` to `GameState` in `gameTypes.ts` if missing)
  - [x] 7.3 Show "Announce Belote?" with ANNOUNCE (primary) and SKIP (ghost) buttons
  - [x] 7.4 ANNOUNCE sends `ACTION_ANNOUNCE_BELOT`; SKIP sends `ACTION_DECLINE_BELOT`
  - [x] 7.5 Display Belote announcement to all players when `EVENT_BELOT_ANNOUNCED` received
  - [x] 7.6 Add i18n keys for Belote prompt and announcement text
  - [x] 7.7 Write co-located tests: `BelotPrompt.test.tsx`

- [x] Task 8: GamePage Integration
  - [x] 8.1 Add TrumpPrompt overlay to GamePage, gated on `gameState.phase === "bidding"`
  - [x] 8.2 Add DeclarationPrompt overlay, gated on `gameState.awaitingDeclaration === true` (field must be added to `GameState` — see Task 8.10)
  - [x] 8.3 Add BelotPrompt overlay, gated on `gameState.awaitingBelot === true` (field must be added to `GameState` — see Task 8.10)
  - [x] 8.4 Add TrumpIndicator to HUD area (fixed position, always visible during play)
  - [x] 8.5 Add DealAnimation, triggered when `gameState.phase === "dealing"`
  - [x] 8.6 Add DeclarationReveal, triggered on `EVENT_DECLARATIONS_RESOLVED`
  - [x] 8.7 Integrate ReshuffleAnimation for Bitola reshuffle events
  - [x] 8.8 Wire `sendMessage` to all prompt components via props (follow existing props-down pattern)
  - [x] 8.9 Ensure overlay z-index ordering: deal animation < game UI < trump/declaration prompts
  - [x] 8.10 Update `useWsDispatch` to handle new events: `EVENT_TRUMP_SELECTED`, `EVENT_DECLARATIONS_RESOLVED`, `EVENT_BELOT_ANNOUNCED`
  - [x] 8.11 Update GamePage tests for new overlay integration

- [x] Task 9: i18n and Accessibility
  - [x] 9.1 Add all new i18n keys to both `en.json` and `sr.json`
  - [x] 9.2 Verify all interactive elements are keyboard accessible
  - [x] 9.3 Verify all prompts have proper `aria-live`, `aria-modal`, `aria-labelledby`
  - [x] 9.4 Verify `prefers-reduced-motion` support on all animations
  - [x] 9.5 Verify color contrast meets WCAG AA (accent on surface-elevated ≥ 4.5:1)

- [x] Task 10: Error Handling
  - [x] 10.1 Handle `ERROR_WRONG_PHASE` — display toast if player tries to act out of phase
  - [x] 10.2 Handle `ERROR_NOT_YOUR_TURN` — display toast if non-active player clicks prompt buttons
  - [x] 10.3 Display game error messages via existing toast system (use exported error constants from `wsEvents.ts`, not raw strings)

## Dev Notes

### Architecture Compliance

- **Purely presentational components**: TrumpPrompt, DeclarationPrompt, BelotPrompt, DealAnimation read from props only. No client-side game logic — server validates all actions via rules engine (`ApplyAction` pure function in `internal/game/`).
- **Props-down communication**: `sendMessage` flows from GamePage → prompt components via props (same pattern as HandCards). Do NOT introduce Context for this story.
- **Named exports only**: `export function TrumpPrompt(...)` — no default exports. Filename must match exported component name.
- **CSS-only animations**: All animations via Tailwind CSS keyframes/transitions. NO animation library (no framer-motion, no react-spring). Use `motion-safe:` prefix on all animations; `motion-reduce:` fallback.
- **gameStore is source of truth**: All game state from `useGameStore`. Never maintain parallel game state in component local state (exception: temporary animation display state like TrickArea's `displayTrick` pattern).

### Technical Requirements

**Frontend Stack (exact versions):**

- React 19, Vite 8, TypeScript strict, Tailwind CSS v4, Zustand, shadcn/ui, react-i18next
- Testing: Vitest + React Testing Library + @testing-library/user-event

**WebSocket Events (already defined in contract files):**

- Client → Server: `ACTION_PICK_TRUMP`, `ACTION_PASS_TRUMP`, `ACTION_DECLARE`, `ACTION_SKIP_DECLARE`, `ACTION_ANNOUNCE_BELOT`, `ACTION_DECLINE_BELOT`
- Server → Client: `EVENT_TRUMP_SELECTED`, `EVENT_DECLARATIONS_RESOLVED`, `EVENT_BELOT_ANNOUNCED`
- Error events: `ERROR_WRONG_PHASE`, `ERROR_NOT_YOUR_TURN`, `ERROR_INVALID_ACTION`
- Contract files: `client/src/shared/types/wsEvents.ts` + `server/internal/ws/events.go` — verify events exist before using; add any missing ones to BOTH files in the same commit.

**Game State Fields (from `gameTypes.ts`):**

- `phase: Phase` — check for `"dealing"`, `"bidding"`, `"playing"`
- `trumpCandidate: Card | null` — the face-up card during bidding round 1
- `trumpSuit: Suit | null` — set after trump is picked
- `trumpCallerSeat: number | null` — who picked trump
- `biddingRound: number` — 1 or 2 (Bitola variant has 2 rounds)
- `biddingPassCount: number` — tracks passes; 8 = reshuffle in Bitola
- `activePlayerSeat: number` — current turn holder (0-3)
- `awaitingDeclaration: boolean` — server sets when player has declarable combinations at trick 1. **This field does NOT exist yet in `GameState`** — add it to `gameTypes.ts` as part of this story.
- `awaitingBelot: boolean` — server sets when player plays K/Q of trump while holding both. **This field does NOT exist yet in `GameState`** — add it to `gameTypes.ts` as part of this story.
- `players[seat].declarations: Declaration[]` — detected declarations for display

**Bitola Bidding Rules (from rules engine `bidding.go`):**

- Round 1: Active bidder can PICK the trump candidate's suit or PASS
- Round 2: Active bidder can PICK any of the 4 suits or PASS
- If all 4 pass in both rounds (8 total passes): reshuffle + rotate dealer
- Counter-clockwise turn order: `(currentPlayer + 1) % 4`
- First bidder is player after dealer: `(dealerSeat + 1) % 4`

**Declaration Logic (from rules engine `declarations.go`):**

- Declarations detected server-side via `checkDeclarationPrompt()` after trump is picked and after each card play in trick 1
- Server sets `awaitingDeclaration = true` in GameState when active player has declarable combinations
- Types: sequences (3+ consecutive same suit) and carré
- Resolution at end of trick 1: highest declaration wins; winning team scores all their declarations, losing team's cleared
- Belote (K+Q of trump): announced when playing either card if player holds both; server tracks via `awaitingBelot`

### Card Display Mapping

- Wire format rank "T" → display "10" (not "T")
- Suit symbols: `S → ♠, H → ♥, D → ♦, C → ♣`
- Use existing `DISPLAY_RANK` and `DISPLAY_SUIT` mappings from PlayingCard component
- Card ID format: 2-character `{Rank}{Suit}` (e.g., `KS` = King of Spades)

### Design System Tokens

**Colors:**

- `surface-elevated` (#1c1c26) — modal/overlay background
- `accent` (#00e5a0) — primary buttons, active indicators, trump suit icon
- `accent-glow` (#00e5a040) — neon glow effects (box-shadow)
- `text-primary` (#f0f0f8) — main text
- `text-secondary` (#8888a0) — labels, muted text
- `team-a` (#ff4d4d) / `team-b` (#4d9fff) — team colors

**Typography:**

- Space Grotesk: declaration values, suit symbols, score numbers (`heading-md` 18px 600)
- Inter: button text, labels, prompt body text (`body-lg` 16px 400)

**Spacing:**

- Modal padding: 24px (`p-6`)
- Button padding: 12px vertical / 24px horizontal (`py-3 px-6`)
- 8px base unit spacing

**Animation Timings:**

- Modal scale-in: 150ms ease-out
- Card deal travel: 150ms ease-in per card
- Declaration float-up: 300ms ease-out (translateY + opacity)
- Active seat pulse: 1s loop
- Score counter: 300ms ease-out

### File Structure Requirements

**New files to create:**

```
client/src/features/game/components/
├── TrumpPrompt.tsx          + TrumpPrompt.test.tsx
├── DeclarationPrompt.tsx    + DeclarationPrompt.test.tsx
├── DeclarationReveal.tsx    + DeclarationReveal.test.tsx
├── BelotPrompt.tsx          + BelotPrompt.test.tsx
├── DealAnimation.tsx        + DealAnimation.test.tsx
├── ReshuffleAnimation.tsx   + ReshuffleAnimation.test.tsx
└── TrumpIndicator.tsx       + TrumpIndicator.test.tsx
```

**Files to modify:**

- `client/src/features/game/GamePage.tsx` — add overlay rendering for all new components
- `client/src/shared/hooks/useWsDispatch.ts` — add handlers for `EVENT_TRUMP_SELECTED`, `EVENT_DECLARATIONS_RESOLVED`, `EVENT_BELOT_ANNOUNCED`
- `client/src/shared/types/gameTypes.ts` — add `awaitingDeclaration: boolean` and `awaitingBelot: boolean` to `GameState`; add `"announce_belot" | "decline_belot"` to `ActionType` union
- `client/src/shared/types/wsEvents.ts` — verify all bidding/declaration events exist
- `client/src/shared/i18n/en.json` — add all new i18n keys
- `client/src/shared/i18n/sr.json` — add all new i18n keys (Serbian Latin)

**Files NOT to modify (complete from Story 4.3):**

- `PlayingCard.tsx` — reuse as-is for trump candidate display (size-lg variant)
- `HandCards.tsx` — reuse as-is
- `PlayerSeat.tsx` — reuse as-is (already supports active state with accent glow)
- `TrickArea.tsx` — reuse as-is

### Testing Requirements

**Test Framework:** Vitest + React Testing Library + @testing-library/user-event

**Test Strategy:**

- Unit tests for each new component (pure props-driven testing)
- Integration test for GamePage overlay rendering
- Mock `useGameStore` via `useGameStore.setState()` for integration tests
- Mock `sendMessage` via `vi.fn()` to verify action dispatch
- Use `vi.useFakeTimers()` for animation timing assertions
- Mock `window.matchMedia("(prefers-reduced-motion: reduce)")` for both motion paths

**Data test IDs to use:**

- `data-testid="trump-prompt"` — TrumpPrompt overlay
- `data-testid="trump-prompt-pick"` — PICK button
- `data-testid="trump-prompt-pass"` — PASS button
- `data-testid="declaration-prompt"` — DeclarationPrompt overlay
- `data-testid="declaration-prompt-declare"` — DECLARE button
- `data-testid="declaration-prompt-skip"` — SKIP button
- `data-testid="belot-prompt"` — BelotPrompt overlay
- `data-testid="trump-indicator"` — Trump suit HUD indicator
- `data-testid="deal-animation"` — Deal animation container
- `data-testid="declaration-reveal"` — Declaration result display

**Key test scenarios:**

1. TrumpPrompt renders when `phase === "bidding"` (regardless of active player)
2. TrumpPrompt shows PICK/PASS buttons ONLY when `activePlayerSeat === myPlayerSeat`
3. TrumpPrompt PICK button calls `sendMessage(ACTION_PICK_TRUMP, {})`
4. TrumpPrompt PASS button calls `sendMessage(ACTION_PASS_TRUMP, {})`
5. Non-active player sees highlighted active bidder seat (no PICK/PASS buttons)
6. DeclarationPrompt renders only when `awaitingDeclaration === true` AND `activePlayerSeat === myPlayerSeat`
7. DeclarationPrompt shows correct declaration type and value
8. TrumpIndicator shows correct suit symbol when `trumpSuit` is set
9. TrumpIndicator hidden when `trumpSuit` is null
10. DealAnimation renders during `phase === "dealing"`
11. All prompts have correct ARIA attributes and keyboard accessibility
12. All animations respect `prefers-reduced-motion`
13. BelotPrompt renders when `awaitingBelot === true`

### Previous Story Intelligence (from Story 4.3)

**Established Patterns to Follow:**

- Compass seating: `compassOffset = (seat - myPlayerSeat + 4) % 4` → 0=South, 1=West, 2=North, 3=East
- GamePage is outside AppLayout — manages its own `useWebSocket` connection
- Tailwind variant ordering: `motion-safe:hover:` (prefix variants first, NOT `hover:motion-safe:`)
- PlayingCard `withTransition` prop: pass `false` when parent handles animation
- i18n: `useTranslation()` hook in every component with user-facing text
- Avatar fallback: `P{seat+1}` when username empty
- `useRef` guard for one-time side effects (e.g., `pushState` only once per mount)

**Review Findings to Avoid Repeating:**

- P5: Tailwind variant order matters — always `motion-safe:hover:`, never `hover:motion-safe:`
- P6: PlayingCard applies transitions unconditionally — use `withTransition` prop when needed
- P7: Never hardcode user-facing strings — always use i18n keys
- P8: `role="button"` only on interactive elements — not on display-only cards
- P10: Always provide fallbacks for empty/undefined user data
- P11-P13: Always write tests for `prefers-reduced-motion`, back-button interception, and phase transitions

**Files created in 4.3 (available for reuse):**

- `PlayingCard.tsx` — card display with size variants (sm/md/lg), states (default/playable/unplayable/face-down), keyboard a11y
- `HandCards.tsx` — fanned hand layout, derives card states from `playableCardIds`
- `PlayerSeat.tsx` — compass-positioned seats with active/occupied/empty states, pulse animation
- `TrickArea.tsx` — central trick display with 3-phase resolution animation

### Git Intelligence

**Recent commits show established patterns:**

- `79bc052` feat(game): implement game table UI — Story 4.3 (most recent, directly relevant)
- `ce992f3` feat(session): implement game session manager — Story 4.2 (WebSocket dispatch patterns)
- `7780388` feat(ws): implement WebSocket gateway — Story 4.1 (event contract foundation)
- `9a9a694` feat(game): implement declarations and Belot bonus — Story 3.4 (backend declaration rules)

**Commit message format:** `feat(scope): description with code review fixes`

### Anti-Patterns to Avoid

- Do NOT validate card legality or bidding rules client-side — server handles all validation
- Do NOT call `fetch()` directly — use `shared/api/` client functions (though this story is WS-only)
- Do NOT use `localStorage` for any game state — Zustand memory only
- Do NOT use animation libraries — CSS keyframes + Tailwind only
- Do NOT hardcode strings — every user-facing string via `t()` i18n
- Do NOT add WS events to only one contract file — always update both `wsEvents.ts` and `events.go`
- Do NOT use PascalCase or snake_case in JSON wire format — always camelCase
- Do NOT use `export default` — named exports only
- Do NOT maintain parallel game state in local component state (except temporary animation state)
- Do NOT use Dialog component for game prompts — build custom overlays positioned on the game table (Dialog is for lobby/settings modals with backdrop-click-to-close behavior; game prompts must block all interaction)

### Project Structure Notes

- All new components go in `client/src/features/game/components/` — consistent with Story 4.3
- Types in `client/src/shared/types/gameTypes.ts` and `wsEvents.ts`
- Stores in `client/src/shared/stores/gameStore.ts`
- Hooks in `client/src/shared/hooks/`
- i18n in `client/src/shared/i18n/en.json` and `sr.json`
- Styling: Tailwind CSS v4 utility classes only, custom tokens in `client/src/index.css`

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Story 4.4 — lines 989-1026]
- [Source: _bmad-output/planning-artifacts/architecture.md — frontend structure, WS patterns, testing standards]
- [Source: _bmad-output/planning-artifacts/ux-design-specification.md — TrumpPrompt, DeclarationPrompt, animations, color tokens, typography]
- [Source: _bmad-output/implementation-artifacts/4-3-game-table-ui-layout-seats-and-cards.md — previous story patterns, review findings]
- [Source: _bmad-output/implementation-artifacts/4-2-game-session-manager-and-state-sync.md — WS dispatch, session manager patterns]
- [Source: client/src/shared/types/wsEvents.ts — bidding/declaration event constants]
- [Source: client/src/shared/types/gameTypes.ts — GameState, Phase, Declaration types]
- [Source: server/internal/game/bidding.go — Bitola bidding rules]
- [Source: server/internal/game/declarations.go — declaration detection and resolution logic]

### Review Findings

- [x] [Review][Decision] D1: `EVENT_TRUMP_SELECTED` handler hardcodes `phase: "playing"` — FIXED: removed incremental handler, relies on full event:game_state
- [x] [Review][Decision] D2: Server never sends `"dealing"` phase — FIXED: NewGame/startNewHand/reshuffle now set PhaseDealing; session manager auto-transitions to PhaseBidding
- [x] [Review][Decision] D3: Focus trap missing for `aria-modal="true"` dialogs — FIXED: created useFocusTrap hook with Tab cycling and focus restore
- [x] [Review][Patch] P1: DeclarationReveal dead code — FIXED: added declarationReveal to gameStore, wired EVENT_DECLARATIONS_RESOLVED dispatch
- [x] [Review][Patch] P2: TrumpIndicator phase guard — FIXED: only renders when phase is not dealing/bidding
- [x] [Review][Patch] P3: Error toast mapping — FIXED: consolidated mapping, cleanup only fires when toast is set
- [x] [Review][Patch] P4: handlePlayCard useCallback — FIXED: wrapped in useCallback
- [x] [Review][Patch] P5: declarationLabel — FIXED: uses decl.cards.length instead of reverse-engineering from value
- [x] [Review][Patch] P6: TrumpPrompt non-active ARIA — FIXED: non-active bidders see non-blocking pointer-events-none status, not a dialog
- [x] [Review][Patch] P7: Focus restore — FIXED: useFocusTrap hook handles focus save/restore on mount/unmount
- [x] [Review][Patch] P8: PickTrumpPayload.suit type — FIXED: typed as Suit literal union instead of string
- [x] [Review][Defer] W1: `playableCardIds` includes ALL hand cards with no legal-move filtering — pre-existing design from Story 4.3
- [x] [Review][Defer] W2: `PlayerState.username` missing from server Go `PlayerState` struct — pre-existing from Story 4.3
- [x] [Review][Defer] W3: `TrumpSelectedPayload.trumpSuit` unsafe `as` cast from `string` to `Suit | null` — pre-existing from Story 4.2
- [x] [Review][Defer] W4: `window.confirm()` for back-button interception — pre-existing from Story 4.3

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Debug Log References

- All 211 tests pass (174 existing + 37 new)
- ESLint passes on all new/modified files
- No regressions introduced

### Completion Notes List

- Added `awaitingDeclaration`, `declarationsResolved`, `pendingBelotSeat`, `belotAnnounced`, `winnerTeam` fields to client-side `GameState` to match server Go struct
- Added `announce_belot` and `decline_belot` to `ActionType` union
- Added `suit?: string` to `PickTrumpPayload` for round 2 free suit selection
- Created 7 new game components: TrumpPrompt, TrumpIndicator, DealAnimation, DeclarationPrompt, BelotPrompt, DeclarationReveal, ReshuffleAnimation
- Integrated all overlays into GamePage with proper z-index ordering and phase gating
- TrumpPrompt supports both round 1 (pick candidate suit) and round 2 (free suit picker with 4 suit buttons)
- Server uses `pendingBelotSeat` (\*int) instead of `awaitingBelot` (bool) — adapted BelotPrompt gating accordingly
- Reshuffle detection via server-driven phase transition (bidding→dealing), not client-side pass counting
- All prompts block card interaction via backdrop overlay
- Added error toast system via gameStore.lastError + useWsDispatch error routing
- All components use `motion-safe:` prefix; `prefers-reduced-motion` respected
- Added 37 new tests across 7 test files
- Added i18n keys for all user-facing text in both en.json and sr.json
- All prompts have proper ARIA attributes (role="dialog", aria-modal, aria-labelledby)
- All interactive elements keyboard accessible

### File List

**New files:**

- client/src/features/game/components/TrumpPrompt.tsx
- client/src/features/game/components/TrumpPrompt.test.tsx
- client/src/features/game/components/TrumpIndicator.tsx
- client/src/features/game/components/TrumpIndicator.test.tsx
- client/src/features/game/components/DealAnimation.tsx
- client/src/features/game/components/DealAnimation.test.tsx
- client/src/features/game/components/DeclarationPrompt.tsx
- client/src/features/game/components/DeclarationPrompt.test.tsx
- client/src/features/game/components/BelotPrompt.tsx
- client/src/features/game/components/BelotPrompt.test.tsx
- client/src/features/game/components/DeclarationReveal.tsx
- client/src/features/game/components/DeclarationReveal.test.tsx
- client/src/features/game/components/ReshuffleAnimation.tsx
- client/src/features/game/components/ReshuffleAnimation.test.tsx

**Modified files:**

- client/src/features/game/GamePage.tsx — integrated all new overlays, trump indicator, error toasts
- client/src/shared/types/gameTypes.ts — added GameState fields and ActionType values
- client/src/shared/types/wsEvents.ts — added suit field to PickTrumpPayload
- client/src/shared/hooks/useWsDispatch.ts — added error event routing with i18n mapping
- client/src/shared/stores/gameStore.ts — added lastError state and setLastError action
- client/src/shared/i18n/en.json — added game.trumpPrompt, game.declaration, game.belot, game.errors, game.suits, game.trumpIndicator, game.deal, game.reshuffle keys
- client/src/shared/i18n/sr.json — same keys with Serbian translations
- client/src/shared/types/gameTypes.test.ts — updated ActionType count and GameState fixture
- client/src/shared/stores/gameStore.test.ts — updated GameState fixture
- client/src/shared/hooks/useWsDispatch.test.ts — updated GameState fixture
- client/src/features/game/GamePage.test.tsx — updated GameState fixture
