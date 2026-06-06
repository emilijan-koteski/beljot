---
stepsCompleted:
  [
    "step-01-init",
    "step-02-discovery",
    "step-02b-vision",
    "step-02c-executive-summary",
    "step-03-success",
    "step-04-journeys",
    "step-05-domain",
    "step-06-innovation",
    "step-07-project-type",
    "step-08-scoping",
    "step-01b-continue",
    "step-09-functional",
    "step-10-nonfunctional",
    "step-11-polish",
    "step-12-complete",
  ]
inputDocuments: ["product-brief-beljot-2026-02-21.md"]
documentCounts:
  briefs: 1
  research: 0
  brainstorming: 0
  projectDocs: 0
classification:
  projectType: web_app
  domain: gaming_entertainment
  complexity: medium
  projectContext: greenfield
workflowType: "prd"
---

# Product Requirements Document - beljot

**Author:** Emilijan
**Date:** 2026-02-21

## Executive Summary

Beljot is a purpose-built desktop web platform for Balkan Belot — the team-based card game deeply embedded in the culture of Macedonia, Serbia, and Croatia. Despite a large, passionate, multigenerational player base, no dedicated online platform exists that implements authentic regional rule variants or provides competitive infrastructure. Beljot fills this gap with a modern multiplayer experience supporting both Croatian and Bitola trump variants, a full account and progression system, CIV 6-style lobby matchmaking, and the first-ever competitive ecosystem for Balkan Belot: ELO-based ranked play, an 8-tier ranking system, quarterly seasons, and leaderboards.

The platform targets three core user profiles: competitive regulars seeking ranked progression, casual/social players recreating the kitchen-table experience with friends online, and diaspora players reconnecting with family across borders through the game. Launch supports English and Serbian (Latin script), with 1001-point matches for competitive play and both 1001/501 modes for casual rooms.

### What Makes This Special

Beljot is the only platform built specifically for the Balkan Belot community — by someone from that community, for players within it. Every existing alternative (VIP Games, Playok, generic card platforms) treats Belot as an afterthought: wrong rules, no regional variants, no competitive features, dated interfaces. Beljot is the first to offer authentic Croatian and Bitola variant support alongside a full competitive ladder. The core insight is straightforward: a game with dedicated, skilled players — including those at a professional competitive level — has had zero online infrastructure. The community exists and the demand exists; this is the platform they've been waiting for.

## Project Classification

- **Project Type:** Web Application (real-time multiplayer, desktop browser)
- **Domain:** Gaming / Entertainment (online multiplayer card game platform)
- **Complexity:** Medium (real-time game state, ELO matchmaking, reconnection handling — no regulatory or compliance concerns)
- **Project Context:** Greenfield (new product, no existing codebase)

## Success Criteria

### User Success

- **Rule correctness is priority #1.** A rule implementation bug is more damaging than any UI or matchmaking issue. Players who know Belot will immediately detect incorrect behavior, and trust in the platform depends on getting the game right for both Croatian and Bitola variants.
- Players complete their first game within 5 minutes of registration (zero-friction onboarding).
- Casual players (Ana profile) can create a private room and be playing with friends in under 2 minutes.
- Competitive players (Marko profile) experience meaningful rank progression — placement matches, tier promotions, and seasonal resets create a compelling loop.
- Diaspora players (Ivan profile) can reliably play real-time games across geographic distance without technical friction.

### Business Success

- **Phase 1 — Closed Circle (Months 1–3):** ~20 active players, zero critical game-breaking bugs, rules confirmed correct by experienced Belot players for both variants.
- **Phase 2 — Soft Opening (Months 3–6):** Organic growth beyond founding circle, coin economy and honor system live and functional, no paid acquisition.
- **Phase 3 — Monetization Readiness:** Triggered at ~200 consistent MAU — the quantitative signal to open monetization discussions.

### Technical Success

- Server uptime >99.5%
- Disconnection rate <5% of active game sessions
- Real-time game state synchronization reliable enough for competitive play
- Reconnection handling works within configurable window (2-minute default)

### Measurable Outcomes

