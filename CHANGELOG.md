# Changelog

All notable changes to the helper program are written here. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the version numbers are those of the helper (`version` in [helper/server.go](helper/server.go)). The message version between the extension and the helper (`protocolVersion`) is a separate number that changes rarely ([helper/docs/PROTOCOL.md](helper/docs/PROTOCOL.md#versions)). Dates are the days a version was released.

## [Unreleased]

Nothing yet.

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
