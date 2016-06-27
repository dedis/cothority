package medco_test

import (
	. "github.com/dedis/cothority/lib/medco"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/random"
	"reflect"
	"testing"
	"github.com/dedis/cothority/lib/network"
	"github.com/stretchr/testify/assert"
)

var suite = network.Suite

func genKey() (secKey abstract.Secret, pubKey abstract.Point) {
	secKey = suite.Secret().Pick(random.Stream)
	pubKey = suite.Point().Mul(suite.Point().Base(), secKey)
	return
}

func genKeys(n int) (abstract.Point, []abstract.Secret, []abstract.Point) {
	priv := make([]abstract.Secret, n)
	pub := make([]abstract.Point, n)
	group := suite.Point().Null()
	for i := 0; i < n; i ++ {
		priv[i], pub[i] = genKey()
		group.Add(group, pub[i])
	}
	return group, priv, pub
}

func TestNullCipherText(t *testing.T) {

	secKey, pubKey := genKey()

	nullEnc := EncryptInt(pubKey, 0)
	nullDec := DecryptInt(secKey, *nullEnc)

	if 0 != nullDec {
		t.Fatal("Decryption of encryption of 0 should be 0, got", nullDec)
	}

	var twoTimesNullEnc = CipherText{suite.Point().Null(), suite.Point().Null()}
	twoTimesNullEnc.Add(*nullEnc, *nullEnc)
	twoTimesNullDec := DecryptInt(secKey, twoTimesNullEnc)

	if 0 != nullDec {
		t.Fatal("Decryption of encryption of 0+0 should be 0, got", twoTimesNullDec)
	}

}

func TestNullCipherVector(t *testing.T) {
	secKey, pubKey := genKey()

	nullVectEnc := *NullCipherVector(10, pubKey)

	nullVectDec := DecryptIntVector(secKey, &nullVectEnc)

	target := []int64{0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	if !reflect.DeepEqual(nullVectDec, target) {
		t.Fatal("Null vector of dimension 4 should be ", target, "got", nullVectDec)
	}

	twoTimesNullEnc := NewCipherVector(10)
	twoTimesNullEnc.Add(nullVectEnc, nullVectEnc)
	twoTimesNullDec := DecryptIntVector(secKey, twoTimesNullEnc)

	if !reflect.DeepEqual(twoTimesNullDec, target) {
		t.Fatal("Null vector + Null vector should be ", target, "got", twoTimesNullDec)
	}
}

func TestHomomorphicOpp(t *testing.T) {
	secKey, pubKey := genKey()

	cv1 := EncryptIntVector(pubKey, []int64{0, 1, 2, 3,100})
	cv2 := EncryptIntVector(pubKey, []int64{0, 0, 1, 100, 3})
	target := []int64{0,1,3,103,103}

	cv3 := NewCipherVector(5).Add(*cv1, *cv2)

	p := DecryptIntVector(secKey, cv3)

	assert.Equal(t, target, p)
}

func TestCryptoDeterministicSwitching(t *testing.T) {
	const N = 5

	groupKey, private, _ := genKeys(N)
	phMasterKey, _, phPrivate := genKeys(N)

	target := []int64{0,0,2,3,2,5}
	cv := EncryptIntVector(groupKey, target)

	dcv := *cv
	for n := 0; n < N; n++ {
		dcv.DeterministicSwitching(&dcv, private[n], phPrivate[n])
	}

	assert.True(t, dcv[0].C.Equal(dcv[1].C))
	assert.True(t, dcv[2].C.Equal(dcv[4].C))

	dec := suite.Point()
	for i, v := range dcv {
		dec.Sub(v.C, phMasterKey)
		need := suite.Point().Mul(suite.Point().Base(), suite.Secret().SetInt64(target[i]))
		assert.True(t, dec.Equal(need))
	}
}

func TestCryptoKeySwitching(t *testing.T) {
	const N = 5
	groupKey, privates, _ := genKeys(N)
	newPrivate, newPublic := genKey()

	target := []int64{1,2,3,4,5}
	cv := EncryptIntVector(groupKey, target)

	origEphem := make([]abstract.Point, len(*cv))
	kscv := make(CipherVector, len(*cv))
	for i, c := range *cv {
		origEphem[i] = c.K
		kscv[i].K = suite.Point().Null()
		kscv[i].C = c.C
	}

	for n := 0; n < N; n++ {
		//res := *NewCipherVector(len(kscv))
		//dbg.Printf("%#v",res)
		//res.KeySwitching(&kscv, &origEphem, newPublic, privates[n])
		//dbg.Printf("%#v", res)
		//kscv = res
		kscv.KeySwitching(&kscv, &origEphem, newPublic, privates[n])
		//dbg.Printf("%#v", kscv)
	}

	res := DecryptIntVector(newPrivate, &kscv)
	//dbg.Lvl1(res)
	assert.True(t, reflect.DeepEqual(res, target))

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
