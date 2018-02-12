# nevv

[![Build Status](https://travis-ci.org/dedis/student_17_evoting.svg?branch=master)](https://travis-ci.org/dedis/student_17_evoting)
[![Coverage Status](https://coveralls.io/repos/github/dedis/student_17_evoting/badge.svg?branch=master)](https://coveralls.io/github/dedis/student_17_evoting?branch=master&service=github)

nevv is a decentralized and distributed e-voting architecture based on verifiable Neff
shuffles.\
It is based on the ideas laid out in the Helios project [1, 2].

## Architecture
nevv is built on top of the cothority architecture developped by DEDIS at EFPL. It provides
a framework to deploy distributed and decentralized systems and protocols. The cothority
uses an alternative blockchain implementation called skipchain to store data [3].

The backbone of the system is a master skipchain storing general configurations.
Each election is stored in a separate chain with the same sequence of blocks.
The life cycle of an election is driven by three underlying protocols.

 - DKG: Distributed key generation algorithm run upon creation of an election [4].
 - Neff: After termination each node produces a shuffle of the ballots with a proof.
 - Decrypt: Each node partially decrypts the ballots.

<p align="center">
  <img src="arch.png" width="400" height="325" />
</p>

## API
The system is accessible through protocol buffer message over websockets.
See ```api.proto``` for a complete overview.

```protobuf
message Login{} // Register in the system
message Open{} // Create a new election
message Cast{} // Cast a ballot in an election
message Shuffle{} // Initiate the shuffle protocol
message Decrypt{} // Start the decryption protocol
message GetBox{} // Get encrypted ballots of an election
message GetMixes{} // Get all the created mixes
message GetPartials{} // Get all the partially decrypted ballots
message Reconstruct{} // Reconstruct plaintext from partials
```

## Installation
```shell
git clone https://github.com/dedis/student_17_evoting
```

## Setup
TODO

## References
[1] **Verifiable Mixing (Shuffling) of ElGamal Pairs**; *C. Andrew Neff*, 2004\
[2] **Helios: Web-based Open-Audit Voting**; *Ben Adida*, 2008\
[3] **Decentralizing authorities into scalable strongest-link cothorities**: *Ford et. al.*, 2015\
[4] **Secure distributed key generation for discrete-log based cryptosystems**; *Gennaro et. al.*, 1999
