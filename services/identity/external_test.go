/*
This is a test-function for the external-methods. Every call in here
should go through the interface created in `external.go`.
*/
package identity

import (
	"testing"
    "encoding/json"
    "fmt"
    _ "regexp"
    _ "github.com/dedis/crypto/nist"

    "github.com/dedis/cothority/sda"
    "github.com/stretchr/testify/assert"
    "github.com/dedis/crypto/config"
    _ "github.com/dedis/crypto/ed25519"
    "github.com/dedis/cothority/network"
    "github.com/dedis/cothority/log"
    "github.com/dedis/cothority/crypto"
)

//func TestExternal_Point(t *testing.T) {
    //kp := config.NewKeyPair(network.Suite)
    //c := NewConfig(3, kp.Public, "motorola")
    //c.Data["motorola"] = "epfl"
    //fmt.Println(c.Device["motorola"])

    //js, err := json.Marshal(&c)
    //sjson := string(js)

    //fmt.Println(sjson)
    //r, _ := regexp.Compile("\\[.*?\\]")
    //arr := r.FindAllString(sjson, -1)
    //fmt.Println(arr)

    //x := make([]int, 100)
    //y := make([]int, 100)
    //z := make([]int, 100)
    //tt := make([]int, 100)

    //json.Unmarshal([]byte(arr[0]), &x)
    //json.Unmarshal([]byte(arr[1]), &y)
    //json.Unmarshal([]byte(arr[2]), &z)
    //json.Unmarshal([]byte(arr[3]), &tt)

    //fmt.Println("X:", x, "Len:", len(x))
    //fmt.Println("Y:", y, "Len:", len(y))
    //fmt.Println("Z:", z, "Len:", len(z))
    //fmt.Println("T:", tt, "Len:", len(tt))

    //p := ed25519.point{}

    //X := fieldElement{x[0], x[1], x[2], x[3], x[4], x[5], x[6],
                              //x[7], x[8], x[9]}

    //foo := Config{}
    //err = json.Unmarshal(js, &foo)
    //fmt.Println(foo, err)
//}

func TestExternal_CreateIdentity(t *testing.T) {
	//t.Skip()
	local := sda.NewLocalTest()
	defer local.CloseAll()
	_, el, s := local.MakeHELS(5, identityService)
	service := s.(*Service)

    keypair := config.NewKeyPair(network.Suite)
    c := NewConfig(3, keypair.Public, "motorola")
    fmt.Println(c)
    ai := AddIdentity{Config: c, Roster: nil}
    req, _ := json.Marshal(&ai)
    fmt.Println(ai)

    air, _ := ExCreateIdentity(string(req), el)
    assert.NotNil(t, service.identities[string(air.Data.Hash)])
}

func TestExternal_ConfigUpdate(t *testing.T) {
    //t.Skip()
    local := sda.NewLocalTest()
	defer local.CloseAll()
	_, el, _ := local.MakeHELS(5, identityService)

    c1 := NewIdentity(el, 50, "one")
	c1.CreateIdentity()

    cu := ConfigUpdate{ID: c1.ID, AccountList: nil}
    req, _ := json.Marshal(&cu)

    cur, _  := ExConfigUpdate(string(req))
    assert.Equal(t, c1.Config.String(), cur.AccountList.String())
}

func TestExternal_ProposeSend(t *testing.T) {
    //t.Skip()
    local := sda.NewLocalTest()
	defer local.CloseAll()
	_ , el, s := local.MakeHELS(5, identityService)
	service := s.(*Service)

	c1 := NewIdentity(el, 50, "one")
    c1.CreateIdentity()

	conf2 := c1.Config.Copy()
	kp2 := config.NewKeyPair(network.Suite)
	conf2.Device["two"] = &Device{kp2.Public}

    ps := ProposeSend{c1.ID, conf2}
    req, _ := json.Marshal(&ps)

    err := ExProposeSend(string(req))
    _ = err

    prop := service.identities[string(c1.ID)].Proposed
    assert.NotNil(t, prop)
    assert.Equal(t, len(prop.Device), 2)
}

func TestExternal_ProposeFetch(t *testing.T) {
    //t.Skip()
    local := sda.NewLocalTest()
	defer local.CloseAll()
	_ , el, _ := local.MakeHELS(5, identityService)

    c1 := NewIdentity(el, 50, "one")
    c1.CreateIdentity()

    pf := ProposeFetch{ID: c1.ID, AccountList: nil}
    req1, _ := json.Marshal(&pf)

    pfr, _ := ExProposeFetch(string(req1))
    assert.Nil(t, pfr.AccountList)

    conf2 := c1.Config.Copy()
	kp2 := config.NewKeyPair(network.Suite)
	conf2.Device["two"] = &Device{kp2.Public}
    log.ErrFatal(c1.ProposeSend(conf2))

    pfr, _ = ExProposeFetch(string(req1))
    prop := service.identities[string(c1.ID)].Proposed
    assert.NotNil(t, pfr.AccountList)
    assert.Equal(t, len(prop.Device), 2)
}

func TestExternal_ProposeVote(t *testing.T) {
    t.Skip()
	l := sda.NewLocalTest()
	_, el, _ := l.GenTree(3, true, true, true)
	defer l.CloseAll()

	c1 := NewIdentity(el, 50, "one1")
	log.ErrFatal(c1.CreateIdentity())

	conf2 := c1.Config.Copy()
	kp2 := config.NewKeyPair(network.Suite)
	conf2.Device["two2"] = &Device{kp2.Public}
	conf2.Data["two2"] = "public2"
	log.ErrFatal(c1.ProposeSend(conf2))
	log.ErrFatal(c1.ProposeFetch())

    hash, _ := c1.Config.Hash()
    sig, _ := crypto.SignSchnorr(network.Suite, c1.Private, hash)

    pv := ProposeVote{ID: c1.ID, Signer: "one1", Signature: &sig}
    req, _ := json.Marshal(&pv)

    ExProposeVote(string(req))
    assert.Equal(t, len(c1.Config.Device), 2)
}
