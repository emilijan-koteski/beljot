# Story 4.3: Game Table UI — Layout, Seats & Cards

Status: done

## Story

As a player,
I want to see a game table with four seats, my hand of cards, and a central trick area,
So that the game feels like sitting at a real card table.

## Acceptance Criteria

1. **Given** a player enters the game view
   **When** GamePage renders
   **Then** the layout fills the full viewport with no scroll and no overflow
   **And** four PlayerSeat components are positioned at compass points: bottom = you (South), right = East, top = North (teammate), left = West
   **And** the TrickArea is centered, occupying ~25% of viewport width
   **And** match chat is accessible as a collapsible sidebar on the right edge

2. **Given** a PlayerSeat is occupied
   **When** it renders
   **Then** it shows the player's username, team color border (Team A or Team B), and avatar placeholder
   **And** the current player's own seat (South) is slightly larger with a "You" label

3. **Given** it is a player's turn
   **When** their seat becomes active
   **Then** the seat displays an `accent` border with `accent-glow` shadow and a subtle pulse animation
   **And** the counter-clockwise turn transition is visually indicated

4. **Given** a player's hand of cards is displayed
   **When** HandCards renders
   **Then** cards are fanned horizontally at the bottom edge with overlapping at a fixed offset
   **And** cards spread wider as the count decreases through the hand

5. **Given** it is the player's turn
   **When** their hand activates
   **Then** playable cards lift (+4px translateY) with `accent-glow` box-shadow and cursor pointer
   **And** unplayable cards dim to 40% opacity with cursor not-allowed
   **And** hovering a playable card adds +2px additional lift

6. **Given** a player clicks a playable card
   **When** the click is registered
   **Then** a single click immediately sends `action:play_card` to the server (no confirmation dialog)
   **And** the card animates from the hand to the TrickArea at the player's compass position (150ms ease-in)

7. **Given** other players' cards
   **When** they are displayed
   **Then** they show as face-down cards (card back design) with count visible

8. **Given** cards are played into the trick
   **When** the TrickArea updates
   **Then** up to 4 cards are displayed in their seat positions (N/S/E/W quadrants)
   **And** when the 4th card is played, a brief pause (~1 second) shows all 4 cards, the winning card highlights with `accent` glow, then cards sweep to the winning team's pile

## Tasks / Subtasks

- [x] Task 1: Fix game route and update RoomLobby navigation (AC: 1)
  - [x] In `client/src/App.tsx`: change route from `path="/game"` to `path="/game/:roomId"` to match RoomLobby's `navigate(\`/game/${room.id}\`)`
  - [x] GamePage reads `roomId` param via `useParams()` from `react-router` as a routing hint; authoritative roomId comes from gameStore once `event:game_state` arrives

- [x] Task 2: Implement GamePage layout (AC: 1, 7)
  - [x] Replace stub `GamePage.tsx` with full viewport layout: `h-screen w-screen overflow-hidden relative bg-background`
  - [x] Import and call `useWebSocket` with `onMessage` set to the dispatch function from `useWsDispatch()` — this connects the WS pipeline for the game page (GamePage is outside AppLayout, so it needs its own WS connection)
  - [x] Position compass seats using absolute positioning (see Architecture Notes below)
  - [x] Place TrickArea at center: `absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2`
  - [x] Place HandCards pinned to bottom edge: `absolute bottom-4 left-1/2 -translate-x-1/2`
  - [x] Place ScorePanel placeholder (fixed top-left): `absolute top-4 left-4` — full ScorePanel in Story 4.6
  - [x] Place TrumpIndicator placeholder (fixed top-right): `absolute top-4 right-16` — wired in Story 4.4
  - [x] Add collapsible chat sidebar shell: toggle button (`absolute right-0 top-1/2`) opens an empty sidebar panel on the right edge; real chat wired in Story 6.2
  - [x] Show a centered loading state when `gameState === null` (WS not yet delivered first snapshot)
  - [x] On `phase === "match_end"`: call `clearGame()` and navigate to `/lobby` after a 2s delay
  - [x] Pass `sendMessage` from `useWebSocket` down to HandCards/PlayingCard via props (not context in this story)
  - [x] Add browser back-button interception: `useEffect` with `window.history.pushState` + `popstate` listener to prevent accidental navigation away during an active game — show a `window.confirm()` dialog before leaving

- [x] Task 3: Derive `myPlayerSeat` on first gameState (AC: 2)
  - [x] In GamePage, `useEffect` on `gameState`: if `myPlayerSeat === null && gameState`, find player with `userId === authStore.user.id` in `gameState.players` and call `setMyPlayerSeat(player.seat)`
  - [x] Compute compass mapping: `compassPosition(seat, myPlayerSeat) = (seat - myPlayerSeat + 4) % 4` → 0=South, 1=East, 2=North, 3=West (counter-clockwise; seat indices increment counter-clockwise)

