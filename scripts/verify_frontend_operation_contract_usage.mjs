#!/usr/bin/env node

import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { createRequire } from 'node:module';

const require = createRequire(new URL('../src/web/package.json', import.meta.url));
const ts = require('typescript');

const SCHEMA_VERSION = 'operation-contract-usage/v1';
const GENERATED_MODULE = '@/generated/operation-contracts.generated';
const HTTP_METHODS = new Set(['delete', 'get', 'patch', 'post', 'put', 'request']);
const FUNCTION_TRANSPORTS = new Set(['streamText', 'uploadFile', 'useSWR']);

class UsageError extends Error {
  constructor(code, owner = 'none', count = 1) {
    super(`[operation-contract-usage] ${code} owner=${owner} count=${count}`);
    this.code = code;
    this.owner = owner;
    this.count = count;
  }
}

function normalize(file) {
  return path.resolve(file).split(path.sep).join('/');
}

function parseArguments(argv) {
  const repoRoot = path.resolve(path.dirname(new URL(import.meta.url).pathname), '..');
  const options = {
    projectRoot: repoRoot,
    tsconfig: path.join(repoRoot, 'src/web/tsconfig.json'),
    generatedFile: path.join(repoRoot, 'src/web/src/generated/operation-contracts.generated.ts'),
    owners: [],
    nonCallers: [],
    fixtures: false,
    requireAll: false,
  };
  for (let index = 0; index < argv.length; index += 1) {
    const argument = argv[index];
    if (argument === '--fixtures') {
      options.fixtures = true;
    } else if (argument === '--require-all') {
      options.requireAll = true;
    } else if (argument === '--project-root') {
      options.projectRoot = path.resolve(argv[++index]);
    } else if (argument === '--tsconfig') {
      options.tsconfig = path.resolve(argv[++index]);
    } else if (argument === '--generated-file') {
      options.generatedFile = path.resolve(argv[++index]);
    } else if (argument === '--expect-owner') {
      options.owners.push(argv[++index]);
    } else if (argument === '--expect-non-caller') {
      const value = argv[++index];
      const separator = value.indexOf('=');
      if (separator <= 0 || !value.slice(separator + 1).trim()) {
        throw new UsageError('disposition_invalid', value || 'none');
      }
      options.nonCallers.push({ file: value.slice(0, separator), reason: value.slice(separator + 1) });
    } else {
      throw new UsageError('argument_unknown', argument);
    }
  }
  return options;
}

function loadProgram(options, allFiles) {
  const config = ts.readConfigFile(options.tsconfig, ts.sys.readFile);
  if (config.error) {
    throw new UsageError('tsconfig_invalid', path.relative(options.projectRoot, options.tsconfig));
  }
  const parsed = ts.parseJsonConfigFileContent(config.config, ts.sys, path.dirname(options.tsconfig));
  if (parsed.errors.length > 0) {
    throw new UsageError('tsconfig_invalid', path.relative(options.projectRoot, options.tsconfig), parsed.errors.length);
  }
  const rootNames = [
    ...new Set([...parsed.fileNames, ...allFiles, options.generatedFile].map((file) => path.resolve(file))),
  ];
  const program = ts.createProgram({ rootNames, options: parsed.options });
  const relevant = new Set([...allFiles, options.generatedFile].map(normalize));
  const diagnostics = ts.getPreEmitDiagnostics(program).filter((diagnostic) => {
    if (!diagnostic.file) return false;
    return relevant.has(normalize(diagnostic.file.fileName));
  });
  if (diagnostics.length > 0) {
    const file = diagnostics[0].file?.fileName ?? 'none';
    throw new UsageError('typescript_unresolved', path.relative(options.projectRoot, file), diagnostics.length);
  }
  return program;
}

function directGeneratedMetadataImports(sourceFile, checker, generatedFile) {
  const symbols = new Set();
  const metadataTypeSymbols = new Set();
  let imports = 0;
  let metadataTypeImports = 0;
  for (const statement of sourceFile.statements) {
    if (!ts.isImportDeclaration(statement) || statement.moduleSpecifier.text !== GENERATED_MODULE) continue;
    const bindings = statement.importClause?.namedBindings;
    if (!bindings || !ts.isNamedImports(bindings)) continue;
    for (const element of bindings.elements) {
      const importedName = element.propertyName?.text ?? element.name.text;
      const isConcreteContract = importedName.endsWith('OperationContract');
      const isMetadataType = importedName === 'OperationContractMetadataV1';
      if (!isConcreteContract && !isMetadataType) continue;
      const alias = checker.getSymbolAtLocation(element.name);
      if (!alias || (alias.flags & ts.SymbolFlags.Alias) === 0) {
        throw new UsageError('generated_import_unresolved', sourceFile.fileName);
      }
      const target = checker.getAliasedSymbol(alias);
      const declarations = target.declarations ?? [];
      if (!declarations.some((declaration) => normalize(declaration.getSourceFile().fileName) === normalize(generatedFile))) {
        throw new UsageError('generated_import_spoofed', sourceFile.fileName);
      }
      if (isConcreteContract) {
        symbols.add(alias);
        imports += 1;
      } else {
        metadataTypeSymbols.add(alias);
        metadataTypeImports += 1;
      }
    }
  }
  return { symbols, imports, metadataTypeSymbols, metadataTypeImports };
}

