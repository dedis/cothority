package ca

import (
	//"fmt"
	"gopkg.in/dedis/onet.v1/log"
	//"github.com/dedis/crypto/abstract"
	//"time"
	//"gopkg.in/dedis/onet.v1/network"
	"github.com/dedis/cothority/dns_id/common_structs"
	"github.com/dedis/cothority/dns_id/skipchain"
	"gopkg.in/dedis/onet.v1"
	//"github.com/dedis/crypto/config"
	//"github.com/stretchr/testify/assert"
	//"io/ioutil"
	//"os"
	"testing"
)

func NewTestCSRDispatcher(local *onet.LocalTest) *CSRDispatcher {
	log.Print("NewTestCSRDispatcher")
	d := NewCSRDispatcher()
	d.CAClient = local.NewClient(ServiceCAName)
	return d
}

func TestSignCert(t *testing.T) {
	l := onet.NewTCPTest()
	hosts, _, _ := l.GenTree(2, true)
	services := l.GetServices(hosts, CAService)

	defer l.CloseAll()

	//log.Print(len(hosts))
	cas := make([]common_structs.CAInfo, 0)
	for index, h := range hosts {
		//cas = append(cas, common_structs.CAInfo{Public: h.ServerIdentity.Public, ServerID: h.ServerIdentity})
		cas = append(cas, common_structs.CAInfo{Public: services[index].(*CA).Public, ServerID: h.ServerIdentity})
	}
	config := &common_structs.Config{
		CAs: cas,
	}

	var id skipchain.SkipBlockID
	d := NewTestCSRDispatcher(l)

	d.SignCert(config, id)
}
