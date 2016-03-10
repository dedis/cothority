package monitor

import (
	"testing"
)

type DummyCounterIO struct {
	rvalue uint64
	wvalue uint64
}

func (dm *DummyCounterIO) Read() uint64 {
	dm.rvalue += 10
	return dm.rvalue
}

func (dm *DummyCounterIO) Written() uint64 {
	dm.wvalue += 10
	return dm.wvalue
}

func TestCounterIOMeasureRecord(t *testing.T) {
	mon, _ := setupMonitor(t)
	dm := &DummyCounterIO{0, 0}
	// create the counter measure
	cm := NewCounterIOMeasure("dummy", dm)
	if cm.baseRead != dm.rvalue || cm.baseWritten != dm.wvalue {
		t.Logf("baseRead = %d vs rvalue = %d || baseWritten = %d vs wvalue = %d", cm.baseRead, dm.rvalue, cm.baseWritten, dm.wvalue)
		t.Fatal("Written() / Read() not working ?")
	}
	//bread, bwritten := cm.baseRead, cm.baseWritten
	cm.Record()
	// check the values again
	if cm.baseRead != dm.rvalue || cm.baseWritten != dm.wvalue {
		t.Fatal("Record() not working for CounterIOMeasure")
	}

	// TODO do a normal test with single measure passing by the Monitor to the
	// Stats and verify if the stats correctly handles it. With CounterIO it
	// does not even appears... ??
	/*str := new(bytes.Buffer)*/
	//stat.WriteHeader(str)
	//stat.WriteValues(str)
	//t.Logf("Stats => %s", str)
	//wr, re := stat.Value("dummy_written"), stat.Value("dummy_read")
	//if wr == nil || wr.Avg() != 10 {
	//t.Logf("stats => %v", stat.values)
	////t.Logf("wr.Avg() = %f", wr.Avg())
	//t.Fatal("Stats don't have the right value (write)")
	//}
	//if re == nil || re.Avg() != 10 {
	//t.Fatal("Stats don't have the right value (read)")
	/*}*/
	End()
	mon.Stop()
}
