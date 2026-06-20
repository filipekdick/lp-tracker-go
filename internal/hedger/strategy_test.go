package hedger

import "testing"

func TestStrategy_Decide(t *testing.T) {
	s := &Strategy{
		LowerThreshold: 0.03,
		UpperThreshold: 0.06,
	}

	tests := []struct {
		name     string
		driftPct float64
		want     Action
	}{
		{"below lower (positive)", 0.02, ActionLimit},
		{"below lower (negative)", -0.02, ActionLimit},
		{"exact lower bound", 0.03, ActionNone},
		{"dead band", 0.045, ActionNone},
		{"exact upper bound", 0.06, ActionMarket},
		{"above upper (positive)", 0.07, ActionMarket},
		{"above upper (negative)", -0.07, ActionMarket},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := s.Decide(tt.driftPct)
			if got != tt.want {
				t.Errorf("Decide(%v) = %v, want %v", tt.driftPct, got, tt.want)
			}
		})
	}
}
