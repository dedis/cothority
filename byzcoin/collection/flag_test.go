package collection

import "testing"

func TestFlag(test *testing.T) {
	flag := flag{true}

	flag.Disable()
	if flag.value {
		test.Error("[flag.go]", "[disable]", "Disable() has no effect on flag.")
	}

	flag.Enable()
	if !(flag.value) {
		test.Error("[flag.go]", "[enable]", "Enable() has no effect on flag.")
	}
}
