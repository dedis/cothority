package main

import (
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/random"	
	"github.com/dedis/crypto/nist"
)

type RoundMedco struct {
	QueryM     abstract.Point
	EphemeralM abstract.Point

	numMatches    int
	numMismatches int

	// root
	PublicRoot  abstract.Point
	PrivateRoot abstract.Secret
	// leaf
	PublicLeaf  abstract.Point
	PrivateLeaf abstract.Secret
	// intermediate
	PublicMid  abstract.Point
	PrivateMid abstract.Secret

	// collective
	CollectivePublic  abstract.Point
	CollectivePrivate abstract.Secret

	// root
	FreshPublicRoot  abstract.Point
	FreshPrivateRoot abstract.Secret
	// leaf
	FreshPublicLeaf  abstract.Point
	FreshPrivateLeaf abstract.Secret
	// intermediate
	FreshPublicMid  abstract.Point
	FreshPrivateMid abstract.Secret

	// collective
	FreshCollectivePublic  abstract.Point
	FreshCollectivePrivate abstract.Secret

	ClientPubKey abstract.Point

	vRoot abstract.Secret
	vLeaf abstract.Secret
	vMid  abstract.Secret
	v     abstract.Secret
	vPub  abstract.Point
}

func NewRoundMedco()*RoundMedco{
	suite := nist.NewAES128SHA256P256()

	SecretRoot := suite.Secret().Pick(suite.Cipher([]byte("Root")))
	//c, _ := SecretRoot.MarshalBinary()
	//fmt.Println("key size",len(c))
	SecretLeaf := suite.Secret().Pick(suite.Cipher([]byte("Leaf")))
	SecretMid := suite.Secret().Pick(suite.Cipher([]byte("Middle")))

	vRoot := suite.Secret().Pick(suite.Cipher([]byte("vRoot")))
	vLeaf := suite.Secret().Pick(suite.Cipher([]byte("vLeaf")))
	vMid := suite.Secret().Pick(suite.Cipher([]byte("vMiddle")))

	FreshSecretRoot := suite.Secret().Pick(suite.Cipher([]byte("Fresh_root")))
	FreshSecretLeaf := suite.Secret().Pick(suite.Cipher([]byte("Fresh_leaf")))
	FreshSecretMid := suite.Secret().Pick(suite.Cipher([]byte("Fresh_middle")))

	PubRoot := suite.Point().Mul(nil, SecretRoot)
	PubLeaf := suite.Point().Mul(nil, SecretLeaf)
	PubMid := suite.Point().Mul(nil, SecretMid)

	FreshPubRoot := suite.Point().Mul(nil, FreshSecretRoot)
	FreshPubLeaf := suite.Point().Mul(nil, FreshSecretLeaf)
	FreshPubMid := suite.Point().Mul(nil, FreshSecretMid)

	numMidNodes := 1

	collectiveSecret := suite.Secret().Add(SecretRoot, SecretLeaf)
	FreshCollectiveSecret := suite.Secret().Add(FreshSecretRoot, FreshSecretLeaf)
	v := suite.Secret().Add(vRoot, vLeaf)

	for i := 0; i < numMidNodes; i++ {
		collectiveSecret = suite.Secret().Add(collectiveSecret, SecretMid)
		FreshCollectiveSecret = suite.Secret().Add(FreshCollectiveSecret, FreshSecretMid)
		v = suite.Secret().Add(v, vMid)
	}

	vPub := suite.Point().Mul(nil, v)

	roundMedcoBase := &RoundMedco{
		PrivateRoot: SecretRoot,
		PrivateLeaf: SecretLeaf,
		PrivateMid:  SecretMid,
	}
	// individual keys
	roundMedcoBase.vRoot = vRoot
	roundMedcoBase.vLeaf = vLeaf
	roundMedcoBase.vMid = vMid

	roundMedcoBase.v = v
	roundMedcoBase.vPub = vPub

	roundMedcoBase.PublicRoot = PubRoot
	roundMedcoBase.PublicLeaf = PubLeaf
	roundMedcoBase.PublicMid = PubMid

	//roundMedcoBase.suite = suite

	// collective keys
	roundMedcoBase.CollectivePrivate = collectiveSecret
	roundMedcoBase.CollectivePublic = suite.Point().Mul(nil, collectiveSecret)

	// fresh keys
	roundMedcoBase.FreshPrivateRoot = FreshSecretRoot
	roundMedcoBase.FreshPrivateLeaf = FreshSecretLeaf
	roundMedcoBase.FreshPrivateMid = FreshSecretMid

	roundMedcoBase.FreshPublicRoot = FreshPubRoot
	roundMedcoBase.FreshPublicLeaf = FreshPubLeaf
	roundMedcoBase.FreshPublicMid = FreshPubMid

	// fresh collective keys
	roundMedcoBase.FreshCollectivePrivate = FreshCollectiveSecret
	roundMedcoBase.FreshCollectivePublic = suite.Point().Mul(nil, FreshCollectiveSecret)
	
	return roundMedcoBase
}

