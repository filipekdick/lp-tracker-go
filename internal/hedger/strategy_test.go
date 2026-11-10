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
		{"below lower (positive)", 0.02, ActionNone},
		{"below lower (negative)", -0.02, ActionNone},
		{"exact lower bound", 0.03, ActionLimit},
		{"passive limit band", 0.045, ActionLimit},
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

	// Edge hysteresis: at rangePositionPct = 98% (near upper tick), thresholds scale by 1.5x (4.5% and 9.0%).
	if got, _ := s.DecideAtRange(0.04, 98.0); got != ActionNone {
		t.Errorf("DecideAtRange(0.04, 98%%) = %v, want %v (edge hysteresis)", got, ActionNone)
	}
	if got, _ := s.DecideAtRange(0.05, 98.0); got != ActionLimit {
		t.Errorf("DecideAtRange(0.05, 98%%) = %v, want %v (edge hysteresis)", got, ActionLimit)
	}
	if got, _ := s.DecideAtRange(0.095, 2.0); got != ActionMarket {
		t.Errorf("DecideAtRange(0.095, 2%%) = %v, want %v (edge hysteresis)", got, ActionMarket)
	}
}
