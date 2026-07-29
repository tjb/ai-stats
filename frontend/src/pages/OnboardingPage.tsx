import { useState, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { CheckCircle2, GitBranch, Copy, Check } from 'lucide-react'
import { api } from '../api/client'
import { getSnippet } from '../lib/ingestSnippet'

interface GitHubOrg {
  id: number
  login: string
  name: string
  avatar_url: string
}

interface CreateIngestKeyResponse {
  id: string
  key: string
  label: string
  created_at: string
}

const ALL_TOOLS = ['Claude Code', 'GitHub Copilot', 'OpenAI Codex', 'Other'] as const

function StepIndicator({ step, total }: { step: number; total: number }) {
  return (
    <div className="flex items-center gap-2 mb-8">
      {Array.from({ length: total }, (_, i) => (
        <div
          key={i}
          className={`h-2 rounded-full transition-all ${
            i + 1 === step
              ? 'w-6 bg-violet-500'
              : i + 1 < step
              ? 'w-2 bg-violet-400'
              : 'w-2 bg-slate-700'
          }`}
        />
      ))}
      <span className="ml-2 text-xs text-slate-500">Step {step} of {total}</span>
    </div>
  )
}

// ---------------------------------------------------------------------------
// Step 1 — Welcome
// ---------------------------------------------------------------------------
function Step1Welcome({ onNext }: { onNext: () => void }) {
  return (
    <div className="flex flex-col items-center text-center">
      <div className="flex h-14 w-14 items-center justify-center rounded-2xl bg-gradient-to-br from-violet-500 to-purple-600 mb-6">
        <span className="text-2xl font-bold text-white">L</span>
      </div>
      <h1 className="text-3xl font-bold text-white mb-3">AI Stats</h1>
      <p className="text-xl font-semibold text-slate-200 mb-3">
        See exactly what your AI coding tools are delivering
      </p>
      <p className="text-sm text-slate-400 mb-10 max-w-sm">
        Set up takes 2 minutes. We'll connect your tools and start tracking ROI immediately.
      </p>
      <button
        onClick={onNext}
        className="rounded-xl bg-gradient-to-r from-violet-500 to-purple-600 px-8 py-3 text-base font-semibold text-white hover:opacity-90 transition-opacity"
      >
        Get started →
      </button>
    </div>
  )
}

// ---------------------------------------------------------------------------
// Step 2 — Tool selection
// ---------------------------------------------------------------------------
function Step2Tools({
  selected,
  onToggle,
  onNext,
}: {
  selected: string[]
  onToggle: (tool: string) => void
  onNext: () => void
}) {
  const [touched, setTouched] = useState(false)
  const hasSelection = selected.length > 0

  function handleNext() {
    setTouched(true)
    if (hasSelection) onNext()
  }

  return (
    <div>
      <h2 className="text-2xl font-bold text-white mb-2">Which AI coding tools does your team use?</h2>
      <p className="text-sm text-slate-400 mb-6">Select all that apply.</p>

      <div className="space-y-3 mb-6">
        {ALL_TOOLS.map((tool) => {
          const checked = selected.includes(tool)
          return (
            <label
              key={tool}
              className={`flex items-center gap-3 cursor-pointer rounded-xl border p-4 transition-colors ${
                checked
                  ? 'border-violet-500/60 bg-violet-500/10'
                  : 'border-slate-700 bg-slate-800/50 hover:border-slate-600'
              }`}
            >
              <input
                type="checkbox"
                className="sr-only"
                checked={checked}
                onChange={() => onToggle(tool)}
              />
              <div
                className={`h-5 w-5 rounded flex items-center justify-center border-2 transition-colors shrink-0 ${
                  checked
                    ? 'border-violet-500 bg-violet-500'
                    : 'border-slate-600'
                }`}
              >
                {checked && (
                  <svg className="h-3 w-3 text-white" viewBox="0 0 12 12" fill="none">
                    <path d="M2 6l3 3 5-5" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" />
                  </svg>
                )}
              </div>
              <span className="text-sm font-medium text-slate-200">{tool}</span>
            </label>
          )
        })}
      </div>

      {touched && !hasSelection && (
        <p className="mb-4 text-sm text-red-400">Please select at least one tool to continue.</p>
      )}

      <button
        onClick={handleNext}
        className="w-full rounded-xl bg-violet-600 py-3 text-sm font-semibold text-white hover:bg-violet-500 transition-colors"
      >
        Next →
      </button>
    </div>
  )
}

// ---------------------------------------------------------------------------
// Step 3 — Connect GitHub org
// ---------------------------------------------------------------------------
function Step3GitHub({
  selectedTools,
  connectedOrgLogin,
  onConnect,
  onNext,
  onSkip,
}: {
  selectedTools: string[]
  connectedOrgLogin: string | null
  onConnect: (orgLogin: string) => void
  onNext: () => void
  onSkip: () => void
}) {
  const copilotRequired = selectedTools.includes('GitHub Copilot')

  const [githubOrgs, setGithubOrgs] = useState<GitHubOrg[] | null>(null)
  const [loadingOrgs, setLoadingOrgs] = useState(false)
  const [connectingOrg, setConnectingOrg] = useState(false)
  const [orgPickerError, setOrgPickerError] = useState<string | null>(null)
  const [manualOrgLogin, setManualOrgLogin] = useState('')

  async function handleLoadOrgs() {
    setLoadingOrgs(true)
    setOrgPickerError(null)
    try {
      const orgs = await api.get<GitHubOrg[]>('/api/github/orgs')
      setGithubOrgs(orgs)
    } catch (e) {
      setOrgPickerError(e instanceof Error ? e.message : 'Failed to load organizations')
    } finally {
      setLoadingOrgs(false)
    }
  }

  async function handleConnectOrg(orgLogin: string) {
    if (!orgLogin.trim()) return
    setConnectingOrg(true)
    setOrgPickerError(null)
    try {
      await api.post('/api/settings/connect-org', { org_login: orgLogin.trim() })
      onConnect(orgLogin.trim())
      setGithubOrgs(null)
    } catch (e) {
      setOrgPickerError(e instanceof Error ? e.message : 'Failed to connect organization')
    } finally {
      setConnectingOrg(false)
    }
  }

  return (
    <div>
      <h2 className="text-2xl font-bold text-white mb-2">Connect your GitHub organization</h2>
      <p className="text-sm text-slate-400 mb-1">
        {copilotRequired
          ? 'Required to pull your Copilot seat data.'
          : 'Optional — enables adoption tracking (how many of your engineers are activated).'}
      </p>

      {connectedOrgLogin ? (
        <div className="mt-4 flex items-center gap-3 p-4 rounded-xl bg-emerald-500/10 border border-emerald-500/30 mb-6">
          <CheckCircle2 className="h-5 w-5 text-emerald-400 shrink-0" />
          <div>
            <p className="text-sm font-semibold text-white">Connected</p>
            <p className="text-xs text-slate-400">@{connectedOrgLogin}</p>
          </div>
        </div>
      ) : (
        <div className="mt-4 mb-6">
          {orgPickerError && (
            <p className="mb-3 text-sm text-red-400">{orgPickerError}</p>
          )}

          {githubOrgs === null ? (
            <button
              onClick={handleLoadOrgs}
              disabled={loadingOrgs}
              className="inline-flex items-center gap-2 rounded-lg bg-slate-800 border border-slate-700 px-4 py-2.5 text-sm font-semibold text-white hover:bg-slate-700 transition-colors disabled:opacity-50"
            >
              <GitBranch className="h-4 w-4" />
              {loadingOrgs ? 'Loading...' : 'Choose a GitHub Organization'}
            </button>
          ) : githubOrgs.length === 0 ? (
            <div className="space-y-3">
              <p className="text-sm text-slate-400">
                No organizations returned by GitHub. Enter your org login directly:
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
                  onClick={() => handleConnectOrg(manualOrgLogin)}
                  disabled={connectingOrg || !manualOrgLogin.trim()}
                  className="rounded-lg bg-violet-600 px-4 py-2 text-sm font-semibold text-white hover:bg-violet-500 disabled:opacity-50 transition-colors"
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
                    <img src={o.avatar_url} alt={o.login} className="h-8 w-8 rounded-md shrink-0" />
                  )}
                  <div className="min-w-0">
                    <p className="text-sm font-semibold text-white truncate">{o.name || o.login}</p>
                    <p className="text-xs text-slate-400">@{o.login}</p>
                  </div>
                  <span className="ml-auto text-xs text-violet-400 shrink-0">
                    {connectingOrg ? 'Connecting...' : 'Connect →'}
                  </span>
                </button>
              ))}
              <div className="pt-2 border-t border-slate-800">
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
                    onClick={() => handleConnectOrg(manualOrgLogin)}
                    disabled={connectingOrg || !manualOrgLogin.trim()}
                    className="rounded-lg bg-violet-600 px-4 py-2 text-sm font-semibold text-white hover:bg-violet-500 disabled:opacity-50 transition-colors"
                  >
                    {connectingOrg ? 'Connecting...' : 'Connect'}
                  </button>
                </div>
              </div>
            </div>
          )}
        </div>
      )}

      <div className="flex items-center gap-3">
        <button
          onClick={onNext}
          disabled={copilotRequired && !connectedOrgLogin}
          className="rounded-xl bg-violet-600 px-6 py-3 text-sm font-semibold text-white hover:bg-violet-500 disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
        >
          Next →
        </button>
        {!copilotRequired && !connectedOrgLogin && (
          <button
            onClick={onSkip}
            className="text-sm text-slate-400 hover:text-slate-300 transition-colors"
          >
            Skip for now
          </button>
        )}
      </div>
    </div>
  )
}