| Metric                    | Target                   | Category   |
| ------------------------- | ------------------------ | ---------- |
| Match completion rate     | >85%                     | Quality    |
| D7 new player retention   | >70%                     | Retention  |
| Session return rate (D14) | >60%                     | Retention  |
| Time to first game        | <5 minutes               | Onboarding |
| Ranked participation      | >30% of Level 5+ players | Engagement |
| Season completion rate    | >50% of ranked players   | Engagement |
| Disconnection rate        | <5%                      | Technical  |
| Server uptime             | >99.5%                   | Technical  |

## Product Scope

### MVP Strategy & Philosophy

**MVP Approach:** Experience MVP — validate that the platform delivers an authentic, correct, zero-friction Balkan Belot experience. The core test: do players who know Belot confirm the rules are right, and do they come back to play again?

**Resource Requirements:** Solo developer (passion project). The leaner the MVP, the faster it ships.

### Phase 1 — MVP

**Core User Journeys Supported:**

- Ana (casual private game with friends) — primary validation journey
- Ivan (diaspora connection via room code) — validates cross-region play
- Darko (room owner setup) — validates room configuration

**Must-Have Capabilities:**

- Email/password registration and login
- Player profile with username and basic match history
- Belot rules engine: **Bitola trump variant only** (32-card deck, 4 players, 2 teams, counter-clockwise play)
- Dealing sequence (3+2), trump selection bidding (Bitola variant: reshuffle and rotate dealer if no one picks in round 2)
- Declarations, Belote bonus (K+Q trump = 20pts), scoring (card points, last trick +10, Capot +100), failed hands
- Instant win (all 8 trump in sequence)
- 1001-point match mode only
- Lobby: Create Room, Browse/Search Rooms, Quick Play (random matchmaking)
- Room configuration: per-move timer or relaxed (reconnect window is server-defined, not configurable per room — default 2 minutes)
- Team assignment (Team A/Team B) in lobby
- Global lobby chat and per-match chat (no teammate whisper)
- Pause system: 1 pause per player per game, stackable, room owner can unpause all
- Auto-play on timer expiry: first eligible card sorted by suit then rank
- Disconnection handling with server-defined reconnect window (2 min default, no AI fill-ins)
- In-app rules reference for Bitola variant
- Desktop web only, English and Serbian (Latin), classic card visuals

**Explicitly Deferred from Phase 1:**

- Coin economy (room buy-in, daily/streak rewards, pot settlement)
- XP/Level progression (lifetime, non-gating)
- Honor system (public trust score)
- Team surrender
- Room owner kick / seat-rearrange controls
- In-game preset emotes
- Additional languages (Macedonian, Croatian)
- Croatian trump variant
- 501-point match mode
- In-app rules reference
- Seasonal rank ladder and leaderboard
- Player search, friends, public profiles
- Social login, mobile support
- Spectator, achievements, cosmetics, tournaments

### Phase 2 — Player Economy & Game Table Enhancements

- **Coin economy:** room buy-in configurable by owner (min 0, owner-freeform above); per-match settlement (not per-hand). Winners split pot; losers forfeit stake. Surrender settles identically (surrendering team loses stake). Abandonment: abandoning player forfeits their own stake, their teammate is refunded, winners split the reduced pot (3× stake when one abandons). Players with insufficient balance for the next match are ejected between matches with a modal — room and remaining seated players persist for the next match.
- **Coin sources:** 5 000 coins on registration; daily login bonus linear 1 000 → 3 100 over a 14-day uninterrupted streak (+~162/day); streak resets on missed day.
- **Quick Play economy:** default 500-coin stake; if balance < 500, player is bracketed into a matching pool by balance (can bracket down to 0).
- **XP/Level system:** lifetime progression (never resets). XP from match completion + game points scored. No gating behavior — purely a career signal, shown on profile and lobby banner.
- **Honor system:** public trust score (0–100) reflecting match-completion behavior. Signals: completed · rage-quit · timeout-abandon · unresolved-disconnect · accepted-surrender (neutral). Players with < 20 completed matches labeled "New Player" regardless of score. Profile shows score, tier label, recent trend. Rooms may optionally require a minimum honor threshold.
- **Team surrender:** one player initiates, teammate must accept; match ends immediately as opponent win; each player may trigger once per game.
- **Room owner controls (pre-game only):** kick a seated player; swap / reassign seats among seated players. Not available once the game has started.
- **In-game preset emotes:** rate-limited (max 1 per 3s per player), visible to all 4 seats.
- **Additional languages:** Macedonian and Croatian translations added to the i18n system (promoted from the later Social phase; decoupled from the friend system).

