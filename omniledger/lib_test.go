package omniledger

import (
	"fmt"
	"github.com/dedis/cothority"
	"github.com/dedis/cothority/omniledger/lib"
	"github.com/dedis/onet"
	"github.com/stretchr/testify/assert"
	"log"
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
	// r1 should be A,B,C,D
	assert.True(t, r1.List[0].Equal(roster1.List[0]))
	assert.True(t, r1.List[1].Equal(roster1.List[1]))
	assert.True(t, r1.List[2].Equal(roster1.List[2]))
	assert.True(t, r1.List[3].Equal(r2.List[0]))

	r1 = lib.ChangeRoster(r1, r2)
	assert.True(t, len(r1.List) == i+2)
	// r1 should be A,B,C,D,E
	assert.True(t, r1.List[0].Equal(roster1.List[0]))
	assert.True(t, r1.List[1].Equal(roster1.List[1]))
	assert.True(t, r1.List[2].Equal(roster1.List[2]))
	assert.True(t, r1.List[3].Equal(r2.List[0]))
	assert.True(t, r1.List[4].Equal(r2.List[1]))

	r1 = lib.ChangeRoster(r1, r2)
	assert.True(t, len(r1.List) == i+3)
	// r1 should be D,E,F,A,B,C
	assert.True(t, r1.List[0].Equal(r2.List[0]))
	assert.True(t, r1.List[1].Equal(r2.List[1]))
	assert.True(t, r1.List[2].Equal(r2.List[2]))
	assert.True(t, r1.List[3].Equal(roster1.List[0]))
	assert.True(t, r1.List[4].Equal(roster1.List[1]))
	assert.True(t, r1.List[5].Equal(roster1.List[2]))
	fmt.Println(roster1, r1)
	log.Print(r1)

	r1 = lib.ChangeRoster(r1, r2)
	assert.True(t, len(r1.List) == i+j-1)
	// r1 should be D,E,F,B,C
	for i := 0; i < len(r2.List); i++ {
		assert.True(t, r2.List[i].Equal(r1.List[i]))
	}
	assert.True(t, r1.List[3].Equal(roster1.List[1]))
	assert.True(t, r1.List[4].Equal(roster1.List[2]))

	r1 = lib.ChangeRoster(r1, r2)
	// r1 should be D,E,F,C
	assert.True(t, len(r1.List) == i+j-2)
	for i := 0; i < len(r2.List); i++ {
		assert.True(t, r2.List[i].Equal(r1.List[i]))
	}
	assert.True(t, r1.List[3].Equal(roster1.List[2]))

	r1 = lib.ChangeRoster(r1, r2)
	assert.True(t, len(r1.List) == i+j-3)
	// r1 should be D,E,F <=> r1 should contains the same nodes as r2 and have the same order
	for k := 0; k < len(r1.List); k++ {
		assert.True(t, r1.List[k].Equal(r2.List[k]))
	}
}

func TestLib_ChangeRoster2(t *testing.T) {
	l := onet.NewLocalTest(cothority.Suite)
	defer l.CloseAll()

	i := 3

	_, roster1, _ := l.GenTree(i, true)

	// Suppose r1 contains nodes A,B,C and r2 contains nodes C,B,A
	r1 := *roster1
	r2 := *roster1

	r2.List[0] = r1.List[2]
	r2.List[2] = r1.List[0]

	r1 = lib.ChangeRoster(r1, r2)

	// r1 should be C,B,A
	for k := 0; k < len(r1.List); k++ {
		assert.True(t, r1.List[k].Equal(r2.List[k]))
	}
}

func TestLib_ChangeRoster3(t *testing.T) {
	l := onet.NewLocalTest(cothority.Suite)
	defer l.CloseAll()

	i := 3
	j := 3

	_, roster1, _ := l.GenTree(i, true)
	_, roster2, _ := l.GenTree(j, true)

	// Suppose r1 contains nodes A,B,C and r2 contains nodes C,D,E
	r1 := *roster1
	r2 := *roster2

	r2.List[0] = r1.List[2]

	r1 = lib.ChangeRoster(r1, r2)
	assert.True(t, len(r1.List) == i+1)
	// r1 should be A,B,C,D
	assert.True(t, r1.List[0].Equal(roster1.List[0]))
	assert.True(t, r1.List[1].Equal(roster1.List[1]))
	assert.True(t, r1.List[2].Equal(roster1.List[2]))
	assert.True(t, r1.List[3].Equal(roster2.List[1]))

	r1 = lib.ChangeRoster(r1, r2)
	assert.True(t, len(r1.List) == i+2)
	// r1 should be C,D,E,A,B
	assert.True(t, r1.List[0].Equal(roster2.List[0]))
	assert.True(t, r1.List[1].Equal(roster2.List[1]))
	assert.True(t, r1.List[2].Equal(roster2.List[2]))
	assert.True(t, r1.List[3].Equal(roster1.List[0]))
	assert.True(t, r1.List[4].Equal(roster1.List[1]))
	fmt.Println(roster1, r1)
	log.Print(r1)

	r1 = lib.ChangeRoster(r1, r2)
	assert.True(t, len(r1.List) == i+j-2)
	// r1 should be C,D,E,B
	assert.True(t, r1.List[0].Equal(roster2.List[0]))
	assert.True(t, r1.List[1].Equal(roster2.List[1]))
	assert.True(t, r1.List[2].Equal(roster2.List[2]))
	assert.True(t, r1.List[3].Equal(roster1.List[1]))
	fmt.Println(roster1, r1)
	log.Print(r1)

	r1 = lib.ChangeRoster(r1, r2)
	assert.True(t, len(r1.List) == i+j-3)
	// r1 should be D,E,F <=> r1 should contains the same nodes as r2 and have the same order
	for k := 0; k < len(r1.List); k++ {
		assert.True(t, r1.List[k].Equal(r2.List[k]))
	}
}

func TestLib_EncodeDecodeDuration(t *testing.T) {
	sec := time.Duration(1) * time.Second
	secBuf := lib.EncodeDuration(sec)
	s, err := lib.DecodeDuration(secBuf)

	assert.Nil(t, err)
	assert.True(t, sec.Nanoseconds() == s.Nanoseconds())
	assert.True(t, sec.Seconds() == s.Seconds())
	assert.True(t, sec.Minutes() == s.Minutes())

	min := time.Duration(1) * time.Minute
	minBuf := lib.EncodeDuration(min)
	m, err := lib.DecodeDuration(minBuf)

	assert.Nil(t, err)
	assert.True(t, m.Nanoseconds() == m.Nanoseconds())
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
