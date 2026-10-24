# Cursor Admin, Analytics and AI Code Tracking APIs: pull-connector spec for AI Stats (Go). Verified revision.

> **Status:** implementation spec, researched and independently re-verified on 2026-10-23 against the
> primary sources cited inline. Claims that could not be confirmed are marked UNVERIFIED. Re-check
> anything load-bearing against the live API before relying on it; vendor APIs change.

## 0. How this was verified

### 0.1 Access to the docs
- cursor.com is still blocked from this environment (`CONNECT tunnel failed, 403`), so I could not read the docs directly.
- forum.cursor.com, docs.getdx.com, speakeasy.com and help.nops.io are also blocked for both curl and WebFetch. For those I had only WebSearch snippets.

### 0.2 Primary mirror and its freshness
- I re-downloaded every doc page from the `mehmetbaykar/cursor-docs-skill` mirror.
- Its `docs_manifest.json` shows a sync on **2026-10-23 at 21:19 UTC**. The run reports 214 pages fetched, 0 failed and 0 stale.
- Per-page sync dates:
  - Admin API: 2026-10-23
  - API overview: 2026-10-22
  - Analytics API: 2026-10-08
  - AI Code Tracking API: 2026-09-28
  - Organization API: 2026-10-23
- The mirror leaves out `/docs/enterprise/*` and `/docs/account/enterprise/*`. Service-account and compliance pages are therefore **not** mirrored.

### 0.3 Mirror cross-check
- For the read endpoints, `mrkhachaturov/agent-harness-docs` is byte-identical to the primary mirror.
- `RanulfoMNeto/docs` is an older sync. It is missing two things:
  - the sentence linking `teamMembers[].id` to the number returned by `/organizations/members`;
  - the audit-log paragraph saying `users` email matching is case-sensitive.
- The old `TSGCFO` snapshot confirms what changed over time:
  - `id` and spend `userId` used to be numbers (`12345`);
  - usage events used to default to `pageSize` 10.

### 0.4 Source keys
These are the original keys plus new ones I added.

| Key | Where |
|---|---|
| [DOC-ADMIN] [DOC-API] [DOC-ANALYTICS] [DOC-AICODE] [DOC-ORG] [DOC-OLD] | Same mirrors as before. Re-fetched and read in full for the read endpoints. |
| [DOC-DASH] | cursor.com/docs/account/teams/analytics, via the mirror `account__teams__analytics.md`. |
| [AIRBYTE] [DEVLAKE] [OLIVARES] [METORIAL] [OFERSHAP] [SKIPPEDNOTE] [GJKAUR] [BICAMERAL] | Same raw.githubusercontent URLs as before. All re-fetched. |
| [OBSERVER] | superbasedapp/observer. **Not an independent client**: its `doc.go` says the drivers were "written from the public Cursor Admin API reference (fetched 2026-10-07), never from a live response … 'synthetic, awaiting live validation'". It only echoes the docs. |
| [ELASTIC] | https://raw.githubusercontent.com/elastic/integrations/main/packages/cursor/docs/README.md (fetched) |
| [DX] [SPEAKEASY] [CORALOGIX] [CHECKPOINT] | docs.getdx.com/connectors/cursor, speakeasy.com/docs/ai-control-plane/org-admin/ai-integrations/cursor, coralogix.com/docs/integrations/ai-observability/cursor, the Check Point Workforce AI guide. **WebSearch snippets only.** |
| [FORUM-REMOVED] | forum.cursor.com/t/165528 and /t/161589. Snippets only. |
| [FORUM-500] | forum.cursor.com/t/getting-500-error-on-teams-filtered-usage-events-endpoint/165632. Snippets only. |
| [FORUM-SCHEMA] | forum.cursor.com/t/daily-usage-api-response-schema-email-vs-user-id/133545. Snippets only. |
| [FORUM-IDS] | forum.cursor.com/t/167128 and /t/169722. Snippets only. |
| [REPO] | the repo: `backend/internal/db/queries.go`, `backend/internal/db/migrations/00{2,4,7}_*.sql`, `backend/internal/github/{commits,copilot}.go`, `backend/internal/crypto/aes.go`, `backend/internal/handlers/ingest.go` |

### 0.5 Confidence levels (redefined)
- **H:** stated in the current docs, verbatim, or in the docs' JSON example. Confirmation from OBSERVER does **not** count, because it is derived from the docs.
- **M:** in the docs but ambiguous; or used by two or more clients that appear to have been run against the live API (Airbyte, DevLake, ofershap, olivares, metorial).
- **L:** a single third-party source, a forum snippet, or my own inference.
- **UNVERIFIED:** not confirmed by any source close to primary.

---

## 1. Base URL, authentication and keys

### 1.1 Base URL and auth scheme
- **Base URL:** `https://api.cursor.com`. There is no version prefix. **H**
- **Auth:** HTTP Basic. The API key is the username and the password is empty. **H** [DOC-API][DOC-ADMIN][DOC-AICODE][DOC-ANALYTICS]
  - curl: `curl -u KEY:`
  - Go: `req.SetBasicAuth(key, "")`
  - Clients that do this: [OLIVARES][SKIPPEDNOTE][OFERSHAP][AIRBYTE].
- **Bearer:** the docs promise Bearer only for the Cloud Agents API (Origin also uses Bearer credentials). **H**
  - The caching example in [DOC-API] does use `Authorization: Bearer` against `/analytics/team/dau`.
  - A forum bug report describes calling `/teams/daily-usage-data` with a Bearer key. **L**
  - Bearer may work in practice, but it is not documented. **Use Basic.**

### 1.2 Key format
- Docs: `crsr_` followed by 64 characters (I counted the 64 `x`s in [DOC-API]). **H**
- [ELASTIC] says keys look like `key_` followed by hex. **L, conflicting**
- **Do not validate the prefix.** Only check that the key is non-empty and contains no whitespace.

### 1.3 Scopes
- **Admin API and AI Code Tracking API:** the docs require `admin:*`. **H** [DOC-API] ([AIRBYTE] says nothing about scope.)
- **Analytics API:** most endpoints require `admin:*`; `/analytics/team/bugbot-reviews` requires `read:*`. **H** [DOC-ANALYTICS]
- **Team routes with their own scopes:** **H** [DOC-ADMIN]
  - Directory groups: reads need `read:*`, writes need `admin:*`.
  - Grok Bot: reads accept `read:*` or `admin:*`.
  - Model access: needs `models:read` / `models:*`, or `admin:*`. "Generic `read:*` keys cannot call these routes."
- **`read:*` on the usage endpoints is UNVERIFIED** (`/teams/daily-usage-data`, `/teams/filtered-usage-events`, `/teams/spend`, `/teams/members`). Evidence so far:
  - [DX] and [SPEAKEASY] tell customers to create a team key with the scope set to **"Read-only"** for usage and spend ingestion. [DX] lists `/teams/daily-usage-data`. **L**
  - [CORALOGIX] tells customers to use `admin:*`. **L**
  - **Recommendation:** document both. Suggest Read-only first, for least privilege. If a call returns 401 or 403, show "this key's scope may be too narrow; try an Admin key".

### 1.4 Plan availability
- The API overview lists Admin, Analytics, AI Code Tracking and Bugbot as "Enterprise teams". **H** [DOC-API]
- The Analytics page says "Only for enterprise teams". AI Code Tracking says "Enterprise only, contact sales". **H**
- The Admin API page marks **only** the write routes "Enterprise only" (user-spend-limit, remove-member). **H**
- Signs that Teams plans may also work:
  - The forum bug report was written by a user on a Team plan. This is the original reporter, not a staff reply as the original spec said. **L**
  - [CHECKPOINT] says "Teams or Enterprise". **L**
- Treat `403` as a possible plan limit, but do not assume that is the only cause (see 2.2).

