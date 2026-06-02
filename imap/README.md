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

- `location`: The URL of the IMAP server in the form `imap://<host>[:<port>]`. When the port is omitted it defaults to 993 for `tls` and 143 otherwise. Credentials may also be embedded as `imap://user:password@host`.
- `username`: Username to login.
- `password`: Password for login.
- `tls`:      TLS mode to use. Possible values are `starttls` (the default), `tls` and `no-tls`.
- `tls_no_verify`: If set to `true`, the client will not verify the server certificate (dangerous; testing only).

## Behavior

- **Folder hierarchy** is preserved across servers even when source and
  destination use different hierarchy delimiters (e.g. Dovecot's `.` vs `/`).
  Each mailbox segment is percent-encoded into the snapshot path, so a folder
  named `Work/Notes` or `Reçus` round-trips intact.
- **Message flags** (`\Seen`, `\Answered`, `\Flagged`, `\Draft`, `\Deleted`,
  and keywords such as `$Junk`) are encoded into each message's file name and
  re-applied on restore. The session-only `\Recent` flag is intentionally
  dropped. Messages are fetched with `BODY.PEEK`, so a backup never alters the
  source mailbox.
- On restore, missing mailboxes (and their parents) are created automatically.

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
