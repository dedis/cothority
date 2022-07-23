package pq

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"errors"
	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/kyber/v3"
	"go.dedis.ch/kyber/v3/share"
	"go.dedis.ch/kyber/v3/util/random"
	"golang.org/x/crypto/hkdf"
	"hash"
)

const LENGTH = 32 + 12

func GenerateSSPoly() *share.PriPoly {
	f := 5
	t := f + 1
	priPoly := share.NewPriPoly(cothority.Suite, t, nil, cothority.Suite.RandomStream())
	return priPoly
}

//func NewWrite()

func GenerateCommitments(priPoly *share.PriPoly, n int) ([][]byte, error) {
	if len(priPoly.Coefficients()) != n {
		return nil, errors.New("Wrong group size")
	}
	rands := make([][8]byte, n)
	for i := 0; i < n; i++ {
		random.Bytes(rands[i][:], random.New())
	}
	shares := priPoly.Shares(n)
	commitments := make([][]byte, n)
	h := sha256.New()
	for i := 0; i < n; i++ {
		shb, err := shares[i].V.MarshalBinary()
		if err != nil {
			return nil, err
		}
		h.Write(rands[i][:])
		h.Write(shb)
		commitments[i] = h.Sum(nil)
		h.Reset()
	}
	return commitments, nil
}

func Encrypt(s kyber.Scalar, mesg []byte) ([]byte, []byte, error) {
	hash := sha256.New
	buf, err := deriveKey(hash, s)
	if err != nil {
		return nil, nil, err
	}
	key := buf[:32]
	nonce := buf[32:LENGTH]

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

func deriveKey(hash func() hash.Hash, s kyber.Scalar) ([]byte, error) {
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
