/* This file implements a procedure of verification
   of developers' PGP singatures on a given commit.
   The return value is "true" if there is more or equal
   number of developers who has signed the commit id
   in respect to threshold value provided as a part of policy,
   and it is "false" otherwise */

package swupdate

import (
	"bytes"
	"strings"

	"github.com/dedis/cothority/log"

	"github.com/dedis/cothority/services/swupdate/verification/parsers"
	"golang.org/x/crypto/openpgp"
)

type ReleasePolicy struct {
	Threshold int      // Sufficient number of developers that must signed off to approve a commit
	PubKeys   []string // Maintenants' personal PGP public keys
}

type SignedCommit struct {
	CommitID   string        // ID of a git commit that has been signed oof by developers
	Policy     ReleasePolicy // Security policy for this very release
	Signatures []string      // Signatures of developers on the corresponding commit
	//Approval   bool         // Flag whether the commit has been approved by developers
}

func checkFileError(err error, filename string) {
	if err != nil {
		log.Errorf("Could not read file", filename)
	}
}

func ApprovalCheck(PolicyFile, SignaturesFile, Id string) (bool, error) {
	var (
		commit     SignedCommit               // Commit corresponding to be verified
		developers openpgp.EntityList         // List of all developers whose public keys are in the policy file
		approvers  map[string]*openpgp.Entity // Map of developers who provided a valid signature. Indexed by public key id (openpgp.PrimaryKey.KeyIdString)
		err        error
	)

	commit.Policy.Threshold, commit.Policy.PubKeys, err = parsers.PolicyScanner(PolicyFile)
	checkFileError(err, PolicyFile)
	commit.Signatures, err = parsers.SigScanner(SignaturesFile)
	checkFileError(err, SignaturesFile)
	commit.CommitID = Id

	approvers = make(map[string]*openpgp.Entity)

	// Creating openpgp entitylist from list of public keys
	developers = make(openpgp.EntityList, 0)
	for _, pubkey := range commit.Policy.PubKeys {
		keybuf, err := openpgp.ReadArmoredKeyRing(strings.NewReader(pubkey))
		if err != nil {
			log.Error("Could not decode armored public key", err)
		}
		for _, entity := range keybuf {
			developers = append(developers, entity)
		}
	}

	// Verifying every signature in the list and counting valid ones
	for _, signature := range commit.Signatures {
		result, err := openpgp.CheckArmoredDetachedSignature(developers, bytes.NewBufferString(commit.CommitID), strings.NewReader(signature))
		if err != nil {
			log.Lvl1("The signature is invalid or cannot be verified due to", err)
		} else {
			if approvers[result.PrimaryKey.KeyIdString()] == nil { // We need to check that this is a unique signature
				approvers[result.PrimaryKey.KeyIdString()] = result
				log.Lvl4("Approver: %+v", result.Identities)
			}
		}
	}

	log.Lvl3("Is release approved? ", len(approvers) >= commit.Policy.Threshold)
	// commit.Approval = (len(approvers) >= commit.Policy.Threshold)

	return len(approvers) >= commit.Policy.Threshold, err
	// return commit, err
}
