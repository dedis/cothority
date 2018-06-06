package collection

import "testing"
import "math/rand"

func TestFieldData(test *testing.T) {
	var data Data

	if !equal(data.Encode([]byte("mydata")), []byte("mydata")) {
		test.Error("[field.go]", "[encode]", "Data encode() function is not the identity function.")
	}

	value, decodeerror := data.Decode([]byte("mydata"))

	if decodeerror != nil {
		test.Error("[field.go]", "[decode]", "Data decode() yields an error.")
	}

	if !equal(value.([]byte), []byte("mydata")) {
		test.Error("[field.go]", "[decode]", "Data decode() function is not the identity function.")
	}

	if len(data.Placeholder()) != 0 {
		test.Error("[field.go]", "[placeholder]", "Non-empty data placeholder.")
	}

	parent, parenterror := data.Parent([]byte("leftdata"), []byte("rightdata"))

	if parenterror != nil {
		test.Error("[field.go]", "[parent]", "Data Parent() yields an error")
	}

	if len(parent) != 0 {
		test.Error("[field.go]", "[parent]", "Non-empty data parent: Data should not propagate anything to its parent.")
	}

	_, err := data.Navigate([]byte("query"), []byte("parentdata"), []byte("leftdata"), []byte("rightdata"))
	if err == nil {
		test.Error("[field.go]", "[navigate]", "Data navigation does not yield errors. It should be impossible to navigate Data.")
	}
}

func TestFieldStake64(test *testing.T) {
	var stake64 Stake64

	for trial := 0; trial < 64; trial++ {
		stake := rand.Uint64()

		value, decodeerror := stake64.Decode(stake64.Encode(stake))

		if decodeerror != nil {
			test.Error("[field.go]", "[encodeconsistency]", "Stake64 decode error on encoded value.")
		}

		if stake != value.(uint64) {
			test.Error("[field.go]", "[encodeconsistency]", "Stake64 ncode / decode inconsistency.")
		}
	}

	_, wrongsizeerror := stake64.Decode(make([]byte, 3))

	if wrongsizeerror == nil {
		test.Error("[field.go]", "[decode]", "Decode() does not yield an error on wrong input size.")
	}

	value, decodeerror := stake64.Decode(stake64.Placeholder())

	if decodeerror != nil {
		test.Error("[field.go]", "[placeholder]", "Decode() yields error on placeholder.")
	}

	if value.(uint64) != 0 {
		test.Error("[field.go]", "[placeholder]", "Non-zero placeholder stake.")
	}

	for trial := 0; trial < 64; trial++ {
		leftstake := rand.Uint64()
		rightstake := rand.Uint64()

		left := stake64.Encode(leftstake)
		right := stake64.Encode(rightstake)

		parentbuffer, parenterror := stake64.Parent(left, right)

		if parenterror != nil {
			test.Error("[field.go]", "[parent]", "Parent() fails on well-formed inputs.")
		}

		parentvalue, decodeerror := stake64.Decode(parentbuffer)

		if decodeerror != nil {
			test.Error("[field.go]", "[parent]", "Parent stake cannot be decoded from Parent() function.")
		}

		if parentvalue.(uint64) != (leftstake + rightstake) {
			test.Error("[field.go]", "[parent]", "Parent stake is not equal to the sum of children stakes.")
		}
	}

	_, wrongsizeerror = stake64.Parent(make([]byte, 3), make([]byte, 8))

	if wrongsizeerror == nil {
		test.Error("[field.go]", "[parent]", "Parent() does not yield an error on ill-formed inputs.")
	}

	_, wrongsizeerror = stake64.Parent(make([]byte, 8), make([]byte, 5))

	if wrongsizeerror == nil {
		test.Error("[field.go]", "[parent]", "Parent() does not yield an error on ill-formed inputs.")
	}

	for trial := 0; trial < 64; trial++ {
		leftstake := uint64(rand.Uint32())
		rightstake := uint64(rand.Uint32())
		parentstake := leftstake + rightstake

		parent := stake64.Encode(parentstake)
		left := stake64.Encode(leftstake)
		right := stake64.Encode(rightstake)

		wrongquerystake := parentstake + uint64(rand.Uint32())
		wrongquery := stake64.Encode(wrongquerystake)

		navigation, err := stake64.Navigate(wrongquery, parent, left, right)

		if err == nil {
			test.Error("[field.go]", "[navigate]", "Error not yielded on illegal Stake64 query.")
		}

		querystake, stakeerror := stake64.Decode(wrongquery)

		if stakeerror != nil {
			test.Error("[field.go]", "[navigate]", "Decode() yields an error on well-formed query.")
		}

		if querystake.(uint64) != wrongquerystake {
			test.Error("[field.go]", "[navigate]", "Navigate altered illegal Stake64 query without navigating.")
		}

		querystake = rand.Uint64() % parentstake
		query := stake64.Encode(querystake)

		navigation, err = stake64.Navigate(query, parent, left, right)

		if err != nil {
			test.Error("[field.go]", "[navigate]", "Error yielded on legal Stake64 query.")
		}

		newquerystake, newqueryerror := stake64.Decode(query)

		if newqueryerror != nil {
			test.Error("[field.go]", "[navigate]", "Decode() yields an error on well-formed query.")
		}

		if querystake.(uint64) >= leftstake {
			if !navigation {
				test.Error("[field.go]", "[navigate]", "Stake64 navigates on wrong child.")
			}

			if newquerystake.(uint64) != querystake.(uint64)-leftstake {
				test.Error("[field.go]", "[navigate]", "Stake64 right navigation doesn't correctly decrease the query.")
			}
		} else {
			if navigation {
				test.Error("[field.go]", "[navigate]", "Stake64 navigates on wrong child.")
			}

			if newquerystake.(uint64) != querystake.(uint64) {
				test.Error("[field.go]", "[navigate]", "Stake64 left navigation altered the query.")
			}
		}
	}

	wrong := make([]byte, 4)
	zero := make([]byte, 8)

	one := make([]byte, 8)
	one[7] = 1

	_, wrongsizeerror = stake64.Navigate(wrong, one, one, zero)

	if wrongsizeerror == nil {
		test.Error("[field.go]", "[navigate]", "Stake64 navigation does not yield an error on ill-formed input.")
	}

	_, wrongsizeerror = stake64.Navigate(zero, wrong, one, zero)

	if wrongsizeerror == nil {
		test.Error("[field.go]", "[navigate]", "Stake64 navigation does not yield an error on ill-formed input.")
	}

	_, wrongsizeerror = stake64.Navigate(zero, one, wrong, one)

	if wrongsizeerror == nil {
		test.Error("[field.go]", "[navigate]", "Stake64 navigation does not yield an error on ill-formed input.")
	}
}
