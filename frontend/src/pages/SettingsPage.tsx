import { useState, useEffect } from 'react'
import {
  GitBranch,
  AlertTriangle,
  CheckCircle2,
  Key,
  ChevronDown,
  ChevronRight,
  Copy,
  Check,
} from 'lucide-react'
import { api, type SettingsResponse, type IngestKey, type CreateIngestKeyResponse, ingestKeysApi } from '../api/client'
import { getSnippet } from '../lib/ingestSnippet'
import { USE_MOCK, mockSettings } from '../api/mock'

interface GitHubOrg {
  id: number
  login: string
  name: string
  avatar_url: string
}

function formatDate(iso: string): string {
  return new Date(iso).toLocaleDateString(undefined, {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
  })
}

export function SettingsPage() {
  const [data, setData] = useState<SettingsResponse | null>(null)
  const [loading, setLoading] = useState(true)
  const [hourlyRate, setHourlyRate] = useState(150)
  const [savingRate, setSavingRate] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [success, setSuccess] = useState<string | null>(null)

  // Ingest keys state
  const [ingestKeys, setIngestKeys] = useState<IngestKey[]>([])
  const [ingestKeysLoading, setIngestKeysLoading] = useState(true)
  const [newKeyLabel, setNewKeyLabel] = useState('')
  const [creatingKey, setCreatingKey] = useState(false)
  const [newKeyResult, setNewKeyResult] = useState<CreateIngestKeyResponse | null>(null)
  const [ingestError, setIngestError] = useState<string | null>(null)
  const [copiedKey, setCopiedKey] = useState(false)
  const [showSetup, setShowSetup] = useState(false)
  const [setupTool, setSetupTool] = useState<'claude_code' | 'codex'>('claude_code')

  // Org picker state
  const [githubOrgs, setGithubOrgs] = useState<GitHubOrg[] | null>(null)
  const [loadingOrgs, setLoadingOrgs] = useState(false)
  const [connectingOrg, setConnectingOrg] = useState(false)
  const [orgPickerError, setOrgPickerError] = useState<string | null>(null)
  const [manualOrgLogin, setManualOrgLogin] = useState('')

  useEffect(() => {
    const load = async () => {
      try {
        const result = USE_MOCK
          ? mockSettings
          : await api.get<SettingsResponse>('/api/settings')
        setData(result)
        setHourlyRate(result.user.hourly_rate)
      } catch (e) {
        setError(e instanceof Error ? e.message : 'Failed to load settings')
      } finally {
        setLoading(false)
      }
    }
    load()
  }, [])

  useEffect(() => {
    const loadKeys = async () => {
      if (USE_MOCK) {
        setIngestKeys([])
        setIngestKeysLoading(false)
        return
      }
      try {
        const keys = await ingestKeysApi.listIngestKeys()
        setIngestKeys(keys)
      } catch {
        // Non-fatal: section just shows empty
      } finally {
        setIngestKeysLoading(false)
      }
    }
    loadKeys()
  }, [])

  async function handleSaveRate() {
    setSavingRate(true)
    setError(null)
    setSuccess(null)
    try {
      await api.post('/api/settings/hourly-rate', { hourly_rate: hourlyRate })
      setSuccess('Hourly rate updated.')
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to update rate')
    } finally {
      setSavingRate(false)
    }
  }

  async function handleDisconnect() {
    if (!confirm('Disconnect your GitHub organization? Your data will be preserved.')) return
    try {
      await api.post('/api/settings/disconnect')
      setData((prev) => (prev ? { ...prev, org: null } : prev))
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to disconnect')
    }
  }

  async function handleDeleteAccount() {
    if (
      !confirm(
        'Delete your account? This is permanent and cannot be undone. All data will be lost.',
      )
    )
      return
    try {
      await api.delete('/api/settings/account')
      window.location.href = '/'
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to delete account')
    }
  }

  async function handleLoadGitHubOrgs() {
    setLoadingOrgs(true)
    setOrgPickerError(null)
    try {
      const orgs = await api.get<GitHubOrg[]>('/api/github/orgs')
      setGithubOrgs(orgs)
    } catch (e) {
      setOrgPickerError(e instanceof Error ? e.message : 'Failed to load orgs')
    } finally {
      setLoadingOrgs(false)
    }
  }

  async function handleConnectOrg(orgLogin: string) {
    setConnectingOrg(true)
    setOrgPickerError(null)
    try {
      await api.post('/api/settings/connect-org', { org_login: orgLogin })
      const result = await api.get<SettingsResponse>('/api/settings')
      setData(result)
      setGithubOrgs(null)
    } catch (e) {
      setOrgPickerError(e instanceof Error ? e.message : 'Failed to connect org')
    } finally {
      setConnectingOrg(false)
    }
  }

  async function handleCreateKey() {
    if (!newKeyLabel.trim()) return
    setCreatingKey(true)
    setIngestError(null)
    try {
      const result = await ingestKeysApi.createIngestKey(newKeyLabel.trim())
      setNewKeyResult(result)
      setNewKeyLabel('')
      // Refresh list (add the key without the raw secret)
      setIngestKeys((prev) => [
        ...prev,
        {
          id: result.id,
          label: result.label,
          created_at: result.created_at,
          last_used_at: null,
        },
      ])
    } catch (e) {
      setIngestError(e instanceof Error ? e.message : 'Failed to create key')
    } finally {
      setCreatingKey(false)
    }
  }

  async function handleRevokeKey(id: string, label: string) {
    if (!confirm(`Revoke key "${label}"? This cannot be undone.`)) return
    try {
      await ingestKeysApi.deleteIngestKey(id)
      setIngestKeys((prev) => prev.filter((k) => k.id !== id))
    } catch (e) {
      setIngestError(e instanceof Error ? e.message : 'Failed to revoke key')
    }
  }

  function handleCopyKey() {
    if (!newKeyResult) return
    navigator.clipboard.writeText(newKeyResult.key).then(() => {
      setCopiedKey(true)
      setTimeout(() => setCopiedKey(false), 2000)
    })
  }

  if (loading) {
    return (
      <div className="flex h-full items-center justify-center p-8">
        <div className="text-slate-400">Loading...</div>
      </div>
    )
  }

  const org = data?.org

  return (
    <div className="p-8 max-w-3xl">
      <div className="mb-8">
        <h1 className="text-2xl font-bold text-white">Settings</h1>
        <p className="text-sm text-slate-400 mt-1">Manage your organization</p>
      </div>

      {error && (
        <div className="mb-4 rounded-lg bg-red-500/10 border border-red-500/20 p-4 text-sm text-red-400">
          {error}
        </div>
      )}
      {success && (
        <div className="mb-4 rounded-lg bg-emerald-500/10 border border-emerald-500/20 p-4 text-sm text-emerald-400">
          {success}
        </div>
      )}

      {/* GitHub Connection */}
      <section className="rounded-xl border border-slate-800 bg-slate-900 p-6 mb-6">
        <div className="flex items-center gap-3 mb-4">
          <GitBranch className="h-5 w-5 text-slate-300" />
          <h2 className="text-base font-semibold text-white">GitHub Organization</h2>
        </div>

        {org ? (
          <div>
            <div className="flex items-center gap-3 p-4 rounded-lg bg-slate-800/50 border border-slate-700/50">
              {org.avatar_url && (
                <img src={org.avatar_url} alt={org.name} className="h-10 w-10 rounded-lg" />
              )}
              <div>
                <p className="text-sm font-semibold text-white">{org.name}</p>
                <p className="text-xs text-slate-400">@{org.github_org_login}</p>
              </div>
              <div className="ml-auto flex items-center gap-1.5 text-emerald-400 text-xs font-medium">
                <CheckCircle2 className="h-3.5 w-3.5" />
                Connected
              </div>
            </div>
            <div className="mt-4">
              <div className="p-4 rounded-lg bg-slate-800/30 border border-slate-700/30">
                <p className="text-xs text-slate-500">Sync Status</p>
                <p className="text-sm font-semibold text-white mt-1">Automatic · Hourly</p>
              </div>
            </div>
          </div>
        ) : (
          <div>
            <p className="text-sm text-slate-400 mb-4">
              Connect your GitHub organization to start analyzing AI tool usage across your team.
            </p>
            {orgPickerError && (
              <p className="mb-3 text-sm text-red-400">{orgPickerError}</p>
            )}
            {githubOrgs === null ? (
              <button
                onClick={handleLoadGitHubOrgs}
                disabled={loadingOrgs}
                className="inline-flex items-center gap-2 rounded-lg bg-slate-800 border border-slate-700 px-4 py-2.5 text-sm font-semibold text-white hover:bg-slate-700 transition-colors disabled:opacity-50"
              >
                <GitBranch className="h-4 w-4" />
                {loadingOrgs ? 'Loading orgs...' : 'Choose a GitHub Organization'}
              </button>
            ) : githubOrgs.length === 0 ? (
              <div className="space-y-4">
                <p className="text-sm text-slate-400">
                  No organizations were returned by GitHub. This happens when your org membership is private
                  or the org has OAuth app restrictions. Enter your org login directly:
                </p>
                <div className="flex gap-2">
                  <input
                    type="text"
                    placeholder="my-github-org"
                    value={manualOrgLogin}
                    onChange={(e) => setManualOrgLogin(e.target.value)}
                    className="flex-1 rounded-lg bg-slate-800 border border-slate-700 px-3 py-2 text-sm text-white placeholder-slate-500 focus:outline-none focus:border-violet-500"
                  />
                  <button
                    onClick={() => handleConnectOrg(manualOrgLogin.trim())}
                    disabled={connectingOrg || !manualOrgLogin.trim()}
                    className="inline-flex items-center gap-2 rounded-lg bg-violet-600 px-4 py-2 text-sm font-semibold text-white hover:bg-violet-500 disabled:opacity-50 transition-colors"
                  >
                    {connectingOrg ? 'Connecting...' : 'Connect'}
                  </button>
                </div>
              </div>
            ) : (
              <div className="space-y-2">
                {githubOrgs.map((o) => (
                  <button
                    key={o.id}
                    onClick={() => handleConnectOrg(o.login)}
                    disabled={connectingOrg}
                    className="w-full flex items-center gap-3 p-3 rounded-lg bg-slate-800/50 border border-slate-700/50 hover:border-violet-500/50 hover:bg-slate-800 transition-colors text-left disabled:opacity-50"
                  >
                    {o.avatar_url && (
                      <img src={o.avatar_url} alt={o.login} className="h-8 w-8 rounded-md flex-shrink-0" />
                    )}
                    <div className="min-w-0">
                      <p className="text-sm font-semibold text-white truncate">{o.name || o.login}</p>
                      <p className="text-xs text-slate-400">@{o.login}</p>
                    </div>
                    <span className="ml-auto text-xs text-violet-400 flex-shrink-0">
                      {connectingOrg ? 'Connecting...' : 'Connect →'}
                    </span>
                  </button>
                ))}
                <div className="pt-2 border-t border-slate-800 mt-2">
                  <p className="text-xs text-slate-500 mb-2">Don't see your org? Enter it manually:</p>
                  <div className="flex gap-2">
                    <input
                      type="text"
                      placeholder="my-github-org"
                      value={manualOrgLogin}
                      onChange={(e) => setManualOrgLogin(e.target.value)}
                      className="flex-1 rounded-lg bg-slate-800 border border-slate-700 px-3 py-2 text-sm text-white placeholder-slate-500 focus:outline-none focus:border-violet-500"
                    />
                    <button
                      onClick={() => handleConnectOrg(manualOrgLogin.trim())}
                      disabled={connectingOrg || !manualOrgLogin.trim()}
                      className="inline-flex items-center gap-2 rounded-lg bg-violet-600 px-4 py-2 text-sm font-semibold text-white hover:bg-violet-500 disabled:opacity-50 transition-colors"
                    >
                      {connectingOrg ? 'Connecting...' : 'Connect'}
                    </button>
                  </div>
                </div>
              </div>
            )}
          </div>
        )}
      </section>

      {/* ROI Settings */}
      <section className="rounded-xl border border-slate-800 bg-slate-900 p-6 mb-6">
        <h2 className="text-base font-semibold text-white mb-4">ROI Configuration</h2>
        <div className="space-y-4">
          <div>
            <label className="text-sm font-medium text-slate-300">
              Average Engineer Hourly Rate
            </label>
            <p className="text-xs text-slate-500 mt-0.5 mb-2">
              Used to calculate dollar value of time saved. Default: $150/hr.
            </p>
            <div className="flex items-center gap-2">
              <span className="text-slate-400 text-sm">$</span>
              <input
                type="number"
                value={hourlyRate}
                onChange={(e) => setHourlyRate(parseInt(e.target.value) || 150)}
                className="w-32 rounded-lg border border-slate-700 bg-slate-800 px-3 py-2 text-sm text-white focus:border-violet-500 focus:outline-none"
                placeholder="150"
                min="1"
                max="10000"
              />
              <span className="text-slate-400 text-sm">/hr</span>
              <button
                onClick={handleSaveRate}
                disabled={savingRate}
                className="rounded-lg bg-slate-700 px-3 py-2 text-sm text-white hover:bg-slate-600 disabled:opacity-50 transition-colors"
              >
                {savingRate ? 'Saving...' : 'Save'}
              </button>
            </div>
          </div>
        </div>
      </section>

      {/* Push Ingest API */}
      <section className="rounded-xl border border-slate-800 bg-slate-900 p-6 mb-6">
        <div className="flex items-center gap-3 mb-1">
          <Key className="h-5 w-5 text-slate-300" />
          <h2 className="text-base font-semibold text-white">Push Ingest API</h2>
        </div>
        <p className="text-xs text-slate-500 mb-5">
          Use API keys to send AI usage data directly to AI Stats from Claude Code hooks or CI
          pipelines.
        </p>

        {ingestError && (
          <div className="mb-4 rounded-lg bg-red-500/10 border border-red-500/20 p-3 text-xs text-red-400">
            {ingestError}
          </div>
        )}

        {/* Revealed key banner */}
        {newKeyResult && (
          <div className="mb-5 rounded-lg border border-violet-500/30 bg-violet-500/5 p-4">
            <p className="text-xs font-medium text-violet-300 mb-2">
              Copy this key now — it won't be shown again.
            </p>
            <div className="flex items-center gap-2">
              <code className="flex-1 rounded bg-slate-950 border border-slate-700 px-3 py-2 text-xs text-emerald-400 font-mono break-all">
                {newKeyResult.key}
              </code>
              <button
                onClick={handleCopyKey}
                className="flex items-center gap-1.5 rounded-lg border border-slate-700 px-3 py-2 text-xs text-slate-300 hover:bg-slate-800 transition-colors shrink-0"
              >
                {copiedKey ? (
                  <Check className="h-3.5 w-3.5 text-emerald-400" />
                ) : (
                  <Copy className="h-3.5 w-3.5" />
                )}
                {copiedKey ? 'Copied' : 'Copy'}
              </button>
            </div>
            <button
              onClick={() => setNewKeyResult(null)}
              className="mt-3 text-xs text-slate-500 hover:text-slate-400 transition-colors"
            >
              Dismiss
            </button>
          </div>
        )}

        {/* Key list */}
        <div className="mb-5">
          {ingestKeysLoading ? (
            <p className="text-xs text-slate-500">Loading keys...</p>
          ) : ingestKeys.length === 0 ? (
            <p className="text-xs text-slate-500">No API keys yet.</p>
          ) : (
            <div className="rounded-lg border border-slate-800 overflow-hidden">
              <table className="w-full text-xs">
                <thead>
                  <tr className="border-b border-slate-800 bg-slate-800/30">
                    <th className="text-left px-4 py-2.5 text-slate-500 font-medium">Label</th>
                    <th className="text-left px-4 py-2.5 text-slate-500 font-medium">Created</th>
                    <th className="text-left px-4 py-2.5 text-slate-500 font-medium">Last used</th>
                    <th className="px-4 py-2.5" />
                  </tr>
                </thead>
                <tbody>
                  {ingestKeys.map((key, i) => (
                    <tr
                      key={key.id}
                      className={i < ingestKeys.length - 1 ? 'border-b border-slate-800/60' : ''}
                    >
                      <td className="px-4 py-3 text-slate-200 font-medium">{key.label}</td>
                      <td className="px-4 py-3 text-slate-400">{formatDate(key.created_at)}</td>
                      <td className="px-4 py-3 text-slate-400">
                        {key.last_used_at ? formatDate(key.last_used_at) : 'Never'}
                      </td>
                      <td className="px-4 py-3 text-right">
                        <button
                          onClick={() => handleRevokeKey(key.id, key.label)}
                          className="rounded border border-red-500/30 px-2.5 py-1 text-xs text-red-400 hover:bg-red-500/10 transition-colors"
                        >
                          Revoke
                        </button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>

        {/* Create key form */}
        <div className="flex items-center gap-2 mb-5">
          <input
            type="text"
            value={newKeyLabel}
            onChange={(e) => setNewKeyLabel(e.target.value)}
            onKeyDown={(e) => e.key === 'Enter' && handleCreateKey()}
            placeholder="Key label (e.g. claude-code-laptop)"
            className="flex-1 rounded-lg border border-slate-700 bg-slate-800 px-3 py-2 text-sm text-white placeholder-slate-500 focus:border-violet-500 focus:outline-none"
          />
          <button
            onClick={handleCreateKey}
            disabled={creatingKey || !newKeyLabel.trim()}
            className="rounded-lg bg-violet-600 px-4 py-2 text-sm font-semibold text-white hover:bg-violet-500 disabled:opacity-50 disabled:cursor-not-allowed transition-colors shrink-0"
          >
            {creatingKey ? 'Generating...' : 'Generate key'}
          </button>
        </div>

        {/* Setup instructions */}
        <div className="border-t border-slate-800 pt-4">
          <button
            onClick={() => setShowSetup((v) => !v)}
            className="flex items-center gap-2 text-xs text-slate-400 hover:text-slate-300 transition-colors"
          >
            {showSetup ? (
              <ChevronDown className="h-3.5 w-3.5" />
            ) : (
              <ChevronRight className="h-3.5 w-3.5" />
            )}
            How to configure your AI tool
          </button>
          {showSetup && (
            <div className="mt-3">
              <div className="flex gap-1 mb-3">
                {(['claude_code', 'codex'] as const).map((tool) => (
                  <button
                    key={tool}
                    onClick={() => setSetupTool(tool)}
                    className={`px-3 py-1 rounded-md text-xs font-medium transition-colors ${
                      setupTool === tool
                        ? 'bg-violet-500/20 text-violet-300 border border-violet-500/30'
                        : 'text-slate-400 hover:text-slate-300'
                    }`}
                  >
                    {tool === 'claude_code' ? 'Claude Code' : 'OpenAI Codex'}
                  </button>
                ))}
              </div>
              <pre className="rounded-lg bg-slate-950 border border-slate-800 p-4 text-xs text-slate-300 font-mono overflow-x-auto whitespace-pre-wrap">
                {getSnippet(setupTool, newKeyResult?.key ?? 'aistats_live_your_key_here')}
              </pre>
            </div>
          )}
        </div>
      </section>

      {/* Danger Zone */}
      <section className="rounded-xl border border-red-500/20 bg-red-500/5 p-6">
        <div className="flex items-center gap-3 mb-4">
          <AlertTriangle className="h-5 w-5 text-red-400" />
          <h2 className="text-base font-semibold text-red-300">Danger Zone</h2>
        </div>
        <div className="space-y-3">
          {org && (
            <div className="flex items-center justify-between py-3 border-b border-red-500/10">
              <div>
                <p className="text-sm font-medium text-slate-200">Disconnect organization</p>
                <p className="text-xs text-slate-500">
                  Removes access token and stops syncing. Your data is preserved.
                </p>
              </div>
              <button
                onClick={handleDisconnect}
                className="rounded-lg border border-red-500/30 px-3 py-1.5 text-xs text-red-400 hover:bg-red-500/10 transition-colors"
              >
                Disconnect
              </button>
            </div>
          )}
          <div className="flex items-center justify-between py-3">
            <div>
              <p className="text-sm font-medium text-slate-200">Delete account</p>
              <p className="text-xs text-slate-500">
                Permanently delete your account and all data. This cannot be undone.
              </p>
            </div>
            <button
              onClick={handleDeleteAccount}
              className="rounded-lg border border-red-500/30 px-3 py-1.5 text-xs text-red-400 hover:bg-red-500/10 transition-colors"
            >
              Delete account
            </button>
          </div>
        </div>
      </section>
    </div>
  )
}