func ElGamalEncrypt2(suite abstract.Suite, pubkey abstract.Point, integer int64) (
	K, C abstract.Point) {

	B := suite.Point().Base()
	i := suite.Secret().SetInt64(integer)
	M := suite.Point().Mul(B, i)

	// ElGamal-encrypt the point to produce ciphertext (K,C).
	k := suite.Secret().Pick(random.Stream) // ephemeral private key
	K = suite.Point().Mul(B, k)             // ephemeral DH public key

	S := suite.Point().Mul(pubkey, k) // ephemeral DH shared secret
	C = S.Add(S, M)                   // message blinded with secret
	return
}

func ElGamalDecrypt2(suite abstract.Suite, prikey abstract.Secret, Ephem abstract.Point, Cipher abstract.Point) (
	message int64) {

	S := suite.Point().Mul(Ephem, prikey) // regenerate shared secret
	M := suite.Point().Sub(Cipher, S)     // use to un-blind the message

	B := suite.Point().Base()
	Bi := suite.Point().Base()
	var MaxInt int64
	MaxInt = 11000

	if M.Equal(B) == true {
		message = 1
	} else if M.Equal(suite.Point().Null()) == true {
		message = 0
	} else {

		for M.Equal(Bi) == false && message < MaxInt {
			i := suite.Secret().SetInt64(message + 1)
			Bi = suite.Point().Mul(B, i) // suite.Point().Add(Bi,B)
			message = message + 1
		}

	}
	return
}

func ElGamalEncrypt(suite abstract.Suite, pubkey abstract.Point, message []byte) (
	K, C abstract.Point, remainder []byte, M abstract.Point) {

	// Embed the message (or as much of it as will fit) into a curve point.
	//M, remainder := suite.Point().Pick(message, random.Stream)
	// As we want to compare the encrypted points, we take a non-random stream
	M, remainder = suite.Point().Pick(message, suite.Cipher([]byte("HelloWorld")))

	B := suite.Point().Base()
	// ElGamal-encrypt the point to produce ciphertext (K,C).
	k := suite.Secret().Pick(random.Stream) // ephemeral private key
	//k := suite.Secret().Pick(suite.Cipher([]byte("Hello")))
	K = suite.Point().Mul(B, k)       // ephemeral DH public key
	S := suite.Point().Mul(pubkey, k) // ephemeral DH shared secret
	C = S.Add(S, M)                   // message blinded with secret
	return
}

func ElGamalDecrypt(suite abstract.Suite, prikey abstract.Secret, K, C abstract.Point) (
	message []byte, err error) {

	// ElGamal-decrypt the ciphertext (K,C) to reproduce the message.
	S := suite.Point().Mul(K, prikey) // regenerate shared secret
	M := suite.Point().Sub(C, S)      // use to un-blind the message
	//fmt.Println("M",M)
	message, err = M.Data() // extract the embedded data
	return
}

// ./deploy -debug 2 simulation/medco.toml
