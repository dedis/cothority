package medco

import (
	"github.com/dedis/crypto/abstract"
	_"github.com/dedis/crypto/random"
	"errors"
	"fmt"
	"github.com/dedis/crypto/random"
)


const MAX_HOMOMORPHIC_INT int64 = 300
var PointToInt map[string]int64 = make(map[string]int64, MAX_HOMOMORPHIC_INT)
var currentGreatestM abstract.Point
var currentGreatestInt int64 = 0

type CipherText struct {
	K, C abstract.Point
}

type DeterministCipherText struct {
	C abstract.Point
}

type CipherVector []CipherText

func (c *CipherText) ReplaceContribution(suite abstract.Suite, prikey abstract.Secret, shortTermPriKey abstract.Secret) {
	egContrib := suite.Point().Mul(c.K, prikey)
	phContrib := suite.Point().Mul(suite.Point().Base(), shortTermPriKey)

	c.C.Sub(c.C, egContrib)
	c.C.Add(c.C, phContrib)
}

func (c *CipherText) Add(c1, c2 CipherText) *CipherText {
	c.C.Add(c1.C,c2.C)
	c.K.Add(c1.K,c2.K)
	return c
}

func (dc *DeterministCipherText) Equals(dc2 *DeterministCipherText) bool {
	return dc2.C.Equal(dc.C)
}

func (cv *CipherVector) Add(cv1, cv2 CipherVector) error{
	if len(cv1) != len(cv2) {
		return errors.New("Cannot add CipherVectors of different lenght.")
	}
	var i int
	for i, _ = range cv1 {
		(*cv)[i].Add(cv1[i], cv2[i])
	}
	return  nil
}

func InitCipherVector(suite abstract.Suite, dim int) *CipherVector {
	cv := make(CipherVector, dim)
	for i := 0; i < dim; i++ {
		cv[i] = CipherText{suite.Point().Null(), suite.Point().Null()}
	}
	return &cv
}

func NullCipherVector(suite abstract.Suite, dim int, pubkey abstract.Point) CipherVector {
	nv := make(CipherVector, dim)
	for i := 0 ; i<dim ; i++ {
		nv[i] = *EncryptInt(suite, pubkey, 0)
	}
	return nv
}

func EncryptBytes(suite abstract.Suite, pubkey abstract.Point, message []byte) (*CipherText, error) {

	// Embed the message into a curve point.
	// As we want to compare the encrypted points, we take a non-random stream
	M, remainder := suite.Point().Pick(message, suite.Cipher([]byte("HelloWorld")))

	if len(remainder) > 0 {
		return &CipherText{nil,nil},
		errors.New(fmt.Sprintf("Message too long: %s (%d bytes too long).",string(message), len(remainder)))
	}

	return EncryptPoint(suite, pubkey, M), nil
}

func EncryptInt(suite abstract.Suite, pubkey abstract.Point, integer int64) *CipherText {

	B := suite.Point().Base()
	i := suite.Secret().SetInt64(integer)
	M := suite.Point().Mul(B, i)

	return EncryptPoint(suite, pubkey, M)
}

func EncryptPoint(suite abstract.Suite, pubkey abstract.Point, M abstract.Point) *CipherText {
	B := suite.Point().Base()
	k := suite.Secret().Pick(random.Stream) // ephemeral private key
	// ElGamal-encrypt the point to produce ciphertext (K,C).
	K := suite.Point().Mul(B, k)       // ephemeral DH public key
	S := suite.Point().Mul(pubkey, k) // ephemeral DH shared secret
	C := S.Add(S, M)                  // message blinded with secret
	return &CipherText{K, C}
}

func DecryptInt(suite abstract.Suite, prikey abstract.Secret, cipher CipherText) int64 {

	S := suite.Point().Mul(cipher.K, prikey) // regenerate shared secret
	M := suite.Point().Sub(cipher.C, S)     // use to un-blind the message

	B := suite.Point().Base()
	var Bi abstract.Point
	var m int64
	var ok bool

	if m, ok = PointToInt[M.String()]; ok {
		return m
	}

	if (currentGreatestInt == 0) {
		currentGreatestM = suite.Point().Null()
	}

	for Bi, m = currentGreatestM, currentGreatestInt; !Bi.Equal(M) && m < MAX_HOMOMORPHIC_INT; Bi, m = Bi.Add(Bi, B), m+1 {
		PointToInt[Bi.String()] = m
	}
	currentGreatestM = Bi
	PointToInt[Bi.String()] = m
	currentGreatestInt = m
	return m
}

func DecryptIntVector(suite abstract.Suite, prikey abstract.Secret, cipherVector CipherVector) []int64 {

	result := make([]int64, len(cipherVector))
	for i, c := range cipherVector {
		result[i] = DecryptInt(suite, prikey, c)
	}
	return result
}
