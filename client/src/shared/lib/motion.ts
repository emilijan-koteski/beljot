/**
 * Single source of truth for timed UI behaviour. Values are milliseconds.
 *
 * Why centralize: tuning game feel is iterative, and a 50 ms tweak to
 * `CARD_THROW` should land in one place — not in five components. Names
 * describe the user-visible event, not the implementation.
 *
 * Conventions:
 *  • Most names map 1:1 to a single setTimeout / animation / transition.
 *  • `*_REDUCED` siblings exist where prefers-reduced-motion mandates a
 *    different (typically much shorter) value.
 *  • Tailwind `duration-XXX` classes that already encode these values are
 *    flagged in the comment so we don't drift.
 */
export const MOTION = {
  // ─── Card flight (game-flow critical) ────────────────────────────
  /** Hand card slides off-screen-bottom on throw. Long enough to register
   *  as deliberate; short enough that it doesn't precede the responding
   *  trick-land animation by an awkward pause. Used as the legacy / reduced-
   *  motion fallback when the CardFlight overlay can't read DOM rects. */
  CARD_THROW: 280,

  /** Self's card flying up from screen-bottom into the south slot.
   *  Slightly longer than opponent landings to match the longer travel.
   *  Legacy / fallback path. */
  CARD_LAND_SELF: 460,

  /** Opponent's card growing from their deck to their slot. Legacy / fallback path. */
  CARD_LAND_OPPONENT: 360,

  /** CardFlight overlay — self throw, hand → south trick slot. The flight
   *  arcs through screen-bottom-center so the gesture reads as one continuous
   *  motion instead of two separate hand-throw + trick-land animations. */
  CARD_FLIGHT_SELF_THROW: 520,

  /** CardFlight overlay — opponent throw, their CardBackStack → their compass
   *  trick slot. */
  CARD_FLIGHT_OPPONENT_THROW: 420,

  /** CardFlight overlay — collect after a trick resolves. All four trick
   *  slots → winner's deck (or, when the local player wins, the hand region
   *  near screen-bottom-center). Runs after TRICK_RESOLVE_PAUSE. */
  CARD_FLIGHT_COLLECT: 560,

  /** Winner-glow hold after the trick fills. Long enough for players to
   *  identify the winning card before the cards collect away. */
  TRICK_RESOLVE_PAUSE: 1000,

  /** Trick cards slide to the winner's pile and fade. Long enough to read
   *  as deliberate; short enough that the next trick doesn't feel slow. */
  TRICK_COLLECT: 600,

  // ─── Card state (interactive) ────────────────────────────────────
  /** Lift / dim when a card becomes playable or unplayable. Mirrors the
   *  Tailwind `duration-150` class on PlayingCard. */
  CARD_STATE: 150,

  // ─── Deal sequence ───────────────────────────────────────────────
  /** Per-card slide as cards deal to seats. Mirrors Tailwind
   *  `duration-300` in DealAnimation. */
  DEAL_CARD: 300,
  /** Inter-card stagger during the deal. */
  DEAL_STAGGER: 80,
  /** Phase 1 of dealing: cards visibly dealt to seats. */
  DEAL_PHASE_DEAL: 800,
  /** Phase 2 of dealing: trump candidate flipped face-up. */
  DEAL_PHASE_TRUMP: 1200,

  // ─── Reshuffle ───────────────────────────────────────────────────
  RESHUFFLE_PULSE: 1200,
  RESHUFFLE_STAGGER: 100,

  // ─── Game-start splash ───────────────────────────────────────────
  /** Minimum hold time for the "Game is starting…" splash on MatchPage when
   *  arriving from RoomPage or from LobbyPage quick-play auto-start. 1500 ms
   *  is long enough for the brain to register a deliberate scene change
   *  without feeling like a spinner — slightly longer than
   *  `TRICK_RESOLVE_PAUSE` so the beat reads as a phase transition, not a
   *  per-trick interaction. Reload / reconnect mounts skip this hold via the
   *  `fromRoom` route-state flag. */
  GAME_STARTING_SPLASH: 1500,
  /** Reduced-motion variant — short enough to feel near-instant, long enough
   *  to mask the deal animation's first frame so the splash still serves its
   *  purpose for users who can perceive the transition. */
  GAME_STARTING_SPLASH_REDUCED: 400,

  // ─── Reveal entrances ────────────────────────────────────────────
  /** Quick zoom-in for input-gating prompts (trump pick, declare,
   *  belot). Mirrors Tailwind `duration-150`. */
  REVEAL_QUICK: 150,
  /** Standard panel slide-in for non-blocking reveals (score reveal,
   *  trump/belot/declaration toasts). Mirrors Tailwind `duration-300`. */
  REVEAL_PANEL: 300,

  // ─── Capot banner ────────────────────────────────────────────────
  /** Capot scale-bounce entrance + dwell. Matches the `capot-scale`
   *  keyframe in `index.css`. */
  CAPOT_BANNER: 2500,
  CAPOT_BANNER_REDUCED: 500,

  // ─── Hover / interactive surface transitions ─────────────────────
  /** Quick hover / press feedback on classic buttons. */
  BUTTON_HOVER: 120,
  /** Player-seat ring & name-pill border transitions (turn glow on/off). */
  SEAT_RING_TRANSITION: 300,
  /** Self-label fade-in delay in the room lobby. */
  ROOM_SELF_LABEL_DELAY: 300,

  // ─── Ring color flip (urgent state) ──────────────────────────────
  /** TimerRing stroke + label color flip (lime → red urgent). */
  RING_COLOR_FLIP: 300,
  /** ButtonTimerRing stroke color flip — slightly faster because the
   *  smaller ring reads quicker. */
  RING_COLOR_FLIP_FAST: 200,

  // ─── Auto-dismiss timers ─────────────────────────────────────────
  /** Error toast auto-hide. */
  TOAST_ERROR: 3000,
  /** Info / status toast (sonner). Same as `TOAST_ERROR` — informational
   *  toasts share the same dwell rhythm as the error toast for consistency. */
  TOAST_INFO: 3000,
  /** Longer toast for events that need more reading time
   *  (e.g. start-game failed, capot announcement). */
  TOAST_LONG: 5000,
  /** Emote bubble dwell. */
  EMOTE_BUBBLE: 2000,
  EMOTE_BUBBLE_REDUCED: 1000,
  /** Chat-rail peek preview when the panel is closed. */
  CHAT_PEEK: 2000,
  /** Hand-end score reveal auto-continues after this long so an AFK player
   *  can't strand the table waiting on their "Continue" click — matches the
   *  8 s auto-close the informational reveals use. The server keeps a longer
   *  backstop for fully-disconnected clients. */
  SCORE_REVEAL_AUTO_CONTINUE: 8000,
  /** Last-trick bonus number flash on the scoreboard. */
  SCORE_BONUS_FADE: 1200,
  SCORE_BONUS_FADE_REDUCED: 300,
  /** Auto-redirect to lobby after match abandonment. */
  RECONNECT_REDIRECT: 3000,

  // ─── Cooldowns (rate limits, not animation) ──────────────────────
  /** Spam guard between emote sends. */
  EMOTE_COOLDOWN: 3000,

  // ─── Server-sync timers ──────────────────────────────────────────
  /** Wall-clock countdown tick (turn ring, reconnect ring, auto-close
   *  ring). Server-aligned — keep at 1000 unless you change all rings. */
  COUNTDOWN_TICK: 1000,
} as const;

/**
 * Internal coordination: a transient flag that must outlive its driving
 * animation needs `duration + small safety buffer`. Centralizing the buffer
 * keeps these flags in lock-step with the durations they shadow.
 */
const FLAG_BUFFER_MS = 40;

export const FLAG_LIFETIME = {
  /** MatchPage's `flyingCardId` — must outlive whichever throw animation runs.
   *  When the CardFlight overlay is active (the common case) this guards the
   *  full self-throw flight; when it falls back to the legacy hand-throw +
   *  trick-land sequence it still covers the longer of the two. */
  FLYING_CARD: Math.max(MOTION.CARD_FLIGHT_SELF_THROW, MOTION.CARD_THROW) + FLAG_BUFFER_MS,
  /** TrickArea's `incomingCompass` — must outlive the longest legacy land. */
  INCOMING_COMPASS: MOTION.CARD_LAND_SELF + FLAG_BUFFER_MS,
} as const;

/**
 * Pick a duration constant respecting prefers-reduced-motion.
 * Pass the normal and reduced values; returns whichever applies.
 */
export function motionDuration(prefersReduced: boolean, normal: number, reduced: number): number {
  return prefersReduced ? reduced : normal;
}
