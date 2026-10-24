# Claude Code Analytics API: pull connector spec (Go), verified revision

> **Status:** implementation spec, researched and independently re-verified on 2026-10-23 against the
> primary sources cited inline. Claims that could not be confirmed are marked UNVERIFIED. Re-check
> anything load-bearing against the live API before relying on it; vendor APIs change.

The original research is dated 2026-10-23. An independent verifier re-checked it on 2026-10-23:
- Every source below was fetched again as Markdown (`curl -sSL <url>.md`).
- The Go SDK zips were downloaded again from proxy.golang.org and grepped.
- The repo claims were checked against `backend`.

Markers used in this revision:
- **CORRECTED**: the original was wrong or misleading.
- **ADDED**: an implementer needs this and the original missed it.
- **UNVERIFIED**: no reachable source confirms the claim.

Nothing has been tested against the live API, because this environment has no Admin key.

| Tag | Source URL |
|---|---|
| S1 | https://platform.claude.com/docs/en/manage-claude/claude-code-analytics-api (guide) |
| S2 | https://platform.claude.com/docs/en/api/beta/organization/usage_report/retrieve_claude_code (API reference) |
| S3 | https://platform.claude.com/docs/en/manage-claude/admin-api (Admin API overview / auth) |
| S4 | https://platform.claude.com/docs/en/manage-claude/admin-api-keys |
| S5 | https://platform.claude.com/docs/en/manage-claude/usage-cost-api (Usage & Cost guide) |
| S6 | https://platform.claude.com/docs/en/api/beta/organization/usage_report/retrieve_messages |
| S7 | https://platform.claude.com/docs/en/api/beta/organization/cost_report/retrieve |
| S8 | https://platform.claude.com/docs/en/manage-claude/analytics-api (which analytics API / Enterprise) |
| S9 | https://platform.claude.com/docs/en/api/beta/organization/analytics (Claude Enterprise Analytics API reference) |
| S10 | https://platform.claude.com/docs/en/api/versioning |
| S11 | https://platform.claude.com/docs/en/api/errors |
| S12 | https://platform.claude.com/docs/en/api/beta/organization (Admin reference: List API Keys, List Workspaces, and the claude_code report, identical to S2) |
| S13 | https://platform.claude.com/docs/en/manage-claude/workspaces (Claude Code workspace) |
| S14 | https://code.claude.com/docs/en/analytics |
| S15 | Go SDK source: proxy.golang.org `github.com/anthropics/anthropic-sdk-go/@v/v1.84.0.zip`, plus `github.com/anthropics/anthropic-sdk-go/v2/@v/v2.2.0.zip` and `v2.3.0.zip` (ADDED) |
| S16 | https://platform.claude.com/docs/en/api/rate-limits |
| S17 | https://platform.claude.com/docs/en/api/migration-2026-10-21 (ADDED) |
| S18 | https://code.claude.com/docs/en/iam (Claude Code Console sign-in, ADDED) |
| S19 | https://code.claude.com/docs/en/costs (ADDED) |

---

## 1. Endpoint

- **`GET https://api.anthropic.com/v1/organizations/usage_report/claude_code`** [S1, S2, S12]
- It returns **daily aggregated** Claude Code metrics for **one UTC day per request**. There is no range query: `starting_at` "returns metrics for this single day only" [S1, S2].
- It is free to use for every organization with Admin API access [S1, S8].

**SDK support (CORRECTED and ADDED).** The conclusion still holds: **write a plain `net/http` client and do not add the SDK dependency for this connector.** The original spec looked only at v1.84.0, though.
- **v1.84.0** (2026-10-20T22:08:37Z) is the last v1 release.
- **The current major is the module `github.com/anthropics/anthropic-sdk-go/v2`.** v2.0.0 is the first Go SDK that uses API version `2026-10-21` [S17]. The latest release is v2.3.0, published 2026-10-23T23:17Z.
- **No version implements the Admin reports.** Neither v1.84.0 nor v2.2.0/v2.3.0 contains code for `/v1/organizations/usage_report/claude_code`, `/usage_report/messages` or `/cost_report` [S15].
- **All of them implement the Enterprise `/v1/organizations/analytics/*` endpoints.** v1.84.0 and v2.2.0 append `?beta=true` to those paths. v2.3.0 no longer does [S15].
- **ADDED, UNVERIFIED (SDK only, not in any public doc page):** SDK v2.2.0 and later also contain `GET /v1/organizations/analytics/cost` and `GET /v1/organizations/analytics/usage`.
  - They are sent with `anthropic-beta: analytics-usage-cost-2026-08-24`.
  - The SDK's doc comment says the cost endpoint "Requires an API key with the `read:analytics` scope, or an Admin API key". It also says it "Includes all cost, whether or not it is attributable to a seat user".
  - `group_by` values include `actor`, `actor_type`, `api_key_id`, `workspace_id`, `product`, `model`, `session_id`, `cost_type`, `token_type` and more.
  - Requesting the doc URL `.../analytics/cost/list.md` returns the `cost_report` page instead.
  - **Do not build on these endpoints.** Watch for them: they may become a single per-key/per-actor cost source for Console orgs.

## 2. Authentication and headers

| Header | Value | Notes |
|---|---|---|
| `x-api-key` | Admin API key `sk-ant-admin01-...` | Created at platform.claude.com/settings/admin-keys by an org member with the **admin** role [S4, S8]. Console admin keys have no selectable scopes. Each one has full access to every endpoint that accepts Admin keys. The exceptions are the service-account, federation-issuer and federation-rule endpoints, which accept only an `org:admin` OAuth token [S4]. **ADDED:** keys get a key expiration chosen at creation [S4], so expect a 401 when a key expires. |
| `anthropic-version` | `2023-06-01` (every guide example) or `2026-10-21` (reference examples; the latest version) | The reference marks it "optional" [S2], but the versioning page says "you must send an `anthropic-version` request header" [S10]. The 2026-10-21 breaking changes [S10, S17] touch neither the claude_code report nor the messages-usage or cost reports. **Admin API:** RBAC groups `roles`→`role_ids`; RBAC group members `group_id`→`rbac_group_id`; RBAC roles `name`→`display_name`; rate-limit `group_type`→`group.type`; API keys `workspace_id`→`scope`. **Enterprise Analytics:** per-user usage/cost reports `actor.email`→`actor.email_address`; activity summaries `summaries`→`data`. **Recommendation:** make the version configurable and default it to `2023-06-01`. Read §11c step 4 for its effect on `api_keys`. |
| `User-Agent` | e.g. `AIStats/<ver> (https://<project-url>)` | The docs ask integrations to set one: "`User-Agent: YourApp/1.0.0 (https://yourapp.com)`" [S1, S5]. |
| `anthropic-beta` | not needed | The claude_code reference lists only `anthropic-version` [S2]. The messages and cost references list `anthropic-beta` as optional [S6, S7], but no beta is needed for them either. |

Other credentials:
- **OAuth bearer token:** `authorization: Bearer <token>` with the `org:admin` scope. Only members with the admin, owner or primary owner role can obtain one [S3].
- **CORRECTED: which header an Admin key uses.**
  - S3 documents the Admin key in `x-api-key`.
  - The reference examples [S2, S6, S7, S12] send `Authorization: Bearer $ANTHROPIC_API_KEY`.
  - The 2026-10-21 migration guide says SDKs now send keys as `Authorization: Bearer` and that "The Claude API accepts a key in either header" [S17].
  - So `Authorization: Bearer sk-ant-admin01-...` very likely works. That is **UNVERIFIED for Admin endpoints specifically.**
  - **Use `x-api-key`, the header every guide documents.**
