#!/usr/bin/env node

import fs from 'node:fs';
import path from 'node:path';
import crypto from 'node:crypto';
import { createRequire } from 'node:module';

const require = createRequire(new URL('../src/web/package.json', import.meta.url));
const ts = require('typescript');

export const SCHEMA_VERSION = 'frontend-surface-sidecar/v1';
const GENERATED_MODULE = '@/generated/operation-contracts.generated';
const SOURCE_EXTENSIONS = new Set(['.ts', '.tsx']);
const HTTP_METHODS = new Set(['request', 'get', 'post', 'put', 'delete', 'patch']);
const CATALOG_RESPONSE_DTOS = new Set(['ModelOption', 'AgentToolDefinition']);
const DTO_ROLES = new Map([
  ['AppCapabilityProjectionResponse', 'app-projection-response'],
  ['ModelOption', 'catalog-response'],
  ['AgentToolDefinition', 'catalog-response'],
  ['UpdateConversationConfigRequest', 'mutation-request'],
  ['CreateAgentRequest', 'mutation-request'],
  ['UpdateAgentRequest', 'mutation-request'],
  ['AgentTool', 'mutation-request']
]);
const DTO_SOURCE_SUFFIXES = new Map([
  ['AppCapabilityProjectionResponse', 'src/web/src/features/releaseProjection/releaseProjection.tsx'],
  ['ModelOption', 'src/web/src/types/api.ts'],
  ['AgentToolDefinition', 'src/web/src/features/agents/agentsApi.ts'],
  ['UpdateConversationConfigRequest', 'src/web/src/types/api.ts'],
  ['CreateAgentRequest', 'src/web/src/features/agents/agentsApi.ts'],
  ['UpdateAgentRequest', 'src/web/src/features/agents/agentsApi.ts'],
  ['AgentTool', 'src/web/src/types/api.ts']
]);
const MUTATION_FUNCTIONS = new Map([
  ['conversationConfigRequest', { id: 'chat-model-mutation', input: 'ModelOption', output: 'UpdateConversationConfigRequest' }],
  ['toolFromCatalogDefinition', { id: 'agent-tool-catalog-projection', input: 'AgentToolDefinition', output: 'AgentTool' }],
  ['serializeAgentMutation', { id: 'agent-mutation', input: 'CreateAgentRequest|UpdateAgentRequest', output: 'Record' }]
]);
const SHARED_OWNER_SUFFIXES = new Set([
  'src/services/http/client.ts',
  'src/services/http/stream.ts',
  'src/services/http/upload.ts',
  'src/lib/swr.ts'
]);
const EXPECTED_OWNER_PATHS = [
  'src/web/src/services/http/client.ts',
  'src/web/src/services/http/stream.ts',
  'src/web/src/services/http/upload.ts',
  'src/web/src/lib/swr.ts',
  'src/web/src/features/chat/api.ts',
  'src/web/src/app/appContext.tsx',
  'src/web/src/app/providers.tsx',
  'src/web/src/features/admin/api.ts',
  'src/web/src/features/agents/agentsApi.ts',
  'src/web/src/features/agents/memoriesApi.ts',
  'src/web/src/features/agents/planStepsApi.ts',
  'src/web/src/features/auth/api.ts',
  'src/web/src/features/console/api.ts',
  'src/web/src/features/knowledge/api.ts',
  'src/web/src/features/marketplace/api.ts',
  'src/web/src/features/mcp/mcpServersApi.ts',
  'src/web/src/features/notifications/notificationsApi.ts',
  'src/web/src/features/publishingChannels/publishingChannelsApi.ts',
  'src/web/src/features/scheduledTasks/scheduledTasksApi.ts',
  'src/web/src/features/tasks/api.ts',
  'src/web/src/features/workflows/workflowsApi.ts',
  'src/web/src/routes/admin/AdminHomePage.tsx',
  'src/web/src/features/releaseProjection/releaseProjection.tsx',
  'src/web/src/routes/marketing/LoginPage.tsx',
  'src/web/src/routes/marketing/RegisterPage.tsx'
];

function fail(code, detail = '') {
  const error = new Error(detail ? `${code}: ${detail}` : code);
  error.code = code;
  throw error;
}

function normalize(file) {
  return path.resolve(file).split(path.sep).join('/');
}

function unwrap(node) {
  let current = node;
  while (
    current && (
      ts.isParenthesizedExpression(current)
      || ts.isAsExpression(current)
      || ts.isSatisfiesExpression(current)
      || ts.isTypeAssertionExpression(current)
      || ts.isNonNullExpression(current)
    )
  ) {
    current = current.expression;
  }
  return current;
}

function literalValue(node) {
  node = unwrap(node);
  if (!node) return undefined;
  if (ts.isStringLiteral(node) || ts.isNoSubstitutionTemplateLiteral(node)) return node.text;
  if (ts.isNumericLiteral(node)) return Number(node.text);
  if (node.kind === ts.SyntaxKind.TrueKeyword) return true;
  if (node.kind === ts.SyntaxKind.FalseKeyword) return false;
  if (node.kind === ts.SyntaxKind.NullKeyword) return null;
  if (ts.isPrefixUnaryExpression(node) && node.operator === ts.SyntaxKind.MinusToken) {
    const value = literalValue(node.operand);
    return typeof value === 'number' ? -value : undefined;
  }
  if (ts.isArrayLiteralExpression(node)) {
    return node.elements.map((element) => literalValue(element));
  }
  if (ts.isObjectLiteralExpression(node)) {
    const value = {};
    for (const property of node.properties) {
      if (ts.isPropertyAssignment(property)) {
        const name = property.name && (ts.isIdentifier(property.name) || ts.isStringLiteral(property.name))
          ? property.name.text
          : null;
        if (name === null) return undefined;
        value[name] = literalValue(property.initializer);
      } else if (ts.isShorthandPropertyAssignment(property)) {
        value[property.name.text] = undefined;
      } else {
        return undefined;
      }
    }
    return value;
  }
  return undefined;
}

