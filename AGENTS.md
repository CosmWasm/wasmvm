# AGENTS instructions

## Formatting

- Run golangci-lint run ./... --fix on the whole repo whenever we change Go files.
- Run `cargo fmt` on any modified Rust files.
- Run `prettier -w` on any modified Markdown files.

## Testing

- Use `golangci-lint run ./... --fix` before running `make test` and make sure that all lint issues are fixed before running tests.
- When Go or Rust code is changed, run `make test` before committing.

## PR message

- Summarize the changes and mention any test commands that were executed.
