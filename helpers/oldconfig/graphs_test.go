package oldconfig

import (
	"fmt"
	"testing"
	"github.com/ineiti/cothorities/sign"

	"github.com/dedis/crypto/nist"
	"github.com/dedis/crypto/random"
)

func TestTreeFromRandomGraph(t *testing.T) {
	//defer profile.Start(profile.CPUProfile, profile.ProfilePath(".")).Stop()
	hc, err := loadGraph("../data/wax.dat", nist.NewAES128SHA256P256(), random.Stream)
	if err != nil || hc == nil {
		fmt.Println("run data/gen.py to generate graphs")
		return
	}
	// if err := ioutil.WriteFile("data/wax.json", []byte(hc.String()), 0666); err != nil {
	// 	fmt.Println(err)
	// }
	//fmt.Println(hc.String())

	// Have root node initiate the signing protocol
	// via a simple annoucement
	hc.SNodes[0].LogTest = []byte("Hello World")
	//fmt.Println(hc.SNodes[0].NChildren())
	//fmt.Println(hc.SNodes[0].Peers())
	hc.SNodes[0].Announce(0, &sign.AnnouncementMessage{LogTest: hc.SNodes[0].LogTest})
}

func Benchmark1000Nodes(b *testing.B) {
	hc, _ := LoadConfig("../data/wax.json")
	hc.SNodes[0].LogTest = []byte("Hello World")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hc.SNodes[0].Announce(0, &sign.AnnouncementMessage{LogTest: hc.SNodes[0].LogTest, Round: i})
	}
}
