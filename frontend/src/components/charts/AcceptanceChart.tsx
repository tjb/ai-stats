import {
  LineChart,
  Line,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
} from 'recharts'

interface DataPoint {
  date: string
  rate: number
}

const tooltipStyle = {
  backgroundColor: '#0f172a',
  border: '1px solid #1e293b',
  borderRadius: '8px',
  color: '#f8fafc',
}

const tickStyle = { fill: '#64748b', fontSize: 11 }

function fmtDate(val: string) {
  const d = new Date(val + 'T00:00:00')
  return `${d.getMonth() + 1}/${d.getDate()}`
}

export function AcceptanceChart({ data }: { data: DataPoint[] }) {
  return (
    <ResponsiveContainer width="100%" height="100%">
      <LineChart data={data} margin={{ top: 5, right: 10, left: -20, bottom: 0 }}>
        <CartesianGrid strokeDasharray="3 3" stroke="#1e293b" />
        <XAxis
          dataKey="date"
          tick={tickStyle}
          tickLine={false}
          axisLine={false}
          tickFormatter={fmtDate}
        />
        <YAxis
          tick={tickStyle}
          tickLine={false}
          axisLine={false}
          tickFormatter={(v: number) => `${v}%`}
        />
        <Tooltip cursor={{ fill: 'rgba(255,255,255,0.05)' }}
          contentStyle={tooltipStyle}
          formatter={(value) => [`${value ?? 0}%`, 'Acceptance Rate']}
          labelStyle={{ color: '#f8fafc', marginBottom: '4px' }}
          itemStyle={{ color: '#c4b5fd' }}
        />
        <Line
          type="monotone"
          dataKey="rate"
          stroke="#8b5cf6"
          strokeWidth={2}
          dot={false}
          activeDot={{ r: 4, fill: '#8b5cf6' }}
        />
      </LineChart>
    </ResponsiveContainer>
  )
}
