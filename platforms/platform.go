package platforms

type Platform interface {
	Configure(*Config)
	Build() (error)
	Deploy() (error)
	Start() (error)
	Stop() (error)
}

type Config struct {
	// Number of machines/nodes
	// Total number of hosts = hpn * nmachs
	Nmachs   int
	// How many logservers to start up
	// Total number of servers used: nmachs + nloggers
	Nloggers int
	// hpn is the replication factor of hosts per node: how many hosts do we want per node
	Hpn      int
	// bf is the branching factor of the tree that we want to build
	Bf       int

	// How many messages to send
	Nmsgs    int
	// The speed of messages/s
	Rate     int
	// How many rounds
	Rounds   int
	// Pre-defined failure rate
	Failures int
	// Rounds for root to wait before failing
	RFail    int
	// Rounds for follower to wait before failing
	FFail    int

	// Debugging-level: 0 is none - 5 is everything
	Debug    int
	// Which app to run
	App      string
	// Coding-suite to run 	[nist256, nist512, ed25519]
	Suite    string
}

func NewConfig() *Config {
	return &Config{
		4, 3, 1, 2,
		100, 30, 10, 0, 0, 0,
		2, "stamp", "ed25519"}
}

