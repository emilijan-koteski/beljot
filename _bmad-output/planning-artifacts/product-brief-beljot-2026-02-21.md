---
stepsCompleted: [1, 2, 3, 4, 5]
inputDocuments: []
date: 2026-02-21
author: Emilijan
---

# Product Brief: beljot

<!-- Content will be appended sequentially through collaborative workflow steps -->

## Executive Summary

Beljot is a dedicated online multiplayer platform for Balkan Belot — the regional card game beloved across Macedonia, Serbia, and Croatia — bringing a modern, competitive-grade experience to a passionate community that has been underserved by outdated, generic card game platforms.

The platform targets four-player team-based Belot (Croatian and Bitola variants) on desktop web, offering both casual social play and a full competitive ecosystem with ELO matchmaking, seasonal rankings, and leaderboards — the first of its kind for this community.

---

## Core Vision

### Problem Statement

Balkan Belot players across Macedonia, Serbia, and Croatia have no dedicated, high-quality online home for their game. The few platforms that exist (VIP Games, Playok) are dated in design, French-language-first in orientation, and offer no competitive infrastructure. Players who want to play with friends at a distance — or compete at a higher level — have nowhere meaningful to go.

### Problem Impact

A culturally rich, multigenerational card game community is scattered across inadequate platforms. Diaspora players can't easily find or play with others who share their regional variant knowledge. No competitive scene can emerge without proper infrastructure — no rankings, no seasons, no stakes. The game lives in kitchens and cafés but has no digital equivalent that does it justice.

### Why Existing Solutions Fall Short

- **VIP Games / Playok**: Outdated UI, no competitive features, not built around Balkan Belot rules or the communities that play them.
- **Generic card game platforms**: Don't implement the specific rule variants (Croatian / Bitola) that define the authentic experience for these players.
- **No platform** offers ELO-based ranked play, seasonal competition, or leaderboards for Balkan Belot — the competitive scene simply doesn't exist online yet.

### Proposed Solution

A purpose-built desktop web application for Balkan Belot, designed from the ground up for the Macedonian, Serbian, and Croatian playing community. The platform features:

- Authentic rules implementation supporting both Croatian and Bitola (Bitola) trump variants, selectable per room
- Full account system with profiles, match history, and statistics
- Casual social play via CIV 6-style lobbies (Browse/Create/Quick Play)
- Competitive ranked mode with ELO matchmaking, 8-tier rank system (Iron → Radiant), quarterly seasons, and leaderboards
- English and Serbian (Latin) language support at launch

### Key Differentiators

- **Only dedicated platform** for authentic Balkan Belot with both regional rule variants
- **First competitive infrastructure** for this community — ELO, seasons, leaderboards where none exist today
- **Community-first design**: built for Balkan players, in their language, with their rules — not a French game with a region selector
- **Modern UX** against a field of decade-old competitors
- **Authentic card experience**: classic standard deck, no gimmicks

---

## Target Users

### Primary Users

#### Persona 1 — The Competitive Regular

**"Marko"** — Late 20s to late 30s, urban Balkan (Belgrade, Zagreb, Skopje or diaspora equivalent). Grew up with Belot as a cultural staple. Plays online but has no good dedicated option. Frustrated by platforms with no ranking, no stakes, no sense of progression. He wants to know where he stands — a rank to climb, a season to grind, a leaderboard to appear on.

- **Current workaround:** Plays on generic platforms (VIP Games, Playok) or doesn't play online at all
- **Core need:** Competitive infrastructure — ELO, ranks, seasons
- **Success moment:** Seeing his rank tier update after a winning streak; reaching a new division for the first time
- **Platform role:** Power user, most active session count, organic ambassador if competitive experience is right

#### Persona 2 — The Social/Casual Player

**"Ana"** — Broad age range (40s–60s+), plays Belot socially at family gatherings and with a fixed circle of friends. Not interested in rankings. Wants to recreate the kitchen-table experience online. Values simplicity and familiarity above all else. **This is the early adopter profile.**

- **Current workaround:** Doesn't play online; waits for in-person occasions
- **Core need:** Create a private room, share it with 3 friends, be playing within 2 minutes. Zero friction.
- **Success moment:** First completed game with her regular group — "it felt just like playing at home"
- **Platform role:** First wave of real users; validates core game experience; drives word-of-mouth in the social circle

#### Persona 3 — The Diaspora Player

