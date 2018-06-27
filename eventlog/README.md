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
  a cryptographic proof (powered by [collection](../omniledger/collection/README.md))
  that the event is indeed stored in the blockchain and has not been tampered.

## Running the service
For the general information about running a conode, or conodes, please see the
[conode documentation](../conode/README.md). EventLog is not stable, so it is
not a part of the `cothority.v2` release, please use the `master` branch.

## Client API
We offer three ways for clients to connect to the event log service. All the
APIs expect an existing OmniLedger object that has a darc with "spawn:eventlog"
and "invoke:eventlog" in its rules. The eventlog signer (which we will create
below) *must* be authorised to use these rules.

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
In java, you need to construct a `EventLogInstance` class. There are two ways
to initialise it, the first for when you do not have an existing eventlog
instance on omniledger to connect to, the other when you do.

```java
// Create the eventlog instance. It expects an omniledger RPC, a list of 
// signers that have the "spawn:eventlog" permission and the darcID for where
// the permission is stored.
EventLogInstance el = new EventLogInstance(olRPC, admins, darcID);
```

If you would like to connect to the same instance, you need to save the result
of `el.getInstanceId()` and of course the omniledger RPC. The constructor
`EventLogInstance(OmniledgerRPC ol, InstanceId id)` connects to an existing
instance.

It's straightforward to log events, as long as the event is correctly signed.
The signer must be the one with the "invoke:eventlog" permission.
```java
Event event = new Event("login", "alice");
InstanceId key = this.el.log(event, signers);
// wait for the block to be added
Event event2 = this.el.get(key);
assertTrue(event.equals(event2));
```

We also have a search API, which allows searching for a particular topic within
a time-range.
```java
long now = System.currentTimeMillis() * 1000 * 1000;
SearchResponse resp = el.search("", now - 1000, now + 1000);
```

Please refer to the javadocs for more information. The javadocs are not hosted
anywhere unfortunately, but it is possible to generate them from the
[source](https://github.com/dedis/cothority/blob/master/external/java/src/main/java/ch/epfl/dedis/lib/omniledger/contracts/EventLogInstance.java).

### CLI
Please see the `el` documentation [here](el/README.md).
