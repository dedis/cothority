package monitor

import (
	"fmt"
	dbg "github.com/dedis/cothority/lib/debug_lvl"
	"net"
)

// Implements a simple proxy
// A <-> D <-> B
// D is the proxy. It will listen for incoming connections on the side of B
// And will connect to A

// server is the address of A,i.e. the monitor process receiving the stats
var server string

// serverConn is the connection object to the server
var serverConn net.Conn

// listen is the address to listen to for producer (processes that send
// measurements data)  to connect to. That way
// processes will connect to the proxy using this address and will send their
// data that will be relayed by the proxy to the server address.
var listenProxy string

// connectServer will try to connect to the server ...
func connectServer(addr string) error {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		fmt.Errorf("Proxy connection to server failed : %v", err)
	}
	server = addr
	serverConn = conn
}

// Proxy will launch a routines that waits for input connections
// It takes the server address to relay messages to, and the listen address
// where clients will contact the proxy
// It will panic if it can not contact the server or can not bind to the address
func Proxy(server, listen string) {
	if err := connectServer(server); err != nil {
		panic(err)
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		panic(fmt.Errorf("Error while binding proxy to addr %s : %v", addr, err))
	}
	listenProxy = addr
	for {
		conn, err := ln.Accept()
		if err != nil {
			dbg.Lvl1("Error proxy accepting connection : ", err)
		}
		dbg.Lvl2("Proxy accepting incoming connection from : ", conn.RemoteAddr().String())
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
			}
		}
		// Proxy data
		n, err = serverConn.Write(buf[:n])
		if err != nil {
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
