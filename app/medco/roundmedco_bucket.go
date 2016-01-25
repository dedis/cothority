package main

import (
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/sign"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/nist"
	//"github.com/dedis/crypto/edwards"
	"fmt"
	"strconv"
	//"time"
)

/*
RoundMedcoBucket is a bare-bones round implementation to be copy-pasted. It
already implements RoundStruct for your convenience.
*/

// The name type of this round implementation
const RoundMedcoBucketType = "medcoBucket"
var suite = nist.NewAES128SHA256P256()

type RoundMedcoBucket struct {
	*sign.RoundStruct
	*RoundMedco

	compare int
	bucket int
	
	QueryT1 abstract.Point
	EphemeralT1 abstract.Point

	bucketQuery abstract.Point
}

func (round *RoundMedcoBucket) Announcement(viewNbr, roundNbr int, in *sign.SigningMessage, out []*sign.SigningMessage) error {
	

	switch{

	case round.IsRoot:

		Message := []byte("code0")
		EphemPublicM, CipherM, _,_ := ElGamalEncrypt(suite, round.PublicLeaf, Message)
		
		val1, err1 := CipherM.MarshalBinary() 
		val2, err2 := EphemPublicM.MarshalBinary()
		
		if (err1 != nil || err2 != nil) {
			dbg.Fatal("Problem marshalling query (root announcement)")
		}
		val := append(val1, val2...)


		i := 1
		Threshold := []byte("bucket"+strconv.Itoa(i))
		EphemPublicT, CipherT, _ ,_:= ElGamalEncrypt(suite, round.PublicLeaf, Threshold)
		val3, err3 := CipherT.MarshalBinary() 
		val4, err4 := EphemPublicT.MarshalBinary()

		if (err3 != nil || err4 != nil) {
			dbg.Fatal("Problem marshalling query (root announcement)")
		}

		val = append(val,val3...)
		val = append(val,val4...)	
		
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

		if  (err1 != nil || err2 != nil) { 
			dbg.Fatal("Problem unmarshalling threshold encryption (leaf announcement)")
		}


		CipherT1 :=  in.Am.Message[2*size:3*size]
		EphemPublicT1 :=  in.Am.Message[3*size:4*size]


		var cipherT1 = suite.Point()
		var ephemeralT1 = suite.Point()


		err3 := cipherT1.UnmarshalBinary(CipherT1)
		err4 := ephemeralT1.UnmarshalBinary(EphemPublicT1)


		if  (err3 != nil || err4 != nil ) { 
			dbg.Fatal("Problem unmarshalling threshold encryption (leaf announcement)")
		}	

		round.QueryT1 = cipherT1 
		round.EphemeralT1 = ephemeralT1 
		
		round.QueryM = cipherM 
		round.EphemeralM = ephemeralM
		
	}
	

	return nil
}


