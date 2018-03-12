# Restricted Access

This scheme has some shortcomings that we will overcome using `ocsmgr`'s
arguments:

- anybody has the right to write documents to the chain
- a writer has to know all public keys of the readers that are allowed
to read the doucment at writing time

# Adding Access Control

In the given implementation anybody with access to the open port of the conode
can request the storage of data. We need to do:

- start with an administrator who can add / remove
    - administrators
    - group-admins who can create and change read and write groups
    - writers
- add write groups with public keys that are allowed to do writes
- add read groups tha the writers can reference instead of a
static list
