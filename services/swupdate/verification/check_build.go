package swupdate

import (
	"os/exec"
	"os/user"

	"io/ioutil"

	"crypto/sha256"

	"encoding/hex"

	"path"

	"os"

	"github.com/dedis/cothority/log"
)

// This file contains functions for pulling source
// code corresponding to a release to be singed,
// retrieving a policy, building the binary, hashing
// it and checking that the hash matches the hash of
// the release

const SigFile = "signatures.txt"
const PolFile = "policy.toml"
const BuildFile = "building.sh"

type Release struct {
	BinHash  string
	GitPath  string
	CommitId string
}

func BuildVerification(r *Release) (bool, error) {
	var approveResult, matchResult bool
	var err error

	// Retrieve HOME directory of a current user
	usr, err := user.Current()
	if err != nil {
		log.Fatal(err)
	}
	// Create temp directory
	dir, err := ioutil.TempDir(usr.HomeDir, "source")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(dir)

	// Collect the source and retrieve PGP signatures
	if err = exec.Command("git", "clone", r.GitPath, dir).Run(); err != nil {
		log.Fatalf("Could not clone the repo: %v", err)
	}
	if err = exec.Command("git", "-C", dir, "fetch", "origin", "refs/notes/signatures:refs/notes/signatures").Run(); err != nil {
		log.Lvlf1("Problem with fetching signatures: %v", err)
	}
	signatures, err := exec.Command("git", "-C", dir, "notes", "--ref=signatures", "show", r.CommitId).Output()
	if err != nil {
		log.Fatalf("Problem with showing signatures: %v", err)
	}
	ioutil.WriteFile(path.Join(dir, SigFile), signatures, 0660)
	log.Lvlf4(string(signatures))

	ch1 := make(chan bool, 1)
	ch2 := make(chan bool, 1)

	// First goroutine to check the approval of maintainers
	go func() {
		d1, er := ApprovalCheck(path.Join(dir, PolFile), path.Join(dir, SigFile), r.CommitId)
		ch1 <- d1
		if er != nil {
			log.Lvlf1("Problem with signatures verification", er)
		}
	}()

	// Second goroutine to check if the BinHash matches the source
	go func() {
		hasher := sha256.New()
		//log.Lvlf3("Current directory: %v", b)
		//if err = exec.Command("git", "-C", dir, "checkout", r.CommitId).Run(); err != nil {
		//	log.Lvlf1("Problem with checking out to the commitId: %v", err)
		//}
		if err = os.Chmod(path.Join(dir, BuildFile), 0777); err != nil {
			log.Lvlf1("Cannot change file mode: %v", err)
		}
		pathToBin, err := exec.Command(path.Join(dir, BuildFile), dir).Output()
		if err != nil {
			log.Lvlf1("Problem with building the source", err)
		}
		log.Print("is the problem after this?")
		buf, err := ioutil.ReadFile(path.Join(dir, string(pathToBin)))
		if err != nil {
			log.Lvlf1("Cannot open the binary to hash: %v", err)
		}
		hasher.Write(buf)
		log.Lvlf1("Hash of the binary is %+v", hex.EncodeToString(hasher.Sum(nil)))
		d2 := (hex.EncodeToString(hasher.Sum(nil)) == r.BinHash)
		ch2 <- d2
	}()

	approveResult = <-ch1
	matchResult = <-ch2
	if !approveResult {
		log.Lvl1("Release has not been approved by maintainers\n")
	}
	if !matchResult {
		log.Lvl1("Source and release do not match!\n")
	}

	return (approveResult && matchResult), err
}
