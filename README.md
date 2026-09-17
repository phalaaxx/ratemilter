# ratemilter

A Postfix milter service that rate-limits **outgoing** mail per mailbox, to
contain a compromised account (leaked password, hijacked webmail session,
etc.) that suddenly starts blasting spam through the server's own SMTP
relay. It also exposes a small HTTP API to inspect and manually
block/unblock mailboxes.

## How it works

1. **Connect** — the client's remote address is recorded.
2. **MailFrom** — the message is only tracked further if either:
   - the SMTP session is SASL-authenticated (Postfix passes an
     `{auth_authen}` macro), or
   - the connection comes from `127.0.0.1` **and** the envelope sender is a
     known local mailbox (checked against a CDB database of local
     mailboxes).

   Anything else (regular inbound mail from the outside world) is accepted
   without being tracked — ratemilter only cares about mail *leaving*
   through this server on behalf of one of its own mailboxes.
3. **Headers** (end of header block) — the envelope sender's outgoing
   message count for the last 30 minutes is checked against a hardcoded
   threshold of **200 messages / 30 minutes**. Once a mailbox crosses that
   threshold:
   - it is marked **blocked** in memory,
   - every message from that mailbox seen in the last 30 minutes (by
     Postfix queue ID) that might still be sitting in the queue is put on
     hold via `postsuper -h -` (see "Privilege requirements" below),
   - the current message itself is also quarantined.

   Once blocked, a mailbox stays blocked until explicitly unblocked (via the
   HTTP API — there is currently no automatic unblock/expiry).

State (per-mailbox message timestamps and the blocked flag) lives in memory
and is periodically pruned by a background goroutine (entries older than 30
minutes are dropped, unless the mailbox is blocked — blocked stays sticky).
It is also **persisted to disk** on shutdown and reloaded on startup, so a
service restart doesn't silently forget which mailboxes were blocked or
reset in-flight counters:

- Save path: `/var/lib/ratemilter/ratemilter.json` (created automatically if
  missing).
- Saved on `SIGINT` *and* `SIGTERM` (i.e. both `Ctrl-C` and a normal
  `systemctl stop`/restart trigger a save).

## HTTP API

Bound by default to `:1704` (see `-http` flag). No authentication of its
own — put it behind a reverse proxy with an IP allowlist and/or basic auth
if exposing it beyond localhost (this is how it's wired up on the mail
server: nginx proxies an internal `/stats`-style location to
`http://localhost:1704/`, gated by an IP allowlist and htpasswd).

| Method | Query | Description |
|---|---|---|
| `GET` | *(none)* | JSON dump of all tracked mailboxes: name, blocked state, message count |
| `GET` | `?monitor=true` | Plain text: `OK` if nothing is blocked, otherwise `blocked:mailbox1,mailbox2,...` — meant for simple monitoring checks (Icinga/Nagios-style) |
| `POST` | `?method=block&mailbox=<addr>` | Manually block a mailbox |
| `POST` | `?method=unblock&mailbox=<addr>` | Manually unblock a mailbox |

## Command-line flags

| Flag | Default | Description |
|---|---|---|
| `-proto` | `unix` | Socket family to listen on: `unix` or `tcp` |
| `-addr` | `/var/spool/postfix/milters/rate.sock` | Milter address or unix socket path to bind to |
| `-cdb` | `/etc/postfix/cdb/virtual-mailbox-maps.cdb` | CDB database listing all local mailboxes |
| `-http` | `:1704` | Bind address for the monitoring/management HTTP API |

## Postfix integration

```
smtpd_milters =
  ...,
  unix:milters/rate.sock
non_smtpd_milters = $smtpd_milters
milter_default_action = accept
```

The socket path is relative to Postfix's `queue_directory`, so
`unix:milters/rate.sock` resolves to
`/var/spool/postfix/milters/rate.sock`.

## Privilege requirements

Putting already-queued messages on hold requires running
`postsuper -h -` as root. ratemilter runs as the unprivileged `postfix`
user (same as the other milters) and shells out to it via `sudo`, which
requires a narrowly-scoped sudoers rule — see `sudoers.d_postfix`:

```
postfix ALL = NOPASSWD: /usr/sbin/postsuper -h -
```

Install this as `/etc/sudoers.d/postfix` (or equivalent) — without it,
automatic holding of in-flight messages from a newly-blocked mailbox will
fail (the mailbox is still marked blocked and new messages are still
quarantined; only the "sweep up what's already queued" step is affected).

## Deployment

Runs as a standalone systemd service (see `ratemilter.service`) as the
`postfix` user/group. The service needs write access to
`/var/lib/ratemilter/` (created automatically on first save if it doesn't
exist, as long as the parent directory is writable by `postfix`).

Build with a recent Go toolchain (module-aware, Go ≥ 1.21):

```sh
go build -o ratemilter .
```

Dependencies (see `go.mod`):
- `github.com/phalaaxx/milter` — milter protocol implementation
- `github.com/phalaaxx/cdb` — CDB database reader, used for the local mailbox lookup

## Known limitations

- The rate limit (200 messages / 30 minutes) and the tracking window are
  hardcoded in `mailbox.go`; changing them requires a rebuild.
- There is no automatic unblock/expiry once a mailbox is flagged — it stays
  blocked until someone calls the `unblock` API.
- The HTTP API has no built-in authentication; it must be protected at the
  network/proxy layer if reachable from anywhere other than localhost.
