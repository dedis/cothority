package medco

import (
	"github.com/btcsuite/goleveldb/leveldb/errors"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sda"
	. "github.com/dedis/cothority/lib/medco"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/config"
	"strconv"
)

type MedcoAPI struct {
	*sda.Client
	entryPoint        *network.Entity
	localClientNumber int64
	public            abstract.Point
	private           abstract.Secret
}

var localClientCounter = int64(0)

func NewMedcoClient(entryPoint *network.Entity) *MedcoAPI {
	keys := config.NewKeyPair(network.Suite)
	newClient := &MedcoAPI{
		Client:            sda.NewClient(MEDCO_SERVICE_NAME),
		entryPoint:        entryPoint,
		localClientNumber: localClientCounter,
		public:            keys.Public,
		private:           keys.Secret,
	}

	localClientCounter += 1
	return newClient
}

func (c *MedcoAPI) CreateSurvey(entities *sda.EntityList, surveyDescription SurveyDescription) (*SurveyID, error) {
	dbg.Lvl1(c, "is creating a survey.")
	resp, err := c.Send(c.entryPoint, &SurveyCreationQuery{nil, *entities, surveyDescription})
	if err != nil {
		dbg.Error("Got error when creating survey: " + err.Error())
		return nil, err
	}
	dbg.Lvl1(c, "successfully created the survey with ID", resp.Msg.(ServiceResponse).SurveyID)
	surveyID := resp.Msg.(ServiceResponse).SurveyID
	return &surveyID,nil
}

func (c *MedcoAPI) SendSurveyResultsData(surveyID SurveyID, grouping, aggregating []int64, groupKey abstract.Point) error {
	dbg.Lvl1(c, "responds {", grouping, ",", aggregating, "}")
	encGrouping := EncryptIntVector(groupKey, grouping)
	encAggregating := EncryptIntVector(groupKey, aggregating)
	_, err := c.Send(c.entryPoint, &SurveyResponseQuery{surveyID, ClientResponse{*encGrouping, *encAggregating}})
	if err != nil {
		dbg.Error("Got error when sending a message: " + err.Error())
		return err
	}
	return nil
}

func (c *MedcoAPI) GetSurveyResults(surveyID SurveyID) (*[][]int64, *[][]int64, error) {
	resp, err := c.Send(c.entryPoint, &SurveyResultsQuery{surveyID, c.public})
	if err != nil {
		dbg.Error("Got error when querying the results: " + err.Error())
		return nil, nil, err
	}
	if encResults, ok := resp.Msg.(SurveyResultResponse); ok == true {
		dbg.Lvl1(c, "got the survey result from", c.entryPoint)
		grp := make([][]int64, len(encResults.Results))
		aggr := make([][]int64, len(encResults.Results))
		for i, res := range encResults.Results {
			grp[i] = DecryptIntVector(c.private, &res.GroupingAttributes)
			aggr[i] = DecryptIntVector(c.private, &res.AggregatingAttributes)
		}
		return &grp, &aggr, nil
	} else {
		dbg.Error("Bad response type from service.")
		return nil, nil, errors.New("Bad response type from service")
	}
}

func (c *MedcoAPI) String() string {
	return "[Client-" + strconv.FormatInt(c.localClientNumber, 10) + "]"
}
