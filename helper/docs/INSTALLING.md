# Installing, checking and removing the helper

The helper installs for **one user**, without administrator rights, and changes nothing outside that user's own folders and registry branch.

## Getting the program

**Download it** from the [Releases page of this repository](https://github.com/Maool2122/remote-files-browser/releases) or from <https://rocuo.com/remote-files/ftp-helper/>. Both have the programs for Windows, macOS and Linux (two processor types each), named `rocuo-files-helper-<system>-<processor>` (with `.exe` on Windows), and the file `SHA256SUMS.txt` to check them ([how to check](../../docs/VERIFYING-DOWNLOADS.md)); the page on the website also has a ZIP file with this whole folder.

Or **build it from this folder**: one command, [BUILDING.md](BUILDING.md). It writes the same programs to `dist/`.

Take the file for your system and processor. On Windows, `amd64` is a normal PC and `arm64` is an ARM PC.

## What the install does

1. Copies the program into a folder of the user (the table below).
2. Writes the registration file `com.rocuo.remote_files_helper.json` into the same folder. It says the name of the helper, where the program is, and **which extensions may start it**. The Chrome Web Store copy of the extension is always on the list.
3. Tells each supported browser where that file is.

| System | The program and the registration file go to | The browsers are told |
| --- | --- | --- |
| Windows | `%LOCALAPPDATA%\Rocuo\RemoteFilesHelper\` | A registry value (the default value of the key) of the current user, one key per browser. It holds the path of the registration file. |
| macOS | `~/Library/Application Support/Rocuo/RemoteFilesHelper/` | A copy of the registration file in the browser's `NativeMessagingHosts` folder. |
| Linux | `$XDG_DATA_HOME/rocuo/remote-files-helper/`, else `~/.local/share/rocuo/remote-files-helper/` | A copy of the registration file in the browser's `NativeMessagingHosts` folder. |

The four browsers that are set up: **Google Chrome, Microsoft Edge, Brave and Chromium**. Other Chromium-based browsers read their own places and are not set up.

### The registry keys (Windows)

All under `HKEY_CURRENT_USER`, each with the path of the registration file as its default value:

```text
Software\Google\Chrome\NativeMessagingHosts\com.rocuo.remote_files_helper
Software\Microsoft\Edge\NativeMessagingHosts\com.rocuo.remote_files_helper
Software\BraveSoftware\Brave-Browser\NativeMessagingHosts\com.rocuo.remote_files_helper
Software\Chromium\NativeMessagingHosts\com.rocuo.remote_files_helper
```

### The folders (macOS and Linux)

```text
macOS   ~/Library/Application Support/{Google/Chrome, Microsoft Edge, BraveSoftware/Brave-Browser, Chromium}/NativeMessagingHosts/
Linux   ~/.config/{google-chrome, microsoft-edge, BraveSoftware/Brave-Browser, chromium}/NativeMessagingHosts/      ($XDG_CONFIG_HOME instead of ~/.config when it is set)
```

## For a person

**Windows:** double-click the file once. A message says that the helper is installed. In the extension, press **Check again**. Nothing else.

**macOS and Linux:** the file has to be allowed to run first (`chmod +x rocuo-files-helper-...`), and the setup prints its message in the terminal, so start it there: `./rocuo-files-helper-... install`. These two systems are built but have not been run yet (see [BUILDING.md](BUILDING.md)).

## On the command line

The file is `rocuo-files-helper.exe` on Windows and `rocuo-files-helper` elsewhere.

```text
rocuo-files-helper install                  same as the double-click
rocuo-files-helper install --quiet          print nothing but errors (for a script)
rocuo-files-helper install --extension-id ID[,ID...]
rocuo-files-helper install --reset-extensions [--extension-id ID]
rocuo-files-helper install --dir FOLDER     install into another folder
rocuo-files-helper install --no-copy        register this program where it is
rocuo-files-helper status
rocuo-files-helper uninstall [--dir FOLDER] [--quiet]
rocuo-files-helper version
```

`--extension-id` takes the address of one more copy of the extension (32 letters from `a` to `p`), for example one loaded from a folder with **Load unpacked**. Give several separated by commas, or repeat the option. **The addresses of an earlier setup are kept**: an `install` adds to the list. `--reset-extensions` forgets the earlier extra addresses first (the Chrome Web Store address always stays).

Running `install` again is safe: it updates the copy (a copy that is running is renamed out of the way and removed by a later install) and writes the registration again.

### `status`

```text
Remote Files Browser helper 1.0.0 (windows/amd64)
This program: C:\Users\you\Downloads\rocuo-files-helper-windows-amd64.exe
Registered for:
  Google Chrome -> C:\Users\you\AppData\Local\Rocuo\RemoteFilesHelper\com.rocuo.remote_files_helper.json
  Microsoft Edge -> C:\Users\you\AppData\Local\Rocuo\RemoteFilesHelper\com.rocuo.remote_files_helper.json
  Brave -> ...
  Chromium -> ...
Chrome starts: C:\Users\you\AppData\Local\Rocuo\RemoteFilesHelper\rocuo-files-helper.exe (found)
Extensions that may use it:
  pckgpblcplbjffapabndpkhnhkioihjd (Chrome Web Store)
  abcdefghijklmnopabcdefghijklmnop (added with --extension-id)
```

Read it like this:

- **Registered for** is what each browser is told. A browser that is not listed does not know the helper. When there is nothing at all, the line says `nothing yet` and the other lines are left out.
- **Chrome starts** is the program that Chrome would start. If it says `NOT FOUND: set the helper up again`, the registration points at a program that is gone: install again.
- **Extensions that may use it** is the list Chrome checks. If the address of your copy of the extension is not here, Chrome refuses to start the helper for it ("Access to the specified native messaging host is forbidden"): add it with `install --extension-id`.

The extension shows the address of the copy it runs in, with the exact line to run, when it meets this problem. You can also read it on the card of the extension in `chrome://extensions` (turn on **Developer mode**).

### Windows: where does the output go?

The Windows program is a GUI program (it has no console window of its own, so that nothing flashes up when Chrome starts it). In a terminal it still prints there. Two things to know:

- **PowerShell does not wait for a GUI program.** Run as a plain command, the prompt comes back at once and the output may arrive late or in the wrong order. Send the output through a pipe and PowerShell waits for it, in order: `.\rocuo-files-helper.exe status | Out-String`.
- **Without any terminal** (the Run window with `Win` + `R`, a shortcut, the Explorer) there is nowhere to print, so the result is shown in a **message box**. `--quiet` hides the success message and still shows an error.

## Removing it

```text
rocuo-files-helper uninstall
```

removes the registration (the registry keys or the files in the browsers' folders), the registration file and the copy. A copy that is running right now cannot be deleted on Windows: it is renamed `*.old-<number>` and the folder can be deleted by hand. In the extension, **Remove the permission** in Settings takes back Chrome's permission to talk to a helper.

### Removing it by hand

If the program is gone, remove the same things yourself:

- **Windows:** delete the four registry keys above (`reg delete "HKCU\Software\Google\Chrome\NativeMessagingHosts\com.rocuo.remote_files_helper" /f`, and the same for the others) and the folder `%LOCALAPPDATA%\Rocuo\RemoteFilesHelper`.
- **macOS and Linux:** delete the `com.rocuo.remote_files_helper.json` files in the folders above and the folder with the program.

## Installing for a team

There is no all-users install. For a team, run the same two commands for each user, from a script that runs as that user:

```text
rocuo-files-helper install --quiet --extension-id THE-ADDRESS-OF-YOUR-COPY
rocuo-files-helper status
```

If you publish your own build of the extension, its address differs from the store address; list it with `--extension-id`. The Chrome Web Store address stays on the list (`publishedExtensionIDs` in `install.go`); change it there if you fork the project.

## Checking a file

The build writes a `SHA256SUMS.txt` next to the programs, one line for each. The download page offers the same file, with one more line for the source ZIP. Compare the checksum of your file with its line:

```text
Windows (PowerShell)   Get-FileHash .\rocuo-files-helper-windows-amd64.exe -Algorithm SHA256
macOS / Linux          shasum -a 256 rocuo-files-helper-darwin-arm64
```

The programs are **not code-signed**, so Windows SmartScreen and macOS Gatekeeper may warn about a downloaded copy. A matching checksum from the page you trust is the check that is possible today. A build you make yourself from this folder is the surest way to know what you run.
