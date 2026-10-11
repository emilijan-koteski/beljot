package match

import (
	"log/slog"
	"time"

	"github.com/emilijan/beljot/server/internal/game"
	"github.com/emilijan/beljot/server/internal/ws"
)

// spAwardMsg is a prepared per-human event:season_points_awarded broadcast, sent
// after event:honor_updated and before event:match_state.
type spAwardMsg struct {
	userID uint
	msg    []byte
}

// spSeatPresent reports whether a human seat was at the table when the match ended,
// which gates games_completed only. It fails open: connected=true does not prove presence.
func spSeatPresent(connected [4]bool, seat, abandonedSeat int) bool {
	if abandonedSeat < 0 {
		return true
	}
	return seat != abandonedSeat && connected[seat]
}

// capotTeams reports which teams made at least one Capot in the given hand results.
// The caller must pass a copy taken under the session lock.
func capotTeams(hands []HandResult) [2]bool {
	var teams [2]bool
	for _, hr := range hands {
		if hr.CapotTeam != nil && (*hr.CapotTeam == game.TeamA || *hr.CapotTeam == game.TeamB) {
			teams[*hr.CapotTeam] = true
		}
	}
	return teams
}

// spMatchFacts is what a finalizer already resolved about how the match ended.
type spMatchFacts struct {
	playerIDs     [4]uint
	botSeats      [4]bool
	connected     [4]bool
	winnerTeam    int
	teamScores    [2]int
	matchMode     string
	surrender     bool
	instantWin    bool
	hands         []HandResult
	abandonedSeat int
}

// buildSPOutcome turns the finalizer's facts into the outcome the season service scores.
// winnerTeam is taken as resolved, never re-derived from scores.
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

// awardSeasonPoints applies Season Points using the finalizer's `now` and prepares the
// per-human event:season_points_awarded messages. Failures are logged and skip the events.
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
			continue
		}
		payload := ws.SeasonPointsAwardedPayload{
			SPChange:     snap.SPChange,
			NewSeasonSP:  snap.SP,
			RankTier:     snap.RankTier,
			RankDivision: snap.RankDivision,
			RankChange:   snap.RankChange,
			Reason:       snap.Reason,
			SeasonName:   snap.SeasonName,
		}
		msgs = append(msgs, spAwardMsg{userID: s.UserID, msg: buildMessage(ws.EventSeasonPointsAwarded, payload)})
	}
	return msgs
}