### Phase 3 — Social & Game Variant Expansion

- Player search by username
- Friend system (requests, friend list with online status, invite to room)
- Public player profiles (username, level, stats, honor tier/score with raw counts, prior-season rank archive — seasons with zero games are omitted)
- Croatian trump variant (forced pick by dealer in round 2 — no reshuffle)
- 501-point match mode
- In-app rules reference for both variants

### Phase 4 — Competitive Climb & Access Expansion

- **Seasonal rank system:** 8 tiers (Iron → Bronze → Silver → Gold → Platinum → Diamond → Immortal → Radiant) across 3-month quarterly seasons. Climb via Season Points (SP) earned per match: 50 (completion) + 100 (win) + floor(team_game_points / 10) + 50 (Capot or instant-win). Abandoners earn 0 SP. No decay. Season end → soft reset (all players start next season at Iron). Profile archives every played season; seasons with zero games are skipped on display.
- Seasonal leaderboard (top players by SP, per season)
- Social login (Facebook, Google)
- Mobile support (PWA or native)

### Phase 5 — Vision

- Spectator/observer mode
- Achievements and badges
- Cosmetics store (card backs, table themes; no pay-to-win)
- Tournaments and bracket events
- Streaming/OBS integration
- Community features (activity feeds)

### Risk Mitigation Strategy

**Technical Risks:**

- _Real-time multiplayer complexity_ — Mitigated by server-authoritative architecture; client is a thin renderer.
- _Rules engine correctness_ — Mitigated by shipping one variant first (Bitola). Validates the engine with real players before adding Croatian variant branching.
- _Single variant first_ — Reduces rules engine complexity by ~10-15% and eliminates variant-switching UI. Croatian variant shares 90%+ of logic and layers on cleanly in Phase 3.

**Market Risks:**

- _Will players adopt an online platform?_ — Mitigated by starting with a known, motivated closed circle (~20 players). Word-of-mouth validates demand.
- _Bitola-only may exclude Croatian-variant players initially_ — Acceptable risk for Phase 1 closed circle if the founding group plays Bitola.

**Resource Risks:**

- _Solo developer bottleneck_ — Mitigated by aggressive MVP scoping. The MVP is now: auth + Bitola rules engine + lobby + chat + disconnect handling. Minimal surface area.

## User Journeys

### Journey 1: Ana's Kitchen Table — Casual Private Game

**Ana**, 52, Skopje. Plays Belot every Sunday with her sister, brother-in-law, and neighbor. They've been playing at her kitchen table for years, but her sister recently moved to Ohrid. The Sunday game stopped.

**Opening Scene:** Ana gets a WhatsApp message from her sister: "I found a website where we can play Belot online — real Belot, not that French nonsense." Ana clicks the link, sees a clean registration page. Email, password, username — done in 30 seconds.

**Rising Action:** She lands in the lobby. Her sister has already created a room: "Nedelen Belot" (Sunday Belot), Bitola variant, relaxed timer, 501 mode. Ana sees it in the room list, clicks to join. Her brother-in-law and neighbor are already there. She picks Team A alongside her sister — just like at home. The room owner starts the game.

**Climax:** Cards are dealt. 3 cards, then 2, then the trump candidate appears. It looks right. It _feels_ right. Ana announces her declaration at the first trick, sees it scored correctly. Her sister plays the King and Queen of trump — "Bela!" — 20 points, just like they've always called it. Mid-hand, her neighbor's doorbell rings — he hits pause. The game freezes for everyone. He comes back a minute later, unpauses, play resumes. The hand plays out, points tallied correctly, last trick bonus applied. They're laughing in the match chat.

