package cliutils

import (
	"testing"
)

func TestUpsertPort(t *testing.T) {
	good := "abs:104"
	medium := "abs:"
	bad := "abs"
	port := "1000"
	if na, err := UpsertPort(good, port); err != nil {
		t.Error("upsert should not gen any error", err)
	} else if na != good {
		t.Error("address should not have changed with a port number inside it")
	}
	if na, err := UpsertPort(medium, port); err != nil {
		t.Error("upsert should not gen any error", err)
	} else if na != medium+port {
		t.Error("address should generated is not correct: added port")
	}
	if na, err := UpsertPort(medium, port); err != nil {
		t.Error("upsert should not gen any error", err)
	} else if na != bad+":"+port {
		t.Error("address should generated is not correct: added port and :")
	}

}
