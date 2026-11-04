// The notes of a release, and the check that a version is ready to be released. Used by .github/workflows/release.yml.
//
//   node tools/release-notes.mjs --version        prints the version of the helper program (the one in helper/server.go)
//   node tools/release-notes.mjs 1.0.0            prints the notes of that version, from CHANGELOG.md (without the heading)
//   node tools/release-notes.mjs --unreleased     prints the notes under "Unreleased"
//   node tools/release-notes.mjs --check v1.0.0   checks that the version is ready: helper/server.go says the same number,
//                                                 CHANGELOG.md has a dated entry with notes in it, and nothing is left
//                                                 under "Unreleased"
//
// The exit code is 0 when it worked and 1 when something is wrong (the problems are printed), 2 when the command is wrong.
import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import { REPOSITORY } from './repository.mjs';

const ROOT = join(import.meta.dirname, '..');
const read = (file) => readFileSync(join(ROOT, file), 'utf8').replace(/\r\n/g, '\n');
const args = process.argv.slice(2);

/** The version of the helper program, from helper/server.go (`version = "1.0.0"`). */
function helperVersion() {
  const version = /^\s*version\s*=\s*"(\d+\.\d+\.\d+)"/m.exec(read('helper/server.go'))?.[1];
  if (!version) {
    console.error('helper/server.go has no line version = "X.Y.Z".');
    process.exit(1);
  }
  return version;
}

/** The sections of CHANGELOG.md: [{ name: '1.0.0' | 'Unreleased', date: '2026-10-28' | undefined, body: '...' }]. */
function sections() {
  const lines = read('CHANGELOG.md').split('\n');
  const found = [];
  for (const line of lines) {
    const heading = /^## \[?([^\]\s]+)\]?(?: - (\d{4}-\d{2}-\d{2}))?\s*$/.exec(line);
    if (heading) found.push({ name: heading[1], date: heading[2], lines: [] });
    else if (found.length) found[found.length - 1].lines.push(line);
  }
  return found.map(({ name, date, lines: body }) => {
    // The link definitions at the end of the file ("[1.0.0]: https://...") belong to no section.
    while (body.length && (body[body.length - 1].trim() === '' || /^\[[^\]]+\]:\s+\S+/.test(body[body.length - 1]))) body.pop();
    return { name, date, body: body.join('\n').trim() };
  });
}

function usage() {
  console.error('Usage: node tools/release-notes.mjs --version | <version> | --unreleased | --check v<version>');
  process.exit(2);
}

if (args.length === 0) usage();

if (args[0] === '--version') {
  console.log(helperVersion());
  process.exit(0);
}

if (args[0] === '--unreleased') {
  console.log(sections().find((section) => section.name === 'Unreleased')?.body ?? '');
  process.exit(0);
}

if (args[0] === '--check') {
  const tag = args[1] ?? '';
  const match = /^v(\d+\.\d+\.\d+)$/.exec(tag);
  if (!match) {
    console.error(`"${tag}" is not a version tag like v1.0.0.`);
    process.exit(2);
  }
  const version = match[1];
  const problems = [];

  const inCode = helperVersion();
  if (inCode !== version) problems.push(`helper/server.go says ${inCode}, the tag says ${version}`);

  const all = sections();
  const entry = all.find((section) => section.name === version);
  if (!entry) {
    problems.push(`CHANGELOG.md has no section "## [${version}] - YYYY-MM-DD"`);
  } else {
    if (!entry.date) problems.push(`the section of ${version} in CHANGELOG.md has no date ("## [${version}] - YYYY-MM-DD")`);
    else {
      const when = new Date(`${entry.date}T00:00:00Z`);
      if (Number.isNaN(when.getTime()) || when.toISOString().slice(0, 10) !== entry.date) problems.push(`the date ${entry.date} in CHANGELOG.md is not a real date`);
      else if (when.getTime() > Date.now() + 36 * 3600 * 1000) problems.push(`the date ${entry.date} in CHANGELOG.md is in the future`);
    }
    if (entry.body.replace(/\s+/g, ' ').length < 20) problems.push(`the section of ${version} in CHANGELOG.md has no notes`);
  }
  const unreleased = all.find((section) => section.name === 'Unreleased');
  if (unreleased && unreleased.body && !/^nothing yet\.?$/i.test(unreleased.body)) problems.push('CHANGELOG.md still has notes under "Unreleased": move them into the section of this version');

  if (problems.length) {
    for (const problem of problems) console.log(`NOT READY  ${problem}`);
    process.exit(1);
  }
  console.log(`Ready: ${tag} (${entry.date}). helper/server.go and CHANGELOG.md agree.`);
  process.exit(0);
}

const version = args[0].replace(/^v/, '');
const entry = sections().find((section) => section.name === version);
if (!entry) {
  console.error(`CHANGELOG.md has no section for ${version}.`);
  process.exit(1);
}

// A link such as (helper/README.md) works in the file CHANGELOG.md, but not on the page of a release. On a release page it must
// be the address of the file in the repository as it was at that tag.
const absolute = (text) =>
  text.replace(/\]\((?!https?:|#|mailto:)([^)\s]+)\)/g, (_, target) => `](https://github.com/${REPOSITORY}/blob/v${version}/${target.replace(/^\.\//, '')})`);
console.log(absolute(entry.body));
