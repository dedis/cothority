package collection

import "testing"

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

	keymatch := recordkeymatch(&collection, leaf)
	querymatch := recordquerymatch(&collection, 0, stake64.Encode(uint64(99)), leaf)
	keymismatch := recordkeymismatch(&collection, []byte("wrongkey"))

	if (keymatch.collection != &collection) || (querymatch.collection != &collection) || (keymismatch.collection != &collection) {
		test.Error("[record.go]", "[constructors]", "Constructors don't set collection appropriately.")
	}

	if !(keymatch.match) || !(querymatch.match) || keymismatch.match {
		test.Error("[record.go]", "[constructors]", "Constructors don't set match appropriately.")
	}

	if !equal(keymatch.key, []byte("mykey")) || !equal(querymatch.key, []byte("mykey")) || !equal(keymismatch.key, []byte("wrongkey")) {
		test.Error("[record.go]", "[constructors]", "Constructors don't set key appropriately")
	}

	if len(keymatch.values) != 2 || len(querymatch.values) != 2 || len(keymismatch.values) != 0 {
		test.Error("[record.go]", "[constructors]", "Constructors don't set the appropriate number of values.")
	}

	if !equal(keymatch.values[0], leaf.values[0]) || !equal(keymatch.values[1], leaf.values[1]) || !equal(querymatch.values[0], leaf.values[0]) || !equal(querymatch.values[1], leaf.values[1]) {
		test.Error("[record.go]", "[constructors]", "Constructors set the wrong values.")
	}

	if !(keymatch.Match()) || !(querymatch.Match()) || keymismatch.Match() {
		test.Error("[record.go]", "[match]", "Match() returns the wrong value.")
	}

	if !equal(keymatch.Key(), []byte("mykey")) || !equal(querymatch.Key(), []byte("mykey")) || !equal(keymismatch.Key(), []byte("wrongkey")) {
		test.Error("[record.go]", "[key]", "Key() returns the wrong value.")
	}

	keymatchvalues, keymatcherror := keymatch.Values()
	querymatchvalues, querymatcherror := querymatch.Values()

	if (keymatcherror != nil) || (querymatcherror != nil) {
		test.Error("[record.go]", "[values]", "Values() yields an error on matching record.")
	}

	if (len(keymatchvalues) != 2) || (len(querymatchvalues) != 2) {
		test.Error("[record.go]", "[values]", "Values() returns the wrong number of values")
	}

	if (keymatchvalues[0].(uint64) != 66) || !equal(keymatchvalues[1].([]byte), leaf.values[1]) || (querymatchvalues[0].(uint64) != 66) || !equal(querymatchvalues[1].([]byte), leaf.values[1]) {
		test.Error("[record.go]", "[values]", "Values() returns the wrong values.")
	}

	_, keymismatcherror := keymismatch.Values()

	if keymismatcherror == nil {
		test.Error("[record.go]", "[values]", "Values() does not yield an error on mismatching record.")
	}

	keymatch.values[0] = keymatch.values[0][:6]
	querymatch.values[0] = querymatch.values[0][:6]

	_, keyillformederror := keymatch.Values()
	_, queryillformederror := querymatch.Values()

	if (keyillformederror == nil) || (queryillformederror) == nil {
		test.Error("[record.go]", "[values]", "Values() does not yield an error on record with ill-formed values.")
	}

	keymatch.values = keymatch.values[:1]
	querymatch.values = querymatch.values[:1]

	_, keyfewerror := keymatch.Values()
	_, queryfewerror := querymatch.Values()

	if (keyfewerror == nil) || (queryfewerror == nil) {
		test.Error("[record.go]", "[values]", "Values() does not yield an error on record with wrong number of values.")
	}

	_, keymatchqueryerror := keymatch.Query()

	if keymatchqueryerror == nil {
		test.Error("[record.go]", "[query]", "Query() does not yield an error on a record without query.")
	}

	querymatchquery, querymatchqueryerror := querymatch.Query()

	if querymatchqueryerror != nil {
		test.Error("[record.go]", "[query]", "Query() yields an error on a valid record with query.")
	}

	if querymatchquery.(uint64) != 99 {
		test.Error("[record.go]", "[query]", "Query() returns wrong query.")
	}

	querymatch.field = 48

	_, querymatchqueryerror = querymatch.Query()

	if querymatchqueryerror == nil {
		test.Error("[record.go]", "[query]", "Query() does not yield an error when field is out of range.")
	}

	querymatch.field = 0
	querymatch.query = querymatch.query[:6]

	_, querymatchqueryerror = querymatch.Query()

	if querymatchqueryerror == nil {
		test.Error("[record.go]", "[query]", "Query() does not yield an error when query is malformed.")
	}
}
