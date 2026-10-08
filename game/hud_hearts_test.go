package game

import "testing"

func TestHealthHeartFills(t *testing.T) {
	for _, tc := range []struct {
		health float64
		want   [3]float64
	}{
		{1, [3]float64{1, 1, 1}},
		{0, [3]float64{0, 0, 0}},
		{.5, [3]float64{1, .5, 0}},
		{1.0 / 3, [3]float64{1, 0, 0}},
	} {
		got := healthHeartFills(tc.health)
		for i := range got {
			if d := got[i] - tc.want[i]; d > 1e-9 || d < -1e-9 {
				t.Fatalf("health %v: got %v want %v", tc.health, got, tc.want)
			}
		}
	}
}
