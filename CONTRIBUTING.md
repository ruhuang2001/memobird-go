# Contributing

Thanks for taking the time to improve `memobird-go`.

## Requirements

- Go 1.26.11 or newer, the version declared in `go.mod`
- Optional: `golangci-lint` v2.15.0, which CI pins

## Getting started

```bash
git clone git@github.com:ruhuang2001/memobird-go.git
cd memobird-go
go mod download
make test    # go test -race ./...
make e2e     # local workflow E2E, artifacts in build/review-e2e
```

`make e2e` uses a local stand-in for the Memobird API and never contacts a real
printer. It sets `MEMOBIRD_E2E=1` and writes artifacts to `build/review-e2e/`.
For a custom artifact directory, invoke the tests directly as described in
[e2e/README.md](e2e/README.md).

## Before opening a pull request

Run the same checks CI runs:

```bash
export GOTOOLCHAIN=go1.26.11
gofmt -l .                 # must print nothing
go vet ./...
golangci-lint run ./...
go test -race ./...
make e2e
go run golang.org/x/vuln/cmd/govulncheck@latest ./...
go mod tidy && git diff --exit-code -- go.mod go.sum
```

A pull request should keep `go.mod`'s `go` directive unchanged unless the change
genuinely requires a newer toolchain; raising it forces every downstream user to
upgrade.

## Design constraints

- `memobird` is the API client. Keep it free of browser, database, and
  config-loader dependencies; `go list -deps ./memobird` must not include
  `chromedp`, `modernc.org/sqlite`, or `spf13/viper`.
- `textrender` is pure Go. A caller that needs a browser renders the page itself
  and submits the image with `PrintImage`; do not add a renderer package here.
- Errors that callers must distinguish are exported sentinels and are tested
  with `errors.Is`; remote API rejections are `*APIError`.
- Print submissions are not idempotent. Do not add hidden retries that could
  print a page twice.

## Commit messages

Use [Conventional Commits](https://www.conventionalcommits.org/) with a scope.
Every commit subject must read `type(scope): summary`; a bare `type: summary`
is rejected in review, including for `docs`, `test`, and `ci` commits.

- `feat(memobird):` a new capability in the API client
- `feat(textrender):` a new capability in text rendering
- `fix(memobird):`, `fix(textrender):` bug fixes
- `test(e2e):` end-to-end test harness changes
- `ci(actions):` GitHub Actions workflow changes
- `docs(contributing):` documentation only
- `chore(deps):` dependency upgrades
- `refactor(memobird):` API client restructuring
- `perf(textrender):` renderer performance improvements

Pick the package, not the file: `fix(textrender): keep trailing spaces in wrapped
lines`, not `fix: update textrender.go`. One commit carries one logical change;
split unrelated edits instead of widening the scope. When a change spans
packages, use the package callers care about. Keep the subject short; add a body
only when the rationale or migration needs explaining.

Keep history linear: rebase onto `main`, and do not create merge commits.

## Releasing

Maintainers only. Releases are cut from `main` after CI is green:

1. Move the `Unreleased` entries in `CHANGELOG.md` into a new `## [vX.Y.Z]`
   section and update the comparison links.
2. Bump `memobird.Version` in `memobird/client.go` to `X.Y.Z`.
3. Update the installation version and any migration notes in `README.md`.
4. Commit and push the release preparation to `main`, wait for CI, then push a
   `vX.Y.Z` tag at that commit. Do not move an existing release tag.

The `Release` workflow verifies that the tag, `memobird.Version`, and the
changelog section agree, runs the vulnerability scan and tests again, and
publishes a GitHub release. Check that the tagged module can be downloaded;
publication of the GitHub release alone does not verify module availability.
