package medco_service

import (
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/cothority/services/medco/store"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/cothority/services/medco/structs"
)

type SurveyCreationQuery struct {
	sda.EntityList
	medco_structs.SurveyDescription
	*medco_structs.SurveyID
}

type ServiceResponse struct {
	SurveyCode int32
}

type SurveyResultsQuery struct {
	ClientPublic abstract.Point
}

type SurveyResultResponse struct {
	Results []store.SurveyResult
}
