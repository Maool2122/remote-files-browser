# Contributing to the helper

Thank you for looking at it. The helper is small on purpose, and the rules below keep it that way.

## What belongs here, and what does not

**Belongs here:** anything that makes FTP, FTPS or SFTP work better for the extension: a server that behaves in an odd way, a clearer error sentence, a safer check, a faster transfer, a bug. Also the `sites` request ([docs/PROTOCOL.md](docs/PROTOCOL.md#sites-helper-110-and-newer)): a better reading of a list that FileZilla, WinSCP or Cyberduck wrote on **macOS or Linux** (the one thing that has never been run on a real installation), or one more program whose saved servers people want to bring along. Read only, and only names, addresses, ports, user names and start folders.

**Does not belong here:**

- Anything that is not a file protocol. The helper is a bridge for one extension, not a general program.
- A **network listener**, a background service, an update check, telemetry or any contact with a server the person did not open. This is the promise on the first page of the README, and the privacy policy of the extension depends on it.
- A **log** of requests, names or passwords.
- A reader of another program's list that keeps a **password, a key or the name of a key file**, even for a moment. The structures that read those lists have no field for them on purpose (`sites.go`), and the tests check that none of them reaches an answer.
- A new library without a strong reason. Every library is code that runs with the person's rights ([THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md)).
- Anything about the rest of the extension (the pages, the vault, the tabs). The extension is closed source and is not part of this repository.

## Before you start

1. Read [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) (the files and how a request travels) and [docs/PROTOCOL.md](docs/PROTOCOL.md) (the messages).
2. Build and run the tests once, so you know they pass before you change anything ([docs/BUILDING.md](docs/BUILDING.md)):

   ```text
   go vet ./...
   go test ./...
   ```

## The rules of a change

1. **A test comes with the change.** The Go tests start their own servers on the local computer (`testservers/`); add the case there. A bug fix starts with a test that fails. A test never connects to an outside server and never uses a real account.
2. **No secrets in the repository.** Not in code, tests, docs, logs or examples: no real host names, passwords, keys or fingerprints of a real server. The test servers use made-up logins.
3. **Keep the messages in plain English.** The sentences in `errors.go` and the messages that the extension shows are read by people who do not know what a "data connection" is. Say what happened and what to check, in short words, and keep the server's own words in brackets.
4. **Change the documents in the same change.** A new or changed message goes into [docs/PROTOCOL.md](docs/PROTOCOL.md); a new setup option into [docs/INSTALLING.md](docs/INSTALLING.md); a new message people can meet into [docs/TROUBLESHOOTING.md](docs/TROUBLESHOOTING.md). Say only what you have checked.
5. **The two sides agree.** `protocolVersion` in `server.go` is the version of the messages, and the extension carries the same number. Raise it only when an old extension and a new helper can no longer understand each other ([PROTOCOL.md](docs/PROTOCOL.md#versions)); that also needs a change of the extension, so open an issue first. Adding an optional field or a new operation does not need it.
6. **Output goes through `say`, never straight to the screen.** On Windows the program has no console of its own, so a command prints in a message box when nobody listens. Anything written to standard output while Chrome talks to the helper breaks the protocol; nothing may be printed there.
7. **Paths on the server are `/` paths.** Use the `path` package for them, never `filepath` (which uses backslashes on Windows). A path from the extension is used on the server only, never on this computer. The local files the helper reads are the private key whose path the person typed and the saved-server lists of `sites.go` (`filepath` is for those files only).
8. **Windows, macOS and Linux.** Code that only one system needs goes in a file with a build tag (`install_windows.go`, `install_other.go`, `ui_windows.go`, `ui_other.go`). `go vet` and the build for the other systems must still pass (`node build.mjs`).
9. **Format and vet.** `gofmt` and `go vet ./...` clean.

## Proposing a change

Describe what the problem is, what you changed, and **which tests you ran**, with the systems they ran on. Small changes are reviewed faster than large ones. If you are not sure whether something belongs here, ask first: open an issue.

For a security problem, **do not** open a public issue: see [SECURITY.md](SECURITY.md).

## Licence of a contribution

By sending a change you agree that it is published under the MIT licence of this folder ([LICENSE](LICENSE)), the same as the rest of the repository.
