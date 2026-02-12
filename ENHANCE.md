# Memobird Playground — Enhancement Review

Date: 2026-02-12  
Scope: Repository review of tracked source code and build/test setup (`go test ./...`, `go test -race ./...`, `go vet ./...` all pass).

## Executive Summary

The project is in a solid state: clear package boundaries, meaningful tests in core modules, and a practical CLI workflow. The highest-value improvements are around **resilience**, **performance bounds**, and **operational hardening**.

Top priorities:

1. Improve runtime robustness (timeouts/retries/context cancellation).
2. Bound render/image workloads to avoid memory spikes.
3. Reduce duplication in API/client and command orchestration layers.
4. Expand tests/CI coverage for config/storage/CLI edge cases.

---

## What’s Already Working Well

- Clear internal package split (`config`, `memobird`, `renderer`, `storage`, `formatter`).
- Good use of `context` and structured logging with `slog`.
- Renderer lifecycle management includes explicit resource cleanup.
- Image processing pipeline (resize + dithering) is deterministic and test-covered.
- API client tests use `httptest` and validate key integration behavior.

---

## 1) Code Quality

### Findings

- **Duplicate request/response handling** in `internal/memobird/client.go`:
  - Each API method repeats request execution, JSON unmarshal, and success checks.
  - This increases maintenance cost and risk of inconsistent behavior.

- **CLI orchestration is monolithic** in `cmd/main.go`:
  - Flag parsing, startup wiring, command routing, and command behavior all live in one file.
  - Adding new commands or shared pre/post behavior will get harder over time.

- **Config loading is less deployment-friendly** in `internal/config/config.go`:
  - `ReadInConfig()` failure is fatal, so env-only deployments are not supported.
  - `SetEnvKeyReplacer` is not configured, making nested env overrides less predictable.

### Recommendations

1. Add a generic API helper in client layer, e.g. `postFormJSON[T any](ctx, endpoint, params)`.
2. Split command handlers into focused functions/files (bind, text, html, image modes).
3. Allow missing config file when env vars are provided; treat missing file as non-fatal.
4. Add `viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))` for reliable env mapping.

### Quick Implementation Notes

- Keep public API unchanged; refactor internals first to avoid breaking users.
- Add table-driven tests around config precedence (defaults < file < env).

---

## 2) Performance

### Findings

- **Unbounded full-page screenshots** in `internal/renderer/renderer.go` (`chromedp.FullScreenshot`) can create very large bitmaps for long pages, increasing memory/time significantly.
- **Image dithering loop uses per-pixel `img.At` calls** in `internal/renderer/image.go`, which is slower than direct pixel buffer access for large images.
- **Two long-lived browser sessions** (`urlSession`, `htmlSession`) increase idle memory footprint.

### Recommendations

1. Add render bounds:
   - max page height,
   - max pixel count,
   - optional pagination/segment rendering for long pages.
2. Optimize monochrome conversion by reading RGBA pixels directly from `img.Pix`.
3. Make rendering quality tunable (fast vs quality):
   - `ApproxBiLinear` for speed mode,
   - `CatmullRom` for quality mode.
4. Consolidate browser session strategy (single allocator + per-tab behavior) unless benchmarks show dual-session advantage.

### Suggested Acceptance Criteria

- Large-page rendering no longer OOMs and stays within defined memory/time limits.
- `ProcessImageForPrint` benchmark improves by at least 20% in speed mode.

---

## 3) Architecture

### Findings

- Core abstractions are good, but orchestration logic is concentrated in `cmd/main.go`.
- Cross-cutting behaviors (validation, retries, logging context, status polling) are not centralized.

### Recommendations

1. Introduce an `app` or `service` layer for print workflows:
   - `BindService`, `PrintTextService`, `PrintImageService`.
2. Keep `cmd` as a thin adapter (flags -> service call).
3. Define narrow interfaces for external effects (`PrinterAPI`, `Renderer`, `BindingStore`) to improve testability.
4. Add package-level docs describing boundaries and dependency direction.

