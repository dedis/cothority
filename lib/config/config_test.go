package config

import (
	"sync"
	"testing"

	"github.com/dedis/cothority/proto/sign"
	"sort"
	"github.com/dedis/crypto/edwards"
	"github.com/dedis/cothority/lib/coconet"
	"strings"
	"strconv"
	"encoding/json"
	"io/ioutil"
	dbg "github.com/dedis/cothority/lib/debug_lvl"
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



// TODO: if in tcp mode associate each hostname in the file with a different
// port. Get the remote address of this computer to combine with those for the
// complete hostnames to be used by the hosts.
// LoadConfig loads a configuration file in the format specified above. It
// populates a HostConfig with HostNode Hosts and goPeer Peers.
func LoadConfig(fname string, opts ...ConfigOptions) (*HostConfig, error) {
	file, err := ioutil.ReadFile(fname)
	if err != nil {
		return nil, err
	}
	return LoadJSON(file, opts...)
}

func LoadJSON(file []byte, optsSlice ...ConfigOptions) (*HostConfig, error) {
	opts := ConfigOptions{}
	if len(optsSlice) > 0 {
		opts = optsSlice[0]
	}

	hc := NewHostConfig()
	var cf ConfigFile
	err := json.Unmarshal(file, &cf)
	if err != nil {
		return hc, err
	}
	connT := GoC
	if cf.Conn == "tcp" {
		connT = TcpC
	}

	// options override file
	if opts.ConnType == "tcp" {
		connT = TcpC
	}

	dir := hc.Dir
	hosts := make(map[string]coconet.Host)
	nameToAddr := make(map[string]string)

	if connT == GoC {
		for _, h := range cf.Hosts {
			if _, ok := hc.Hosts[h]; !ok {
				nameToAddr[h] = h
				// it doesn't make sense to only make 1 go host
				if opts.Faulty == true {
					gohost := coconet.NewGoHost(h, dir)
					hosts[h] = coconet.NewFaultyHost(gohost)
				} else {
					hosts[h] = coconet.NewGoHost(h, dir)
				}
			}
		}

	} else if connT == TcpC {
		localAddr := ""

		if opts.GenHosts {
			localAddr, err = GetAddress()
			if err != nil {
				return nil, err
			}
		}

		for i, h := range cf.Hosts {

			addr := h
			if opts.GenHosts {
				p := strconv.Itoa(StartConfigPort)
				addr = localAddr + ":" + p
				//dbg.Lvl3("created new host address: ", addr)
				StartConfigPort += 10
			} else if opts.Port != "" {
				dbg.Lvl3("attempting to rewrite port: ", opts.Port)
				// if the port has been specified change the port
				hostport := strings.Split(addr, ":")
				dbg.Lvl3(hostport)
				if len(hostport) == 2 {
					addr = hostport[0] + ":" + opts.Port
				}
				dbg.Lvl3(addr)
			} else if len(opts.Hostnames) != 0 {
				addr = opts.Hostnames[i]
			}

			nameToAddr[h] = addr
			// add to the hosts list if we havent added it before
			if _, ok := hc.Hosts[addr]; !ok {
				// only create the tcp hosts requested
				if opts.Host == "" || opts.Host == addr {
					if opts.Faulty == true {
						tcpHost := coconet.NewTCPHost(addr)
						hosts[addr] = coconet.NewFaultyHost(tcpHost)
					} else {
						hosts[addr] = coconet.NewTCPHost(addr)
					}
				} else {
					hosts[addr] = nil // it is there but not backed
				}
			}
		}
	}
	suite := edwards.NewAES128SHA256Ed25519(true)
	//suite := nist.NewAES128SHA256P256()
	if opts.Suite != nil {
		suite = opts.Suite
	}
	rand := suite.Cipher([]byte("example"))
	//fmt.Println("hosts", hosts)
	_, err = ConstructTree(cf.Tree, hc, "", suite, rand, hosts, nameToAddr, opts)
	if connT != GoC {
		hc.Dir = nil
	}

	dbg.Lvl3("IN LOAD JSON")
	// add a hostlist to each of the signing nodes
	var hostList []string
	for h := range hosts {
		hostList = append(hostList, h)
	}
	for _, sn := range hc.SNodes {
		sn.HostList = make([]string, len(hostList))
		sortable := sort.StringSlice(hostList)
		sortable.Sort()
		copy(sn.HostList, sortable)
		// set host list on view 0
		//dbg.Lvl3("in config hostlist", sn.HostList)
		sn.SetHostList(0, sn.HostList)
	}

	return hc, err
}

