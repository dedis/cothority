Navigation: [DEDIS](https://github.com/dedis/doc/tree/master/README.md) ::
[Cothority](https://github.com/dedis/cothority/tree/master/README.md) ::
[Applications](https://github.com/dedis/cothority/blob/master/doc/Applications.md) ::
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
  delegation support is configured using [DARC](../byzcoin/README.md#darc).
- Configurable acceptance criteria - we execute a smart-contract on all nodes,
  nodes only accept the event if the smart-contract returns a positive result.
- Existance proof - once an event is logged, an authorised client can request
  a cryptographic proof (powered by [trie](../byzcoin/trie/README.md))
  that the event is indeed stored in the blockchain and has not been tampered.

## Running the service
For the general information about running a conode, or conodes, please see the
[conode documentation](../conode/README.md). EventLog is not stable, so it is
not a part of the `cothority.v2` release, please use the `master` branch.

## Client API

We offer three ways for clients to connect to the event log service. All the
APIs expect an existing ByzCoin object that has a Darc with "spawn:eventlog"
and "invoke:eventlog" in its rules. The eventlog signer (which we will create
below) *must* be authorised to use these rules.

An example transcript of correctly setting up and using an Eventlog is:

```
# make the ByzCoin instance

$ bcadmin c --roster ../../conode/public.toml
Created ByzCoin with ID 7ad741d44e216fc4475da60b8656b904937639415ec27f7003e13408d6e0510c.
export BC="/Users/jallen/Library/Application Support/bc/data/bc-7ad741d44e216fc4475da60b8656b904937639415ec27f7003e13408d6e0510c.cfg"
$ export BC="/Users/jallen/Library/Application Support/bc/data/bc-7ad741d44e216fc4475da60b8656b904937639415ec27f7003e13408d6e0510c.cfg"

# make a keypair for the new eventlog, give it permissions on the genesis darc

$ el create --keys
Identity: ed25519:2a53df71edad603e56477d33e82d675a3499ba4719f809fabea95ce546c16b5f
export PRIVATE_KEY=120c9566ebbcf91887675298485945bad2d3f7be3ae7d6a56bdebd5b8378a80a
$ export PRIVATE_KEY=120c9566ebbcf91887675298485945bad2d3f7be3ae7d6a56bdebd5b8378a80a
$ bcadmin add spawn:eventlog --identity ed25519:2a53df71edad603e56477d33e82d675a3499ba4719f809fabea95ce546c16b5f
$ bcadmin add invoke:eventlog --identity ed25519:2a53df71edad603e56477d33e82d675a3499ba4719f809fabea95ce546c16b5f

# check the persmissions on the genesis darc

$ bcadmin show

# make the new eventlog

$ ./el create 
export EL=b9a6c3868b01e19f6d3d0f62c881582d5a5bd98046dd4e4274b579fd6e66b643
$ export EL=b9a6c3868b01e19f6d3d0f62c881582d5a5bd98046dd4e4274b579fd6e66b643

# use the eventlog

$ ./el log -content test
$ ./el search
2018-09-28 13:42:23		test
```

### Go API

For a working example of the Go API, see the `el` directory, where there is
a command-line interface to the Eventlog.

The detailed API can be found on
[godoc](https://godoc.org/github.com/dedis/cothority/eventlog).

### Java API
In java, you need to construct a `EventLogInstance` class. There are two ways
to initialise it, the first for when you do not have an existing eventlog
instance on ByzCoin to connect to, the other when you do.

```java
// Create the eventlog instance. It expects a ByzCoin RPC, a list of 
// signers that have the "spawn:eventlog" permission and the darcID for where
// the permission is stored.
EventLogInstance el = new EventLogInstance(bcRPC, admins, darcID);
```

If you would like to connect to the same instance, you need to save the result
of `el.getInstanceId()` and of course the ByzCoin RPC. The constructor
`EventLogInstance(ByzCoinRPC bc, InstanceId id)` connects to an existing
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
[source](https://github.com/dedis/cothority/blob/master/external/java/src/main/java/ch/epfl/dedis/lib/byzcoin/contracts/EventLogInstance.java).

### CLI
Please see the `el` documentation [here](el/README.md).
