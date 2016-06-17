package medco_test

import (
	. "github.com/dedis/cothority/services/medco/structs"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/random"
	"reflect"
	"testing"
	"github.com/dedis/cothority/lib/network"
	"github.com/stretchr/testify/assert"
)

var suite = network.Suite

func genKeys() (secKey abstract.Secret, pubKey abstract.Point) {
	secKey = suite.Secret().Pick(random.Stream)
	pubKey = suite.Point().Mul(suite.Point().Base(), secKey)
	return
}

func TestNullCipherText(t *testing.T) {

	secKey, pubKey := genKeys()

	nullEnc := EncryptInt(suite, pubKey, 0)
	nullDec := DecryptInt(suite, secKey, *nullEnc)

	if 0 != nullDec {
		t.Fatal("Decryption of encryption of 0 should be 0, got", nullDec)
	}

	var twoTimesNullEnc = CipherText{suite.Point().Null(), suite.Point().Null()}
	twoTimesNullEnc.Add(*nullEnc, *nullEnc)
	twoTimesNullDec := DecryptInt(suite, secKey, twoTimesNullEnc)

	if 0 != nullDec {
		t.Fatal("Decryption of encryption of 0+0 should be 0, got", twoTimesNullDec)
	}

}

func TestNullCipherVector(t *testing.T) {
	secKey, pubKey := genKeys()

	nullVectEnc := NullCipherVector(suite, 10, pubKey)

	nullVectDec := DecryptIntVector(suite, secKey, nullVectEnc)

	target := []int64{0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	if !reflect.DeepEqual(nullVectDec, target) {
		t.Fatal("Null vector of dimension 4 should be ", target, "got", nullVectDec)
	}

	twoTimesNullEnc := InitCipherVector(suite, 10)
	err := twoTimesNullEnc.Add(nullVectEnc, nullVectEnc)
	twoTimesNullDec := DecryptIntVector(suite, secKey, *twoTimesNullEnc)

	if !reflect.DeepEqual(twoTimesNullDec, target) {
		t.Fatal("Null vector + Null vector should be ", target, "got", twoTimesNullDec)
	}
	if err != nil {
		t.Fatal("No error should be produced, got", err)
	}
}

func TestEqualDeterministCipherText(t *testing.T) {
	dcv1 := DeterministCipherVector{DeterministCipherText{suite.Point().Base()}, DeterministCipherText{suite.Point().Null()}}
	dcv2 := DeterministCipherVector{DeterministCipherText{suite.Point().Base()}, DeterministCipherText{suite.Point().Null()}}
	ga1 := GroupingAttributes(dcv1)
	ga2 := GroupingAttributes(dcv2)

	assert.True(t, dcv1.Equal(&dcv2))
	assert.True(t, dcv1.Equal(&dcv1))
	assert.True(t, ga1.Equal(&ga2))

	dcv1 = DeterministCipherVector{}
	dcv2 = DeterministCipherVector{}
	assert.True(t, dcv1.Equal(&dcv2))
	assert.True(t, dcv1.Equal(&dcv1))

	var nilp *DeterministCipherVector
	pdcv1 := &dcv1
	assert.True(t, pdcv1.Equal(&dcv2))
	assert.False(t, pdcv1.Equal(nilp))

	pdcv1 = nil
	assert.False(t, pdcv1.Equal(&dcv2))
	assert.True(t, pdcv1.Equal(nilp))
}
