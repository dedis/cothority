package collection

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestRecord(test *testing.T) {
	stake64 := Stake64{}
	data := Data{}

	collection := New(stake64, data)
	collection.Add([]byte("mykey"), uint64(66), []byte("mydata"))

	var leaf *node

	if collection.root.children.left.placeholder() {
		leaf = collection.root.children.right
	} else {
		leaf = collection.root.children.left
	}

	keyMatch := recordKeyMatch(&collection, leaf)
	queryMatch := recordQueryMatch(&collection, 0, stake64.Encode(uint64(99)), leaf)
	keyMismatch := recordKeyMismatch(&collection, []byte("wrongkey"))

	if (keyMatch.collection != &collection) || (queryMatch.collection != &collection) || (keyMismatch.collection != &collection) {
		test.Error("[record.go]", "[constructors]", "Constructors don't set collection appropriately.")
	}

	if !(keyMatch.match) || !(queryMatch.match) || keyMismatch.match {
		test.Error("[record.go]", "[constructors]", "Constructors don't set match appropriately.")
	}

	if !equal(keyMatch.key, []byte("mykey")) || !equal(queryMatch.key, []byte("mykey")) || !equal(keyMismatch.key, []byte("wrongkey")) {
		test.Error("[record.go]", "[constructors]", "Constructors don't set key appropriately")
	}

	if len(keyMatch.values) != 2 || len(queryMatch.values) != 2 || len(keyMismatch.values) != 0 {
		test.Error("[record.go]", "[constructors]", "Constructors don't set the appropriate number of values.")
	}

	if !equal(keyMatch.values[0], leaf.values[0]) || !equal(keyMatch.values[1], leaf.values[1]) || !equal(queryMatch.values[0], leaf.values[0]) || !equal(queryMatch.values[1], leaf.values[1]) {
		test.Error("[record.go]", "[constructors]", "Constructors set the wrong values.")
	}

	if !(keyMatch.Match()) || !(queryMatch.Match()) || keyMismatch.Match() {
		test.Error("[record.go]", "[match]", "Match() returns the wrong value.")
	}

	if !equal(keyMatch.Key(), []byte("mykey")) || !equal(queryMatch.Key(), []byte("mykey")) || !equal(keyMismatch.Key(), []byte("wrongkey")) {
		test.Error("[record.go]", "[key]", "Key() returns the wrong value.")
	}

	keyMatchValues, keyMatchError := keyMatch.Values()
	queryMatchValues, queryMatchError := queryMatch.Values()

	if (keyMatchError != nil) || (queryMatchError != nil) {
		test.Error("[record.go]", "[values]", "Values() yields an error on matching record.")
	}

	if (len(keyMatchValues) != 2) || (len(queryMatchValues) != 2) {
		test.Error("[record.go]", "[values]", "Values() returns the wrong number of values")
	}

	if (keyMatchValues[0].(uint64) != 66) || !equal(keyMatchValues[1].([]byte), leaf.values[1]) || (queryMatchValues[0].(uint64) != 66) || !equal(queryMatchValues[1].([]byte), leaf.values[1]) {
		test.Error("[record.go]", "[values]", "Values() returns the wrong values.")
	}

	_, keymismatcherror := keyMismatch.Values()

	if keymismatcherror == nil {
		test.Error("[record.go]", "[values]", "Values() does not yield an error on mismatching record.")
	}

	keyMatch.values[0] = keyMatch.values[0][:6]
	queryMatch.values[0] = queryMatch.values[0][:6]

	_, keyIllFormedError := keyMatch.Values()
	_, queryIllFormedError := queryMatch.Values()

	if (keyIllFormedError == nil) || (queryIllFormedError) == nil {
		test.Error("[record.go]", "[values]", "Values() does not yield an error on record with ill-formed values.")
	}

	keyMatch.values = keyMatch.values[:1]
	queryMatch.values = queryMatch.values[:1]

	_, keyFewError := keyMatch.Values()
	_, queryFewError := queryMatch.Values()

	if (keyFewError == nil) || (queryFewError == nil) {
		test.Error("[record.go]", "[values]", "Values() does not yield an error on record with wrong number of values.")
	}

	_, keyMatchQueryError := keyMatch.Query()

	if keyMatchQueryError == nil {
		test.Error("[record.go]", "[query]", "Query() does not yield an error on a record without query.")
	}

	queryMatchQuery, queryMatchQueryError := queryMatch.Query()

	if queryMatchQueryError != nil {
		test.Error("[record.go]", "[query]", "Query() yields an error on a valid record with query.")
	}

	if queryMatchQuery.(uint64) != 99 {
		test.Error("[record.go]", "[query]", "Query() returns wrong query.")
	}

	queryMatch.field = 48

	_, queryMatchQueryError = queryMatch.Query()

	if queryMatchQueryError == nil {
		test.Error("[record.go]", "[query]", "Query() does not yield an error when field is out of range.")
	}

	queryMatch.field = 0
	queryMatch.query = queryMatch.query[:6]

	_, queryMatchQueryError = queryMatch.Query()

	if queryMatchQueryError == nil {
		test.Error("[record.go]", "[query]", "Query() does not yield an error when query is malformed.")
	}
}

func TestRecordEmpty(test *testing.T) {
	collection := New(Data{})

	_, err := collection.Get([]byte{}).Record()
	require.NotNil(test, err)
}
