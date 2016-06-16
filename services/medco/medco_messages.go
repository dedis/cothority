package medco_service

import (
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/cothority/services/medco/structs"
)


type SurveyCreationQuery struct {
	SurveyID *medco_structs.SurveyID
	sda.EntityList
	medco_structs.SurveyDescription
}

type SurveyResponseQuery struct {
	SurveyID medco_structs.SurveyID
	medco_structs.ClientResponse
}

type SurveyResultsQuery struct {
	SurveyID     medco_structs.SurveyID
	ClientPublic abstract.Point
}

type ServiceResponse struct {
	SurveyID medco_structs.SurveyID
}


type SurveyResultResponse struct {
	Results []medco_structs.SurveyResult
}
