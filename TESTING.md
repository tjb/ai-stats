# AI Stats Manual Testing Guide

## Prerequisites

Both services must be running before any test:

```bash
# Terminal 1
make dev-backend
# → Go server on http://localhost:8080

# Terminal 2
make dev-frontend
# → Vite on http://localhost:3000
```

Database must be running and migrations applied (migrations run automatically on backend start).

---

## 1. Landing Page

**URL:** http://localhost:3000

- [ ] Page loads without redirecting to login
- [ ] Nav shows "Open source" link and "Sign in with GitHub" button
- [ ] Badge reads "Now with GitHub Copilot Usage API"
- [ ] Headline: "Know exactly what your AI coding tools are worth"
- [ ] Two CTAs: "Start free with GitHub" and "View on GitHub"
- [ ] Browser mockup shows 4 metric cards and 2 mini bar charts
- [ ] "View on GitHub" scrolls to the open source section
- [ ] Open source section links out to the GitHub repo
- [ ] Footer shows Privacy and Terms links

---

## 2. Authentication

**URL:** http://localhost:3000/login

- [ ] Login page loads
- [ ] "Continue with GitHub" button visible
- [ ] Clicking button redirects to GitHub OAuth
- [ ] After GitHub approval, redirected to http://localhost:3000/dashboard
- [ ] Sidebar shows your GitHub name and email
- [ ] Sign out button clears session and redirects to /login

**GitHub OAuth App callback must be set to:** `http://localhost:8080/api/auth/callback`

---

## 3. Dashboard — No Org State

**URL:** http://localhost:3000/dashboard (logged in, no org connected)

- [ ] "Connect your GitHub org" empty state shown
- [ ] "Try with demo data" button visible
- [ ] Clicking "Try with demo data" shows spinner
- [ ] After a moment, dashboard loads with Acme Corp (Demo) data
- [ ] 4 metric cards visible: Active Users, Acceptance Rate, Lines Accepted, Est. Value Saved
- [ ] All metric cards show values and change indicators (green for positive)
- [ ] Est. Value Saved card has violet border

---

## 4. Dashboard — With Data

After loading demo data:

- [ ] Header shows org name "Acme Corp (Demo)"
- [ ] Subheader reads "Last 30 days · Updated hourly"
- [ ] "Sync now" button in top right
- [ ] Acceptance Rate chart renders (line chart, last 90 days)
- [ ] Daily Active Users chart renders (bar chart)
- [ ] Suggestions vs Acceptances chart renders (grouped bar)
- [ ] Acceptance Rate by Language chart renders (horizontal bar)
- [ ] Chart tooltips show white/violet text (not black) on hover
- [ ] Clicking "Sync now" triggers a sync and refreshes data

---

## 5. Insights Page

**URL:** http://localhost:3000/insights

- [ ] Page loads with language breakdown table
- [ ] Editor breakdown table visible
- [ ] Activity heatmap renders
- [ ] **Team AI Utilization** card visible at the top (violet border)
- [ ] Utilization % = AI active users ÷ total coding developers
- [ ] Shows "X of Y developers used an AI coding tool in the last 30 days"
- [ ] Circular progress ring renders the utilization %
- [ ] If no git org connected, shows "—" with a prompt to connect

---

## 6. Reports Page

**URL:** http://localhost:3000/reports

- [ ] Page loads with list of generated reports (empty initially)
- [ ] Month selector visible
- [ ] "Generate Report" button works without 500 error
- [ ] After generating, report appears in list

---

## 7. Settings Page

**URL:** http://localhost:3000/settings

- [ ] Page loads showing connected org (or "no org" state)
- [ ] Hourly rate field shows current value (default: $150)
- [ ] Updating hourly rate saves successfully
- [ ] "Push Ingest API" section visible
- [ ] Ingest keys list shown (empty initially)

---

## 8. Push Ingest API — End-to-End

### Step 1: Get your session cookie

Open browser devtools (F12) → Application → Cookies → `localhost` → copy the value of `aistats_session`.

```bash
SESSION="<paste aistats_session cookie value here>"
```

### Step 2: Create an ingest API key

```bash
curl -s -X POST http://localhost:8080/api/settings/ingest-keys \
  -H "Content-Type: application/json" \
  -H "Cookie: aistats_session=$SESSION" \
  -d '{"label": "test key"}' | jq .
```

**Expected response:**
```json
{
  "id": "...",
  "key": "aistats_live_...",
  "label": "test key",
  "created_at": "..."
}
```

- [ ] Response contains a `key` starting with `aistats_live_`
- [ ] Copy the key — **it will not be shown again**

```bash
KEY="aistats_live_<paste key here>"
```

### Step 3: Verify key appears in list

```bash
curl -s http://localhost:8080/api/settings/ingest-keys \
  -H "Cookie: aistats_session=$SESSION" | jq .
```

