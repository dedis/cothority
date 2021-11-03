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

## Email signup and recovery

In order to support self-signup to the chain and easy recovery using email,
the `EmailSignup` and `EmailRecovery` endpoints can be used.
When using these endpoints, one of the nodes will be the central holder of
information on how to signup and recover accounts.
This is necessary, as secret information needs to be stored, in order to:
- pay the coins to signup a new user
- sign the transaction for recovery

To initialize the system, `EmailSetup` has to be called in order to
register the following information:
- information about the user account that will spawn new users
- information about the SMTP server to send the emails
- the baseURL for signing up a new user

### Setup of the system

1. Create a new email user in the https://login.c4dt.org
   frontend - make sure that there are appropriate recovery options in case
   you loose the private key.
2. Add a new DARC to the email user and call it `EmailDarc` - of course you
   can choose another name.
3. Add a device to the user and call it `Email`.
4. Use the `phapp` to register the new email user to a given node:

```bash
./phapp email setup -bcID $bcID -private co1/private.toml \
    -user_device "$userDevice" -baseURL "$baseURL" \
    -smtp_host dummy:25 -smtp_from root@localhost -smtp_reply_to root@localhost
```

Only one email user can be stored in a node!
If there was an email user stored before, this will overwrite the
information in the node.
Of course the email user will still exist, but recovery will not be possible
anymore!
`smtp_host` needs to point to an SMTP server that allows sending of emails.
The syntax is: `host:port`.

### Email signup

Once an email user has been defined, anybody can request a new account by
sending their email address to the system.
The node will do the following:
1. Abort if a user with that email already exists.
2. Spawn a new user and use the email address in the contact info.
3. Update the `EmailDarc` with the new user.
4. Add the new user to the contacts of the `EmailUser`.
5. Send an email with instructions to the address given.

### Email recovery

To recover an account, an email address can be sent to the node.
The node will do the following:
1. Check whether one of the contacts of the `EmailUser` matches the given email.
2. If such a contact is found, try to recover it (the user might have
   removed the recovery link).
3. If the recovery has been successful, send an email with recovery information.

### Spam protection

In order to protect against spam attacks, a hardcoded limit of 100 signups
is imposed in the system.

### Failures

Currently, there is an undefined behaviour if two different accounts have the
same email address. In the first implementation, it chooses the first user
that is found with the given email and recovers it.
