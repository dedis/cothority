package lib

import (
	"errors"

	"go.dedis.ch/kyber/v3"
	"go.dedis.ch/kyber/v3/share/dkg/rabin"
	"go.dedis.ch/kyber/v3/util/random"

	"go.dedis.ch/cothority/v3"
)

// RandomKeyPair creates a random public/private Diffie-Hellman key pair.
func RandomKeyPair() (x kyber.Scalar, X kyber.Point) {
	x = cothority.Suite.Scalar().Pick(random.New())
	X = cothority.Suite.Point().Mul(x, nil)
	return
}

// SharedSecret represents the needed information to do shared encryption and decryption.
type SharedSecret struct {
	Index   int
	V       kyber.Scalar
	X       kyber.Point
	Commits []kyber.Point
}

// NewSharedSecret takes an initialized DistKeyGenerator and returns the
// minimal set of values necessary to do shared encryption/decryption.
func NewSharedSecret(dkg *dkg.DistKeyGenerator) (*SharedSecret, error) {
	if dkg == nil {
		return nil, errors.New("no valid dkg given")
	}
	if !dkg.Finished() {
		return nil, errors.New("dkg is not finished yet")
	}
	dks, err := dkg.DistKeyShare()
	if err != nil {
		return nil, err
	}
	return &SharedSecret{
		Index:   dks.Share.I,
		V:       dks.Share.V,
		X:       dks.Public(),
		Commits: dks.Commits,
	}, nil
}

// DKGSimulate runs an offline version of the DKG protocol.
func DKGSimulate(nbrNodes, threshold int) (dkgs []*dkg.DistKeyGenerator, err error) {
	dkgs = make([]*dkg.DistKeyGenerator, nbrNodes)
	scalars := make([]kyber.Scalar, nbrNodes)
	points := make([]kyber.Point, nbrNodes)

	// 1a - initialisation
	for i := range scalars {
		scalars[i] = cothority.Suite.Scalar().Pick(cothority.Suite.RandomStream())
		points[i] = cothority.Suite.Point().Mul(scalars[i], nil)
	}

	// 1b - key-sharing
	for i := range dkgs {
		dkgs[i], err = dkg.NewDistKeyGenerator(cothority.Suite, scalars[i], points, threshold)
		if err != nil {
			return
		}
	}
	// Exchange of Deals
	responses := make([][]*dkg.Response, nbrNodes)
	for i, p := range dkgs {
		responses[i] = make([]*dkg.Response, nbrNodes)
		deals, err := p.Deals()
		if err != nil {
			return nil, err
		}
		for j, d := range deals {
			responses[i][j], err = dkgs[j].ProcessDeal(d)
			if err != nil {
				return nil, err
			}
		}
	}
	// ProcessResponses
	for _, resp := range responses {
		for j, r := range resp {
			for k, p := range dkgs {
				if r != nil && j != k {
					p.ProcessResponse(r)
				}
			}
		}
	}

	// Secret commits
	for _, p := range dkgs {
		commit, err := p.SecretCommits()
		if err != nil {
			return nil, err
		}
		for _, p2 := range dkgs {
			p2.ProcessSecretCommits(commit)
		}
	}

	// Verify if all is OK
	for _, p := range dkgs {
		if !p.Finished() {
			return nil, errors.New("one of the dkgs is not finished yet")
		}
	}
	return
}
