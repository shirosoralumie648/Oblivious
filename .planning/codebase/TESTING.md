---
description: Testing strategy and coverage
created: 2026-04-27
---
# Testing

## Test Frameworks

| Framework | Purpose | Location |
|-----------|---------|----------|
| Go Testing | Go unit tests | `*_test.go` files |
| Testing (stdlib) | Go test runner | Built into Go |
| Testify | Assertions, mocking | Go tests |
| Vitest | Frontend unit tests | `package.json` scripts |
| jsdom | DOM environment | `vite.config.ts` |
| Testing Library | Component testing | `@testing-library/react` |
| Playwright | E2E testing | `playwright.config.ts` |

## Test Structure

### Go Backend Tests

| Type | Count | Location Pattern |
|------|-------|------------------|
| Unit Tests | 15+ | `*_test.go` alongside source |
| Handler Tests | 6+ | `*_handler_test.go` |
| Middleware Tests | 2+ | `*_middleware_test.go` |
| Service Tests | 4+ | `*_service_test.go` |
| Store Tests | 3+ | `*_store_test.go` |
| Gateway Tests | 2+ | `*_gateway_test.go` |

**Test File Organization:**
```
internal/
├── auth/
│   ├── service.go
│   ├── service_test.go
│   ├── store.go
│   └── store_test.go
├── chat/
│   ├── service.go
│   ├── service_test.go
│   ├── gateway.go
│   └── gateway_test.go
├── http/
│   ├── auth_handler.go
│   ├── auth_handler_test.go
│   ├── auth_middleware.go
│   └── auth_middleware_test.go
```

### Frontend Tests

| Type | Count | Location Pattern |
|------|-------|------------------|
| Component Tests | 2+ | `*.test.tsx` |
| Unit Tests | 2+ | `*.test.ts` |

**Frontend Test Locations:**
```
src/web/
├── src/
│   ├── components/
│   │   └── __tests__/
│   └── utils/
│       └── __tests__/
```

## Coverage Highlights

### High Coverage Areas
- **Authentication**: Handler and middleware tests with mocked services
- **Chat Service**: Core business logic tested
- **Config Loading**: Configuration parsing validated

### Low Coverage Areas
- **WebSocket Gateway**: Limited gateway test coverage
- **Billing Integration**: Worker logic partially tested
- **Relay Handler**: New code, limited test coverage

### Missing Coverage
- **Database Integration Tests**: No integration tests with real database
- **E2E Tests**: Minimal Playwright coverage
- **Load Tests**: No performance test suite

## Test Patterns

### Go Testing Patterns

| Pattern | Usage | Example |
|---------|-------|---------|
| Table-Driven Tests | Multiple test cases | Handler tests with cases |
| Mocking with Interfaces | Store/Service mocking | `MockChatStore` |
| testify/assert | Assertions | `assert.NoError(t, err)` |
| testify/require | Fatal assertions | `require.NotNil(t, svc)` |
| Setup/Teardown | Test isolation | `t.Cleanup()` |
| Subtests | Grouped cases | `t.Run("name", ...)` |
| HTTP Test Recorder | Handler testing | `httptest.NewRecorder()` |

**Example Handler Test Pattern:**
```go
func TestHandler_Method(t *testing.T) {
    // Arrange
    mockStore := &MockStore{}
    svc := NewService(mockStore)
    handler := NewHandler(svc)
    
    // Build request
    req := httptest.NewRequest(http.MethodPost, "/path", body)
    rec := httptest.NewRecorder()
    
    // Act
    handler.Method(rec, req)
    
    // Assert
    assert.Equal(t, http.StatusOK, rec.Code)
    assert.JSONEq(t, expectedJSON, rec.Body.String())
}
```

### Frontend Testing Patterns

| Pattern | Usage | Example |
|---------|-------|---------|
| Component Rendering | React Testing Library | `render(<Component />)` |
| User Event Simulation | User interactions | `userEvent.click()` |
| Mocking Fetch | API mocking | `vi.mock('./api')` |
| Snapshot Testing | UI consistency | `toMatchSnapshot()` |

## Testing Commands

| Command | Purpose | Location |
|---------|---------|----------|
| `go test ./...` | Run all Go tests | `src/server/` |
| `go test -v ./internal/...` | Verbose Go tests | `src/server/` |
| `go test -cover ./...` | Coverage report | `src/server/` |
| `npm test` | Run frontend tests | `src/web/` |
| `npm run test:unit` | Unit tests only | `src/web/` |
| `npm run test:e2e` | E2E tests | `src/web/` |
| `npx playwright test` | Playwright E2E | `src/web/` |

## Gaps and Recommendations

### Critical Gaps
1. **Integration Tests**: No database integration test suite
2. **E2E Coverage**: Minimal Playwright coverage for critical user flows
3. **Load Testing**: No performance/stress testing
4. **Contract Testing**: No API contract validation

### Medium Priority
1. **WebSocket Testing**: Gateway needs more comprehensive tests
2. **Billing Worker**: Async job processing needs test coverage
3. **Frontend Component Tests**: Expand React component coverage
4. **Error Scenarios**: More edge case and error handling tests

### Recommendations
1. Add integration test suite with test database
2. Implement contract testing with OpenAPI specs
3. Add critical path E2E tests (auth, chat, billing)
4. Create load test suite for WebSocket and API endpoints
5. Add mutation testing to verify test quality

---

*Document created: 2026-04-27*
*Applies to: Oblivious Project*