// ---------------------------------------------------------------------------
// Step 4 — Install hook
// ---------------------------------------------------------------------------
function Step4Hook({
  selectedTools,
  onComplete,
  completing,
}: {
  selectedTools: string[]
  onComplete: () => void
  completing: boolean
}) {
  const [ingestKey, setIngestKey] = useState<string | null>(null)
  const [keyError, setKeyError] = useState<string | null>(null)
  const [copied, setCopied] = useState(false)

  const tool: 'claude_code' | 'codex' = selectedTools.includes('Claude Code')
    ? 'claude_code'
    : 'codex'

  useEffect(() => {
    let cancelled = false
    async function generateKey() {
      try {
        const result = await api.post<CreateIngestKeyResponse>('/api/settings/ingest-keys', {
          label: 'onboarding-key',
        })
        if (!cancelled) setIngestKey(result.key)
      } catch (e) {
        if (!cancelled)
          setKeyError(e instanceof Error ? e.message : 'Failed to generate ingest key')
      }
    }
    generateKey()
    return () => { cancelled = true }
  }, [])

  const snippet = ingestKey ? getSnippet(tool, ingestKey) : getSnippet(tool, 'aistats_live_your_key_here')

  function handleCopy() {
    navigator.clipboard.writeText(snippet).then(() => {
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    })
  }

  return (
    <div>
      <h2 className="text-2xl font-bold text-white mb-2">Install the tracking hook</h2>
      <p className="text-sm text-slate-400 mb-6">
        One command installs everything. Events are queued locally and sent when your session ends.
      </p>

      {keyError && (
        <p className="mb-4 text-sm text-red-400">{keyError}</p>
      )}

      {!ingestKey && !keyError && (
        <p className="mb-4 text-xs text-slate-500">Generating your ingest key...</p>
      )}

      <div className="relative rounded-xl bg-slate-950 border border-slate-800 p-4 mb-4">
        <pre className="text-xs text-slate-300 font-mono overflow-x-auto whitespace-pre-wrap pr-16">
          {snippet}
        </pre>
        <button
          onClick={handleCopy}
          className="absolute top-3 right-3 flex items-center gap-1.5 rounded-lg border border-slate-700 px-3 py-1.5 text-xs text-slate-300 hover:bg-slate-800 transition-colors"
        >
          {copied ? (
            <Check className="h-3.5 w-3.5 text-emerald-400" />
          ) : (
            <Copy className="h-3.5 w-3.5" />
          )}
          {copied ? 'Copied' : 'Copy'}
        </button>
      </div>

      <button
        onClick={onComplete}
        disabled={completing}
        className="w-full rounded-xl bg-gradient-to-r from-violet-500 to-purple-600 py-3 text-sm font-semibold text-white hover:opacity-90 disabled:opacity-50 transition-opacity"
      >
        {completing ? 'Setting up...' : 'Done — go to dashboard'}
      </button>
    </div>
  )
}

