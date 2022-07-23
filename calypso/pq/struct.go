package pq

import "go.dedis.ch/kyber/v3"

type Write struct {
	Commitments [][]byte
	Publics     []kyber.Point
	CtxtHash    []byte
}
