package main

import (
	"bytes"
	"errors"
	"io"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
	dbg "github.com/dedis/cothority/lib/debug_lvl"

	"fmt"
	"github.com/dedis/cothority/lib/coconet"
)

type Client struct {
	Mux sync.Mutex // coarse grained mutex

	name    string
	Servers map[string]coconet.Conn // signing nodes I work/ communicate with

	// client history maps request numbers to replies from TSServer
	// maybe at later phases we will want pair(reqno, TSServer) as key
	history map[SeqNo]TimeStampMessage
	reqno   SeqNo // next request number in communications with TSServer

	// maps response request numbers to channels confirming
	// where response confirmations are sent
	doneChan map[SeqNo]chan error

	nRounds     int    // # of last round messages were received in, as perceived by client
	curRoundSig []byte // merkle tree root of last round
	// roundChan   chan int // round numberd are sent in as rounds change
	Error error
}

func NewClient(name string) (c *Client) {
	c = &Client{name: name}
	c.Servers = make(map[string]coconet.Conn)
	c.history = make(map[SeqNo]TimeStampMessage)
	c.doneChan = make(map[SeqNo]chan error)
	// c.roundChan = make(chan int)
	return
}

func (c *Client) Name() string {
	return c.name
}

func (c *Client) Close() {
	for _, c := range c.Servers {
		c.Close()
	}
}

func (c *Client) handleServer(s coconet.Conn) error {
	for {
		tsm := &TimeStampMessage{}
		err := s.GetData(tsm)
		if err != nil {
			if err == coconet.ErrNotEstablished {
				continue
			}
			log.Warn("error getting from connection:", err)
			return err
		}
		c.handleResponse(tsm)
	}
}

// Act on type of response received from srrvr
func (c *Client) handleResponse(tsm *TimeStampMessage) {
	switch tsm.Type {
	default:
		log.Println("Message of unknown type")
	case StampReplyType:
		// Process reply and inform done channel associated with
		// reply sequence number that the reply was received
		// we know that there is no error at this point
		c.ProcessStampReply(tsm)

	}
}

func (c *Client) AddServer(name string, conn coconet.Conn) {
	//c.Servers[name] = conn
	go func(conn coconet.Conn) {
		maxwait := 30 * time.Second
		curWait := 100 * time.Millisecond
		for {
			err := conn.Connect()
			if err != nil {
				time.Sleep(curWait)
				curWait = curWait * 2
				if curWait > maxwait {
					curWait = maxwait
				}
				continue
			} else {
				c.Mux.Lock()
				c.Servers[name] = conn
				c.Mux.Unlock()
				dbg.Lvl3("Success: connected to server:", conn)
				err := c.handleServer(conn)
				// if a server encounters any terminating error
				// terminate all pending client transactions and kill the client
				if err != nil {
					dbg.Lvl2("EOF detected: sending EOF to all pending TimeStamps")
					c.Mux.Lock()
					for _, ch := range c.doneChan {
						dbg.Lvl2("Sending to Receiving Channel")
						ch <- io.EOF
					}
					c.Error = io.EOF
					c.Mux.Unlock()
					return
				} else {
					// try reconnecting if it didn't close the channel
					continue
				}
			}
		}
	}(conn)
}

// Send data to server given by name (data should be a timestamp request)
func (c *Client) PutToServer(name string, data coconet.BinaryMarshaler) error {
	c.Mux.Lock()
	defer c.Mux.Unlock()
	conn := c.Servers[name]
	if conn == nil {
		return errors.New(fmt.Sprintf("Invalid server/not connected", name, c.Servers[name]))
	}
	return conn.PutData(data)
}

var ErrClientToTSTimeout error = errors.New("client timeouted on waiting for response")

// When client asks for val to be timestamped
// It blocks until it get a coll_stamp reply back
func (c *Client) TimeStamp(val []byte, TSServerName string) error {
	c.Mux.Lock()
	if c.Error != nil {
		c.Mux.Unlock()
		return c.Error
	}
	c.reqno++
	myReqno := c.reqno
	c.doneChan[c.reqno] = make(chan error, 1) // new done channel for new req
	c.Mux.Unlock()
	// send request to TSServer
	err := c.PutToServer(TSServerName,
		&TimeStampMessage{
			Type:  StampRequestType,
			ReqNo: myReqno,
			Sreq:  &StampRequest{Val: val}})
	if err != nil {
		if err != coconet.ErrNotEstablished {
			dbg.Lvl3(c.Name(), "error timestamping to ", TSServerName, ": ", err)
		}
		// pass back up all errors from putting to server
		return err
	}
	dbg.Lvl4("Client Sent timestamp request to", TSServerName)

	// get channel associated with request
	c.Mux.Lock()
	myChan := c.doneChan[myReqno]
	c.Mux.Unlock()

	// wait until ProcessStampReply signals that reply was received
	select {
	case err = <-myChan:
		//log.Println("-------------client received  response from" + TSServerName)
		break
	case <-time.After(10 * ROUND_TIME):
		dbg.Lvl3("client timeouted on waiting for response from" + TSServerName)
		break
		// err = ErrClientToTSTimeout
	}
	if err != nil {
		dbg.Lvl3(c.Name(), "error received from DoneChan:", err)
		return err
	}

	// delete channel as it is of no longer meaningful
	c.Mux.Lock()
	delete(c.doneChan, myReqno)
	c.Mux.Unlock()
	return err
}

func (c *Client) ProcessStampReply(tsm *TimeStampMessage) {
	// update client history
	c.Mux.Lock()
	c.history[tsm.ReqNo] = *tsm
	done := c.doneChan[tsm.ReqNo]

	// can keep track of rounds by looking at changes in the signature
	// sent back in a messages
	if bytes.Compare(tsm.Srep.Sig, c.curRoundSig) != 0 {
		c.curRoundSig = tsm.Srep.Sig
		c.nRounds++

		c.Mux.Unlock()
		//c.roundChan <- c.nRounds
	} else {
		c.Mux.Unlock()
	}
	done <- nil
}
