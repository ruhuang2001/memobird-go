# Local end-to-end checks

Run `make e2e` with Chrome or Chromium installed. No printer credentials or
external print service are used. Artifacts are saved in `build/review-e2e/`.

Failure cases defined before the fixes:

- A short phrase that fits on one line wraps at its final space, or loses its
  final word when `MaxLines` is one.
- A long paragraph is fully measured despite a one-line output limit.
- Repeated renders start a new Chrome process instead of reusing the browser.
- Canceling a render stops the shared browser and breaks the next request.
- Inline HTML scripts do not run, leaving their generated content missing.
- Long pages lose content when captured in segments.
- Importing only the API client still compiles Chrome dependencies.
- Moving image processing changes the rendered print payload or breaks the
  bind, persist, restore, render, convert, and submit workflow.

The workflow check uses a real browser, SQLite, and a local HTTP server standing
in for the vendor API. The saved PNG is the actual conversion request payload;
the JSON receipt records accepted submissions. It does not verify physical
paper output or the vendor's image conversion implementation.

## Dependency security regression (#8)

Failure cases recorded before upgrading dependencies:

- The trusted embedded font or a caller-supplied valid TTF no longer renders.
- Invalid or truncated custom fonts panic, allocate excessively, or succeed.
- Rendering changes image dimensions or pixels for the same trusted inputs.
- A rejected font breaks the next render or the local print workflow.
- The upgraded modules require a newer Go version than CI actually uses.

`TestE2EFontRendering` exercises the public renderer with the embedded Go Regular
font, explicit Go Regular and Go Bold custom fonts, invalid bytes, and a truncated
TTF. It retains PNGs and `font-report.json` with decoded-pixel hashes, elapsed time,
and Go allocation observations. These measurements are observations, not resource
limits or proof against every malicious font. No physical printer is contacted.

Security checks matching CI (run from the repository root):

```sh
GOTOOLCHAIN=go1.26.10 go run golang.org/x/vuln/cmd/govulncheck@v1.1.4 ./...
GOTOOLCHAIN=go1.26.10 make e2e
GOTOOLCHAIN=go1.26.10 go test -race ./...
GOTOOLCHAIN=go1.26.10 go vet ./...
GOTOOLCHAIN=go1.26.10 go build ./...
# Build the scanner for the host, then analyze Windows imports:
GOTOOLCHAIN=go1.26.10 GOBIN=$PWD/build/tools go install golang.org/x/vuln/cmd/govulncheck@v1.1.4
GOTOOLCHAIN=go1.26.10 GOOS=windows GOARCH=amd64 build/tools/govulncheck ./...
```

To compare an upgrade, run the font case on each dependency revision with
`MEMOBIRD_E2E=1 MEMOBIRD_E2E_ARTIFACTS=<absolute-output-directory> go test ./e2e -run TestE2EFontRendering -count=1 -v`.
Compare `Name`, `Width`, `Height`, and `PixelSHA256` in the two reports; timing
and allocation counters can vary between runs and with race instrumentation.

Verified locally on 2026-10-22 with Go 1.26.10 (darwin/arm64):

- Browser E2E, local bind/persist/restore/render/convert/submit workflow, existing
  race suite, build, and vet passed. No vendor or physical print requests.
- Source govulncheck returned zero findings for the host and windows/amd64.
  The JSON stream includes advisory metadata; metadata alone is not a finding.
- All valid font PNGs remained 384x90 with identical decoded-pixel hashes before
  and after the upgrade. Invalid and truncated fonts returned parse errors; a
  subsequent default-font render succeeded. Under race instrumentation, valid
  cases allocated about 1.02 MB and rejected cases 496 bytes. These controlled
  inputs are not an exploit reproduction or exhaustive malformed-font corpus.
- GitHub vulnerability-alerts enablement was verified by HTTP 204; the dependency
  graph SBOM endpoint returned 41 packages. Remote Actions execution remains to
  be verified after the changes are pushed.

The original font parsing call was reachable via caller-provided `FontData`.
The precis/norm and Windows advisory matches did not establish application
reachability; the upgraded target-specific scans found no remaining findings.