- [ ] Key appears in list with label "test key"
- [ ] Raw key value is NOT returned in the list (only id, label, created_at, last_used_at)

### Step 4: Send a test event

```bash
curl -s -X POST http://localhost:8080/api/ingest/events \
  -H "Content-Type: application/json" \
  -H "X-AIStats-Key: $KEY" \
  -d "{
    \"tool_source\": \"claude_code\",
    \"date\": \"$(date +%Y-%m-%d)\",
    \"events\": [
      {
        \"type\": \"session_end\",
        \"lines_written\": 120,
        \"duration_minutes\": 32,
        \"user_email\": \"you@example.com\"
      }
    ]
  }" | jq .
```

**Expected response:**
```json
{"accepted": 1}
```

- [ ] Response is `{"accepted": 1}`

### Step 5: Verify event landed in DB

```bash
psql "$DATABASE_URL" \
  -c "SELECT tool_source, date, total_acceptances, total_lines_accepted, total_active_users, raw_json FROM tool_usage_snapshots WHERE tool_source = 'claude_code';"
```

- [ ] Row exists with `tool_source = claude_code`
- [ ] `total_lines_accepted` = 120
- [ ] `total_acceptances` = 1 (session count)
- [ ] `total_active_users` = 1
- [ ] `raw_json` contains `"confidence": "high"` and `"source": "push_ingest"`

### Step 6: Test bad key rejection

```bash
curl -s -X POST http://localhost:8080/api/ingest/events \
  -H "Content-Type: application/json" \
  -H "X-AIStats-Key: aistats_live_fakekeyhere" \
  -d '{"tool_source":"claude_code","date":"2026-01-01","events":[]}' | jq .
```

- [ ] Returns `{"error": "invalid API key"}` with HTTP 401

### Step 7: Revoke the key

```bash
KEY_ID="<paste id from step 2>"

curl -s -X DELETE "http://localhost:8080/api/settings/ingest-keys/$KEY_ID" \
  -H "Cookie: aistats_session=$SESSION" | jq .
```

- [ ] Returns `{"ok": true}`
- [ ] Key no longer appears in the list (re-run step 3 to confirm)
- [ ] Sending an event with the revoked key returns 401

---

## 9. Git Trailer Attribution — End-to-End

This test requires a real GitHub org connected (not demo data) and at least one repo.

### Step 1: Make a test commit with Claude Code trailer

In any repo belonging to your connected GitHub org:

```bash
git commit --allow-empty -m "test: attribution test

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>"
git push
```

### Step 2: Trigger Claude Code sync

```bash
SESSION="<paste aistats_session cookie value here>"

curl -s -X POST http://localhost:8080/api/tools/sync \
  -H "Content-Type: application/json" \
  -H "Cookie: aistats_session=$SESSION" \
  -d '{"tool_source": "claude_code"}' | jq .
```

**Expected response:**
```json
{"synced": 1, "tool_source": "claude_code"}
```

- [ ] Response shows `synced` > 0

### Step 3: Verify attribution in DB

```bash
psql "$DATABASE_URL" \
  -c "SELECT tool_source, date, total_acceptances, raw_json FROM tool_usage_snapshots WHERE tool_source = 'claude_code' ORDER BY date DESC LIMIT 5;"
```

- [ ] Row exists for today's date with `tool_source = claude_code`
- [ ] `raw_json` contains `"confidence": "medium"` and a list of attributed commit SHAs

### Step 4: Verify trailer patterns are detected

The following trailer formats should all be picked up by the sync:

```bash
# Standard Claude Code format
git commit --allow-empty -m "test 1
Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>"

# Shorter format
git commit --allow-empty -m "test 2
Co-Authored-By: Claude <claude@anthropic.com>"

# Should NOT be attributed (not Anthropic)
git commit --allow-empty -m "test 3
Co-Authored-By: Jane Smith <jane@example.com>"

git push
```

Re-run the sync and verify:
- [ ] 2 additional attributed commits (not 3)
- [ ] The non-Claude commit is ignored

---

## 10. Unit Tests

Run the automated test suite:

```bash
cd backend
go test ./... -v
```

- [ ] All tests pass
- [ ] `TestIsClaudeAttributed` shows 7 subtests all passing

---

## Known Limitations (not bugs)

- **Cursor and Codeium** have no API and don't inject git trailers — they cannot be attributed in the current version
- **Git trailer attribution undercounts** if developers edit commit messages before pushing — push ingest is the complete solution
- **Demo org sync** (`tool_source: claude_code`) will return 0 synced — "acme-corp" is not a real GitHub org
- **Dashboard** currently shows Copilot data only — Claude Code attribution from push ingest will appear after the dashboard is updated to aggregate across all tool sources (upcoming)
