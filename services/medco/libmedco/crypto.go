package libmedco

import (
	"fmt"
	"github.com/dedis/cothority/network"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/random"
)

// MaxHomomorphicInt is upper bound for integers used in messages, a failed decryption will return this value.
const MaxHomomorphicInt int64 = 300

// BytesToPointEncodingSeed default seed used in bytes encoding.
const BytesToPointEncodingSeed string = "seed"

// PointToInt creates a map between EC points and integers.
var PointToInt = make(map[string]int64, MaxHomomorphicInt)
var currentGreatestM abstract.Point
var currentGreatestInt int64
var suite = network.Suite

// CipherText is an ElGamal encrypted point.
type CipherText struct {
	K, C abstract.Point
}

// CipherVector is a slice of ElGamal encrypted points.
type CipherVector []CipherText

// DeterministCipherText deterministic encryption of a point.
type DeterministCipherText struct {
	Point abstract.Point
}

// DeterministCipherVector slice of deterministic encrypted points.
type DeterministCipherVector []DeterministCipherText

// Constructors
//______________________________________________________________________________________________________________________

// NewCipherText creates a ciphertext of null elements.
func NewCipherText() *CipherText {
	return &CipherText{K: suite.Point().Null(), C: suite.Point().Null()}
}

// NewCipherVector creates a ciphervector of null elements.
func NewCipherVector(length int) *CipherVector {
	cv := make(CipherVector, length)
	for i := 0; i < length; i++ {
		cv[i] = CipherText{suite.Point().Null(), suite.Point().Null()}
	}
	return &cv
}

// NewDeterministicCipherText create determinist cipher text of null element.
func NewDeterministicCipherText() *DeterministCipherText {
	dc := DeterministCipherText{suite.Point().Null()}
	return &dc
}

// NewDeterministicCipherVector creates a vector of determinist ciphertext of null elements.
func NewDeterministicCipherVector(length int) *DeterministCipherVector {
	dcv := make(DeterministCipherVector, length)
	for i := 0; i < length; i++ {
		dcv[i] = DeterministCipherText{suite.Point().Null()}
	}
	return &dcv
}

// Encryption
//______________________________________________________________________________________________________________________

// EncryptPoint creates an elliptic curve point from an integer and encrypt it using ElGamal encryption.
func EncryptPoint(pubkey abstract.Point, M abstract.Point) *CipherText {
	B := suite.Point().Base()
	k := suite.Scalar().Pick(random.Stream) // ephemeral private key
	// ElGamal-encrypt the point to produce ciphertext (K,C).
	K := suite.Point().Mul(B, k)      // ephemeral DH public key
	S := suite.Point().Mul(pubkey, k) // ephemeral DH shared secret
	C := S.Add(S, M)                  // message blinded with secret
	return &CipherText{K, C}
}

// EncryptBytes embeds the message into a curve point with deterministic padding and encypts it using ElGamal encryption.
func EncryptBytes(pubkey abstract.Point, message []byte) (*CipherText, error) {
	// As we want to compare the encrypted points, we take a non-random stream
	M, remainder := suite.Point().Pick(message, suite.Cipher([]byte(BytesToPointEncodingSeed)))

	if len(remainder) > 0 {
		return &CipherText{nil, nil},
			fmt.Errorf("Message too long: %s (%d bytes too long).", string(message), len(remainder))
	}

	return EncryptPoint(pubkey, M), nil
}

// EncryptInt encodes i as iB, encrypt it into a CipherText and returns a pointer to it.
func EncryptInt(pubkey abstract.Point, integer int64) *CipherText {
	B := suite.Point().Base()
	i := suite.Scalar().SetInt64(integer)
	M := suite.Point().Mul(B, i)
	return EncryptPoint(pubkey, M)
}

// EncryptIntVector encrypts a []int into a CipherVector and returns a pointer to it.
func EncryptIntVector(pubkey abstract.Point, intArray []int64) *CipherVector {
	cv := make(CipherVector, len(intArray))
	for i, n := range intArray {
		cv[i] = *EncryptInt(pubkey, n)
	}
	return &cv
}

// NullCipherVector encrypts an 0-filled slice under the given public key.
func NullCipherVector(length int, pubkey abstract.Point) *CipherVector {
	return EncryptIntVector(pubkey, make([]int64, length))
}

// Decryption
//______________________________________________________________________________________________________________________

// DecryptPoint decrypts an elliptic point from an El-Gamal cipher text.
func DecryptPoint(prikey abstract.Scalar, c CipherText) abstract.Point {
	S := suite.Point().Mul(c.K, prikey) // regenerate shared secret
	M := suite.Point().Sub(c.C, S)      // use to un-blind the message
	return M
}

// DecryptInt decrypts an integer from an ElGamal cipher text where integer are encoded in the exponent.
func DecryptInt(prikey abstract.Scalar, cipher CipherText) int64 {
	M := DecryptPoint(prikey, cipher)
	return discreteLog(M)
}

// DecryptIntVector decrypts a cipherVector.
func DecryptIntVector(prikey abstract.Scalar, cipherVector *CipherVector) []int64 {
	result := make([]int64, len(*cipherVector))
	for i, c := range *cipherVector {
		result[i] = DecryptInt(prikey, c)
	}
	return result
}

