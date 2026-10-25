package botlab

import (
	"strconv"
	"strings"
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
