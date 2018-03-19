Navigation: [DEDIS](https://github.com/dedis/doc/tree/master/README.md) ::
[Cothority](../README.md) ::
[Apps](../doc/Apps.md) ::
Proof of Personhood

# Proof of Personhood

Different services on the internet would like to offer anonymity to users, but
still require the ability to block users that misbehave or to reject computer
generated logins. The current state of the art uses Captchas to verify if a
connection is made by a user or by an automatic process. Unfortunately it is
very difficult today to create Captchas that are easy for humans and difficult
for machines.

Also the Tor-network has this problem, that due to the anonymity it provides,
some users can abuse the network to attack websites by impersonating a multitude
of users, while in reality they are only one entity. So a lot of websites
present a Captcha when visited through the Tor-network, to prevent some kind of
spamming attacks.

Our solution to this problem is Proof of Personhood, where we link a physical
person to a cryptographic token in such a way that one physical person can only
receive one token. Furthermore this token can be used to authenticate as
anonymous member of the group with a tag that is specific to that user/service
combination. This allows for example Wikipedia to authenticate a user and eventually
block him, while making it impossible for example that Wikipedia and an evoting
service collude to find out what that user is voting on an anonymous voting
platform.

Work done by Maria Fernanda Borge Chavez at the DEDIS-lab at EPFL worked out how
we can use cryptography to create a token that can prove a person knows a
secret, without giving away which person of a group it is. Furthermore, if we
have different services like Wikipedia, Tor, or even voting systems, it is
possible that each service gets its own view of the group and that they cannot
collude to learn more about connections between the user.

## Links

This is a list of available documents regarding Proof of Personhood as it evolved
over time, papers, reports, and presentations.

- [Client API](service/README.md)
- Student project: [Adding DAGA to PoP](https://github.com/dedis/student_17/daga_pop):
  - [Report](https://docs.google.com/viewer?url=https://github.com/dedis/student_17/raw/master/pop_ethereum/report_pop_ethereum.pdf)
  - [Presentation](https://docs.google.com/viewer?url=https://github.com/dedis/student_17/raw/master/pop_ethereum/presentation_pop_ethereum.pdf)
- Student project: [Proof of Personhood on Ethereum](https://github.com/dedis/student_17/pop_ethereum):
  - [Report](https://docs.google.com/viewer?url=https://github.com/dedis/student_17/raw/master/pop_ethereum/report_pop_ethereum.pdf)
  - [Presentation](https://docs.google.com/viewer?url=https://github.com/dedis/student_17/raw/master/pop_ethereum/presentation_pop_ethereum.pdf)
- Student project: [Cross Platform Mobile App](https://github.com/dedis/student_17/cpmac):
- Paper: Proof-of-Personhood: Redemocratizing Permissionless Cryptocurrencies: https://zerobyte.io/publications/2017-BKJGGF-pop.pdf
- Slides: PoP talk at 33c3: https://pop.dedis.ch/documents/slides_pop_33c3.pdf
- Paper: Pseudonym Parties: An Offline Foundation for Online ... - Bryan Ford: http://ww.bford.info/log/2007/0327-PseudonymParties.pdf
