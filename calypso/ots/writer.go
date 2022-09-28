package ots

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"errors"
	"go.dedis.ch/cothority/v3/calypso/ots/protocol"
	"go.dedis.ch/cothority/v3/darc"
	"go.dedis.ch/kyber/v3"
	"go.dedis.ch/kyber/v3/share"
	"go.dedis.ch/kyber/v3/share/pvss"
	"go.dedis.ch/kyber/v3/suites"
	"go.dedis.ch/onet/v3/log"
	"go.dedis.ch/protobuf"
	"golang.org/x/crypto/hkdf"
	"hash"
)

const KEY_LENGTH = 32
const LENGTH = KEY_LENGTH + 12

func RunPVSS(suite suites.Suite, n int, t int, pubs []kyber.Point,
	policy darc.ID) ([]*pvss.PubVerShare, *share.PubPoly, []kyber.Point,
	kyber.Scalar, error) {
	hash := sha256.New()
	hash.Write(policy)
	// TODO: Check if this is safe
	h := suite.Point().Pick(suite.XOF(hash.Sum(nil)))
	secret := suite.Scalar().Pick(suite.RandomStream())
	shares, poly, err := pvss.EncShares(suite, h, pubs, secret, t)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	proofs := make([]kyber.Point, n)
	for i := 0; i < n; i++ {
		proofs[i] = poly.Eval(shares[i].S.I).V
	}
	return shares, poly, proofs, secret, nil
}

func Encrypt(suite suites.Suite, secret kyber.Scalar, mesg []byte) ([]byte,
	[]byte, error) {
	hash := sha256.New
	shared := suite.Point().Mul(secret, nil)
	buf, err := deriveKey(hash, shared)
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

func Decrypt(shared kyber.Point, ctxt []byte) ([]byte, error) {
	hash := sha256.New
	buf, err := deriveKey(hash, shared)
	if err != nil {
		return nil, err
	}
	key := buf[:32]
	nonce := buf[32:LENGTH]
	aes, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aesgcm, err := cipher.NewGCM(aes)
	return aesgcm.Open(nil, nonce, ctxt, nil)
}

func deriveKey(hash func() hash.Hash, shared kyber.Point) ([]byte, error) {
	sb, err := shared.MarshalBinary()
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

func ElGamalDecrypt(suite suites.Suite, sk kyber.Scalar,
	reencs []*protocol.EGP) []*pvss.PubVerShare {
	size := len(reencs)
	decShares := make([]*pvss.PubVerShare, size)
	for i := 0; i < size; i++ {
		var decSh []byte
		var tmpSh pvss.PubVerShare
		tmp := reencs[i]
		for _, C := range tmp.Cs {
			S := suite.Point().Mul(sk, tmp.K)
			decShPart := suite.Point().Sub(C, S)
			decShPartData, _ := decShPart.Data()
			decSh = append(decSh, decShPartData...)
		}
		err := protobuf.Decode(decSh, &tmpSh)
		if err != nil {
			log.Errorf("Cannot decode share")
			decShares[i] = nil
		} else {
			decShares[i] = &tmpSh
		}
	}
	return decShares
}
