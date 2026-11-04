// Checks the documents of this repository, so that a document cannot point to something that is gone or say a number that is
// not true any more. It is one step of CI (the job "Documents and scope") and of a release.
//
//   node tools/check-docs.mjs
//
// What it checks:
//   - every link and picture in the Markdown files and in the HTML tags inside them: the file exists (with the exact upper and
//     lower case, which GitHub needs and Windows does not), and a `#heading` exists in the file it points to;
//   - every address of this repository (`github.com/<owner>/<repo>/blob|tree/main/<path>`) points to a file that exists, and
//     no address names another account for this repository (the account and the name are in tools/repository.mjs);
//   - the helper version is the same in helper/server.go, in the newest section of CHANGELOG.md and in the documents that say
//     it, and the message version (protocol) written in the documents is the one in helper/server.go;
//   - no placeholder (TODO, FIXME, TBD, XXX, lorem ipsum) is left in a document;
//   - every document in docs/ is listed in docs/README.md;
//   - the two licence files are the same;
//   - every `uses:` of a workflow is a full commit address with the version in a comment (or a local action), and every
//     workflow says what it may do (`permissions:`).
//
// Exit code 0: all true. Exit code 1: the problems are printed.
import { readdirSync, readFileSync } from 'node:fs';
import { join, posix } from 'node:path';
import { REPOSITORY } from './repository.mjs';

const ROOT = join(import.meta.dirname, '..');
/** Folders that are not in the repository (they are built or installed), so a link into them is a dead link on GitHub. */
const NOT_IN_REPOSITORY = new Set(['.git', 'node_modules', 'dist']);

const problems = [];
const fail = (where, message) => problems.push(`FAIL  ${where}  ${message}`);
const text = (rel) => readFileSync(join(ROOT, rel), 'utf8').replace(/\r\n/g, '\n');
const tryText = (rel) => {
  try {
    return text(rel);
  } catch {
    fail(rel, 'cannot be read');
    return '';
  }
};

// ---------- The files ----------
function walk(dir = '', out = []) {
  for (const entry of readdirSync(join(ROOT, dir), { withFileTypes: true })) {
    if (NOT_IN_REPOSITORY.has(entry.name)) continue;
    const rel = dir ? `${dir}/${entry.name}` : entry.name;
    if (entry.isDirectory()) walk(rel, out);
    else out.push(rel);
  }
  return out;
}
const files = walk();
const markdown = files.filter((file) => file.endsWith('.md'));
const textFiles = files.filter((file) => /\.(md|ya?ml|json)$/.test(file));

// ---------- Does a path exist, with the exact case? ----------
const listings = new Map();
function namesIn(dir) {
  if (!listings.has(dir)) listings.set(dir, new Set(readdirSync(join(ROOT, dir || '.'))));
  return listings.get(dir);
}
/** True when `rel` (a path relative to the repository, with `/`) exists as a file or folder, spelled exactly like that. */
function existsExact(rel) {
  if (rel === '' || rel === '.') return true;
  let dir = '';
  for (const part of rel.split('/')) {
    if (NOT_IN_REPOSITORY.has(part)) return false;
    let names;
    try {
      names = namesIn(dir);
    } catch {
      return false;
    }
    if (!names.has(part)) return false;
    dir = dir ? `${dir}/${part}` : part;
  }
  return true;
}
const isFile = (rel) => files.includes(rel);

// ---------- The headings of a Markdown file, as GitHub makes the anchors ----------
/** Takes the HTML tags out of a heading in one pass (`<code>x</code>` becomes `x`). A `<` with no `>` after it, and `<>`, are only text. */
function stripTags(text) {
  let out = '';
  let from = 0;
  while (from < text.length) {
    const open = text.indexOf('<', from);
    if (open === -1) return out + text.slice(from);
    const close = text.indexOf('>', open + 1);
    if (close === -1) return out + text.slice(from);
    if (close === open + 1) {
      out += text.slice(from, close + 1);
    } else {
      out += text.slice(from, open);
    }
    from = close + 1;
  }
  return out;
}

