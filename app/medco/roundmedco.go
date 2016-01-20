package main

import (
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/sign"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/random"	
	"github.com/dedis/crypto/nist"
	"fmt"
	"strconv"
	"encoding/binary"
	//"time"
)

/*
RoundMedco is a bare-bones round implementation to be copy-pasted. It
already implements RoundStruct for your convenience.
*/

// The name type of this round implementation
const RoundMedcoType = "medco"


type RoundMedco struct {
	*sign.RoundStruct
	//suite nist
	Name int

	compare int
	bucket int

	numBuckets int
	
	QueryM abstract.Point
	EphemeralM abstract.Point
	
	QueryT1 abstract.Point
	EphemeralT1 abstract.Point

	QueryT2 abstract.Point
	EphemeralT2 abstract.Point

	numMatches int
	numMismatches int

	// root
	PublicRoot abstract.Point 	
	PrivateRoot abstract.Secret 
	// leaf
	PublicLeaf abstract.Point 	
	PrivateLeaf abstract.Secret 
	// intermediate
	PublicMid abstract.Point 	
	PrivateMid abstract.Secret
 
	// collective
	CollectivePublic abstract.Point
	CollectivePrivate abstract.Secret

	// root
	FreshPublicRoot abstract.Point 	
	FreshPrivateRoot abstract.Secret 
	// leaf
	FreshPublicLeaf abstract.Point 	
	FreshPrivateLeaf abstract.Secret 
	// intermediate
	FreshPublicMid abstract.Point 	
	FreshPrivateMid abstract.Secret 

	// collective	
	FreshCollectivePublic abstract.Point
	FreshCollectivePrivate abstract.Secret

	ClientPubKey abstract.Point

	vRoot abstract.Secret 
	vLeaf abstract.Secret
	vMid abstract.Secret	
	v abstract.Secret
	vPub abstract.Point

	bucketQuery abstract.Point
}



func (round *RoundMedco) Announcement(viewNbr, roundNbr int, in *sign.SigningMessage, out []*sign.SigningMessage) error {
	
	suite := nist.NewAES128SHA256P256() 

	switch{

	case round.IsRoot:

		numBuckets := round.numBuckets
		Message := []byte("code0")
		EphemPublicM, CipherM, _ := ElGamalEncrypt(suite, round.PublicLeaf, Message)
		
		val1, err1 := CipherM.MarshalBinary() 
		val2, err2 := EphemPublicM.MarshalBinary()
		
		if (err1 != nil || err2 != nil) {
			dbg.Fatal("Problem marshalling query (root announcement)")
		}
		val := append(val1, val2...)


		for i := 0; i < numBuckets; i++ {

			Threshold := []byte("bucket"+strconv.Itoa(i))
			EphemPublicT, CipherT, _ := ElGamalEncrypt(suite, round.PublicLeaf, Threshold)
			val3, err3 := CipherT.MarshalBinary() 
			val4, err4 := EphemPublicT.MarshalBinary()

			if (err3 != nil || err4 != nil) {
				dbg.Fatal("Problem marshalling query (root announcement)")
			}

			val = append(val,val3...)
			val = append(val,val4...)	
		}
		
		for _, o := range out {
			o.Am.Message = val 
		}
		
	default:
		
		for o ,_ := range out {
			out[o].Am = in.Am
		}


	case round.IsLeaf:

		size := 65
		//numBuckets := round.numBuckets

		//l := len(in.Am.Message)

		CipherM := in.Am.Message[0:size]
		EphemPublicM :=  in.Am.Message[size:2*size]

		var cipherM = suite.Point()
		var ephemeralM = suite.Point()
		
		
		err1 := cipherM.UnmarshalBinary(CipherM)
		err2 := ephemeralM.UnmarshalBinary(EphemPublicM)

		if  (err1 != nil || err2 != nil) { 
			dbg.Fatal("Problem unmarshalling threshold encryption (leaf announcement)")
		}

		//start := 2*size

		//round.QueryT = []byte("")
		//round.EphemeralT = []byte("")


			CipherT1 :=  in.Am.Message[2*size:3*size]
			EphemPublicT1 :=  in.Am.Message[3*size:4*size]
			CipherT2 :=  in.Am.Message[4*size:5*size]
			EphemPublicT2 :=  in.Am.Message[5*size:6*size]

			var cipherT1 = suite.Point()
			var ephemeralT1 = suite.Point()
			var cipherT2 = suite.Point()
			var ephemeralT2 = suite.Point()

			err3 := cipherT1.UnmarshalBinary(CipherT1)
			err4 := ephemeralT1.UnmarshalBinary(EphemPublicT1)
			err5 := cipherT2.UnmarshalBinary(CipherT2)
			err6 := ephemeralT2.UnmarshalBinary(EphemPublicT2)

			if  (err3 != nil || err4 != nil || err5 != nil || err6 != nil) { 
				dbg.Fatal("Problem unmarshalling threshold encryption (leaf announcement)")
			}	

			round.QueryT1 = cipherT1 //append(round.QueryT1, cipherT1)
			round.EphemeralT1 = ephemeralT1 //append(round.EphemeralT1, ephemeralT1)
			//fmt.Println("query1",round.QueryT1)
			round.QueryT2 = cipherT2 //append(round.QueryT2, cipherT2)
			round.EphemeralT2 = ephemeralT2//append(round.EphemeralT2, ephemeralT2)
			//fmt.Println("query2",round.QueryT2)



		/*for i := 0; i < numBuckets; i++ {

			CipherT :=  in.Am.Message[start:start+size]
			EphemPublicT :=  in.Am.Message[start+size:start+2*size]

			var cipherT = suite.Point()
			var ephemeralT = suite.Point()

			err3 := cipherT.UnmarshalBinary(CipherT)
			err4 := ephemeralT.UnmarshalBinary(EphemPublicT)

			if  (err3 != nil || err4 != nil) { 
				dbg.Fatal("Problem unmarshalling threshold encryption (leaf announcement)")
			}	

			round.QueryT1 = append(round.QueryT1, cipherT)
			round.EphemeralT1 = append(round.EphemeralT1, ephemeralT1)
			fmt.Println("query",round.QueryT)

			start = start + size
		}*/
		
		round.QueryM = cipherM 
		round.EphemeralM = ephemeralM
		
	}

	return nil
}


