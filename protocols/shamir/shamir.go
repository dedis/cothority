package main

import (
	"errors"
	"fmt"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/config"
	"github.com/dedis/crypto/edwards"
	"github.com/dedis/crypto/poly"
	"github.com/dedis/crypto/poly/promise"
	"github.com/dedis/crypto/random"
	"os"
)

var debug bool = true

/////////////// HELPERS FUNCTIONS ////////////////
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

//////////////// END OF HELPERS FUNCTIONS ///////////////

/////////////// JOINT SECRET SHARING ///////////////////

type ShamirInfo struct {
	T int // how many insurer needed to reconstruct private key
	R int // how many insurer needed to verify secret
	N int // how many insurers to split the secret into
}

// a wrapper around the info that a peers i needs to keep about a peer j != i
// he needs to know the j promise and to keep a record of its response (response made by i) of j's promise
type PeerInfo struct {
	// index in the "array" of shares (for promise j will be split into n shares, peer i's share is at index Index)
	// Often in the code there will be like for i,jp := range matrix ... : "i" is often Index, but in some case,
	// you might want to make a NxM matrix where the Index will then change.
	Index int
	// public key of the peer
	PublicKey abstract.Point
	// The promise that the peer has created and shared
	Promise *promise.Promise
	// The associated state (needed to reveal a share)
	State *promise.State
	// Peer i construct the PeerInfo struct about peer j
	// Peer i will evaluate its share on the polynomial of peer j
	// and will construct a Response object /either signature or blameproof if share is incorrect/
	// This response will have to be sent back to peer j
	Response *promise.Response

	// Schnorr part
	// Represent the r = v - c*x part in the Schnorr Signature algorithm
	// with v = random secret, c = global challenge and x private key
	ChallengeResponse *abstract.Secret
}

// JointPromiser a struct representing the data that a single must have in order to establish the distributed verifiable secret sharing
type JointPromiser struct {
	// The info about how many nodes, how many insurers needed etc
	Info ShamirInfo

	// The node base key pair
	NodeKey *config.KeyPair

	// 'promiser' role key pair
	PromiserKeyPair *config.KeyPair

	// its promise
	Promise *promise.Promise
	// its related state
	State *promise.State

	// info about the promise (and related response) for each other peer
	PeerInfos []PeerInfo

	// Distributed Public Commitment / Polynomial shared by every nodes
	DistPubPoly *poly.PubPoly
	// Distributed Private Share specific to this node.
	// Can be checked against the DistPubPoly
	DistPrivShare *abstract.Secret

	// SChnorr part challenge response
	ChallengeResponse *abstract.Secret
	// Schnorr signaure sigma ==> same for everyone
	Signature *abstract.Secret
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
			NodeKey:         secretKeys[i],
			PromiserKeyPair: promiserKeys[i],
			PeerInfos:       make([]PeerInfo, n, n)}
	}

	// create the promises of each peers i with the public key of each 'insurers' j
	for _, jp := range matrix {
		insurerList := make([]abstract.Point, n, n)
		copy(insurerList, publicPeerKeys)
		jp.Promise = new(promise.Promise).ConstructPromise(jp.PromiserKeyPair, jp.NodeKey, info.T, info.R, insurerList)
		jp.State = new(promise.State).Init(*jp.Promise)
	}

	// Then for each peer i, reference every other promises of peer j(thus the matrix)
	for i, jp := range matrix {
		for j, jpp := range matrix {
			peerData := PeerInfo{
				Index:     i,
				PublicKey: publicPeerKeys[j],
				Promise:   jpp.Promise,
				State:     jpp.State,
			}
			jp.PeerInfos[j] = peerData
		}
		//fmt.Printf("[+] Peer Data (peer %d) Promises len(%d) %+v\n", i, len(jp.PeerDatas), jp.PeerDatas[:])
		//fmt.Printf("[+] *** JointPromise[0].Promise == JointPromise[0].PeerData[0].Promise ? %+v", matrix[0].Promise.Equal(matrix[0].PeerDatas[0].Promise))
	}
	return matrix
}

func verifyMatrix(matrix []*JointPromiser) error {
	for i, jp := range matrix {
		p := jp.Promise
		for _, jpp := range matrix {
			pp := jpp.PeerInfos[i].Promise
			if !p.Equal(pp) {
				return errors.New("Promise of Peer %d is NOT equal to its PeerData[%d].Promise!")
			}
		}
	}
	return nil
}

// THis function will, for each peer i, produce the responses of the promises of all peers j
func produceResponses(promisers []*JointPromiser) error {
	for i, jp := range promisers {
		// iterate over the others promises
		for j, peerData := range jp.PeerInfos {
			// peer i produce response checking for pubpoly j at index i PUBj(i)
			resp, err := peerData.Promise.ProduceResponse(peerData.Index, jp.NodeKey)
			if err != nil {
				fmt.Printf("[+] Peer %d producing response share error (index %d,secretKey:%+v)  : %v\n", i, peerData.Index, jp.NodeKey, err)
				return err
			}
			jp.PeerInfos[j].Response = resp
		}
	}
	return nil
}

