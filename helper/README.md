# Remote Files Browser helper

A small, open source program that lets the Chrome extension **Remote Files Browser FTP by Rocuo** open **FTP, FTPS and SFTP** servers.

A Chrome extension can only make web requests. FTP, FTPS and SFTP need other kinds of network connection, and Chrome does not give those to an extension. So this program, running on your own computer, opens the connection, and the extension talks to it through Chrome's [native messaging](https://developer.chrome.com/docs/extensions/develop/concepts/native-messaging).

This folder is the whole program: the source, its tests, its test servers and its documents. Nothing in it needs the rest of the extension to be read, built or unit-tested. (The extension is closed source: the tests that run it in a real Chrome belong to it and are not in this repository; [docs/BUILDING.md](docs/BUILDING.md) says which.)

- Version `1.1.0`, message version (protocol) `1`.
- Written in Go. One file per system, about 7 MB on Windows, no other software to install.
- Licence: [MIT](LICENSE). The libraries it uses have their own licences: [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md).

## What it is, and what it is not

| It is | It is not |
| --- | --- |
| A program that Chrome starts when the extension asks for an FTP, FTPS or SFTP connection, and that ends when the extension page closes or Chrome stops. | A service, a daemon or a startup item. It does not start with the computer and runs nothing in the background. |
| A client: it connects **out** to the server you typed in. | A server. It listens on no port and accepts no connection from anyone. |
| Run as you, without administrator rights, and it writes only its own copy and its registration. | Something that needs an administrator, a driver or a change of the system. |
| Silent towards Rocuo: it makes no connection except to the servers you open, sends no statistics and checks for no updates by itself. | A tracker. There is no account, no analytics and no server of Rocuo behind it. |

## How it fits together

```
 Chrome extension page  <--- native messaging (JSON on stdin/stdout) --->  rocuo-files-helper  <--- FTP / FTPS / SFTP --->  your server
```

1. The extension page asks Chrome to start the program named `com.rocuo.remote_files_helper`.
2. Chrome looks the name up in a small registration file (a "native messaging host manifest") that this program wrote when it was installed. **Chrome starts the program only for the extensions listed in that file.**
3. Chrome starts the program with the address of the extension as its only argument, and connects its standard input and output to the extension page.
4. The two talk in JSON messages ([PROTOCOL.md](docs/PROTOCOL.md)). The program opens the connection to the server, lists folders, reads and writes files, and answers.
5. When the page closes, Chrome closes the pipe, the program ends the connections and exits.

More: [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md).

## Install

You rarely need this page: the extension shows these steps itself, with the right file for your system, when you open an FTP, FTPS or SFTP address.

