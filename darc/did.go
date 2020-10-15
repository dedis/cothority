package darc

// Support for verification using DIDs on DARCs requires resolving the
// DID to a DID document containing public keys. There is preliminary work
// done in the `darc-resolve-did` branch which relies on an external
// service like Indy VDR to resolve Sovrin/Indy DIDs. One of its limitations is
// that it only supports resolving the latest DID Document and not the DID
// Document as it existed at a certain point in time. This might pose problems
// while replaying the chain. A more concrete solution would involve following
// the Indy `NYM` and `POOL` ledgers which is largely future work.
// The `did-contract` branch implements a patriciatree that may be used to
// verify the state root hash on Indy ledgers.
