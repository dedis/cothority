package medco_service

import (
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/protocols/medco"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/btcsuite/goleveldb/leveldb/errors"
	"github.com/dedis/crypto/abstract"
	"strconv"
)

type MedcoClient struct {
	*sda.Client
	entryPoint *network.Entity
	localClientNumber int64
}

var localClientCounter = int64(0)

func NewMedcoClient(entryPoint *network.Entity) *MedcoClient {
	newClient := &MedcoClient{
		Client:                sda.NewClient(MEDCO_SERVICE_NAME),
		entryPoint:        entryPoint,
		localClientNumber: localClientCounter,
	}
	newClient.Addresses = []string{newClient.String()}
	localClientCounter += 1
	return newClient
}


func (c *MedcoClient) CreateSurvey(entities *sda.EntityList) error {
	dbg.Lvl1(c, "is creating a survey.")
	resp, err := c.Send(c.entryPoint, &SurveyCreationQuery{*entities})
	if err != nil {
		dbg.Error("Got error when creating survey: "+err.Error())
		return err
	}
	dbg.Lvl1(c, "successfully created the survey with code", resp.Msg.(ServiceResponse).SurveyCode)
	return nil
}

func (c *MedcoClient) SendSurveyResultsData(data []int64, groupKey abstract.Point) error {
	suite := network.Suite
	cv := medco.EncryptIntArray(suite, groupKey, data)
	_, err := c.Send(c.entryPoint, &SurveyResponseData{*cv})
	if err != nil {
		dbg.Error("Got error when sending a message: "+err.Error())
		return err
	}
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
		dbg.Lvl1(c, "got the survey result from", c.entryPoint)
		results := medco.DecryptIntVector(suite, c.private, encResults.Vect)
		return &results, nil
	} else {
		dbg.Error("Bad response type from service.")
		return nil, errors.New("Bad response type from service")
	}
}

func (c *MedcoClient) String() string {
	return "[Client-"+strconv.FormatInt(c.localClientNumber,10)+"]"
}

