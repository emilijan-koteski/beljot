package game

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// This file is the drift gate for the two facts the CLIENT hand-mirrors from
// this package:
//
//   - client/src/features/match/lib/variantRules.ts mirrors RulesFor's presets
//   - client/src/features/match/lib/declarations.ts mirrors detectDeclarations
//
// Both mirrors declare themselves mirrors in prose, and until now the only
// enforcement was two independently hand-written test suites: flipping a preset
// in RulesFor, or changing the dedup in detectDeclarations, failed nothing on
// the client. These goldens close that: the Go side owns them, the TS side
// imports them read-only and asserts its mirror reproduces them.
//
// Deliberately NOT a wire change — VariantRules stays `json:"-"` and is never
// serialized to a client at runtime. This is a build-time contract only.
//
// Regenerate with: UPDATE_GOLDENS=1 go test ./internal/game/ -run Contract

const rulesGoldensDir = "testdata/contract"

// variantRuleFacts is the serializable projection of VariantRules. It is a
// separate struct rather than json tags on VariantRules itself, precisely so
// that adding tags to the live config (and risking it leaking onto the wire)
// is never needed to keep this gate.
//
// DeclarationsEnabled and StopAtTarget are deliberately ABSENT. This golden pins
// what a VARIANT implies, and both of those are per-room settings the owner
// chooses — both presets return the same value for each, so pinning them here
// would record a constant while implying the TS mirror should model them. The
// client learns both from their match_state wire flags instead, which are gated
// by the ws events golden and the Zod schema.
type variantRuleFacts struct {
	DealShape          string `json:"dealShape"`
	HasTrumpCandidate  bool   `json:"hasTrumpCandidate"`
	AllPassOutcome     string `json:"allPassOutcome"`
	DeclarationOverlap bool   `json:"declarationOverlap"`
	DeclarationTiming  string `json:"declarationTiming"`
	TieRule            string `json:"tieRule"`
}

func factsFor(v Variant) variantRuleFacts {
	r := RulesFor(v)
	return variantRuleFacts{
		DealShape:          string(r.DealShape),
		HasTrumpCandidate:  r.HasTrumpCandidate,
		AllPassOutcome:     string(r.AllPassOutcome),
		DeclarationOverlap: r.DeclarationOverlap,
		DeclarationTiming:  string(r.DeclarationTiming),
		TieRule:            string(r.TieRule),
	}
}

// meldFact is one detected declaration, flattened to card IDs so the TS mirror
// can compare without reproducing the Card struct's JSON shape.
type meldFact struct {
	Type  string   `json:"type"`
	Value int      `json:"value"`
	Cards []string `json:"cards"`
}

// handCase is one hand run through the detector under BOTH overlap settings,
// so a single golden pins the shared detection AND the variant-specific dedup.
type handCase struct {
	Name           string     `json:"name"`
	Hand           []string   `json:"hand"`
	WithoutOverlap []meldFact `json:"withoutOverlap"`
	WithOverlap    []meldFact `json:"withOverlap"`
}

func TestVariantRulesAndDeclarationsContract(t *testing.T) {
	require.NoError(t, os.MkdirAll(rulesGoldensDir, 0o755))

	rules := map[string]variantRuleFacts{
		string(VariantBitola):  factsFor(VariantBitola),
		string(VariantCroatia): factsFor(VariantCroatia),
	}

	// Hands chosen so each one isolates a rule the client mirror could get
	// wrong. The overlap pair is the Story 12.5 divergence: the same eight
	// cards yield a different meld SET per variant, which is the single fact
	// that had no cross-language enforcement at all.
	hands := []struct {
		name string
		ids  []string
	}{
		// J of spades belongs to both a spade run and the four jacks. Under
		// one-card-one-group only the higher-valued carré survives; under
		// overlap both do.
		{"overlap_run_and_carre_share_a_jack", []string{"9S", "TS", "JS", "JH", "JD", "JC", "7H", "8D"}},
		// Five in suit: subsumes its own 3- and 4-card subsequences under both
		// settings, so this pins "longer runs swallow shorter ones".
		{"five_card_run_subsumes_shorter", []string{"9S", "TS", "JS", "QS", "KS", "7H", "8D", "9C"}},
		// Two disjoint runs in different suits — no shared card, so both
		// settings must agree.
		{"two_disjoint_runs", []string{"7S", "8S", "9S", "TH", "JH", "QH", "7D", "8C"}},
		// 7s and 8s are worth nothing and are not declarable as a carré.
		{"four_sevens_not_declarable", []string{"7S", "7H", "7D", "7C", "9S", "TH", "JD", "QC"}},
		// Nothing at all — the empty case must serialize as [] on both sides.
		{"no_melds", []string{"7S", "9H", "JD", "KC", "AS", "8H", "TD", "QC"}},
	}

	cases := make([]handCase, 0, len(hands))
	for _, h := range hands {
		hand := mustCards(t, h.ids)
		cases = append(cases, handCase{
			Name:           h.name,
			Hand:           h.ids,
			WithoutOverlap: toMeldFacts(detectDeclarations(hand, false)),
			WithOverlap:    toMeldFacts(detectDeclarations(hand, true)),
		})
	}

	for _, g := range []struct {
		file string
		data any
	}{
		{"variant_rules.json", rules},
		{"declaration_melds.json", cases},
	} {
		t.Run(g.file, func(t *testing.T) {
			actual, err := json.MarshalIndent(g.data, "", "  ")
			require.NoError(t, err)
			actual = append(actual, '\n')

			path := filepath.Join(rulesGoldensDir, g.file)

			if os.Getenv("UPDATE_GOLDENS") == "1" {
				require.NoError(t, os.WriteFile(path, actual, 0o644))
				t.Logf("updated golden: %s", path)
				return
			}

			expected, err := os.ReadFile(path)
			if err != nil {
				// Same rule as the ws goldens (D118): a missing golden hard-fails
				// rather than bootstrapping itself as the new truth.
				t.Fatalf("missing golden %s — rerun with UPDATE_GOLDENS=1 to regenerate: %v", path, err)
			}
			if !bytes.Equal(expected, actual) {
				t.Errorf("golden drift in %s\n--- expected ---\n%s\n--- actual ---\n%s\nrerun with UPDATE_GOLDENS=1 if intentional",
					path, string(expected), string(actual))
			}
		})
	}
}

