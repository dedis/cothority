Navigation: [DEDIS](https://github.com/dedis/doc/tree/master/README.md) ::
[Cothority](../README.md) ::
Links

# Cothority-links

This document collects web-pages, blog-posts, wikis,
godocs, papers and more in a central place as a reference.

## Key Repositories

Software quality is often emphasised in our research. Therefore, the key
repositories are often useful for the industry, the general public and future
research projects. We organise it into three key repositories below.

### Cothority

Cothority is a collection of applications that run
on a set of servers called conodes.

- An [overview](../README.md) of its functionalities
- [Godoc](https://godoc.org/github.com/dedis/cothority) of the project
- Two webpages:
  - [Pulsar](https://pulsar.dedis.ch) - publicly verifiable randomness
  - [Status](http://status.dedis.ch) - status of our test-network
- [CISC](../cisc/README.md) - store ssh keys, webpages or any key/value pair on a blockchain
- [Javascript](../external/js/cothority/README.md)
code to connect to conodes

### Kyber

Kyber is a library exporting basic cryptographic primitives mainly geared
towards elliptic curves.

- An [overview](https://github.com/dedis/kyber/blob/master/README.md)
- [Godoc](https://godoc.org/github.com/dedis/kyber) of the project
- [Javascript](../external/js/kyber/README.md)
code to use basic cryptographic primitives - not as extensive as the kyber library

### Onet

Overlay Network (onet) is the framework used by cothority to define the protocols,
services and apps. It offers peer-to-peer connections and a websockets interface
for communication with clients.

- An [overview](https://github.com/dedis/onet/wiki) of its functionalities
- [Godoc](https://godoc.org/github.com/dedis/onet) of the project

### Others

- [Cothority template](https://github.com/dedis/cothority_template/wiki) is a good
place to start if you want to implement your own protocol and/or service and
connect it to an app.
- [Protobuf](https://github.com/dedis/protobuf) is a simple protobuf implementation
that ignores the niceness of .proto-files and uses go-structures only
- PriFi
- MedCo
- Mobi

## Blog Posts

Bryan Ford, the professor at EPFL's [DEDIS](https://dedis.epfl.ch) lab, has a number
of blog posts related to the cothority:

- [Skipchains](https://bford.github.io/2017/08/01/skipchain/) - how do you know
it's on the blockchain?
- [Byzcoin](https://bford.github.io/2016/10/25/mining/) - Untangling mining
incentives in Byzcoin and Bitcoin
- [CoSi](http://bford.github.io/2016/03/10/apple/) - Apple, FBI and software transparency
- [PoP](https://bford.github.io/2015/10/07/names.html) - Let's verify real people,
not real names

## Papers

A number of papers have been written that are implemented partially or fully
in the cothority:

- SCARAB (Onchain-Secrets): Hidden in plain sight
https://eprint.iacr.org/2018/209
- OmniLedger: A Secure, Scale-Out, Decentralized Ledger via Sharding.
https://eprint.iacr.org/2017/406
- MedCo: Enabling Privacy-Conscious Exploration of Distributed Clinical and Genomic Data. https://infoscience.epfl.ch/record/232605/files/GenoPri17_paper_6_CAMERA_READY.pdf?version=1
- Scalable Bias-Resistant Distributed Randomness. https://infoscience.epfl.ch/record/230355/files
- CHAINIAC: Proactive Software-Update Transparency via Collectively Signed Skipchains and Verified Builds. https://infoscience.epfl.ch/record/229405/files/usenixsec17-final.pdf?version=1
- UnLynx: A Decentralized System for Privacy-Conscious Data Sharing. https://infoscience.epfl.ch/record/229308?ln=en
- PriFi: A Low-Latency and Tracking-Resistant Protocol for Local-Area Anonymous Communication. https://infoscience.epfl.ch/record/223389/files/p181-barman.pdf?version=1
- AnonRep: Towards Tracking-Resistant Anonymous Reputation. https://infoscience.epfl.ch/record/223118?ln=en
- Keeping Authorities “Honest or Bust” with Decentralized Witness Cosigning. https://infoscience.epfl.ch/record/221010/files/1503.08768v4.pdf?version=1
- Bitcoin Meets Collective Signing. https://infoscience.epfl.ch/record/220211/files/16-poster_abstract.pdf?version=1
- Managing Identities Using Blockchains and CoSi. https://infoscience.epfl.ch/record/220210/files/1_Managing_identities_bryan_ford_etc.pdf?version=1
- Enhancing Bitcoin Security and Performance with Strong Consistency via Collective Signing. https://infoscience.epfl.ch/record/220209?ln=en
- Seeking Anonymity in an Internet Panopticon https://infoscience.epfl.ch/record/214120?ln=en
- Dissent: Accountable Group Anonymity.
https://infoscience.epfl.ch/record/212686/files/ccs88-corrigan-gibbs.pdf?version=1
- Low-latency Blockchain Consensus. https://infoscience.epfl.ch/record/228942/files/main.pdf?version=2

## Webpages

- [Pulsar](https://pulsar.dedis.ch) - publicly verifiable randomness
- [Status](http://status.dedis.ch) - status of our test-network
- [PoP](https://pop.dedis.ch) - 2nd pop-party ever held
