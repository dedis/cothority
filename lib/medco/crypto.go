package medco

import (
	"errors"
	"fmt"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/random"
)

const MAX_HOMOMORPHIC_INT int64 = 300
const BYTES_TO_POINT_ENCODING_SEED string = "seed"

var PointToInt map[string]int64 = make(map[string]int64, MAX_HOMOMORPHIC_INT)
var currentGreatestM abstract.Point
var currentGreatestInt int64 = 0
var suite abstract.Suite = network.Suite

type CipherText struct {
	K, C  abstract.Point

}

type CipherVector []CipherText

type DeterministCipherText struct {
	Point abstract.Point
}


type DeterministCipherVector []DeterministCipherText


// Constructors
//______________________________________________________________________________________________________________________

func NewCipherText() *CipherText {
	return &CipherText{K: suite.Point().Null(), C: suite.Point().Null()}
}

func NewCipherVector(length int) *CipherVector {
	cv := make(CipherVector, length)
	for i := 0; i < length; i++ {
		cv[i] = CipherText{suite.Point().Null(), suite.Point().Null()}
	}
	return &cv
}

func NewDeterministicCipherText() *DeterministCipherText {
	dc := DeterministCipherText{suite.Point().Null()}
	return &dc
}

func NewDeterministicCipherVector(length int) *DeterministCipherVector {
	dcv := make(DeterministCipherVector, length)
	for i:=0; i < length; i++ {
		dcv[i] = DeterministCipherText{suite.Point().Null()}
	}
	return &dcv
}


// Encryption
//______________________________________________________________________________________________________________________

func EncryptPoint(pubkey abstract.Point, M abstract.Point) *CipherText {
	B := suite.Point().Base()
	k := suite.Secret().Pick(random.Stream) // ephemeral private key
	// ElGamal-encrypt the point to produce ciphertext (K,C).
	K := suite.Point().Mul(B, k)      // ephemeral DH public key
	S := suite.Point().Mul(pubkey, k) // ephemeral DH shared secret
	C := S.Add(S, M)                  // message blinded with secret
	return &CipherText{K, C}
}

// EncryptBytes embeds the message into a curve point.
func EncryptBytes(pubkey abstract.Point, message []byte) (*CipherText, error) {
	// As we want to compare the encrypted points, we take a non-random stream
	M, remainder := suite.Point().Pick(message, suite.Cipher([]byte(BYTES_TO_POINT_ENCODING_SEED)))

	if len(remainder) > 0 {
		return &CipherText{nil, nil},
		errors.New(fmt.Sprintf("Message too long: %s (%d bytes too long).", string(message), len(remainder)))
	}

	return EncryptPoint(pubkey, M), nil
}

// EncryptInt encodes i as iB, encrypt it into a CipherText and returns a pointer to it
func EncryptInt(pubkey abstract.Point, integer int64) *CipherText {
	B := suite.Point().Base()
	i := suite.Secret().SetInt64(integer)
	M := suite.Point().Mul(B, i)
	return EncryptPoint(pubkey, M)
}

// EncryptIntVector encrypts a []int into a CipherVector and returns a pointer to it
func EncryptIntVector(pubkey abstract.Point, intArray []int64) *CipherVector {
	cv := make(CipherVector, len(intArray))
	for i, n := range intArray {
		cv[i] = *EncryptInt(pubkey, n)
	}
	return &cv
}

// NullCipherVector encrypts an 0-filled slice under the given public key
func NullCipherVector(length int, pubkey abstract.Point) *CipherVector {
	return EncryptIntVector(pubkey, make([]int64, length))
}

// Decryption
//______________________________________________________________________________________________________________________

func DecryptPoint(prikey abstract.Secret, c CipherText) abstract.Point {
	S := suite.Point().Mul(c.K, prikey) // regenerate shared secret
	M := suite.Point().Sub(c.C, S)      // use to un-blind the message
	return M
}

func DecryptInt(prikey abstract.Secret, cipher CipherText) int64 {
	M := DecryptPoint(prikey,cipher)
	return discreteLog(M)
}

func DecryptIntVector(prikey abstract.Secret, cipherVector *CipherVector) []int64 {
	result := make([]int64, len(*cipherVector))
	for i, c := range (*cipherVector) {
		result[i] = DecryptInt(prikey, c)
	}
	return result
}

func discreteLog(P abstract.Point) int64 {
	B := suite.Point().Base()
	var Bi abstract.Point
	var m int64
	var ok bool

	if m, ok = PointToInt[P.String()]; ok {
		return m
	}

	if currentGreatestInt == 0 {
		currentGreatestM = suite.Point().Null()
	}

	for Bi, m = currentGreatestM, currentGreatestInt; !Bi.Equal(P) && m < MAX_HOMOMORPHIC_INT; Bi, m = Bi.Add(Bi, B), m+1 {
		PointToInt[Bi.String()] = m
	}
	currentGreatestM = Bi
	PointToInt[Bi.String()] = m
	currentGreatestInt = m
	return m
}

