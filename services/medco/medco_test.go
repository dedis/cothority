package medco_test

import (
	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/sda"
	"github.com/dedis/cothority/services/medco"
	. "github.com/dedis/cothority/services/medco/libmedco"
	"testing"
	"time"
)

func TestServiceMedco(t *testing.T) {
	//t.Skip()
	defer log.AfterTest(t)
	log.TestOutput(testing.Verbose(), 1)
	local := sda.NewLocalTest()
	// generate 5 hosts, they don't connect, they process messages, and they
	// don't register the tree or entitylist
	_, el, _ := local.GenTree(5, true, true, true)
	defer local.CloseAll()

	// Send a request to the service
	client := medco.NewMedcoClient(el.List[0])

	surveyDesc := SurveyDescription{1, 10}
	surveyID, err := client.CreateSurvey(el, surveyDesc)
	if err != nil {
		t.Fatal("Service did not start.")
	}

	<-time.After(0 * time.Second)

	log.Lvl1("Sending response data... ")
	dataHolder := make([]*medco.MedcoAPI, 10)
	for i := 0; i < 10; i++ {
		dataHolder[i] = medco.NewMedcoClient(el.List[i%5])
		grp := make([]int64, 1)
		aggr := make([]int64, 10)
		grp[0] = int64(i % 4)
		aggr[i] = 3
		dataHolder[i].SendSurveyResultsData(*surveyID, grp, aggr, el.Aggregate)
	}

	<-time.After(0 * time.Second)

	grp, aggr, err := client.GetSurveyResults(*surveyID)
	if err != nil {
		t.Fatal("Service could not output the results.")
	}

	log.Lvl1("Service output:")
	for i, _ := range *grp {
		log.Lvl1(i, ")", (*grp)[i], "->", (*aggr)[i])
	}
}
