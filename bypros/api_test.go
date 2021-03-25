package bypros

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.dedis.ch/cothority/v3/skipchain"
	"go.dedis.ch/onet/v3/network"
	"go.dedis.ch/protobuf"
	"golang.org/x/xerrors"
)

const status = "fake status"

func TestClientFollow(t *testing.T) {
	overlay := &fakeOverlay{}
	overlayClient = overlay

	scID := skipchain.NewSkipBlock().Hash
	host := &network.ServerIdentity{Description: "fake1"}
	target := &network.ServerIdentity{Description: "fake2"}

	client := NewClient()
	err := client.Follow(host, target, scID)
	require.NoError(t, err)

	require.Len(t, overlay.dest, 1)
	require.Equal(t, host, overlay.dest[0])

	expected := &Follow{
		Target: target,
		ScID:   scID,
	}

	require.Len(t, overlay.sent, 1)
	require.Equal(t, expected, overlay.sent[0])
}

func TestClientUnFollow(t *testing.T) {
	overlay := &fakeOverlay{}
	overlayClient = overlay

	host := &network.ServerIdentity{Description: "fake1"}

	client := NewClient()
	err := client.Unfollow(host)
	require.NoError(t, err)

	require.Len(t, overlay.dest, 1)
	require.Equal(t, host, overlay.dest[0])

	expected := &Unfollow{}

	require.Len(t, overlay.sent, 1)
	require.Equal(t, expected, overlay.sent[0])
}

func TestClientQuery(t *testing.T) {
	host := &network.ServerIdentity{Description: "fake1"}
	query := "fake query"

	overlay := &fakeOverlay{}

	client := Client{OverlayClient: overlay}
	res, err := client.Query(host, query)

	require.NoError(t, err)
	require.Equal(t, []byte("fake reply"), res)

	require.Len(t, overlay.dest, 1)
	require.Equal(t, host, overlay.dest[0])

	expected := &Query{
		Query: query,
	}

	require.Len(t, overlay.sent, 1)
	require.Equal(t, expected, overlay.sent[0])
}

func TestClientCatchUP(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	scID := skipchain.NewSkipBlock().Hash
	host := &network.ServerIdentity{Address: "tls://127.0.0.1:0"}
	target := &network.ServerIdentity{Address: "tls://127.0.0.1:0"}

	overlay := &fakeOverlay{}

	client := Client{OverlayClient: overlay}

	responses, err := client.CatchUP(ctx, host, target, scID, scID, 3)
	require.NoError(t, err)

	expected := []CatchUpResponse{
		{Status: CatchUpStatus{Message: status, BlockIndex: 0, BlockHash: []byte{}}},
		{Status: CatchUpStatus{Message: status, BlockIndex: 1, BlockHash: []byte{}}},
		{Status: CatchUpStatus{Message: status, BlockIndex: 2, BlockHash: []byte{}}},
		{Done: true, Status: CatchUpStatus{BlockHash: []byte{}}},
	}

	for i := 0; i < len(expected); i++ {
		select {
		case resp := <-responses:
			require.Equal(t, expected[i], resp)
		case <-time.After(time.Second):
			t.Error("didn't received after timeout")
		}
	}

	select {
	case resp := <-responses:
		t.Error("unexpected message", resp)
	default:
	}
}

// -----------------------------------------------------------------------------
// Utility functions

type fakeOverlay struct {
	dest []*network.ServerIdentity
	sent []interface{}
}

func (o *fakeOverlay) SendProtobuf(dst *network.ServerIdentity, msg interface{}, ret interface{}) error {
	o.dest = append(o.dest, dst)
	o.sent = append(o.sent, msg)

	reply, ok := ret.(*QueryReply)
	if ok {
		reply.Result = []byte("fake reply")
	}

	return nil
}

func (o *fakeOverlay) OpenWS(url string) (WsHandler, error) {
	return &fakeWs{n: 3}, nil
}

type fakeWs struct {
	n   int
	cur int
}

func (f *fakeWs) Close() error {
	return nil
}
func (f *fakeWs) Write(messageType int, data []byte) error {
	return nil
}

func (f *fakeWs) Read() (messageType int, p []byte, err error) {
	resp := CatchUpResponse{}

	if f.cur == f.n {
		resp.Done = true
	} else {
		resp.Status = CatchUpStatus{Message: status, BlockIndex: f.cur}
	}

	f.cur++

	buf, err := protobuf.Encode(&resp)
	if err != nil {
		return 0, nil, xerrors.Errorf("failed to encode: %v", err)
	}

	return 0, buf, nil
}