function sourceLocation(node, projectRoot) {
  const sourceFile = node.getSourceFile();
  const start = sourceFile.getLineAndCharacterOfPosition(node.getStart(sourceFile));
  return {
    file: path.relative(projectRoot, sourceFile.fileName).split(path.sep).join('/'),
    line: start.line + 1,
    column: start.character + 1
  };
}

function declarationTypeName(node, checker) {
  const type = checker.getTypeAtLocation(node);
  const symbol = type.aliasSymbol ?? type.getSymbol?.();
  if (symbol && symbol.name && symbol.name !== '__type') return symbol.name;
  const rendered = checker.typeToString(type);
  return rendered && rendered !== 'any' && rendered !== 'unknown' ? rendered : null;
}

function propertyName(name) {
  if (!name) return null;
  if (ts.isIdentifier(name) || ts.isStringLiteral(name) || ts.isNumericLiteral(name)) return name.text;
  return null;
}

function projectionSourcePath(generatedFile) {
  return path.join(path.dirname(generatedFile), 'release-projection.generated.ts');
}

function generatedReleaseProjection(program, generatedFile) {
  const sourceFile = program.getSourceFile(projectionSourcePath(generatedFile));
  if (!sourceFile) fail('generated_release_projection_missing');
  const declarations = new Map();
  for (const statement of sourceFile.statements) {
    if (!ts.isVariableStatement(statement)) continue;
    for (const declaration of statement.declarationList.declarations) {
      if (ts.isIdentifier(declaration.name)) declarations.set(declaration.name.text, declaration.initializer);
    }
  }
  const digest = literalValue(declarations.get('releaseProjectionDigest'));
  const capabilities = literalValue(declarations.get('releaseCapabilityProjection'));
  const surfaces = literalValue(declarations.get('releaseSurfaceProjection'));
  if (!/^sha256:[0-9a-f]{64}$/.test(digest ?? '') || !Array.isArray(capabilities) || capabilities.length === 0 || !Array.isArray(surfaces) || surfaces.length === 0) {
    fail('generated_release_projection_invalid');
  }
  return { digest, capabilities, surfaces };
}

function discoverDTOContracts(sourceFile, projectRoot, checker, contracts) {
  const relative = path.relative(projectRoot, sourceFile.fileName).split(path.sep).join('/');
  for (const statement of sourceFile.statements) {
    if (!ts.isTypeAliasDeclaration(statement) && !ts.isInterfaceDeclaration(statement)) continue;
    const name = statement.name.text;
    const role = DTO_ROLES.get(name);
    const expectedSource = DTO_SOURCE_SUFFIXES.get(name);
    const fixtureSource = relative.endsWith('scripts/testdata/frontend-surface/production/transports.ts');
    if (expectedSource && relative !== expectedSource && !fixtureSource) continue;
    if (!role || contracts.some((entry) => entry.name === name)) continue;
    const type = checker.getTypeAtLocation(statement.name);
    contracts.push({
      source: sourceLocation(statement, projectRoot),
      name,
      role,
      fields: checker.getPropertiesOfType(type).map((field) => field.name).sort()
    });
  }
}

function collectMutationFields(node) {
  const fields = new Set();
  const visit = (current) => {
    if (ts.isPropertyAssignment(current) || ts.isShorthandPropertyAssignment(current)) {
      const name = propertyName(current.name);
      if (name) fields.add(name);
    }
    if (ts.isVariableDeclaration(current) && ts.isIdentifier(current.name) && current.name.text === 'fields') {
      const value = unwrap(current.initializer);
      if (value && ts.isArrayLiteralExpression(value)) {
        for (const element of value.elements) {
          const literal = literalValue(element);
          if (typeof literal === 'string') fields.add(literal);
        }
      }
    }
    if (ts.isBinaryExpression(current) && current.operatorToken.kind === ts.SyntaxKind.EqualsToken && ts.isPropertyAccessExpression(current.left)) {
      if (ts.isIdentifier(current.left.expression) && current.left.expression.text === 'result') fields.add(current.left.name.text);
    }
    ts.forEachChild(current, visit);
  };
  visit(node);
  return [...fields].sort();
}

function discoverMutationContracts(sourceFile, projectRoot, contracts) {
  const visit = (statement) => {
    const candidates = [];
    if (ts.isFunctionDeclaration(statement) && statement.name && statement.body) {
      candidates.push({ name: statement.name.text, node: statement, body: statement.body });
    } else if (ts.isVariableStatement(statement)) {
      for (const declaration of statement.declarationList.declarations) {
        const initializer = unwrap(declaration.initializer);
        if (
          ts.isIdentifier(declaration.name)
          && initializer
          && (ts.isArrowFunction(initializer) || ts.isFunctionExpression(initializer))
          && initializer.body
        ) {
          candidates.push({ name: declaration.name.text, node: declaration, body: initializer.body });
        }
      }
    }
    for (const candidate of candidates) {
      const spec = MUTATION_FUNCTIONS.get(candidate.name);
      if (!spec || contracts.some((entry) => entry.id === spec.id)) continue;
      contracts.push({
        source: sourceLocation(candidate.node, projectRoot),
        id: spec.id,
        inputType: spec.input,
        outputType: spec.output,
        fields: collectMutationFields(candidate.body),
        capabilityIdOmitted: !candidate.body.getText().includes('capabilityId')
      });
    }
    ts.forEachChild(statement, visit);
  };
  visit(sourceFile);
}

function findFunction(sourceFiles, name) {
  for (const sourceFile of sourceFiles) {
    const found = sourceFile.statements.find((statement) => ts.isFunctionDeclaration(statement) && statement.name?.text === name);
    if (found) return found;
  }
  return null;
}

