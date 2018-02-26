package lib

import (
	"testing"

	"github.com/dedis/kyber"

	"github.com/stretchr/testify/assert"
)

func TestSplit(t *testing.T) {
	_, X := RandomKeyPair()
	ballots := genBox(X, 2).Ballots

	a, b := Split(ballots)
	assert.Equal(t, ballots[0].Alpha, a[0])
	assert.Equal(t, ballots[0].Beta, b[0])
	assert.Equal(t, ballots[1].Alpha, a[1])
	assert.Equal(t, ballots[1].Beta, b[1])
}

func TestCombine(t *testing.T) {
	_, X1 := RandomKeyPair()
	_, X2 := RandomKeyPair()

	a, b := []kyber.Point{X1, X1}, []kyber.Point{X2, X2}
	ballots := Combine(a, b)

	assert.Equal(t, X1, ballots[0].Alpha)
	assert.Equal(t, X1, ballots[1].Alpha)
	assert.Equal(t, X2, ballots[0].Beta)
	assert.Equal(t, X2, ballots[1].Beta)
}
