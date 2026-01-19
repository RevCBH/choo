---
task: 1
status: pending
backpressure: "test -f .github/workflows/ci.yml && head -1 .github/workflows/ci.yml | grep -q 'name:'"
depends_on: []
---

# Create GitHub Actions CI Workflow

**Parent spec**: `/specs/CI.md`
**Task**: #1 of 3 in implementation plan

## Objective

Create the GitHub Actions workflow file that runs build, test, vet, and lint jobs on pull requests and pushes to main.

## Dependencies

### Task Dependencies (within this unit)
- None

### External Dependencies
- GitHub Actions must be enabled on the repository

## Deliverables

### Files to Create/Modify
```
.github/
└── workflows/
    └── ci.yml    # CREATE: CI workflow definition
```

### Content

```yaml
# .github/workflows/ci.yml
name: CI

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version: '1.22'

      - name: Build
        run: go build -v ./...

      - name: Test
        run: go test -v -race -coverprofile=coverage.txt ./...

      - name: Vet
        run: go vet ./...

  lint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version: '1.22'

      - name: golangci-lint
        uses: golangci/golangci-lint-action@v4
        with:
          version: latest
```

## Backpressure

### Validation Command
```bash
test -f .github/workflows/ci.yml && head -1 .github/workflows/ci.yml | grep -q "name:"
```

### Success Criteria
- File `.github/workflows/ci.yml` exists
- File begins with valid YAML containing `name:` key

## NOT In Scope
- Branch protection rules configuration
- Coverage upload to external services
- Go module caching optimization
- Matrix testing across Go versions
- Any Go code changes
