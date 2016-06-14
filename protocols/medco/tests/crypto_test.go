package medco_test

import (
	"github.com/dedis/cothority/lib/dbg"
	. "github.com/dedis/cothority/services/medco/structs"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/edwards"
	"github.com/dedis/crypto/random"
	"reflect"
	"testing"
)

var suite = edwards.NewAES128SHA256Ed25519(false)

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
	dbg.SetDebugVisible(3)
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
