# Spec (verified and corrected): Claude Code OpenTelemetry → AI Stats OTLP/HTTP receiver

> **Status:** implementation spec, researched and independently re-verified on 2026-10-23 against the
> primary sources cited inline. Claims that could not be confirmed are marked UNVERIFIED. Re-check
> anything load-bearing against the live API before relying on it; vendor APIs change.

## How this was verified (independent re-check, 2026-10-24)

I did not rely on the original author's captures. Every claim below was checked again against one of these:

- **[doc]** The primary docs, fetched again:
  - `https://code.claude.com/docs/en/monitoring-usage.md`, `managed-settings.md`, `server-managed-settings.md`, `settings-reference.md`.
  - The OTLP spec and protos from `raw.githubusercontent.com/open-telemetry/opentelemetry-proto/main`.
- **[src]** Source code:
  - `openai/codex` (`codex-rs/otel/*`, `codex-rs/config/src/types.rs`, `codex-rs/protocol/src/protocol.rs`).
  - `google-gemini/gemini-cli` (`packages/core/src/telemetry/*`).
  - `microsoft/vscode-copilot-chat` (`src/platform/otel/*`).
  - `open-telemetry/opentelemetry-js` (`otlp-exporter-base` retry logic, metric exporter temporality).
  - `open-telemetry/opentelemetry-rust` (`opentelemetry-otlp` endpoint resolution).
  - `open-telemetry/opentelemetry-collector` (`otlpreceiver`).
- **[cap2]** My own captures with the installed **Claude Code 2.1.309**:
  - Setup: a clean `env -i`, an isolated `HOME`, a fake API key, and `ANTHROPIC_BASE_URL` pointed at a local fake Messages API.
  - Sink: my own capture sink, `verify-scripts/sink.py`.
  - 19 runs covering: `http/json`, `http/protobuf`+gzip, delta and cumulative temporality, trailing-slash base path, per-signal endpoints, `OTEL_RESOURCE_ATTRIBUTES` with a space, server replies of 503 with Retry-After of 1, 8, 12 and 30 s, 500, 400, 429 without Retry-After, partial success, a percent-encoded header, no protocol set, content flags on, a real `git commit`, and a `gh pr create`.
  - Evidence: `<verification scratch: verify-runs/v*/cap/>`.
- **[go2]** Go program `<verification scratch: verify-go/main.go>`:
  - Built against `slim/otlp v1.11.0`, `protobuf v1.36.11` and `genproto/googleapis/rpc v0.0.0-20260706201446-f0a921348800` with Go 1.25.0.
  - Re-aggregates the author's captures and mine.
- **[sql2]** The spec's DDL and rollup query, re-run on local Postgres 16 in a throwaway DB (`verify-go/sql_test.sql`).

Labels used below:

- **UNVERIFIED**: no reachable source confirms the claim.
- **[CORRECTED]**: the original spec was wrong.
- **[ADDED]**: missing from the original spec.

## 0. Decisions

1. **Routes.** `POST /otlp/v1/metrics`, `POST /otlp/v1/logs`, `POST /otlp/v1/traces` (traces accepted and discarded).
   - Admins set `OTEL_EXPORTER_OTLP_ENDPOINT=https://<aistats-host>/otlp`.
   - Claude Code's JS exporter appends `/v1/<signal>` to the generic endpoint, with or without a trailing slash: `…/otlp/` produced `/otlp/v1/metrics` and `/otlp/v1/logs` [cap2 v1].
   - Per-signal endpoints (`OTEL_EXPORTER_OTLP_METRICS_ENDPOINT`, `…_LOGS_ENDPOINT`) are used verbatim, with no path appended [cap2 v8; doc].
2. **Protocols.** Accept `http/protobuf` and `http/json` on the same paths, uncompressed or gzip. No gRPC in v1.
   - Claude Code has no default protocol [doc].
   - **[ADDED]** If `OTEL_EXPORTER_OTLP_PROTOCOL` (or the per-signal variant) is unset, Claude Code exports **nothing** and prints nothing to stderr. The debug log shows `[ERROR] [3P telemetry] Telemetry init failed: Unknown protocol set in OTEL_EXPORTER_OTLP_METRICS_PROTOCOL or OTEL_EXPORTER_OTLP_PROTOCOL env var: undefined` [cap2 v11/v12]. Onboarding must make the protocol mandatory.
   - Other tools send JSON regardless of their docs (see Part 3: Gemini CLI `http`, Copilot), so JSON support is required, not optional.
