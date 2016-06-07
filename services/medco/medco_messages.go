package medco_service

import (
	"github.com/dedis/cothority/protocols/medco"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/cothority/lib/sda"
)


type SurveyCreationQuery struct {
	sda.EntityList
}

type ServiceResponse struct {
	SurveyCode int32
}

type ClientResponse struct {
	ProbabilisticGroupingAttributes medco.CipherVector
	AggregatingAttributes medco.CipherVector
}

type SurveyResultsQuery struct {
	ClientPublic abstract.Point
}

type SurveyResultResponse struct {
	Vect medco.CipherVector
}
