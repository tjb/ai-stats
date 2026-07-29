# Security Policy

## Reporting a vulnerability

If you discover a security vulnerability in AI Stats, please **do not open a public GitHub
issue**. Instead, report it privately via [GitHub Security Advisories](https://github.com/tjb/ai-stats/security/advisories/new).

Please include:

- A description of the vulnerability and its potential impact
- Steps to reproduce (a minimal proof of concept is very helpful)
- The affected version/commit, if known

We'll acknowledge your report as quickly as we can and keep you updated as we investigate and
fix the issue. We ask that you give us a reasonable amount of time to address the problem
before any public disclosure.

## Supported versions

AI Stats does not yet have a formal release/versioning process — only the `main` branch is
supported. Please make sure you're running the latest commit before reporting an issue.

## Scope

AI Stats is a self-hosted application. Things that are particularly worth flagging:

- Authentication/session handling (`backend/internal/auth`)
- GitHub OAuth token storage and encryption (`backend/internal/crypto`)
- The push-ingest API key auth path (`backend/internal/handlers/ingest.go`)
- Any SQL injection, XSS, SSRF, or IDOR issue

Deployment/hardening of your own infrastructure (database credentials, network exposure, TLS
termination, etc.) is your responsibility as the operator of a self-hosted instance.
