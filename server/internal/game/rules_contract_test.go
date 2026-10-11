package game

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

const rulesGoldensDir = "testdata/contract"

// variantRuleFacts is the serializable projection of VariantRules pinned by the golden, kept
// separate so the live config never needs json tags.
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

	hands := []struct {
		name string
		ids  []string
	}{
		{"overlap_run_and_carre_share_a_jack", []string{"9S", "TS", "JS", "JH", "JD", "JC", "7H", "8D"}},
		{"five_card_run_subsumes_shorter", []string{"9S", "TS", "JS", "QS", "KS", "7H", "8D", "9C"}},
		{"two_disjoint_runs", []string{"7S", "8S", "9S", "TH", "JH", "QH", "7D", "8C"}},
		{"four_sevens_not_declarable", []string{"7S", "7H", "7D", "7C", "9S", "TH", "JD", "QC"}},
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

// TestDetectDeclarationsIsDeterministic checks detectDeclarations returns byte-identical output for
// repeated multi-meld hands, catching map-order nondeterminism every run.
func TestDetectDeclarationsIsDeterministic(t *testing.T) {
	hands := []struct {
		name string
		ids  []string
	}{
		{"run_and_carre", []string{"9S", "TS", "JS", "JH", "JD", "JC", "7H", "8D"}},
		{"two_runs_two_suits", []string{"7S", "8S", "9S", "TH", "JH", "QH", "7D", "8C"}},
		{"two_carres", []string{"JS", "JH", "JD", "JC", "9S", "9H", "9D", "9C"}},
	}

	for _, h := range hands {
		t.Run(h.name, func(t *testing.T) {
			hand := mustCards(t, h.ids)

			for _, overlap := range []bool{false, true} {
				first, err := json.Marshal(toMeldFacts(detectDeclarations(hand, overlap)))
				require.NoError(t, err)

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
