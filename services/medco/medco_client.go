package medco_service

import (
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/btcsuite/goleveldb/leveldb/errors"
	"github.com/dedis/crypto/abstract"
	"strconv"
	."github.com/dedis/cothority/services/medco/structs"
	"github.com/dedis/crypto/config"
)

type MedcoClient struct {
	*sda.Client
	entryPoint *network.Entity
	localClientNumber int64
	public abstract.Point
	private abstract.Secret
}

var localClientCounter = int64(0)

func NewMedcoClient(entryPoint *network.Entity) *MedcoClient {
	keys := config.NewKeyPair(network.Suite)
	newClient := &MedcoClient{
		Client:			sda.NewClient(MEDCO_SERVICE_NAME),
		entryPoint:   		entryPoint,
		localClientNumber:	localClientCounter,
		public:			keys.Public,
		private:		keys.Secret,
	}

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

func (c *MedcoClient) SendSurveyResultsData(grouping, aggregating []int64, groupKey abstract.Point) error {
	suite := network.Suite
	encGrouping := EncryptIntArray(suite, groupKey, grouping)
	encAggregating := EncryptIntArray(suite, groupKey, aggregating)
	_, err := c.Send(c.entryPoint, &ClientResponse{*encGrouping, *encAggregating})
	if err != nil {
		dbg.Error("Got error when sending a message: "+err.Error())
		return err
	}
	return nil
}

func (c *MedcoClient) GetSurveyResults() (*[][]int64, *[][]int64, error) {
	suite := network.Suite
	resp, err := c.Send(c.entryPoint, &SurveyResultsQuery{c.public})
	if err != nil {
		dbg.Error("Got error when querying the results: "+err.Error())
		return nil, nil, err
	}
	if encResults, ok := resp.Msg.(SurveyResultResponse); ok == true {
		dbg.Lvl1(c, "got the survey result from", c.entryPoint)
		grp := make([][]int64, len(encResults.Results))
		aggr := make([][]int64, len(encResults.Results))
		for i, res := range encResults.Results {
			grp[i] = DecryptIntVector(suite, c.private, res.GroupingAttributes)
			aggr[i] = DecryptIntVector(suite, c.private, res.AggregatingAttributes)
		}
		return &grp, &aggr, nil
	} else {
		dbg.Error("Bad response type from service.")
		return nil, nil, errors.New("Bad response type from service")
	}
}

func (c *MedcoClient) String() string {
	return "[Client-"+strconv.FormatInt(c.localClientNumber,10)+"]"
}

