package lib

import (
	"testing"

	"go.dedis.ch/kyber/v4/util/random"

	"github.com/stretchr/testify/assert"

	"go.dedis.ch/cothority/v4"
)

func TestElGamal(t *testing.T) {
	secret := cothority.Suite.Scalar().Pick(random.New())
	public := cothority.Suite.Point().Mul(secret, nil)
	message := []byte("nevv")

	K, C := Encrypt(public, message)
	dec, _ := Decrypt(secret, K, C).Data()
	assert.Equal(t, message, dec)
}
