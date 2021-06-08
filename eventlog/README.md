Navigation: [DEDIS](https://github.com/dedis/doc/tree/master/README.md) ::
[Cothority](https://github.com/dedis/cothority/tree/main/README.md) ::
[Applications](https://github.com/dedis/cothority/blob/main/doc/Applications.md) ::
EventLog

# EventLog

The EventLog (EL) service is for logging events into
[ByzCoin](../byzcoin/README.md).
Contrary to ordinary event logging services, we offer better security and
auditability. Below are some of the main features that sets us apart.

- Collective witness - a collection of nodes, or conodes, independently observe
  the logging of an event. The event will only be accepted if a 2/3-majority
  think it is valid, e.g., the timestamp is reasonable, the client is
  authorised and so on.
- Distributed access control - fine-grained client access control with
  delegation support is configured using [DARC](../darc/README.md#darc).
- Configurable acceptance criteria - we execute a smart-contract on all nodes,
  nodes only accept the event if the smart-contract returns a positive result.
- Existance proof - once an event is logged, an authorised client can request
  a cryptographic proof (powered by [trie](../byzcoin/trie/README.md))
  that the event is indeed stored in the blockchain and has not been tampered.

## Running the service
The EL service is built into conodes. For the general information about
running a conode, please see the [conode documentation](../conode/README.md).

## Client API

We offer three ways for clients to connect to the EL service. All the
APIs expect an existing ByzCoin object that has a Darc with "spawn:eventlog"
and "invoke:eventlog" in its rules. The eventlog signer (which we will create
below) *must* be authorised to use these rules.

An example transcript of correctly setting up and using an Eventlog is:

```
# Make the ByzCoin instance

$ bcadmin c -roster ../../conode/public.toml
Created ByzCoin with ID 7ad741d44e216fc4475da60b8656b904937639415ec27f7003e13408d6e0510c.
export BC="/Users/jallen/Library/Application Support/bc/data/bc-7ad741d44e216fc4475da60b8656b904937639415ec27f7003e13408d6e0510c.cfg"
$ export BC="/Users/jallen/Library/Application Support/bc/data/bc-7ad741d44e216fc4475da60b8656b904937639415ec27f7003e13408d6e0510c.cfg"

# Make a keypair for the new eventlog, give it permissions on the genesis darc.
# The private key is stored in the el configuration directory.
$ el key
ed25519:2a53df71edad603e56477d33e82d675a3499ba4719f809fabea95ce546c16b5f
$ bcadmin darc rule -rule spawn:eventlog -identity ed25519:2a53df71edad603e56477d33e82d675a3499ba4719f809fabea95ce546c16b5f
$ bcadmin darc rule -rule invoke:eventlog.log -identity ed25519:2a53df71edad603e56477d33e82d675a3499ba4719f809fabea95ce546c16b5f

# Check the persmissions on the genesis darc.
$ bcadmin darc show

# Make the new eventlog.
$ ./el create -sign ed25519:2a53df71edad603e56477d33e82d675a3499ba4719f809fabea95ce546c16b5f
export EL=b9a6c3868b01e19f6d3d0f62c881582d5a5bd98046dd4e4274b579fd6e66b643
$ export EL=b9a6c3868b01e19f6d3d0f62c881582d5a5bd98046dd4e4274b579fd6e66b643

# Use the eventlog.
$ ./el log -sign ed25519:2a53df71edad603e56477d33e82d675a3499ba4719f809fabea95ce546c16b5f -topic "hello" -content "world"
$ ./el search
```

### Go API

The detailed API can be found on
[godoc](https://godoc.org/go.dedis.ch/cothority/eventlog). You may find example
usage in `api_test.go`.

### Java API
In java, you need to construct a `EventLogInstance` class. There are two ways
to initialise it, the first for when you do _not_ have an existing eventlog
instance on ByzCoin to connect to, the other is when you do. Below we give a
general overview. Please see the [Java docs](https://www.javadoc.io/doc/ch.epfl.dedis/cothority)
for more information.

```java
// Create the eventlog instance. It expects a ByzCoin RPC,
// the darc ID that has the "spawn:eventlog" rule, a list of
// signers that are authorized to in the "spawn:eventlog" rule
// and the counters (used for preventing replay attacks).
// You can get the counters using bcRPC.getSignerCounters.
EventLogInstance el = new EventLogInstance(bcRPC, darcID, admins, signerCounters);
```

If you would like to connect to the same instance, you need to save the result
of `el.getInstanceId()` and the ByzCoin RPC. Then use
`EventLogInstance.fromByzcoin(ByzCoinRPC bc, InstanceId id)` to connects to an
existing instance.

It's straightforward to log events, as long as the event is correctly signed.
The signer must be the one with the "invoke:eventlog.log" permission.
```java
Event event = new Event("login", "alice");
InstanceId key = this.el.log(event, signers, signerCounters);
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

### CLI
Please see the `el` documentation [here](el/README.md).
