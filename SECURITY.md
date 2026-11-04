# Security policy

The helper program of Remote Files Browser FTP opens your servers and carries your passwords while it works, so a security problem here matters. Thank you for taking it seriously.

## Report a problem in private

**Please do not open a public issue or post it anywhere else for a security problem.** Tell us in private, in one of two ways:

1. **GitHub's private form:** open the **Security** tab of this repository, press **Report a vulnerability**, or go straight to <https://github.com/Maool2122/remote-files-browser/security/advisories/new>. Only the maintainers can read it.
2. **E-mail:** **info@rocuo.com**, with the subject "Security: Remote Files Browser helper".

Please say:

- which part it is about: the helper program, or the build and release process;
- the version (`rocuo-files-helper version`), the operating system and the browser;
- what you found and what it lets a person do, with steps we can follow;
- whether you want to be named when it is fixed.

**Do not send passwords, keys, tokens or private files**, and do not test against servers that are not yours. If you must show something, use the test servers of this repository (`go run ./testserver` in `helper/`).

We will read every report, tell you that it arrived, keep you informed while we work on it, and tell you when a fixed version is out. We ask you to give us time to fix the problem and to publish it **after** the fixed version is available.

The Chrome extension is a separate product with closed source and is not part of this repository. If you find a problem in the extension, write to **info@rocuo.com** in the same way; the private form above is for this repository.

## Which versions get fixes

Only the **newest release**. The helper program does not update itself: after a fix, a person has to download the new one (the extension tells them when a newer helper is needed). The release notes say so when a fix needs it.

## What is in scope

- The code of the helper program (`helper/`).
- The build, test and release tools and the workflows (`tools/`, `.github/`): for example a way to get something into a release that is not in the source, or to read a secret.
- Documents that tell people to do something unsafe.

Things that count, for example: a web page or another extension that can start the helper; a way around the question about trusting a server (a TLS certificate or an SSH host key); a message that makes the helper read or write something on the computer that it should not; a password that ends up in an address, a log or plain text; a problem in the way the release files are made or checked.

## What is not

- **The helper programs are not code-signed** (and the Mac one is not notarized). This is known and written down ([docs/VERIFYING-DOWNLOADS.md](docs/VERIFYING-DOWNLOADS.md)).
- Anything that needs **control of the person's computer or of their Chrome profile**. A program that can already run as the person can do everything the helper can do.
- **Plain FTP is plain text.** That is how FTP works. The helper tells the extension that the link is not encrypted, so that the person is warned.
- The behaviour of the servers a person connects to.
- A message from an automatic scanner that has no effect on a person. One example is `InsecureSkipVerify` in the helper's TLS code, which is explained, with the check that replaces it, in [helper/SECURITY.md](helper/SECURITY.md).
- Problems of Chrome, the Chrome Web Store, GitHub or Go themselves. Report those to their makers.

## How the project is kept safe

- The helper has **no server of ours behind it, no account and no tracking**, opens no port and keeps no log ([helper/SECURITY.md](helper/SECURITY.md) lists every claim with the file where you can verify it).
- No secret is kept in the repository. The release workflow uses only GitHub's own temporary token.
- Every GitHub Action is pinned to a full commit, the workflows have the smallest rights they need, and Dependabot proposes updates.
- Every week the known-vulnerability list is checked for the Go code (`govulncheck`) ([`security.yml`](.github/workflows/security.yml)).
- A check fails when something that belongs to the closed-source extension is added to this repository (`tools/check-scope.mjs`).
- Releases are made in the open by a workflow, come with checksums and a build attestation, and are drafts that a maintainer reads before publishing ([docs/VERIFYING-DOWNLOADS.md](docs/VERIFYING-DOWNLOADS.md)).

More about the helper program: [helper/SECURITY.md](helper/SECURITY.md).
