# Evoting admin tool

This is the evoting master chain admin tool. It can be used to make a new master
chain:

```
$ ./evoting-admin -admins 0,1,2,3 -pin bf6d681a9e84e0046414b67d1bb3e6e4 -roster ../../conode/public.toml
I : (                               main.main:  83) - Master ID: [57 223 155 178 205 105 248 71 28 42 23 91 215 233 71 232 62 49 96 99 38 241 28 211 170 55 123 60 57 30 225 220]
I : (                               main.main:  86) - Master ID in hex: 39df9bb2cd69f8471c2a175bd7e947e83e31606326f11cd3aa377b3c391ee1dc
```

Update an existing master chain. Give it the same arguments as create, but also include
the `-id` argument to tell it which one to update.

The -user and -sig args are required. The -user is the SCIPER of an admin from the previous block. The -sig is a signature generated
by the authentication server's "/auth/login/txt" endpoint.

```
$ ./evoting-admin -admins 0,1,2,3,4,5,6 -pin bf6d681a9e84e0046414b67d1bb3e6e4 -roster ../../conode/public.toml -key 0d75f6903e7fbcb5e8623c942f707e4d36fbfbfdefdd7ae8b50633d0ed86a3a2 -id 39df9bb2cd69f8471c2a175bd7e947e83e31606326f11cd3aa377b3c391ee1dc -user 12345 -sig e169539b37bb4f7ed61e3d53c7444342e68f72bbe62a180a78efa3adabaa15d96ae8b8de280a7ed0d3974b2eabf011779b1ba4abb2ce35f89dbd0f728798a104
I : (                               main.main:  83) - Master ID: [57 223 155 178 205 105 248 71 28 42 23 91 215 233 71 232 62 49 96 99 38 241 28 211 170 55 123 60 57 30 225 220]
I : (                               main.main:  86) - Master ID in hex: 39df9bb2cd69f8471c2a175bd7e947e83e31606326f11cd3aa377b3c391ee1dc
```

See the current status of the master chain:

```
$ ./evoting-admin -show -roster ../../conode/public.toml -id 39df9bb2cd69f8471c2a175bd7e947e83e31606326f11cd3aa377b3c391ee1dc 
 Admins: [0 1 2 3 4 5 6]
 Roster: [tls://localhost:7002 tls://localhost:7004 tls://localhost:7006]
    Key: 0d75f6903e7fbcb5e8623c942f707e4d36fbfbfdefdd7ae8b50633d0ed86a3a2
```

Note that -show requires both `-id` and `-roster` arguments.

# Dumping votes

- `extract` was used to get the DB files from the backup files Marc-Andre gave us.
- `dump` dumps one election at a time
- `dumpa-all` dumps all the elections that were on the master chain for the production election.