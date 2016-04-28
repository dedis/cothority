package medco

import (
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/protocols/medco"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/btcsuite/goleveldb/leveldb/errors"
)

type MedcoClient struct {
	*sda.Client
	entryPoint *network.Entity
}

func NewMedcoClient(entryPoint *network.Entity) *MedcoClient {
	newClient := &MedcoClient{
		Client:                sda.NewClient(MEDCO_SERVICE_NAME),
		entryPoint:        entryPoint,
	}
	return newClient
}


func (c *MedcoClient) StartService(entities *sda.EntityList) error {
	resp, err := c.Send(c.entryPoint, &SurveyCreationQuery{*entities})
	if err != nil {
		dbg.Error("Got error when starting the service: "+err.Error())
		return err
	}
	dbg.Lvl1("Successfully started the service: (empty?)", resp.Msg.(ServiceResponse).Code, resp.Msg.(ServiceResponse).Text)
	return nil
}

func (c *MedcoClient) SendSurveyResultsData(data []int64) error {
	suite := network.Suite
	cv := medco.EncryptIntArray(suite, c.Public, data)
	_, err := c.Send(c.entryPoint, &SurveyResponseData{*cv})
	if err != nil {
		dbg.Error("Got error when sending a message: "+err.Error())
		return err
	}
	dbg.Lvl1("Successfully answered survey")
	return nil
}

func (c *MedcoClient) GetSurveyResults() (*[]int64, error) {
	suite := network.Suite
	resp, err := c.Send(c.entryPoint, &SurveyResultsQuery{c.Public})
	if err != nil {
		dbg.Error("Got error when querying the results: "+err.Error())
		return nil, err
	}
	if encResults, ok := resp.Msg.(SurveyResultResponse); ok == true {
		results := medco.DecryptIntVector(suite, c.Private, encResults.CipherVector)
		return &results, nil
	} else {
		dbg.Error("Bad response type from service.")
		return nil, errors.New("Bad response type from service")
	}

}

