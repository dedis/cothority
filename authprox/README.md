# Authentication Proxy

A service that takes authentication info and a message as input, checks that the
authentication info is valid, encodes an identifier from the auth info into the
given message, and uses a sharded secret key to give a partial signature back
to the client.

From there, the client interpolates several signatures and sends the resulting
signature to a downstream system that needs to know if a quorum of Authentication
Proxies have seen evidence of an authentication claim.

The first (and probably only) user of this is byzcoin/darc/darc.go's
type IdentityProxy, where the Verify function implements the verification
side of this scheme.

