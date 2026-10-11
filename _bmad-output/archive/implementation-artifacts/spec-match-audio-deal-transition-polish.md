---
title: 'Match polish: event sounds, deal animation, trick-collect sequencing, Croatian auto-skip, suit order'
type: 'feature'
created: '2026-09-26'
status: 'done'
baseline_commit: 'a290a7c8294ce081443d77a490c4bde50902e766'
route: 'dispatch'
review_loop_iteration: 0
context:
  - '{project-root}/docs/audio.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** (1) Only card-play and trick-collect make sound. (2) New hands appear instantly with no deal. (3) When tricks resolve in quick succession, one collect snapshot overwrites the other: sweeps are cut, cards vanish, and the score reveal and next-hand prompt open over unfinished animations. (4) A Croatian player with no melds must click "Skip". (5) The trump-bidding and waiting dialogs order suits ♠♥♦♣ instead of ♠♥♣♦. (6) After a Belote/Rebelote K/Q lands, the turn freezes on the announcer before the reveal shows, because a bot waits a second full think delay (1–2.5 s) to announce.

**Approach:** Client-first polish on the existing audio engine and card-flight layer. Add CC0 sounds for match win/lose, capot, deal, declaration and Belote reveal, avatar popups, and an urgent-timer clock tick. Add a real deal animation for every deal in both variants. Make the trick-collect lifecycle overlap-safe and hold local input until the sweep ends. Auto-skip meld-less Croatian seats. Reorder the suit constant. Make the Belote hand-off immediate.

## Boundaries & Constraints

**Decisions (user, 2026-09-26):**
- Keep all goals in one spec.
- Croatian auto-skip is **instant**: the skip is sent on mount when the seat has no melds. The user accepts that the table can see which seats hold nothing (overriding the uniform-footprint rationale in `projection.go`).
- **Server grace for deals:** each animated deal (new hand, reshuffle, match start, Bitola second deal) adds its animation length to the next actor's `TurnExpiresAt` and to a bot actor's think delay. Broadcasts are not paused.
- Belote/Rebelote reveal plays the declaration sound. It stays the centered 8 s panel.
- **Urgent clock tick:** while the viewer's own decision timer is in `TimerRing`'s red zone (≤1/8 of total, `URGENT_FRACTION`), a clock tick plays once per whole second until 0. Own decisions are: own turn, bid, Belote or Bitola-declare prompt, and the Croatian window while unanswered. Other seats' timers and auto-close, score-reveal and reconnect rings never tick.

**Always:**
- All new sounds go through `playSfx` with sound-preference and volume gating. They are keyed to the moment the UI shows (overlay mount, bubble appear, deal packet land). A resync (`event:match_state`, or a reveal rebuilt from `lastHandResult`) never sounds; use dedupe keys.
- Assets are CC0 MP3s under `client/public/audio/sfx`, recorded in `docs/audio.md` with source, licence and recipe.
- Sequence per hand: last card → winner glow → sweep → (capot) → score reveal → continue → deal animation → bidding prompt.
- Reduced motion skips flights but keeps sounds and a readable beat.
- A deal is identified by `handNumber:dealerSeat`. Mounting into a mid-hand state (reload or reconnect) never replays a deal.
- Every locale gets every new i18n key; mk is all-Cyrillic.

