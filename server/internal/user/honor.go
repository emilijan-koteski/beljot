package user

import (
	"math"
	"time"
)

// Honor score (Story 9.7). Honor answers one question: "does this player finish
// the matches they start, RIGHT NOW?" It is a server-authoritative signal (NFR8)
// and THIS FILE IS ITS SINGLE SOURCE OF TRUTH.
//
// The client carries one documented mirror at client/src/shared/lib/honor.ts,
// under the same manual-sync convention as level.go / xpLevel.ts and
// events.go / wsEvents.ts. That mirror is DISPLAY ONLY — it buckets a
// server-supplied score into a tier for colouring and never makes a gating
// decision. If you change the formula or the tier bands here, change it there
// in the same commit.
//
// Two buckets only — completed and abandoned (Story 9.7 D1). "Rage quit" and
// "disconnect" are the same event on this server (a socket close), and per-move
// timer expiry auto-plays a card instead of abandoning anyone, so the epic's
// rage_quits / timeout_abandons buckets have no producer and are not modelled.
//
// Every function here is PURE: no DB, no clock reads (time is always a
// parameter), no side effects. That is what makes the honor math table-testable
// and what lets the same arithmetic run in the profile read path, on the auth
// envelope, inside event:honor_updated, and in Story 9.8's join gate without
// any of them disagreeing.

// Tunable constants. These are placeholders per
// sprint-change-proposal-2026-06-18.md#Section 4 ("honor weights stay as
// placeholders, tuned during each story's planning"), so they are named consts:
// a retuning pass must never have to touch logic.
const (
	// honorHalfLifeDays is how many days it takes for one match's contribution
	// to decay to half its weight. 90 days: an abandonment is half-forgiven in
	// three months and ~94% gone in a year.
	honorHalfLifeDays = 90.0

	// honorAbandonPenalty is how many completed matches one abandonment
	// offsets. At 4.0, a single abandon in twenty drops Trusted to Fair.
	honorAbandonPenalty = 4.0

	// honorPriorCompleted is the Bayesian pseudo-count of completed matches
	// (added to BOTH numerator and denominator). Beta(4,1) smoothing — the
	// standard remedy for "1-of-1 reads as 100%".
	honorPriorCompleted = 4.0

	// honorPriorAbandoned is the Bayesian pseudo-count of abandoned matches
	// (denominator only). Together with honorPriorCompleted it puts a player
	// with no history at 100*4/5 = 80.
	honorPriorAbandoned = 1.0

	// honorNewPlayerMinMatches is the raw FINISHED-OR-ABANDONED match count below
	// which the client suppresses the score and tier and shows a "New Player"
	// chip. Compared against the RAW lifetime totals, never the decayed weights.
	honorNewPlayerMinMatches = 5

	// honorTrendWindow is how many recent matches the trend comparison spans.
	honorTrendWindow = 20

	// honorTrendThreshold is the minimum |delta| between the recent window's
	// score and the PRECEDING window's score before the trend renders as up/down
	// rather than flat. Both sides are equal-size windows — see
	// HonorTrendWindowed for why comparing against the lifetime score was wrong.
	honorTrendThreshold = 2

	// honorEventWeight is what one just-finished match contributes to its
	// bucket. It is 1.0 by definition — decay is applied to the STORED weight
	// before this is added, so a brand-new event is always at full strength.
	honorEventWeight = 1.0

	// honorMaxClockSkew is how far ahead of `now` a stored honor_decayed_at may
	// be before ApplyHonorEvents stops trusting it and resets the reference to
	// now. A stamp in the future is normal at small magnitudes (clock skew
	// between app instances, an NTP step, or the 000017 backfill's Postgres
	// NOW() running marginally ahead of the app host) and must be clamped
	// forward, never rolled back — rolling it back double-decays the interval.
	// But clamping forward WITHOUT a ceiling means DecayFactor returns 1.0 for
	// as long as the stamp stays ahead, so a wildly wrong stamp would freeze
	// decay, and therefore freeze forgiveness, indefinitely and silently.
	// 24h is far beyond any plausible skew and far below any timescale the
	// 90-day half-life cares about.
	honorMaxClockSkew = 24 * time.Hour
)

// Tier tokens. The server returns these STABLE MACHINE TOKENS; the client maps
// token -> i18n label + colour. A display string must never cross the wire.
const (
	HonorTierExemplary   = "exemplary"   // 95-100
	HonorTierTrusted     = "trusted"     // 85-94
	HonorTierFair        = "fair"        // 70-84
	HonorTierUnreliable  = "unreliable"  // 50-69
	HonorTierProblematic = "problematic" // 0-49
)

// Trend direction tokens, same machine-token rule as the tiers.
const (
	HonorTrendUp   = "up"
	HonorTrendFlat = "flat"
	HonorTrendDown = "down"
)

