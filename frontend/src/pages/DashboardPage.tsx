import { useState, useEffect } from 'react'
import { RefreshCw, Settings, AlertCircle } from 'lucide-react'
import { Link } from 'react-router-dom'
import { api, type DashboardResponse, type SyncAllResponse } from '../api/client'
import { USE_MOCK, mockDashboard } from '../api/mock'
import { MetricCard } from '../components/dashboard/MetricCard'
import { AcceptanceChart } from '../components/charts/AcceptanceChart'
import { SuggestionsChart } from '../components/charts/SuggestionsChart'
import { ActiveUsersChart } from '../components/charts/ActiveUsersChart'
import { LanguageChart } from '../components/charts/LanguageChart'
import { formatCurrency, formatHours } from '../lib/utils'

export function DashboardPage() {
  const [data, setData] = useState<DashboardResponse | null>(null)
  const [loading, setLoading] = useState(true)
  const [syncing, setSyncing] = useState(false)
  const [loadingDemo, setLoadingDemo] = useState(false)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    loadDashboard()
  }, [])

  async function loadDashboard() {
    try {
      setLoading(true)
      setError(null)
      const result = USE_MOCK
        ? mockDashboard
        : await api.get<DashboardResponse>('/api/dashboard')
      setData(result)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to load dashboard')
    } finally {
      setLoading(false)
    }
  }

  async function handleSync() {
    setSyncing(true)
    try {
      await api.post<SyncAllResponse>('/api/tools/sync/all')
      await loadDashboard()
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Sync failed')
    } finally {
      setSyncing(false)
    }
  }

  async function handleLoadDemo() {
    setLoadingDemo(true)
    try {
      await api.post('/api/demo/activate')
      await loadDashboard()
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to load demo')
    } finally {
      setLoadingDemo(false)
    }
  }

  if (loading) {
    return (
      <div className="flex h-full items-center justify-center p-8">
        <div className="text-slate-400">Loading...</div>
      </div>
    )
  }

  if (error && error !== 'no_org') {
    return (
      <div className="flex h-full items-center justify-center p-8">
        <div className="text-red-400">{error}</div>
      </div>
    )
  }

  if (error === 'no_org' || !data) {
    return (
      <div className="flex h-full items-center justify-center p-8">
        <div className="max-w-md text-center">
          <div className="mx-auto mb-4 flex h-14 w-14 items-center justify-center rounded-2xl bg-slate-800 border border-slate-700">
            <AlertCircle className="h-6 w-6 text-slate-400" />
          </div>
          <h2 className="text-xl font-bold text-white">Connect your GitHub org</h2>
          <p className="mt-2 text-sm text-slate-400">
            To see your AI tool analytics, connect your GitHub organization in Settings.
          </p>
          <div className="mt-6 flex flex-col items-center gap-3">
            <button
              onClick={handleLoadDemo}
              disabled={loadingDemo}
              className="inline-flex items-center gap-2 rounded-xl bg-gradient-to-r from-violet-500 to-purple-600 px-6 py-3 text-sm font-semibold text-white hover:opacity-90 disabled:opacity-60"
            >
              <RefreshCw className={`h-4 w-4 ${loadingDemo ? 'animate-spin' : ''}`} />
              {loadingDemo ? 'Loading...' : 'Try with demo data'}
            </button>
            <Link
              to="/settings"
              className="inline-flex items-center gap-2 rounded-xl border border-slate-700 px-6 py-3 text-sm font-semibold text-slate-300 hover:bg-slate-800 transition-colors"
            >
              <Settings className="h-4 w-4" />
              Connect real org
            </Link>
          </div>
        </div>
      </div>
    )
  }

  const { org, metrics, charts } = data
  const hasData = (charts.acceptance_rate?.length ?? 0) > 0

  // No data yet state
  if (!hasData) {
    return (
      <div className="flex h-full items-center justify-center p-8">
        <div className="max-w-md text-center">
          <div className="mx-auto mb-4 flex h-14 w-14 items-center justify-center rounded-2xl bg-violet-500/10 border border-violet-500/20">
            <RefreshCw className="h-6 w-6 text-violet-400" />
          </div>
          <h2 className="text-xl font-bold text-white">Syncing your data...</h2>
          <p className="mt-2 text-sm text-slate-400">
            Your organization is connected. Click sync to pull your AI tool usage data.
          </p>
          <button
            onClick={handleSync}
            disabled={syncing}
            className="mt-6 inline-flex items-center gap-2 rounded-xl bg-gradient-to-r from-violet-500 to-purple-600 px-6 py-3 text-sm font-semibold text-white hover:opacity-90 disabled:opacity-50"
          >
            <RefreshCw className={`h-4 w-4 ${syncing ? 'animate-spin' : ''}`} />
            {syncing ? 'Syncing...' : 'Sync data now'}
          </button>
        </div>
      </div>
    )
  }

  return (
    <div className="p-8">
      {/* Header */}
      <div className="flex items-center justify-between mb-8">
        <div>
          <h1 className="text-2xl font-bold text-white">{org.name}</h1>
          <p className="text-sm text-slate-400 mt-1">Last 30 days · Updated hourly</p>
        </div>
        <button
          onClick={handleSync}
          disabled={syncing}
          className="flex items-center gap-2 rounded-lg border border-slate-700 bg-slate-800 px-4 py-2 text-sm text-slate-300 hover:bg-slate-700 transition-colors disabled:opacity-50"
        >
          <RefreshCw className={`h-4 w-4 ${syncing ? 'animate-spin' : ''}`} />
          {syncing ? 'Syncing...' : 'Sync now'}
        </button>
      </div>

      {/* Metric Cards */}
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4 mb-8">
        <MetricCard
          label="Active Users"
          value={metrics.active_users.toString()}
          change={metrics.active_users_change}
        />
        <MetricCard
          label="Acceptance Rate"
          value={`${metrics.acceptance_rate}%`}
          change={Math.round(metrics.acceptance_rate_change * 10) / 10}
          changeLabel="pts vs last month"
        />
        <MetricCard
          label="Lines Accepted"
          value={metrics.lines_accepted.toLocaleString()}
          change={metrics.lines_accepted_change}
        />
        <MetricCard
          label="Est. Value Saved"
          value={formatCurrency(metrics.dollar_value_saved)}
          description={`${formatHours(metrics.hours_saved)} saved · $${metrics.hourly_rate}/hr`}
          className="border-violet-500/30 bg-gradient-to-br from-violet-500/5 to-purple-600/5"
        />
        {org.github_member_count !== null && (() => {
          const memberCount = org.github_member_count!
          const activeUsers = metrics.active_users
          // GitHub's /orgs/{org}/members excludes the org owner, so effective
          // denominator is memberCount + 1 (the owner). When memberCount is 0
          // the org has only the owner, so show active users without a rate.
          const total = memberCount + 1
          const rate = (activeUsers / total) * 100
          const isLow = rate < 25
          return (
            <MetricCard
              label="Adoption Rate"
              value={`${activeUsers} / ${total}`}
              description={
                <span className={isLow ? 'text-amber-400' : 'text-emerald-400'}>
                  {Math.round(rate)}% adoption
                </span>
              }
              className={
                isLow
                  ? 'border-amber-500/30 bg-amber-500/5'
                  : 'border-emerald-500/30 bg-emerald-500/5'
              }
            />
          )
        })()}
      </div>

      {/* Charts row 1 */}
      <div className="grid gap-6 lg:grid-cols-2 mb-6">
        <div className="rounded-xl border border-slate-800 bg-slate-900 p-6">
          <h3 className="text-sm font-medium text-slate-300 mb-4">
            Acceptance Rate — Last 90 Days
          </h3>
          <div className="h-48">
            <AcceptanceChart data={charts.acceptance_rate} />
          </div>
        </div>
        <div className="rounded-xl border border-slate-800 bg-slate-900 p-6">
          <h3 className="text-sm font-medium text-slate-300 mb-4">Daily Active Users</h3>
          <div className="h-48">
            <ActiveUsersChart data={charts.active_users} />
          </div>
        </div>
      </div>

      {/* Charts row 2 */}
      <div className="grid gap-6 lg:grid-cols-2">
        <div className="rounded-xl border border-slate-800 bg-slate-900 p-6">
          <h3 className="text-sm font-medium text-slate-300 mb-4">
            Suggestions vs Acceptances
          </h3>
          <div className="h-48">
            <SuggestionsChart data={charts.suggestions} />
          </div>
        </div>
        {(charts.languages?.length ?? 0) > 0 && (
          <div className="rounded-xl border border-slate-800 bg-slate-900 p-6">
            <h3 className="text-sm font-medium text-slate-300 mb-4">
              Acceptance Rate by Language
            </h3>
            <div className="h-48">
              <LanguageChart data={charts.languages} />
            </div>
          </div>
        )}
      </div>
    </div>
  )
}