### 1.5 Team keys and organization keys
- A team key acts within exactly one team. Organization endpoints require an Organization API key, and `/teams/*` requires a team key. **H** [DOC-ORG]
- A forum reply says an org key used against `/teams/*` can come back as a **500**, not a 401 or 403. **L** [FORUM-500]
- Each Organization API key carries **exactly one** scope: `members:read`, `members:*`, `usage:*`, `models:read`, `models:*`, `auditlogs:read` or `admin:*`. **H** [DOC-ORG]

### 1.6 Headers and request body
- On POST, send `Content-Type: application/json`. **H** (doc examples)
- Send `Accept: application/json`. **L** ([OLIVARES] only)
- **Always send a JSON body on POST, at least `{}`.** A forum reply says a POST with the JSON content type and no body can return **500**. **L** [FORUM-500]

## 2. Rate limits, errors, caching and polling

### 2.1 Rate limits
"Limits apply per authenticated user, team, or organization, and most are scoped to a single endpoint. Unless an endpoint documents a different limit, the default is 20 requests per minute." [DOC-API] **H**

| API / route | Limit | Conf |
|---|---|---|
| Admin, most routes, incl. `/teams/members`, `/teams/daily-usage-data` ("per team"), `/teams/spend`, `/teams/audit-logs` ("per team") | 20/min | H |
| `/teams/filtered-usage-events`, `/organizations/filtered-usage-events` | 60/min ("per team") | H |
| `/teams/user-spend-limit` (write, unused) | 250/min | H |
| `/teams/user-spend-limits` (bulk write, unused) | 20/min | H |
| `/teams/remove-member` (write, unused) | 50/min | H |
| Organization API, most routes | 20/min **per endpoint** | H |
| Analytics, team-level | 100/min per team | H |
| Analytics, by-user | 50/min per team | H |
| `/analytics/team/conversation-insights` | 20/min | H |
| AI Code Tracking | 20/min per endpoint | H |

- Whether the Admin API's 20/min is a separate budget per endpoint or one budget shared across the team is **UNVERIFIED**.
- [ELASTIC] says several agents polling one team "share the 20 requests-per-minute budget". **L**
- **Implement one limiter per (team, endpoint), plus a global ceiling of 20/min per team for all non-events Admin routes combined.**

### 2.2 Status codes and error bodies
- **429:** Admin and Organization API responses include `Retry-After: 60` and the body `{"code":"error","message":"Rate limit exceeded"}`. **H** [DOC-API]
  - The Analytics 429 body is different: `{"error":"Too Many Requests","message":"Rate limit exceeded. Please try again later."}`. **H** [DOC-ANALYTICS]
- **Error bodies are not uniform. Parse only the HTTP status, and keep `message` (or `error`) for display.** **H**
  - Generic: `{"error":"<Status>","message":"…"}` [DOC-API]
  - Model access and Grok Bot: `{"code":"error","message":"…"}` [DOC-ADMIN]
  - remove-member: `{"error":"…"}` only
  - user-spend-limit: `{"outcome":"error","message":"…"}`
- **401** means one of several things, not always a bad key:
  - an invalid key; **H**
  - **a key missing the required scope** (documented for directory groups, Grok Bot and model access); **H**
  - a request to `conversation-insights` when "Disable Conversation Insights" is on. **H**
- **403** also has several meanings:
  - "Valid API key but insufficient permissions (e.g., Enterprise features on non-Enterprise plan)", with the message `"Enterprise access required"`; **H**
  - a feature that is not yet rolled out (for example the bulk spend-limit route); **H**
  - [DOC-ORG]: a scope mismatch fails "typically 401 or 403". **H**
- **400:**
  - invalid parameters, e.g. `"Some users are not in the team"`;
  - an unrecognized `hostingType` on filtered-usage-events.
  - **H**
- **500:** the generic body; "contact support if persistent". **H** Forum posts show 500s from both usage endpoints. **L**
- `X-RateLimit-*` headers are not documented. **UNVERIFIED**

### 2.3 Retries and backoff
- Retry 429, 500, 502, 503 and 504 at most 5 times. **M** [AIRBYTE] (max_retries 5)
- Wait for `Retry-After`, which can be seconds or an HTTP-date. [DEVLAKE retry_after.go] handles both.
- Without `Retry-After`, back off exponentially: 1, 2, 4, 8, 16 s. That is the backoff in the docs' example. [DOC-API]
- Do not retry 400, 401 or 403.

### 2.4 Polling and caching
- **Polling:** poll `/teams/daily-usage-data` and `/teams/filtered-usage-events` at most once an hour, because data is aggregated hourly. **H** The same applies to `/organizations/pooled-usage` and `/organizations/filtered-usage-events`. **H**
- AI Code Tracking is "ingested in near real-time but polling every few minutes is sufficient". **H** [DOC-API]
- **ETag caching** is documented only for Analytics (team-level and by-user) and AI Code Tracking:
  - `ETag` / `If-None-Match` / `Cache-Control: public, max-age=900`;
  - a 304 has no body and does not count against the rate limit.
  - **H**
- ETag on Admin `/teams/*` is **UNVERIFIED** (not documented).
- Analytics guidance: use `YYYY-MM-DD` or shortcuts such as `7d` rather than full timestamps, so requests can hit the cache. **H**

---

## 3. Admin API endpoints used by the connector

### 3.1 `GET /teams/members`
No body and no pagination. **H**

Response (verbatim example):
`{"teamMembers":[{"id":"user_PDSPmvukpYgZEDXsoNirw3CFhy","name":"Alex","email":"developer@company.com","role":"member","isRemoved":false}, …]}`

| Field | Type | Conf | Notes |
|---|---|---|---|
| `id` | string `user_…` | H | "This route always returns the `user_…` form." It encodes the same user as the plain number returned by `GET /organizations/members` and by OTel `cursor.user.account_id`. It used to be numeric [DOC-OLD]. Cursor staff confirmed `/teams/members` and `/teams/spend` use the encoded form, while dashboard exports and the org API use the number [FORUM-IDS, L]. |
| `email`, `name` | string | H | PII. |
| `role` | string, e.g. `member`, `owner` | H; full value set UNVERIFIED | The docs also mention `free-owner` as an admin role, in the remove-member rules. |
| `isRemoved` | boolean | H | |
| `userId` | string | L | Only in the [AIRBYTE] schema. |

- **Removed members** are inconsistent:
  - the forum reports they are omitted [FORUM-REMOVED, L];
  - an older staff reply says they "may still appear … for usage tracking and billing purposes" [L].
- Count licensed seats as rows where `!isRemoved`.

### 3.2 `POST /teams/daily-usage-data`
Limited to 20/min per team; poll at most hourly. **H**

**Request body:**

| Field | Type | Req | Conf | Notes |
|---|---|---|---|---|
| `startDate` | number, epoch ms | required | H | |
| `endDate` | number, epoch ms | required | H | Whether the bound is inclusive is **UNVERIFIED**. The "both bounds inclusive" text exists only for filtered-usage-events. |
| `page` | number, starting at 1 | optional | H | Paging is on only when **both** `page` and `pageSize` are sent. |
| `pageSize` | number | optional | H; maximum **UNVERIFIED** | "Number of **users** per page." The docs example uses 1000. The org variant documents 1-1000 with a default of 1000. |

**Rules:**
- **Date range cannot exceed 30 days.** **H**
- Without paging, only active users come back. With `page` and `pageSize`, the response covers all members who held a membership during the range, adds `isActive` per user per day, and includes `pagination`. Members who joined after the period are excluded. **H**
- **Always send `page` and `pageSize`** so inactive members are counted (the denominator for adoption rates).
  - [AIRBYTE], [OFERSHAP] and [DEVLAKE] all send pageSize 100. [OBSERVER] uses the unpaged form.
