# EventLog

The EventLog (EL) service is for logging events into
[OmniLedger](../omniledger/README.md).
Contrary to ordinary event logging services, we offer better security and
auditability. Below are some of the main features that sets us apart.

- Collective witness - a collection of nodes, or conodes, indepdently observe
  the logging of an event. The event will only be accepted if a 2/3-majority
  think it is valid, e.g., the timestamp is reasonable, the client is
  authorised and so on.
- Distributed access control - fine-grained client access control with
  delegation support is configured using [DARC](../omniledger/README.md#darc).
- Configurable acceptance criteria - we execute a smart-contract on all nodes,
  nodes only accept the event if the smart-contract returns a positive result.
- Existance proof - once an event is logged, an authorised client can request
  for a cryptographic proof (powered by [collection](../omniledger/collection/README.md))
  that the event is indeed stored in the blockchain and has not been tampered.

## Running the service
For the general information about running a conode, or conodes, please see the
[conode documentation](../conode/README.md). EventLog is not stable, so it is
not a part of the `cothority.v2` release, please use the `master` branch.

Alternatively, you can try to use the conodes hosted by us. However, we do not
guarantee that they'll always be up to date.
TODO: add more info about how to connect to our conodes.

## Client API
We offer three ways for clients to connect to the event log service.

### Go API
To get started, you need a signer and a roster, then we can initialise the
client like so:
```go
r := onet.NewRoster(/* server identities */)
signer := darc.NewSignerEd25519(nil, nil)
c := eventlog.NewClient(r)
err := c.Init(signer, 5*time.Second)
if err != nil {
	// something bad happened, check the error message
}
```
If initialisation is ok, you can log an event using `Log`, which would return
the ID of the event. A new event can be created using `eventlog.NewEvent`. With
the event ID, one can use `GetEvent` to retrieve the event later.

The detailed API can be found on
[godoc](https://godoc.org/github.com/dedis/cothority/eventlog).

### Java API
The Java API is nearly identical to the Go API. To start, initialise the
EventLog class like so:
```java
long blockInterval = 1000000000; // 1 second, blockInterval is in nanoseconds
EventLog el = new EventLog(roster, signers, blockInterval);
```
See the class documentation for how to initialise a roster, usually we parse it
from a TOML file. If no exceptions are thrown, we can log and read events using
`log` and `get`.
```java
Event event = new Event("login", "alice");
byte[] key = this.el.log(event);
// wait for the block to be added
Event event2 = this.el.get(key);
assertTrue(event.equals(event2));
```

### CLI
Please see the `el` documentation [here](el/README.md).
