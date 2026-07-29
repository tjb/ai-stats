// Mock data for development when the backend is not running.
// Toggle USE_MOCK to false to use the real API.
// The mock data mirrors the structure returned by the Go backend.

import type {
  DashboardResponse,
  InsightsResponse,
  Report,
  SettingsResponse,
} from './client'

export const USE_MOCK = import.meta.env.VITE_USE_MOCK === 'true'

function daysAgo(n: number): string {
  const d = new Date()
  d.setDate(d.getDate() - n)
  return d.toISOString().split('T')[0]
}

function rng(min: number, max: number) {
  return Math.floor(min + Math.random() * (max - min))
}

// Generate 90 days of time-series data
const days90 = Array.from({ length: 90 }, (_, i) => {
  const baseUsers = rng(38, 55)
  const suggestions = baseUsers * 45 + rng(0, 500)
  const acceptances = Math.floor(suggestions * (0.32 + Math.random() * 0.15))
  return {
    date: daysAgo(89 - i),
    suggestions,
    acceptances,
    users: baseUsers,
    rate: Math.round((acceptances / suggestions) * 1000) / 10,
  }
})

const last30 = days90.slice(-30)
const prev30 = days90.slice(-60, -30)

function sumPeriod(arr: typeof days90) {
  return arr.reduce(
    (acc, d) => ({
      suggestions: acc.suggestions + d.suggestions,
      acceptances: acc.acceptances + d.acceptances,
      linesAccepted: acc.linesAccepted + Math.floor(d.acceptances * 2.8),
      maxUsers: Math.max(acc.maxUsers, d.users),
    }),
    { suggestions: 0, acceptances: 0, linesAccepted: 0, maxUsers: 0 },
  )
}

const t30 = sumPeriod(last30)
const tp = sumPeriod(prev30)

function pctChange(a: number, b: number) {
  if (b === 0) return 0
  return Math.round(((a - b) / b) * 100)
}

export const mockDashboard: DashboardResponse = {
  org: {
    id: 'demo-org',
    name: 'Acme Corp (Demo)',
    github_org_login: 'acme-corp',
    avatar_url: null,
    github_member_count: null,
  },
  metrics: {
    active_users: t30.maxUsers,
    active_users_change: pctChange(t30.maxUsers, tp.maxUsers),
    acceptance_rate: Math.round((t30.acceptances / t30.suggestions) * 1000) / 10,
    acceptance_rate_change:
      Math.round(
        ((t30.acceptances / t30.suggestions - tp.acceptances / tp.suggestions) * 100 * 10),
      ) / 10,
    lines_accepted: t30.linesAccepted,
    lines_accepted_change: pctChange(t30.linesAccepted, tp.linesAccepted),
    hours_saved: Math.round((t30.linesAccepted * 2) / 60 * 10) / 10,
    dollar_value_saved: Math.round((t30.linesAccepted * 2) / 60 * 150),
    hourly_rate: 150,
  },
  charts: {
    acceptance_rate: days90.map((d) => ({ date: d.date, rate: d.rate })),
    active_users: days90.map((d) => ({ date: d.date, users: d.users })),
    suggestions: days90.map((d) => ({
      date: d.date,
      suggestions: d.suggestions,
      acceptances: d.acceptances,
    })),
    languages: [
      { language: 'TypeScript', rate: 42.3 },
      { language: 'Python', rate: 38.7 },
      { language: 'Go', rate: 35.1 },
      { language: 'Rust', rate: 29.4 },
      { language: 'Java', rate: 25.8 },
      { language: 'Other', rate: 21.2 },
    ],
  },
}

