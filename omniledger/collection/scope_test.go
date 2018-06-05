package collection

import "testing"
import "encoding/hex"

func TestScopeMask(test *testing.T) {
	type round struct {
		path     string
		mask     string
		bits     int
		expected bool
	}

	rounds := []round{
		{"85f46bd1ba19d1014b1179edd451ece95296e4a8c765ba8bba86c16893906398", "85f46bd1ba19d1014b1179edd451ece95296e4a8c765ba8bba86c16893906398", 256, true},
		{"85f46bd1ba19d1014b1179edd451ece95296e4a8c765ba8bba86c16893906398", "85f46bd1ba19d1014b1179edd451ece95296e4a8c765ba8bba86c16893906398", 25, true},
		{"fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff0", "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff", 252, true},
		{"fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff0", "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff", 253, false},
		{"fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff0", "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff", 0, true},
		{"85f46bd1ba19d1014b1179edd451ece95296e4a8c765ba8bba86c16893906390", "85f46bd1ba19d1014b1179edd451ece95296e4a8c765ba8bba86c16893906398", 252, true},
		{"85f46bd1ba19d1014b1179edd451ece95296e4a8c765ba8bba86c16893906390", "85f46bd1ba19d1014b1179edd451ece95296e4a8c765ba8bba86c16893906398", 254, false},
		{"85f46bd1ba19d1014b1179edd451ece95296e4a8c765ba8bba86c16893906390", "85f46bd1ba19d1014b1179edd451ece95296e4a8c765ba8bba86c16893906398", 2, true},
		{"85f46bd1ba1ad1014b1179edd451ece95296e4a8c765ba8bba86c16893906398", "85f46bd1ba19d1014b1179edd451ece95296e4a8c765ba8bba86c16893906398", 46, true},
		{"85f46bd1ba1ad1014b1179edd451ece95296e4a8c765ba8bba86c16893906398", "85f46bd1ba19d1014b1179edd451ece95296e4a8c765ba8bba86c16893906398", 47, false},
		{"85f46bd1ba1ad1014b1179edd451ece95296e4a8c765ba8bba86c16893906398", "85f46bd1ba19d1014b1179edd451ece95296e4a8c765ba8bba86c16893906398", 45, true},
		{"85f46bd1ba18d1014b1179edd451ece95296e4a8c765ba8bba86c16893906398", "85f46bd1ba19d1014b1179edd451ece95296e4a8c765ba8bba86c16893906398", 47, true},
		{"85f46bd1ba18d1014b1179edd451ece95296e4a8c765ba8bba86c16893906398", "85f46bd1ba19d1014b1179edd451ece95296e4a8c765ba8bba86c16893906398", 48, false},
		{"85f46bd1ba18d1014b1179edd451ece95296e4a8c765ba8bba86c16893906398", "85f46bd1ba19d1014b1179edd451ece95296e4a8c765ba8bba86c16893906398", 1, true},
	}

	for _, round := range rounds {
		maskvalue, _ := hex.DecodeString(round.mask)
		mask := mask{maskvalue, round.bits}

		pathslice, _ := hex.DecodeString(round.path)
		path := digest(pathslice)

		if mask.match(path, 256) != round.expected {
			test.Error("[scope.go]", "[match]", "Wrong match on reference round.")
		}

		if !(mask.match(path, 0)) {
			test.Error("[scope.go]", "[match]", "No match on zero-bit match.")
		}
	}
}

func TestScopeMethods(test *testing.T) {
	scope := scope{}

	value, _ := hex.DecodeString("1234567890")
	scope.Add(value, 3)

	if len(scope.masks) != 1 {
		test.Error("[scope.go]", "[add]", "Add does not add to masks.")
	}

	if !match(scope.masks[0].value, value, 24) || (scope.masks[0].bits != 3) {
		test.Error("[scope.go]", "[add]", "Add adds wrong mask.")
	}

	value, _ = hex.DecodeString("0987654321")
	scope.Add(value, 40)

	if len(scope.masks) != 2 {
		test.Error("[scope.go]", "[add]", "Add does not add to masks.")
	}

	if !match(scope.masks[1].value, value, 40) || (scope.masks[1].bits != 40) {
		test.Error("[scope.go]", "[add]", "Add adds wrong mask.")
	}

	scope.All()

	if len(scope.masks) != 0 {
		test.Error("[scope.go]", "[all]", "All does not wipe masks.")
	}

	if !(scope.all) {
		test.Error("[scope.go]", "[all]", "All does not properly set the all flag.")
	}

	scope.Add(value, 40)
	scope.None()

	if len(scope.masks) != 0 {
		test.Error("[scope.go]", "[none]", "None does not wipe masks.")
	}

	if scope.all {
		test.Error("[scope.go]", "[none]", "None does not properly set the all flag.")
	}
}

func TestScopeMatch(test *testing.T) {
	scope := scope{}

	pathslice, _ := hex.DecodeString("85f46bd1ba18d1014b1179edd451ece95296e4a8c765ba8bba86c16893906398")
	path := digest(pathslice)

	scope.None()
	if scope.match(path, 12) {
		test.Error("[scope.go]", "[all]", "No-mask match succeeds after None().")
	}

	scope.All()
	if !(scope.match(path, 13)) {
		test.Error("[scope.go]", "[all]", "No-mask match fails after All().")
	}

	nomatch, _ := hex.DecodeString("fa91")
	scope.Add(nomatch, 16)

	if scope.match(path, 256) {
		test.Error("[scope.go]", "[match]", "Scope match succeeds on a non-matching mask.")
	}

	maybematch, _ := hex.DecodeString("86")
	scope.Add(maybematch, 8)

	if scope.match(path, 256) {
		test.Error("[scope.go]", "[match]", "Scope match succeeds on a non-matching mask.")
	}

	if !(scope.match(path, 6)) {
		test.Error("[scope.go]", "[match]", "Scope match fails on a matching mask.")
	}

	scope.Add(maybematch, 6)

	if !(scope.match(path, 44)) {
		test.Error("[scope.go]", "[match]", "Scope match fails on matching mask.")
	}
}

func TestScopeClone(test *testing.T) {
	scope := scope{}
	scope.All()

	path, _ := hex.DecodeString("fa91")
	scope.Add(path, 5)

	path, _ = hex.DecodeString("0987654321")
	scope.Add(path, 37)

	path, _ = hex.DecodeString("1234567890")
	scope.Add(path, 42)

	clone := scope.clone()

	if !(clone.all) {
		test.Error("[scope.go]", "[clone]", "clone() does not copy all flag.")
	}

	if len(clone.masks) != len(scope.masks) {
		test.Error("[scope.go]", "[clone]", "clone() does not copy the correct number of masks")
	}

	for index := 0; index < len(clone.masks); index++ {
		if clone.masks[index].bits != scope.masks[index].bits {
			test.Error("[scope.go]", "[clone]", "clone() does not properly copy the number of bits in a mask.")
		}

		if !equal(clone.masks[index].value, scope.masks[index].value) {
			test.Error("[scope.go]", "[clone]", "clone() does not properly copy the mask value.")
		}
	}
}
