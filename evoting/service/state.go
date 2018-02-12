package service

import (
	"math/rand"
	"time"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

// stamp marks an logged in user in the log.
type stamp struct {
	// user identifier (Sciper number).
	user uint32
	// admin flags if the user has admin priviledge.
	admin bool
	// time shows how long the stamp is already in the log.
	time int
}

// state is a wrapper around the log map.
type state struct {
	// log is map from nonce to user stamp.
	log map[string]*stamp
}

// schedule periodically increments the time counter for each user in the
// state log and removes him if the time limit has been reached.
func (s *state) schedule(interval time.Duration) chan bool {
	ticker := time.NewTicker(interval)
	stop := make(chan bool)
	go func() {
		for {
			select {
			case <-ticker.C:
				for nonce, stamp := range s.log {
					if stamp.time == 5 {
						delete(s.log, nonce)
					} else {
						stamp.time++
					}
				}
			case <-stop:
				ticker.Stop()
				return
			}
		}
	}()

	return stop
}

// register a new user in the log and return 32 character nonce as a token.
func (s *state) register(user uint32, admin bool) string {
	token := nonce(32)
	s.log[token] = &stamp{user, admin, 0}
	return token
}

// nonce returns a random string for a given length n.
func nonce(n int) string {
	const chars = "0123456789abcdefghijklmnopqrstuvwxyz"

	bytes := make([]byte, n)
	for i := range bytes {
		bytes[i] = chars[rand.Intn(len(chars))]
	}
	return string(bytes)
}
