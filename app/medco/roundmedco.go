package main

import (
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/sign"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/random"	
	"github.com/dedis/crypto/nist"
	"fmt"
	"strconv"
	"time"
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
	
	Query abstract.Point
	Ephemeral abstract.Point
	Threshold abstract.Point

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
	
	suite := nist.NewAES128SHA256P256() //round.suite //

	switch{

	case round.IsRoot:

		Message := []byte("code0")

		EphemPublicM, CipherM, _ := ElGamalEncrypt(suite, round.CollectivePublic, Message) // no error checking for encryption?
		
		/*var CipherT = suite.Secret()
		var EphemPublicT = suite.Secret()

		if (round.bucket == 1 && round.compare == 0) {
			Threshold := []byte("greater than T")
			EphemPublicT, CipherT, _ = ElGamalEncrypt(suite, round.CollectivePublic, Threshold)	
			fmt.Println("CipherT",CipherT)
			fmt.Println("EphemPublicT",EphemPublicT)
		}

		fmt.Println("CipherT",CipherT)
		fmt.Println("EphemPublicT",EphemPublicT)*/
		
		/*val3, err3 := round.PublicLeaf.MarshalBinary()
		if (err3 != nil ) {
			dbg.Fatal("Problem sending announcement to middle layer")
		}*/

		val1, err1 := CipherM.MarshalBinary() 
		val2, err2 := EphemPublicM.MarshalBinary()
		if (err1 != nil || err2 != nil) {
			dbg.Fatal("Problem sending announcement to middle layer")
		}

		val := append(val1, val2...)
		/*if (round.bucket == 1 && round.compare == 0) {
			val3, err3 := CipherT.MarshalBinary() 
			val4, err4 := EphemPublicT.MarshalBinary()

			if (err3 != nil || err4 != nil) {
				dbg.Fatal("Problem sending announcement to middle layer")
			}
			val = append(val,val3...)
			val = append(val,val4...)
		}
*/
		//val = append(val,val3...) // send root's public key

		for _, o := range out {
			o.Am.Message = val 
		}
		
	default:
		
		for o ,_ := range out {
			out[o].Am = in.Am
		}


	case round.IsLeaf:

		//l := len(in.Am.Message)
		size := 65

		Cipher := in.Am.Message[0:size]
		EphemPublic :=  in.Am.Message[size:2*size]
		//PubKey :=  in.Am.Message[2*l/3:l]

		var cipher = suite.Point()
		var ephemeral = suite.Point()
		
		//var pubkey = suite.Point()

		err1 := cipher.UnmarshalBinary(Cipher)
		err2 := ephemeral.UnmarshalBinary(EphemPublic)

		if  (err1 != nil || err2 != nil ) { //|| err3 != nil
			dbg.Fatal(err1)
		}

		/*if (round.bucket == 1 && round.compare == 0) {
			var threshold = suite.Point()
			Threshold := in.Am.Message[2*size:3*size]	
			err3 := threshold.UnmarshalBinary(Threshold)
			
			if  (err3 != nil) { 
				dbg.Fatal(err3)
			}
			round.Threshold = threshold
		}*/

		//err3 := pubkey.UnmarshalBinary(PubKey)
		
		//fmt.Println("public key",pubkey) // put in round.pubkey?
		//round.ClientPubKey = pubkey
		round.Query = cipher 
		round.Ephemeral = ephemeral
		
	}

	return nil
}


