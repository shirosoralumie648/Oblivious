---
description: Coding conventions and standards
created: 2026-04-27
---
# Conventions

## Naming Conventions

| Element | Pattern | Examples |
|---------|---------|----------|
| Go Files | snake_case.go | `chat_handler.go`, `auth_middleware.go` |
| Go Packages | Short, lowercase | `auth`, `chat`, `billing` |
| Go Types | PascalCase | `ChatHandler`, `AuthService` |
| Go Interfaces | PascalCase-er | `Handler`, `Service`, `Store` |
| Go Functions | PascalCase (exported), camelCase (private) | `GetUser()`, `validateToken()` |
| Go Constants | CamelCase | `defaultTimeout`, `maxRetries` |
| Go Test Files | `*_test.go` | `handler_test.go` |
| TypeScript Files | camelCase.ts | `apiClient.ts`, `useAuth.ts` |
| React Components | PascalCase.tsx | `ChatWindow.tsx`, `LoginForm.tsx` |
| React Hooks | camelCase with `use` prefix | `useAuth`, `useChat` |
| CSS Classes | kebab-case | `.chat-container`, `.btn-primary` |
| Environment Variables | UPPER_SNAKE_CASE | `DATABASE_URL`, `JWT_SECRET` |

## Code Organization

### Go Backend Structure
```
internal/
├── auth/           # Authentication domain
│   ├── service.go  # Business logic
│   ├── store.go    # Data access
│   └── ...
├── chat/           # Chat domain
│   ├── service.go
│   ├── gateway.go  # WebSocket handling
│   └── store.go
├── http/           # HTTP handlers
│   ├── server.go   # Server setup
│   ├── *_handler.go
│   └── *_middleware.go
├── db/             # Database utilities
└── config/         # Configuration
```

### Domain-Driven Design Patterns
- **Bounded Contexts**: Each domain (auth, chat, billing) has its own package
- **Service Layer**: Business logic in `service.go` files
- **Store Layer**: Data access in `store.go` files
- **Handler Layer**: HTTP handling in `*_handler.go` files
- **Dependency Injection**: Services receive stores via constructors

## Style Guidelines

### Go Style
- **Linting**: Uses `gofmt` for formatting
- **Imports**: Grouped as stdlib, third-party, local
- **Error Handling**: Explicit error checking, early returns
- **Context**: `context.Context` passed through call chains
- **Documentation**: Exported items have doc comments

### TypeScript/React Style
- **Linting**: ESLint with React/TypeScript rules
- **Formatting**: Prettier (implied by standard React projects)
- **Imports**: Absolute imports preferred (`@/` alias)
- **Components**: Functional components with hooks
- **State Management**: React hooks (useState, useEffect)
- **API Calls**: Custom hooks or service functions

## Common Patterns

### Go Patterns
| Pattern | Where Used | Example |
|---------|------------|---------|
| Constructor Pattern | Store/Service creation | `NewChatStore(db *sql.DB) *ChatStore` |
| Interface Segregation | Store abstractions | `ChatStore` interface |
| Dependency Injection | Handler setup | Handlers receive services |
| Middleware Chain | HTTP processing | Auth, CORS, logging |
| Repository Pattern | Data access | Store layer methods |
| Circuit Breaker | Resilience (implied) | Billing integration |

### React Patterns
| Pattern | Where Used | Example |
|---------|------------|---------|
| Custom Hooks | Reusable logic | `useAuth`, `useChat` |
| Context API | Global state | Authentication context |
| Composition | Component building | Small, reusable components |
| Container/Presentational | Separation | Containers with state, presentational with props |

## Anti-Patterns Observed

| Anti-Pattern | Location | Recommendation |
|--------------|----------|----------------|
| Large Handler Files | `chat_handler.go` (300+ lines) | Split into route-specific handlers |
| Deep Nesting | Some business logic | Early returns, extract methods |
| Mixed Concerns | Gateway has business logic | Move to service layer |
| String Concatenation | SQL queries | Use parameterized queries |
| Missing Input Validation | Some handlers | Add validation at boundaries |

## File Size Guidelines

- **Go Files**: Keep under 500 lines
- **Handler Files**: Split by resource/entity
- **Service Files**: Split by domain operation
- **React Components**: One component per file
- **Test Files**: Mirror source structure

## Documentation Standards

### Go Documentation
```go
// ServiceName provides [what it does].
// It handles [specific responsibility].
type ServiceName struct {
    // fields documented if not obvious
}

// MethodName does [what] and returns [what].
// It requires [prerequisites] and returns [error conditions].
func (s *ServiceName) MethodName(ctx context.Context, param string) (Result, error) {
    // implementation
}
```

### API Documentation
- Endpoints documented in design docs
- Request/response schemas defined
- Authentication requirements specified
- Error responses documented

## Git Conventions

- **Branch Names**: `feature/description`, `fix/description`
- **Commit Messages**: Conventional commits format
- **PR Size**: Keep under 500 lines changed
- **Review Requirements**: At least one approval

## Testing Conventions

See [TESTING.md](./TESTING.md) for detailed testing standards.

---

*Document created: 2026-04-27*
*Applies to: Oblivious Project*