- **Pagination counts users, not rows.** In paged mode each user appears to get one row per day in the range, as the inactive-user row in the example suggests. Response size is therefore roughly `pageSize × days` rows: 1000 users × 30 days is about 30,000 rows in one response. *(Inference, L.)*
  - For incremental runs use ≤7-day windows, or `pageSize` 100-200.
- `subscriptionIncludedReqs`, `usageBasedReqs` and `apiKeyReqs` "count raw usage events, not billable request units". For billable units, sum `requestsCosts` from filtered-usage-events. **H**
- Relative strings such as `"7d"` or `"now"`:
  - appear only in the [GJKAUR] tutorial and are not documented for this POST;
  - are documented only for audit logs, Analytics and AI Code Tracking query strings.
  - Do not use them here.
- Removed members are reported missing from this endpoint while still present in filtered-usage-events. **L** [FORUM-REMOVED]
- Staff say `email` "is never empty" in this report, and there are no plans to add a user ID. **L** [FORUM-SCHEMA]

**Response:** see the verbatim example in [DOC-ADMIN].
- Top-level fields: `data[]`, `period{startDate,endDate}` (ms) and, in paged mode only, `pagination{page,pageSize,totalUsers,totalPages,hasNextPage,hasPreviousPage}`.
- An inactive row has zero counts and `null` strings.

| Field | Type | Conf | Notes |
|---|---|---|---|
| `userId` | **number** per the team docs | M; live type **UNVERIFIED** | The type conflicts across sources:<br>- Team docs, [AIRBYTE] and [METORIAL] say number.<br>- [DOC-ORG] (org variant) says a `user_…` string.<br>- [DEVLAKE] decodes it into a Go `string` (which would fail on a JSON number) and its fixture has `"user_example123"`.<br>Decode it as string-or-number and **do not join on it**; join on `email`. |
| `day` | string `YYYY-MM-DD` | H | Time zone **UNVERIFIED**. The example `date` 1710720000000 is 2024-03-18T00:00Z (I checked), which fits UTC. The dashboard lets users pick a time zone [DOC-DASH]; the API does not document one. |
| `date` | number, epoch ms | H | |
| `email` | string | H | PII. Airbyte's primary key is **(`date`, `email`)**; the original spec said `email`, `day`. |
| `isActive` | boolean | H | "Only present with pagination", although the unpaged example shows it too. Use `*bool`. |
| `totalLinesAdded`, `totalLinesDeleted` | number | H | All lines changed in the editor [OFERSHAP guide, L]. |
| `acceptedLinesAdded`, `acceptedLinesDeleted` | number | H | Undercounts agent-mode writes [OFERSHAP guide, L]. |
| `totalApplies`, `totalAccepts`, `totalRejects`, `totalTabsShown`, `totalTabsAccepted`, `composerRequests`, `chatRequests`, `agentRequests`, `cmdkUsages`, `subscriptionIncludedReqs`, `apiKeyReqs`, `usageBasedReqs`, `bugbotUsages` | number (int) | H | |
| `mostUsedModel`, `applyMostUsedExtension`, `tabMostUsedExtension`, `clientVersion` | string or null | H | |
| `completions`, `premiumRequests` | number | L | Only in the [DEVLAKE] fixture and struct. That fixture also **lacks** `date`, `cmdkUsages`, `apiKeyReqs`, `bugbotUsages` and the extension fields, so **treat every field as optional**. |

### 3.3 `POST /teams/filtered-usage-events`
Limited to 60/min per team; poll at most hourly. **H**

**Request body:** every field is optional, and filters combine with AND. **H**
- `startDate`, `endDate`: epoch ms, **both bounds inclusive**. For non-overlapping daily windows, set `endDate` to the last millisecond of the day (`…T23:59:59.999Z`).
- `page`: starts at 1; default 1.
- `pageSize`: default **100**, maximum **1000**.
  - The older docs defaulted to 10 [DOC-OLD]. The org endpoint still documents 10.
  - Always send it explicitly.
- `email`: string. A forum post reports "Email not valid" for removed users [L].
- `userId`: a **number**. Whether the encoded `user_…` form is accepted is **UNVERIFIED**.
- `serviceAccountId`: string.
- `cloudAgentId`: string; `*` matches all.
- `automationId`: string (UUID); `*` matches all.
- `hostingType`: one of `CLOUD`, `SELF_HOSTED`, `SELF_HOSTED_POOL`, `SELF_HOSTED_MACHINE`. Any other value returns 400. It covers inference spend only.

**Range and history:**
- The **maximum date range is UNVERIFIED**; none is documented.
  - The docs' own example window is exactly 30 days (2,592,000,000 ms).
  - [DEVLAKE] and [AIRBYTE] (`step: P30D`) both chunk into 30-day windows. [OBSERVER] records 30 days as an assumption.
  - [OFERSHAP via tomevault] claims "max 30 days".
  - Use windows of 30 days or less, and one UTC day per window for incremental syncs.
- **How far back history goes is UNVERIFIED.**
  - [AIRBYTE]: "rolling window", no length given.
  - [CHECKPOINT]: "usage history limited to the last 30 days" (L).
  - [OFERSHAP guide]: "full billing cycle history and no retention window" (L).
  - [DEVLAKE] backfills 90 days.
  - On first connect, attempt a 90-day backfill. Record the earliest day that returned rows, and treat empty older windows as "beyond retention", not as zero usage.

**Response:** see the verbatim example in [DOC-ADMIN]: `totalUsageEventsCount`, `pagination{numPages,currentPage,pageSize,hasNextPage,hasPreviousPage}`, `usageEvents[]`, `period{startDate,endDate}`.

| Field | Type | Conf | Notes |
|---|---|---|---|
| `timestamp` | **string** of epoch ms | H | [OBSERVER] also accepts a bare number. |
| `userEmail` | string | H | PII. May be missing for later-removed users [FORUM-REMOVED, L]. A staff reply says it is present for former members [L]. [DEVLAKE] **skips** events with no email. **Do not drop them**: they are spend. Store them under an "unknown user" bucket. |
| `serviceAccountId`, `serviceAccountName` | string; omitted for human users | H | The [DEVLAKE] fixture contains the literal string `"null"` (L). Normalize it to empty. |
| `cloudAgentId`, `automationId` | string; omitted when not applicable | H | |
| `conversationId` | string; omitted when there is no conversation | H | The docs call it a join key to AI Code Tracking. That API exposes conversation ids only on the limited-alpha commit-details endpoint, and its example id is `conv_abc123`, not a UUID. **Whether the join works is UNVERIFIED.** |
| `model` | string | H | |
| `kind` | string, e.g. `Usage-based`, `Included in Business` | H; full value set UNVERIFIED | [OFERSHAP] also reads `kindLabel` / `kind_label` (L). |
| `maxMode`, `isTokenBasedCall`, `isChargeable`, `isHeadless` | boolean | H | |
| `requestsCosts` | number; can be fractional (1.4 in the docs) | H | Use `float64`. |
| `tokenUsage` | object or undefined | H | The docs say it is "present when `isTokenBasedCall` is `true`". **However**, the [DEVLAKE] e2e fixture (a captured payload from 2026-07-09) has `isTokenBasedCall:false` **with** a full `tokenUsage` (`totalCents` 33.70615), plus `chargedCents` 8 and `requestsCosts` 2 [L]. **Decode as a pointer and record tokens whenever it is non-nil.** |
| `tokenUsage.{inputTokens,outputTokens,cacheWriteTokens,cacheReadTokens}` | number (int) | H | |
| `tokenUsage.totalCents` | number | H | "Total model cost in cents". Before the discount and the Cursor Token Rate (inferred from the examples, L). |
| `tokenUsage.discountPercentOff` | number or undefined | H | |
| `chargedCents` | number | H | The amount charged; it includes the Cursor Token Rate when one applies. "Use this field to reconcile event-level costs with `/teams/spend` totals." Works on both token-based and request-based plans. |
| `cursorTokenFee` | number or undefined | H | Present only for third-party models, including Auto routing to one. Absent for first-party models (Grok, Composer) and for request-based enterprise accounts. |
| `hostingType`, `isFreeBugbot` | | L | Only in [DEVLAKE] / [METORIAL] structs. Undocumented. |