// HonorEvent is one finished match's honor contribution for one player: the
// only thing that varies is which of the two buckets it lands in. A struct
// rather than a bare bool so the call sites read as data, not as a mystery
// boolean, and so a future third bucket (see Story 9.7 D1 — none exists today)
// is an additive change.
type HonorEvent struct {
	// Abandoned is true for the single seat whose reconnect window expired, and
	// false for every seat that saw the match through (including the three
	// innocent players in an abandoned match).
	Abandoned bool
}

// HonorSnapshot is one player's honor state immediately after a write. It is
// what event:honor_updated and the profile DTO render, so it carries the
// AUTHORITATIVE recomputed score rather than the lagging honor_score column.
type HonorSnapshot struct {
	Score          int
	Tier           string
	CompletedTotal int64
	AbandonedTotal int64
	IsNewPlayer    bool
	// Level is the player's lifetime level (Story 9.5 curve over total_xp),
	// carried here so the room roster can show level beside honor from the ONE
	// user read HonorForUsers already does. Populated ONLY by HonorForUsers —
	// NewHonorSnapshot has no user row and leaves it zero, so never render it
	// from a snapshot built anywhere else.
	Level int
}

// NewHonorSnapshot builds the renderable state from a user's stored honor
// columns, recomputing the score at `now`. This is the ONE place read paths
// (profile, auth envelope, match-end event) should go through, so none of them
// can accidentally reach for the stale honor_score column instead.
func NewHonorSnapshot(completedWeight, abandonedWeight float64, decayedAt *time.Time, completedTotal, abandonedTotal int64, now time.Time) HonorSnapshot {
	score := HonorScore(completedWeight, abandonedWeight, decayedAt, now)
	return HonorSnapshot{
		Score:          score,
		Tier:           HonorTier(score),
		CompletedTotal: completedTotal,
		AbandonedTotal: abandonedTotal,
		IsNewPlayer:    IsNewPlayer(completedTotal, abandonedTotal),
	}
}

// DecayFactor returns the multiplier that ages the stored honor weights forward
// from decayedAt to now: 0.5 ^ (elapsedDays / honorHalfLifeDays).
//
// A nil decayedAt means "never decayed" (the column is NULL) and returns exactly
// 1.0. A now that precedes decayedAt — clock skew, or a caller passing a
// deliberately fixed test clock — also returns 1.0 rather than a factor above 1,
// so weights can never be inflated by going backwards in time.
//
// This is the whole reason a running weight is exact rather than approximate:
// because every stored term decays by this same factor,
//
//	Σ 0.5^((now − tᵢ)/H) = 0.5^((now − last)/H) · Σ 0.5^((last − tᵢ)/H)
//
// so "decay forward, then add the new event" is algebraically identical to
// summing every match's weight from scratch (Story 9.7 D3).
func DecayFactor(decayedAt *time.Time, now time.Time) float64 {
	if decayedAt == nil {
		return 1.0
	}
	elapsed := now.Sub(*decayedAt)
	if elapsed <= 0 {
		return 1.0
	}
	days := elapsed.Hours() / 24.0
	return math.Pow(0.5, days/honorHalfLifeDays)
}

// HonorScore is the AUTHORITATIVE honor value: it recomputes from the stored
// weights on every read, so it is never stale. The users.honor_score column is a
// denormalized snapshot for SQL filtering only — never render it, never gate on
// it (see the migration 000017 header).
//
//	f     = DecayFactor(decayedAt, now)
//	C     = completedWeight × f
//	A     = abandonedWeight × f
//	honor = round( 100 × (C + 4) / (C + 4 + 4·A + 1) )   clamped to [0,100]
//
// Note the decay direction: an INACTIVE player's C and A both shrink, so their
// honor drifts back toward the 100*4/5 = 80 prior. That is intended — honor is a
// statement about current reliability, not a lifetime trophy.
//
// Negative weights are impossible (the DB CHECKs forbid them) but are clamped to
// zero anyway so a corrupt row degrades to the prior instead of producing
// nonsense. Rounding is math.Round (half away from zero), matching the Postgres
// ROUND(numeric) used by the migration backfill so both paths agree exactly.
func HonorScore(completedWeight, abandonedWeight float64, decayedAt *time.Time, now time.Time) int {
	f := DecayFactor(decayedAt, now)

	c := completedWeight * f
	a := abandonedWeight * f
	// Non-finite inputs degrade to the prior, never to a tier. +Inf is reachable
	// only by a manual/operator write (Postgres numeric accepts 'Infinity' and the
	// CHECK (>= 0) permits it), but left unguarded it produced Inf/Inf = NaN ->
	// math.Round(NaN) = NaN -> int(NaN) is implementation-defined (a large
	// negative on amd64) -> the score < 0 clamp turned it into 0, i.e. the WORST
	// tier in danger red. That was the opposite of how the tested NaN case
	// behaves. Both now land on the 80 prior. (Review pass 2.)
	if c < 0 || math.IsNaN(c) || math.IsInf(c, 0) {
		c = 0
	}
	if a < 0 || math.IsNaN(a) || math.IsInf(a, 0) {
		a = 0
	}

	numerator := 100 * (c + honorPriorCompleted)
	denominator := c + honorPriorCompleted + honorAbandonPenalty*a + honorPriorAbandoned
	// denominator is >= honorPriorCompleted + honorPriorAbandoned = 5 for any
	// non-negative c/a, so it can never be zero.
	score := int(math.Round(numerator / denominator))

	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}
	return score
}

