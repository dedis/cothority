package monitor

import (
	"bytes"
	"fmt"
	"github.com/dedis/cothority/lib/dbg"
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
	rc["hosts"] = "2"
	stats := NewStats(rc)

	m1 := NewSingleMeasure("round_wall", 10)
	m2 := NewSingleMeasure("round_wall", 30)
	stats.Update(m1)
	stats.Update(m2)
	stats.Collect()
	val := stats.values["round_wall"]
	if val.Avg() != 20 {
		t.Error("Aggregate or Update not working")
	}
}
func TestStatsOrder(t *testing.T) {
	dbg.TestOutput(testing.Verbose(), 3)
	m := make(map[string]string)
	m["machines"] = "1"
	m["hosts"] = "1"
	m["bf"] = "2"
	// create stats
	stat := NewStats(m)
	m1 := NewSingleMeasure("round", 10)
	m2 := NewSingleMeasure("setup", 5)
	stat.Update(m1)
	stat.Update(m2)
	str := new(bytes.Buffer)
	stat.WriteHeader(str)
	stat.WriteValues(str)

	stat2 := NewStats(m)
	stat2.Update(m1)
	stat2.Update(m2)

	str2 := new(bytes.Buffer)
	stat2.WriteHeader(str2)
	stat2.WriteValues(str2)
	if !bytes.Equal(str.Bytes(), str2.Bytes()) {
		t.Fatal("KeyOrder / output not the same for same stats")
	}
}
func TestStatsAverage(t *testing.T) {
	dbg.TestOutput(testing.Verbose(), 3)
	m := make(map[string]string)
	m["machines"] = "1"
	m["hosts"] = "1"
	m["bf"] = "2"
	// create stats
	stat1 := NewStats(m)
	stat2 := NewStats(m)
	m1 := NewSingleMeasure("round", 10)
	m2 := NewSingleMeasure("setup", 5)
	stat1.Update(m1)
	stat2.Update(m2)

	str := new(bytes.Buffer)
	avgStat := AverageStats([]*Stats{stat1, stat2})
	avgStat.WriteHeader(str)
	avgStat.WriteValues(str)

	stat3 := NewStats(m)
	stat4 := NewStats(m)
	stat3.Update(m1)
	stat4.Update(m2)

	str2 := new(bytes.Buffer)
	avgStat2 := AverageStats([]*Stats{stat3, stat4})
	avgStat2.WriteHeader(str2)
	avgStat2.WriteValues(str2)

	if !bytes.Equal(str.Bytes(), str2.Bytes()) {
		t.Fatal("Average are not the same !")
	}
}

func TestStatsAverageFiltered(t *testing.T) {
	dbg.TestOutput(testing.Verbose(), 3)
	m := make(map[string]string)
	m["machines"] = "1"
	m["hosts"] = "1"
	m["bf"] = "2"
	// create the filter entry
	m["filter_round"] = "50"
	// create stats
	stat1 := NewStats(m)
	stat2 := NewStats(m)
	m1 := NewSingleMeasure("round", 10)
	m2 := NewSingleMeasure("round", 20)
	m3 := NewSingleMeasure("round", 150)
	stat1.Update(m1)
	stat1.Update(m2)
	stat1.Update(m3)
	stat2.Update(m1)
	stat2.Update(m2)
	stat2.Update(m3)

	/* stat2.Collect()*/
	//val := stat2.Value("round")
	//if val.Avg() != (10+20)/2 {
	//t.Fatal("Average with filter does not work?")
	//}

	str := new(bytes.Buffer)
	avgStat := AverageStats([]*Stats{stat1, stat2})
	avgStat.WriteHeader(str)
	avgStat.WriteValues(str)

	stat3 := NewStats(m)
	stat4 := NewStats(m)
	stat3.Update(m1)
	stat3.Update(m2)
	stat3.Update(m3)
	stat4.Update(m1)
	stat4.Update(m2)
	stat4.Update(m3)

	str2 := new(bytes.Buffer)
	avgStat2 := AverageStats([]*Stats{stat3, stat4})
	avgStat2.WriteHeader(str2)
	avgStat2.WriteValues(str2)

	if !bytes.Equal(str.Bytes(), str2.Bytes()) {
		t.Fatal("Average are not the same !")
	}

}
