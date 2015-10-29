package sign_test

import
(
	"testing"
	"time"

	"github.com/dedis/cothority/proto/sign"
	"github.com/dedis/cothority/lib/config"
)

// Configuration file data/exconf.json
//       0
//      / \
//     1   4
//    / \   \
//   2   3   5
func TestTreeSmallConfigVote(t *testing.T) {
	hc, err := config.LoadConfig("testdata/exconf.json")
	if err != nil {
		t.Fatal(err)
	}

	err = hc.Run(false, sign.Voter)
	if err != nil {
		t.Fatal(err)
	}

	// Achieve consensus on removing a node
	vote := &sign.Vote{Type: sign.AddVT, Av: &sign.AddVote{Name: "host5", Parent: "host4"}}
	err = hc.SNodes[0].StartVotingRound(vote)

	if err != nil {
		t.Error(err)
	}

}

func TestTCPStaticConfigVote(t *testing.T) {
	hc, err := config.LoadConfig("testdata/extcpconf.json", config.ConfigOptions{ConnType: "tcp", GenHosts: true})
	if err != nil {
		t.Error(err)
	}
	defer func() {
		for _, n := range hc.SNodes {
			n.Close()
		}
		time.Sleep(1 * time.Second)
	}()

	err = hc.Run(false, sign.Voter)
	if err != nil {
		t.Fatal(err)
	}

	// give it some time to set up
	time.Sleep(2 * time.Second)

	hc.SNodes[0].Message = []byte("Hello Voting")
	vote := &sign.Vote{Type: sign.RemoveVT, Rv: &sign.RemoveVote{Name: "host5", Parent: "host4"}}
	err = hc.SNodes[0].StartVotingRound(vote)

	if err != nil {
		t.Error(err)
	}
}
