package monitor

import (
	"bytes"
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

// proxy connections opened
var proxyConns []net.Conn

// connectServer will try to connect to the server ...
func connectSink(redirection string) error {
	conn, err := net.Dial("tcp", redirection)
	if err != nil {
		return fmt.Errorf("Proxy connection to server %s failed : %v", redirection, err)
	}
	serverConn = conn
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
	proxyConns := make([]net.Conn, 0)
	for {
		conn, err := ln.Accept()
		if err != nil {
			dbg.Lvl1("Error proxy accepting connection : ", err)
			continue
		}
		dbg.Lvl2("Proxy accepting incoming connection from : ", conn.RemoteAddr().String())
		proxyConns = append(proxyConns, conn)
		go proxyConnection(conn)
	}
}

// The core of the file : read any input from the connection and outputs it into
// the server connection
func proxyConnection(conn net.Conn) {
	var buf [1024]byte
	nerr := 0
	for {
		// Receive data
		n, err := conn.Read(buf[:])
		if err != nil {
			dbg.Lvl1("Error receiving data from ", conn.RemoteAddr().String(), " : ", err)
			nerr += 1
			if nerr > 1 {
				dbg.Lvl1("Too many error from ", conn.RemoteAddr().String(), " : Abort connection")
				break
			}
		}
		if bytes.Contains(buf[:n], []byte("end")) {
			// the end
			conn.Close()
			dbg.Lvl2("Proxy detected end of measurement. Closing conn")
			break
		}
		// Proxy data
		_, err = serverConn.Write(buf[:n])
		if err != nil {
			dbg.Lvl1("Error proxying data :", err)
			break
		}
	}
}

// proxyDataServer send the data to the server...
func proxyDataServer(data []byte) {
	_, err := serverConn.Write(data)
	if err != nil {
		panic(fmt.Errorf("Error proxying data to server : %v", err))
	}
}
