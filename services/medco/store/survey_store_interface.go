package store

import (
	"github.com/dedis/cothority/protocols/medco"
	"github.com/satori/go.uuid"
	"github.com/dedis/cothority/services/medco"
)

const MAX_GROUP_ATTR int = 10  //we must have this limit because slices cannot be used as keys in maps

//type ProbabilisticGroupingAttributes medco.CipherVector
//type DeterministicGroupingAttributes medco.CipherVector
//type AggregatingAttributes medco.CipherVector

type GroupingAttributes [MAX_GROUP_ATTR]medco.DeterministCipherText


/*type ProbabilisticGroupingAttributesEntry struct {
	Id uuid.UUID
	GroupingAttributes medco.CipherVector
	
}
type DeterministicGroupingAttributesEntry struct {
	Id uuid.UUID
	GroupingAttributes medco.CipherVector
}
type AggregatingAttributesEntry struct {
	Id uuid.UUID
	AggregatingAttributes medco.CipherVector
}*/

type SurveyResult struct {
	GroupingAttributes medco.CipherVector
	AggregatingAttributes medco.CipherVector
}


type SurveyStoreInterface interface {

	// Inserts a new client response in the -ClientResponses- table
	InsertClientResponse(medco_service.ClientResponse)

	// Creates two maps -ProbabilisticGroupingAttributes- [UUID]CipherVector and -AggregatingAttributes- [UUID] [MAX_GROUP_ATTR]DeterministCipher
	// and return the content of the first table
	PollProbabilisticGroupingAttributes()  *map[uuid.UUID]medco.CipherVector

	// *** The key switch converts probabilist encryption into determinist encryption ***

	// Pushes the join between the map given as argument with the -AggregatingAttributes- map in the -GroupedResponses- map
	// and performs local group-wise aggregation
	PushDeterministicGroupingAttributes(map[uuid.UUID][MAX_GROUP_ATTR]medco.DeterministCipherText)

	// Retrieves the group-wise aggregated responses
	PollLocallyAggregatedResponses()  *map[[MAX_GROUP_ATTR]medco.DeterministCipherText]medco.CipherVector

	// *** The private aggregate protocol aggregates the results among the cothority

	// Pushes the result of the private aggregate protocol in the -AggregatedResponses- map (= merge the maps)
	PushCothorityAggregatedGroups(map[[MAX_GROUP_ATTR]medco.DeterministCipherText]medco.CipherVector)

	// Creates two maps -DeterministicGroupingAttributes- [UUID] CipherVector and -AggregatedAttributes- [UUID] CipherVector (= split keyset and valueset)
	PollDeterministicGroupingAttributes() *map[uuid.UUID] [MAX_GROUP_ATTR]medco.DeterministCipherText

	// *** The key switch converts deterministic encryption into probabilistic under querier key ***

	// Pushes the join between the map given as argument and -AggregatedAttributes- in the -DeliverableResults- table (= reconstruct the map and merge it)
	PushQuerierKeyEncryptedGroupingAttributes(map[uuid.UUID] [MAX_GROUP_ATTR]medco.DeterministCipherText)

	// Retrieve the -DeliverableResults- table
	PollDeliverableResults()[]SurveyResult
}
