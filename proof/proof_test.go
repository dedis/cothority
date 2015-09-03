package proof

import (
	"crypto/sha256"
	"testing"

	"github.com/ineiti/cothorities/hashid"
)

func TestPath(t *testing.T) {

	newHash := sha256.New
	hash := newHash()
	n := 13

	leaves := make([]hashid.HashId, n)
	for i := range leaves {
		leaves[i] = make([]byte, hash.Size())
		for j := range leaves[i] {
			leaves[i][j] = byte(i)
		}
		// println("leaf", i, ":", hex.EncodeToString(leaves[i]))
		// fmt.Println("leaf", i, ":", leaves[i])
	}

	root, proofs := ProofTree(newHash, leaves)
	for i := range proofs {
		if proofs[i].Check(newHash, root, leaves[i]) == false {
			t.Error("check failed at leaf", i)
		}
	}
}

func TestPathLong(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	newHash := sha256.New
	hash := newHash()
	n := 100 // takes 6 secons
	for k := 0; k < n; k++ {
		leaves := make([]hashid.HashId, k)
		for i := range leaves {
			leaves[i] = make([]byte, hash.Size())
			for j := range leaves[i] {
				leaves[i][j] = byte(i)
			}
		}

		root, proofs := ProofTree(newHash, leaves)
		for i := range proofs {
			if proofs[i].Check(newHash, root, leaves[i]) == false {
				t.Error("check failed at leaf", i)
			}
		}
	}
}
