#!/usr/bin/env node

import fs from 'node:fs';
import http from 'node:http';
import path from 'node:path';
import { spawn, spawnSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';

const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const serverDir = path.join(repoRoot, 'src', 'server');
const webDir = path.join(repoRoot, 'src', 'web');
const nodeBin = process.execPath;
const skipE2E = process.argv.includes('--skip-e2e');

const localEnv = {
  ...process.env,
  COREPACK_HOME: process.env.COREPACK_HOME ?? path.join(repoRoot, '.tmp', 'corepack'),
  GOPATH: process.env.COMMERCIAL_LOCAL_GOPATH ?? path.join(repoRoot, '.tmp', 'gopath-v3'),
  GOCACHE: process.env.COMMERCIAL_LOCAL_GOCACHE ?? path.join(repoRoot, '.tmp', 'go-build-v3'),
  GOMODCACHE: process.env.COMMERCIAL_LOCAL_GOMODCACHE ?? path.join(repoRoot, '.tmp', 'go-mod-v3'),
  GOTOOLCHAIN: process.env.COMMERCIAL_LOCAL_GOTOOLCHAIN ?? 'auto',
  NODE_PATH: [path.join(repoRoot, 'node_modules'), process.env.NODE_PATH].filter(Boolean).join(path.delimiter),
  PLAYWRIGHT_BROWSERS_PATH:
    process.env.PLAYWRIGHT_BROWSERS_PATH ?? path.join(repoRoot, '.tmp', 'ms-playwright'),
};

for (const dir of [
  localEnv.COREPACK_HOME,
  localEnv.GOPATH,
  localEnv.GOCACHE,
  localEnv.GOMODCACHE,
  localEnv.PLAYWRIGHT_BROWSERS_PATH,
]) {
  fs.mkdirSync(dir, { recursive: true });
}

function fail(message) {
  console.error(`[commercial-local] ${message}`);
  process.exit(1);
}

function exists(...segments) {
  return fs.existsSync(path.join(repoRoot, ...segments));
}

function requireFile(label, ...segments) {
  const fullPath = path.join(repoRoot, ...segments);
  if (!fs.existsSync(fullPath)) {
    fail(`${label} is missing at ${path.relative(repoRoot, fullPath)}. Run pnpm install first.`);
  }
  return fullPath;
}

function commandWorks(command, args = ['--version']) {
  const result = spawnSync(command, args, { env: localEnv, encoding: 'utf8' });
  return result.status === 0;
}

function findGo() {
  const candidates = [
    process.env.GO_BIN,
    'go',
    'C:\\Progra~1\\Go\\bin\\go.exe',
    'C:\\Program Files\\Go\\bin\\go.exe',
    '/usr/local/go/bin/go',
  ].filter(Boolean);

  for (const candidate of candidates) {
    if (commandWorks(candidate, ['version'])) {
      return candidate;
    }
  }
  fail('Go is required for local commercial verification. Set GO_BIN if it is not on PATH.');
}

function run(label, command, args, options = {}) {
  console.log(`[commercial-local] START ${label}`);
  const result = spawnSync(command, args, {
    cwd: options.cwd ?? repoRoot,
    env: { ...localEnv, ...(options.env ?? {}) },
    stdio: 'inherit',
    shell: false,
  });
  if (result.status !== 0) {
    fail(`${label} failed with exit code ${result.status ?? 'unknown'}`);
  }
  console.log(`[commercial-local] PASS  ${label}`);
}

function runReport(label, command, args, options = {}) {
  console.log(`[commercial-local] REPORT ${label}`);
  const result = spawnSync(command, args, {
    cwd: options.cwd ?? repoRoot,
    env: { ...localEnv, ...(options.env ?? {}) },
    stdio: 'inherit',
    shell: false,
  });
  if (result.status !== 0) {
    fail(`${label} report failed with exit code ${result.status ?? 'unknown'}`);
  }
}

function checkExternal(label, command, args, options = {}) {
  const result = spawnSync(command, args, { env: localEnv, encoding: 'utf8' });
  if (result.status === 0) {
    console.log(`[commercial-local] STRICT-DEPENDENCY AVAILABLE ${label}`);
    return true;
  }
  if (options.missingHint) {
    console.log(`[commercial-local] STRICT-DEPENDENCY MISSING ${label}: ${options.missingHint}`);
    return false;
  }
  const detail =
    `${result.stderr ?? ''}${result.stdout ?? ''}`
      .replace(/\0/g, '')
      .replace(/[^\t\n\r -~]/g, '')
      .trim()
      .split(/\r?\n/)[0] ?? '';
  console.log(`[commercial-local] STRICT-DEPENDENCY MISSING ${label}${detail ? `: ${detail}` : ''}`);
  return false;
}

function requestOK(url) {
  return new Promise((resolve) => {
    const request = http.get(url, (response) => {
      response.resume();
      resolve(response.statusCode >= 200 && response.statusCode < 500);
    });
    request.on('error', () => resolve(false));
    request.setTimeout(1000, () => {
      request.destroy();
      resolve(false);
    });
  });
}

async function waitForURL(url, timeoutMS = 30000) {
  const deadline = Date.now() + timeoutMS;
  while (Date.now() < deadline) {
    if (await requestOK(url)) {
      return;
    }
    await new Promise((resolve) => setTimeout(resolve, 500));
  }
  fail(`Vite preview did not become ready at ${url}`);
}

async function runCommercialJourney(playwrightCLI, viteCLI) {
  if (skipE2E) {
    console.log('[commercial-local] SKIP  browser commercial journey (--skip-e2e)');
    return;
  }

  console.log('[commercial-local] START browser preview');
  const preview = spawn(
    nodeBin,
    [viteCLI, 'preview', '--host', '127.0.0.1', '--port', '4173'],
    {
      cwd: webDir,
      env: localEnv,
      stdio: ['ignore', 'pipe', 'pipe'],
      shell: false,
    },
  );

  preview.stdout.on('data', (chunk) => process.stdout.write(`[commercial-local:web] ${chunk}`));
  preview.stderr.on('data', (chunk) => process.stderr.write(`[commercial-local:web] ${chunk}`));

  try {
    await waitForURL('http://127.0.0.1:4173/');
    console.log('[commercial-local] PASS  browser preview');
    run(
      'browser commercial journey',
      nodeBin,
      [
        playwrightCLI,
        'test',
        '--grep',
        'commercial journey covers onboarding',
      ],
      {
        cwd: webDir,
        env: { PLAYWRIGHT_SKIP_WEB_SERVER: 'true' },
      },
    );
  } finally {
    preview.kill();
  }
}

const goBin = findGo();
const vitestCLI = requireFile('Vitest CLI', 'node_modules', 'vitest', 'vitest.mjs');
const tscCLI = requireFile('TypeScript CLI', 'node_modules', 'typescript', 'bin', 'tsc');
const viteCLI = requireFile('Vite CLI', 'node_modules', 'vite', 'bin', 'vite.js');
const playwrightCLI = requireFile('Playwright CLI', 'node_modules', '@playwright', 'test', 'cli.js');
const commercialPreflight = requireFile('Commercial preflight verifier', 'scripts', 'verify-commercial-preflight.mjs');

console.log('[commercial-local] Local verifier only. This is partial evidence, not final commercial readiness.');

run('server Go suite', goBin, ['test', '-p', '1', './...', '-count=1'], { cwd: serverDir });
run('frontend Vitest suite', nodeBin, [vitestCLI, 'run'], { cwd: webDir });
run('frontend TypeScript gate', nodeBin, [tscCLI, '--noEmit'], { cwd: webDir });
run('frontend production build', nodeBin, [viteCLI, 'build'], { cwd: webDir });
await runCommercialJourney(playwrightCLI, viteCLI);

console.log('[commercial-local] STRICT READINESS CHECKS NOT PROVEN BY THIS SCRIPT');
runReport('strict final preflight report', nodeBin, [commercialPreflight, '--local']);
checkExternal('WSL/bash strict verifier runtime', 'bash', ['--version'], {
  missingHint: 'bash exited non-zero; install Git Bash or configure a default WSL distribution for the strict verifier',
});
checkExternal('Docker daemon for DB/deploy/backup gates', 'docker', ['ps']);
console.log('[commercial-local] Required final gate remains: TEST_DATABASE_URL, OBLIVIOUS_TARGET_ARTIFACT_DIR, and COMMERCIAL_COMPLETION_RUN_DEPLOY=true COMMERCIAL_COMPLETION_RUN_K8S=true COMMERCIAL_COMPLETION_RUN_BACKUP_RESTORE=true COMMERCIAL_COMPLETION_RUN_TARGET_EVIDENCE=true bash scripts/verify-commercial-completion.sh');
console.log('[commercial-local] RESULT: local executable commercial gates passed; final release evidence remains external/no-skip strict verifier work.');