// Key Switching
//______________________________________________________________________________________________________________________

// ReplaceContribution computes the new CipherText with the old mask contribution replaced by new and save in receiver
func (c *CipherText) ReplaceContribution(cipher *CipherText, old, new abstract.Point) *CipherText {
	c.C.Sub(cipher.C, old)
	c.C.Add(cipher.C, new)
	return c
}


// DeterministicSwitching perform one step in the deterministic switching process and store result in reciever
func (c *CipherText) DeterministicSwitching(cipher *CipherText, private abstract.Secret, phContrib abstract.Point) *CipherText {
	egContrib := suite.Point().Mul(cipher.K, private)
	c.ReplaceContribution(cipher, egContrib, phContrib)
	return c
}

func (cv *CipherVector) DeterministicSwitching(cipher *CipherVector, private abstract.Secret, phContrib abstract.Point) *CipherVector {
	for i,c := range *cipher {
		(*cv)[i].DeterministicSwitching(&c, private, phContrib)
	}
	return cv
}

func (c *CipherText) ProbabilisticSwitching(cipher *CipherText, PHContrib abstract.Point, targetPublic abstract.Point) *CipherText {
	r := suite.Secret().Pick(random.Stream)
	EGEphemContrib := suite.Point().Mul(suite.Point().Base(), r)
	EGContrib := suite.Point().Mul(targetPublic, r)
	c.ReplaceContribution(cipher, PHContrib, EGContrib)
	c.K.Add(cipher.K, EGEphemContrib)
	return c
}


func (cv *CipherVector) ProbabilisticSwitching(cipher *CipherVector, phContrib, targetPublic abstract.Point) *CipherVector {
	for i,c := range *cipher {
		(*cv)[i].ProbabilisticSwitching(&c, phContrib, targetPublic)
	}
	return cv
}


func (c *CipherText) KeySwitching(cipher *CipherText, originalEphemeralKey, newKey abstract.Point, private abstract.Secret) *CipherText {
	r := suite.Secret().Pick(random.Stream)
	oldContrib := suite.Point().Mul(originalEphemeralKey, private)
	newContrib := suite.Point().Mul(newKey, r)
	ephemContrib := suite.Point().Mul(suite.Point().Base(), r)
	c.ReplaceContribution(cipher, oldContrib, newContrib)
	c.K.Add(cipher.K, ephemContrib)
	return c
}

func (cv *CipherVector) KeySwitching(cipher *CipherVector, originalEphemeralKeys *[]abstract.Point, newKey abstract.Point, private abstract.Secret) *CipherVector {
	for i, c := range *cipher {
		(*cv)[i].KeySwitching(&c, (*originalEphemeralKeys)[i], newKey, private)
	}
	return cv
}

// Homomorphic operations
//______________________________________________________________________________________________________________________

func (c *CipherText) Add(c1, c2 CipherText) *CipherText {
	c.C.Add(c1.C, c2.C)
	c.K.Add(c1.K, c2.K)
	return c
}

func (cv *CipherVector) Add(cv1, cv2 CipherVector) *CipherVector {
	for i, _ := range cv1 {
		(*cv)[i].Add(cv1[i], cv2[i])
	}
	return cv
}


func (c *CipherText) Sub(c1, c2 CipherText) *CipherText {
	c.C.Sub(c1.C, c2.C)
	c.K.Sub(c1.K, c2.K)
	return c
}

func (cv *CipherVector) Sub(cv1, cv2 CipherVector) *CipherVector {
	for i, _ := range cv1 {
		(*cv)[i].Sub(cv1[i], cv2[i])
	}
	return cv
}


// Representation
//______________________________________________________________________________________________________________________

func (dc *DeterministCipherText) Equal(dc2 *DeterministCipherText) bool {
	return dc2.Point.Equal(dc.Point)
}

func (dcv *DeterministCipherVector) Equal(dcv2 *DeterministCipherVector) bool {
	if dcv == nil || dcv2 == nil {
		return dcv == dcv2
	}
	for i, _ := range *dcv2 {
		if !(*dcv)[i].Equal(&(*dcv2)[i]) {
			return false
		}
	}
	return true
}

func (dc DeterministCipherText) String() string {
	cstr := "<nil>"
	if dc.Point != nil {
		cstr = dc.Point.String()[1:4]
	}
	return fmt.Sprintf("DetCipherText{%s}", cstr)
}

func (c CipherText) String() string {
	cstr := "nil"
	kstr := cstr
	if c.C != nil {
		cstr = c.C.String()[1:7]
	}
	if c.K != nil {
		kstr = c.K.String()[1:7]
	}
	return fmt.Sprintf("CipherText{%s,%s}", kstr, cstr)
}