function baseSlug(heading) {
  const plain = heading.replace(/!\[([^\]]*)\]\([^)]*\)/g, '$1').replace(/\[([^\]]*)\]\([^)]*\)/g, '$1');
  return stripTags(plain)
    .replace(/[`*~]/g, '')
    .trim()
    .toLowerCase()
    .replace(/[^\p{L}\p{N}\p{M} _-]/gu, '')
    .replace(/ /g, '-');
}

const anchorCache = new Map();
function anchorsOf(rel) {
  if (anchorCache.has(rel)) return anchorCache.get(rel);
  const anchors = new Set();
  const occurrences = new Map();
  let fence = null;
  for (const line of text(rel).split('\n')) {
    const fenceStart = /^\s*(```+|~~~+)/.exec(line);
    if (fenceStart) {
      if (!fence) fence = fenceStart[1][0];
      else if (fenceStart[1][0] === fence) fence = null;
      continue;
    }
    if (fence) continue;
    const heading = /^ {0,3}#{1,6}\s+(.+?)(?:\s+#+)?\s*$/.exec(line);
    if (heading) {
      const original = baseSlug(heading[1]);
      let slug = original;
      while (occurrences.has(slug)) {
        occurrences.set(original, occurrences.get(original) + 1);
        slug = `${original}-${occurrences.get(original)}`;
      }
      occurrences.set(slug, 0);
      anchors.add(slug);
    }
    for (const [, name] of line.matchAll(/<a\s[^>]*?(?:id|name)="([^"]+)"/g)) anchors.add(name);
  }
  anchorCache.set(rel, anchors);
  return anchors;
}

// ---------- The links of a Markdown file ----------
/** The links and pictures outside code: [{ line, target }] */
function linksOf(rel) {
  const found = [];
  let fence = null;
  text(rel).split('\n').forEach((raw, index) => {
    const fenceStart = /^\s*(```+|~~~+)/.exec(raw);
    if (fenceStart) {
      if (!fence) fence = fenceStart[1][0];
      else if (fenceStart[1][0] === fence) fence = null;
      return;
    }
    if (fence) return;
    // A code span may hold anything, also something that looks like a link.
    const line = raw.replace(/(`+)(?!`)(.+?)(?<!`)\1(?!`)/g, (span) => ' '.repeat(span.length));
    for (const match of line.matchAll(/!?\[(?:[^\[\]]|\[[^\]]*\])*\]\(\s*<?([^\s)>]+)>?(?:\s+"[^"]*")?\s*\)/g)) found.push({ line: index + 1, target: match[1] });
    // A picture inside a link text, [![alt](picture)](address), is hidden in the match above: look for pictures again.
    for (const match of line.matchAll(/!\[[^\]]*\]\(\s*<?([^\s)>]+)>?/g)) found.push({ line: index + 1, target: match[1] });
    for (const match of line.matchAll(/^\s{0,3}\[[^\]]+\]:\s+<?(\S+?)>?(?:\s|$)/g)) found.push({ line: index + 1, target: match[1] });
    for (const match of line.matchAll(/<(?:img|a|source)\s[^>]*?\b(?:src|href)="([^"]+)"/g)) found.push({ line: index + 1, target: match[1] });
  });
  // The same link found twice is checked once.
  return [...new Map(found.map((link) => [`${link.line}\n${link.target}`, link])).values()];
}

let linkCount = 0;
function checkLink(file, line, target) {
  if (/^(https?:|mailto:|tel:|data:)/i.test(target)) return; // addresses of the repository are checked below
  linkCount++;
  const where = `${file}:${line}`;
  const hash = target.indexOf('#');
  let pathPart = (hash < 0 ? target : target.slice(0, hash)).replace(/\?.*$/, '');
  const anchor = hash < 0 ? '' : target.slice(hash + 1);
  let destination;
  if (pathPart === '') destination = file;
  else {
    try {
      pathPart = decodeURIComponent(pathPart);
    } catch {
      return fail(where, `the link "${target}" is not a valid address`);
    }
    destination = pathPart.startsWith('/') ? posix.normalize(pathPart.slice(1)) : posix.normalize(posix.join(posix.dirname(file), pathPart));
    if (destination.startsWith('..')) return fail(where, `the link "${target}" points outside the repository`);
    destination = destination.replace(/\/$/, '');
    if (!existsExact(destination)) return fail(where, `the link "${target}" points to "${destination}", which does not exist (check the upper and lower case too)`);
  }
  if (anchor && destination.endsWith('.md') && isFile(destination)) {
    let wanted = anchor;
    try {
      wanted = decodeURIComponent(anchor);
    } catch {
      // keep the raw text
    }
    if (!anchorsOf(destination).has(wanted)) fail(where, `the link "${target}": ${destination} has no heading that makes the anchor "#${wanted}"`);
  } else if (anchor && pathPart !== '' && !isFile(destination) && existsExact(destination)) {
    fail(where, `the link "${target}" has an anchor, but "${destination}" is a folder`);
  }
}

for (const file of markdown) for (const { line, target } of linksOf(file)) checkLink(file, line, target);

// ---------- The addresses of this repository ----------
{
  const [owner, name] = REPOSITORY.split('/');
  const escaped = (value) => value.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
  let repoLinks = 0;
  for (const file of textFiles) {
    text(file).split('\n').forEach((line, index) => {
      const where = `${file}:${index + 1}`;
      for (const [, who] of line.matchAll(new RegExp(`github\\.com[/:]([A-Za-z0-9_.-]+)/${escaped(name)}(?![A-Za-z0-9_-])`, 'g'))) {
        if (who !== owner) fail(where, `the repository is "${REPOSITORY}", but this address says "${who}/${name}"`);
      }
      for (const [, , path] of line.matchAll(new RegExp(`github\\.com/${escaped(REPOSITORY)}/(blob|tree)/main/([^\\s)"'>#?]*)`, 'g'))) {
        repoLinks++;
        if (!existsExact(path.replace(/\/$/, ''))) fail(where, `the address points to "${path}" on the main branch, which does not exist`);
      }
    });
  }
  linkCount += repoLinks;
}

