# Why the helper is open source

**Short answer: the helper is the part that runs outside the browser, on your computer, with your own rights and with your passwords passing through it. That is the part you should be able to check, so it is open. Anyone can read it, use it, test it on their own system and build something with it.**

The Chrome extension **Remote Files Browser FTP by Rocuo** is a free product with closed source. This repository holds only the helper.

- [1. What is open and what is not](#1-what-is-open-and-what-is-not)
- [2. Why the helper, and not the extension](#2-why-the-helper-and-not-the-extension)
- [3. What you can check yourself](#3-what-you-can-check-yourself)
- [4. What you cannot check here, and what limits it](#4-what-you-cannot-check-here-and-what-limits-it)
- [5. Anyone can use it](#5-anyone-can-use-it)
- [6. You can build something with what is in it](#6-you-can-build-something-with-what-is-in-it)
- [7. It can get better, on more systems](#7-it-can-get-better-on-more-systems)
- [8. It does not depend on us](#8-it-does-not-depend-on-us)
- [9. What open source does not mean here](#9-what-open-source-does-not-mean-here)
- [10. If you want to make your own thing from it](#10-if-you-want-to-make-your-own-thing-from-it)

## 1. What is open and what is not

| Open (MIT licence), in this repository | Not in this repository |
| --- | --- |
| The **helper program** `rocuo-files-helper`: the source in Go, its tests, the test servers, the build script, the documents of the helper, and the workflows that build and check it. | The **Chrome extension** (the screens you see in Chrome). It is free to use, from the [Chrome Web Store](https://chromewebstore.google.com/detail/remote-files-browser-ftp/pckgpblcplbjffapabndpkhnhkioihjd), and its source is closed. |
| The **protocol**: every message between an extension and the helper, written down in [helper/docs/PROTOCOL.md](../helper/docs/PROTOCOL.md). | The website rocuo.com and the page of the Chrome Web Store. |
| The documents of this repository. | The **names** "Rocuo", "Remote Files Browser" and "Remote Files Browser FTP by Rocuo", the Rocuo logo, the icon and the pictures. They are not under the MIT licence: [TRADEMARKS.md](../TRADEMARKS.md). |

The libraries inside the helper keep their own licences: [helper/THIRD_PARTY_NOTICES.md](../helper/THIRD_PARTY_NOTICES.md).

## 2. Why the helper, and not the extension

Chrome keeps an extension in a sandbox: it cannot open an FTP or SFTP connection, and it cannot start a program. To get FTP, FTPS and SFTP, the extension asks Chrome for one extra permission and talks to a small program that you install once. That program is different from the rest of the product in three ways:

- **It runs outside the browser's sandbox**, on your computer, with your own rights.
- **Your passwords and files pass through it.** It opens the connection to your server and carries what you send and receive.
- **You install it yourself**, from a file. A file is only as trustworthy as what it was built from.

For a program like that, "trust us" is not enough. With the source open, you (or anyone who reads code) can answer "what does it really do?" from the code and not from a promise, and you can check that the file you got was built from that code ([VERIFYING-DOWNLOADS.md](VERIFYING-DOWNLOADS.md)).

## 3. What you can check yourself

- **The helper has no web client and writes no log.** [helper/SECURITY.md](../helper/SECURITY.md) lists every claim with the file where you can verify it. For example, `go list -deps .` in `helper/` shows no `net/http`.
- **It listens on no port and runs no service.** It is a client: it connects out to the server you opened, and ends when Chrome closes the pipe.
- **It makes no connection except to the servers you open.** There is no server of Rocuo behind it, no statistics and no update check.
- **Chrome starts it only for the extensions on its list.** The list is written when the helper is installed, and the program that decides is Chrome, not the helper ([helper/docs/INSTALLING.md](../helper/docs/INSTALLING.md)).
- **It never decides about trust alone.** A new or changed TLS certificate or SSH host key is stopped, and goes back to the person with its SHA-256 fingerprint ([helper/docs/PROTOCOL.md](../helper/docs/PROTOCOL.md)).
- **You can compare what you download with what the source builds.** The helper builds to the same bytes from the same source and the same Go version, so a download can be checked against your own build. Releases made here also carry a build attestation from GitHub ([VERIFYING-DOWNLOADS.md](VERIFYING-DOWNLOADS.md)).
- **Researchers can read it and report problems** through a private channel: [SECURITY.md](../SECURITY.md).

## 4. What you cannot check here, and what limits it

The extension is closed source, so its code cannot be read in this repository. That is a real limit, and it is better said plainly:

- The helper does what **the extension that Chrome lets start it** asks. Trust in the whole is therefore also trust in the extension.
- What narrows it: the helper can only be asked for the operations that [the protocol](../helper/docs/PROTOCOL.md) lists, it stops at every trust question and hands it back to the person, and Chrome starts it only for the listed extensions.
- What the extension itself does is described in its [privacy policy](https://rocuo.com/remote-files/privacy-policy/) and in the permission list that Chrome shows in the store and in `chrome://extensions`.

If you need to read the extension's code too, this project cannot give you that. The helper, which is the part with the most reach, it can.

## 5. Anyone can use it

The code is under the [MIT licence](../LICENSE): you may use it for any purpose, at home or at work, change it and include it in your own work, as long as you keep the licence notice. You need nobody's permission.

## 6. You can build something with what is in it

Some of what is in here took time to find out. If you build an extension, a native-messaging program or a file tool, these parts may save you days:

| What | Where | Why it may help you |
| --- | --- | --- |
| A **native messaging helper in Go**: the framing (4-byte length and JSON), requests with flow control, streaming of large files in pieces, cancelling at any moment | [`helper/`](../helper), [PROTOCOL.md](../helper/docs/PROTOCOL.md), [ARCHITECTURE.md](../helper/docs/ARCHITECTURE.md) | A complete, tested example of the pattern that every extension needs when it must do what a browser cannot. |
| **Registering a native helper for one user, on three systems**, without administrator rights: the Windows registry, the macOS and Linux manifest folders, only for chosen extension IDs, with a status command | [`helper/install.go`](../helper/install.go), [`install_windows.go`](../helper/install_windows.go), [`install_other.go`](../helper/install_other.go), [INSTALLING.md](../helper/docs/INSTALLING.md) | The part that is usually done badly or not at all. |
| **FTP, FTPS and SFTP behind one set of operations**: list, stat, read, write, make a folder, rename, delete, with the same errors for every protocol | [`helper/backend.go`](../helper/backend.go), [`ftp.go`](../helper/ftp.go), [`sftp.go`](../helper/sftp.go) | One interface for protocols that answer very differently, with the odd cases of real servers handled. |
| **Trust on first use** with SHA-256 fingerprints of TLS certificates and SSH host keys, and a stronger warning when one changes | [`helper/trust.go`](../helper/trust.go), [`errors.go`](../helper/errors.go) | The model that good FTP and SSH programs use, in very little code. |
| **Test servers**: real FTP, FTP with explicit and implicit TLS, SFTP and WebDAV servers that live in memory and start with one command (`go run ./testserver`) | [`helper/testservers/`](../helper/testservers), [`helper/testserver/`](../helper/testserver), [TESTING.md](TESTING.md) | Test any FTP or SFTP client against known servers without an account and without a network. |
| **Reproducible builds of a Go program** for six systems from one computer, with checksums and a GitHub build attestation | [`helper/build.mjs`](../helper/build.mjs), [`.github/workflows/release.yml`](../.github/workflows/release.yml), [VERIFYING-DOWNLOADS.md](VERIFYING-DOWNLOADS.md) | Lets the people who download your program check it. |

## 7. It can get better, on more systems

Today the maintainers have run the helper only on Windows. The programs for macOS and Linux build and pass `go vet`, and GitHub Actions runs their tests, but nobody has used them on a real Mac or Linux PC with Chrome yet. Open source is how that gets fixed: a person with that system can run the tests, find what is different and send the fix. [PLATFORMS.md](PLATFORMS.md) lists exactly what is needed.

The same goes for servers. Every FTP, FTPS and SFTP server answers a little differently, and people who run an odd one can bring it as a test case instead of a complaint.

## 8. It does not depend on us

If Rocuo stopped tomorrow, the helper's code, the protocol and the build instructions would still be here, and the helper has no server of ours that must keep running. The download page on rocuo.com is a convenience; the [Releases](https://github.com/Maool2122/remote-files-browser/releases) of this repository carry the same kind of files.

## 9. What open source does not mean here

- **It is not a promise of support or of dates.** This is a small project. Issues and pull requests are read by people, and the maintainers decide what goes in: [CONTRIBUTING.md](../CONTRIBUTING.md) says what fits and what does not (nothing that tracks, nothing that contacts a server the person did not open, nothing that runs remote code).
- **The helper in a release is not a different program.** The files of a release are built from the tagged source by the workflow in this repository ([RELEASING.md](RELEASING.md)).
- **The extension is not open.** Opening the helper does not open the extension, and nothing here gives a right to its code, its name or its pictures.
- **Security reports are private** until a fix is out: [SECURITY.md](../SECURITY.md).

## 10. If you want to make your own thing from it

You may fork it, change it and publish your own version. Please follow the licence (keep the notice) and the [rules about names and pictures](../TRADEMARKS.md): your version needs **its own name, icon and pictures**, and must not say or suggest that it is made or approved by Rocuo.

What a fork that ships its own helper has to change:

| What | Where |
| --- | --- |
| The name by which Chrome finds the helper | `hostName` in `helper/install.go` |
| Which extension may start the helper | `publishedExtensionIDs` in `helper/install.go` (the ID of **your own** extension) |
| Names of the files and folders of the helper | `helper/install.go` (the install folders), `helper/build.mjs` (the file names) |
| The module name and the texts that name the project | `helper/go.mod`, the documents |

The extension that talks to your helper is yours to write: the protocol is in [helper/docs/PROTOCOL.md](../helper/docs/PROTOCOL.md).
