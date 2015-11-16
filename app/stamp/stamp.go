package main

import (
	"errors"
	log "github.com/Sirupsen/logrus"
	"github.com/dedis/cothority/lib/app"
	"github.com/dedis/cothority/lib/coconet"
	dbg "github.com/dedis/cothority/lib/debug_lvl"
	"github.com/dedis/cothority/lib/graphs"
	"github.com/dedis/cothority/proto/sign"
	"io/ioutil"
	"net"
	"os"
	"strconv"
	"time"
	"github.com/dedis/cothority/lib/monitor"
)

func main() {
	conf := &app.ConfigColl{}
	app.ReadConfig(conf)

	switch app.RunFlags.Mode {
	case "server":
		RunServer(&app.RunFlags, conf)
	case "client":
		RunClient(&app.RunFlags, conf)
	}
}

func RunServer(Flags *app.Flags, conf *app.ConfigColl) {
	hostname := Flags.Hostname

	dbg.Lvl3(Flags.Hostname, "Starting to run")
	if conf.Debug > 1 {
		sign.DEBUG = true
	}

	// fmt.Println("EXEC TIMESTAMPER: " + hostname)
	if Flags.Hostname == "" {
		log.Fatal("no hostname given")
	}

	// load the configuration
	//dbg.Lvl3("loading configuration")
	var hc *graphs.HostConfig
	var err error
	s := app.GetSuite(conf.Suite)
	opts := graphs.ConfigOptions{ConnType: "tcp", Host: hostname, Suite: s}
	if conf.Failures > 0 || conf.FFail > 0 {
		opts.Faulty = true
	}

	hc, err = graphs.LoadConfig(conf.Hosts, conf.Tree, s, opts)
	if err != nil {
		dbg.Fatal(err)
	}

	for i := range hc.SNodes {
		// set FailureRates
		if conf.Failures > 0 {
			hc.SNodes[i].FailureRate = conf.Failures
		}
		// set root failures
		if conf.RFail > 0 {
			hc.SNodes[i].FailAsRootEvery = conf.RFail
		}
		// set follower failures
		// a follower fails on %ffail round with failureRate probability
		hc.SNodes[i].FailAsFollowerEvery = conf.FFail
	}

	// Wait for everybody to be ready before going on
	ioutil.WriteFile("coll_stamp_up/up" + hostname, []byte("started"), 0666)
	for {
		_, err := os.Stat("coll_stamp_up")
		if err == nil {
			dbg.Lvl4(hostname, "waiting for others to finish")
			time.Sleep(time.Second)
		} else {
			break
		}
	}
	dbg.Lvl3(hostname, "thinks everybody's here")

	err = hc.Run(true, sign.MerkleTree, hostname)
	if err != nil {
		log.Fatal(err)
	}

	defer func(sn *sign.Node) {
		dbg.Lvl3("Program timestamper has terminated:", hostname)
		sn.Close()
	}(hc.SNodes[0])

	stampers, _, err := RunTimestamper(hc, 0, conf, hostname)
	// get rid of the hc information so it can be GC'ed
	hc = nil
	if err != nil {
		log.Fatal(err)
	}
	for _, s := range stampers {
		// only listen if this is the hostname specified
		if s.Name() == hostname {
			s.Logger = Flags.Logger
			s.Hostname = hostname
			s.App = "stamp"
			if s.IsRoot(0) {
				if app.RunFlags.Logger == "" {
					monitor.Disable()
				} else {
					if err := monitor.ConnectSink(app.RunFlags.Logger); err != nil {
						dbg.Fatal("Root could not connect to monitor sink :", err)
					}
				}

				dbg.Lvl1("Root timestamper at:", hostname, conf.Rounds, "Waiting: ", conf.RootWait)
				// wait for the other nodes to get set up
				time.Sleep(time.Duration(conf.RootWait) * time.Second)

				dbg.Lvl1("Starting root-round")
				s.Run("root", conf.Rounds)
				// dbg.Lvl3("\n\nROOT DONE\n\n")

			} else if !conf.TestConnect {
				dbg.Lvl3("Running regular timestamper on:", hostname)
				s.Run("regular", conf.Rounds)
				// dbg.Lvl1("\n\nREGULAR DONE\n\n")
			} else {
				// testing connection
				dbg.Lvl1("Running connection-test on:", hostname)
				s.Run("test_connect", conf.Rounds)
			}
		}
	}
}

// run each host in hostnameSlice with the number of clients given
func RunTimestamper(hc *graphs.HostConfig, nclients int, conf *app.ConfigColl, hostnameSlice ...string) ([]*Server, []*Client, error) {
	dbg.Lvl3("RunTimestamper on", hc.Hosts)
	hostnames := make(map[string]*sign.Node)
	// make a list of hostnames we want to run
	if hostnameSlice == nil {
		hostnames = hc.Hosts
	} else {
		for _, h := range hostnameSlice {
			sn, ok := hc.Hosts[h]
			if !ok {
				return nil, nil, errors.New("hostname given not in config file:" + h)
			}
			hostnames[h] = sn
		}
	}

	Clients := make([]*Client, 0, len(hostnames) * nclients)
	// for each client in
	stampers := make([]*Server, 0, len(hostnames))
	for _, sn := range hc.SNodes {
		if _, ok := hostnames[sn.Name()]; !ok {
			dbg.Lvl1("signing node not in hostnmaes")
			continue
		}
		stampers = append(stampers, NewServer(conf, sn))
		if hc.Dir == nil {
			dbg.Lvl3(hc.Hosts, "listening for clients")
			stampers[len(stampers) - 1].Listen()
		}
	}
	dbg.Lvl3("stampers:", stampers)
	clientsLists := make([][]*Client, len(hc.SNodes[1:]))
	for i, s := range stampers[1:] {
		// cant assume the type of connection
		clients := make([]*Client, nclients)

		h, p, err := net.SplitHostPort(s.Name())
		if hc.Dir != nil {
			h = s.Name()
		} else if err != nil {
			log.Fatal("RunTimestamper: bad Tcp host")
		}
		pn, err := strconv.Atoi(p)
		if hc.Dir != nil {
			pn = 0
		} else if err != nil {
			log.Fatal("port is not valid integer")
		}
		hp := net.JoinHostPort(h, strconv.Itoa(pn + 1))
		//dbg.Lvl4("client connecting to:", hp)

		for j := range clients {
			clients[j] = NewClient("client" + strconv.Itoa((i - 1) * len(stampers) + j))
			dbg.Lvl3("Created a new client from stamp.go")
			var c coconet.Conn

			// if we are using tcp connections
			if hc.Dir == nil {
				// the timestamp server serves at the old port + 1
				dbg.Lvl4("new tcp conn")
				c = coconet.NewTCPConn(hp)
			} else {
				dbg.Lvl4("new go conn")
				c, _ = coconet.NewGoConn(hc.Dir, clients[j].Name(), s.Name())
				stoc, _ := coconet.NewGoConn(hc.Dir, s.Name(), clients[j].Name())
				s.Clients[clients[j].Name()] = stoc
			}
			// connect to the server from the client
			// This will connect to stamper server and waits for response.
			// Sending stamp request is done in client.go..... ><
			clients[j].AddServer(s.Name(), c)
			//clients[j].Sns[s.Name()] = c
			//clients[j].Connect()
		}
		Clients = append(Clients, clients...)
		clientsLists[i] = clients
	}

	return stampers, Clients, nil
}
