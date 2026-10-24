# GitHub Copilot Usage Metrics API: implementation spec for AI Stats (independently verified)

> **Status:** implementation spec, researched and independently re-verified on 2026-10-23 against the
> primary sources cited inline. Claims that could not be confirmed are marked UNVERIFIED. Re-check
> anything load-bearing against the live API before relying on it; vendor APIs change.

Re-verified 2026-10-23 against freshly downloaded primary sources. Today `backend/internal/github/copilot.go` calls `GET /orgs/{org}/copilot/usage?per_page=28`. That path is gone from every current GitHub OpenAPI description (latest, 2022-11-28, 2026-03-10 and GHEC). So are the legacy `/orgs/{org}/copilot/metrics` and `/orgs/{org}/team/{team_slug}/copilot/metrics` [S1]. HIGH.

Note: go-github still exposes legacy methods such as `GetOrganizationMetrics` (`/orgs/{org}/copilot/metrics`) [S19]. Do not use them. Its newer `Get*MetricsReport` methods wrap the endpoints below.

Markers used in this document:
- **CORRECTED** marks a statement that differs from the reviewed draft.
- **UNVERIFIED** marks a claim that no source reachable from this sandbox confirms.

## Source legend

**Primary sources (fetched and read directly on 2026-10-23):**
- **[S1]** GitHub OpenAPI, latest: https://raw.githubusercontent.com/github/rest-api-description/main/descriptions/api.github.com/api.github.com.json
  - The `.2022-11-28.json` and `.2026-03-10.json` variants have JSON-identical definitions for every `/copilot/metrics/reports/*` path and both report schemas.
  - **[S1-ghec]** https://raw.githubusercontent.com/github/rest-api-description/main/descriptions/ghec/ghec.json. Its definitions match except for doc URLs.
- **[S2]** Field reference: https://raw.githubusercontent.com/github/docs/main/content/copilot/reference/copilot-usage-metrics/copilot-usage-metrics.md
- **[S3]** Example schema: .../content/copilot/reference/copilot-usage-metrics/example-schema.md
- **[S4]** Team-level metrics: .../content/copilot/reference/copilot-usage-metrics/team-level-metrics.md
- **[S5]** Concepts: .../content/copilot/concepts/billing-and-usage/copilot-usage-metrics/copilot-metrics.md
- **[S6]** Reconciling: .../content/copilot/reference/copilot-usage-metrics/reconciling-usage-metrics.md
- **[S7]** REST intro: .../content/rest/copilot/copilot-usage-metrics.md
- **[S8]** Permission metadata, under `.../src/github-apps/data/`:
  - `fpt-2022-11-28/server-to-server-permissions.json`
  - `fpt-2022-11-28/fine-grained-pat-permissions.json`
  - `ghec-2022-11-28/server-to-server-permissions.json`
  - `ghec-2022-11-28/fine-grained-pat-permissions.json`
- **[S9]** Billing tutorial: .../content/billing/tutorials/automate-usage-reporting.md
- **[S10]** Billing usage REST intro: .../content/rest/billing/usage.md
- **[S11]** SKU names: .../content/billing/reference/product-and-sku-names.md
- **[S12]** OAuth scopes: .../content/apps/oauth-apps/building-oauth-apps/scopes-for-oauth-apps.md
- **[S13]** `.../data/reusables/copilot/usage-metrics-permissions.md`
- **[S14]** `.../data/reusables/copilot/copilot-metrics-dashboard-prereq.md`
- **[S20]** `.../content/copilot/how-tos/administer-copilot/view-usage-and-adoption.md`
- **[S21]** API versions:
  - `.../content/rest/about-the-rest-api/api-versions.md`
  - `.../data/tables/rest-api-versions.yml`
  - `.../src/rest/lib/config.json`
- **[S22]** GHES 3.22 OpenAPI: `.../rest-api-description/main/descriptions/ghes-3.22/ghes-3.22.json`
- **[S23]** AI Credits billing concept:
  - `.../content/copilot/concepts/billing-and-usage/organizations-and-enterprises/billing.md`
  - `.../data/variables/product.yml` (`prodname_ai_credits_value: '$0.01 USD'`)
  - `.../data/variables/copilot.yml`
- **[S24]** OAuth app access restrictions:
  - `.../content/organizations/managing-oauth-access-to-your-organizations-data/about-oauth-app-access-restrictions.md`
  - `.../data/reusables/organizations/oauth_app_restrictions_default.md`
- **[S25]** Billing reports reference: `.../content/billing/reference/billing-reports.md`
- **[S26]** `.../content/copilot/reference/metrics-data.md` (`last_activity_at`)

**Open-source consumers (read directly):**
- [S15] copilot-metrics-viewer README
- [S16] copilot-metrics-viewer `server/services/github-copilot-usage-api.ts`
- [S17] Apache DevLake `report_download_helper.go`
- [S18] Apache DevLake `user_metrics_collector.go`
- [S19] go-github `github/copilot.go`

**Changelog posts.** github.blog is still unreachable from this sandbox (proxy 403 and DNS failure, re-tested). Everything below comes from WebSearch result summaries only:
- [C1] 2025-02-21-deprecation-of-beta-github-copilot-usage-api-endpoint
- [C2] 2026-01-29-closing-down-notice-of-legacy-copilot-metrics-apis
- [C3] 2026-02-27-copilot-metrics-is-now-generally-available
- [C4] 2026-02-25-org-level-metrics-api-now-includes-pull-request-throughput-metric-parity
- [C5] 2026-07-17-repository-level-github-copilot-usage-metrics-generally-available
- [C6] 2026-07-17-github-copilot-app-now-available-in-the-usage-metrics-api
- [C7] changelog/2026/04/27/github-copilot-is-moving-to-usage-based-billing
- [C8] 2026-07-02-improved-accuracy-and-coverage-in-copilot-usage-metrics-reports
- [C9] 2026-06-19-ai-credits-consumed-per-user-now-in-the-copilot-usage-metrics-api

**Confidence scale:**
- HIGH: read verbatim in the OpenAPI or github/docs source.
- MEDIUM: from OSS code or comments, or inferred.
- LOW: from a search summary only.
- UNVERIFIED: no reachable source confirms it.

---

## 1. Deprecation timeline

| Item | Status | Source | Confidence |
|---|---|---|---|
| `GET /orgs/{org}/copilot/usage` (beta) | Deprecated starting Feb 2025. No new data written after that; reachable through Feb 2025 within its 28-day retention. | [C1] | **LOW** (CORRECTED from MEDIUM: summary only) |
| Consequence for AI Stats | The current sync has most likely returned no data or 404 since early 2025. | inference | MEDIUM |
| `/copilot/usage`, `/copilot/metrics`, `/team/{slug}/copilot/metrics` | Absent from all current OpenAPI descriptions, including GHEC | [S1] | HIGH |
| `/rest/copilot/copilot-usage` and `/rest/copilot/copilot-metrics` docs URLs | `redirect_from` the "Copilot usage metrics" REST page | [S7] | HIGH |
| Legacy "Copilot Metrics API" | Shut down 2026-04-02 | [S15], [C2] | **MEDIUM** (CORRECTED from HIGH: OSS README plus a search summary) |
| User-level Feature Engagement API and Direct Data Access API | Sunset 2026-03-02 | [C2] | LOW |
| Copilot usage metrics | GA 2026-02-27, including org-level visibility | [C3] | LOW |
| Org-level PR metrics | Added 2026-02-25. Enterprise-level PR metrics came 2026-02-19. | [C4] | LOW |
| `repos-1-day` | GA 2026-07-17 | [C5] | LOW |
| `totals_by_copilot_app` | Added 2026-07-17 | [C6] | LOW |
| `ai_credits_used` in per-user reports | Added 2026-06-19, in both users-1-day and users-28-day | [S15], [S16], [C9] | MEDIUM |
| `ai_credits_used` attribution fix | 2026-07-02: earlier values were understated (some users showed 0.0) | [C8] | LOW |
| Copilot billing unit | AI Credits are now the billing unit for Business and Enterprise | [S23] | HIGH (current state) |
| Switch date to AI Credits | 2026-06-01. Annual Pro/Pro+ stay on premium requests until expiry. | [C7] | LOW (date) |

GitHub "strongly recommend[s] using the Copilot usage metrics API for new integrations and analyses". It calls it "the primary API resource being actively developed and maintained" [S5]. HIGH.

---

## 2. Endpoints: Copilot usage metrics reports

All endpoints are `GET` and return JSON containing **download links**, not inline data. Paths, operationIds, parameters and response codes below were re-read from [S1]. HIGH.

### 2.1 Organization scope (what AI Stats needs)

| Report | Path | Query | Documented responses | operationId |
|---|---|---|---|---|
| Org aggregate, 1 day | `/orgs/{org}/copilot/metrics/reports/organization-1-day` | `day` (required) | 200, 204, 403, 404, 500 | `copilot/copilot-organization-one-day-usage-metrics` |
| Org aggregate, latest 28 days | `/orgs/{org}/copilot/metrics/reports/organization-28-day/latest` | none | 200, 403, 404, 500 | `copilot/copilot-organization-usage-metrics` |
| Per-user, 1 day | `/orgs/{org}/copilot/metrics/reports/users-1-day` | `day` (required) | 200, 204, 403, 404, 500 | `copilot/copilot-organization-users-one-day-usage-metrics` |
| Per-user, latest 28 days | `/orgs/{org}/copilot/metrics/reports/users-28-day/latest` | none | 200, 403, 404, 500 | `copilot/copilot-organization-users-usage-metrics` |
| Per-repository, 1 day | `/orgs/{org}/copilot/metrics/reports/repos-1-day` | `day` (required) | 200, 204, 403, 404, 500 | `copilot/copilot-organization-repos-one-day-report` |
| User-to-team mapping, 1 day | `/orgs/{org}/copilot/metrics/reports/user-teams-1-day` | `day` (required) | 200, 204, 403, 404, 500 | `copilot/copilot-organization-user-teams-one-day-report` |