// ---------- Versions ----------
const serverGo = tryText('helper/server.go');
const helperVersion = /^\s*version\s*=\s*"(\d+\.\d+\.\d+)"/m.exec(serverGo)?.[1];
const protocol = /^\s*protocolVersion\s*=\s*(\d+)/m.exec(serverGo)?.[1];
if (!helperVersion) fail('helper/server.go', 'the line version = "X.Y.Z" was not found');
if (!protocol) fail('helper/server.go', 'the line protocolVersion = N was not found');

const changelog = tryText('CHANGELOG.md');
const newest = [...changelog.matchAll(/^## \[?(\d+\.\d+\.\d+)\]?(?: - (\d{4}-\d{2}-\d{2}))?\s*$/gm)][0];
if (!newest) fail('CHANGELOG.md', 'has no version section (## [1.2.3] - YYYY-MM-DD)');
else {
  if (helperVersion && newest[1] !== helperVersion) fail('CHANGELOG.md', `the newest version is ${newest[1]}, helper/server.go says ${helperVersion}`);
  if (!newest[2]) fail('CHANGELOG.md', `the section of ${newest[1]} has no date`);
  // The links at the bottom are optional (they need a tag to exist), but when they are there they must name the newest version.
  const unreleased = /^\[Unreleased\]:\s*\S+\/compare\/v(\d+\.\d+\.\d+)\.\.\.HEAD\s*$/m.exec(changelog);
  if (unreleased && unreleased[1] !== newest[1]) fail('CHANGELOG.md', `the link [Unreleased] compares from v${unreleased[1]}, the newest version is ${newest[1]}`);
}

const said = (rel, pattern, what, expected) => {
  const value = pattern.exec(tryText(rel))?.[1];
  if (value === undefined) fail(rel, `does not say ${what} (the line that carries it was not found)`);
  else if (value !== expected) fail(rel, `says ${what} ${value}, the code says ${expected}`);
};
if (helperVersion) {
  said('README.md', /Version `([^`]+)`, message version/, 'the helper version', helperVersion);
  said('helper/README.md', /^- Version `([^`]+)`/m, 'the helper version', helperVersion);
  // "Protocol **1** is used by every helper version so far (`1.0.0` and `1.1.0`)": the last version named is the newest one.
  said('helper/docs/PROTOCOL.md', /^Protocol \*\*\d+\*\*[^\n]*`(\d+\.\d+\.\d+)`[^`\n]*$/m, 'the newest helper version', helperVersion);
  said('helper/docs/INSTALLING.md', /Remote Files Browser helper (\d+\.\d+\.\d+) \(/, 'the helper version in the example', helperVersion);
}
if (protocol) {
  said('README.md', /message version \(protocol\) `([^`]+)`/, 'the protocol', protocol);
  said('helper/README.md', /message version \(protocol\) `([^`]+)`/, 'the protocol', protocol);
  said('helper/docs/PROTOCOL.md', /^Protocol \*\*(\d+)\*\* (?:goes with|is used by)/m, 'the protocol', protocol);
}

// ---------- Placeholders ----------
for (const file of markdown) {
  let fence = null;
  text(file).split('\n').forEach((raw, index) => {
    const fenceStart = /^\s*(```+|~~~+)/.exec(raw);
    if (fenceStart) {
      if (!fence) fence = fenceStart[1][0];
      else if (fenceStart[1][0] === fence) fence = null;
      return;
    }
    if (fence) return;
    const line = raw.replace(/(`+)(?!`)(.+?)(?<!`)\1(?!`)/g, '');
    const hit = /\b(TODO|FIXME|TBD|XXX)\b|lorem ipsum/i.exec(line);
    if (hit) fail(`${file}:${index + 1}`, `a placeholder is left in the text ("${hit[0]}")`);
  });
}

// ---------- The index of the documents ----------
const index = tryText('docs/README.md');
for (const file of markdown.filter((f) => /^docs\/[^/]+\.md$/.test(f) && f !== 'docs/README.md')) {
  const name = file.slice('docs/'.length);
  if (!index.includes(`](${name})`) && !index.includes(`](${name}#`)) fail('docs/README.md', `does not link to ${file}`);
}
if (!linksOf('README.md').some(({ target }) => /^docs\/README\.md(#.*)?$/.test(target))) fail('README.md', 'does not link to docs/README.md');

// ---------- The licence ----------
if (text('LICENSE') !== tryText('helper/LICENSE')) fail('LICENSE', 'is not the same as helper/LICENSE');
if (!/^MIT License/.test(text('LICENSE'))) fail('LICENSE', 'does not start with "MIT License"');

// ---------- The workflows ----------
for (const file of files.filter((f) => /^\.github\/(workflows|actions)\/.+\.ya?ml$/.test(f))) {
  const body = text(file);
  body.split('\n').forEach((line, index) => {
    const use = /^\s*-?\s*uses:\s*(\S+)(?:\s+#\s*(.*))?\s*$/.exec(line);
    if (!use || use[1].startsWith('./')) return;
    if (!/^[\w.-]+\/[\w./-]+@[0-9a-f]{40}$/.test(use[1])) fail(`${file}:${index + 1}`, `"${use[1]}" is not a full commit address (owner/repo@40 characters): a tag can be moved`);
    else if (!/^v\d/.test(use[2] ?? '')) fail(`${file}:${index + 1}`, `"${use[1]}" needs the version in a comment after it (# v1.2.3), so that Dependabot can update it`);
  });
  if (file.startsWith('.github/workflows/') && !/^permissions:/m.test(body)) fail(file, 'has no top-level "permissions:" (say what the workflow may do)');
}

// ---------- Result ----------
if (problems.length) {
  for (const problem of problems) console.log(problem);
  console.log(`\n${problems.length} problem(s) in the documents`);
  process.exit(1);
}
console.log(`Documents are true: ${markdown.length} Markdown files, ${linkCount} links, helper ${helperVersion}, protocol ${protocol}.`);
