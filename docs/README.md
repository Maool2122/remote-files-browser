# Documents

Every document of this repository, by what you want to do. The front page of the repository is the [README](../README.md).

This repository holds the **helper program** only. The Chrome extension that uses it is closed source, so it has no documents here; it is free from the [Chrome Web Store](https://chromewebstore.google.com/detail/remote-files-browser-ftp/pckgpblcplbjffapabndpkhnhkioihjd) ([why the split](WHY-OPEN-SOURCE.md)).

## I use the helper

| Document | What is in it |
| --- | --- |
| [../helper/README.md](../helper/README.md) | The helper program: what it does and does not do, every command, where it installs on each system |
| [../helper/docs/INSTALLING.md](../helper/docs/INSTALLING.md) | Installing the helper, step by step, for each system; checking a file; removing it |
| [../helper/docs/TROUBLESHOOTING.md](../helper/docs/TROUBLESHOOTING.md) | When the helper does not work: what each message means and what to do |
| [../SUPPORT.md](../SUPPORT.md) | Where to ask for help, and how to report a bug |
| [../CHANGELOG.md](../CHANGELOG.md) | What changed in each version |

## I want to know if I can trust it

| Document | What is in it |
| --- | --- |
| [WHY-OPEN-SOURCE.md](WHY-OPEN-SOURCE.md) | Why the helper is open and the extension is not, what you can check yourself, what you cannot, and what the licence covers |
| [VERIFYING-DOWNLOADS.md](VERIFYING-DOWNLOADS.md) | How to check a helper program: checksum, build attestation, your own build |
| [../helper/SECURITY.md](../helper/SECURITY.md) | The security claims of the helper, each with the file where you can verify it |
| [../SECURITY.md](../SECURITY.md) | How to report a vulnerability in private, what is in scope, how the project is kept safe |
| [PLATFORMS.md](PLATFORMS.md) | On which systems the helper has really been run, and on which it has not |
| [../helper/THIRD_PARTY_NOTICES.md](../helper/THIRD_PARTY_NOTICES.md) | The libraries inside the helper and their licences |

## I want to build, change or learn from it

| Document | What is in it |
| --- | --- |
| [../helper/docs/ARCHITECTURE.md](../helper/docs/ARCHITECTURE.md) | How the helper program is built, file by file |
| [../helper/docs/PROTOCOL.md](../helper/docs/PROTOCOL.md) | The messages between an extension and the helper, every field |
| [../helper/docs/BUILDING.md](../helper/docs/BUILDING.md) | Building and testing the helper, the test servers |
| [TESTING.md](TESTING.md) | The tests, what each one proves, how to run and write one, what is not tested |
| [../CONTRIBUTING.md](../CONTRIBUTING.md) | How to contribute: what helps, the rules of a change, the pull request |
| [../helper/CONTRIBUTING.md](../helper/CONTRIBUTING.md) | The same, for the code of the helper |
| [../CODE_OF_CONDUCT.md](../CODE_OF_CONDUCT.md) | How we treat each other |
| [../TRADEMARKS.md](../TRADEMARKS.md) | What the licence does not cover: the names and the look |

## I help test on another system

| Document | What is in it |
| --- | --- |
| [PLATFORMS.md](PLATFORMS.md) | What has been run, what counts as "tested", and the steps to try the helper on macOS or Linux |

## I maintain it

| Document | What is in it |
| --- | --- |
| [RELEASING.md](RELEASING.md) | Making a release, the numbers that must agree, and the settings the repository needs |
| [TESTING.md](TESTING.md#what-continuous-integration-runs) | What continuous integration runs |

`node tools/check-docs.mjs` checks that every link and every version in these documents is true, so a document cannot point to a file that is gone ([tools/check-docs.mjs](../tools/check-docs.mjs)). The pictures of the front page are in [images/](images).
