Navigation: [DEDIS](https://github.com/dedis/doc/tree/master/README.md) ::
[Cothority](../doc/README.md) ::
[Apps](../doc/Apps.md) ::
[Proof of Personhood](README.md) ::
Proof of Personhood CLI

# Proof of Personhood CLI

The pop CLI can be used by the following group of users:

- organizers (org)
- attendees (attendee)

Each group has its own set of commands to start, finalize and use the keys.

## Organizers

An organizer has to run his own conode to be able to create and
participate as an organizer in a party. For setting up a conode,
see [../conode/README.md].

Once the conode is setup, he needs to _link_ to the conode, then
discuss what configuration to use together with the other organizers
and _store_ this configuration.

During the party, each organizer has to scan every attendee's public
key and he has to make sure that the attendee leaves the room once he's
been scanned by all organizers. When the organizer scanned all
public keys, he can _register_ the keys. Once all organizers
registered the keys, they can _finalize_ the party and create a
*Final Statement*.

### Link

First every organizer has to link to his conode:

```bash
pop org link 127.0.0.1:2002
# Read pin from command line
pop org link 127.0.0.1:2002 PIN
```

### Store the configuration

The organizers have to decide on the name of the party, the time
and the location. Once they agree on this information, they can
create a `description.toml`-file by adding all their conode's
`public.toml`-files at the end of the file - example:

```toml
Name = "Proof-of-Personhood Party"
DateTime = "2017-08-08 15:00 UTC"
Location = "Earth, City"
[[servers]]
  Address = "tcp://127.0.0.1:2002"
  Public = "lWHQZiMkxchMd7hfpn18HLqJUqiGRpIzSTyFYzWD9Zc="
  Description = "Cot-1"
[[servers]]
  Address = "tcp://127.0.0.1:2004"
  Public = "hbCeQID3RhQ0tVI8G/XkmyJv+Xr8qosVpCyJUppZcPw="
  Description = "Cot-2"
```

This file has to be an exact copy for each organizer, as the conodes
will calculate a hash on that file and use it to reference the
party afterwards.

```toml
pop org config description.toml
```

This will print out the hash of the description, which should be the same
for all organizers. If it differs, the conodes will consider them as
being part of a different party and they will not be able to create a valid
final statement.

### Register attendees public keys

The attendees create public/private key pairs, store the private key
and send the public key to the organizers. Each organizer should have
all keys from all attendees.

Supposing three attendees were at the party, then to register all public keys,
each organizer has to do:

```bash
pop org public "[key1,key2,key3]" DESCRIPTION_HASH
```

### Finalize the party

Now that all keys are stored by all organizers, each organizer can start
to finalize the party and create the _collective signature_ on the
final.toml:

```bash
pop org final DESCRIPTION_HASH
```

This will output the final statement, if successful. Store this final
statement to give to the attendees. This can be saved as `final.toml`

## Attendees

### Create public/private key pair

Each attendee has to create his key-pair. He is responsible to store
this keypair securely:

```bash
pop -d 2 attendee create
```

### Send public key to organizers

By any means, email or qrcode, these public keys should now be sent to
all the organizers.

### Create pop-token

The pop-token can be created by running the following command:

```bash
pop attendee join PRIVATE_KEY `final.toml`
```

### Sign a message

An attendee with a valid pop-token can sign a message it receives from
a service. The service will send a `message` and a `context`. The message
is a nonce, while the context will be the same and represent the service.

```bash
pop attendee sign MESSAGE CONTEXT DESCRIPTION_HASH
```

This command will output the signature and the tag that has to be
sent back to the service.

## Authentication Service

A service will want to verify signatures from attendees. For this, he
will have to store the final statement locally to be able to verify
if a signature is valid or not. Storing this final statement can be
done in this way:

```bash
pop auth store `final.toml`
```

### Verify a message

Once the final statement has been stored, the service can verify if
a signature/tag pair is valid:

```bash
pop attendee verify MESSAGE CONTEXT SIGNATURE TAG DESCRIPTION_HASH
```

It will return whether the signature is valid or not.
