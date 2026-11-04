## What this changes, and why

<!-- Say what a person will see or what a developer will find different. Link the issue if there is one: "Fixes #123". -->

## Kind of change

- [ ] Bug fix
- [ ] New feature or change of behaviour
- [ ] Documents only
- [ ] Build, CI or tools
- [ ] Helper program (Go)

## Checks

- [ ] In `helper/`: `gofmt -l .` prints nothing, `go vet ./...` and `go test -count=1 ./...` pass (or this is documents only).
- [ ] `node tools/check-docs.mjs` and `node tools/check-scope.mjs` pass.
- [ ] A test comes with the change (or this is documents only). A bug fix starts with a test that fails.
- [ ] If it changes the messages between the extension and the helper: the message version (`protocolVersion` in `helper/server.go`) changed **only** if an old extension and a new helper can no longer understand each other, and I opened an issue first.
- [ ] The documents that describe it are changed in this pull request (the helper's `docs/`, [docs/](https://github.com/Maool2122/remote-files-browser/blob/main/docs/README.md)), and [CHANGELOG.md](https://github.com/Maool2122/remote-files-browser/blob/main/CHANGELOG.md) has a line under "Unreleased".
- [ ] I could run it on: <!-- Windows, macOS, Linux, ... If you could not run something, say so here: that is fine. -->

## Safety

- [ ] **No new network connection** except to the server that the person opened, and no tracking or analytics.
- [ ] **The helper still opens no port, runs no service, keeps no log and checks for no updates.**
- [ ] **No remote code** (nothing downloaded and run).
- [ ] **No new library**, or a strong reason for it is written above.
- [ ] **No secret** in the change: no password, key, token, private address or personal name, also not in a test, a picture or a comment.
- [ ] **Only the helper**: nothing that belongs to the closed-source extension.

## Notes for the reviewer

<!-- Anything that is hard to see in the diff: what you tried, what you left out on purpose, what you could not test. -->
