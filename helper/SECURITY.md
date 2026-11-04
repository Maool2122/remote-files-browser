# Security

This page says what the helper is built to do, what it does **not** protect against, and how to report a problem. It is meant to be read next to the code: every statement can be checked in the file it names.

## What the helper is

A small program that opens FTP, FTPS and SFTP connections for one browser extension, **as the person who runs it**. It has no more rights than that person and gives no right to anyone else.

## What it does and does not do

| It does | It does not |
| --- | --- |
| Start only when Chrome starts it, for the extension on the list of its registration file, and stop when that page closes. | Listen on any network port. Run as a service or in the background. Start with the computer. |
| Connect to the one server the person opened, and only after the extension asked. | Contact Rocuo or anyone else. Check for updates. Send usage data. The program does not even contain a web client: `go list -deps .` lists no `net/http` (the WebDAV server in `testservers/` is for tests and is not part of the program). |
| Keep a password **in memory, for that connection only**. | Write a password, a file name, a server name or a request to disk or to a log. It keeps no log. The only text it ever writes to its error output is a fatal error of its own (a broken pipe, a message that is too large), never request data. |
| Read **one** local file besides its own: the private key file whose path the person typed for an SFTP connection (`loadPrivateKey` in `sftp.go`). | Read or write any other local file while it works. Downloaded and uploaded files go through the extension, which writes and reads the person's files. |
| Write files only when it is installed or removed: its own copy and its registration file in the person's folder, and the per-user registry keys (Windows) ([INSTALLING.md](docs/INSTALLING.md)). | Need administrator rights. Touch anything outside the person's own folders and registry branch. |

## Who can drive it

- Chrome starts the helper **only for the extensions listed in `allowed_origins`** of its registration file: the Chrome Web Store address of Remote Files Browser, plus the addresses a person added with `install --extension-id`. A web page, another extension or another browser profile cannot start it through Chrome.
- The helper does **not** check that address again: it is given as the command-line argument by Chrome, and Chrome has already checked it. Anyone who can run the program by hand on that computer can talk to it through the same pipe, but that person could equally run an FTP program, with the same rights.
- Because the helper is a general FTP/SFTP client, **an extension that is allowed to start it can make it connect to any server** that the computer can reach, and can ask it to log in with a key file the person's account can read (it cannot read the key back: the helper only uses it to log in). So add only extensions you trust with `--extension-id`. `status` shows who is on the list.

## The identity of the server

A password must never go to the wrong server, so the helper does not log in before it knows who it is talking to.

- **FTPS and FTP with TLS** (`trust.go`, `tlsConfig`): TLS 1.2 or newer. A certificate that is valid for the typed host name and chains to the computer's own list of authorities is accepted. Any other certificate is accepted **only** if its SHA-256 fingerprint is in the list the extension sends with `open` (`trusted`). Otherwise the helper stops with a `trust` error that carries the fingerprint and the reason; the person decides in the extension.
- **Why `InsecureSkipVerify: true` is in the code.** Go's TLS verification stops at the first failure, before our fingerprint list can be asked. So the standard check is switched off and replaced by `VerifyConnection`, which runs the same check (`leaf.Verify` with the host name and the intermediates) **and** the fingerprint list, and it runs again when a session is resumed. A code scanner will flag the flag; the function next to it is the reason it is safe.
- **SFTP** (`hostKeyCallback`): the helper keeps no `known_hosts` file and trusts no host key by itself. A host key is accepted only if its SHA-256 is in `trusted`. The first connection to every SFTP server therefore asks the person once (the extension shows the `SHA256:...` form that OpenSSH shows). A changed key is a new question.
- The helper keeps **no list of trusted servers**: the extension does, and sends the list each time.
- **FTP without TLS** (`none`, or `auto` when the server has no TLS) sends the password and the files in plain text. The extension shows **Not encrypted** and, when the person chooses `none`, a warning that the password and the files can be read on the network. `auto` never goes down to plain text when TLS is offered but the login fails: a wrong password stays a wrong password (`TestFTPWrongPasswordIsAuthAndNeverDowngrades`).

## Resource limits

- A message from the extension over **32 MiB** ends the helper (`maxIncoming`). A message the helper sends stays below Chrome's 1 MB limit.
- FTP uses **at most 6** connections per session. Connecting times out after 20 seconds; a download that gets no data for 90 seconds stops.
- A download sends a piece only when the extension gave **credit** for it, so a slow page can never make the helper hold unlimited data in memory.
- `cancel` closes the network connection that a request waits on, so nothing stays stuck behind a slow server.

## Known limits

- **The programs are not code-signed** (no certificate has been bought yet). Windows SmartScreen and macOS Gatekeeper may warn. Compare the checksum ([INSTALLING.md](docs/INSTALLING.md#checking-a-file)) or build it yourself ([BUILDING.md](docs/BUILDING.md)).
- **It is not a sandbox.** It runs with the rights of the person, like any program they start.
- **A private key file path** is read with the person's rights. A key with a passphrase is decrypted in memory only.
- **Windows is the only system that has been run.** The macOS and Linux programs compile; their registration and start from Chrome have not been run.
- The libraries (`go.sum`) are listed in [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md). A vulnerability scan (`govulncheck ./...`) was run on 2026-11-04. With Go 1.27.0 it reported vulnerabilities **in the Go standard library** (among them in `crypto/x509` and `crypto/tls`, which the program uses) that later patch releases of Go fix; with Go 1.27.3 it reports none that the program can reach (it names one advisory about the `openpgp` package of `golang.org/x/crypto`, which the helper does not use). The standard library is part of every program, so **build a release with the newest patch release of Go**. The repository runs the scan again every week ([`security.yml`](../.github/workflows/security.yml)); the Go version that built a program is inside the file (`go version -m <file>`).

## Reporting a problem

If you find a security problem, **please do not open a public issue**. Report it privately: the way is in the [security policy of this repository](../SECURITY.md) (GitHub's private "Report a vulnerability" form, or e-mail). Say "security of the Remote Files Browser helper" and include the version (`rocuo-files-helper version`), the system, and the steps. Do not send passwords, keys or private files.
