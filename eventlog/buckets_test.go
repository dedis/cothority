package eventlog

import (
	"testing"
	"time"

	omniledger "github.com/dedis/student_18_omniledger/omniledger/service"
	"github.com/stretchr/testify/require"
)

func TestBucket_UpdateBucket(t *testing.T) {
	tn := time.Now().Unix()
	b := bucket{
		Start: tn,
	}
	objID := omniledger.ObjectID{
		DarcID:     omniledger.ZeroDarc,
		InstanceID: omniledger.GenNonce(),
	}
	event := Event{
		Topic:     "Art",
		Content:   "Rembrandt",
		Timestamp: tn - 1000,
	}
	// time is in the past, so fail
	_, err := b.updateBucket([]byte("dummy_id"), objID.Slice(), event)
	require.NotNil(t, err)

	// time in the future, should pass
	event.Timestamp = tn + 1000
	_, err = b.updateBucket([]byte("dummy_id"), objID.Slice(), event)
	require.Nil(t, err)

	require.Equal(t, event.Timestamp, b.End)
	require.Equal(t, tn, b.Start)
	require.Equal(t, b.EventRefs[0], objID.Slice())
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
