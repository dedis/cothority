package swupdate

import (
	"testing"

	"github.com/dedis/cothority/log"
)

func TestMain(m *testing.M) {
	log.MainTest(m)
}

func TestBuildVerification(t *testing.T) {
	exampleRe := Release{
		BinHash:  "47fc4fc8b49d808d12ec811947077e6bbdaae1b8838a14e53309ef4178a46df1",
		GitPath:  "https://github.com/nikirill/Signal-Android.git",
		CommitId: "975ae735dc4617410681a02e4fe9a9a492417ef8"}

	res, err := BuildVerification(&exampleRe)
	log.Lvlf1("output is %+v and %+v", res, err)
}
