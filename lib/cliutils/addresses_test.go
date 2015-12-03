package cliutils

import (
	"strconv"
	"testing"
)

func TestVerifyPort(t *testing.T) {
	good := "abs:104"
	medium := "abs:"
	bad := "abs"
	port := 1000
	ports := strconv.Itoa(port)
	if na, err := VerifyPort(good, port); err != nil {
		t.Error("VerifyPort should not generate any error", err)
	} else if na != good {
		t.Error("address should not have changed with a port number inside it")
	}
	if na, err := VerifyPort(medium, port); err != nil {
		t.Error("VerifyPort should not gen any error", err)
	} else if na != medium+ports {
		t.Error("address should generated is not correct: added port")
	}
	if na, err := VerifyPort(bad, port); err != nil {
		t.Error("VerifyPort should not gen any error", err)
	} else if na != bad+":"+ports {
		t.Error("address should generated is not correct: added port and:")
	}
}

func TestGetPort(t *testing.T) {
	if GetPort("localhost", 2000) != 2000 {
		t.Error("Didn't get correct default-port")
	}
	if GetPort("localhost:2001", 2000) != 2001 {
		t.Error("Didn't extract correct port")
	}
}

func TestGetAddress(t *testing.T) {
	if GetAddress("localhost") != "localhost" {
		t.Error("Didn't get correct address for address-only")
	}
	if GetAddress("localhost:2000") != "localhost" {
		t.Error("Didn't separate address and port")
	}
}
