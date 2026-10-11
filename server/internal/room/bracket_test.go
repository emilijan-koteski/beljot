package room

import "testing"

// TestQuickPlayBuyIn checks the binary bracket: players who can afford 500 pay
// the standard stake, everyone else plays free.
func TestQuickPlayBuyIn(t *testing.T) {
	tests := []struct {
		name    string
		balance int
		want    int
	}{
		{"zero balance plays free", 0, 0},
		{"just below threshold plays free", 499, 0},
		{"exactly at threshold pays standard", 500, quickPlayStandardBuyIn},
		{"just above threshold pays standard", 501, quickPlayStandardBuyIn},
		{"large balance pays standard", 1_000_000, quickPlayStandardBuyIn},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := quickPlayBuyIn(tt.balance); got != tt.want {
				t.Errorf("quickPlayBuyIn(%d) = %d, want %d", tt.balance, got, tt.want)
			}
		})
	}
}
