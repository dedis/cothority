package monitor

import (
	"fmt"
	"testing"
)

func TestNewDataFilter(t *testing.T) {
	rc := make(map[string]string)
	rc["filter_round"] = "50"
	rc["filter_verify"] = "90"
	df := NewDataFilter(rc)
	if df.percentiles["round"] == 0 || df.percentiles["verify"] == 0 {
		t.Error("Datafilter not correctly parsed the run config")
	}
	if df.percentiles["round"] != 50.0 || df.percentiles["verify"] != 90.0 {
		t.Error(fmt.Sprintf("datafilter not correctly parsed the percentile: %f vs 50 or %f vs 90", df.percentiles["round"], df.percentiles["verifiy"]))
	}
}

func TestDataFilterFilter(t *testing.T) {
	rc := make(map[string]string)
	rc["filter_round"] = "75"
	df := NewDataFilter(rc)

	values := []float64{35, 20, 15, 40, 50}
	filtered := df.Filter("round", values)
	shouldBe := []float64{35, 20, 15, 40}
	if len(shouldBe) != len(filtered) {
		t.Error(fmt.Sprintf("Filter returned %d values instead of %d", len(filtered), len(shouldBe)))
	}
	for i, v := range filtered {
		if v != shouldBe[i] {
			t.Error(fmt.Sprintf("Element %d = %d vs %d", i, filtered[i], shouldBe[i]))
		}
	}
}

func TestStatsUpdate(t *testing.T) {
	rc := make(map[string]string)
	rc["machines"] = "2"
	rc["ppm"] = "2"
	stats := NewStats(rc)

	m1 := Measure{
		Name:        "round",
		WallTime:    10,
		CPUTimeUser: 20,
		CPUTimeSys:  30,
	}
	m2 := Measure{
		Name:        "round",
		WallTime:    10,
		CPUTimeUser: 20,
		CPUTimeSys:  30,
	}
	stats.Update(m1)
	stats.Update(m2)
	stats.Aggregate()
	meas := stats.measures["round"]
	if meas.Wall.Avg() != 10 || meas.User.Avg() != 20 {
		t.Error("Aggregate or Update not working")
	}
}
