package Elgamal //should be in the general crypto project

import (
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/nist"
	"github.com/dedis/crypto/random"
)

func ElGamalEncrypt(suite abstract.Suite, pubkey abstract.Point, message []byte) (
	K, C abstract.Point, remainder []byte) {

	M, remainder := suite.Point().Pick(message, random.Stream)
	k := suite.Scalar().Pick(random.Stream) 
	K = suite.Point().Mul(nil, k)           
	S := suite.Point().Mul(pubkey, k)      
	C = S.Add(S, M)                         
	return 
}
	
	func PartialElGamalEncrypt(suite abstract.Suite, pubkey abstract.Point, M abstract.Point) (
	K, C abstract.Point, remainder []byte) {
  
	k := suite.Scalar().Pick(random.Stream) 
	K = suite.Point().Mul(nil, k)           
	S := suite.Point().Mul(pubkey, k)       
	C = S.Add(S, M)                         
	return
}

func ElGamalDecrypt(suite abstract.Suite, prikey abstract.Scalar, K, C abstract.Point) (
	message []byte, err error) {

	S := suite.Point().Mul(K, prikey) 
	M := suite.Point().Sub(C, S)     
	message, err = M.Data()         
	return
}
	
	func  PartialElGamalDecrypt(suite abstract.Suite, prikey abstract.Scalar, K, C abstract.Point) (
	M  abstract.Point, err error) {

	S := suite.Point().Mul(K, prikey) 
	M = suite.Point().Sub(C, S)      
	_ , err = M.Data()
	return
}
