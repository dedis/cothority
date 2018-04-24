package collection

import (
	"crypto/sha256"
	"testing"
)

func TestUpdateProxy(test *testing.T) {
	collection := New()

	proxy := collection.proxy([][]byte{[]byte("firstkey"), []byte("secondkey"), []byte("thirdkey")})

	if proxy.collection != &collection {
		test.Error("[update.go]", "[proxy]", "proxy() method sets wrong collection pointer.")
	}

	if len(proxy.paths) != 3 {
		test.Error("[update.go]", "[proxy]", "proxy() method sets the wrong number of paths.")
	}

	if !(proxy.paths[sha256.Sum256([]byte("firstkey"))]) ||
		!(proxy.paths[sha256.Sum256([]byte("secondkey"))]) ||
		!(proxy.paths[sha256.Sum256([]byte("thirdkey"))]) {

		test.Error("[update.go]", "[proxy]", "proxy() method does not set the paths provided.")
	}
}

func TestUpdateProxyMethods(test *testing.T) {
	ctx := testCtx("[update.go]", test)

	stake64 := Stake64{}
	collection := New(stake64)

	proxy := collection.proxy([][]byte{[]byte("firstkey"), []byte("secondkey"), []byte("thirdkey")})

	collection.Add([]byte("firstkey"), uint64(66))
	record := proxy.Get([]byte("firstkey"))

	if !(record.Match()) {
		test.Error("[update.go]", "[get]", "Proxy method get() does not return an existing record.")
	}

	values, _ := record.Values()
	if values[0].(uint64) != 66 {
		test.Error("[update.go]", "[get]", "Proxy method get() returns wrong values.")
	}

	err := proxy.Add([]byte("secondkey"), uint64(33))
	if err != nil {
		test.Error("[update.go]", "[add]", "Proxy method add() yields an error on valid key.")
	}

	err = proxy.Add([]byte("secondkey"), uint64(22))
	if err == nil {
		test.Error("[update.go]", "[add]", "Proxy method add() does not yield an error when adding an existing key.")
	}

	record, _ = collection.Get([]byte("secondkey")).Record()

	if !(record.Match()) {
		test.Error("[update.go]", "[add]", "Proxy method add() does not add the record provided.")
	}

	values, _ = record.Values()
	if values[0].(uint64) != 33 {
		test.Error("[update.go]", "[add]", "Proxy method add() adds wrong values.")
	}

	err = proxy.Set([]byte("secondkey"), uint64(22))
	if err != nil {
		test.Error("[update.go]", "[set]", "Proxy method set() yields an error when setting on an existing key.")
	}

	record, _ = collection.Get([]byte("secondkey")).Record()
	values, _ = record.Values()

	if values[0].(uint64) != 22 {
		test.Error("[update.go]", "[set]", "Proxy method set() does not set the correct values.")
	}

	err = proxy.Set([]byte("thirdkey"), uint64(11))
	if err == nil {
		test.Error("[update.go]", "[set]", "Proxy method set() does not yield an error when setting on a non-existing key.")
	}

	err = proxy.SetField([]byte("firstkey"), 0, uint64(11))
	if err != nil {
		test.Error("[update.go]", "[setfield]", "Proxy method setfield() does yields an error when setting on an existing key.")
	}

	record, _ = collection.Get([]byte("firstkey")).Record()
	values, _ = record.Values()

	if values[0].(uint64) != 11 {
		test.Error("[update.go]", "[setfield]", "Proxy method setfield() does not set the correct values.")
	}

	err = proxy.SetField([]byte("thirdkey"), 0, uint64(99))
	if err == nil {
		test.Error("[update.go]", "[setfield]", "Proxy method setfield() does not yield an error when setting on a non-existing key.")
	}

	err = proxy.Remove([]byte("secondkey"))
	if err != nil {
		test.Error("[update.go]", "[remove]", "Proxy method remove() yields an error when removing an existing key.")
	}

	record, _ = collection.Get([]byte("secondkey")).Record()
	if record.Match() {
		test.Error("[update.go]", "[remove]", "Proxy method remove() does not remove an existing key.")
	}

	err = proxy.Remove([]byte("secondkey"))
	if err == nil {
		test.Error("[update.go]", "[remove]", "Proxy method remove() does not yield an error when removing a non-existing key.")
	}

	ctx.shouldPanic("[get]", func() {
		proxy.Get([]byte("otherkey"))
	})

	ctx.shouldPanic("[add]", func() {
		proxy.Add([]byte("otherkey"), uint64(12))
	})

	ctx.shouldPanic("[set]", func() {
		proxy.Set([]byte("otherkey"), uint64(12))
	})

	ctx.shouldPanic("[setfield]", func() {
		proxy.SetField([]byte("otherkey"), 0, uint64(12))
	})

	ctx.shouldPanic("[remove]", func() {
		proxy.Remove([]byte("otherkey"))
	})
}

