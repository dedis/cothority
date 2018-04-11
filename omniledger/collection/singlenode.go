package collection

import "errors"

// Private methods (collection) (single node operations)

func (this *Collection) placeholder(node *node) {
	node.known = true
	node.key = []byte{}
	node.values = make([][]byte, len(this.fields))

	for index := 0; index < len(this.fields); index++ {
		node.values[index] = this.fields[index].Placeholder()
	}

	node.children.left = nil
	node.children.right = nil

	this.update(node)
}

func (this *Collection) update(node *node) error {
	if !(node.known) {
		return errors.New("Updating an unknown node.")
	}

	if node.leaf() {
		node.label = sha256(true, node.key, node.values)
	} else {
		if !(node.children.left.known) || !(node.children.right.known) {
			return errors.New("Updating internal node with unknown children.")
		}

		node.values = make([][]byte, len(this.fields))

		for index := 0; index < len(this.fields); index++ {
			parentvalue, parenterror := this.fields[index].Parent(node.children.left.values[index], node.children.right.values[index])

			if parenterror != nil {
				return parenterror
			}

			node.values[index] = parentvalue
		}

		node.label = sha256(false, node.values, node.children.left.label[:], node.children.right.label[:])
	}

	return nil
}
