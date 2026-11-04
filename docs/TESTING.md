# Testing

Everything that can be tested on one computer is tested with a few commands. No account, no server of ours and no real FTP server is needed: the tests start their own servers on `127.0.0.1`.

This page says what the tests are, what each one proves, how to run one alone, how to write a new one, and what is **not** tested. The build and the test notes of the helper itself are in [helper/docs/BUILDING.md](../helper/docs/BUILDING.md).

- [The kinds of test](#the-kinds-of-test)
- [What the Go tests prove](#what-the-go-tests-prove)
- [The last full run](#the-last-full-run)
- [Run one test alone](#run-one-test-alone)
- [Write a test](#write-a-test)
- [What continuous integration runs](#what-continuous-integration-runs)
- [What is not tested by a script](#what-is-not-tested-by-a-script)

## The kinds of test

| Kind | Where it is | Run it with | Needs |
| --- | --- | --- | --- |
| Go tests of the helper | `helper/*_test.go` (11 files), with in-memory test servers in `helper/testservers/` and `helper/testserver/` | `cd helper && go test -count=1 ./...` | Go |
| `go vet` | all of `helper/` | `cd helper && go vet ./...` | Go |
| Formatting | all of `helper/` | `cd helper && gofmt -l .` (it prints nothing when all is well) | Go |
| The six builds | `helper/build.mjs` | `node helper/build.mjs` | Go, Node.js |
| Document checks | `tools/check-docs.mjs` | `node tools/check-docs.mjs` | Node.js |
| Known problems in the libraries and in the Go standard library | all of `helper/` | `cd helper && go run golang.org/x/vuln/cmd/govulncheck@latest ./...` (every week in CI) | Go |

`-count=1` makes Go run the tests again and not show an old result from its cache.

## What the Go tests prove

The tests start **their own servers on the local computer** (`helper/testservers/`, `127.0.0.1`, random ports, in memory). They never connect to an outside server and need no network and no account. The test servers use made-up logins (`alice` / `secret`).

| File | What it checks |
| --- | --- |
| `protocol_test.go` | The framing (round trip, little-endian length, too large, cut frames), the error objects, the trust set. |
| `ftp_test.go` | FTP: plain, explicit and implicit TLS with the trust question, `auto` with and without TLS on the server, a wrong password (and that it never falls back to something weaker), anonymous, a big listing in batches, cancelling a download and using the session afterwards, parallel downloads, a server that allows only two connections, using a closed session, an unreachable server. |
| `sftp_test.go` | SFTP: password flow, wrong password, key files (with and without a passphrase), parallel transfers, an unreachable server. |
| `flow_test.go` | The walk-through that every protocol must pass (the function `exercise`; the FTP and the SFTP tests both run it): nested folders, uploads (an empty file, 2.5 MB, names with accents and spaces), listing, `stat` of a file, a folder and a missing path, download whole and from the middle, a missing file giving an error with words, rename, delete of a file and of a folder, an upload cancelled half way leaving nothing behind, and the session still working after that. |
| `install_test.go` | The registration: only the listed extensions, copy + register + uninstall, earlier extension addresses are kept, only valid addresses are trusted from an old file, the text of `status` and `install`, `--no-copy`. |
| `sites_test.go`, `sites_windows_test.go` | The import of saved servers (helper 1.1.0): sample lists in the formats of FileZilla, WinSCP (a file and, on Windows, a registry branch of its own that the test makes and removes) and Cyberduck; every kind of server, folders and groups, the start folder, broken, unreadable and too large files, the limits, the `sites` message through the real framed protocol, and that **no password, key or key file name is ever in the answer** (the sample lists hold made-up ones on purpose, and a test fails when one of them comes out). |
| `harness_test.go`, `servers_test.go` | Helpers: a client that speaks the real framed protocol to the helper, and the test servers. |
| `install_windows_test.go`, `install_other_test.go` | The install tests never touch the real registration. On Windows the registration points at a registry branch of its own (`Software\RocuoHelperTest-<number>`) that is removed afterwards; on macOS and Linux a temporary home folder is used. |

The WebDAV test server in `helper/testservers/` is for trying other clients: the helper program itself speaks FTP, FTPS and SFTP.

## The last full run

On 2026-11-04, on Windows 11 with Go 1.27.3, for helper 1.1.0:

| Check | Result |
| --- | --- |
| `gofmt -l .` | Printed nothing. |
| `go vet ./...` | Clean. It is also clean when run for the other five systems the helper is built for (Windows on ARM, macOS and Linux, Intel/AMD and ARM), which type-checks the code and the tests of each: **nothing was run there**. |
| `go test -count=1 ./...` | **51 tests passed, none failed, none skipped** (about 10 seconds). |
| `node helper/build.mjs` | All six programs were built from the files of this repository. Their SHA-256 checksums are the same as those of the files on the download page of the helper (built with the same Go version, 1.27.3). |
| `govulncheck ./...` | The vulnerability database was from 2026-11-02. Your code is affected by 0 vulnerabilities, and the program imports no package with one; one advisory is in a library that the helper requires but does not call ([helper/SECURITY.md](../helper/SECURITY.md#known-limits)). |
| `node tools/check-docs.mjs`, `node tools/check-scope.mjs` | Both pass. |

The same commands run on GitHub's computers on every change (below). The macOS and Linux results of those runs are in the Actions tab of the repository.

## Run one test alone

```bash
cd helper
go test -run TestName ./...       # one Go test (the name is a pattern, so a part of it is enough)
go test -v -run TestFTPPlain ./... # plain FTP with the whole walk-through of flow_test.go (-v shows the test running)
go run ./testserver -bulk 2000    # the test servers on their own, to try another client (stop with Ctrl+C)
```

A test prints `--- FAIL` with its name when it fails, and the run ends with a non-zero exit code.

## Write a test

Put `thing_test.go` in `helper/`. The test servers in `helper/testservers/` are real servers kept in memory and bound to `127.0.0.1`; they are used only by tests and are never part of the helper program. To check a protocol operation, look at `flow_test.go`: it runs the same steps against FTP and against SFTP through the real framed protocol (`harness_test.go`).

Test what the person sees, and the failing cases too (a wrong password, a name that exists, a server that stops, a cancelled transfer). When you fix a bug, add the test that would have caught it, in the same pull request ([CONTRIBUTING.md](../CONTRIBUTING.md)).

## What continuous integration runs

[`.github/workflows/ci.yml`](../.github/workflows/ci.yml) runs on every push to `main` and every pull request:

- `go vet` and `go test` of the helper on Windows (must pass), and on Linux and macOS (experimental: they do not block, and they are the first signal for a system nobody has run yet);
- the six builds of the helper program, so that a change which breaks one system is seen before a release;
- the document check.

[`.github/workflows/security.yml`](../.github/workflows/security.yml) runs `govulncheck` every week and on changes to the Go module files.

## What is not tested by a script

- **The extension.** It is closed source. The maintainers run their own tests of it with this helper in a real Chrome against the test servers, on Windows; those tests are not in this repository, and they were last run with helper 1.0.0 (2026-10-28), not repeated for 1.1.0. What the extension and the helper say to each other is written down in [PROTOCOL.md](../helper/docs/PROTOCOL.md), and the Go tests here check the helper's side of it.
- **The import of saved servers** (helper 1.1.0). It was tried with sample lists in the formats of FileZilla, WinSCP and Cyberduck, and on Windows with the compiled program run through its pipes on sample files and on a test registry key. It has **never been run against the lists of a real installation** of those programs, and never on macOS or Linux, where the lists are in other places ([PROTOCOL.md](../helper/docs/PROTOCOL.md#sites-helper-110-and-newer)).
- The helper on **macOS, Linux and Windows on ARM**: built and vetted, never run by the maintainers ([PLATFORMS.md](PLATFORMS.md)).
- **Windows 10.** The tests were run on Windows 11.
- **Real servers of a company.** The FTP, FTPS and SFTP tests use test servers made from other people's libraries (an FTP server library and a Go SSH library). Other servers can still answer differently: report one that does ([SUPPORT.md](../SUPPORT.md)).
- **A start from Chrome on another system**, the registration code of macOS and Linux (`install_other.go`) with the real browsers, and Edge, Brave and Chromium.
