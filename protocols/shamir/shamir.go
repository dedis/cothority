package main

import (
	"fmt"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/config"
	"github.com/dedis/crypto/edwards"
	"github.com/dedis/crypto/poly/promise"
	"github.com/dedis/crypto/random"
)

var suite = edwards.NewAES128SHA256Ed25519(false)

func produceKeyPair() *config.KeyPair {
	keyPair := new(config.KeyPair)
	keyPair.Gen(suite, random.Stream)
	return keyPair
}

func produceKeyPairList(nInsurers int) []*config.KeyPair {
	arr := make([]*config.KeyPair, nInsurers, nInsurers)
	for i := 0; i < nInsurers; i++ {
		arr[i] = produceKeyPair()
	}
	return arr
}

func producePublicList(insurerKeys []*config.KeyPair) []abstract.Point {
	var n int = len(insurerKeys)
	arr := make([]abstract.Point, n, n)
	for i := 0; i < n; i++ {
		arr[i] = insurerKeys[i].Public
	}
	return arr
}

type JointPromiser struct {
	// long term key pair
	secretKey *config.KeyPair

	// 'promiser' role key pair
	promiserKeyPair *config.KeyPair

	// its promise
	Promise *promise.Promise
	// its related state
	State *promise.State

	// Promises of each others peers
	PeerPromises []*promise.Promise
}

// produce an array of JointPromiser struct that each one has the other promises ==> Matrix !
func produceJointPromiserMatrix(secretKeys, promiserKeys []*config.KeyPair) []*JointPromiser {
	var n int = len(secretKeys)
	matrix := make([]*JointPromiser, n, n)
	for i := 0; i < n; i++ {
		matrix[i] = &JointPromiser{
			secretKey:       secretKeys[i],
			promiserKeyPair: promiserKeys[i],
			PeerPromises:    make([]*promise.Promise, n-1, n-1)}
	}
	return matrix
}

func shamirPromiseTest() {
	fmt.Printf("########## Shamir Promiser Test ###########\n")
	var T int = 2 // minimal to reconstruct
	var R int = 3 // minimal to verify
	var N int = 3 // number of participants

	// every peer's priv/pub long term key pair
	peerSecretKeys := produceKeyPairList(N)

	// every peer' priv/pub "promiser role" key pair
	promiserKeys := produceKeyPairList(N)
	insurerKeys := producePublicList(promiserKeys)

	// create JointPromiser structure ==> Matrix of peers with each having the other's promises
	matrix := produceJointPromiserMatrix(peerSecretKeys, promiserKeys)

	// create the promises of each peers
	for i, jp := range matrix {
		insurerList := make([]abstract.Point, N-1, N-1)
		insurerList = append(insurerList, insurerKeys[:i]...)
		insurerList = append(insurerList, insurerKeys[i+1:]...)
		jp.Promise = new(promise.Promise).ConstructPromise(jp.secretKey, jp.promiserKeyPair, T, R, insurerList)
		jp.State = new(promise.State).Init(*jp.Promise)
	}

	fmt.Printf("Matrix of Joint Promiser constructed ...\n")

}

func singlePromiseTest() {
	fmt.Printf("########## Single Promiser Test ############\n")
	// minimal number of "insurers" needed to reconstruct master secret
	var T int = 2

	// minimal number of "insurers" needed to verify master secret
	var R int = 3

	// number of insurers
	var N int = 3

	// long-term key of the "host" of the insurer
	var secretKey = produceKeyPair()

	// Promiser  key for the promise operation
	var promiserKey = produceKeyPair()

	// Produce the omniscient general setup with all public / private key pairs
	var insurerKeys = produceKeyPairList(N)

	// Produce the public view of the setup,i.e. every insurer's public key
	var insurerList = producePublicList(insurerKeys)

	// promiser part : it construct its promise, then "send" it to insurers
	var basicPromise = new(promise.Promise).ConstructPromise(secretKey, promiserKey, T, R, insurerList)
	var state = new(promise.State).Init(*basicPromise)
	fmt.Printf("** Promiser **\tConstructed the promise from every insurers public key\n")
	// HERE IS THE TRANSMISSION PART (marshall + unmarshal)
	// END

	// insurers part : verify promise? and produce response
	var responses = make([]*promise.Response, N, N)
	var err error
	for i := 0; i < N; i++ {
		//insurer point of view : construct response from own private key
		responses[i], err = basicPromise.ProduceResponse(i, insurerKeys[i])
		if err != nil {
			fmt.Printf("Error while producing response share from insurers %d : %v", i, err)
		}
	}
	fmt.Printf("** Insurer  **\tProduced response share for all insurers\n")

	// Promiser part : adding responses from insurers to its promise

	for i, resp := range responses {
		err = state.AddResponse(i, resp)
		if err != nil {
			fmt.Printf("Error while adding response share %d to promiser's promise : %v\n", i, err)
		}
	}
	fmt.Printf("** Promiser **\tAdded all responses to the state's promise.\n")
	err = state.PromiseCertified()
	if err != nil {
		fmt.Printf("Error: Promiser's promise is not certified !!\n")
	} else {
		fmt.Printf("** Promiser **\tPromiser's promise IS INDEED Certified ;)\n")
	}
	// Client / Promiser part : reveal a share
	for i := 0; i < N; i++ {
		secret, err := state.RevealShare(i, insurerKeys[i])
		if err != nil {
			fmt.Printf("Error while revealing private share from insurer %d : %v", i, err)
		}
		fmt.Printf("** Insurer  **\tProduced response share %d : %v\n", i, secret)
	}

}

func main() {
	singlePromiseTest()
	shamirPromiseTest()
}