function containsCall(node, name) {
  let found = false;
  const visit = (current) => {
    if (ts.isCallExpression(current) && callName(current) === name) found = true;
    if (!found) ts.forEachChild(current, visit);
  };
  visit(node);
  return found;
}

function containsLiteral(node, expected) {
  let found = false;
  const visit = (current) => {
    if ((ts.isStringLiteral(current) || ts.isNoSubstitutionTemplateLiteral(current)) && current.text === expected) found = true;
    if (!found) ts.forEachChild(current, visit);
  };
  visit(node);
  return found;
}

function projectionProviderContract(sourceFiles, projectRoot) {
  const provider = findFunction(sourceFiles, 'ReleaseProjectionProvider');
  const apiFactory = findFunction(sourceFiles, 'createReleaseProjectionApi');
  if (!provider?.body || !apiFactory) fail('projection_provider_missing');
  const parameter = provider.parameters[0]?.name;
  const props = parameter && ts.isObjectBindingPattern(parameter)
    ? parameter.elements.map((element) => propertyName(element.propertyName ?? element.name)).filter(Boolean).sort()
    : [];
  if (!containsCall(provider.body, 'useAppContext') || !containsCall(provider.body, 'load') || !containsLiteral(provider.body, 'authenticated')) {
    fail('projection_provider_invalid');
  }
  return {
    source: sourceLocation(provider, projectRoot),
    component: 'ReleaseProjectionProvider',
    responseType: 'AppCapabilityProjectionResponse',
    operationId: 'getAppReadinessCapabilities',
    authSource: 'useAppContext',
    authenticatedStatus: 'authenticated',
    stateSource: 'api.load',
    props
  };
}

function exposureSurfaceKind(file) {
  if (file.endsWith('/app/router.tsx')) return 'router';
  if (file.endsWith('/WorkspaceLayout.tsx')) return 'workspace-navigation';
  if (file.endsWith('/ConsoleLayout.tsx')) return 'console-navigation';
  if (file.endsWith('/AdminSidebar.tsx')) return 'admin-navigation';
  if (file.includes('/routes/marketing/')) return 'public';
  return 'product';
}

function isProductionSource(file, sourceRoot) {
  const normalized = normalize(file);
  if (!normalized.startsWith(`${normalize(sourceRoot)}/`)) return false;
  if (!SOURCE_EXTENSIONS.has(path.extname(file))) return false;
  const relative = path.relative(sourceRoot, file).split(path.sep).join('/');
  const segments = relative.split('/');
  const lower = relative.toLowerCase();
  if (segments.some((segment) => segment === '__tests__' || segment === 'fixtures' || segment === 'mocks')) return false;
  if (/(^|[./])(__snapshots__|snapshots)(\/|$)/i.test(relative)) return false;
  if (/(^|[./])(?:test|spec)(?:\.[^/]*)?$/i.test(relative) || /\.(?:test|spec)\.[^.]+$/i.test(relative)) return false;
  if (lower.includes('/testdata/') || lower.includes('/fixture/')) return false;
  return !/\.generated\.(?:ts|tsx)$/.test(lower);
}

function collectFiles(root) {
  const files = [];
  const walk = (directory) => {
    for (const entry of fs.readdirSync(directory, { withFileTypes: true })) {
      const full = path.join(directory, entry.name);
      if (entry.isDirectory()) walk(full);
      else if (SOURCE_EXTENSIONS.has(path.extname(entry.name))) files.push(full);
    }
  };
  walk(root);
  return files.sort((left, right) => normalize(left).localeCompare(normalize(right)));
}

function sha256(value) {
  return `sha256:${crypto.createHash('sha256').update(value).digest('hex')}`;
}

function digestFiles(files, projectRoot) {
  const entries = files.map((file) => ({
    file: path.relative(projectRoot, file).split(path.sep).join('/'),
    digest: sha256(fs.readFileSync(file))
  }));
  return sha256(JSON.stringify(entries));
}

function resolveGeneratedSymbols(program, checker, generatedFile) {
  const generated = new Set();
  const sourceFile = program.getSourceFile(generatedFile);
  if (!sourceFile) fail('generated_contract_missing', generatedFile);
  for (const statement of sourceFile.statements) {
    if (!ts.isVariableStatement(statement)) continue;
    for (const declaration of statement.declarationList.declarations) {
      if (!ts.isIdentifier(declaration.name) || !declaration.name.text.endsWith('OperationContract')) continue;
      const symbol = checker.getSymbolAtLocation(declaration.name);
      if (symbol) generated.add(symbol);
    }
  }
  return { generated, sourceFile };
}

function generatedContractFromExpression(expression, checker, generatedSymbols, visited = new Set()) {
  expression = unwrap(expression);
  if (!expression) return [];
  if (ts.isIdentifier(expression)) {
    const symbol = checker.getSymbolAtLocation(expression);
    if (!symbol || visited.has(symbol)) return [];
    visited.add(symbol);
    const target = (symbol.flags & ts.SymbolFlags.Alias) !== 0 ? checker.getAliasedSymbol(symbol) : symbol;
    if (generatedSymbols.has(target) || generatedSymbols.has(symbol)) return [target];
    const result = [];
    for (const declaration of target.declarations ?? []) {
      if (ts.isVariableDeclaration(declaration) && declaration.initializer) {
        result.push(...generatedContractFromExpression(declaration.initializer, checker, generatedSymbols, visited));
      }
    }
    return result;
  }
  if (ts.isPropertyAccessExpression(expression) || ts.isElementAccessExpression(expression)) {
    return generatedContractFromExpression(expression.expression, checker, generatedSymbols, visited);
  }
  const result = [];
  expression.forEachChild((child) => result.push(...generatedContractFromExpression(child, checker, generatedSymbols, visited)));
  return result;
}

function generatedContractInArgument(argument, checker, generatedSymbols) {
  const found = generatedContractFromExpression(argument, checker, generatedSymbols);
  return [...new Set(found)];
}

