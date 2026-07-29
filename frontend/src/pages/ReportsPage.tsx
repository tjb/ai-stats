import { useState, useEffect } from 'react'
import { FileText, Link as LinkIcon, Check } from 'lucide-react'
import { reportsApi, type Report } from '../api/client'

// ---- Helpers ----

function fmt(n: number): string {
  return n.toLocaleString()
}

function fmtPct(n: number): string {
  return n.toFixed(1) + '%'
}

function fmtMoney(n: number): string {
  return '$' + n.toLocaleString('en-US', { minimumFractionDigits: 0, maximumFractionDigits: 0 })
}

function todayStr(): string {
  return new Date().toISOString().slice(0, 10)
}

function firstOfMonthStr(): string {
  const d = new Date()
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-01`
}

function presetRange(days: number): { start: string; end: string } {
  const end = new Date()
  const start = new Date()
  start.setDate(end.getDate() - (days - 1))
  return {
    end: end.toISOString().slice(0, 10),
    start: start.toISOString().slice(0, 10),
  }
}

// ---- Stat card ----

function StatCard({ label, value, sub }: { label: string; value: string; sub?: string }) {
  return (
    <div className="rounded-xl border border-slate-800 bg-slate-900 p-5">
      <p className="text-xs text-slate-400 uppercase tracking-wide">{label}</p>
      <p className="mt-1 text-2xl font-bold text-white">{value}</p>
      {sub && <p className="mt-0.5 text-xs text-slate-500">{sub}</p>}
    </div>
  )
}

// ---- Section wrapper ----

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div className="rounded-xl border border-slate-800 bg-slate-900 p-6">
      <h3 className="text-sm font-semibold text-slate-300 uppercase tracking-wide mb-4">{title}</h3>
      {children}
    </div>
  )
}

// ---- Report detail view ----

function ReportView({ report }: { report: Report }) {
  const c = report.content
  const planningTotal = c.planning_tool_uses + c.coding_tool_uses
  const planningWidth = planningTotal > 0 ? (c.planning_tool_uses / planningTotal) * 100 : 0

  return (
    <div className="space-y-4 mt-6">
      {/* Executive Summary */}
      <Section title="Executive Summary">
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
          <StatCard label="Lines Accepted" value={fmt(c.total_accepted_lines)} />
          <StatCard label="Active Developers" value={fmt(c.active_developers)} />
          <StatCard
            label="Hours Saved"
            value={c.estimated_hours_saved.toFixed(1) + ' hrs'}
          />
          <StatCard
            label="Est. Value Saved"
            value={fmtMoney(c.estimated_value_saved)}
            sub={`at $${c.hourly_rate}/hr`}
          />
        </div>
      </Section>

      {/* Adoption */}
      {c.adoption_rate !== null && (
        <Section title="Adoption">
          <div className="grid gap-4 sm:grid-cols-2">
            <StatCard
              label="Adoption Rate"
              value={fmtPct(c.adoption_rate)}
              sub={
                c.github_member_count !== null
                  ? `${c.active_developers} of ${c.github_member_count} members`
                  : undefined
              }
            />
            {c.github_member_count !== null && (
              <StatCard label="GitHub Members" value={fmt(c.github_member_count)} />
            )}
          </div>
        </Section>
      )}

      {/* Quality Signal */}
      <Section title="Quality Signal">
        <div className="grid gap-4 sm:grid-cols-2">
          <StatCard label="Acceptance Rate" value={fmtPct(c.acceptance_rate)} />
          <StatCard label="Suggested Lines" value={fmt(c.total_suggested_lines)} />
        </div>
      </Section>

      {/* ROI Detail */}
      <Section title="ROI Detail">
        <div className="grid gap-4 sm:grid-cols-3">
          <StatCard label="Hours Saved" value={c.estimated_hours_saved.toFixed(1) + ' hrs'} />
          <StatCard label="Value Saved" value={fmtMoney(c.estimated_value_saved)} />
          <StatCard label="Hourly Rate Used" value={`$${c.hourly_rate}/hr`} />
        </div>
      </Section>

      {/* Planning vs Coding */}
      {planningTotal > 0 && (
        <Section title="Planning vs. Coding">
          <div className="grid gap-4 sm:grid-cols-3 mb-4">
            <StatCard label="Planning Ratio" value={fmtPct(c.planning_ratio)} />
            <StatCard label="Planning Uses" value={fmt(c.planning_tool_uses)} />
            <StatCard label="Coding Uses" value={fmt(c.coding_tool_uses)} />
          </div>
          <div className="h-2.5 rounded-full overflow-hidden bg-slate-700 flex">
            <div className="h-full bg-violet-500" style={{ width: `${planningWidth}%` }} />
            <div className="h-full bg-slate-600" style={{ width: `${100 - planningWidth}%` }} />
          </div>
          <p className="mt-2 text-xs text-slate-500">
            Violet = planning &middot; gray = coding. Bash classified as planning.
          </p>
        </Section>
      )}
    </div>
  )
}

// ---- Main page ----

export function ReportsPage() {
  const [startDate, setStartDate] = useState<string>(firstOfMonthStr())
  const [endDate, setEndDate] = useState<string>(todayStr())
  const [reports, setReports] = useState<Report[]>([])
  const [selectedReport, setSelectedReport] = useState<Report | null>(null)
  const [generating, setGenerating] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [copiedId, setCopiedId] = useState<string | null>(null)

  useEffect(() => {
    reportsApi.list().then(setReports).catch(() => setError('Failed to load reports'))
  }, [])

  async function handleGenerate() {
    setGenerating(true)
    setError(null)
    try {
      const report = await reportsApi.generate(startDate, endDate)
      setReports((prev) => [report, ...prev])
      setSelectedReport(report)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to generate report')
    } finally {
      setGenerating(false)
    }
  }

  function applyPreset(days: number) {
    const { start, end } = presetRange(days)
    setStartDate(start)
    setEndDate(end)
  }

  function handleCopyLink(report: Report) {
    if (!report.share_token) return
    const url = `${window.location.origin}/r/${report.share_token}`
    navigator.clipboard.writeText(url)
    setCopiedId(report.id)
    setTimeout(() => setCopiedId(null), 2000)
  }

  return (
    <div className="p-8">
      {/* Header */}
      <div className="mb-8">
        <h1 className="text-2xl font-bold text-white">Reports</h1>
        <p className="text-sm text-slate-400 mt-1">Generate and share ROI summaries for any date range</p>
      </div>

      {/* Date range controls */}
      <div className="rounded-xl border border-slate-800 bg-slate-900 p-5 mb-6">
        <div className="flex flex-wrap items-end gap-3">
          <div>
            <label className="block text-xs text-slate-400 mb-1">Start date</label>
            <input
              type="date"
              value={startDate}
              onChange={(e) => setStartDate(e.target.value)}
              className="rounded-lg border border-slate-700 bg-slate-800 px-3 py-2 text-sm text-white focus:outline-none focus:ring-1 focus:ring-violet-500"
            />
          </div>
          <div>
            <label className="block text-xs text-slate-400 mb-1">End date</label>
            <input
              type="date"
              value={endDate}
              onChange={(e) => setEndDate(e.target.value)}
              className="rounded-lg border border-slate-700 bg-slate-800 px-3 py-2 text-sm text-white focus:outline-none focus:ring-1 focus:ring-violet-500"
            />
          </div>
          <button
            onClick={handleGenerate}
            disabled={generating}
            className="flex items-center gap-2 rounded-lg bg-gradient-to-r from-violet-500 to-purple-600 px-4 py-2 text-sm font-semibold text-white hover:opacity-90 transition-opacity disabled:opacity-50"
          >
            <FileText className="h-4 w-4" />
            {generating ? 'Generating...' : 'Generate Report'}
          </button>
          <div className="flex gap-2 ml-auto">
            {([7, 30, 90] as const).map((d) => (
              <button
                key={d}
                onClick={() => applyPreset(d)}
                className="rounded-lg border border-slate-700 px-3 py-2 text-xs text-slate-300 hover:bg-slate-800 transition-colors"
              >
                Last {d} days
              </button>
            ))}
          </div>
        </div>
      </div>

      {error && (
        <div className="mb-4 rounded-lg border border-red-500/20 bg-red-500/10 p-4 text-sm text-red-400">
          {error}
        </div>
      )}

      {/* Report history */}
      {reports.length > 0 && (
        <div className="rounded-xl border border-slate-800 bg-slate-900 overflow-hidden mb-6">
          <div className="px-5 py-3 border-b border-slate-800">
            <p className="text-xs font-semibold text-slate-400 uppercase tracking-wide">Report History</p>
          </div>
          <div className="divide-y divide-slate-800">
            {reports.map((r) => {
              const isSelected = selectedReport?.id === r.id
              const copied = copiedId === r.id
              return (
                <div
                  key={r.id}
                  className={`flex items-center justify-between px-5 py-3 ${isSelected ? 'bg-slate-800/60' : 'hover:bg-slate-800/30'} transition-colors`}
                >
                  <div>
                    <p className="text-sm font-medium text-white">
                      {r.start_date} — {r.end_date}
                    </p>
                    <p className="text-xs text-slate-500 mt-0.5">
                      Generated {new Date(r.created_at).toLocaleDateString()}
                    </p>
                  </div>
                  <div className="flex items-center gap-2">
                    {r.share_token && (
                      <button
                        onClick={() => handleCopyLink(r)}
                        className="flex items-center gap-1.5 rounded-lg border border-slate-700 px-3 py-1.5 text-xs text-slate-300 hover:bg-slate-800 transition-colors"
                      >
                        {copied ? (
                          <><Check className="h-3.5 w-3.5 text-emerald-400" />Copied!</>
                        ) : (
                          <><LinkIcon className="h-3.5 w-3.5" />Copy link</>
                        )}
                      </button>
                    )}
                    <button
                      onClick={() => setSelectedReport(isSelected ? null : r)}
                      className={`rounded-lg px-3 py-1.5 text-xs font-semibold transition-colors ${
                        isSelected
                          ? 'bg-violet-600 text-white'
                          : 'border border-slate-700 text-slate-300 hover:bg-slate-800'
                      }`}
                    >
                      {isSelected ? 'Viewing' : 'View'}
                    </button>
                  </div>
                </div>
              )
            })}
          </div>
        </div>
      )}

      {/* Empty state */}
      {reports.length === 0 && !generating && (
        <div className="rounded-xl border border-slate-800 bg-slate-900 p-16 text-center">
          <div className="mx-auto mb-4 flex h-14 w-14 items-center justify-center rounded-2xl bg-slate-800 border border-slate-700">
            <FileText className="h-6 w-6 text-slate-400" />
          </div>
          <p className="text-sm text-slate-400">
            No reports yet — select a date range and click Generate.
          </p>
        </div>
      )}

      {/* Selected report detail */}
      {selectedReport && <ReportView report={selectedReport} />}
    </div>
  )
}
