package ws_test

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/emilijan/beljot/server/internal/game"
	"github.com/emilijan/beljot/server/internal/ws"
)

// updateGoldens regenerates the JSON goldens under
// server/internal/ws/testdata/events/. Run with `UPDATE_GOLDENS=1 go test
// ./internal/ws/... -run Contract` to refresh after a payload field/tag change.
var updateGoldens = flag.Bool("update-goldens", os.Getenv("UPDATE_GOLDENS") == "1",
	"regenerate JSON goldens under testdata/events/")

// goldensDir is the on-disk location of the WS event goldens. Mirrored on the
// TS side via a relative import in client/src/shared/types/wsEvents.contract.test.ts.
const goldensDir = "testdata/events"

// TestEventsJSONContract is the drift gate between server/internal/ws/events.go
// and client/src/shared/types/wsEvents.ts. Each entry in the table marshals a
// representative payload struct and compares it byte-for-byte against the
// stored golden. The TS side parses these same goldens through Zod schemas;
// any rename or tag change here updates the goldens (with UPDATE_GOLDENS=1)
// and forces a paired update on the TS side or the parse fails.
func TestEventsJSONContract(t *testing.T) {
	// Helper: build a 4-player GameState with realistic non-zero fields.
	// The struct is large but the golden captures only the JSON-tagged fields.
	gameStateSample := func() *game.GameState {
		return &game.GameState{
			ID:                    42,
			RoomID:                7,
			Variant:               game.VariantBitola,
			MatchMode:             "1001",
			Phase:                 game.PhasePlaying,
			OwnerSeat:             0,
			HandNumber:            3,
			DealerSeat:            2,
			TrumpSuit:             ptrSuit(game.SuitSpades),
			TrumpCallerSeat:       ptrInt(1),
			TrumpCandidate:        nil,
			BiddingRound:          2,
			BiddingPassCount:      0,
			Deck:                  []game.Card{},
			TrickNumber:           4,
			CurrentTrick:          []game.TrickCard{},
			LeadSuit:              nil,
			TrickWinnerSeat:       nil,
			AwaitingDeclaration:   false,
			DeclarationsResolved:  true,
			Players:               sampleFourPlayers(),
			TeamScores:            [2]int{210, 180},
			HandPoints:            [2]int{60, 80},
			DeclarationPoints:     [2]int{20, 0},
			TricksWon:             [2]int{2, 2},
			PendingBelotSeat:      nil,
			BelotAnnounced:        false,
			WinnerTeam:            nil,
			LastHandResult:        nil,
			ActivePlayerSeat:      1,
			TurnExpiresAt:         nil,
			TimerDurationSec:      0,
			PreviousPhase:         "",
			PausedPlayers:         [4]bool{false, false, false, false},
			PauseUsed:             [4]bool{false, false, false, false},
			TurnTimeRemaining:     0,
			SurrenderProposerSeat: nil,
			SurrenderUsed:         [4]bool{false, false, false, false},
			DisconnectedSeat:      -1,
			ReconnectExpiresAt:    nil,
		}
	}

	// handScoredSample mirrors session/manager.go's broadcast — a map[string]any,
	// not a typed struct. Capturing the wire shape verbatim is what the contract
	// test cares about; the TS schema parses against the same keys.
	handScoredSample := map[string]any{
		"teamACardPoints": 82,
		"teamBCardPoints": 80,
		"teamADeclPoints": 20,
		"teamBDeclPoints": 0,
		"lastTrickTeam":   0,
		"lastTrickBonus":  10,
		"capot":           false,
		"capotTeam":       nil,
		"capotBonus":      0,
		"failedContract":  false,
		"contractingTeam": 0,
		"teamAHandTotal":  112,
		"teamBHandTotal":  80,
		"teamAMatchScore": 312,
		"teamBMatchScore": 280,
	}

	// declarationsResolvedSample mirrors the manager.go broadcast — also a
	// map[string]any with a list of decl maps.
	declarationsResolvedSample := map[string]any{
		"winnerTeam": 0,
		"declarations": []map[string]any{
			{
				"playerSeat": 0,
				"type":       "sequence",
				"value":      50,
				"cards":      []string{"JS", "QS", "KS", "AS"},
			},
		},
	}

	cases := []struct {
		name       string
		sample     any
		goldenFile string
	}{
		{
			name:       "EventMatchState",
			sample:     gameStateSample(),
			goldenFile: "event_match_state.json",
		},
		{
			name: "CardPlayedPayload",
			sample: ws.CardPlayedPayload{
				PlayerSeat: 1,
				CardID:     "JH",
				AutoPlayed: false,
			},
			goldenFile: "card_played.json",
		},
		{
			name: "TrickResolvedPayload",
			// Mirrors session/manager.go's broadcast — also a map literal.
			sample: map[string]any{
				"winnerSeat": 2,
				"winnerTeam": 0,
				"cards":      []string{"7H", "JH", "QH", "AH"},
			},
			goldenFile: "trick_resolved.json",
		},
		{
			name:       "EventHandScored",
			sample:     handScoredSample,
			goldenFile: "event_hand_scored.json",
		},
		{
			name: "MatchEndPayload",
			sample: ws.MatchEndPayload{
				WinnerTeam:       0,
				TeamAFinalScore:  1010,
				TeamBFinalScore:  640,
				MatchDurationSec: 1234,
				OutcomeReason:    ws.OutcomeReasonNatural,
			},
			goldenFile: "match_end.json",
		},
		{
			name: "MatchAbandonedPayload",
			sample: ws.MatchAbandonedPayload{
				AbandonedByPlayer: 2,
				TeamAFinalScore:   420,
				TeamBFinalScore:   380,
				MatchDurationSec:  600,
			},
			goldenFile: "match_abandoned.json",
		},
		{
			name: "TrumpSelectedPayload",
			sample: ws.TrumpSelectedPayload{
				PlayerSeat: 1,
				TrumpSuit:  "S",
				CardID:     "7S",
			},
			goldenFile: "trump_selected.json",
		},
		{
			name:       "DeclarationsResolvedPayload",
			sample:     declarationsResolvedSample,
			goldenFile: "declarations_resolved.json",
		},
		{
			name: "PlayerDeclaredPayload",
			sample: ws.PlayerDeclaredPayload{
				PlayerSeat: 2,
			},
			goldenFile: "player_declared.json",
		},
		{
			name: "BelotAnnouncedPayload",
			sample: ws.BelotAnnouncedPayload{
				PlayerSeat: 0,
				Team:       0,
				CardID:     "KS",
			},
			goldenFile: "belot_announced.json",
		},
		{
			name: "MatchPausedPayload",
			sample: ws.MatchPausedPayload{
				PausedBy:      1,
				PausedPlayers: [4]bool{false, true, false, false},
			},
			goldenFile: "match_paused.json",
		},
		{
			name: "MatchResumedPayload",
			sample: ws.MatchResumedPayload{
				ResumedBy:     1,
				OwnerOverride: false,
			},
			goldenFile: "match_resumed.json",
		},
		{
			name: "AutoActionPayload",
			sample: ws.AutoActionPayload{
				PlayerSeat: 1,
				Type:       ws.AutoActionPassTrump,
			},
			goldenFile: "auto_action.json",
		},
		{
			name: "CoinSettlementPayload",
			sample: ws.CoinSettlementPayload{
				CoinDelta:  500,
				NewBalance: 5500,
				Pot:        2000,
			},
			goldenFile: "coin_settlement.json",
		},
		{
			name: "XPAwardedPayload",
			sample: ws.XPAwardedPayload{
				XPEarned:   99,
				NewTotalXP: 1099,
				NewLevel:   4,
				LeveledUp:  true,
			},
			goldenFile: "xp_awarded.json",
		},
		{
			name: "PlayerDisconnectedPayload",
			sample: ws.PlayerDisconnectedPayload{
				PlayerSeat:         3,
				Username:           "dave",
				ReconnectExpiresAt: "2026-05-01T12:34:56Z",
			},
			goldenFile: "player_disconnected.json",
		},
		{
			name: "PlayerReconnectedPayload",
			sample: ws.PlayerReconnectedPayload{
				PlayerSeat: 3,
			},
			goldenFile: "player_reconnected.json",
		},
		{
			name: "SurrenderProposedPayload",
			sample: ws.SurrenderProposedPayload{
				ProposerSeat:     2,
				ProposerTeam:     0,
				ProposerUsername: "carol",
				PartnerSeat:      0,
			},
			goldenFile: "surrender_proposed.json",
		},
		{
			name: "SurrenderDeclinedPayload",
			sample: ws.SurrenderDeclinedPayload{
				ProposerSeat:  2,
				DecliningSeat: 0,
			},
			goldenFile: "surrender_declined.json",
		},
	}

	require.NoError(t, os.MkdirAll(goldensDir, 0o755))

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Indent for human-readable goldens; trailing newline so editors
			// don't muck with the file (POSIX text-file convention).
			actual, err := json.MarshalIndent(tc.sample, "", "  ")
			require.NoError(t, err)
			actual = append(actual, '\n')

			goldenPath := filepath.Join(goldensDir, tc.goldenFile)

			if *updateGoldens {
				require.NoError(t, os.WriteFile(goldenPath, actual, 0o644))
				t.Logf("updated golden: %s", goldenPath)
				return
			}

			expected, err := os.ReadFile(goldenPath)
			if err != nil {
				if os.IsNotExist(err) {
					// AC10 / D118: a missing golden must hard-fail unless the
					// caller explicitly asked for regeneration. The previous
					// bootstrap-on-missing path silently treated a deleted
					// golden as the new truth, defeating the drift gate.
					if !*updateGoldens {
						t.Fatalf("missing golden %s — rerun with UPDATE_GOLDENS=1 to regenerate", goldenPath)
					}
					require.NoError(t, os.WriteFile(goldenPath, actual, 0o644),
						"failed to write golden %s", goldenPath)
					t.Logf("regenerated golden: %s", goldenPath)
					return
				}
				t.Fatalf("failed to read golden %s: %v", goldenPath, err)
			}

			if !bytes.Equal(expected, actual) {
				t.Errorf("golden drift in %s\n--- expected ---\n%s\n--- actual ---\n%s\nrerun with UPDATE_GOLDENS=1 if intentional",
					goldenPath, string(expected), string(actual))
			} else {
				assert.True(t, true) // record the assertion for -v output
			}
		})
	}
}

