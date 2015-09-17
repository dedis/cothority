package stamp_test

import (
	"fmt"
	"strconv"
	"sync"
	"testing"
	"time"

	log "github.com/Sirupsen/logrus"

	"github.com/dedis/cothority/coconet"
	"github.com/dedis/cothority/sign"
	"github.com/dedis/cothority/stamp"
	"github.com/dedis/cothority/lib/oldconfig"
)

// TODO: messages should be sent hashed eventually

// func init() {
// 	log.SetFlags(log.Lshortfile)
// 	//log.SetOutput(ioutil.Discard)
// }

// Configuration file data/exconf.json
//       0
//      / \
//     1   4
//    / \   \
//   2   3   5
func init() {
	sign.DEBUG = true
}

func TestTSSIntegrationHealthy(t *testing.T) {
	failAsRootEvery := 0     // never fail on announce
	failAsFollowerEvery := 0 // never fail on commit or response
	RoundsPerView := 100
	if err := runTSSIntegration(RoundsPerView, 4, 5, 0, failAsRootEvery, failAsFollowerEvery); err != nil {
		t.Fatal(err)
	}
}

func TestTSSIntegrationFaulty(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping faulty test in short mode.")
	}

	// not mixing view changes with faults
	RoundsPerView := 100
	failAsRootEvery := 0     // never fail on announce
	failAsFollowerEvery := 0 // never fail on commit or response

	faultyNodes := make([]int, 0)
	faultyNodes = append(faultyNodes, 2, 5)
	if err := runTSSIntegration(RoundsPerView, 4, 4, 20, failAsRootEvery, failAsFollowerEvery, faultyNodes...); err != nil {
		t.Fatal(err)
	}
}

func TestTSSViewChange1(t *testing.T) {
	RoundsPerView := 2
	nRounds := 12
	failAsRootEvery := 0     // never fail on announce
	failAsFollowerEvery := 0 // never fail on commit or response

	if err := runTSSIntegration(RoundsPerView, 1, nRounds, 0, failAsRootEvery, failAsFollowerEvery); err != nil {
		t.Fatal(err)
	}
}

func TestTSSViewChange2(t *testing.T) {
	RoundsPerView := 3
	nRounds := 8
	failAsRootEvery := 0     // never fail on announce
	failAsFollowerEvery := 0 // never fail on commit or response

	if err := runTSSIntegration(RoundsPerView, 1, nRounds, 0, failAsRootEvery, failAsFollowerEvery); err != nil {
		t.Fatal(err)
	}
}

// Each View Root fails on its 3rd round of being root
// View Change is initiated as a result
// RoundsPerView very large to avoid other reason for ViewChange
func TestTSSViewChangeOnRootFailure(t *testing.T) {
	RoundsPerView := 1000
	nRounds := 12
	failAsRootEvery := 3     // fail on announce every 3rd round
	failAsFollowerEvery := 0 // never fail on commit or response

	if err := runTSSIntegration(RoundsPerView, 1, nRounds, 0, failAsRootEvery, failAsFollowerEvery); err != nil {
		t.Fatal(err)
	}
}

// Faulty Followers fail every 3rd round
func TestTSSViewChangeOnFollowerFailureNoRate(t *testing.T) {
	RoundsPerView := 1000
	nRounds := 12
	failAsRootEvery := 0 // never fail on announce
	// selected faultyNodes will fail on commit and response every 3 rounds
	// if they are followers in the view
	failAsFollowerEvery := 3

	faultyNodes := make([]int, 0)
	faultyNodes = append(faultyNodes, 2, 3)
	if err := runTSSIntegration(RoundsPerView, 1, nRounds, 0, failAsRootEvery, failAsFollowerEvery, faultyNodes...); err != nil {
		t.Fatal(err)
	}
}

// Faulty Followers fail every 3rd round, with probability failureRate%
func TestTSSViewChangeOnFollowerFailureWithRate(t *testing.T) {
	RoundsPerView := 1000
	nRounds := 12
	failAsRootEvery := 0 // never fail on announce
	failureRate := 10
	// selected faultyNodes will fail on commit and response every 3 rounds
	// if they are followers in the view
	failAsFollowerEvery := 3

	faultyNodes := make([]int, 0)
	faultyNodes = append(faultyNodes, 2, 3)
	if err := runTSSIntegration(RoundsPerView, 1, nRounds, failureRate, failAsRootEvery, failAsFollowerEvery, faultyNodes...); err != nil {
		t.Fatal(err)
	}
}

