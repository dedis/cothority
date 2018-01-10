// +build darwin linux

package main

import (
	"log"
	"syscall"
)

func raiseLimit() {
	// Raising the FD limit like this might belong in a more central place,
	// but for the moment, this is the only test where it is biting us.
	// (and even then, only for Jeff's Mac: why?)
	var rLimit syscall.Rlimit
	err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rLimit)
	if err != nil {
		log.Fatal("Error Getting Rlimit ", err)
	}

	if rLimit.Cur < 2048 {
		rLimit.Cur = 2048
		err = syscall.Setrlimit(syscall.RLIMIT_NOFILE, &rLimit)
		if err != nil {
			log.Fatal("Error Setting Rlimit ", err)
		}
	}
}
