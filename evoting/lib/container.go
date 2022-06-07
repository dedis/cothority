package lib

import (
	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/kyber/v3"
	"go.dedis.ch/kyber/v3/proof"
	"go.dedis.ch/kyber/v3/share/dkg/rabin"
	"go.dedis.ch/kyber/v3/shuffle"
	"go.dedis.ch/kyber/v3/util/random"
)

// Box is a wrapper around a list of encrypted ballots.
type Box struct {
	Ballots []*Ballot
}

// genMix generates n mixes with corresponding proofs out of the ballots.
func (b *Box) genMix(key kyber.Point, n int) []*Mix {
	mixes := make([]*Mix, n)

	x, y := Split(b.Ballots)
	for i := range mixes {
		v, w, prover := shuffle.Shuffle(cothority.Suite, nil, key, x, y, random.New())
		proof, _ := proof.HashProve(cothority.Suite, "", prover)
		mixes[i] = &Mix{
			Ballots: Combine(v, w),
			Proof:   proof,
		}
		x, y = v, w
	}
	return mixes
}

// genPartials generates partial decryptions for a given list of shared secrets.
func (m *Mix) genPartials(dkgs []*dkg.DistKeyGenerator) []*Partial {
	partials := make([]*Partial, len(dkgs))

	for i, gen := range dkgs {
		secret, _ := NewSharedSecret(gen)
		points := make([]kyber.Point, len(m.Ballots))
		for j, ballot := range m.Ballots {
			points[j] = Decrypt(secret.V, ballot.Alpha, ballot.Beta)
		}
		partials[i] = &Partial{
			Points: points,
		}
	}
	return partials
}

// Split separates the ElGamal pairs of a list of ballots into separate lists.
func Split(ballots []*Ballot) (alpha, beta []kyber.Point) {
	n := len(ballots)
	alpha, beta = make([]kyber.Point, n), make([]kyber.Point, n)
	for i := range ballots {
		alpha[i] = ballots[i].Alpha
		beta[i] = ballots[i].Beta
	}
	return
}

// Combine creates a list of ballots from two lists of points.
func Combine(alpha, beta []kyber.Point) []*Ballot {
	ballots := make([]*Ballot, len(alpha))
	for i := range ballots {
		ballots[i] = &Ballot{Alpha: alpha[i], Beta: beta[i]}
	}
	return ballots
}
