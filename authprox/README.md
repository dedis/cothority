Navigation: [DEDIS](https://github.com/dedis/doc/tree/master/README.md) ::
[Cothority](../README.md) ::
[Applications](../doc/Applications.md) ::
Authentication Proxy

# Authentication Proxy

A service that takes authentication info and a message as input, checks that the
authentication info is valid, encodes an identifier from the auth info into the
given message, and uses a sharded secret key to give a partial signature back
to the client.

From there, the client interpolates the partial signatures and sends the
resulting final signature to a downstream system that needs to know if a quorum
of Authentication Proxies have seen evidence of an authentication claim.

The first user of this is byzcoin/darc/darc.go's type IdentityProxy, where the
Verify function implements the verification side of this scheme.

## Enrolling new authentication systems

In order to start generating partial signatures attesting to some authentication
information, a set of authentication proxies need to have shares of a secret
stored into them. An administrator of the system will use the `apadmin` tool to do this.

During the enrollment process, a secret is generated in the RAM of the `apadmin`
tool, sharded, and sent to the authentication proxies. It then exits, causing the
only copy of the complete secret to be lost. All users of the system need
to trust that `apadmin` has discarded the original secret key. (Having the authentication
proxies execute the DKG protocol among them would remove this risk, but we
decided that was overkill for this job.)

The roster of the authentication proxies can be disjoint from the roster of
the distributed system that will consume (i.e. verify) the generated signatures.
(However, the only example client for the moment is `el`, which assumes that
the Authentication Proxy roster is one and the same as the Byzcoin roster.)

The threshold for reassembling full signatures from the partial signatures
is fixed during enrollment. For n servers, the threshold is set at
n - (n-1)/3, i.e. for 7 servers, 5 signatures are required.

## Signatures

Clients gather some kind of evidence from the identity provider showing what
their name is. The clients then present this information, and a message to be
signed, to the Authentication Proxies one after another (or in parallel).  When
the client has gathered the threshold of these partial signatures, it
reassembles them into a final signature. It can then present that final
signature to a system that wants proof of the ID of the human driving the
client.

The first (and likely only) system that consumes these user/message binding
signatures is the `proxy` Darc Identity type. Thus, Byzcoin instances which are
protected by a Darc with a rule like "proxy$pubkey:user@example.com" will only
be accessible when a transaction arrives with a signature for that Darc Identity.
