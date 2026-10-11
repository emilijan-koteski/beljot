package match

import (
	"log/slog"

	"github.com/emilijan/beljot/server/internal/game"
	"github.com/emilijan/beljot/server/internal/ws"
)

// coinSettlementMsg is a prepared per-human event:coin_settlement broadcast, sent
// by the finalize path after event:match_end.
type coinSettlementMsg struct {
	userID uint
	msg    []byte
}

// settleMatch credits winners in one wallet transaction and builds the per-human
// event:coin_settlement messages. Deltas are always returned; wallet failures are logged and skip the events.
func (m *Manager) settleMatch(roomID uint, playerIDs [4]uint, botSeats [4]bool, winningTeam, coinBuyIn int) (deltas [4]int, msgs []coinSettlementMsg) {
	if coinBuyIn <= 0 {
		return deltas, nil
	}

	deltas, credits, pot := computeSettlement(playerIDs, botSeats, winningTeam, coinBuyIn)

	if m.walletSettler == nil {
		return deltas, nil
	}

	if err := m.walletSettler.ApplySettlement(credits); err != nil {
		slog.Error("session: failed to apply coin settlement", "roomID", roomID, "error", err)
		var failedDeltas [4]int
		for seat := 0; seat < 4; seat++ {
			if botSeats[seat] || playerIDs[seat] == 0 {
				continue
			}
			failedDeltas[seat] = -coinBuyIn
		}
		return failedDeltas, nil
	}

	humanIDs := humanUserIDs(playerIDs)
	balances, err := m.walletSettler.GetBalances(humanIDs)
	if err != nil {
		slog.Error("session: failed to read balances after settlement", "roomID", roomID, "error", err)
		return deltas, nil
	}

	for seat := 0; seat < 4; seat++ {
		uid := playerIDs[seat]
		if botSeats[seat] || uid == 0 {
			continue
		}
		nb, ok := balances[uid]
		if !ok {
			continue
		}
		payload := ws.CoinSettlementPayload{CoinDelta: deltas[seat], NewBalance: nb, Pot: pot}
		msgs = append(msgs, coinSettlementMsg{userID: uid, msg: buildMessage(ws.EventCoinSettlement, payload)})
	}
	return deltas, msgs
}

// computeSettlement returns per-seat net coin deltas, winner credits and the pot of
// human stakes, split evenly among human winners with any remainder to the lowest seats.
func computeSettlement(playerIDs [4]uint, botSeats [4]bool, winningTeam int, buyIn int) (deltas [4]int, credits map[uint]int, pot int) {
	credits = make(map[uint]int)
	if buyIn <= 0 {
		return deltas, credits, 0
	}

	numHumans := 0
	winnerSeats := make([]int, 0, 2)
	for seat := 0; seat < 4; seat++ {
		if botSeats[seat] {
			continue
		}
		numHumans++
		deltas[seat] = -buyIn
		if game.TeamForSeat(seat) == winningTeam {
			winnerSeats = append(winnerSeats, seat)
		}
	}

	pot = numHumans * buyIn

	n := len(winnerSeats)
	if n == 0 {
		return deltas, credits, pot
	}

	share := pot / n
	remainder := pot % n
	for i, seat := range winnerSeats {
		amount := share
		if i < remainder {
			amount++
		}
		deltas[seat] = amount - buyIn
		credits[playerIDs[seat]] = amount
	}
	return deltas, credits, pot
}
