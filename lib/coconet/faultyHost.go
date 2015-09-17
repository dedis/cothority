package coconet

type HostState int

const (
	ALIVE HostState = iota
	DEAD
)

const DefaultState HostState = ALIVE

type FaultyHost struct {
	Host

	State HostState

	// DeadFor[x]=true is Host muslt play dead when x occurs
	// ex: DeadFor["commit"] is set true if host must fail on commit
	DeadFor map[string]bool
}

func NewFaultyHost(host Host, state ...HostState) *FaultyHost {
	fh := &FaultyHost{}

	fh.Host = host
	if len(state) > 0 {
		fh.State = state[0]
	} else {
		fh.State = DefaultState
	}
	fh.DeadFor = make(map[string]bool)

	return fh
}

func (fh *FaultyHost) Die() {
	fh.State = DEAD
}

// returns true is host has failed (for simulating failures)
func (fh *FaultyHost) IsDead() bool {
	return fh.State == DEAD
}

// returns true is host should play dead in a specific case
// ex: DeadFor["commit"] is set true if host must fail on commit
func (fh *FaultyHost) IsDeadFor(x string) bool {
	return fh.DeadFor[x]
}

// ex: DeadFor["commit"] is set true if host must fail on commit
func (fh *FaultyHost) SetDeadFor(x string, val bool) {
	fh.DeadFor[x] = val
}