function findProperty(object, name) {
  object = unwrap(object);
  if (!object || !ts.isObjectLiteralExpression(object)) return null;
  return object.properties.find((property) => (
    ts.isPropertyAssignment(property)
    && property.name
    && (ts.isIdentifier(property.name) || ts.isStringLiteral(property.name))
    && property.name.text === name
  ))?.initializer ?? null;
}

function findObjectExpression(expression, checker, visited = new Set()) {
  expression = unwrap(expression);
  if (!expression) return null;
  if (ts.isObjectLiteralExpression(expression) || ts.isArrayLiteralExpression(expression)) return expression;
  if (ts.isIdentifier(expression)) {
    const symbol = checker.getSymbolAtLocation(expression);
    if (!symbol || visited.has(symbol)) return null;
    visited.add(symbol);
    const target = (symbol.flags & ts.SymbolFlags.Alias) !== 0 ? checker.getAliasedSymbol(symbol) : symbol;
    for (const declaration of target.declarations ?? []) {
      if (ts.isVariableDeclaration(declaration) && declaration.initializer) {
        const result = findObjectExpression(declaration.initializer, checker, visited);
        if (result) return result;
      }
    }
  }
  return null;
}

function callName(expression) {
  expression = unwrap(expression);
  if (ts.isCallExpression(expression)) return callName(expression.expression);
  if (ts.isIdentifier(expression)) return expression.text;
  if (ts.isPropertyAccessExpression(expression)) return expression.name.text;
  return '';
}

function encoderFromExpression(expression, checker, operation, generatedSymbols) {
  expression = unwrap(expression);
  if (expression && ts.isConditionalExpression(expression)) {
    const branch = operation.request.mediaType === null ? expression.whenTrue : expression.whenFalse;
    return encoderFromExpression(branch, checker, operation, generatedSymbols);
  }
  const call = expression && ts.isCallExpression(expression) ? expression : null;
  const name = callName(call);
  const ids = {
    noneRequestEncoder: 'none',
    jsonRequestEncoder: 'json',
    formDataRequestEncoder: 'form-data',
    rawRequestEncoder: 'raw'
  };
  if (!call || !(name in ids)) fail('request_encoder_unresolved', operation.operationId);
  const symbols = generatedContractInArgument(call.arguments[0], checker, generatedSymbols);
  if (symbols.length !== 1) fail('request_encoder_identity_unresolved', operation.operationId);
  return { id: ids[name], mediaType: operation.request.mediaType, schemaIdentity: operation.request.schemaIdentity };
}

function decoderFromExpression(expression, checker, operation, generatedSymbols) {
  expression = unwrap(expression);
  const call = expression && ts.isCallExpression(expression) ? expression : null;
  const name = callName(call);
  const ids = {
    jsonEnvelopeDecoder: 'json-envelope',
    textResponseDecoder: 'text',
    rawResponseDecoder: 'raw-response',
    noneResponseDecoder: 'none'
  };
  if (!call || !(name in ids) || call.arguments.length < 2) fail('response_decoder_unresolved', operation.operationId);
  const symbols = generatedContractInArgument(call.arguments[0], checker, generatedSymbols);
  if (symbols.length !== 1) fail('response_decoder_identity_unresolved', operation.operationId);
  const status = literalValue(call.arguments[1]);
  if (!Number.isSafeInteger(status)) fail('response_status_unresolved', operation.operationId);
  const success = operation.successResponses.find((response) => Number(response.status) === status);
  if (!success) fail('response_status_unknown', operation.operationId);
  return {
    id: ids[name],
    status,
    mediaType: success.mediaType,
    schemaIdentity: success.schemaIdentity
  };
}

function operationFromContract(expression, checker, generatedSymbols, metadataBySymbol) {
  const unwrapped = unwrap(expression);
  if (unwrapped && ts.isIdentifier(unwrapped)) {
    const symbol = checker.getSymbolAtLocation(unwrapped);
    const target = symbol && (symbol.flags & ts.SymbolFlags.Alias) !== 0 ? checker.getAliasedSymbol(symbol) : symbol;
    const declaration = target?.declarations?.find((item) => ts.isVariableDeclaration(item));
    if (declaration?.initializer && declaration.initializer !== unwrapped) {
      return operationFromContract(declaration.initializer, checker, generatedSymbols, metadataBySymbol);
    }
  }
  if (unwrapped && ts.isCallExpression(unwrapped)) {
    const symbols = generatedContractInArgument(unwrapped.arguments[0], checker, generatedSymbols);
    if (symbols.length === 1 && (callName(unwrapped).endsWith('Transport') || callName(unwrapped) === 'jsonTransport')) {
      const operation = metadataBySymbol.get(symbols[0]);
      if (!operation) fail('generated_operation_metadata_unresolved');
      const helperName = callName(unwrapped);
      const status = helperName === 'noContentTransport'
        ? 204
        : unwrapped.arguments.length > 1 ? literalValue(unwrapped.arguments[1]) : 200;
      if (!Number.isSafeInteger(status)) fail('response_status_unresolved', operation.operationId);
      const success = operation.successResponses.find((response) => Number(response.status) === status);
      if (!success) fail('response_status_unknown', operation.operationId);
      return {
        operation,
        requestEncoder: {
          id: operation.request.mediaType === null ? 'none' : 'json',
          mediaType: operation.request.mediaType,
          schemaIdentity: operation.request.schemaIdentity
        },
        responseDecoder: {
          id: helperName === 'noContentTransport' ? 'none' : 'json-envelope',
          status,
          mediaType: success.mediaType,
          schemaIdentity: success.schemaIdentity
        }
      };
    }
  }
  const object = findObjectExpression(expression, checker);
  if (!object || !ts.isObjectLiteralExpression(object)) fail('transport_contract_unresolved');
  const operationExpression = findProperty(object, 'operation');
  const symbols = generatedContractInArgument(operationExpression, checker, generatedSymbols);
  if (symbols.length !== 1) fail('transport_operation_identity_unresolved');
  const operation = metadataBySymbol.get(symbols[0]);
  if (!operation) fail('generated_operation_metadata_unresolved');
  const requestEncoder = encoderFromExpression(findProperty(object, 'requestEncoder'), checker, operation, generatedSymbols);
  const responseDecoder = decoderFromExpression(findProperty(object, 'responseDecoder'), checker, operation, generatedSymbols);
  return { operation, requestEncoder, responseDecoder };
}

