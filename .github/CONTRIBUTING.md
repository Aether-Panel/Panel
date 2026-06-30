# Contributing to Aether-Panel

First off, thank you for considering contributing to Aether-Panel! It's people like you that make Open Source such a great community.

## Project Structure (Standard Go Layout)

This project follows the official standard Go project layout:

- `cmd/panel/`: Contains the main entry point for the application. When running or building, target `cmd/panel/main.go`.
- `internal/`: Private application code. This code is specific to this project and cannot be imported by other projects. It contains core business logic, services, databases, etc.
- `pkg/`: Public library code. Code that could safely be imported and used by external projects.
- `client/`: Contains the frontend UI codebase (React/Vue/Astro etc.).
- `files/`: Contains utility functions specifically for filesystem interactions.

## Development Workflow

We use a standard `Makefile` to simplify development tasks.

### Prerequisites

- Go 1.24+
- Docker (for full E2E testing and environment setup)

### Common Commands

- `make run`: Run the application locally.
- `make build`: Build the Go binary to the `bin/` directory.
- `make test`: Run integration and unit tests.
- `make lint`: Run `golangci-lint`.
- `make fmt`: Auto-format code using `gofmt`.
- `make docker-build`: Rebuild the docker images.

## Coding Guidelines

- **Linting**: We enforce strict linting using `gocritic` and `revive` through `golangci-lint`. Ensure your code passes all lint checks before submitting a PR.
- **Formatting**: Always run `make fmt` (which uses `gofmt`) before committing.
- **Comments**: Exported functions and structs in `pkg/` should have appropriate GoDoc comments.

## Submitting a Pull Request

1. Fork the repository and create your feature branch from `dev2.0`.
2. Commit your changes with descriptive commit messages.
3. Push to your branch and open a Pull Request against `dev2.0`.

Thank you for contributing!