- **Cost formula (L, my inference, checked arithmetically):** `chargedCents ≈ totalCents × (1 − discountPercentOff/100) + cursorTokenFee`.
  - 20.18232 + 1.18 = 21.36232
  - 40.167 × 0.9 + 1.18 = 37.3303 ≈ 37.33
  - Request-billed events do not follow it: in the DevLake fixture, `chargedCents` 8 = 2 × 4 ¢.
- `isChargeable:false` can occur together with `chargedCents > 0` (third docs example). **H**
- Event ordering: the example is newest first, but ordering is **UNVERIFIED**.
- **There is no event id.**

### 3.4 `POST /teams/spend`
Limited to 20/min. **H**

- **Request** (all fields optional; `{}` is valid per the docs):
  - `searchTerm`: matches names and emails.
  - `sortBy`: `amount`, `date` or `user`; default `date`.
  - `sortDirection`: `asc` or `desc`; default `desc`.
  - `page`: default 1.
  - `pageSize`: default and maximum **UNVERIFIED**. Send 100, as [AIRBYTE], [DEVLAKE], [OBSERVER] and [OFERSHAP] do.
- The response does not echo the page number. Paginate while `page < totalPages`. **H** (docs example; [OFERSHAP], [OBSERVER])
  - [DEVLAKE] stops on `pagination.hasNextPage`, which spend never returns, so DevLake reads only page 1. Do not copy that.
- **Response** (verbatim example in [DOC-ADMIN]): `{teamMemberSpend[], subscriptionCycleStart, totalMembers, totalPages}`.
  - **H** fields: `userId` (`user_…`, the same namespace as `teamMembers[].id`), `name`, `email`, `role`, `spendCents`, `overallSpendCents`, `fastPremiumRequests`, `hardLimitOverrideDollars`, `monthlyLimitDollars`, `effectivePerUserLimitDollars`.
  - `spendCents`: on-demand spend only (excludes included usage).
  - `overallSpendCents`: on-demand plus included usage.
  - Precision was raised on 2026-06-04: use `float64` and `NUMERIC`.
  - `fastPremiumRequests`: "Number of usage-based premium requests made during the billing cycle". The original spec's word "legacy" was its own inference.
  - `hardLimitOverrideDollars`: 0 means no override.
  - `monthlyLimitDollars`: number or **null**.
  - `subscriptionCycleStart`: epoch ms. The legacy [METORIAL] interface types it as a string, so decode it as FlexInt64.
- Fields found only in third-party sources (L): `includedSpendCents`, `billingTier`, `removedAt`, `apiPercentUsed`, `autoPercentUsed`, `totalPercentUsed`, `profilePictureUrl`, `usagePromoExpiresAt` ([AIRBYTE] schema); `limitedUsersCount` ([OFERSHAP]).
- Covers the current billing cycle only. There are no date parameters. **H**

### 3.5 `GET /teams/audit-logs` (optional)
- **Query parameters:** **H**
  - `startTime`, `endTime`: default 7 days ago and now.
  - `eventTypes`: comma-separated.
  - `search`: matches `user_email`, `event_type` and `event_id`; not `event_data`.
  - `page`: default 1.
  - `pageSize`: 1-500, default 100.
  - `users`: emails, `user_…` ids or numbers. Email matching is **case-sensitive**. At most `pageSize` users per request.
- **Date formats:** `now`, `today`, `yesterday`, `Nd`, `Nh`, `Ns`, ISO 8601, `YYYY-MM-DD` (00:00 UTC), or Unix seconds or milliseconds. The range must be 30 days or less. **H**
- **Response:** `{events[{event_id, timestamp (ISO), ip_address, user_email, event_type, application_type, event_data}], pagination{page,pageSize,totalCount,totalPages,hasNextPage,hasPreviousPage}, params{teamId,startDate,endDate}}`. Events come oldest first. **H**
- **Retention:** 30 days. **L** [ELASTIC]
- Do not store `ip_address` or `event_data`.

---

## 4. Newer and adjacent APIs (same base URL, Basic auth)

### 4.1 Analytics API (Enterprise only) [DOC-ANALYTICS]
- **Team-level endpoints:** `GET /analytics/team/{agent-edits, tabs, dau, client-versions, models, top-file-extensions, mcp, commands, plans, skills, ask-mode, conversation-insights, leaderboard, bugbot, bugbot-reviews}`. **H**
  - `bugbot-reviews` was missing from the original spec. It needs `read:*`; all the others need `admin:*`.
- **Common parameters:** **H**
  - `startDate` / `endDate`: `YYYY-MM-DD`, ISO, `now` / `today` (current date at 00:00), `yesterday`, or `Nd`. The default is the last 7 days. Times are truncated to 00:00 UTC. **Ranges are limited to 30 days.**
    - The best-practices section also says "e.g., 1-3 months"; which limit the server enforces is **UNVERIFIED**.
    - Whether the `endDate` day itself is included is **UNVERIFIED**.
  - `users`: emails or `user_…` ids. An `@` marks an email; the `user_` prefix marks an id.
- **Paging:**
  - Leaderboard: `page`, `pageSize` (default 10, max 500).
  - Bugbot: `pageSize` default 100, max 250.
  - Bugbot-reviews: `pageSize` default 100, max 250.
  - Bugbot also takes `prState` (`merged` or `all`) and `repo`.
  - **H**
- **Response shapes** (snake_case): **H**
  - `dau`: `data[]` of `{date, dau, cli_dau, cloud_agent_dau, bugbot_dau}`; `params` holds `{metric, teamId (number), startDate, endDate}`.
  - `agent-edits`: `{event_date, total_suggested_diffs, total_accepted_diffs, total_rejected_diffs, total_green_lines_accepted, total_red_lines_accepted, total_green_lines_rejected, total_red_lines_rejected, total_green_lines_suggested, total_red_lines_suggested, total_lines_suggested, total_lines_accepted}`.
  - `tabs`: the same, but with `total_suggestions`, `total_accepts` and `total_rejects` in place of the three `*_diffs` fields.
  - `models`: `{date, model_breakdown:{"<model>":{messages,users}}}`.
- **By-user endpoints:** `GET /analytics/by-user/{agent-edits, tabs, models, top-file-extensions, client-versions, mcp, commands, plans, skills, ask-mode}`. They take `page` (default 1) and `pageSize` (default 100, max 500). **H**
  - Response: `{data:{"<email>":[…]}, pagination{page,pageSize,totalUsers,totalPages,hasNextPage,hasPreviousPage}, params{…, userMappings:[{id,email}]}}`.
- **Rate limits:** 100/min (team-level) and 50/min (by-user), per team. The 429 body is the generic `{"error","message"}`. ETag caching is supported on both. **H**
- **conversation-insights:** the docs say it returns aggregate insights, "not raw conversation exports or raw conversation content". It returns **401** when Conversation Insights is disabled. **H**
  - It is still derived from conversation content, so **do not ingest it** under the metadata-only principle. This is a policy decision.
- **bugbot-reviews:** returns finding `title`, `description` and `locations` for dry-run reviews. That is code-derived content: **do not ingest it.**

### 4.2 AI Code Tracking API (Enterprise, **Alpha**; response shapes may change) [DOC-AICODE]

**Common parameters:** **H**
- `startDate`: ISO, `now` or `Nd`; default now − 7 d.
- `endDate`: ISO, `now` or `0d`; default now.
- `page`: default 1.
- `pageSize`: default 100, max 1000. Not accepted on the CSV routes.
- `user`: one email, `user_…` or numeric id.

