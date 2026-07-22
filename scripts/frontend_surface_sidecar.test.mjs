import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { after, describe, it } from 'node:test';
import assert from 'node:assert/strict';

import { extractSidecar } from './frontend_surface_sidecar.mjs';

const repositoryRoot = path.resolve(path.dirname(new URL(import.meta.url).pathname), '..');
const fixtureRoot = path.join(repositoryRoot, 'scripts/testdata/frontend-surface/production');
const fixtureConfig = path.join(fixtureRoot, 'tsconfig.json');
const fixtureGenerated = path.join(fixtureRoot, 'generated/client.generated.ts');
const counts = {
  httpClient: 0,
  rawFetch: 0,
  swr: 0,
  multipartUpload: 0,
  sseStream: 0,
  eventSource: 0,
  websocket: 0,
  exposure: 0,
  generatedConsumer: 0,
  exclusion: 0,
  deterministic: 0,
  zeroInventory: 0,
  invalidConfig: 0,
  genericOnlyIdentity: 0,
  unresolvedDecoder: 0,
  malformedGenerated: 0,
  methodMismatch: 0,
  pathMismatch: 0
};

function fixtureOptions() {
  return { root: fixtureRoot, tsconfig: fixtureConfig, generatedFile: fixtureGenerated };
}

describe('frontend surface sidecar', () => {
  it('classifies every transport taxonomy through exact generated contracts', () => {
    const result = extractSidecar(fixtureOptions());
    const kindCounts = new Map();
    for (const operation of result.operations) {
      kindCounts.set(operation.transport.kind, (kindCounts.get(operation.transport.kind) ?? 0) + 1);
    }
    for (const [kind, counter] of [
      ['http-client', 'httpClient'],
      ['raw-fetch', 'rawFetch'],
      ['swr', 'swr'],
      ['multipart-upload', 'multipartUpload'],
      ['sse-stream', 'sseStream'],
      ['event-source', 'eventSource'],
      ['websocket', 'websocket']
    ]) {
      assert.ok((kindCounts.get(kind) ?? 0) > 0, `missing taxonomy ${kind}`);
      counts[counter] += kindCounts.get(kind);
    }
    assert.equal(result.unresolved.length, 0);
    assert.equal(result.operations.length, 8);
  });

  it('records product exposure and generated consumer evidence in the same program', () => {
    const result = extractSidecar(fixtureOptions());
    assert.ok(result.sourceScope.filesScanned > 0);
    assert.ok(result.exposures.length > 0);
    assert.ok(result.generatedConsumers > 0);
    assert.match(result.sourceScope.sourceDigest, /^sha256:[0-9a-f]{64}$/);
    counts.exposure += 1;
    counts.generatedConsumer += 1;
  });

  it('is byte deterministic for the same source and compiler configuration', () => {
    const first = JSON.stringify(extractSidecar(fixtureOptions()));
    const second = JSON.stringify(extractSidecar(fixtureOptions()));
    assert.equal(first, second);
    counts.deterministic += 1;
  });

  it('excludes test sources and generated transport calls from production inventory', () => {
    const result = extractSidecar({
      root: path.dirname(fixtureRoot),
      tsconfig: fixtureConfig,
      generatedFile: fixtureGenerated
    });
    assert.equal(result.sourceScope.filesScanned, 1);
    assert.equal(result.operations.length, 8);
    assert.equal(result.operations.some((entry) => entry.source.file.includes('excluded.test')), false);
    assert.equal(result.operations.some((entry) => entry.source.file.includes('.generated.')), false);
    counts.exclusion += 1;
  });

  it('rejects empty source inventory and invalid compiler configuration', () => {
    const emptyRoot = fs.mkdtempSync(path.join(os.tmpdir(), 'oblivious-sidecar-empty-'));
    try {
      assert.throws(() => extractSidecar({ root: emptyRoot, tsconfig: fixtureConfig, generatedFile: fixtureGenerated }), /source_inventory_empty/);
      counts.zeroInventory += 1;
      assert.throws(() => extractSidecar({ root: fixtureRoot, tsconfig: path.join(emptyRoot, 'missing.json'), generatedFile: fixtureGenerated }), /tsconfig_missing/);
      counts.invalidConfig += 1;
    } finally {
      fs.rmSync(emptyRoot, { recursive: true, force: true });
    }
  });

  it('records exact-symbol, decoder, generated, method, and path mutations as failures', () => {
    const cases = [
      {
        counter: 'genericOnlyIdentity',
        mutate: (source) => source.replace(
          "client.get('/fixture/users', undefined, listTransport);",
          "client.get<{ users: unknown[] }>('/fixture/users');"
        ),
        code: 'transport_contract_unresolved'
      },
      {
        counter: 'unresolvedDecoder',
        mutate: (source) => source
          .replace('declare function rawResponseDecoder(operation: unknown, status: number): unknown;', 'declare function unknownDecoder(operation: unknown, status: number): unknown;')
          .replace('responseDecoder: rawResponseDecoder(streamOperationContract, 200)', 'responseDecoder: unknownDecoder(streamOperationContract, 200)'),
        code: 'response_decoder_unresolved'
      },
      {
        counter: 'methodMismatch',
        mutate: (source) => source.replace(
          "client.get('/fixture/users', undefined, listTransport);",
          "client.post('/fixture/users', undefined, undefined, listTransport);"
        ),
        code: 'transport_method_mismatch'
      },
      {
        counter: 'pathMismatch',
        mutate: (source) => source.replace("client.get('/fixture/users'", "client.get('/fixture/wrong'"),
        code: 'transport_path_mismatch'
      }
    ];
    for (const fixtureCase of cases) {
      withMutatedFixture(fixtureCase.mutate, (options) => {
        const result = extractSidecar(options);
        assert.ok(result.unresolved.some((entry) => entry.code === fixtureCase.code), fixtureCase.code);
        counts[fixtureCase.counter] += 1;
      });
    }

    withMutatedFixture(
      (source) => source,
      (options) => {
        const generated = fs.readFileSync(options.generatedFile, 'utf8')
          .replace("operationId: 'listUsers'", "operationIdMissing: 'listUsers'");
        fs.writeFileSync(options.generatedFile, generated, 'utf8');
        const result = extractSidecar(options);
        assert.ok(
          result.unresolved.some((entry) => ['generated_operation_metadata_unresolved', 'transport_operation_identity_unresolved'].includes(entry.code)),
          JSON.stringify(result.unresolved)
        );
        counts.malformedGenerated += 1;
      }
    );
  });
});

function withMutatedFixture(mutate, assertion) {
  const temporaryRoot = fs.mkdtempSync(path.join(os.tmpdir(), 'oblivious-sidecar-mutation-'));
  try {
    fs.cpSync(fixtureRoot, temporaryRoot, { recursive: true });
    const sourcePath = path.join(temporaryRoot, 'transports.ts');
    fs.writeFileSync(sourcePath, mutate(fs.readFileSync(sourcePath, 'utf8')), 'utf8');
    assertion({
      root: temporaryRoot,
      tsconfig: path.join(temporaryRoot, 'tsconfig.json'),
      generatedFile: path.join(temporaryRoot, 'generated/client.generated.ts')
    });
  } finally {
    fs.rmSync(temporaryRoot, { recursive: true, force: true });
  }
}

after(() => {
  const destination = process.env.FRONTEND_SIDECAR_TEST_COUNTS;
  if (destination) fs.writeFileSync(destination, `${JSON.stringify(counts)}\n`, 'utf8');
});
