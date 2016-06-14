package store

import (
	. "github.com/dedis/cothority/services/medco/structs"
	"github.com/satori/go.uuid"
)

type SurveyResult struct {
	GroupingAttributes    CipherVector
	AggregatingAttributes CipherVector
}

type SurveyStoreInterface interface {

	// Inserts a new client response in the -ClientResponses- table
	InsertClientResponse(ClientResponse)

	// Creates two maps -ProbabilisticGroupingAttributes- [UUID]CipherVector and -AggregatingAttributes- [UUID] [MAX_GROUP_ATTR]DeterministCipher
	// and return the content of the first table
	PollProbabilisticGroupingAttributes() *map[uuid.UUID]CipherVector

	// *** The key switch converts probabilist encryption into determinist encryption ***

	// Pushes the join between the map given as argument with the -AggregatingAttributes- map in the -GroupedResponses- map
	// and performs local group-wise aggregation
	PushDeterministicGroupingAttributes(map[uuid.UUID]GroupingAttributes)

	// Retrieves the group-wise aggregated responses
	PollLocallyAggregatedResponses() *map[GroupingKey]CipherVector

	// *** The private aggregate protocol aggregates the results among the cothority

	// Pushes the result of the private aggregate protocol in the -AggregatedResponses- map (= merge the maps)
	PushCothorityAggregatedGroups(map[GroupingKey]CipherVector)

	// Creates two maps -DeterministicGroupingAttributes- [UUID] CipherVector and -AggregatedAttributes- [UUID] CipherVector (= split keyset and valueset)
	PollDeterministicGroupingAttributes() *map[uuid.UUID]GroupingAttributes

	// *** The key switch converts deterministic encryption into probabilistic under querier key ***

	// Pushes the join between the map given as argument and -AggregatedAttributes- in the -DeliverableResults- table (= reconstruct the map and merge it)
	PushQuerierKeyEncryptedGroupingAttributes(map[uuid.UUID]GroupingAttributes)

	// Retrieve the -DeliverableResults- table
	PollDeliverableResults() []SurveyResult
}