function transportKind(node, checker) {
  if (ts.isNewExpression(node) && ts.isIdentifier(node.expression)) {
    if (node.expression.text === 'WebSocket') return { protocol: 'websocket', kind: 'websocket' };
    if (node.expression.text === 'EventSource') return { protocol: 'sse', kind: 'event-source' };
  }
  if (!ts.isCallExpression(node)) return null;
  const expression = node.expression;
  if (ts.isIdentifier(expression)) {
    if (expression.text === 'streamText') return { protocol: 'sse', kind: 'sse-stream' };
    if (expression.text === 'uploadFile') return { protocol: 'http', kind: 'multipart-upload' };
    if (expression.text === 'useSWR') return { protocol: 'http', kind: 'swr' };
    // Use symbol resolution for fetch instead of syntax name
    const symbol = checker.getSymbolAtLocation(expression);
    const symbolName = symbol?.getName();
    if (symbolName === 'fetch' || expression.text === 'fetchFn') return { protocol: 'http', kind: 'raw-fetch' };
    return null;
  }
  if (!ts.isPropertyAccessExpression(expression) || !HTTP_METHODS.has(expression.name.text)) return null;
  const receiverType = checker.typeToString(checker.getTypeAtLocation(expression.expression));
  if (!/(HttpClient|OperationTransportContract|SWRTransportKey)/.test(receiverType)) return null;
  return { protocol: 'http', kind: 'http-client' };
}

function operationArguments(node, kind, checker) {
  if (kind === 'raw-fetch') return { operation: null, contract: node.arguments[2], path: node.arguments[0], init: node.arguments[1] };
  if (kind === 'sse-stream') return { operation: node.arguments[2], contract: node.arguments[3], path: node.arguments[0], init: node.arguments[5] };
  if (kind === 'multipart-upload') return { operation: node.arguments[2], contract: node.arguments[3], path: node.arguments[0] };
  if (kind === 'swr') {
    const tuple = findObjectExpression(node.arguments[0], checker);
    return {
      operation: tuple && ts.isArrayLiteralExpression(tuple) ? tuple.elements[1] : null,
      contract: tuple && ts.isArrayLiteralExpression(tuple) ? tuple.elements[2] : null,
      path: tuple && ts.isArrayLiteralExpression(tuple) ? tuple.elements[0] : null
    };
  }
  const method = ts.isPropertyAccessExpression(node.expression) ? node.expression.name.text : 'get';
  const contractIndex = method === 'request' ? 2 : node.arguments.length >= 4 ? 3 : 2;
  return { operation: null, contract: node.arguments[contractIndex], path: node.arguments[0], method, init: node.arguments[1] };
}

function invocationMethod(node, kind, args, checker) {
  if (ts.isNewExpression(node)) return 'GET';
  if (kind === 'swr') return 'GET';
  if (kind === 'sse-stream' || kind === 'multipart-upload') return 'POST';
  if (kind === 'http-client' && args.method !== 'request') return args.method.toUpperCase();
  const init = findObjectExpression(args.init, checker);
  if (!init || !ts.isObjectLiteralExpression(init)) return 'GET';
  const method = literalValue(findProperty(init, 'method'));
  return typeof method === 'string' ? method.toUpperCase() : null;
}

function invocationPathMatches(expression, normalizedPath) {
  const value = literalValue(expression);
  // Dynamic paths cannot be statically verified — return true to skip the check
  if (typeof value !== 'string') return true;
  let pathname;
  try {
    pathname = new URL(value, 'http://oblivious.invalid').pathname;
  } catch {
    return false;
  }
  const actual = pathname.split('/').filter(Boolean);
  const expected = normalizedPath.split('/').filter(Boolean);
  if (actual.length !== expected.length) return false;
  return expected.every((segment, index) => (
    segment.startsWith('{') && segment.endsWith('}') ? actual[index].length > 0 : actual[index] === segment
  ));
}

function validateTransportInvocation(node, kind, args, operation, checker) {
  const method = invocationMethod(node, kind, args, checker);
  if (method === null || method !== operation.method.toUpperCase()) fail('transport_method_mismatch', operation.operationId);
  if (args.path && !invocationPathMatches(args.path, operation.normalizedPath)) fail('transport_path_mismatch', operation.operationId);
}

