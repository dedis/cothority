package collection

import "testing"
import "sort"
import csha256 "crypto/sha256"
import "encoding/binary"

func TestNavigatorsConstructors(test *testing.T) {
	ctx := testctx("[navigators.go]", test)

	stake64 := Stake64{}
	data := Data{}

	collection := New(stake64, data, stake64)
	navigator := collection.Navigate(2, uint64(14))

	if navigator.collection != &collection {
		test.Error("[navigators.go]", "[constructors]", "Navigator constructor sets wrong collection pointer.")
	}

	if navigator.field != 2 {
		test.Error("[navigators.go]", "[constructors]", "Navigator constructor sets wrong field number.")
	}

	if !equal(navigator.query, stake64.Encode(uint64(14))) {
		test.Error("[navigators.go]", "[constructors]", "Navigator constructor sets wrong field value.")
	}

	ctx.should_panic("[constructors]", func() {
		collection.Navigate(3, uint64(14))
	})

	ctx.should_panic("[constructors]", func() {
		collection.Navigate(-1, uint64(14))
	})

	ctx.should_panic("[constructors]", func() {
		collection.Navigate(1, "wrongtype")
	})
}

func TestNavigatorsRecord(test *testing.T) {
	stake64 := Stake64{}
	collection := New(stake64)

	entries := make([]int, 512)

	for index := 0; index < 512; index++ {
		entries[index] = index
		key := make([]byte, 8)
		binary.BigEndian.PutUint64(key, uint64(index))
		collection.Add(key, uint64(index))
	}

	sort.Slice(entries, func(i int, j int) bool {
		keyi := make([]byte, 8)
		keyj := make([]byte, 8)

		binary.BigEndian.PutUint64(keyi, uint64(entries[i]))
		binary.BigEndian.PutUint64(keyj, uint64(entries[j]))

		pathi := sha256(keyi)
		pathj := sha256(keyj)

		for index := 0; index < csha256.Size; index++ {
			if pathi[index] < pathj[index] {
				return true
			} else if pathi[index] > pathj[index] {
				return false
			}
		}

		return false
	})

	query := uint64(0)

	for index := 0; index < 512; index++ {
		for stake := 0; stake < entries[index]; stake++ {
			record, err := collection.Navigate(0, query).Record()

			if err != nil {
				test.Error("[navigators.go]", "[record]", "Navigation fails on valid query.")
			}

			values, _ := record.Values()
			if int(values[0].(uint64)) != entries[index] {
				test.Error("[navigators.go]", "[record]", "Navigation yields wrong record.")
			}

			query++
		}
	}

	rootvalue, _ := stake64.Decode(collection.root.values[0])
	_, error := collection.Navigate(0, rootvalue.(uint64)+1).Record()

	if error == nil {
		test.Error("[navigators.go]", "[record]", "Navigation does not yield an error on invalid query.")
	}

	collection.root.children.left.known = false

	_, error = collection.Navigate(0, uint64(0)).Record()

	if error == nil {
		test.Error("[navigators.go]", "[record]", "Navigation does not yield an error on unknown subtree.")
	}

	collection.scope.None()
	collection.Collect()

	_, error = collection.Navigate(0, uint64(0)).Record()

	if error == nil {
		test.Error("[navigators.go]", "[record]", "Navigation does not yield an error on unknown tree.")
	}
}
