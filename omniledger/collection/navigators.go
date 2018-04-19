package collection

import "errors"

// Navigator is an object representing a search of a field's value on the collection.
// It allows to get the record associated with a given value, as searched by the Navigate function of the field.
type Navigator struct {
	collection *Collection
	field      int
	query      []byte
}

// Constructors

// Navigate creates a Navigator associated with a given field and value.
func (c *Collection) Navigate(field int, value interface{}) Navigator {
	if (field < 0) || (field >= len(c.fields)) {
		panic("Field unknown.")
	}

	return Navigator{c, field, c.fields[field].Encode(value)}
}

// Methods

// Record returns the Record obtained by navigating the tree to the searched field's value.
// It returns an error if the value in question is in an unknown subtree or if the Navigate function of the field returns an error.
func (n Navigator) Record() (Record, error) {
	cursor := n.collection.root

	for {
		if !(cursor.known) {
			return Record{}, errors.New("record lies in an unknown subtree")
		}

		if cursor.leaf() {
			return recordQueryMatch(n.collection, n.field, n.query, cursor), nil
		}
		if !(cursor.children.left.known) || !(cursor.children.right.known) {
			return Record{}, errors.New("record lies in an unknown subtree")
		}

		navigation, err := n.collection.fields[n.field].Navigate(n.query, cursor.values[n.field], cursor.children.left.values[n.field], cursor.children.right.values[n.field])
		if err != nil {
			return Record{}, err
		}

		if navigation == Right {
			cursor = cursor.children.right
		} else {
			cursor = cursor.children.left
		}
	}
}