**Never:**
- No pauses or sleeps between server broadcasts.
- No changes to the rules engine, scoring, or the WS event contract. Belote stays the same two actions (announce/decline), sent after `pendingBelotSeat` confirms.
- No changes to hand-fan sort order.
- No audio outside the match page. No third-party audio or animation library.
- No commit until the user says so.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Fast trick | `trick_resolved` N while N-1 snapshot is still live | N-1 is finalised at once; N gets a full glow + sweep; score reveal waits for N's sweep | fallback timer still clears |
| Click during sweep | own turn, snapshot live | card not playable until the snapshot clears | server timer unchanged |
| New hand | `hand_complete` → bidding, new deal key | face-down packets fly from dealer seat to seats in variant order; hand and back stacks fill per packet; prompt mounts after | — |
| Bitola second deal | pick → hand grows 5→8 | 3/3/3/2 packets + candidate to picker, animated | — |
| Reshuffle (all pass) | dealer rotates, same hand number | new deal animates | — |
| Reconnect mid-bidding | first state after mount | no deal animation, no sounds | — |
| Match end | `MatchResult` mounts | win or lose sound for viewer's team, once | dedupe by match |
| Abandonment | `ReconnectOverlay` result line | non-abandoning team: win; abandoner's partner: lose | — |
| Capot | `CapotAnimation` mounts | capot sound once | dedupe by hand |
| Declaration / belot reveal | panel mounts | declaration sound once | dedupe by reveal |
| Avatar popup | emote bubble (if emotes visible), Bitola declare banner, surrender banner or prompt | popup sound; proposer silent for own surrender | — |
| Suit order | bidder grid, waiting chips | ♠ ♥ ♣ ♦ | — |
| Croatian, no melds | declaring phase opens | skip sent at mount; "Nothing to declare" + waiting (answered+own)/4 | send once |
| Bot Belote | bot's K/Q play sets `pendingBelotSeat` | bot announces after ≤200 ms; turn advances at once | — |
| Timeout during local Belote prompt | server auto-plays the card | local prompt closes; no stale choice kept | — |
| Local Belote prompt | human clicks K/Q | prompt shows the running per-move ring | — |
| Own timer turns red | own deadline, remaining ≤ total/8 | one tick per whole second until 0; stops when the decision is made or the turn moves | no tick on remount into an already-red state beyond the remaining seconds |
| Other seat's timer red | opponent or partner turn | silent | — |

</frozen-after-approval>

## Code Map

- `client/src/shared/audio/audioEngine.ts:35-43` -- `SfxName` + `SFX_URLS` variant arrays; `playSfx(name,{dedupeKey})` :198. `audioEngine.test.ts:773-784` asset-exists list must include new names.
- `client/src/features/match/MatchPage.tsx`:
  - collect effect :494-576, `handleFlightComplete` :583-596 (clears on any `collect-` absence, not per trick), `isCollecting` :601-618, `isMyTurn` :1510
  - overlay gates :683-706 (`overlayPhase`, score/capot), :892-901 (match result), `declRevealReady` :770-779
  - `CapotAnimation` :2284, `MatchResult` :2310, `DeclarationReveal` :2198, `BelotReveal` :2214, `TrumpPrompt` :2107-2141, placeholder `DealAnimation` :2110 (phase `dealing`, hand 1 only), reshuffle detector :917-926
  - seat backs count :1732-1736, emote/declare/surrender bubbles :1744-1768, :2042, :2248-2281, `emotesVisible` :1578
- `client/src/features/match/components/CardFlight.tsx` -- `CardFlightDescriptor`; `card` is required and always face-up, so add a face-down option (`PlayingCard card={null} state="face-down"`).
- `client/src/features/match/components/DealAnimation.tsx` -- placeholder; replace it.
- `HandCards.tsx`, `PlayerSeat.tsx` `CardBackStack` :118-187 (`data-seat-deck`) -- targets for progressive fill.
- `client/src/shared/hooks/useWsDispatch.ts` -- `system:emote` :1011, `event:player_declared` :525, `event:surrender_proposed` :646 (no sounds in the `match_state` branch :159).
- `client/src/shared/lib/motion.ts` -- add deal constants. Existing `DEAL_*` values are unused.
- `client/src/features/match/components/DeclarationPrompt.tsx:95-127,246-251` -- `sendOnce` latch, `waiting`, `AUTO_SKIP_SEC` ring. Candidates come from `MatchPage.tsx:1357-1368` `promptDeclarations`.
- `client/src/features/match/components/TimerRing.tsx:89-90` -- `URGENT_FRACTION` red threshold. Export and reuse it; don't duplicate. `useTurnCountdown` drives the seconds.
- `client/src/features/match/components/TrumpPrompt.tsx:48` -- `SUITS` feeds grid :75 and chips :227. `HandCards` `SUIT_ORDER` is separate; leave it.
- Server grace:
  - `server/internal/match/live_match.go`: `setTurnExpiry` :1627; dealing→bidding rewrites :521/1803/1982/2051; `StartMatch` :371
  - `bot_driver.go`: `botThinkDelay` :150
  - `game/bidding.go`: second deal :170-186, reshuffle `reshuffleAndRedeal` :249
  - Keep the server grace constants equal to the client deal durations, with a cross-reference comment on each side.