### Benefits

- Easier to add features (scheduled printing, retries, print status polling).
- Faster unit testing without spinning Chrome or HTTP servers.

---

## 4) Dependencies & Build

### Findings

- Dependency set is reasonable, but includes heavy stacks:
  - `viper` (large transitive tree),
  - `modernc.org/sqlite` (binary-size/perf tradeoffs vs CGO SQLite).
- CI pipeline in `.github/workflows/go.yml` runs build and tests, but omits race detector and vet/static lint.

### Recommendations

1. Keep current dependencies for now, but document tradeoffs in README.
2. Add dependency hygiene automation:
   - Dependabot/Renovate,
   - `govulncheck` in CI,
   - periodic `go mod tidy` check in CI.
3. Align CI with local quality bar:
   - add `go test -race ./...`,
   - add `go vet ./...`,
   - optionally add `golangci-lint` with a minimal rule set.

---

## 5) Best Practices, Security, Reliability

### Findings

- URL validation in `internal/renderer/renderer.go` only enforces scheme; host constraints are minimal.
- Renderer uses `--no-sandbox`; acceptable in some environments but weakens isolation for untrusted content.
- API calls rely on request timeout but lack retry/backoff for transient network failures.
- Logs include full URLs; these can include sensitive query params.

### Recommendations

1. Harden URL validation:
   - require non-empty host,
   - optionally add allowlist/denylist mode.
2. Make sandbox behavior configurable (`secure` default, `no-sandbox` opt-in for constrained environments).
3. Add retry policy (exponential backoff + jitter) for idempotent calls and temporary failures.
4. Redact sensitive data in logs (query params/tokens/access-like fields).
5. Use `signal.NotifyContext` in CLI to support graceful cancellation (Ctrl+C).

---

## 6) Testing & CI Gaps

### Current State

- Good unit tests for `formatter`, `renderer`, and `memobird`.
- No tests for `internal/config`, `internal/storage`, or `cmd` behavior.

### Recommendations

1. Add tests for `internal/config`:
   - defaults,
   - file loading,
   - env override precedence,
   - validation failures.
2. Add tests for `internal/storage`:
   - migration behavior,
   - upsert behavior,
   - no-row behavior,
   - concurrent access scenario (to catch lock issues early).
3. Add lightweight `cmd` tests for flag conflict and missing-user-id flows.
4. Add benchmark tracking for image processing regression detection.

---

## Prioritized Backlog (Actionable)

### P0 (High Impact / Low-Medium Effort)

1. Refactor client request boilerplate into a single generic helper.
2. Add bounded render safeguards (max height/pixels/time).
3. Expand config loading to support env-only mode + env key replacer.
4. Extend CI with race + vet + vuln scan.

### P1 (High Impact / Medium Effort)

1. Split `cmd/main.go` into command handlers + service layer.
2. Add retry/backoff and log-redaction strategy.
3. Add missing tests for `config` and `storage`.

### P2 (Strategic / Medium-Higher Effort)

1. Optimize dithering internals and add speed/quality mode.
2. Revisit browser session architecture (single vs dual session, benchmark-driven).
3. Re-evaluate dependency footprint if binary size/startup time becomes a target KPI.

---

## Suggested 3-Phase Execution Plan

### Phase 1 (1–2 days)

- Implement P0 items 1, 3, and 4.
- Add tests for config precedence and storage CRUD path.

### Phase 2 (2–4 days)

- Implement render bounds and safe defaults.
- Introduce retries + graceful shutdown context.
- Refactor CLI orchestration into handlers.

### Phase 3 (as needed)

- Benchmark and optimize image pipeline.
- Decide on dependency simplification based on measured binary/runtime goals.

---

## Success Metrics to Track

- Fewer duplicated code paths in client and command layers.
- Stable runtime behavior on long/complex pages (no OOM/timeouts beyond configured bounds).
- Higher test coverage in currently untested packages (`config`, `storage`, `cmd`).
- CI catches race/vet/vulnerability issues before merge.

