package network

import (
	"strconv"
	"sync"
	"testing"

	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/crypto/abstract"
	"golang.org/x/net/context"
)

/*
On MacOSX, for maximum number of hosts, use
http://b.oldhu.com/2012/07/19/increase-tcp-max-connections-on-mac-os-x/
sudo sysctl -w kern.maxfiles=12288
sudo sysctl -w kern.maxfilesperproc=10240
ulimit -n 10240
sudo sysctl -w kern.ipc.somaxconn=2048
*/

// There seems to be an error if a lot of hosts communicate with each other
// - this function tries to trigger that error so that it can be removed
// It generates one connection between each host and then starts sending
// messages all around.
func TestHugeConnections(t *testing.T) {
	defer dbg.AfterTest(t)
	// How many hosts are run
	nbrHosts := 10
	// 16MB of message size
	msgSize := 1024 * 1024 * 1
	big := bigMessage{
		Msize: msgSize,
		Msg:   make([]byte, msgSize),
		Pcrc:  25,
	}
	bigMessageType := RegisterMessageType(big)

	dbg.TestOutput(testing.Verbose(), 3)
	privkeys := make([]abstract.Secret, nbrHosts)
	ids := make([]*Entity, nbrHosts)
	hosts := make([]SecureHost, nbrHosts)
	// 2-dimensional array of connections between all hosts, where only
	// the upper-right half is populated. The lower-left half is the
	// mirror of the upper-right half, and the diagonal is empty, as there
	// are no connections from one host to itself.
	conns := make([][]SecureConn, nbrHosts)
	wg := sync.WaitGroup{}
	// Create all hosts and open the connections
	for i := 0; i < nbrHosts; i++ {
		privkeys[i], ids[i] = genEntity("localhost:" + strconv.Itoa(2000+i))
		hosts[i] = NewSecureTCPHost(privkeys[i], ids[i])
		dbg.Lvl5("Host is", hosts[i], "id is", ids[i])
		go func(h int) {
			err := hosts[h].Listen(func(c SecureConn) {
				dbg.Lvl5(2000+h, "got a connection")
				nm, err := c.Receive(context.TODO())
				if err != nil {
					t.Fatal("Couldn't receive msg:", err)
				}
				if nm.MsgType != bigMessageType {
					t.Fatal("Received message type is wrong")
				}
				big_copy := nm.Msg.(bigMessage)
				if big_copy.Msize != msgSize {
					t.Fatal(h, "Message-size is wrong:", big_copy.Msize, big_copy, big)
				}
				if big_copy.Pcrc != 25 {
					t.Fatal("CRC is wrong")
				}
				// And send it back
				dbg.Lvl3(h, "sends it back")

				go func(h int) {
					dbg.Lvl3(h, "Sending back")
					err := c.Send(context.TODO(), &big)
					if err != nil {
						t.Fatal(h, "couldn't send message:", err)
					}
				}(h)
				dbg.Lvl3(h, "done sending messages")
			})
			if err != nil {
				t.Fatal(err)
			}
		}(i)
		conns[i] = make([]SecureConn, nbrHosts)
		for j := 0; j < i; j++ {
			wg.Add(1)
			var err error
			dbg.Lvl5("Connecting", ids[i], "with", ids[j])
			conns[i][j], err = hosts[i].Open(ids[j])
			if err != nil {
				t.Fatal("Couldn't open:", err)
			}
			// Populate also the lower left for easy sending to
			// everybody
			conns[j][i] = conns[i][j]
		}
	}

	// Start sending messages back and forth
	for i := 0; i < nbrHosts; i++ {
		for j := 0; j < i; j++ {
			c := conns[i][j]
			go func(conn SecureConn, i, j int) {
				defer wg.Done()
				dbg.Lvl3("Sending from", i, "to", j, ":")
				ctx := context.TODO()
				if err := conn.Send(ctx, &big); err != nil {
					t.Fatal(i, j, "Couldn't send:", err)
				}
				nm, err := conn.Receive(context.TODO())
				if err != nil {
					t.Fatal(i, j, "Couldn't receive:", err)
				}
				bc := nm.Msg.(bigMessage)
				if bc.Msize != msgSize {
					t.Fatal(i, j, "Message-size is wrong")
				}
				if bc.Pcrc != 25 {
					t.Fatal(i, j, "CRC is wrong")
				}
				dbg.Lvl3(i, j, "Done")
			}(c, i, j)
		}
	}
	wg.Wait()

	// Close all
	for _, h := range hosts {
		if err := h.Close(); err != nil {
			t.Fatal("Couldn't close:", err)
		}
	}
}

type bigMessage struct {
	Msize int
	Msg   []byte
	Pcrc  int
}
