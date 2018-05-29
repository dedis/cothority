package eventlog

import (
	"fmt"
	"testing"
	"time"

	"github.com/dedis/student_18_omniledger/omniledger/darc"
	omniledger "github.com/dedis/student_18_omniledger/omniledger/service"
	"github.com/stretchr/testify/require"
)

func TestClient_Log(t *testing.T) {
	s := newSer(t)
	leader := s.services[0]
	defer s.close()

	owner := darc.NewSignerEd25519(nil, nil)
	c := NewClient(s.roster)
	err := c.Init(owner)
	require.Nil(t, err)

	ids, err := c.Log(NewEvent("auth", "user alice logged out"),
		NewEvent("auth", "user bob logged out"))
	require.Nil(t, err)
	require.True(t, len(ids) == 2)

	// Loop while we wait for the next block to be created.
	found := false
	for ct := 0; ct < 10; ct++ {
		req := &omniledger.GetProof{
			Version: omniledger.CurrentVersion,
			Key:     ids[1],
			ID:      c.ID,
		}
		resp, err := leader.omni.GetProof(req)
		if err == nil {
			p := resp.Proof.InclusionProof
			if p.Match() {
				found = true
				break
			}
		} else {
			t.Log("err", err)
		}
		time.Sleep(1 * time.Second)
	}
	if !found {
		t.Fatal("timeout")
	}

	// Fetch index, see if it has two things in it.
	req := &omniledger.GetProof{
		Version: omniledger.CurrentVersion,
		Key:     indexKey.Slice(),
		ID:      c.ID,
	}
	resp, err := leader.omni.GetProof(req)
	if err != nil {
		t.Log("err", err)
	}

	p := resp.Proof.InclusionProof
	if !p.Match() {
		t.Fatal("proof of exclusion of index")
	}
	v, _ := p.Values()
	if len(v) != 2 {
		t.Fatal("values length")
	}
	idx := v[0].([]byte)
	expected := 2 * 64
	if len(idx) != expected {
		t.Fatalf("index key content is %v, expected %v", len(idx), expected)
	}
}

func TestClient_Log1000(t *testing.T) {
	s := newSer(t)
	leader := s.services[0]
	defer s.close()

	owner := darc.NewSignerEd25519(nil, nil)
	c := NewClient(s.roster)
	err := c.Init(owner)
	require.Nil(t, err)

	for ct := 0; ct < 1000; ct++ {
		_, err := c.Log(NewEvent("auth", fmt.Sprintf("user %v logged in", ct)))
		require.Nil(t, err)
	}
	leader.waitForBlock()
	leader.waitForBlock()

	// TODO: Get this check working again, though maybe it would
	// be better to check once the search API is ready.

	// s.check(t, "index len correct", func() bool {
	// 	req := &omniledger.GetProof{
	// 		Version: omniledger.CurrentVersion,
	// 		Key:     indexKey.Slice(),
	// 		ID:      c.ID,
	// 	}
	// 	resp, err := leader.omni.GetProof(req)
	// 	if err != nil {
	// 		t.Log("err", err)
	// 	}

	// 	p := resp.Proof.InclusionProof
	// 	if !p.Match() {
	// 		return false
	// 	}
	// 	v, _ := p.Values()
	// 	if len(v) != 2 {
	// 		t.Fatal("values length")
	// 	}
	// 	idx := v[0].([]byte)
	// 	expected := 1000 * 64
	// 	return len(idx) == expected
	// })
}