- Deal shapes: Bitola 3+2 from dealer+1, flip candidate, then 3/3/3/(2+candidate) after pick. Croatian 3+3+2 face-down.
- Belote:
  - Client: `MatchPage.tsx` `handlePlayCard` :1098, `belotPromptCardId`/`handleLocalBelotDecision` :1121-1125 (never cleared on server auto-play), `pendingBelotSeat` effect :358-363, local `BelotPrompt` :2167 (no `turnExpiresAt`), `BelotReveal` :2214.
  - Server: bot answer is scheduled via `bot_driver.go:117-122` with a full `botThinkDelay`; `game/playing.go:91-96` hold; `declarations.go:603` announce.
  - Tests: `MatchPage.test.tsx:1513-1611`, `bot_driver_test.go:362`.

## Tasks & Acceptance

**Execution:**
- [x] `client/public/audio/sfx/*`, `docs/audio.md` -- download CC0 Kenney packs (Casino Audio: shuffle/place/chips; Music Jingles: win/lose/capot; Interface Sounds: popup) and convert per the existing recipe. If a source is unreachable, HALT.
- [x] `audioEngine.ts` (+test) -- register `deal`, `dealPacket`, `declaration`, `capot`, `matchWin`, `matchLose`, `popup`, `clockTick`.
- [x] `MatchPage.tsx` (+ small hook) -- one urgent-tick driver for the viewer's own active deadline, per matrix.
- [x] `MatchPage.tsx` + `matchStore.ts` -- overlap-safe collect: a new snapshot finalises the old one (drop its flights, reset `isCollecting`); `handleFlightComplete` matches the current trick's `receivedAt`; gate own-card playability on no live snapshot.
- [x] `motion.ts`, `CardFlight.tsx`, `DealAnimation.tsx`, `MatchPage.tsx` -- deal controller keyed by deal id: packets per variant, progressive hand/back fill, sounds, gate `TrumpPrompt`/candidate until done, reduced-motion path. Remove dead reshuffle/dealing placeholders it supersedes.
- [x] `MatchPage.tsx`, `ReconnectOverlay.tsx`, `useWsDispatch.ts` -- event sounds per matrix, including `BelotReveal`.
- [x] `DeclarationPrompt.tsx` (+`MatchPage.tsx` wiring) -- instant auto-skip when there are no melds; show "nothing to declare" + waiting N/4 with own seat counted optimistically.
- [x] `TrumpPrompt.tsx` -- `SUITS = ["S","H","C","D"]`; add DOM-order test.
- [x] `live_match.go`, `bot_driver.go` (+Go tests) -- deal grace on the next actor's deadline and on bot think delay; a bot's Belote answer uses a ≤200 ms beat.
- [x] `MatchPage.tsx` -- clear local Belote prompt/choice when the server moves past that card; pass `turnExpiresAt` to the local `BelotPrompt`.
- [x] Unit tests for matrix rows in `MatchPage.test.tsx`, `DeclarationPrompt.test.tsx`, `useWsDispatch.test.ts`.

**Acceptance Criteria:**
- Given a live 4-seat match, when the last trick is played fast, then the final four cards stay visible through the glow and sweep before any overlay.
- Given any variant, when a hand starts, then a deal animation with sound plays before the bidding prompt, and the first bidder's ring starts full.
- Given a bot holds K+Q of trump, when it plays one, then the Belote reveal appears and the next seat activates within about 0.5 s of the card landing.

## Verification

