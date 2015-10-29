package monitor

import (
	"encoding/json"
	"fmt"
	dbg "github.com/dedis/cothority/lib/debug_lvl"
	"net"
)

// Implements a simple proxy
// A <-> D <-> B
// D is the proxy. It will listen for incoming connections on the side of B
// And will connect to A

// serverConn is the connection object to the server
var serverConn net.Conn

// to write back the measure to the server
var serverEnc *json.Encoder

// proxy connections opened
var proxyConns []net.Conn

var proxyDone chan bool

func init() {
	proxyDone = make(chan bool)
}

// connectServer will try to connect to the server ...
func connectSink(redirection string) error {
	conn, err := net.Dial("tcp", redirection)
	if err != nil {
		return fmt.Errorf("Proxy connection to server %s failed : %v", redirection, err)
	}
	serverConn = conn
	serverEnc = json.NewEncoder(conn)
	return nil
}

// Proxy will launch a routines that waits for input connections
// It takes a redirection address soas to where redirect incoming packets
// Proxy will listen on Sink:SinkPort variables so that the user do not
// differentiate between connecting to a proxy or directly to the sink
// It will panic if it can not contact the server or can not bind to the address
func Proxy(redirection string) {
	// Connect to the sink
	if err := connectSink(redirection); err != nil {
		panic(err)
	}
	dbg.Lvl2("Proxy connected to sink ", redirection)
	// Here it listens the same way monitor.go would
	// usually 0.0.0.0:4000
	ln, err := net.Listen("tcp", Sink+":"+SinkPort)
	if err != nil {
		panic(fmt.Errorf("Error while binding proxy to addr %s : %v", Sink+":"+SinkPort, err))
	}
	dbg.Lvl2("Proxy listening on ", Sink+":"+SinkPort)
	var newConn = make(chan bool)
	var closeConn = make(chan bool)
	var finished = false
	// Listen for every incoming connections
	go func() {
		for finished == false {
			conn, err := ln.Accept()
			if err != nil {
				operr, ok := err.(*net.OpError)
				// the listener is closed
				if ok && operr.Op == "accept" {
					break
				}
				dbg.Lvl1("Error proxy accepting connection : ", err)
				continue
			}
			dbg.Lvl2("Proxy accepting incoming connection from : ", conn.RemoteAddr().String())
			newConn <- true
			go proxyConnection(conn, closeConn)
		}
	}()

	// notify every new connection and every end of connections. When every
	// conenctions are ended, send an "end" measure to the sink.
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
}

// The core of the file : read any input from the connection and outputs it into
// the server connection
func proxyConnection(conn net.Conn, done chan bool) {
	dec := json.NewDecoder(conn)
	nerr := 0
	m := Measure{}
	for {
		// Receive data
		if err := dec.Decode(&m); err != nil {
			dbg.Lvl1("Error receiving data from ", conn.RemoteAddr().String(), " : ", err)
			nerr += 1
			if nerr > 1 {
				dbg.Lvl1("Too many error from ", conn.RemoteAddr().String(), " : Abort connection")
				break
			}
		}
		if m.Name == "end" {
			// the end
			dbg.Lvl2("Proxy detected end of measurement. Closing conn")
			break
		}
		// Proxy data
		if err := serverEnc.Encode(m); err != nil {
			dbg.Lvl2("Error proxying data :", err)
			break
		}
		m = Measure{}
	}
	conn.Close()
	done <- true
}

// proxyDataServer send the data to the server...
func proxyDataServer(data []byte) {
	_, err := serverConn.Write(data)
	if err != nil {
		panic(fmt.Errorf("Error proxying data to server : %v", err))
	}
}
