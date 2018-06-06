package eventlog

import (
	"testing"
	"time"

	omniledger "github.com/dedis/cothority/omniledger/service"
	"github.com/stretchr/testify/require"
)

func TestBucket_UpdateBucket(t *testing.T) {
	b := bucket{}
	objID := omniledger.ObjectID{
		DarcID:     omniledger.ZeroDarc,
		InstanceID: omniledger.GenNonce(),
	}
	event := NewEvent("Art", "Rembrandt")

	// new time, we should see an update in b.Start
	_, err := b.updateBucket([]byte("dummy_id"), objID.Slice(), event)
	require.Nil(t, err)
	require.Equal(t, event.When, b.Start)

	// time is in the past, we should see another update
	event.When = event.When - 1000
	_, err = b.updateBucket([]byte("dummy_id"), objID.Slice(), event)
	require.Nil(t, err)
	require.Equal(t, event.When, b.Start)

	// check the references
	require.Equal(t, b.EventRefs[0], objID.Slice())
	require.Equal(t, b.EventRefs[1], objID.Slice())
}

func TestBucket_NewLink(t *testing.T) {
	tn := time.Now().Unix()
	b := bucket{
		Start: tn,
	}
	oldID := omniledger.ObjectID{
		DarcID:     omniledger.ZeroDarc,
		InstanceID: omniledger.GenNonce(),
	}
	newID := omniledger.ObjectID{
		DarcID:     omniledger.ZeroDarc,
		InstanceID: omniledger.GenNonce(),
	}
	scs, newBucket, err := b.newLink(oldID.Slice(), newID.Slice(), []byte("dummy event"))
	require.Nil(t, err)
	require.Equal(t, newBucket.Prev, oldID.Slice())
	require.Equal(t, 1, len(scs))
}
