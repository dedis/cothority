package medco

import (
	"github.com/dedis/cothority/protocols/medco"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/cothority/lib/sda"
)


type SurveyCreationQuery struct {
	sda.EntityList
}

type ServiceResponse struct {
	code int16
	text string
}

type SurveyResponseData struct {
	medco.CipherVector
}

type SurveyResultsQuery struct {
	ClientPublic abstract.Point
}

type SurveyResultResponse struct {
	medco.CipherVector
}
