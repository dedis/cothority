package store_test

import (
	"github.com/dedis/crypto/edwards"
	"github.com/dedis/crypto/random"
 	"github.com/dedis/cothority/lib/dbg"
 	"testing"
 	"fmt"
 	"github.com/dedis/cothority/protocols/medco"
 	"github.com/dedis/cothority/services/medco/store"
 	"github.com/satori/go.uuid"
)

var suite = edwards.NewAES128SHA256Ed25519(false)

func TestStoring(t *testing.T) {
	dbg.Lvl1("Test beginning")
	
	//construc variables
	secKey := suite.Secret().Pick(random.Stream)
	pubKey := suite.Point().Mul(suite.Point().Base(), secKey)
	nullEnc := medco.EncryptInt(suite, pubKey, 0)//*CipherText
	oneEnc := medco.EncryptInt(suite, pubKey, 1)//*CipherText
	
	var dnull medco.DeterministCipherText
	var done medco.DeterministCipherText
	dnull.C = nullEnc.C
	done.C = oneEnc.C//deterministic ciphertext
	
	nullVectEnc := medco.NullCipherVector(suite, 4, pubKey)//CipherVector
	_=nullVectEnc
	testCipherVect1 := make(medco.CipherVector, 4)
	for i, p := range []int64{1,2,3,6} {
		testCipherVect1[i] = *medco.EncryptInt(suite, pubKey, p)
	}
	
	testCipherVect2 := make(medco.CipherVector, 4)
	for i, p := range []int64{2,4,8,6} {
		testCipherVect2[i] = *medco.EncryptInt(suite, pubKey, p)
	}
	
	
	//constructor test
	storage := store.NewSurvey()
	_=storage
	
	//AddAggregate & GetAggregateLoc Test
	//fmt.Println("FIRST AGGREGATION")
	storage.InsertClientResponse(store.ClientResponse{nil,testCipherVect1})
	
	
	if !(len(storage.PollDeliverableResults()) == 1){
		fmt.Println("aggregation error")
		t.Errorf("aggregation error")
	} else {
		fmt.Println("aggregation OK")
		t.Logf("aggregation OK")
	}
	
	//storage.DisplayResults()
	storage.InsertClientResponse(store.ClientResponse{nil,testCipherVect2})
	//fmt.Println("SECOND AGGREGATION")
	
	
	if !(len(storage.PollDeliverableResults()) == 1){
		fmt.Println("aggregation error")
		t.Errorf("aggregation error")
	} else {
		t.Logf("second aggregation OK")
		fmt.Println("aggregation OK")
	}
	
	//storage.DisplayResults()
	
	//GROUPING
	storage = store.NewSurvey()
	storage.InsertClientResponse(store.ClientResponse{testCipherVect2,testCipherVect2})
	storage.InsertClientResponse(store.ClientResponse{testCipherVect1,testCipherVect2})
	storage.InsertClientResponse(store.ClientResponse{testCipherVect2,testCipherVect1})
	
	if !(len(storage.ClientResponses) == 3){
		fmt.Println("insertion error")
		t.Errorf("insertion error")
	} else {
		t.Logf("insertion OK")
		fmt.Println("insertion OK")
	}	
	
	probaGroups := storage.PollProbabilisticGroupingAttributes()
	//verif two maps creation -> OK
	var indexes []uuid.UUID
	for i,v := range *probaGroups{
		_=v
		//fmt.Println(i, " : ", v)
		indexes = append(indexes, i)
	}
	
	//for i,v := range storage.AggregatingAttributes{
	//	fmt.Println(i, " : ", v)
	//}
	groupedAttr := make(map[uuid.UUID][store.MAX_GROUP_ATTR]medco.DeterministCipherText)
	groupedAttr[indexes[0]] = [store.MAX_GROUP_ATTR]medco.DeterministCipherText{dnull, done}
	groupedAttr[indexes[1]] = [store.MAX_GROUP_ATTR]medco.DeterministCipherText{dnull, dnull}
	groupedAttr[indexes[2]] = [store.MAX_GROUP_ATTR]medco.DeterministCipherText{dnull, done}
	
	locRes := storage.PushDeterministicGroupingAttributes(groupedAttr)
	_=locRes
	//for i,v := range *locRes{
	//	fmt.Println(i, " : ", v)
	//}
	
	if !(len(storage.LocGroupingResults) == 2){
		fmt.Println("PushDeterministicGroupingAttributes error")
		t.Errorf("PushDeterministicGroupingAttributes error")
	} else {
		t.Logf("inserPushDeterministicGroupingAttributestion OK")
		fmt.Println("PushDeterministicGroupingAttributes OK")
	}
	
	storage.PushCothorityAggregatedGroups(storage.LocGroupingResults)
	
	if !(len(storage.LocGroupingResults) == 2){
		fmt.Println("PushCothorityAggregatedGroups error")
		t.Errorf("PushCothorityAggregatedGroups error")
	} else {
		t.Logf("PushCothorityAggregatedGroups OK")
		fmt.Println("PushCothorityAggregatedGroups OK")
	}
	
	groupedDetAttr := storage.PollDeterministicGroupingAttributes()
	if !(len(*groupedDetAttr) == 2){
		fmt.Println("PollDeterministicGroupingAttributes error")
		t.Errorf("PollDeterministicGroupingAttributes error")
	} else {
		t.Logf("PollDeterministicGroupingAttributes OK")
		fmt.Println("PollDeterministicGroupingAttributes OK")
	}
	
	var indexes1 []uuid.UUID
	for i,v := range *groupedDetAttr{
		_=v
		//fmt.Println(i, " : ", v)
		indexes1 = append(indexes1, i)
	}
	
	//for i,v := range *groupedDetAttr{
	//	fmt.Println(i, " : ", v)
	//}
	
	//for i,v := range storage.GroupedAggregatingAttributes{
	//	fmt.Println(i, " : ", v)
	//}
	
	reencrGroupAttr := make(map[uuid.UUID]medco.CipherVector)
	reencrGroupAttr[indexes1[0]] = testCipherVect2
	reencrGroupAttr[indexes1[1]] = testCipherVect1
	//reencrGroupAttr[indexes[2]] = [100]medco.DeterministCipherText{dnull, done}
	
	storage.PushQuerierKeyEncryptedGroupingAttributes(reencrGroupAttr)
	
	//storage.DisplayResults()
	
	if !(len(storage.PollDeliverableResults()) == 2){
		fmt.Println("PushQuerierKeyEncryptedGroupingAttributes error")
		t.Errorf("PushQuerierKeyEncryptedGroupingAttributes error")
	} else {
		t.Logf("PushQuerierKeyEncryptedGroupingAttributes OK")
		fmt.Println("PushQuerierKeyEncryptedGroupingAttributes OK")
	}
	
	
	
	dbg.Lvl1("... Done")
}