func (round *RoundMedco) Commitment(in []*sign.SigningMessage, out *sign.SigningMessage) error {
 
	suite := nist.NewAES128SHA256P256() 

	switch{

	case round.IsRoot:

		mess := in[0].Com.Message
		//l := len(mess)
		size := 65

		var Result = suite.Point()
		var Ephem = suite.Point()
		
		err1 := Result.UnmarshalBinary(mess[0:size])
		err2 := Ephem.UnmarshalBinary(mess[size:2*size])


		if (err1 != nil || err2 != nil){
			dbg.Fatal("Problem unmarshalling result (root commitment)")
		}

		/*result, err := ElGamalDecrypt(suite, round.PrivateRoot, Ephem, Result)

		if (err != nil ){
			fmt.Println("-----herehere?")
			dbg.Fatal("Problem decrypting query (root commitment)")
		}
		
		fmt.Println("-----here?")
		fmt.Println("result",string(result))*/

		
	default:

		//for o ,_ := range out {
			out.Com.Message = in[0].Com.Message
		//}


	case round.IsLeaf:
	
		//numMessages := 10
		//numBuckets := round.numBuckets

		val := []byte("")

		decryptedQueryM, errM := ElGamalDecrypt(suite, round.PrivateLeaf, round.EphemeralM, round.QueryM)
		
		if (errM != nil){
			fmt.Println("Problem decrypting query threshold (leaf commitment)")
		}

		//start := 0
		//size := 65

		//for i := 0; i < numBuckets; i++ {
			decryptedQueryT1, errT1 := ElGamalDecrypt(suite, round.PrivateLeaf, round.EphemeralT1, round.QueryT1)
			if (errT1 != nil ){
				fmt.Println("Problem decrypting query threshold (leaf commitment)")
			}
			decryptedQueryT2, errT2 := ElGamalDecrypt(suite, round.PrivateLeaf, round.EphemeralT2, round.QueryT2)
			if (errT2 != nil ){
				fmt.Println("Problem decrypting query threshold (leaf commitment)")
			}

			fmt.Println("Are you in",string(decryptedQueryT1),"for",string(decryptedQueryM),"?")
			fmt.Println("Are you in",string(decryptedQueryT2),"for",string(decryptedQueryM),"?")
			//start = start + size
		//}


		message1 := make([]byte,8)
		message2 := make([]byte,8)
		binary.PutUvarint(message1,1)
		binary.PutUvarint(message2,0)
		Ephem1, Cipher1, _ := ElGamalEncrypt(suite, round.PublicRoot, message1)
		Ephem2, Cipher2, _ := ElGamalEncrypt(suite, round.PublicRoot, message2)

		res1, e1 := ElGamalDecrypt(suite, round.PrivateRoot, Ephem1, Cipher1)
		fmt.Println("res1",res1)

			if (e1 != nil ){
				dbg.Fatal(e1)
			}
		res2, e2 := ElGamalDecrypt(suite, round.PrivateRoot, Ephem2, Cipher2)
		fmt.Println("res2",res2)

			if (e2 != nil ){
				dbg.Fatal(e2)
			}

		SumCipher := suite.Point().Add(Cipher1,Cipher2)	
		SumEphem := suite.Point().Add(Ephem1,Ephem2)	



		res3, e3 := ElGamalDecrypt(suite, round.PrivateRoot, SumEphem, SumCipher)

			if (e3 != nil ){
				dbg.Fatal(e3)
			}
					fmt.Println("res3",res3)



		/*for i := 2; i < numMessages; i++ {

			message := make([]byte,8)
			if (i < numMessages/2) {
				//var uint64 x =1
				binary.PutUvarint(message,1)
				//message = []byte(x) 
				fmt.Println("x",message)
			} else {
				//message = []byte(strconv.Itoa(0)) 
				binary.PutUvarint(message,0)
			} 
				
			Ephem, Cipher, _ := ElGamalEncrypt(suite, round.PublicRoot, message)

			res, e := ElGamalDecrypt(suite, round.PrivateRoot, Ephem, Cipher)
			fmt.Println("res",res)

			if (e != nil ){
				fmt.Println("-----here")
				dbg.Fatal(e)
			}
		

			SumCipher = suite.Point().Add(SumCipher,Cipher)	
			
			SumEphem = suite.Point().Add(SumEphem,Ephem)		
			sum, err1 := SumCipher.MarshalBinary()	
			ephem, err2 := SumEphem.MarshalBinary()	

			if (err1 != nil || err2 != nil) {
				dbg.Fatal("Problem marshalling result (leaf commitment)")
			}	
			val = append(sum, ephem...)		
		}*/
		out.Com.Message = val
		/*fmt.Println("sum",SumCipher)

		result, err := ElGamalDecrypt(suite, round.PrivateRoot, SumEphem, SumCipher)

		if (err != nil ){
			fmt.Println("-----here")
			dbg.Fatal(err)
		}
		
		fmt.Println("result",string(result))*/

	}

	return nil
}