// HonorScoreForCounts computes honor from raw undecayed match counts. Used for
// the recent-window trend, where the 20-match window IS the recency mechanism so
// no additional decay applies (Story 9.7 "Trend").
func HonorScoreForCounts(completed, abandoned int, now time.Time) int {
	return HonorScore(float64(completed), float64(abandoned), nil, now)
}

// HonorTier buckets a score into one of the five stable tokens. Scores outside
// [0,100] cannot occur (HonorScore clamps) but are bucketed defensively at the
// nearest end rather than returning an empty string, which would render as a
// missing i18n key.
func HonorTier(score int) string {
	switch {
	case score >= 95:
		return HonorTierExemplary
	case score >= 85:
		return HonorTierTrusted
	case score >= 70:
		return HonorTierFair
	case score >= 50:
		return HonorTierUnreliable
	default:
		return HonorTierProblematic
	}
}

// IsNewPlayer reports whether the client should suppress the numeric score and
// tier in favour of a "New Player" chip.
//
// It takes the RAW lifetime totals, never the decayed weights: a returning
// veteran's weights have decayed toward zero, and labelling them "New Player"
// would be both wrong and insulting.
//
// The floor counts EXPERIENCE (completed + abandoned), not successes. Flooring on
// completions alone — which is what AC2/AC6 literally specify and what shipped
// for review — let the worst possible actor hide behind the newcomer chip
// forever: 0 completed and 20 abandoned is a real score of 5 ("problematic"),
// yet it suppressed identically to a genuine first-timer, and Story 9.8's gate
// reads isNewPlayer off the same envelope. Overridden by PO decision 2026-07-29
// (code review), in the same spirit as D1-D3.
//
// Suppression is PRESENTATION ONLY. The server still returns the real score and
// tier when this is true, because Story 9.8's join gate needs them.
func IsNewPlayer(completedTotal, abandonedTotal int64) bool {
	return completedTotal+abandonedTotal < honorNewPlayerMinMatches
}

// HonorTrendWindow exposes how many recent matches the trend query should read,
// so the caller and the math cannot disagree about the window size.
func HonorTrendWindow() int {
	return honorTrendWindow
}

// HonorTrend compares two honor scores and returns the signed delta plus a
// stable direction token. A small delta reads as flat so ordinary noise does not
// render as a trend arrow.
//
// Both arguments MUST come from samples of the same size — see
// HonorTrendWindowed, which is the only caller that should exist.
func HonorTrend(recentScore, baselineScore int) (delta int, direction string) {
	delta = recentScore - baselineScore
	switch {
	case delta >= honorTrendThreshold:
		return delta, HonorTrendUp
	case delta <= -honorTrendThreshold:
		return delta, HonorTrendDown
	default:
		return delta, HonorTrendFlat
	}
}

// HonorTrendWindowed compares the newest window of matches against the window
// immediately before it, and is the ONLY correct way to compute the trend.
//
// It renders flat unless the two windows hold the SAME number of matches. That
// guard is the whole point of the two-window shape (code review 2026-07-29): the
// Beta(4,1) prior adds 4 pseudo-completions, and their drag depends on sample
// size. Comparing a 20-match window (a flawless one caps at 100*24/25 = 96)
// against a lifetime score computed from an unbounded decayed weight (98-100 for
// an active player) produced a permanent "Slipping -3" for players who had never
// abandoned a single match, and "Improving +12" for a player who had been idle
// for a year. Equal windows carry an identical prior drag, so the difference is
// behaviour rather than arithmetic.
//
// Consequence worth knowing: a player needs 2*honorTrendWindow finished matches
// before any trend renders. Below that the honest answer is "not enough
// evidence", and flat is how that is shown.
func HonorTrendWindowed(recentCompleted, recentAbandoned, priorCompleted, priorAbandoned int, now time.Time) (delta int, direction string) {
	recentTotal := recentCompleted + recentAbandoned
	priorTotal := priorCompleted + priorAbandoned
	if recentTotal == 0 || recentTotal != priorTotal {
		return 0, HonorTrendFlat
	}
	return HonorTrend(
		HonorScoreForCounts(recentCompleted, recentAbandoned, now),
		HonorScoreForCounts(priorCompleted, priorAbandoned, now),
	)
}
