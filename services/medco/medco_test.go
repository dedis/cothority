package medco_service_test

import (
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/cothority/services/medco"
	"testing"
	"github.com/dedis/cothority/services/medco/structs"
)

func TestServiceMedco(t *testing.T) {
	//t.Skip()
	defer dbg.AfterTest(t)
	dbg.TestOutput(testing.Verbose(), 1)
	local := sda.NewLocalTest()
	// generate 5 hosts, they don't connect, they process messages, and they
	// don't register the tree or entitylist
	_, el, _ := local.GenTree(5, true, true, true)
	defer local.CloseAll()

	// Send a request to the service
	client := medco_service.NewMedcoClient(el.List[0])
	surveyDesc := medco_structs.SurveyDescription{2,10}
	if client.CreateSurvey(el, surveyDesc) != nil {
		t.Fatal("Service did not start.")
	}

	dbg.Lvl1("Sending response data... ")
	dataHolder := make([]*medco_service.MedcoClient, 10)
	for i := 0; i < 10; i++ {
		dataHolder[i] = medco_service.NewMedcoClient(el.List[i%5])
		grp := make([]int64, 2)
		aggr := make([]int64, 10)
		//grp[i%2] = int64(2)
		aggr[i%2] = 3
		dataHolder[i].SendSurveyResultsData(grp, aggr, el.Aggregate)
	}

	grp, aggr, err := client.GetSurveyResults()
	if err != nil {
		t.Fatal("Service could not output the results.")
	}

	dbg.Lvl1("Service output:")
	for i, _ := range *grp {
		dbg.Lvl1(i, ")", (*grp)[i], "->", (*aggr)[i])
	}
}