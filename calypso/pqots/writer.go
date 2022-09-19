package pqots

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"errors"
	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/cothority/v3/calypso/pqots/protocol"
	"go.dedis.ch/kyber/v3"
	"go.dedis.ch/kyber/v3/share"
	"go.dedis.ch/kyber/v3/suites"
	"go.dedis.ch/kyber/v3/util/random"
	"go.dedis.ch/onet/v3/log"
	"go.dedis.ch/protobuf"
	"golang.org/x/crypto/hkdf"
	"hash"
)

const KEY_LENGTH = 32
const LENGTH = KEY_LENGTH + 12

// t = f + 1
func GenerateSSPoly(t int) *share.PriPoly {
	priPoly := share.NewPriPoly(cothority.Suite, t, nil, cothority.Suite.RandomStream())
	return priPoly
}

func GenerateCommitments(priPoly *share.PriPoly, n int) ([]*share.PriShare, [][]byte, [][]byte, error) {
	var rand [8]byte
	rands := make([][]byte, n)
	shares := priPoly.Shares(n)
	commitments := make([][]byte, n)
	h := sha256.New()
	for i := 0; i < n; i++ {
		rands[i] = make([]byte, 8)
		random.Bytes(rand[:], random.New())
		copy(rands[i], rand[:])
		shb, err := shares[i].V.MarshalBinary()
		if err != nil {
			return nil, nil, nil, err
		}
		h.Write(shb)
		h.Write(rands[i])
		commitments[i] = h.Sum(nil)
		h.Reset()
	}
	return shares, rands, commitments, nil
}

func Encrypt(s kyber.Scalar, mesg []byte) ([]byte, []byte, error) {
	hash := sha256.New
	buf, err := deriveKey(hash, s)
	if err != nil {
		return nil, nil, err
	}
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

func Decrypt(s kyber.Scalar, ctxt []byte) ([]byte, error) {
	hash := sha256.New
	buf, err := deriveKey(hash, s)
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

func ElGamalDecrypt(suite suites.Suite, sk kyber.Scalar,
	reencs []*protocol.EGP) []*share.PriShare {
	size := len(reencs)
	decShares := make([]*share.PriShare, size)
	for i := 0; i < size; i++ {
		var decSh []byte
		var tmpSh share.PriShare
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
