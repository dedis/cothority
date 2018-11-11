package lib

import (
	"github.com/dedis/onet"
	"time"
)

type ChainConfig struct {
	Roster       *onet.Roster
	ShardCount   int
	EpochSize    time.Duration
	Timestamp    time.Time
	ShardRosters []onet.Roster
}
