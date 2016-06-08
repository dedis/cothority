package medco_service_test

import (
	"testing"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/cothority/services/medco"
)

func TestServiceMedco(t *testing.T) {
	defer dbg.AfterTest(t)
	dbg.TestOutput(testing.Verbose(), 1)
	local := sda.NewLocalTest()
	// generate 5 hosts, they don't connect, they process messages, and they
	// don't register the tree or entitylist
	_, el, _ := local.GenTree(5, true, true, true)
	defer local.CloseAll()

	// Send a request to the service
	client := medco_service.NewMedcoClient(el.List[0])

	if client.CreateSurvey(el) != nil {
		t.Fatal("Service did not start.")
	}

	dbg.Lvl1("Sending response data... ")
	dataHolder := make([]*medco_service.MedcoClient, 4)
	//expected := make([]int64, 4)
	for i:=0; i < 4; i++ {
		dataHolder[i] = medco_service.NewMedcoClient(el.List[i%2])
		grp := make([]int64, 2)
		aggr := make([]int64, 4)
		grp[i%2] = 1
		aggr[i] = 1
		dataHolder[i].SendSurveyResultsData(grp, aggr, el.Aggregate)
	}
	grp,aggr ,err := client.GetSurveyResults()
	if err != nil {
		t.Fatal("Service could not output the results.")
	}

	dbg.Lvl1("Service output:")
	for i,_ := range *grp {
		dbg.Lvl1(i,")", (*grp)[i],"->", (*aggr)[i])
	}
	//if !reflect.DeepEqual(*results, expected) {
	//	t.Fatal("Wrong results.")
	//}


}
