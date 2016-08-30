package medco

import (
	"strconv"

	"github.com/btcsuite/goleveldb/leveldb/errors"
	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/network"
	"github.com/dedis/cothority/sda"
	"github.com/dedis/cothority/services/medco/libmedco"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/config"
)

// API represents a client with the server to which he is connected and its public/private key pair.
type API struct {
	*sda.Client
	entryPoint        *network.ServerIdentity
	localClientNumber int64
	public            abstract.Point
	private           abstract.Scalar
}

var localClientCounter = int64(0)

// NewMedcoClient constructor of a client.
func NewMedcoClient(entryPoint *network.ServerIdentity) *API {
	keys := config.NewKeyPair(network.Suite)
	newClient := &API{
		Client:            sda.NewClient(ServiceName),
		entryPoint:        entryPoint,
		localClientNumber: localClientCounter,
		public:            keys.Public,
		private:           keys.Secret,
	}

	localClientCounter++
	return newClient
}

// CreateSurvey creates a survey based on a set of entities (servers) and a survey description.
func (c *API) CreateSurvey(entities *sda.Roster, surveyDescription libmedco.SurveyDescription) (*libmedco.SurveyID, error) {
	log.Lvl1(c, "is creating a survey.")
	resp, err := c.Send(c.entryPoint, &SurveyCreationQuery{nil, *entities, surveyDescription})
	if err != nil {
		return nil, err
	}
	log.Lvl1(c, "successfully created the survey with ID", resp.Msg.(ServiceResponse).SurveyID)
	surveyID := resp.Msg.(ServiceResponse).SurveyID
	return &surveyID, nil
}

// SendSurveyResultsData creates and sends a client response encrypted with the cothority collective key.
func (c *API) SendSurveyResultsData(surveyID libmedco.SurveyID, grouping, aggregating []int64, groupKey abstract.Point) error {
	log.Lvl1(c, "responds {", grouping, ",", aggregating, "}")
	encGrouping := libmedco.EncryptIntVector(groupKey, grouping)
	encAggregating := libmedco.EncryptIntVector(groupKey, aggregating)
	_, err := c.Send(c.entryPoint, &SurveyResponseQuery{surveyID,
		libmedco.ClientResponse{
			ProbabilisticGroupingAttributes: *encGrouping,
			AggregatingAttributes:           *encAggregating}})
	if err != nil {
		return err
	}
	return nil
}

// GetSurveyResults to get the result from associated server and decrypt the response using its private key.
func (c *API) GetSurveyResults(surveyID libmedco.SurveyID) (*[][]int64, *[][]int64, error) {
	resp, err := c.Send(c.entryPoint, &SurveyResultsQuery{surveyID, c.public})
	if err != nil {
		return nil, nil, err
	}
	if encResults, ok := resp.Msg.(SurveyResultResponse); ok == true {
		log.Lvl1(c, "got the survey result from", c.entryPoint)
		grp := make([][]int64, len(encResults.Results))
		aggr := make([][]int64, len(encResults.Results))
		for i, res := range encResults.Results {
			grp[i] = libmedco.DecryptIntVector(c.private, &res.GroupingAttributes)
			aggr[i] = libmedco.DecryptIntVector(c.private, &res.AggregatingAttributes)
		}
		return &grp, &aggr, nil
	}

	log.Error("Bad response type from service.")
	return nil, nil, errors.New("Bad response type from service")

}

// String permits to have the string representation of a client.
func (c *API) String() string {
	return "[Client-" + strconv.FormatInt(c.localClientNumber, 10) + "]"
}
