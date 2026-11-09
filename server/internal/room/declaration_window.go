package room

import (
	"github.com/emilijan/beljot/server/internal/apperr"
	"github.com/emilijan/beljot/server/internal/game"
)

// resolveDeclarationLastTrick validates the requested last declaration trick and keeps it only for a variant whose
// declarations run in a turn window with declarations on. Every other room stores the default.
func resolveDeclarationLastTrick(requested *int, variant string, declarationsEnabled bool) (int, error) {
	if requested == nil {
		return game.DefaultDeclarationLastTrick, nil
	}
	if *requested < game.MinDeclarationLastTrick || *requested > game.MaxDeclarationLastTrick {
		return 0, apperr.ErrInvalidDeclarationLastTrick
	}
	usesWindow := declarationsEnabled && game.RulesFor(game.Variant(variant)).DeclarationTiming == game.DeclarationTimingTurnWindow
	if !usesWindow {
		return game.DefaultDeclarationLastTrick, nil
	}
	return *requested, nil
}
