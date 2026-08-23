package game_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/emilijan/beljot/server/internal/game"
	"github.com/emilijan/beljot/server/internal/game/testfixtures"
)

// projectionDecls is the meld layout the declaration scenarios share: one meld
// per team, drawn from cards the NewGameFirstTrick fixture really deals to
// those hands, so "a foreign meld card" and "a foreign hand card" are the same
// physical cards — exactly the property the privacy rule protects.
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

// TestProjectForSeat_CardPrivacy is the missing "no test anywhere asserts card
// privacy" (Story 12.10). For every seat, in every phase shape that carries
// hidden information, a marshalled projection may contain ONLY cards that seat
// is allowed to see: its own hand, the public trump candidate, the current
// trick, and the cards of declarations the projection itself still carries
// (its own, plus — once resolved — the public reveal). Everything else (other
// hands, the undealt deck, face-down pairs, other seats' unresolved melds)
// must be absent, structurally, wherever a future field might put a card.
func TestProjectForSeat_CardPrivacy(t *testing.T) {
	scenarios := []struct {
		name  string
		build func() *game.GameState
	}{
		{
			// Bitola bidding: 5-card hands, a public candidate, and the 11
			// undealt stage-2 cards that D96 found on the wire.
			name:  "bitola bidding with the 11-card deck held back",
			build: func() *game.GameState { return testfixtures.NewGameJustDealt() },
		},
		{
			// Croatian forced-dealer state: 6 open cards plus 2 face-down per
			// seat, hidden from everyone — the owner included — until trump
			// resolves. The dealer-on-clock state is the deepest reachable
			// point of the single round.
			name:  "croatian dealer on the clock with face-down pairs hidden",
			build: func() *game.GameState { return testfixtures.NewGameCroatianMidBidding(3) },
		},
		{
			// The moment after: a pick resolved Croatian bidding, so every
			// seat's pair merged into its hand. Deterministic — the engine's
			// pick path shuffles nothing — and driven through ApplyAction so
			// the scenario can never drift from the real post-pick shape. The
			// generic body then proves each seat's frame carries its own 8
			// merged cards intact and nobody else's.
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
			// After resolution the engine has already nil'd the losers' melds;
			// what remains is the public reveal and passes through for every
			// recipient.
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

					// The projection must never mutate its input: slices share
					// backing arrays, so an in-place mask would reach back into
					// the live state. The fixtures are deterministic, so a
					// freshly built twin IS the expected pre-projection state.
					assert.Equal(t, sc.build(), source,
						"ProjectForSeat must not mutate the source state")

					data, err := json.Marshal(projected)
					require.NoError(t, err)
					var raw map[string]any
					require.NoError(t, json.Unmarshal(data, &raw))

					// The undealt deck never reaches the wire at all — no key,
					// not an empty array (json:"-" on GameState.Deck).
					assert.NotContains(t, raw, "deck")

					// Whitelist of everything this seat may see.
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

					// The whitelist scan would pass vacuously on an over-masked
					// frame, so pin the recipient's OWN data intact.
					assert.Equal(t, source.Players[seat].Hand, projected.Players[seat].Hand,
						"own hand must survive projection untouched")
					assert.Equal(t, source.Players[seat].Declarations, projected.Players[seat].Declarations,
						"own declarations must survive projection untouched")

					// handCount carries the REAL hand length for all four seats
					// — the count is public, the cards are not.
					for i := range source.Players {
						assert.Equal(t, len(source.Players[i].Hand), projected.Players[i].HandCount,
							"seat %d's handCount must be its real hand length", i)
					}

					// Masked hands and unresolved foreign melds serialize as
					// empty arrays, never null — the client's strict schema
					// parses arrays.
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

					// Once resolved, the reveal is public: every recipient sees
					// the surviving melds verbatim.
					if source.DeclarationsResolved {
						for i := range source.Players {
							assert.Equal(t, source.Players[i].Declarations, projected.Players[i].Declarations,
								"resolved declarations are public and must pass through for seat %d", i)
						}
					}

					// Pending Belote: the holder sees their own prompt, nobody
					// else learns the seat holds the second trump royal.
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

// buildCroatianPostPick drives the forced-dealer fixture through the real pick
// so the post-pick projection scenario tests the engine's own output. Panics
// instead of taking a *testing.T because the scenario table's build functions
// are plain factories.
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

// TestProjectForSeat_PostPickMergedHand pins the one reveal moment explicitly:
// once a Croatian pick resolves bidding, each seat's OWN frame carries all 8
// merged cards — the formerly hidden pair included — while the pair still
// never leaks into any other seat's frame (the scenario table's whitelist scan
// covers that half structurally).
func TestProjectForSeat_PostPickMergedHand(t *testing.T) {
	pre := testfixtures.NewGameCroatianMidBidding(3)
	post := buildCroatianPostPick()

	for seat := 0; seat < 4; seat++ {
		projected := game.ProjectForSeat(post, seat)

		require.Len(t, projected.Players[seat].Hand, 8,
			"seat %d's own frame must carry the full merged hand", seat)
		assert.Zero(t, projected.Players[seat].FaceDownCount, "seat %d", seat)

		// The two formerly hidden cards are now IN the owner's projected hand.
		for _, c := range pre.Players[seat].FaceDownCards {
			assert.Contains(t, projected.Players[seat].Hand, c,
				"seat %d's formerly hidden card %s must surface in its own frame", seat, c)
		}

		// And in nobody else's.
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
