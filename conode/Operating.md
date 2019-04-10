Navigation: [DEDIS](https://github.com/dedis/doc/tree/master/README.md) ::
[Cothority](../README.md) ::
[Conode](README.md) ::
Operating a Conode

# Operating a Conode

Here you find some general information about how to run a conode. For command
line examples, please refer to:
- [Command Line](CLI.md) for running a conode in a virtual machine or on a
server
- [Docker](Docker.md) how to run a conode with the pre-compiled Docker image

# WebSocket TLS

Conode-conode communication is automatically secured via TLS when you use
the configuration from `conode setup` unchanged.

However, conode-client communication happens on the next port up from the
conode-conode port, and it defaults to WebSockets inside of HTTP. It is
recommended to arrange for this port to be wrapped in TLS as well.

When this port is using TLS, you must explicitly advertise this fact
when you add your server to a cothority. You do this by setting the
Url field in the toml file:

```
[[servers]]
  Address = "tls://excellent.example.com:7770"
  Url = "https://excellent.example.com:7771"
  Suite = "Ed25519"
  Public = "ad91a87dd89d31e4fc77ee04f1fc684bb6697bcef96720b84422437ff00b79e3"
  Description = "My excellent example server."
  [servers.Services]
    [servers.Services.ByzCoin]
      ...etc...
```

## Reverse proxy

Conode should only be run as a non-root user.

The current recommended way to add HTTPS to the websocket port is to use a web
server like Apache or nginx in reverse proxy mode to forward connections from
port 443 the websocket port, which is the conode's port plus 1.

In this case, the Url in the TOML file would be `https://excellent.example.com`
(no port number because 443 is the default for HTTPS).

## Built-in TLS: specifying certificate files

If you would like the conode to run TLS on the WebSocket interface, you
can tell it where to find the private key and a certificate for that
key:

```
WebSocketTLSCertificate = "/etc/fullchain.pem"
WebSocketTLSCertificateKey = "/etc/privkey.pem"
```

In this case, it is up to you to get get a certificate from a
certificate authority, and to update `fullchain.pem` when
needed in order to renew the certificate.

## Built-in TLS: Using Let's Encrypt certificates

Using the Let's Encrypt CA, and the `certbot` client program,
you can get free certificates for domains which you control.
`certbot` writes the files it creates into `/etc/letsencrypt`.

If the user you use to run the conode has the rights to read from the
directory where Let's Encrypt writes the private key and the current
certificate, you can arrange for conode to share the TLS certificate
used by the server as a whole:

```
WebSocketTLSCertificate = "/etc/letsencrypt/live/conode.example.com/fullchain.pem"
WebSocketTLSCertificateKey = "/etc/letsencrypt/live/conode.example.com/privkey.pem"
```

Let's Encrypt certificates expire every 90 days, so you will need
to restart your conode when the `fullchain.pem` file is refreshed.

# Backups

On Linux, the following files need to be backed up:
1. `$HOME/.config/conode/private.toml`
2. `$HOME/.local/share/conode/$PUBLIC_KEY.db`

The DB file is a [BoltDB](https://github.com/etcd-io/bbolt) file, and more
information about considerations while backing them up is in [Database
backup](https://github.com/dedis/onet/tree/master/Database-backup-and-recovery.md).

## Recovery from a crash

If you have a backup of the private.toml file and a recent backup of the .db
file, you can put them onto a new server, and start the conode. The IP address
in the private.toml file must match the IP address on the server.

# Roster IPs should be movable

In order to facilitate IP address switches, it is recommended that the public IP
address for the leader of critical skipchains should be a virtual address. For
example, if you have two servers:
* 10.0.0.2 conode-live, also with secondary address 10.0.0.1
* 10.0.0.3 conode-standby

You can keep both servers running, and use scp to move the DB file from
conode-live to conode-standby. Both servers should have the same private.toml
file, which includes the line `Address = "tcp://10.0.0.1:7770"`

In the event that conode-live is down and unrecoverable, you can add 10.0.0.1 as
a secondary address to conode-standby and start the conode on it. From this
moment on, you must be sure that conode-live does not boot, or if it does, that
it *does not* have the secondary address on it anymore. You could do so by not
adding the secondary address to boot-time configs, and only move it manually.

The address 10.0.0.1 will be in the Roster of any skipchains, and nodes which
are following that skipchain will still be able to contact the leader, even if
it is now running on a different underlying server.

Note: The address part of a server identity has name resolution applied to it.
Thus it would be possible to set the roster of a skipchain to include a server
identity like "tcp://conode-master.example.com:6979" and then change the
definition of conode-master.example.com in DNS in order to change the IP address
of the master.
