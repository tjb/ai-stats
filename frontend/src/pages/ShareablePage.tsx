import { useEffect, useState } from 'react'
import { useParams } from 'react-router-dom'
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

function formatDate(dateStr: string): string {
  return new Date(dateStr + 'T00:00:00').toLocaleDateString('en-US', {
    month: 'long',
    day: 'numeric',
    year: 'numeric',
  })
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

// ---- Report content ----

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
          <StatCard label="Hours Saved" value={c.estimated_hours_saved.toFixed(1) + ' hrs'} />
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

export function ShareablePage() {
  const { token } = useParams<{ token: string }>()
  const [report, setReport] = useState<Report | null>(null)
  const [fetchFailed, setFetchFailed] = useState(false)
  // A missing token is known at render time; only a failed fetch needs state.
  const error = !token || fetchFailed

  useEffect(() => {
    if (!token) return
    reportsApi
      .getByShareToken(token)
      .then(setReport)
      .catch(() => setFetchFailed(true))
  }, [token])

  return (
    <div className="min-h-screen bg-slate-950 py-12 px-6">
      <div className="mx-auto max-w-4xl">
        {/* Header */}
        <div className="mb-8">
          {/* Logo */}
          <div className="flex items-center gap-3 mb-6">
            <div className="h-8 w-8 rounded-lg bg-gradient-to-br from-violet-500 to-purple-600 flex items-center justify-center flex-shrink-0">
              <span className="text-sm font-bold text-white">A</span>
            </div>
            <span className="text-lg font-semibold text-white">AI Stats</span>
          </div>

          {/* Report title and date range */}
          {report && (
            <>
              <h1 className="text-2xl font-bold text-white">AI Development Report</h1>
              <p className="text-sm text-slate-400 mt-1">
                {formatDate(report.content.start_date)} &ndash; {formatDate(report.content.end_date)}
              </p>
            </>
          )}
        </div>

        {/* Loading state */}
        {!report && !error && (
          <div className="flex items-center justify-center py-24">
            <p className="text-slate-400 text-sm">Loading report&hellip;</p>
          </div>
        )}

        {/* Error state */}
        {error && (
          <div className="flex items-center justify-center py-24">
            <p className="text-slate-400 text-sm">
              This report link is invalid or has been removed.
            </p>
          </div>
        )}

        {/* Report content */}
        {report && <ReportView report={report} />}

        {/* Footer */}
        <div className="mt-12 text-center">
          <p className="text-xs text-slate-500">Powered by AI Stats</p>
        </div>
      </div>
    </div>
  )
}