- [x] Task 4: Create `PlayingCard` component (AC: 4, 5, 6, 7)
  - [x] Create `client/src/features/game/components/PlayingCard.tsx`
  - [x] Props: `card: Card | null` (null = face-down), `state: "default" | "playable" | "unplayable" | "face-down"`, `size: "sm" | "md" | "lg"`, `onClick?: () => void`
  - [x] Face-up layout: rank (top-left + bottom-right) + suit symbol (center). Hearts/Diamonds: `text-red-500`. Spades/Clubs: `text-text-primary`
  - [x] Suit display symbols: ♠ for S, ♥ for H, ♦ for D, ♣ for C
  - [x] Rank display: `7`, `8`, `9`, `10` (not `T`), `J`, `Q`, `K`, `A` — the card ID uses `T` for ten but the UI renders `10`
  - [x] Sizes: `sm` = `w-10 h-14`, `md` = `w-14 h-20`, `lg` = `w-20 h-28`
  - [x] States via Tailwind:
    - `playable`: `motion-safe:translate-y-[-4px] shadow-[0_0_12px_var(--color-accent-glow)] cursor-pointer`; hover adds `motion-safe:translate-y-[-6px]`
    - `unplayable`: `opacity-40 cursor-not-allowed`
    - `face-down`: render card back (dark surface with repeating pattern or solid `bg-surface-elevated border border-border`)
  - [x] Transitions: `motion-safe:transition-transform motion-safe:duration-150` on all states in HandCards context
  - [x] `onClick` fires only when `state === "playable"`; sends `action:play_card` via sendMessage passed from GamePage (no local game logic)
  - [x] **Keyboard accessibility**: when `state === "playable"`, set `tabIndex={0}` and handle `onKeyDown` for Enter and Space keys (fire `onClick`). When not playable, set `tabIndex={-1}`
  - [x] **Focus ring**: `focus-visible:ring-2 focus-visible:ring-accent focus-visible:ring-offset-2 focus-visible:ring-offset-background` on playable cards
  - [x] `aria-label="[rank] of [suit]"` for face-up (e.g., `"King of Spades"`), `aria-label="face-down card"` for face-down; `aria-disabled="true"` when unplayable
  - [x] Card background: `bg-surface rounded-lg border border-border`
  - [x] NO `transition` CSS on cards in trick area (animation handled separately in TrickArea); add transitions only in HandCards state

- [x] Task 5: Create `HandCards` component (AC: 4, 5, 6)
  - [x] Create `client/src/features/game/components/HandCards.tsx`
  - [x] Props: `hand: Card[]`, `isMyTurn: boolean`, `playableCardIds: string[]`, `onPlayCard: (cardId: string) => void`
  - [x] Fan layout: cards absolutely positioned with `left` offset = `cardIndex * overlap`. Dynamic overlap: `overlap = Math.max(32, 56 - (hand.length - 1) * 4)px` → cards spread wider as count drops
  - [x] Container: `relative h-20 flex items-end`, width = `(hand.length - 1) * overlap + cardWidth`
  - [x] Card state: when `isMyTurn`, card is `playable` if cardId in `playableCardIds`, else `unplayable`; when not my turn, all cards are `default`
  - [x] `playableCardIds` is derived by GamePage from `gameState` — NOT computed in HandCards (no local game logic)
  - [x] Card ordering: preserve server-provided order; do NOT re-sort client-side
  - [x] `onPlayCard` calls `sendMessage(ACTION_PLAY_CARD, { cardId })` — wired in GamePage
  - [x] Helper to compute `cardId` from `Card` object: `` `${card.rank}${card.suit}` `` (e.g., `Card { rank: "K", suit: "S" }` → `"KS"`)

- [x] Task 6: Create `PlayerSeat` component (AC: 2, 3)
  - [x] Create `client/src/features/game/components/PlayerSeat.tsx`
  - [x] Props: `player: PlayerState | null`, `isSelf: boolean`, `isActive: boolean`, `teamColor: "teamA" | "teamB"`, `cardCount?: number`
  - [x] States:
    - Empty seat: dashed border, "Waiting..." label (`text-text-secondary`) — use i18n key `game.seat.waiting`
    - Occupied: avatar circle (initials from username) + username label + team color border
    - Active: `border-accent shadow-[0_0_16px_var(--color-accent-glow)] motion-safe:animate-pulse` (Tailwind `animate-pulse` with `motion-safe:` prefix)
    - Self seat: slightly larger (`scale-110`) with "You" badge below username — use i18n key `game.seat.you`
  - [x] Team colors: Team A = `border-team-a`, Team B = `border-team-b`
  - [x] Avatar: circle div, initials = first letter of username, uppercase. Background: `bg-surface-elevated`
  - [x] For non-self occupied seats: render `cardCount` face-down PlayingCard components stacked or show card count badge (e.g., "×8")
  - [x] Font: username label uses `font-body text-sm`, "You" badge uses `font-body text-xs`
  - [x] `aria-live="polite"` on the active state indicator region
  - [x] `aria-label="{username}, {team} team, {active/waiting}"` on the seat container

