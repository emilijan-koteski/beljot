package match

import (
	"log/slog"

	"github.com/emilijan/beljot/server/internal/game"
	"github.com/emilijan/beljot/server/internal/ws"
)

// coinSettlementMsg is a prepared per-human event:coin_settlement broadcast.
// Built during settlement but SENT by the finalize path AFTER event:match_end,
// to preserve the Story 8.5-1 ordering contract (match_end → settlement → state).
type coinSettlementMsg struct {
	userID uint
	msg    []byte
}

// settleMatch applies coin settlement for a finished match (Story 9.2). It is a
// no-op (zero deltas, no messages) when the match has no economy (coinBuyIn 0).
// Otherwise it computes the per-seat deltas + winner credits, applies the
// credits in one wallet transaction, reads each human's resulting balance, and
// builds the per-human event:coin_settlement messages for the caller to send.
//
// Deltas are ALWAYS returned (even when the wallet write/read degrades) so the
// match row records the economic outcome. Wallet failures are logged and the
// settlement events are skipped rather than risk pushing a wrong newBalance to
// a client — mirroring the existing "persist failure must not strand clients"
// best-effort philosophy at the finalize paths.
func (m *Manager) settleMatch(roomID uint, playerIDs [4]uint, botSeats [4]bool, winningTeam, coinBuyIn int) (deltas [4]int, msgs []coinSettlementMsg) {
	if coinBuyIn <= 0 {
		return deltas, nil
	}

	deltas, credits, pot := computeSettlement(playerIDs, botSeats, winningTeam, coinBuyIn)

	if m.walletSettler == nil {
		// No settler wired — record deltas on the match row but skip wallet I/O.
		return deltas, nil
	}

	if err := m.walletSettler.ApplySettlement(credits); err != nil {
		slog.Error("session: failed to apply coin settlement", "roomID", roomID, "error", err)
		// The credit transaction rolled back, so winners were NOT credited — but
		// every human was already debited buyIn at StartMatch. Record the TRUE
		// wallet outcome (all humans −buyIn, winners uncredited) instead of the
		// optimistic computed deltas, so the match row matches wallet reality.
		// No events are sent (no reliable newBalance to push).
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
			// Missing balance — don't risk corrupting the client's value.
			continue
		}
		payload := ws.CoinSettlementPayload{CoinDelta: deltas[seat], NewBalance: nb, Pot: pot}
		msgs = append(msgs, coinSettlementMsg{userID: uid, msg: buildMessage(ws.EventCoinSettlement, payload)})
	}
	return deltas, msgs
}

// computeSettlement is the single source of pot math for Story 9.2. It is pure
// (DB-free, table-tested): given the seat→userID map, which seats are bots, the
// winning team, and the per-human stake captured at StartMatch, it returns the
// net per-seat coin deltas, the winner credit map, and the pot.
//
// Model (Decision A/C in the story):
//
//   - Stakes were already debited from every HUMAN seat at StartMatch, so the
//     pot is implicit: pot = (number of human seats) × buyIn.
//   - Human seats on winningTeam are winners; they split the pot equally.
//     share = pot / numHumanWinners; the remainder (when an odd human count
//     makes the split fractional) is distributed one coin at a time to winners
//     in ascending SEAT order (Decision C: remainder to the lowest seat).
//   - A winner's NET delta = creditedShare − buyIn (they already paid buyIn at
//     start). A loser's net delta = −buyIn (their stake stays in the pot). Bot
//     seats are never charged or paid → delta 0, never in credits.
//   - numHumanWinners == 0 (AC #9): credits is empty and the losers' forfeited
//     stakes are removed from circulation (a house sink). deltas still record
//     each human loser's −buyIn.
//
// deltas is seat-indexed (matches player{N}_coin_delta on the match row and the
// coinDelta in event:coin_settlement). credits is userID→amount for the wallet
// ApplySettlement transaction.
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
		// Every human is charged buyIn at start; the loser branch keeps that as
		// the forfeit. Winners get their share added below.
		deltas[seat] = -buyIn
		if game.TeamForSeat(seat) == winningTeam {
			winnerSeats = append(winnerSeats, seat) // ascending seat order
		}
	}

	pot = numHumans * buyIn

	n := len(winnerSeats)
	if n == 0 {
		// No human on the winning team — the forfeited stakes are a sink.
		return deltas, credits, pot
	}

	share := pot / n
	remainder := pot % n
	for i, seat := range winnerSeats {
		amount := share
		if i < remainder {
			amount++ // remainder coins go to the lowest seats first
		}
		deltas[seat] = amount - buyIn
		credits[playerIDs[seat]] = amount
	}
	return deltas, credits, pot
}
