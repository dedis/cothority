Navigation: [DEDIS](https://github.com/dedis/doc/tree/master/README.md) ::
[Cothority](../README.md) ::
[Apps](../doc/Applications.md) ::
E-voting

# E-voting

Our e-voting system is inspired by the first version of Helios where the encrypted
ballots are shuffled and anonymized before they are decrypted. Instead of the
shuffle used in Helios, we implemented a Neff shuffle which is much faster than
the original Helios shuffle.

To further reduce single points of failure we introduce the cothority and store
all the votes on a skipchain, so that they can be publicly verified by a third
party.

As this system is to be used in an [EPFL](https://epfl.ch) election, we added
an authentication to the Gaspar/Tequila service.

The evoting service has the following features:

* Distributed setup by election overseers
* Authentication through Tequila
* Anonymity of the votes through encryption on the client’s machine
* Tamper-proof storage of all votes on a blockchain
* Verifiable distributed shuffling using Neff-shuffles
* Shared decryption once the election is over
* Verifiability of the process by external verifiers

# Implementation

<p align="center">
  <img src="system.png" />
</p>

## Background

EPFL conducts elections across different departments and associations every year.
In fall, 2017, the DEDIS Lab at EPFL started working on implementing an evoting
systems to be used for Elections in May, 2018. The voting system aims to be
tamper-proof, auditable and decentralised.

## Decentralization
The evoting system runs on a set of nodes, which collectively perform all the
operations. The nodes, called conodes (https://github.com/dedis/cothority) make
extensive use of a custom blockchain implementation by the DEDIS Lab, called
Skipchains. Skipchains can be thought of as a distributed tamper resistant
datastore (as long as 2/3rds of the participating nodes are not malicious).
Addition of new data "blocks" to the chain must be ratified by 2/3rd majority
of participating nodes. We use skipchains to store information pertaining to
elections in the following way:

<p align="center">
  <img src="arch.png" width="400" height="325" />
</p>

We have two levels of indirection -  the master skipchain and a per election
skipchain. The first block of the master skipchain holds configuration common
to all elections like the list of participating nodes in evoting, public key of
authorization server to verify signatures (refer to Identification section below),
IDs of administrators who’re allowed to setup new elections. Further blocks hold
a reference to the first block of an election specific skipchain and they’re
added when an election administrator proposes a new election.

The per-election skipchain’s first block contains information about the particular
information, such as the list of candidate IDs and eligible voter IDs. The election
creation protocol results in each node calculating a secret which it keeps to
itself (Distributed Key Generation). This secret is then used to construct an
aggregate public key for the election that is used to encrypt every voters ballot
for the election. The distribution of secret allows us to decentralise the
shuffling and voting process which is described later in the text.

## Identification
We rely on using EPFL’s authentication service, called Tequila to identify a user.
The identification process requires a central server that interacts with tequila
and generates a signature on successful authorization. This signature is then
verified on every conode before performing any election operation.

## Vote encryption
The evoting web application allows an administrator to set up a "choose M of N"
type of election. A voter after logging in may select his/her choice(s).

Before submitting the ballot to a conode, the web application encrypts the ballot
using the election's aggregate public key. The submission of encrypted ballot
results in the addition of a new block to the skipchain. The block contains the
ID of the user who cast his/her vote and their vote in encrypted format. We
therefore notice that at this point, the evoting system prevents an adversary
from figuring out who a particular voter voted for but it doesn’t stop the
adversary from figuring out if they have voted or not voted at all. It should
also be noted that the current implementation does not allow a voter to verify
if their vote was cast as intended, i.e. if the encrypted ballot is not changed
while being transfered to the conode by a malware on their device. The voter can
however, verify if their vote is indeed stored or not in the skipchain.


## Shuffling and Decryption of Ballots
In order to preserve anonymity of votes, we need to remove voter information from
the encrypted ballots and permute and store them such that no adversary can
determine a vote from the shuffled permutation. At the same time, it is essential
that an auditor should be able to verify if the shuffling has been done correctly
and the nodes themselves have not acted maliciously. Neff Shuffles are well suited
for this task and provide a proof that allows auditors to verify that the shuffles
have been performed correctly.

After the shuffling phase, the ballots are anonymized but still encrypted. On
receiving a decryption request, every conode decrypts the ballot using their share of the secret.
These partial decryptions can then be used to reconstruct the fully decrypted ballots
(as long as a configurable threshold of nodes are able to verify the shuffle and
partially decrypt the ballots). The distribution in decryption phase gives no
single node full control over the decryption of ballots and to act maliciously.
Finally, the decrypted anonymised ballots are stored in the skipchain and they
can be used to aggregate the vote counts for each candidate.

# Usage

## Docker setup

Dockerhub contains an image of a conode with the evoting service. The following
instructions assume you have docker installed and running.

* Pull the image from Dockerhub

```
$ docker pull dedis/conode:evoting
```

* Prepare configurations files for the conode

```
$ docker run -it --rm -P --name conode -v ~/conode_data:/conode_data dedis/conode:evoting ./conode setup
```

The above command would write the conode configuration files (public/private keys
and database) to `~/conode_data`

* Start a container to run the evoting service

```
$ docker run --rm -P --name conode -v ~/conode_data:/conode_data dedis/conode:evoting
```

* To ensure the container is up and running the output of `docker ps` should be
something like

```
CONTAINER ID        IMAGE                  COMMAND                  CREATED                  STATUS              PORTS                                              NAMES
7fd75268fdbd        dedis/conode:evoting   "./conode -debug 3 s…"   Less than a second ago   Up 2 seconds        0.0.0.0:32843->6879/tcp, 0.0.0.0:32842->6880/tcp   conode
```

Note that the host ports are assigned randomly by docker. Please use the `-p` flag
to have more control over port mappings.


## Managing the master skipchain

`app/app.go` provides a way to manage the master skipchain for evoting. Assuming
you have a go environment set up, the following instructions describe how to build
the evoting app

```bash
$ go get github.com/dedis/cothority/evoting/app
$ cd $GOPATH/src/github.com/dedis/cothority/evoting/app && go build -o $GOPATH/bin/evoting ./...
```

If `$GOPATH/bin` is in your `$PATH` then the evoting app should be accessible by

```bash
$ evoting --help
```

# Links
- Student Project: EPFL e-voting:
  - [Backend](https://github.com/dedis/student_17/evoting-backend)
  - [Frontend](https://github.com/dedis/epfl-evoting/tree/master/evoting)
- Paper: **Verifiable Mixing (Shuffling) of ElGamal Pairs**; *C. Andrew Neff*, 2004
- Paper: **Helios: Web-based Open-Audit Voting**; *Ben Adida*, 2008
- Paper: **Decentralizing authorities into scalable strongest-link cothorities**: *Ford et. al.*, 2015
- Paper: **Secure distributed key generation for discrete-log based cryptosystems**; *Gennaro et. al.*, 1999