- **Also accepted:** a personal key or a service-account key that **isn't scoped to a workspace**, sent in `x-api-key`. It has the same permissions as the linked account [S1, S3].
- **Rejected:** "workspace API keys don't work" [S1, S5].

Availability limits:
- Individual accounts cannot use the Admin API [S1, S3].
- The Claude Code Analytics API is **not available on Claude Platform on AWS** [S1, S3].
- An Admin API key cannot call the Claude Enterprise Analytics API. An Analytics API key (`sk-ant-api01-...`, `read:analytics`) cannot call the Admin API [S8].
- A key created in one org cannot manage another org [S4].

Minimal request [S1]:
```bash
curl "https://api.anthropic.com/v1/organizations/usage_report/claude_code?starting_at=2025-09-08&limit=20" \
  -H "anthropic-version: 2023-06-01" \
  -H "x-api-key: $ANTHROPIC_ADMIN_KEY"
```

## 3. Query parameters

| Param | Type | Required | Rules |
|---|---|---|---|
| `starting_at` | string | **yes** | A UTC date in `YYYY-MM-DD` format, pattern `^\d{4}-\d{2}-\d{2}$`, format `date` [S2]. It means UTC midnight of that day and returns that single day only [S1, S2]. **Do not send an RFC 3339 timestamp here**, unlike the messages and cost reports. The pattern implies a timestamp is rejected, but the exact error is **UNVERIFIED** (presumably 400 `invalid_request_error`). What happens for today's date or a future date is **UNVERIFIED**. |
| `limit` | integer | no | Records per page (not time buckets). Default **20**, max **1000** [S1, S2], min **1** [S2]. |
| `page` | string | no | Opaque cursor copied from the previous response's `next_page` [S1, S2]. |
| `ending_at` | — | **does not exist** | The endpoint has no `ending_at` [S2]. To backfill, make one request per day. |

## 4. Pagination

- The response carries `has_more: boolean` and `next_page: string | null`. `next_page` is null when no more pages exist [S2].
- **Loop:**
  1. Send the first request (with an optional `limit`).
  2. While `has_more == true`, request again with the same `starting_at` and `page=<next_page>`.
  3. Stop when `has_more` is false [S1].
- The guide's second-page example sends `starting_at` plus `page` and omits `limit` [S1]. **Recommendation:** resend the same `starting_at` and `limit` every time.
- Cursors look like `page_MjAyNS0wNS0xNFQwMDowMDowMFo=` [S1, S2]. **ADDED:** that example base64-decodes to `2025-05-14T00:00:00Z`, which has nothing to do with the requested date. Treat cursors as strictly opaque and never parse them.
- Docs: "The cursor encodes the position of the last record and ensures stable pagination even as new data arrives. Each pagination session maintains a consistent data boundary to ensure you don't miss or duplicate records" [S1].
- **UNVERIFIED:** whether cursors for this endpoint expire. HTTP 410 cursor expiry is documented only for the Enterprise cost and usage reports [S9]. Handle a 4xx on a `page=` request by restarting that day from page 1.

## 5. Response schema (exact field names and types, from S2 unless noted)

Top level (`BetaClaudeCodeUsageReport`):
- `data`: array of record
- `has_more`: boolean
- `next_page`: string | null

Record fields:

| Field | Type | Notes |
|---|---|---|
| `date` | string (date-time) | The UTC day as an RFC 3339 timestamp at midnight UTC, e.g. `2025-08-08T00:00:00Z` [S1, S2]. |
| `actor` | object, union on `type` | `{"type":"user_actor","email_address":string}` or `{"type":"api_actor","api_key_name":string}` [S1, S2]. `user_actor` = OAuth sign-in (the most common case); `api_actor` = API-key sign-in [S1]. The record has **no user ID and no API key ID**. **ADDED:** the Console dashboard shows "API key users … by key identifier, OAuth users … by email address" [S14]. |
| `organization_id` | string | A bare organization UUID, e.g. `dc9f6c26-b22c-4831-8d01-0446bada88f1` [S1, S2]. **ADDED:** the Enterprise Analytics reports return a tagged `org_...` ID instead [S9]. Do not assume the two formats are comparable. |
| `customer_type` | `"api"` \| `"subscription"` | The reference says "api for API customers, subscription for Pro/Team customers" [S2]. The guide says records cover only usage authenticated against your Claude Console org, whatever the value [S1]. |
| `subscription_type` | optional: `"enterprise"` \| `"team"` \| null | The subscription tier for subscription customers; `null` for API customers [S2]. It is not in the guide's field list [S1]. Decode it as optional/nullable. |
| `terminal_type` | string | The terminal or environment, e.g. `vscode`, `iTerm.app`, `tmux` [S1, S2]. Free-form. |
| `is_remote` | boolean | True for remote Claude Code sessions such as Claude Code on the web. "Remote and local usage are reported as separate rows" [S2]. It is not in the guide's field list [S1]. |
| `core_metrics.num_sessions` | number | Distinct sessions this actor started [S1, S2]. |
| `core_metrics.lines_of_code.added` / `.removed` | number | Lines Claude Code added or removed across all files [S1, S2]. |
| `core_metrics.commits_by_claude_code` | number | Commits made through Claude Code's commit functionality [S1, S2]. |
| `core_metrics.pull_requests_by_claude_code` | number | PRs made through Claude Code's PR functionality [S1, S2]. |
| `tool_actions` | **`map[object]`** keyed by tool | Each value is `{accepted:number, rejected:number}`. Documented keys: `edit_tool`, `multi_edit_tool`, `write_tool`, `notebook_edit_tool` [S1, S2]. Decode it as a map so new keys don't break parsing. Acceptance rate = `accepted / (accepted + rejected)` [S1]. |
| `model_breakdown[]` | array | One entry per model [S1, S2]. |
| `model_breakdown[].model` | string | e.g. `claude-opus-5-5`, `claude-sonnet-5-5` [S1, S2]. The guide's prose example says `claude-opus-5` [S1]. Free-form. |
| `model_breakdown[].tokens.input` / `.output` / `.cache_read` / `.cache_creation` | number | Token counts for that model [S1, S2]. |
| `model_breakdown[].estimated_cost.currency` | string | "currently always `USD`" [S1, S2]. |
| `model_breakdown[].estimated_cost.amount` | **JSON number (not a string)** | **The unit is cents (minor currency units).** Guide: "Estimated cost in cents USD for this model" [S1]. Reference: "Estimated cost amount in minor currency units (e.g., cents for USD)" [S2]. The examples are `113`, `186` and `42`, all integers [S1, S2], so `186` = $1.86. **Do not divide by 1000 or treat the value as dollars.** **CORRECTED attribution:** the sentence "Spend figures in the Console dashboard are estimates for analytics purposes. For actual costs, refer to your billing page" [S14] describes the Console *dashboard*. The API only labels this field "Estimated cost". Treating it as non-billing-grade is still the right call. |

**Unit comparison across Anthropic APIs:**
- **Claude Code Analytics:** `amount` is a JSON **number** in cents [S1, S2].
- **Usage & Cost `cost_report`:** `amount` is a **decimal string** in cents, e.g. `"123.45"` = $1.23 [S5, S7].
  - **ADDED:** the reference example is `"123.78912"`, which carries sub-cent precision [S7]. Parse it as a decimal and store it as NUMERIC, never as a float.
- **Enterprise Analytics:** `amount` is a **decimal string** in cents, e.g. `"41280.000000"` = $412.80 [S8, S9].
  - **ADDED:** on the per-user reports, `amount` is "post-discount, pre-credit" and `list_amount` is the pre-discount list price [S9].
  - S8 warns against binary floating-point parsing.