// # Messages per round, # rounds, failure rate[0..100], list of faulty nodes
func runTSSIntegration(RoundsPerView, nMessages, nRounds, failureRate, failAsRootEvery, failAsFollowerEvery int, faultyNodes ...int) error {
	//stamp.ROUND_TIME = 1 * time.Second
	var hostConfig *oldconfig.HostConfig
	var err error

	// load config with faulty or healthy hosts
	opts := oldconfig.ConfigOptions{}
	if len(faultyNodes) > 0 {
		opts.Faulty = true
	}
	hostConfig, err = oldconfig.LoadConfig("../test/data/exconf.json", opts)
	if err != nil {
		return err
	}
	log.Printf("load config returned dir: %p", hostConfig.Dir)

	// set FailureRates as pure percentages
	if len(faultyNodes) > 0 {
		for i := range hostConfig.SNodes {
			hostConfig.SNodes[i].FailureRate = failureRate
		}
	}

	// set root failures
	if failAsRootEvery > 0 {
		for i := range hostConfig.SNodes {
			hostConfig.SNodes[i].FailAsRootEvery = failAsRootEvery

		}
	}
	// set followerfailures
	for _, f := range faultyNodes {
		hostConfig.SNodes[f].FailAsFollowerEvery = failAsFollowerEvery
	}

	for _, n := range hostConfig.SNodes {
		n.RoundsPerView = RoundsPerView
	}

	err = hostConfig.Run(true, sign.MerkleTree)
	if err != nil {
		return err
	}

	// Connect all TSServers to their clients, except for root TSServer
	ncps := 3 // # clients per TSServer
	stampers := make([]*stamp.Server, len(hostConfig.SNodes))
	for i := range stampers {
		stampers[i] = stamp.NewServer(hostConfig.SNodes[i])
		defer func() {
			hostConfig.SNodes[i].Close()
			time.Sleep(1 * time.Second)
		}()
	}

	clientsLists := make([][]*stamp.Client, len(hostConfig.SNodes[1:]))
	for i, s := range stampers[1:] {
		clientsLists[i] = createClientsForTSServer(ncps, s, hostConfig.Dir, 0+i+ncps)
	}

	for i, s := range stampers[1:] {
		go s.Run("regular", nRounds)
		go s.ListenToClients()
		go func(clients []*stamp.Client, nRounds int, nMessages int, s *stamp.Server) {
			log.Println("clients Talk")
			time.Sleep(1 * time.Second)
			clientsTalk(clients, nRounds, nMessages, s)
			log.Println("Clients done Talking")
		}(clientsLists[i], nRounds, nMessages, s)

	}

	log.Println("RUNNING ROOT")
	stampers[0].ListenToClients()
	stampers[0].Run("root", nRounds)
	log.Println("Done running root")
	// After clients receive messages back we need a better way
	// of waiting to make sure servers check ElGamal sigs
	// time.Sleep(1 * time.Second)
	log.Println("DONE with test")
	return nil
}

func TestGoConnTimestampFromConfig(t *testing.T) {
	oldconfig.StartConfigPort += 2010
	nMessages := 1
	nClients := 1
	nRounds := 1

	hc, err := oldconfig.LoadConfig("../test/data/exconf.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range hc.SNodes {
		n.RoundsPerView = 1000
	}
	err = hc.Run(true, sign.MerkleTree)
	if err != nil {
		t.Fatal(err)
	}

	stampers, clients, err := hc.RunTimestamper(nClients)
	if err != nil {
		log.Fatal(err)
	}

	for _, s := range stampers[1:] {
		go s.Run("regular", nRounds)
		go s.ListenToClients()
	}
	go stampers[0].Run("root", nRounds)
	go stampers[0].ListenToClients()
	log.Println("About to start sending client messages")

	time.Sleep(1 * time.Second)
	for r := 0; r < nRounds; r++ {
		var wg sync.WaitGroup
		for _, c := range clients {
			for i := 0; i < nMessages; i++ {
				messg := []byte("messg:" + strconv.Itoa(r) + "." + strconv.Itoa(i))
				wg.Add(1)
				go func(c *stamp.Client, messg []byte, i int) {
					defer wg.Done()
					server := "NO VALID SERVER"
					c.Mux.Lock()
					for k := range c.Servers {
						server = k
						break
					}
					c.Mux.Unlock()
					c.TimeStamp(messg, server)
				}(c, messg, r)
			}
		}
		// wait between rounds
		wg.Wait()
		fmt.Println("done with round:", r, nRounds)
	}

	// give it some time before closing the connections
	// so that no essential messages are denied passing through the network
	time.Sleep(5 * time.Second)
	for _, h := range hc.SNodes {
		h.Close()
	}
	for _, c := range clients {
		c.Close()
	}
}

func TestTCPTimestampFromConfigViewChange(t *testing.T) {
	RoundsPerView := 5
	if err := runTCPTimestampFromConfig(RoundsPerView, sign.MerkleTree, 1, 1, 5, 0); err != nil {
		t.Fatal(err)
	}
}

func TestTCPTimestampFromConfigHealthy(t *testing.T) {
	RoundsPerView := 5
	if err := runTCPTimestampFromConfig(RoundsPerView, sign.MerkleTree, 1, 1, 5, 0); err != nil {
		t.Fatal(err)
	}
}