1. Get the file for your system from the [Releases page of this repository](https://github.com/Maool2122/remote-files-browser/releases) (built by GitHub from the tagged source, with `SHA256SUMS.txt` and a build attestation) or from the download page, <https://rocuo.com/remote-files/ftp-helper/>. Check the file against `SHA256SUMS.txt` ([how](../docs/VERIFYING-DOWNLOADS.md)). Or build it yourself from this folder with one command ([docs/BUILDING.md](docs/BUILDING.md)).
2. Open the file once (double-click on Windows). It copies itself for your user and tells Chrome where it is. No administrator is needed, and Chrome needs no restart.
3. Go back to the extension and press **Check again**.

On the command line the same program does more (the file name is `rocuo-files-helper.exe` on Windows, `rocuo-files-helper` elsewhere):

```text
rocuo-files-helper                       double-click: install for the current user
rocuo-files-helper install               same, with options:
      --extension-id ID[,ID...]            let one more copy of the extension use the helper (see below)
      --reset-extensions                   forget the addresses of an earlier setup first
      --dir FOLDER                         install into FOLDER instead of the usual place
      --no-copy                            register this program where it is, without copying it
      --quiet                              print nothing but errors
rocuo-files-helper uninstall [--dir FOLDER] [--quiet]
                                         remove the registration and the copy
rocuo-files-helper status                show what is installed and which extensions may use it
rocuo-files-helper version               print the version
```

Where it goes, and how Chrome is told:

| System | The program is copied to | Chrome is told where it is |
| --- | --- | --- |
| Windows | `%LOCALAPPDATA%\Rocuo\RemoteFilesHelper\` | One registry value of the current user per browser (`HKEY_CURRENT_USER\Software\...\NativeMessagingHosts\com.rocuo.remote_files_helper`) that points to the registration file in that folder. Chrome, Edge, Brave and Chromium. |
| macOS | `~/Library/Application Support/Rocuo/RemoteFilesHelper/` | A copy of the registration file in the `NativeMessagingHosts` folder of each browser under `~/Library/Application Support/`. |
| Linux | `$XDG_DATA_HOME/rocuo/remote-files-helper/` (else `~/.local/share/...`) | A copy of the registration file in `~/.config/<browser>/NativeMessagingHosts/` (`$XDG_CONFIG_HOME` when it is set). |

Details and the silent install: [docs/INSTALLING.md](docs/INSTALLING.md).

### A copy of the extension loaded from a folder

The registration file lists the extensions that may start the helper. By default that is the Chrome Web Store copy only (`chrome-extension://pckgpblcplbjffapabndpkhnhkioihjd/`). A copy that you load from a folder ("Load unpacked", for development) has an address of its own, and Chrome refuses to start the helper for it ("Access to the specified native messaging host is forbidden"). Add it:

```text
rocuo-files-helper install --extension-id THE-ADDRESS-OF-YOUR-COPY
```

The address is the long ID on the card of the extension in `chrome://extensions`. The extension also shows the address and this exact line on its own screen when it meets this problem. `install --extension-id` keeps the addresses of earlier setups (add `--reset-extensions` to drop them). `status` lists them all.

## What it can do

- **FTP.** TLS is used when the server offers it (explicit TLS, then plain if the server has none). The extension can require TLS (never falls back) or turn it off. The extension shows **Not encrypted** when the link is plain. Passive mode with EPSV, switchable off for old servers and strict firewalls. An empty user name means anonymous.
- **FTPS.** TLS from the first byte (port 990 by default).
- **SFTP.** SSH with a password or a private key file. The key file is read by this program only, never by the extension. Keep-alive every 30 seconds.
- **For every type:** list a folder (in batches of 500 names, so a big folder starts to show quickly), read and write files, resume a download from a byte position, make folders, rename, delete (folders with everything inside; on SFTP a link is removed, never followed), cancel at any moment.
- **Trust.** A certificate that the computer trusts passes. Any other certificate, and any SSH host key that is not on the list the extension sent, is **stopped** with a `trust` error that carries the SHA-256 fingerprint. The extension shows it and asks you. The program never decides alone and keeps no list of its own.
- **Passwords.** The extension sends the password with the `open` request. The program keeps it in memory for that connection only and writes it nowhere.
- **Saved servers of other programs (since 1.1.0).** When the extension asks, and only then (the person presses a button in the extension), the program reads the lists that **FileZilla**, **WinSCP** and **Cyberduck** keep for the same person on the same computer, and hands back names, server addresses, ports, user names and start folders, so the extension can offer them as connections ([docs/PROTOCOL.md](docs/PROTOCOL.md#sites-helper-110-and-newer), [sites.go](sites.go)). It **never reads a password or a key file** from them (the code has no field for either), changes nothing in those programs, and looks nowhere else. FTP, FTPS and SFTP entries only; WebDAV, cloud storage and SCP entries are counted and left out.

## Safety, in short

- Only the listed extensions can start it (Chrome checks that, not this program).
- It listens on no port and writes no file except its own copy and its registration.
- It talks to the servers you open and to no one else.
- It reads a local file only for a reason the person gave: the private key file whose path they typed, and, when they press **Import saved servers** in the extension, the saved-server lists named above (never their passwords or key files).
- It never decides about trust: a new or changed certificate or host key goes back to the person.

The full picture and how to report a problem: [SECURITY.md](SECURITY.md).

## Build and test

You need [Go](https://go.dev/dl/) 1.26 or newer, and nothing else.

```text
go vet ./...
go test ./...                       the tests start their own FTP, FTPS and SFTP servers on this computer
node build.mjs                      build for Windows, macOS and Linux into dist/ (with SHA256SUMS.txt)
go build -o rocuo-files-helper .    build for this computer only
```

`go test` never connects to an outside server: it starts local servers on random ports ([testservers/](testservers)). More: [docs/BUILDING.md](docs/BUILDING.md).

## Documents

| Document | What is in it |
| --- | --- |
| [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) | How the program works, file by file, and how a request travels. |
| [docs/PROTOCOL.md](docs/PROTOCOL.md) | Every message between the extension and the program. |
| [docs/INSTALLING.md](docs/INSTALLING.md) | Install, remove, check; every system; silent install; what is written where. |
| [docs/TROUBLESHOOTING.md](docs/TROUBLESHOOTING.md) | Every message the extension can show about the helper, and what to do. |
| [docs/BUILDING.md](docs/BUILDING.md) | Build, test, and what has been run on which system. |
| [SECURITY.md](SECURITY.md) | The safety design, what it protects against, how to report a problem. |
| [CONTRIBUTING.md](CONTRIBUTING.md) | How to change it. |
| [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md) | Libraries and their licences. |

## Status

Built and tested on Windows (unit tests, the extension in a real Chrome, a real FTP server over explicit TLS). The programs for macOS and Linux build and pass `go vet`, but **have not been run**: treat them as untested until someone does ([how you can help](../docs/PLATFORMS.md)). The programs are not code-signed, so Windows SmartScreen and macOS Gatekeeper warn about a downloaded copy.