// Brute-Forces the discrete log for integer decoding.
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

	for Bi, m = currentGreatestM, currentGreatestInt; !Bi.Equal(P) && m < MaxHomomorphicInt; Bi, m = Bi.Add(Bi, B), m+1 {
		PointToInt[Bi.String()] = m
	}
	currentGreatestM = Bi
	PointToInt[Bi.String()] = m
	currentGreatestInt = m
	return m
}

// Switching
//______________________________________________________________________________________________________________________

// ReplaceContribution computes the new CipherText with the old mask contribution replaced by new and save in receiver.
func (c *CipherText) ReplaceContribution(cipher *CipherText, old, new abstract.Point) *CipherText {
	c.C.Sub(cipher.C, old)
	c.C.Add(c.C, new)
	return c
}

// DeterministicSwitching performs one step in the deterministic switching process and store result in receiver.
func (c *CipherText) DeterministicSwitching(cipher *CipherText, private abstract.Scalar, phContrib abstract.Point) *CipherText {
	egContrib := suite.Point().Mul(cipher.K, private)
	c.ReplaceContribution(cipher, egContrib, phContrib)
	c.K = cipher.K
	return c
}

// DeterministicSwitching perform one step in the deterministic switching process on a vector and stores the result in receiver.
func (cv *CipherVector) DeterministicSwitching(cipher *CipherVector, private abstract.Scalar, phContrib abstract.Point) *CipherVector {
	for i, c := range *cipher {
		(*cv)[i].DeterministicSwitching(&c, private, phContrib)
	}
	return cv
}

// ProbabilisticSwitching performs one step in the Probabilistic switching process and stores result in receiver.
func (c *CipherText) ProbabilisticSwitching(cipher *CipherText, PHContrib abstract.Point, targetPublic abstract.Point) *CipherText {
	r := suite.Scalar().Pick(random.Stream)
	EGEphemContrib := suite.Point().Mul(suite.Point().Base(), r)
	EGContrib := suite.Point().Mul(targetPublic, r)
	c.ReplaceContribution(cipher, PHContrib, EGContrib)
	c.K.Add(cipher.K, EGEphemContrib)
	return c
}

// ProbabilisticSwitching performs one step in the Probabilistic switching process on a vector and stores result in receiver.
func (cv *CipherVector) ProbabilisticSwitching(cipher *CipherVector, phContrib, targetPublic abstract.Point) *CipherVector {
	for i, c := range *cipher {
		(*cv)[i].ProbabilisticSwitching(&c, phContrib, targetPublic)
	}
	return cv
}

// KeySwitching performs one step in the Key switching process and stores result in receiver.
func (c *CipherText) KeySwitching(cipher *CipherText, originalEphemeralKey, newKey abstract.Point, private abstract.Scalar) *CipherText {
	r := suite.Scalar().Pick(random.Stream)
	oldContrib := suite.Point().Mul(originalEphemeralKey, private)
	newContrib := suite.Point().Mul(newKey, r)
	ephemContrib := suite.Point().Mul(suite.Point().Base(), r)
	c.ReplaceContribution(cipher, oldContrib, newContrib)
	c.K.Add(cipher.K, ephemContrib)
	return c
}

// KeySwitching performs one step in the Key switching process on a vector and stores result in receiver.
func (cv *CipherVector) KeySwitching(cipher *CipherVector, originalEphemeralKeys *[]abstract.Point, newKey abstract.Point, private abstract.Scalar) *CipherVector {
	for i, c := range *cipher {
		(*cv)[i].KeySwitching(&c, (*originalEphemeralKeys)[i], newKey, private)
	}
	return cv
}

// Homomorphic operations
//______________________________________________________________________________________________________________________

// Add two ciphertexts and stores result in receiver.
func (c *CipherText) Add(c1, c2 CipherText) *CipherText {
	c.C.Add(c1.C, c2.C)
	c.K.Add(c1.K, c2.K)
	return c
}

// Add two ciphervectors and stores result in receiver.
func (cv *CipherVector) Add(cv1, cv2 CipherVector) *CipherVector {
	for i := range cv1 {
		(*cv)[i].Add(cv1[i], cv2[i])
	}
	return cv
}

// Sub two ciphertexts and stores result in receiver.
func (c *CipherText) Sub(c1, c2 CipherText) *CipherText {
	c.C.Sub(c1.C, c2.C)
	c.K.Sub(c1.K, c2.K)
	return c
}

// Sub two cipherVectors and stores result in receiver.
func (cv *CipherVector) Sub(cv1, cv2 CipherVector) *CipherVector {
	for i := range cv1 {
		(*cv)[i].Sub(cv1[i], cv2[i])
	}
	return cv
}

// Representation
//______________________________________________________________________________________________________________________

// Equal checks equality between deterministic ciphertexts.
func (dc *DeterministCipherText) Equal(dc2 *DeterministCipherText) bool {
	return dc2.Point.Equal(dc.Point)
}

// Equal checks equality between deterministic ciphervector.
func (dcv *DeterministCipherVector) Equal(dcv2 *DeterministCipherVector) bool {
	if dcv == nil || dcv2 == nil {
		return dcv == dcv2
	}
	for i := range *dcv2 {
		if !(*dcv)[i].Equal(&(*dcv2)[i]) {
			return false
		}
	}
	return true
}

// String representation of deterministic ciphertext.
func (dc DeterministCipherText) String() string {
	cstr := "<nil>"
	if dc.Point != nil {
		cstr = dc.Point.String()[1:4]
	}
	return fmt.Sprintf("DetCipherText{%s}", cstr)
}

// String returns a string representation of a ciphertext.
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