func (round *RoundMedcoBucket) Commitment(in []*sign.SigningMessage, out *sign.SigningMessage) error {
 
	 

	switch{

	case round.IsRoot:

		//mess := in[0].Com.Message
		sumCipher_b1 := suite.Point().Null()
		sumEphem_b1 := suite.Point().Null()
		sumCipher_b2 := suite.Point().Null()
		sumEphem_b2 := suite.Point().Null()
		size := 65


		for i ,_ := range in {
			
			mess := in[i].Com.Message

			var Result_b1 = suite.Point()
			var Ephem_b1 = suite.Point()

			var Result_b2 = suite.Point()
			var Ephem_b2 = suite.Point()
			
			err1 := Result_b1.UnmarshalBinary(mess[0:size])
			err2 := Ephem_b1.UnmarshalBinary(mess[size:2*size])

			err3 := Result_b2.UnmarshalBinary(mess[2*size:3*size])
			err4 := Ephem_b2.UnmarshalBinary(mess[3*size:4*size])

			if (err1 != nil || err2 != nil || err3 != nil || err4 != nil){
				dbg.Fatal("Problem unmarshalling result (root commitment)")
			}

			sumCipher_b1 = suite.Point().Add(sumCipher_b1,Result_b1)
			sumEphem_b1 = suite.Point().Add(sumEphem_b1,Ephem_b1)

			sumCipher_b2 = suite.Point().Add(sumCipher_b2,Result_b2)
			sumEphem_b2 = suite.Point().Add(sumEphem_b2,Ephem_b2)

		}


		res_b1 := ElGamalDecrypt2(suite, round.PrivateRoot, sumEphem_b1, sumCipher_b1)
		fmt.Println("----- Number of patients in bucket 0:",res_b1)
		res_b2 := ElGamalDecrypt2(suite, round.PrivateRoot, sumEphem_b2, sumCipher_b2)
		fmt.Println("----- Number of patients in bucket 1:",res_b2)


		
	default:
		
		sumCipher_b1 := suite.Point().Null()
		sumEphem_b1 := suite.Point().Null()

		sumCipher_b2 := suite.Point().Null()
		sumEphem_b2 := suite.Point().Null()

		size := 65


		for i ,_ := range in {
			
			mess := in[i].Com.Message

			var Result_b1 = suite.Point()
			var Ephem_b1 = suite.Point()

			var Result_b2 = suite.Point()
			var Ephem_b2 = suite.Point()
			
			err1 := Result_b1.UnmarshalBinary(mess[0:size])
			err2 := Ephem_b1.UnmarshalBinary(mess[size:2*size])

			err3 := Result_b2.UnmarshalBinary(mess[2*size:3*size])
			err4 := Ephem_b2.UnmarshalBinary(mess[3*size:4*size])

			if (err1 != nil || err2 != nil || err3 != nil || err4 != nil){
				dbg.Fatal("Problem unmarshalling result (root commitment)")
			}

			sumCipher_b1 = suite.Point().Add(sumCipher_b1,Result_b1)
			sumEphem_b1 = suite.Point().Add(sumEphem_b1,Ephem_b1)

			sumCipher_b2 = suite.Point().Add(sumCipher_b2,Result_b2)
			sumEphem_b2 = suite.Point().Add(sumEphem_b2,Ephem_b2)

		}
		
		cipher_b1, err5 := sumCipher_b1.MarshalBinary()
		ephem_b1, err6 := sumEphem_b1.MarshalBinary()


		cipher_b2, err7 := sumCipher_b2.MarshalBinary()
		ephem_b2, err8 := sumEphem_b2.MarshalBinary()

		if (err5 != nil || err6 != nil || err7 != nil || err8 != nil){
			dbg.Fatal("Problem marshalling profile (middle commitment)")
		}
		val := append(cipher_b1,ephem_b1...)
		val = append(val, cipher_b2...)
		val = append(val, ephem_b2...)

		out.Com.Message = val

	case round.IsLeaf:
	
		numMessages := 50

		decryptedQueryM, errM := ElGamalDecrypt(suite, round.PrivateLeaf, round.EphemeralM, round.QueryM)
		
		if (errM != nil){
			fmt.Println("Problem decrypting query threshold (leaf commitment)")
		}

		decryptedQueryT1, errT1 := ElGamalDecrypt(suite, round.PrivateLeaf, round.EphemeralT1, round.QueryT1)
		if (errT1 != nil ){
			fmt.Println("Problem decrypting query threshold (leaf commitment)")
		}

		fmt.Println("Are you in",string(decryptedQueryT1),"for",string(decryptedQueryM),"?")


		var int0 int64
		var int1 int64

		int0 = 0
		int1 = 1

		SumCipher_b1 := suite.Point().Null()
		SumEphem_b1 := suite.Point().Null()	

		SumCipher_b2 := suite.Point().Null()
		SumEphem_b2 := suite.Point().Null()	

		for i := 0; i < numMessages; i++ {

			Ephem_b1, Cipher_b1 := ElGamalEncrypt2(suite, round.PublicRoot, int0)
			SumCipher_b1 = suite.Point().Add(SumCipher_b1,Cipher_b1)
		
			SumEphem_b1 = suite.Point().Add(SumEphem_b1,Ephem_b1)	

			Ephem_b2, Cipher_b2 := ElGamalEncrypt2(suite, round.PublicRoot, int1)
			SumCipher_b2 = suite.Point().Add(SumCipher_b2,Cipher_b2)
		
			SumEphem_b2 = suite.Point().Add(SumEphem_b2,Ephem_b2)

					
		}

		cipher_b1, err1 := SumCipher_b1.MarshalBinary()
		ephem_b1, err2 := SumEphem_b1.MarshalBinary()

		cipher_b2, err3 := SumCipher_b2.MarshalBinary()
		ephem_b2, err4 := SumEphem_b2.MarshalBinary()

		if (err1 != nil || err2 != nil || err3 != nil || err4 != nil){
			dbg.Fatal("Problem marshalling profile (leaf commitment)")
		}
		val := append(cipher_b1,ephem_b1...)
		val = append(val,cipher_b2...)
		val = append(val,ephem_b2...)

		out.Com.Message = val
		/*res := ElGamalDecrypt2(suite, round.PrivateRoot, SumEphem, SumCipher)
		fmt.Println("----- Result",res)*/
	

	}

	return nil
}


func (round *RoundMedcoBucket) Challenge(in *sign.SigningMessage, out []*sign.SigningMessage) error {
	return nil
}

func (round *RoundMedcoBucket) Response(in []*sign.SigningMessage, out *sign.SigningMessage) error {
	return nil
}

func (round *RoundMedcoBucket) SignatureBroadcast(in *sign.SigningMessage, out []*sign.SigningMessage) error {
	return nil
}

