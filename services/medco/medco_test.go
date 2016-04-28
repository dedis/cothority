package medco_test

import (
	"testing"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/cothority/services/medco"
	"time"
	"reflect"
)

func TestServiceMedco(t *testing.T) {
	defer dbg.AfterTest(t)
	dbg.TestOutput(testing.Verbose(), 4)
	local := sda.NewLocalTest()
	// generate 5 hosts, they don't connect, they process messages, and they
	// don't register the tree or entitylist
	_, el, _ := local.GenTree(5, true, true, true)
	defer local.CloseAll()

	// Send a request to the service
	client := medco.NewMedcoClient(el.List[0])

	if client.StartService(el) != nil {
		t.Fatal("Service did not start.")
	}
	dbg.Lvl1("Waiting for survey creation...")
	<- time.After(1*time.Second)

	dbg.Lvl1("Sending response data...")
	dataHolder := make([]*medco.MedcoClient, 4)
	expected := make([]int64, 4)
	for i:=0; i < 4; i++ {
		dataHolder[i] = medco.NewMedcoClient(el.List[i])
		res := make([]int64, 4)
		res[i] = 1
		expected[i] += 1
		dataHolder[i].SendSurveyResultsData(res)
	}
	<- time.After(1*time.Second)

	results,err := client.GetSurveyResults()
	if err != nil {
		t.Fatal("Service could not output the results.")
	}

	dbg.Lvl1("Service output:", *results, "expected:", expected)
	if !reflect.DeepEqual(*results, expected) {
		t.Fatal("Wrong results.")
	}


}
