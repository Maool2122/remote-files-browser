// Builds the helper program for every system.
//
//   node helper/build.mjs            all systems
//   node helper/build.mjs windows    only the systems of one kind (windows, darwin or linux)
//
// Needs Go 1.26 or newer (https://go.dev/dl/). The programs are written to helper/dist/, with a file of SHA-256 sums.
// They are not signed: a signed Windows program and a notarized Mac program need the publisher's certificates (see
// docs/BUILDING.md in this folder). Nothing here uploads anything.
import { spawnSync } from 'node:child_process';
import { createHash } from 'node:crypto';
import { mkdirSync, readFileSync, readdirSync, rmSync, writeFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const here = dirname(fileURLToPath(import.meta.url));
const dist = join(here, 'dist');

const targets = [
  { os: 'windows', arch: 'amd64' },
  { os: 'windows', arch: 'arm64' },
  { os: 'darwin', arch: 'arm64' },
  { os: 'darwin', arch: 'amd64' },
  { os: 'linux', arch: 'amd64' },
  { os: 'linux', arch: 'arm64' },
];

const only = process.argv[2];
const chosen = targets.filter((t) => !only || t.os === only);
if (chosen.length === 0) {
  console.error(`Unknown system "${only}". Use windows, darwin or linux.`);
  process.exit(2);
}

rmSync(dist, { recursive: true, force: true });
mkdirSync(dist, { recursive: true });

for (const target of chosen) {
  const file = `rocuo-files-helper-${target.os}-${target.arch}${target.os === 'windows' ? '.exe' : ''}`;
  // windowsgui: no console window flashes up when Chrome starts the helper, and a double-click shows a message box.
  const flags = target.os === 'windows' ? '-s -w -H=windowsgui' : '-s -w';
  // -trimpath: no folder names in the program. -buildvcs=false: no commit stamp, so the same source gives the same file.
  const result = spawnSync('go', ['build', '-trimpath', '-buildvcs=false', '-ldflags', flags, '-o', join(dist, file), '.'], {
    cwd: here,
    stdio: 'inherit',
    env: { ...process.env, GOOS: target.os, GOARCH: target.arch, CGO_ENABLED: '0' },
  });
  if (result.status !== 0) {
    console.error(`The build for ${target.os}/${target.arch} failed.`);
    process.exit(result.status ?? 1);
  }
  console.log(`built ${file}`);
}

const sums = readdirSync(dist)
  .filter((name) => name.startsWith('rocuo-files-helper-'))
  .sort()
  .map((name) => `${createHash('sha256').update(readFileSync(join(dist, name))).digest('hex')}  ${name}`);
writeFileSync(join(dist, 'SHA256SUMS.txt'), sums.join('\n') + '\n');
console.log(`\n${sums.join('\n')}`);
