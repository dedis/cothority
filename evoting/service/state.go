package service

import (
	"encoding/hex"
	"sync"
	"time"

	"github.com/dedis/kyber/util/random"
)

// stamp marks a logged-in user.
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

// get retrieves a user stamp from the log.
func (s *state) get(key string) *stamp {
	s.mux.Lock()
	defer s.mux.Unlock()

	return s.log[key]
}

// timeLimit sets how long a user can be logged in.
var timeLimit = 10 * time.Minute

// register a new user in the log and return 32 character nonce as a token.
func (s *state) register(user uint32, admin bool) string {
	s.mux.Lock()

	token := nonce(128)
	s.log[token] = &stamp{user, admin, 0}
	_ = time.AfterFunc(timeLimit, func() {
		s.mux.Lock()
		delete(s.log, token)
		s.mux.Unlock()
	})

	s.mux.Unlock()
	return token
}

// nonce returns a random string for a given bit length.
func nonce(bits uint) string {
	return hex.EncodeToString(random.Bits(bits, false, random.New()))
}
