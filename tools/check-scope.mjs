// Checks that this repository holds only the helper program, its documents and its tools.
//
//   node tools/check-scope.mjs
//
// The Chrome extension that uses the helper is closed source. It must never be committed here, not by a mistake of a
// maintainer and not in a pull request. This check fails when a file or a folder is here that is not part of the open part:
//   - a folder or file in the root that is not on the list below;
//   - in `tools/`, anything but `.mjs` files; in `docs/`, anything but `.md` files and the `images` folder;
//   - anywhere, a file that belongs to a browser extension project: the folders `src/` and `public/`, `package.json`,
//     `package-lock.json`, `manifest.json`, `tsconfig*.json`, `vite.config.*`, TypeScript and JSX files, a packed extension
//     (`.crx`) and zip files.
//
// Exit code 0: only the open part is here. Exit code 1: the problems are printed.
import { readdirSync } from 'node:fs';
import { join } from 'node:path';

const ROOT = join(import.meta.dirname, '..');

/** What may be in the root of the repository. */
const ROOT_ENTRIES = new Set([
  '.editorconfig',
  '.git',
  '.gitattributes',
  '.github',
  '.gitignore',
  'CHANGELOG.md',
  'CODE_OF_CONDUCT.md',
  'CONTRIBUTING.md',
  'LICENSE',
  'README.md',
  'SECURITY.md',
  'SUPPORT.md',
  'TRADEMARKS.md',
  'docs',
  'helper',
  'tools',
]);

/** Folders that are built or installed, not committed. */
const SKIP = new Set(['.git', 'dist', 'node_modules']);

/** Files that belong to an extension project. [pattern for the path, what it is] */
const EXTENSION_FILES = [
  [/(^|\/)(src|public)\//, 'a folder of the extension (src/ or public/)'],
  [/(^|\/)(package|package-lock|manifest|tsconfig(\.[\w-]+)?)\.json$/, 'a file of the extension project'],
  [/(^|\/)vite\.config\.[\w]+$/, 'a build file of the extension'],
  [/\.(tsx?|jsx|crx|zip)$/i, 'extension code, a packed extension or an archive'],
];

const problems = [];
const found = [];

function walk(dir = '') {
  for (const entry of readdirSync(join(ROOT, dir), { withFileTypes: true })) {
    if (SKIP.has(entry.name)) continue;
    const rel = dir ? `${dir}/${entry.name}` : entry.name;
    if (!dir && !ROOT_ENTRIES.has(entry.name)) problems.push(`NOT HERE  ${rel}  is not part of the open source helper repository`);
    if (entry.isDirectory()) walk(rel);
    else found.push(rel);
  }
}
walk();

for (const rel of found) {
  for (const [pattern, what] of EXTENSION_FILES) {
    if (pattern.test(rel)) problems.push(`NOT HERE  ${rel}  looks like ${what}`);
  }
  if (rel.startsWith('tools/') && !/^tools\/[^/]+\.mjs$/.test(rel)) problems.push(`NOT HERE  ${rel}  tools/ holds only .mjs files`);
  if (rel.startsWith('docs/') && !/^docs\/([^/]+\.md|images\/[^/]+)$/.test(rel)) problems.push(`NOT HERE  ${rel}  docs/ holds only .md files and the images folder`);
}

if (problems.length) {
  for (const problem of [...new Set(problems)]) console.log(problem);
  console.log('\nOnly the helper program, its documents and its tools belong in this repository. The Chrome extension is closed source (docs/WHY-OPEN-SOURCE.md).');
  process.exit(1);
}
console.log(`Scope is right: ${found.length} files, all part of the open source helper, its documents and its tools.`);
