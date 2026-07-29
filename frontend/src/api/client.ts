// All API calls go through this module.
// The Vite dev server proxies /api/* to http://localhost:8080
// so no CORS configuration is needed in development.

const BASE = ''

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(BASE + path, {
    credentials: 'include',
    headers: { 'Content-Type': 'application/json', ...init?.headers },
    ...init,
  })

  if (res.status === 401) {
    // Redirect to login
    window.location.href = '/login'
    return Promise.reject(new Error('Unauthorized'))
  }

  if (!res.ok) {
    const body = await res.json().catch(() => ({ error: res.statusText }))
    throw new Error(body.error ?? `HTTP ${res.status}`)
  }

  return res.json()
}

export const api = {
  get: <T>(path: string) => request<T>(path),
  post: <T>(path: string, body?: unknown) =>
    request<T>(path, { method: 'POST', body: JSON.stringify(body) }),
  delete: <T>(path: string) => request<T>(path, { method: 'DELETE' }),
}

// ---- Response types matching the Go backend ----

export interface User {
  id: string
  email: string
  name: string | null
  image: string | null
  hourly_rate: number
}

export interface OrgSummary {
  id: string
  name: string
  github_org_login: string
  avatar_url: string | null
  github_member_count: number | null
}

export interface DashboardMetrics {
  active_users: number
  active_users_change: number
  acceptance_rate: number
  acceptance_rate_change: number
  lines_accepted: number
  lines_accepted_change: number
  hours_saved: number
  dollar_value_saved: number
  hourly_rate: number
}

export interface ChartPoint { date: string; rate: number }
export interface ActiveUserPoint { date: string; users: number }
export interface SuggestionPoint { date: string; suggestions: number; acceptances: number }
export interface LanguageRatePoint { language: string; rate: number }

export interface DashboardCharts {
  acceptance_rate: ChartPoint[]
  active_users: ActiveUserPoint[]
  suggestions: SuggestionPoint[]
  languages: LanguageRatePoint[]
}

export interface DashboardResponse {
  org: OrgSummary
  metrics: DashboardMetrics
  charts: DashboardCharts
}

export interface LanguageStat {
  language: string
  suggestions: number
  acceptances: number
  lines: number
  rate: number
}

export interface EditorStat {
  editor: string
  suggestions: number
  acceptances: number
  users: number
  rate: number
}

export interface HeatmapPoint { date: string; users: number }

export interface UtilizationTrendPoint { date: string; utilization_percent: number }
export interface ToolBreakdownRow { tool_source: string; active_users: number; lines_accepted: number }

export interface PlanningTrendPoint {
  date: string
  planning_ratio_percent: number
  planning_uses: number
  coding_uses: number
}

export interface InsightsResponse {
  languages: LanguageStat[]
  editors: EditorStat[]
  activity_heatmap: HeatmapPoint[]
  utilization_percent: number
  ai_active_users: number
  total_coding_developers: number
  utilization_trend: UtilizationTrendPoint[]
  utilization_delta_pp: number
  utilization_prev_percent: number
  tool_breakdown: ToolBreakdownRow[]
  hours_saved: number
  dollar_value_saved: number
  hourly_rate: number
  acceptance_rate: number
  lines_accepted: number
  weekly_active_users: number
  prev_week_active_users: number
  planning_tool_uses: number
  coding_tool_uses: number
  planning_ratio: number
  prev_planning_ratio: number
  planning_trend: PlanningTrendPoint[]
}

export interface ReportContent {
  // Executive Summary
  total_accepted_lines: number
  total_lines_written: number
  active_developers: number
  estimated_hours_saved: number
  estimated_value_saved: number

  // Adoption
  adoption_rate: number | null
  github_member_count: number | null

  // Quality Signal
  acceptance_rate: number
  total_suggested_lines: number

  // ROI Detail
  hourly_rate: number

  // Planning vs Coding
  planning_tool_uses: number
  coding_tool_uses: number
  planning_ratio: number

  // Date range echo
  start_date: string
  end_date: string
}

export interface Report {
  id: string
  start_date: string
  end_date: string
  share_token: string | null
  content: ReportContent
  created_at: string
}

export interface SettingsResponse {
  user: {
    id: string
    email: string
    name: string | null
    image: string | null
    hourly_rate: number
  }
  org: OrgSummary | null
}

export interface SyncAllResult {
  tool_source: string
  synced: number
  error?: string
}

export interface SyncAllResponse {
  results: SyncAllResult[]
}

export interface IngestKey {
  id: string
  label: string
  created_at: string
  last_used_at: string | null
}

export interface CreateIngestKeyResponse {
  id: string
  key: string
  label: string
  created_at: string
}

export const ingestKeysApi = {
  listIngestKeys: () => api.get<IngestKey[]>('/api/settings/ingest-keys'),
  createIngestKey: (label: string) =>
    api.post<CreateIngestKeyResponse>('/api/settings/ingest-keys', { label }),
  deleteIngestKey: (id: string) =>
    api.delete<{ ok: boolean }>(`/api/settings/ingest-keys/${id}`),
}

export const reportsApi = {
  list: () => api.get<Report[]>('/api/reports'),
  generate: (startDate: string, endDate: string) =>
    api.post<Report>('/api/reports/generate', { start_date: startDate, end_date: endDate }),
  getByShareToken: (token: string) =>
    api.get<Report>(`/api/reports/share/${token}`),
}
