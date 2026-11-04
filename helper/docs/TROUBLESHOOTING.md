# When something does not work

Start with the one command that shows the whole set-up:

```text
rocuo-files-helper status
```

(On Windows in PowerShell: `.\rocuo-files-helper.exe status | Out-String`, see [INSTALLING.md](INSTALLING.md#windows-where-does-the-output-go).) It says which browsers know the helper, which program Chrome would start and whether that program exists, and **which extension addresses are allowed**. Most problems are visible there.

## The extension cannot reach the helper

The extension turns Chrome's messages into screens of its own, each with the one fix for it. Find the title of the screen:

| The screen says | What it means | What to do |
| --- | --- | --- |
| **FTP, FTPS and SFTP need a small helper program**<br>"Chrome has not allowed the helper program yet." | The permission to talk to a helper is optional and has not been given. | Press **Allow the helper**. Chrome asks once: choose **Allow**. The page opens again by itself; your tabs stay. |
| **Open this page again to use the helper** | Chrome has given the permission, but gives the function that starts the helper only to pages that are opened *after* that. This page is older. | Press **Open this page again**. |
| **FTP, FTPS and SFTP need a small helper program**<br>"The helper program is not installed on this computer (Chrome could not find it)." | The permission is there, but Chrome finds no helper: it was never installed, or it was moved or deleted. | Install it: [INSTALLING.md](INSTALLING.md). Then press **Check again**. |
| **The helper does not know this copy of the extension** | The helper is installed, but its list of allowed extensions does not have the address of this copy. This is what happens with an extension loaded from a folder (**Load unpacked**): its address differs from the Chrome Web Store one. | Run the line the screen shows (`... install --extension-id THE-ADDRESS`). **Copy the line** puts it on the clipboard. Then **Check again**. |
| **The helper program stopped** | It started and ended: a crash, or a security program that ended it. | **Check again**. If it repeats, install the helper again and look at your security program's list of blocked programs. |
| **The helper program could not be started** | Chrome found the program but could not run it: a damaged copy, or a program that is blocked. | Install the helper again. Check that no security program blocks `rocuo-files-helper`. |
| **The helper program is out of date** | The helper speaks an older message version than the extension. | Install the newest helper (the same steps; the new copy replaces the old one). |
| **This extension is older than the helper program** | The helper is newer than the extension. | Update the extension: Settings (the gear) → **Check for updates**. Then **Check again**. |

### "chrome.runtime.connectNative is not a function"

This is the message an earlier build showed, and it is the same thing as the **Open this page again** case above. The cause:

> Chrome gives `chrome.runtime.connectNative` to an extension page **only if the `nativeMessaging` permission was already granted when the page was opened**. When you grant the permission later (in the question Chrome shows over the open page), the permission is true at once, but the page that is open **still has no such function**. Only a page opened afterwards has it.

This was measured with the real permission question in Chrome for Testing 149 and in 156: before the question the function is `undefined`; in a page that was opened before the permission it is still `undefined` after **Allow**; after the page is opened again (and in any new page) it is a function.

The extension now knows this. It asks for the permission, then opens the page again by itself and carries on with the address you had typed. If you still see this exact sentence somewhere, you are running an old copy of the extension: press **Reload** on its card in `chrome://extensions`, then open its page again.

### Chrome's own words, and what the extension makes of them

When Chrome cannot start the helper, it closes the connection with a sentence of its own. The extension turns each into one of the screens above:

| Chrome says | Screen |
| --- | --- |
| Access to native messaging requires nativeMessaging permission. | Allow the helper |
| Specified native messaging host not found. | not installed |
| Access to the specified native messaging host is forbidden. | does not know this copy of the extension |
| Failed to start native messaging host. | could not be started |
| Native host has exited. / Error when communicating with the native messaging host. | stopped |

### "forbidden": which address?

Chrome starts the helper only for the extensions on the list in the registration file. The list always has the Chrome Web Store address. A copy loaded from a folder has an address that depends **only on the full path of that folder**, so moving or renaming the folder gives a new address. The screen shows the address of the copy you are using and the exact line for it, for example:

```text
"%LOCALAPPDATA%\Rocuo\RemoteFilesHelper\rocuo-files-helper.exe" install --extension-id abcdefghijklmnopabcdefghijklmnop
```

The addresses you added earlier stay on the list ([INSTALLING.md](INSTALLING.md)), so moving a folder only adds one more; `--reset-extensions` clears the old ones.

## The helper starts, but the server says no

The helper turns what the network and the servers say into plain sentences. The common ones:

| Message | What to check |
| --- | --- |
| The server name could not be found. Check how it is spelled and that you are online. | The spelling of the address; your internet connection; for a name inside a company, the VPN. |
| The server refused the connection. Check the address and the port. | The port (FTP 21, FTPS 990, SFTP 22 are the usual ones) and that the server is running. |
| The server did not answer in time. | A firewall that drops the traffic, a wrong address, a server that is down. The connection attempt gives up after 20 seconds. |
| The server did not accept the user name or password. | The login. For FTP, an empty user name means "anonymous", which many servers refuse. |
| The server closed the connection (...). | The server's own words follow. A server may do this when it already serves too many connections: the helper opens up to 6 at once for FTP (a test server that allows only two is part of the tests), so close other FTP programs that use the same account. |
| The server says no (550): ... | The server refused that file or folder: it does not exist, or the account may not use it. |
| The connection to the server was lost (...). | The network dropped, or the server closed an idle connection. Open it again. A download that breaks, or that the server ends with an error, is reported as an error, so do not trust that file. |
| The server has no more room: ... | The disk or quota of the account is full. |

### Import saved servers: what the window says

The extension's start page has **Import your saved servers**. It asks the helper (1.1.0 or newer) to read the lists of FileZilla, WinSCP and Cyberduck. It never reads passwords: each imported connection asks for its password the first time you open it.

| The window says | What it means | What to do |
| --- | --- | --- |
| **The helper program is too old to read the saved servers of other programs.** | Your helper is older than 1.1.0. | Install the newest helper ([INSTALLING.md](INSTALLING.md); the new copy replaces the old one), then press **Look for saved servers** again. |
| **No saved servers were found.** | None of the usual places has a list for the user you are signed in as. The helper looks for FileZilla's `sitemanager.xml`, WinSCP's `WinSCP.ini` and registry sessions (Windows), and Cyberduck's bookmarks (Windows and macOS). | A portable copy that keeps its settings next to the program is not read. Add the server with **New connection** instead. |
| **FileZilla could not be read** (or WinSCP, Cyberduck), with a sentence | The list is there but is damaged, too large (more than 16 MiB) or cannot be opened by your account. | Open the program once and save its list again, or add the servers by hand. |
| **N saved entries are not FTP, FTPS or SFTP ... They are left out.** | WebDAV, cloud storage and SCP entries (and entries without a server) cannot be imported. | Add a WebDAV server with **New connection**. |
| **Already in your list.** | A connection with the same address is saved. | Nothing: it is not added twice. |
| **Your list is full (30 connections).** | The extension keeps at most 30. | Remove one first. |

### Folders do not open, or transfers hang (FTP)

FTP sends the files over a second connection that the server opens in a way ("passive mode") a strict firewall or an old server may not follow. In the connection form, turn on **Old server mode** (the `noEpsv` field): the helper then uses the older passive command. If a download stops receiving data, it ends with a time-out after 90 seconds.

### The identity question ("Do you trust this server?")

For FTPS, FTP with TLS and SFTP, a server that the computer cannot verify (a self-signed certificate, a new SSH key) makes the extension ask **Do you trust this server?** It shows the SHA-256 fingerprint (for SSH also the `SHA256:...` form of OpenSSH) and why the certificate is not trusted. Compare it with what the server's owner publishes, **then** choose **Trust and connect**. The extension keeps only the fingerprint. A changed fingerprint is asked again, as **This server has changed**, with a stronger warning: that can be an attack, or a server that was re-installed. Settings has **Forget trusted servers** to take every answer back.

### FTP without encryption

With **Use TLS when the server offers it**, the helper tries explicit TLS and continues in plain text only if the server answers that it has no TLS. The bar of the folder then says **Not encrypted**. To stop instead of continuing, choose **Require TLS**.

### SFTP with a key file

| Message | Meaning |
| --- | --- |
| Enter the user name for this server. | SFTP needs a user name. |
| Enter the password for this server, or choose a key file. | Neither a password nor a key was given. |
| The key file could not be read: NAME | The path is wrong, or the file cannot be read by your account. |
| This key is protected by a passphrase. Enter the passphrase as the password. | Type the passphrase of the key in the password field. |
| The passphrase of the key is not right. | Retype it. |
| This file is not a private key that can be used (...). Use an OpenSSH or PEM key. | A `.pub` file or another format was given. |

## Windows

- **"Windows protected your PC" (SmartScreen)** can appear for a downloaded copy, because the program is not code-signed yet. **More info → Run anyway** starts it. Compare the checksum first ([INSTALLING.md](INSTALLING.md#checking-a-file)).
- **A security program** can block or end a program that is not signed or not well known. If the helper "stopped" screen repeats, add `rocuo-files-helper.exe` in `%LOCALAPPDATA%\Rocuo\RemoteFilesHelper\` to the allowed list.
- **You can delete the downloaded file** after the install: Chrome starts the copy in `%LOCALAPPDATA%\Rocuo\RemoteFilesHelper\`, not the download (unless you installed with `--no-copy`).
- **The prompt came back at once and the output is late or missing** in PowerShell: it does not wait for a GUI program. Use `| Out-String` ([INSTALLING.md](INSTALLING.md#windows-where-does-the-output-go)).

## Still stuck

Collect these, none of which contains a password:

1. The words on the extension's screen.
2. The output of `rocuo-files-helper status`.
3. The Chrome version (`chrome://version`) and the helper version (`rocuo-files-helper version`).

and [open an issue](https://github.com/Maool2122/remote-files-browser/issues/new/choose) (a bug report or a question). Everything in an issue is public, so do not put passwords, keys, private server addresses or files from your server in it.