3. **Auth.** Reuse `ingest_api_keys` (`lum_live_` + base64url, SHA-256 via `ingest.HashKey`), as in `backend/internal/handlers/ingest.go`. Both headers arrive verbatim [cap2 v1].
   - **[CORRECTED/ADDED] Recommend `X-AIStats-Key=lum_live_…`** as the primary header (no space, nothing to encode).
     - With `OTEL_EXPORTER_OTLP_HEADERS=Authorization=Bearer%20lum_live_…` (the percent-encoded form that OTel's env-var format suggests), Claude Code sends the literal `Bearer%20lum_live_…`. It does **not** decode it [cap2 v17].
     - The existing `strings.HasPrefix(auth, "Bearer ")` check would answer that with 401.
     - Either accept `Bearer%20` as well, or document the `X-AIStats-Key` form.
   - **[ADDED]** A DB error during key lookup must return **503**, not 401.
     - The existing `PostEvents` maps every `GetIngestKeyByHash` error to 401. The function wraps with `%w`, so use `errors.Is(err, pgx.ErrNoRows)` → 401, and anything else → 503.
     - 401 is non-retryable, so a DB blip would otherwise drop data.
4. **Source of truth for tokens and cost.** The metrics `claude_code.token.usage` and `claude_code.cost.usage`.
   - Default temporality is delta [doc, cap2]. Cumulative is supported with start-time-aware differencing (§2.6).
   - **[ADDED]** Gemini CLI and Copilot default to **cumulative** (Part 3), so the cumulative path is needed in v1, not only as an option.
5. **Events.** Never store content-bearing attributes. In v1, persist only a strict allowlist. Use events to detect content-logging flags.
6. **Go dependencies** (all `go 1.25.0`-compatible; proxy.golang.org re-checked):
   - `go.opentelemetry.io/proto/slim/otlp v1.11.0`: `go 1.25.0`, requires only `google.golang.org/protobuf v1.36.11`.
   - `google.golang.org/protobuf v1.36.11`. v1.36.12 is also `go 1.23` and usable.
   - `google.golang.org/genproto/googleapis/rpc v0.0.0-20260706201446-f0a921348800` (`go 1.25.0`).
   - Avoid `go.opentelemetry.io/proto/otlp v1.11.1`, `slim/otlp v1.11.1`, genproto `v0.0.0-20261019180522-ce908d29299b` and `collector/pdata ≥ v1.66.0`: all declare `go 1.26.0`.

---

## Part 1 — Claude Code telemetry

### 1.1 Configuration env vars

Source: [doc](https://code.claude.com/docs/en/monitoring-usage#common-configuration-variables). All rows were re-verified.

| Var | Values / default | AI Stats recommendation |
|---|---|---|
| `CLAUDE_CODE_ENABLE_TELEMETRY` | `1`, required | `1` |
| `OTEL_METRICS_EXPORTER` | comma list: `otlp`, `prometheus`, `console`, `none` | `otlp` |
| `OTEL_LOGS_EXPORTER` | comma list: `otlp`, `console`, `none` | `none` in v1 |
| `OTEL_EXPORTER_OTLP_PROTOCOL` | `grpc`, `http/json`, `http/protobuf`; **no default** (unset → no export at all, see §0.2) | `http/protobuf` |
| `OTEL_EXPORTER_OTLP_{METRICS,LOGS,TRACES}_PROTOCOL` | per-signal override | unset |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | generic; `/v1/<signal>` appended | `https://<host>/otlp` |
| `OTEL_EXPORTER_OTLP_{METRICS,LOGS,TRACES}_ENDPOINT` | full URL, used verbatim | unset |
| `OTEL_EXPORTER_OTLP_HEADERS` | `k=v` list; per-signal `…_HEADERS` are **merged** with it. Values are not percent-decoded [cap2 v17]. | `X-AIStats-Key=lum_live_…` |
| `OTEL_METRIC_EXPORT_INTERVAL` | ms, default `60000` | default |
| `OTEL_LOGS_EXPORT_INTERVAL` | ms, default `5000` | default |
| `OTEL_EXPORTER_OTLP_METRICS_TEMPORALITY_PREFERENCE` | `delta` (default) or `cumulative` | default |
| `OTEL_LOG_USER_PROMPTS` | default off | off |
| `OTEL_LOG_ASSISTANT_RESPONSES` | default off; falls back to `OTEL_LOG_USER_PROMPTS` when unset | `0` |
| `OTEL_LOG_TOOL_DETAILS` | default off | off |
| `OTEL_LOG_TOOL_CONTENT` | default off; trace spans only | off |
| `OTEL_LOG_RAW_API_BODIES` | default off; `1` or `file:<dir>`; ignored in project/local settings | off |
| `OTEL_LOG_MANAGED_SETTINGS` | default off (v2.1.274+) | off |
| `CLAUDE_CODE_OTEL_CONTENT_MAX_LENGTH` | default `61440` UTF-16 code units | n/a |
| `CLAUDE_CODE_OTEL_HEADERS_HELPER_DEBOUNCE_MS` | default 1740000 ms (29 min) | n/a |

Wire-level notes:

- **Content-Length.** Claude Code sends `Content-Length` on `http/*` requests, never chunked [doc; cap2, including gzip].
- **Compression.** `OTEL_EXPORTER_OTLP_COMPRESSION=gzip` is not in Claude Code's docs, but it is honored: `Content-Encoding: gzip`, and `Content-Length` is the compressed size [cap2 v2]. Default is uncompressed.
- **User-Agent.** `OTel-OTLP-Exporter-JavaScript/0.208.0` [cap2].
- **mTLS.**
  - `http/*`: `CLAUDE_CODE_CLIENT_CERT` / `CLAUDE_CODE_CLIENT_KEY` / `CLAUDE_CODE_CLIENT_KEY_PASSPHRASE`, with the CA in `NODE_EXTRA_CA_CERTS`.
  - `grpc`: `OTEL_EXPORTER_OTLP_CLIENT_KEY` / `OTEL_EXPORTER_OTLP_CLIENT_CERTIFICATE`, with the CA in `OTEL_EXPORTER_OTLP_CERTIFICATE` **[ADDED]** [doc].
- **Dynamic headers.** `otelHeadersHelper` works for `http/*` only and runs at startup, then every 29 min [doc].
  - **[ADDED]** If the helper fails, exports fail until it works again [doc].
  - Use it for token rotation instead of a static key in `env`.
- **Subprocesses.** `OTEL_*` vars are not passed to Claude Code's subprocesses [doc].

### 1.2 Wire shape (re-observed, 2.1.309)

**Metrics.**

- One `ResourceMetrics` per export, with scope `{name:"com.anthropic.claude_code", version:"2.1.309"}`.
- Delta exports contain only the series that changed.
- Cumulative exports re-send **every** series each interval with a fixed per-series `startTimeUnixNano`. Each series has its own start time, equal to its first measurement, not a per-process start [cap2 v7].
- There is a final flush at process exit [cap2].

**Logs.**

- Scope `com.anthropic.claude_code.events`.
- `body={"stringValue":"claude_code.<event>"}`.
- Attribute `event.name="<event>"`.
- `LogRecord.event_name` (field 12) is empty; severity is unset; `timeUnixNano == observedTimeUnixNano` [cap2].
- Derive the event name from `attributes["event.name"]`, falling back to `body`.

**Value types.**

- Metric values are always `asDouble`, `sum` with `isMonotonic:true` [cap2].
- Log attributes mix types:
  - `intValue`: `event.sequence`, `input_tokens`, `cost_usd_micros`, `response_length`.
  - `doubleValue`: `cost_usd`.
  - Strings: `prompt_length`, `duration_ms` on `tool_result`, `success`, `tool_input_size_bytes`, `tool_result_size_bytes`.
  - `boolValue`: `has_hooks`.
  - The doc confirms `retention_sweep` counters are strings.
- OTLP/JSON writes fixed64 timestamps as strings and `intValue` as bare numbers. protojson accepts both [go2].

Example (delta, `http/json`, trimmed; the real payload matches the original spec's example in shape):

```json
{"resourceMetrics":[{"resource":{"attributes":[{"key":"team.id","value":{"stringValue":"platform"}},{"key":"host.arch","value":{"stringValue":"amd64"}},{"key":"os.type","value":{"stringValue":"linux"}},{"key":"os.version","value":{"stringValue":"6.18.44-fc-v153"}},{"key":"service.name","value":{"stringValue":"claude-code"}},{"key":"service.version","value":{"stringValue":"2.1.309"}}]},
 "scopeMetrics":[{"scope":{"name":"com.anthropic.claude_code","version":"2.1.309"},"metrics":[{"name":"claude_code.token.usage","description":"Number of tokens used","unit":"tokens",
  "sum":{"aggregationTemporality":1,"isMonotonic":true,"dataPoints":[{"attributes":[{"key":"team.id","value":{"stringValue":"platform"}},{"key":"user.id","value":{"stringValue":"a670…2509"}},{"key":"session.id","value":{"stringValue":"2c3d5b66-…"}},{"key":"terminal.type","value":{"stringValue":"dumb"}},{"key":"model","value":{"stringValue":"claude-opus-5-5"}},{"key":"query_source","value":{"stringValue":"main"}},{"key":"effort","value":{"stringValue":"medium"}},{"key":"type","value":{"stringValue":"input"}}],
  "startTimeUnixNano":"1792800315293000000","timeUnixNano":"1792800315319000000","asDouble":100}]}}]}]}]}
```

Timestamps have millisecond precision (the last 6 digits are 0) [cap2].

### 1.3 Resource attributes

Resource attributes [doc service-information]:

- `service.name`: `claude-code` for terminal sessions, `claude-code-desktop` for Desktop Code-tab sessions.
- `service.version`: the Claude Code version. **[ADDED]** For Code-tab sessions it is the Desktop app version.
- `os.type`, `os.version`, `host.arch`.
- `wsl.version`, only on WSL.
- Meter name: `com.anthropic.claude_code`.

`OTEL_RESOURCE_ATTRIBUTES`:

- Keys go into the resource and are copied onto every datapoint and event. Built-in keys win collisions, except `vcs.*` [doc].
- `OTEL_METRICS_INCLUDE_RESOURCE_ATTRIBUTES=false` keeps them resource-only.
- Format: comma-separated `k=v`. No spaces, double quotes, commas, semicolons or backslashes; percent-encode anything else. Quotes are kept literally [doc].
- A value with a space makes Claude Code copy **none** of the keys onto datapoints or events [doc].
  - **[ADDED, cap2 v8]** The keys **still appear in the resource block**, with the space intact (`team.id="a b"`).
  - **The receiver must therefore read team/department from datapoint attributes and fall back to resource attributes.** The resource is also the only place they appear when `OTEL_METRICS_INCLUDE_RESOURCE_ATTRIBUTES=false`.

### 1.4 Standard attributes (on datapoints and log records, not the resource)

Source: [doc](https://code.claude.com/docs/en/monitoring-usage#standard-attributes). Re-verified.

| Attribute | Meaning | Gate (default) |
|---|---|---|
| `session.id` | Claude Code session ID (`/clear` replaces it) | `OTEL_METRICS_INCLUDE_SESSION_ID` (true) |
| `ccr.session.id` | cloud session ID (`CLAUDE_CODE_REMOTE_SESSION_ID`) | same |
| `app.version` | Claude Code version | `OTEL_METRICS_INCLUDE_VERSION` (false) |
| `app.entrypoint` | `cli`, `sdk-cli`, `sdk-ts`, `sdk-py`, `claude-vscode`, `claude-in-slack`, …; `sdk-cli` observed for `-p` [cap2 v10] | `OTEL_METRICS_INCLUDE_ENTRYPOINT` (false) |
| `organization.id` | org UUID | when authenticated |
| `user.account_uuid`, `user.account_id` | account UUID; tagged `user_01…` | `OTEL_METRICS_INCLUDE_ACCOUNT_UUID` (true) |
| `user.id` | random install id in `~/.claude.json`; 64 hex chars observed | always |
| `user.email` | sign-in email or cloud-session credentials | when available |
| `terminal.type` | e.g. `iTerm.app`, `vscode`; observed `$TERM` value (`dumb`, `xterm-256color`) | when detected |
| custom keys | from `OTEL_RESOURCE_ATTRIBUTES` | `OTEL_METRICS_INCLUDE_RESOURCE_ATTRIBUTES` (true) |
| `vcs.repository.url.full`, `vcs.owner.name`, `vcs.repository.name`, `vcs.provider.name` | from `origin`, lowercased. SSH `git@github.com:Example-Org/Example-Repo.git` → `https://github.com/example-org/example-repo` [cap2 v10]. | `OTEL_METRICS_INCLUDE_REPOSITORY` (false; v2.1.269+) |

Identity notes, all [doc]:

- **Direct API key, Bedrock, Vertex / Google Cloud Agent Platform, Foundry.** Only `user.id` and `session.id` are present. Identity has to come from `OTEL_RESOURCE_ATTRIBUTES` such as `enduser.id=…`.
- **Claude apps gateway.** `user.id` is the IdP subject; adds `user.groups` and `identity.source: gateway-oidc`. Startup events logged before sign-in carry the anonymous `user.id`.
- **Claude Tag.** Sessions run as the org's shared identity; `app.entrypoint=claude-in-slack`, visible only with `OTEL_METRICS_INCLUDE_ENTRYPOINT=true`.
- **Events-only attributes.** `prompt.id`, `workspace.host_paths`, `workflow.run_id`, `workflow.name`.

### 1.5 Metrics (all 8 re-observed on the wire)

Every metric is an OTLP `sum` with `isMonotonic:true`, `aggregationTemporality` 1 or 2, and `asDouble` values [cap2].

**[CORRECTED]** `commit.count` and `pull_request.count` are now **observed**, no longer inferred:

- A real `git commit` run through the Bash tool produced `claude_code.commit.count` = 1, delta, no metric-specific attributes [cap2 v18].
- A `gh pr create` (fake `gh` printing a PR URL) produced `claude_code.pull_request.count` = 1, with the same shape [cap2 v19].

| Metric | Unit | Metric-specific attributes | Observed |
|---|---|---|---|
| `claude_code.session.count` | `""` | `start_type`: `fresh`, `resume`, `continue`, `agents_view` (filter out `agents_view`) | yes |
| `claude_code.lines_of_code.count` | `""` | `type` (`added`, `removed`), `model` | yes; a 3-line Write with a trailing newline gave added=4, plus an explicit removed=0 |
| `claude_code.pull_request.count` | `""` | none | yes |
| `claude_code.commit.count` | `""` | none | yes |
| `claude_code.cost.usage` | `USD` | `model`, `query_source` (`main`, `subagent`, `auxiliary`), `speed` (`fast`, else absent), `effort` (`low`…`max`, absent if the model has no effort), `agent.name`, `skill.name`, `plugin.name`, `marketplace.name`, `mcp_server.name`, `mcp_tool.name` (redacted unless `OTEL_LOG_TOOL_DETAILS=1`, v2.1.273+) | yes |
| `claude_code.token.usage` | `tokens` | `type` (`input`, `output`, `cacheRead`, `cacheCreation`), plus the same attributes as cost | yes |
| `claude_code.code_edit_tool.decision` | `""` | `tool_name`, `decision`, `source`, `language` (observed `Plain text`) | yes |
| `claude_code.active_time.total` | `s` | `type`: `user` or `cli` | yes |

Semantics, all re-verified [doc]:

- **Token types.** `input` excludes cache reads and writes. `cacheCreation` = 5-minute + 1-hour cache writes. GenAI semconv `input` = `input + cacheRead + cacheCreation`.
- **Exactly once.** Each streaming response is counted exactly once.
- **Cost is an estimate**, not billing data.
- **`model`** is present only on `token.usage`, `cost.usage` and `lines_of_code.count`.
- **`query_source` differs by signal.** Metrics use categories; events use subsystem names. Observed: metrics `main`, events `sdk` in a `-p` run [cap2].
- **Ground truth re-confirmed [go2].**
  - The author's run1 (delta JSON) and run3 (delta gzip protobuf) each sum to 240/84/6000/1000 tokens and $0.00753.
  - Run2 (cumulative), taking the last value per series (name + attributes + start), gives the same totals.
  - These match Claude Code's own `--output-format json` result.

### 1.6 Events (OTLP log records) and content flags

Events are delivered only when `OTEL_LOGS_EXPORTER` is set. Every event carries the standard attributes plus `event.name`, `event.timestamp` and `event.sequence`; most also carry `prompt.id` [doc, cap2].

Correlation keys [doc]:

- `request_id`: from the `request-id` header, or `x-amzn-requestid` on Bedrock (v2.1.282+). Present on `api_request`, `api_error`, `api_refusal`, `assistant_response` and `api_response_body` when either header exists.
- `client_request_id`: only on `api_request` / `api_error` on first-party API connections. **[ADDED]** Not present in any capture made through a custom `ANTHROPIC_BASE_URL` [cap2].
- `message.uuid`, `tool_use_id`.

Content-bearing attributes. **AI Stats must never persist these.**

| Event (`claude_code.*`) | Safe metadata | Content / identifying (gate) |
|---|---|---|
| `user_prompt` | `prompt_length` (string), `message.uuid`, `command_source` | `prompt` **and** `prompt_text` (v2.1.287+): `<REDACTED>` unless `OTEL_LOG_USER_PROMPTS=1`, in which case both carry the full prompt [cap2 v10]. `command_name` is verbatim for custom/plugin/MCP commands only with `OTEL_LOG_TOOL_DETAILS=1`. |
| `assistant_response` | `response_length`, `model`, `request_id`, `query_source`, `message.uuid` | `response`: `OTEL_LOG_ASSISTANT_RESPONSES`, falling back to `OTEL_LOG_USER_PROMPTS`. With only `OTEL_LOG_USER_PROMPTS=1`, the text arrived [cap2 v10]. |
| `tool_result` | `tool_name`, `success`, `duration_ms`, `error_type`, `tool_input_size_bytes`, `tool_result_size_bytes`, `mcp_server_scope`, `tool_use_id`; `decision_type` / `decision_source` are documented but **were not present** in the 2.1.309 `-p` capture | `tool_parameters`, `tool_input`, `error`, `vcs.ref.head.*`, all with `OTEL_LOG_TOOL_DETAILS=1`. Observed: `tool_input` = `{"file_path":"…/notes.txt","content":"alpha\nbeta\ngamma\n"}`, i.e. **file content** [cap2 v10]. |
| `api_request` | `model`, `cost_usd`, `cost_usd_micros` (int), `duration_ms`, `input_tokens`, `output_tokens`, `cache_read_tokens`, `cache_creation_tokens`, `request_id`, `client_request_id` (first-party only), `speed` (`fast`/`normal`), `query_source`, `effort`; undocumented but observed: `cache_creation_5m_tokens`, `cache_creation_1h_tokens`, `ttft_ms` | attribution names are real only with `OTEL_LOG_TOOL_DETAILS=1` |
| `api_error` / `api_retries_exhausted` | `model`, `status_code`, `duration_ms`, `attempt` / `total_attempts`, `total_retry_duration_ms`, `request_id`, `speed` | `error` (free text; drop it) |
| `api_refusal` | `has_category`, `has_explanation`, `server_fallback_hop`, `attempt` | `category` (`OTEL_LOG_TOOL_DETAILS`) |
| `api_request_body` / `api_response_body` | | `body` (≤ content limit) or `body_ref`. **Drop the whole event.** |
| `tool_decision` | `tool_name` (user MCP tools → `mcp_tool`), `decision`, `source`, `tool_source`, `classifier_verdict`, `permission_mode`, `tool_use_id` | `tool_parameters`, `classifier_category` (`OTEL_LOG_TOOL_DETAILS`) |
| `safety_event` | `category`, `decision`, `decision_source`, `reason_code`, `reason_type`, `rule_source` (v2.1.307+) | standard attributes can include `workspace.host_paths` |
| `mcp_server_connection` | `status`, `transport_type`, `server_scope`, `duration_ms`, `error_code`, `is_plugin`, `plugin_id_hash` | `server_name`, `error` (`OTEL_LOG_TOOL_DETAILS`) |
| `hook_*` | counts and durations | `hook_matcher` (flag); `hook_definitions` (flag plus detailed beta tracing) |
| `feedback_survey` | `event_type`, `survey_type`, `appearance_id` | `response` |
| `managed_settings_resolved` | sources, helper state | `managed_settings.settings` and `resolved_sha256` (`OTEL_LOG_MANAGED_SETTINGS`). **[CORRECTED]** `managed_settings.helper.path` is present whenever a policy helper is selected, regardless of the flag; it is not "always present". |
| **[ADDED]** `compaction` | `trigger`, `success`, `duration_ms`, `pre_tokens`, `post_tokens` | `error` (free-text message; drop it) |

Other events, metadata only (third-party names redacted unless `OTEL_LOG_TOOL_DETAILS=1`):

- `permission_mode_changed`, `auth`, `internal_error`
- `plugin_installed`, `plugin_loaded`, `skill_activated`, `at_mention`
- `hook_plugin_metrics`, `subagent_completed`, `retention_sweep`

[doc] **Doc tension, not a strict contradiction:**

- Security & privacy says: "Raw file contents and code snippets are not included in metrics or events."
- The same page documents `tool_input` as "JSON-serialized tool arguments… Individual values over 512 characters are truncated… Applies to all tools."
- So with `OTEL_LOG_TOOL_DETAILS=1`, up to 512 characters of each Write/Edit argument (that is, code) do appear [cap2 v10].
- Worth reporting upstream as a docs clarification.

Traces (beta) need `CLAUDE_CODE_ENHANCED_TELEMETRY_BETA=1` plus `OTEL_TRACES_EXPORTER`. Spans can carry content (`user_prompt`, `file_path`, `full_command`, `tool.output`; under detailed beta tracing also `new_context`, `system_prompt_preview`, `response.model_output` and a `claude_code.system_prompt` event) [doc]. AI Stats does not ingest traces.

**[CORRECTED nuance]** Log records carry `traceId`/`spanId`:

- in `-p`/SDK sessions with an inbound `TRACEPARENT` [doc];
- and, per the doc, records "emitted while an interaction is active carry the interaction span's IDs". UNVERIFIED whether that also happens with tracing on and no inbound `TRACEPARENT`.

Irrelevant as long as the receiver never reads these fields (§2.3 C6).

### 1.7 Receiver privacy rules (normative)

1. **Metrics.**
   - Keep only the 8 `claude_code.*` metrics; accept and ignore other names. Ignored points are **not** counted as rejected.
   - Allowlisted dimensions: `model`, `type`, `query_source`, `decision`, `source`, `tool_name`, `language`, `start_type`, `speed`, `effort`, plus team.
   - **[CORRECTED] Team:** read `team.id` → `department` from datapoint attributes, **falling back to resource attributes** (§1.3).
   - **User identity:** precedence `user.email` → `user.account_uuid` → `enduser.id` → `user.id`, from datapoint attributes, then resource. Hash with `ingest.HashKey`.
     - **[CORRECTED]** The existing push-ingest does **not** lowercase before hashing: `ingest.HashKey(ev.UserEmail)` in `backend/internal/handlers/ingest.go`.
     - Either hash the raw value, to stay comparable with push-ingest `distinctUsers`, or lowercase in **both** paths together.
     - Store the identity *kind* too; the original migration omitted the `user_key_kind` column that its `Point` struct carries. The same person shows up under different keys (email vs. install id) across auth methods.
   - Never store `session.id`, `terminal.type`, `vcs.*` (unless a repo feature is designed) or the attribution name attributes.
2. **Logs, if enabled.**
   - Use a per-event allowlist, never a denylist. Drop `api_request_body`, `api_response_body` and any `system_prompt` event entirely. Drop all `error` attributes.
   - Never log raw payloads.
   - Flag content logging when any of these is seen:
     - `prompt` / `prompt_text` present and not `<REDACTED>`;
     - `response` not `<REDACTED>`;
     - any `tool_parameters`, `tool_input`, `body` or `body_ref`.
   - Warn via an org setting/UI banner, and via `partial_success{rejected_log_records:0, error_message:"…"}`, which the OTLP spec allows [doc OTLP #partial-success-1].
3. **Traces.** `200` + `ExportTraceServiceResponse.partial_success{rejected_spans:N, error_message:"AI Stats does not store traces"}`. The client MUST NOT retry. JSON form: `{"partialSuccess":{"rejectedSpans":"2","errorMessage":"…"}}` [go2].

### 1.8 Org-wide rollout

**Delivery** [doc managed-settings]:

- **File:** `/Library/Application Support/ClaudeCode/managed-settings.json` (macOS), `/etc/claude-code/managed-settings.json` (Linux/WSL), `C:\Program Files\ClaudeCode\managed-settings.json` (Windows). Plus an optional `managed-settings.d/*.json`, merged alphabetically. Reloaded when the file changes.
- **MDM:** macOS profile domain `com.anthropic.claudecode`; Windows `HKLM\SOFTWARE\Policies\ClaudeCode` value `Settings` (REG_SZ / REG_EXPAND_SZ). Checked every 30 min.
- **Server-managed:** the claude.ai admin console, fetched at startup and polled hourly.
  - **[ADDED]** A delivered `OTEL_EXPORTER_OTLP_ENDPOINT` **always** requires the user to accept the security approval dialog in interactive sessions [doc server-managed-settings].
  - Non-interactive runs (`-p`, SDK, VS Code panel, Desktop Code tab) apply it for that run only.
  - So a server-managed rollout is not silent.

**Locking (v2.1.217+)** [doc]:

- Managed `OTEL_EXPORTER_OTLP_ENDPOINT` removes every developer-set per-signal endpoint.
- Managed `OTEL_EXPORTER_OTLP_PROTOCOL` removes developer-set per-signal protocols.
- Managed `OTEL_EXPORTER_OTLP_HEADERS`, `…_CLIENT_KEY` or `…_CLIENT_CERTIFICATE` removes the developer-set per-signal versions **and every developer-set endpoint variable**, generic or per-signal.
- Exporter selectors follow per-key precedence; set them in managed settings too.
- **[ADDED]** Since v2.1.251, a launcher (Desktop / self-hosted runner) that names an OTLP endpoint pins the destination the same way.

**[ADDED, resolves an open question]** Managed `env` values **do override a developer's shell exports** in terminal CLI sessions:

- settings-reference: "A value here overwrites the same variable exported in your shell, and when more than one settings file sets a variable, the highest-precedence one applies."
- Exception: when the Desktop app or a self-hosted runner launches the session, the launch environment wins over every settings file.
- So `OTEL_LOG_USER_PROMPTS=0` in managed settings is effective in the CLI.

**Repo settings.** A repository's `.claude/settings.json` / `.claude/settings.local.json` cannot enable telemetry, redirect it, or turn on content flags.

- **[ADDED]** A repository **can** turn a signal off by setting its exporter selector to `none`, unless managed settings, `--settings` or the launch environment set it [doc].

**Per-team tagging.** Deploy per-group managed settings carrying `OTEL_RESOURCE_ATTRIBUTES=team.id=<team>`.

- **[CORRECTED]** This is an AI Stats recommendation, **not** something the managed-settings doc describes. Its "Split a file-based policy across teams" section is about teams co-owning policy fragments.
- Caveat: drop-ins merge per key, so the later file's `OTEL_RESOURCE_ATTRIBUTES` replaces the earlier one whole.

**Other surfaces.**

- **Cloud sessions:** server-managed settings, or environment variables (never credentials in environment variables). Claude Tag sessions don't receive server-managed settings.
- **Cowork** is configured separately [doc].

**Verify a rollout:** look for `claude_code.session.count`; `claude --debug-file <path>` and search for `[3P telemetry]` [doc].

Recommended `managed-settings.json`:

```json
{"env":{
  "CLAUDE_CODE_ENABLE_TELEMETRY":"1",
  "OTEL_METRICS_EXPORTER":"otlp",
  "OTEL_LOGS_EXPORTER":"none",
  "OTEL_EXPORTER_OTLP_PROTOCOL":"http/protobuf",
  "OTEL_EXPORTER_OTLP_ENDPOINT":"https://aistats.example.internal/otlp",
  "OTEL_EXPORTER_OTLP_HEADERS":"X-AIStats-Key=lum_live_XXXX",
  "OTEL_RESOURCE_ATTRIBUTES":"team.id=platform",
  "OTEL_LOG_USER_PROMPTS":"0","OTEL_LOG_ASSISTANT_RESPONSES":"0","OTEL_LOG_TOOL_DETAILS":"0"
}}
```

The receiver must still enforce §1.7.

---

## Part 2 — OTLP/HTTP receiver essentials

### 2.1 Request contract

Source: [OTLP spec](https://github.com/open-telemetry/opentelemetry-proto/blob/main/docs/specification.md). Re-verified.

- **Transport.** HTTP POST over HTTP/1.1 or HTTP/2.
- **Default paths:** `/v1/metrics`, `/v1/logs`, `/v1/traces`. Non-default paths MAY be configured.
- **Content types.** `application/x-protobuf` and `application/json`, on the same port, dispatched by `Content-Type`. Parse it with `mime.ParseMediaType`, so parameters such as `; charset=utf-8` are tolerated.
- **Compression.** Servers MUST support `none` and `gzip` (`Content-Encoding: gzip`).
- **Size limit.** The server MUST limit body size including after decompression. 64 MiB is recommended and SHOULD be configurable. Exceeding it MUST return `413 Content Too Large`.
  - AI Stats default: 16 MiB. Observed Claude Code bodies were ≤ ~7 KB uncompressed.
- **Empty requests** SHOULD get a success response.
  - **[ADDED]** A zero-length body is valid protobuf (it decodes to an empty message), but `protojson` rejects zero bytes with `syntax error … unexpected token` [go2].
  - Treat `len(body)==0` as an empty request for both formats.
- **Invalid UTF-8.** Decoders SHOULD replace it with U+FFFD.
  - **[ADDED]** `google.golang.org/protobuf` cannot do that. `proto.Unmarshal` fails with `string field contains invalid UTF-8`, and `protojson` with `invalid UTF-8 in string` [go2].
  - The batch then has to be 400, which is non-retryable.
  - Acceptable, since this is a SHOULD and metric attribute values are ASCII, but document it. The realistic trigger is content truncated mid-surrogate in log events, which AI Stats doesn't keep anyway.
- **Default port** is 4318. AI Stats uses its existing port behind TLS.

### 2.2 Response contract

- **Response Content-Type** MUST equal the request's.
- **4xx/5xx bodies** MUST be a `google.rpc.Status` message.
  - **[CORRECTED wording]** Encode it in the request's encoding: binary for `application/x-protobuf`, protojson for `application/json`. The reference collector does exactly this.
  - `Status.code` is optional and informational; `message` SHOULD be developer-facing.
  - **[ADDED]** For an unsupported Content-Type there is no request encoding to mirror. The reference collector answers `415` with `text/plain` and the body `415 unsupported media type, supported: [application/json, application/x-protobuf]`. Do the same.
- **Partial success messages** (re-verified from the protos):
  - `ExportMetricsPartialSuccess{int64 rejected_data_points=1; string error_message=2}`
  - `ExportLogsPartialSuccess{int64 rejected_log_records=1; string error_message=2}`
  - `ExportTracePartialSuccess{int64 rejected_spans=1; string error_message=2}`

| Case | HTTP | Body | Client behaviour (spec + observed) |
|---|---|---|---|
| Accepted | 200 | Empty `Export*ServiceResponse`: protobuf **0 bytes**, JSON **`{}`** [go2]; `partial_success` MUST be unset | done |
| Some points invalid | 200 | `{"partialSuccess":{"rejectedDataPoints":"3","errorMessage":"…"}}` (int64 as string) [go2] | MUST NOT retry; observed no retry [cap2 v9] |
| Undecodable | 400 | Status, SHOULD include `BadRequest` details | MUST NOT retry; observed no retry, data dropped [cap2 v5] |
| Missing or invalid key | 401 | Status | not retried |
| **DB error during key lookup** **[ADDED]** | **503** | Status | retried |
| Too large | 413 | Status | not retried |
| Bad Content-Type or Content-Encoding | 415 | `text/plain` (Content-Type) or Status (Content-Encoding) | not retried |
| Overloaded, DB down or slow | **503** (or 429), with an **optional, short** `Retry-After` | Status | retried with identical bytes (below) |
| **Any 500** | 500 | Status | **observed: not retried, data lost** [cap2 v4]. Never use 500 for transient errors. |

Retry rules:

- **Retryable codes** are exactly 429, 502, 503 and 504; all other 4xx/5xx MUST NOT be retried.
- **Retries resend identical bytes** (same md5): after 503 + `Retry-After: 1`, and after 429 without `Retry-After` (≈1 s backoff) [cap2 v3, v6].
- **[ADDED] Retried payloads can arrive after later exports.** In v6, a retried interval landed after the next interval's export. Delta storage must not assume ordering.
- **[CORRECTED, critical] Retry-After must be short.** Claude Code's OTel JS exporter retries only within the export deadline, `OTEL_EXPORTER_OTLP_TIMEOUT`, default **10 000 ms**.
  - It makes at most 5 retries, with backoff 1 s × 1.5^n capped at 5 s, ±20 % jitter.
  - If `Retry-After` exceeds the remaining deadline it gives up immediately: "Export retry time … exceeds remaining timeout …, not retrying further." [src opentelemetry-js `retrying-transport.ts`, `shared-configuration.ts`].
  - Verified with Claude Code 2.1.309:
    - `Retry-After: 8` → retried 8 s later [cap2 v15];
    - `Retry-After: 12` → **never retried, point lost** [cap2 v16];
    - `Retry-After: 30` → never retried [cap2 v13].
  - **The original sketch's `Retry-After: 30` would therefore guarantee data loss.** Send `Retry-After: 1`–`3`, or omit it.
- **[ADDED] Keep handler latency well under 10 s.** Use a DB context timeout of about 3–5 s, then answer 503.
- **[ADDED] The exit flush has a short window.** When the final export (the only export for short `-p` runs) got 503 with `Retry-After: 1`, Claude Code made 3 attempts about 1 s apart and then exited before a successful 4th [cap2 v14]. The exact shutdown window is UNVERIFIED (about 2–3 s observed). Receiver availability at session end matters.
- **Inserts must be idempotent.** The spec also accepts duplicates on reconnects ("If the server disconnects without returning a response, the client SHOULD retry and send the same request").

### 2.3 OTLP/JSON rules and protojson behaviour

Spec rules, re-verified:

- lowerCamelCase keys.
- Enums MUST be integers.
- 64-bit integers, including fixed64 `*UnixNano`, are decimal strings; decoders accept numbers or strings.
- `traceId`/`spanId` are hex.
- Receivers MUST ignore unknown fields.

`protojson` v1.36.11, re-tested [go2]:

- **C1:** default `Unmarshal` rejects unknown fields (`unknown field "futureField"`). **C2:** use `UnmarshalOptions{DiscardUnknown:true}`.
- **C3:** numbers and strings both decode for int64/fixed64.
- **C4/C5:** enum names and snake_case keys are also accepted (lenient, harmless).
- **C6:** hex `traceId`/`spanId` are silently base64-decoded into 24/12 bytes. `base64.StdEncoding.EncodeToString` recovers the hex text.
- **C8b:** length%4==1 errors (`invalid value for bytes field traceId`).
- **[ADDED] U1:** `"timeUnixNano":"18446744073709551615"` decodes without error and casts to `-1` as int64. Reject any timestamp `> math.MaxInt64` before writing to `BIGINT`. The author's code does this, but the spec text didn't say so.
- **[ADDED] N1:** `"asDouble":"NaN"` and `"Infinity"` are accepted by protojson. Reject NaN/Inf, as the spec already says.
- **[ADDED]** A `NumberDataPoint` with no `value` oneof set is invalid per `metrics.proto`. Count it as rejected.

### 2.4 Go packages (proxy.golang.org, re-checked 2026-10-24)

| Module | Version | `go` / deps | Verdict |
|---|---|---|---|
| `go.opentelemetry.io/proto/slim/otlp` | **v1.11.0** (2026-07-22) | `go 1.25.0`; only `google.golang.org/protobuf v1.36.11`. Packages: `collector/{metrics,logs,trace}/v1`, `metrics/v1`, `logs/v1`, `common/v1`, `resource/v1`, **`trace/v1`** [ADDED]. No gRPC. | **Use** |
| `go.opentelemetry.io/proto/slim/otlp` | v1.11.1 (2026-09-30, @latest) | `go 1.26.0`, protobuf v1.36.12 | not until Go 1.26 |
| `go.opentelemetry.io/proto/otlp` | v1.11.1 (@latest) | v1.11.1: `go 1.26.0` + grpc v1.84.0 + grpc-gateway v2.31.0. v1.11.0: `go 1.25.0` + grpc v1.82.1. | avoid |
| `google.golang.org/protobuf` | v1.36.11; latest v1.36.12 (both `go 1.23`) | | **Use** |
| `google.golang.org/genproto/googleapis/rpc` | v0.0.0-20260706201446-f0a921348800 (`go 1.25.0`, protobuf v1.36.11). Latest v0.0.0-20261019180522-ce908d29299b is `go 1.26.0`. | | **Use** for `status.Status` |
| `go.opentelemetry.io/collector/pdata` | latest v1.69.0; v1.66.0+ are `go 1.26.0`; v1.65.0 is `go 1.25.0` (json-iterator, featuregate, grpc v1.83.0, protobuf v1.36.12) | | avoid |

The `go get` of the three "Use" modules works with go1.25.0 [go2].

### 2.5 Implementation sketch

The author's package at `<verification scratch: otel-gotest/otlp/>` passes `go test`; re-run here. Corrections are marked.

```go
const DefaultMaxBodyBytes = 16 << 20
var jsonUnmarshal = protojson.UnmarshalOptions{DiscardUnknown: true}
// ReadBody: mime.ParseMediaType(Content-Type) → x-protobuf | json (else 415 text/plain);
// Content-Encoding "" | identity | gzip (else 415; corrupt gzip → 400);
// http.MaxBytesReader on the wire + io.LimitReader(max+1) after decompression → 413.
// [ADDED] len(body)==0 → treat as empty request (200), for JSON too.
func ReadBody(w http.ResponseWriter, r *http.Request, max int64) ([]byte, Format, error)
func Unmarshal(b []byte, f Format, m proto.Message) error
func WriteMessage(w http.ResponseWriter, f Format, code int, m proto.Message)
func WriteStatus(w http.ResponseWriter, f Format, httpCode int, grpcCode int32, msg string)

type Point struct { Metric string; Cumulative bool; SeriesID string; StartNano, TimeNano int64; Day time.Time;
  Value float64; UserKey, UserKeyKind, Team, Model, Type, QuerySource, ServiceName string }
// ExtractClaudeCodePoints: keep the 8 claude_code.* Sums; asDouble|asInt; skip NO_RECORDED_VALUE and 0;
// reject NaN/Inf/negative/no-value/time==0/time>MaxInt64/non-Sum/UNSPECIFIED temporality (→ partial_success).
// Team/user: datapoint attrs first, then resource attrs [CORRECTED: must be explicit].
// SeriesID = sha256(canon(resource attrs) | scope.name | metric | canon(dp attrs) [| start_time if cumulative]).
```

Handler (`backend/internal/handlers/otlp.go`):

```go
func (h *OTLPHandler) Metrics(w http.ResponseWriter, r *http.Request) {
  // 1. format from Content-Type (415 text/plain if unsupported)
  // 2. auth: X-AIStats-Key, or Authorization "Bearer " [ADDED: also accept "Bearer%20"]
  //    GetIngestKeyByHash: errors.Is(err, pgx.ErrNoRows) → 401 Status(16); other err → 503 Status(14) [CORRECTED]
  // 3. ReadBody → 413/415/400; 4. Unmarshal → 400 Status(3)
  // 5. ExtractClaudeCodePoints; hash UserKey with ingest.HashKey (see §1.7 lowercase note)
  // 6. ctx with ~3-5 s timeout; InsertOTelPoints error/timeout → 503 Status(14) + "Retry-After: 1" [CORRECTED from 30]; never 500
  // 7. 200 + ExportMetricsServiceResponse{PartialSuccess: rejected>0 ? {...} : nil}
}
```

Further notes:

- **[ADDED]** Throttle `UpdateIngestKeyLastUsed`, for example only when `last_used_at` is older than 5 min. OTLP clients post every 5–60 s per developer and signal.
- Routes go next to `/api/ingest/events`, outside the JWT group. The global CORS middleware is irrelevant (no `Origin`).
- `ReadTimeout` 15 s and `WriteTimeout` 60 s are fine, but the effective budget is the client's 10 s export timeout.

### 2.6 Aggregating daily per-user-per-model tokens and cost without double counting

Background: `metrics.proto` (re-verified field numbers: `NumberDataPoint{attributes=7, start_time_unix_nano=2, time_unix_nano=3, as_double=4, as_int=6, exemplars=5, flags=8}`).

- DELTA = non-overlapping intervals.
- CUMULATIVE = total since a fixed start; a reset MUST use a new start time.
- `start_time_unix_nano` is "optional but strongly encouraged".

**Delta.**

- Sum the values.
- Primary key `(organization_id, series_id, start_time_unix_nano, time_unix_nano)` with `ON CONFLICT DO NOTHING`. Retries are byte-identical [cap2], so they dedup.
- The day bucket is the UTC date of `time_unix_nano`.
- **[ADDED] Edge case.** With `OTEL_METRICS_INCLUDE_SESSION_ID=false`, two concurrent processes of the same user on the same host have identical series IDs. Timestamps are millisecond precision, so equal start and end ms would collide and under-count. This is very unlikely. Optional mitigation: add `value` to the key.

**Cumulative.**

- The series includes the start time.
- Upsert the latest point per `(series, day)`:

  ```sql
  INSERT … ON CONFLICT (organization_id, series_id, day) DO UPDATE
  SET last_time_unix_nano = EXCLUDED.last_time_unix_nano, last_value = EXCLUDED.last_value
  WHERE EXCLUDED.last_time_unix_nano > otel_cumulative_days.last_time_unix_nano;
  ```

- Daily increment = `last(day) − last(previous day)`.
- Verified on Postgres 16 [sql2]: day 1 = 200 (delta 100 + cumulative 100, with the duplicate delta deduplicated); day 2 = 60 (150 − 100 + 10 for a restarted series; an out-of-order older point was ignored); cost 0.25.
- **[ADDED] Edge cases:**
  - If retention pruning removes a still-running series' previous-day row, `LAG` falls back to 0 and over-counts. Prune only rows older than N days that are not the latest row of their series, or roll up before pruning.
  - A cumulative point with `start_time_unix_nano == 0` cannot reveal restarts. Rely on the "decrease means reset" fallback.

Migration `backend/internal/db/migrations/009_otel_metrics.sql`. 009 is the next free number, since 008 is the latest in the repo; re-check before merging if other work also adds migrations.

```sql
CREATE TABLE otel_delta_points (
    organization_id      TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    series_id            TEXT NOT NULL,
    start_time_unix_nano BIGINT NOT NULL,
    time_unix_nano       BIGINT NOT NULL,
    day                  DATE NOT NULL,
    tool_source          TEXT NOT NULL,
    metric_name          TEXT NOT NULL,
    user_key_hash        TEXT NOT NULL DEFAULT '',
    user_key_kind        TEXT NOT NULL DEFAULT '',   -- [ADDED] user.email | user.account_uuid | enduser.id | user.id
    team                 TEXT NOT NULL DEFAULT '',
    service_name         TEXT NOT NULL DEFAULT '',   -- [ADDED] claude-code vs claude-code-desktop
    model                TEXT NOT NULL DEFAULT '',
    attr_type            TEXT NOT NULL DEFAULT '',
    query_source         TEXT NOT NULL DEFAULT '',
    value                DOUBLE PRECISION NOT NULL,
    received_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (organization_id, series_id, start_time_unix_nano, time_unix_nano)
);
CREATE INDEX otel_delta_points_org_day_idx ON otel_delta_points (organization_id, day);
-- otel_cumulative_days: as in the original (PK organization_id, series_id, day; last_time_unix_nano, last_value),
-- plus the same two [ADDED] columns.
```

The rollup query is as in the original. Re-run on Postgres 16, every aggregate is COALESCEd, and the results matched [sql2].

**Optional exact path.** `api_request` events carry `request_id` and integer `cost_usd_micros`.

- They require `OTEL_LOGS_EXPORTER=otlp`.
- `request_id` can be absent (no `request-id` / `x-amzn-requestid` header).
- **Never sum both sources.**

### 2.7 Tests to add

1. Fixtures from run1/run2/run3 (author's) and v1/v2/v7 (mine) all aggregate to the client-reported totals.
2. The protojson unknown-field case.
3. A gzip bomb returns 413.
4. 415 (with a `text/plain` body) for an unknown Content-Type; 415 for an unknown Content-Encoding.
5. A 400 carries a decodable Status in the request's encoding.
6. A retried identical delta request is counted once.
7. Cumulative across midnight, restart and out-of-order points.
8. A DB error in insert **and in key lookup** → 503 with `Retry-After` ≤ 3, never 500 or 401.
9. Response Content-Type mirrors the request.
10. `user_prompt` with real text is never persisted and raises the warning.
11. **[ADDED]** A zero-length JSON body → 200.
12. **[ADDED]** `timeUnixNano` above MaxInt64 → rejected.
13. **[ADDED]** Team is read from the resource when datapoints lack it (space-in-value case).
14. **[ADDED]** `Authorization: Bearer%20lum_live_…` is accepted, or explicitly 401 with a helpful message.
15. **[ADDED]** Cumulative JSON from Gemini- or Copilot-style producers (histogram `sum` for Copilot).

---

## Part 3 — Other coding agents (lower priority)

### OpenAI Codex CLI

**Token/cost metrics** [src]:

- **`codex.turn.token_usage`**, an f64 histogram (`meter.f64_histogram`, values recorded from i64).
  - One sample per model per turn.
  - `token_type` ∈ `total`, `input`, `cached_input`, `cache_write_input`, `output`, `reasoning_output`, plus one extra caller-supplied tag pair.
  - Use `HistogramDataPoint.sum`, which is `optional` in the proto; treat a missing sum as rejected.
  - **[ADDED, src protocol.rs]** `input` **includes** `cached_input`: `non_cached_input() = input_tokens − cached_input()`.
  - UNVERIFIED: whether `input` also includes `cache_write_input`, and whether `output` includes `reasoning_output` (OpenAI API semantics suggest it does).
- **`codex.turn.cost_microusd`**, a **u64 counter**, so it arrives as an OTLP Sum with `asInt` [src client.rs].
  - Tags: `turn.id`, `conversation.id`, `turn.interrupted`, optional `speed`, `reasoning_effort`.
  - **[ADDED]** `turn.id` makes a new series every turn (high cardinality; in cumulative mode every past turn series is re-sent).
- **[ADDED]** Log event `codex.turn_cost` with `usage.estimated_usd`.
- `codex.sse_event` (`event.kind=response.completed`) carries `input_token_count`, `output_token_count`, `cached_token_count`, `cache_write_token_count`, `reasoning_token_count`, `tool_token_count`.
  - **[ADDED] Quirk:** `tool_token_count` is set from `usage.total_tokens`.

**Identity and privacy:**

- Metric tags are `app.version`, `auth_mode`, `model`, `originator`, `service_name` and `session_source`, with no user identity.
- Log events (log target) carry `user.account_id`, `user.email`, `conversation.id`, `terminal.type` [src shared.rs].
- **[CORRECTED]** The prompt flag is the config key `otel.log_user_prompt` (singular). The redaction placeholder is `[REDACTED]`, not `<REDACTED>`.
- `codex.agent_response` is emitted only with `otel.log_agent_responses=true` plus an OTLP log exporter.

**Config [CORRECTED: now first-party, from `codex-rs/config/src/types.rs` and the README]:**

- `[otel]` with `metrics_exporter = { otlp-http = { endpoint = "…", protocol = "binary"|"json", headers = {…} } }`. The log exporter is the `exporter` key; `trace_exporter` exists too.
- The default `metrics_exporter` is `statsig` (OpenAI's analytics, following `analytics.enabled`); the default log exporter is `none`.
- **The endpoint is used as-is.** Codex calls `.with_endpoint(endpoint)`, and opentelemetry-rust's `resolve_http_endpoint` uses a programmatic endpoint verbatim. The README's example is `endpoint = "https://collector.example.com/v1/logs"`. So configure `https://<host>/otlp/v1/metrics`.
- Metrics default to delta; `OTEL_EXPORTER_OTLP_METRICS_TEMPORALITY_PREFERENCE` accepts `delta`, `cumulative` or `lowmemory` (case-insensitive).

### Gemini CLI

**Token/cost metrics** [doc + src]:

- **`gemini_cli.token.usage`**: attributes `model` and `type` ∈ `input`, `output`, `thought`, `cache`, `tool`.
  - Mapping: `input` = `promptTokenCount`, `cache` = `cachedContentTokenCount`, `thought` = `thoughtsTokenCount`, `tool` = `toolUsePromptTokenCount`.
  - UNVERIFIED (ai.google.dev blocked): whether `input` includes `cache`. Gemini API semantics suggest it does.
- Also `gen_ai.client.token.usage` (input/output only) and `gen_ai.client.operation.duration`, `gemini_cli.lines.changed` (`type`, `function_name`), `gemini_cli.session.count`, `gemini_cli.api.request.count`.
- Log `gemini_cli.api_response` has the six `*_token_count` fields. There is no cost metric.

**Identity and privacy:**

- Common attributes: `session.id`, `installation.id`, `active_approval_mode`, `user.email` (when authenticated).
- **`logPrompts` defaults to `true`**: strip `prompt` and `response_text`.

**Config:**

- `GEMINI_TELEMETRY_ENABLED`, `…_TARGET` (default `local`), `…_OTLP_ENDPOINT` (default `http://localhost:4317`), `…_OTLP_PROTOCOL` `grpc`|`http` (default `grpc`), `…_OTLP_HEADERS`, `…_LOG_PROMPTS`.
- **[ADDED, src sdk.ts]** `http` uses `@opentelemetry/exporter-*-otlp-http`, which means **OTLP/JSON**. It appends `/v1/traces|logs|metrics` to the endpoint path. The metric export interval is 10 s.
- **No temporality is set, so the OTel JS default applies: CUMULATIVE**, unless `OTEL_EXPORTER_OTLP_METRICS_TEMPORALITY_PREFERENCE=delta` [src opentelemetry-js `OTLPMetricExporterBase.ts`].
- gzip is used only on the gRPC path.

### GitHub Copilot (VS Code)

**Token/cost metrics:**

- **`gen_ai.client.token.usage`**: a histogram with `gen_ai.token.type` ∈ `input`, `output` only (no cache split), plus `gen_ai.operation.name`, `gen_ai.provider.name`, `gen_ai.request.model`, optional `gen_ai.response.model` and `server.address` [src genAiMetrics.ts].
- Also `gen_ai.client.operation.duration` and extension counters: `copilot_chat.session.count`, `…lines_of_code.count`, `…pull_request.count`, `…edit.acceptance.count`, `…tool.call.count`, and others.
- Events: `gen_ai.client.inference.operation.details`, `copilot_chat.agent.turn`. No cost metric [doc].

**Identity and privacy:**

- Resource: `service.name=copilot-chat`, `service.version`, `session.id`.
- Identity only with `captureIdentity` / `COPILOT_OTEL_CAPTURE_IDENTITY=true`: `user.name` on spans; `process.user.name` and `host.name` on the resource; Local harness only.
- Content is off by default (`COPILOT_OTEL_CAPTURE_CONTENT`) [doc].

**Config:**

- Enabled by the setting `github.copilot.chat.otel.enabled`, `COPILOT_OTEL_ENABLED=true`, any `OTEL_EXPORTER_OTLP_ENDPOINT`, or **[ADDED]** `dbSpanExporter.enabled`. **[ADDED]** `COPILOT_OTEL_ENDPOINT` beats `OTEL_EXPORTER_OTLP_ENDPOINT`.
- The default exporter is `otlp-http` at `http://localhost:4318`.
- **[CORRECTED]** The doc says the `OTEL_EXPORTER_OTLP_PROTOCOL` default is `http/protobuf` ("only `grpc` changes behavior"). The source builds the HTTP exporters from `@opentelemetry/exporter-*-otlp-http`, which means **OTLP/JSON** on the wire [src otelServiceImpl.ts].
- The exporter appends `/v1/traces`, `/v1/logs` and `/v1/metrics` after stripping a trailing slash. This is now first-party verified, not only via OpenObserve.
- Metric interval 10 s; no temporality set, so **cumulative** by default (JS default).
- Enterprise-managed `telemetry` settings exist per the VS Code doc; their fields are UNVERIFIED (docs.github.com blocked).

### GenAI semconv

**[CORRECTED]** Changelog 374 replaced `gen_ai.client.token.usage` (and `gen_ai.token.type`) with **per-operation histograms**: `gen_ai.client.inference.operation.input_tokens` / `…output_tokens`.

Separately, the token-metrics doc defines **usage counters** (unit `{token}`, broken down by `gen_ai.token.modality`) as the primary instruments for totals and cost:

- `gen_ai.client.inference.usage.input_tokens` (includes cached)
- `…usage.output_tokens` (includes reasoning)
- `…usage.cache_read.input_tokens`
- `…usage.cache_write.input_tokens`
- `…usage.reasoning.output_tokens`

Use the histograms only for distributions. All of these are at Development stability.

### Normalization

Store `uncached_input`, `cache_read`, `cache_write`, `output` and `reasoning_output`, and normalize per tool:

- **Claude Code:** `input` → `uncached_input`.
- **Semconv usage counters:** `uncached = input − cache_read − cache_write`.
- **Codex:** `uncached = input − cached_input`, minus `cache_write_input` (UNVERIFIED whether to subtract it).
- **Gemini:** `uncached = input − cache` (UNVERIFIED).
- **Copilot:** input/output only.
- Histogram producers (Codex, Copilot): use `HistogramDataPoint.sum` under the same delta/cumulative rules.

## Open questions (updated)

**Resolved:**

- Managed env vs. shell: managed wins in the CLI (§1.8).
- `commit.count` / `pull_request.count` shape: observed (§1.5).
- Codex endpoint as-is: first-party verified.

**Still open:**

- UTC vs. per-org timezone.
- HMAC vs. unsalted SHA-256, and whether to lowercase (must change together with push-ingest).
- Per-user rows vs. team-only.
- Traces: 200 + partial_success vs. 404.
- gRPC support. Claude Code doc examples and the Gemini CLI default use gRPC.
- Root `/v1/*` aliases.
- Events vs. metrics as the cost source.
- Model-name normalization.
- Claude Tag exclusion (needs `OTEL_METRICS_INCLUDE_ENTRYPOINT=true`).
- Whether Gemini/Codex `input` includes cache tokens.
- Copilot enterprise-managed `telemetry` fields.
- The exact Claude Code shutdown flush window.

## Verification notes

High confidence overall. I re-fetched every primary source that was reachable: the Claude Code monitoring, managed-settings, server-managed-settings and settings-reference docs; the OTLP spec and protos; proxy.golang.org module metadata; and Codex, Gemini CLI, Copilot Chat, opentelemetry-js, opentelemetry-rust and collector sources on raw.githubusercontent.com. I also ran 19 independent Claude Code 2.1.309 capture experiments with my own sink, Go decode tests and a Postgres re-run of the SQL.

Most of the original spec's concrete claims held up: env vars, wire shape, metric names, units and attributes, delta/cumulative behaviour, the ground-truth totals, protojson quirks, Go module versions and the SQL results.

The most important corrections:
- Retry-After: 30 on DB errors guarantees data loss. The JS exporter abandons retries beyond its 10 s export deadline; verified empirically (8 s retried, 12 s and 30 s not).
- A DB error during key lookup must be 503, not 401.
- Recommend the X-AIStats-Key header, because a percent-encoded Bearer is not decoded.
- Read team from resource attributes as a fallback.
- The existing push-ingest does not lowercase before hashing, contrary to what the spec said.
- Gemini CLI and Copilot send OTLP/JSON with cumulative temporality by default, so JSON and cumulative support are required.

commit.count and pull_request.count are now observed rather than inferred. Codex's endpoint-as-is behaviour and config syntax are now confirmed from first-party source.

Remaining UNVERIFIED items: whether Gemini 'input' includes cached tokens (ai.google.dev blocked); whether Codex input includes cache_write and output includes reasoning; Copilot enterprise-managed telemetry fields (docs.github.com blocked); the exact Claude Code shutdown flush window; and whether log records carry trace IDs with tracing on but no inbound TRACEPARENT.

Evidence:
- Captures: `<verification scratch: verify-runs/>`
- Downloaded sources: `<verification scratch: verify-dl/>`
- Go and SQL checks: `<verification scratch: verify-go/>`

Nothing in the repo was modified.

Corrections made during verification:

- **Claim:** Handler sketch step 6: on InsertOTelPoints error set `Retry-After: 30` and return 503 so the client retries.  
  **Finding:** WRONG and causes data loss. The OTel JS exporter inside Claude Code only retries within its export deadline (OTEL_EXPORTER_OTLP_TIMEOUT, default 10000 ms) and gives up immediately when Retry-After exceeds the remaining time. Tested with Claude Code 2.1.309: Retry-After 8 was retried after 8 s with identical bytes; Retry-After 12 and 30 were never retried and the data was lost. Use Retry-After 1-3 or omit it, and keep handler latency well under 10 s (DB context timeout of about 3-5 s). (opentelemetry-js experimental/packages/otlp-exporter-base/src/retrying-transport.ts and configuration/shared-configuration.ts (timeoutMillis 10000); captures verify-runs/v13_ra30, v15_ra8, v16_ra12)
- **Claim:** Hash the user identity lowercased with ingest.HashKey, 'matching the existing push-ingest distinctUsers hashing'.  
  **Finding:** The existing push-ingest does NOT lowercase: it calls ingest.HashKey(ev.UserEmail) on the raw value (backend/internal/handlers/ingest.go:176). The author's otlp.go does call strings.ToLower. Hashes would not match. Either hash the raw value, or lowercase in both paths together. (backend/internal/handlers/ingest.go; scratchpad otel-gotest/otlp/otlp.go:264)
- **Claim:** Auth: missing or invalid API key → 401 (reusing the IngestHandler pattern).  
  **Finding:** Incomplete and risky. The existing PostEvents maps EVERY GetIngestKeyByHash error, including DB outages, to 401. 401 is non-retryable, so a DB blip drops OTLP data. GetIngestKeyByHash wraps with %w, so the OTLP handler should send errors.Is(err, pgx.ErrNoRows) to 401 and anything else to 503. (backend/internal/db/queries.go:864-877; backend/internal/handlers/ingest.go:83-88; OTLP spec #retryable-response-codes)
- **Claim:** Admin sets OTEL_EXPORTER_OTLP_HEADERS=Authorization=Bearer lum_live_… (recommended form).  
  **Finding:** Works only with a literal space. With the percent-encoded form (Bearer%20lum_live_…), Claude Code sends the literal 'Bearer%20…' without decoding it, and the existing HasPrefix("Bearer ") check would answer 401. Recommend X-AIStats-Key=lum_live_… (base64url key, nothing to encode), or also accept 'Bearer%20'. (capture verify-runs/v17_hdrenc)
- **Claim:** Persist team from team.id/department (implicitly from datapoint attributes, where OTEL_RESOURCE_ATTRIBUTES keys are copied).  
  **Finding:** Incomplete. When an OTEL_RESOURCE_ATTRIBUTES value contains a space, Claude Code copies none of the keys onto datapoints or events, but they still arrive in the resource block (team.id='a b'). Resource is also the only location when OTEL_METRICS_INCLUDE_RESOURCE_ATTRIBUTES=false. The receiver must fall back to resource attributes. The author's code does; the spec text did not say so. (capture verify-runs/v8_persignal; doc #attribute-telemetry-to-desktop-ssh-sessions)
- **Claim:** claude_code.commit.count and claude_code.pull_request.count were not triggered; their shape is inferred.  
  **Finding:** Now verified. A real git commit through the Bash tool produced commit.count=1, and a gh pr create (fake gh) produced pull_request.count=1. Both are Sum, delta, isMonotonic=true, asDouble, unit "", with no metric-specific attributes. (captures verify-runs/v18_commit, v19_pr)
- **Claim:** Claude Code has no default protocol, so the admin must set OTEL_EXPORTER_OTLP_PROTOCOL.  
  **Finding:** Correct, but the consequence was missing. With the protocol unset, telemetry init fails entirely and nothing is exported, with no stderr output. The debug log shows '[3P telemetry] Telemetry init failed: Unknown protocol set in OTEL_EXPORTER_OTLP_METRICS_PROTOCOL or OTEL_EXPORTER_OTLP_PROTOCOL env var: undefined'. (captures verify-runs/v11_noproto, v12_noproto_dbg (debug.log))
- **Claim:** Empty requests SHOULD get a success response (implicitly handled by Unmarshal).  
  **Finding:** protojson rejects a zero-length body ('syntax error … unexpected token'). Zero-length protobuf decodes fine. Treat len(body)==0 as an empty request for both formats, or a valid empty JSON export becomes a non-retryable 400. (verify-go/main.go output E1/E3)
- **Claim:** Invalid UTF-8 in strings SHOULD be replaced with U+FFFD.  
  **Finding:** Correct as an OTLP SHOULD, but not achievable with google.golang.org/protobuf. proto.Unmarshal fails with 'string field contains invalid UTF-8' and protojson with 'invalid UTF-8 in string', so the whole batch becomes a 400. Document this as a known deviation. (OTLP spec #utf-8-string-handling; verify-go/main.go output U8/U8b)
- **Claim:** protojson behaviour C1-C8b (unknown fields rejected by default, int64 number/string, hex IDs silently mis-decoded).  
  **Finding:** All confirmed. Added: a fixed64 timestamp of 18446744073709551615 decodes silently and casts to int64 -1, so reject values > MaxInt64 before writing BIGINT. 'NaN' and 'Infinity' strings are accepted for asDouble, so reject them. A NumberDataPoint without a value oneof is invalid per metrics.proto. (verify-go/main.go output U1/N1; metrics.proto NumberDataPoint comments)
- **Claim:** Every 4xx/5xx body MUST be a protobuf-encoded google.rpc.Status; 415 for bad Content-Type returns a Status.  
  **Finding:** Clarification. The Status is serialized in the request's encoding (binary or protojson), and the response Content-Type must mirror the request. For an unsupported Content-Type there is nothing to mirror: the reference collector replies 415 text/plain '415 unsupported media type, supported: [application/json, application/x-protobuf]'. (opentelemetry-collector receiver/otlpreceiver/otlphttp.go (errorHandler, handleUnmatchedContentType))
- **Claim:** Recommended managed-settings rollout; server-managed env change needs a security approval dialog (generic).  
  **Finding:** More specific: a server-managed OTEL_EXPORTER_OTLP_ENDPOINT ALWAYS requires approval in interactive sessions. Non-interactive runs apply it for that run only. So a server-managed rollout is not silent. (code.claude.com/docs/en/server-managed-settings.md #environment-variables-and-the-approval-dialog)
- **Claim:** Open question: can managed-settings env values such as OTEL_LOG_USER_PROMPTS=0 override a developer's shell env in terminal CLI sessions?  
  **Finding:** Answered by the docs: yes. 'A value here overwrites the same variable exported in your shell, and when more than one settings file sets a variable, the highest-precedence one applies.' Exception: when the Desktop app or a self-hosted runner launches the session, the launch environment wins. (code.claude.com/docs/en/settings-reference.md #how-env-values-interact-with-your-shell)
- **Claim:** Per-team tagging: deploy a different file per group carrying OTEL_RESOURCE_ATTRIBUTES=team.id=<team> ([CC managed settings]).  
  **Finding:** Not in the cited doc. managed-settings.md never mentions OTEL_RESOURCE_ATTRIBUTES; its 'Split a file-based policy across teams' section is about co-owning policy fragments. This is a reasonable AI Stats recommendation, not a sourced fact. Also, drop-ins merge per key, so a later file's OTEL_RESOURCE_ATTRIBUTES replaces an earlier one whole. (code.claude.com/docs/en/managed-settings.md)
- **Claim:** managed_settings_resolved: managed_settings.helper.path is always present.  
  **Finding:** Inaccurate. It is present whenever Claude Code selected a policy helper, regardless of OTEL_LOG_MANAGED_SETTINGS; it is absent when there is no helper (as in all my captures). (monitoring-usage.md #managed-settings-resolved-event; capture v1 001 logs)
- **Claim:** Other events (incl. compaction) are metadata only.  
  **Finding:** compaction carries 'error: Error message when compaction failed' (free text). Drop it like other error attributes. (monitoring-usage.md #compaction-event)
- **Claim:** tool_result safe metadata includes decision_type and decision_source; api_request includes client_request_id.  
  **Finding:** Both are documented, but none was present in Claude Code 2.1.309 -p captures through a custom ANTHROPIC_BASE_URL. client_request_id is documented as first-party connections only. The allowlist must not require them. (captures verify-runs/v10_content, v1_json_trailing)
- **Claim:** Doc contradiction: tool_input contained file content despite 'Raw file contents and code snippets are not included in metrics or events'.  
  **Finding:** Observation confirmed (tool_input held file_path plus content 'alpha\nbeta\ngamma\n' with OTEL_LOG_TOOL_DETAILS=1). But the same doc says tool_input is 'JSON-serialized tool arguments… values over 512 characters are truncated… Applies to all tools'. This is a tension or clarification issue rather than an undocumented leak. (capture verify-runs/v10_content; monitoring-usage.md #tool-result-event and #security-and-privacy)
- **Claim:** Logs: Claude Code log records only carry traceId/spanId in -p/SDK runs with TRACEPARENT.  
  **Finding:** Partly. The doc also says a record 'emitted while an interaction is active carries the interaction span's IDs'. Whether that applies with tracing enabled and no inbound TRACEPARENT is UNVERIFIED. Harmless if the receiver never reads the fields. (monitoring-usage.md lines on inbound trace context)
- **Claim:** slim/otlp v1.11.0 packages: collector/metrics/v1, collector/logs/v1, collector/trace/v1, metrics/v1, logs/v1, common/v1, resource/v1.  
  **Finding:** Also contains trace/v1 (needed by collector/trace/v1). All other module/version/go-directive claims were re-verified on proxy.golang.org. Minor: pdata v1.65.0 requires protobuf v1.36.12, which is still go-1.25-compatible. (proxy.golang.org .mod/.info files; GOMODCACHE listing)
- **Claim:** Codex: codex.user_prompt.prompt is redacted unless log_user_prompts; config syntax and 'endpoint used as-is' come only from third-party docs.  
  **Finding:** Config key is otel.log_user_prompt (singular), and the placeholder is '[REDACTED]'. The [otel] syntax (metrics_exporter = { otlp-http = { endpoint, protocol = binary|json, headers } }; default metrics_exporter = statsig; default log exporter none) is verified from codex-rs/config/src/types.rs. 'Endpoint used as-is' is verified from codex .with_endpoint() plus opentelemetry-rust resolve_http_endpoint ('programmatic configuration overrides…'), and the README example endpoint ends in /v1/logs. (openai/codex codex-rs/config/src/types.rs, codex-rs/otel/src/metrics/client.rs, codex-rs/otel/README.md; opentelemetry-rust opentelemetry-otlp/src/exporter/http/mod.rs)
- **Claim:** Codex: codex.turn.cost_microusd is a counter in micro-USD; codex.sse_event carries tool_token_count; whether token_type=input includes cached_input is unverified.  
  **Finding:** The counter is a u64_counter, so it arrives as an OTLP Sum with asInt, and its turn.id tag creates a new series every turn. tool_token_count is actually populated from usage.total_tokens (source quirk). input DOES include cached_input (non_cached_input() = input_tokens - cached_input()). There is also a codex.turn_cost log event with usage.estimated_usd. (codex-rs/otel/src/metrics/client.rs, codex-rs/otel/src/events/session_telemetry.rs, codex-rs/protocol/src/protocol.rs)
- **Claim:** Gemini CLI: GEMINI_TELEMETRY_OTLP_PROTOCOL grpc/http (default grpc); no temporality or encoding stated.  
  **Finding:** Missing facts from source. 'http' uses @opentelemetry/exporter-*-otlp-http, which is OTLP/JSON. It appends /v1/traces|logs|metrics to the endpoint path, and the metric interval is 10 s. No temporality is configured, so the OTel JS default CUMULATIVE applies unless OTEL_EXPORTER_OTLP_METRICS_TEMPORALITY_PREFERENCE=delta. gzip is used only on gRPC. Whether 'input' includes 'cache' tokens is UNVERIFIED (ai.google.dev blocked). (google-gemini/gemini-cli packages/core/src/telemetry/sdk.ts, loggers.ts; opentelemetry-js OTLPMetricExporterBase.ts)
- **Claim:** Copilot (VS Code): default exporter otlp-http, protocol http/protobuf; per OpenObserve, the exporter appends /v1/<signal>.  
  **Finding:** The doc does say the default is http/protobuf, but the source builds the HTTP exporters from @opentelemetry/exporter-*-otlp-http, which is OTLP/JSON on the wire. Appending /v1/traces|logs|metrics (after stripping a trailing slash) is now verified from first-party source. Metrics are cumulative by default (JS default), with a 10 s interval. gen_ai.client.token.usage has gen_ai.token.type input|output only. COPILOT_OTEL_ENDPOINT overrides OTEL_EXPORTER_OTLP_ENDPOINT, and dbSpanExporter.enabled also enables OTel. (microsoft/vscode-copilot-chat src/platform/otel/node/otelServiceImpl.ts, src/platform/otel/common/genAiMetrics.ts, genAiAttributes.ts; vscode-docs monitoring-agents.md)
- **Claim:** GenAI semconv: gen_ai.client.token.usage replaced by per-direction metrics (changelog 374); the new names are counters with unit {token}.  
  **Finding:** Conflated. Changelog 374 replaced the histogram with per-operation HISTOGRAMS (gen_ai.client.inference.operation.input_tokens / output_tokens). The gen_ai.client.inference.usage.* COUNTERS ({token}, by gen_ai.token.modality) are a separate family and are the primary instruments for totals and cost. All are Development stability. (open-telemetry/semantic-conventions-genai changelog.d/374.breaking.md and docs/gen-ai/gen-ai-token-metrics.md)
- **Claim:** Migration 009 stores per-user rows; the Point struct has UserKeyKind and ServiceName.  
  **Finding:** Internal inconsistency: the DDL has no user_key_kind or service_name columns although the extractor produces them (needed to separate identity kinds and claude-code vs claude-code-desktop). Added to the corrected DDL. The SQL itself re-ran correctly on Postgres 16 (day1=200, day2=60, cost 0.25). (spec §2.5/§2.6; verify-go/sql_test.sql run)
- **Claim:** (Not covered) Behaviour of failures at session end.  
  **Finding:** Added: when the final export at process exit received 503 + Retry-After 1, Claude Code made 3 attempts about 1 s apart and then exited before a successful 4th, losing the data. For short -p sessions the exit flush is the only export, so receiver availability at session end matters. The exact window is UNVERIFIED. (capture verify-runs/v14_ra1_shutdown)
- **Claim:** (Not covered) Delivery order of retried exports.  
  **Finding:** Added: a retried payload can arrive after the next interval's export (429 run). Delta storage must not assume ordering; the PK-based dedup and the cumulative WHERE-guarded upsert both tolerate this. (capture verify-runs/v6_429)
