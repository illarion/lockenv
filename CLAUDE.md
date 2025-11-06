# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

lockenv is a CLI-friendly secret storage tool designed to allow safe committing of secrets to source control management (SCM) systems.

## Development Commands

Since this is a new Go project, common commands include:

```bash
# Initialize Go dependencies
go mod tidy

# Build the project
go build

# Run tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Format code
go fmt ./...

# Lint code (requires golangci-lint)
golangci-lint run

# Run a specific test
go test -run TestName ./...
```

## Architecture Notes

This is a new Go module (github.com/live-labs/lockenv) using Go 1.24.5. The project aims to provide:
- CLI-friendly interface for secret storage
- Safe storage mechanism that allows committing encrypted secrets to SCM
- Simple and straightforward usage

When implementing features:
- Follow Go idioms and best practices
- Always use the standard library where possible
- Always refer to context7 mcp to get updated documentation and examples of Go and libraries
- Use early returns instead of nested conditionals
- Implement proper error handling with meaningful error messages
- Consider CLI user experience for all interfaces
- Ensure secrets are properly encrypted before storage
- Provide clear documentation for CLI commands and options

## Security Considerations

Given this is a secret storage tool:
- All secrets must be encrypted before storage
- Use industry-standard encryption algorithms
- Implement secure key derivation functions
- Consider using established crypto libraries rather than implementing custom crypto
- Ensure proper memory handling to avoid secret leakage
- Implement secure deletion of sensitive data from memory
