package omniledger

import (
	"fmt"
	"github.com/dedis/cothority"
	"github.com/dedis/cothority/omniledger/lib"
	"github.com/dedis/onet"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestLib_ChangeRoster(t *testing.T) {
	l := onet.NewLocalTest(cothority.Suite)
	defer l.CloseAll()

	i := 3
	j := 3

	_, roster1, _ := l.GenTree(i, true)
	_, roster2, _ := l.GenTree(j, true)

	// Suppose r1 contains nodes A,B,C and r2 contains nodes D,E,F
	r1 := *roster1
	r2 := *roster2

	r1 = lib.ChangeRoster(r1, r2)
	assert.True(t, len(r1.List) == i+1)
	assert.True(t, r1.List[3] == r2.List[0]) // r1 should contain A,B,C,D

	r1 = lib.ChangeRoster(r1, r2)
	assert.True(t, len(r1.List) == i+2)
	assert.True(t, r1.List[4] == r2.List[1]) // r1 should contain A,B,C,D,E

	r1 = lib.ChangeRoster(r1, r2)
	assert.True(t, len(r1.List) == i+3)
	assert.True(t, r1.List[5] == r2.List[2]) // r1 should contain A,B,C,D,E,F
	fmt.Println("LIST:", r1.List)

	r1 = lib.ChangeRoster(r1, r2)
	assert.True(t, len(r1.List) == i+j-1) // r1 should contain B,C,D,E,F
	fmt.Println("LIST:", r1.List)

	r1 = lib.ChangeRoster(r1, r2)
	assert.True(t, len(r1.List) == i+j-2) // r1 should contain C,D,E,F

	r1 = lib.ChangeRoster(r1, r2)
	assert.True(t, len(r1.List) == i+j-3) // r1 should contain D,E,F

	// r1 should contains the same nodes as r2
	for k := 0; k < len(r1.List); k++ {
		assert.True(t, r1.List[k].Equal(r2.List[k]))
	}
}

func TestLib_EncodeDecodeDuration(t *testing.T) {
	d := time.Duration(1) * time.Second
	dBuf := lib.EncodeDuration(d)
	dd, err := lib.DecodeDuration(dBuf)
	if err != nil {
		fmt.Println(err)
	}

	fmt.Println(d, dd)

	assert.True(t, d.Nanoseconds() == dd.Nanoseconds())
}

func TestLib_ShardingMultiplier(t *testing.T) {
	l := onet.NewLocalTest(cothority.Suite)
	defer l.CloseAll()

	nodeCount := 8
	_, roster, _ := l.GenTree(nodeCount, true)

	shardCount := 2
	seed := int64(1)

	shardRosters := lib.Sharding(roster, shardCount, seed)

	assert.True(t, len(shardRosters) == shardCount)

	for _, sr := range shardRosters {
		assert.True(t, len(sr.List) == nodeCount/shardCount)
	}
}

func TestLib_ShardingNotMultiplier(t *testing.T) {
	l := onet.NewLocalTest(cothority.Suite)
	defer l.CloseAll()

	nodeCount := 19
	_, roster, _ := l.GenTree(nodeCount, true)

	shardCount := 4
	seed := int64(1)

	shardRosters := lib.Sharding(roster, shardCount, seed)

	assert.True(t, len(shardRosters) == shardCount)

	for ind, sr := range shardRosters {
		if ind < nodeCount%shardCount {
			assert.True(t, len(sr.List) == nodeCount/shardCount+1)
		} else {
			assert.True(t, len(sr.List) == nodeCount/shardCount)
		}
	}
}
