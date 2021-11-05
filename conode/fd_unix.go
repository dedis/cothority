//go:build !windows && !darwin
// +build !windows,!darwin

package main

import (
	"syscall"

	"go.dedis.ch/onet/v3/log"
)

func init() {
	raiseFdLimit = func() {
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
		log.Info("File descriptor limit is:", rLimit.Cur)
	}
}
