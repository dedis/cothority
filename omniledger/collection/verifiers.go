package collection

// Methods (collection) (verifiers)

func (this *Collection) Verify(proof Proof) bool {
	if this.root.transaction.inconsistent {
		panic("Verify() called on inconsistent root.")
	}

	if (proof.root.Label != this.root.label) || !(proof.consistent()) {
		return false
	}

	if !(this.root.known) {
		proof.root.to(this.root)
	}

	path := sha256(proof.key)
	cursor := this.root

	for depth := 0; depth < len(proof.steps); depth++ {
		if !(cursor.children.left.known) {
			proof.steps[depth].Left.to(cursor.children.left)
		}

		if !(cursor.children.right.known) {
			proof.steps[depth].Right.to(cursor.children.right)
		}

		if bit(path[:], depth) {
			cursor = cursor.children.right
		} else {
			cursor = cursor.children.left
		}
	}

	return true
}