**Commands:**
- `cd client && npx vitest run` -- all pass
- `cd client && npx tsc --noEmit && npx eslint . && npx prettier --check .` -- clean
- `cd server && go test ./... && golangci-lint run ./...` -- clean

**Manual checks:**
- `make dev`, then drive a live match with the Playwright MCP (quick-play bot fill + fast WS bots per the debug-harness note). Confirm: collect sequencing (rAF recorder), deal in both variants, Croatian auto-skip, suit order, and sounds (spy on `playSfx`).

## Implementation Notes

- **Assets** (all CC0 Kenney, sources + sha256 + recipe in `docs/audio.md`; the recipe regenerates the shipped files byte-for-byte): `card-shuffle` (deal), `card-place-1..4` (packet lands, onset-trimmed), `chips-stack-1..6` (declaration + Belote reveal), `jingle-capot` (HIT11), `jingle-win` (SAX02), `jingle-lose` (SAX07), `popup-1..2` (Interface `drop_002/003`), `clock-tick` (Interface `tick_004`). Engine dedupe memory raised 64 → 256.
- **Deal timing** (one schedule, `features/match/lib/dealSchedule.ts`): lead-in 400 ms (opening deals only), packet flight 320 ms, stagger 140 ms, candidate flip 400 ms → candidate first deal 2100 ms, all-before-bidding 2260 ms, candidate second deal 740 ms. Server `deal_grace.go` mirrors these; match start adds the 1500 ms `GAME_STARTING_SPLASH`. Deal shape is read from the state (candidate present or not), never the variant.
- **Deal controller** (`useDealController`): detection runs during render so the first frame of a new hand already shows empty stacks and no prompt; timers are anchored to the deal's start (resume-safe); sounds/flights keyed by `dealId@startedAt` because `handNumber:dealerSeat` recurs (four reshuffles, a new match). The opening deal animates only when arriving from the room and the hand is still fresh (round 1, no bids). Mid-deal: no seat is lit and no seat ring runs, the hand fan and back stacks fill per packet, the trump prompt / Bitola declaration prompt / own-card clicks wait. A reshuffle shows the existing `match.reshuffle.message` caption. `ReshuffleAnimation` and the old `DealAnimation` placeholder are gone; no new i18n keys.
- **Collect**: snapshots are stamped monotonically; every clear goes through `clearPendingResolvedTrick(receivedAt)`; collect flights are named `collect-<receivedAt>-<card>` and a newer snapshot drops the older trick's flights before paint; `isCollecting` is keyed to the live snapshot; own cards are unplayable while a snapshot is live or a deal runs.
- **Sounds**: reveals/capot/result/abandonment use a `soundKey` prop (a per-payload key, `payloadSoundKey`); the capot banner rebuilt from `lastHandResult` is silent. Popups are played by the WS dispatcher (emote only when bubbles are visible; surrender silent for the proposer). The urgent tick is one driver (`UrgentTick`) fed the viewer's own deadline (turn, bid, Belote/Bitola-declare prompt) or the Croatian window (deadline started when its prompt mounts); ticks at each whole second in the red zone down to 1 (silent at 0), stop at the click, re-verify the second against the clock.
- **Server**: the grace is a Manager switch (`dealGraceEnabled`, on in `NewManager`); tests that are not about the deal opt out with `SetDealGraceForTest(false)`. The grace applies to the next actor only (every later transition clears it). Bot Belote answer: `botBelotBeat` = 150 ms.
- **Live check** (`make dev`, Playwright + fast WS bots and a server-bot room): opening, reshuffle, Croatian 3/3/2 face-down and Bitola second deals animate with sounds and gate the prompt; first ring mounts with ~14.97 s of 15 s left; collect → score reveal → continue → deal → prompt in order; no own-card click while a snapshot was live; Croatian meld-less seat auto-skips on mount showing "No declarations / 4/4"; suit tiles and chips read SHCD; urgent tick sounds once at 1 s on a 15 s timer; win jingle on the result. A bot-held Belote did not occur in the live run (covered by `TestBot_BelotAnswerTakesAShortBeat`).

