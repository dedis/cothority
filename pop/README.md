# Proof of Personhood

Online anonymity often appears to undermine accountability, offering little incentive for civil behavior, but accountability failures usually result not from anonymity itself but from the disposability of virtual identities. A user banned for misbehavior—e.g., spamming from a free E-mail account or stuffing an online ballot box—can simply open other accounts or connect from other IP addresses. This applies to all kind of services like Wikipedia for editing, Tor-service for connecting to websites, voting-systems and even crypto-currencies using blockchains can be made more distributed by using pseudonym tokens.

Several solutions have been designed and tried, however none of them is perfect and their costs are considerable. For example, permissionless blockchain systems use proof-of-work and proof-of-stake mechanisms to allow dynamic membership and to prevent double spending attacks; on the other hand the disadvantages of these approaches are well known: high amounts of wasted energy and resources, and, at least for bitcoin, centralization of the currency. Online services, for example, Facebook and Quora, adopted a Real-name policy, refusing users who want to use pseudonyms.

A different approach to solve the anonymity problem are Pseudonym parties. It’s goal is to link a physical user to an electronic identity. A first project on this topic has been started with http://proofofindividuality.tk/ where the proof is done using a video-conference between different participants. However, the system didn’t show yet that it resists against sybil-attacks by being present in multiple conferences at the same time, or even spoof the system using computer-generated avatars. 

## PoP-party

The goal of the Proof-of-Personhood party (pop-party) is to give one proof-of-personhood token (pop-token) to one attendee of the party. The idea is that a group of people will organize a party, in a particular place at a specific time, open to anyone. Attendees are welcome to come dressed as they want, for example with masks, if they feel that this will protect their anonymity. 
All attendees have to have their pop-token registered with one of the organisers who will publish a list of all tokens at the end of the party. Using this pop-token, every attendee can identify himself to a service-provider as being part of the group that was present at the party. Because it is pseudonymous and not anonymous, misbehaving attendees can be banned from a service.
To avoid de-anonymisation by colluding services, we use anonymous group signatures, so an pop-token will get a different pseudonym for every service used. If a pop-token generated a pseudonym used for an edit in Wikipedia, then it cannot be linked to the pseudonym generated for use in a discussion service.

# How to use it

For creating a PoP-Party, you need the following ingredients:

- organisers - each organiser needs a server where he can run his conode, and
a computer to access the conode
- party-room - where you can gather people, explain the happening, and let them
get out of the room one by one
- paper-keys - simple keys that you can print out and give to the attendees
- attendees - people who want to get a pop-token

There are five steps in using the pop-app:

1. Set up a cothority
2. Create a pop-party configuration
3. Add public keys to the pop-party
4. Create a party-transcript
5. Identify yourself against a service using your pop-token

## Setting up a cothority

### Creating nodes

The setting up of a cothority is described in the wiki at
https://github.com/dedis/cothority/wiki/Conode . For pop you cannot use our
cothority, as you need a trusted access to a conode.

### Compilation of pop

You can compile and install pop with the following command:

```go
go get https://github.com/dedis/cothority/pop
```

### Linking to your node

Each computer of an organiser needs first to link with his conode:

```go
pop org link addr_conode
```

If your conode is running on the server, you will see a message printed on the
standard output, like:
 
```log
I : (           service.(*Service).PinRequest:  65) - PIN: 464843
```

Of course your PIN will differ. Now you can link your conode and your computer:

```go
pop org link addr_conode PIN
```

`pop` should reply with `Successfully linked...`.

### Creating group-file

You also need all conodes' definition united in a file called `group.toml`. You'll
find the conodes' definition in the `~/.config/conode/public.toml`-file on the
server. Merge all these files, so that they look something like (this is the
dedis-cothority, which will _not_ work for you):

```toml
[[servers]]
  Address = "tcp://78.46.227.60:7770"
  Public = "5eI+WFOaCdMhHY+g+zR11IZV4MBtg+k8jm59FqqHwQY="
  Description = "Ineiti's Cothority-server"

[[servers]]
  Address = "tcp://192.33.210.8:7770"
  Public = "A2vzFuHqbn6Z4LtxNBnRbAtnlL+dxELMTPNsP5Nek88="
  Description = "EPFL Cothority-server"
```

