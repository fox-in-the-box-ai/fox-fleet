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

## Internationalization (i18n)

The panel SPA loads translations from `panel/spa/static/i18n/<locale>.json`. English (`en.json`) is the reference locale.

**Adding a new language:**

1. Copy `en.json` to `<locale>.json` (e.g. `de.json`).
2. Translate every value. Keep the JSON keys identical to `en.json`.
3. Add the `<option>` to the language selector in `panel/spa/static/index.html`.
4. Run `scripts/validate-i18n.sh` — it checks that all locales have the same keys as `en.json`.

**Adding a new UI string:**

1. Add the key and English value to `en.json`.
2. Add the same key with translated values to every other locale file.
3. Reference it in HTML with `data-i18n="your.key"` or in JS with `t("your.key")`.
4. Run `scripts/validate-i18n.sh` to verify key parity.

## PR checklist

Before requesting review:

- [ ] `make build` succeeds
- [ ] `make test` passes
- [ ] `make lint` passes
- [ ] Ticket ID cited in PR description
- [ ] No new warnings introduced
