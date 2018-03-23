Navigation: [DEDIS](https://github.com/dedis/doc/tree/master/README.md) ::
[Cothority](../README.md) ::
[Building Blocks](../doc/BuildingBlocks.md) ::
RandHound

# RandHound

Bias-resistant public randomness is a critical component
in many (distributed) protocols. Generating public randomness
is hard, however, because active adversaries may behave
dishonestly to bias public random choices toward their advantage.
Existing solutions do not scale to hundreds or thousands
of participants, as is needed in many decentralized systems.
We propose two large-scale distributed protocols, RandHound
and RandHerd, which provide publicly-verifiable, unpredictable,
and unbiasable randomness against Byzantine adversaries. RandHound
relies on an untrusted client to divide a set of randomness
servers into groups for scalability, and it depends on the pigeonhole
principle to ensure output integrity, even for non-random,
adversarial group choices. RandHerd implements an efficient,
decentralized randomness beacon.

RandHerd is structurally
similar to a BFT protocol, but uses RandHound in a one-time
setup to arrange participants into verifiably unbiased random
secret-sharing groups, which then repeatedly produce random
output at predefined intervals. Our prototype demonstrates that
RandHound and RandHerd achieve good performance across
hundreds of participants while retaining a low failure probability
by properly selecting protocol parameters, such as a group size
and secret-sharing threshold. For example, when sharding 512
nodes into groups of 32, our experiments show that RandHound
can produce fresh random output after 240 seconds. RandHerd,
after a setup phase of 260 seconds, is able to generate fresh
random output in intervals of approximately 6 seconds. For this
configuration, both protocols operate at a failure probability of
at most 0.08% against a Byzantine adversary.

## Research Papers

- [RandHound](https://eprint.iacr.org/2016/1067.pdf)Scalable Bias-Resistant
Distributed Randomness
