# Evoting admin tool

This is the evoting master chain admin tool. It can be used to make a new master
chain:

```
$ ./evoting-admin -admins 0,1,2,3 -pin bf6d681a9e84e0046414b67d1bb3e6e4 -roster ../../conode/public.toml
I : (                               main.main:  83) - Master ID: [57 223 155 178 205 105 248 71 28 42 23 91 215 233 71 232 62 49 96 99 38 241 28 211 170 55 123 60 57 30 225 220]
I : (                               main.main:  86) - Master ID in hex: 39df9bb2cd69f8471c2a175bd7e947e83e31606326f11cd3aa377b3c391ee1dc
```

There is support in the tool (and the evoting service) for updating a master
chain config, but it is not well tested. In practice, we use one single master
chain config per election season, and we do not modify it during the production
election.

See the current status of the master chain:

```
$ ./evoting-admin -show -roster ../../conode/public.toml -id 39df9bb2cd69f8471c2a175bd7e947e83e31606326f11cd3aa377b3c391ee1dc 
 Admins: [0 1 2 3 4 5 6]
 Roster: [tls://localhost:7002 tls://localhost:7004 tls://localhost:7006]
    Key: 0d75f6903e7fbcb5e8623c942f707e4d36fbfbfdefdd7ae8b50633d0ed86a3a2
```

Note that -show requires both `-id` and `-roster` arguments.