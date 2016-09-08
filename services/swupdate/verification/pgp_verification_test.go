package swupdate

import (
	"testing"

	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/services/swupdate/verification/parsers"
)

func TestApprovalCheck(t *testing.T) {
	var (
		PolicyFile     = "example/policy.toml"
		SignaturesFile = "example/signatures.txt"
		ReleaseFile    = "example/release.toml"
	)

	_, _, commitId, _ := parsers.ReleaseScanner(ReleaseFile)
	decision, err := ApprovalCheck(PolicyFile, SignaturesFile, commitId)
	if err != nil {
		log.Panic("Problem with verifying approval of developers", err)
	}
	log.Printf("Is release approved? %+v", decision)
}
