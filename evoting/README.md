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

<p align="center">
  <img src="arch.png" width="400" height="325" />
</p>

## Links
- Student Project: EPFL e-voting:
  - [Backend](https://github.com/dedis/student_17/evoting-backend)
  - [Frontend](https://github.com/dedis/student_17/evoting-frontend)
- Paper: **Verifiable Mixing (Shuffling) of ElGamal Pairs**; *C. Andrew Neff*, 2004
- Paper: **Helios: Web-based Open-Audit Voting**; *Ben Adida*, 2008
- Paper: **Decentralizing authorities into scalable strongest-link cothorities**: *Ford et. al.*, 2015
- Paper: **Secure distributed key generation for discrete-log based cryptosystems**; *Gennaro et. al.*, 1999
