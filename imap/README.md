# IMAP integration

## Overview

**IMAP (Internet Message Access Protocol)** is a standard email protocol used by mail clients to retrieve messages from a mail server over a TCP/IP connection.
It is widely adopted for managing and accessing email stored on remote servers.

This integration allows:

If a pre-built package exists for your system and architecture,
you can simply install it using:

```sh
$ plakar pkg add imap
```

Otherwise,
you can first build it:

```sh
$ plakar pkg build imap
```

This should produce `imap-vX.Y.Z.ptar` that can be installed with:

```bash
$ plakar pkg add ./imap-v0.1.0.ptar
```

## Configuration

The configuration parameters are as follow:

- `location`: The URL of the IMAP server in the form imap://<host>:<port>.
- `username`: Username to login.
- `password`: Password for login.
- `tls`:      TlS mode to use.  Possible values are tls (the default), starttls and no-tls.
- `tls_no_verify`: If set to yes, the client will not verify the server certificate in tls mode.

## Examples

```bash
# configure an IMAP source connector
$ plakar source add myIMAPsrc imap://imap.mydomain.com:143 \
    username=myuser     \
    password=mypassword \
    tls=starttls

# backup the mailbox
$ plakar backup @myIMAPsrc

# configure an IMAP destination connector
$ plakar destination add myIMAPdst imap://imap.alsomydomain.com:143 username=alsomyuser password=alsomypassword tls=starttls

# restore the snapshot to the destination
$ plakar restore -to @myIMAPdst <snapid>
```
