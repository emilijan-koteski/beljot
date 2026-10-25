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

var updateGoldens = flag.Bool("update-goldens", os.Getenv("UPDATE_GOLDENS") == "1",
	"regenerate JSON goldens under testdata/events/")

const goldensDir = "testdata/events"

// TestEventsJSONContract is the drift gate between events.go and the client's wsEvents.ts.
// Each payload is compared byte-for-byte against a golden that the client also parses through Zod.
func TestEventsJSONContract(t *testing.T) {
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
			MustPickTrump:         false,
			DeclarationsEnabled:   true,
			StopAtTarget:          false,
			DeclarationsTransfer:  true,
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

	declarationsResolvedSample := ws.DeclarationsResolvedPayload{
		WinnerTeam: ptrInt(0),
		Contested:  true,
		Declarations: []ws.DeclarationPayload{
			{
				PlayerSeat: 0,
				Type:       "sequence",
				Value:      50,
				Cards:      []string{"JS", "QS", "KS", "AS"},
			},
			{
				PlayerSeat: 0,
				Type:       "four_of_a_kind",
				Value:      200,
				Cards:      []string{"JS", "JH", "JD", "JC"},
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
			sample:     game.ProjectForSeat(gameStateSample(), 0),
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
			name:       "Phases",
			sample:     game.AllPhases(),
			goldenFile: "phases.json",
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
			name: "AutoActionPayload_PickTrump",
			sample: ws.AutoActionPayload{
				PlayerSeat: 3,
				Type:       ws.AutoActionPickTrump,
			},
			goldenFile: "auto_action_pick_trump.json",
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
			name: "HonorUpdatedPayload",
			sample: ws.HonorUpdatedPayload{
				HonorScore:          83,
				HonorTier:           "fair",
				HonorCompletedTotal: 20,
				HonorAbandonedTotal: 1,
				IsNewPlayer:         false,
			},
			goldenFile: "honor_updated.json",
		},
		{
			name: "SeasonPointsAwardedPayload",
			sample: ws.SeasonPointsAwardedPayload{
				SPChange:     -20,
				NewSeasonSP:  590,
				RankTier:     "silver",
				RankDivision: ptrInt(3),
				RankChange:   ws.RankChangeDemoted,
				Reason:       ws.SPReasonNormal,
				SeasonName:   "2026 Q4",
			},
			goldenFile: "season_points_awarded.json",
		},
		{
			name: "SeasonPointsAwardedPayload (master, partner abandoned)",
			sample: ws.SeasonPointsAwardedPayload{
				SPChange:     -12,
				NewSeasonSP:  1288,
				RankTier:     "master",
				RankDivision: nil,
				RankChange:   ws.RankChangeNone,
				Reason:       ws.SPReasonPartnerAbandoned,
				SeasonName:   "2026 Q4",
			},
			goldenFile: "season_points_awarded_partner_abandoned.json",
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
					t.Fatalf("missing golden %s — rerun with UPDATE_GOLDENS=1 to regenerate", goldenPath)
				}
				t.Fatalf("failed to read golden %s: %v", goldenPath, err)
			}

			if !bytes.Equal(expected, actual) {
				t.Errorf("golden drift in %s\n--- expected ---\n%s\n--- actual ---\n%s\nrerun with UPDATE_GOLDENS=1 if intentional",
					goldenPath, string(expected), string(actual))
			} else {
				assert.True(t, true)
			}
		})
	}
}

func ptrInt(v int) *int              { return &v }
func ptrSuit(v game.Suit) *game.Suit { return &v }
func ptrString(v string) *string     { return &v }

// sampleFourPlayers builds a deterministic, team-correct 4-player roster with populated hands.
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
			AvatarURL:    ptrString("https://assets.example.com/avatars/0f8b6c1e-2a4d-4e6f-9a1b-3c5d7e9f1a2b/128.webp"),
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
