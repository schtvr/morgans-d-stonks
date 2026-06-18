package portfolio

import (
	"testing"
	"time"
)

func TestDownsampleMarketCandlePoints(t *testing.T) {
	t.Parallel()
	var pts []MarketCandlePoint
	for i := 0; i < 10; i++ {
		pts = append(pts, MarketCandlePoint{AsOf: time.Unix(int64(i), 0).UTC(), Close: float64(i)})
	}
	out := DownsampleMarketCandlePoints(pts, 4)
	if len(out) != 4 {
		t.Fatalf("len %d", len(out))
	}
	if out[0].Close != 0 || out[len(out)-1].Close != 9 {
		t.Fatalf("endpoints %+v", out)
	}
}
