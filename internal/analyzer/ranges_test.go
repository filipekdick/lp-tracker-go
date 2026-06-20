package analyzer

import (
	"math"
	"testing"
)

// TestLognormalBandsClosedForm pins the bands against the lognormal formula and
// a hand-computed value. These are deterministic, so the tolerance is tiny.
func TestLognormalBandsClosedForm(t *testing.T) {
	const (
		sigma = 0.60
		T     = 1.0 // day
	)
	// s = sigma*sqrt(T/365) = 0.60*sqrt(1/365) = 0.60*0.0523424 = 0.03140547
	s := sigma * math.Sqrt(T/365.0)
	b1 := LognormalBands(sigma, T, 1)
	if math.Abs(b1.UpPct-(math.Exp(s)-1)) > 1e-12 {
		t.Fatalf("1σ up = %.12f, want %.12f", b1.UpPct, math.Exp(s)-1)
	}
	if math.Abs(b1.DownPct-(1-math.Exp(-s))) > 1e-12 {
		t.Fatalf("1σ down = %.12f, want %.12f", b1.DownPct, 1-math.Exp(-s))
	}
	// Hand value: s = 0.60*sqrt(1/365) = 0.031405467; exp(s)-1 = 0.031903789.
	if math.Abs(b1.UpPct-0.031903789) > 1e-7 {
		t.Fatalf("1σ up = %.9f, want ≈0.031903789", b1.UpPct)
	}

	b2 := LognormalBands(sigma, T, 2)
	if math.Abs(b2.UpPct-(math.Exp(2*s)-1)) > 1e-12 {
		t.Fatalf("2σ up = %.12f, want %.12f", b2.UpPct, math.Exp(2*s)-1)
	}
	if math.Abs(b2.DownPct-(1-math.Exp(-2*s))) > 1e-12 {
		t.Fatalf("2σ down = %.12f, want %.12f", b2.DownPct, 1-math.Exp(-2*s))
	}
}

// TestBandTimeScaling verifies the log half-width scales exactly with sqrt(T):
// doubling T multiplies it by sqrt(2).
func TestBandTimeScaling(t *testing.T) {
	const sigma = 0.5
	w1 := LogHalfWidth(sigma, 1, 1)
	w2 := LogHalfWidth(sigma, 2, 1)
	if math.Abs(w2/w1-math.Sqrt2) > 1e-12 {
		t.Fatalf("doubling T scaled width by %.12f, want sqrt(2)=%.12f", w2/w1, math.Sqrt2)
	}
}

// TestBandAsymmetry verifies the up move is strictly larger in magnitude than
// the down move (lognormal: upside compounds, downside is floored at zero).
func TestBandAsymmetry(t *testing.T) {
	for _, sigma := range []float64{0.2, 0.6, 1.2} {
		for _, T := range []float64{0.5, 1, 7} {
			for _, z := range []float64{1, 2} {
				b := LognormalBands(sigma, T, z)
				if !(b.UpPct > b.DownPct) {
					t.Fatalf("sigma=%v T=%v z=%v: up=%.6f not > down=%.6f", sigma, T, z, b.UpPct, b.DownPct)
				}
			}
		}
	}
}

// TestBandsZeroInputs guards degenerate inputs.
func TestBandsZeroInputs(t *testing.T) {
	if b := LognormalBands(0, 1, 1); b.UpPct != 0 || b.DownPct != 0 {
		t.Fatalf("zero sigma should give zero bands, got %+v", b)
	}
	if b := LognormalBands(0.5, 0, 1); b.UpPct != 0 {
		t.Fatalf("zero horizon should give zero bands, got %+v", b)
	}
}
