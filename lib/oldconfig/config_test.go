package oldconfig

import (
	"sync"
	"testing"

	"github.com/dedis/cothority/sign"
)

func TestLoadConfig(t *testing.T) {
	_, err := LoadConfig("../data/exconf.json")
	if err != nil {
		t.Error("error parsing json file:", err)
	}
}

func TestPubKeysConfig(t *testing.T) {
	_, err := LoadConfig("../data/exconf.json", ConfigOptions{ConnType: "tcp", GenHosts: true})
	if err != nil {
		t.Fatal("error parsing json file:", err)
	}
	// if err := ioutil.WriteFile("data/exconf_wkeys.json", []byte(hc.String()), 0666); err != nil {
	// 	t.Fatal(err)
	// }
}

func TestPubKeysOneNode(t *testing.T) {
	// has hosts 8089 - 9094 @ 172.27.187.80
	done := make(chan bool)
	hosts := []string{
		":6095",
		":6096",
		":6097",
		":6098",
		":6099",
		":6100"}
	nodes := make(map[string]*sign.Node)
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, host := range hosts {
		wg.Add(1)
		go func(host string) {
			hc, err := LoadConfig("../data/exconf_wkeys.json", ConfigOptions{ConnType: "tcp", Host: host, Hostnames: hosts})
			if err != nil {
				done <- true
				t.Fatal(err)
			}

			err = hc.Run(false, sign.MerkleTree, host)
			if err != nil {
				done <- true
				t.Fatal(err)
			}

			mu.Lock()
			nodes[host] = hc.SNodes[0]
			mu.Unlock()

			if hc.SNodes[0].IsRoot(0) {
				hc.SNodes[0].LogTest = []byte("Hello World")
				err = hc.SNodes[0].Announce(0, &sign.AnnouncementMessage{LogTest: hc.SNodes[0].LogTest})
				if err != nil {
					t.Fatal(err)
				}
				done <- true
				hc.SNodes[0].Close()
			}
			wg.Done()
		}(host)
	}
	<-done
	wg.Wait()
	for _, sn := range nodes {
		sn.Close()
	}
}
