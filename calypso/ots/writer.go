package ots

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"errors"
	"go.dedis.ch/cothority/v3/darc"
	"go.dedis.ch/kyber/v3"
	"go.dedis.ch/kyber/v3/share"
	"go.dedis.ch/kyber/v3/share/pvss"
	"go.dedis.ch/kyber/v3/suites"
	"golang.org/x/crypto/hkdf"
	"hash"
)

const KEY_LENGTH = 32
const LENGTH = KEY_LENGTH + 12

func RunPVSS(suite suites.Suite, n int, t int, pubs []kyber.Point,
	policy darc.ID) ([]*pvss.PubVerShare, *share.PubPoly, []kyber.Point,
	error) {
	//g := suite.Point().Base()
	hash := sha256.New()
	hash.Write(policy)
	// TODO: Check if this is safe
	h := suite.Point().Pick(suite.XOF(hash.Sum(nil)))
	secret := suite.Scalar().Pick(suite.RandomStream())
	shares, poly, err := pvss.EncShares(suite, h, pubs, secret, t)
	if err != nil {
		return nil, nil, nil, err
	}
	proofs := make([]kyber.Point, n)
	for i := 0; i < n; i++ {
		proofs[i] = poly.Eval(shares[i].S.I).V
	}
	return shares, poly, proofs, nil
}

func Encrypt(suite suites.Suite, s kyber.Scalar, mesg []byte) ([]byte,
	[]byte, error) {
	//secret, err := suite.Point().Mul(s, nil).MarshalBinary()
	//if err != nil {
	//	return nil, nil, err
	//}
	hash := sha256.New
	secret := suite.Point().Mul(s, nil)
	buf, err := deriveKey(hash, secret)
	key := buf[:KEY_LENGTH]
	nonce := buf[KEY_LENGTH:LENGTH]

	aes, err := aes.NewCipher(key)
	if err != nil {
		return nil, nil, err
	}
	aesgcm, err := cipher.NewGCM(aes)
	if err != nil {
		return nil, nil, err
	}
	ctxt := aesgcm.Seal(nil, nonce, mesg, nil)
	ctxtHash := sha256.Sum256(ctxt)
	return ctxt, ctxtHash[:], nil
}

func deriveKey(hash func() hash.Hash, s kyber.Point) ([]byte, error) {
	sb, err := s.MarshalBinary()
	if err != nil {
		return nil, err
	}
	hkdf := hkdf.New(hash, sb, nil, nil)
	key := make([]byte, LENGTH, LENGTH)
	n, err := hkdf.Read(key)
	if err != nil {
		return nil, err
	}
	if n < LENGTH {
		return nil, errors.New("HKDF-derived key too short")
	}
	return key, nil
}
