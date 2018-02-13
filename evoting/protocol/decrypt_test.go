package protocol

import (
	"errors"
	"testing"
	"time"

	"github.com/dedis/onet"

	"github.com/stretchr/testify/assert"

	"github.com/dedis/cothority/evoting/lib"
)

var decryptServiceID onet.ServiceID

type decryptService struct {
	*onet.ServiceProcessor
	secret   *lib.SharedSecret
	election *lib.Election
}

func init() {
	new := func(ctx *onet.Context) (onet.Service, error) {
		return &decryptService{ServiceProcessor: onet.NewServiceProcessor(ctx)}, nil
	}
	decryptServiceID, _ = onet.RegisterNewService(NameDecrypt, new)
}

func (s *decryptService) NewProtocol(node *onet.TreeNodeInstance, conf *onet.GenericConfig) (
	onet.ProtocolInstance, error) {

	switch node.ProtocolName() {
	case NameDecrypt:
		instance, _ := NewDecrypt(node)
		decrypt := instance.(*Decrypt)
		decrypt.Secret = s.secret
		decrypt.Election = s.election
		return decrypt, nil
	default:
		return nil, errors.New("Unknown protocol")
	}
}

func TestDecryptProtocol(t *testing.T) {
	for _, nodes := range []int{3} {
		runDecrypt(t, nodes)
	}
}

func runDecrypt(t *testing.T, n int) {
	local := onet.NewLocalTest(lib.Suite)
	defer local.CloseAll()

	nodes, roster, tree := local.GenBigTree(n, n, 1, true)

	election := &lib.Election{Roster: roster, Stage: lib.Shuffled}
	dkgs := election.GenChain(n)

	services := local.GetServices(nodes, decryptServiceID)
	for i := range services {
		services[i].(*decryptService).secret, _ = lib.NewSharedSecret(dkgs[i])
		services[i].(*decryptService).election = election
	}

	instance, _ := services[0].(*decryptService).CreateProtocol(NameDecrypt, tree)
	decrypt := instance.(*Decrypt)
	decrypt.Secret, _ = lib.NewSharedSecret(dkgs[0])
	decrypt.Election = election
	decrypt.Start()

	select {
	case <-decrypt.Finished:
		// partials, _ := election.Partials()
		// for _, partial := range partials {
		// 	fmt.Println(partial)
		// 	assert.False(t, partial.Flag)
		// }
	case <-time.After(5 * time.Second):
		assert.True(t, false)
	}
}
