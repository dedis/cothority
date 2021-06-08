Navigation: [DEDIS](https://github.com/dedis/doc/tree/master/README.md) ::
[Cothority](https://github.com/dedis/cothority/tree/main/README.md) ::
Personhood.Online

# Personhood.Online

Personhood.online is a very simple implementation of the three basic services:

- Currency - being able to send and receive coins
- Voting and Deliberation - proposing a questionnaire
- Information - Discussion forum

Each of the three is based upon the premise that we want to allow formation
of scalable self-organizing communities.

THIS IS REALLY INSECURE AND ONLY TO BE USED AS A MOCKUP! PLEASE DON'T OPEN
ANY ISSUES ABOUT AUTHENTICATION PROBLEMS OR ANY OTHER STUFF! ONCE WE HAVE A
WORKING MOCKUP, WE'LL DO PROPER CONTRACTS AND ALL!

## Currency

Either use this service or the current coin-services implemented in ByzCoin.
Probably the latter.

## Voting and Deliberation

Start with a very simple twitter-like questionnaire that participants can fill
out. Each questionnaire comes with a number of coins attached, and every
participant receives a number of coins upon filling out the questionnaire.
Participants can also reload a questionnaire if it is empty.

## Information

A very simple twitter machine with the following possibilities:
- send a message by attaching coins and a reward per read
- see a list of messages, ordered by most valuable to read
- recharge a message so it is read by more people (also gives some coins
  back to the writer)
