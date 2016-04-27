package medco

import (
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/protocols/medco"
	"github.com/dedis/cothority/lib/dbg"
)

type MedcoClient struct {
	*sda.Client
	entryPoint network.Entity
}

func NewMedcoClient(entryPoint network.Entity) *MedcoClient {
	newClient := &MedcoClient{
		Client:                sda.NewClient(MEDCO_SERVICE_NAME),
		entryPoint:        entryPoint,
	}
	return newClient
}


func (c *MedcoClient) StartService(entities sda.EntityList) {
	c.Send(c.entryPoint, &SurveyCreationQuery{entities})
}

func (c *MedcoClient) SendSurveyResultsData(data []int64) {
	suite := network.Suite
	cv := medco.EncryptIntArray(suite, c.Public, data)
	_, err := c.Send(c.entryPoint, &SurveyResponseData{cv})
	if err != nil {
		dbg.Lvl1("Got error when sending a message: "+err.Error())
	}
	dbg.Lvl1("Successfully answered survey")
}

func (c *MedcoClient) GetSurveyResults() *[]int64 {
	suite := network.Suite
	resp, err := c.Send(c.entryPoint, &SurveyResultsQuery{c.Public})
	if err != nil {
		dbg.Lvl1("Got error when querying the results: "+err.Error())
		return nil
	}
	if encResults, ok := resp.Msg.(SurveyResultResponse); ok == true {
		return &medco.DecryptIntVector(suite, c.Private, encResults)
	} else {
		dbg.Lvl1("Bad response type from service.")
		return nil
	}

}