func TestUpdateProxyHas(test *testing.T) {
	collection := New()
	proxy := collection.proxy([][]byte{[]byte("firstkey"), []byte("secondkey"), []byte("thirdkey")})

	if !(proxy.has([]byte("firstkey"))) || !(proxy.has([]byte("secondkey"))) || !(proxy.has([]byte("thirdkey"))) {
		test.Error("[update.go]", "[has]", "Proxy method has() returns false on whitelisted key.")
	}

	if proxy.has([]byte("otherkey")) {
		test.Error("[update.go]", "[has]", "Proxy method has() returns true on non-whitelisted key.")
	}
}

type TestUpdateSingleRecordUpdate struct {
	record Proof
}

func (t TestUpdateSingleRecordUpdate) Records() []Proof {
	return []Proof{t.record}
}

func (t TestUpdateSingleRecordUpdate) Check(collection ReadOnly) bool {
	return collection.Get(t.record.Key).Match()
}

func (t TestUpdateSingleRecordUpdate) Apply(collection ReadWrite) {
	values, _ := collection.Get(t.record.Key).Values()
	value := values[0].(uint64)

	collection.Set(t.record.Key, value+1)
}

type TestUpdateDoubleRecordUpdate struct {
	from Proof
	to   Proof
}

func (t TestUpdateDoubleRecordUpdate) Records() []Proof {
	return []Proof{t.from, t.to}
}

func (t TestUpdateDoubleRecordUpdate) Check(collection ReadOnly) bool {
	if !(collection.Get(t.from.Key).Match()) || !(collection.Get(t.to.Key).Match()) {
		return false
	}

	values, _ := collection.Get(t.from.Key).Values()
	value := values[0].(uint64)

	return value > 0
}

func (t TestUpdateDoubleRecordUpdate) Apply(collection ReadWrite) {
	values, _ := collection.Get(t.from.Key).Values()
	from := values[0].(uint64)

	values, _ = collection.Get(t.to.Key).Values()
	to := values[0].(uint64)

	collection.Set(t.from.Key, from-1)
	collection.Set(t.to.Key, to+1)
}

func TestUpdatePrepare(test *testing.T) {
	ctx := testCtx("[update.go]", test)

	stake64 := Stake64{}
	collection := New(stake64)

	collection.Begin()
	collection.Add([]byte("mykey"), uint64(66))
	collection.End()

	proof, _ := collection.Get([]byte("mykey")).Proof()

	singleRecord := TestUpdateSingleRecordUpdate{proof}
	update, err := collection.Prepare(singleRecord)

	if err != nil {
		test.Error("[update.go]", "[prepare]", "Prepare() yields an error on a valid update.")
	}

	if update.transaction != collection.transaction.id {
		test.Error("[update.go]", "[prepare]", "Prepare() sets wrong transaction id.")
	}

	if !equal(update.update.Records()[0].Key, []byte("mykey")) {
		test.Error("[update.go]", "[prepare]", "Prepare() sets wrong user update.")
	}

	if len(update.proxy.paths) != 1 {
		test.Error("[update.go]", "[prepare]", "Prepare() sets the wrong number of proxy paths.")
	}

	if !(update.proxy.paths[sha256.Sum256([]byte("mykey"))]) {
		test.Error("[update.go]", "[prepare]", "Prepare() sets wrong proxy paths.")
	}

	singleRecord.record.Steps[0].Left.Label[0]++
	_, err = collection.Prepare(singleRecord)

	if err == nil {
		test.Error("[update.go]", "[prepare]", "Prepare() does not yield an error on an invalid proof.")
	}

	collection.Begin()
	collection.Add([]byte("myotherkey"), uint64(15))
	collection.End()

	from, _ := collection.Get([]byte("mykey")).Proof()
	to, _ := collection.Get([]byte("myotherkey")).Proof()

	doublerecord := TestUpdateDoubleRecordUpdate{from, to}
	update, err = collection.Prepare(doublerecord)

	if err != nil {
		test.Error("[update.go]", "[prepare]", "Prepare() yields an error on a valid update:")
	}

	if update.transaction != collection.transaction.id {
		test.Error("[update.go]", "[prepare]", "Prepare() sets wrong transaction id.")
	}

	if !equal(update.update.Records()[0].Key, []byte("mykey")) || !equal(update.update.Records()[1].Key, []byte("myotherkey")) {
		test.Error("[update.go]", "[prepare]", "Prepare() sets wrong user update.")
	}

	if len(update.proxy.paths) != 2 {
		test.Error("[update.go]", "[prepare]", "Prepare() sets the wrong number of proxy paths.")
	}

	if !(update.proxy.paths[sha256.Sum256([]byte("mykey"))]) || !(update.proxy.paths[sha256.Sum256([]byte("myotherkey"))]) {
		test.Error("[update.go]", "[prepare]", "Prepare() sets wrong proxy paths.")
	}

	doublerecord.from.Steps[0].Left.Label[0]++
	_, err = collection.Prepare(doublerecord)

	if err == nil {
		test.Error("[update.go]", "[prepare]", "Prepare() does not yield an error on an invalid proof.")
	}

	collection.root.transaction.inconsistent = true

	ctx.shouldPanic("[prepare]", func() {
		collection.Prepare(singleRecord)
	})
}

