import { useState, useEffect } from 'react'
import { BarChart, Bar, XAxis, YAxis, Tooltip, ResponsiveContainer, LineChart, Line } from 'recharts'
import { TrendingUp, TrendingDown, Minus } from 'lucide-react'
import { api, type InsightsResponse, type PlanningTrendPoint } from '../api/client'
import { USE_MOCK, mockInsights } from '../api/mock'
import { formatCurrency, formatHours } from '../lib/utils'
import { MetricCard } from '../components/dashboard/MetricCard'

const TOOL_LABELS: Record<string, string> = {
  copilot: 'GitHub Copilot',
  claude_code: 'Claude Code',
  codex: 'OpenAI Codex',
}

function WoWBadge({ current, prev }: { current: number; prev: number }) {
  const diff = current - prev
  const isPositive = diff > 0
  const isNegative = diff < 0

  return (
    <div className="mt-2 flex items-center gap-1.5">
      {isPositive && <TrendingUp className="h-3.5 w-3.5 text-emerald-400" />}
      {isNegative && <TrendingDown className="h-3.5 w-3.5 text-red-400" />}
      {!isPositive && !isNegative && <Minus className="h-3.5 w-3.5 text-slate-500" />}
      <span
        className={
          isPositive
            ? 'text-xs font-medium text-emerald-400'
            : isNegative
              ? 'text-xs font-medium text-red-400'
              : 'text-xs font-medium text-slate-500'
        }
      >
        {isPositive && '+'}
        {diff} vs prior week
      </span>
    </div>
  )
}

