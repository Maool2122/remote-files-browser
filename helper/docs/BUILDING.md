# Building and testing the helper

## What you need

- **Go 1.26 or newer** (<https://go.dev/dl/>). `go.mod` says `go 1.26.0`; the release files of version 1.1.0 were built with Go 1.27.3. Build releases with the newest Go: an older one (1.27.0) puts two known advisories of the Go standard library into the program ([SECURITY.md](../SECURITY.md)).
- **Node.js** only for `build.mjs`, the small script that builds for every system and writes the checksums. You can use `go build` directly instead.
- Nothing else. The program is pure Go (`CGO_ENABLED=0`), so there is no C compiler, and every system can be built from any system.

The libraries come from the Go module cache (`go.mod`, `go.sum`); the first build downloads them. The list is in [THIRD_PARTY_NOTICES.md](../THIRD_PARTY_NOTICES.md).

## Build

From this folder (`helper/`):

```text
node build.mjs              all six programs
node build.mjs windows      only the Windows ones (windows, darwin or linux)
```

or from the root of the repository, `node helper/build.mjs` (the same script).

The programs are written to `dist/` (not in git), with a `SHA256SUMS.txt`:

| File | For |
| --- | --- |
| `rocuo-files-helper-windows-amd64.exe` | Windows, normal PCs |
| `rocuo-files-helper-windows-arm64.exe` | Windows on ARM |
| `rocuo-files-helper-darwin-arm64` | macOS, Apple silicon |
| `rocuo-files-helper-darwin-amd64` | macOS, Intel |
| `rocuo-files-helper-linux-amd64` | Linux, 64-bit PCs |
| `rocuo-files-helper-linux-arm64` | Linux on ARM |

The flags (`build.mjs`): `go build -trimpath -buildvcs=false -ldflags "-s -w"` with `CGO_ENABLED=0`. `-trimpath` keeps your folder names out of the program, `-buildvcs=false` keeps a commit stamp out of it, `-s -w` drops the debug tables. **On Windows the flag `-H=windowsgui` is added**: the program is a GUI program, so no console window flashes up when Chrome starts it, and a double-click shows message boxes (see [ARCHITECTURE.md](ARCHITECTURE.md)).

To build one program by hand for the computer you are on:

```text
go build -trimpath -buildvcs=false -o rocuo-files-helper .
```

### The same bytes

The build does not put the time, your folder names or a commit stamp into the program, so the same source and the same Go version give the same file. This was checked on Windows. For version 1.0.0 (Go 1.27.0): on 2026-10-28 the two Windows programs built from the source ZIP, in another folder, had the same SHA-256 as the published files, and on 2026-11-04 all six programs did. For version 1.1.0 (Go 1.27.3): on 2026-11-04 the Windows 64-bit program built twice had the same SHA-256 both times, and **all six programs** (Windows, macOS and Linux, Intel/AMD and ARM) built from the unpacked source ZIP of 1.1.0, in another folder, had the same SHA-256 and the same size as the files published at <https://rocuo.com/remote-files/ftp-helper/>. Unpack the source ZIP, run `node build.mjs` (or `node build.mjs windows`) and compare with `SHA256SUMS.txt` to check what you run. These checks were made on one Windows computer, which also cross-compiled the macOS and Linux programs; a build on a Mac or on a Linux computer has not been compared, and another Go version gives other files.

### Not signed

The programs are **not code-signed**. A signed Windows program needs a code-signing certificate, and a Mac program needs a Developer ID and notarization. These belong to the publisher, so the build never does them. Until a release is signed, Windows SmartScreen and macOS Gatekeeper may warn about a downloaded copy; the checksum in `SHA256SUMS.txt` is how to compare ([INSTALLING.md](INSTALLING.md#checking-a-file)).

## Test

```text
go vet ./...
go test ./...
```

The tests start **their own servers on the local computer** (`testservers/`, `127.0.0.1`, random ports, in memory). They never connect to an outside server and need no network and no account. The test servers use made-up logins (`alice` / `secret`).

| File | What it checks |
| --- | --- |
| `protocol_test.go` | The framing (round trip, little-endian length, too large, cut frames), the error objects, the trust set. |
| `ftp_test.go` | FTP: plain, explicit and implicit TLS with the trust question, `auto` with and without TLS on the server, a wrong password (and that it never falls back to something weaker), anonymous, a big listing in batches, cancelling a download and using the session afterwards, parallel downloads, a server that allows only two connections, using a closed session, an unreachable server. |
| `sftp_test.go` | SFTP: password flow, wrong password, key files (with and without a passphrase), parallel transfers, an unreachable server. |
| `flow_test.go` | The walk-through that every protocol must pass (FTP and SFTP both run it): nested folders (and again), uploads (an empty file, 2.5 MB, names with accents and spaces), listing, `stat` of a file, a folder and a missing path, download whole and from the middle, a missing file giving an error with words, rename, delete of a file and of a folder, an upload cancelled half way leaving nothing behind, and the session still working after that. |
| `install_test.go` | The registration: only the listed extensions, copy + register + uninstall, **earlier extension addresses are kept**, only valid addresses are trusted from an old file, the text of `status` and `install`, `--no-copy`. |
| `sites_test.go`, `sites_windows_test.go` | The saved-server import: sample lists in the formats of FileZilla, WinSCP (a file and, on Windows, a registry branch of its own) and Cyberduck; every kind of server, folders and groups, the start folder, broken, unreadable and too large files, the limits, the `sites` message, and that no password, key or key file name is ever in the answer. |
| `harness_test.go`, `servers_test.go` | Helpers: a client that speaks the real framed protocol to the server, and the test servers. |

The install tests never touch the real registration. On Windows `install_windows_test.go` points the registration at a registry branch of its own (`Software\RocuoHelperTest-<number>`) and removes it afterwards; on macOS and Linux `install_other_test.go` uses a temporary home folder.

### The test servers on their own

```text
go run ./testserver -bulk 2000
```

starts FTP, FTP with explicit TLS, FTP with implicit TLS, FTP that allows only two connections, SFTP and WebDAV, with a few files (and, with `-bulk N`, `N` small files in `/bulk`). It prints one line of JSON with the ports and the fingerprints and runs until its standard input is closed. Use it to try another client against a known server.

### The tests of the extension

The extension is closed source, so its own tests are not in this repository. They run the built extension in a real Chrome (Chrome for Testing) against this helper and the test servers above, and the maintainers run them on Windows before a release. What the extension and the helper say to each other is written down in [PROTOCOL.md](PROTOCOL.md), and the Go tests in this folder check the helper's side of it.
## What has and has not been run

- **Windows:** built; `go vet ./...` is clean and the 51 Go tests of version 1.1.0 pass (2026-11-04, Go 1.27.3, run with `go test -count=1 ./...` so that nothing comes from the cache). The maintainers' own tests of the extension with this helper in a real Chrome (53 and 28 checks; they are not in this repository) passed in Chrome for Testing 149 and in 156 on 2026-10-28 with version 1.0.0; they were **not run for 1.1.0** (they refuse to run when a helper is already registered on the computer, and one was). The saved-server import was run through the helper's own pipes with the compiled Windows program (a test of the maintainers' own, not in this repository), and the extension's import window in a real Chrome against a stand-in for the helper.
- **macOS and Linux:** the programs **compile**, and `go vet ./...` passes for both (`GOOS=linux`, `GOOS=darwin`, `CGO_ENABLED=0`). **Nothing has been run on those systems**: not the Go tests, not the registration code (`install_other.go`), not a start from Chrome. Before a release, run on a Mac and a Linux computer at least `go test ./...`, `install`, `status` and one connection by hand.

## Changing the code

See [CONTRIBUTING.md](../CONTRIBUTING.md) for the rules, and [ARCHITECTURE.md](ARCHITECTURE.md) for how to add a protocol or an operation.