// ---------------------------------------------------------------------------
// Main OnboardingPage
// ---------------------------------------------------------------------------
export function OnboardingPage() {
  const navigate = useNavigate()
  const [step, setStep] = useState(1)
  const [selectedTools, setSelectedTools] = useState<string[]>([])
  const [connectedOrgLogin, setConnectedOrgLogin] = useState<string | null>(null)
  const [completing, setCompleting] = useState(false)
  const [completeError, setCompleteError] = useState<string | null>(null)

  // Guard: if onboarding already complete, redirect to dashboard
  useEffect(() => {
    api
      .get<{ completed: boolean }>('/api/onboarding')
      .then((data) => {
        if (data.completed) navigate('/dashboard', { replace: true })
      })
      .catch(() => {
        // If the endpoint fails (e.g. unauthenticated), let normal auth redirect handle it
      })
  }, [navigate])

  const needsHookStep =
    selectedTools.includes('Claude Code') || selectedTools.includes('OpenAI Codex')

  // The visible step count depends on whether step 4 will appear.
  // We always show 4 dots when hook step is possible, 3 when it isn't.
  // We keep it simple: always show 4-step indicator; step 4 may be skipped.
  const totalSteps = 4

  function toggleTool(tool: string) {
    setSelectedTools((prev) =>
      prev.includes(tool) ? prev.filter((t) => t !== tool) : [...prev, tool]
    )
  }

  async function handleComplete() {
    setCompleting(true)
    setCompleteError(null)
    try {
      await api.post('/api/onboarding/complete', {
        tools: selectedTools,
        github_org_login: connectedOrgLogin,
      })
      navigate('/dashboard')
    } catch (e) {
      setCompleteError(e instanceof Error ? e.message : 'Failed to complete onboarding')
      setCompleting(false)
    }
  }

  // Advance from step 3: if hook step not needed, complete immediately
  function handleStep3Next() {
    if (needsHookStep) {
      setStep(4)
    } else {
      handleComplete()
    }
  }

  function handleStep3Skip() {
    if (needsHookStep) {
      setStep(4)
    } else {
      handleComplete()
    }
  }

  return (
    <div className="min-h-screen bg-slate-950 flex items-center justify-center p-6">
      <div className="w-full max-w-lg">
        {step > 1 && <StepIndicator step={step} total={totalSteps} />}

        {completeError && (
          <div className="mb-6 rounded-xl bg-red-500/10 border border-red-500/20 p-4 text-sm text-red-400">
            {completeError}
          </div>
        )}

        {step === 1 && <Step1Welcome onNext={() => setStep(2)} />}

        {step === 2 && (
          <Step2Tools
            selected={selectedTools}
            onToggle={toggleTool}
            onNext={() => setStep(3)}
          />
        )}

        {step === 3 && (
          <Step3GitHub
            selectedTools={selectedTools}
            connectedOrgLogin={connectedOrgLogin}
            onConnect={setConnectedOrgLogin}
            onNext={handleStep3Next}
            onSkip={handleStep3Skip}
          />
        )}

        {step === 4 && (
          <Step4Hook
            selectedTools={selectedTools}
            onComplete={handleComplete}
            completing={completing}
          />
        )}
      </div>
    </div>
  )
}