- [x] Task 7: Create `TrickArea` component (AC: 8)
  - [x] Create `client/src/features/game/components/TrickArea.tsx`
  - [x] Props: `trick: TrickCard[]`, `winnerSeat: number | null`, `myPlayerSeat: number`
  - [x] Layout: `relative w-[25vw] aspect-square` — cards placed in N/S/E/W quadrants using absolute offsets
  - [x] Card placement per compass position (relative to myPlayerSeat):
    - South (0): bottom-center
    - East (1): center-right
    - North (2): top-center
    - West (3): center-left
  - [x] Empty state: subtle oval outline (`border border-border rounded-full opacity-30`)
  - [x] `resolving` state: when `winnerSeat !== null` AND trick has 4 cards, apply `accent` glow to the winning card: `shadow-[0_0_20px_var(--color-accent)]`
  - [x] Trick resolution sequence (use `useEffect` watching `trick.length` and `winnerSeat`):
    - When 4th card arrives: set `resolving = true` for ~1000ms
    - After 1000ms: set `sweeping = true`, apply CSS transition that moves all cards off-screen toward winning team's corner (150ms) — use `motion-safe:transition-all motion-safe:duration-150`
    - After transition: call `onTrickResolved` prop (GamePage clears trick by receiving next game state via WS)
    - Keep internal `displayTrick` state during the 1000ms pause — don't immediately clear on WS `event:trick_resolved`
  - [x] `motion-reduce:` variant: skip the 1s pause and sweeping animation — immediately clear the trick

- [x] Task 8: Assemble GamePage with all components (AC: 1–8)
  - [x] Import PlayerSeat, HandCards, TrickArea into GamePage
  - [x] Wire `useWebSocket` and `useWsDispatch` (see "WebSocket Wiring in GamePage" dev note below for exact pattern)
  - [x] Compute `playableCardIds` in GamePage from `gameState`:
    - My turn = `gameState.activePlayerSeat === myPlayerSeat && gameState.phase === "playing"`
    - `playableCardIds`: pass ALL card IDs from my hand when it's my turn (full legal-move validation happens server-side; Story 4.4 adds declaration phase handling)
    - **Important**: Do NOT implement client-side card legality logic — the server validates every action via `error:illegal_play`
  - [x] Place 4 PlayerSeat components using compass positions derived from `myPlayerSeat`
  - [x] Render opponent cards: for North/West/East seats, pass `cardCount={player.hand.length}` to PlayerSeat (no actual card data)
  - [x] `onPlayCard` handler in GamePage: `sendMessage(ACTION_PLAY_CARD, { cardId })` — single send, no optimistic update

- [x] Task 9: Add i18n translation keys (AC: all)
  - [x] Add keys to `client/src/i18n/en.json` under `game` namespace:
    - `game.seat.waiting`: "Waiting..."
    - `game.seat.you`: "You"
    - `game.chat.title`: "Match Chat"
    - `game.chat.placeholder`: "Chat available soon"
    - `game.loading`: "Connecting to game..."
    - `game.leaveConfirm`: "Leave the game? You may lose your progress."
  - [x] Add same keys to `client/src/i18n/sr.json` with Serbian (Latin) translations:
    - `game.seat.waiting`: "Čeka se..."
    - `game.seat.you`: "Ti"
    - `game.chat.title`: "Čet u meču"
    - `game.chat.placeholder`: "Čet uskoro dostupan"
    - `game.loading`: "Povezivanje na igru..."
    - `game.leaveConfirm`: "Napusti igru? Možeš izgubiti napredak."

- [x] Task 10: Write tests (AC: all)
  - [x] `client/src/features/game/components/PlayingCard.test.tsx`:
    - Renders face-up card with correct rank and suit symbol
    - Renders face-down card with no suit/rank visible
    - `playable` state: has accent glow class and cursor-pointer
    - `unplayable` state: has 40% opacity and cursor-not-allowed
    - Calls `onClick` only when state is `playable`
    - Has correct aria-label (e.g., "King of Spades")
    - Keyboard: Enter/Space fires onClick when playable
    - Has tabIndex={0} when playable, tabIndex={-1} when not
  - [x] `client/src/features/game/components/HandCards.test.tsx`:
    - Renders correct number of cards
    - Cards are `playable` when `isMyTurn && cardId in playableCardIds`
    - Cards are `unplayable` when `isMyTurn && cardId NOT in playableCardIds`
    - Cards are `default` when not my turn
    - `onPlayCard` called with correct cardId on playable card click
  - [x] `client/src/features/game/components/PlayerSeat.test.tsx`:
    - Renders empty seat with "Waiting..." text when player is null
    - Renders occupied seat with username and team color border
    - Renders active state with accent border and pulse animation class
    - Renders self seat with "You" badge and scale-110
    - Has correct aria-label including username, team, and active/waiting state
  - [x] `client/src/features/game/components/TrickArea.test.tsx`:
    - Renders empty state with oval outline when no cards
    - Renders played cards in correct compass positions
    - Shows accent glow on winning card during resolving state
    - Clears display after resolution timeout (use `vi.useFakeTimers`)
  - [x] `client/src/features/game/GamePage.test.tsx`:
    - Renders loading state when `gameState === null`
    - Renders 4 PlayerSeat components when gameState is set
    - Derives myPlayerSeat from gameState.players matching authStore.user.id