func (round *RoundMedco) Commitment(in []*sign.SigningMessage, out *sign.SigningMessage) error {
 
	suite := nist.NewAES128SHA256P256() //round.suite //

	switch{

	case round.IsRoot:


		mess := in[0].Com.Message
		l := len(mess)
		numMessages := l/130

		var Query   = suite.Point()
		var Ephem   = suite.Point()

		err1 := Query.UnmarshalBinary(mess[0:65])
		err2 := Ephem.UnmarshalBinary(mess[65:130])

		if (err1 != nil || err2 != nil ){
				dbg.Fatal("Problem unmarshalling query (root commitment)")
		}

		// subtract the leaf's ElGamal contribution
		tmp := suite.Point().Mul(Ephem, round.PrivateRoot) // key
		PartialQuery := suite.Point().Sub(Query,tmp)
		
		var PH_Q = suite.Point()
		var ElG_Q = suite.Point()

		if (round.compare == 1 && round.bucket == 0) { 
			// add Pohlig-Hellman contribution
			PH_Q = suite.Point().Add(PartialQuery,round.FreshPublicRoot)
		} else if (round.bucket == 1 && round.compare == 0) { 
			// add ElGamal contribution with client's public key
			tmp2 := suite.Point().Mul(round.PublicLeaf,round.vRoot)
			ElG_Q = suite.Point().Add(PartialQuery,tmp2)
		} else {
			fmt.Println("Dont know what to do!!")
		}

		/*decryptedQuery, err := ElGamalDecrypt(suite, round.PrivateLeaf, round.vPub, ElG_Q)

		if (err != nil){
			fmt.Println("Problem !!!")
				dbg.Fatal(err)
		}
		fmt.Println("decrypted?",string(decryptedQuery))*/


		start := 130
		size := 65

		for i := 0; i < numMessages-1; i++ {

			cipher := mess[start:start+size]
			ephemeral := mess[start+size:start+2*size]

			var Cipher = suite.Point()
			var Ephem = suite.Point()

			err1 := Ephem.UnmarshalBinary(ephemeral)
			err2 := Cipher.UnmarshalBinary(cipher)

			if (err1 != nil || err2 != nil ){
				dbg.Fatal("Problem unmarshalling profile (root commitment)")
			}
			// subtract the leaf's ElGamal contribution
			tmp := suite.Point().Mul(Ephem, round.PrivateRoot) // key
			PartialModifiedCipher := suite.Point().Sub(Cipher,tmp) 

			// add Pohlig-Hellman contribution
 			PH_C := suite.Point().Add(PartialModifiedCipher,round.FreshPublicRoot) // key 

			start = start + 2*size

			if (round.compare == 1 && round.bucket == 0) {
				if PH_C.Equal(PH_Q){
		 			round.numMatches = round.numMatches + 1
		 		} else {
		 			round.numMismatches = round.numMismatches + 1
		 		}

		 		/*fmt.Println("-------->Number of matches:",round.numMatches)
	 			fmt.Println("-------->Number of mismatches:",round.numMismatches)*/			
			} else if (round.bucket == 1 && round.compare == 0) {
				round.bucketQuery = ElG_Q
			}
		}
		fmt.Println("--------> Number of matches:",round.numMatches)
	 	fmt.Println("--------> Number of mismatches:",round.numMismatches)


	default:


		mess := in[0].Com.Message
		l := len(mess)
		numMessages := l/130

		var Query = suite.Point()
		var Ephem = suite.Point()

		err1 := Query.UnmarshalBinary(mess[0:65])
		err2 := Ephem.UnmarshalBinary(mess[65:130])
		if (err1 != nil || err2 != nil){
			dbg.Fatal("Problem unmarshalling query (middle commitment)")
		}

		// subtract the leaf's ElGamal contribution
		tmp := suite.Point().Mul(Ephem, round.PrivateMid) // key
		PartialQuery := suite.Point().Sub(Query,tmp)
		
		var ModifiedQuery = suite.Point()

		if (round.compare == 1 && round.bucket == 0) { 
			// add Pohlig-Hellman contribution
			ModifiedQuery = suite.Point().Add(PartialQuery,round.FreshPublicMid)
		} else if (round.bucket == 1 && round.compare == 0) { 
			// add ElGamal contribution with client's public key
			tmp2 := suite.Point().Mul(round.PublicLeaf,round.vMid)
			ModifiedQuery = suite.Point().Add(PartialQuery,tmp2)
		} else {
			fmt.Println("Dont know what to do!!")
		}

		ModQuery, err3 := ModifiedQuery.MarshalBinary()
		ephem, err4 := Ephem.MarshalBinary()

		if (err3 != nil || err4 != nil){
			dbg.Fatal("Problem marshalling query (middle commitment)")
		}

		val := append(ModQuery,ephem...)


		start := 130
		size := 65


		for i := 0; i < numMessages-1; i++ {

			cipher := mess[start:start+size]
			ephemeral := mess[start+size:start+2*size]
			
			var Cipher = suite.Point()
			var Ephem = suite.Point()

			err1 := Ephem.UnmarshalBinary(ephemeral)
			err2 := Cipher.UnmarshalBinary(cipher)

			if (err1 != nil || err2 != nil ){
				dbg.Fatal("Problem unmarshalling profile (middle commitment)")
			}

			// subtract the leaf's ElGamal contribution
			tmp := suite.Point().Mul(Ephem, round.PrivateMid) // key
			PartialModifiedCipher := suite.Point().Sub(Cipher,tmp) 

			// add Pohlig-Hellman contribution
			ModifiedCipher := suite.Point().Add(PartialModifiedCipher,round.FreshPublicMid)

 			ModCipher, err3 := ModifiedCipher.MarshalBinary()
 			ephem, err4 := Ephem.MarshalBinary()

 			if (err3 != nil || err4 != nil ){
				dbg.Fatal("Problem marshalling profile (middle commitment)")
			}
			val = append(val, ModCipher...)
			val  = append(val, ephem...)

			start = start + 2*size
		}
		out.Com.Message = val 
		fmt.Println()
		


	case round.IsLeaf:
	
		numMessages := 10000

		// subtract the leaf's ElGamal contribution
		tmp := suite.Point().Mul(round.Ephemeral, round.PrivateLeaf) // key
		PartialQuery := suite.Point().Sub(round.Query,tmp)
		
		var ModifiedQuery = suite.Point()

		if (round.compare == 1 && round.bucket == 0) { 
			// add Pohlig-Hellman contribution
			ModifiedQuery = suite.Point().Add(PartialQuery,round.FreshPublicLeaf)
		} else if (round.bucket == 1 && round.compare == 0) { 
			// add ElGamal contribution with client's public key
			tmp2 := suite.Point().Mul(round.PublicLeaf,round.vLeaf)
			ModifiedQuery = suite.Point().Add(PartialQuery,tmp2)

			/*tmp2 := suite.Point().Mul(round.PublicLeaf,round.vLeaf)
			ModifiedThresh = suite.Point().Add(PartialQuery,tmp2)*/
		} else {
			fmt.Println("Dont know what to do!!")
		}

		ModQuery, err1 := ModifiedQuery.MarshalBinary()
		ephem, err2 := round.Ephemeral.MarshalBinary()

		if (err1 != nil || err2 != nil){
			dbg.Fatal("Problem marshalling query (leaf commitment)")
		}

		val := append(ModQuery,ephem...)

		start_t := time.Now()
		for i := 0; i < numMessages; i++ {

			message := []byte("code"+strconv.Itoa(i))

			Ephem, Cipher, _ := ElGamalEncrypt(suite, round.CollectivePublic, message)

			// subtract ElGamal contribution
			tmp := suite.Point().Mul(Ephem, round.PrivateLeaf)
			PartialModifiedCipher := suite.Point().Sub(Cipher,tmp)

			// add Pohlig-Hellman contribution
			ModifiedCipher := suite.Point().Add(PartialModifiedCipher,round.FreshPublicLeaf)
			
			ModCipher, err := ModifiedCipher.MarshalBinary()
			ephem, err2    := Ephem.MarshalBinary()
			if (err != nil || err2 != nil){
				dbg.Fatal("Problem marshalling profile (leaf commitment)")
			}
			val = append(val,ModCipher...)
			val = append(val,ephem...)

			
		}
		out.Com.Message = val
		elapsed_t := time.Since(start_t)
		fmt.Println("time to generate messages and change from ELG to PH\n",elapsed_t)

	}

	return nil
}


