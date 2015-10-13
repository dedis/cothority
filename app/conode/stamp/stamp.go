package main
import (
	"flag"
	dbg "github.com/dedis/cothority/lib/debug_lvl"
)

var file string
var server string
var debug int

func init() {
	flag.StringVar(&file, "file", "", "The file to be stamped")
	flag.StringVar(&server, "server", "localhost", "The server to connect to")
	flag.IntVar(&debug, "debug", 1, "Debug-level: 1 - few, 5 - lots")
}


func main() {
	flag.Parse()
	if file == "" {
		dbg.Fatal("Please give a filename")
	}
	if server == ""{
		server = "localhost"
	}
}