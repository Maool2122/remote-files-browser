# Verifying what you download and what you run

Open source lets you read the code. It does not by itself prove that the file you downloaded was made from that code. This page shows how to check a helper program: by its checksum, by where it was built, and by building it yourself.

- [In short](#in-short)
- [Check the checksum](#check-the-checksum)
- [Check where it was built](#check-where-it-was-built)
- [Build it yourself](#build-it-yourself)
- [If Windows or macOS warns about the helper](#if-windows-or-macos-warns-about-the-helper)
- [What these checks do not prove](#what-these-checks-do-not-prove)
- [If a check fails](#if-a-check-fails)

## In short

| You have | Check it with | What it tells you |
| --- | --- | --- |
| A helper program (`rocuo-files-helper-...`) | its line in `SHA256SUMS.txt` | The file is the one that the page you trust lists, byte for byte |
| A helper program from GitHub Releases | `gh attestation verify` | GitHub Actions built it in this repository, from a named commit |
| Any helper program | build it yourself (`node helper/build.mjs`) | You run what you compiled; with the same Go version you get the same bytes |

The helper programs are **not signed or notarized** yet (see [the end of this page](#if-windows-or-macos-warns-about-the-helper)). The checksum and the build are the checks that are possible today.

The Chrome extension is a separate, closed-source product and is not covered by this page: you get it from the Chrome Web Store, which signs and updates it.

## Check the checksum

Every release has a file `SHA256SUMS.txt` with one line for each file: the SHA-256 checksum, two spaces, the name. The download page of the publisher, <https://rocuo.com/remote-files/ftp-helper/>, has a `SHA256SUMS.txt` for the files it offers. **A checksum from the same place as the file only proves that the download was not damaged.** Comparing it with the checksum from a *second* place (the GitHub release and the download page, or your own build) is what makes it a check.

**Windows (PowerShell)**

```powershell
Get-FileHash .\rocuo-files-helper-windows-amd64.exe -Algorithm SHA256
```

**macOS**

```bash
shasum -a 256 rocuo-files-helper-darwin-arm64
```

**Linux**

```bash
sha256sum rocuo-files-helper-linux-amd64
```

Compare the result with the line for your file in `SHA256SUMS.txt`. On macOS and Linux you can let the computer compare, for the one file you downloaded (the third line is for a folder with several of the files; `--ignore-missing` skips the ones you did not download):

```bash
grep ' rocuo-files-helper-linux-amd64$' SHA256SUMS.txt | sha256sum -c          # Linux; prints "...: OK" or "...: FAILED"
grep ' rocuo-files-helper-darwin-arm64$' SHA256SUMS.txt | shasum -a 256 -c     # macOS
sha256sum -c --ignore-missing SHA256SUMS.txt                                    # Linux, every file that is in the folder
```

If one character differs, **do not run the file** ([if a check fails](#if-a-check-fails)).

A file built by the release workflow can have another checksum than a file with the same name from the download page: the helper contains the Go standard library, so a different Go version gives a different file even from the same source. Each place has its own `SHA256SUMS.txt`; compare a file only with the checksums that come with it, or with the attestation below.

## Check where it was built

The files of a GitHub release are built by the workflow [`release.yml`](../.github/workflows/release.yml) and get a **build attestation**: a statement, signed by GitHub, that this repository's workflow produced exactly this file from a named commit. You need the [GitHub command line tool](https://cli.github.com/) (version 2.49 or newer, signed in once with `gh auth login`):

```bash
gh attestation verify rocuo-files-helper-linux-amd64 --repo Maool2122/remote-files-browser
```

It prints the result of the check, with the commit and the workflow. It also works for `SHA256SUMS.txt`. A file that was changed after the build, or built anywhere else, fails.

Two more facts you can read from the program itself, with Go installed (`go version -m` prints how a Go program was built: the Go version, `-trimpath`, `CGO_ENABLED=0`, the flags):

```bash
go version -m rocuo-files-helper-linux-amd64
```

## Build it yourself

This is the surest way to know what you run. You need Go and Node.js ([helper/docs/BUILDING.md](../helper/docs/BUILDING.md#what-you-need)):

```bash
git clone https://github.com/Maool2122/remote-files-browser.git
cd remote-files-browser
git checkout vX.Y.Z          # the tag of the release you want
node helper/build.mjs        # or: node helper/build.mjs windows | darwin | linux
```

The programs and a `SHA256SUMS.txt` are in `helper/dist/`. The build puts no time, no folder names and no commit stamp into the program (`-trimpath -buildvcs=false`, no C code), so **the same source built with the same Go version gives the same file**. This was checked on one Windows computer (the macOS and Linux programs are cross-compiled on Windows). With Go 1.27.0 it was checked on 2026-10-28 for the two Windows programs of version 1.0.0 and on 2026-11-04 for all six. With Go 1.27.3 it was checked on 2026-11-04 for **all six programs of version 1.1.0**: each was built again from the files of the repository, in another folder, and had the same checksum as the file on the download page ([helper/docs/BUILDING.md](../helper/docs/BUILDING.md#the-same-bytes)). It was not checked on other kinds of computer (a build on a Mac or on a Linux computer has not been compared), and a different Go version gives a different file. The release notes of a release say which Go version built it.

**Use the newest patch release of Go.** The Go standard library, including the code that checks certificates, is part of the program. Older patch releases of Go have security fixes that a program built with them lacks ([helper/SECURITY.md](../helper/SECURITY.md#known-limits)).

## If Windows or macOS warns about the helper

The programs are **not code-signed** (Windows) and **not notarized** (macOS). A signature needs a certificate that belongs to the publisher and costs money each year; until the publisher has one, the system does not know who made the file and may warn:

- **Windows:** SmartScreen may say "Windows protected your PC". Check the checksum first. If it matches, **More info**, then **Run anyway**.
- **macOS:** Gatekeeper may refuse a program that came from a browser. Check the checksum first. If it matches, control-click the file and choose **Open**.

A warning is not a sign that the file is bad, and a missing warning is not a sign that it is good. The checksum and the build are your checks.

What the helper can and cannot do is written down in [helper/SECURITY.md](../helper/SECURITY.md), with every claim tied to the file where you can check it. In short: it runs as you, opens no port, starts only for the listed extensions, talks only to the servers you open and sends nothing to anyone else.

## What these checks do not prove

- They show that a file was made from this source. They do not show that **the source is good**. Read it: the helper is small enough to read (on 2026-11-04 the program was 16 files of Go, about 3,400 lines, blank lines included, without the tests and the test servers), and [helper/docs/ARCHITECTURE.md](../helper/docs/ARCHITECTURE.md) and [helper/SECURITY.md](../helper/SECURITY.md) say what to look for.
- They say nothing about the **extension**, whose source is closed ([WHY-OPEN-SOURCE.md](WHY-OPEN-SOURCE.md#4-what-you-cannot-check-here-and-what-limits-it)).
- The attestation proves that the build happened in this repository. It does not prove that nobody with write access put something bad into the repository: that is what reading the changes and the review of every pull request are for.
- The checksum from a place that an attacker controls proves nothing. Use two places.
- A build you make yourself is only as good as the Go and Node.js on your computer. `helper/go.sum` pins the libraries to exact versions and checksums.

## If a check fails

Do not run the file. Write to **info@rocuo.com** or use **private vulnerability reporting** as described in [SECURITY.md](../SECURITY.md): say which file, where you got it, the checksum you got and the one you expected. If you only had a bad download, downloading again fixes it.
