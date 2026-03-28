# AGENTS.md

## Project: GoHashCheckMe

A Go CLI tool that runs commands on files and tracks SHA256 hashes to skip unchanged files (audit mode).

## Commands

```bash
make dev      # Format, vet, test, build (full cycle)
make test     # Run tests (-short flags)
make test-race# Race detector tests
make build    # Build to build/ghc
make lint     # golangci-lint (install if missing)
```

## Workflow

1. **Before changes**: Run `make test-race` to ensure tests pass
2. **After changes**: `make dev` validates the full pipeline
3. **PR/merge**: CI runs on `.github/workflows/test.yml`

## Gotchas

- `make test` uses `-short` flag; use `make test-verbose` for full output
- `go.mod` pins Go 1.24.4
- CGO is disabled (`CGO_ENABLED=0`) for cross-platform builds
- Hash file format is JSONL (one JSON object per line)
- `audit` vs `changed` fields: `audited=true` means checked against history, `changed=true` means file differs from last run
