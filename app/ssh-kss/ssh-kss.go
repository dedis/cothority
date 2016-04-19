// This is the ssh-keystore-server part that listens for requests of keystore-clients
// and will sign these requests.
package main

import ()
import (
	"bufio"
	"fmt"
	"github.com/codegangsta/cli"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/services/sshks"
	"github.com/dedis/crypto/config"
	"io"
	"os"
	"strings"
)

var serverKS *sshks.ServerKS

func main() {
	app := cli.NewApp()
	app.Name = "SSH keystore server"
	app.Usage = "Serves as a server to listen to requests"
	app.Flags = []cli.Flag{
		cli.IntFlag{
			Name:  "debug, d",
			Value: 1,
			Usage: "debug-level: 1 for terse, 5 for maximal",
		},
		cli.StringFlag{
			Name:  "config, c",
			Value: "/etc/ssh-ks",
			Usage: "The configuration-file of ssh-keystore",
		},
	}
	app.Action = func(c *cli.Context) {
		dbg.SetDebugVisible(c.Int("debug"))
		file := c.String("config")
		var err error
		serverKS, err = sshks.ReadServerKS(file)
		if err != nil {
			serverKS, err = askServerConfig(os.Stdin, os.Stdout)
			if err != nil {
				dbg.Fatal("While creating new config:", err)
			}
			err = serverKS.WriteConfig(file)
			if err != nil {
				dbg.Fatal("Couldn't write config:", err)
			}
		}
		serverKS.Config.List()
		err = serverKS.Start()
		if err != nil {
			dbg.Fatal("Couldn't start server:", err)
		}
		serverKS.WaitForClose()
	}
	app.Run(os.Args)
}

func askServerConfig(in io.Reader, out io.Writer) (*sshks.ServerKS, error) {
	inb := bufio.NewReader(in)
	ip := getArg(inb, out, "Please enter an IP:port where this server has to be reached",
		"localhost:2000")
	sshd := getArg(inb, out, "Where is the system-ssh-directory located",
		"/etc/sshd")
	ssh := getArg(inb, out, "Where should the authorized_keys be stored",
		"/root/.ssh")
	return createServerConfig(ip, sshd, ssh)
}

func createServerConfig(ip, dirSSHD, dirSSH string) (*sshks.ServerKS, error) {
	if ip == "" {
		ip = "localhost:2000"
	}
	pair := config.NewKeyPair(network.Suite)
	return sshks.NewServerKS(pair, ip, dirSSHD, dirSSH)
}

func getArg(in *bufio.Reader, out io.Writer, question, def string) string {
	fmt.Fprintf(out, "%s [%s]: ", question, def)
	b, _ := in.ReadString('\n')
	str := strings.TrimSpace(string(b))
	if str == "" {
		return def
	} else {
		return str
	}
}