export const mockInsights: InsightsResponse = {
  languages: [
    { language: 'TypeScript', suggestions: 42300, acceptances: 17895, lines: 17895, rate: 42.3 },
    { language: 'Python', suggestions: 23500, acceptances: 9095, lines: 9095, rate: 38.7 },
    { language: 'Go', suggestions: 14100, acceptances: 4949, lines: 4949, rate: 35.1 },
    { language: 'Rust', suggestions: 9400, acceptances: 2764, lines: 2764, rate: 29.4 },
    { language: 'Java', suggestions: 6200, acceptances: 1600, lines: 1600, rate: 25.8 },
    { language: 'Other', suggestions: 4000, acceptances: 848, lines: 848, rate: 21.2 },
  ],
  editors: [
    { editor: 'vscode', suggestions: 85000, acceptances: 35700, users: 890, rate: 42.0 },
    { editor: 'jetbrains', suggestions: 14500, acceptances: 4495, users: 150, rate: 31.0 },
    { editor: 'neovim', suggestions: 2000, acceptances: 480, users: 20, rate: 24.0 },
  ],
  activity_heatmap: last30.map((d) => ({ date: d.date, users: d.users })),
  utilization_percent: 68.3,
  ai_active_users: 41,
  total_coding_developers: 60,
  utilization_trend: last30.map((d, i) => ({
    date: d.date,
    utilization_percent: Math.round(((30 + i * 0.8 + Math.sin(i) * 5) / 60) * 1000) / 10,
  })),
  utilization_delta_pp: 3.2,
  utilization_prev_percent: 65.1,
  tool_breakdown: [
    { tool_source: 'copilot', active_users: 35, lines_accepted: 74000 },
    { tool_source: 'codex', active_users: 12, lines_accepted: 18500 },
    { tool_source: 'claude_code', active_users: 18, lines_accepted: 26000 },
  ],
  hours_saved: Math.round((t30.linesAccepted * 2) / 60 * 10) / 10,
  dollar_value_saved: Math.round((t30.linesAccepted * 2) / 60 * 150),
  hourly_rate: 150,
  acceptance_rate: Math.round((t30.acceptances / t30.suggestions) * 1000) / 10,
  lines_accepted: t30.linesAccepted,
  weekly_active_users: 38,
  prev_week_active_users: 32,
  planning_tool_uses: 180,
  coding_tool_uses: 120,
  planning_ratio: 60.0,
  prev_planning_ratio: 52.0,
  planning_trend: [
    { date: '2026-03-20', planning_ratio_percent: 55, planning_uses: 22, coding_uses: 18 },
    { date: '2026-03-21', planning_ratio_percent: 58, planning_uses: 28, coding_uses: 20 },
    { date: '2026-03-22', planning_ratio_percent: 60, planning_uses: 30, coding_uses: 20 },
    { date: '2026-03-23', planning_ratio_percent: 62, planning_uses: 31, coding_uses: 19 },
    { date: '2026-03-24', planning_ratio_percent: 60, planning_uses: 30, coding_uses: 20 },
    { date: '2026-03-25', planning_ratio_percent: 63, planning_uses: 35, coding_uses: 21 },
    { date: '2026-03-26', planning_ratio_percent: 60, planning_uses: 24, coding_uses: 16 },
  ],
}

export const mockReports: Report[] = [
  {
    id: 'report-1',
    month: '2026-02',
    content: {
      month: '2026-02',
      organization_name: 'Acme Corp',
      total_suggestions: 85000,
      total_acceptances: 35700,
      total_lines_accepted: 99960,
      acceptance_rate: 42.0,
      max_active_users: 52,
      hours_saved: 3332,
      dollar_value_saved: 499800,
    },
    share_token: 'share-abc123',
    created_at: '2026-03-01T00:00:00Z',
  },
  {
    id: 'report-2',
    month: '2026-01',
    content: {
      month: '2026-01',
      organization_name: 'Acme Corp',
      total_suggestions: 79000,
      total_acceptances: 31600,
      total_lines_accepted: 88480,
      acceptance_rate: 40.0,
      max_active_users: 48,
      hours_saved: 2949,
      dollar_value_saved: 442400,
    },
    share_token: 'share-def456',
    created_at: '2026-02-01T00:00:00Z',
  },
]

export const mockSettings: SettingsResponse = {
  user: {
    id: 'user-1',
    email: 'demo@acme.com',
    name: 'Demo User',
    image: null,
    hourly_rate: 150,
  },
  org: {
    id: 'demo-org',
    name: 'Acme Corp (Demo)',
    github_org_login: 'acme-corp',
    avatar_url: null,
    github_member_count: null,
  },
}