// this function will , for each peer i, add every responses of peers j to its promise to see if its verified
func verifyCertifiedMatrix(matrix []*JointPromiser) error {
	var err error = nil
	for i, jp := range matrix {
		state := jp.State
		for j, jjp := range matrix {
			resp := jjp.PeerInfos[i].Response
			err = state.AddResponse(jjp.PeerInfos[i].Index, resp)
			if err != nil {
				fmt.Printf("[-] Error while certifying matrix shares : Peer %d certifying response from peer %d (err: %v)\n", i, j, err)
				return err
			}
		}

		err = state.PromiseCertified()
		if err != nil {
			fmt.Printf("[+] Peer %d could not certified its promise : %v\n", i, err)
			return err
		}
	}
	return nil
}

// Produce the master pub polynomial which consists off adding off each PubPoly's peer
func produceDistributedPublicPoly(matrix []*JointPromiser) {
	master := matrix[0].Promise.PubPoly()
	// range over the promises of each peers
	for _, jp := range matrix[1:] {
		// Add homomorphically the different pubpoly to produce the master
		master = master.Add(master, jp.Promise.PubPoly())
	}
	for i, _ := range matrix {
		matrix[i].DistPubPoly = master
	}
}

// Produces the Sj , the respectives share of the Distributed Priv Polynomial (which is not computed by anyone)
func produceDistributedPriPolyShares(matrix []*JointPromiser) error {
	for i, jp := range matrix {
		share := suite.Secret()
		for j, peer := range jp.PeerInfos {
			sji, err := peer.State.RevealShare(peer.Index, jp.NodeKey)
			if err != nil {
				fmt.Printf("[-] Error while revealing share S(i,j) = S%d,%d : %v\n", i, j, err)
				return err
			}
			share.Add(share, sji)
		}
		jp.DistPrivShare = &share
	}
	return nil
}

// Each respective Master Share can be verified against the Master PubPoly shared by everyone
func verifyMasterShareAgainstDistPubPoly(matrix []*JointPromiser) error {
	for i, jp := range matrix {
		valid := jp.DistPubPoly.Check(i, *jp.DistPrivShare)
		if !valid {
			fmt.Printf("[-] Master Share for peer %d is not valid ! Aigh !\n", i)
			return errors.New("Verify Master Share Failed")
		}
	}
	return nil
}

type matrixFunction func(matrix []*JointPromiser) error

func exitOnError(fm matrixFunction, matrix []*JointPromiser, successMsg string) {
	if err := fm(matrix); err != nil {
		fmt.Printf("Error : %v. ABORT\n")
		os.Exit(1)
	} else {
		if debug {
			fmt.Printf(successMsg)
		}
	}
}

// Construct a matrix with a shared secret, shraded into shares amongst peers
// The shares are verifiable against the shared public polynomial
func constructJointShamirMatrix(info ShamirInfo, peerSecretKeys []*config.KeyPair) []*JointPromiser {
	// list of peer's long term public key
	publicPeerKeys := producePublicList(peerSecretKeys)

	// every peer' priv/pub "promiser role" key pair
	promiserKeys := produceKeyPairList(info.N)

	// create JointPromiser structure ==> Matrix of peers with each having the other's promises
	matrix := produceJointPromiserMatrix(peerSecretKeys, promiserKeys, publicPeerKeys, info)

	fmt.Printf("[+] Matrix of Joint Promiser constructed (%d x %d) ...\n", len(matrix), len(matrix))
	exitOnError(verifyMatrix, matrix, "[+] Matrix's construct is valid\n")
	// INSURERS PART
	exitOnError(produceResponses, matrix, "[+] Every Shares are validated by each peers ...\n")
	exitOnError(verifyCertifiedMatrix, matrix, "[+] All Peers successfully CERTIFIED their promises !\n")
	produceDistributedPublicPoly(matrix)
	if debug {
		fmt.Printf("[+] Distrubed PubPoly successfully created amongst each peer !\n")
	}
	exitOnError(produceDistributedPriPolyShares, matrix, "[+] Generated Distributed Shares for every peers! \n")
	exitOnError(verifyMasterShareAgainstDistPubPoly, matrix, "[+] Distributed Shares verified (shares of the global polynomial)\n")
	return matrix
}

type SharedPoly []*JointPromiser

// Return the distributed public polynomial of the matrix
func getDistPubPoly(matrix []*JointPromiser) *poly.PubPoly {
	return matrix[0].DistPubPoly
}

