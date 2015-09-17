package sign

import "time"

// Constants we expect might be used by other packages
// TODO: can this be replaced by the application using the signer?
var ROUND_TIME time.Duration = 1 * time.Second
var HEARTBEAT = ROUND_TIME + ROUND_TIME/2

var GOSSIP_TIME time.Duration = 3 * ROUND_TIME
