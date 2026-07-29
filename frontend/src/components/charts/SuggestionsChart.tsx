import {
  AreaChart,
  Area,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  Legend,
  ResponsiveContainer,
} from 'recharts'

interface DataPoint {
  date: string
  suggestions: number
  acceptances: number
}

const tooltipStyle = {
  backgroundColor: '#0f172a',
  border: '1px solid #1e293b',
  borderRadius: '8px',
  color: '#f8fafc',
}

function fmtDate(val: string) {
  const d = new Date(val + 'T00:00:00')
  return `${d.getMonth() + 1}/${d.getDate()}`
}

export function SuggestionsChart({ data }: { data: DataPoint[] }) {
  return (
    <ResponsiveContainer width="100%" height="100%">
      <AreaChart data={data} margin={{ top: 5, right: 10, left: -20, bottom: 0 }}>
        <defs>
          <linearGradient id="colorSuggestions" x1="0" y1="0" x2="0" y2="1">
            <stop offset="5%" stopColor="#6366f1" stopOpacity={0.3} />
            <stop offset="95%" stopColor="#6366f1" stopOpacity={0} />
          </linearGradient>
          <linearGradient id="colorAcceptances" x1="0" y1="0" x2="0" y2="1">
            <stop offset="5%" stopColor="#8b5cf6" stopOpacity={0.4} />
            <stop offset="95%" stopColor="#8b5cf6" stopOpacity={0} />
          </linearGradient>
        </defs>
        <CartesianGrid strokeDasharray="3 3" stroke="#1e293b" />
        <XAxis
          dataKey="date"
          tick={{ fill: '#64748b', fontSize: 11 }}
          tickLine={false}
          axisLine={false}
          tickFormatter={fmtDate}
        />
        <YAxis tick={{ fill: '#64748b', fontSize: 11 }} tickLine={false} axisLine={false} />
        <Tooltip cursor={{ fill: 'rgba(255,255,255,0.05)' }}
          contentStyle={tooltipStyle}
          labelStyle={{ color: '#f8fafc', marginBottom: '4px' }}
          itemStyle={{ color: '#c4b5fd' }}
        />
        <Legend
          iconType="circle"
          iconSize={8}
          wrapperStyle={{ paddingTop: '16px', fontSize: '12px', color: '#94a3b8' }}
        />
        <Area
          type="monotone"
          dataKey="suggestions"
          stroke="#6366f1"
          fill="url(#colorSuggestions)"
          strokeWidth={2}
          name="Suggestions"
        />
        <Area
          type="monotone"
          dataKey="acceptances"
          stroke="#8b5cf6"
          fill="url(#colorAcceptances)"
          strokeWidth={2}
          name="Acceptances"
        />
      </AreaChart>
    </ResponsiveContainer>
  )
}
