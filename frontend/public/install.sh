#!/bin/bash
# AI Stats install script
# This script is served by your own self-hosted instance at /install.sh.
# Usage: curl -sSL <your-instance-url>/install.sh | AISTATS_KEY=aistats_live_xxx bash
#    or: curl -sSL <your-instance-url>/install.sh | bash -s -- --key aistats_live_xxx --url <your-instance-url>

set -e

AISTATS_URL="${AISTATS_URL:-http://localhost:8080}"
HOOKS_DIR="${HOME}/.claude/hooks"
SETTINGS="${HOME}/.claude/settings.json"
QUEUE="${HOME}/.claude/aistats-queue.jsonl"

# ── Parse args ────────────────────────────────────────────────────────────────
while [[ $# -gt 0 ]]; do
  case "$1" in
    --key) AISTATS_KEY="$2"; shift 2 ;;
    --url) AISTATS_URL="$2"; shift 2 ;;
    *) shift ;;
  esac
done

# ── Validate ──────────────────────────────────────────────────────────────────
if [ -z "$AISTATS_KEY" ]; then
  echo "error: AISTATS_KEY is required."
  echo ""
  echo "  curl -sSL <your-instance-url>/install.sh | AISTATS_KEY=aistats_live_xxx bash"
  echo ""
  echo "Get your key at: ${AISTATS_URL}/settings"
  exit 1
fi

if ! command -v python3 &>/dev/null; then
  echo "error: python3 is required but not found."
  exit 1
fi

echo ""
echo "  Installing AI Stats Claude Code hooks..."
echo ""

# ── Create hooks directory ────────────────────────────────────────────────────
mkdir -p "${HOOKS_DIR}"

# ── Write aistats-ingest.sh ───────────────────────────────────────────────────
cat > "${HOOKS_DIR}/aistats-ingest.sh" << 'INGEST_EOF'
#!/bin/bash
# PostToolUse hook — appends one event to the local queue.
# No network call here; aistats-flush.sh sends everything at session end.
KEY="${AISTATS_INGEST_KEY}"
if [ -z "$KEY" ]; then exit 0; fi
QUEUE="${HOME}/.claude/aistats-queue.jsonl"
DATE="$(date +%Y-%m-%d)"
EMAIL="${USER}@local"
STDIN=$(cat)
RESULT=$(echo "$STDIN" | python3 -c "
import sys, json
d = json.load(sys.stdin)
tool = d.get('tool_name', 'unknown')
inp = d.get('tool_input', {})
if tool == 'Edit': lines = len(inp.get('new_string','').splitlines())
elif tool == 'Write': lines = len(inp.get('content','').splitlines())
elif tool == 'MultiEdit': lines = sum(len(e.get('new_string','').splitlines()) for e in inp.get('edits',[]))
elif tool == 'NotebookEdit': lines = len(inp.get('new_source','').splitlines())
else: lines = 0
print(f'{tool}|{lines}')
" 2>/dev/null || echo "unknown|0")
TOOL=$(echo "$RESULT" | cut -d'|' -f1)
LINES=$(echo "$RESULT" | cut -d'|' -f2)
echo "{\"date\":\"${DATE}\",\"tool_name\":\"${TOOL}\",\"lines_written\":${LINES},\"user_email\":\"${EMAIL}\"}" >> "${QUEUE}"
exit 0
INGEST_EOF

# ── Write aistats-flush.sh ────────────────────────────────────────────────────
cat > "${HOOKS_DIR}/aistats-flush.sh" << 'FLUSH_EOF'
#!/bin/bash
# Stop hook — sends all queued events in one batched request per day.
# Fires once when the session ends. Queue is only cleared on success.
KEY="${AISTATS_INGEST_KEY}"
URL="${AISTATS_ORG_URL:-http://localhost:8080}"
QUEUE="${HOME}/.claude/aistats-queue.jsonl"
if [ -z "$KEY" ] || [ ! -s "$QUEUE" ]; then exit 0; fi
python3 - "${QUEUE}" "${KEY}" "${URL}" << 'PYEOF'
import sys, json, urllib.request
queue_path, key, url = sys.argv[1], sys.argv[2], sys.argv[3]
with open(queue_path) as f:
    lines = [l.strip() for l in f if l.strip()]
by_date = {}
for line in lines:
    try:
        ev = json.loads(line)
        by_date.setdefault(ev.get('date',''), []).append({
            'type': 'tool_use',
            'tool_name': ev.get('tool_name', 'unknown'),
            'lines_written': ev.get('lines_written', 0),
            'duration_minutes': 0,
            'user_email': ev.get('user_email', ''),
        })
    except Exception:
        pass
ok = True
for date, events in by_date.items():
    payload = json.dumps({'tool_source': 'claude_code', 'date': date, 'events': events}).encode()
    req = urllib.request.Request(
        f'{url}/api/ingest/events', data=payload,
        headers={'Content-Type': 'application/json', 'X-AIStats-Key': key},
        method='POST',
    )
    try:
        urllib.request.urlopen(req, timeout=10)
    except Exception as e:
        print(f'aistats-flush: {e}', file=sys.stderr)
        ok = False
if ok:
    open(queue_path, 'w').close()
PYEOF
exit 0
FLUSH_EOF

chmod +x "${HOOKS_DIR}/aistats-ingest.sh" "${HOOKS_DIR}/aistats-flush.sh"

# ── Patch ~/.claude/settings.json ────────────────────────────────────────────
python3 - "${SETTINGS}" "${AISTATS_KEY}" "${AISTATS_URL}" << 'PYEOF'
import sys, json, os

settings_path, key, url = sys.argv[1], sys.argv[2], sys.argv[3]
hooks_dir = os.path.expanduser('~/.claude/hooks')

# Load existing settings or start fresh
if os.path.exists(settings_path):
    with open(settings_path) as f:
        settings = json.load(f)
else:
    settings = {}

# Merge env vars
settings.setdefault('env', {})
settings['env']['AISTATS_INGEST_KEY'] = key
settings['env']['AISTATS_ORG_URL'] = url

# Merge hooks — add our entries only if not already present
settings.setdefault('hooks', {})

ingest_cmd = f'{hooks_dir}/aistats-ingest.sh'
flush_cmd  = f'{hooks_dir}/aistats-flush.sh'

# PostToolUse
ptu = settings['hooks'].setdefault('PostToolUse', [])
has_ingest = any(
    any(h.get('command') == ingest_cmd for h in entry.get('hooks', []))
    for entry in ptu
)
if not has_ingest:
    ptu.append({'matcher': '.*', 'hooks': [{'type': 'command', 'command': ingest_cmd}]})

# Stop
stop = settings['hooks'].setdefault('Stop', [])
has_flush = any(
    any(h.get('command') == flush_cmd for h in entry.get('hooks', []))
    for entry in stop
)
if not has_flush:
    if stop:
        stop[0].setdefault('hooks', []).append({'type': 'command', 'command': flush_cmd})
    else:
        stop.append({'hooks': [{'type': 'command', 'command': flush_cmd}]})

with open(settings_path, 'w') as f:
    json.dump(settings, f, indent=2)
    f.write('\n')

print(f'  ✓ Patched {settings_path}')
PYEOF

# ── Done ──────────────────────────────────────────────────────────────────────
echo "  ✓ Created ${HOOKS_DIR}/aistats-ingest.sh"
echo "  ✓ Created ${HOOKS_DIR}/aistats-flush.sh"
echo ""
echo "  AI Stats is ready. Start a new Claude Code session to begin tracking."
echo ""