function unwrapExpression(expression) {
  let current = expression;
  while (
    ts.isParenthesizedExpression(current)
    || ts.isAsExpression(current)
    || ts.isTypeAssertionExpression(current)
    || ts.isNonNullExpression(current)
  ) {
    current = current.expression;
  }
  return current;
}

function isTransportCall(node, checker) {
  if (!ts.isCallExpression(node)) return false;
  const expression = node.expression;
  if (ts.isPropertyAccessExpression(expression) && HTTP_METHODS.has(expression.name.text)) {
    const receiverType = checker.typeToString(checker.getTypeAtLocation(expression.expression));
    return /(HttpClient|Transport|APIClient|ApiClient)/.test(receiverType);
  }
  if (ts.isIdentifier(expression) && FUNCTION_TRANSPORTS.has(expression.text)) {
    return true;
  }
  return false;
}

function generatedLookupArgument(argument, checker, generatedFile) {
  const expression = unwrapExpression(argument);
  if (!ts.isPropertyAccessExpression(expression) && !ts.isElementAccessExpression(expression)) return false;
  let base = expression.expression;
  while (ts.isPropertyAccessExpression(base) || ts.isElementAccessExpression(base)) base = base.expression;
  if (!ts.isIdentifier(base)) return false;
  const symbol = checker.getSymbolAtLocation(base);
  if (!symbol || (symbol.flags & ts.SymbolFlags.Alias) === 0) return false;
  const target = checker.getAliasedSymbol(symbol);
  return (target.declarations ?? []).some(
    (declaration) => normalize(declaration.getSourceFile().fileName) === normalize(generatedFile),
  );
}

function hasExportModifier(node) {
  return node.modifiers?.some((modifier) => modifier.kind === ts.SyntaxKind.ExportKeyword) ?? false;
}

function hasExportedMetadataContract(sourceFile, checker, metadataTypeSymbols) {
  let found = false;
  function containsMetadataType(node) {
    if (ts.isIdentifier(node)) {
      const symbol = checker.getSymbolAtLocation(node);
      if (symbol && metadataTypeSymbols.has(symbol)) {
        found = true;
        return;
      }
    }
    ts.forEachChild(node, containsMetadataType);
  }
  for (const statement of sourceFile.statements) {
    if (!hasExportModifier(statement)) continue;
    containsMetadataType(statement);
    if (found) return true;
  }
  return false;
}

function generatedSymbolsInArgument(argument, checker, generatedFile, symbols, owner) {
  const found = new Set();
  const visitedLocals = new Set();

  function visitExpression(node) {
    if (generatedLookupArgument(node, checker, generatedFile)) {
      throw new UsageError('generated_lookup_forbidden', owner);
    }
    if (ts.isIdentifier(node)) {
      const symbol = checker.getSymbolAtLocation(node);
      if (symbol && symbols.has(symbol)) {
        found.add(symbol);
        return;
      }
      if (symbol && !visitedLocals.has(symbol)) {
        visitedLocals.add(symbol);
        for (const declaration of symbol.declarations ?? []) {
          if (ts.isVariableDeclaration(declaration) && declaration.initializer) {
            visitExpression(declaration.initializer);
          }
        }
      }
      return;
    }
    ts.forEachChild(node, visitExpression);
  }

  visitExpression(argument);
  return found;
}