## Creating a pop-party configuration

Now you are ready to create a common configuration for all organisators, call
it `pop_desc.toml`. An example is:

```toml
Name = "33c3 Proof-of-Personhood Party"
DateTime = "2016-12-29 15:00 UTC"
Location = "Earth, Germany, Hamburg, Hall A1"
```

### Store the configuration with all organisers

Every organiser has to take that file and store it as his configuration. For
the moment, only one pop-party can be stored:

```go
go org config pop_desc.toml group.toml
```

## Add public keys to the pop-party

Before you can add public keys to the pop-party, you have to create them, print
them out and distribute them to the attendees. Of course they might not trust
you (which is a good sign), so prepare to explain to them how they can generate
their own keys.

### Print paper-keys

The following script uses libreoffice to create the paper-keys. As it's a bit
hacky, you have to make sure that libreoffice itself is closed before you start
the script:

```bash
cd $GOPATH/src/github.com/dedis/cothority/pop
./paperkey.sh nbr_keys
```

This will create `nbr_keys` random private/public key pairs and store them as
PDFs. After having them printed out, you should delete the PDFs.

### Scan keys

Once the party comes to the signing-part, you put all the attendees in a row
and let them pass before each organiser. Each organiser has to scan the 
attendees' public keys using the pop-android-app (sorry for iPhone users):

https://github.com/dedis/Android-CISC/tree/pop-app

After launching the app, you have three buttons, from left to right:

- scan - scans a new public key
- send - sends the list of public keys via email
- clean - cleans all scanned public keys

### Send and compare the keys

Once all attendees have been scanned by an organiser, he can send the keys to
his email to retrieve them. Now all organisers have to manually check that all
have the same keys. Best of all is to have a video-transcript of the scanning,
as well as of the comparing and merging of the keys, so that full transparency
can be given.

## Create a party-transcript

Now every organiser can create the transcript. First you need to send all
public keys to your conode, create and wait for all the other organisers to do the
same, and finally you can retrieve the transcript.

### Sending list of keys

Copy the `list_of_keys` from your email, and send it to your conode:

```go
pop org public list_of_keys
```

### Create transcript

Now you can launch the transcript-creation:

```go
pop org final
```

All but the last organisers will receive an error: `Wrong pop-status: 2`. This
means that the others didn't confirm the list of public keys yet.

### Retrieve transcript

For all but the last organiser, you'll need to run the same command again, in
order to receive the transcript. Or you can simply copy the transcript of the 
last organiser to your computer.

```go
pop org final
```

## Use the pop-token

A pop-token is composed of the final statement created in the previous step,
and the private key of an attendee. An attendee can sign a message using the 
pop-token, and a service can verify the signature to be valid. The following
is guaranteed:

- for a given service, the same attendee will create the same `tag`, even
for different messages
- for two different services, the `tag` of the attendees are not comparable
in any manner
- one or more services cannot collude by using the `tag`s or signatures to
match a `tag` from one service to a `tag` from another service.

### Attendees 

An attendee wants to identify himself against a service using a pop-token.

#### Creation of pop-token

First an attendee needs to create his pop-token. For this he needs the
`final.toml` of the organisers, and his `private_key`:

```go
pop client join final.toml private_key
```

Now the pop-token is stored. For the moment only one pop-token can be used
at the same time.

#### Signing of a message

Usually the service will send a `message` and a `context` to the attendee. The
message can be anything, while the `context` needs to be the same for a given
service.

```go
pop client sign message context
```

This `signature` and `tag` can then be sent to the service.

### Service

The service can take the `signature`, the `tag`, together with the `message`
and `context` and verify its validity:

```go
pop client verify message context tag signature
```

If the tag and signature match the message and the context, and if they
come from the final.toml, the `pop`-binary returns success, else an 
error.