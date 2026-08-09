package httpapi

import "testing"

func TestValidOverviewPeriod(t *testing.T) {
	for _, period := range []string{"today", "yesterday", "7d", "month"} {
		if !validOverviewPeriod(period) {
			t.Errorf("validOverviewPeriod(%q) = false, want true", period)
		}
	}
	for _, period := range []string{"", "30d", "all", "MONTH"} {
		if validOverviewPeriod(period) {
			t.Errorf("validOverviewPeriod(%q) = true, want false", period)
		}
	}
}
