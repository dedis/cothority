package collection

import "testing"
import "encoding/binary"

func TestVerifiersVerify(test *testing.T) {
	ctx := testCtx("[verifiers.go]", test)

	stake64 := Stake64{}
	data := Data{}

	collection := New(stake64, data)
	unknown := New(stake64, data)
	unknown.scope.None()

	collection.Begin()
	unknown.Begin()

	for index := 0; index < 512; index++ {
		key := make([]byte, 8)
		binary.BigEndian.PutUint64(key, uint64(index))

		collection.Add(key, uint64(index), key)
		unknown.Add(key, uint64(index), key)
	}

	collection.End()
	unknown.End()

	for index := 0; index < 512; index++ {
		key := make([]byte, 8)
		binary.BigEndian.PutUint64(key, uint64(index))

		proof, _ := collection.Get(key).Proof()
		if !(unknown.Verify(proof)) {
			test.Error("[verifiers.go]", "[verify]", "Verify() fails on valid proof.")
		}
	}

	ctx.verify.tree("[verify]", &unknown)

	for index := 0; index < 512; index++ {
		key := make([]byte, 8)
		binary.BigEndian.PutUint64(key, uint64(index))

		ctx.verify.values("[verify]", &unknown, key, uint64(index), key)
	}

	proof, _ := collection.Get(make([]byte, 8)).Proof()
	proof.Steps[0].Left.Label[0]++

	if unknown.Verify(proof) {
		test.Error("[verifiers.go]", "[verify]", "Verify() accepts an inconsistent proof.")
	}

	collection.Add([]byte("mykey"), uint64(1066), []byte("myvalue"))

	proof, _ = collection.Get(make([]byte, 8)).Proof()

	if unknown.Verify(proof) {
		test.Error("[verifiers.go]", "[verify]", "Verify() accepts a consistent proof from a wrong root.")
	}

	collection.root.transaction.inconsistent = true
	ctx.shouldPanic("[verify]", func() {
		collection.Verify(proof)
	})
}