func TestUpdateApply(test *testing.T) {
	ctx := testCtx("[update.go]", test)

	collection := New()
	ctx.shouldPanic("[apply]", func() {
		collection.Apply(33)
	})
}

func TestUpdateApplyUpdate(test *testing.T) {
	ctx := testCtx("[update.go]", test)

	stake64 := Stake64{}
	collection := New(stake64)

	collection.Add([]byte("alice"), uint64(4))
	collection.Add([]byte("bob"), uint64(0))

	aliceProof, _ := collection.Get([]byte("alice")).Proof()
	aliceUpdate, _ := collection.Prepare(TestUpdateSingleRecordUpdate{aliceProof})

	err := collection.Apply(aliceUpdate)

	if err != nil {
		test.Error("[update.go]", "[applyUpdate]", "applyUpdate() yields an error when applying a valid update.")
	}

	alice, _ := collection.Get([]byte("alice")).Record()
	alicevalues, _ := alice.Values()
	alicevalue := alicevalues[0].(uint64)

	if alicevalue != 5 {
		test.Error("[update.go]", "[applyUpdate]", "applyUpdate() does not apply the update.")
	}

	collection.Begin()

	aliceProof, _ = collection.Get([]byte("alice")).Proof()
	aliceUpdate, _ = collection.Prepare(TestUpdateSingleRecordUpdate{aliceProof})

	err = collection.Apply(aliceUpdate)
	if err != nil {
		test.Error("[update.go]", "[applyUpdate]", "applyUpdate() yields an error when applying a valid update.")
	}

	err = collection.Apply(aliceUpdate)
	if err != nil {
		test.Error("[update.go]", "[applyUpdate]", "applyUpdate() yields an error when applying a valid update.")
	}

	collection.End()

	alice, _ = collection.Get([]byte("alice")).Record()
	alicevalues, _ = alice.Values()
	alicevalue = alicevalues[0].(uint64)

	if alicevalue != 7 {
		test.Error("[update.go]", "[applyUpdate]", "applyUpdate() does not apply the update.")
	}

	johnproof, _ := collection.Get([]byte("john")).Proof()
	johnupdate, _ := collection.Prepare(TestUpdateSingleRecordUpdate{johnproof})

	err = collection.Apply(johnupdate)

	if err == nil {
		test.Error("[update.go]", "[applyUpdate]", "applyUpdate() does not yield an error on an invalid update.")
	}

	aliceProof, _ = collection.Get([]byte("alice")).Proof()
	bobproof, _ := collection.Get([]byte("bob")).Proof()

	alicetobobupdate, _ := collection.Prepare(TestUpdateDoubleRecordUpdate{aliceProof, bobproof})

	err = collection.Apply(alicetobobupdate)
	if err != nil {
		test.Error("[update.go]", "[applyUpdate]", "applyUpdate() yields an error when applying a valid update.")
	}

	alice, _ = collection.Get([]byte("alice")).Record()
	alicevalues, _ = alice.Values()
	alicevalue = alicevalues[0].(uint64)

	bob, _ := collection.Get([]byte("bob")).Record()
	bobvalues, _ := bob.Values()
	bobvalue := bobvalues[0].(uint64)

	if (alicevalue != 6) || (bobvalue != 1) {
		test.Error("[update.go]", "[applyUpdate]", "applyUpdate does not apply the proper update.")
	}

	collection.Begin()

	aliceProof, _ = collection.Get([]byte("alice")).Proof()
	bobproof, _ = collection.Get([]byte("bob")).Proof()

	alicetobobupdate, _ = collection.Prepare(TestUpdateDoubleRecordUpdate{aliceProof, bobproof})
	bobtoaliceupdate, _ := collection.Prepare(TestUpdateDoubleRecordUpdate{bobproof, aliceProof})

	err = collection.Apply(alicetobobupdate)
	if err != nil {
		test.Error("[update.go]", "[applyUpdate]", "applyUpdate() yields an error when applying a valid update.")
	}

	err = collection.Apply(bobtoaliceupdate)
	if err != nil {
		test.Error("[update.go]", "[applyUpdate]", "applyUpdate() yields an error when applying a valid update.")
	}

	err = collection.Apply(bobtoaliceupdate)
	if err != nil {
		test.Error("[update.go]", "[applyUpdate]", "applyUpdate() yields an error when applying a valid update.")
	}

	collection.End()

	alice, _ = collection.Get([]byte("alice")).Record()
	alicevalues, _ = alice.Values()
	alicevalue = alicevalues[0].(uint64)

	bob, _ = collection.Get([]byte("bob")).Record()
	bobvalues, _ = bob.Values()
	bobvalue = bobvalues[0].(uint64)

	if (alicevalue != 7) || (bobvalue != 0) {
		test.Error("[update.go]", "[applyUpdate]", "applyUpdate does not apply the proper update.")
	}

	aliceProof, _ = collection.Get([]byte("alice")).Proof()
	bobproof, _ = collection.Get([]byte("bob")).Proof()

	bobtoaliceupdate, _ = collection.Prepare(TestUpdateDoubleRecordUpdate{bobproof, aliceProof})

	err = collection.Apply(bobtoaliceupdate)
	if err == nil {
		test.Error("[update.go]", "[applyUpdate]", "applyUpdate() does not yield an error when applying an invalid update.")
	}

	ctx.shouldPanic("[applyUpdate]", func() {
		collection.Apply(alicetobobupdate)
	})
}

