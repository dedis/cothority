package medco

import (
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/cothority/lib/medco"
)


type SurveyCreationQuery struct {
	SurveyID *medco.SurveyID
	sda.EntityList
	medco.SurveyDescription
}

type SurveyResponseQuery struct {
	SurveyID medco.SurveyID
	medco.ClientResponse
}

type SurveyResultsQuery struct {
	SurveyID     medco.SurveyID
	ClientPublic abstract.Point
}

type ServiceResponse struct {
	SurveyID medco.SurveyID
}


type SurveyResultResponse struct {
	Results []medco.SurveyResult
}