**Resolution:** The game finishes at 501. Ana checks the score summary — every point accounted for. She messages her sister: "Same time next Sunday?" The Sunday game is back. It never needed to be complicated. It just needed to exist.

**Capabilities revealed:** Registration flow, room browsing/joining, team self-assignment, Bitola variant rules engine, relaxed timer mode, 501 scoring, match chat, declaration handling, score summary, pause/unpause system.

---

### Journey 2: Marko's Climb — Ranked Competitive Play

**Marko**, 31, Belgrade. Played Belot semi-professionally in local café tournaments. He's sharp, strategic, and frustrated — there's nowhere online to test himself against other serious players. No rankings, no stakes, no way to know where he stands.

**Opening Scene:** Marko registers, picks a username. He wants to go straight to ranked but sees it's locked — Level 5 required. Fair enough. He hits Quick Play to start grinding XP.

**Rising Action:** His first few casual games are smooth — Croatian variant, 1001 mode, per-move timer. The rules are correct. He notices the card point table is right, declarations work properly, failed hands transfer points correctly. He earns XP from game points. After roughly 20 games, he hits Level 5. Ranked mode unlocks.

**Climax:** Marko queues for ranked. The system matches him with players around his hidden ELO. First match — it's tight, competitive, the timer keeps things moving. One opponent hesitates too long on a critical trick — timer expires, and the system auto-plays their first eligible card (sorted by suit, then rank). It's a weak play. Marko capitalizes. He wins. Then two more placement matches. After three games, his rank reveals: Silver II. He can see the tier ladder — Iron through Radiant. The season has just started. He has three months to climb.

**Resolution:** Two weeks in, Marko checks the seasonal leaderboard. He's Gold I now, 47th on the board. He sees names he recognizes from café tournaments. For the first time, the competitive Belot scene exists online — and he has a number next to his name. He queues another match.

**Capabilities revealed:** Registration, XP/level progression, Level 5 gate for ranked, Quick Play matchmaking, Croatian variant rules engine, 1001 scoring, per-move timer, auto-play on timer expiry (first eligible card by suit/rank order), ELO matchmaking, placement matches, rank reveal, tier system, seasonal leaderboard.

---

### Journey 3: Ivan's Saturday Bridge — Diaspora Connection

**Ivan**, 42, Stuttgart. Macedonian, moved to Germany 12 years ago. Misses Saturday card nights with his cousins back in Bitola. They tried playing over video call once — it didn't work.

**Opening Scene:** Ivan's cousin sends him a room code via Viber: "SUBOTA-BELOT". Ivan registers (he's new to the platform), enters the lobby, searches by room code.

**Rising Action:** He finds the room — Bitola variant, relaxed timer, 1001 mode. His two cousins are already in, waiting for a fourth. Ivan joins, picks Team B with his regular partner. The fourth slot fills — another cousin who was also waiting for an invite. The room owner starts the game.

**Climax:** The dealing feels right — 3-2 split, trump candidate revealed. Second round of bidding, nobody picks — in Bitola variant, the deck reshuffles and the dealer rotates. Exactly how they play at home. Ivan announces a sequence declaration, it's validated correctly against the others. The hand plays out counter-clockwise, scoring is precise. They're bantering in the match chat between hands, half in Macedonian, half giving each other grief.

**Resolution:** They play three full 1001-point games that evening. Ivan checks his match history — all three games recorded with full scoring detail. His stats show his win/loss record. He messages the group chat: "Next Saturday, same time." The distance doesn't matter anymore.

**Capabilities revealed:** Registration, room search by code, team assignment, Bitola variant with reshuffle/rotate mechanic, relaxed timer, 1001 scoring, declaration validation, counter-clockwise play, match chat, match history, player stats.

---

### Journey 4: Room Owner — Setting Up the Table

**Darko**, 38, Zagreb. Organizing a game for his group of four. He's the one who always sets the rules at the café — now he does it online.

**Opening Scene:** Darko logs in, hits "Create Room" from the lobby. He sees the configuration options.

