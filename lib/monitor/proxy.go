package monitor

import (
	"encoding/json"
	"fmt"
	"github.com/dedis/cothority/lib/dbg"
	"io"
	"net"
	"strconv"
	"sync/atomic"
)

// Implements a simple proxy
// A <-> D <-> B
// D is the proxy. It will listen for incoming connections on the side of B
// And will connect to A

// serverConn is the connection object to the server
var serverConn net.Conn

// to write back the measure to the server
var serverEnc *json.Encoder
var serverDec *json.Decoder
var readyCount int64

// proxy connections opened
var proxyConns map[string]*json.Encoder

// Proxy will launch a routine that waits for input connections
// It takes a redirection address soas to where redirect incoming packets
// Proxy will listen on Sink:SinkPort variables so that the user do not
// differentiate between connecting to a proxy or directly to the sink
// It will panic if it can not contact the server or can not bind to the address
func Proxy(redirection string) error {
	// Connect to the sink
	if err := connectToSink(redirection); err != nil {
		return err
	}
	dbg.Lvl2("Proxy connected to sink", redirection)
	// The proxy listens on the port one lower than itself
	_, port, err := net.SplitHostPort(redirection)
	if err != nil {
		dbg.Fatal("Couldn't get port-numbre from", redirection)
	}
	portNbr, err := strconv.Atoi(port)
	if err != nil {
		dbg.Fatal("Couldn't convert", port, "to a number")
	}
	sinkAddr := Sink + ":" + strconv.Itoa(portNbr-1)
	ln, err := net.Listen("tcp", sinkAddr)
	if err != nil {
		return fmt.Errorf("Error while binding proxy to addr %s: %v", sinkAddr, err)
	}
	dbg.Lvl2("Proxy listening on", sinkAddr)
	newConn := make(chan bool)
	closeConn := make(chan bool)
	finished := false
	proxyConns = make(map[string]*json.Encoder)
	readyCount = 0

	// Listen for incoming connections
	go func() {
		for finished == false {
			conn, err := ln.Accept()
			if err != nil {
				operr, ok := err.(*net.OpError)
				// the listener is closed
				if ok && operr.Op == "accept" {
					break
				}
				dbg.Lvl1("Error proxy accepting connection:", err)
				continue
			}
			dbg.Lvl3("Proxy accepting incoming connection from:", conn.RemoteAddr().String())
			newConn <- true
			proxyConns[conn.RemoteAddr().String()] = json.NewEncoder(conn)
			go proxyConnection(conn, closeConn)
		}
	}()

	// Listen for replies and give them further
	go func() {
		for finished == false {
			m := Measure{}
			err := serverDec.Decode(&m)
			if err != nil {
				return
			}
			dbg.Lvlf3("Proxy received %+v", m)
			c, ok := proxyConns[m.Sender]
			if !ok {
				return
			}
			dbg.Lvl3("Found connection")
			c.Encode(m)
		}
	}()

	go func() {
		// notify every new connection and every end of connection. When all
		// connections are closed, send an "end" measure to the sink.
		var nconn int
		for finished == false {
			select {
			case <-newConn:
				nconn += 1
			case <-closeConn:
				nconn -= 1
				if nconn == 0 {
					// everything is finished
					serverEnc.Encode(Measure{Name: "end"})
					serverConn.Close()
					ln.Close()
					finished = true
					break
				}
			}
		}
	}()
	return nil
}

// connectToSink starts the connection with the server
func connectToSink(redirection string) error {
	conn, err := net.Dial("tcp", redirection)
	if err != nil {
		return fmt.Errorf("Proxy connection to server %s failed: %v", redirection, err)
	}
	serverConn = conn
	serverEnc = json.NewEncoder(conn)
	serverDec = json.NewDecoder(conn)
	return nil
}

// The core of the file: read any input from the connection and outputs it into
// the server connection
func proxyConnection(conn net.Conn, done chan bool) {
	dec := json.NewDecoder(conn)
	nerr := 0
	for {
		m := Measure{}
		// Receive data
		if err := dec.Decode(&m); err != nil {
			if err == io.EOF {
				break
			}
			dbg.Lvl1("Error receiving data from", conn.RemoteAddr().String(), ":", err)
			nerr += 1
			if nerr > 1 {
				dbg.Lvl1("Too many errors from", conn.RemoteAddr().String(), ": Abort connection")
				break
			}
		}
		dbg.Lvl3("Proxy received", m)

		// Implement our own ready-count, so it doesn't have to go through the
		// main monitor which might be far away.
		switch m.Name {
		case "ready":
			atomic.AddInt64(&readyCount, 1)
		case "ready_count":
			m.Ready = int(readyCount)
			err := json.NewEncoder(conn).Encode(m)
			if err != nil {
				dbg.Lvl2("Couldn't send ready-result back to client")
				break
			}
		default:
			// Proxy data - add who is sending, as we only have one channel
			// to the server
			m.Sender = conn.RemoteAddr().String()
			if err := serverEnc.Encode(m); err != nil {
				dbg.Lvl2("Error proxying data :", err)
				break
			}
			if m.Name == "end" {
				// the end
				dbg.Lvl2("Proxy detected end of measurement. Closing connection.")
				break
			}
		}
	}
	conn.Close()
	done <- true
}
