Navigation: [DEDIS](https://github.com/dedis/doc/tree/master/README.md) ::
[Cothority](../../README.md) ::
[Apps](../../doc/Apps.md) ::
[E-Voting](../README.md) ::
Client API

# Client API

The system is accessible through protocol buffer message over websockets.
See ```struct.go``` for a complete overview.

```protobuf
message Open{} // Create a new election
message Cast{} // Cast a ballot in an election
message Shuffle{} // Initiate the shuffle protocol
message Decrypt{} // Start the decryption protocol
message Reconstruct{} // Reconstruct plaintext from partials
message GetElections{} // Retrieve all elections for a user
message GetBox{} // Get encrypted ballots of an election
message GetMixes{} // Get all the created mixes
message GetPartials{} // Get all the partially decrypted ballots
```
