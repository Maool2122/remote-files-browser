# How the helper works

This document explains the program for someone who wants to read or change the code. For what it does for a user, see the [README](../README.md). For the exact messages, see [PROTOCOL.md](PROTOCOL.md).

## Three ways to start it

`main.go` looks at how it was started:

| Started as | What happens | Code |
| --- | --- | --- |
| `rocuo-files-helper chrome-extension://ID/` | **Host mode.** Chrome started it. It speaks the native messaging protocol on standard input and output until Chrome closes the pipe. | `runHost` in `main.go`, then `server.go` |
| `rocuo-files-helper` (no arguments) | **Setup by double-click.** It installs itself for the current user and shows a message box. | `runSetup` in `main.go`, then `install.go` |
| `rocuo-files-helper install`, `uninstall`, `status`, `version` | **Commands.** | `cmdInstall`, `cmdUninstall`, `statusReport` |

Chrome gives the host the address of the extension as the first argument (`chrome-extension://<id>/`). The program only needs to see that argument to know it is in host mode. The decision whether this extension may start it was already made by Chrome, from the registration file.

## The files

| File | What it does |
| --- | --- |
| `main.go` | Entry point, the commands, the usage text, the message after a double-click. |
| `install.go` | The setup: copy the program, write the registration file, merge the list of allowed extensions, `status` report, `uninstall`. Works the same on every system. |
| `install_windows.go` | The Windows part of the setup: registry values (`HKEY_CURRENT_USER`) for Chrome, Edge, Brave and Chromium. |
| `install_other.go` | The macOS and Linux part: a registration file in the `NativeMessagingHosts` folder of each browser. |
| `ui_windows.go`, `ui_other.go` | Windows: a message box, and joining the terminal the command was typed in (the Windows build has no console window of its own). Elsewhere: nothing needed. |
| `protocol.go` | Native messaging framing (4-byte length + JSON) and the `message` type that holds every request. |
| `server.go` | Reads requests, runs each one in its own goroutine, keeps the open sessions, and sends answers and events. `version` and `protocolVersion` live here. |
| `backend.go` | The `backend` interface: what a connection to a server must be able to do. |
| `ftp.go` | The FTP and FTPS backend (library `jlaffaye/ftp`), with a small pool of connections per server. |
| `sftp.go` | The SFTP backend (libraries `pkg/sftp` and `x/crypto/ssh`). |
| `trust.go` | The identity checks: TLS certificates and SSH host keys. The program never decides alone. |
| `errors.go` | Turns the errors of the libraries into plain sentences with a code the extension understands. |
| `testservers/` | FTP, FTPS, SFTP and WebDAV servers for the tests. They run on this computer on random ports. Not part of the program. |
| `testserver/main.go` | Starts the test servers and prints their ports as one line of JSON (the real-Chrome tests of the extension use it). |
| `*_test.go` | Unit tests, protocol tests, and tests against the test servers. |
| `build.mjs` | Builds the program for every system and writes `SHA256SUMS.txt`. |

## How a request travels

```
extension page --postMessage--> Chrome --stdin--> readMessage -> dispatch -> start -> handle -> backend (ftp / sftp)
extension page <-onMessage----- Chrome <-stdout-- writer.send <- ok / fail / event <-----------+
```

1. `server.run` reads one message at a time (`readMessage`). Anything that is not valid JSON is ignored.
2. `dispatch` handles the few messages that belong to a request that is **already running** (`credit`, `chunk`, `end`, `cancel`) and starts a goroutine for everything else.
3. `start` records the request in `ops` (so later messages can find it), runs `handle` and sends exactly one final answer: `ok` with the result or `fail` with an error. A panic inside is turned into an error; it never ends the program.
4. `handle` finds the session (`open` makes one, the other operations look it up by the `session` name) and calls the backend.
5. Long operations send **events** first: `entries` for a listing, `meta` and `data` for a download, `ready` and `ack` for an upload. See the protocol.
6. Everything that leaves the program goes through one `writer`, which locks and writes each message whole, because several requests answer at the same time. If writing fails, Chrome is gone, and the program cancels everything.

## Flow control

