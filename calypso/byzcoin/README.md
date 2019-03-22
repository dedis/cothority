Navigation: [DEDIS](https://github.com/dedis/doc/tree/master/README.md) ::
[Cothority](../README.md) ::
[Applications](../doc/Applications.md) ::
[Calypso](../README.md) ::
ByzCoin Plugin

# ByzCoin Plugin

Calypso itself impelemnts the secret-sharing cothority that allows to re-share a secret to an
ephemeral public key. For every re-encryption of the original secret, all nodes need to get to
a consensus that the re-encryption is valid. For this, different plugins can be activated in
Calypso.

This plugin allows ByzCoin to do the authorization-control of the re-encryption:

- A writer creates a write-instance with the encrypted secret
- A reader asks for read-access which is granted depending on the DARC of the write-instance
- If the read-access is granted, the reader can send the proof to the calypso service and get
the secret re-encrypted

## Using it

If you want to use the ByzCoin plugin, it is enough to add

```go
import _ "go.dedis.ch/cothority/v3/calypso/byzcoin"
```

to your main file.