# Contributing

Thank you for looking at this project. It is open so that anyone can read it, use it, check it and build on it ([why](docs/WHY-OPEN-SOURCE.md)), and a good contribution is welcome whether it is a line of code, a test on a system we do not have, or a sentence in a document.

This repository holds the **helper program** only. The Chrome extension that uses it is closed source and is not part of this repository, so a pull request cannot change it. A change to the helper that would also need a change in the extension (a new message, for example) starts with an issue.

By taking part you agree to follow the [Code of Conduct](CODE_OF_CONDUCT.md).

## Ways to help

| You | Start with |
| --- | --- |
| Found something that does not work | A [bug report](https://github.com/Maool2122/remote-files-browser/issues/new?template=bug_report.yml) (no passwords or private addresses, please) |
| Use macOS, Linux, Windows 10 or Windows on ARM | **Try the helper program** and send a [platform report](https://github.com/Maool2122/remote-files-browser/issues/new?template=platform_report.yml): [docs/PLATFORMS.md](docs/PLATFORMS.md). This is the most valuable help we can get today |
| Run an FTP, FTPS or SFTP server that answers in an odd way | A bug report that says what the server answered (no private data). It can become a test |
| Want a package for your system (Homebrew, Scoop, winget, a distribution) | An issue, so that we agree on the way ([docs/PLATFORMS.md](docs/PLATFORMS.md#other-ways-to-help)) |
| Have an idea | A [feature request](https://github.com/Maool2122/remote-files-browser/issues/new?template=feature_request.yml) |
| Found a mistake or a gap in a document | A pull request. Documents are as important as code here |
| Want to fix a bug or build a feature | Read on |
| Found a security problem | **Not here.** [SECURITY.md](SECURITY.md) |

## Before you write code

1. **Open an issue first** for anything bigger than a small fix, so that we can agree it belongs here before you spend your time. Small fixes and document changes can go straight to a pull request.
2. **Read** [helper/docs/ARCHITECTURE.md](helper/docs/ARCHITECTURE.md) (how the program is built) and [helper/CONTRIBUTING.md](helper/CONTRIBUTING.md) (the rules for the code).
3. **Set it up:** install [Go](https://go.dev/dl/) (the newest patch release) and [Node.js](https://nodejs.org/) 22 or newer, then follow [helper/docs/BUILDING.md](helper/docs/BUILDING.md). Run the tests once before you change anything, so you know they pass.

## The rules of a change

1. **A test comes with the change.** A bug fix starts with a test that fails. Tests never use a real account or a server that is not on your own computer ([docs/TESTING.md](docs/TESTING.md)).
2. **Keep the promises of the helper.** These are what makes it safe to run, and a change that breaks one will not be accepted:
   - **It opens no port, runs no service, keeps no log and checks for no updates.** It contacts only the server that the person opened ([helper/SECURITY.md](helper/SECURITY.md)).
   - **No server of ours, no account, no tracking, no analytics.**
   - **No remote code.** Nothing is downloaded and run.
   - **A password is never in an address, a log or plain text.** It stays in memory for that connection.
   - **Other programs' saved servers are read only when the person asks, read only, and never with a password or a key.** Names, addresses, ports, user names and start folders only (`helper/sites.go`).
   - **Trust is asked, never assumed.** A new or changed certificate or host key goes back to the person.
   - **No new library without a strong reason:** every one is code that runs with the person's rights.
3. **No secrets in the repository.** Not in code, tests, documents, screenshots or comments: no password, key, token, private address, real server name or personal name. The test servers use made-up logins (`alice` / `secret`).
4. **Plain words in every message a person can read.** Say what happened and what to check, in short words, and keep the server's own words in brackets. Comments say *why*, not *what*.
5. **Change the documents in the same change:** the helper's own `helper/docs/` for its messages and set-up, [docs/](docs/README.md) for the rest, and [CHANGELOG.md](CHANGELOG.md) under "Unreleased". Say only what you have checked.
6. **A small change is a good change.** One idea for each pull request. No unrelated clean-ups or renames in the same change.
7. **The two sides of the helper agree.** `protocolVersion` in `helper/server.go` is the version of the messages, and the closed-source extension carries the same number. It changes only when an old extension and a new helper can no longer understand each other ([helper/docs/PROTOCOL.md](helper/docs/PROTOCOL.md#versions)); adding an optional field or a new operation does not need it.
8. **Style.** `gofmt` for Go, LF line endings (`.editorconfig` and `.gitattributes` take care of it).
9. **Only the helper belongs here.** Nothing that belongs to the extension (screens, TypeScript, a manifest, a package list). `node tools/check-scope.mjs` fails when something like that is added.

## Check before you send

```bash
cd helper
gofmt -l .                      # prints nothing when all is well
go vet ./...
go test -count=1 ./...          # the tests start their own servers on 127.0.0.1
cd ..
node tools/check-docs.mjs       # every link, anchor and version in the documents
node tools/check-scope.mjs      # only the helper, its documents and its tools are here
```

The pull request template asks the same questions, so that you and the reviewer can both see them. If you could not run something (for example because you are on another system), say so in the pull request: that is fine, and it tells the reviewer what to look at.

## Sending a pull request

1. Fork the repository and make a branch.
2. Make your change, with its test and its documents.
3. Write a short, clear message for each commit (what changed and why, in the present tense).
4. Open the pull request and fill in the template. Link the issue ("Fixes #123").
5. A maintainer reviews it. We may ask for changes: please do not take it personally, it is how the code stays small and safe. We cannot promise how fast we answer, but we read everything.
6. CI runs on your pull request (for your first one a maintainer may have to press "Approve and run" first: GitHub asks for this for new contributors). The Windows job must be green. The Linux and macOS jobs are experimental ([docs/PLATFORMS.md](docs/PLATFORMS.md)): if one fails, look at it, and tell us if you think it is not your change.

## The licence of your contribution

This project is under the [MIT licence](LICENSE). When you send a change, you confirm that you wrote it, or that you have the right to send it, and you agree that it is published under that licence, like the rest of the repository. You keep the copyright of your work.

## The name and the pictures

The code is MIT. The names "Rocuo" and "Remote Files Browser FTP by Rocuo", the logo, the icon and the pictures are not part of the licence: [TRADEMARKS.md](TRADEMARKS.md) says what you may do with them (a fork is welcome, with a name of its own).

## Who maintains it

The project is published by [Rocuo](https://rocuo.com) and maintained at <https://github.com/Maool2122/remote-files-browser>. Releases are made by a workflow and published by a maintainer ([docs/RELEASING.md](docs/RELEASING.md)).
