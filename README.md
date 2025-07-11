# Katpool Blockfetcher

Fetches new block templates from the Kaspa network via gRPC and publishes them to a Redis channel. `katpool-app` listens to this channel.

## Prerequisites

Before you begin, ensure you have the following installed on your system:

- **Go** (version ≥ 1.20): [Download Go](https://golang.org/dl/)
- **Python 3** and **pip**: Required for pre-commit hooks
- **Git**: For version control

### Verify Installation

Check if Go is properly installed:

```bash
go version
```

Check if Python and pip are available:

```bash
python3 --version
pip3 --version
```

## Pre-commit Hooks

This project uses [`pre-commit`](https://pre-commit.com) to automatically format and lint Go code before each commit, ensuring code quality and consistency.

### Setup Pre-commit

1. Install pre-commit:

```bash
pip3 install pre-commit
```

2. Install the Git hooks:

```bash
pre-commit install
```

3. Run hooks on all files (recommended for first-time setup):

```bash
pre-commit run --all-files
```

### Installing Go Linters and Formatters

Some pre-commit hooks require Go tools to be installed and on your PATH. Run these commands to install them:

1. goimports is part of golang.org/x/tools

```bash
go install golang.org/x/tools/cmd/goimports@latest
```

2. golint is part of golang.org/x/lint
```bash
go install golang.org/x/lint/golint@latest
```

3. Ensure $GOPATH/bin is on your PATH

```bash
export PATH=$PATH:$(go env GOPATH)/bin
```

4. Verify installation
```bash
which goimports
which golint
```

### Troubleshooting Pre-commit

If a commit fails due to pre-commit hooks:

1. Review the error messages
2. Fix any formatting or linting issues
3. Run hooks manually to verify fixes:

```bash
pre-commit run --all-files
```

4. Stage your changes and commit again
