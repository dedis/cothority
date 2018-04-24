package service

import (
	"syscall"

	"github.com/dedis/onet/log"
)

func raiseFdLimit() {
	// Raising the FD limit like this might belong in a more central place.
	var rLimit syscall.Rlimit
	err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rLimit)
	if err != nil {
		log.Fatal("Error Getting Rlimit ", err)
	}

	if rLimit.Cur < rLimit.Max {
		rLimit.Cur = rLimit.Max
		err = syscall.Setrlimit(syscall.RLIMIT_NOFILE, &rLimit)
		if err != nil {
			log.Warn("Error Setting Rlimit:", err)
		}
	}

	err = syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rLimit)
	log.Warn("File descriptor limit is:", rLimit.Cur)
}
