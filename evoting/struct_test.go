package evoting

import (
	"testing"

	"github.com/dedis/cothority"
	"github.com/dedis/kyber/util/key"
	"github.com/stretchr/testify/assert"
)

func TestDigest(t *testing.T) {
	login := &Login{ID: []byte{0, 1, 2}, User: 3}
	assert.Equal(t, []byte{0, 1, 2, 3}, login.Digest())
}

func TestSchnorr(t *testing.T) {
	kp := key.NewKeyPair(cothority.Suite)
	x, X := kp.Private, kp.Public

	login := &Login{ID: []byte{0, 1, 2}, User: 3}
	login.Sign(x)
	login.Signature = append(login.Signature, byte(0))
	assert.NotNil(t, login.Verify(X))

	login.Sign(x)
	assert.Nil(t, login.Verify(X))
}
