# Contributing

Thanks for taking the time to improve `memobird-go`.

## Requirements

- Go 1.26.11 or newer, the version declared in `go.mod`
- A local Chrome or Chromium installation to run the browser end-to-end tests
- Optional: `golangci-lint` v2.15.0, which CI pins

## Getting started

```bash
git clone git@github.com:ruhuang2001/memobird-go.git
cd memobird-go
go mod download
make test    # go test -race ./...
make e2e     # real Chrome rendering plus a local workflow, artifacts in build/review-e2e
```

`make e2e` uses a local stand-in for the Memobird API and never contacts a real
printer, so it is safe to run on any machine with Chrome installed. If Chrome
lives somewhere unusual, set `MEMOBIRD_E2E` and `MEMOBIRD_E2E_ARTIFACTS`, or
invoke `go test ./renderer -run TestBrowserE2E` with your own environment.

## Before opening a pull request

Run the same checks CI runs:

```bash
gofmt -l .                 # must print nothing
go vet ./...
golangci-lint run ./...
go test -race ./...
go run golang.org/x/vuln/cmd/govulncheck@latest ./...
go mod tidy && git diff --exit-code -- go.mod go.sum
```

A pull request should keep `go.mod`'s `go` directive unchanged unless the change
genuinely requires a newer toolchain; raising it forces every downstream user to
upgrade.

## Design constraints

- `memobird` is the API client. Keep it free of browser dependencies; `go list
  -deps ./memobird` must not include `chromedp` or the `renderer` package.
- `renderer` is opt-in and may depend on Chrome. Anything testable without a
  browser should have a unit test in `renderer_test.go`; browser behaviour
  belongs in `browser_e2e_test.go` behind `MEMOBIRD_E2E=1`.
- Errors that callers must distinguish are exported sentinels and are tested
  with `errors.Is`; remote API rejections are `*APIError`.
- Print submissions are not idempotent. Do not add hidden retries that could
  print a page twice.

## Commit messages

Use [Conventional Commits](https://www.conventionalcommits.org/):

- `feat:` a new capability
- `fix:` a bug fix
- `docs:` documentation only
- `chore(deps):` dependency updates
- `refactor:`, `test:`, `ci:` as their names suggest

Keep history linear: rebase onto `main`, and do not create merge commits.

## Releasing

Maintainers only. Releases are cut from `main` after CI is green:

1. Move the `Unreleased` entries in `CHANGELOG.md` into a new `## [vX.Y.Z]`
   section and update the comparison links.
2. Bump `memobird.Version` in `memobird/client.go` to `X.Y.Z`.
3. Commit, then push a `vX.Y.Z` tag.

The `Release` workflow verifies that the tag, `memobird.Version`, and the
changelog section agree, runs the vulnerability scan and tests again, and
publishes a GitHub release. pkg.go.dev picks up the tag automatically.