func toMeldFacts(decls []Declaration) []meldFact {
	out := make([]meldFact, 0, len(decls))
	for _, d := range decls {
		ids := make([]string, 0, len(d.Cards))
		for _, c := range d.Cards {
			ids = append(ids, string(c.Rank)+string(c.Suit))
		}
		out = append(out, meldFact{Type: string(d.Type), Value: d.Value, Cards: ids})
	}
	return out
}

func mustCards(t *testing.T, ids []string) []Card {
	t.Helper()
	out := make([]Card, 0, len(ids))
	for _, id := range ids {
		require.Lenf(t, id, 2, "card id %q must be rank+suit", id)
		out = append(out, Card{Rank: Rank(id[0:1]), Suit: Suit(id[1:2])})
	}
	return out
}

// TestDetectDeclarationsIsDeterministic is the guard that keeps the golden above
// honest, and it exists because the golden alone could not.
//
// detectDeclarations groups cards into `bySuit` and `byRank` maps. Go randomizes
// map iteration order per run, so for any hand holding more than one meld the
// detector used to emit them in a different order from run to run — same melds,
// same values, different list order. That order is not cosmetic: it is the order
// melds reach the wire, this golden, and the client's reveal panel.
//
// The golden DOES catch it, but only when the dice fall the wrong way — measured
// at roughly one run in four, which reads as "flaky CI" rather than "bug" and
// invites a retry instead of a fix. This test catches it every single run: it
// asks for the same hand many times in one process and requires byte-identical
// output, so a reintroduced map range fails immediately and unambiguously.
//
// Multi-meld hands only — a single-meld hand has nothing to reorder and would
// make this vacuous.
func TestDetectDeclarationsIsDeterministic(t *testing.T) {
	hands := []struct {
		name string
		ids  []string
	}{
		// A run and a carré sharing the Jack of spades — the exact hand whose
		// two melds used to swap places in declaration_melds.json.
		{"run_and_carre", []string{"9S", "TS", "JS", "JH", "JD", "JC", "7H", "8D"}},
		// Two runs in different suits: the bySuit range, with nothing else in play.
		{"two_runs_two_suits", []string{"7S", "8S", "9S", "TH", "JH", "QH", "7D", "8C"}},
		// Two carrés: the byRank range, with nothing else in play.
		{"two_carres", []string{"JS", "JH", "JD", "JC", "9S", "9H", "9D", "9C"}},
	}

	for _, h := range hands {
		t.Run(h.name, func(t *testing.T) {
			hand := mustCards(t, h.ids)

			for _, overlap := range []bool{false, true} {
				first, err := json.Marshal(toMeldFacts(detectDeclarations(hand, overlap)))
				require.NoError(t, err)

				// 200 iterations: with two melds a coin-flip ordering survives 200
				// identical draws with probability 2^-199, so a reintroduced map
				// range is caught on effectively every run rather than one in four.
				for i := range 200 {
					again, err := json.Marshal(toMeldFacts(detectDeclarations(hand, overlap)))
					require.NoError(t, err)
					require.Equal(t, string(first), string(again),
						"overlap=%v: meld order changed on iteration %d — detectDeclarations "+
							"must walk AllSuits/AllRanks, never the bySuit/byRank maps", overlap, i)
				}
			}
		})
	}
}
