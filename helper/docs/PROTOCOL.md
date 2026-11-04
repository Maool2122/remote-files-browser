# The messages (protocol 1)

This is everything the extension and the helper say to each other. The code that reads and writes it is `protocol.go` (framing) and `server.go` (the operations); the matching client is in the extension (closed source), and a small open one is the test client in `harness_test.go`.

Protocol **1** is used by every helper version so far (`1.0.0` and `1.1.0`). The number only changes when an old extension and a new helper (or the other way round) can no longer understand each other ([Versions](#versions)).

## Transport

Chrome's native messaging. Every message, in both directions, is:

```
4 bytes   the length of the JSON text in bytes, an unsigned little-endian number
n bytes   the JSON text (UTF-8)
```

- The extension can send up to 32 MiB per message (`maxIncoming`). The helper reads a bigger one as an error and stops.
- The helper sends less than 1 MiB per message (`maxOutgoing`, a little under 1 MiB), because Chrome refuses more from a native program. A piece of a download is 192 KiB of data, which is 256 KiB as base64 text.
- The helper reads standard input until Chrome closes it, then cancels everything that is running, closes the connections to the servers and exits. It writes nothing to standard output except messages, so a log line must never be printed there (it would break the framing). Anything for a human goes to standard error.

## Requests, answers and events

A **request** is a JSON object with a number `id` that the extension chooses (unique among the requests that are still running) and an `op`:

```json
{"id": 7, "op": "list", "session": "s1", "path": "/docs/"}
```

Every request gets exactly **one answer** with the same `id`:

```json
{"id": 7, "ok": true}
{"id": 7, "ok": false, "error": {"code": "generic", "message": "The file or folder was not found."}}
```

A successful answer carries the result of the operation next to `id` and `ok`. A failed one carries an `error` ([Errors](#errors)).

A long request also sends **events** before its answer. They carry the `id` of the request and a name:

```json
{"id": 7, "event": "entries", "entries": [{"n": "a.txt", "d": false, "s": 12, "t": 1760000000000}]}
```

Four messages are **not requests**: they belong to a request that is already running, they name it with `target` (its `id`), and they get no answer: `credit`, `chunk`, `end` and `cancel`. If the request has finished meanwhile, they are ignored. A message that is not valid JSON is ignored. A request with an `op` the helper does not know gets a `generic` error that asks to update the helper. **Fields the helper does not read are ignored**, so a newer extension can send more than an older helper understands (this is why adding a field never needs a new protocol number).

## The operations

All paths are absolute and use `/`, on every kind of server. `session` is a name the extension chooses in `open`.

### `hello`

Always the first message. No other fields.

Answer: `{"version": "1.1.0", "protocol": 1, "os": "windows", "arch": "amd64"}`.

The extension compares `protocol` with its own number. A helper with a lower number is "too old" (the extension asks the person for the newest helper); a higher number means "update the extension".

### `open`

Opens a connection to a server and keeps it under the name `session`. If a session with that name exists it is closed first.

| Field | Meaning |
| --- | --- |
| `session` | The name of the connection. |
| `kind` | `ftp`, `ftps` or `sftp`. |
| `host`, `port` | The server. `port` `0` means the usual one: 21 for FTP, 990 for implicit TLS, 22 for SFTP. |
| `user`, `password` | The login. For FTP an empty `user` logs in as `anonymous`. For SFTP the user is required. With a key file, `password` is the passphrase of the key. |
| `keyPath` | SFTP only: the path of a private key file (a leading `~` is the home folder). The helper reads it; the extension never does. Empty: log in with the password. |
| `tls` | FTP and FTPS: `implicit` (TLS from the first byte), `explicit` (the server must accept `AUTH TLS`), `none` (plain), or `auto` (try explicit TLS, go on without it if the server does not know it). Empty: `auto` for `ftp`, `implicit` for `ftps`. |
| `noEpsv` | FTP: do not use the extended passive command (for old servers and strict firewalls). |
| `trusted` | A list of SHA-256 fingerprints (lower case hex) that the person has trusted. See [Trust](#trust). |

Answer: `{"secure": true, "note": "Encrypted (TLS)", "home": "/"}`. `secure` says whether the link is encrypted; `note` is a short sentence for the toolbar; `home` is the folder the person lands in after login.

### `close`

`{"op": "close", "session": "s1"}` closes the connection. Answer: `{}`.

### `list`

`session`, `path`. The listing arrives in **batches** of up to 500 names as events, so a big folder starts to show quickly:

```json
{"id": 7, "event": "entries", "entries": [ {"n": "a.txt", "d": false, "s": 12, "t": 1760000000000, "m": "rw-r--r--"}, ... ]}
```

| Key | Meaning |
| --- | --- |
| `n` | The name. |
| `d` | `true` for a folder. |
| `s` | The size in bytes, `-1` when the server does not say. |
| `t` | The time of last change, in milliseconds since 1970; `0` when the server does not say. |
| `m` | The permissions as `rwxr-xr-x`, only when the server says. |
| `l` | `true` for a link, only then. |

Then the answer `{}`.

### `stat`

`session`, `path`. Answer: `{"exists": true, "dir": false, "size": 12}`. `size` is `-1` when the server does not say (a folder on some FTP servers). A path that is not there is **not an error**: the answer is `{"exists": false, "dir": false, "size": 0}`.

### `read` (download)

`session`, `path`, `offset` (where to start, `0` for the whole file), `credit` (how many pieces the helper may send at first; `8` when it is `0`).

Events and answer:

1. `meta` with `{"size": 1234, "resumed": true}`. `size` is the size of the whole file, or `-1` when the server does not say. `resumed` is `false` when the server could not start in the middle: the helper then reads and throws the first `offset` bytes away itself. **The data you receive always starts at `offset`.**
2. `data` with `{"b64": "..."}`, a piece of the file (up to 192 KiB before base64), as many as there is credit for.
3. The answer `{"bytes": 1234}`: how many bytes were sent.

**Flow control.** The helper sends a `data` piece only when it has credit for it. To let it go on, the extension sends `{"op": "credit", "target": 7, "n": 4}` (`n` pieces, at least 1). A slow page therefore never makes the helper pile up data in memory.

If the connection to the server breaks in the middle, or the server ends the transfer with an error, the answer is an error instead of `bytes`: the file may be cut short and the extension must not treat it as complete.

### `write` (upload)

`session`, `path`. The file is written at `path`; an existing file is replaced.

1. The helper opens the destination and sends the event `ready`.
2. The extension sends pieces: `{"op": "chunk", "target": 7, "b64": "..."}`. After writing each one the helper sends the event `ack` with `{"n": 1, "bytes": <size of the piece>}`.
3. The extension finishes with `{"op": "end", "target": 7}`. The helper closes the file and sends the answer `{"bytes": <total>}`. If the server reports a problem when the file is closed, the answer is an error.

On `cancel`, or when anything fails, the helper throws the upload away and removes the part that was already written.

### `mkdir`, `remove`, `rename`

| `op` | Fields |
| --- | --- |
| `mkdir` | `session`, `path` (the missing folders above it are made too, and a folder that already exists is not an error) |
| `remove` | `session`, `path`, `dir` (`true` for a folder; a folder is removed with everything inside; on SFTP a link is removed, never followed) |
| `rename` | `session`, `from`, `to` |

Each answers `{}`.

### `cancel`

`{"op": "cancel", "target": 7}` stops the request with that `id`. Whatever it was waiting for on the network is closed, so a cancel is never stuck behind a slow server. The cancelled request answers with the error code `cancelled`.

### `sites` (helper 1.1.0 and newer)

`{"op": "sites"}` asks for the servers that the person saved in other programs, so that the extension can offer them as new connections. The extension sends it only when the person presses **Look for saved servers** in its import window. There are no other fields and no `session` is needed.

The helper looks, for the person who runs it and nobody else, in the places where these programs keep their lists:

| Program | Where | On |
| --- | --- | --- |
| FileZilla | `sitemanager.xml` in the FileZilla folder: `%APPDATA%\FileZilla\` on Windows; `~/.config/filezilla/`, `~/.filezilla/` and, on macOS, `~/Library/Application Support/FileZilla/` on the others | every system |
| WinSCP | the file `%APPDATA%\WinSCP.ini`, and the saved sessions in the registry of the current user (`HKEY_CURRENT_USER\Software\Martin Prikryl\WinSCP 2\Sessions`) | Windows |
| Cyberduck | the `.duck` bookmark files in the `Bookmarks` folder of Cyberduck (`%APPDATA%\Cyberduck\Bookmarks`, `~/Library/Application Support/Cyberduck/Bookmarks`) | Windows and macOS |

Answer: `{"sources": [...]}`, one entry for every program that exists on this kind of computer, found or not:

```json
{"sources": [
  {"id": "filezilla", "name": "FileZilla", "found": true, "skipped": 1, "sites": [
    {"name": "Studio website", "kind": "sftp", "host": "studio.example.net", "port": 2222, "user": "anna", "path": "/var/www/html", "group": "Clients"},
    {"name": "Shop uploads", "kind": "ftp", "tls": "explicit", "host": "shop.example.net", "user": "anna", "path": "/uploads", "group": "Clients"}
  ]},
  {"id": "cyberduck", "name": "Cyberduck", "found": false, "skipped": 0, "sites": []}
]}
```

| Key of a program | Meaning |
| --- | --- |
| `id`, `name` | The program: `filezilla`, `winscp` or `cyberduck`, and its name for a person. |
| `found` | `true` when the program's list was there. |
| `sites` | The servers the extension can open: FTP, FTPS and SFTP only. |
| `skipped` | How many entries were left out because they are another kind (WebDAV, cloud storage, SCP, ...) or have no server. |
| `truncated` | Only when there were more than 500 servers; the first 500 are sent. |
| `error` | Only when the list is there but could not be read: a plain sentence, without a file name. |

| Key of a server | Meaning |
| --- | --- |
| `name` | What the person called it. The host name when it has no name. |
| `kind` | `ftp`, `ftps` (TLS from the first byte) or `sftp`. |
| `tls` | FTP and FTPS only: `auto`, `explicit`, `implicit` or `none`, the same words `open` takes. Left out when the program does not say. |
| `host`, `port` | The server. `port` is left out when the program uses the usual one. |
| `user` | The user name, when the program has one. |
| `path` | The start folder, when the program has one. |
| `group` | The folder in which the program keeps the entry, for display only. |

Every text is cut to 200 characters. A list file larger than 16 MiB is not read, and at most 1,000 Cyberduck bookmark files are read.

**What is never in the answer, and never read:** a password (in any form: plain, encoded or hashed), the path or name of a key file, a passphrase. The code that reads the lists has no field for them, so they are thrown away while the file is read; the Go tests (`sites_test.go`) check that none of them appears in an answer. The helper never writes to the files of the other programs, and an answer goes only to the extension that asked.

An older helper does not know `sites` and answers the generic error "The helper does not know the request ... Update the helper." The extension checks `version` first (1.1.0 or newer) and shows its "update the helper" screen instead.

## Errors

```json
{"id": 7, "ok": false, "error": {"code": "auth", "message": "The server did not accept the user name or password."}}
```

`message` is a plain sentence that can be shown to the person as it is. `code` says what the extension should do:

| `code` | Meaning | What the extension does |
| --- | --- | --- |
| `auth` | The login was refused (or a password or key is missing). | Asks for the password again. |
| `trust` | The identity of the server is new or changed. Carries `fingerprint`, `fpKind`, optionally `reason` and `display` ([Trust](#trust)). | Asks the person. |
| `cancelled` | The request was cancelled. | Nothing. |
| `session` | The connection to the server was lost or closed, or the `session` named in the request is not open. | Opens it again. |
| `generic` | Anything else: no such file, the server says no, no room, name not found, connection refused, timeout... | Shows the message. |

## Trust

A server proves who it is with a TLS certificate (FTPS, and FTP with TLS) or an SSH host key (SFTP).

- A certificate that the computer's own list of authorities accepts for the host name that was typed passes.
- Any other certificate or host key passes only if its SHA-256 fingerprint is in `trusted`.
- Otherwise the helper stops with a `trust` error:

  | Key | Meaning |
  | --- | --- |
  | `fingerprint` | SHA-256 as lower case hex. This is what the extension stores and sends back in `trusted`. |
  | `fpKind` | `tls`, or the SSH key type such as `ssh-ed25519`. |
  | `reason` | For a certificate: `self-signed`, `the certificate is made for another name`, `the certificate has expired or is not valid yet`, `issued by an authority that is not known`, or `the certificate could not be checked`. |
  | `display` | For SSH: the fingerprint the way OpenSSH writes it (`SHA256:...`), because that is what a person can compare. |

The extension shows this, asks, remembers a "yes" (the fingerprint only) and repeats `open` with the fingerprint in `trusted`. The helper keeps no list. A changed fingerprint is asked again.

## Versions

- `version` (`"1.1.0"`) is the version of the program. It is informational, with one use: an extension that needs an operation which only newer helpers know (`sites` needs 1.1.0) compares it before it asks.
- `protocol` (`1`) is the version of the messages. `protocolVersion` in `server.go` is the version of the messages. The extension carries the same number, and the maintainers' checks fail when the two differ.
- **Raise the number only when the two sides can no longer understand each other.** Adding an optional field, a new operation or a new event does not need it, as long as an old side that does not know it still does the right thing (`sites` was added to helper 1.1.0 this way: an older helper answers that it does not know the request). Removing or changing the meaning of a field does.

## A short session

```text
extension -> {"id":1,"op":"hello"}
helper    -> {"id":1,"ok":true,"version":"1.1.0","protocol":1,"os":"linux","arch":"amd64"}
extension -> {"id":2,"op":"open","session":"s1","kind":"ftp","host":"ftp.example.com","port":21,"user":"alice","password":"...","tls":"auto"}
helper    -> {"id":2,"ok":true,"secure":true,"note":"Encrypted (TLS)","home":"/home/alice"}
extension -> {"id":3,"op":"list","session":"s1","path":"/home/alice/"}
helper    -> {"id":3,"event":"entries","entries":[{"n":"notes.txt","d":false,"s":12,"t":1760000000000}]}
helper    -> {"id":3,"ok":true}
extension -> {"id":4,"op":"read","session":"s1","path":"/home/alice/notes.txt","offset":0,"credit":8}
helper    -> {"id":4,"event":"meta","size":12,"resumed":true}
helper    -> {"id":4,"event":"data","b64":"SGVsbG8gd29ybGQK"}
helper    -> {"id":4,"ok":true,"bytes":12}
extension -> {"id":5,"op":"close","session":"s1"}
helper    -> {"id":5,"ok":true}
```

(Shown without the 4-byte length in front of each message.)

## Trying it by hand

The Go tests start the helper's server and talk to it with a small client that speaks the real framed protocol (`harness_test.go`). It is a good starting point for another client.