### Verbatim example response, API reference [S2] (re-checked: identical to S2 and S12)
```json
{
  "data": [
    {
      "actor": {
        "email_address": "user@emaildomain.com",
        "type": "user_actor"
      },
      "core_metrics": {
        "commits_by_claude_code": 8,
        "lines_of_code": {
          "added": 342,
          "removed": 128
        },
        "num_sessions": 15,
        "pull_requests_by_claude_code": 2
      },
      "customer_type": "api",
      "date": "2025-08-08T00:00:00Z",
      "is_remote": false,
      "model_breakdown": [
        {
          "estimated_cost": {
            "amount": 186,
            "currency": "USD"
          },
          "model": "claude-opus-5-5",
          "tokens": {
            "cache_creation": 2340,
            "cache_read": 8790,
            "input": 45230,
            "output": 12450
          }
        },
        {
          "estimated_cost": {
            "amount": 42,
            "currency": "USD"
          },
          "model": "claude-sonnet-5-5",
          "tokens": {
            "cache_creation": 890,
            "cache_read": 3420,
            "input": 23100,
            "output": 5680
          }
        }
      ],
      "organization_id": "12345678-1234-5678-1234-567812345678",
      "terminal_type": "iTerm.app",
      "tool_actions": {
        "edit_tool": {
          "accepted": 25,
          "rejected": 3
        },
        "multi_edit_tool": {
          "accepted": 12,
          "rejected": 1
        },
        "notebook_edit_tool": {
          "accepted": 5,
          "rejected": 2
        },
        "write_tool": {
          "accepted": 8,
          "rejected": 0
        }
      },
      "subscription_type": "enterprise"
    }
  ],
  "has_more": true,
  "next_page": "page_MjAyNS0wNS0xNFQwMDowMDowMFo="
}
```

### Verbatim example response, guide [S1] (re-checked: identical)
```json
{
  "data": [
    {
      "date": "2025-09-08T00:00:00Z",
      "actor": {
        "type": "user_actor",
        "email_address": "developer@company.com"
      },
      "organization_id": "dc9f6c26-b22c-4831-8d01-0446bada88f1",
      "customer_type": "api",
      "terminal_type": "vscode",
      "core_metrics": {
        "num_sessions": 5,
        "lines_of_code": {
          "added": 1543,
          "removed": 892
        },
        "commits_by_claude_code": 12,
        "pull_requests_by_claude_code": 2
      },
      "tool_actions": {
        "edit_tool": {
          "accepted": 45,
          "rejected": 5
        },
        "multi_edit_tool": {
          "accepted": 12,
          "rejected": 2
        },
        "write_tool": {
          "accepted": 8,
          "rejected": 1
        },
        "notebook_edit_tool": {
          "accepted": 3,
          "rejected": 0
        }
      },
      "model_breakdown": [
        {
          "model": "claude-opus-5-5",
          "tokens": {
            "input": 100000,
            "output": 35000,
            "cache_read": 10000,
            "cache_creation": 5000
          },
          "estimated_cost": {
            "currency": "USD",
            "amount": 113
          }
        }
      ]
    }
  ],
  "has_more": false,
  "next_page": null
}
```

### Suggested Go decode types (recommendation, derived from S2)
```go
type ccUsageReport struct {
	Data     []ccRecord `json:"data"`
	HasMore  bool       `json:"has_more"`
	NextPage *string    `json:"next_page"`
}

type ccRecord struct {
	Date             time.Time `json:"date"` // RFC 3339, midnight UTC
	Actor            ccActor   `json:"actor"`
	OrganizationID   string    `json:"organization_id"` // bare UUID (Enterprise APIs use "org_...")
	CustomerType     string    `json:"customer_type"`     // "api" | "subscription"
	SubscriptionType *string   `json:"subscription_type"` // "enterprise" | "team" | null (optional)
	TerminalType     string    `json:"terminal_type"`
	IsRemote         bool      `json:"is_remote"`
	CoreMetrics      struct {
		NumSessions int64 `json:"num_sessions"`
		LinesOfCode struct {
			Added   int64 `json:"added"`
			Removed int64 `json:"removed"`
		} `json:"lines_of_code"`
		CommitsByClaudeCode      int64 `json:"commits_by_claude_code"`
		PullRequestsByClaudeCode int64 `json:"pull_requests_by_claude_code"`
	} `json:"core_metrics"`
	ToolActions    map[string]ccToolAction `json:"tool_actions"` // edit_tool, multi_edit_tool, write_tool, notebook_edit_tool, ...
	ModelBreakdown []ccModelUsage          `json:"model_breakdown"`
}

type ccActor struct {
	Type         string `json:"type"`                    // "user_actor" | "api_actor"
	EmailAddress string `json:"email_address,omitempty"` // user_actor only
	APIKeyName   string `json:"api_key_name,omitempty"`  // api_actor only
}

type ccToolAction struct {
	Accepted int64 `json:"accepted"`
	Rejected int64 `json:"rejected"`
}

type ccModelUsage struct {
	Model  string `json:"model"`
	Tokens struct {
		Input         int64 `json:"input"`
		Output        int64 `json:"output"`
		CacheRead     int64 `json:"cache_read"`
		CacheCreation int64 `json:"cache_creation"`
	} `json:"tokens"`
	EstimatedCost struct {
		Currency string      `json:"currency"` // "USD"
		Amount   json.Number `json:"amount"`   // CENTS; typed "number" (not guaranteed integer) -> store NUMERIC
	} `json:"estimated_cost"`
}
```
- Use `json.Number` for `amount`: the docs type it as "number", and every example happens to be an integer.
- **ADDED:** every count field is also documented only as JSON "number". `encoding/json` fails the **whole decode** if it meets a value like `1.0` while decoding into `int64`. Whether non-integer counts ever occur is **UNVERIFIED**. If robustness matters more than strictness, decode counts as `json.Number` (or `float64`) and convert, logging any value that has a fractional part.

## 6. Row granularity (important for the upsert key)

- The guide says "Each record represents one user's activity for the specified day" [S1].
- The reference adds that `is_remote` splits **remote and local usage into separate rows** [S2]. Each row also carries `terminal_type` and `customer_type`/`subscription_type`. **More than one row per actor per day is possible.**
- **Recommendation:**
  - Key raw rows on `(org, date, actor.type, actor identity, terminal_type, is_remote, customer_type)`.
  - Or aggregate to `(org, date, actor)` in the connector before upserting.
  - Never assume one row per actor.
  - The exact split rule is **UNVERIFIED** (see Open questions).

## 7. Data freshness and lag

- "Metrics are available with up to a 1-hour delay for consistency." "Data typically appears within 1 hour of user activity completion. To ensure consistent pagination results, only data older than 1 hour is included in responses" [S1].
- No real-time data: daily aggregates only. Use OpenTelemetry for real-time [S1].
- All dates are UTC [S1].
- **Recommendation:**
  - Today's (UTC) and yesterday's rows are partial and will grow.
  - On each sync (e.g. hourly), re-pull a trailing window of at least today plus the previous 2 days.
  - Replace or upsert whole days idempotently.

## 8. Rate limits and errors

- **The docs give no rate limit for this endpoint** [S1, S2, S12, S16]. A web search found no figure either. **UNVERIFIED.**
- Usage & Cost API, for comparison: "The API supports polling once per minute for sustained use. For short bursts (for example, downloading paginated data), more frequent polling is acceptable" [S5].
- Enterprise Analytics API, for comparison: a default of 60 requests/min per organization, across all its endpoints [S8].
- **Recommendation:**
  - Run one sync at a time per org, with requests in series.
  - Use `limit=1000`.
  - On 429, honor `retry-after`.
  - On 5xx/529, use exponential backoff with jitter and a cap.

