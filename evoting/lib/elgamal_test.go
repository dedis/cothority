package lib

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestElGamal(t *testing.T) {
	secret := Suite.Scalar().Pick(Stream)
	public := Suite.Point().Mul(secret, nil)
	message := []byte("nevv")

	K, C := Encrypt(public, message)
	dec, _ := Decrypt(secret, K, C).Data()
	assert.Equal(t, message, dec)
}
