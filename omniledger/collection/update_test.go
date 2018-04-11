package collection

import "testing"

func TestUpdateProxy(test *testing.T) {
	collection := New()

	proxy := collection.proxy([][]byte{[]byte("firstkey"), []byte("secondkey"), []byte("thirdkey")})

	if proxy.collection != &collection {
		test.Error("[update.go]", "[proxy]", "proxy() method sets wrong collection pointer.")
	}

	if len(proxy.paths) != 3 {
		test.Error("[update.go]", "[proxy]", "proxy() method sets the wrong number of paths.")
	}

	if !(proxy.paths[sha256([]byte("firstkey"))]) || !(proxy.paths[sha256([]byte("secondkey"))]) || !(proxy.paths[sha256([]byte("thirdkey"))]) {
		test.Error("[update.go]", "[proxy]", "proxy() method does not set the paths provided.")
	}
}

func TestUpdateProxyMethods(test *testing.T) {
	ctx := testctx("[update.go]", test)

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

	error := proxy.Add([]byte("secondkey"), uint64(33))
	if error != nil {
		test.Error("[update.go]", "[add]", "Proxy method add() yields an error on valid key.")
	}

	error = proxy.Add([]byte("secondkey"), uint64(22))
	if error == nil {
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

	error = proxy.Set([]byte("secondkey"), uint64(22))
	if error != nil {
		test.Error("[update.go]", "[set]", "Proxy method set() yields an error when setting on an existing key.")
	}

	record, _ = collection.Get([]byte("secondkey")).Record()
	values, _ = record.Values()

	if values[0].(uint64) != 22 {
		test.Error("[update.go]", "[set]", "Proxy method set() does not set the correct values.")
	}

	error = proxy.Set([]byte("thirdkey"), uint64(11))
	if error == nil {
		test.Error("[update.go]", "[set]", "Proxy method set() does not yield an error when setting on a non-existing key.")
	}

	error = proxy.SetField([]byte("firstkey"), 0, uint64(11))
	if error != nil {
		test.Error("[update.go]", "[setfield]", "Proxy method setfield() does yields an error when setting on an existing key.")
	}

	record, _ = collection.Get([]byte("firstkey")).Record()
	values, _ = record.Values()

	if values[0].(uint64) != 11 {
		test.Error("[update.go]", "[setfield]", "Proxy method setfield() does not set the correct values.")
	}

	error = proxy.SetField([]byte("thirdkey"), 0, uint64(99))
	if error == nil {
		test.Error("[update.go]", "[setfield]", "Proxy method setfield() does not yield an error when setting on a non-existing key.")
	}

	error = proxy.Remove([]byte("secondkey"))
	if error != nil {
		test.Error("[update.go]", "[remove]", "Proxy method remove() yields an error when removing an existing key.")
	}

	record, _ = collection.Get([]byte("secondkey")).Record()
	if record.Match() {
		test.Error("[update.go]", "[remove]", "Proxy method remove() does not remove an existing key.")
	}

	error = proxy.Remove([]byte("secondkey"))
	if error == nil {
		test.Error("[update.go]", "[remove]", "Proxy method remove() does not yield an error when removing a non-existing key.")
	}

	ctx.should_panic("[get]", func() {
		proxy.Get([]byte("otherkey"))
	})

	ctx.should_panic("[add]", func() {
		proxy.Add([]byte("otherkey"), uint64(12))
	})

	ctx.should_panic("[set]", func() {
		proxy.Set([]byte("otherkey"), uint64(12))
	})

	ctx.should_panic("[setfield]", func() {
		proxy.SetField([]byte("otherkey"), 0, uint64(12))
	})

	ctx.should_panic("[remove]", func() {
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

func (this TestUpdateSingleRecordUpdate) Records() []Proof {
	return []Proof{this.record}
}

func (this TestUpdateSingleRecordUpdate) Check(collection ReadOnly) bool {
	return collection.Get(this.record.Key()).Match()
}

func (this TestUpdateSingleRecordUpdate) Apply(collection ReadWrite) {
	values, _ := collection.Get(this.record.Key()).Values()
	value := values[0].(uint64)

	collection.Set(this.record.Key(), value+1)
}

type TestUpdateDoubleRecordUpdate struct {
	from Proof
	to   Proof
}

func (this TestUpdateDoubleRecordUpdate) Records() []Proof {
	return []Proof{this.from, this.to}
}

func (this TestUpdateDoubleRecordUpdate) Check(collection ReadOnly) bool {
	if !(collection.Get(this.from.Key()).Match()) || !(collection.Get(this.to.Key()).Match()) {
		return false
	}

	values, _ := collection.Get(this.from.Key()).Values()
	value := values[0].(uint64)

	return value > 0
}

func (this TestUpdateDoubleRecordUpdate) Apply(collection ReadWrite) {
	values, _ := collection.Get(this.from.Key()).Values()
	from := values[0].(uint64)

	values, _ = collection.Get(this.to.Key()).Values()
	to := values[0].(uint64)

	collection.Set(this.from.Key(), from-1)
	collection.Set(this.to.Key(), to+1)
}

func TestUpdatePrepare(test *testing.T) {
	ctx := testctx("[update.go]", test)

	stake64 := Stake64{}
	collection := New(stake64)

	collection.Begin()
	collection.Add([]byte("mykey"), uint64(66))
	collection.End()

	proof, _ := collection.Get([]byte("mykey")).Proof()

	singlerecord := TestUpdateSingleRecordUpdate{proof}
	update, error := collection.Prepare(singlerecord)

	if error != nil {
		test.Error("[update.go]", "[prepare]", "Prepare() yields an error on a valid update.")
	}

	if update.transaction != collection.transaction.id {
		test.Error("[update.go]", "[prepare]", "Prepare() sets wrong transaction id.")
	}

	if !equal(update.update.Records()[0].Key(), []byte("mykey")) {
		test.Error("[update.go]", "[prepare]", "Prepare() sets wrong user update.")
	}

	if len(update.proxy.paths) != 1 {
		test.Error("[update.go]", "[prepare]", "Prepare() sets the wrong number of proxy paths.")
	}

	if !(update.proxy.paths[sha256([]byte("mykey"))]) {
		test.Error("[update.go]", "[prepare]", "Prepare() sets wrong proxy paths.")
	}

	singlerecord.record.steps[0].Left.Label[0]++
	_, error = collection.Prepare(singlerecord)

	if error == nil {
		test.Error("[update.go]", "[prepare]", "Prepare() does not yield an error on an invalid proof.")
	}

	collection.Begin()
	collection.Add([]byte("myotherkey"), uint64(15))
	collection.End()

	from, _ := collection.Get([]byte("mykey")).Proof()
	to, _ := collection.Get([]byte("myotherkey")).Proof()

	doublerecord := TestUpdateDoubleRecordUpdate{from, to}
	update, error = collection.Prepare(doublerecord)

	if error != nil {
		test.Error("[update.go]", "[prepare]", "Prepare() yields an error on a valid update:")
	}

	if update.transaction != collection.transaction.id {
		test.Error("[update.go]", "[prepare]", "Prepare() sets wrong transaction id.")
	}

	if !equal(update.update.Records()[0].Key(), []byte("mykey")) || !equal(update.update.Records()[1].Key(), []byte("myotherkey")) {
		test.Error("[update.go]", "[prepare]", "Prepare() sets wrong user update.")
	}

	if len(update.proxy.paths) != 2 {
		test.Error("[update.go]", "[prepare]", "Prepare() sets the wrong number of proxy paths.")
	}

	if !(update.proxy.paths[sha256([]byte("mykey"))]) || !(update.proxy.paths[sha256([]byte("myotherkey"))]) {
		test.Error("[update.go]", "[prepare]", "Prepare() sets wrong proxy paths.")
	}

	doublerecord.from.steps[0].Left.Label[0]++
	_, error = collection.Prepare(doublerecord)

	if error == nil {
		test.Error("[update.go]", "[prepare]", "Prepare() does not yield an error on an invalid proof.")
	}

	collection.root.transaction.inconsistent = true

	ctx.should_panic("[prepare]", func() {
		collection.Prepare(singlerecord)
	})
}

func TestUpdateApply(test *testing.T) {
	ctx := testctx("[update.go]", test)

	collection := New()
	ctx.should_panic("[apply]", func() {
		collection.Apply(33)
	})
}

func TestUpdateApplyUpdate(test *testing.T) {
	ctx := testctx("[update.go]", test)

	stake64 := Stake64{}
	collection := New(stake64)

	collection.Add([]byte("alice"), uint64(4))
	collection.Add([]byte("bob"), uint64(0))

	aliceproof, _ := collection.Get([]byte("alice")).Proof()
	aliceupdate, _ := collection.Prepare(TestUpdateSingleRecordUpdate{aliceproof})

	error := collection.Apply(aliceupdate)

	if error != nil {
		test.Error("[update.go]", "[applyupdate]", "applyupdate() yields an error when applying a valid update.")
	}

	alice, _ := collection.Get([]byte("alice")).Record()
	alicevalues, _ := alice.Values()
	alicevalue := alicevalues[0].(uint64)

	if alicevalue != 5 {
		test.Error("[update.go]", "[applyupdate]", "applyupdate() does not apply the update.")
	}

	collection.Begin()

	aliceproof, _ = collection.Get([]byte("alice")).Proof()
	aliceupdate, _ = collection.Prepare(TestUpdateSingleRecordUpdate{aliceproof})

	error = collection.Apply(aliceupdate)
	if error != nil {
		test.Error("[update.go]", "[applyupdate]", "applyupdate() yields an error when applying a valid update.")
	}

	error = collection.Apply(aliceupdate)
	if error != nil {
		test.Error("[update.go]", "[applyupdate]", "applyupdate() yields an error when applying a valid update.")
	}

	collection.End()

	alice, _ = collection.Get([]byte("alice")).Record()
	alicevalues, _ = alice.Values()
	alicevalue = alicevalues[0].(uint64)

	if alicevalue != 7 {
		test.Error("[update.go]", "[applyupdate]", "applyupdate() does not apply the update.")
	}

	johnproof, _ := collection.Get([]byte("john")).Proof()
	johnupdate, _ := collection.Prepare(TestUpdateSingleRecordUpdate{johnproof})

	error = collection.Apply(johnupdate)

	if error == nil {
		test.Error("[update.go]", "[applyupdate]", "applyupdate() does not yield an error on an invalid update.")
	}

	aliceproof, _ = collection.Get([]byte("alice")).Proof()
	bobproof, _ := collection.Get([]byte("bob")).Proof()

	alicetobobupdate, _ := collection.Prepare(TestUpdateDoubleRecordUpdate{aliceproof, bobproof})

	error = collection.Apply(alicetobobupdate)
	if error != nil {
		test.Error("[update.go]", "[applyupdate]", "applyupdate() yields an error when applying a valid update.")
	}

	alice, _ = collection.Get([]byte("alice")).Record()
	alicevalues, _ = alice.Values()
	alicevalue = alicevalues[0].(uint64)

	bob, _ := collection.Get([]byte("bob")).Record()
	bobvalues, _ := bob.Values()
	bobvalue := bobvalues[0].(uint64)

	if (alicevalue != 6) || (bobvalue != 1) {
		test.Error("[update.go]", "[applyupdate]", "applyupdate does not apply the proper update.")
	}

	collection.Begin()

	aliceproof, _ = collection.Get([]byte("alice")).Proof()
	bobproof, _ = collection.Get([]byte("bob")).Proof()

	alicetobobupdate, _ = collection.Prepare(TestUpdateDoubleRecordUpdate{aliceproof, bobproof})
	bobtoaliceupdate, _ := collection.Prepare(TestUpdateDoubleRecordUpdate{bobproof, aliceproof})

	error = collection.Apply(alicetobobupdate)
	if error != nil {
		test.Error("[update.go]", "[applyupdate]", "applyupdate() yields an error when applying a valid update.")
	}

	error = collection.Apply(bobtoaliceupdate)
	if error != nil {
		test.Error("[update.go]", "[applyupdate]", "applyupdate() yields an error when applying a valid update.")
	}

	error = collection.Apply(bobtoaliceupdate)
	if error != nil {
		test.Error("[update.go]", "[applyupdate]", "applyupdate() yields an error when applying a valid update.")
	}

	collection.End()

	alice, _ = collection.Get([]byte("alice")).Record()
	alicevalues, _ = alice.Values()
	alicevalue = alicevalues[0].(uint64)

	bob, _ = collection.Get([]byte("bob")).Record()
	bobvalues, _ = bob.Values()
	bobvalue = bobvalues[0].(uint64)

	if (alicevalue != 7) || (bobvalue != 0) {
		test.Error("[update.go]", "[applyupdate]", "applyupdate does not apply the proper update.")
	}

	aliceproof, _ = collection.Get([]byte("alice")).Proof()
	bobproof, _ = collection.Get([]byte("bob")).Proof()

	bobtoaliceupdate, _ = collection.Prepare(TestUpdateDoubleRecordUpdate{bobproof, aliceproof})

	error = collection.Apply(bobtoaliceupdate)
	if error == nil {
		test.Error("[update.go]", "[applyupdate]", "applyupdate() does not yield an error when applying an invalid update.")
	}

	ctx.should_panic("[applyupdate]", func() {
		collection.Apply(alicetobobupdate)
	})
}

func TestUpdateApplyUserUpdate(test *testing.T) {
	ctx := testctx("[update.go]", test)

	stake64 := Stake64{}
	collection := New(stake64)

	collection.Add([]byte("alice"), uint64(4))
	collection.Add([]byte("bob"), uint64(0))

	aliceproof, _ := collection.Get([]byte("alice")).Proof()
	error := collection.Apply(TestUpdateSingleRecordUpdate{aliceproof})

	if error != nil {
		test.Error("[update.go]", "[applyuserupdate]", "applyuserupdate() yields an error when applying a valid user update.")
	}

	alice, _ := collection.Get([]byte("alice")).Record()
	alicevalues, _ := alice.Values()
	alicevalue := alicevalues[0].(uint64)

	if alicevalue != 5 {
		test.Error("[update.go]", "[applyuserupdate]", "applyuserupdate() does not apply the update.")
	}

	aliceproof, _ = collection.Get([]byte("alice")).Proof()
	aliceproof.steps[0].Left.Label[0]++

	error = collection.Apply(TestUpdateSingleRecordUpdate{aliceproof})

	if error == nil {
		test.Error("[update.go]", "[applyuserupdate]", "applyuserupdate() does not yield an error when applying an invalid user update.")
	}

	johnproof, _ := collection.Get([]byte("john")).Proof()
	error = collection.Apply(TestUpdateSingleRecordUpdate{johnproof})

	if error == nil {
		test.Error("[update.go]", "[applyuserupdate]", "applyuserupdate() does not yield an error when applying an invalid user update.")
	}

	ctx.should_panic("[applyuserupdate]", func() {
		collection.Begin()

		aliceproof, _ := collection.Get([]byte("alice")).Proof()

		collection.Apply(TestUpdateSingleRecordUpdate{aliceproof})
		collection.Apply(TestUpdateSingleRecordUpdate{aliceproof})

		collection.End()
	})
}