function operationRecord(node, projectRoot, checker, generatedSymbols, metadataBySymbol, browserEventsByOperation) {
  const kind = transportKind(node, checker);
  if (!kind) return null;
  if (ts.isNewExpression(node)) {
    const operationSymbols = generatedContractFromExpression(node.arguments?.[0], checker, generatedSymbols);
    if (operationSymbols.length !== 1) fail('websocket_operation_identity_unresolved');
    const operation = metadataBySymbol.get(operationSymbols[0]);
    if (!operation) fail('generated_operation_metadata_unresolved');
    const eventSource = kind.kind === 'event-source';
    const eventSourceResponse = eventSource
      ? operation.successResponses.find((response) => response.mediaType === 'text/event-stream')
      : null;
    if (eventSource && !eventSourceResponse) fail('event_source_response_unresolved', operation.operationId);
    return makeOperationRecord(node, projectRoot, kind, operation, {
      id: 'none', mediaType: null, schemaIdentity: { kind: 'none', value: null }
    }, {
      id: eventSource ? 'event-source' : 'raw-response',
      status: eventSource ? Number(eventSourceResponse.status) : 101,
      mediaType: eventSource ? eventSourceResponse.mediaType : null,
      schemaIdentity: eventSource
        ? eventSourceResponse.schemaIdentity
        : { kind: 'none', value: null }
    }, browserEventsByOperation.get(operation.operationId) ?? []);
  }
  const args = operationArguments(node, kind.kind, checker);
  let operation;
  let requestEncoder;
  let responseDecoder;
  if (kind.kind === 'raw-fetch') {
    const symbols = generatedContractInArgument(args.contract, checker, generatedSymbols);
    if (symbols.length !== 1) fail('transport_operation_identity_unresolved');
    operation = metadataBySymbol.get(symbols[0]);
    if (!operation) fail('generated_operation_metadata_unresolved');
    ({ requestEncoder, responseDecoder } = operationFromContract(args.contract, checker, generatedSymbols, metadataBySymbol));
  } else if (kind.kind === 'sse-stream' || kind.kind === 'multipart-upload' || kind.kind === 'swr') {
    const symbols = generatedContractInArgument(args.operation, checker, generatedSymbols);
    if (symbols.length !== 1) fail('transport_operation_identity_unresolved');
    operation = metadataBySymbol.get(symbols[0]);
    if (!operation) fail('generated_operation_metadata_unresolved');
    ({ requestEncoder, responseDecoder } = operationFromContract(args.contract, checker, generatedSymbols, metadataBySymbol));
  } else {
    ({ operation, requestEncoder, responseDecoder } = operationFromContract(args.contract, checker, generatedSymbols, metadataBySymbol));
  }
  if (!operation) fail('operation_unresolved');
  validateTransportInvocation(node, kind.kind, args, operation, checker);
  return makeOperationRecord(
    node,
    projectRoot,
    kind,
    operation,
    requestEncoder,
    responseDecoder,
    browserEventsByOperation.get(operation.operationId) ?? []
  );
}

function makeOperationRecord(node, projectRoot, transport, operation, requestEncoder, responseDecoder, events) {
  const location = sourceLocation(node, projectRoot);
  const method = operation.method;
  return {
    source: { ...location, symbol: operation.operationId },
    contract: operation,
    operation,
    transport: {
      protocol: transport.protocol,
      kind: transport.kind,
      method,
      pathTemplate: operation.normalizedPath
    },
    request: {
      mediaType: requestEncoder.mediaType,
      encoder: requestEncoder.id,
      schemaIdentity: requestEncoder.schemaIdentity,
      schemaRef: requestEncoder.schemaIdentity.value
    },
    response: {
      status: responseDecoder.status,
      mediaType: responseDecoder.mediaType,
      decoder: responseDecoder.id,
      schemaIdentity: responseDecoder.schemaIdentity,
      schemaRef: responseDecoder.schemaIdentity.value
    },
    requestEncoder,
    responseDecoder,
    events
  };
}

function discoverExposures(sourceFile, projectRoot, checker, exposures, policyViolations, capabilityIds) {
  const visit = (node) => {
    if (ts.isObjectLiteralExpression(node)) {
      const pathValue = findProperty(node, 'path') ?? findProperty(node, 'to') ?? findProperty(node, 'href');
      const capabilityValue = findProperty(node, 'capabilityId');
      const pathLiteral = literalValue(pathValue);
      const capabilityId = literalValue(capabilityValue);
      if (typeof pathLiteral === 'string' && (typeof capabilityId === 'string' || capabilityValue)) {
        const kind = pathValue === findProperty(node, 'href') ? 'link' : 'navigation';
        const source = sourceLocation(node, projectRoot);
        exposures.push({
          source,
          kind,
          surfaceKind: exposureSurfaceKind(source.file),
          productPath: pathLiteral,
          catalogSubject: null,
          capabilityId: typeof capabilityId === 'string' ? capabilityId : null,
          capabilitySource: 'release-projection'
        });
      }
    }
    if (ts.isPropertyAccessExpression(node) && node.name.text === 'capabilityId') {
      const subject = declarationTypeName(node.expression, checker);
      if (subject && CATALOG_RESPONSE_DTOS.has(subject)) {
        exposures.push({
          source: sourceLocation(node, projectRoot),
          kind: 'selector',
          surfaceKind: 'catalog-selector',
          productPath: null,
          catalogSubject: `${subject}.capabilityId`,
          capabilityId: null,
          capabilitySource: 'server-catalog'
        });
      }
      const parent = node.parent;
      if (parent && ts.isBinaryExpression(parent) && [ts.SyntaxKind.QuestionQuestionToken, ts.SyntaxKind.BarBarToken].includes(parent.operatorToken.kind)) {
        const other = parent.left === node ? parent.right : parent.left;
        const fallback = literalValue(other);
        if (typeof fallback === 'string' && capabilityIds.has(fallback)) policyViolations.push('capability_fallback_literal');
      }
    }
    if (ts.isCallExpression(node) && callName(node) === 'isCapabilityEnabled') {
      const argument = unwrap(node.arguments[0]);
      const subject = argument && ts.isPropertyAccessExpression(argument) ? declarationTypeName(argument.expression, checker) : null;
      exposures.push({
        source: sourceLocation(node, projectRoot),
        kind: 'availability-guard',
        surfaceKind: 'authenticated-projection',
        productPath: null,
        catalogSubject: subject && CATALOG_RESPONSE_DTOS.has(subject) ? `${subject}.capabilityId` : null,
        capabilityId: typeof literalValue(argument) === 'string' ? literalValue(argument) : null,
        capabilitySource: 'app-projection'
      });
    }
    if (ts.isObjectLiteralExpression(node)) {
      const mappedCapabilities = node.properties.flatMap((property) => {
        if (!ts.isPropertyAssignment(property) || propertyName(property.name) === 'capabilityId') return [];
        const value = literalValue(property.initializer);
        return typeof value === 'string' && capabilityIds.has(value) ? [value] : [];
      });
      if (mappedCapabilities.length >= 2) policyViolations.push('client_capability_map');
    }
    ts.forEachChild(node, visit);
  };
  visit(sourceFile);
}

