package lib

import (
	"testing"

	"go.dedis.ch/kyber/v4"

	"github.com/stretchr/testify/assert"
)

func TestRandomKeyPair(t *testing.T) {
	x1, X1 := RandomKeyPair()
	x2, X2 := RandomKeyPair()
	assert.NotEqual(t, x1, x2)
	assert.NotEqual(t, X1, X2)
}

func TestDKGSimulate(t *testing.T) {
	dkgs, _ := DKGSimulate(5, 4)
	assert.Equal(t, 5, len(dkgs))

	secrets := make([]*SharedSecret, 5)
	for i, dkg := range dkgs {
		secrets[i], _ = NewSharedSecret(dkg)
	}

	var private kyber.Scalar
	for _, secret := range secrets {
		if private != nil {
			assert.NotEqual(t, private.String(), secret.V.String())
		}
		private = secret.V
	}
}
