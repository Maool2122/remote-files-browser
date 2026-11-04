# Releasing

Only maintainers (people with write access to the repository) make releases. This page is the checklist, in the order it is done, and the settings the repository needs once.

## What a release is

A release starts from a **tag** `vX.Y.Z` on `main`, where `X.Y.Z` is the version of the **helper program** (`version` in `helper/server.go`). [`.github/workflows/release.yml`](../.github/workflows/release.yml) then:

1. checks that the tag, `helper/server.go` and `CHANGELOG.md` say the same version, checks the documents and the scope, and runs `gofmt`, `go vet` and the Go tests (on Windows);
2. builds the **six helper programs** with the newest patch release of Go (on Linux);
3. writes `SHA256SUMS.txt` for them, and records where everything was built (a GitHub **build attestation**, [VERIFYING-DOWNLOADS.md](VERIFYING-DOWNLOADS.md#check-where-it-was-built));
4. makes a **draft** release with the notes of `CHANGELOG.md` and the built files.

Nothing is public until a maintainer reads the draft and presses **Publish release**. The workflow does not sign the programs ([PLATFORMS.md](PLATFORMS.md)) and uploads them nowhere else.

The Chrome extension is a closed-source product with its own way to release, and the website rocuo.com is run by the Rocuo team: neither is part of this repository, and a release here changes neither.

## The numbers

| Where | What it holds | When it changes |
| --- | --- | --- |
| `helper/server.go` `version` | The version of the helper program. **The tag must be this number with a `v`.** It is written again in `README.md`, `helper/README.md`, `helper/docs/PROTOCOL.md` and in the example in `helper/docs/INSTALLING.md` | Every release |
| `CHANGELOG.md` | A section `## [X.Y.Z] - YYYY-MM-DD` | Every release |
| `protocolVersion` in `helper/server.go` | The message version between the extension and the helper. The closed-source extension carries the same number | Only when an old extension and a new helper (or the other way round) can no longer understand each other. That needs a change of the extension too, so open an issue first |

`node tools/check-docs.mjs` compares the places that can be compared, and `node tools/release-notes.mjs --check vX.Y.Z` checks that a version is ready to be released.

## The steps

1. **Prepare the release in a pull request.**
   - Set the new version in `helper/server.go` and in the four places of the table above that say it.
   - In `CHANGELOG.md` move the notes under "Unreleased" into a new section `## [X.Y.Z] - YYYY-MM-DD` (today's date) and leave "Unreleased" with `Nothing yet.` When the first release exists, keep the two links at the bottom of the file up to date (`[Unreleased]: https://github.com/Maool2122/remote-files-browser/compare/vX.Y.Z...HEAD`, and `[X.Y.Z]: https://github.com/Maool2122/remote-files-browser/releases/tag/vX.Y.Z` for every version).
   - Update the documents the change touches (the helper's own `docs/` and [docs/](README.md)).
2. **Run everything on your computer:**

   ```bash
   cd helper && gofmt -l . && go vet ./... && go test -count=1 ./... && cd ..
   node tools/check-docs.mjs
   node tools/check-scope.mjs
   node tools/release-notes.mjs --check vX.Y.Z
   ```

   The maintainers also run the helper with the real extension in a real Chrome on Windows before a release; that test belongs to the closed-source extension and is not in this repository ([TESTING.md](TESTING.md#what-is-not-tested-by-a-script)).
3. **Merge** the pull request when CI is green.
4. **Optional trial run:** Actions, **Release**, **Run workflow** on `main`. It checks and builds everything and keeps the files for seven days; it makes no release.
5. **Tag the commit of `main` that passed**, and push the tag:

   ```bash
   git switch main && git pull
   node tools/release-notes.mjs --check vX.Y.Z
   git tag -a vX.Y.Z -m "Remote Files Browser FTP helper X.Y.Z"
   git push origin vX.Y.Z
   ```

   (`git tag -s` makes a signed tag when you have a signing key. The workflow does not need one.)
6. **Wait for the workflow** (Actions, **Release**). If a step fails, read it, fix the cause in a new pull request, and see "If something goes wrong" below.
7. **Read the draft** (Releases): the notes are the section of `CHANGELOG.md` plus a line saying which run, which commit and which Go version built the files. It must hold the six `rocuo-files-helper-*` programs and `SHA256SUMS.txt`. Download one program and check it as a user would ([VERIFYING-DOWNLOADS.md](VERIFYING-DOWNLOADS.md)).
8. **Publish release.**

The download page of the helper on rocuo.com has its own copy of the files, which the Rocuo team builds and publishes. They can have other checksums than the files of a release here, because another Go version gives other bytes ([VERIFYING-DOWNLOADS.md](VERIFYING-DOWNLOADS.md#check-the-checksum)).

## If something goes wrong

- **The workflow failed before the draft was made.** Open the run and read the step that failed. If it was a hiccup (a download that timed out), use **Re-run failed jobs** on the Actions page: the last step adds the files to a draft that already exists. If the cause is in the code or the documents, fix it in a pull request and release the **next patch number**; the number of the failed attempt is simply not used. (An admin can delete the unused tag and use the number again, after lifting the tag rule below for a moment. Moving a tag is not needed and is best avoided.)
- **A published release is wrong.** With immutable releases (below) its files and tag can no longer be changed, and that is the point. Do not delete it: publish a new, fixed patch version, and edit the notes of the wrong one to say so (the notes can still be edited).
- **A security problem.** Follow [SECURITY.md](../SECURITY.md#report-a-problem-in-private) and make the fix as a patch release like any other.

## Settings of the repository

Done once, by an admin of the repository (whoever can open its **Settings**). The first rows matter most.

| Setting | Where on GitHub | Why |
| --- | --- | --- |
| **Immutable releases** | Settings, **Releases**, "Enable release immutability" | A published release can no longer have its files or tag changed (it applies to releases made after the switch, so turn it on before the first one). The draft-then-publish way of the workflow is the way GitHub recommends for it |
| **Private vulnerability reporting** | Settings, **Advanced Security**, "Private vulnerability reporting", Enable | [SECURITY.md](../SECURITY.md) tells people to use it |
| **Dependabot alerts and security updates**, **secret scanning with push protection** | Settings, **Advanced Security** | Known problems in libraries and committed secrets are found and reported; [.github/dependabot.yml](../.github/dependabot.yml) already asks for updates every week |
| **Code scanning with CodeQL "default setup"** | Settings, **Advanced Security**, Code scanning | Go, GitHub Actions and JavaScript (the small checking programs in `tools/`). It is a setting and not a workflow file |
| **A rule for `main`** | Settings, **Rules**, Rulesets (the rule is called "main") | Require a pull request (no approval is needed, because the project has one maintainer today), require the checks `Helper (Windows)`, `Helper for all six systems` and `Documents and scope`, and block force pushes and deletion. The Linux and macOS jobs are experimental, so they are not required. The role "Repository admin" is on the bypass list ("Always allow"), so a maintainer can still push a small fix straight to `main`, and GitHub prints a note when that happens. Everyone else works through a pull request |
| **A rule for tags `v*`** | Settings, **Rules**, Rulesets (the rule is called "release tags") | Nobody deletes, moves or force-pushes a release tag. The bypass list is empty on purpose: an admin who must remove a tag made by mistake first sets the rule to "Disabled" for a moment, as "If something goes wrong" says above |
| **Actions** | Settings, **Actions**, General | Workflow permissions: "Read repository contents and packages permissions". Leave "Allow GitHub Actions to create and approve pull requests" off. Fork pull request workflows: require approval for first-time contributors |
| **Labels** | Issues, **Labels** | The [platform report form](../.github/ISSUE_TEMPLATE/platform_report.yml) uses `platform-report`, and the other forms use `bug`, `enhancement` and `question`. This repository was made without GitHub's usual set of labels, so they were created by hand with GitHub's usual names, colours and descriptions: `bug`, `documentation`, `duplicate`, `enhancement`, `good first issue`, `help wanted`, `invalid`, `question` and `wontfix`, next to `platform-report`. Dependabot makes `dependencies` and `go` itself when it opens a pull request |
| **Features** | Settings, General | Issues on. Wiki, Projects and Discussions off: the documents are in the repository and questions come as issues ([SUPPORT.md](../SUPPORT.md)) |
| **About box** | Main page of the repository, the gear next to "About" | A description, the website `https://rocuo.com/remote-files/`, and topics such as `ftp`, `ftps`, `sftp`, `native-messaging`, `golang`, `chrome-extension` |

## After the first release

The checks in [VERIFYING-DOWNLOADS.md](VERIFYING-DOWNLOADS.md) were tried on files built on one computer, because no release existed yet. After the first real release:

- try every command of that page on the real release files, and correct the page where the real result differs;
- read the first runs of CI, in particular the **experimental** Linux and macOS jobs, and update [PLATFORMS.md](PLATFORMS.md) with what they show;
- ask someone with a Mac or a Linux computer to follow [PLATFORMS.md](PLATFORMS.md#try-the-helper-on-macos-or-linux);
- remove this section.
