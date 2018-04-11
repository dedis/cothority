package collection

import "errors"

type getter struct {
	collection *Collection
	key        []byte
}

// Constructors

func (this *Collection) Get(key []byte) getter {
	return getter{this, key}
}

// Methods

func (this getter) Record() (Record, error) {
	path := sha256(this.key)

	depth := 0
	cursor := this.collection.root

	for {
		if !(cursor.known) {
			return Record{}, errors.New("Record lies in an unknown subtree.")
		}

		if cursor.leaf() {
			if equal(cursor.key, this.key) {
				return recordkeymatch(this.collection, cursor), nil
			} else {
				return recordkeymismatch(this.collection, this.key), nil
			}
		} else {
			if bit(path[:], depth) {
				cursor = cursor.children.right
			} else {
				cursor = cursor.children.left
			}

			depth++
		}
	}
}

func (this getter) Proof() (Proof, error) {
	var proof Proof

	proof.collection = this.collection
	proof.key = this.key

	proof.root = dumpnode(this.collection.root)

	path := sha256(this.key)

	depth := 0
	cursor := this.collection.root

	if !(cursor.known) {
		return proof, errors.New("Record lies in unknown subtree.")
	}

	for {
		if !(cursor.children.left.known) || !(cursor.children.right.known) {
			return proof, errors.New("Record lies in unknown subtree.")
		}

		proof.steps = append(proof.steps, step{dumpnode(cursor.children.left), dumpnode(cursor.children.right)})

		if bit(path[:], depth) {
			cursor = cursor.children.right
		} else {
			cursor = cursor.children.left
		}

		depth++

		if cursor.leaf() {
			break
		}
	}

	return proof, nil
}
