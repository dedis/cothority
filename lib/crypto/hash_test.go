package crypto_test

import (
	"github.com/dedis/cothority/lib/crypto"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/crypto/edwards/ed25519"
	"io/ioutil"
	"testing"
)

func TestHash(t *testing.T) {
	tmpfile := "/tmp/hash_test.bin"
	for _, i := range []int{16, 128, 1024} {
		str := make([]byte, i)
		err := ioutil.WriteFile(tmpfile, str, 0777)
		if err != nil {
			t.Fatal("Couldn't write file")
		}

		hash, err := crypto.HashFile(ed25519.NewAES128SHA256Ed25519(false), tmpfile)
		if err != nil {
			t.Fatal("Couldn't hash", tmpfile, err)
		}
		if len(hash) != 32 {
			t.Fatal("Length of sha256 should be 32")
		}
	}
}

func TestHashSlice(t *testing.T) {
	tmpfile := "/tmp/hash_test.bin"
	str := make([]byte, 1234)
	err := ioutil.WriteFile(tmpfile, str, 0777)
	if err != nil {
		t.Fatal("Couldn't write file")
	}

	for _, i := range []int{16, 128, 1024} {
		dbg.Lvl3("Reading", i, "bytes")
		hash, err := crypto.HashFileSlice(ed25519.NewAES128SHA256Ed25519(false),
			tmpfile, i)
		if err != nil {
			t.Fatal("Couldn't hash", tmpfile, err)
		}
		if len(hash) != 32 {
			t.Fatal("Length of sha256 should be 32")
		}
	}
}
