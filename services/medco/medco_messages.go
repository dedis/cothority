package medco

import (
	"github.com/dedis/cothority/sda"
	. "github.com/dedis/cothority/services/medco/libmedco"
	"github.com/dedis/crypto/abstract"
)

type SurveyCreationQuery struct {
	SurveyID *SurveyID
	sda.Roster
	SurveyDescription
}

type SurveyResponseQuery struct {
	SurveyID SurveyID
	ClientResponse
}

type SurveyResultsQuery struct {
	SurveyID     SurveyID
	ClientPublic abstract.Point
}

type ServiceResponse struct {
	SurveyID SurveyID
}

type SurveyResultResponse struct {
	Results []SurveyResult
}