- [x] Task 11: Regression suite
  - [x] `npx vitest run` — all frontend tests pass
  - [x] `npx tsc --noEmit` — no TypeScript errors

### Review Findings

- [x] [Review][Decision] D1: TrickArea sweep animation entirely absent — FIXED: implemented sweeping state, off-screen CSS transitions, and 3-phase resolution (glow → sweep → clear)
- [x] [Review][Decision] D2: South seat position `bottom-24` deviates from spec's `bottom-4` — KEPT: intentional deviation to avoid HandCards overlap
- [x] [Review][Patch] P1: TrickArea displayTrick stuck when trick resets from non-4 length — FIXED: added force-sync branch for non-4 reset
- [x] [Review][Patch] P2: TrickArea winnerSeat always null during animation — FIXED: useWsDispatch now stores trickWinnerSeat from EVENT_TRICK_RESOLVED payload
- [x] [Review][Patch] P3: GamePage back-button pushState fires on every gameState change — FIXED: useRef guard pushes sentinel entry only once
- [x] [Review][Patch] P4: HandCards negative containerWidth when hand is empty — FIXED: early return for empty hand
- [x] [Review][Patch] P5: Invalid Tailwind variant order `hover:motion-safe:` — FIXED: changed to `motion-safe:hover:`
- [x] [Review][Patch] P6: PlayingCard applies transition classes unconditionally including in TrickArea — FIXED: added `withTransition` prop, TrickArea passes `false`
- [x] [Review][Patch] P7: Hardcoded "Trump:" string not translated — FIXED: added `game.trump` i18n key in both en/sr
- [x] [Review][Patch] P8: PlayingCard role="button" unconditional on face-down and default cards — FIXED: role only set when `isPlayable`
- [x] [Review][Patch] P9: Chat sidebar overlaps toggle button when open — FIXED: toggle hidden when sidebar open, close button added in sidebar
- [x] [Review][Patch] P10: PlayerSeat empty username produces blank avatar initial — FIXED: fallback to `P{seat+1}`
- [x] [Review][Patch] P11: Missing test — prefers-reduced-motion immediate-clear path in TrickArea — FIXED: test added
- [x] [Review][Patch] P12: Missing test — popstate back-button interception — FIXED: test added
- [x] [Review][Patch] P13: Missing test — match_end phase triggers navigate to /lobby — FIXED: test added
- [x] [Review][Defer] W1: EVENT_CARD_PLAYED cardId[0]/[1] parsing assumes 2-char format — pre-existing in useWsDispatch.ts, not this story's scope [useWsDispatch.ts:82-86] — deferred, pre-existing

## Dev Notes

### Architecture Compliance

