package collection

import "errors"

type navigator struct {
	collection *Collection
	field      int
	query      []byte
}

// Constructors

func (this *Collection) Navigate(field int, value interface{}) navigator {
	if (field < 0) || (field >= len(this.fields)) {
		panic("Field unknown.")
	}

	return navigator{this, field, this.fields[field].Encode(value)}
}

// Methods

func (this navigator) Record() (Record, error) {
	cursor := this.collection.root

	for {
		if !(cursor.known) {
			return Record{}, errors.New("Record lies in an unknown subtree.")
		}

		if cursor.leaf() {
			return recordquerymatch(this.collection, this.field, this.query, cursor), nil
		} else {
			if !(cursor.children.left.known) || !(cursor.children.right.known) {
				return Record{}, errors.New("Record lies in an unknown subtree.")
			}

			navigation, error := this.collection.fields[this.field].Navigate(this.query, cursor.values[this.field], cursor.children.left.values[this.field], cursor.children.right.values[this.field])
			if error != nil {
				return Record{}, error
			}

			if navigation == Right {
				cursor = cursor.children.right
			} else {
				cursor = cursor.children.left
			}
		}
	}
}
