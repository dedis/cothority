package main

import (
	"fmt"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/config"
	"github.com/dedis/crypto/edwards"
	"github.com/dedis/crypto/poly/promise"
	"github.com/dedis/crypto/random"
	"os"
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

type ShamirInfo struct {
	T int // how many insurer needed to reconstruct private key
	R int // how many insurer needed to verify secret
	N int // how many insurers to split the secret into
}

// a wrapper around the info that a peers i needs to keep about a peer j != i
// he needs to know the j promise and to keep a record of its response (response made by i) of j's promise
type PeerData struct {
	Index    int // index in the "array" of shares (for promise j will be split into n-1 shares, peer i response has index Index in the n-1 shares)
	Promise  *promise.Promise
	Response *promise.Response
}

type JointPromiser struct {
	// long term key pair
	SecretKey *config.KeyPair

	// 'promiser' role key pair
	PromiserKeyPair *config.KeyPair

	// its promise
	Promise *promise.Promise
	// its related state
	State *promise.State

	// info about the promise (and related response) for each other peer
	PeerDatas []PeerData
}

// produce an array of JointPromiser struct that each one has the other promises ==> Matrix !
func produceJointPromiserMatrix(secretKeys, promiserKeys []*config.KeyPair, publicPeerKeys []abstract.Point, info ShamirInfo) []*JointPromiser {
	var n int = info.N
	matrix := make([]*JointPromiser, n, n)
	// Let's start to make each step separately to fully understand :
	// first init the struct
	// then construct the promise of each peer
	// then share each other's promises
	for i := 0; i < n; i++ {
		matrix[i] = &JointPromiser{
			SecretKey:       secretKeys[i],
			PromiserKeyPair: promiserKeys[i],
			PeerDatas:       make([]PeerData, n, n)}
	}

	// create the promises of each peers i with the public key of each 'insurers' j
	for _, jp := range matrix {
		insurerList := make([]abstract.Point, n, n)
		copy(insurerList, publicPeerKeys)

		//fmt.Printf("Insurerlist for peer %d : %v\n", i, insurerList)
		jp.Promise = new(promise.Promise).ConstructPromise(jp.PromiserKeyPair, jp.SecretKey, info.T, info.R, insurerList)
		jp.State = new(promise.State).Init(*jp.Promise)
		//fmt.Printf("Promise created for peer %d : %v\n", i, jp.Promise)
	}

	// Then for each peer i, reference every other promises of peer j(thus the matrix)
	for i, jp := range matrix {
		for j, jpp := range matrix {
			peerData := PeerData{
				Index:   i,
				Promise: jpp.Promise,
			}
			jp.PeerDatas[j] = peerData
		}
		//fmt.Printf("Peer Data (peer %d) Promises len(%d) %+v\n", i, len(jp.PeerDatas), jp.PeerDatas[:])
		//fmt.Printf("*** JointPromise[0].Promise == JointPromise[0].PeerData[0].Promise ? %+v", matrix[0].Promise.Equal(matrix[0].PeerDatas[0].Promise))
	}
	return matrix
}

func verifyMatrix(matrix []*JointPromiser) bool {
	for i, jp := range matrix {
		p := jp.Promise
		for _, jpp := range matrix {
			pp := jpp.PeerDatas[i].Promise
			if !p.Equal(pp) {
				return false
			}
		}
	}
	return true
}

// THis function will, for each peer i, produce the responses of the promises of all peers j
func produceSharesMatrix(promisers []*JointPromiser) {
	for i, jp := range promisers {
		// iterate over the others promises
		for j, peerData := range jp.PeerDatas {
			// peer i produce response share for pubpoly j at index i PUBj(i)
			resp, err := peerData.Promise.ProduceResponse(peerData.Index, jp.SecretKey)
			if err != nil {
				fmt.Printf("Peer %d producing response share error (index %d,secretKey:%+v)  : %v\n", i, peerData.Index, jp.SecretKey, err)
				os.Exit(1)
			}
			jp.PeerDatas[j].Response = resp
		}
	}
}

// this function will , for each peer i, add every responses of peers j to its promise to see if its verified
func certifyMatrix(matrix []*JointPromiser) {
	var err error
	for i, jp := range matrix {
		state := jp.State
		for j, jjp := range matrix {
			resp := jjp.PeerDatas[i].Response
			err = state.AddResponse(jjp.PeerDatas[i].Index, resp)
			if err != nil {
				fmt.Printf("Error while certifying matrix shares : Peer %d certifying response from peer %d (err: %v)\n", i, j, err)
			}
		}

		err = state.PromiseCertified()
		if err != nil {
			fmt.Printf("Peer %d could not certified its promise : %v\n", i, err)
			os.Exit(1)
		}
		fmt.Printf("Peer %d successfully CERTIFIED its promise !\n", i)
	}
}

func jointShamirPromiseTest() {
	fmt.Printf("########## Shamir Promiser Test ###########\n")
	info := ShamirInfo{
		T: 2,
		R: 2,
		N: 3,
	}
	fmt.Printf("Shamir Configuration : T = %d, R = %d, N = %d\n", info.T, info.R, info.N)
	// every peer's priv/pub long term key pair
	peerSecretKeys := produceKeyPairList(info.N)
	for i, pk := range peerSecretKeys {
		fmt.Printf("Long Term priv/pub key of peer %d : %+v\n", i, *pk)
	}
	// list of peer's long term public key
	publicPeerKeys := producePublicList(peerSecretKeys)
	fmt.Printf("Public Peer Key List :\n%+v\n", publicPeerKeys)

	// every peer' priv/pub "promiser role" key pair
	promiserKeys := produceKeyPairList(info.N)
	for i, pk := range promiserKeys {
		fmt.Printf("Promiser priv/pub key of peer %d : %+v\n", i, *pk)
	}

	// create JointPromiser structure ==> Matrix of peers with each having the other's promises
	matrix := produceJointPromiserMatrix(peerSecretKeys, promiserKeys, publicPeerKeys, info)

	fmt.Printf("Matrix of Joint Promiser constructed (%d x %d) ...\n", len(matrix), len(matrix))
	fmt.Printf("Is Matrix valid ? %+v\n", verifyMatrix(matrix))
	// INSURERS PART
	produceSharesMatrix(matrix)
	fmt.Printf("Matrix of Shares constructed ...\n")
	certifyMatrix(matrix)

}

func singleShamirPromiseTest() {
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
	fmt.Printf("** Promiser **\tConstructed the promise from every insurers public key \n")
	// HERE IS THE TRANSMISSION PART (marshall + unmarshal)
	// END

	// insurers part : verify promise? and produce response
	var responses = make([]*promise.Response, N, N)
	var err error
	for i := 0; i < N; i++ {
		//insurer point of view : construct response from own private key
		responses[i], err = basicPromise.ProduceResponse(i, insurerKeys[i])
		if err != nil {
			fmt.Printf("Error while producing response share from insurers %d : %v\n", i, err)
			os.Exit(1)
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
		os.Exit(1)
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
	singleShamirPromiseTest()
	jointShamirPromiseTest()
}
