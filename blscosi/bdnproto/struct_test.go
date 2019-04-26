package bdnproto

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.dedis.ch/kyber/v3"
	"go.dedis.ch/kyber/v3/pairing/bn256"
	"go.dedis.ch/kyber/v3/sign"
	"go.dedis.ch/kyber/v3/sign/bdn"
	"go.dedis.ch/kyber/v3/util/random"
)

func TestBdnSignature_Verify(t *testing.T) {
	msg := []byte("abc")
	suite := bn256.NewSuite()
	sk1, pk1 := bdn.NewKeyPair(suite, random.New())
	sk2, pk2 := bdn.NewKeyPair(suite, random.New())
	_, pk3 := bdn.NewKeyPair(suite, random.New())

	mask, err := sign.NewMask(suite, []kyber.Point{pk1, pk2, pk3}, nil)
	require.NoError(t, err)
	mask.SetBit(0, true)
	mask.SetBit(1, true)

	sig1, err := bdn.Sign(suite, sk1, msg)
	require.NoError(t, err)
	sig2, err := bdn.Sign(suite, sk2, msg)
	require.NoError(t, err)

	asig, err := bdn.AggregateSignatures(suite, [][]byte{sig1, sig2}, mask)
	require.NoError(t, err)

	buf, err := asig.MarshalBinary()
	require.NoError(t, err)

	sig := BdnSignature(append(buf, mask.Mask()...))
	pubkeys := []kyber.Point{pk1, pk2, pk3}

	// we expect an error as the default threshold with n = 3 assumes f = 0.
	require.Error(t, sig.Verify(suite, msg, pubkeys))

	policy := sign.NewThresholdPolicy(2)
	// now it passes with the new policy.
	require.NoError(t, sig.VerifyWithPolicy(suite, msg, pubkeys, policy))
	// the signature should still be verified even with a policy.
	wrongMsg := []byte("cba")
	require.Error(t, sig.VerifyWithPolicy(suite, wrongMsg, pubkeys, policy))
}