function generatedMetadata(program, checker, generatedFile) {
  const { generated, sourceFile } = resolveGeneratedSymbols(program, checker, generatedFile);
  const operationArray = sourceFile.statements
    .flatMap((statement) => ts.isVariableStatement(statement) ? [...statement.declarationList.declarations] : [])
    .find((declaration) => ts.isIdentifier(declaration.name) && declaration.name.text === 'operationContracts');
  const array = operationArray ? unwrap(operationArray.initializer) : null;
  const values = array && ts.isArrayLiteralExpression(array) ? array.elements.map((element) => literalValue(element)) : [];
  const metadataBySymbol = new Map();
  for (const symbol of generated) {
    for (const declaration of symbol.declarations ?? []) {
      if (!ts.isVariableDeclaration(declaration) || !declaration.initializer) continue;
      const initializer = unwrap(declaration.initializer);
      let index = null;
      if (initializer && ts.isElementAccessExpression(initializer)) index = literalValue(initializer.argumentExpression);
      const value = Number.isSafeInteger(index) ? values[index] : literalValue(initializer);
      if (value && typeof value.operationId === 'string') metadataBySymbol.set(symbol, value);
    }
  }
  if (metadataBySymbol.size === 0) fail('generated_operation_inventory_empty');
  const browserEventDeclaration = sourceFile.statements
    .flatMap((statement) => ts.isVariableStatement(statement) ? [...statement.declarationList.declarations] : [])
    .find((declaration) => ts.isIdentifier(declaration.name) && declaration.name.text === 'browserEventContracts');
  const browserEventValues = browserEventDeclaration
    ? literalValue(browserEventDeclaration.initializer)
    : undefined;
  if (!Array.isArray(browserEventValues) || browserEventValues.length === 0) {
    fail('generated_browser_event_inventory_empty');
  }
  const browserEventsByOperation = new Map();
  for (const row of browserEventValues) {
    if (
      !row
      || typeof row.operationId !== 'string'
      || !['sse', 'websocket'].includes(row.transport)
      || !Array.isArray(row.events)
      || row.events.length === 0
      || browserEventsByOperation.has(row.operationId)
    ) {
      fail('generated_browser_event_invalid');
    }
    for (const event of row.events) {
      if (
        !event
        || !['client', 'server'].includes(event.direction)
        || !['message', 'event'].includes(event.kind)
        || !event.schemaIdentity
        || !['ref', 'inline', 'none'].includes(event.schemaIdentity.kind)
      ) {
        fail('generated_browser_event_invalid', row.operationId);
      }
    }
    browserEventsByOperation.set(row.operationId, row.events);
  }
  return { metadataBySymbol, browserEventsByOperation };
}

