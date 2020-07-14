# Cothority API v2

This is a new start for the cothority API, mostly geared towards ByzCoin. Compared to the old API, it tries to avoid
the following errors:

- mix between definitions of services and live data
    - define the constants in a central place
    - have services implementations with more information, e.g., skipchain should be bound to one ID
    - instance-definitions that separate correctly the data and the update
- manual updating of instances from byzcoin
    - use the new `byzcoin.GetUpdats` service endpoint to get informed when new things happen

The v1 will still be kept around, but at least the byzcoin-contracts will be reproduced in v2.
Perhaps also the services might get a copy in v2.

## Elements of every contract

For every contract described in v2, the following elements must be present:
- `NameStruct` to merge the `Instance` and the protobuf representation of `Name`
- `NameContract` as a namespace representing all constants needed to work with the contract
    - `contractID` - as given in ByzCoin
    - `attribute*` - all attributes that can be passed to an existing instance
    - `command*` - all commands available in the instance
    - `rule(spawn|*)` - available rules, one for `spawn`ing, and one for every command
- `NameInst` as a `BehaviorSubject<NameStruct>` with:
    - `retrieve(ByzCoinRPC, InstanceID)`
    - `commands*` - as in `NameContract`, but for this instance
