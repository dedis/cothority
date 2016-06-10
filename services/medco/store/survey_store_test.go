package store_test

import (
	"github.com/dedis/crypto/random"
 	"github.com/dedis/cothority/lib/dbg"
 	"testing"
 	"fmt"
 	"github.com/dedis/cothority/services/medco/store"
	."github.com/dedis/cothority/services/medco/structs"
	"github.com/dedis/cothority/lib/network"
)

var suite = network.Suite




func TestStoring(t *testing.T) {
	dbg.Lvl1("Test beginning")
	
	//construc variables
	secKey := suite.Secret().Pick(random.Stream)
	pubKey := suite.Point().Mul(suite.Point().Base(), secKey)
	nullEnc := EncryptInt(suite, pubKey, 0)//*CipherText
	oneEnc := EncryptInt(suite, pubKey, 1)//*CipherText
	oneBEnc := EncryptInt(suite, pubKey, 1)//*CipherText

	oneEnc.SwitchToDeterministic(suite, secKey, pubKey)
	oneBEnc.SwitchToDeterministic(suite, secKey, pubKey)


	
	var dnull DeterministCipherText
	var done DeterministCipherText
	var doneB DeterministCipherText
	dnull.C = nullEnc.C
	done.C = oneEnc.C//deterministic ciphertext
	doneB.C = oneBEnc.C
	
	nullVectEnc := NullCipherVector(suite, 4, pubKey)//CipherVector
	_=nullVectEnc
	testCipherVect1 := make(CipherVector, 4)
	for i, p := range []int64{1,2,3,6} {
		testCipherVect1[i] = *EncryptInt(suite, pubKey, p)
	}
	
	testCipherVect2 := make(CipherVector, 4)
	for i, p := range []int64{2,4,8,6} {
		testCipherVect2[i] = *EncryptInt(suite, pubKey, p)
	}
	
	
	//constructor test
	storage := store.NewSurvey()
	_=storage
	
	//AddAggregate & GetAggregateLoc Test
	//fmt.Println("FIRST AGGREGATION")
	storage.InsertClientResponse(ClientResponse{nil,testCipherVect1})
	
	
	if !(len(storage.PollDeliverableResults()) == 1){
		fmt.Println("aggregation error")
		t.Errorf("aggregation error")
	} else {
		fmt.Println("aggregation OK")
		t.Logf("aggregation OK")
	}
	
	//storage.DisplayResults()
	storage.InsertClientResponse(ClientResponse{nil,testCipherVect2})
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
	storage.InsertClientResponse(ClientResponse{testCipherVect2,testCipherVect2})
	storage.InsertClientResponse(ClientResponse{testCipherVect1,testCipherVect2})
	storage.InsertClientResponse(ClientResponse{testCipherVect2,testCipherVect1})
	//storage.InsertClientResponse(ClientResponse{testCipherVect2,testCipherVect1})
	
	if !(len(storage.ClientResponses) == 3){
		fmt.Println("insertion error")
		t.Errorf("insertion error")
	} else {
		t.Logf("insertion OK")
		fmt.Println("insertion OK")
	}	
	
	probaGroups := storage.PollProbabilisticGroupingAttributes()
	//verif two maps creation -> OK
	var indexes []TempID
	for i,v := range *probaGroups{
		_=v
		//fmt.Println(i, " : ", v)
		indexes = append(indexes, i)
	}
	
	//for i,v := range storage.AggregatingAttributes{
	//	fmt.Println(i, " : ", v)
	//}
	groupedAttr := make(map[TempID]GroupingAttributes)
	groupedAttr[indexes[0]] = [MAX_GROUP_ATTR]DeterministCipherText{dnull, done}
	groupedAttr[indexes[1]] = [MAX_GROUP_ATTR]DeterministCipherText{dnull, dnull}
	groupedAttr[indexes[2]] = [MAX_GROUP_ATTR]DeterministCipherText{dnull, doneB}
	//groupedAttr[indexes[3]] = [MAX_GROUP_ATTR]DeterministCipherText{dnull, doneB}

	storage.PushDeterministicGroupingAttributes(groupedAttr)
	//for i,v := range *locRes{
	//	fmt.Println(i, " : ", v)
	//}
	
	if !(len(storage.LocGroupingAggregating) == 2){
		fmt.Println("PushDeterministicGroupingAttributes error")
		t.Errorf("PushDeterministicGroupingAttributes error")
	} else {
		t.Logf("PushDeterministicGroupingAttributestion OK")
		fmt.Println("PushDeterministicGroupingAttributes OK")
	}
	
	storage.PushCothorityAggregatedGroups(storage.LocGroupingGroups,storage.LocGroupingAggregating)
	
	if !(len(storage.LocGroupingAggregating) == 2){
		fmt.Println("PushCothorityAggregatedGroups error")
		t.Errorf("PushCothorityAggregatedGroups error")
	} else {
		t.Logf("PushCothorityAggregatedGroups OK")
		fmt.Println("PushCothorityAggregatedGroups OK")
	}
	
	groupedDetAttr, aggrAttr := storage.PollCothorityAggregatedGroups()
	if !(len(*groupedDetAttr) == 2){
		fmt.Println("PollDeterministicGroupingAttributes error")
		t.Errorf("PollDeterministicGroupingAttributes error")
	} else {
		t.Logf("PollDeterministicGroupingAttributes OK")
		fmt.Println("PollDeterministicGroupingAttributes OK")
	}
	
	var indexes1 []TempID
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
	
	reencrGroupAttr := make(map[TempID]CipherVector)
	reencrGroupAttr[indexes1[0]] = testCipherVect2
	reencrGroupAttr[indexes1[1]] = testCipherVect1
	//reencrGroupAttr[indexes[2]] = [100]medco.DeterministCipherText{dnull, done}
	
	storage.PushQuerierKeyEncryptedData(reencrGroupAttr, *aggrAttr)
	
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