A download could fill the memory if the page reads slowly. So the program sends a `data` message only when the extension has given it a **credit** for it (`credit` in the `read` request, and `credit` messages afterwards). Without credit it waits. An upload works the other way round: the extension sends a `chunk`, and the program answers with an `ack` after writing it, so the extension can see how much has really been written and keep only a few chunks in flight.

A chunk of a download is 192 KiB (256 KiB as base64 text), so a message stays well under the 1 MB that Chrome accepts from a native host (`maxOutgoing` in `protocol.go`).

## Connections and sessions

`open` creates a **session**: a named connection to one server (the extension chooses the name). The session lives until `close`, until the connection is lost, or until the program exits.

- **FTP and FTPS** (`ftp.go`) keep a pool of at most 6 connections to the server, because FTP does one thing per connection at a time. The pool lets a listing and several transfers run together. Connecting to the server times out after 20 seconds, and a download that receives no data for 90 seconds stops with a timeout.
- **SFTP** (`sftp.go`) uses one SSH connection with many channels, and sends a keep-alive every 30 seconds.

Every operation takes a `context`. `cancel` cancels the context of that request, which closes the network read or write that was waiting, so a cancel is never stuck behind a slow server.

## Trust

An FTPS certificate or an SSH host key is checked in `trust.go`:

- A certificate that the computer's own list of authorities accepts, for the host name that was typed, passes.
- Anything else passes only if its SHA-256 fingerprint is in the `trusted` list that came with `open`.
- Otherwise the connection stops with a `trust` error (`trustError`) that carries the fingerprint, its kind (`tls` or the SSH key type such as `ssh-ed25519`), and for a certificate the reason (`self-signed`, `made for another name`, `expired`, ...). The extension shows it, asks the person, remembers an answer of "trust", and sends `open` again with the fingerprint in `trusted`.

The program keeps no list of trusted servers. The list lives in the extension, where the person can see and clear it.

## Errors

`errors.go` has one job: every error that leaves the program is `{code, message}`, where `message` is a plain sentence that can be shown as it is, and `code` tells the extension what to do (`auth`, `trust`, `cancelled`, `session`, `generic`). `classify` recognises the FTP reply codes (421, 530, 550, ...), the SFTP status codes, the network errors (name not found, connection refused, timeout, connection lost) and gives each a sentence. An unknown error is passed on as it is rather than hidden.

## Setup and registration

`install.go` does the setup in a fixed order:

1. Check that every `--extension-id` really looks like one (32 letters from `a` to `p`).
2. Make the install folder, copy the running program there (`copyFile`). A running program cannot be overwritten on Windows, but it can be **renamed**, so an old copy that is in use is renamed `*.old-<number>` and removed by a later install.
3. Read the extension addresses of an earlier setup from the registration file (`earlierExtensionIDs`), add the new ones (unless `--reset-extensions`), and write the file (`buildManifest`). The Chrome Web Store address (`publishedExtensionIDs`) is always first.
4. Tell the browsers where the file is (`registerHost`: registry values on Windows, copies of the file elsewhere).

`status` (`statusReport`) reads the same places back and prints them: what each browser is told, whether the program it would start exists, and which extensions may use it. It is the first thing to look at when Chrome says the helper cannot be found or is forbidden.

## The Windows build has no console

The Windows program is built as a GUI program (`-H=windowsgui`). Two reasons: no console window flashes up when Chrome starts the helper, and a double-click shows a message box instead of a black window that closes at once. The cost: a GUI program has no terminal of its own. `attachConsole` (`ui_windows.go`) joins the terminal that the command was typed in, so `status` prints there. When there is none (the Run window, a shortcut), the result goes into a message box so that the person is never left with nothing.

PowerShell does not wait for a GUI program when you run it as a plain command, so the output can arrive late or out of order. See [TROUBLESHOOTING.md](TROUBLESHOOTING.md).

## Adding another protocol

1. Implement `backend` in a new file (the interface is small and documented in `backend.go`).
2. Add the `kind` to `opOpen` in `server.go`.
3. Add a test server in `testservers/` and tests next to the others.
4. If the messages change in a way an older extension cannot understand, raise `protocolVersion` (see [PROTOCOL.md](PROTOCOL.md), "Versions").
