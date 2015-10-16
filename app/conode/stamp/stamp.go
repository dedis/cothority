package main
import (
	"flag"
	dbg "github.com/dedis/cothority/lib/debug_lvl"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/cothority/lib/app"
	"github.com/dedis/cothority/app/conode/defs"
	"os"
	"strings"
	"github.com/dedis/cothority/lib/hashid"
	"github.com/dedis/cothority/lib/coconet"
	"encoding/base64"
)

var file string
var server string
var debug int
var suiteString string = "ed25519"
var suite abstract.Suite

func init() {
	flag.StringVar(&file, "file", "", "The file to be stamped")
	flag.StringVar(&server, "server", "localhost", "The server to connect to")
	flag.IntVar(&debug, "debug", 1, "Debug-level: 1 - few, 5 - lots")
	flag.StringVar(&suiteString, "suite", suiteString, "Which suite to use [ed25519]")
}


func main() {
	flag.Parse()
	if file == "" {
		dbg.Fatal("Please give a filename")
	}
	if server == "" {
		server = "localhost"
	}
	if ! strings.Contains(server, ":") {
		server += ":2001"
	}

	suite = app.GetSuite(suiteString)

	// Then get a connection
	dbg.Lvl1("Connecting to", server)
	conn := coconet.NewTCPConn(server)
	err := conn.Connect()
	if err != nil {
		dbg.Fatal("Error when getting the connection to the host:", err)
	}

	// Creating the hash of the file and send it over the net

	var myHash hashid.HashId
	msg := &defs.TimeStampMessage{
		Type:  defs.StampRequestType,
		ReqNo: 0,
		Sreq:  &defs.StampRequest{Val: myHash}}

	err = conn.Put(msg)
	if err != nil {
		dbg.Fatal("Couldn't send hash-message to server: ", err)
	}

	// Wait for the signed message
	tsm := &defs.TimeStampMessage{}
	err = conn.Get(tsm)
	if err != nil {
		dbg.Fatal("Error while receiving signature")
	}

	// Asking to close the connection
	err = conn.Put(&defs.TimeStampMessage{
		ReqNo:1,
		Type: defs.StampClose,
	})
	conn.Close()

	err = verifySignature(myHash, tsm.Srep)
	if err != nil{
		dbg.Fatal("Verification of signature failde:", err)
	}

	// Print to the screen, and write to file
	dbg.Printf("%+v", tsm)
	f, err := os.Create("signature.sign")
	if err != nil {
		dbg.Fatal("Couldn't create signature.sign")
	}
	file := base64.NewEncoder(base64.StdEncoding, f)

	err = suite.Write(file, msg.Sreq)
	if err != nil {
		dbg.Fatal("Couldn't write to file")
	}
	err = suite.Write(file, tsm.Srep)
	if err != nil {
		dbg.Fatal("Couldn't write to file")
	}
	file.Close()
	f.Close()
	dbg.Print("All done - file is written")
}

func verifySignature(message hashid.HashId, reply defs.StampReply) error{
	dbg.Lvl1("Not checking signature")
	return nil
}