export function extractSidecar({ root, tsconfig, generatedFile = null }) {
  const sourceRoot = path.resolve(root);
  const configPath = path.resolve(tsconfig);
  if (!fs.statSync(sourceRoot).isDirectory()) fail('source_root_missing', sourceRoot);
  if (!fs.existsSync(configPath) || !fs.statSync(configPath).isFile()) fail('tsconfig_missing', configPath);
  const config = ts.readConfigFile(configPath, ts.sys.readFile);
  if (config.error) fail('tsconfig_invalid');
  const parsed = ts.parseJsonConfigFileContent(config.config, ts.sys, path.dirname(configPath));
  if (parsed.errors.length > 0) fail('tsconfig_invalid', String(parsed.errors.length));
  const projectRoot = path.dirname(configPath);
  const repositoryRoot = findRepoRoot(projectRoot);
  const files = collectFiles(sourceRoot);
  const productionFiles = files.filter((file) => isProductionSource(file, sourceRoot));
  if (productionFiles.length === 0) fail('source_inventory_empty');
  const rootNames = [...new Set([...parsed.fileNames, ...files, ...(generatedFile ? [path.resolve(generatedFile)] : [])])];
  const program = ts.createProgram({ rootNames, options: parsed.options });
  const checker = program.getTypeChecker();
  const generatedPath = path.resolve(generatedFile ?? path.join(projectRoot, 'src/generated/operation-contracts.generated.ts'));
  const { metadataBySymbol: generatedMetadataBySymbol, browserEventsByOperation } = generatedMetadata(program, checker, generatedPath);
  const generatedSymbols = new Set(generatedMetadataBySymbol.keys());
  const operations = [];
  const exposures = [];
  const dtoContracts = [];
  const mutationContracts = [];
  const policyViolations = [];
  const unresolved = [];
  const callerFiles = new Set();
  const releaseProjection = generatedReleaseProjection(program, generatedPath);
  const capabilityIds = new Set(releaseProjection.capabilities.map((entry) => entry.capabilityId));
  const productionSourceFiles = [];
  for (const file of productionFiles) {
    const sourceFile = program.getSourceFile(file);
    if (!sourceFile) continue;
    productionSourceFiles.push(sourceFile);
    const relative = path.relative(repositoryRoot, file).split(path.sep).join('/');
    const isShared = [...SHARED_OWNER_SUFFIXES].some((suffix) => relative.endsWith(suffix));
    discoverDTOContracts(sourceFile, repositoryRoot, checker, dtoContracts);
    discoverMutationContracts(sourceFile, repositoryRoot, mutationContracts);
    if (!isShared) discoverExposures(sourceFile, repositoryRoot, checker, exposures, policyViolations, capabilityIds);
    const visit = (node) => {
      const kind = transportKind(node, checker);
      if (kind && !isShared) {
        try {
          const record = operationRecord(
            node,
            repositoryRoot,
            checker,
            generatedSymbols,
            generatedMetadataBySymbol,
            browserEventsByOperation
          );
          if (record) {
            operations.push(record);
            callerFiles.add(relative);
          }
        } catch (error) {
          unresolved.push({ source: sourceLocation(node, repositoryRoot), code: error.code ?? 'operation_unresolved' });
        }
      }
      ts.forEachChild(node, visit);
    };
    visit(sourceFile);
  }
  const enforceOwnerClosure = path.basename(sourceRoot) === 'src' && path.basename(path.dirname(sourceRoot)) === 'web';
  const known = new Set(EXPECTED_OWNER_PATHS);
  if (enforceOwnerClosure) {
    const discoveredOwnerFiles = [...callerFiles].sort();
    for (const file of discoveredOwnerFiles) {
      if (!known.has(file)) unresolved.push({ source: { file, line: 1, column: 1 }, code: 'new_owner_outside_closed_set' });
    }
    for (const expected of EXPECTED_OWNER_PATHS) {
      if (!fs.existsSync(path.join(repositoryRoot, expected))) {
        unresolved.push({ source: { file: expected, line: 1, column: 1 }, code: 'owner_missing' });
        continue;
      }
      if (expected.endsWith('/providers.tsx')) {
        if (callerFiles.has(expected)) unresolved.push({ source: { file: expected, line: 1, column: 1 }, code: 'non_caller_transport_use' });
        continue;
      }
      const shared = [...SHARED_OWNER_SUFFIXES].some((suffix) => expected.endsWith(suffix));
      if (!shared && !callerFiles.has(expected)) {
        unresolved.push({ source: { file: expected, line: 1, column: 1 }, code: 'owner_transport_use_missing' });
      }
    }
  }
  const sharedOwners = EXPECTED_OWNER_PATHS.filter((file) => [...SHARED_OWNER_SUFFIXES].some((suffix) => file.endsWith(suffix)));
  const ownerClosure = enforceOwnerClosure
    ? {
      expected: EXPECTED_OWNER_PATHS.length,
      resolved: EXPECTED_OWNER_PATHS.length - 1,
      nonCallers: 1,
      sharedOwners: sharedOwners.length,
      files: EXPECTED_OWNER_PATHS.map((file) => {
        if (file.endsWith('/providers.tsx')) return { file, disposition: 'non-caller', reason: 'compiler-proven-zero-transport-calls' };
        if ([...SHARED_OWNER_SUFFIXES].some((suffix) => file.endsWith(suffix))) {
          return { file, disposition: 'shared-transport-owner', reason: 'shared-wrapper-owned-by-extractor-taxonomy' };
        }
        return { file, disposition: 'transport-caller', reason: 'exact-generated-operation-symbol-resolved' };
      })
    }
    : { expected: 0, resolved: callerFiles.size, nonCallers: 0, sharedOwners: 0, files: [] };
  operations.sort((left, right) => `${left.source.file}:${left.source.line}:${left.source.column}`.localeCompare(`${right.source.file}:${right.source.line}:${right.source.column}`));
  exposures.sort((left, right) => JSON.stringify(left).localeCompare(JSON.stringify(right)));
  dtoContracts.sort((left, right) => left.name.localeCompare(right.name));
  mutationContracts.sort((left, right) => left.id.localeCompare(right.id));
  unresolved.sort((left, right) => JSON.stringify(left).localeCompare(JSON.stringify(right)));
  if (operations.length === 0) fail('operation_inventory_empty');
  const configDigest = sha256(fs.readFileSync(configPath));
  const sourceDigest = digestFiles(productionFiles, projectRoot);
  return {
    schemaVersion: SCHEMA_VERSION,
    sourceScope: {
      root: path.relative(repositoryRoot, sourceRoot).split(path.sep).join('/'),
      include: ['**/*.ts', '**/*.tsx'],
      excludeKinds: ['test', 'test-support', 'fixture', 'mock', 'snapshot'],
      generatedPolicy: 'consumer-only',
      filesScanned: productionFiles.length,
      sourceDigest,
      ownerClosure
    },
    extractor: {
      name: 'oblivious-frontend-surface',
      typescriptVersion: ts.version,
      configDigest
    },
    operations,
    exposures,
    releaseProjection,
    dtoContracts,
    mutationContracts,
    projectionProvider: projectionProviderContract(productionSourceFiles, repositoryRoot),
    policyViolations: [...new Set(policyViolations)].sort(),
    generatedConsumers: [...generatedMetadataBySymbol.keys()].length,
    unresolved
  };
}

function findRepoRoot(start) {
  let current = path.resolve(start);
  while (current !== path.dirname(current)) {
    if (fs.existsSync(path.join(current, '.git'))) return current;
    current = path.dirname(current);
  }
  return path.resolve(start);
}

function parseArgs(argv) {
  const options = { root: null, tsconfig: null, generatedFile: null, output: null };
  for (let index = 0; index < argv.length; index += 1) {
    const arg = argv[index];
    if (arg === '--root') options.root = argv[++index];
    else if (arg === '--tsconfig') options.tsconfig = argv[++index];
    else if (arg === '--generated-file') options.generatedFile = argv[++index];
    else if (arg === '--output') options.output = argv[++index];
    else fail('argument_invalid', arg);
  }
  if (!options.root || !options.tsconfig) fail('argument_invalid', 'root/tsconfig');
  return options;
}

if (import.meta.url === `file://${process.argv[1]}`) {
  try {
    const options = parseArgs(process.argv.slice(2));
    const result = extractSidecar(options);
    if (result.unresolved.length > 0) fail('frontend_sidecar_unresolved');
    const encoded = `${JSON.stringify(result)}\n`;
    if (options.output) {
      fs.mkdirSync(path.dirname(path.resolve(options.output)), { recursive: true });
      fs.writeFileSync(options.output, encoded, 'utf8');
    } else {
      process.stdout.write(encoded);
    }
  } catch (error) {
    process.stderr.write(`${error.code ?? 'frontend_sidecar_failed'}\n`);
    process.exitCode = 1;
  }
}