**"Ivan"** — 30s–50s, Macedonian/Serbian/Croatian living abroad (Vienna, Stuttgart, Stockholm, etc.). Misses the card table as a cultural touchstone. Wants to play with family back home and occasionally compete with strangers. Part casual, part competitive — flexible depending on the week.

- **Current workaround:** WhatsApp calls while playing on separate platforms; or simply not playing
- **Core need:** Reliable real-time connection, easy way to find specific friends' rooms without a formal friend system
- **Success moment:** Playing a Saturday evening room with cousins back home with zero technical friction
- **Platform role:** Loyal long-term user; high motivation to return regularly; may recruit both ends of the connection (abroad and home)

---

### Secondary Users

No formal secondary user roles in v1. The room owner acts as a lightweight admin (sets rules, manages team assignment) but is also a player. No spectator or observer role in scope.

---

### User Journey

#### Discovery

- Primary channel: direct invitation from a friend (room name/code sharing)
- Secondary: word of mouth within Balkan social and diaspora communities
- No paid acquisition planned for passion project phase

#### Onboarding

- Registration required (email/password) — no guest play
- In-app rules reference available from the start: covers full Belot rules, both Croatian variant and Bitola variant, with clear explanations for regional differences (critical for players who know "a version" of the game but need to understand this platform's implementation)
- First game: casual room, ideally with known players

#### Core Usage

- **Ana / casual:** Create or join private rooms on a recurring schedule (weekly sessions with a fixed group)
- **Marko / competitive:** Splits time between ranked queue and private games; monitors rank and leaderboard standing
- **Ivan / diaspora:** Weekend private rooms with family; occasional ranked games during the week

#### The "Aha" Moment

- **Ana:** First completed private game with friends — seamless, familiar, fun
- **Marko:** First rank promotion or placement reveal after season start
- **Ivan:** Reconnecting with home through a Saturday game, zero technical friction

#### Long-Term Retention

- **Competitive loop:** Seasonal rank resets create a recurring motivation to return and re-climb each quarter
- **Social loop:** Fixed friend groups establish habitual play sessions
- **Rules reference:** Reduces drop-off from confusion about variant differences — players who understand the rules stay engaged

---

## Success Metrics

### User Success Metrics

These measure whether the platform is creating real value for players:

- **Match completion rate** — % of started matches that finish without abandonment. Target: >85%. Primary quality signal for the game experience.
- **New player retention (D7)** — % of newly registered players who complete at least one game in their first 7 days. Target: >70%.
- **Session return rate** — % of players who return for a second session within 14 days of their first. Target: >60%.
- **Time to first game** — Minutes from registration to completing a first match. Target: <5 minutes (validates zero-friction onboarding).

---

### Business Objectives

Phased, reflecting passion project → public launch trajectory:

**Phase 1 — Closed Circle (Months 1–3):**

- Stable, bug-free gameplay validated by the founding player group (~20 active players)
- All core rules confirmed correct by players who know Belot in practice
- Zero critical game-breaking bugs in production

**Phase 2 — Soft Opening (Months 3–6):**

- Platform open beyond the founding circle via organic word of mouth
- Ranked system live and sufficiently populated for matchmaking to function
- No paid acquisition — growth driven entirely by player referral

**Phase 3 — Monetization Readiness:**

- Triggered at ~200 consistent monthly active players
- Decision is feeling-based but 200 MAU is the clear quantitative signal to open monetization discussions

---

### Key Performance Indicators

| KPI                       | Target                                        | Phase      |
| ------------------------- | --------------------------------------------- | ---------- |
| Monthly Active Players    | 20+ (Phase 1), 100+ (Phase 2), 200+ (Phase 3) | Growth     |
| Match completion rate     | >85%                                          | Quality    |
| D7 new player retention   | >70%                                          | Retention  |
| Session return rate (D14) | >60%                                          | Retention  |
| Ranked participation rate | >30% of Level 5+ players                      | Engagement |
| Season completion rate    | >50% of ranked players finish a season        | Engagement |
| Disconnection rate        | <5% of active game sessions                   | Technical  |
| Server uptime             | >99.5%                                        | Technical  |

---

## MVP Scope

### Core Features (v1)

#### Authentication & Accounts

- Email/password registration and login (registration required, no guest play)
- Player profile: username, stats summary, match history
- No social login

#### Rules Engine

- Full Balkan Belot — Croatian variant and Bitola variant (selectable per room)
- 32-card standard deck, 4 players, 2 teams, counter-clockwise play throughout
- Dealing sequence: 3 cards each → 2 cards each → trump candidate revealed
- Trump selection bidding (Croatian variant: last player forced to pick; Bitola variant: reshuffle and rotate dealer if no one picks in round 2)
- Declarations: player-announced at first trick, highest card wins ties, only winning team's declarations count
- Belote bonus: K+Q of trump in same hand = 20pts, announced during play
- Scoring: full card point table (trump/non-trump), last trick +10, Capot +100 (replaces last trick bonus)
- Failed hand: 0pts for failing team, all points transferred to opponents
- Instant win declaration: all 8 trump in sequence = match over immediately (in both 1001 and 501 modes)
- Match modes: 1001 (competitive-eligible) and 501 (casual/quick only)

#### Lobby & Matchmaking

- **Quick Play** — random matchmaking queue
- **Browse/Search Rooms** — searchable room list by name/code
- **Create Room** — owner configures: mode (1001/501), trump variant (Croatian/Bitola), per-move timer or relaxed, reconnect window duration
- Team assignment (Team A/Team B) in lobby — friends can self-assign
- 2, 3, or 4 friends can queue together; remaining slots filled by matchmaking

#### Chat

- **Global lobby chat** — visible to all logged-in players
- **Match chat** — per-room chat visible only to the 4 players in that match
- No private/whisper chat between teammates (anti-cheating rule)

#### Player Progression

- XP earned from match play (game points = XP earned)
- Level system — XP only increases, never decreases
- Level 5 (approx. 20 completed games) unlocks ranked mode

#### Casual Mode

- Private and public rooms
- Per-move timer (selectable duration) or relaxed (no timer)
- No ELO impact; disconnection = no XP for that match for disconnecting player

#### Competitive Mode

- Separate ranked queue (Level 5+ only, registration required)
- ELO matchmaking engine (hidden from players)
- 8 visible rank tiers: Iron → Bronze → Silver → Gold → Platinum → Diamond → Immortal → Radiant (each with I/II/III sub-ranks; Radiant = top-N players only)
- Quarterly seasons (3 months); rank silently preserved across seasons
- 3-game placement matches at season start; rank revealed after placement
- Ranked only supports 1001-point matches
- Seasonal leaderboard

#### Disconnection Handling

- Reconnect window: 2 minutes default, configurable at room creation
- No AI fill-ins; match abandoned if player doesn't reconnect
- Competitive: scaled ELO penalty (×0.5 early abandon → ×2.0 late abandon)
- Casual: disconnecting player earns no XP; remaining players earn partial XP based on game progress

#### Stats & Match History

- Per-player statistics: win/loss record, points scored, rank history
- Full match history with result and scoring detail

#### In-App Rules Reference

- Complete Belot rules documentation
- Croatian variant and Bitola variant explained with regional differences highlighted
- Accessible from lobby and during game

#### Platform & UI

- Desktop web application only
- Classic standard playing card visuals (♠♥♦♣)
- English and Serbian (Latin script) language support
- No monetization features

---

### Out of Scope for MVP

- Mobile support (phone/tablet)
- Spectator/observer mode
- Friend system and friend requests
- Social login (Facebook, Google, etc.)
- Additional languages (Macedonian, Croatian planned for future)
- Monetization layer (cosmetics, premium, ads)
- AI/bot opponents
- Tournaments or special competitive events
- Achievements or badge system
- In-game emotes
- Custom card skins or themes
- Additional game variants beyond Belot

---

### MVP Success Criteria

The MVP is considered successful when:

- The founding player group (~20 players) completes sessions without game-breaking bugs or rule errors
- Match completion rate exceeds 85% in the closed circle phase
- Players who know Belot confirm the rules implementation is correct for both Croatian variant and Bitola variant
- At least one full quarterly season completes with ranked play functional

---

### Future Vision

If Beljot succeeds and grows beyond the founding circle:

- **Mobile** — Progressive web app or native mobile client
- **Social layer** — Friend system, friend requests, player search
- **Additional languages** — Macedonian and Croatian as natural next step
- **Spectator mode** — Watch ongoing matches (particularly ranked games)
- **In-game emotes** — Preset reactions and expressions at the table
- **Monetization** — Cosmetic items (card backs, table themes), optional premium features; no pay-to-win
- **Tournaments** — Seasonal events, bracket-style competitions
- **Achievements** — Milestones, badges, profile flair
- **Community features** — Public player profiles, activity feeds
- **Streaming support** — OBS integration or stream-friendly layout for content creators covering the competitive scene
