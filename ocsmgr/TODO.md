
# Restricted Access

This scheme has some shortcomings that we will overcome using `ocsmgr`'s
arguments:

- anybody has the right to write documents to the chain
- a writer has to know all public keys of the readers that are allowed
to read the doucment at writing time

## Adding a writer and a reader

Now we need somebody to write and somebody to read the file:

```bash
ocsmngr manage role create writer:alice
ocsmngr manage role create reader:bob
```

This prints out the private keys of the users that you need for accessing the
skipchain from another account. For easier handling, you can store them:

```bash
ALICE=$( ocsmngr manage role list | grep alice | cut -f 2 )
BOB=$( ocsmngr manage role list | grep bob | cut -f 2 )
```

# Access control

There are three access-rights, and every user only has one at any given time:
- admin - can add and remove rights from other users
- write - can add new documents to the chain
- read - can request a re-encryption of the key and fetch the document
