package collection

import "crypto/sha256"

// Methods (collection) (verifiers)

// Verify verifies that a given Proof is correct.
// It moreover adds the nodes from the Proof to the temporary nodes of the collection.
func (c *Collection) Verify(proof Proof) bool {
	if c.root.transaction.inconsistent {
		panic("Verify() called on inconsistent root.")
	}

	if (proof.Root.Label != c.root.label) || !(proof.Consistent()) {
		return false
	}

	if !(c.root.known) {
		proof.Root.to(c.root)
	}

	path := sha256.Sum256(proof.Key)
	cursor := c.root

	for depth := 0; depth < len(proof.Steps); depth++ {
		if !(cursor.children.left.known) {
			proof.Steps[depth].Left.to(cursor.children.left)
		}

		if !(cursor.children.right.known) {
			proof.Steps[depth].Right.to(cursor.children.right)
		}

		if bit(path[:], depth) {
			cursor = cursor.children.right
		} else {
			cursor = cursor.children.left
		}
	}

	return true
}