**Error handling (CORRECTED and ADDED)** [S11, S16]:
- Error body: `{"type":"error","error":{"type":"<error_type>","message":"..."},"request_id":"req_..."}`. The `type` values may grow over time.
- Every response carries a `request-id` header; log it. Responses also carry `anthropic-organization-id` [S11].
- Status codes:
  - 400 `invalid_request_error`. This type "may also be used for other 4XX status codes not listed".
  - 401 `authentication_error`: malformed, revoked or **expired** key.
  - **402 `billing_error`**.
  - 403 `permission_error`.
  - 404 `not_found_error`.
  - **409 `conflict_error`**.
  - **413 `request_too_large`**.
  - 429 `rate_limit_error`.
  - 500 `api_error`.
  - **504 `timeout_error`**.
  - 529 `overloaded_error`.
- `retry-after` is a number of **seconds** [S16]. **Not every 429 carries it.** A usage-tier spend-cap 429 has no `retry-after` and keeps failing until access resumes, at 00:00 UTC on the first of the next month [S11, S16]. Whether spend caps ever block Admin report endpoints is **UNVERIFIED**.
- **Recommendation:** on a 429 without `retry-after`, use capped exponential backoff, then give up for this run and surface the error.

## 9. Retention and earliest date

- "Historical Claude Code analytics data is retained and accessible through the API. There is no specified deletion period for this data" [S1].
- **No earliest available date is documented** [S1, S2]. **UNVERIFIED.** The reference example uses 2025-08-08 [S2]; the guide uses 2025-09-08 [S1].
- **Recommendation:** use a configurable backfill horizon (e.g. 90 days). Walk days backward and treat empty `data` as "no activity", not as an error.

## 10. Coverage: Bedrock, Vertex and subscriptions

- "This API only tracks Claude Code usage on the Claude API." It excludes Claude in Amazon Bedrock, Claude in Microsoft Foundry, Claude Models on Google Cloud, and Claude Platform on AWS [S1].
- It also excludes Claude Code sessions signed in with a Claude **subscription plan** (Pro, Max, Team, Enterprise). That usage belongs to the claude.ai org and the API does not return it [S1].
  - For Enterprise orgs, use the Claude Enterprise Analytics API (§12).
  - For Team, use the claude.ai analytics dashboard or its spend-report export. The Enterprise Analytics API "is not available on the Teams plan" [S14].
  - For any plan, OpenTelemetry also works [S1, S14].
- The Enterprise Analytics API also does **not** return Claude Code activity that goes through Amazon Bedrock [S8].
- **ADDED:** S19 says that on Bedrock, Google Cloud's Agent Platform and Microsoft Foundry, Claude Code is billed to the cloud account. Its suggested reporting there is OpenTelemetry or an LLM gateway.
- **Consequence for AI Stats:** Claude Code usage through Bedrock or Vertex can only be captured by our own OTel or push-ingest path, by a gateway, or by cloud billing exports.

## 11. Usage & Cost Admin API: attributing API-key spend by workspace, key and model

Both endpoints use the same auth as above: Admin key in `x-api-key`, workspace keys rejected [S5].

Availability:
- Not available on Claude Platform on AWS [S3, S5].
- On Claude Models on Google Cloud (Anthropic-managed), only the **usage** endpoint is available. Call it through the org-level base URL with Google Cloud credentials; `roles/claude.admin` is the only predefined role that allows it [S5].
- S5 does not mention Bedrock.

Claude Enterprise (claude.ai) orgs must use the Enterprise Analytics API instead [S5, S8].

### 11a. `GET /v1/organizations/usage_report/messages` (tokens only, no dollars) [S5, S6]

**Query parameters:**

| Param | Type / values | Notes |
|---|---|---|
| `starting_at` | **RFC 3339 date-time** (required) | Returns buckets that start on or after this time, snapped to the start of the minute/hour/day in UTC. |
| `ending_at` | optional RFC 3339 | Returns buckets that end before this time. |
| `bucket_width` | `1d` (default) \| `1h` \| `1m` | |
| `limit` | number of **time buckets** per page | `1d`: default 7, max 31. `1h`: default 24, max 168. `1m`: default 60, max 1440 [S5, S6]. |
| `page` | string | The `next_page` value from the previous response. |
| `group_by[]` | any subset of `account_id`, `api_key_id`, `context_window`, `inference_geo`, `model`, `service_account_id`, `service_tier`, `speed`, `workspace_id` | Arrays use bracket notation: repeat the parameter for each value. |
| `api_key_ids[]`, `workspace_ids[]`, `models[]`, `account_ids[]`, `service_account_ids[]` | filters | |
| `service_tiers[]` | `batch`, `flex`, `flex_discount`, `priority`, `priority_on_demand`, `standard` | |
| `context_window[]` | `0-200k`, `200k-1M` | Note the singular parameter name. |
| `inference_geos[]` | `global`, `us`, `not_available` | |
| `speeds[]` | `standard`, `fast` | |

**Response:**
- Top level: `{data:[bucket], has_more, next_page}`.
- Buckets are oldest first. Empty intervals are included with `results: []`. A page holds at most `limit` buckets [S6].
- Bucket: `{starting_at, ending_at (exclusive), results:[...]}` [S6].
- Token counts per result item: `uncached_input_tokens`, `cache_read_input_tokens`, `cache_creation.ephemeral_5m_input_tokens`, `cache_creation.ephemeral_1h_input_tokens`, `output_tokens`, `server_tool_use.web_search_requests` [S6].
- Dimensions per result item, each null unless grouped on: `api_key_id`, `workspace_id`, `model`, `account_id`, `service_account_id`, `service_tier`, `context_window`, `inference_geo`, `speed` [S6]. Further null cases:
  - `workspace_id` is null for the **default workspace** [S5, S6].
  - `api_key_id` is null for usage in the Console, such as the playground [S5, S6].
  - **ADDED:** `account_id` is null "for non-OAuth requests" [S6]. It is populated only for OAuth-authenticated traffic.
  - **ADDED:** `service_account_id` is null "for non-OIDC-federation requests" [S6].
- Code execution is **not** in the usage endpoint [S5].

### 11b. `GET /v1/organizations/cost_report` (USD, billed) [S5, S7]

**Query parameters:**
- `starting_at` (RFC 3339, required)
- `ending_at`
- `bucket_width`: **`1d` only**
- `limit`: default 7, min 1, max 31 buckets
- `page`
- `group_by[]`: **only `workspace_id` and `description`**. **There is no `api_key_id` or `model` grouping.** Grouping by `description` adds parsed `model`, `token_type`, `service_tier`, `context_window`, `speed` and related fields [S5, S7].

**Result item fields** [S7]:
- `amount`: **decimal string in cents** (`"123.45"` = $1.23). Values can carry sub-cent decimals (example `"123.78912"`).
- `currency`: `"USD"`
- `cost_type`: `tokens` \| `web_search` \| `code_execution` \| `session_usage` \| null
- `description`: e.g. `"Claude Opus 5.5 Usage - Input Tokens"`
- `model`
- `token_type`: `uncached_input_tokens` \| `cache_read_input_tokens` \| `cache_creation.ephemeral_5m_input_tokens` \| `cache_creation.ephemeral_1h_input_tokens` \| `output_tokens`
- `service_tier`: `batch` \| `standard` \| null
- `context_window`, `inference_geo`, `speed`, `workspace_id`
- **ADDED:** the docs contradict each other on `inference_geo`. S7 says "Always `null` if not grouping by inference geo", but `inference_geo` is not a valid `group_by` value here. S5 says grouping by `description` includes `inference_geo`. Treat it as nullable.