- **Review-round live check** (`make dev`, Playwright sound spy on `AudioBufferSourceNode.start` + rAF/store recorder; quick-play bot fill for Croatian, fast WS bots and a server-bot room for Bitola):
  - Deals: Croatian 3/3/2 face-down (12 packets, prompt at ~2.24 s), Bitola 3+2 + candidate flip (~2.09 s), second deal 2+candidate to the taker (~0.73 s), and the all-pass reshuffle with its caption all animated, with shuffle + per-packet sounds and the grace on the deadline.
  - Hand end ran last card → glow → sweep → score reveal → continue → deal → prompt.
  - Croatian meld-less auto-skip closed the phase 17 ms after the prompt mounted. Suit tiles and chips read SHCD. The urgent tick fired 3× per own red zone and stayed silent at 0. The lose jingle played once at the result.
  - A reload during hand 1's fresh bidding showed "Reconnecting…" and no deal. A server bot's Belote reveal and turn hand-off came 161 ms after the K/Q.
- **Found live, fixed:** a quick-play arrival could skip the splash and opening deal. The first `event:match_state` let `useReconnectionRedirect` navigate to `/match` before the matchmaking page's own `fromRoom` redirect (a pre-existing race). The hook now carries `state: { fromRoom: true }` when leaving `/matchmaking/:id` or `/rooms/:id` (+ test).

## Spec Change Log

## Review Triage Log

Iteration 0 (blind-hunter BH, edge-case-hunter ECH, verification-gap VG).

