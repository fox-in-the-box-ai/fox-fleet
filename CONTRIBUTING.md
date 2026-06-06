# Contributing to Fox Fleet

## Filing issues

Use the issue templates. Bug reports need: steps to reproduce, expected behavior, actual behavior. Feature requests need: use case, proposed solution.

## Opening PRs

1. Branch from `main`: `feat/<ticket>`, `fix/<ticket>`, `chore/<desc>`.
2. One logical change per PR. Cite the ticket in the PR description.
3. All checks must pass: `make lint`, `make test`, `make build`.
4. Squash-merge to `main`.

## Code style

- Go: `gofmt` + `goimports` (enforced by `golangci-lint`).
- Commit messages: imperative, present tense. Reference ticket IDs.
- No `Co-authored-by` trailers in commits.
- No AI tool names in commits, PR descriptions, or code comments.

## PR checklist

Before requesting review:

- [ ] `make build` succeeds
- [ ] `make test` passes
- [ ] `make lint` passes
- [ ] Ticket ID cited in PR description
- [ ] No new warnings introduced
