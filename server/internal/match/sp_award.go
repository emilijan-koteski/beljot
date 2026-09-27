package match

import (
	"log/slog"
	"time"

	"github.com/emilijan/beljot/server/internal/game"
	"github.com/emilijan/beljot/server/internal/ws"
)

// Season Points at match end (Story 13.4). This file does NOT compute SP.
//
// A change depends on every seated human's CURRENT season SP (the formula's
// expected result compares the two teams' averages), and only the season
// service can read those totals -- under the same row lock it writes with, so
// two matches sharing a player cannot both score from one stale total. So the
// match side's whole job is to describe the match: buildSPOutcome captures the
// facts the finalizer already resolved (seats, winner, scores, target,
// surrender, instant win, Capot teams, abandoned seat) and awardSeasonPoints
// hands that outcome to the SPAwarder and turns its snapshots into events. The
// formula itself is season.ComputeSPChanges.

// spAwardMsg is a prepared per-human event:season_points_awarded broadcast. Built
// during awardSeasonPoints but SENT by the finalize path AFTER the
// event:honor_updated loop and BEFORE the trailing event:match_state, preserving
// the Story 8.5-1 ordering contract:
//
//	match_end | match_abandoned
//	  -> coin_settlement -> xp_awarded -> honor_updated
//	  -> season_points_awarded -> match_state
//
// Mirrors honorUpdateMsg, xpAwardMsg and coinSettlementMsg.
type spAwardMsg struct {
	userID uint
	msg    []byte
}

// spSeatPresent reports whether a human seat was at the table when the match
// reached its terminal end. It is the games_completed gate (OutcomeSeat.Completed)
// and NOTHING ELSE: since Story 13.4 every human seat is scored by its team's
// result whether present or not, and only the abandoned seat takes the
// abandonment penalty.
//
// It is the SAME RULE computeHonorEvents applies (honor_record.go), read off the
// same `connected` snapshot and deliberately expressed here rather than by
// calling into honor's bucketing — that function returns honor events, not a
// presence answer.
//
// A natural end or an accepted surrender (abandonedSeat == -1) reached a real
// terminal state, so every human seat is present by construction. On the
// abandonment path the expired seat is absent by definition (that is why its
// timer fired, so it is checked independently of its flag), and so is any OTHER
// seat sitting inside its own overlapping reconnect window.
//
// The gate FAILS OPEN: `connected` false reliably means absent, but true does NOT
// reliably mean present (HandleDisconnect only maintains it for drops observed in
// four phases — see the note at reconnect.go).
func spSeatPresent(connected [4]bool, seat, abandonedSeat int) bool {
	if abandonedSeat < 0 {
		return true
	}
	return seat != abandonedSeat && connected[seat]
}

// capotTeams reports which teams made at least one Capot in the match, read off
// the buffered hand results (each carries the Capot team). One flag per team, not
// a count: the bonus is once per match however many Capots a team makes.
//
// The caller must pass a COPY taken under the session lock — never
// session.handResults read unlocked. See the hoisted snapshot in handleMatchEnd
// and the under-lock snapshot in handleSeatReconnectTimeout.
func capotTeams(hands []HandResult) [2]bool {
	var teams [2]bool
	for _, hr := range hands {
		if hr.CapotTeam != nil && (*hr.CapotTeam == game.TeamA || *hr.CapotTeam == game.TeamB) {
			teams[*hr.CapotTeam] = true
		}
	}
	return teams
}

// spMatchFacts is what a finalizer already resolved about how the match ended,
// gathered in one place so both finalizers build the outcome the same way.
type spMatchFacts struct {
	playerIDs  [4]uint
	botSeats   [4]bool
	connected  [4]bool
	winnerTeam int
	teamScores [2]int
	matchMode  string
	surrender  bool
	instantWin bool
	// hands is the finalizer's under-lock COPY of the buffered hand results.
	hands []HandResult
	// abandonedSeat is -1 for a natural end or an accepted surrender.
	abandonedSeat int
}

