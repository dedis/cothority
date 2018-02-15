package service

import (
	"math/rand"
	"sync"
	"time"
)

const limit = 5

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
	mux sync.Mutex
	// log is map from nonce to user stamp.
	log map[string]*stamp
}

// handle increments all user timestamps and removes the users whose timestamps equal the limit.
func (s *state) handle() {
	s.mux.Lock()
	for key, value := range s.log {
		if value.time == limit {
			delete(s.log, key)
		} else {
			value.time++
		}
	}
	s.mux.Unlock()
}

// get retrieves a user stamp from the log.
func (s *state) get(key string) *stamp {
	s.mux.Lock()
	defer s.mux.Unlock()

	return s.log[key]
}

// schedule periodically increments the time counter for each user in the
// state log and removes him if the time limit has been reached.
func (s *state) schedule(interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		for {
			select {
			case <-ticker.C:
				s.handle()
			}
		}
	}()
}

// register a new user in the log and return 32 character nonce as a token.
func (s *state) register(user uint32, admin bool) string {
	s.mux.Lock()
	token := nonce(32)
	s.log[token] = &stamp{user, admin, 0}
	s.mux.Unlock()
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
