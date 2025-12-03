# AGENTS.md

## Development Commands

```bash
# Build and test
go build
go test ./...
go test -run TestSpecificFunction ./internal/core

# Code quality
go fmt ./...
golangci-lint run
go mod tidy
```

## Code Style Guidelines

- Use early returns instead of nested conditionals
- Follow Go idioms and standard library patterns
- Import groups: stdlib, third-party, internal packages
- Error handling: wrap with fmt.Errorf("%w", err) for context
- Use meaningful error messages with operation context
- Variable names: camelCase, be descriptive but concise
- File permissions: mask to 0700/0600 for security
- Always clear sensitive data with crypto.ClearBytes()
- Context cancellation checks in long-running operations
- Use filepath.Join for cross-platform paths