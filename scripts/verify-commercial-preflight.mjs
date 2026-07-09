#!/usr/bin/env node

import fs from 'node:fs';
import crypto from 'node:crypto';
import path from 'node:path';
import { spawnSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';

const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const finalMode = !process.argv.includes('--local');
const targetEvidenceOnly = process.argv.includes('--target-evidence-only');
const results = [];
let targetEvidenceManifest = null;
let targetEvidenceManifestPath = '';

const requiredArtifactKinds = [
  'strict-verifier-log',
  'deployment-log',
  'kubernetes-validation',
  'workflow-telemetry',
  'request-log-observability',
  'rag-indexing-proof',
  'relay-realtime-proof',
  'relay-batch-proof',
  'marketplace-payout-proof',
  'marketplace-governance-proof',
  'provider-runtime-config',
  'grpc-smoke-report',
  'secret-audit',
  'microservice-database-proof',
];
const requiredProviderArtifacts = ['stripe', 'alipay', 'wechatpay'];

function scrub(value) {
  return String(value ?? '')
    .replace(/\0/g, '')
    .replace(/[^\t\n\r -~]/g, '')
    .replace(/([a-z][a-z0-9+.-]*:\/\/[^:\s/@]+:)[^@\s/]+@/gi, '$1<redacted>@')
    .replace(/([?&#](?:token|password|pass|api[_-]?key|secret|signature)=)[^&#\s]*/gi, '$1<redacted>')
    .trim();
}

function firstLine(value) {
  return scrub(value).split(/\r?\n/).find(Boolean) ?? '';
}

function record(status, label, detail = '') {
  results.push({ status, label, detail });
  const suffix = detail ? ` - ${detail}` : '';
  console.log(`[commercial-preflight] ${status.padEnd(4)} ${label}${suffix}`);
}

function pass(label, detail) {
  record('PASS', label, detail);
}

function warn(label, detail) {
  record('WARN', label, detail);
}

function fail(label, detail) {
  record('FAIL', label, detail);
}

function repoPath(...segments) {
  return path.join(repoRoot, ...segments);
}

function pathExists(label, ...segments) {
  const fullPath = repoPath(...segments);
  if (fs.existsSync(fullPath)) {
    pass(label, path.relative(repoRoot, fullPath));
    return true;
  }
  fail(label, `missing ${path.relative(repoRoot, fullPath)}`);
  return false;
}

function run(command, args = [], options = {}) {
  try {
    return spawnSync(command, args, {
      cwd: options.cwd ?? repoRoot,
      env: process.env,
      encoding: 'utf8',
      timeout: options.timeout ?? 15000,
      windowsHide: true,
    });
  } catch (error) {
    return { status: null, error };
  }
}

function commandDetail(result) {
  if (result?.error) {
    return result.error.message;
  }
  return firstLine(`${result?.stderr ?? ''}${result?.stdout ?? ''}`);
}

function checkCommand(label, command, args, hint) {
  const result = run(command, args);
  if (result.status === 0) {
    pass(label);
    return true;
  }
  fail(label, hint ?? commandDetail(result) ?? `${command} failed`);
  return false;
}

function checkAnyCommand(label, candidates, args) {
  const failures = [];
  for (const command of candidates.filter(Boolean)) {
    const result = run(command, args);
    if (result.status === 0) {
      pass(label, command);
      return command;
    }
    failures.push(`${command}: ${commandDetail(result)}`);
  }
  fail(label, failures.find(Boolean) ?? `${label} command not found`);
  return '';
}

function isInside(child, parent) {
  const relative = path.relative(path.resolve(parent), path.resolve(child));
  return relative === '' || (!relative.startsWith('..') && !path.isAbsolute(relative));
}

function envIsTrue(name) {
  return String(process.env[name] ?? '').toLowerCase() === 'true';
}

function requireEnv(name, detail = 'required for strict final readiness') {
  if (String(process.env[name] ?? '').trim()) {
    pass(`env ${name}`, 'configured');
    return true;
  }
  fail(`env ${name}`, detail);
  return false;
}

function requireTrueEnv(name) {
  if (envIsTrue(name)) {
    pass(`env ${name}`, 'true');
    return true;
  }
  fail(`env ${name}`, 'must be true for strict final readiness');
  return false;
}

function rejectTrueEnv(name) {
  if (envIsTrue(name)) {
    fail(`env ${name}`, 'must not be true for strict final readiness');
    return false;
  }
  pass(`env ${name}`, 'not enabled');
  return true;
}

function checkBash() {
  const result = run('bash', ['--version']);
  if (result.status === 0) {
    pass('bash strict verifier runtime');
    return;
  }

  const gitBash = 'C:\\Program Files\\Git\\bin\\bash.exe';
  if (fs.existsSync(gitBash) && run(gitBash, ['--version']).status === 0) {
    fail(
      'bash strict verifier runtime',
      `bare bash exited non-zero; Git Bash exists at ${gitBash} but is not the command used by the canonical strict gate`,
    );
    return;
  }

  fail('bash strict verifier runtime', 'bare bash exited non-zero; install Git Bash or configure a default WSL distribution');
}

function checkPackageRunner() {
  if (run('pnpm', ['--version']).status === 0) {
    pass('pnpm package runner');
    return;
  }
  if (run('corepack', ['--version']).status === 0) {
    pass('pnpm package runner', 'corepack available');
    return;
  }
  const windowsCorepack = 'C:\\Program Files\\nodejs\\node_modules\\corepack\\dist\\corepack.js';
  if (fs.existsSync(windowsCorepack)) {
    pass('pnpm package runner', 'Windows corepack script available');
    return;
  }
  fail('pnpm package runner', 'pnpm or corepack is required by the strict verifier');
}

function checkDocker() {
  checkCommand('Docker daemon for deploy and DB gates', 'docker', ['ps'], 'docker ps must succeed');
}

function checkKubectl() {
  if (!checkCommand('kubectl command', 'kubectl', ['version', '--client=true'], 'kubectl is required for Kubernetes validation')) {
    return;
  }
  checkCommand('kubectl current context', 'kubectl', ['config', 'current-context'], 'kubectl current-context must be set');
  checkCommand('kubectl reachable cluster', 'kubectl', ['cluster-info'], 'kubectl cluster-info must reach the target cluster');
}

function checkK8sSecret() {
  const raw = String(process.env.OBLIVIOUS_K8S_SECRET_FILE ?? '').trim();
  if (!raw) {
    fail('Kubernetes secret file', 'OBLIVIOUS_K8S_SECRET_FILE is required');
    return;
  }
  const fullPath = path.resolve(raw);
  if (!fs.existsSync(fullPath)) {
    fail('Kubernetes secret file', `not found: ${fullPath}`);
    return;
  }
  const examplePath = repoPath('deploy', 'kubernetes', 'secret.example.yaml');
  if (path.resolve(fullPath).toLowerCase() === path.resolve(examplePath).toLowerCase()) {
    fail('Kubernetes secret file', 'refusing deploy/kubernetes/secret.example.yaml as runtime proof');
    return;
  }
  if (isInside(fullPath, repoRoot)) {
    fail('Kubernetes secret file', 'strict final readiness requires an external, untracked secret file outside the repository');
    return;
  }
  const body = fs.readFileSync(fullPath, 'utf8');
  if (/REPLACE_ME|CHANGE_ME|change-me-in-production/i.test(body)) {
    fail('Kubernetes secret file', 'contains placeholder values');
    return;
  }
  pass('Kubernetes secret file', 'external filled file');
}

function readJSONFile(filePath) {
  try {
    return JSON.parse(fs.readFileSync(filePath, 'utf8'));
  } catch (error) {
    fail('target evidence JSON', error.message);
    return null;
  }
}

function currentCommit() {
  const result = run('git', ['rev-parse', 'HEAD']);
  if (result.status !== 0) {
    return '';
  }
  return firstLine(result.stdout);
}

function likelyManifestCommit(manifest) {
  return (
    manifest?.commit ??
    manifest?.gitCommit ??
    manifest?.currentCommit ??
    manifest?.release?.commit ??
    manifest?.metadata?.commit ??
    ''
  );
}

function checkTargetEvidence() {
  const raw = String(process.env.OBLIVIOUS_TARGET_EVIDENCE_FILE ?? '').trim();
  if (!raw) {
    fail('target evidence manifest', 'OBLIVIOUS_TARGET_EVIDENCE_FILE is required');
    return;
  }
  const fullPath = path.resolve(raw);
  if (!fs.existsSync(fullPath)) {
    fail('target evidence manifest', `not found: ${fullPath}`);
    return;
  }
  if (isInside(fullPath, repoRoot)) {
    fail('target evidence manifest', 'strict final readiness requires an external, untracked target evidence manifest outside the repository');
    return;
  }
  const body = fs.readFileSync(fullPath, 'utf8');
  if (/\b(TODO|REPLACE_ME|CHANGE_ME)\b/i.test(body)) {
    fail('target evidence manifest', 'contains placeholder markers');
    return;
  }
  const manifest = readJSONFile(fullPath);
  if (!manifest) {
    return;
  }
  targetEvidenceManifest = manifest;
  targetEvidenceManifestPath = fullPath;
  const head = currentCommit();
  const manifestCommit = likelyManifestCommit(manifest);
  if (head && manifestCommit && head !== manifestCommit) {
    fail('target evidence manifest', `commit mismatch: manifest ${manifestCommit}, repo ${head}`);
    return;
  }
  if (head && !manifestCommit) {
    warn('target evidence manifest', 'JSON parsed, commit field will be enforced by verify-target-release-evidence.sh');
    return;
  }
  pass('target evidence manifest', 'JSON parsed');
}

function safeArtifactID(value) {
  return typeof value === 'string' && /^[A-Za-z0-9_.-]+$/.test(value);
}

function artifactSHA256(filePath) {
  return crypto.createHash('sha256').update(fs.readFileSync(filePath)).digest('hex');
}

function checkArtifactDir() {
  const raw = String(process.env.OBLIVIOUS_TARGET_ARTIFACT_DIR ?? '').trim();
  if (!raw) {
    fail('target artifact body directory', 'OBLIVIOUS_TARGET_ARTIFACT_DIR is required for strict final readiness');
    return;
  }
  const fullPath = path.resolve(raw);
  if (!fs.existsSync(fullPath) || !fs.statSync(fullPath).isDirectory()) {
    fail('target artifact body directory', `not a directory: ${fullPath}`);
    return;
  }
  if (isInside(fullPath, repoRoot)) {
    fail('target artifact body directory', 'strict final readiness requires an external, untracked artifact body directory outside the repository');
    return;
  }
  const files = fs.readdirSync(fullPath).filter((entry) => fs.statSync(path.join(fullPath, entry)).isFile());
  if (files.length === 0) {
    fail('target artifact body directory', 'directory exists but contains no artifact body files');
    return;
  }
  pass('target artifact body directory', `${files.length} file(s)`);

  if (!targetEvidenceManifest) {
    warn('target artifact body coverage', 'manifest was not available for artifact body coverage checks');
    return;
  }
  const artifacts = targetEvidenceManifest.artifacts;
  if (!Array.isArray(artifacts) || artifacts.length === 0) {
    fail('target artifact body coverage', 'manifest artifacts[] is missing or empty');
    return;
  }

  const missingFamilies = new Set(requiredArtifactKinds);
  const missingProviders = new Set(requiredProviderArtifacts);
  const failures = [];
  for (const artifact of artifacts) {
    if (!artifact || typeof artifact !== 'object') {
      failures.push('manifest contains a non-object artifact entry');
      continue;
    }
    const artifactID = artifact.id;
    if (!safeArtifactID(artifactID)) {
      failures.push(`artifact id is unsafe or missing: ${scrub(artifactID)}`);
      continue;
    }
    if (typeof artifact.kind === 'string') {
      missingFamilies.delete(artifact.kind);
    }
    if (artifact.kind === 'provider-live-rail' && typeof artifact.provider === 'string') {
      missingProviders.delete(artifact.provider);
    }

    const bodyPath = path.join(fullPath, `${artifactID}.json`);
    if (!fs.existsSync(bodyPath)) {
      failures.push(`${artifactID}.json is missing`);
      continue;
    }
    if (typeof artifact.sha256 !== 'string' || !/^[0-9a-f]{64}$/i.test(artifact.sha256)) {
      failures.push(`${artifactID} manifest sha256 is missing or invalid`);
      continue;
    }
    const actualSHA256 = artifactSHA256(bodyPath);
    if (actualSHA256 !== artifact.sha256.toLowerCase()) {
      failures.push(`${artifactID}.json sha256 mismatch`);
      continue;
    }
    const body = readJSONFile(bodyPath);
    if (!body) {
      failures.push(`${artifactID}.json is not valid JSON`);
      continue;
    }
    if (body.artifactId !== artifactID || body.kind !== artifact.kind) {
      failures.push(`${artifactID}.json lineage does not match manifest id/kind`);
    }
  }

  for (const kind of missingFamilies) {
    failures.push(`manifest missing artifact kind ${kind}`);
  }
  for (const provider of missingProviders) {
    failures.push(`manifest missing provider-live-rail artifact for ${provider}`);
  }

  if (failures.length > 0) {
    fail('target artifact body coverage', failures.slice(0, 4).join('; '));
    if (failures.length > 4) {
      warn('target artifact body coverage', `${failures.length - 4} additional issue(s) not shown`);
    }
    return;
  }
  pass(
    'target artifact body coverage',
    `${artifacts.length} artifact body file(s) match ${path.relative(repoRoot, targetEvidenceManifestPath) || targetEvidenceManifestPath}`,
  );
}

function checkTargetEvidenceVerifier() {
  if (!targetEvidenceManifestPath) {
    fail('target evidence verifier', 'OBLIVIOUS_TARGET_EVIDENCE_FILE must be a valid manifest before verifier execution');
    return;
  }
  const artifactDir = String(process.env.OBLIVIOUS_TARGET_ARTIFACT_DIR ?? '').trim();
  if (!artifactDir) {
    fail('target evidence verifier', 'OBLIVIOUS_TARGET_ARTIFACT_DIR is required before verifier execution');
    return;
  }
  const bashCommand = process.env.BASH || 'bash';
  const result = run(bashCommand, [repoPath('scripts', 'verify-target-release-evidence.sh'), targetEvidenceManifestPath], {
    timeout: 60000,
  });
  if (result.status === 0) {
    pass('target evidence verifier', 'manifest and artifact bodies passed full target verifier');
    return;
  }
  const verifierLines = scrub(`${result?.stderr ?? ''}${result?.stdout ?? ''}`)
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter(Boolean);
  const diagnostic =
    verifierLines.find((line) => !line.includes('[target-release-evidence] FAIL')) ??
    verifierLines[0] ??
    commandDetail(result) ??
    'verify-target-release-evidence.sh failed';
  fail('target evidence verifier', diagnostic);
}

function checkGrpcSmokeDeliverable() {
  const dockerfile = repoPath('Dockerfile.server');
  if (!fs.existsSync(dockerfile)) {
    fail('gRPC smoke deliverable', 'Dockerfile.server missing');
    return;
  }
  const body = fs.readFileSync(dockerfile, 'utf8');
  if (!body.includes('oblivious-grpc-smoke')) {
    fail('gRPC smoke deliverable', 'Dockerfile.server must package oblivious-grpc-smoke for target smoke without a Go toolchain');
    return;
  }
  pass('gRPC smoke deliverable', 'Docker image packages oblivious-grpc-smoke');
}

function finishPreflight() {
  const failures = results.filter((result) => result.status === 'FAIL');
  const warnings = results.filter((result) => result.status === 'WARN');
  console.log(`[commercial-preflight] SUMMARY pass=${results.length - failures.length - warnings.length} warn=${warnings.length} fail=${failures.length}`);
  if (failures.length > 0) {
    console.log('[commercial-preflight] RESULT: strict final commercial preflight failed.');
    process.exit(finalMode ? 1 : 0);
  }

  console.log('[commercial-preflight] RESULT: strict final commercial preflight passed.');
}

if (targetEvidenceOnly) {
  console.log('[commercial-preflight] Checking target evidence artifact body prerequisites.');
} else {
  console.log('[commercial-preflight] Checking strict final commercial release prerequisites.');
}
if (!finalMode) {
  console.log('[commercial-preflight] --local mode still reports final blockers but does not fail the process.');
}

const commonPaths = [
  ['strict verifier script', ['scripts', 'verify-commercial-completion.sh']],
  ['target evidence verifier', ['scripts', 'verify-target-release-evidence.sh']],
  ['target evidence implementation', ['scripts', 'verify_target_release_evidence.py']],
];
const fullPreflightPaths = [
  ['target evidence assembler', ['scripts', 'assemble-target-release-evidence.sh']],
  ['target gRPC smoke script', ['scripts', 'target-grpc-smoke.sh']],
  ['server Dockerfile', ['Dockerfile.server']],
];

for (const [label, segments] of targetEvidenceOnly ? commonPaths : [...commonPaths, ...fullPreflightPaths]) {
  pathExists(label, ...segments);
}

if (targetEvidenceOnly) {
  requireTrueEnv('COMMERCIAL_COMPLETION_RUN_TARGET_EVIDENCE');
  rejectTrueEnv('COMMERCIAL_COMPLETION_ALLOW_ENV_SKIPS');
  rejectTrueEnv('OBLIVIOUS_TARGET_EVIDENCE_ALLOW_COMMIT_MISMATCH');
  checkTargetEvidence();
  checkArtifactDir();
  checkTargetEvidenceVerifier();
  finishPreflight();
  process.exit(0);
}

checkBash();
checkAnyCommand('Go toolchain', [process.env.GO_BIN, 'go', 'C:\\Progra~1\\Go\\bin\\go.exe', 'C:\\Program Files\\Go\\bin\\go.exe'], ['version']);
checkAnyCommand('Python runtime for target evidence', [process.env.PYTHON, 'python', 'python3'], ['--version']);
pass('Node.js runtime', process.version);
checkPackageRunner();
checkDocker();
checkKubectl();

requireEnv('TEST_DATABASE_URL');
for (const name of [
  'COMMERCIAL_COMPLETION_RUN_DEPLOY',
  'COMMERCIAL_COMPLETION_RUN_K8S',
  'COMMERCIAL_COMPLETION_RUN_BACKUP_RESTORE',
  'COMMERCIAL_COMPLETION_RUN_TARGET_EVIDENCE',
]) {
  requireTrueEnv(name);
}
rejectTrueEnv('COMMERCIAL_COMPLETION_ALLOW_ENV_SKIPS');
rejectTrueEnv('OBLIVIOUS_TARGET_EVIDENCE_ALLOW_COMMIT_MISMATCH');

checkK8sSecret();
checkTargetEvidence();
checkArtifactDir();
checkTargetEvidenceVerifier();
checkGrpcSmokeDeliverable();
finishPreflight();