**Exclusions:**
- Priority Tier costs are excluded; track them through the usage endpoint [S5].
- Code execution appears only in the cost endpoint, under the description `Code Execution Usage` [S5].

**Freshness and polling:** "typically appears within 5 minutes of API request completion, though delays might occasionally be longer." Polling once a minute is fine for sustained use [S5].

### 11c. Attribution recipe (recommendations; inferences are marked)

1. **Exact dollars by workspace and model:** call `cost_report` with `group_by[]=workspace_id&group_by[]=description` [S5, S7].
2. **Isolate Claude Code spend on the Console.**
   - **What the docs say:** the first time a member signs in to Claude Code with a Console account, Anthropic auto-creates a workspace named **"Claude Code"** [S13, S19].
   - **CORRECTED:** S13 says Claude Code usage there is **rate-limited separately**. It also says this is "the only workspace that supports per-user monthly spend limits". It does not say the workspace is "spend-limited separately"; any workspace can have spend limits.
   - **CORRECTED and ADDED:** S13 says Claude Code mints a per-user API key in this workspace at sign-in. Since Claude Code v2.1.242, though, Console sign-in also offers a keyless "Sign in with your Console account (recommended)" path. It keeps an OAuth token and **creates no API key** [S18]. The legacy path, and certain managed-settings cases, still mint a key [S18].
   - **UNVERIFIED:** which workspace keyless OAuth traffic is billed to.
   - **Finding the workspace ID:** call `GET /v1/organizations/workspaces` [S12].
     - Params: `after_id`, `before_id`, `limit` (1–1000, default 20).
     - **ADDED** params: `include_archived` (default false) and `include_default` (default false). The Default Workspace appears only with `include_default=true` [S12, S13].
     - Response `data[]`: `{type:"workspace", id, name, archived_at, created_at, compartment_id, data_residency, display_color, external_key_id, tags}`, plus `has_more`, `first_id` and `last_id`.
     - Matching on `name == "Claude Code"` is an inference. Whether the name is stable (it may be renameable) is **UNVERIFIED**, and no tag marks the workspace.
   - Filter `cost_report` rows by that workspace ID to get billed (not estimated) Claude Code dollars.
3. **Spend per API key or per user.** Only **tokens** are available per key.
   - Call `usage_report/messages` with `group_by[]=api_key_id&group_by[]=model`, then multiply by list prices to get an estimate.
   - The docs: "use the Usage API to track token consumption as a cost proxy". They recommend the Claude Code Analytics API for per-user Claude Code cost, "without the performance limitations of breaking down costs by many API keys" [S5].
   - **ADDED (inference, UNVERIFIED):** keyless OAuth Claude Code sign-ins have no `api_key_id`. `group_by[]=account_id` (populated only for OAuth requests [S6]) is the likely way to attribute their tokens per user.
4. **Key metadata:** call `GET /v1/organizations/api_keys` [S12].
   - Response fields: `data[].{type:"api_key", id, name, status, created_at, created_by, expires_at, partial_key_hint, principal, scope}`.
     - `status` is `active` \| `archived` \| `expired` \| `inactive`.
     - `created_by` is nullable `{type: "user"|"service_account", id}`.
     - `principal` is nullable: `{type:"user_actor", user_id}` or `{type:"service_account_actor", service_account_id}`.
     - `scope` is `{"type":"workspace","workspace_id":...}` or `{"type":"organization"}`.
   - Pagination: `after_id`/`before_id`; `limit` 1–1000, default 20; `has_more`, `first_id` and `last_id` in the response.
   - Filters: `status`, `workspace_id`, `created_by_user_id`.
   - **CORRECTED:** the spec defaults `anthropic-version` to `2023-06-01`. Under that version, the top-level `workspace_id` is still returned, but it is **null for keys bound to the Default Workspace and for all-workspaces keys** [S17]. `scope.workspace_id` carries the real ID even for the Default Workspace [S12, S13, S17]. Version `2026-10-21` removes `workspace_id` entirely [S10, S17]. **Read `scope` first and fall back to `workspace_id`.** S3 describes `scope` on every key without tying it to a version, but its presence under `2023-06-01` is not explicitly version-qualified (low risk).
   - **Caveat:** Claude Code `api_actor` rows carry only `api_key_name` [S2]. Joining them to key IDs by name is best-effort, because the docs do not say names are unique.

## 12. Claude Enterprise (claude.ai) Analytics API: the alternative for subscription seats

**Key and access:**
- **Key:** an Analytics API key `sk-ant-api01-...` with the `read:analytics` scope [S4, S8].
- **Who creates it:** only the **primary owner** [S8].
- **Where:** claude.ai → Organization settings → API (claude.ai/admin-settings/api-access) [S4, S8].
- **ADDED:** the primary owner must first **enable public API access** there [S8], and the Analytics API must be enabled for the org before a `read:analytics` key works [S4].
- **ADDED:** S14 gives a different URL for creating the key, claude.ai/analytics/api-keys. Treat the exact UI path as **UNVERIFIED**.
- **Headers:** the key goes in `x-api-key`, with `anthropic-version` on every request [S8].
- **Base path:** `https://api.anthropic.com/v1/organizations/analytics/` [S8].
- **ADDED, plan applicability:** engagement data is available on all Enterprise plans. **The cost and usage endpoints apply to usage-based Enterprise plans; for seat-based plans they reflect usage credits only** [S8]. The API is not available on Team [S14].

**Data availability and freshness:**
- Data exists only for dates on or after **2026-01-01** [S8, S9].
- **Engagement endpoints:**
  - They have a **1-day lag**: a day's data is ready around 13:00–13:30 UTC the next day. Until then, the most recent available day is usually two days before the current UTC date [S8].
  - Asking for a date that is too recent returns a 400 that names the latest available day [S8, S9].
  - Values may be revised by a few percent over the following days [S9].
- **Cost and usage endpoints:**
  - Data usually arrives within 4 hours (up to 24 hours) and is "typically refreshed every 4 hours" [S8, S9].
  - Values can be revised until about 7 days after month end [S8].
  - Responses include `data_refreshed_at`, which is null when no export covers the range; `data` is then empty [S9]. Pin `ending_at` to `data_refreshed_at` or earlier for stable results [S8, S9].

**Operational limits:**
- A default of **60 requests/min per organization**, across all endpoints of this API [S8].
- **CORRECTED:** S8 states the cursor-binding rule only for **the cost and usage endpoints**: changing a parameter mid-sequence and passing an old cursor returns 400.
- On those same four reports (`usage_report`, `user_usage_report`, `cost_report`, `user_cost_report`), a cursor can **expire with HTTP 410** after a data refresh, and pagination must restart from page 1 [S9].
- **ADDED (CMEK):** with customer-managed encryption keys, `claude_code_metrics.core_metrics.lines_of_code` and `commit_count` can be **zero**, and `pull_request_count` can be lower than actual [S8].

**Endpoints relevant to Claude Code** [S9]:

`GET /v1/organizations/analytics/users`:
- **Parameters:**
  - `date` (YYYY-MM-DD) **or** `starting_date`/`ending_date` (rollup mode). `ending_date` is **exclusive**, at most today, and at most 366 days after `starting_date`.
  - `limit` 1–1000 (default 100); `page`.
  - `filter[]`: dimensions `project_id`, `rbac_group_id`, `user_id`, at most 100 entries.
  - `group_by[]=rbac_group_id`; `order_by`; `order`.