// Return a HASH of a message || Point
func hashSchnorr(suite abstract.Suite, message []byte, p abstract.Point) abstract.Secret {
	pb, _ := p.MarshalBinary()
	c := suite.Cipher(pb)
	c.Message(nil, nil, message)
	return suite.Secret().Pick(c)
}

// eval the public shared poly at point i
func evalSharedPoly(i int, poly SharedPoly) abstract.Point {
	return poly[0].DistPubPoly.Eval(i)
}

// return the share of the shared poly of peer i
func secretSharedPoly(i int, poly SharedPoly) *abstract.Secret {
	return poly[i].DistPrivShare
}

// produce the shared commitment of a schnorr response
// Yi = RandomPoly(j) + Hash(m || KeyPoly.secreetcommit) * KeyPoly(j)
func produceSchnorrResponse(keyPoly, randomPoly SharedPoly, hash abstract.Secret) {
	n := len(keyPoly)
	for i := 0; i < n; i++ {
		// take shared secrets of  both key poly + random poly
		randomSharedSecret := *secretSharedPoly(i, randomPoly)
		keySharedSecret := *secretSharedPoly(i, keyPoly)
		// compute response
		response := suite.Secret().Add(randomSharedSecret, suite.Secret().Mul(hash, keySharedSecret))
		// TODO for simplicity put that value into both structs
		keyPoly[i].ChallengeResponse = &response
		randomPoly[i].ChallengeResponse = &response
	}

	// broadcast that i.e. put that response into each peerData struct
	// again into both for simplicity
	for i := 0; i < n; i++ {
		keyPeerData := keyPoly[i].PeerInfos
		randomPeerData := randomPoly[i].PeerInfos
		for j, _ := range keyPeerData {
			keyPeerData[j].ChallengeResponse = keyPoly[j].ChallengeResponse
		}
		for j, _ := range randomPeerData {
			randomPeerData[j].ChallengeResponse = keyPoly[j].ChallengeResponse
		}
	}
}

// Populate the amtrix with the "sigma" of the schnorr signature
//func computeSchnorrSignatures(keyPoly, randomPoly SharedPoly) {
//	n := len(keyPoly)
//	// compute the coef of the lagrange interpolation
//	// l is number of peers needed to reconstruct poly
//	// j is index of the peer we are evaluating the coef for (so it gets skipped)
//	omega := func(l, j int) int64 {
//		val := 1.0
//		for i := 1; i <= l; i += 1 {
//			if i == j {
//				continue
//			}
//			val = val * ((float64(i)) / (i - j))
//		}
//		return int64(val)
//	}
//	k := keyPoly[0].DistPubPoly.GetK()
//	kr := randomPoly[0].DistPubPoly.GetK()
//	if k != kr {
//		fmt.Printf("Error : Public Poly's K (%d) differs from random Poly's K (%d)\n", k, kr)
//		os.Exit(1)
//	}
//	val := suite.Secret()
//	om := suite.Secret()
//	// compute the 4. step in stinson01provably.pdf paper
//	for i := 1; i <= k; i++ {
//		om = om.SetInt64(omega(k, i))
//		val = val.Add(*(keyPoly[i-1].ChallengeResponse), om)
//	}
//	// broadcast it
//	for i := 0; i < n; i++ {
//		keyPoly[i].Signature = &val
//		randomPoly[i].Signature = &val
//	}
//}
//
// use prishares lagrange interpolation
func computeSchnorrSignatures2(keyPoly, randomPoly SharedPoly) {
	k := keyPoly[0].DistPubPoly.GetK()
	kr := randomPoly[0].DistPubPoly.GetK()
	if k != kr {
		fmt.Printf("Error : Public Poly's K (%d) differs from random Poly's K (%d)\n", k, kr)
		os.Exit(1)
	}
	pri := poly.PriShares{}
	pri.Empty(suite, k, len(keyPoly))
	for i := range keyPoly {
		pri.SetShare(i, *keyPoly[i].ChallengeResponse)
	}
	sig := pri.Secret()
	for i := range keyPoly {
		keyPoly[i].Signature = &sig
	}
	for i := range randomPoly {
		randomPoly[i].Signature = &sig
	}
}

// verify the partial schnorr response of each peers
func verifySchnorrReponses(keyPoly, randomPoly SharedPoly, msg []byte) {
	hash := hashSchnorr(suite, msg, randomPoly[0].DistPubPoly.SecretCommit())
	// lets verify only for peer 0 ...
	for j, pi := range keyPoly[0].PeerInfos {
		// gamma * G
		left := suite.Point().Mul(suite.Point().Base(), *pi.ChallengeResponse)
		// F'(j) + H(m||V) * F(j)
		right := suite.Point().Add(evalSharedPoly(j, randomPoly), suite.Point().Mul(evalSharedPoly(j, keyPoly), hash))
		if !right.Equal(left) {
			fmt.Printf("[-] Verifying Schnorr Signature Response for Peer %d fails :/\n", j)
			os.Exit(1)
		}
	}
	fmt.Printf("[+] Verified Schnorr Signatures Response for all peers !\n")
}

