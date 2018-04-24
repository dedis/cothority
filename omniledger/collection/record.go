package collection

import "errors"

// Record holds the result of a search query.
// It has getters to see the query, if the search was successful, the key and the value.
type Record struct {
	collection *Collection

	field int
	query []byte
	match bool //if matched the query

	key    []byte
	values [][]byte
}

// Constructors

func recordKeyMatch(collection *Collection, node *node) Record {
	return Record{collection, 0, []byte{}, true, node.key, node.values}
}

func recordQueryMatch(collection *Collection, field int, query []byte, node *node) Record {
	return Record{collection, field, query, true, node.key, node.values}
}

func recordKeyMismatch(collection *Collection, key []byte) Record {
	return Record{collection, 0, []byte{}, false, key, [][]byte{}}
}

// Getters

// Query returns the original query, decoded, that generated the record.
// It returns an error if the record was generated from a getter (key search).
func (r Record) Query() (interface{}, error) {
	if len(r.query) == 0 {
		return nil, errors.New("no query specified")
	}

	if len(r.values) <= r.field {
		return nil, errors.New("field out of range")
	}

	value, err := r.collection.fields[r.field].Decode(r.query)

	if err != nil {
		return nil, err
	}

	return value, nil
}

// Match returns true if the record match the query that generated it, and false otherwise.
func (r Record) Match() bool {
	return r.match
}

// Key returns the key of the record
func (r Record) Key() []byte {
	return r.key
}

// Values returns a copy of the values of a record.
// If the record didn't match the query, an error will be returned.
func (r Record) Values() ([]interface{}, error) {
	if !(r.match) {
		return []interface{}{}, errors.New("no match found")
	}

	if len(r.values) != len(r.collection.fields) {
		return []interface{}{}, errors.New("wrong number of values")
	}

	var values []interface{}

	for index := 0; index < len(r.values); index++ {
		value, err := r.collection.fields[index].Decode(r.values[index])

		if err != nil {
			return []interface{}{}, err
		}

		values = append(values, value)
	}

	return values, nil
}
