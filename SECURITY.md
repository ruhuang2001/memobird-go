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

This library sends requests to the official Memobird Web API and renders text
locally with Go font and image packages. It does not run a browser, render HTML,
or fetch page URLs. Reports that matter here include credential leakage in logs
or errors, unsafe handling of API responses, and denial of service through text
layout or caller-supplied font data.

Both print methods upload image content to the vendor for conversion and
submission. Applications are responsible for deciding what content may be sent,
protecting credentials, and bounding untrusted text, image, and font inputs.
Vulnerabilities in the Memobird service or printer firmware are outside this
repository's scope, though we are interested in how this library exposes them.