**`GET /analytics/ai-code/commits`:** returns `{items[], totalCount, page, pageSize}`. **H**

| Item field | Type |
|---|---|
| `commitHash`, `userId` (`user_…`), `userEmail` | string |
| `repoName`, `branchName` | string or null |
| `isPrimaryBranch` | boolean or null, **and may be absent**. Use `*bool`. |
| `commitSource` | `ide`, `cli` or `cloud` |
| `totalLinesAdded`, `totalLinesDeleted`, `tabLinesAdded`, `tabLinesDeleted`, `composerLinesAdded`, `composerLinesDeleted` | number |
| `nonAiLinesAdded`, `nonAiLinesDeleted` | number or null; computed as `max(0, total − AI)` |
| `message` | string or null. The commit message: **do not store it.** |
| `commitTs` | ISO string or null. Not changed when a commit is amended. |
| `createdAt` | ISO string; the ingestion time |

**`GET /analytics/ai-code/changes`:** returns `{items[], totalCount, page, pageSize}`. **H**
- Item fields: `changeId`, `userId`, `userEmail`, `source` (`TAB` or `COMPOSER`), `model` (string or null), `totalLinesAdded`, `totalLinesDeleted`, `createdAt`, `metadata[]`.
- `metadata[]` entries: `{fileName, fileExtension, linesAdded, linesDeleted}`. `fileName` is omitted in privacy mode.
- **Do not store `fileName`**; it is a repository path. `fileExtension` is fine.

**`.csv` variants** of both routes: they stream in 10,000-record pages on the server side. **H**

**`GET /analytics/ai-code/commits/:commitHash[,…]`:** **limited alpha, select users only.** **H**
- It returns `rangeAnnotations` (file paths and line ranges) and `conversations[]` with `title`, `tldr`, `overview` and `summaryBullets`.
- **That is content-derived: do not ingest it.**

**Caveats:** **H**
- "Commit hashes are not unique or unchangeable"; amended commits can appear twice. Deduplicate on (`commitHash`, `repoName`, `userId`) and keep the latest `createdAt`.
- Metrics cover only the git repo at the top level of the workspace root; multi-root workspaces are not supported.
- Some fields are absent when the client has privacy mode on.

**Rate limit and caching:** 20/min per endpoint; ETag supported. **H**

**Attribution reliability:** squash merges and initial commits show up as `nonAi` lines. **L** [OFERSHAP guide]

### 4.3 Organization API (Organization key) [DOC-ORG]
Out of scope for v1.

**Routes:** **H**
- `POST /organizations/{daily-usage-data, filtered-usage-events, spend, pooled-usage}`
- `GET /organizations/members`

**Scopes:** **H**
- Usage routes need `usage:*` or `admin:*`.
- Members need `members:read` or higher.

**Common body rules:** **H**
- `organizationId` (`org_…`) is required and must match the key.
- `teamIds` (`number[]`) is optional:
  - "at most 100" is documented for `daily-usage-data` and `spend`, not for events;
  - every team id must belong to the organization.

**`/organizations/daily-usage-data`:** **H**
- `startDate` and `endDate` are **optional** (defaults: 7 days ago and now), unlike the team route.
- `pageSize` 1-1000, default 1000.
- Takes a `userEmail` (or `userEmails`) filter.
- Rows carry `teamId`.
- `userId` is a `user_…` string.
- The range is limited to 30 days.

**`/organizations/filtered-usage-events`:** **H**
- `pageSize` defaults to 10.
- No `cloudAgentId`, `automationId` or `hostingType` filters are documented.
- Events carry `teamId`.
- Sum `chargedCents` to reconcile with `pooled-usage` `usedCents`.

**`/organizations/spend`:** this shape **differs** from the team route, so **the spend types cannot be shared**. **H**
- The window is the organization contract window, returned in `period`.
- `sortBy`: `email`, `name` or `spendCents`; default `email asc`.
- `pageSize`: 1-1000, default 100.
- It omits `subscriptionCycleStart`, `overallSpendCents`, `fastPremiumRequests`, `hardLimitOverrideDollars` and `monthlyLimitDollars`.
- The daily-usage and event row types can be shared if `TeamID *int64` is added.

**`/organizations/members`:** `pageSize` is capped at 200. `id` is numeric. **H**

---

## 5. Go connector design (`backend/internal/cursor/`, modeled on `backend/internal/github/`)

### 5.1 Types
Keep the original type set (`FlexString`, `FlexInt64`, `Member`, `DailyUsageRequest`, `DailyUsage`, `UsageEventsRequest`, `TokenUsage`, `UsageEvent`, `EventsPagination`, `SpendRequest`, `MemberSpend`), with these changes:
- `UsageEvent.TokenUsage *TokenUsage`: record it whenever it is non-nil, even when `IsTokenBasedCall` is false (see 3.3).
- `UsageEvent.ServiceAccountID`: normalize the string `"null"` to empty. `FlexString` already does this.
- `spendResponse.SubscriptionCycleStart`: use `FlexInt64`, not `int64`.
- `DailyUsage`: every field may be absent. Leave zero values for counts and pointers for strings. Note the [DEVLAKE] fixture has no `date` field, so **derive the day from `day` (parse it as a UTC date) and fall back to `date`.**
- Optionally add `UserID FlexString json:"userId,omitempty"` to `UsageEventsRequest`. The docs type it as a number, so send the numeric form only.
- Do not use `DisallowUnknownFields`; payloads gain fields over time ([AIRBYTE] sets `additionalProperties: true`).
- AI Code Tracking types: `IsPrimaryBranch *bool`, `NonAiLinesAdded *int64`, `NonAiLinesDeleted *int64`, `CommitTs *string`. Do **not** decode `Message` at all.

### 5.2 HTTP client
- Use `&http.Client{Timeout: 30 * time.Second}`, like `ghClient` in `backend/internal/github/commits.go:59`. **verified [REPO]**
- Auth: `req.SetBasicAuth(key, "")`. Always send a JSON body on POST (`{}` at minimum).
- **Key storage:** encrypt the key at rest with `crypto.Encrypt` / `crypto.Decrypt` in `backend/internal/crypto/aes.go`, as `organizations.github_access_token` is in `queries.go`. **verified [REPO]**
- Never log the key, and never put it in a URL.
- **Read-only by construction:** build only `GET /teams/members`, `POST /teams/{daily-usage-data, filtered-usage-events, spend}` and optionally `GET /teams/audit-logs` and `GET /analytics/*`. Never construct a write route.
- **Retries:** see 2.3. Cap retries on 500 at 2. A 500 can mean an empty body or the wrong key type (L).
- **Error mapping:**
  - 401: "invalid key, or key scope too narrow (try an Admin key)".
  - 403: "not available on this plan or not enabled for this team".
  - In both cases include the server's `message` (or `error`) text.

