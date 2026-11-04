# Changelog

All notable changes to the helper program are written here. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the version numbers are those of the helper (`version` in [helper/server.go](helper/server.go)). The message version between the extension and the helper (`protocolVersion`) is a separate number that changes rarely ([helper/docs/PROTOCOL.md](helper/docs/PROTOCOL.md#versions)). Dates are the days a version was released.

## [Unreleased]

Nothing yet.

## [1.1.0] - 2026-11-04

The helper program that goes with version 1.2.0 of the Chrome extension (the new window **Import saved servers**). It has been online as downloads at <https://rocuo.com/remote-files/ftp-helper/> since 2026-11-04; the new files replaced those of 1.0.0 under the same names. Its source is in this repository.

### Added

- **The `sites` request.** The extension can ask for the servers that the person saved in **FileZilla** (`sitemanager.xml`), **WinSCP** (`WinSCP.ini` and, on Windows, the registry of the current user) and **Cyberduck** (the `.duck` bookmark files), so that it can offer them as connections. It is sent only when the person presses **Look for saved servers** in the extension. The answer holds names, kinds of server, addresses, ports, user names and start folders, and **never a password, a key or the name of a key file**: the structures that read the lists have no field for them, and the tests check that none reaches an answer. Only FTP, FTPS and SFTP entries are used; the others are counted and left out. Nothing is written to the files of the other programs. Limits: a list file of at most 16 MiB, 500 servers per program, 200 characters per text, 1,000 Cyberduck bookmark files. The message version stays `1`: an older helper answers that it does not know the request, and the extension then says that the helper is too old ([helper/docs/PROTOCOL.md](helper/docs/PROTOCOL.md#sites-helper-110-and-newer)).
- **14 new Go tests** (51 in all): sample lists in the formats of the three programs, a registry key made for the test on Windows, and a check that the words of a password or of a key file never reach an answer.

### Changed

- The release files are built with Go 1.27.3. `govulncheck` finds no known vulnerability that the program reaches. The files of 1.0.0, built with Go 1.27.0, did reach two advisories of the Go standard library ([helper/SECURITY.md](helper/SECURITY.md#known-limits)).
- To update, run the new program once as for the first install: it replaces the old copy and writes the registration again ([helper/docs/INSTALLING.md](helper/docs/INSTALLING.md)).

### Known limits

- The saved-server import was tried only with **sample lists** in the formats of the three programs (and with the Windows program run through its pipes on them). It was not run against a real installation of FileZilla, WinSCP or Cyberduck, and not on macOS or Linux ([docs/TESTING.md](docs/TESTING.md#what-is-not-tested-by-a-script)).
- The limits of 1.0.0 stay: only the Windows program has been run, and the programs are not code-signed or notarized.

## [1.0.0] - 2026-10-28

The first version of the helper program. It has been online as downloads at <https://rocuo.com/remote-files/ftp-helper/> since 2026-10-28, together with version 1.1.0 of the Chrome extension that uses it. Its source is in this repository.

### Added

- **FTP, FTPS and SFTP** for the Chrome extension Remote Files Browser FTP by Rocuo, through Chrome's native messaging. FTP uses explicit TLS when the server offers it (the extension can require TLS or turn it off), FTPS uses TLS from the first byte, and SFTP works with a password or a private key file. Passive mode with EPSV can be switched off for old servers.
- **The operations the extension needs, for all three kinds:** list a folder (in batches, so a big folder starts to show quickly), read a file (also from a byte position), write a file (an upload that is cancelled leaves nothing behind), make a folder, rename, delete (a folder with everything inside), and cancel at any moment. Several transfers can run at the same time.
- **Trust is asked, never assumed.** A certificate the computer does not trust and an SSH host key that is not on the list the extension sent are stopped with the SHA-256 fingerprint, and the person decides.
- **Install for the current user, without administrator rights:** `install`, `uninstall`, `status` and `version`. The program copies itself, and tells Chrome, Edge, Brave and Chromium where it is. Only the listed extensions can start it, and `--extension-id` adds a copy of the extension loaded from a folder.
- **Six programs:** Windows, macOS and Linux, on Intel/AMD and ARM. The helper is pure Go with no C code, so every system can be built from any system, and the same source built with the same Go version gives the same file.
- **Its own tests:** 37 Go tests, with FTP, FTP over TLS, SFTP and WebDAV test servers that live in memory and start with `go run ./testserver`.

### Known limits

- Only the Windows program has been run and tested. The macOS and Linux programs build and pass `go vet`, and have not been run ([docs/PLATFORMS.md](docs/PLATFORMS.md)).
- The programs are not code-signed or notarized, so Windows SmartScreen and macOS Gatekeeper may warn about a downloaded copy ([docs/VERIFYING-DOWNLOADS.md](docs/VERIFYING-DOWNLOADS.md)).
