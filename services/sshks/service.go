package sshks

/*
SSH-ks is a keystorage service for SSH keys. You can have a set of clients
that communicate with a cothority to keep the list of public keys updated.
A number of servers track all changes and update their .authorized_hosts
accordingly.
*/

import "github.com/dedis/cothority/lib/sda"

// ServiceName can be used to refer to the name of this service
const ServiceName = "SSHks"

func init() {
	sda.RegisterNewService(ServiceName, newSSHKSService)
	sshksSID = sda.ServiceFactory.ServiceID(ServiceName)
}

var sshksSID sda.ServiceID

// Service handles adding new SkipBlocks
type Service struct {
	*sda.ServiceProcessor
	path string
}

func newSSHKSService(c sda.Context, path string) sda.Service {
	s := &Service{
		ServiceProcessor: sda.NewServiceProcessor(c),
		path:             path,
	}
	/*
		if err := s.RegisterMessage(s.PropagateSkipBlock); err != nil {
			dbg.Fatal("Registration error:", err)
		}
	*/
	return s
}
