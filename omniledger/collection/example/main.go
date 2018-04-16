package main

import (
	"fmt"
	"github.com/dedis/lleap/collection"
)

func main() {

	Collection := collection.New(collection.Data{})

	/*
	 * CRUD
	 */

	// Creates a record
	Collection.Add([]byte("record"), []byte("data"))

	// Remove throws an error on non-existing keys
	err := Collection.Remove([]byte("record-nonexisting"))
	if err != nil {
		fmt.Println("Expected error (key not found):", err)
	}

	// Creates a record with "data2", then changes to "data3"
	Collection.Set([]byte("record2"), []byte("data2"))
	Collection.SetField([]byte("record2"), 0, []byte("data3"))

	// You cannot overwrite records
	err = Collection.Add([]byte("record2"), []byte("data4"))
	if err != nil {
		fmt.Println("Expected error (collision):", err)
	}

	fmt.Println("-------------")
	fmt.Println("Now fetching some existing data :")
	fmt.Println("")

	// Fetching existing data

	record, recordFetchingError := Collection.Get([]byte("record")).Record()
	recordFound := record.Match()
	data, recordNotFoundError := record.Values()

	fmt.Println("Record fetching error (doesn't indicate the record exists or not): ", recordFetchingError)
	fmt.Println("Record found:", recordFound) // can also test record == nil
	fmt.Println("Data retrieved:", string(data[0].([]byte)))
	fmt.Println("Error while fetching the record:", recordNotFoundError)

	// Fetching non-existing data

	fmt.Println("-------------")
	fmt.Println("Now fetching some non-existing data :")
	fmt.Println("")

	record, recordFetchingError = Collection.Get([]byte("nonexisting-record")).Record()
	recordFound = record.Match()
	data, recordNotFoundError = record.Values()

	fmt.Println("Record fetching error (doesn't indicate the record exists or not): ", recordFetchingError)
	fmt.Println("Record found:", recordFound) // can also test record == nil
	fmt.Println("Error while fetching the record:", recordNotFoundError)

	// the "record fetching error" happens only when the collection is able to tell whether the record exists or not;
	// this only happens when the collection is a verifier

	/*
	 * Verification
	 */

	fmt.Println("-------------")

	// Verifier needs to have the same type (collections.Data{}) as the collection
	verifier := collection.NewVerifier(collection.Data{})

	// a verifier (who does not already have "record") does not accept updates that aren't part of a Proof
	err = verifier.Add([]byte("record"), []byte("somedata"))
	if err != nil {
		fmt.Println("Expected error (unknown subtree):", err)
	}

	fmt.Println("-------------")
	fmt.Println("Now fetching some data in the verifier (who does not have any data):")

	record, recordFetchingError = verifier.Get([]byte("nonexisting-record")).Record()
	fmt.Println("Record fetching error (doesn't indicate the record exists or not): ", recordFetchingError)

	// you see the difference with collection/verifier, now recordFetchingError is set since verifier has no idea whether "nonexisting-record"
	// exists or not.

	fmt.Println("-------------")

	// let's transfer some data to the verifier
	Collection = collection.New(collection.Data{})
	verifier = collection.NewVerifier(collection.Data{})

	// proof that the record *doesn't* exist
	proof1, err := Collection.Get([]byte("record")).Proof()

	if err != nil {
		panic(err)
	}

	Collection.Add([]byte("record"), []byte("data"))

	// proof that the record does exist
	proof2, err := Collection.Get([]byte("record")).Proof()

	if err != nil {
		panic(err)
	}

	// The proof can be sent over the network:
	// buffer := Collection.Serialize(proof) // A []byte that contains a representation of proof.
	// proofagain, deserialize_err := Collection.Deserialize(buffer)

	if verifier.Verify(proof1) {
		fmt.Println("Verifier accepted the proof about \"record\".")
	} else {
		fmt.Println("Verifier did not accept")
	}

	// now the verifier is able to add something
	err = verifier.Add([]byte("record"), []byte("somedata"))
	if err != nil {
		fmt.Println(err)
	}

	if verifier.Verify(proof2) {
		fmt.Println("Verifier accepted the proof about \"record\".")
	} else {
		fmt.Println("Verifier did not accept")
	}
}
