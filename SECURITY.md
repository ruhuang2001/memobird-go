# Security Policy

## Supported versions

The latest tagged minor release receives security fixes.

| Version | Supported |
| ------- | --------- |
| 0.0.x   | Yes       |

## Reporting a vulnerability

Report suspected vulnerabilities privately through GitHub's
[Security Advisories](https://github.com/ruhuang2001/memobird-go/security/advisories/new)
for this repository. Do not open a public issue.

Please include the affected version, a reproduction or proof of concept, and
the impact you believe it has. You can expect an acknowledgement within a few
days; a fix and coordinated disclosure follow once the report is validated.

## Dependency vulnerabilities

CI runs `govulncheck` against the Go standard library and every dependency on
each push. A failing scan blocks a release. Dependabot keeps modules and GitHub
Actions current; its pull requests still require the full CI suite to pass.

## Scope

This library sends requests to the official Memobird Web API and, for local
rendering, drives a headless Chrome instance. Reports that matter here include
credential leakage in logs or errors, injection into rendered HTML, unsafe URL
handling, unbounded memory growth from remote responses, and denial of service
through the renderer. Vulnerabilities in the Memobird service itself, in Chrome,
or in the printer firmware are outside this repository's scope, though we are
still interested in how this library exposes them.
