package scheduler

/*

The interface should be :

INPUT : list of client's public keys

OUTPUT : list of slots

*/

import (
	"github.com/dedis/crypto/abstract"
)

type ScheduleDescription struct {
}

type Scheduler interface {
	AddClientToSchedule(pk abstract.Point) error

	FinalizeClientSet() error

	RelayPerformShuffle()

	TrusteePerformShuffle()

	TrusteeValidateSchedule()

	RelayValidateSchedule()

	ClientPayloadEmbeddable(roundId int32) (int64, int64)
}

type SchedulerFactory func() Scheduler