- Rows are sorted by email address by default.
- **Row content:**
  - Each row has `claude_code_metrics` and other per-product metric blocks.
  - **CORRECTED:** `user{type:"user",id,email_address}` is *optional*; it is null on grouped rows.
  - **CORRECTED:** `last_activity_date` is *optional*. It is "omitted from the response while last-activity reporting is not enabled for this organization".
  - Grouped rows add `rbac_group_id`, `rbac_group_name` and `distinct_user_count`.
  - **ADDED:** rows carry **no per-row date**. In single-day mode, the date is the one you requested.
- **`claude_code_metrics.core_metrics` fields:**
  - `commit_count`
  - `distinct_session_count` (number or null)
  - `lines_of_code.added_count` and `lines_of_code.removed_count`
  - `pull_request_count`
  - `artifacts_created_count` (counted from 2026-08-17; 0 on earlier days)
- **`claude_code_metrics.tool_actions`:** `{edit_tool, multi_edit_tool, notebook_edit_tool, write_tool}`, each with `accepted_count` and `rejected_count`.
- **Response and differences from the Admin API:**
  - The response has `data` and `next_page` only, with no `has_more`.
  - Field names differ from the Admin API: `commit_count`, not `commits_by_claude_code`, and so on.
  - This endpoint has **no per-model tokens or cost**.

`GET /v1/organizations/analytics/user_cost_report` and `.../user_usage_report`:
- **ADDED, scope:** "Only cost [usage] attributable to a seat user is included". For org-wide totals, including direct API-key and automation traffic, use the bucketed `/cost_report` and `/usage_report` [S9].
- **Range:**
  - `starting_at`/`ending_at` in RFC 3339; `ending_at` is exclusive.
  - At most 31 days, within the last 365 days, and not before 2026-01-01T00:00:00Z.
  - `ending_at` defaults to the earlier of now and `starting_at` + 31 days.
- **Filtering and grouping:**
  - Filter with `products[]=claude_code`. Note the underscore; the Slack product is `claude-tag`, with a hyphen.
  - Break out with `group_by[]=model`. Also available: `product`, `rbac_group_id`, `cost_type`, `token_type`, `context_window`, `inference_geo`, `speed`, `claude_tag_*`, `slack_channel_id`.
  - Also available: `exclude_deleted_users`, `user_ids[]`, and `order_by`/`order`. `order_by` is `amount`/`list_amount` for cost, and `total_tokens` etc. for usage.
- **`bucket_width`** can be `1d`, `1h` or `1m`, and is optional. When it is omitted, each row aggregates the whole range and `starting_at`/`ending_at` are null. When it is set, `ending_at` is required, and `1m` allows at most 24 hours.
- **`limit`** is 1–1000, **default 20**. Rows fanned out by `cost_type`/`token_type` do not count toward it, so `data` can exceed `limit`.
- **Row fields:**
  - `actor{type:"user_actor", user_id, email_address (nullable: deleted users and service accounts), name (nullable), deleted}`.
  - Cost rows: `amount` and `list_amount` as **decimal strings in cents**, plus `currency`, `requests` (nullable), `starting_at` and `ending_at`.
  - Usage rows: token fields plus `total_tokens`.
- **Top level:** `data_refreshed_at` (nullable), `has_more`, `next_page`, and `organization_id` (`org_...`).

`GET /v1/organizations/analytics/cost_report` and `/usage_report`:
- These return organization-wide totals, including direct API-key and automation traffic [S9].
- **ADDED:** `limit` counts time buckets. `1d`: default 7, max 31. `1h`: default 24, max 168. `1m`: default 60, **max 256**.
- **ADDED:** each bucket reports at most its top 100 groups, with no remainder row, so grouped buckets are not exhaustive [S9].

API version 2026-10-21 removed `actor.email` from the per-user usage and cost reports (use `actor.email_address`), and `summaries` from the activity summaries endpoint (use `data`) [S10, S17].

## 13. Connector algorithm (recommendation)

1. **Config:**
   - Admin key, encrypted at rest with the existing `backend/internal/crypto/aes.go` (verified to exist).
   - Base URL, default `https://api.anthropic.com`.
   - `anthropic-version`, default `2023-06-01`.
   - Backfill days.
2. **Sync run for org O:**
   - For each UTC day D from `max(last_complete_day - 2, today - backfill)` through today (UTC):
     - Request `starting_at=D` with `limit=1000`.
     - Follow `next_page` until `has_more` is false.
     - Collect all rows for D, then replace D's rows in one transaction, so the run is idempotent.
     - If a request fails mid-pagination, discard the partial day and retry D from page 1.
3. **Errors:**
   - 401/403: mark the connector misconfigured and stop. A 401 can mean the key expired.
   - 400: log the body and skip the day.
   - 429: honor `retry-after` (seconds). If the header is absent, use capped backoff, then abort the run.
   - 5xx/529: exponential backoff with jitter and a cap.
   - Log `request-id` in every case [S11, S16].
4. **Privacy (AI Stats principle):**
   - `actor.email_address` is PII. Hash it before storing anything, including in `raw_json`.
   - Verified in the repo: `backend/internal/handlers/ingest.go` hashes emails with `ingest.HashKey`, which is **plain unsalted SHA-256** (`backend/internal/ingest/keys.go`).
   - **ADDED (repo-level recommendation, not from Anthropic docs):** emails are low-entropy, so unsalted SHA-256 can be reversed with a dictionary. Consider HMAC-SHA256 keyed with a server-side secret for new code. Whichever function you pick, use the same one everywhere identities must join (e.g. against GitHub or OTel users).
   - Store metadata and counts only. The API returns no prompts or code [S2].
5. **Rollup into the existing `tool_usage_snapshots` table** (`tool_source='claude_code'`):
   - Verified: the table is created in migration 002, which has `UNIQUE (organization_id, tool_source, date)`. 004 and 007 alter it. The next migration number is 009.
   - This mapping is a suggestion:

   | Column | Value |
   |---|---|
   | `total_active_users` | distinct actors with `num_sessions > 0` |
   | `total_suggestions` | Σ(accepted + rejected) over `tool_actions` |
   | `total_acceptances` | Σ accepted |
   | `total_lines_accepted` | Σ `lines_of_code.added` |

   - Also store cost cents (Σ `estimated_cost.amount`) and per-model tokens in a new per-model or per-actor table, with the cents unit explicit (NUMERIC).
   - Remember to COALESCE nullable SQL aggregates before scanning them into non-pointer Go types.

## Open questions (updated)

