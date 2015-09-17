package sign_test

import (
	"strconv"
	"testing"

	"github.com/dedis/cothority/sign"
	"github.com/dedis/cothority/lib/config"
)

// func init() {
// 	log.SetOutput(ioutil.Discard)
// }

// one after the other by the root (one signature per message created)
func SimpleRoundsThroughput(N int, b *testing.B) {
	hc, _ := config.LoadConfig("../test/data/extcpconf.json", config.ConfigOptions{ConnType: "tcp", GenHosts: true})
	hc.Run(false, sign.PubKey)

	for n := 0; n < b.N; n++ {
		for i := 0; i < N; i++ {
			hc.SNodes[0].LogTest = []byte("hello world" + strconv.Itoa(i))
			hc.SNodes[0].Announce(DefaultView, &sign.AnnouncementMessage{LogTest: hc.SNodes[0].LogTest, Round: 0})

		}
		for _, sn := range hc.SNodes {
			sn.Close()
		}

	}
}

func BenchmarkSimpleRoundsThroughput100(b *testing.B) {
	SimpleRoundsThroughput(100, b)
}

func BenchmarkSimpleRoundsThroughput200(b *testing.B) {
	SimpleRoundsThroughput(200, b)
}