func (round *RoundMedco) Challenge(in *sign.SigningMessage, out []*sign.SigningMessage) error {
	return nil
}

func (round *RoundMedco) Response(in []*sign.SigningMessage, out *sign.SigningMessage) error {
	return nil
}

func (round *RoundMedco) SignatureBroadcast(in *sign.SigningMessage, out []*sign.SigningMessage) error {
	return nil
}

func ElGamalEncrypt(suite abstract.Suite, pubkey abstract.Point, message []byte) (
	K, C abstract.Point, remainder []byte) {

	// Embed the message (or as much of it as will fit) into a curve point.
	//M, remainder := suite.Point().Pick(message, random.Stream)
	// As we want to compare the encrypted points, we take a non-random stream
	M, remainder := suite.Point().Pick(message, suite.Cipher([]byte("HelloWorld")))

	B := suite.Point().Base()
	// ElGamal-encrypt the point to produce ciphertext (K,C).
	k := suite.Secret().Pick(random.Stream) // ephemeral private key
	K = suite.Point().Mul(B, k)           // ephemeral DH public key
	S := suite.Point().Mul(pubkey, k)       // ephemeral DH shared secret
	C = S.Add(S, M)                         // message blinded with secret
	return
}

func ElGamalDecrypt(suite abstract.Suite, prikey abstract.Secret, K, C abstract.Point) (
	message []byte, err error) {

	// ElGamal-decrypt the ciphertext (K,C) to reproduce the message.
	S := suite.Point().Mul(K, prikey) // regenerate shared secret
	M := suite.Point().Sub(C, S)      // use to un-blind the message
	message, err = M.Data()           // extract the embedded data
	return
}
// ./deploy -debug 2 simulation/medco.toml