func TestTCPTimestampFromConfigFaulty(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping faulty test in short mode.")
	}

	// not mixing view changes with faults
	RoundsPerView := 100
	// not mixing view changes with faults
	aux2 := sign.HEARTBEAT
	sign.HEARTBEAT = 4 * sign.ROUND_TIME

	faultyNodes := make([]int, 0)
	faultyNodes = append(faultyNodes, 2, 5)
	if err := runTCPTimestampFromConfig(RoundsPerView, sign.MerkleTree, 1, 1, 5, 20, faultyNodes...); err != nil {
		t.Fatal(err)
	}

	sign.HEARTBEAT = aux2
}

func TestTCPTimestampFromConfigVote(t *testing.T) {
	// not mixing view changes with faults
	RoundsPerView := 3
	// not mixing view changes with faults
	aux2 := sign.HEARTBEAT
	sign.HEARTBEAT = 4 * sign.ROUND_TIME

	if err := runTCPTimestampFromConfig(RoundsPerView, sign.Voter, 0, 0, 15, 0); err != nil {
		t.Fatal(err)
	}

	sign.HEARTBEAT = aux2
}

func runTCPTimestampFromConfig(RoundsPerView int, signType, nMessages, nClients, nRounds, failureRate int, faultyNodes ...int) error {
	var hc *oldconfig.HostConfig
	var err error
	oldconfig.StartConfigPort += 2010

	// load config with faulty or healthy hosts
	if len(faultyNodes) > 0 {
		hc, err = oldconfig.LoadConfig("../test/data/extcpconf.json", oldconfig.ConfigOptions{ConnType: "tcp", GenHosts: true, Faulty: true})
	} else {
		hc, err = oldconfig.LoadConfig("../test/data/extcpconf.json", oldconfig.ConfigOptions{ConnType: "tcp", GenHosts: true})
	}
	if err != nil {
		return err
	}

	// set FailureRates
	if len(faultyNodes) > 0 {
		for i := range hc.SNodes {
			hc.SNodes[i].FailureRate = failureRate
		}
	}

	for _, n := range hc.SNodes {
		n.RoundsPerView = RoundsPerView
	}

	err = hc.Run(true, sign.Type(signType))
	if err != nil {
		return err
	}

	stampers, clients, err := hc.RunTimestamper(nClients)
	if err != nil {
		return err
	}

	for _, s := range stampers[1:] {
		go s.Run("regular", nRounds)
	}
	go stampers[0].Run("root", nRounds)
	log.Println("About to start sending client messages")

	for r := 1; r <= nRounds; r++ {
		var wg sync.WaitGroup
		for _, c := range clients {
			for i := 0; i < nMessages; i++ {
				messg := []byte("messg:" + strconv.Itoa(r) + "." + strconv.Itoa(i))
				wg.Add(1)

				// CLIENT SENDING
				go func(c *stamp.Client, messg []byte, i int) {
					defer wg.Done()
					server := "NO VALID SERVER"

				retry:
					c.Mux.Lock()
					for k := range c.Servers {
						server = k
						break
					}
					c.Mux.Unlock()
					log.Infoln("timestamping")
					err := c.TimeStamp(messg, server)
					if err == stamp.ErrClientToTSTimeout {
						log.Errorln(err)
						return
					}
					if err != nil {
						time.Sleep(1 * time.Second)
						fmt.Println("retyring because err:", err)
						goto retry
					}
					log.Infoln("timestamped")
				}(c, messg, r)

			}
		}
		// wait between rounds
		wg.Wait()
		log.Println("done with round:", r, " of ", nRounds)
	}

	// give it some time before closing the connections
	// so that no essential messages are denied passing through the network
	time.Sleep(1 * time.Second)
	for _, h := range hc.SNodes {
		h.Close()
	}
	for _, c := range clients {
		c.Close()
	}
	return nil
}

// Create nClients for the TSServer, with first client associated with number fClient
func createClientsForTSServer(nClients int, s *stamp.Server, dir *coconet.GoDirectory, fClient int) []*stamp.Client {
	clients := make([]*stamp.Client, 0, nClients)
	for i := 0; i < nClients; i++ {
		clients = append(clients, stamp.NewClient("client"+strconv.Itoa(fClient+i)))

		// intialize TSServer conn to client
		ngc, err := coconet.NewGoConn(dir, s.Name(), clients[i].Name())
		if err != nil {
			panic(err)
		}
		s.Clients[clients[i].Name()] = ngc

		// intialize client connection to sn
		ngc, err = coconet.NewGoConn(dir, clients[i].Name(), s.Name())
		if err != nil {
			panic(err)
		}
		clients[i].AddServer(s.Name(), ngc)
	}

	return clients
}

func clientsTalk(clients []*stamp.Client, nRounds, nMessages int, s *stamp.Server) {
	// have client send messages
	for r := 0; r < nRounds; r++ {
		var wg sync.WaitGroup
		for _, client := range clients {
			for i := 0; i < nMessages; i++ {
				messg := []byte("messg" + strconv.Itoa(r) + strconv.Itoa(i))
				wg.Add(1)
				go func(client *stamp.Client, messg []byte, s *stamp.Server, i int) {
					defer wg.Done()
					client.TimeStamp(messg, s.Name())
				}(client, messg, s, r)
			}
		}
		// wait between rounds
		wg.Wait()
	}
}