### 2.2 Enterprise scope

All paths are under `/enterprises/{enterprise}/copilot/metrics/reports/`. Only `repos-1-day` documents a 204.

| Path suffix | operationId |
|---|---|
| `enterprise-1-day?day=` | `copilot/copilot-enterprise-one-day-usage-metrics` |
| `enterprise-28-day/latest` | `copilot/copilot-enterprise-usage-metrics` |
| `users-1-day?day=` | `copilot/copilot-users-one-day-usage-metrics` |
| `users-28-day/latest` | `copilot/copilot-users-usage-metrics` |
| `repos-1-day?day=` | `copilot/copilot-enterprise-repos-one-day-report` |
| `user-teams-1-day?day=` | `copilot/copilot-enterprise-user-teams-one-day-report` |

`{enterprise}` is the enterprise slug [S1]. HIGH.

**Do not use (ADDED):** `GET /enterprises/{enterprise}/copilot/usage-records` [S1-ghec]. HIGH.
- Public preview, EMU enterprise owners only, `read:enterprise`, cursor pagination with `per_page` up to 25.
- It returns raw request and response `body` strings, which means prompts and completions.
- That breaks AI Stats' "never store prompts or code" principle.

### 2.3 Parameters

| Name | In | Type | Required | Notes |
|---|---|---|---|---|
| `org` | path | string | yes | Organization name, not case sensitive [S1]. HIGH. |
| `enterprise` | path | string | yes | Slug version of the enterprise name [S1]. HIGH. |
| `day` | query | string (`format: date`) | yes on `*-1-day` | `YYYY-MM-DD`, example `2025-10-13` [S1]. HIGH. Days are UTC calendar days [S5]. |

The response to future dates, or to dates outside the retention window, is not documented. **UNVERIFIED.** Treat any 4xx other than 403 as "no data for that day" and do not retry forever.

### 2.4 Other request details

- **Pagination:** none on these endpoints; the only inputs are the path and `day` [S1]. HIGH.
- **Team endpoints:** none. Build team metrics by joining `user-teams-1-day` with `users-1-day` [S4]. HIGH.
- **API version header:** definitions are identical in `2022-11-28` and `2026-03-10` [S1].
  - AI Stats' current `X-GitHub-Api-Version: 2022-11-28` works. HIGH.
  - **ADDED:** `2022-11-28` reaches end of support on **March 10, 2028**. `2026-03-10` has no end date yet [S21]. HIGH.
  - An unsupported version returns `410 Gone` [S21]. HIGH.
  - copilot-metrics-viewer pins `2026-03-10` [S16].
- **Headers:** send `Accept: application/vnd.github+json` and `Authorization: Bearer <token>`.
- **GHES:** not supported. **CORRECTED to HIGH:** the GHES 3.22 OpenAPI contains no Copilot paths at all [S22], and [S15] says the same.
- **GHE.com (data residency):** override the API base URL, for example `https://api.SUBDOMAIN.ghe.com` [S15], [S16]. MEDIUM.
- **Rate limits:** a 365-day backfill of 3 report types is about 1,100 API calls. That is within the normal 5,000/hour for OAuth tokens. Whether these endpoints have special limits is **UNVERIFIED**. Downloading signed URLs does not count against the GitHub API limit (MEDIUM, inference).

### 2.5 Step 1 response: download links (JSON)

**1-day endpoints** use schema `copilot-usage-metrics-1-day-report` [S1]. HIGH.
- `download_links`: `string[]` (`format: uri`), required.
- `report_day`: `string` (date), required.
- OpenAPI example: `{"download_links":["https://example.com/copilot-usage-report-1.ndjson","https://example.com/copilot-usage-report-2.ndjson"],"report_day":"2025-07-01"}`.

**28-day endpoints** use schema `copilot-usage-metrics-28-day-report` [S1]. HIGH.
- `download_links`: `string[]`, required.
- `report_start_day` and `report_end_day`: date, required.