// buildSPOutcome turns the finalizer's facts into the outcome the season service
// scores. Pure.
//
// winnerTeam is the value the FINALIZER already resolved — *finalState.WinnerTeam
// on the natural path, or 1 - TeamForSeat(abandonedSeat) on the abandonment path
// — never re-derived from scores here: the taker can win a both-cross hand with
// fewer points. The target is the engine's own (game.MatchTarget).
func buildSPOutcome(f spMatchFacts) MatchOutcome {
	o := MatchOutcome{
		WinnerTeam:    f.winnerTeam,
		TeamScores:    f.teamScores,
		Target:        game.MatchTarget(f.matchMode),
		Surrender:     f.surrender,
		InstantWin:    f.instantWin,
		CapotTeams:    capotTeams(f.hands),
		AbandonedSeat: f.abandonedSeat,
	}
	for seat := range o.Seats {
		s := OutcomeSeat{Team: game.TeamForSeat(seat), IsBot: f.botSeats[seat]}
		if !s.IsBot {
			s.UserID = f.playerIDs[seat]
			s.Completed = spSeatPresent(f.connected, seat, f.abandonedSeat)
		}
		o.Seats[seat] = s
	}
	return o
}

// awardSeasonPoints applies Season Points for a finished match and prepares the
// per-human event:season_points_awarded messages. It is a no-op (no mutation, no
// messages) when no SPAwarder is wired or when every seat is a bot.
//
// `now` is THE FINALIZER'S OWN STAMP, threaded through rather than read here.
// That is the whole point of SPAwarder taking a time: the season a match lands in
// must be decided once, by the code that decided the match was over, not by
// whichever layer happens to call the clock last. Contrast recordHonor, which
// reads the clock itself on purpose (honor's decay reference must be the instant
// of the write).
//
// EVERY human seat is scored, absent ones included: games_played counts every
// human seat in the match while games_completed counts only the present ones
// (Story 13.1 D10), and the SP change follows the team's result either way.
//
// Each event carries the APPLIED change (snap.SPChange, new total minus the
// previous one), which differs from the formula's own number at the 0 floor.
//
// Mirrors settleMatch's, awardXP's and recordHonor's best-effort degradation: an
// ApplySeasonPoints failure is logged and the events are skipped, but the caller
// still fires match_end / match_abandoned and match_state so clients are never
// stranded on the table.
func (m *Manager) awardSeasonPoints(roomID uint, outcome MatchOutcome, now time.Time) []spAwardMsg {
	if m.spAwarder == nil {
		return nil
	}
	humans := 0
	for _, s := range outcome.Seats {
		if !s.IsBot && s.UserID != 0 {
			humans++
		}
	}
	if humans == 0 {
		return nil
	}

	snapshots, err := m.spAwarder.ApplySeasonPoints(outcome, now)
	if err != nil {
		slog.Error("session: failed to award season points", "roomID", roomID, "error", err)
		return nil
	}

	var msgs []spAwardMsg
	for _, s := range outcome.Seats {
		if s.IsBot || s.UserID == 0 {
			continue
		}
		snap, ok := snapshots[s.UserID]
		if !ok {
			// Missing from the returned snapshots — skip rather than push a wrong
			// value (the same rule awardXP's newTotals and recordHonor's snapshots
			// lookups apply).
			continue
		}
		payload := ws.SeasonPointsAwardedPayload{
			SPEarned:    snap.SPChange,
			NewSeasonSP: snap.SP,
			RankTier:    snap.RankTier,
			TieredUp:    snap.TieredUp,
			SeasonName:  snap.SeasonName,
		}
		msgs = append(msgs, spAwardMsg{userID: s.UserID, msg: buildMessage(ws.EventSeasonPointsAwarded, payload)})
	}
	return msgs
}
