Navigation: [DEDIS](https://github.com/dedis/doc/tree/master/README.md) ::
[Cothority](../README.md) ::
Migration to V1

# Migration From V0 to V1

If you are about to migrate from V0 to V1, here is a summary of things to take
care of:
- moving of libraries
- depend on V1
- renaming of methods
- new method-signatures
- simulation

Of course you do all this in a new branch, so if something goes wrong, you can
easily go back!

## Moving of libraries

Some libraries have moved to a new place. Run these commands in the base-directory
of your project to link to the new libraries. This will also update all your
dependencies to `onet.v1`, `crypto.v0`, and `cothority.v1`.

```bash
#!/usr/bin/env bash

for path in :github.com/dedis/cothority/sda:gopkg.in/dedis/onet.v1: \
    :github.com/dedis/cothority/network:gopkg.in/dedis/onet.v1/network: \
    :github.com/dedis/cothority/log:gopkg.in/dedis/onet.v1/log: \
    :github.com/dedis/cothority/monitor:gopkg.in/dedis/onet.v1/simul/monitor: \
    :github.com/dedis/cothority/crypto:gopkg.in/dedis/onet.v1/crypto: \
    :github.com/dedis/crypto:gopkg.in/dedis/crypto.v0: \
    :github.com/dedis/cothority/protocols/manage:gopkg.in/dedis/cothority.v1/messaging:; do
        find . -name "*go" | xargs perl -pi -e "s$path"
done
```

In general, protocols, services, and apps moved from `dedis/cothority/(protocols|services|app)/*`
directly in the root-directory of `dedis/cothority`

## Renaming of methods

This is a list of new names for methods that you can do with a simple

```bash

for oldnew in sda\\.:onet. \
	manage\\.:messaging. \
	network\\.Body:network.Message \
	onet\\.ProtocolRegisterName:onet.GlobalProtocolRegister \
	network\\.RegisterHandler:network.RegisterMessage \
	ServerIdentity\\.Addresses:ServerIdentity.Address \
	CreateProtocolService:CreateProtocol \
	CreateProtocolSDA:CreateProtocol \
    RegisterPacketType:RegisterMessage \
    network\\.Packet:network.Envelope sda\\.Conode:onet.Server \
    UnmarshalRegistered:Unmarshal MarshalRegisteredType:Marshal ; do
    	echo replacing $oldnew
        find . -name "*go" | xargs -n 1 perl -pi -e s:$oldnew:g
done
```

Take care of the correct order, as `RegisterMessage` exists once in the old
`ServiceProcessor`, and once in the new `network`-library.

## New method-signatures

These methods and interfaces changed, please adjust manually:

* Old: `msgT, msgVal, err := UnmarshalRegisteredType(buffer, constructor)`
* New: `msgT, msgPointer, err := Unmarshal(buffer)`

So take care and rewrite your methods to use the `msgPointer` and not a `msgValue`.

* Old: `NewServiceFunc func(c *Context, path string) Service`
* New: `NewServiceFunc func(c *Context) Service`

There is no path passed to the `NewServiceFunc` anymore. But you can use
`context.Load` and `context.Save` instead.

* Old: `func(si *network.ServerIdentity, msg interface{})(ret interface{}, err error)`
* New: `func(msg interface{})(ret interface{}, err ClientError)`

The function given to `context.RegisterHandler` doesn't get the caller's `si`
anymore, as this was mostly an anonymous random client. The return-value is not
an `error` anymore, but a `ClientError` that can also return a numerical error-
code that can be tested in the caller.

You can use `onet.NewClientError` to convert an `error` into a `ClientError` or
`onet.NewClientErrorCode` to return an error with a code. If you chose the latter
option, you're only allowed to return error-codes from 4200 to 4999, included.

* Old: `reply, err := c.Send(dst, msg)`
* New: `cerr := c.SendProtobuf(dst, msg, *reply)`

Instead of getting the message as a return from the `Send`-method, you can now
give a pointer to a struct to the `SendProtobuf`-method. If `*reply` is `nil`,
then the result of the call to `dst` will be discarded.

* Old: `CreateProtocol(SDA|Service)`
* New: `CreateProtocol`

There is no distinction anymore between `CreateProtocolSDA` and `CreateProtocolService`.

* Old: `Service.NewProtocol` was needed
* New: if `Service.NewProtocol` returns `nil, nil`, ONet handles creation of the
protocol. This is the default behaviour in `ServiceProcessor`

### Propagation-function

The propagation-function `manage.Propagation` changed the way it's used. Now you
can define a propagation-function like so:

```go
func propagationCaller(network.Message){
	// Do something with the message
}

// Later
	propagate, err :=
		messaging.NewPropagationFunc(c, "ProtocolID", propagationCaller)
	// Use propagate
	replies, err := propagate(roster, msg, msec)
```

In case you have to broadcast something to all nodes of a roster.

## Update

Once you changed all those libraries, imports and methods, you're ready to
update and build again:

```go
go get -u ./...
go build .
```
