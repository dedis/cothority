package parsers

import (
	"io/ioutil"
	"testing"

	"github.com/dedis/cothority/log"
)

func TestSigScanner(t *testing.T) {
	log.TestOutput(testing.Verbose(), 4)
	signame := "/tmp/sigs.txt"
	ioutil.WriteFile(signame, []byte(TestFileSignatures), 0660)
	blocks, err := SigScanner(signame)
	if err != nil {
		t.Fatal("Error while parsing blocks:", err)
	}

	log.Printf("%+v", blocks)
}

func TestPolicyScanner(t *testing.T) {
	log.TestOutput(testing.Verbose(), 4)
	polname := "/tmp/policy.toml"
	ioutil.WriteFile(polname, []byte(TestFilePolicy), 0660)
	thres, devkeys, err := PolicyScanner(polname)
	if err != nil {
		t.Fatal("Error while parsing blocks:", err)
	}

	log.Printf("%d\n %+v\n", thres, devkeys)
}

func TestReleaseScanner(t *testing.T) {
	log.TestOutput(testing.Verbose(), 4)
	relname := "/tmp/release.toml"
	ioutil.WriteFile(relname, []byte(TestFileRelease), 0660)
	hash, path, id, err := ReleaseScanner(relname)
	if err != nil {
		t.Fatal("Error while parsing blocks:", err)
	}

	log.Printf("%+v\n %+v\n %+v\n", hash, path, id)
}
