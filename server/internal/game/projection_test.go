package game_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/emilijan/beljot/server/internal/game"
	"github.com/emilijan/beljot/server/internal/game/testfixtures"
)

// projectionDecls returns one meld per team built from cards the NewGameFirstTrick fixture deals
// to those seats.
func projectionDecls() []game.Declaration {
	return []game.Declaration{
		{
			Type: game.DeclarationSequence,
			Cards: []game.Card{
				{Rank: game.Rank7, Suit: game.SuitSpades},
				{Rank: game.Rank8, Suit: game.SuitSpades},
				{Rank: game.Rank9, Suit: game.SuitSpades},
			},
			PlayerSeat: 0,
			Value:      20,
		},
		{
			Type: game.DeclarationSequence,
			Cards: []game.Card{
				{Rank: game.RankJack, Suit: game.SuitDiamonds},
				{Rank: game.RankQueen, Suit: game.SuitDiamonds},
				{Rank: game.RankKing, Suit: game.SuitDiamonds},
				{Rank: game.RankAce, Suit: game.SuitDiamonds},
			},
			PlayerSeat: 1,
			Value:      50,
		},
	}
}

// TestProjectForSeat_CardPrivacy checks every seat's marshalled projection contains only cards
// that seat may see, in every phase shape with hidden information.
func TestProjectForSeat_CardPrivacy(t *testing.T) {
	scenarios := []struct {
		name  string
		build func() *game.GameState
	}{
		{
			name:  "bitola bidding with the 11-card deck held back",
			build: func() *game.GameState { return testfixtures.NewGameJustDealt() },
		},
		{
			name:  "croatian dealer on the clock with face-down pairs hidden",
			build: func() *game.GameState { return testfixtures.NewGameCroatianMidBidding(3) },
		},
		{
			name:  "croatian post-pick with merged 8-card hands",
			build: buildCroatianPostPick,
		},
		{
			name: "trick 1 with unresolved declarations",
			build: func() *game.GameState {
				return testfixtures.NewGameWithDeclarations(projectionDecls())
			},
		},
		{
			name: "pending belote on seat 2",
			build: func() *game.GameState {
				gs := testfixtures.NewGameFirstTrick(game.SuitHearts)
				pending := 2
				gs.PendingBelotSeat = &pending
				return gs
			},
		},
		{
			name: "post-resolution declarations pass through",
			build: func() *game.GameState {
				gs := testfixtures.NewGameWithDeclarations(projectionDecls())
				gs.DeclarationsResolved = true
				return gs
			},
		},
	}

	for _, sc := range scenarios {
		t.Run(sc.name, func(t *testing.T) {
			for seat := 0; seat < 4; seat++ {
				t.Run(seatName(seat), func(t *testing.T) {
					source := sc.build()
					projected := game.ProjectForSeat(source, seat)

					assert.Equal(t, sc.build(), source,
						"ProjectForSeat must not mutate the source state")

					data, err := json.Marshal(projected)
					require.NoError(t, err)
					var raw map[string]any
					require.NoError(t, json.Unmarshal(data, &raw))

					assert.NotContains(t, raw, "deck")

					allowed := map[string]bool{}
					for _, c := range source.Players[seat].Hand {
						allowed[c.String()] = true
					}
					if source.TrumpCandidate != nil {
						allowed[source.TrumpCandidate.String()] = true
					}
					for _, tc := range source.CurrentTrick {
						allowed[tc.Card.String()] = true
					}
					for _, p := range projected.Players {
						for _, d := range p.Declarations {
							for _, c := range d.Cards {
								allowed[c.String()] = true
							}
						}
					}

					for _, id := range cardIDsInPayload(t, raw) {
						assert.True(t, allowed[id],
							"seat %d's frame carries card %s it is not allowed to see", seat, id)
					}

					assert.Equal(t, source.Players[seat].Hand, projected.Players[seat].Hand,
						"own hand must survive projection untouched")
					assert.Equal(t, source.Players[seat].Declarations, projected.Players[seat].Declarations,
						"own declarations must survive projection untouched")

					for i := range source.Players {
						assert.Equal(t, len(source.Players[i].Hand), projected.Players[i].HandCount,
							"seat %d's handCount must be its real hand length", i)
					}

					players, ok := raw["players"].([]any)
					require.True(t, ok)
					require.Len(t, players, 4)
					for i, p := range players {
						seatObj, ok := p.(map[string]any)
						require.True(t, ok)
						if i == seat {
							continue
						}
						hand, ok := seatObj["hand"].([]any)
						require.True(t, ok, "seat %d's masked hand must serialize as [], not null", i)
						assert.Empty(t, hand, "seat %d's hand must be empty in seat %d's frame", i, seat)
						if !source.DeclarationsResolved {
							decls, ok := seatObj["declarations"].([]any)
							require.True(t, ok, "seat %d's masked declarations must serialize as [], not null", i)
							assert.Empty(t, decls,
								"seat %d's unresolved declarations must be empty in seat %d's frame", i, seat)
						}
					}

					if source.DeclarationsResolved {
						for i := range source.Players {
							assert.Equal(t, source.Players[i].Declarations, projected.Players[i].Declarations,
								"resolved declarations are public and must pass through for seat %d", i)
						}
					}

					if source.PendingBelotSeat != nil {
						if *source.PendingBelotSeat == seat {
							require.NotNil(t, projected.PendingBelotSeat,
								"the pending seat must keep its own belote prompt")
							assert.Equal(t, seat, *projected.PendingBelotSeat)
						} else {
							assert.Nil(t, projected.PendingBelotSeat,
								"pendingBelotSeat must be null for every other seat")
						}
					}
				})
			}
		})
	}
}

func seatName(seat int) string {
	return [4]string{"seat 0", "seat 1", "seat 2", "seat 3"}[seat]
}

// buildCroatianPostPick drives the forced-dealer fixture through the real pick, panicking on error
// because scenario builders take no *testing.T.
func buildCroatianPostPick() *game.GameState {
	gs := testfixtures.NewGameCroatianMidBidding(3)
	suit := game.SuitSpades
	picked, err := game.ApplyAction(gs, game.Action{
		Type:       game.ActionPickTrump,
		PlayerSeat: gs.ActivePlayerSeat,
		Suit:       &suit,
	})
	if err != nil {
		panic("projection fixture: Croatian forced pick rejected: " + err.Error())
	}
	return picked
}

// TestProjectForSeat_PostPickMergedHand checks each seat's own frame carries all 8 merged cards
// once a Croatian pick resolves bidding.
func TestProjectForSeat_PostPickMergedHand(t *testing.T) {
	pre := testfixtures.NewGameCroatianMidBidding(3)
	post := buildCroatianPostPick()

	for seat := 0; seat < 4; seat++ {
		projected := game.ProjectForSeat(post, seat)

		require.Len(t, projected.Players[seat].Hand, 8,
			"seat %d's own frame must carry the full merged hand", seat)
		assert.Zero(t, projected.Players[seat].FaceDownCount, "seat %d", seat)

		for _, c := range pre.Players[seat].FaceDownCards {
			assert.Contains(t, projected.Players[seat].Hand, c,
				"seat %d's formerly hidden card %s must surface in its own frame", seat, c)
		}

		for other := 0; other < 4; other++ {
			if other == seat {
				continue
			}
			for _, c := range pre.Players[other].FaceDownCards {
				assert.NotContains(t, projected.Players[seat].Hand, c,
					"seat %d's frame leaked seat %d's card %s", seat, other, c)
			}
		}
	}
}
