package cosi

import (
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/config"
	"github.com/dedis/crypto/edwards"
	"testing"
)

var testSuite = edwards.NewAES128SHA256Ed25519(false)

func TestCosiCommitment(t *testing.T) {
	var length = 5
	cosis := genCosis(length)
	commitments := genCommitments(cosis)
	root := genCosi()
	root.Commit(commitments)
	// compute the aggregate commitment ourself...
	aggCommit := testSuite.Point().Null()
	// add commitment of children
	for _, com := range commitments {
		aggCommit = aggCommit.Add(aggCommit, com.Commitment)
	}
	// add commitment of root
	aggCommit = aggCommit.Add(aggCommit, root.commitment)
	if !aggCommit.Equal(root.aggregateCommitment) {
		t.Fatal("Aggregate Commitment are not equal")
	}
}

func TestCosiChallenge(t *testing.T) {
	root, children := genPostCommitmentPhaseCosi(5)
	msg := []byte("Hello World Cosi\n")
	chal, err := root.CreateChallenge(msg)
	if err != nil {
		t.Fatal("Error during challenge generation")
	}
	for _, child := range children {
		child.Challenge(chal)
		if !child.challenge.Equal(chal.Challenge) {
			t.Fatal("Error during challenge on children")
		}
	}
}

// TestCosiResponse will test wether the response generation is correct or not
func TestCosiResponse(t *testing.T) {

}

func genSecrets(nb int) []abstract.Secret {
	panic("aie")
}

func genPoints(nb int) []abstract.Point {
	panic("aie")
}

func genKeyPair(nb int) []*config.KeyPair {
	var kps []*config.KeyPair
	for i := 0; i < nb; i++ {
		kps = append(kps, config.NewKeyPair(testSuite))
	}
	return kps
}

func genCosi() *Cosi {
	kp := config.NewKeyPair(testSuite)
	return NewCosi(testSuite, kp.Secret)
}
func genCosis(nb int) []*Cosi {
	kps := genKeyPair(nb)
	var cosis []*Cosi
	for _, kp := range kps {
		cosis = append(cosis, NewCosi(testSuite, kp.Secret))
	}
	return cosis
}

func genCommitments(cosis []*Cosi) []*Commitment {
	commitments := make([]*Commitment, len(cosis))
	for i := range cosis {
		commitments[i] = cosis[i].CreateCommitment()
	}
	return commitments
}

// genPostCommitmentPhaseCosi returns the Root and its Children Cosi. They have
// already made the Commitment phase.
func genPostCommitmentPhaseCosi(nb int) (*Cosi, []*Cosi) {
	cosis := genCosis(nb)
	commitments := genCommitments(cosis)
	root := genCosi()
	root.Commit(commitments)
	return root, cosis
}