1. **Rate limit:** the rate limit for `/usage_report/claude_code` is undocumented; a web search confirmed nothing. Confirm it empirically or with Anthropic support.
2. **Earliest date:** the earliest available date is undocumented. Probe with a real key.
3. **Row split:** what exactly splits rows (`terminal_type`, `is_remote`, `customer_type`)? Until known, aggregate per actor per day.
4. **Fractional cents:** can `estimated_cost.amount` or the count fields ever be non-integer? Decode defensively.
5. **`customer_type='subscription'`:** what do these rows mean, given that S1 says every record is Console-authenticated usage while S2 says "subscription for Pro/Team customers"?
6. **API-key name joins:** `api_actor` names are not documented as unique, so joins to `/api_keys` are best-effort.
7. **`anthropic-version`:** `2023-06-01` and `2026-10-21` should both work for these reports, but neither has been tested live.
8. **NEW: Bearer header.** Does `Authorization: Bearer <admin key>` work for Admin endpoints? S17 says either header works for API keys; it is untested for Admin keys.
9. **NEW: keyless sign-in.** Do keyless Console OAuth sign-ins (Claude Code ≥ v2.1.242) bill into the "Claude Code" workspace? Do they appear in `usage_report/messages` with `account_id` set and `api_key_id` null? Do they appear as `user_actor` in the claude_code report?
10. **NEW: SDK-only endpoints.** The `/v1/organizations/analytics/cost` and `/analytics/usage` endpoints (beta `analytics-usage-cost-2026-08-24`) exist only in SDK v2.2.0+ source. Will they be documented and opened to Admin keys?
11. **Workspace name:** can the "Claude Code" workspace be renamed? Is there a stable marker for it?
12. **Bedrock in Usage & Cost:** S5 does not say whether Bedrock usage appears in the Usage & Cost API. Assume Bedrock and Vertex (Google-managed) spend is absent.

## Verification notes

I fetched all 16 cited sources again. I also read three the spec did not cite: the API 2026-10-21 migration guide, the Claude Code IAM page and the Claude Code costs page. I downloaded and grepped the Go SDK v1.84.0, v2.2.0 and v2.3.0.

High confidence (verified word for word against the primary docs):
- **claude_code endpoint:** the path and method; `starting_at` as YYYY-MM-DD for a single day; `limit` default 20, min 1, max 1000; the `page` cursor and the `has_more`/`next_page` loop; no `ending_at`.
- **claude_code response:** every field name and type, including the `user_actor`/`api_actor` union, `is_remote`, optional `subscription_type` and `tool_actions` as a map. `estimated_cost.amount` is a JSON number in cents. Both verbatim examples match.
- **Freshness, retention and coverage:** the 1-hour freshness, no specified retention, and the exclusions for Bedrock, Foundry, Google Cloud, Claude Platform on AWS and subscription plans.
- **Auth:** the Admin key types and prefixes and who can create them; org:admin OAuth; non-workspace personal and service-account keys; workspace keys rejected.
- **Usage & Cost:** the messages and cost report params, limits and fields.
- **Enterprise Analytics:** the 2026-01-01 floor, the 60 req/min limit, freshness rules, field names and decimal-string cents.
- **Versioning:** the breaking-change list for 2026-10-21.
- **Repo:** the claims about aes.go, email hashing and tool_usage_snapshots.

Errors I found in the original spec:
- It treated SDK v1.84.0 as current, but the v2 module (v2.3.0) is current. The conclusion is unchanged.
- It implied x-api-key is the only header for Admin keys, but the docs now say either header is accepted.
- Its HTTP error list was incomplete.
- It misstated the Claude Code workspace spend limits, and it missed the new keyless Console sign-in that mints no API key.
- It ignored the `workspace_id` null semantics on api_keys under its own recommended default version, 2023-06-01.
- It overgeneralized the Enterprise cursor-binding rule.
- It overstated the /users row fields.
- It omitted the seat-only scope of the per-user reports, the usage-based-plan applicability, the CMEK zeroing and the semantics of account_id.

Still UNVERIFIED, because no source covers them and there is no live key:
- The rate limit for the claude_code endpoint and the earliest available date.
- The exact rule for splitting rows.
- Whether amounts or counts can be non-integer.
- What customer_type='subscription' rows mean.
- Whether Bearer auth works with Admin keys.
- Where keyless OAuth Claude Code traffic is attributed.
- How a timestamp `starting_at` or a future date is rejected.
- Whether the undocumented SDK-only /v1/organizations/analytics/cost and /usage endpoints actually work.

Two third-party integration pages (Elastic, Bruin) were blocked by the network proxy, and GitHub code search was not available in this session.

Corrections made during verification:

- **Claim:** No official Go SDK support: anthropic-sdk-go v1.84.0 (published 2026-10-20) has no code for the Admin report endpoints and appends ?beta=true to the Enterprise analytics paths [S15].  
  **Finding:** The facts about v1.84.0 check out (published 2026-10-20T22:08:37Z; only analytics/* paths, with ?beta=true). The claim is incomplete, though. The current major is the module github.com/anthropics/anthropic-sdk-go/v2: v2.0.0 is the first on API 2026-10-21 per S17, and the latest is v2.3.0, published 2026-10-23. v2.2.0 and v2.3.0 also lack usage_report/claude_code, usage_report/messages and cost_report, so the 'hand-roll net/http' conclusion stands. v2.3.0 no longer appends ?beta=true. v2.2.0+ also adds two endpoints missing from the public docs, /v1/organizations/analytics/cost and /analytics/usage, sent with anthropic-beta: analytics-usage-cost-2026-08-24. The SDK says they accept a read:analytics key 'or an Admin API key' and can group by api_key_id, workspace_id, actor, product and model. These are UNVERIFIED and should not be relied on. (proxy.golang.org anthropic-sdk-go v1.84.0, /v2 v2.2.0 and v2.3.0 zips (grepped); https://platform.claude.com/docs/en/api/migration-2026-10-21)
- **Claim:** The reference examples show Authorization: Bearer $ANTHROPIC_API_KEY, but an Admin API key goes in x-api-key [S3].  
  **Finding:** Misleading. S3 does document Admin keys in x-api-key. But the 2026-10-21 migration guide says SDKs now send keys as Authorization: Bearer and that 'The Claude API accepts a key in either header'. That matches the Bearer examples in every reference page. Bearer with an Admin key therefore probably works; it is UNVERIFIED for Admin endpoints specifically. x-api-key remains the safe, documented choice. (https://platform.claude.com/docs/en/api/migration-2026-10-21; https://platform.claude.com/docs/en/manage-claude/admin-api)
- **Claim:** Error status codes: 400 invalid_request_error, 401, 403, 404, 429, 500 api_error, 529 overloaded_error; honor retry-after on 429.  
  **Finding:** Incomplete. S11 also lists 402 billing_error, 409 conflict_error, 413 request_too_large and 504 timeout_error. It says 404's type is not_found_error, and that invalid_request_error may be used for other unlisted 4xx codes. S16 says retry-after is in seconds and 'Not every 429 carries it'. A usage-tier spend-cap 429 has no retry-after and keeps failing until 00:00 UTC on the first of next month. A 401 can mean an expired key, since keys have expirations. (https://platform.claude.com/docs/en/api/errors; https://platform.claude.com/docs/en/api/rate-limits)
- **Claim:** Claude Code mints a per-user API key in an auto-created 'Claude Code' workspace at sign-in, and that workspace is rate-limited and spend-limited separately [S13].  
  **Finding:** Partly wrong and now incomplete. S13 says Claude Code usage is 'rate-limited separately' and that the workspace 'is the only workspace that supports per-user monthly spend limits'. It does not say the workspace is spend-limited separately. More importantly, since Claude Code v2.1.242 Console sign-in offers a keyless 'Sign in with your Console account (recommended)' path that 'creates no API key'. Per-API-key attribution therefore misses those users. Which workspace keyless traffic bills to is UNVERIFIED. (https://platform.claude.com/docs/en/manage-claude/workspaces; https://code.claude.com/docs/en/iam)
- **Claim:** Use scope.workspace_id: API version 2026-10-21 drops the top-level workspace_id [S10] (while the spec recommends a default anthropic-version of 2023-06-01).  
  **Finding:** Needs correcting for the recommended default version. Under 2023-06-01, api_keys still return workspace_id, but it is null for keys bound to the Default Workspace and for all-workspaces keys. scope.workspace_id carries the real ID even for the Default Workspace. Read scope first and fall back to workspace_id. The spec's field list also omitted type:'api_key' and the status enum (active|archived|expired|inactive). It omitted that created_by and principal are nullable. (https://platform.claude.com/docs/en/api/migration-2026-10-21 (workspace_id on API keys); https://platform.claude.com/docs/en/api/beta/organization (List API Keys))
- **Claim:** Find the Claude Code workspace ID with GET /v1/organizations/workspaces, which returns data[].{id,name,archived_at,...} and paginates with after_id/before_id/limit.  
  **Finding:** Correct but incomplete. The endpoint also has include_archived (default false) and include_default (default false). The Default Workspace is listed only with include_default=true. Response fields also include type, created_at, compartment_id, data_residency, display_color, external_key_id and tags. No documented marker identifies the Claude Code workspace beyond the name 'Claude Code'. (https://platform.claude.com/docs/en/api/beta/organization (List Workspaces); https://platform.claude.com/docs/en/manage-claude/workspaces)
- **Claim:** Enterprise Analytics: cursors are bound to the query that issued them; changing a parameter mid-sequence returns 400 [S8].  
  **Finding:** Overgeneralized. S8 states the rule only for the cost and usage endpoints. HTTP 410 cursor expiry is documented for all four cost/usage reports: usage_report, user_usage_report, cost_report and user_cost_report. (https://platform.claude.com/docs/en/manage-claude/analytics-api; https://platform.claude.com/docs/en/api/beta/organization/analytics)
- **Claim:** /v1/organizations/analytics/users: each row has user{type,id,email_address}, last_activity_date, and claude_code_metrics.  
  **Finding:** Overstated. `user` is optional and null on grouped (group_by[]=rbac_group_id) rows. `last_activity_date` is optional and 'Omitted from the response while last-activity reporting is not enabled for this organization'. Rows carry no per-row date. ending_date is exclusive and at most today, and the default sort is by email address. distinct_user_count, rbac_group_id and rbac_group_name appear on grouped rows. (https://platform.claude.com/docs/en/api/beta/organization/analytics (List User Activity))
- **Claim:** user_cost_report / user_usage_report: range at most 31 days, products[]=claude_code, group_by[]=model, bucket_width 1d/1h/1m, limit 1-1000.  
  **Finding:** Accurate but missing important behavior. Only cost or usage 'attributable to a seat user' is included; org-wide totals including API-key and automation traffic need the bucketed cost_report/usage_report. The default limit is 20. bucket_width is optional: when omitted, rows aggregate the whole range and starting_at/ending_at are null. When it is set, ending_at is required, and 1m allows at most 24h. ending_at defaults to min(now, starting_at+31d). amount is 'post-discount, pre-credit' and list_amount is pre-discount. actor.email_address is null for deleted accounts and service accounts. requests and data_refreshed_at are nullable. organization_id is a tagged 'org_...' value, unlike the bare UUID in the claude_code report. (https://platform.claude.com/docs/en/api/beta/organization/analytics)
- **Claim:** Enterprise Analytics API is the alternative for subscription seats, with cost/usage endpoints giving per-user cost (section 12).  
  **Finding:** Omits plan applicability and a data-quality caveat. 'The cost and usage endpoints apply to usage-based Enterprise plans; for seat-based Enterprise plans, they reflect usage credits only.' The primary owner must also enable public API access, and the Analytics API must be enabled for the org. With customer-managed encryption keys (CMEK), claude_code lines_of_code and commit_count can be zero and pull_request_count can be lower than actual. (https://platform.claude.com/docs/en/manage-claude/analytics-api; https://platform.claude.com/docs/en/manage-claude/admin-api-keys)
- **Claim:** estimated_cost is an estimate: 'Spend figures in the Console dashboard are estimates… For actual costs, refer to your billing page' [S14].  
  **Finding:** Misattributed context. That sentence in S14 describes the Console dashboard's spend figures, not the API field. The API docs only label the field 'Estimated cost' (S1, S2). The practical advice, not billing-grade, still holds. (https://code.claude.com/docs/en/analytics)
- **Claim:** Analytics key created by the primary owner at claude.ai -> Organization settings -> API [S4, S8].  
  **Finding:** S4 and S8 agree (claude.ai/admin-settings/api-access), but S14 gives a different location, claude.ai/analytics/api-keys. The exact UI path is therefore UNVERIFIED. S14 also states the Enterprise Analytics API is not available on the Team plan. (https://code.claude.com/docs/en/analytics; https://platform.claude.com/docs/en/manage-claude/analytics-api)
- **Claim:** usage_report/messages dimensions are each null unless grouped on; workspace_id is null for the default workspace; api_key_id is null for Console playground usage.  
  **Finding:** Incomplete. account_id is also null 'for non-OAuth requests', so it is populated only for OAuth traffic. That makes group_by[]=account_id the likely per-user attribution path for keyless OAuth Claude Code sign-ins (inference, UNVERIFIED). service_account_id is null 'for non-OIDC-federation requests'. Code execution is not in the usage endpoint. (https://platform.claude.com/docs/en/api/beta/organization/usage_report/retrieve_messages; https://platform.claude.com/docs/en/manage-claude/usage-cost-api)
- **Claim:** cost_report amount is a decimal string in cents ('123.45' = $1.23).  
  **Finding:** Verified, but the reference example value is '123.78912', which shows sub-cent precision. Parse it as a decimal and store it as NUMERIC. Separately, the cost_report inference_geo field is documented as 'Always null if not grouping by inference geo', yet inference_geo is not a valid group_by value there, while S5 says grouping by description includes it. That is a doc inconsistency, so treat the field as nullable. (https://platform.claude.com/docs/en/api/beta/organization/cost_report/retrieve; https://platform.claude.com/docs/en/manage-claude/usage-cost-api)
- **Claim:** Enterprise /cost_report and /usage_report give organization-wide totals including direct API-key traffic.  
  **Finding:** Verified, but the spec omits that each time bucket reports at most its top 100 groups with no remainder row, so grouped results are not exhaustive. It also omits that limit counts buckets (1d default 7/max 31, 1h 24/168, 1m 60/max 256). (https://platform.claude.com/docs/en/api/beta/organization/analytics (Get Cost Over Time))
- **Claim:** starting_at must be YYYY-MM-DD; do not send an RFC 3339 timestamp.  
  **Finding:** Consistent with the documented pattern ^\d{4}-\d{2}-\d{2}$. The actual error a timestamp produces, and the behavior for today's or future dates, are not documented (UNVERIFIED). The example cursor 'page_MjAyNS0wNS0xNFQwMDowMDowMFo=' base64-decodes to 2025-05-14T00:00:00Z, unrelated to the request date, so cursors must not be parsed. No cursor expiry (410) is documented for this endpoint. (https://platform.claude.com/docs/en/api/beta/organization/usage_report/retrieve_claude_code)
- **Claim:** Hash actor.email_address 'as the existing ingest handler already hashes emails'.  
  **Finding:** Verified in the repo: backend/internal/handlers/ingest.go uses ingest.HashKey, which is plain unsalted SHA-256 (backend/internal/ingest/keys.go). Unsalted SHA-256 of low-entropy emails can be reversed with a dictionary. This is a repo-level recommendation, not an Anthropic-doc fact: consider HMAC with a server secret for new code. Also verified: crypto/aes.go exists; tool_usage_snapshots is created in migration 002 with UNIQUE(organization_id, tool_source, date) and altered in 004/007; the next migration is 009. (backend/internal/ingest/keys.go; backend/internal/handlers/ingest.go; backend/internal/db/migrations/)
