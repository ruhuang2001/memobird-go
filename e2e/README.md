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