### 5.3 Sync algorithm (UTC days)
1. **Members:** fetch once per run. This doubles as the connection test (Airbyte's `check` stream is `members`). Seats = count of members where `!isRemoved`.
2. **Daily usage:**
   - **Incremental:** window [00:00 UTC of today − 7 days, now] (DevLake uses 7 days, Airbyte 2). Always send `page` (from 1) and a `pageSize` of 100-200; loop on `pagination.hasNextPage`.
   - **Backfill:** windows of 30 days or less (`endDate − startDate ≤ 30·86 400 000 − 1` ms). Use smaller windows for large teams (rows ≈ users × days).
   - Upsert raw rows on (org, `day`, lower(email)), **replacing** the old values.
3. **Usage events:**
   - One UTC day per window: `startDate` = 00:00:00.000, `endDate` = 23:59:59.999, both inclusive.
   - On every hourly run, re-fetch today and the previous 1-2 days. (Airbyte looks back 1 day; DevLake 7.)
   - `pageSize` 1000; loop on `hasNextPage`.
   - **Fetch every page into memory before writing. Check that the event count equals `totalUsageEventsCount`.** Then replace the day in one transaction: `DELETE … WHERE org=$1 AND event_day=$2`, then insert. If the count does not match (events can land mid-pagination on "today"), keep the existing rows and retry on the next run.
   - The API has no event id. Use replace-by-window and do not upsert on a hash.
   - If you need a per-row key, the **actual** prior art is:
     - [DEVLAKE]: `sha256(timestamp|userEmail|conversationId|model|chargedCents|requestsCosts)`, no occurrence counter;
     - [OBSERVER]: `sha256(identity|conversationId|tsMs|model|outputTokens[|n])`, where `n` is a repeat counter **within one page**, truncated to 16 bytes.
     - Both can collide on identical events. Observer's counter depends on page boundaries.
     - (The original spec wrongly merged these into one recipe.)
   - Throughput ceiling: 60 req/min × 1000 rows ≈ 60k events/min.
4. **Spend:** snapshot once per run, keyed by (org, `subscriptionCycleStart`, lower(email), captured date). Loop pages until `page >= totalPages`.

### 5.4 Cost semantics
- Per-event billed cost: `chargedCents`.
  - The docs say summing it reconciles with `/teams/spend` totals.
  - **UNVERIFIED** whether that means `overallSpendCents` or `spendCents`. `overallSpendCents` is the more likely, since `chargedCents` covers included usage too (the docs' `Included in Business` event has `chargedCents` 8).
- Model-only cost: `tokenUsage.totalCents`.
- Cursor markup: `cursorTokenFee`.
- Billing-unit volume: `requestsCosts`.
- **Tokens:** sum them whenever `tokenUsage` is present. A nil `tokenUsage` means "no token data", not 0 tokens.
- Store money as `NUMERIC(14,6)`. `COALESCE` nullable SQL aggregates before scanning them into non-pointer Go types (repo convention).

### 5.5 Privacy-first mapping and AI Stats integration
**Data to drop:**
- AI Code Tracking: `message`, `metadata[].fileName`, and everything from commit-details.
- Audit logs: `ip_address` and `event_data`.
- Analytics: `conversation-insights`, and bugbot-reviews finding text and locations.

**Personal data handling:**
- `email`, `name` and `userEmail` are PII.
- Default (team-level) mode: aggregate per org per day before storing.
- Opt-in per-user mode: store a keyed hash of the lower-cased email.

**Writing team-level rows to `tool_usage_snapshots` (`tool_source='cursor'`).** The table is verified in [REPO] migration 002 plus 004 and 007.

**Critical issue:** `Queries.UpsertToolSnapshot` (`backend/internal/db/queries.go` ~line 716) is **additive** on conflict.
- `total_suggestions`, `total_acceptances`, `total_lines_accepted`, `total_planning_uses` and `total_coding_uses` are summed with the stored values. The active, licensed and coding-developer counts take `GREATEST`.
- It was built for the push-ingest path, which receives incremental batches.
- A pull connector that re-fetches the trailing 7 days every hour would **multiply the counts**.
- **Add a replace-semantics query** (e.g. `ReplaceToolSnapshot` with `DO UPDATE SET col = EXCLUDED.col`), or recompute each day from raw rows and overwrite it.

**Columns to fill:**
- `total_suggestions` / `total_acceptances`:
  - Copilot fills these from completion suggestions and acceptances only ([REPO] `github/copilot.go`).
  - For a like-for-like comparison, use Σ`totalTabsShown` / Σ`totalTabsAccepted`, and put Σ`totalApplies` / Σ`totalAccepts` in `raw_json`.
  - The original proposal of tabs + applies is an alternative (L, design choice).
- `total_lines_accepted` = Σ`acceptedLinesAdded`. It undercounts agent mode.
- `total_active_users` = number of rows with `isActive` (paged mode).
- `total_licensed_users` = number of members where `!isRemoved`.
- `total_coding_developers` (migration 004): the denominator of all committers. Cursor cannot supply it. Leave it at 0, or let the GitHub sync own it.
- `total_planning_uses` / `total_coding_uses` (migration 007): a suggested mapping is planning = `chatRequests`, coding = `agentRequests + composerRequests + cmdkUsages` (L, design proposal; check against `handlers/ingest.go` semantics).
- `raw_json`: aggregates only, with no emails.

## Verification notes

Overall the original spec is accurate on the core request and response contracts. I re-read every endpoint path, method, parameter and response field for /teams/members, /teams/daily-usage-data, /teams/filtered-usage-events, /teams/spend, /teams/audit-logs, the Analytics API, the AI Code Tracking API and the Organization API. The source was a docs mirror synced 2026-10-23 21:19 UTC (0 failed, 0 stale pages), cross-checked against a byte-identical second mirror and an older snapshot. All verbatim examples, limits, defaults, the 429 shape and Basic auth match.

**Errors I found** (none changes an endpoint contract):
- **Additive snapshot upsert (most serious).** The repo's existing UpsertToolSnapshot is additive. Re-fetching trailing days, as the spec recommends, would double-count unless a replace-semantics query is added.
- **tokenUsage on request-billed events.** A captured DevLake payload has tokenUsage on an isTokenBasedCall:false event, so gating token counts on that flag drops data.
- **Wrong dedup-hash attribution.** The recipe the spec credits to DevLake and observer matches neither client.
- **Circular confidence.** Observer was cited as independent confirmation but is built only from the docs.
- **Error-body and status mapping.** Error bodies are not uniform, and a missing scope returns 401 on the documented routes.
- **Analytics omissions.** The spec left out /analytics/team/bugbot-reviews and the per-endpoint pageSize limits.
- **AI Code Tracking omissions.** Missing were non-unique commit hashes, an absent isPrimaryBranch, the /changes fields and file paths, and that commit-details is limited alpha with content-derived data.
- **Org API detail.** The org spend shape differs from the team route, so the types cannot be shared.
- **Minor attribution slips.** These are the Airbyte primary key ([date, email]) and the claim that `{}` is valid "per Airbyte".

**Still UNVERIFIED** after this pass:
- **read:* scope on the usage routes.** Third-party DX and Speakeasy guides say Read-only team keys work; Coralogix says admin:*.
- **Range and retention.** The maximum date range and the history retention for filtered-usage-events (30-day chunks are the common practice).
- **Daily usage.** The live type of daily-usage userId, the time zone of `day`, whether endDate is inclusive, and the maximum pageSize. The same questions apply to /teams/spend pageSize.
- **Spend reconciliation.** Whether summed chargedCents matches overallSpendCents or spendCents.
- **Removed members.** Their handling (forum reports conflict).
- **Event ordering and page stability.** The order of usageEvents and whether pages stay stable mid-window.
- **Caching and rate-limit scope.** ETag on /teams/*, whether the Admin 20/min budget is per endpoint or shared, and whether X-RateLimit headers exist.
- **conversationId join.** Whether event conversationIds join to AI Code Tracking.

**Source limits.** cursor.com, forum.cursor.com, docs.getdx.com, speakeasy.com and help.nops.io were all unreachable (proxy refusal, plus DNS failure under WebFetch). Forum and vendor evidence is therefore from search snippets only and rated L. Before release, capture one live response per endpoint with a real key to settle the unverified items.

Corrections made during verification:

- **Claim:** Error bodies are always {"error": "<Status text>", "message": "<detail>"} [DOC-API]  
  **Finding:** That is only the generic shape. The Admin page documents other shapes. Model-access and Grok Bot routes use {"code":"error","message":...}. The Admin/Org 429 body is {"code":"error",...}. remove-member returns {"error":...} only. user-spend-limit returns {"outcome":"error",...}. Branch on HTTP status and treat the body as display text only. (https://raw.githubusercontent.com/mehmetbaykar/cursor-docs-skill/main/skills/cursor-docs/references/account__teams__admin-api.md (Model access Errors, Grok Bot Errors, remove-member, user-spend-limit sections))
- **Claim:** Handle 403 'Enterprise access required' as 'not available on this plan'; treat 401 and 403 as 'check key, scope or plan'  
  **Finding:** The documented status codes are more complex. A missing scope returns 401, not 403, on directory-group, Grok Bot and model-access routes ('A write with a read:* key returns 401'). conversation-insights returns 401 when Conversation Insights is disabled. 403 also covers features not yet rolled out (bulk spend limits). The org docs say a scope mismatch is 'typically 401 or 403'. A forum reply says an org key used on /teams/* can return 500. So 401 does not always mean a bad key, and 403 does not always mean the plan. (DOC-ADMIN (Team directory groups, Grok Bot, model access); DOC-ANALYTICS (Conversation Insights); DOC-ORG (Key differences); forum.cursor.com/t/165632 (WebSearch snippet))
- **Claim:** Whether a read:* key can call /teams/daily-usage-data, filtered-usage-events, spend or members is UNVERIFIED; assume admin:*. Required scope admin:* is H [DOC-API][AIRBYTE]  
  **Finding:** It is still UNVERIFIED against primary docs, but there is new third-party evidence. DX and Speakeasy setup guides tell customers to create the team key with scope 'Read-only' for usage and spend ingestion; DX lists /teams/daily-usage-data. Coralogix says to use admin:*. Also, the Airbyte manifest does not mention admin:* at all; it only says the key 'starts with crsr_'. The docs cite for admin:* is DOC-API alone. (WebSearch snippets of docs.getdx.com/connectors/cursor/, speakeasy.com/docs/ai-control-plane/org-admin/ai-integrations/cursor, coralogix.com/docs/integrations/ai-observability/cursor; airbyte manifest.yaml)
- **Claim:** Key format: crsr_ followed by 64 characters (H)  
  **Finding:** The docs placeholder does contain exactly 64 x's after crsr_. However, Elastic's Cursor integration README says 'The key format is key_ followed by hexadecimal characters'. Do not validate the key prefix. (api.md mirror; https://raw.githubusercontent.com/elastic/integrations/main/packages/cursor/docs/README.md)
- **Claim:** Dedup key: sha256(timestamp|userEmail|serviceAccountId|conversationId|model|kind|chargedCents|requestsCosts|tokens…) plus an occurrence counter, as in [OBSERVER] and [DEVLAKE]  
  **Finding:** Neither client uses that recipe. DevLake computeEventId = sha256(timestamp|userEmail|conversationId|model|chargedCents|requestsCosts), with no occurrence counter. Observer usageJoinValue = sha256(identity|conversationId|tsMs|model|outputTokens[|n]), where n is a repeat ordinal within one page, truncated to 16 bytes. The spec merged the two. (apache/devlake backend/plugins/cursor/tasks/collector_utils.go; superbasedapp/observer internal/vendorapi/cursor/usage.go)
- **Claim:** [OBSERVER] is an independent open-source client that raises claims to confidence H  
  **Finding:** Observer is derived from the docs. Its doc.go says the drivers were 'written from the public Cursor Admin API reference (fetched 2026-10-07), never from a live response… synthetic, awaiting live validation', and its fixtures are the docs' examples. Citing it as confirmation is circular. Its daily-usage driver also uses the UNPAGED, active-only form. (https://raw.githubusercontent.com/superbasedapp/observer/main/internal/vendorapi/cursor/doc.go and usage.go)
- **Claim:** tokenUsage is present only when isTokenBasedCall is true; count tokens only when isTokenBasedCall is true  
  **Finding:** The docs say tokenUsage is 'present when isTokenBasedCall is true', but they do not say it is absent otherwise. DevLake's e2e raw fixture (captured 2026-07-09) has isTokenBasedCall:false with a full tokenUsage (5932 in, 4761 out, 495701 cache-read, totalCents 33.70615), plus chargedCents 8 and requestsCosts 2. Gating tokens on isTokenBasedCall would drop token data on request-billed plans. Record tokens whenever tokenUsage is non-nil. (https://raw.githubusercontent.com/apache/devlake/main/backend/plugins/cursor/e2e/raw_tables/_raw_cursor_usage_events.csv)
- **Claim:** Suggested team-level mapping into tool_usage_snapshots (implicitly through the existing upsert), with re-fetching the trailing 7 days on each run  
  **Finding:** This is critical in the AI Stats repo. Queries.UpsertToolSnapshot (backend/internal/db/queries.go ~716) is additive on conflict: total_suggestions/acceptances/lines/planning/coding = old + new. Re-fetching the trailing days every hour would multiply the counts. A replace-semantics query is needed. Also, the table has columns total_coding_developers (migration 004) and total_planning_uses / total_coding_uses (migration 007) that the spec's mapping ignores. (backend/internal/db/queries.go; backend/internal/db/migrations/002_multi_tool_sources.sql, 004_coding_developers.sql, 007_planning_ratio.sql)
- **Claim:** daily-usage natural key is (email, day) [AIRBYTE primary key]  
  **Finding:** Airbyte's daily_usage primary_key is [date, email], where date is epoch ms. It is equivalent in meaning but not what the spec stated. (airbyte-integrations/connectors/source-cursor/manifest.yaml)
- **Claim:** `{}` is a valid /teams/spend body [AIRBYTE]  
  **Finding:** Airbyte never sends a literal {}: its paginator injects page and pageSize into the JSON body on the first request. {} is valid because the docs mark every spend parameter optional. Separately, a forum reply says a POST with no body at all (JSON content type, empty payload) can return 500, so always send a JSON body. (airbyte manifest.yaml (paginator inject_on_first_request); DOC-ADMIN spend params; forum.cursor.com/t/165632 (snippet))
- **Claim:** Analytics team-level endpoint list: agent-edits … leaderboard, bugbot, conversation-insights  
  **Finding:** The list omits /analytics/team/bugbot-reviews, which is exactly the endpoint that needs read:*. It also omits paging limits: leaderboard pageSize defaults to 10 (max 500); bugbot and bugbot-reviews default to 100 (max 250). bugbot-reviews returns finding title, description and locations for dry runs, which is code-derived content. (account__teams__analytics-api.md mirror)
- **Claim:** The generic 429 body is 'documented elsewhere'  
  **Finding:** The generic body {"error":"Too Many Requests","message":"Rate limit exceeded. Please try again later."} is the one documented for the Analytics API specifically. Admin and Org use {"code":"error","message":"Rate limit exceeded"} with Retry-After: 60. (account__teams__analytics-api.md (Rate Limits); api.md)
- **Claim:** conversationId can be joined to the AI Code Tracking API  
  **Finding:** The docs do say this. However, AI Code Tracking exposes conversation ids only through GET /analytics/ai-code/commits/:commitHash, which is 'limited alpha and only available to select users', and its example id is 'conv_abc123', while events use UUIDs. Whether the join works is UNVERIFIED. The same endpoint returns conversation title, tldr, overview and summaryBullets plus file paths, which are content-derived and must not be ingested. (account__teams__ai-code-tracking-api.md mirror)
- **Claim:** AI Code Tracking section lists commits fields and other routes only  
  **Finding:** Implementers need several documented details the spec left out. endDate accepts '0d'. isPrimaryBranch may be absent, not just null. 'Commit hashes are not unique or unchangeable' (amends produce duplicates). Multi-root workspaces are unsupported. Privacy mode omits fields. The /changes items have changeId, source TAB|COMPOSER, model and metadata[] with fileName, which is a file path. CSV routes do not take page/pageSize. Data is ingested in near real time. (account__teams__ai-code-tracking-api.md; api.md (Poll at Appropriate Intervals))
- **Claim:** Org API: body accepts teamIds (number[], at most 100); design so the team and org paths share types  
  **Finding:** 'At most 100 teams' is documented for /organizations/daily-usage-data and /organizations/spend, not for filtered-usage-events. Org daily-usage startDate/endDate are optional (defaults 7 days ago / now), unlike the team route. Org spend has a different shape: a contract window in 'period', sortBy email/name/spendCents (default email asc), pageSize 1-1000 (default 100), and no subscriptionCycleStart, overallSpendCents, fastPremiumRequests, hardLimitOverrideDollars or monthlyLimitDollars. Spend types cannot be shared. Each org key carries exactly one scope, and the scopes include auditlogs:read and models:read/models:*. (account__organizations__organization-admin-api.md mirror)
- **Claim:** A forum reply says the usage endpoints also work on the Teams plan  
  **Finding:** It is the original bug reporter (not a staff reply) who describes calling /teams/daily-usage-data on a Team plan, and with a Bearer admin key. Check Point says 'Teams or Enterprise'. Officially the overview lists Enterprise; the Admin page marks only its write routes 'Enterprise only'. Analytics and AI Code Tracking say Enterprise only on their own pages. (WebSearch snippets: forum.cursor.com/t/165528, Check Point Workforce AI guide; api.md; admin-api.md)
- **Claim:** Mirrors were identical except the directory-groups text and the description of the member id  
  **Finding:** mehmetbaykar and mrkhachaturov are byte-identical for the read-endpoint sections. RanulfoMNeto lacks the member-id/org-number sentence and also lacks the audit-log paragraph saying users email matching is case-sensitive. The primary mirror was synced 2026-10-23 21:19 UTC with 0 failed and 0 stale pages, so it is current. (diff of the three mirrors; mehmetbaykar docs_manifest.json)
- **Claim:** Audit logs: query params, date formats and response (no retention stated)  
  **Finding:** These are confirmed, with omissions. Shortcuts also include 'today' and 'yesterday'. users email matching is case-sensitive. The maximum number of users per request equals pageSize. Elastic's integration says the Admin API retains audit events for only 30 days (L). (admin-api.md mirror; elastic/integrations packages/cursor/docs/README.md)
- **Claim:** Retention of filtered-usage-events history is UNVERIFIED; ofershap claims no retention window  
  **Finding:** It is still UNVERIFIED. The ofershap guide does say 'full billing cycle history and no retention window' (line 194). Check Point says usage history is limited to the last 30 days (L). The docs' own events example uses a window of exactly 30 days (2,592,000,000 ms). Airbyte also chunks events into 30-day steps (P30D) and looks back 1 day. The dashboard shows 365 days of the Daily Usage chart, but that says nothing about the API. (ofershap .cursor/rules/cursor-api-data-guide.mdc; Check Point guide (snippet); airbyte manifest; account__teams__analytics.md)
- **Claim:** daily-usage userId type conflict; decode string-or-number and join on email  
  **Finding:** This is confirmed, with more evidence. DevLake decodes userId into a Go string (which would fail on a JSON number) and its fixture uses 'user_example123', which hints the live type is a string (L). Cursor staff said on the forum that email is the identifier for this report, 'is never empty', and that there are no plans to add a user ID. Joining on email is right. The DevLake fixture also lacks date, cmdkUsages, apiKeyReqs, bugbotUsages and the extension fields, so all fields should be optional and day should be derived from 'day'. (devlake daily_usage_extractor.go and _raw_cursor_daily_usage.csv; forum.cursor.com/t/133545 (snippet))
- **Claim:** Always send page and pageSize (1000) for daily-usage-data  
  **Finding:** Sending both is correct; it is the only way to get inactive members. But the docs define pageSize as 'Number of users per page', and paged mode returns per-user-per-day rows with isActive. A page can therefore hold about pageSize × days rows, roughly 30k for 1000 users × 30 days (inference). Airbyte, DevLake and ofershap all use pageSize 100. (admin-api.md daily usage parameters and example; airbyte/devlake/ofershap clients)
- **Claim:** userEmail may be missing for removed users; filtered-usage-events (DevLake handling not mentioned)  
  **Finding:** DevLake's extractor skips events with an empty userEmail (logs a warning and drops the row). That would under-report spend. Keep those events in an 'unknown user' bucket. A staff reply in search results says filtered-usage-events returns userEmail for former members, which conflicts with the bug report. (devlake usage_events_extractor.go; forum snippets 161589 / 167128)
- **Claim:** spend pagination: Paginate while page < totalPages [OBSERVER][OFERSHAP]; send pageSize 100 [DEVLAKE][OBSERVER][AIRBYTE]  
  **Finding:** This is confirmed. Note that DevLake's spend collector stops on pagination.hasNextPage, which /teams/spend never returns, so DevLake reads only page 1; do not copy it. Metorial's legacy interface types subscriptionCycleStart as a string (its current zod schema uses a number), so decode it tolerantly. (devlake user_spend_collector.go + usage_events_collector.go nextPostPage; metorial admin-client.ts)
- **Claim:** fastPremiumRequests: 'A legacy usage-based premium request count'  
  **Finding:** The docs say 'Number of usage-based premium requests made during the billing cycle'. The word 'legacy' is the spec's own inference. (admin-api.md Get Spending Data)
- **Claim:** Rate limits are per team, per minute; Admin 20/min table  
  **Finding:** The table is correct. The overview says limits apply 'per authenticated user, team, or organization'. Organization API limits are explicitly 'per endpoint'; Admin-route text says 'per team' without 'per endpoint'. Elastic notes that several pollers share one team's 20 rpm budget. Whether the Admin budget is shared across endpoints is still UNVERIFIED, so use a per-team ceiling. The spec also omitted /teams/remove-member (50/min) and /teams/user-spend-limits (20/min), both write routes and unused. (api.md; admin-api.md; elastic README)
- **Claim:** Bearer auth is documented only for the Cloud Agents API, so do not use Bearer  
  **Finding:** Confirmed for the docs; the Origin API also uses Bearer. A forum bug report describes a working Bearer call to /teams/daily-usage-data (L), so Bearer may be accepted in practice. Basic is the documented scheme, so keep Basic. (api.md; forum.cursor.com/t/165528 (snippet))
- **Claim:** Analytics: max range 30 days; dates truncated to 00:00 UTC; today/now  
  **Finding:** Confirmed. Both 'now' and 'today' resolve to the current date at 00:00:00. Whether the endDate day itself is included is not documented (UNVERIFIED). Conversation Insights is documented as aggregate data ('not raw conversation exports or raw conversation content') and returns 401 when disabled. Skipping it is a policy choice, not a doc fact. (account__teams__analytics-api.md)
- **Claim:** requestsCosts can be fractional (1.4, 3.8) [DEVLAKE test]  
  **Finding:** 1.4 is confirmed in the docs example. I could not locate 3.8 in the DevLake files I fetched, so that value is unverified. float64 remains correct either way. (admin-api.md example; devlake tasks/*.go)
- **Claim:** Repo references: ghClient 30 s, internal/crypto/aes.go for encrypted tokens, tool_usage_snapshots columns  
  **Finding:** Confirmed. ghClient = &http.Client{Timeout: 30 * time.Second} is at backend/internal/github/commits.go:59. crypto.Encrypt/Decrypt are used for GitHubAccessToken in queries.go. tool_usage_snapshots has total_suggestions, total_acceptances, total_lines_accepted, total_active_users, total_licensed_users and raw_json, plus three extra columns (see the UpsertToolSnapshot finding). Copilot fills total_suggestions from completion suggestions only, so tabs-only is the like-for-like Cursor mapping. (the repo backend/internal/github/commits.go, copilot.go, db/queries.go, migrations)