- **Purely presentational game components** — PlayingCard, HandCards, TrickArea read data only from props. All state from server via WebSocket. No local game logic in any component.
- **No client-side card legality** — do not implement "which cards are legal to play" in the frontend. Pass all cards as playable when it's the player's turn; the server rejects illegal plays via `error:illegal_play`.
- **No animation library** — CSS keyframes as Tailwind config extensions + `transition-*` utilities only. All animations must use `motion-safe:` prefix and provide `motion-reduce:` fallback.
- **sendMessage flows from GamePage down** — `useWebSocket` is called once in GamePage; `sendMessage` is passed to HandCards/PlayingCard via props (not a context in this story).
- **gameStore as truth** — all rendered state derives from `useGameStore`. Never maintain a parallel copy of game state in component local state (exception: TrickArea's internal `displayTrick` during the 1-second resolution pause).
- **Named exports only** — no `export default`. Component filename must match the exported component name exactly.

### Route Fix Required

RoomLobby navigates `navigate(\`/game/${room.id}\`)`but the current route in`App.tsx`is`path="/game"`. This causes a 404-style redirect. Fix: update App.tsx route to `path="/game/:roomId"`. GamePage extracts the param:

```tsx
import { useParams } from "react-router";
const { roomId } = useParams<{ roomId: string }>();
```

GamePage does NOT use `:roomId` for game logic — the authoritative roomId comes from `gameState.roomId` via WS. The URL param just ensures routing works.

### WebSocket Wiring in GamePage

GamePage is rendered OUTSIDE `AppLayout` — it has no top nav bar and no inherited WS connection. It must establish its own:

```tsx
import { useWsDispatch } from "../../shared/hooks/useWsDispatch";
import { useWebSocket } from "../../shared/hooks/useWebSocket";

export function GamePage() {
  const dispatch = useWsDispatch();
  const { sendMessage } = useWebSocket({ onMessage: dispatch });
  // ... pass sendMessage to HandCards/PlayingCard via props
}
```

The `useWebSocket` hook manages connection lifecycle by component mount. When GamePage unmounts (navigating to lobby), the connection closes automatically.

### Compass Seat Mapping

```ts
// compassOffset 0 = South (self), 1 = East, 2 = North (teammate), 3 = West
// (Counter-clockwise seat indexing — seat indices increment CCW around the table.)
function compassOffset(seat: number, myPlayerSeat: number): number {
  return (seat - myPlayerSeat + 4) % 4;
}

// CSS positions for each compass point
const SEAT_POSITIONS = {
  0: "bottom-4 left-1/2 -translate-x-1/2", // South (self)
  1: "right-4 top-1/2 -translate-y-1/2", // East
  2: "top-4 left-1/2 -translate-x-1/2", // North
  3: "left-4 top-1/2 -translate-y-1/2", // West
};
```

Counter-clockwise play direction: seat index increments counter-clockwise. So if `myPlayerSeat=0`, turn order is 0→1→2→3→0 (which visually goes South→East→North→West→South, counter-clockwise around the table when viewed from above; seat indices increment counter-clockwise).

### Derive `myPlayerSeat` in GamePage

```tsx
const user = useAuthStore((s) => s.user);
const gameState = useGameStore((s) => s.gameState);
const { myPlayerSeat, setMyPlayerSeat } = useGameStore();

useEffect(() => {
  if (gameState && user && myPlayerSeat === null) {
    const myPlayer = gameState.players.find((p) => p.userId === user.id);
    if (myPlayer !== undefined) setMyPlayerSeat(myPlayer.seat);
  }
}, [gameState, user, myPlayerSeat, setMyPlayerSeat]);
```

### Browser Back-Button Interception

Prevent accidental navigation during active game:

```tsx
useEffect(() => {
  if (!gameState) return;
  const handlePopState = () => {
    const leave = window.confirm(t("game.leaveConfirm"));
    if (!leave) {
      window.history.pushState(null, "", window.location.href);
    } else {
      clearGame();
      navigate("/lobby");
    }
  };
  window.history.pushState(null, "", window.location.href);
  window.addEventListener("popstate", handlePopState);
  return () => window.removeEventListener("popstate", handlePopState);
}, [gameState, clearGame, navigate]);
```

### Team Color from Seat

```ts
// From architecture: teams 0+2 = Team A, 1+3 = Team B
function teamColor(seat: number): "teamA" | "teamB" {
  return seat % 2 === 0 ? "teamA" : "teamB";
}
```

### Card ID Conversion

The `Card` type from `gameTypes.ts` has `{ rank: Rank, suit: Suit }`. The card ID string uses two-character format (e.g., `"KS"`). Convert:

```ts
function cardId(card: Card): string {
  return `${card.rank}${card.suit}`;
}
```

Display rank mapping for UI — the wire format uses `T` for ten but users see `10`:

```ts
const DISPLAY_RANK: Record<Rank, string> = {
  "7": "7",
  "8": "8",
  "9": "9",
  T: "10",
  J: "J",
  Q: "Q",
  K: "K",
  A: "A",
};

const DISPLAY_SUIT: Record<Suit, string> = {
  S: "♠",
  H: "♥",
  D: "♦",
  C: "♣",
};

const SUIT_FULL_NAME: Record<Suit, string> = {
  S: "Spades",
  H: "Hearts",
  D: "Diamonds",
  C: "Clubs",
};

const RANK_FULL_NAME: Record<Rank, string> = {
  "7": "Seven",
  "8": "Eight",
  "9": "Nine",
  T: "Ten",
  J: "Jack",
  Q: "Queen",
  K: "King",
  A: "Ace",
};
```

Use `RANK_FULL_NAME` and `SUIT_FULL_NAME` for `aria-label` (e.g., `"King of Spades"`).
Use `DISPLAY_RANK` and `DISPLAY_SUIT` for visual rendering.

### Available Design Tokens (from index.css)

Use these exact Tailwind token names — do NOT use raw hex values:

| Token            | Tailwind class                 | Value       |
| ---------------- | ------------------------------ | ----------- |
| Background       | `bg-background`                | `#0a0a0f`   |
| Surface          | `bg-surface`                   | `#13131a`   |
| Surface elevated | `bg-surface-elevated`          | `#1c1c26`   |
| Border           | `border-border`                | `#2a2a38`   |
| Accent (teal)    | `border-accent`, `text-accent` | `#00e5a0`   |
| Accent glow      | custom shadow                  | `#00e5a040` |
| Team A           | `border-team-a`, `text-team-a` | `#ff4d4d`   |
| Team B           | `border-team-b`, `text-team-b` | `#4d9fff`   |
| Text primary     | `text-text-primary`            | `#f0f0f8`   |
| Text secondary   | `text-text-secondary`          | `#8888a0`   |
| Warning          | `text-warning`                 | `#eab308`   |

For accent glow shadow (not a direct Tailwind utility), use arbitrary value: `shadow-[0_0_12px_var(--color-accent-glow)]`

### Font Tokens

- **`font-display`** (Space Grotesk): Use for score numbers, headings, room/player names in prominent positions
- **`font-body`** (Inter): Use for labels, body text, button text, chat, form inputs

In game components: username labels use `font-body`, ScorePanel placeholder numbers use `font-display`.

### Accessibility Requirements

Per UX spec (WCAG 2.1 AA):

1. **Keyboard navigation**: All interactive elements focusable via keyboard. PlayingCard when `playable` gets `tabIndex={0}`, responds to Enter/Space. Tab order follows hand left-to-right.
2. **Focus ring**: `focus-visible:ring-2 focus-visible:ring-accent focus-visible:ring-offset-2 focus-visible:ring-offset-background` on all interactive elements.
3. **`aria-live` regions**: PlayerSeat active indicator uses `aria-live="polite"`.
4. **`aria-disabled`**: Set on unplayable cards.
5. **Color independence**: Team identity uses color + text label ("Team A"/"Team B"). Card playability uses glow + positional lift (not color alone).
6. **`prefers-reduced-motion`**: ALL animations/transitions must use `motion-safe:` prefix. Animations must not play for users with reduced motion preference.

### prefers-reduced-motion Pattern

Tailwind CSS v4 provides `motion-safe:` and `motion-reduce:` variants. Use:

```tsx
// Correct: animations only play if user allows motion
<div className="motion-safe:animate-pulse motion-safe:transition-transform motion-safe:duration-150">

// For TrickArea resolution: skip the 1s visual pause
// in motion-reduce mode, clear trick immediately
```

Apply `motion-safe:` to: `animate-pulse`, `transition-*`, `duration-*`, `translate-y-*` (card lift). Apply `motion-reduce:transition-none` where instant feedback is needed.

### HandCards Fan Layout

```tsx
// Dynamic overlap: cards spread wider as hand shrinks
const overlap = Math.max(32, 56 - (hand.length - 1) * 4);
const containerWidth = (hand.length - 1) * overlap + CARD_WIDTH_MD;

// Each card: absolute positioning
style={{ left: index * overlap }}
```

### TrickArea Card Quadrant Positions

Place each played card in its compass quadrant within the TrickArea box:

```ts
const TRICK_CARD_POSITIONS = {
  0: "bottom-0 left-1/2 -translate-x-1/2", // South
  1: "top-1/2 right-0 -translate-y-1/2", // East
  2: "top-0 left-1/2 -translate-x-1/2", // North
  3: "top-1/2 left-0 -translate-y-1/2", // West
};
```

### TrickArea Resolution Sequence

The server sends events in this order: `event:card_played` × 4 → `event:trick_resolved`. The TrickArea must show all 4 cards for ~1 second before sweeping. Since `useWsDispatch` clears `currentTrick` on `EVENT_TRICK_RESOLVED`, TrickArea should maintain a local `displayTrick` state:

```tsx
const [displayTrick, setDisplayTrick] = useState<TrickCard[]>([]);
const [resolving, setResolving] = useState(false);

// Check if user prefers reduced motion
const prefersReducedMotion = useMemo(
  () => window.matchMedia("(prefers-reduced-motion: reduce)").matches,
  [],
);

useEffect(() => {
  if (trick.length > displayTrick.length) {
    // New card arrived — update display immediately
    setDisplayTrick(trick);
  }
  if (trick.length === 0 && displayTrick.length === 4) {
    // Server cleared trick (trick_resolved) — start animation
    if (prefersReducedMotion) {
      // Skip animation for reduced-motion users
      setDisplayTrick([]);
      return;
    }
    setResolving(true);
    const t = setTimeout(() => {
      setDisplayTrick([]);
      setResolving(false);
    }, 1000);
    return () => clearTimeout(t);
  }
}, [trick]); // eslint-disable-line react-hooks/exhaustive-deps — displayTrick is intentionally excluded: including it causes infinite loops since the effect modifies it. The trick prop alone drives all transitions.
```

### Chat Sidebar (Placeholder for Epic 6)

Story 4.3 only implements the sidebar shell:

- A `<button>` at the right edge: `absolute right-0 top-1/2 -translate-y-1/2`; shows chat icon with `aria-label` using i18n key
- A panel `<aside>` that slides in from the right when open: `absolute right-0 top-0 h-full w-64 bg-surface border-l border-border`
- Panel contains only a header using `game.chat.title` i18n key and a placeholder paragraph using `game.chat.placeholder` key
- The sidebar must NOT overlap the game table when closed; when open, it clips to the rightmost 256px of the viewport

### Existing Code to Reuse — DO NOT Reinvent

| Symbol                                          | File                              | Use                                                                           |
| ----------------------------------------------- | --------------------------------- | ----------------------------------------------------------------------------- |
| `useGameStore`                                  | `shared/stores/gameStore.ts`      | `gameState`, `myPlayerSeat`, `setMyPlayerSeat`, `clearGame`                   |
| `useAuthStore`                                  | `shared/stores/authStore.ts`      | `user.id` to find myPlayerSeat                                                |
| `useWebSocket`                                  | `shared/hooks/useWebSocket.ts`    | `sendMessage(type, payload)` for play_card action                             |
| `useWsDispatch`                                 | `shared/hooks/useWsDispatch.ts`   | Already routes `event:card_played` / `event:trick_resolved` to gameStore      |
| `ACTION_PLAY_CARD`                              | `shared/types/wsEvents.ts`        | WS event constant for card play — value is `"action:play_card"`               |
| `GameState`, `Card`, `TrickCard`, `PlayerState` | `shared/types/gameTypes.ts`       | All types already defined                                                     |
| `Rank`, `Suit`                                  | `shared/types/gameTypes.ts`       | Union literal types for card encoding                                         |
| `Button`                                        | `shared/components/ui/button.tsx` | shadcn Button for chat toggle — variants: `default`, `ghost`, `outline`, etc. |
| `useTranslation`                                | `react-i18next`                   | `const { t } = useTranslation()` — use for all user-facing strings            |
| `useParams`                                     | `react-router`                    | `const { roomId } = useParams<{ roomId: string }>()` — extract URL param      |
| `useNavigate`                                   | `react-router`                    | `const navigate = useNavigate()` — for lobby redirect on match end            |
| `AppLayout` is NOT used                         | `App.tsx`                         | GamePage is outside AppLayout — no top nav bar                                |

### Files to Create

| File                                                       | Purpose                                         |
| ---------------------------------------------------------- | ----------------------------------------------- |
| `client/src/features/game/components/PlayingCard.tsx`      | Single card (face-up or face-down)              |
| `client/src/features/game/components/PlayingCard.test.tsx` | PlayingCard unit tests                          |
| `client/src/features/game/components/HandCards.tsx`        | Fanned hand at bottom of viewport               |
| `client/src/features/game/components/HandCards.test.tsx`   | HandCards unit tests                            |
| `client/src/features/game/components/PlayerSeat.tsx`       | Seat with player info, active state             |
| `client/src/features/game/components/PlayerSeat.test.tsx`  | PlayerSeat unit tests                           |
| `client/src/features/game/components/TrickArea.tsx`        | Central trick display with resolution animation |
| `client/src/features/game/components/TrickArea.test.tsx`   | TrickArea unit tests                            |
| `client/src/features/game/GamePage.test.tsx`               | GamePage integration tests                      |

### Files to Modify

| File                                    | Changes                                                                                    |
| --------------------------------------- | ------------------------------------------------------------------------------------------ |
| `client/src/features/game/GamePage.tsx` | Replace stub with full layout — compass seats, TrickArea, HandCards, chat shell, WS wiring |
| `client/src/App.tsx`                    | Route `path="/game"` → `path="/game/:roomId"`                                              |
| `client/src/i18n/en.json`               | Add `game.seat.*`, `game.chat.*`, `game.loading` keys                                      |
| `client/src/i18n/sr.json`               | Add same keys with Serbian (Latin) translations                                            |

### Testing Patterns — MANDATORY

- Tests in Vitest + React Testing Library (same as all other frontend tests)
- Use `@testing-library/user-event` for click and keyboard events; use `fireEvent` only for events not in userEvent
- Mock `useGameStore` and `useAuthStore` with `vi.mock`
- Game component tests are purely presentational — mock props directly, no store wiring needed
- GamePage tests wire stores via `useGameStore.setState(...)` (Zustand testing pattern used in Story 4.2 tests)
- Use `data-testid` attributes for key elements: `data-testid="game-page"`, `data-testid="player-seat-{compassPosition}"`, `data-testid="trick-area"`, `data-testid="hand-cards"`, `data-testid="playing-card-{cardId}"`
- Test file location: co-located with source file in same directory
- Test descriptions use present tense: `it('renders face-up card with correct rank')`, not `it('should render...')`

### Scope Boundaries — What This Story Does NOT Do

- **No trump bidding UI** — TrumpPrompt is Story 4.4
- **No declaration UI** — DeclarationPrompt is Story 4.4
- **No per-move timer** — TimerRing is Story 4.5
- **No score panel details** — ScorePanel is Story 4.6 (placeholder only in 4.3)
- **No card deal animation** — cards appear instantly from gameState
- **No full legal card validation** — all cards are playable on your turn; server validates
- **No reconnect overlay** — Epic 5
- **No chat functionality** — sidebar shell only; Epic 6
- **No match end screen** — simple redirect to lobby on match_end phase

### Previous Story Intelligence (from 4.2)

- `gameStore.setGameState(gameState)` replaces the full state — call this from WS events, not from components
- `gameStore.myPlayerSeat` starts as `null` — must be derived from gameState.players on first snapshot
- `useWsDispatch` already routes `event:card_played` → adds card to `gameState.currentTrick`; `event:trick_resolved` → clears `gameState.currentTrick` to `[]`
- WS connection is managed at the AppLayout level for lobby; the GamePage (outside AppLayout) must call `useWebSocket` itself. The `useWebSocket` hook manages lifecycle by component mount — GamePage gets its own connection that closes on unmount.
- `event:game_state` fires on game start and on reconnect — always replaces full state
- Review findings from 4.2: `event:card_played` handler does NOT compute `activePlayerSeat` client-side (that was a review fix) — the server sends authoritative state. Client just updates `currentTrick`.
- Session manager broadcasts multi-event sequences as separate ordered messages: `event:card_played` → `event:trick_resolved` → `event:hand_scored`. Frontend animations depend on this ordering.

### Git Intelligence (from recent commits)

Recent commit pattern: `feat(scope): implement <feature> with code review fixes`

- Commit scope for this story: `feat(game): implement game table UI, seats, and cards`
- Frontend tests: `npx vitest run`
- TypeScript check: `npx tsc --noEmit`

### References

- [Source: _bmad-output/planning-artifacts/epics.md, Epic 4, Story 4.3, lines 939-988]
- [Source: _bmad-output/planning-artifacts/architecture.md — frontend file structure, game feature folder, component names]
- [Source: _bmad-output/planning-artifacts/ux-design-specification.md — PlayingCard, PlayerSeat, TrickArea, HandCards component specs, game table layout, design tokens, accessibility requirements]
- [Source: client/src/shared/types/gameTypes.ts — GameState, Card, TrickCard, PlayerState, Rank, Suit types]
- [Source: client/src/shared/types/wsEvents.ts — ACTION_PLAY_CARD constant, payload interfaces]
- [Source: client/src/shared/stores/gameStore.ts — myPlayerSeat, setMyPlayerSeat, clearGame, setGameState]
- [Source: client/src/shared/hooks/useWebSocket.ts — sendMessage(type, payload) API, UseWebSocketReturn interface]
- [Source: client/src/shared/hooks/useWsDispatch.ts — event:card_played and event:trick_resolved routing to gameStore]
- [Source: client/src/index.css — design tokens: --color-accent, --color-team-a, --color-team-b, font-display, font-body]
- [Source: client/src/App.tsx — GamePage route currently path="/game" at line 37, needs fix to "/game/:roomId"]
- [Source: client/src/features/lobby/RoomLobby.tsx — navigates to /game/${room.id} at line 107]
- [Source: client/src/shared/components/ui/button.tsx — shadcn Button with ghost/default/outline variants]
- [Source: _bmad-output/archive/implementation-artifacts/4-2-game-session-manager-and-state-sync.md — previous story patterns, review findings]
- [Source: _bmad-output/project-context.md — project rules: named exports only, i18n keys, test patterns, feature folder structure]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Debug Log References

- Added `username` field to `PlayerState` in `gameTypes.ts` — required for game UI to show player names; server sends this field in game state payload
- Fixed pre-existing test files (`useWsDispatch.test.ts`, `gameStore.test.ts`, `gameTypes.test.ts`) that broke after adding `username` to `PlayerState`

### Completion Notes List

- Implemented full game table UI with 4 compass-positioned player seats, central trick area, fanned hand cards, and collapsible chat sidebar shell
- PlayingCard supports face-up/face-down, playable/unplayable/default states with full keyboard accessibility (Enter/Space), ARIA labels, and motion-safe animations
- HandCards renders dynamically spaced fan layout with overlap that widens as card count decreases
- PlayerSeat shows avatar initials, team color borders, active pulse animation, "You" badge for self, and card count for opponents
- TrickArea displays cards in compass quadrants with 1-second resolution pause, winner glow highlight, and prefers-reduced-motion support
- GamePage establishes its own WebSocket connection (outside AppLayout), derives myPlayerSeat from first game state, handles match_end navigation, and intercepts browser back button
- All i18n keys added to both English and Serbian (Latin) translation files
- 35 new tests across 5 test files — all passing
- Full regression suite: 170 tests pass, 0 failures
- Zero new TypeScript errors introduced

### Change Log

- 2026-04-12: Implemented Story 4.3 — Game Table UI with Layout, Seats & Cards

### File List

**New files:**

- `client/src/features/game/components/PlayingCard.tsx`
- `client/src/features/game/components/PlayingCard.test.tsx`
- `client/src/features/game/components/HandCards.tsx`
- `client/src/features/game/components/HandCards.test.tsx`
- `client/src/features/game/components/PlayerSeat.tsx`
- `client/src/features/game/components/PlayerSeat.test.tsx`
- `client/src/features/game/components/TrickArea.tsx`
- `client/src/features/game/components/TrickArea.test.tsx`
- `client/src/features/game/GamePage.test.tsx`

**Modified files:**

- `client/src/features/game/GamePage.tsx` — replaced stub with full game table layout
- `client/src/App.tsx` — route `/game` -> `/game/:roomId`
- `client/src/shared/i18n/en.json` — added `game.*` i18n keys
- `client/src/shared/i18n/sr.json` — added `game.*` i18n keys (Serbian Latin)
- `client/src/shared/types/gameTypes.ts` — added `username` field to `PlayerState`
- `client/src/shared/hooks/useWsDispatch.test.ts` — added `username` to test fixtures
- `client/src/shared/stores/gameStore.test.ts` — added `username` to test fixtures
- `client/src/shared/types/gameTypes.test.ts` — added `username` to test fixtures
