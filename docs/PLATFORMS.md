# Platforms: what has been run, and how you can help

The **helper program** is a program per system, and that is where the gaps are: it has been run and tested on Windows only. This page says plainly what has been run and what has not, and how you can close a gap. It is the page for you if you have a Mac, a Linux computer, a Windows 10 PC or a Windows on ARM computer.

- [At a glance](#at-a-glance)
- [What has been checked](#what-has-been-checked)
- [How a system counts as tested](#how-a-system-counts-as-tested)
- [Try the helper on macOS or Linux](#try-the-helper-on-macos-or-linux)
- [Other ways to help](#other-ways-to-help)

## At a glance

| System | Helper (FTP, FTPS, SFTP) |
| --- | --- |
| Windows 11, 64-bit (Intel/AMD) | **Run and tested** |
| Windows 10, 64-bit | The same program, **not tested separately** |
| Windows on ARM | Built (`windows-arm64`), **never run** |
| macOS, Apple silicon and Intel | Built, **never run** by the maintainers |
| Linux, 64-bit Intel/AMD and ARM | Built, **never run** by the maintainers |
| ChromeOS | There is no build: the helper needs a program on the computer |

"Built" means that the program compiles for that system (and `go vet`, which also type-checks the code of that system, is clean for Windows, Linux and macOS). "Never run" means that the maintainers have not started it there: not the registration code (`helper/install_other.go`) and not a start from Chrome. The Go tests do run on GitHub's Linux and macOS computers (below), which is useful, but it is not the same as a person using the helper with Chrome. Until that changes, treat macOS and Linux as untested, whatever the downloads are called.

The helper programs are **not signed or notarized** on any system yet ([VERIFYING-DOWNLOADS.md](VERIFYING-DOWNLOADS.md)).

The **import of saved servers** (helper 1.1.0, [PROTOCOL.md](../helper/docs/PROTOCOL.md#sites-helper-110-and-newer)) is a separate thing to try. It reads the lists that FileZilla (every system), Cyberduck (Windows and macOS) and WinSCP (Windows) keep for the person. It was tried with sample lists, on Windows only, and has **never been run against a real installation** of those programs on any system.

## What has been checked

- **Windows 11, by the maintainers:** `go vet` and the Go tests of the helper ([TESTING.md](TESTING.md#the-last-full-run)), and, for helper 1.0.0 (2026-10-28), the maintainers' own tests of the extension with this helper in a real Chrome against FTP, FTPS, SFTP and WebDAV test servers. The extension is closed source, so those tests are not in this repository, and they were not repeated for 1.1.0.
- **Linux and macOS, by GitHub Actions:** the workflow [`ci.yml`](../.github/workflows/ci.yml) runs the helper's `go vet` and `go test` on a Linux and a macOS computer as **experimental** jobs (a failure there does not turn the whole run red). Look at the latest runs in the Actions tab of the repository to see whether they are green. Even a green run proves only that the tests pass there; it does not replace starting the helper from a real Chrome on a real computer.
- **All systems, the builds:** `node helper/build.mjs` builds the six programs from any computer (the helper is pure Go with no C code), and CI does that on every change.

## How a system counts as tested

We change the table above from "never run" to "tested" when **at least two people** have reported the full list below on that system (and on different computers), or one of the maintainers has done it on a computer of their own:

1. `go test ./...` passes in `helper/` (needs Go, [helper/docs/BUILDING.md](../helper/docs/BUILDING.md#what-you-need)).
2. The downloaded or built program installs (`install`), `status` shows it registered for the browser in use, and **Chrome starts it** (the extension's **Test the helper** in Settings answers).
3. One connection of each type works against a test server: FTP, FTP over TLS (FTPS) and SFTP. That means: list a folder, download a file, upload a file, make a folder, rename, delete.
4. `uninstall` removes everything it should.
5. The report says the system, the processor, the browser and its version, and the version of the helper (`version`).

## Try the helper on macOS or Linux

You need the extension (free, from the [Chrome Web Store](https://chromewebstore.google.com/detail/remote-files-browser-ftp/pckgpblcplbjffapabndpkhnhkioihjd); it needs Chrome 110 or newer) and the helper for your system. Use a computer where you do not mind a test.

1. **Get the program.** From the [Releases](https://github.com/Maool2122/remote-files-browser/releases) of this repository, and [check it](VERIFYING-DOWNLOADS.md). Or build it: `node helper/build.mjs darwin` or `node helper/build.mjs linux`. The names are `rocuo-files-helper-darwin-arm64`, `-darwin-amd64`, `-linux-amd64` and `-linux-arm64`.
2. **Allow it to run.** A downloaded file is not executable: `chmod +x rocuo-files-helper-...`. On macOS, Gatekeeper may stop an unsigned program that came from a browser: control-click the file and choose **Open** once, or remove the quarantine mark yourself (`xattr -d com.apple.quarantine FILE`) if you checked the file.
3. **Install it** in a terminal: `./rocuo-files-helper-... install`, then `./rocuo-files-helper-... status`. `status` shows where it was put and which browsers it told (the places are in [helper/docs/INSTALLING.md](../helper/docs/INSTALLING.md)).
4. **Start a test server** in another terminal (needs Go): `cd helper && go run ./testserver`. It prints one line of JSON and then keeps running (stop it with Ctrl+C). The line holds the made-up login (`"user":"alice"`, `"password":"secret"`) and the port of each server, which are all on your own computer (`127.0.0.1`): `ftp` (plain FTP), `ftpTls` (FTP that offers TLS), `ftps` (FTP with TLS from the first byte), `sftp`, and `webdav`.
5. **Use it from the extension.** Open `ftp://alice@127.0.0.1:PORT` (with the port of `ftp`), press **Allow the helper**, then **Check again**. Do the list from [How a system counts as tested](#how-a-system-counts-as-tested) for the three kinds: `ftp://` with the port of `ftpTls` (and of `ftp`), `ftps://` with the port of `ftps`, and `sftp://` with the port of `sftp`. The extension asks you to trust the certificate or the host key of the test server: that is the trust question working. Give the password `secret`.
6. **Report** with the form [Platform report](https://github.com/Maool2122/remote-files-browser/issues/new?template=platform_report.yml): what worked, what did not, and the output of `status` and `version` (check that it shows no passwords or private paths you do not want to share).
7. **Remove it** with `./rocuo-files-helper-... uninstall`.

If something fails, [helper/docs/TROUBLESHOOTING.md](../helper/docs/TROUBLESHOOTING.md) lists what each message means. A failure is a useful report too.

## Other ways to help

- **Run the helper's Go tests** on macOS, Linux, Windows 10 or Windows on ARM and send the result: `cd helper && go test -count=1 ./...`. This is the quickest useful report; it takes a minute.
- **Check the other browsers.** The helper registers itself for Chrome, Edge, Brave and Chromium (the places are in [helper/docs/INSTALLING.md](../helper/docs/INSTALLING.md)). Tell us which of them really start it on your system.
- **Try the import of saved servers** if you use FileZilla, WinSCP or Cyberduck. Install the helper, open the start page of the extension, press **Import your saved servers**, then **Look for saved servers**, and tell us which of your servers were found and whether the name, the address, the port, the user and the folder were right. Describe an entry that was read wrong; never send the list file, because it can hold passwords.
- **Bring a server that answers in an odd way.** Every FTP, FTPS and SFTP server differs a little. A failing case, described without private data (what the server answered), becomes a test in `helper/` and keeps the next version from breaking on it.
- **Signing and notarization.** A signed Windows program (Authenticode) and a signed and notarized Mac program need the publisher's certificates. The code can be prepared by anyone (a signing step in the release workflow that reads its secrets from a protected GitHub environment); the secrets stay with the publisher. Open an issue to discuss.
- **A package for your system.** A Homebrew formula, a Scoop or winget manifest or a distribution package that installs the helper from the release files and their checksums is welcome, as a separate repository or a pull request that adds the recipe.
- **Documents.** Tell us where this page, [helper/docs/INSTALLING.md](../helper/docs/INSTALLING.md) or the troubleshooting page said something that was not true on your system.

Thank you. Every report, also "it worked at once", helps the next person decide whether to trust it.
