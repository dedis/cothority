package monitor

import (
	"testing"
	"time"
)

type DummyCounterIO struct {
	rvalue uint64
	wvalue uint64
}

func (dm *DummyCounterIO) Rx() uint64 {
	dm.rvalue += 10
	return dm.rvalue
}

func (dm *DummyCounterIO) Tx() uint64 {
	dm.wvalue += 10
	return dm.wvalue
}

func TestCounterIOMeasureRecord(t *testing.T) {
	setupMonitor(t)
	dm := &DummyCounterIO{0, 0}
	// create the counter measure
	cm := NewCounterIOMeasure("dummy", dm)
	if cm.baseRx != dm.rvalue || cm.baseTx != dm.wvalue {
		t.Logf("baseRx = %d vs rvalue = %d || baseTx = %d vs wvalue = %d", cm.baseRx, dm.rvalue, cm.baseTx, dm.wvalue)
		t.Fatal("Tx() / Rx() not working ?")
	}
	//bread, bwritten := cm.baseRx, cm.baseTx
	cm.Record()
	// check the values again
	if cm.baseRx != dm.rvalue || cm.baseTx != dm.wvalue {
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
	EndAndCleanup()
	time.Sleep(100 * time.Millisecond)
}