export function InsightsPage() {
  const [data, setData] = useState<InsightsResponse | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    const load = async () => {
      try {
        const result = USE_MOCK
          ? mockInsights
          : await api.get<InsightsResponse>('/api/insights')
        setData(result)
      } catch (e) {
        setError(e instanceof Error ? e.message : 'Failed to load insights')
      } finally {
        setLoading(false)
      }
    }
    load()
  }, [])

  if (loading) {
    return (
      <div className="flex h-full items-center justify-center p-8">
        <div className="text-slate-400">Loading...</div>
      </div>
    )
  }

  if (error || !data) {
    return (
      <div className="flex h-full items-center justify-center p-8">
        <div className="text-red-400">{error ?? 'No data'}</div>
      </div>
    )
  }

  // Empty state — no activity at all
  if (data.ai_active_users === 0) {
    return (
      <div className="p-8">
        <div className="mb-8">
          <h1 className="text-2xl font-bold text-white">Insights</h1>
          <p className="text-sm text-slate-400 mt-1">Last 30 days breakdown</p>
        </div>
        <div className="rounded-xl border border-slate-800 bg-slate-900 p-10 text-center">
          <p className="text-sm text-slate-400">
            No activity recorded yet. Start tracking by installing the Claude Code hook or
            connecting a GitHub org in Settings.
          </p>
        </div>
      </div>
    )
  }

  const toolBreakdown = data.tool_breakdown ?? []
  const languages = data.languages ?? []
  const editors = data.editors ?? []

  const hasCopilotData = toolBreakdown.some(
    (t) => t.tool_source === 'copilot' && t.active_users > 0,
  )

  return (
    <div className="p-8">
      <div className="mb-8">
        <h1 className="text-2xl font-bold text-white">Insights</h1>
        <p className="text-sm text-slate-400 mt-1">Last 30 days breakdown</p>
      </div>

      {/* Headline stat cards */}
      <div className="mb-8 grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <MetricCard
          label="AI Adoption"
          value={
            data.total_coding_developers > 0 ? `${data.utilization_percent}%` : '—'
          }
          change={
            data.total_coding_developers > 0 && data.utilization_delta_pp !== 0
              ? data.utilization_delta_pp
              : undefined
          }
          changeLabel="pts vs prior 30d"
          description={
            data.total_coding_developers > 0
              ? `${data.ai_active_users} of ${data.total_coding_developers} developers`
              : undefined
          }
        />
        <MetricCard
          label="Est. Value Saved"
          value={formatCurrency(data.dollar_value_saved)}
          description={`${formatHours(data.hours_saved)} saved`}
        />
        <MetricCard
          label="Edit Rate"
          value={`${data.acceptance_rate}%`}
          description="Accepted AI suggestions"
        />
        <MetricCard
          label="Lines Accepted"
          value={data.lines_accepted.toLocaleString()}
          description="AI-written lines merged"
        />
      </div>

      {/* Claude Code: Planning Ratio */}
      {(data.planning_tool_uses + data.coding_tool_uses) === 0 && (
        <div className="mb-8 rounded-xl border border-violet-500/20 bg-violet-950/10 p-6">
          <h2 className="text-base font-semibold text-violet-300 mb-1">Claude Code: Planning Ratio</h2>
          <p className="text-sm text-slate-500">
            Tracks how often your team uses Claude to plan and research vs. write code directly.
            Data appears after your first Claude Code session ends.
          </p>
        </div>
      )}
      {(data.planning_tool_uses + data.coding_tool_uses) > 0 && (() => {
        const ratio = data.planning_ratio
        const delta = data.planning_ratio - data.prev_planning_ratio
        const totalUses = data.planning_tool_uses + data.coding_tool_uses
        const planningWidth = totalUses > 0 ? (data.planning_tool_uses / totalUses) * 100 : 0

        let interpretiveLabel: string
        if (ratio < 20) {
          interpretiveLabel = 'Your team primarily uses Claude to write code'
        } else if (ratio <= 60) {
          interpretiveLabel = 'Your team uses Claude for both planning and implementation'
        } else {
          interpretiveLabel = 'Your team uses Claude primarily as a thinking partner'
        }

        return (
          <div className="mb-8 rounded-xl border border-violet-500/30 bg-violet-950/20 p-6">
            <h2 className="text-base font-semibold text-violet-300 mb-4">
              Claude Code: Planning Ratio
            </h2>

            {/* Large % + delta badge */}
            <div className="flex items-center gap-3 mb-4">
              <span className="text-4xl font-bold text-white">{ratio.toFixed(1)}%</span>
              {delta !== 0 ? (
                <span
                  className={`inline-flex items-center gap-1 rounded-full px-2.5 py-1 text-xs font-medium ${
                    delta > 0
                      ? 'bg-emerald-900/50 text-emerald-400'
                      : 'bg-red-900/50 text-red-400'
                  }`}
                >
                  {delta > 0 ? '↑' : '↓'} {delta > 0 ? '+' : ''}{delta.toFixed(1)}pts vs prior 30d
                </span>
              ) : (
                <span className="inline-flex items-center gap-1 rounded-full px-2.5 py-1 text-xs font-medium bg-slate-800 text-slate-500">
                  — vs prior 30d
                </span>
              )}
            </div>

            {/* Stacked bar */}
            <div className="mb-2 h-3 rounded-full overflow-hidden bg-slate-700 flex">
              <div
                className="h-full bg-violet-500"
                style={{ width: `${planningWidth}%` }}
              />
              <div
                className="h-full bg-slate-600"
                style={{ width: `${100 - planningWidth}%` }}
              />
            </div>
            <p className="text-xs text-slate-400 mb-3">
              {data.planning_tool_uses.toLocaleString()} planning uses &middot;{' '}
              {data.coding_tool_uses.toLocaleString()} coding uses
            </p>

            {/* Interpretive label */}
            <p className="text-sm text-slate-300 italic mb-4">"{interpretiveLabel}"</p>

            {/* Trend chart */}
            <ResponsiveContainer width="100%" height={120}>
              <LineChart
                data={data.planning_trend}
                margin={{ top: 4, right: 4, bottom: 4, left: 4 }}
              >
                <XAxis dataKey="date" hide />
                <YAxis hide domain={['auto', 'auto']} />
                <Tooltip
                  contentStyle={{
                    background: '#1e1b4b',
                    border: '1px solid #4c1d95',
                    borderRadius: 8,
                  }}
                  labelStyle={{ color: '#f8fafc', fontSize: 11 }}
                  itemStyle={{ color: '#c4b5fd', fontSize: 11 }}
                  formatter={(value: number, name: string) => {
                    if (name === 'planning_ratio_percent') return [`${Number(value).toFixed(1)}%`, 'Planning ratio']
                    if (name === 'planning_uses') return [value, 'Planning uses']
                    if (name === 'coding_uses') return [value, 'Coding uses']
                    return [value, name]
                  }}
                />
                <Line
                  type="monotone"
                  dataKey="planning_ratio_percent"
                  stroke="#8b5cf6"
                  strokeWidth={2}
                  dot={(props: {
                    cx: number
                    cy: number
                    payload: PlanningTrendPoint
                    index: number
                  }) => {
                    const { cx, cy, payload } = props
                    const lowVolume = payload.planning_uses + payload.coding_uses < 5
                    return (
                      <circle
                        key={`dot-${payload.date}`}
                        cx={cx}
                        cy={cy}
                        r={3}
                        fill="#8b5cf6"
                        stroke="#8b5cf6"
                        opacity={lowVolume ? 0.4 : 1}
                      />
                    )
                  }}
                />
              </LineChart>
            </ResponsiveContainer>

            <p className="mt-3 text-xs text-slate-500">
              Bash is classified as planning. Claude Code only.
            </p>
          </div>
        )
      })()}

      {/* Weekly Active Users small stat */}
      <div className="mb-8 rounded-xl border border-slate-800 bg-slate-900 p-6 inline-block min-w-[220px]">
        <p className="text-sm font-medium text-slate-400">Weekly Active Users</p>
        <p className="mt-2 text-3xl font-bold text-white">{data.weekly_active_users}</p>
        <p className="text-xs text-slate-500 mt-0.5">active this week</p>
        <WoWBadge current={data.weekly_active_users} prev={data.prev_week_active_users} />
      </div>

      {/* Tool Breakdown */}
      <div className="mb-8 rounded-xl border border-slate-800 bg-slate-900 p-6">
        <h2 className="text-base font-semibold text-white mb-1">AI Tools in Use</h2>
        <p className="text-xs text-slate-500 mb-6">Active users by tool — last 30 days</p>
        {toolBreakdown.length === 0 ? (
          <p className="text-sm text-slate-500">No tool usage data available yet.</p>
        ) : (
          <>
            <ResponsiveContainer width="100%" height={160}>
              <BarChart
                data={toolBreakdown.map((t) => ({
                  ...t,
                  label: TOOL_LABELS[t.tool_source] ?? t.tool_source,
                }))}
                margin={{ top: 4, right: 4, bottom: 4, left: 4 }}
              >
                <XAxis
                  dataKey="label"
                  tick={{ fill: '#94a3b8', fontSize: 12 }}
                  axisLine={false}
                  tickLine={false}
                />
                <Tooltip
                  cursor={{ fill: 'rgba(255,255,255,0.05)' }}
                  contentStyle={{
                    background: '#1e1b4b',
                    border: '1px solid #4c1d95',
                    borderRadius: 8,
                  }}
                  labelStyle={{ color: '#f8fafc' }}
                  itemStyle={{ color: '#c4b5fd' }}
                  formatter={(v: number) => [`${v} users`, 'Active Users']}
                />
                <Bar dataKey="active_users" fill="#8b5cf6" radius={[4, 4, 0, 0]} />
              </BarChart>
            </ResponsiveContainer>

            {/* Per-tool stats below chart */}
            <div className="mt-4 grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
              {toolBreakdown.map((t) => (
                <div
                  key={t.tool_source}
                  className="rounded-lg border border-slate-800 bg-slate-800/50 px-4 py-3"
                >
                  <p className="text-xs font-semibold text-slate-300 mb-0.5">
                    {TOOL_LABELS[t.tool_source] ?? t.tool_source}
                  </p>
                  <p className="text-xs text-slate-500">
                    {t.active_users.toLocaleString()} users &middot;{' '}
                    {t.lines_accepted.toLocaleString()} lines
                  </p>
                </div>
              ))}
            </div>
          </>
        )}
      </div>

      {/* Daily Active Users */}
      <div className="mb-8 rounded-xl border border-slate-800 bg-slate-900 p-6">
        <h2 className="text-base font-semibold text-white mb-2">Daily Active Users</h2>
        <p className="text-xs text-slate-500 mb-6">Daily active users — last 30 days</p>
        <ResponsiveContainer width="100%" height={160}>
          <BarChart
            data={data.activity_heatmap}
            margin={{ top: 4, right: 4, bottom: 0, left: 0 }}
          >
            <XAxis
              dataKey="date"
              tickFormatter={(v: string) => v.slice(5)}
              tick={{ fill: '#94a3b8', fontSize: 11 }}
              axisLine={false}
              tickLine={false}
              interval={6}
            />
            <Tooltip
              cursor={{ fill: 'rgba(255,255,255,0.05)' }}
              contentStyle={{
                background: '#1e1b4b',
                border: '1px solid #4c1d95',
                borderRadius: 8,
              }}
              labelStyle={{ color: '#f8fafc' }}
              itemStyle={{ color: '#c4b5fd' }}
              formatter={(v: number) => [`${v} users`, 'Active Users']}
            />
            <Bar dataKey="users" fill="#8b5cf6" radius={[3, 3, 0, 0]} />
          </BarChart>
        </ResponsiveContainer>
      </div>

      {/* Utilization Trend */}
      <div className="mb-8 rounded-xl border border-violet-500/30 bg-violet-950/20 p-6">
        <div className="flex items-center justify-between">
          <div>
            <p className="text-sm font-medium text-violet-300 mb-1">Utilization Trend</p>
            <p className="text-4xl font-bold text-white">
              {data.total_coding_developers > 0 ? `${data.utilization_percent}%` : '—'}
            </p>
            {data.total_coding_developers > 0 && (
              <p className="text-sm mt-1 mb-2">
                {data.utilization_delta_pp > 0 ? (
                  <span className="text-green-400">
                    ↑ +{data.utilization_delta_pp.toFixed(1)}pts vs prior 30d
                  </span>
                ) : data.utilization_delta_pp < 0 ? (
                  <span className="text-red-400">
                    ↓ {Math.abs(data.utilization_delta_pp).toFixed(1)}pts vs prior 30d
                  </span>
                ) : (
                  <span className="text-slate-500">— vs prior 30d</span>
                )}
              </p>
            )}
            <p className="text-sm text-slate-400 mt-2">
              {data.total_coding_developers > 0
                ? `${data.ai_active_users} of ${data.total_coding_developers} developers used an AI coding tool in the last 30 days`
                : data.ai_active_users > 0
                  ? `${data.ai_active_users} developer${data.ai_active_users !== 1 ? 's' : ''} actively used AI coding tools in the last 30 days`
                  : 'No activity recorded yet — sync to pull latest data'}
            </p>
          </div>
          <div className="hidden sm:block" style={{ width: 200, height: 80 }}>
            <ResponsiveContainer width="100%" height="100%">
              <LineChart
                data={data.utilization_trend}
                margin={{ top: 4, right: 4, bottom: 4, left: 4 }}
              >
                <Line
                  type="monotone"
                  dataKey="utilization_percent"
                  stroke="#8b5cf6"
                  strokeWidth={2}
                  dot={false}
                />
                <Tooltip
                  cursor={{ fill: 'rgba(255,255,255,0.05)' }}
                  contentStyle={{
                    background: '#1e1b4b',
                    border: '1px solid #4c1d95',
                    borderRadius: 8,
                  }}
                  labelStyle={{ color: '#f8fafc', fontSize: 11 }}
                  itemStyle={{ color: '#c4b5fd', fontSize: 11 }}
                  formatter={(v: number) => [`${v}%`, 'Utilization']}
                />
              </LineChart>
            </ResponsiveContainer>
          </div>
        </div>
        <p className="mt-4 text-xs text-slate-500">
          Utilization = unique AI tool users ÷ total developers who committed code.
          Sources: git trailer attribution (Claude Code) + push ingest (all tools).
        </p>
      </div>

      <div className="grid gap-8 lg:grid-cols-2">
        {/* Language Breakdown */}
        <div className="rounded-xl border border-slate-800 bg-slate-900 p-6">
          <h2 className="text-base font-semibold text-white mb-6">By Language</h2>
          <div className="space-y-4">
            {languages.length > 0 ? (
              languages.map((lang) => (
                <div key={lang.language}>
                  <div className="flex items-center justify-between mb-1.5">
                    <span className="text-sm font-medium text-slate-200">{lang.language}</span>
                    <div className="flex items-center gap-4 text-xs text-slate-400">
                      <span>{lang.suggestions.toLocaleString()} suggestions</span>
                      <span className="font-semibold text-violet-400">{lang.rate}%</span>
                    </div>
                  </div>
                  <div className="h-1.5 rounded-full bg-slate-800">
                    <div
                      className="h-1.5 rounded-full bg-gradient-to-r from-violet-500 to-purple-600"
                      style={{ width: `${Math.min(lang.rate, 100)}%` }}
                    />
                  </div>
                </div>
              ))
            ) : hasCopilotData ? (
              <p className="text-sm text-slate-500">No language data returned for this period.</p>
            ) : (
              <p className="text-sm text-slate-500">
                Language breakdown is available with GitHub Copilot. Other tools report aggregate
                metrics only.
              </p>
            )}
          </div>
        </div>

        {/* Editor Breakdown */}
        <div className="rounded-xl border border-slate-800 bg-slate-900 p-6">
          <h2 className="text-base font-semibold text-white mb-6">By Editor</h2>
          <div className="space-y-4">
            {editors.length > 0 ? (
              editors.map((editor) => (
                <div
                  key={editor.editor}
                  className="flex items-center justify-between py-3 border-b border-slate-800 last:border-0"
                >
                  <div>
                    <p className="text-sm font-medium text-slate-200">{editor.editor}</p>
                    <p className="text-xs text-slate-500 mt-0.5">
                      {editor.users.toLocaleString()} active users
                    </p>
                  </div>
                  <div className="text-right">
                    <p className="text-sm font-bold text-violet-400">{editor.rate}%</p>
                    <p className="text-xs text-slate-500">edit rate</p>
                  </div>
                </div>
              ))
            ) : hasCopilotData ? (
              <p className="text-sm text-slate-500">No editor data returned for this period.</p>
            ) : (
              <p className="text-sm text-slate-500">
                Editor breakdown is available with GitHub Copilot. Other tools report aggregate
                metrics only.
              </p>
            )}
          </div>
        </div>
      </div>
    </div>
  )
}
