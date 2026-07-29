export const CODEX_SNIPPET = [
  '# Wrap the codex command to report usage after each session',
  '# Add to ~/.bashrc or ~/.zshrc:',
  'export AISTATS_INGEST_KEY="aistats_live_your_key_here"',
  `export AISTATS_ORG_URL="${typeof window !== 'undefined' ? window.location.origin : 'http://localhost:8080'}"`,
  '',
  'codex() {',
  '  local start=$(date +%s)',
  '  command codex "$@"',
  '  local mins=$(( ($(date +%s) - start) / 60 ))',
  '  curl -s -X POST $AISTATS_ORG_URL/api/ingest/events \\',
  '    -H "X-AIStats-Key: $AISTATS_INGEST_KEY" \\',
  '    -H "Content-Type: application/json" \\',
  '    -d "{\\"tool_source\\":\\"codex\\",\\"date\\":\\"$(date +%Y-%m-%d)\\",\\"events\\":[{\\"type\\":\\"session_end\\",\\"lines_written\\":0,\\"duration_minutes\\":$mins,\\"user_email\\":\\"$USER@example.com\\"}]}"',
  '}',
].join('\n')

export function getSnippet(tool: 'claude_code' | 'codex', key: string): string {
  if (tool === 'claude_code') {
    return [
      '# Paste this in your terminal',
      `curl -sSL ${window.location.origin}/install.sh | AISTATS_KEY=${key} bash`,
      '',
      "# Then start a new Claude Code session. That's it.",
    ].join('\n')
  }
  return CODEX_SNIPPET.replace(/aistats_live_your_key_here/g, key)
}