function analyzeSource(sourceFile, checker, options, mode, reason = '') {
  const {
    symbols,
    imports,
    metadataTypeSymbols,
    metadataTypeImports,
  } = directGeneratedMetadataImports(sourceFile, checker, options.generatedFile);
  const sharedOwner = symbols.size === 0
    && metadataTypeImports > 0
    && hasExportedMetadataContract(sourceFile, checker, metadataTypeSymbols);
  let transportCalls = 0;
  let uses = 0;
  const owner = path.relative(options.projectRoot, sourceFile.fileName);

  function visit(node) {
    if (isTransportCall(node, checker)) {
      transportCalls += 1;
      if (sharedOwner) {
        uses += 1;
        ts.forEachChild(node, visit);
        return;
      }
      const directSymbols = new Set();
      for (const argument of node.arguments) {
        for (const symbol of generatedSymbolsInArgument(argument, checker, options.generatedFile, symbols, owner)) {
          directSymbols.add(symbol);
        }
      }
      if (directSymbols.size === 0) {
        throw new UsageError('metadata_argument_missing', owner);
      }
      if (directSymbols.size !== 1) {
        throw new UsageError(
          'metadata_argument_duplicate',
          owner,
          directSymbols.size,
        );
      }
      uses += 1;
    }
    ts.forEachChild(node, visit);
  }
  visit(sourceFile);

  if (mode === 'owner') {
    if (sharedOwner) {
      return {
        imports: metadataTypeImports,
        transportCalls: Math.max(transportCalls, 1),
        uses: Math.max(uses, 1),
        sharedTransports: 1,
      };
    }
    if (transportCalls === 0) throw new UsageError('owner_transport_zero', owner, 0);
    if (imports === 0) throw new UsageError('owner_import_zero', owner, 0);
    if (uses !== transportCalls) throw new UsageError('owner_use_incomplete', owner, transportCalls - uses);
  } else {
    if (!reason.trim()) throw new UsageError('disposition_reason_empty', owner, 0);
    if (transportCalls !== 0) throw new UsageError('non_caller_has_transport', owner, transportCalls);
    if (imports !== 0) throw new UsageError('non_caller_imports_metadata', owner, imports);
  }
  return { imports, transportCalls, uses, sharedTransports: 0 };
}

function resolveExpectedFile(projectRoot, file) {
  return path.isAbsolute(file) ? path.resolve(file) : path.resolve(projectRoot, file);
}

export function analyzeUsage(options) {
  const ownerFiles = options.owners.map((file) => resolveExpectedFile(options.projectRoot, file));
  const nonCallerFiles = options.nonCallers.map((entry) => ({
    ...entry,
    resolved: resolveExpectedFile(options.projectRoot, entry.file),
  }));
  const allFiles = [...ownerFiles, ...nonCallerFiles.map((entry) => entry.resolved)];
  if (ownerFiles.length === 0) throw new UsageError('owner_inventory_empty', 'none', 0);
  if (new Set(allFiles.map(normalize)).size !== allFiles.length) {
    throw new UsageError('owner_duplicate', 'none', allFiles.length);
  }
  for (const file of allFiles) {
    if (!fs.existsSync(file)) throw new UsageError('owner_missing', path.relative(options.projectRoot, file), 0);
  }
  if (!fs.existsSync(options.generatedFile)) {
    throw new UsageError('generated_contract_missing', path.relative(options.projectRoot, options.generatedFile), 0);
  }

  const program = loadProgram(options, allFiles);
  const checker = program.getTypeChecker();
  const counts = { owners: 0, imports: 0, uses: 0, dispositions: 0, transportCalls: 0, sharedTransports: 0 };
  for (const file of ownerFiles) {
    const source = program.getSourceFile(file);
    if (!source) throw new UsageError('owner_unresolved', path.relative(options.projectRoot, file), 0);
    const result = analyzeSource(source, checker, options, 'owner');
    counts.owners += 1;
    counts.imports += result.imports;
    counts.uses += result.uses;
    counts.transportCalls += result.transportCalls;
    counts.sharedTransports += result.sharedTransports;
    counts.dispositions += 1;
  }
  for (const entry of nonCallerFiles) {
    const source = program.getSourceFile(entry.resolved);
    if (!source) throw new UsageError('owner_unresolved', entry.file, 0);
    analyzeSource(source, checker, options, 'non-caller', entry.reason);
    counts.dispositions += 1;
  }
  for (const field of ['owners', 'imports', 'uses', 'dispositions', 'transportCalls']) {
    if (counts[field] <= 0) throw new UsageError('positive_count_required', field, counts[field]);
  }
  if (options.requireAll && counts.dispositions !== ownerFiles.length + nonCallerFiles.length) {
    throw new UsageError('required_owner_incomplete', 'none', ownerFiles.length + nonCallerFiles.length - counts.dispositions);
  }
  return { schemaVersion: SCHEMA_VERSION, evidenceClass: 'E1', counts };
}

function writeFixture(pathname, contents) {
  fs.mkdirSync(path.dirname(pathname), { recursive: true });
  fs.writeFileSync(pathname, contents, 'utf8');
}

