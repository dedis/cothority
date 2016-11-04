package main

import (
	"os"
	"strconv"

	"math/rand"

	"time"

	"github.com/dedis/cothority/log"
)

var parallel = 4
var run chan bool

func main() {
	run = make(chan bool)
	nodesS, groupSizeS, failureQuotientS := os.Args[1], os.Args[2], os.Args[3]
	nodes, err := strconv.Atoi(nodesS)
	log.ErrFatal(err)
	groupSize, err := strconv.Atoi(groupSizeS)
	log.ErrFatal(err)
	failureQuotient, err := strconv.Atoi(failureQuotientS)
	log.ErrFatal(err)
	log.Printf("Calculating for:\nNodes: %d\nGroupSize: %d\nfailureQuotient: %d\n",
		nodes, groupSize, failureQuotient)

	for i := 0; i < parallel; i++ {
		go func() {
			for {
				run <- calcOne(nodes, groupSize, float64(1)/float64(failureQuotient))
			}
		}()
	}

	good := 0
	bad := 0

	now := time.Now()
	for {
		if <-run {
			good++
		} else {
			bad++
		}
		if time.Now().Sub(now).Seconds() > 1.0 {
			now = time.Now()
			log.Printf("Good/Bad: %d/%d - Probability: %f",
				good, bad, float64(bad)/float64(good))
		}
	}
}

func calcOne(n, g int, p float64) bool {
	distribution := rand.Perm(n)
	f := int(float64(n) * p)
	fGroup := int(float64(g) * p)
	groupThreshold := fGroup + 1
	failThreshold := g - groupThreshold + 1

	badPerGroup := 0
	for node, t := range distribution {
		if node%g == 0 {
			badPerGroup = 0
		}
		if t < f {
			badPerGroup++
			if badPerGroup >= failThreshold {
				return false
			}
		}
	}
	return true
}