func (round *RoundMedco) Challenge(in *sign.SigningMessage, out []*sign.SigningMessage) error {

	suite := nist.NewAES128SHA256P256() //round.suite //

	if (round.bucket == 1 && round.compare == 0) {
		switch{

		case round.IsRoot:

			query, err1 := round.bucketQuery.MarshalBinary()
			//ephem, err2 := round.vPub.MarshalBinary()

			if (err1 != nil ){ //|| err2 != nil
				dbg.Fatal("Problem marshalling query (root challenge)")
			}
			val := query//append(query, ephem...)

			for o ,_ := range out {

				out[o].Chm.Message = val
			}
		
		default:
			for o ,_ := range out {
				out[o].Chm = in.Chm
			}

		case round.IsLeaf:

			query := in.Chm.Message
			//l := len(query)

			var Query = suite.Point()
			//var Ephem   = suite.Point()

			err1 := Query.UnmarshalBinary(query[0:65])
			//err2 := Ephem.UnmarshalBinary(query[65:130])

			if (err1 != nil ){
				dbg.Fatal("Problem unmarshalling query (leaf challenge)")
			}
			// ElGamalDecrypt(suite, private key, ephemeral public key, cipher)
			decryptedQuery, err := ElGamalDecrypt(suite, round.PrivateLeaf, round.vPub, Query)

			if (err != nil ){
				fmt.Println("Problem decrypting query (leaf challenge)")
				dbg.Fatal(err)
			}
			fmt.Println("query",string(decryptedQuery))

	}

	
	}
	return nil
}

func (round *RoundMedco) Response(in []*sign.SigningMessage, out *sign.SigningMessage) error {
		/*if ClientCipher1 == round.Query {
			round.numMatches = round.numMatches + 1
		} else {
			round.numMismatches = round.numMismatches + 1
		}

		if ClientCipher2 == round.Query {
			round.numMatches = round.numMatches + 1
		} else {
			round.numMismatches = round.numMismatches + 1
		}
		
		//fmt.Println("Number of matches",round.numMatches)

		match := strconv.Itoa(round.numMatches) 
		//mismatch := strconv.Itoa(round.numMismatches) 

		_, err1 := ClientCipher1.MarshalBinary() 
		_, err2 := ClientCipher2.MarshalBinary() 


		if err1 != nil {
				dbg.Fatal("Problem marshalling the encryption")
			}

		if err2 != nil {
				dbg.Fatal("Problem marshalling the encryption")
			}


		_, ResultMatches, _ := ElGamalEncrypt(suite, CollPublic, []byte(match)) 
		//_, ResultMismatches, _ := ElGamalEncrypt(suite, ClientPublic, []byte(mismatch)) 


		val, err := ResultMatches.MarshalBinary() 
		if err != nil {
			dbg.Fatal("Problem sending result to middle layer")
		}
		out.Com.Message = val*/
	return nil
}

func (round *RoundMedco) SignatureBroadcast(in *sign.SigningMessage, out []*sign.SigningMessage) error {
	return nil
}

func ElGamalEncrypt(suite abstract.Suite, pubkey abstract.Point, message []byte) (
	K, C abstract.Point, remainder []byte) {

	// Embed the message (or as much of it as will fit) into a curve point.
	//M, remainder := suite.Point().Pick(message, random.Stream)
	// As we want to compare the encrypted points, we take a zero-random stream
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