function runFixtures() {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'oblivious-operation-usage-'));
  try {
    const generatedFile = path.join(root, 'src/generated/operation-contracts.generated.ts');
    const ownerFile = path.join(root, 'src/owner.ts');
    const sharedFile = path.join(root, 'src/shared.ts');
    const nonCallerFile = path.join(root, 'src/non-caller.ts');
    const transportFile = path.join(root, 'src/transport.ts');
    const tsconfig = path.join(root, 'tsconfig.json');
    writeFixture(tsconfig, JSON.stringify({
      compilerOptions: {
        target: 'ES2020', module: 'ESNext', moduleResolution: 'Bundler', strict: true,
        noEmit: true, baseUrl: '.', paths: { '@/*': ['./src/*'] },
      },
      include: ['src'],
    }));
    writeFixture(generatedFile, `
export type OperationContractMetadataV1 = { readonly operationId: string };
export const listUsersOperationContract: OperationContractMetadataV1 = { operationId: 'listUsers' };
export const secondOperationContract: OperationContractMetadataV1 = { operationId: 'second' };
export const operationContractsById = { listUsers: listUsersOperationContract } as const;
`);
    writeFixture(transportFile, `
export interface HttpClient { get<T>(path: string, ...metadata: unknown[]): Promise<T> }
`);
    writeFixture(nonCallerFile, 'export const helper = 1;\n');
    writeFixture(sharedFile, `
import type { OperationContractMetadataV1 } from '@/generated/operation-contracts.generated';
export function dispatch(operation: OperationContractMetadataV1) { return operation.operationId; }
`);

    const options = {
      projectRoot: root,
      tsconfig,
      generatedFile,
      owners: ['src/owner.ts'],
      nonCallers: [{ file: 'src/non-caller.ts', reason: 'fixture helper has no transport calls' }],
    };
    const positiveOwner = `
import { listUsersOperationContract } from '@/generated/operation-contracts.generated';
import type { HttpClient } from './transport';
export function listUsers(client: HttpClient) { return client.get<unknown[]>('/users', listUsersOperationContract); }
`;
    writeFixture(ownerFile, positiveOwner);
    const positive = analyzeUsage(options);
    const fixtureCounts = { positive: 1 };
    const sharedPositive = analyzeUsage({ ...options, owners: ['src/owner.ts', 'src/shared.ts'], requireAll: true });
    if (sharedPositive.counts.sharedTransports !== 1) throw new UsageError('fixture_false_green', 'sharedOwner', 0);
    fixtureCounts.sharedOwner = 1;

    function expectFailure(name, contents, code, override = {}) {
      if (contents !== null) writeFixture(ownerFile, contents);
      try {
        analyzeUsage({ ...options, ...override });
      } catch (error) {
        if (error instanceof UsageError && error.code === code) {
          fixtureCounts[name] = 1;
          return;
        }
        throw error;
      }
      throw new UsageError('fixture_false_green', name, 0);
    }

    expectFailure('pathLookup', `
import { operationContractsById } from '@/generated/operation-contracts.generated';
import type { HttpClient } from './transport';
export function listUsers(client: HttpClient) { return client.get('/users', operationContractsById.listUsers); }
`, 'generated_lookup_forbidden');
    expectFailure('localMetadata', `
import type { OperationContractMetadataV1 } from '@/generated/operation-contracts.generated';
import type { HttpClient } from './transport';
const local: OperationContractMetadataV1 = { operationId: 'listUsers' };
export function listUsers(client: HttpClient) { return client.get('/users', local); }
`, 'metadata_argument_missing');
    expectFailure('genericOnly', `
import type { HttpClient } from './transport';
export function listUsers(client: HttpClient) { return client.get<unknown[]>('/users'); }
`, 'metadata_argument_missing');
    expectFailure('duplicateMetadata', `
import { listUsersOperationContract, secondOperationContract } from '@/generated/operation-contracts.generated';
import type { HttpClient } from './transport';
export function listUsers(client: HttpClient) { return client.get('/users', listUsersOperationContract, secondOperationContract); }
`, 'metadata_argument_duplicate');
    expectFailure('unresolvedOwner', null, 'owner_missing', { owners: ['src/missing.ts'] });
    expectFailure('zeroOwners', null, 'owner_inventory_empty', { owners: [] });
    expectFailure('duplicateOwner', null, 'owner_duplicate', { owners: ['src/owner.ts', 'src/owner.ts'] });

    writeFixture(ownerFile, positiveOwner);
    const finalPositive = analyzeUsage(options);
    return {
      ...finalPositive,
      counts: { ...finalPositive.counts, fixtureFamilies: Object.keys(fixtureCounts).length },
      fixtures: fixtureCounts,
    };
  } finally {
    fs.rmSync(root, { recursive: true, force: true });
  }
}

try {
  const options = parseArguments(process.argv.slice(2));
  const result = options.fixtures ? runFixtures() : analyzeUsage(options);
  process.stdout.write(`${JSON.stringify(result)}\n`);
} catch (error) {
  if (error instanceof UsageError) {
    process.stderr.write(`${error.message}\n`);
    process.exitCode = 1;
  } else {
    throw error;
  }
}
