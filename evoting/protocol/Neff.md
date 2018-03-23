Navigation: [DEDIS](https://github.com/dedis/doc/tree/master/README.md) ::
[Cothority](../../README.md) ::
[Building Blocks](../../doc/BuildingBlocks.md) ::
Neff Shuffle

# Neff Shuffle

For an e-voting system, a simple way to proceed is to have everybody encrypt
his vote to a public key and then decrypt all votes after the election is
finished. The big flaw in this scheme is that the holder of the private key can
discover each vote and make a link from the vote to the votee.

To remove the trust assumption of the holder of this private key, we can
shuffle all the votes in a way that the outcome of the shuffle cannot be
correlated to the input, but can be proven to be correct. Now we can decrypt
the votes without being able to trace what each person voted.

The Neff shuffle is faster than for example the Sako-Kilian shuffle and can
shuffle 1'000 voters in 2 seconds on a 2018 hardware laptop.

## Reserach Papers

- [Helios](https://www.usenix.org/event/sec08/tech/full_papers/adida/adida.pdf)
Web-based Open-Audit Voting
- [Sako-Kilian shuffle](https://ng.gnunet.org/sites/default/files/SK.pdf)
Receipt-Free Mix-Type Voting Scheme
- [Neff](http://web.cs.elte.hu/~rfid/p116-neff.pdf) A Verifiable Secret Shuffle
and its Application to E-Voting
- [Improved Neff](http://courses.csail.mit.edu/6.897/spring04/Neff-2004-04-21-ElGamalShuffles.pdf)
Verifiable Mixing (Shuffling) of ElGamal Pairs
