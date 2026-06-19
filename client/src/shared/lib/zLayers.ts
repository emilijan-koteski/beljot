/**
 * Single source of truth for in-match stacking order (z-index).
 *
 * Every overlay/layer that can paint over the table reads its tier from here so
 * "which dialog is above which" is decided by intent, not by scattered `z-NN`
 * classes and DOM order. Higher number = closer to the viewer.
 *
 * Two bands:
 *   • Table band (0–50)  — everything that lives *under* every dialog.
 *   • Dialog band (60+)  — reveals, prompts, utilities, blockers, chrome.
 *
 * The values are intentionally spaced so new layers can slot between existing
 * ones without a renumber. Apply via inline `style={{ zIndex: Z.X }}` (not a
 * Tailwind `z-` class) so the value is traceable to this file.
 *
 * ── The one subtlety: the bidder's hand ──────────────────────────────────
 * Action-prompt panels use {@link OverlayBackdrop}, which paints a full-screen
 * dim (`PROMPT_DIM`) and the panel (`PROMPT`) as two separate global layers.
 * During the TRUMP prompt the active bidder's hand is lifted to `BIDDER_HAND`
 * — between the dim and the panel — so they can read their cards to decide
 * take/pass. The declaration and belot prompts deliberately do NOT lift the
 * hand (you don't pick a card to answer them), so it stays in `SEATS`, dimmed.
 * This only works while the trump prompt's backdrop is rendered "untrapped"
 * (no positioned ancestor with its own z-index between it and the page root).
 */
export const Z = {
  // ── Table band — below every dialog ──────────────────────────────────
  /** Felt, ambience, wordmark, score panel, trump-suit pill. */
  TABLE: 0,
  /** The cards currently played to the centre (trick area). */
  TRICK: 10,
  /** Seats: avatars, names, opponent card-backs, and the resting hand. */
  SEATS: 20,
  /** Persistent corner HUD chrome: scoreboard, trump indicator, match stake.
   *  Above seat avatars (so a seat that drifts into a corner on small screens
   *  can't occlude it — the score panel vs. the north partner avatar on phones),
   *  but below flying cards and every dialog. */
  HUD: 25,
  /** Flying cards — throws + the trick-collect sweep. Above seats so a card is
   *  never hidden by an avatar, but below every dialog. */
  CARD_FLIGHT: 30,
  /** Avatar-anchored banners: emote bubbles, surrender-opponent banner. */
  SEAT_BANNER: 40,
  /** Full-table animations: deal, reshuffle. */
  TABLE_ANIM: 50,

  // ── Dialog band — above the table ────────────────────────────────────
  /** Informational reveals (trump-taken, declarations, belote/rebelote). */
  REVEAL: 60,
  /** Action-prompt backdrop dim (trump / declaration / belot prompts). */
  PROMPT_DIM: 70,
  /** Active bidder's hand, lifted above the dim during the TRUMP prompt only. */
  BIDDER_HAND: 72,
  /** Action-prompt panels + the end-of-hand score reveal + the match result. */
  PROMPT: 74,
  /** Player-opened utilities — settings, rules, emote picker — above prompts. */
  UTIL: 80,
  /** Surrender prompts (request-confirm + partner-accept) — above other prompts. */
  SURRENDER: 90,
  /** System blockers: pause, reconnect countdown, match-abandoned. */
  BLOCKER: 100,
  /** Capot celebration — a brief, self-closing flourish above everything game. */
  CAPOT: 110,
  /** Always-reachable chrome: chat dock + mobile HUD menu (not hidden by blockers). */
  CHROME_TOP: 120,
  /** Transient system feedback — the error toast. Absolute top. */
  TOAST: 130,
} as const;

export type ZLayer = (typeof Z)[keyof typeof Z];