| # | Source | Finding | Verdict | Evidence | Route |
|---|--------|---------|---------|----------|-------|
| 1 | VG | Deal grace on timer paths (`handleTimerExpiry` reshuffle, `handleHandCompleteTimeout` fallback) untested | gap (pre-verified) | Every grace test drives `StartMatch`/`HandleAction`; deleting either `setDealGraceLocked` line keeps the suite green. | patch |
| 2 | BH | Same as #1 (timeout paths gained grace, no tests) | gap | Same root cause as #1. | patch (with #1) |
| 3 | VG | Deal packet flights never generated in any test; reduced-motion "no flights" check is vacuous | gap (pre-verified) | Deal describe has no `getBoundingClientRect` spy, so `dealFlights` returns `[]` at the `!origin` guard in both modes. | patch |
| 4 | VG | Urgent tick "stops at the click" only tested for a card play | gap (pre-verified) | Only "stops as soon as the viewer plays" clicks; pass/pick/skip/Belote marks are unguarded. | patch |
| 5 | VG | `GAME_STARTING_SPLASH` (mirrored by server `matchStartSplash`) not pinned on the client | gap (pre-verified) | Client tests read the constant by name; retuning it passes every test. | patch |
| 6 | VG | `TestBot_ForcedDealerPickAdvancesHandWithoutRejection` now absorbs the Croatian match-start grace (3.77 s of a 5 s wait) | low | It lacks the `SetDealGraceForTest(false)` its siblings got; 1.5 s + 2.26 s grace lands inside its `waitFor(5s)`. | patch |
| 7 | ECH | Reload during hand 1's fresh round-1 bidding replays splash + opening deal (fromRoom survives in `history.state`) | medium | `cameFromRoom` is read from `location.state` every render; browsers keep `history.state` on reload, so `animateOpening` is true again and `detectDeal(null, fresh hand 1)` animates — violating "reload never replays a deal". | patch |
| 8 | BH, ECH | Rejected action leaves `answeredDeadline`/`answeredWindowHand` set, silencing the urgent tick for a turn still owed | low | Markers are set before `sendMessage` and only superseded by a new deadline; an `error` reply leaves the deadline unchanged. Direct fix: reset them in the error path. | patch |
| 9 | BH | `expect(within(...).getByTestId("playing-card-AH"))` has no matcher | low | `MatchPage.test.tsx` candidate-flip test; passes only because `getByTestId` throws. | patch |
| 10 | BH | `trickA` fixture has seat 0 playing K♠ while seat 0 still holds K♠ | low | Impossible table state; the "holds the viewer's card" test renders KS twice. Direct fixture fix. | patch |
| 11 | BH | Deal-flight target width `compact ? 15 : 26` restates `CardBackStack`'s `cw` | low | `PlayerSeat.tsx:131` holds the same literals; a resize would land packets at the wrong size. Direct dedupe. | patch |
| 12 | BH | `DEDUPE_MEMORY` comment says a hand spends "roughly twenty" keys | low | Its own list (shuffle, 8+4 packets, 8 collects) already exceeds 20 before ticks/reveals. Comment fix. | patch |
| 13 | BH | A WS reconnect without remount could animate a fresh hand the viewer never saw | false | A true disconnect moves the match to `PhaseDisconnected` (reconnect.go), so no deal happens while the socket is down; the resync returns to the already-observed deal id. | reject |
| 14 | BH | Croatian meld-less auto-skip still waits behind the 8 s trump-taken reveal | false | The skip fires on prompt mount exactly as the frozen intent states; the reveal is a pre-existing deliberate beat that gates every seat's prompt equally, and the server window is 20 s. (Surfaced to the user.) | reject |
| 15 | BH, ECH | Match-start grace assumes the 1500 ms splash; reduced-motion clients hold 400 ms | low | `ringDrainStyle` stretches an over-full deadline, so the ring still starts full; the reduced-motion first bidder gets ~1.1 s extra. The server cannot know a client's motion preference; fix is more than a direct correction. | reject |
| 16 | BH | Deal durations copied in four places with no cross-check | false | Both sides pin literal values (`dealSchedule.test.ts` 2100/2260/740, `deal_grace_test.go`), so retiming one side fails its own suite; only the splash was unpinned (#5). | reject |
| 17 | BH, ECH | Client treats `declaring` as a valid second-deal phase; server grants grace only for `playing` | false | No reachable rules config combines a trump candidate with the dedicated declaration phase (Bitola = candidate + trick-1 declarations; Croatia = all-before-bidding). | reject |
| 18 | BH | Bitola second deal plays under the trump-taken reveal; candidate flight starts under the panel | low | Real layering (Z.REVEAL 60 > Z.CARD_FLIGHT 30), but the panel shows that same candidate and the card emerging from beneath it toward the taker reads as intended; a fix needs cross-overlay sequencing. (Checked in the live run.) | reject |
| 19 | BH | New `role="status"` region created with its text may not be announced | low | Old placeholder had no live region at all; SR-only nicety, fix is a page-level live-region restructure. | reject |
| 20 | BH | `detectDeal`/`useUrgentTick`/`payloadSoundKey` lack direct unit tests | false | Named cases behave: reshuffle rotation compares consecutive observations, run ids carry `startedAt`, and `event:trump_selected` only sets `trumpReveal` (never `matchState`), so a pick arrives in one snapshot. | reject |
| 21 | BH | Two Go tests use real sleeps | low | Margins are asymmetric-safe; the Belote timing now also has the deterministic `TestBot_BelotBeatIsAtMost200ms`. Rewrite is more than a direct correction. | reject |
| 22 | BH | Emote popup gate re-implements `emotesVisible` by hand | low | Conditions match today; drift needs a future edit to `emotesVisible`, and a shared selector is a refactor. | reject |
| 23 | ECH | `rebuiltScoreRevealRef` is per-mount, so an in-app remount during a rebuilt capot reveal could sound the jingle | low | Needs a MatchPage remount without reload while the rebuilt payload is in the store; a reload clears the store and rebuilds silently. Fix adds module state. | reject |
| 24 | ECH | Auto-skip sent while the socket is down is latched with no retry | low | `sendOnce` latch is pre-existing (manual Skip behaves the same); a real disconnect flips the phase to `disconnected`, unmounting the prompt and resetting the latch. | reject |
| 25 | ECH | Pause during a deal clears the grace, so a quick unpause lets a bot act mid-deal | low | Needs pause + unpause within ~1 s of a deal starting; the human deadline keeps its grace via the captured remaining time. | reject |
