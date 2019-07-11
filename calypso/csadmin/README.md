# CSADMIN - The Calypso CLI

The `csadmin` command line interface offers a straightforward way to interact
with the Calypso ecosystem. Combined with its brother - `bcadmin` - , it
provides the fastest and simplest way to use Calypso.

## Complete scenario

This scenario shows a complete use path from the creation of the LTS to the
recover of encrypted data.

In this scenario, we are always using the default signer and the default darc,
which are the admin ones. In most of the cases, those can (and should) be
specified with the `--sign` and `--darc` options.

For testing purposes, we recommend running `go build && ./run_nodes.sh -d tmp -v
2` from `cothority/conode` in order to launch a test setup with 3 conodes. Then
a roster can be created with `bcadmin create tmp/public.toml`. The byzcoin id
needed later will be printed.

**1) Authorize nodes and add the Calypso rules**

Each node must be authorized in order to be able to participate in the DKG
(Distributed Key Generation). The following command must be done for each
"private.toml" file of the nodes: 

```bash
$ csadmin authorize <private.toml> <byzcoin id>
```

Then add the rules for the specific Calypso contracts:

```bash
bcadmin darc rule -rule spawn:calypsoWrite --identity <identity>
bcadmin darc rule -rule spawn:calypsoRead --identity <identity>
```

**2) Create an instance of LTS**

Spawn a new instance of the LTS contract:

```bash
$ csadmin contract lts spawn
> Spawned new LTS contract. Its instance id is: 
> <lts instance id>
```

**3) Start a new DKG**

With the instance id of the previously spawned LTS contract, start the new DKG.
X is the collective public key:

```bash
$ csadmin dkg start --instid <lts instance id>
> LTS created:
> - ByzcoinID: <byz id>
> - InstanceID: <inst id>
> - X: <lts public key>
```

It is also possible to directly export the pub key to a file with "-export",
which send the hex string representation to STDOUT.

**4) Spawn a write instance**

With the instance id of the previously spawned LTS contract and the public key,
spawn the write instance:

```bash
$ csadmin contract write spawn --instid <lts instance id> --data "Hello, world." -key <lts public key>
> spawned a new write instance. Its instance id is:
> <write instance id>
```

**5) Spawn a read instance**

With the instance id of previously spawned write instance, request a read on the
encrypted data:

```bash
$ csadmin contract read spawn --instid <write instance id>
> Spawned a new read instance. Its instance id is:
> <read instance id>
```

By default, it uses the public key of the signer to specify what public key
should be used to encrypt the data. But it is possible to provide a different
public key as a hexadecimal string:

```bash
$ csadmin contract read spawn --instid <write instance id> --key <hex pub key>
```

**6) Send a decrypt request**

With the read and write instances, it is now possible to request the encrypted
data. The response, `DecryptKeyReply`, is exported and saved to a file:

```bash
$ csadmin decrypt --writeid <write instance id> --readid <read instance id> -x > reply.bin
```

Note that the data has been re-ecrypted under the public key specified in the
read instance. This is why the final step - recover - is needed.

**7) Recover decrypted secret**

With the re-encrypted data saved previously in the `reply.bin`, the content can
now be recovered. By default, it uses the private key of the signer to decrypt
the re-encrypted data:

```bash
$ csadmin recover < reply.bin
> key decrypted:
> Hello, world.
```

alternatively, it is possible to specify the path of the private key that should
be used to decrypt the re-encrypted data:

```bash
$ csadmin recover --key <private key path> < reply.bin
```