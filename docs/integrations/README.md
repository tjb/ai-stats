# Integration specs

Implementation specs for the data sources AI Stats connects to. Each one was researched from the
vendor's primary sources and then independently re-verified; the verification notes at the end of
each file list what was corrected. Claims that could not be confirmed are marked UNVERIFIED.

| Spec | Source | What AI Stats gets from it |
|---|---|---|
| [GitHub Copilot usage metrics](github-copilot-usage-metrics.md) | Copilot usage metrics REST API (report downloads) and billing usage endpoints | Org and per-user daily Copilot activity, PR metrics, per-user AI credits |
| [Anthropic Claude Code Analytics](anthropic-claude-code-analytics.md) | Claude Code Analytics Admin API, Usage & Cost Admin API, Enterprise Analytics API | Per-user daily sessions, lines, commits/PRs, tokens and estimated cost by model |
| [Claude Code OpenTelemetry](claude-code-opentelemetry.md) | Claude Code's built-in OTLP export, rolled out through managed settings | Push telemetry: tokens, cost, sessions, commits/PRs per user, plus OTLP/HTTP receiver rules |
| [Cursor Admin API](cursor-admin-api.md) | Cursor Admin, Analytics and AI Code Tracking APIs | Per-user daily usage, per-event tokens and cost, spend |

All four were researched on 2026-10-23. Vendor APIs change, so re-check anything load-bearing
against the live API when building or debugging a connector.
