package botlab

import (
	"strconv"
	"strings"

	"github.com/emilijan/beljot/server/internal/bot"
)

// Flags switch experimental rule variants on in the lab bot.
type Flags struct {
	set  map[string]bool
	Text string
}

func (f Flags) On(name string) bool { return f.set[name] }

func ParseFlags(list string) Flags {
	f := Flags{set: map[string]bool{}, Text: list}
	for _, p := range strings.Split(list, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			f.set[p] = true
		}
	}
	return f
}

// ruffThreshold returns the ruffNN flag as a probability, or 0 when unset.
func (f Flags) ruffThreshold() float64 {
	for k := range f.set {
		if strings.HasPrefix(k, "ruff") {
			if n, err := strconv.Atoi(strings.TrimPrefix(k, "ruff")); err == nil {
				return float64(n) / 100
			}
		}
	}
	return 0
}

// TuningFromFlags builds the search tuning from the default plus the phase-3 flags; unknown flags are ignored.
func TuningFromFlags(f Flags) bot.SearchTuning {
	t := bot.DefaultSearchTuning()
	if f.On("phase2") {
		t = bot.SearchTuning{}
		t.SampleCaps[3], t.SampleCaps[4], t.SampleCaps[5] = 16, 30, 40
	}
	switch {
	case f.On("defTrump3"):
		t.DefenderTrumpLeadBonus = 3
	case f.On("defTrump10"):
		t.DefenderTrumpLeadBonus = 10
	}
	if f.On("defTrumpVeto") {
		t.DefenderTrumpLeadBonus = bot.DefenderTrumpLeadVeto
	}
	if f.On("noDefTrump") {
		t.DefenderTrumpLeadBonus = 0
	}
	if f.On("tenUnder3") {
		t.TenUnderAceBonus = 3
	}
	if f.On("noTenUnder") {
		t.TenUnderAceBonus = 0
	}
	if l := f.riskLambda(); l > 0 {
		t.LeadRiskLambda = l
	}
	if f.On("noRisk") {
		t.LeadRiskLambda = 0
	}
	if f.On("drawCond") {
		t.TrumpDrawConditional = true
	}
	if f.On("noDrawCond") {
		t.TrumpDrawConditional = false
	}
	if f.On("race") {
		t.RaceObjective = true
	}
	if f.On("noRace") {
		t.RaceObjective = false
	}
	if f.On("opening12") {
		t.OpeningSamples = 12
	}
	if f.On("noOpening") {
		t.OpeningSamples = 0
	}
	if f.On("caps2x") {
		t.SampleCaps[3], t.SampleCaps[4], t.SampleCaps[5] = 32, 60, 80
	}
	if f.On("noCaps") {
		t.SampleCaps = [9]int{}
	}
	return t
}

// riskLambda returns the riskNN flag as NN/10 (risk10 = 1.0), or 0 when unset.
func (f Flags) riskLambda() float64 {
	for k := range f.set {
		if strings.HasPrefix(k, "risk") {
			if n, err := strconv.Atoi(strings.TrimPrefix(k, "risk")); err == nil {
				return float64(n) / 10
			}
		}
	}
	return 0
}