func ptrInt(v int) *int              { return &v }
func ptrSuit(v game.Suit) *game.Suit { return &v }

// sampleFourPlayers builds a deterministic 4-player roster with hands of
// varying length so the JSON shape covers populated arrays. Values are
// stable and chosen to be team-shape-correct (seats 0/2 = teamA, 1/3 = teamB).
func sampleFourPlayers() [4]game.PlayerState {
	return [4]game.PlayerState{
		{
			Seat:         0,
			UserID:       10,
			Username:     "alice",
			Team:         "teamA",
			Hand:         []game.Card{{Rank: game.Rank7, Suit: game.SuitClubs}},
			Declarations: []game.Declaration{},
			Connected:    true,
			Level:        3,
		},
		{
			Seat:         1,
			UserID:       20,
			Username:     "bob",
			Team:         "teamB",
			Hand:         []game.Card{{Rank: game.Rank8, Suit: game.SuitHearts}},
			Declarations: []game.Declaration{},
			Connected:    true,
			Level:        1,
		},
		{
			Seat:         2,
			UserID:       30,
			Username:     "carol",
			Team:         "teamA",
			Hand:         []game.Card{{Rank: game.Rank9, Suit: game.SuitDiamonds}},
			Declarations: []game.Declaration{},
			Connected:    true,
			Level:        2,
		},
		{
			Seat:         3,
			UserID:       40,
			Username:     "dave",
			Team:         "teamB",
			Hand:         []game.Card{{Rank: game.RankAce, Suit: game.SuitSpades}},
			Declarations: []game.Declaration{},
			Connected:    true,
			Level:        0,
		},
	}
}
