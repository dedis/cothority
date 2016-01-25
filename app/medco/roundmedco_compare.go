package main

import (
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/sign"
	"github.com/dedis/crypto/nist"
	"fmt"
	"strconv"
	//"time"
)

/*
RoundMedcoCompare is a bare-bones round implementation to be copy-pasted. It
already implements RoundStruct for your convenience.
*/

// The name type of this round implementation
const RoundMedcoCompareType = "medcoCompare"


type RoundMedcoCompare struct {
	*sign.RoundStruct
	*RoundMedco
}


func (round *RoundMedcoCompare) Announcement(viewNbr, roundNbr int, in *sign.SigningMessage, out []*sign.SigningMessage) error {
	
	suite := nist.NewAES128SHA256P256() 

	switch{

	case round.IsRoot:

		Message := []byte("code0")

		var CipherM = suite.Point()
		var EphemPublicM = suite.Point()


		EphemPublicM, CipherM, _, _ = ElGamalEncrypt(suite, round.CollectivePublic, Message)
		val1, err1 := CipherM.MarshalBinary() 
		val2, err2 := EphemPublicM.MarshalBinary()
		if (err1 != nil || err2 != nil) {
			dbg.Fatal("Problem marshalling query (root announcement)")
		}

		val := append(val1, val2...)

		for _, o := range out {
			o.Am.Message = val 
		}
		
	default:
		
		for o ,_ := range out {
			out[o].Am = in.Am
		}

	case round.IsLeaf:

		size := 65

		CipherM := in.Am.Message[0:size]
		EphemPublicM :=  in.Am.Message[size:2*size]

		var cipherM = suite.Point()
		var ephemeralM = suite.Point()
		
		err1 := cipherM.UnmarshalBinary(CipherM)
		err2 := ephemeralM.UnmarshalBinary(EphemPublicM)

		if  (err1 != nil || err2 != nil ) { 
			dbg.Fatal(err1)
		}

		round.QueryM = cipherM 
		round.EphemeralM = ephemeralM
		
	}
	return nil
}


func (round *RoundMedcoCompare) Commitment(in []*sign.SigningMessage, out *sign.SigningMessage) error {
 
	suite := nist.NewAES128SHA256P256() 

	switch{

	case round.IsRoot:

		mess := in[0].Com.Message
		size := 65
		l := len(mess)
		numMessages := l/130

		var QueryM = suite.Point()
		var EphemM = suite.Point()

		err1 := QueryM.UnmarshalBinary(mess[0:size])
		err2 := EphemM.UnmarshalBinary(mess[size:2*size])
		if (err1 != nil || err2 != nil){
			dbg.Fatal("Problem unmarshalling query (middle commitment)")
		}


		tmpM := suite.Point().Mul(EphemM, round.PrivateRoot) // key
		PartialQueryM := suite.Point().Sub(QueryM,tmpM)
			
		// add Pohlig-Hellman contribution
		PH_Q := suite.Point().Add(PartialQueryM,round.FreshPublicRoot)
		

		start := 2*size

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

			if PH_C.Equal(PH_Q){
		 		round.numMatches = round.numMatches + 1
			} else {
	 			round.numMismatches = round.numMismatches + 1
	 		}
		
			
		}
		fmt.Println("--------> Number of matches:",round.numMatches)
	 	fmt.Println("--------> Number of mismatches:",round.numMismatches)


	default:
		mess := in[0].Com.Message
		size := 65
		l := len(mess)
		numMessages := l/130


		var QueryM = suite.Point()
		var EphemM = suite.Point()

		err1 := QueryM.UnmarshalBinary(mess[0:size])
		err2 := EphemM.UnmarshalBinary(mess[size:2*size])
		if (err1 != nil || err2 != nil){
			dbg.Fatal("Problem unmarshalling query (middle commitment)")
		}


		tmpM := suite.Point().Mul(EphemM, round.PrivateMid) // key
		PartialQueryM := suite.Point().Sub(QueryM,tmpM)
		
		//var ModifiedQueryM = suite.Point()
		
			
		// add Pohlig-Hellman contribution
		ModifiedQueryM := suite.Point().Add(PartialQueryM,round.FreshPublicMid)
		

		ModQueryM, err1 := ModifiedQueryM.MarshalBinary()
		ephemM, err2 := EphemM.MarshalBinary()

		if (err1 != nil || err2 != nil){
			dbg.Fatal("Problem marshalling query (leaf commitment)")
		}
		val := append(ModQueryM,ephemM...)


		start := 2*size

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
		


	case round.IsLeaf:
	
		numMessages := 10
			// remove ElGamal contribution
			tmpM := suite.Point().Mul(round.EphemeralM, round.PrivateLeaf) // key
			PartialQueryM := suite.Point().Sub(round.QueryM,tmpM)
		
			var ModifiedQueryM = suite.Point()

			// add Pohlig-Hellman contribution
			ModifiedQueryM = suite.Point().Add(PartialQueryM,round.FreshPublicLeaf)

			ModQueryM, err1 := ModifiedQueryM.MarshalBinary()
			ephemM, err2 := round.EphemeralM.MarshalBinary()

			if (err1 != nil || err2 != nil){
				dbg.Fatal("Problem marshalling query (leaf commitment)")
			}

			val := append(ModQueryM,ephemM...)


			for i := 0; i < numMessages; i++ {

				message := []byte("code"+strconv.Itoa(i)) 

				Ephem, Cipher, _, _ := ElGamalEncrypt(suite, round.CollectivePublic, message)

				// remove ElGamal contribution
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

	}

	return nil
}


func (round *RoundMedcoCompare) Challenge(in *sign.SigningMessage, out []*sign.SigningMessage) error {
	return nil
}

func (round *RoundMedcoCompare) Response(in []*sign.SigningMessage, out *sign.SigningMessage) error {
	return nil
}

func (round *RoundMedcoCompare) SignatureBroadcast(in *sign.SigningMessage, out []*sign.SigningMessage) error {
	return nil
}
