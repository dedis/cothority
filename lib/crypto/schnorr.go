package crypto

import (
	"fmt"

	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/random"
)

type SchnorrSig struct {
	Challenge abstract.Secret
	Response  abstract.Secret
}

func (ss *SchnorrSig) MarshalBinary() ([]byte, error) {
	cbuf, err := ss.Challenge.MarshalBinary()
	if err != nil {
		return nil, err
	}
	rbuf, err := ss.Response.MarshalBinary()
	return append(cbuf, rbuf...), err
}

// SignSchnorr creates a SchnorrSig from a msg and a private key
func SignSchnorr(suite abstract.Suite, private abstract.Secret, msg []byte) (SchnorrSig, error) {
	k := suite.Secret().Pick(random.Stream)
	r := suite.Point().Mul(nil, k)

	rbuf, err := r.MarshalBinary()
	if err != nil {
		return SchnorrSig{}, err
	}

	cipher := suite.Cipher(rbuf)
	cipher.Message(nil, nil, msg)
	e := suite.Secret().Pick(cipher)
	// s = k - xe
	// s = k
	s := suite.Secret().Add(suite.Secret().Zero(), k)
	// xe
	xe := suite.Secret().Mul(private, e)
	// s = k - xe
	s = s.Sub(s, xe)
	return SchnorrSig{
		Challenge: e,
		Response:  s,
	}, nil
}

// VerifySchnorr verifies a given SchnorrSig
func VerifySchnorr(suite abstract.Suite, public abstract.Point, msg []byte, sig SchnorrSig) error {
	// compute rv = g^s * g^e
	gs := suite.Point().Mul(nil, sig.Response)
	ge := suite.Point().Mul(nil, sig.Challenge)
	rv := suite.Point().Add(gs, ge)

	// recompute challenge (e)
	rvBuff, err := rv.MarshalBinary()
	if err != nil {
		return err
	}
	cipher := suite.Cipher(rvBuff)
	cipher.Message(nil, nil, msg)
	e := suite.Secret().Pick(cipher)

	if !e.Equal(sig.Challenge) {
		return fmt.Errorf("Challenge reconstructed %v is not equal to one given in signature %v", e, sig.Challenge)
	}
	//  everything OK
	return nil

}