**Rising Action:** He sets the room up: names it "Zagreb Ekipa", picks Croatian variant (his group's preference), 1001 mode, per-move timer at 30 seconds, reconnect window at 3 minutes (one of them has spotty wifi). The room is created and appears in the room list. He copies the room name and sends it to his group chat.

**Climax:** Players join one by one. Darko sees them arrive in the lobby. Two friends want to be on the same team — they self-assign to Team A. The fourth arrives, takes the open Team B slot. All four seats filled. Darko reviews the setup: Croatian variant, 1001, 30s timer, 3min reconnect. Everything's right. He starts the game. Mid-game, one friend needs a quick break and uses his pause. A minute passes — Darko decides the break has been long enough and uses his room owner override to unpause all active pauses. Play resumes.

**Resolution:** The game launches with his exact configuration. Timer ticks on each move — keeps the pace up, just like when they play for real. Mid-game, his friend's wifi drops. The reconnect window activates — "Player reconnecting..." — and 90 seconds later he's back, game state intact. The room owner role did its job: Darko set the table, his way.

**Capabilities revealed:** Create Room flow, room configuration (variant, mode, timer, reconnect window), room visibility in browse list, room code/name sharing, team self-assignment, lobby state display, game start trigger, pause system (player pause + room owner override unpause), reconnect window activation.

---

### Journey 5: Edge Case — Disconnection and Timer Expiry

**Scenario A — Disconnection Mid-Game:** Competitive ranked match, 1001 mode, Croatian variant. Four players, mid-game — the score is close (Team A 620, Team B 580). Player 3 (Team B) loses internet connection during trick 4 of a hand.

Player 3's client goes offline. The server detects the disconnection. The other three players see a notification: "Player 3 disconnecting — reconnect window: 2:00." The game pauses. A countdown begins.

**Reconnection Success:** Player 3's internet comes back at 1:12 remaining. Their client reconnects, game state is restored exactly where it left off — same cards in hand, same trick in progress. The notification clears. Play resumes. No data lost, no advantage gained.

**Reconnection Failure:** The 2-minute window expires. Player 3 does not reconnect. The match is abandoned. Player 3 receives a scaled ELO penalty (mid-game abandon = ~×1.5 multiplier). The other three players receive no ELO change — the match is voided. Player 3 earns no XP.

**If this were casual:** Player 3 earns no XP. The remaining three players earn partial XP based on game progress. The match ends.

**Scenario B — Timer Expiry (Connected but Inactive):** Same match, but Player 3 is connected — they're just distracted or stalling. The 30-second per-move timer counts down. It hits zero. The system auto-plays for Player 3: it selects the **first eligible card** from their hand, sorted by suit then rank, that is a legal play under the current trick's rules (follows suit requirements, trump obligations, etc.). The card is played automatically. Play continues — no pause, no penalty beyond the forced suboptimal play. Player 3's turn is over.

**Capabilities revealed:** Disconnection detection, reconnect window with countdown, game state preservation, game pause/resume, abandon handling, scaled ELO penalty (competitive), partial XP calculation (casual), match voiding, auto-play on timer expiry (first eligible card sorted by suit/rank), legal move validation for auto-play.

---

### Journey Requirements Summary

| Journey             | Key Capabilities Revealed                                                                                                                                              |
| ------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Ana (Casual)        | Registration, room browse/join, team assignment, Bitola variant, relaxed timer, 501 mode, match chat, declarations, scoring, pause/unpause                             |
| Marko (Competitive) | XP/level progression, Level 5 gate, Quick Play, Croatian variant, 1001 mode, per-move timer, auto-play on expiry, ELO matchmaking, placement, rank system, leaderboard |
| Ivan (Diaspora)     | Room search by code, Bitola reshuffle/rotate, match history, player stats, cross-region real-time play                                                                 |
| Darko (Room Owner)  | Create Room config, variant/mode/timer/reconnect settings, room sharing, lobby management, game start, pause override                                                  |
| Edge Cases          | Disconnect detection, reconnect window, game state preservation, pause/resume, ELO penalty scaling, partial XP, match voiding, auto-play (suit/rank order)             |

**Cross-cutting capabilities:** Authentication, real-time game state sync, rules engine (both variants), scoring engine, lobby system, chat system, pause system (1 per player per game, stackable, owner override), auto-play on timer expiry (first legal card by suit/rank sort).

## Web Application Requirements

### Overview

Single Page Application (SPA) for desktop browsers. Real-time multiplayer with persistent WebSocket connections for game state, lobby updates, and chat. No SEO requirements. No accessibility compliance targets for MVP.

### Browser Support

- **Target:** Modern evergreen browsers only — Chrome, Firefox, Edge, Safari (latest 2 versions)
- **Desktop only:** No responsive/mobile layout required for MVP
- **Minimum viewport:** 1280x720+

### Real-Time Architecture

- **WebSocket connections** required for: game state synchronization, lobby updates (player join/leave, room creation), chat messages (global and match), pause/unpause events, disconnection detection and reconnect handling, timer synchronization
- **Server-authoritative game state** — all game logic (card validation, scoring, declarations, auto-play) runs server-side; client renders received state only; no client-side game logic
- **Latency tolerance:** Turn-based play; sub-second latency acceptable; timer display syncs within ±1 second of server time

### SPA Requirements

- Client-side routing for lobby, game room, profile, leaderboard, and rules reference views
- Authenticated session state persists across page refreshes
- i18n support for English and Serbian (Latin) at launch, extensible for future languages

## Functional Requirements

### User Account Management

- FR1: Players can register a new account using email and password
- FR2: Players can log in and maintain authenticated sessions across page refreshes
- FR3: Players can authenticate using third-party social login providers (Google, Facebook)
- FR4: Players can view their own profile displaying username, level, stats summary, and match history
- FR5: Players can search for other players by username
- FR6: Players can send, accept, and decline friend requests and maintain a friend list

### Game Rules Engine

- FR7: The system enforces Bitola variant rules: 3+2 dealing sequence, reshuffle-and-rotate-dealer trump bidding mechanic when no player selects trump in round 2, counter-clockwise play, and variant-specific scoring
- FR8: The system enforces Croatian variant rules: 3+2 dealing sequence, forced trump selection by last player in bidding, counter-clockwise play, and variant-specific scoring
- FR9: The system validates and scores declarations at the first trick — highest-value set wins ties; only the winning team's declarations count
- FR10: The system awards the Belote bonus (K+Q of trump held by same player = 20 pts) when announced during play
- FR11: The system applies failed hand scoring: the failing team scores 0 pts and all points transfer to opponents
- FR12: The system awards last-trick bonus (+10 pts) to the team winning the final trick, and applies Capot scoring (+100 pts, replacing last-trick bonus) when one team takes all tricks
- FR13: The system detects and resolves the instant-win condition when a player holds all 8 trump in sequence
- FR14: The system supports 1001-point match mode
- FR15: The system supports 501-point match mode

### Lobby & Room Management

- FR16: Players can create a room and configure settings: game variant (Bitola/Croatian), match mode (1001/501), timer style (per-move or relaxed), and reconnect window duration
- FR17: Players can browse a searchable list of open rooms by room name or code
- FR18: Players can join a room via the browse list or by entering a room name/code directly
- FR19: Players can queue for Quick Play to be matched into a random available game
- FR20: Players can self-assign to Team A or Team B within a room lobby before a game starts
- FR21: Room owners can start the game once all four player slots are filled
- FR22: Room owners can override and clear all active player pauses during a match

### Real-Time Game Session

- FR23: Four players can participate in a real-time Belot match with game state continuously synchronized to all participants
- FR24: Players can pause an active match (1 pause per player per game; multiple active pauses stack; room owner can override all)
- FR25: The system auto-plays the first eligible legal card (sorted by suit then rank) on behalf of a player when their per-move timer expires
- FR26: The system detects a player disconnection, pauses the match, and displays a reconnect countdown timer to all remaining players
- FR27: A disconnected player can reconnect within the reconnect window and resume from preserved server-side game state
- FR28: The system abandons the match and applies appropriate XP/ELO outcomes when the reconnect window expires without reconnection
- FR28a: During an active match, a player can initiate a team surrender request; the request is only executed if their teammate accepts it; upon acceptance, the match ends immediately as a win for the opposing team; each player may trigger a surrender request at most once per game
- FR29: Players can access an in-app rules reference for both Belot variants from the lobby and during an active match

### Communication

- FR30: All logged-in players can send and receive messages in a global lobby chat
- FR31: Players in an active match can send and receive messages in match-scoped chat, visible only to the four match participants
- FR32: Players can express reactions during a match using preset in-game emotes

### Player Progression & Competitive System

- FR33: Players earn XP from completed matches proportional to game points scored in that match
- FR34: Players advance through a level system as XP accumulates, with Level 5 unlocking access to ranked mode
- FR35: Level 5+ players can queue for ranked competitive matches with ELO-based opponent pairing
- FR36: The system conducts 3 placement matches per season before revealing a player's initial rank
- FR37: Players can view their current rank tier (8 tiers: Iron → Bronze → Silver → Gold → Platinum → Diamond → Immortal → Radiant)
- FR38: The system applies scaled ELO penalties to players who abandon ranked matches, with penalty scaling by game progress at time of abandonment
- FR39: Players can view a seasonal leaderboard of top-ranked players
- FR40: The system runs quarterly ranked seasons with rank resets; prior season rank history is preserved and viewable

### Stats & Match History

- FR41: Players can view their full match history with per-match scoring detail and outcomes
- FR42: Players can view career statistics including win/loss record, points scored, and rank history across all seasons
- FR43: Remaining players in an abandoned casual match receive partial XP based on game progress at time of abandonment; the abandoning player receives none

### Platform, Localization & Future Capabilities

- FR44: Players can use the platform in English or Serbian (Latin script), with language selectable as a player preference
- FR45: The platform supports additional languages (Macedonian, Croatian) as extended language options
- FR46: The platform is fully functional on desktop web browsers (Chrome, Firefox, Edge, Safari — latest 2 major versions)
- FR47: Players can view public-facing profiles of other players
- FR48: Players can observe ongoing matches in spectator/observer mode
- FR49: Players can earn and display achievements and badges on their profile
- FR50: Players can purchase cosmetic items (card backs, table themes) that have no effect on gameplay
- FR51: Players can participate in bracket-style tournament events with seasonal scheduling
- FR52: Players can access the platform via a mobile-optimized experience (progressive web app or native client)

## Non-Functional Requirements

### Performance

- The per-move timer display must remain synchronized with server time within ±1 second
- Game state updates received from the server must render on the client within 200ms
- All card play actions (card selection, trick resolution, score update) must be reflected across all four clients within 500ms under normal network conditions
- The application shell (initial page load) must complete within 3 seconds on a standard broadband connection
- WebSocket reconnection attempts must begin automatically within 1 second of detecting a dropped connection

### Security

- All client-server communication must use encrypted protocols (HTTPS for requests, WSS for WebSocket connections)
- User passwords must be stored using a one-way cryptographic hash; plaintext passwords must never be stored or logged
- All game logic (card validation, declaration scoring, auto-play selection, ELO calculation) must execute server-side; the client must not be able to influence game outcomes by sending unsanctioned messages
- Authentication sessions must use time-limited tokens with secure refresh mechanisms
- Player account data must be accessible only to the authenticated account owner and authorized platform administrators

### Scalability

- The system must support Phase 1 capacity: up to 10 concurrent game sessions (~40 simultaneous players) without performance degradation
- The system must support Phase 2–3 capacity: up to 50 concurrent game sessions (~200 simultaneous players) without architectural redesign
- The platform architecture must permit horizontal scaling to accommodate growth beyond Phase 3 without a full rebuild

### Reliability

- Server uptime must exceed 99.5% measured on a rolling monthly basis
- The rate of WebSocket connections dropped during active game sessions must remain below 5%
- Server-side game state must be fully preserved through any client disconnection event, ensuring reconnecting players restore to an identical match state
- A single player's disconnection must not affect game state integrity or client connectivity for the remaining three players