// verify the signature !
func verifySharedSignature(keyPoly, randomPoly SharedPoly, msg []byte) {
	// left part of verification
	signCommitment := suite.Point().Mul(suite.Point().Base(), *(keyPoly[0].Signature))
	// right part of verification
	randomCommit := randomPoly[0].DistPubPoly.SecretCommit()
	keyCommit := keyPoly[0].DistPubPoly.SecretCommit()
	hash := hashSchnorr(suite, msg, randomCommit)
	// V + H(m||V)*Y
	right := suite.Point().Add(randomCommit, suite.Point().Mul(keyCommit, hash))
	if right.Equal(signCommitment) {
		fmt.Printf("[+] Distributed Schnorr Signature Works !!!\n")
	} else {
		fmt.Printf("[-] Error Signatures dont match :(\n")
		os.Exit(1)
	}
}

func distributedSecureSchnorrSignature() {
	fmt.Printf("########## Distributed Schnorr Test ###########\n")
	info := ShamirInfo{
		T: 5,
		R: 7,
		N: 10,
	}
	fmt.Printf("[+] Joint Shamir Configuration : T = %d, R = %d, N = %d\n", info.T, info.R, info.N)

	msg := []byte("This is the message to be signed")

	// every peer's priv/pub long term key pair
	peerSecretKeys := produceKeyPairList(info.N)
	// produce the public / private shared poloynomial
	fmt.Printf("\n\t ### PUBLIC / PRIVATE KEY Polynomial setup ###\n\n")
	keyMatrix := constructJointShamirMatrix(info, peerSecretKeys)
	// from which you can extract the commitment of the distributed secret
	//keyCommit := getDistPubPoly(keyMatrix).SecretCommit()
	// produce the random shared secret as public polynomial
	fmt.Printf("\n\t ### PUBLIC / PRIVATE Random Polynomial setup ###\n\n")
	randomMatrix := constructJointShamirMatrix(info, peerSecretKeys)
	// again, extract the commitment of the distributed secret
	//secretCommit := getDistPubPoly(randomMatrix).SecretCommit()
	fmt.Printf("\n\t ### Schnorr Signature Steps ###\n\n")
	hash := hashSchnorr(suite, msg, getDistPubPoly(randomMatrix).SecretCommit())
	produceSchnorrResponse(keyMatrix, randomMatrix, hash)
	fmt.Printf("[+] Produced Schnorr Responses for each peers!\n")
	verifySchnorrReponses(keyMatrix, randomMatrix, msg)
	computeSchnorrSignatures2(keyMatrix, randomMatrix)
	fmt.Printf("[+] Produced SIGMA of the signature & broadcasted it!\n")
	verifySharedSignature(keyMatrix, randomMatrix, msg)

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
			fmt.Printf("[-] Error while producing response share from insurers %d : %v\n", i, err)
			os.Exit(1)
		}
	}
	fmt.Printf("** Insurer  **\tProduced response share for all insurers\n")

	// Promiser part : adding responses from insurers to its promise

	for i, resp := range responses {
		err = state.AddResponse(i, resp)
		if err != nil {
			fmt.Printf("[-] Error while adding response share %d to promiser's promise : %v\n", i, err)
		}
	}
	fmt.Printf("** Promiser **\tAdded all responses to the state's promise.\n")
	err = state.PromiseCertified()
	if err != nil {
		fmt.Printf("[-] Error: Promiser's promise is not certified !!\n")
		os.Exit(1)
	} else {
		fmt.Printf("** Promiser **\tPromiser's promise IS INDEED Certified ;)\n")
	}
	// Client / Promiser part : reveal a share
	for i := 0; i < N; i++ {
		secret, err := state.RevealShare(i, insurerKeys[i])
		if err != nil {
			fmt.Printf("[-] Error while revealing private share from insurer %d : %v", i, err)
		}
		fmt.Printf("** Insurer  **\tProduced response share %d : %v\n", i, secret)
		state.PriShares.SetShare(i, secret)
	}

	reconstructedSecret := state.PriShares.Secret()
	if reconstructedSecret.Equal(secretKey.Secret) {
		fmt.Printf("** Promiser / Client ** Reconstruction of the secret succeeded !!\n")
	} else {
		fmt.Printf("** Promiser / Client ** Reconstruction of the secret failed :/\n")
		os.Exit(1)
	}

	fmt.Printf("Public Poly of Promise : %+v\n", basicPromise.PubPoly())

}

func main() {
	//singleShamirPromiseTest()
	distributedSecureSchnorrSignature()
}
