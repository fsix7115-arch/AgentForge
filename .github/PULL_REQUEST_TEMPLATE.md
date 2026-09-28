## Description

What does this PR do? Why?

## Type of change

- [ ] Bug fix (non-breaking change that fixes an issue)
- [ ] New feature (non-breaking change that adds functionality)
- [ ] Breaking change (fix or feature that would cause existing behavior to change)
- [ ] Documentation only
- [ ] Refactor (no behavior change)
- [ ] Tests only

## Testing done

- [ ] `go test ./...` passes
- [ ] `go test -race ./...` passes
- [ ] `go vet ./...` passes
- [ ] `go build -o /dev/null ./cmd/agentforge` succeeds
- [ ] Tested with `CGO_ENABLED=0` (required for Android cross-compile)

If you added new functionality:
- [ ] New tests cover the new code
- [ ] `agentforge doctor` still works offline (no network calls in the doctor path)

If you changed a provider or added one:
- [ ] Tested against the real API, not just mocks

## Tested on a phone?

If applicable: device, Android version, Termux version, result.

## Checklist

- [ ] The binary still cross-compiles: `GOOS=android GOARCH=arm64 CGO_ENABLED=0 go build ./cmd/agentforge`
- [ ] No new external dependencies added without discussion
- [ ] Comments explain why, not just what
- [ ] The PR description explains the reason for the change, not just what changed
