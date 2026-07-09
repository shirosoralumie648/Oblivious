# Builtin Lorem Ipsum Demo-Only Boundary

Date: 2026-07-05

## Scope

This slice closes the local commercial-evidence gap where the `lorem_ipsum` placeholder generator could appear in the default commercial builtin tool catalog. The tool remains available for direct demo/testing, but it is no longer counted as a default commercial Agent capability.

## Runtime Changes

- Changed the default commercial builtin policy for `lorem_ipsum` to `false`.
- Kept `LoremIpsumTool` registered so existing direct tests and demo-only usage still work.
- Added default catalog coverage proving `ListDefaultCommercialBuiltinTools()` excludes `lorem_ipsum`.
- Normalized built-in file/path helper tools to POSIX-style path semantics via `path` instead of host-dependent `filepath`, keeping tool output stable across Linux, Windows, WSL, and container runners.
- Added Agent execution coverage proving explicitly configured `lorem_ipsum` builtin calls are rejected before the tool is invoked.
- Updated the stub/hardcoded/TODO audit row to mark this as demo-only rather than a remaining default-commercial exposure.

## RED Evidence

Default commercial catalog RED command:

```bash
cd src/server && GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache go test ./internal/mcp -run TestBuiltinToolsCommercialDefaults -count=1 -v
```

Observed failure before changing the default commercial policy:

```text
=== RUN   TestBuiltinToolsCommercialDefaults
    builtin_test.go:32: expected lorem_ipsum to be disabled by default commercial policy, got names=map[... lorem_ipsum:true ...]
--- FAIL: TestBuiltinToolsCommercialDefaults (0.00s)
FAIL
FAIL	oblivious/server/internal/mcp	0.003s
FAIL
```

## GREEN Evidence

Focused MCP command:

```bash
cd src/server && GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache go test ./internal/mcp -run 'TestBuiltinToolsCommercialDefaults|TestDefaultEnabledBuiltinsDoNotReturnPlaceholderOutput|TestLoremIpsumTool' -count=1 -v
```

Result:

```text
--- PASS: TestLoremIpsumTool (0.00s)
--- PASS: TestBuiltinToolsCommercialDefaults (0.00s)
--- PASS: TestDefaultEnabledBuiltinsDoNotReturnPlaceholderOutput (0.06s)
PASS
ok  	oblivious/server/internal/mcp	0.062s
```

Current MCP builtin/path command:

```bash
cd src/server && GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache go test ./internal/mcp -run 'Test(BuiltinToolsCommercialDefaults|DefaultEnabledBuiltinsDoNotReturnPlaceholderOutput|LoremIpsumTool|MimeTypeFromExtension|FilePathTools)' -count=1 -v
```

Result:

```text
--- PASS: TestMimeTypeFromExtension (0.01s)
--- PASS: TestFilePathTools (0.00s)
--- PASS: TestLoremIpsumTool (0.00s)
--- PASS: TestBuiltinToolsCommercialDefaults (0.00s)
--- PASS: TestDefaultEnabledBuiltinsDoNotReturnPlaceholderOutput (0.05s)
PASS
ok  	oblivious/server/internal/mcp	0.062s
```

Focused Agent command:

```bash
cd src/server && GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache go test ./internal/agent -run 'TestExecuteToolRejectsLoremIpsumDemoBuiltinBeforeCallingTool|TestExecuteToolRejectsDisabledCommercialBuiltinBeforeCallingTool|TestExecuteToolAllowsEnabledCommercialBuiltin' -count=1 -v
```

Result:

```text
--- PASS: TestExecuteToolRejectsDisabledCommercialBuiltinBeforeCallingTool (0.00s)
--- PASS: TestExecuteToolRejectsLoremIpsumDemoBuiltinBeforeCallingTool (0.00s)
--- PASS: TestExecuteToolAllowsEnabledCommercialBuiltin (0.00s)
PASS
ok  	oblivious/server/internal/agent	0.012s
```

## Commercial Boundary

This removes a placeholder generator from default commercial Agent evidence. It does not by itself complete Agent commercial readiness; live structured streaming, cancellation, trace joins, and target-runtime tool execution evidence are still required.