func TestUpdateApplyUserUpdate(test *testing.T) {
	ctx := testCtx("[update.go]", test)

	stake64 := Stake64{}
	collection := New(stake64)

	collection.Add([]byte("alice"), uint64(4))
	collection.Add([]byte("bob"), uint64(0))

	aliceproof, _ := collection.Get([]byte("alice")).Proof()
	err := collection.Apply(TestUpdateSingleRecordUpdate{aliceproof})

	if err != nil {
		test.Error("[update.go]", "[applyUserUpdate]", "applyUserUpdate() yields an error when applying a valid user update.")
	}

	alice, _ := collection.Get([]byte("alice")).Record()
	alicevalues, _ := alice.Values()
	alicevalue := alicevalues[0].(uint64)

	if alicevalue != 5 {
		test.Error("[update.go]", "[applyUserUpdate]", "applyUserUpdate() does not apply the update.")
	}

	aliceproof, _ = collection.Get([]byte("alice")).Proof()
	aliceproof.Steps[0].Left.Label[0]++

	err = collection.Apply(TestUpdateSingleRecordUpdate{aliceproof})

	if err == nil {
		test.Error("[update.go]", "[applyUserUpdate]", "applyUserUpdate() does not yield an error when applying an invalid user update.")
	}

	johnproof, _ := collection.Get([]byte("john")).Proof()
	err = collection.Apply(TestUpdateSingleRecordUpdate{johnproof})

	if err == nil {
		test.Error("[update.go]", "[applyUserUpdate]", "applyUserUpdate() does not yield an error when applying an invalid user update.")
	}

	ctx.shouldPanic("[applyUserUpdate]", func() {
		collection.Begin()

		aliceproof, _ := collection.Get([]byte("alice")).Proof()

		collection.Apply(TestUpdateSingleRecordUpdate{aliceproof})
		collection.Apply(TestUpdateSingleRecordUpdate{aliceproof})

		collection.End()
	})
}