**Download links:**
- They are "signed URLs with a limited expiration time" [S1]. HIGH.
- "Download the file at each URL within the link's expiration window" [S4]. HIGH.
- The TTL and host are **UNVERIFIED** (not documented anywhere). Download right away and re-request links if a download fails.
- There may be **more than one link**. Download all of them [S1], [S16]. HIGH.
- **ADDED:** copilot-metrics-viewer reports that large orgs get files **time-partitioned**, so the same user appears in several files of a 28-day report. It also says a single day can be sharded across files. It re-aggregates per user to avoid duplicates (its issue #398) [S16]. MEDIUM.
  - Key rows by `(user_id, day)` and log duplicates.
  - Whether 1-day shards can contain duplicate `(user_id, day)` rows is **UNVERIFIED**.

### 2.6 Step 2: downloading the report file

- Use a plain `GET` **without** the GitHub `Authorization` header. The signature is in the URL.
  - DevLake uses bare `http.Get(url)` [S17]. copilot-metrics-viewer's `downloadReport` sends no headers [S16]. MEDIUM-HIGH.
  - Never forward the GitHub token to the blob host.
  - Use a dedicated `http.Client` with a timeout. Today's code uses `http.DefaultClient`, which has no timeout.
- **ADDED:** a download can return 404. DevLake logs "Report blob not found" and skips it [S17]. MEDIUM. The behaviour on an expired signature (likely 403) is **UNVERIFIED**. Re-request links once, then fail.
- Format: newline-delimited JSON (NDJSON) [S4] (HIGH), consistent with [S17], [S18].

**File shapes:**

| Report | File contents | Source and confidence |
|---|---|---|
| `organization-1-day`, `enterprise-1-day` | One aggregated record per org or enterprise for the day | [S2] HIGH. The viewer wraps it as `{day_totals:[record]}` [S16]. |
| `organization-28-day/latest` | One wrapper object `{report_start_day, report_end_day, created_at, organization_id, enterprise_id, copilot_feature_engagement?, day_totals:[...]}` | [S2], [S3] HIGH. **CORRECTED:** `created_at` is missing from the [S3] example; treat it as optional. |
| `users-1-day` | One row per `(user_id, day, organization_id)` | [S4] HIGH |
| `users-28-day/latest` | **Conflict, UNVERIFIED.** [S2] says one record per user for the period, plus `report_start_day` and `report_end_day`. [S16] code and comments say one row per user **per active day**, each carrying both window fields. Use `users-1-day`. | |
| `user-teams-1-day` | One row per `(user, team)` pair | [S4] HIGH |
| `repos-1-day` | One row per repository with PR activity that day | [S1], [S2] HIGH |

**Robust Go parsing:**
- Wrap the body in a `json.Decoder` and call `Decode` until `io.EOF`. This handles NDJSON, a single object or a pretty-printed object.
- If the first non-space byte is `[`, decode as an array. The [S3] examples render rows as JSON arrays.

### 2.7 Status codes and edge cases

| Response | Meaning | Source | Confidence |
|---|---|---|---|
| `204 No Content` | Documented on the org `*-1-day` endpoints and enterprise `repos-1-day` ("A header with no content is returned") | [S1] | HIGH |
| `404` | Documented as "Resource not found". DevLake also sees it when the day's report is not generated yet, and treats it as skip-and-retry. | [S1], [S17] | HIGH that it is documented; MEDIUM on meaning |
| 204 for small orgs | DevLake comment: returned when "the org had fewer than 5 active Copilot users on that day". GitHub docs document a fewer-than-5 rule only for teams in user-teams reports. | [S17] | **UNVERIFIED** |
| `200` with body `""` or `null` | DevLake: returned "for dates before Copilot usage data was available". Treat as empty. | [S17] | MEDIUM |
| Empty `download_links` | Treat as no data | [S17], [S16] | MEDIUM |
| `403` | Documented. Usually a missing role, scope or permission, policy disabled, or OAuth app restriction (§3). | [S1] | HIGH documented; MEDIUM on causes |
| `500` | Documented | [S1] | HIGH |
| `401` | Bad or expired token. General GitHub behaviour; not listed on these operations. | — | **UNVERIFIED** for these endpoints |
| `410 Gone` | Unsupported `X-GitHub-Api-Version` | [S21] | HIGH |
| 400 or 403 from missing GitHub App permissions | Reported by the viewer | [S15] | MEDIUM |

**ADDED.** To tell a "report not ready" 404 from an "org or access" 404, call `organization-28-day/latest` as a health check. If it also returns 403 or 404, the problem is access, not timing. MEDIUM (inference).

---

## 3. Authentication and authorization

### 3.1 Who can call the metrics endpoints

**Org endpoints** [S1]. HIGH.
- Callers: "Organization owners and authorized users with fine-grained 'View Organization Copilot Metrics' permission".
- "OAuth app tokens and personal access tokens (classic) need the `read:org` scope."

**Enterprise endpoints** [S1]. HIGH.
- Callers: "Enterprise owners, billing managers, and authorized users with fine-grained 'View Enterprise Copilot Metrics' permission".
- Classic and OAuth tokens need "either the `manage_billing:copilot` or `read:enterprise`" scope.

**Custom roles.** An org or enterprise custom role carrying "View Organization/Enterprise Copilot Metrics" grants access [S13], [S5]. HIGH.

**Policy prerequisite.** The "Copilot usage metrics" policy must be enabled [S14]. The REST page says it must be "Enabled everywhere" for the enterprise [S7]. HIGH.
- [S14] links both the enterprise policy page and the org page (`manage-policies#enabling-copilot-features-and-models-in-your-organization`).
- The org page does not name the policy. Its exact org-level UI label for orgs without an enterprise is **UNVERIFIED**.

**Implication for AI Stats.** The existing OAuth scopes `read:user user:email read:org` (in `backend/internal/auth/oauth.go`) satisfy the documented **scope** requirement for all six org report endpoints. HIGH. These conditions still apply:
- The signed-in user must be an org owner or hold the custom role.
- The policy must be enabled.
- **OAuth app access restrictions (UPGRADED from LOW-MEDIUM):**
  - These restrictions are "enabled by default" for new organizations.
  - Unapproved OAuth apps lose API access to private organization resources until an owner approves them [S24]. HIGH.
  - Whether Copilot metrics count as "private organization resources" is **UNVERIFIED** but likely.
  - Because the caller must already be an owner, they can approve the AI Stats app.

The current 403 message in `copilot.go` mentions a `copilot_usage_data:read` scope. No such scope exists in [S12] (HIGH), so replace the message. The current 404 message ("organization not found or Copilot not enabled") is also misleading now that the endpoint itself is gone.

### 3.2 Fine-grained PAT and GitHub App permissions

From [S8], re-read. HIGH unless marked otherwise.

| Endpoint group | Permission key | UI title | Access | App user-to-server | App installation token | Fine-grained PAT |
|---|---|---|---|---|---|---|
| `/orgs/{org}/copilot/metrics/reports/*` (all 6) | `organization_copilot_metrics` | "Organization Copilot metrics" | read | yes | yes | yes |
| `/enterprises/{e}/copilot/metrics/reports/*` (all 6) | `enterprise_copilot_metrics` | "Enterprise Copilot metrics" | read | yes | yes | no (absent from both FG-PAT files) |
| `/organizations/{org}/settings/billing/{ai_credit,premium_request}/usage`, `/usage`, `/usage/summary` | `organization_administration` | "Administration" | read | yes | yes per [S8] (**conflicts** with [S15], which says GitHub Apps "cannot read billing today": **UNVERIFIED**) | listed in metadata, **but see §6.3** |
| `/orgs/{org}/copilot/billing`, `/orgs/{org}/copilot/billing/seats` | `organization_copilot_seat_management` or `organization_administration` (`additional-permissions: true`) | "GitHub Copilot Business" or "Administration" | read | yes | yes | yes |
| `/enterprises/{e}/settings/billing/{ai_credit,premium_request}/usage`, `/usage`, `/usage/summary` | `enterprise_billing` | "Enterprise billing" | read | yes | yes | no |
| `/users/{username}/settings/billing/*` | `plan` | "Plan" | read | yes | **no** | yes |

**ADDED.** The `/organizations/{org}/settings/billing/*` endpoints are **absent** from both the GHEC OpenAPI and the GHEC permission files. They appear only in the fpt (api.github.com) variants. HIGH. This supports, but does not prove, the report in §6.3 that they 404 for enterprise-owned orgs.

**What copilot-metrics-viewer asks for** [S15]. MEDIUM.
- Its GitHub App requests "Organization Copilot metrics: Read".
- **CORRECTED:** its README says the new API "requires" members read, Organization Copilot metrics and Copilot seat management, and that otherwise endpoints return 400 or 403.
- [S8] lists only "Organization Copilot metrics" for the report endpoints. The other two are probably needed for the viewer's own team and seat features (inference).

---

## 4. Report record schemas (NDJSON contents)

Types are JSON Schema types. "Null" means `null` or absent. Everything is HIGH from [S2], with examples in [S3], unless marked otherwise.

- Activity breakdown arrays "are always present but can be empty (`[]`)". Optional arrays are marked nullable and can be absent [S2].
- **ADDED:** decode defensively anyway. Several fields that [S2] calls non-nullable are missing from the [S3] examples (§4.3, §4.6).

### 4.1 Identification fields

| Field | Type | Null | Notes |
|---|---|---|---|
| `day` | string `YYYY-MM-DD` | no | Inside each `day_totals[]` entry in 28-day aggregates |
| `enterprise_id` | string | no per [S2] | Can be `""` for orgs not owned by an enterprise. [S3] prose says org-scoped repo rows include it "only when an enterprise owns the organization". **CORRECTED:** treat absent and `""` the same. |
| `organization_id` | string | yes | Org-scope reports only. Also present in enterprise-scoped `repos-1-day` rows. |

IDs in report rows are **strings**, for example `"organization_id": "999"` and `"agent_id": "2246796"`. `user_id`, `team_id` and `repo_id` are integers [S2], [S3], [S4].

### 4.2 Org/enterprise aggregated day record

This is the `organization-1-day` record and each `organization-28-day.day_totals[]` entry. It is verified unchanged from the draft.

**Active-user counts.** Non-nullable integers:
- `daily_active_users`, `weekly_active_users`, `monthly_active_users`
- `monthly_active_chat_users`, `monthly_active_agent_users`
- `{daily,weekly,monthly}_active_copilot_cloud_agent_users`
- `{daily,weekly,monthly}_active_copilot_code_review_users`
- `{daily,weekly,monthly}_passive_copilot_code_review_users`

**Active-user counts.** Nullable integers:
- `daily_active_cli_users`: omitted when there is no CLI use; independent of IDE counts.
- `daily_active_copilot_app_users`.
- `daily_active_vscode_agent_users`.
- `weekly_active_vscode_agent_users`: Sunday through the report day.
- `monthly_active_vscode_agent_users`: first of the month through the report day.

**Windows:**
- `weekly_*` is a trailing 7 days and `monthly_*` a trailing 28 days.
- The vscode-agent weekly and monthly counts are calendar-to-date instead.
- Active-user counts include users seen only through server-side telemetry.

**Activity totals.** Integers, not null: `user_initiated_interaction_count`, `code_generation_activity_count`, `code_acceptance_activity_count`, `loc_suggested_to_add_sum`, `loc_suggested_to_delete_sum`, `loc_added_sum`, `loc_deleted_sum`.

**Breakdowns:**
- Arrays, not null: `totals_by_ide`, `totals_by_feature`, `totals_by_language_feature`, `totals_by_language_model`, `totals_by_model_feature`.
- Nullable: `totals_by_cli` (object), `totals_by_copilot_app` (object), `totals_by_vscode_agent` (object), `totals_by_3rd_party_agent` (array), `totals_by_ai_adoption_phase` (array).
- `pull_requests`: object, **not** null.

**Customization fields** (all nullable):
- Integers: `distinct_{skill,custom_agent,mcp,slash_cmd,plugin}_use_count`.
- Arrays: `totals_by_{skill,custom_agent,mcp,slash_cmd,plugin}`.

Aggregated records have no `user_id`, `user_login` or `used_*` [S2].

### 4.3 28-day aggregate wrapper

| Field | Type | Null |
|---|---|---|
| `report_start_day` | string | no |
| `report_end_day` | string | no |
| `created_at` | string (ISO 8601) | no per [S2], **but absent in the [S3] example: treat as optional** |
| `organization_id` / `enterprise_id` | string | see §4.1 |
| `copilot_feature_engagement` | object | yes (null or absent when unavailable or the report predates it) |
| `day_totals` | array of §4.2 records | no |

`copilot_feature_engagement`:
- `active_user_count`: integer.
- `totals_by_feature[]`: entries of `{feature, engaged_user_count}`.
- Features are exactly `code_completion`, `agent_edit`, `code_review_passive`, `code_review_active`, `cloud_agent`, `copilot_cli` and `github_app`. Here `github_app` means the Copilot app; elsewhere that value is `copilot_app`.
- Never sum `engaged_user_count` values; users overlap across features.

### 4.4 Per-user record (`users-1-day`)

Per-user rows have no active-user counts, no `pull_requests` and no `totals_by_ai_adoption_phase` [S2].

| Field | Type | Null | Notes |
|---|---|---|---|
| `day`, `enterprise_id`, `organization_id` | string | §4.1 | |
| `report_start_day`, `report_end_day` | string | | 28-day per-user only |
| `user_id` | integer | no | |
| `user_login` | string | no | |
| `ai_credits_used` | **number** | no per docs | See below |
| 7 activity counters (as §4.2) | integer | no | |
| `used_agent`, `used_chat`, `used_cli`, `used_copilot_app` | boolean | no | |
| `used_copilot_coding_agent`, `used_copilot_cloud_agent` | boolean | no | Always the same value |
| `used_copilot_code_review_active`, `used_copilot_code_review_passive` | boolean | **yes** | `null` when there is no signal |
| `used_vscode_agent` | boolean | yes | |
| `ai_adoption_phase` | object | no | `{phase, phase_number, version}`. Defaults to "No Cohort"/0; `version` example `v1`. |
| `distinct_*_use_count` (5) | integer | yes | |
| `totals_by_{skill,custom_agent,mcp,slash_cmd,plugin}` | array | yes | |
| `totals_by_cli` | object | yes | Per-user form includes `last_known_cli_version` |
| `totals_by_copilot_app`, `totals_by_vscode_agent` | object | yes | |
| `totals_by_3rd_party_agent` | array | yes | Per-user entries have no `session_count` |
| `totals_by_ide`, `totals_by_feature`, `totals_by_language_feature`, `totals_by_language_model`, `totals_by_model_feature` | array | no | |

**About `ai_credits_used`:**
- Docs: "Total AI credits consumed by the user in the reporting period". Not broken down by feature, model or surface; "for consumption analysis, not invoicing totals" [S2].
- [S16] types it optional because it is absent on older reports.
- [C9] (LOW) says it is per user per day and comes from the same data as the billing API.

### 4.5 Breakdown objects

**Dimension fields:**
- `totals_by_ide[]`: `ide`.
- `totals_by_feature[]`: `feature`.
- `totals_by_language_feature[]`: `language` and `feature`. It has **no** `user_initiated_interaction_count`.
- `totals_by_language_model[]`: `language` and `model`, chat only.
- `totals_by_model_feature[]`: `model` and `feature`, chat only.

**Metric fields** in each entry are the 7 counters from §4.2. HIGH.

**Per-user `totals_by_ide[]` extras:**
- `last_known_ide_version`: `{ide_version, sampled_at|null}`, nullable.
- `last_known_plugin_version`: `{plugin, plugin_version, sampled_at|null}`, nullable.
- Both are omitted on the capped "others" bucket.
- [S3] shows `plugin` and `plugin_version` as `""`.

**Nested objects:**
- `totals_by_cli`: `{session_count, request_count, prompt_count, token_usage:{output_tokens_sum, prompt_tokens_sum, avg_tokens_per_request (number|null)}}`. The per-user form adds `last_known_cli_version:{cli_version (default "unknown"), sampled_at}`.
  - **Note:** these are CLI token totals; IDE token counts are not exposed.
- `totals_by_copilot_app`: the same shape with no version field. App activity is **also** counted in the `copilot_app` feature rows and in top-level counters, unlike CLI.
- `totals_by_vscode_agent`: `{session_count, total_user_messages}`.
- `totals_by_3rd_party_agent[]`: `{agent_id (string), agent_name (string), user_initiated_interaction_count (integer), session_count (integer, aggregate only)}`.
  - Group by `agent_id`; `agent_name` can change. Examples: "Claude (Anthropic)", "Codex (OpenAI)".
  - The nested `user_initiated_interaction_count` counts **job starts**. Do not reconcile it with the top-level field [S6].
- Customization arrays: `{skill|custom_agent|mcp|slash_cmd|plugin: string, interaction_count: integer}`.
  - Up to 25 named entries, ordered by count, plus a trailing `other`. Slash commands can also include a separate `custom` bucket.
  - Counts are events, not users. Plugin interactions are a subset of skill interactions; do not add them together.

**Dimension values:**
- `feature`: `code_completion`, `chat_inline`, `chat_panel_ask_mode`, `chat_panel_edit_mode`, `chat_panel_agent_mode`, `chat_panel_plan_mode`, `chat_panel_custom_mode`, `chat_panel_unknown_mode`, `agent_edit`, `copilot_cli`, `copilot_app`, `others`.
- `ide` (non-exhaustive): `vscode`, `visualstudio`, `intellij`, `eclipse`, `xcode`, `neovim`, `vim`, `emacs`, `zed`.
- `model`: specific identifiers such as `gpt-5.4` or `claude-sonnet-4.6`, plus `auto`, `unknown`, `others`.
- `language`: free-form; `unknown` is possible.
- **ADDED:** [S6] writes the bucket as `Unknown` while [S2] and [S3] use `unknown`. Compare case-insensitively.

### 4.6 `pull_requests` object (aggregate and repo reports)

**Integers:**
- `total_created`, `total_reviewed`, `total_merged`
- `total_suggestions`, `total_applied_suggestions`
- `total_created_by_copilot`, `total_reviewed_by_copilot`
- `total_merged_created_by_copilot`, `total_merged_reviewed_by_copilot`
- `total_copilot_suggestions`, `total_copilot_applied_suggestions`

**Nullable numbers:**
- `median_minutes_to_merge`
- `median_minutes_to_merge_copilot_authored`
- `median_minutes_to_merge_copilot_reviewed`

**Array:** `copilot_suggestions_by_comment_type[]` of `{comment_type, total_copilot_suggestions, total_copilot_applied_suggestions}`.
- **CORRECTED:** [S2] says it is "Always present", but it is missing from both aggregated records in [S3]. Treat it as optional.

**Notes:**
- Medians may be **absent**, not only null, when nothing merged. See the second enterprise `day_totals` record and the repo rows in [S3].
- **ADDED:** applied counts can exceed suggestion counts on the same day. [S3] has a repo row with `total_copilot_suggestions: 0` and `total_copilot_applied_suggestions: 1`. Do not assume a daily ratio is at most 1.
- Org and enterprise totals differ because of deduplication and attribution timing [S2], [S5].

### 4.7 `totals_by_ai_adoption_phase[]` (aggregate only)

| Field | Type | Null |
|---|---|---|
| `phase` | string | no |
| `phase_number` | integer | no |
| `total_engaged_users` | integer | no |
| `users_in_phase_28d` | integer | yes |
| `total_pull_requests_merged` | integer | no |

The `avg_*` fields are all numbers and not null: `avg_user_initiated_interactions`, `avg_code_generation_activities`, `avg_code_acceptance_activities`, `avg_loc_added`, `avg_loc_deleted`, `avg_pull_requests_reviewed`, `avg_pull_requests_created`, `avg_pull_requests_merged`, `avg_pull_requests_median_minutes_to_merge`, `avg_pull_requests_minutes_to_review`, `avg_pull_requests_review_cycles`.
- Despite their names, the last two are documented as **medians**.

Phase values are `No Cohort`, `Phase 1`, `Phase 2` and `Phase 3`.
- For rolling rates, use `users_in_phase_28d` as the denominator, not `total_engaged_users`.

### 4.8 `user-teams-1-day` row

Fields: `user_id` (int), `user_login`, `day`, `organization_id` (org scope), `enterprise_id` (enterprise scope), `team_id` (int), `slug`. HIGH.

How to use it [S2], [S4]:
- Teams with fewer than 5 seated Copilot users on a given day are excluded.
- Inner-join to `users-1-day` on `(user_id, day, organization_id)`.
- Never join to the 28-day per-user report.
- Use `COUNT(DISTINCT user_id)` over the window rather than summing daily counts.
- Multi-team users count in each of their teams.
- Org scope includes only org teams. Enterprise scope includes enterprise and business teams.

### 4.9 `repos-1-day` row

Fields:
- `day`, `enterprise_id`, `organization_id`.
- `repo_id` (int), `repo_owner_name`, `repo_name`.
- `repo_visibility`: `PRIVATE`, `INTERNAL` or `PUBLIC`.
- `pull_requests` (§4.6).
- `pull_request_review_times[]`: array, not nullable.

Each `pull_request_review_times[]` entry:
- `authored_by` and `reviewed_by`: currently always `"human"`.
- `total_merged`: integer.
- `median_*` and `p90_*` variants of `minutes_ready_to_first_review`, `minutes_first_to_final_review` and `minutes_final_review_to_merge`: numbers, not null, rounded to 2 decimals.

**CORRECTED:**
- PRs that became ready for review **before 2026-09-21** are excluded from this array but still count in `pull_requests.total_merged`.
- The data is not backfilled.
- `[]` means no qualifying data, not zero minutes.
- Bot-authored PRs (including Copilot cloud agent) and PRs without human review are excluded [S2].

### 4.10 Go types sketch (CORRECTED)

The draft's sketch declared several fields under one json tag and left snake_case fields untagged. With `encoding/json`, fields that share one name are dropped, and `EnterpriseID` would not match `enterprise_id`. Every field needs its own tag:

```go
type ReportLinks struct {
    DownloadLinks  []string `json:"download_links"`
    ReportDay      string   `json:"report_day,omitempty"`       // 1-day
    ReportStartDay string   `json:"report_start_day,omitempty"` // 28-day
    ReportEndDay   string   `json:"report_end_day,omitempty"`   // 28-day
}

type ActivityCounts struct {
    UserInitiatedInteractionCount int64 `json:"user_initiated_interaction_count"`
    CodeGenerationActivityCount   int64 `json:"code_generation_activity_count"`
    CodeAcceptanceActivityCount   int64 `json:"code_acceptance_activity_count"`
    LocSuggestedToAddSum          int64 `json:"loc_suggested_to_add_sum"`
    LocSuggestedToDeleteSum       int64 `json:"loc_suggested_to_delete_sum"`
    LocAddedSum                   int64 `json:"loc_added_sum"`
    LocDeletedSum                 int64 `json:"loc_deleted_sum"`
}

type Breakdown struct {
    IDE      string `json:"ide,omitempty"`
    Feature  string `json:"feature,omitempty"`
    Language string `json:"language,omitempty"`
    Model    string `json:"model,omitempty"`
    ActivityCounts // embedded: fields are promoted by encoding/json
}

type PullRequests struct {
    TotalCreated                        int64    `json:"total_created"`
    TotalReviewed                       int64    `json:"total_reviewed"`
    TotalMerged                         int64    `json:"total_merged"`
    MedianMinutesToMerge                *float64 `json:"median_minutes_to_merge"`
    TotalSuggestions                    int64    `json:"total_suggestions"`
    TotalAppliedSuggestions             int64    `json:"total_applied_suggestions"`
    TotalCreatedByCopilot               int64    `json:"total_created_by_copilot"`
    TotalReviewedByCopilot              int64    `json:"total_reviewed_by_copilot"`
    TotalMergedCreatedByCopilot         int64    `json:"total_merged_created_by_copilot"`
    TotalMergedReviewedByCopilot        int64    `json:"total_merged_reviewed_by_copilot"`
    MedianMinutesToMergeCopilotAuthored *float64 `json:"median_minutes_to_merge_copilot_authored"`
    MedianMinutesToMergeCopilotReviewed *float64 `json:"median_minutes_to_merge_copilot_reviewed"`
    TotalCopilotSuggestions             int64    `json:"total_copilot_suggestions"`
    TotalCopilotAppliedSuggestions      int64    `json:"total_copilot_applied_suggestions"`
}

type ThirdPartyAgent struct {
    AgentID                       string `json:"agent_id"`
    AgentName                     string `json:"agent_name"`
    UserInitiatedInteractionCount int64  `json:"user_initiated_interaction_count"`
    SessionCount                  *int64 `json:"session_count"` // aggregate only
}

type OrgDayRecord struct {
    Day                     string `json:"day"`
    EnterpriseID            string `json:"enterprise_id"`
    OrganizationID          string `json:"organization_id"`
    DailyActiveUsers        int64  `json:"daily_active_users"`
    WeeklyActiveUsers       int64  `json:"weekly_active_users"`
    MonthlyActiveUsers      int64  `json:"monthly_active_users"`
    MonthlyActiveChatUsers  int64  `json:"monthly_active_chat_users"`
    MonthlyActiveAgentUsers int64  `json:"monthly_active_agent_users"`
    DailyActiveCLIUsers     *int64 `json:"daily_active_cli_users"`
    ActivityCounts
    TotalsByIDE             []Breakdown       `json:"totals_by_ide"`
    TotalsByFeature         []Breakdown       `json:"totals_by_feature"`
    TotalsByLanguageFeature []Breakdown       `json:"totals_by_language_feature"`
    TotalsByLanguageModel   []Breakdown       `json:"totals_by_language_model"`
    TotalsByModelFeature    []Breakdown       `json:"totals_by_model_feature"`
    TotalsBy3rdPartyAgent   []ThirdPartyAgent `json:"totals_by_3rd_party_agent"`
    PullRequests            *PullRequests     `json:"pull_requests"` // pointer: detect absence
}

type UserDayRecord struct {
    Day                          string   `json:"day"`
    EnterpriseID                 string   `json:"enterprise_id"`
    OrganizationID               string   `json:"organization_id"`
    UserID                       int64    `json:"user_id"`
    UserLogin                    string   `json:"user_login"` // decode only; do not persist by default
    AICreditsUsed                *float64 `json:"ai_credits_used"`
    UsedAgent                    bool     `json:"used_agent"`
    UsedChat                     bool     `json:"used_chat"`
    UsedCLI                      bool     `json:"used_cli"`
    UsedCopilotApp               bool     `json:"used_copilot_app"`
    UsedCopilotCloudAgent        bool     `json:"used_copilot_cloud_agent"`
    UsedCopilotCodeReviewActive  *bool    `json:"used_copilot_code_review_active"`
    UsedCopilotCodeReviewPassive *bool    `json:"used_copilot_code_review_passive"`
    ActivityCounts
    TotalsByIDE             []Breakdown       `json:"totals_by_ide"`
    TotalsByFeature         []Breakdown       `json:"totals_by_feature"`
    TotalsByLanguageFeature []Breakdown       `json:"totals_by_language_feature"`
    TotalsBy3rdPartyAgent   []ThirdPartyAgent `json:"totals_by_3rd_party_agent"`
}

type UserTeamRow struct {
    UserID         int64  `json:"user_id"`
    UserLogin      string `json:"user_login"`
    Day            string `json:"day"`
    OrganizationID string `json:"organization_id"`
    EnterpriseID   string `json:"enterprise_id"`
    TeamID         int64  `json:"team_id"`
    Slug           string `json:"slug"`
}
```

---

## 5. Data latency, retention and coverage

**Freshness** [S5], [S6], [S20]. HIGH.
- Data for a day is available "within two full UTC days after that day closes" [S5].
  - **CORRECTED arithmetic:** on UTC date T, availability is guaranteed only for days up to T-3. Day T-2 may or may not be ready.
- It "typically finalizes within three full UTC days" [S6], so values for days after T-4 can still change.
- The dashboard may lag up to three full UTC days [S20].
- DevLake rewinds 2 days on incremental runs and keeps a 4-day trailing backfill window for daily metrics, because "reports are generated hours after midnight" [S17]. MEDIUM.

**History available:**
- **CORRECTED scope:** the text "available starting from October 10, 2025, and historical data can be accessed for up to 1 year" appears only on `enterprise-1-day` and enterprise `users-1-day` [S1]. It is not on enterprise `repos-1-day` or `user-teams-1-day`. HIGH.
- Org-level analytics start December 12, 2025 [S5]. HIGH.
- No retention window is stated on the org endpoints. Assuming the same 1-year limit is **UNVERIFIED**. DevLake hard-codes 365 days for both scopes [S17] (MEDIUM).
- The NDJSON export is positioned for "custom BI tools or long-term storage" [S5]. AI Stats should keep what it downloads.

**Attribution** [S5]. HIGH.
- A user must hold a seat somewhere in the enterprise.
- Their usage is attributed to **every org they belong to**, so org totals do not add up to the enterprise total.
- **ADDED implication:** an org's active users can exceed the seats billed to that org, for example when seats sit in a "shell" org (inference).

**Coverage** [S5], [S6]. HIGH.
- Excluded: Copilot Chat on github.com and GitHub Mobile.
- IDE telemetry must be on for breakdowns.
- Server-side-only users count in active totals and may appear in `totals_by_ide`. Other breakdowns and LoC stay empty for them.
- CLI is reported separately from IDE metrics and is not in IDE active counts.
- Minimum versions apply, for example VS Code 1.107.1 with Copilot Chat 0.35.3.
- Seats and licences are not in these reports. Use the user-management API (§7).

---

## 6. Billing endpoints: AI Credits and premium requests

### 6.1 Endpoints

From [S1] for org and user paths and [S1-ghec] for enterprise paths. HIGH.

| Path (GET) | Purpose | Query params | Documented responses |
|---|---|---|---|
| `/organizations/{org}/settings/billing/ai_credit/usage` | AI Credit usage, org | `year`, `month`, `day`, `user`, `model`, `product` | 200, 400, 403, 404, 500, 503 |
| `/organizations/{org}/settings/billing/premium_request/usage` | Premium request usage, org | same | 200, 400, 403, 404, 500, 503 |
| `/organizations/{org}/settings/billing/usage` | All metered usage, org. "Only available to organizations with access to the enhanced billing platform." | `year`, `month` (**no default**), `day` | 200, 400, 403, 500, 503 |
| `/organizations/{org}/settings/billing/usage/summary` | Usage summary, org (public preview) | `year`, `month`, `day`, `repository` (`owner/repo`), `product`, `sku` | 200, 400, 403, 500, 503 |
| `/enterprises/{enterprise}/settings/billing/ai_credit/usage` | AI Credit usage, enterprise | `year`, `month`, `day`, `organization`, `user`, `model`, `product`, **`cost_center_id`** (`'none'` = no cost center) | 200, 400, 403, 404, 500, 503 |
| `/users/{username}/settings/billing/ai_credit/usage` | Personal plans only | **CORRECTED:** `year`, `month`, `day`, `model`, `product` (no `user`) | 200, 400, 403, 404, 500, 503 |

**Notes:**
- **CORRECTED:** in [S1-ghec], `cost_center_id` is on the enterprise `ai_credit/usage`, `premium_request/usage` and `usage/summary` endpoints. [S9] says cost-center filtering is "available only for enterprise usage summary endpoints". The OpenAPI wins.
- Labelling `premium_request/usage` as the "pre-June-2026 model" is inference and is not stated in [S1]. It is useful for history before the AI-Credit switch.
- User endpoints apply only when the user bought their own Copilot plan [S10]. HIGH.

**Parameter semantics** [S1]. HIGH.
- `year` is an integer and defaults to the current year.
- `month` is 1 to 12 and defaults to the current month on `ai_credit`, `premium_request` and `summary`.
- `day` is 1 to 31.
- `user`, `model` and `product` are case-insensitive.
- [S9] also mentions an `hour` filter. It is not in [S1]. **UNVERIFIED.**

**Retention:** "Only data from the past 24 months is accessible" on `ai_credit`, `premium_request` and `summary` [S1]. HIGH. The plain `/usage` endpoint states no retention.

### 6.2 Response schema

Schema `billing-ai-credit-usage-report-org`. The premium request schema has the identical shape [S1]. HIGH.

**CORRECTED: the verbatim OpenAPI example (the draft's example was embellished):**

```json
{ "timePeriod": {"year": 2025}, "organization": "GitHub",
  "usageItems": [ { "product": "Copilot", "sku": "Copilot AI Credits", "model": "GPT-5", "unitType": "credits",
    "pricePerUnit": 0.01, "grossQuantity": 100, "grossAmount": 1.0, "discountQuantity": 0, "discountAmount": 0.0,
    "netQuantity": 100, "netAmount": 1.0 } ] }
```

**Fields:**
- Field names are camelCase.
- Required top-level fields: `timePeriod` (with required `year`; `month` and `day` optional), `organization`, `usageItems`.
- Optional top-level fields: `user`, `product`, `model`. Reading them as echoes of the filters is an inference (MEDIUM).
- **`usageItems[]` has no user field.** Per-user spend needs `?user=<login>` once per user.
  - [S15] reports that some enterprise plans return only enterprise-level aggregates, so per-user queries can come back as $0. MEDIUM.
- All `usageItems` fields are required: `product`, `sku`, `model`, `unitType` (strings) and `pricePerUnit`, `grossQuantity`, `grossAmount`, `discountQuantity`, `discountAmount`, `netQuantity`, `netAmount` (numbers).

**Example values:**
- The premium request example uses sku `"Copilot Premium Request"`, unitType `"requests"` and pricePerUnit `0.04`.
- **CORRECTED:** the AI Credit examples are inconsistent:
  - Org example: product `Copilot`, sku `Copilot AI Credits`, unitType `credits`.
  - User example: product `Copilot AI Credits`, sku `AI Credit`, unitType `ai-credits`.
  - Do not hard-code `unitType`, `product` or `sku` display strings.
  - Whether `sku` in responses is a display name or the identifier in [S11] is **UNVERIFIED**.

**CORRECTED: unit value is documented, not just an example:**
- 1 AI credit = **$0.01 USD**, and "a $10 USD budget covers 1,000 AI credits" [S23]. HIGH.
- Included pool: Business 1,900 and Enterprise 3,900 credits per user per month. The pool is shared at the billing entity and resets at 00:00 UTC on the 1st [S23]. HIGH.
- Code completions and next edit suggestions are **not** billed in AI credits [S23]. HIGH.
- So `ai_credits_used × $0.01` is gross consumption before the included-pool discount. It is not net billed spend.

**SKU identifiers** [S11]. HIGH.
- AI credit SKUs: `copilot_ai_credit`, `coding_agent_ai_credit`, `code_quality_ai_credit`, `spark_ai_credits`.
- Seat SKUs: `copilot_for_business`, `copilot_enterprise`, `copilot_standalone`.
- Products include `copilot` and `sandbox`.

**Alternative (ADDED):** the downloadable "AI usage report" gives per-user, per-model, per-day AI-credit detail with input, output and cache token counts, for up to 31 days [S25]. HIGH that it exists. The enterprise API for it is `/enterprises/{enterprise}/settings/billing/reports` [S1-ghec]; its request details are **UNVERIFIED**.

### 6.3 Billing auth

**Org endpoints.** Caller must be "an administrator of an organization within an enterprise or an organization account" [S1]. [S9] says org owner or billing manager. HIGH.

**Enterprise endpoints (ADDED caller list)** [S1-ghec]. HIGH.
- Callers: "an administrator or billing manager of the enterprise, an organization owner in the enterprise, or a custom role holder with fine-grained read access to enterprise billing".
- An installation token for an app installed on the enterprise with enterprise billing read also works. Installation tokens expire after 1 hour [S9].
- An org owner in an enterprise may therefore be able to call the enterprise endpoint with `organization=<org>`. Whether results are limited to their org, and which classic scope that needs, is **UNVERIFIED**.

**Fine-grained PATs: conflicting sources.**
- [S9]: "The billing usage endpoints do not support fine-grained personal access tokens."
- [S8] lists org `Administration: read` for fine-grained PATs.
- Use a classic PAT or OAuth token. MEDIUM.

**Classic scopes are not documented** for any `/settings/billing/*` usage endpoint, in [S1] or the docs (HIGH that they are absent). Real-world reports from [S15] (MEDIUM, **UNVERIFIED** by GitHub docs):
- `manage_billing:copilot` works for standalone orgs.
- For enterprise-owned orgs, the org endpoint returns **404**. Call `/enterprises/{slug}/settings/billing/ai_credit/usage` with `manage_billing:enterprise`, SSO-authorized.
  - Circumstantial support: [S1-ghec] and the GHEC permission data omit the org billing endpoints.

**Scope facts:**
- `manage_billing:enterprise` = "Read and write enterprise billing data" [S12]. HIGH.
- **CORRECTED:** `manage_billing:copilot` is **not listed** in the OAuth scopes doc [S12]. Its write capability is shown by [S1], where it authorizes `POST/DELETE /orgs/{org}/copilot/billing/selected_users|selected_teams` and enterprise Copilot policy writes. HIGH.
- Requesting it from users is a write-capable grant, which matters for a privacy-first product.
- Whether AI Stats' `read:org` is enough for billing endpoints is **UNVERIFIED** (probably not).

**Lighter alternative.** Per-user `ai_credits_used` in `users-1-day` needs only `read:org` plus the metrics role (HIGH, [S1]).
- It is not broken down by model.
- It is "for consumption analysis, not invoicing totals" [S2].
- Aggregate it to team level per §8 before storing.

---

## 7. Seats and licensed users (replaces the "licensed = active" proxy)

**`GET /orgs/{org}/copilot/billing`** [S1]. HIGH.
- Public preview. "Only organization owners."
- Classic tokens need `manage_billing:copilot` **or `read:org`**.
- Responses: 200, 401, 403, 404, 422, 500.
- Returns:
  - `seat_breakdown{total, added_this_cycle, pending_invitation, pending_cancellation, active_this_cycle, inactive_this_cycle}` (integers).
  - `plan_type` (`business` or `enterprise`).
  - `seat_management_setting` (`assign_all`, `assign_selected`, `disabled` or `unconfigured`).
  - `public_code_suggestions`, `ide_chat`, `platform_chat`, `cli`.
- Required fields: `seat_breakdown`, `public_code_suggestions`, `seat_management_setting`.
- **ADDED:** it is a **current snapshot**, so historical days cannot be backfilled with seat counts.

**`GET /orgs/{org}/copilot/billing/seats`** [S1]. HIGH.
- **ADDED pagination:** `page` and `per_page` (default 50, **max 100**), with a `Link` header.
- Response: `{total_seats, seats[]}`.
- Seat fields: `assignee`, `organization`, `assigning_team`, `pending_cancellation_date`, `last_activity_at`, `last_activity_editor`, `last_authenticated_at`, `created_at` (required), `updated_at`, `plan_type`.
- Same scopes as above.
- `last_activity_at` is kept for 90 days and can take up to 24 hours to update [S26].

---

## 8. Migration mapping for AI Stats

**Field mapping into `copilot_usage_snapshots`**, from the `organization-1-day` record.

| Current column | New source | Caveat |
|---|---|---|
| `total_suggestions` | `code_generation_activity_count` | **CORRECTED/ADDED:** the top-level value counts all surfaces (chat, agent, app). For continuity with the old completions-only metric, use the `totals_by_feature[feature="code_completion"]` entry (MEDIUM, inference). Either way, re-baseline and do not diff across the cutover [S4]. |
| `total_acceptances` | `code_acceptance_activity_count` | Includes chat apply, insert, copy and terminal insert [S2] |
| `total_lines_accepted` | `loc_added_sum` | Includes agent and edit mode [S2] |
| (lines suggested) | `loc_suggested_to_add_sum` | Excludes agent edits [S2] |
| `total_active_users` | `daily_active_users` | Includes server-side-only users. Excludes CLI-only users, who are in `daily_active_cli_users` [S2], [S6]. |
| `total_licensed_users` | `seat_breakdown.total` from `GET /orgs/{org}/copilot/billing` | Snapshot only; org owners only. Org active users can exceed org seats because of attribution [S5]. |
| `breakdown_json` language and editor | `totals_by_language_feature` (language), `totals_by_ide` (editor) | No language-by-IDE cross-tab [S4]. The old per-row `active_users` has no equivalent. |

**Key note.** `copilot_usage_snapshots.organization_id` is AI Stats' internal organization ID (a foreign key). It is not the report's `organization_id` string. Keep upserting on the existing `UNIQUE (organization_id, date)`.

**Sync algorithm (recommended, CORRECTED windows):**
1. Pick the days to fetch, all UTC:
   - Start at `max(lastSyncedDay - 5, today - 365, 2025-12-12)`. The 365-day bound is **UNVERIFIED** for org scope.
   - End at `today - 1`.
   - Always re-fetch `today-5 .. today-1`. Data is guaranteed only through T-3 and finalizes after about 3 more days ([S5], [S6]). Re-pulling only 3 days is not enough.
2. Call `organization-1-day?day=d` for each day.
   - 204, 404, a `""` or `null` body, or empty links mean "not ready". Retry only for days ≥ today-7. Older days count as "no data" so retries stay bounded.
   - Treat 403 as a configuration error (§3.1) and stop the sync.
3. Download every link with a timeout and **no** auth header. If a download fails, re-request the links once.
4. Decode with a streaming `json.Decoder`.
5. Upsert by `(internal organization id, date)`.
6. Optionally, fetch `organization-28-day/latest` once for a fast 28-day initial backfill.

**Team-level (privacy-first default):**
- Fetch `users-1-day` and `user-teams-1-day` for the same day.
- Join in memory on `(user_id, day, organization_id)` and aggregate to `(team_id, day)`.
- Persist **only** team aggregates. Drop `user_login` and `user_id` unless the operator opts in.
- Teams with fewer than 5 seated users are absent [S4], so the team sum is less than the org total.
- Users in several teams are counted in each team.

**Error messages for the operator:**
- **403:** "Requires an org owner or a custom role with 'View Organization Copilot Metrics'. The token needs `read:org`. The org's 'Copilot usage metrics' policy must be enabled, and the AI Stats OAuth app must be approved if the org restricts third-party OAuth apps."
- **404 on `organization-28-day/latest`:** "Org not found, no access, or no Copilot usage metrics for this org."
- **404 or 204 on a recent 1-day report:** "Report not generated yet; will retry."

---

## Open questions (re-assessed)

1. **Signed URL TTL and host:** undocumented. **UNVERIFIED.** Measure empirically.
2. **`users-28-day/latest` row granularity:** the docs say one record per user; [S16] says per user per day. **UNVERIFIED.** Use `users-1-day`.
3. **Org endpoint retention:** the 1-year limit is stated only for enterprise-1-day and enterprise users-1-day. Org data starts 2025-12-12. **UNVERIFIED.**
4. **DevLake's "204 when fewer than 5 active users":** **UNVERIFIED** in GitHub docs.
5. **Classic scope for org billing endpoints:** undocumented. **UNVERIFIED**. [S15] reports `manage_billing:copilot` (standalone orgs).
6. **Fine-grained PAT and GitHub App support for billing:** contradictory across [S8], [S9] and [S15]. **UNVERIFIED.**
7. **Org-level `ai_credit` 404 for enterprise-owned orgs:** reported by [S15]; circumstantially supported by GHEC omitting those paths. **UNVERIFIED.** An org owner may be able to call the enterprise endpoint ([S1-ghec] caller list).
8. **Billing data latency:** **UNVERIFIED.**
9. **`ai_credits_used` granularity:** [C9] (LOW) says per day in users-1-day. 28-day semantics are unclear. It is not for invoicing [S2].
10. **Changelog dates:** github.blog is still unreachable, so every [C*] date is LOW.
11. **Org-level policy name for orgs without an enterprise:** [S14] links the org policies page, but that page does not name the policy. **UNVERIFIED.**
12. **OAuth app access restrictions on these endpoints:** restrictions are default-on for new orgs and block API access to private org resources [S24]. Whether that covers the metrics endpoints is **UNVERIFIED** but likely.
13. **Duplicate `(user_id, day)` rows across multi-file 1-day reports:** **UNVERIFIED** ([S16] hints at sharding).

## Verification notes

I re-downloaded every primary source the draft cited: four OpenAPI variants plus GHES 3.22, the github/docs Markdown, permission JSON and docs variables, and the five OSS consumer files. I checked each claim programmatically or by reading the text.

**Confirmed (HIGH).** These match the sources exactly:
- **Metrics report endpoints:** all paths, methods, operationIds, parameters (`day` as YYYY-MM-DD, `format: date`, required on 1-day), response codes (204 only where the spec said), the two download-link schemas, and the auth wording (`read:org` for org; `manage_billing:copilot` or `read:enterprise` for enterprise).
- **API versions:** the definitions are JSON-identical in 2022-11-28 and 2026-03-10.
- **Fields and joins:** nearly all record field names, types and nullability in S2 and S3, the team join recipe, and the fewer-than-5-seats team exclusion.
- **Freshness, history and permissions:** the freshness wording, the 2025-12-12 org start date, attribution rules, and the permission keys and titles.
- **Billing:** the billing endpoint paths, parameters, schema field names, 24-month retention, and SKU identifiers.

**Corrected.** About 30 discrepancies, mostly incomplete rather than wrong:
- The billing example JSON was not verbatim.
- The user billing endpoint has no `user` parameter.
- `cost_center_id` is also on the enterprise `ai_credit` and `premium_request` endpoints.
- The enterprise billing endpoints also accept org owners in the enterprise.
- The $0.01 credit value and the included-pool sizes are documented (HIGH), not just examples.
- GHES non-support is HIGH via the GHES OpenAPI.
- The Go type sketch would not decode as written.
- The freshness arithmetic means a 3-day re-pull is too short.
- `created_at` and `copilot_suggestions_by_comment_type` are missing from the official examples despite being "non-null".
- The new EMU-only usage-records endpoint returns prompt bodies and must be avoided.

**Still unverified.** The things an implementer most needs to watch:
- Signed-URL TTL and host.
- Whether `users-28-day` is per user or per user per day.
- Org-level retention.
- DevLake's "204 when fewer than 5 users" rule.
- Classic scope, fine-grained PAT and GitHub App support for the billing endpoints (the sources conflict).
- Whether the org `ai_credit` endpoint 404s for enterprise-owned orgs; this has circumstantial support because GHEC omits those paths.
- Billing latency.
- Whether OAuth app restrictions apply to the metrics endpoints.

**Changelog dates.** github.blog is still blocked, so every changelog date relies on WebSearch summaries (LOW), although they agree with the draft.

**Bottom line.** The core implementation path is reliable: org 1-day reports via `read:org` plus the owner or custom role plus the policy, signed-URL NDJSON download without the auth header, `users-1-day` joined with `user-teams-1-day` for team aggregates, and `ai_credits_used` for per-user credits. The billing-API auth story is not settled and should be tested empirically before AI Stats requests any extra scope.

Corrections made during verification:

- **Claim:** §6.2 example response: {"timePeriod": {"year": 2025, "month": 6, "day": 1}, "organization": "GitHub", "user": "octocat", "product": "Copilot", "model": "GPT-5", ...}  
  **Finding:** Not verbatim. The OpenAPI example billing-ai-credit-usage-report-org is {"timePeriod":{"year":2025},"organization":"GitHub","usageItems":[...]}. It has no month or day and no user, product or model echo. The schema allows those fields as optional, so calling them 'only echoes of the filters' is inference. (S1 components/examples/billing-ai-credit-usage-report-org)
- **Claim:** §6.2 AI Credit example uses unitType "credits" (a single representative shape)  
  **Finding:** The OpenAPI examples are inconsistent. The org example has product Copilot, sku 'Copilot AI Credits' and unitType 'credits'. The user example has product 'Copilot AI Credits', sku 'AI Credit' and unitType 'ai-credits'. Implementers must not hard-code these strings. (S1 examples billing-ai-credit-usage-report-org and billing-ai-credit-usage-report-user)
- **Claim:** §6.2 '1 credit = $0.01 is an example only; MEDIUM'  
  **Finding:** It is documented. data/variables/product.yml has prodname_ai_credits_value '$0.01 USD', and the billing concept says 'a $10 USD budget covers 1,000 AI credits'. Upgraded to HIGH. Also documented: an included pool of 1,900 (Business) or 3,900 (Enterprise) credits per user per month, pooled and reset monthly at 00:00 UTC; code completions and next edit suggestions are not billed in AI credits. (github/docs data/variables/product.yml, data/variables/copilot.yml, content/copilot/concepts/billing-and-usage/organizations-and-enterprises/billing.md)
- **Claim:** §6.1 /users/{username}/settings/billing/ai_credit/usage query params are 'as org'  
  **Finding:** The user endpoint has no `user` parameter. Its params are year, month, day, model and product only. (S1 /users/{username}/settings/billing/ai_credit/usage)
- **Claim:** §6.1 enterprise ai_credit endpoint 'adds organization (and cost_center_id on summary [S9])'  
  **Finding:** The GHEC OpenAPI lists cost_center_id on the enterprise ai_credit/usage and premium_request/usage endpoints too, as well as on usage/summary. S9's sentence 'Cost center filtering is available only for enterprise usage summary endpoints' conflicts with the OpenAPI. (S1-ghec /enterprises/{enterprise}/settings/billing/ai_credit/usage parameters)
- **Claim:** §6.1 billing endpoints (response codes not listed)  
  **Finding:** Missing documented codes. ai_credit, premium_request and the user endpoints return 200/400/403/404/500/503. The org /usage and /usage/summary endpoints document no 404. /usage has no default for month, and its 24-month retention note is absent. (S1)
- **Claim:** §6.3 billing role: org owner or billing manager; enterprise endpoint needs manage_billing:enterprise  
  **Finding:** The spec omitted that the enterprise ai_credit, premium_request and summary endpoints also accept 'an organization owner in the enterprise' and custom role holders with enterprise billing read. Classic scopes are not stated in the OpenAPI for any billing usage endpoint. (S1-ghec endpoint descriptions)
- **Claim:** §6.3 'Requesting manage_billing:copilot is a write-capable scope' (implicitly sourced to the scopes doc)  
  **Finding:** manage_billing:copilot is not listed in the OAuth scopes doc (S12). Its write capability is confirmed in the OpenAPI instead: it authorizes POST/DELETE /orgs/{org}/copilot/billing/selected_users and selected_teams, and enterprise Copilot policy writes. (S12; S1/S1-ghec write endpoint descriptions)
- **Claim:** §3.2 GitHub App installation token = yes for org billing endpoints  
  **Finding:** S8 does mark server-to-server true for org Administration read. But copilot-metrics-viewer states 'Fine-grained PATs and GitHub Apps cannot read billing today'. The conflict is unresolved, so it is marked UNVERIFIED. (S8 fpt server-to-server-permissions.json; S15 README)
- **Claim:** (not mentioned) org billing endpoints availability on GHEC  
  **Finding:** /organizations/{org}/settings/billing/* is absent from ghec.json and from the GHEC permission metadata; it appears only in the fpt variants. This circumstantially supports S15's report that the org endpoint 404s for enterprise-owned orgs. (S1-ghec; S8 ghec-2022-11-28 files)
- **Claim:** §1 Legacy Copilot Metrics API closed 2026-04-02: HIGH; other changelog rows MEDIUM  
  **Finding:** The spec's own scale defines 'only from a search summary' as LOW, yet [C*] rows were rated MEDIUM and the 04-02 closure HIGH. github.blog is still unreachable (proxy 403 and DNS failure). Re-rated: closure MEDIUM (S15 OSS README plus summary); C1, C3, C4, C5, C6 and C7 dates LOW. WebSearch summaries do agree with the dates. (S15; WebSearch summaries of C1-C9)
- **Claim:** §2.4 'GHES: not supported [S15]. MEDIUM'  
  **Finding:** Upgraded to HIGH. The GHES 3.22 OpenAPI description contains zero Copilot paths. (rest-api-description descriptions/ghes-3.22/ghes-3.22.json)
- **Claim:** §2.4 API version: 2022-11-28 works (no lifecycle info)  
  **Finding:** Missing: 2022-11-28 end of support is March 10, 2028; 2026-03-10 has no scheduled end. An unsupported version returns 410 Gone. (github/docs data/tables/rest-api-versions.yml; content/rest/about-the-rest-api/api-versions.md)
- **Claim:** §4.3 created_at: string, not nullable  
  **Finding:** S2 says not nullable, but the S3 28-day enterprise examples omit created_at entirely. Treat it as optional. (S2; S3 enterprise-level schema example)
- **Claim:** §4.6 copilot_suggestions_by_comment_type: 'Always present, possibly empty'  
  **Finding:** It is absent from both aggregated day_totals records in the S3 enterprise example, despite S2's wording. Decode it as optional. (S3)
- **Claim:** (not mentioned) daily PR suggestion ratios  
  **Finding:** An S3 repo row has total_copilot_suggestions 0 and total_copilot_applied_suggestions 1, so applied counts can exceed suggested counts on a given day. Per-day acceptance ratios must not assume the value is at most 1. (S3 repository-level schema example)
- **Claim:** §4.1/§4.9 enterprise_id is "" for orgs not owned by an enterprise  
  **Finding:** S2 says it is empty. S3 prose says org-scoped repo rows include enterprise_id 'only when an enterprise owns the organization', which implies it may be absent, while the S3 example shows "". Treat absent and "" the same. (S2; S3)
- **Claim:** §4.9 'Data for pull_request_review_times starts 2026-09-21 and is not backfilled'  
  **Finding:** More precisely, PRs that became ready for review before 2026-09-21 are excluded from the array but still counted in pull_requests.total_merged. The array is non-nullable; [] means no qualifying data, not zero minutes. Bot-authored PRs (including the Copilot cloud agent) and PRs without human review are excluded. (S2 Pull request review time fields)
- **Claim:** §4.10 Go types sketch  
  **Finding:** As written it would not decode. `IDE, Feature, Language, Model string `json:"ide,omitempty"`` gives four fields the same JSON name, and encoding/json drops conflicting names. Untagged fields such as EnterpriseID, DailyActiveUsers, UsedCLI and LocAddedSum do not match snake_case keys, because the case-insensitive match does not ignore underscores. PullRequests was undefined. Replaced with fully tagged structs. (Go encoding/json semantics; field names from S2)
- **Claim:** §5 'Enterprise 1-day reports: available starting from October 10, 2025 ... up to 1 year'  
  **Finding:** That sentence appears only on enterprise-1-day and enterprise users-1-day, not on enterprise repos-1-day or user-teams-1-day. (S1 operation descriptions)
- **Claim:** §8 sync: range ends at today-2; 'Always re-pull the last 3 days'  
  **Finding:** 'Within two full UTC days after that day closes' means availability is guaranteed only through T-3. 'Typically finalizes within three full UTC days' means days after T-4 can still change. A 3-day re-pull misses late finalization; re-fetch T-5..T-1 and bound retries (DevLake uses a 4-day trailing window). (S5 Data freshness; S6 Delayed telemetry; S17)
- **Claim:** §8 total_suggestions -> code_generation_activity_count  
  **Finding:** The top-level count includes chat, agent and Copilot app generations. For continuity with the legacy completions-only metric, the totals_by_feature[code_completion] entry is the closer match (inference). Re-baseline either way. (S2 feature dimension; S4 caveats)
- **Claim:** §8 total_licensed_users -> seat_breakdown.total  
  **Finding:** /orgs/{org}/copilot/billing is a current snapshot (public preview, org owners only), so it cannot backfill historical days. Because org attribution counts any enterprise-seated member, an org's active users can exceed its billed seats. (S1; S5 attribution)
- **Claim:** §8 'Upsert by (organization_id, date)'  
  **Finding:** In AI Stats, copilot_usage_snapshots.organization_id is the internal org FK (UNIQUE (organization_id, date)), not the report's organization_id string. The spec should say which one it means. (backend/internal/db/migrations/001_initial.sql)
- **Claim:** §3.1 OAuth app access restrictions: 'general GitHub behaviour, not verified. LOW-MEDIUM'  
  **Finding:** The docs confirm that OAuth app access restrictions are enabled by default for new organizations and that unapproved OAuth apps lose API access to private organization resources. Whether these restrictions apply to the metrics endpoints specifically is still UNVERIFIED. (github/docs about-oauth-app-access-restrictions.md; data/reusables/organizations/oauth_app_restrictions_default.md)
- **Claim:** (missing) other Copilot usage endpoints  
  **Finding:** The GHEC OpenAPI has GET /enterprises/{enterprise}/copilot/usage-records in the copilot-usage-metrics subcategory (public preview, EMU owners only, read:enterprise, per_page up to 25). It returns raw request and response bodies, which means prompts and completions. AI Stats must not use it under its privacy-first rule. (S1-ghec /enterprises/{enterprise}/copilot/usage-records)
- **Claim:** §3.2 copilot-metrics-viewer 'recommends' members read and Copilot seat management read  
  **Finding:** The README says the new API 'requires' members read, Organization Copilot metrics and Copilot seat management, and that missing permissions produce 400/403. The OpenAPI permission metadata lists only 'Organization Copilot metrics' for the report endpoints, so the extra permissions are probably for the viewer's own features. (S15 README line ~344; S8)
- **Claim:** §7 seats endpoint: 'Paginated per-seat records'  
  **Finding:** Details were missing: page and per_page (default 50, max 100), a Link header, and a response of {total_seats, seats[]} where only created_at is required in a seat. /orgs/{org}/copilot/billing also documents 401 and 422, and its required fields are seat_breakdown, public_code_suggestions and seat_management_setting. (S1)
- **Claim:** §2.5/§2.6 multiple download links: 'download all and concatenate'  
  **Finding:** Incomplete. copilot-metrics-viewer reports that large orgs get time-partitioned files in which the same user appears in several files (issue #398), and says one day can be sharded. Concatenating per-user rows can therefore double count; key rows by (user_id, day). (S16 comments in fetchLatestUserReport and fetchUserReportForDate)
- **Claim:** §2.7 status codes  
  **Finding:** Missing: a signed-URL download can itself return 404 (DevLake logs 'Report blob not found' and skips). 500 is documented on every metrics endpoint. 410 Gone is returned for an unsupported API version. 401 is not listed on these operations. (S17 downloadReport; S1; S21)
- **Claim:** §1 ai_credits_used added 2026-06-19 [S15],[S16]  
  **Finding:** Corroborated by a search summary of changelog 2026-06-19-ai-credits-consumed-per-user-now-in-the-copilot-usage-metrics-api: per user per day, in both users-1-day and users-28-day, from the same data as billing. A 2026-07-02 fix (C8) corrected understated values such as users showing 0.0. LOW for the summary-derived details. (WebSearch summaries of C9 and C8)
- **Claim:** §4.5 language values 'free-form, with unknown possible'  
  **Finding:** S6 writes the bucket as 'Unknown' while S2 and S3 examples use lowercase 'unknown'. Compare case-insensitively. (S6 'Unknown values'; S2; S3)
- **Claim:** §4.7 avg_pull_requests_minutes_to_review and avg_pull_requests_review_cycles are averages  
  **Finding:** S2 documents both as medians despite the avg_ prefix. (S2 AI adoption phase fields)
- **Claim:** §3.1 the current copilot.go 403 message names a non-existent scope (only issue flagged)  
  **Finding:** Confirmed: copilot_usage_data:read is not in S12. In addition, the 404 message 'organization not found or Copilot not enabled' is now misleading because the endpoint itself was removed. http.DefaultClient (no timeout) is used and should be replaced. (backend/internal/github/copilot.go; S12)
