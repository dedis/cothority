package main

import (
	"github.com/dedis/cothority/lib/app"
	"github.com/dedis/cothority/lib/conode"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/monitor"
	"github.com/dedis/cothority/lib/sign"
	"time"
	"github.com/dedis/crypto/nist"
	"fmt"
)

func init(){

}

func main() {
	conf := &app.ConfigColl{}
	app.ReadConfig(conf)

	// we must know who we are
	if app.RunFlags.Hostname == "" {
		dbg.Fatal("Hostname empty: Abort")
	}

	// Do some common setup
	if app.RunFlags.Mode == "client" {
		app.RunFlags.Hostname = app.RunFlags.Name
	}
	hostname := app.RunFlags.Hostname
	if hostname == conf.Hosts[0] {
		dbg.Lvlf3("Tree is %+v", conf.Tree)
	}

	
	suite := nist.NewAES128SHA256P256()

	SecretRoot := suite.Secret().Pick(suite.Cipher([]byte("Root")))
	SecretLeaf := suite.Secret().Pick(suite.Cipher([]byte("Leaf")))
	SecretMid  := suite.Secret().Pick(suite.Cipher([]byte("Middle")))

	vRoot := suite.Secret().Pick(suite.Cipher([]byte("vRoot")))
	vLeaf := suite.Secret().Pick(suite.Cipher([]byte("vLeaf")))
	vMid  := suite.Secret().Pick(suite.Cipher([]byte("vMiddle")))


	FreshSecretRoot := suite.Secret().Pick(suite.Cipher([]byte("Fresh_root")))
	FreshSecretLeaf := suite.Secret().Pick(suite.Cipher([]byte("Fresh_leaf")))
	FreshSecretMid  := suite.Secret().Pick(suite.Cipher([]byte("Fresh_middle")))


	PubRoot := suite.Point().Mul(nil, SecretRoot) 
	PubLeaf := suite.Point().Mul(nil, SecretLeaf) 
	PubMid  := suite.Point().Mul(nil, SecretMid) 


	FreshPubRoot := suite.Point().Mul(nil, FreshSecretRoot) 
	FreshPubLeaf := suite.Point().Mul(nil, FreshSecretLeaf) 
	FreshPubMid  := suite.Point().Mul(nil, FreshSecretMid) 
 	
 	numMidNodes := 1

	collectiveSecret := suite.Secret().Add(SecretRoot, SecretLeaf) 
	FreshCollectiveSecret := suite.Secret().Add(FreshSecretRoot, FreshSecretLeaf) 
	v := suite.Secret().Add(vRoot, vLeaf) 

	
	for i := 0; i < numMidNodes; i++ {
		collectiveSecret = suite.Secret().Add(collectiveSecret, SecretMid)
		FreshCollectiveSecret = suite.Secret().Add(FreshCollectiveSecret, FreshSecretMid)
		v = suite.Secret().Add(v, vMid)
	}

	
	vPub := suite.Point().Mul(nil, v)



	sign.RegisterRoundFactory(RoundMedcoType,
		func(node *sign.Node) sign.Round {
			dbg.Lvl3("Making new RoundMedco", node.Name())
			round := &RoundMedco{}
			
			if node.Name() == "127.0.0.1:2000" {
				round.Name = 1
			} else if node.Name() == "127.0.0.1:2005" {
				round.Name = 2
			} else if node.Name() == "127.0.0.1:2010" {
				round.Name = 3
			}
			//fmt.Println("name",round.Name)

			round.compare = 0
			round.bucket = 1
			round.numBuckets = 2

			// individual keys
			round.PrivateRoot = SecretRoot
			round.PrivateLeaf = SecretLeaf
			round.PrivateMid  = SecretMid

			round.vRoot = vRoot
			round.vLeaf = vLeaf
			round.vMid  = vMid

			round.v  = v
			round.vPub = vPub

			round.PublicRoot = PubRoot
			round.PublicLeaf = PubLeaf
			round.PublicMid  = PubMid

			//round.suite = suite


			// collective keys
			round.CollectivePrivate = collectiveSecret
			round.CollectivePublic = suite.Point().Mul(nil, collectiveSecret) 

			// fresh keys
			round.FreshPrivateRoot = FreshSecretRoot
			round.FreshPrivateLeaf = FreshSecretLeaf
			round.FreshPrivateMid  = FreshSecretMid


			round.FreshPublicRoot = FreshPubRoot
			round.FreshPublicLeaf = FreshPubLeaf
			round.FreshPublicMid  = FreshPubMid


			// fresh collective keys
			round.FreshCollectivePrivate = FreshCollectiveSecret
			round.FreshCollectivePublic = suite.Point().Mul(nil, FreshCollectiveSecret)

	round.RoundStruct = sign.NewRoundStruct(node, RoundMedcoType)
	// If you're sub-classing from another round-type, don't forget to remove
	// the above line, call the constructor of your parent round and add
	// round.Type = RoundMedcoType
	return round
		})

	dbg.Lvl3(hostname, "Starting to run")

	start_total := time.Now()

	app.RunFlags.StartedUp(len(conf.Hosts))
	peer := conode.NewPeer(hostname, conf.ConfigConode)

	if app.RunFlags.AmRoot {
		for {
			time.Sleep(time.Second)
			setupRound := sign.NewRoundSetup(peer.Node)
			dbg.Lvl1("Starting StartAnnouncementWithWait")
			if err := peer.StartAnnouncementWithWait(setupRound, 2*60*time.Second); err != nil {
				dbg.Lvl1("Error while doing StartAnnouncement", err)
			}
			dbg.Lvl1("End StartAnnouncementWithWait")

			counted := <-setupRound.Counted
			dbg.Lvl1("Number of peers counted:", counted)
			if counted == len(conf.Hosts) {
				dbg.Lvl1("All hosts replied")
				break
			}
		}
	}

	if app.RunFlags.AmRoot {
		round, err := sign.NewRoundFromType(RoundMedcoType, peer.Node)
		if err != nil {
			dbg.Fatal("Couldn't create", RoundMedcoType, "round:", err)
		}
		//peer.StartAnnouncement(round)
		peer.StartAnnouncementWithWait(round, time.Minute*60)
		peer.SendCloseAll()
	} else {
		peer.LoopRounds(RoundMedcoType, conf.Rounds)
	} 

	dbg.Lvlf3("Done - flags are %+v", app.RunFlags)
	monitor.End()
	elapsed_total := time.Since(start_total)
	fmt.Println("elapsed_total\n",elapsed_total)
}

